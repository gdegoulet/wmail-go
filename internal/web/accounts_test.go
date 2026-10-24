package web

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"wmail-go/internal/config"
	"wmail-go/internal/mailconn/mailtest"
)

type mailPorts struct{ imap, smtp int }

func startMail(t *testing.T) mailPorts {
	t.Helper()
	return mailPorts{imap: mailtest.StartIMAP(t, "none"), smtp: mailtest.StartSMTP(t, "none")}
}

// accountForm renvoie un formulaire de compte valide pour les serveurs de test.
func accountForm(tok string, p mailPorts, action string) url.Values {
	return url.Values{
		"csrf": {tok}, "action": {action}, "email": {"test@localhost"}, "display_name": {"Test"},
		"imap_host": {"127.0.0.1"}, "imap_port": {strconv.Itoa(p.imap)}, "imap_security": {"none"},
		"imap_login": {mailtest.User}, "imap_password": {mailtest.Password},
		"smtp_host": {"127.0.0.1"}, "smtp_port": {strconv.Itoa(p.smtp)}, "smtp_security": {"none"},
		"block_remote": {"on"},
	}
}

// loggedIn crée un compte webmail connecté, clé de récupération confirmée, et renvoie le jeton CSRF.
func (e *env) loggedIn(login string) (c *httpClient, tok string) {
	e.t.Helper()
	cl := e.client()
	e.register(cl, login)
	_, page := e.do(cl, "GET", "/accounts/new", nil, nil)
	return cl, csrf(e.t, page)
}

func TestFirstAccountFlow(t *testing.T) {
	e := newEnv(t, nil)
	ports := startMail(t)
	c, tok := e.loggedIn("alice")

	resp, page := e.do(c, "GET", "/accounts/new", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "premier compte mail") {
		t.Error("texte d'accueil du premier compte attendu")
	}

	// Test seul : n'enregistre rien.
	resp, page = e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "test"), nil)
	e.expect(resp, 200, "")
	if strings.Count(page, "connexion et authentification réussies") != 2 {
		t.Fatalf("IMAP et SMTP doivent réussir :\n%s", page)
	}
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/accounts/new")

	// Le premier compte n'est pas enregistré si le test échoue.
	bad := accountForm(tok, ports, "save")
	bad.Set("imap_password", "faux")
	resp, page = e.do(c, "POST", "/accounts/save", bad, nil)
	e.expect(resp, 422, "")
	if !strings.Contains(page, "identifiant ou mot de passe refusé") || !strings.Contains(page, "doit réussir le test") {
		t.Errorf("échec d'authentification attendu :\n%s", page)
	}
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/accounts/new")

	// Enregistrement : SMTP sans identifiants = ceux de l'IMAP (le test SMTP passe).
	resp, _ = e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil)
	e.expect(resp, 303, "/")

	resp, page = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 200, "")
	for _, want := range []string{"Boîte de réception", "Projets", "2026", "Envoyés", "Corbeille"} {
		if !strings.Contains(page, want) && want != "Envoyés" && want != "Corbeille" {
			t.Errorf("dossier %q absent :\n%s", want, page)
		}
	}
	if !strings.Contains(page, "Sent") || !strings.Contains(page, "Trash") {
		t.Errorf("dossiers Sent et Trash attendus (pas d'attribut spécial sur ce serveur) :\n%s", page)
	}

	// Le compte survit à une nouvelle connexion (fichier chiffré réécrit).
	c2 := e.client()
	e.expect(e.login(c2, "alice", pw), 303, "/")
	resp, page = e.do(c2, "GET", "/", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "Projets") {
		t.Error("liste des dossiers absente après reconnexion")
	}
}

func TestAccountValidation(t *testing.T) {
	e := newEnv(t, nil)
	ports := startMail(t)
	c, tok := e.loggedIn("alice")

	cases := map[string]func(url.Values){
		"adresse invalide":    func(v url.Values) { v.Set("email", "pas une adresse") },
		"hôte invalide":       func(v url.Values) { v.Set("imap_host", "http://x/y") },
		"port invalide":       func(v url.Values) { v.Set("smtp_port", "70000") },
		"sécurité inconnue":   func(v url.Values) { v.Set("imap_security", "ssl3") },
		"mot de passe absent": func(v url.Values) { v.Del("imap_password") },
		"identifiant absent":  func(v url.Values) { v.Del("imap_login") },
	}
	for name, mut := range cases {
		v := accountForm(tok, ports, "save")
		mut(v)
		resp, page := e.do(c, "POST", "/accounts/save", v, nil)
		if resp.StatusCode != 400 {
			t.Errorf("%s : statut %d, attendu 400", name, resp.StatusCode)
		}
		if !strings.Contains(page, `role="alert"`) {
			t.Errorf("%s : message d'erreur absent", name)
		}
	}
	resp, _ := e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/accounts/new") // rien n'a été enregistré
}

func TestAccountCSRF(t *testing.T) {
	e := newEnv(t, nil)
	ports := startMail(t)
	c, _ := e.loggedIn("alice")
	v := accountForm("mauvais", ports, "save")
	resp, _ := e.do(c, "POST", "/accounts/save", v, nil)
	e.expect(resp, 403, "")
}

func TestSecondAccountEditDeleteSelect(t *testing.T) {
	e := newEnv(t, nil)
	ports := startMail(t)
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil))

	// Un deuxième compte s'enregistre sans test, même injoignable.
	second := accountForm(tok, ports, "save")
	second.Set("name", "Cassé")
	second.Set("imap_port", "1")
	resp, _ := e.do(c, "POST", "/accounts/save", second, nil)
	e.expect(resp, 303, "/")

	_, page := e.do(c, "GET", "/accounts", nil, nil)
	if strings.Count(page, "/edit") != 2 || !strings.Contains(page, "Cassé") {
		t.Fatalf("deux comptes attendus :\n%s", page)
	}
	ids := idsFrom(page)
	if len(ids) != 2 {
		t.Fatalf("identifiants %v", ids)
	}

	// Changement de compte actif : le compte cassé affiche une erreur claire et un lien de correction.
	resp, _ = e.do(c, "POST", "/accounts/select", url.Values{"csrf": {tok}, "id": {ids[1]}}, nil)
	e.expect(resp, 303, "/")
	_, page = e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, "connexion refusée") || !strings.Contains(page, "/accounts/"+ids[1]+"/edit") {
		t.Errorf("erreur de compte attendue :\n%s", page)
	}
	resp, _ = e.do(c, "POST", "/accounts/select", url.Values{"csrf": {tok}, "id": {"inconnu"}}, nil)
	e.expect(resp, 404, "")

	// Modification : le mot de passe n'est jamais renvoyé dans la page, et vide = conservé.
	resp, page = e.do(c, "GET", "/accounts/"+ids[1]+"/edit", nil, nil)
	e.expect(resp, 200, "")
	if strings.Contains(page, mailtest.Password) {
		t.Error("le mot de passe stocké ne doit pas apparaître dans la page")
	}
	fix := accountForm(tok, ports, "save")
	fix.Set("id", ids[1])
	fix.Set("name", "Réparé")
	fix.Del("imap_password")
	fix.Del("smtp_password")
	resp, _ = e.do(c, "POST", "/accounts/save", fix, nil)
	e.expect(resp, 303, "/")
	_, page = e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, "Projets") {
		t.Errorf("le compte réparé doit lister ses dossiers :\n%s", page)
	}
	_, page = e.do(c, "GET", "/accounts", nil, nil)
	if !strings.Contains(page, "Réparé") || strings.Contains(page, "Cassé") || strings.Count(page, "/edit") != 2 {
		t.Errorf("modification mal appliquée :\n%s", page)
	}

	// Suppression.
	resp, _ = e.do(c, "POST", "/accounts/"+ids[0]+"/delete", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 303, "/accounts")
	resp, _ = e.do(c, "POST", "/accounts/"+ids[1]+"/delete", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 303, "/accounts")
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/accounts/new")
	resp, _ = e.do(c, "POST", "/accounts/"+ids[0]+"/delete", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 404, "")
}

func TestAccountsAreIsolatedBetweenUsers(t *testing.T) {
	e := newEnv(t, nil)
	ports := startMail(t)
	a, tokA := e.loggedIn("alice")
	e.expect1(e.do(a, "POST", "/accounts/save", accountForm(tokA, ports, "save"), nil))
	_, page := e.do(a, "GET", "/accounts", nil, nil)
	id := idsFrom(page)[0]

	b, tokB := e.loggedIn("bob")
	resp, _ := e.do(b, "GET", "/accounts/"+id+"/edit", nil, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(b, "POST", "/accounts/"+id+"/delete", url.Values{"csrf": {tokB}}, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(b, "POST", "/accounts/select", url.Values{"csrf": {tokB}, "id": {id}}, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(a, "GET", "/accounts/"+id+"/edit", nil, nil)
	e.expect(resp, 200, "")
}

func TestPrivateNetworkBlockedInForm(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.AllowedPrivateHosts = nil })
	ports := startMail(t)
	c, tok := e.loggedIn("alice")
	resp, page := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "test"), nil)
	e.expect(resp, 200, "")
	if strings.Count(page, "réseau privé non autorisé") != 2 {
		t.Fatalf("le réseau privé doit être refusé pour IMAP et SMTP :\n%s", page)
	}
	resp, _ = e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil)
	e.expect(resp, 422, "")
}

func TestMailTestRateLimit(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.RateLimit.MailTest = config.RateRule{
			PerIP:    config.Limit{Count: 100, Window: time.Hour},
			PerLogin: config.Limit{Count: 2, Window: time.Hour},
		}
	})
	ports := startMail(t)
	c, tok := e.loggedIn("alice")
	for range 2 {
		resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "test"), nil)
		e.expect(resp, 200, "")
	}
	resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "test"), nil)
	e.expect(resp, 429, "")
	if resp.Header.Get("Retry-After") == "" {
		t.Error("Retry-After attendu")
	}
}

func TestAccountLimit(t *testing.T) {
	e := newEnv(t, nil)
	ports := startMail(t)
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil))
	for range maxMailAccounts - 1 {
		resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil)
		e.expect(resp, 303, "/")
	}
	resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil)
	e.expect(resp, 400, "")
}

// Certificat auto-signé : refusé tant que « Vérifier le certificat » est coché, accepté une fois décoché (IMAP et SMTP).
func TestSelfSignedCertificateOption(t *testing.T) {
	e := newEnv(t, nil)
	ports := mailPorts{imap: mailtest.StartIMAP(t, "tls"), smtp: mailtest.StartSMTP(t, "tls")}
	c, tok := e.loggedIn("alice")
	form := func(verifyIMAP, verifySMTP bool) url.Values {
		v := accountForm(tok, ports, "test")
		v.Set("imap_security", "tls")
		v.Set("smtp_security", "tls")
		if verifyIMAP {
			v.Set("imap_verify", "on")
		}
		if verifySMTP {
			v.Set("smtp_verify", "on")
		}
		return v
	}
	_, page := e.do(c, "GET", "/accounts/new", nil, nil)
	if !strings.Contains(page, `name="imap_verify" type="checkbox" checked`) || !strings.Contains(page, `name="smtp_verify" type="checkbox" checked`) {
		t.Error("la vérification du certificat doit être cochée par défaut (IMAP et SMTP)")
	}
	if strings.Count(page, "certificat auto-signé, expiré") != 2 {
		t.Error("un texte d'aide doit accompagner chacune des deux cases")
	}

	_, page = e.do(c, "POST", "/accounts/save", form(true, true), nil)
	if strings.Count(page, "décochez") < 2 || strings.Contains(page, "connexion et authentification réussies") {
		t.Errorf("avec vérification, le certificat auto-signé doit être refusé pour IMAP et SMTP, avec l'indication de décocher :\n%.3000s", page)
	}
	// Chaque serveur est réglé séparément.
	_, page = e.do(c, "POST", "/accounts/save", form(false, true), nil)
	if strings.Count(page, "connexion et authentification réussies") != 1 {
		t.Errorf("IMAP sans vérification doit réussir, SMTP avec vérification doit échouer :\n%.3000s", page)
	}
	_, page = e.do(c, "POST", "/accounts/save", form(true, false), nil)
	if strings.Count(page, "connexion et authentification réussies") != 1 {
		t.Error("SMTP sans vérification doit réussir, IMAP avec vérification doit échouer")
	}
	_, page = e.do(c, "POST", "/accounts/save", form(false, false), nil)
	if strings.Count(page, "connexion et authentification réussies") != 2 {
		t.Errorf("sans vérification, IMAP et SMTP doivent réussir :\n%.3000s", page)
	}

	// Enregistrement : le réglage est conservé et reproposé à la modification.
	save := form(false, false)
	save.Set("action", "save")
	resp, _ := e.do(c, "POST", "/accounts/save", save, nil)
	e.expect(resp, 303, "/")
	_, list := e.do(c, "GET", "/accounts", nil, nil)
	edit := htmlUnescape(regexp.MustCompile(`href="(/accounts/[0-9a-f]{24}/edit)"`).FindStringSubmatch(list)[1])
	_, page = e.do(c, "GET", edit, nil, nil)
	if strings.Contains(page, `name="imap_verify" type="checkbox" checked`) || strings.Contains(page, `name="smtp_verify" type="checkbox" checked`) {
		t.Error("les cases décochées doivent rester décochées à la modification")
	}
	// La liste des dossiers fonctionne avec le certificat auto-signé.
	_, home := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(home, "Boîte de réception") {
		t.Errorf("la boîte doit s'ouvrir sans vérification du certificat :\n%.1500s", home)
	}
}
