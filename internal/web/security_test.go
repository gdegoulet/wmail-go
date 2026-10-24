package web

// Contrôles de sécurité de bout en bout, demandés dans la matrice de test (J-01 à J-04) : un message piégé est
// réellement servi par le serveur, puis la réponse est examinée. Aucun accès à une vraie boîte : GreenMail seulement.

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/config"
	"wmail-go/internal/mailconn/mailtest"
)

// piegeMessage fabrique un message HTML contenant les pièges classiques : script, gestionnaire en ligne,
// lien javascript:, iframe, objet, image distante et pixel espion.
func piegeMessage() []byte {
	return []byte("From: Pirate <pirate@example.org>\r\n" +
		"To: alice@example.org\r\n" +
		"Subject: Piege\r\n" +
		"Date: Mon, 02 Mar 2026 10:00:00 +0100\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		`<html><body><p>bonjour</p>` +
		`<script>window.vole=1</script>` +
		`<img src="x" onerror="window.vole=2">` +
		`<a href="javascript:window.vole=3">clic</a>` +
		`<iframe src="https://pisteur.example/x"></iframe>` +
		`<object data="https://pisteur.example/o"></object>` +
		`<img src="https://pisteur.example/grand.png" width="400" height="300">` +
		`<img src="https://pisteur.example/pixel.gif" width="1" height="1">` +
		`<form action="https://pirate.example/vol"><input name="mdp" type="password"></form>` +
		`</body></html>` + "\r\n")
}

// ouvreMessagePiege rend le corps du message piégé et renvoie le HTML servi au navigateur.
func ouvreMessagePiege(t *testing.T, block bool) (string, *env, *httpClient) {
	t.Helper()
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add(piegeMessage(), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	form := accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save")
	if !block {
		form.Del("block_remote")
	}
	e.expect1(e.do(c, "POST", "/accounts/save", form, nil))
	_, page := e.do(c, "GET", "/?q=Piege&scope=subject", nil, nil)
	m := regexp.MustCompile(`uid=(\d+)`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("message piégé introuvable :\n%s", page)
	}
	_, body := e.do(c, "GET", "/mail/body?folder=INBOX&uid="+m[1], nil, nil)
	return body, e, c
}

// J-01 : rien d'exécutable ne doit sortir du serveur, et le document du corps est confiné.
func TestSecurityMessageScriptsNeutralised(t *testing.T) {
	body, e, c := ouvreMessagePiege(t, true)
	for _, interdit := range []string{"<script", "onerror=", "javascript:", "<iframe", "<object", "window.vole"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(interdit)) {
			t.Errorf("%q présent dans le corps servi :\n%s", interdit, body)
		}
	}
	if !strings.Contains(body, "bonjour") {
		t.Error("le texte légitime du message doit rester")
	}
	// Le document du corps est servi avec sa propre politique, et la page qui l'héberge l'isole.
	resp, _ := e.do(c, "GET", "/mail/body?folder=INBOX&uid=1", nil, nil)
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") { // couvre script-src, qui n'est donc pas répété
		t.Errorf("politique du corps trop permissive : %q", csp)
	}
	if !strings.Contains(csp, "sandbox ") || strings.Contains(csp, "allow-scripts") {
		t.Errorf("le corps doit être servi en bac à sable sans allow-scripts : %q", csp)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	_, page := e.do(c, "GET", "/?uid=1", nil, nil)
	if !strings.Contains(page, `sandbox="allow-popups allow-popups-to-escape-sandbox"`) {
		t.Error("le corps doit être affiché dans une iframe en bac à sable, sans allow-scripts")
	}
}

// J-02 : images distantes et pixels espions.
func TestSecurityRemoteImagesAndTrackers(t *testing.T) {
	body, _, _ := ouvreMessagePiege(t, true)
	if strings.Contains(body, "pisteur.example") {
		t.Errorf("une référence distante a survécu au blocage :\n%s", body)
	}
	if !strings.Contains(body, "data-blocked") {
		t.Error("les images bloquées doivent être signalées (data-blocked)")
	}
	// Images autorisées : le pixel espion (1×1) reste retiré, la grande image passe.
	body, _, _ = ouvreMessagePiege(t, false)
	if strings.Contains(body, "pixel.gif") {
		t.Error("le pixel espion doit être retiré même quand les images distantes sont autorisées")
	}
	if !strings.Contains(body, "grand.png") {
		t.Errorf("l'image légitime doit s'afficher quand l'utilisateur l'autorise :\n%s", body)
	}
}

// J-03 : un POST venant d'un autre site est refusé (jeton CSRF et contrôle d'origine).
func TestSecurityCSRFRejected(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	// Jeton absent ou faux.
	for _, v := range []url.Values{{}, {"csrf": {"faux"}}} {
		v.Set("language", "fr")
		resp, _ := e.do(c, "POST", "/settings/preferences", v, nil)
		if resp.StatusCode != 403 {
			t.Errorf("POST sans jeton valable : statut %d, attendu 403", resp.StatusCode)
		}
	}
	// Jeton correct mais requête annoncée comme venant d'un autre site.
	resp, _ := e.do(c, "POST", "/settings/preferences", url.Values{"csrf": {tok}, "language": {"fr"}},
		map[string]string{"Sec-Fetch-Site": "cross-site"})
	if resp.StatusCode != 403 {
		t.Errorf("POST inter-sites : statut %d, attendu 403", resp.StatusCode)
	}
}

// J-04 : un serveur IMAP sur une adresse privée est refusé, sauf s'il est explicitement autorisé.
func TestSecurityPrivateNetworkBlocked(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.AllowedPrivateHosts = nil })
	ports := startMail(t)
	c, tok := e.loggedIn("alice")
	_, page := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "test"), nil)
	if n := strings.Count(page, "réseau privé non autorisé"); n < 2 {
		t.Errorf("le réseau privé doit être refusé pour IMAP et SMTP (%d refus) :\n%s", n, page)
	}
	// Autorisé explicitement : la connexion est permise.
	e2 := newEnv(t, func(c *config.Config) { c.AllowedPrivateHosts = []string{"127.0.0.1"} })
	c2, tok2 := e2.loggedIn("bob")
	_, page = e2.do(c2, "POST", "/accounts/save", accountForm(tok2, ports, "test"), nil)
	if strings.Contains(page, "réseau privé non autorisé") {
		t.Errorf("l'hôte autorisé ne doit plus être refusé :\n%s", page)
	}
}
