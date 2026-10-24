package web

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"wmail-go/internal/totp"
)

var (
	secretRe  = regexp.MustCompile(`<code>([A-Z2-7]+)</code>`)
	backupRe  = regexp.MustCompile(`(?m)^(\d{4}-\d{4}-\d{2})$`)
	pendingRe = regexp.MustCompile(`wm_2fa=`)
)

func TestTwoFactorFullFlow(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")

	// activation : secret, mauvais code, bon code
	resp, _ := e.do(c, "POST", "/settings/2fa/start", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 303, "/settings/2fa")
	_, page := e.do(c, "GET", "/settings/2fa", nil, nil)
	m := secretRe.FindStringSubmatch(page)
	if m == nil || !strings.Contains(page, "/settings/2fa/qr.png") {
		t.Fatalf("page d'installation :\n%s", page)
	}
	secret := m[1]
	resp, png := e.do(c, "GET", "/settings/2fa/qr.png", nil, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(png, "\x89PNG") {
		t.Errorf("QR : %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp, page = e.do(c, "POST", "/settings/2fa/enable", url.Values{"csrf": {tok}, "code": {"000000"}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "Code incorrect") {
		t.Fatalf("mauvais code : %d", resp.StatusCode)
	}
	step := totp.Step(time.Now())
	code, _ := totp.Code(secret, step)
	resp, page = e.do(c, "POST", "/settings/2fa/enable", url.Values{"csrf": {tok}, "code": {code}}, nil)
	codes := backupRe.FindAllStringSubmatch(page, -1)
	if resp.StatusCode != 200 || len(codes) != totp.BackupCount {
		t.Fatalf("activation : %d, %d codes de secours\n%s", resp.StatusCode, len(codes), page)
	}
	// le QR n'est plus servi une fois activé
	if resp, _ = e.do(c, "GET", "/settings/2fa/qr.png", nil, nil); resp.StatusCode != 404 {
		t.Errorf("QR encore servi : %d", resp.StatusCode)
	}

	// nouvelle connexion : mot de passe seul → demande du code, pas de session
	c2 := e.client()
	resp, page = e.do(c2, "POST", "/login", url.Values{"login": {"alice"}, "password": {pw}}, nil)
	if resp.StatusCode != 200 || !strings.Contains(page, `action="/login/2fa"`) || !pendingRe.MatchString(strings.Join(resp.Header.Values("Set-Cookie"), ";")) {
		t.Fatalf("étape du code absente : %d", resp.StatusCode)
	}
	if r, _ := e.do(c2, "GET", "/", nil, nil); r.StatusCode != 303 || !strings.HasPrefix(r.Header.Get("Location"), "/login") {
		t.Error("session ouverte sans le second facteur")
	}
	resp, page = e.do(c2, "POST", "/login/2fa", url.Values{"code": {"123456"}}, nil)
	if resp.StatusCode != 401 || !strings.Contains(page, "Code incorrect") {
		t.Fatalf("faux code : %d", resp.StatusCode)
	}
	// le code utilisé pour l'activation est refusé (usage unique) ; le pas suivant est accepté
	old, _ := totp.Code(secret, step)
	if resp, _ = e.do(c2, "POST", "/login/2fa", url.Values{"code": {old}}, nil); resp.StatusCode != 401 {
		t.Errorf("code réutilisé accepté : %d", resp.StatusCode)
	}
	next, _ := totp.Code(secret, step+1)
	resp, _ = e.do(c2, "POST", "/login/2fa", url.Values{"code": {next}, "trust": {"on"}}, nil)
	e.expect(resp, 303, "/")
	if r, _ := e.do(c2, "GET", "/settings", nil, nil); r.StatusCode != 200 {
		t.Errorf("session non ouverte : %d", r.StatusCode)
	}

	// « ne plus demander » : la même identité de navigateur se connecte avec le mot de passe seul
	e.do(c2, "POST", "/logout", url.Values{"csrf": {csrfOf(t, e, c2)}}, nil)
	resp, _ = e.do(c2, "POST", "/login", url.Values{"login": {"alice"}, "password": {pw}}, nil)
	e.expect(resp, 303, "/")
	// un autre navigateur (sans cookie) doit encore donner un code ; un code de secours sert une fois
	c3 := e.client()
	e.do(c3, "POST", "/login", url.Values{"login": {"alice"}, "password": {pw}}, nil)
	resp, _ = e.do(c3, "POST", "/login/2fa", url.Values{"code": {codes[0][1]}}, nil)
	e.expect(resp, 303, "/")
	c4 := e.client()
	e.do(c4, "POST", "/login", url.Values{"login": {"alice"}, "password": {pw}}, nil)
	if resp, _ = e.do(c4, "POST", "/login/2fa", url.Values{"code": {codes[0][1]}}, nil); resp.StatusCode != 401 {
		t.Errorf("code de secours réutilisé : %d", resp.StatusCode)
	}

	// désactivation avec un code de secours : la connexion ne demande plus rien
	tok3 := csrfOf(t, e, c3)
	resp, page = e.do(c3, "POST", "/settings/2fa/disable", url.Values{"csrf": {tok3}, "code": {codes[1][1]}}, nil)
	if resp.StatusCode != 200 || strings.Contains(page, "est activée") {
		t.Fatalf("désactivation : %d", resp.StatusCode)
	}
	c5 := e.client()
	resp, _ = e.do(c5, "POST", "/login", url.Values{"login": {"alice"}, "password": {pw}}, nil)
	e.expect(resp, 303, "/")
}

func csrfOf(t *testing.T, e *env, c *httpClient) string {
	t.Helper()
	_, page := e.do(c, "GET", "/settings", nil, nil)
	return csrf(t, page)
}

func TestTwoFactorPendingExpiresAndLimitsTries(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("bob")
	e.do(c, "POST", "/settings/2fa/start", url.Values{"csrf": {tok}}, nil)
	_, page := e.do(c, "GET", "/settings/2fa", nil, nil)
	secret := secretRe.FindStringSubmatch(page)[1]
	code, _ := totp.Code(secret, totp.Step(time.Now()))
	e.do(c, "POST", "/settings/2fa/enable", url.Values{"csrf": {tok}, "code": {code}}, nil)

	c2 := e.client()
	e.do(c2, "POST", "/login", url.Values{"login": {"bob"}, "password": {pw}}, nil)
	for i := 0; i < pendingTries; i++ {
		if resp, _ := e.do(c2, "POST", "/login/2fa", url.Values{"code": {"111111"}}, nil); resp.StatusCode != 401 {
			t.Fatalf("essai %d : %d", i, resp.StatusCode)
		}
	}
	// au-delà : l'étape est détruite, même le bon code ne passe plus
	next, _ := totp.Code(secret, totp.Step(time.Now())+1)
	resp, page := e.do(c2, "POST", "/login/2fa", url.Values{"code": {next}}, nil)
	if resp.StatusCode != 401 || !strings.Contains(page, "expiré") {
		t.Errorf("après trop d'essais : %d", resp.StatusCode)
	}
	// sans cookie d'étape : retour à la connexion
	if resp, _ := e.do(e.client(), "POST", "/login/2fa", url.Values{"code": {next}}, nil); resp.StatusCode != 303 {
		t.Errorf("sans étape : %d", resp.StatusCode)
	}
}

// Revoir le secret : refusé sans code valide, affiché une seule fois ensuite.
func TestTwoFactorShowSecretAgain(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	e.do(c, "POST", "/settings/2fa/start", url.Values{"csrf": {tok}}, nil)
	_, page := e.do(c, "GET", "/settings/2fa", nil, nil)
	secret := regexp.MustCompile(`<code>([A-Z2-7]+)</code>`).FindStringSubmatch(page)
	if secret == nil {
		t.Fatalf("secret absent :\n%s", page)
	}
	step := totp.Step(time.Now())
	code, _ := totp.Code(secret[1], step)
	if r, _ := e.do(c, "POST", "/settings/2fa/enable", url.Values{"csrf": {tok}, "code": {code}}, nil); r.StatusCode != 200 {
		t.Fatalf("activation : statut %d", r.StatusCode)
	}

	resp, page := e.do(c, "POST", "/settings/2fa/show", url.Values{"csrf": {tok}, "code": {"000000"}}, nil)
	if resp.StatusCode != 401 || strings.Contains(page, secret[1]) {
		t.Fatalf("code invalide : statut %d, le secret ne doit pas apparaître", resp.StatusCode)
	}
	code2, _ := totp.Code(secret[1], step+1) // pas suivant : le premier a servi à activer
	resp, _ = e.do(c, "POST", "/settings/2fa/show", url.Values{"csrf": {tok}, "code": {code2}}, nil)
	e.expect(resp, 303, "/settings/2fa")
	_, page = e.do(c, "GET", "/settings/2fa", nil, nil)
	if !strings.Contains(page, secret[1]) {
		t.Errorf("le secret doit être affiché après un code valide :\n%s", page)
	}
	_, page = e.do(c, "GET", "/settings/2fa", nil, nil)
	if strings.Contains(page, secret[1]) {
		t.Error("le secret ne doit être affiché qu'une fois")
	}
}
