package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"wmail-go/internal/config"
	"wmail-go/internal/store"
	"wmail-go/internal/vault"
)

const pw = "un mot de passe assez long"

type env struct {
	t   *testing.T
	cfg config.Config
	srv *Server
	ts  *httptest.Server
}

func newEnv(t *testing.T, mutate func(*config.Config)) *env {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.AllowedPrivateHosts = []string{"127.0.0.1"} // les serveurs de test écoutent en local
	cfg.RateLimit.Login = config.RateRule{PerIP: config.Limit{Count: 100, Window: time.Hour}, PerLogin: config.Limit{Count: 100, Window: time.Hour}}
	if mutate != nil {
		mutate(&cfg)
	}
	svc, err := vault.NewService(vault.Params{Time: 1, MemoryKiB: 64, Threads: 1}, 4)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(cfg.DataDir, svc, store.Options{MaxUsers: cfg.MaxUsers, PasswordMinLength: cfg.PasswordMinLength})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &env{t: t, cfg: cfg, srv: srv, ts: ts}
}

// client suit les redirections à la main pour vérifier chaque étape.
func (e *env) client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (e *env) do(c *http.Client, method, path string, form url.Values, hdr map[string]string) (*http.Response, string) {
	e.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, e.ts.URL+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func (e *env) expect(resp *http.Response, status int, loc string) {
	e.t.Helper()
	if resp.StatusCode != status {
		e.t.Fatalf("statut %d, attendu %d (%s %s)", resp.StatusCode, status, resp.Request.Method, resp.Request.URL.Path)
	}
	// « /login » accepte « /login?… » : la redirection garde la page demandée et signale l'expiration.
	if got := resp.Header.Get("Location"); loc != "" && got != loc && !(loc == "/login" && strings.HasPrefix(got, "/login?")) {
		e.t.Fatalf("redirection vers %q, attendu %q", got, loc)
	}
}

var (
	csrfRe = regexp.MustCompile(`name="csrf" value="([^"]+)"`)
	keyRe  = regexp.MustCompile(`<code id="recovery-key">([A-Z2-7-]+)</code>`)
)

func csrf(t *testing.T, page string) string {
	t.Helper()
	m := csrfRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("jeton CSRF absent de la page :\n%s", page)
	}
	return m[1]
}

// register crée un compte et valide l'écran de clé de récupération. Elle renvoie la clé.
func (e *env) register(c *http.Client, login string) string {
	e.t.Helper()
	resp, _ := e.do(c, "POST", "/register", url.Values{"login": {login}, "password": {pw}, "password2": {pw}}, nil)
	e.expect(resp, 303, "/recovery-key")
	_, page := e.do(c, "GET", "/recovery-key", nil, nil)
	m := keyRe.FindStringSubmatch(page)
	if m == nil {
		e.t.Fatalf("clé de récupération absente :\n%s", page)
	}
	resp, _ = e.do(c, "POST", "/recovery-key", url.Values{"csrf": {csrf(e.t, page)}}, nil)
	e.expect(resp, 303, "/")
	return m[1]
}

func (e *env) login(c *http.Client, login, password string) *http.Response {
	resp, _ := e.do(c, "POST", "/login", url.Values{"login": {login}, "password": {password}}, nil)
	return resp
}

func TestRegisterLoginLogoutFlow(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()

	resp, page := e.do(c, "GET", "/login", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "Connexion") {
		t.Error("page de connexion en français attendue par défaut")
	}
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/login")

	resp, _ = e.do(c, "POST", "/register", url.Values{"login": {"Alice"}, "password": {pw}, "password2": {pw}}, nil)
	e.expect(resp, 303, "/recovery-key")
	var cookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == cookieName {
			cookie = ck
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie de session mal configuré : %+v", cookie)
	}

	// Tant que la clé n'est pas confirmée, tout renvoie vers l'écran de la clé.
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/recovery-key")
	resp, _ = e.do(c, "GET", "/settings", nil, nil)
	e.expect(resp, 303, "/recovery-key")
	resp, page = e.do(c, "GET", "/recovery-key", nil, nil)
	e.expect(resp, 200, "")
	if !keyRe.MatchString(page) {
		t.Fatal("clé affichée absente")
	}
	resp, txt := e.do(c, "GET", "/recovery-key.txt", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") || !strings.Contains(txt, keyRe.FindStringSubmatch(page)[1]) {
		t.Error("téléchargement de la clé incorrect")
	}
	resp, _ = e.do(c, "POST", "/recovery-key", url.Values{}, nil)
	e.expect(resp, 403, "")
	resp, _ = e.do(c, "POST", "/recovery-key", url.Values{"csrf": {csrf(t, page)}}, nil)
	e.expect(resp, 303, "/")

	// La clé n'est plus affichée.
	resp, _ = e.do(c, "GET", "/recovery-key", nil, nil)
	e.expect(resp, 303, "/")
	resp, _ = e.do(c, "GET", "/recovery-key.txt", nil, nil)
	e.expect(resp, 404, "")

	// Sans compte mail, l'accueil propose de configurer le premier.
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/accounts/new")
	resp, page = e.do(c, "GET", "/settings", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "alice") {
		t.Errorf("le login doit apparaître dans la barre de navigation :\n%s", page)
	}

	// Déconnexion : CSRF obligatoire, puis session fermée.
	resp, _ = e.do(c, "POST", "/logout", url.Values{}, nil)
	e.expect(resp, 403, "")
	resp, _ = e.do(c, "POST", "/logout", url.Values{"csrf": {csrf(t, page)}}, nil)
	e.expect(resp, 303, "/login")
	resp, _ = e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 303, "/login")

	// Reconnexion (login insensible à la casse).
	resp = e.login(c, "ALICE", "mauvais mot de passe")
	e.expect(resp, 401, "")
	resp = e.login(c, "ALICE", pw)
	e.expect(resp, 303, "/")
	resp, _ = e.do(c, "GET", "/settings", nil, nil)
	e.expect(resp, 200, "")
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	resp, _ := e.do(c, "POST", "/register", url.Values{"login": {"alice"}, "password": {"court"}, "password2": {"court"}}, nil)
	e.expect(resp, 400, "")
	resp, page := e.do(c, "POST", "/register", url.Values{"login": {"alice"}, "password": {pw}, "password2": {pw + "x"}}, nil)
	e.expect(resp, 400, "")
	if !strings.Contains(page, "ne sont pas identiques") {
		t.Error("message de mots de passe différents attendu")
	}
	resp, _ = e.do(c, "POST", "/register", url.Values{"login": {"a b"}, "password": {pw}, "password2": {pw}}, nil)
	e.expect(resp, 400, "")
	for _, ck := range c.Jar.Cookies(&url.URL{Scheme: "http", Host: strings.TrimPrefix(e.ts.URL, "http://")}) {
		if ck.Name == cookieName {
			t.Fatal("aucune session ne doit être créée après un échec")
		}
	}
	e.register(e.client(), "alice")
	resp, _ = e.do(e.client(), "POST", "/register", url.Values{"login": {"ALICE"}, "password": {pw}, "password2": {pw}}, nil)
	e.expect(resp, 409, "")
}

func TestRegistrationClosed(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.AllowRegistration = false })
	c := e.client()
	resp, page := e.do(c, "GET", "/login", nil, nil)
	e.expect(resp, 200, "")
	if strings.Contains(page, `href="/register"`) {
		t.Error("le lien d'inscription ne doit pas apparaître")
	}
	resp, _ = e.do(c, "GET", "/register", nil, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(c, "POST", "/register", url.Values{"login": {"alice"}, "password": {pw}, "password2": {pw}}, nil)
	e.expect(resp, 404, "")
}

func TestMaxUsers(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.MaxUsers = 1 })
	e.register(e.client(), "alice")
	resp, _ := e.do(e.client(), "POST", "/register", url.Values{"login": {"bob"}, "password": {pw}, "password2": {pw}}, nil)
	e.expect(resp, 403, "")
}

func TestCrossOriginPostRefused(t *testing.T) {
	e := newEnv(t, nil)
	e.register(e.client(), "alice")
	c := e.client()
	form := url.Values{"login": {"alice"}, "password": {pw}}

	resp, _ := e.do(c, "POST", "/login", form, map[string]string{"Origin": "http://evil.example"})
	e.expect(resp, 403, "")
	resp, _ = e.do(c, "POST", "/login", form, map[string]string{"Sec-Fetch-Site": "cross-site"})
	e.expect(resp, 403, "")
	resp, _ = e.do(c, "POST", "/login", form, map[string]string{"Origin": e.ts.URL})
	e.expect(resp, 303, "/")
	resp, _ = e.do(e.client(), "POST", "/login", form, map[string]string{"Sec-Fetch-Site": "same-origin"})
	e.expect(resp, 303, "/")
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.RateLimit.Login = config.RateRule{
			PerIP:    config.Limit{Count: 100, Window: time.Hour},
			PerLogin: config.Limit{Count: 2, Window: time.Hour},
		}
	})
	e.register(e.client(), "alice")
	c := e.client()
	e.expect(e.login(c, "alice", "faux mot de passe 1"), 401, "")
	e.expect(e.login(c, "alice", "faux mot de passe 2"), 401, "")
	resp := e.login(c, "alice", "faux mot de passe 3")
	e.expect(resp, 429, "")
	if resp.Header.Get("Retry-After") == "" {
		t.Error("Retry-After attendu")
	}
	// Même le bon mot de passe est refusé pendant le blocage, mais un autre login n'est pas touché.
	e.expect(e.login(c, "alice", pw), 429, "")
	e.expect(e.login(c, "bob", "faux mot de passe 1"), 401, "")
}

func TestLoginRateLimitByIP(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.RateLimit.Login = config.RateRule{
			PerIP:    config.Limit{Count: 3, Window: time.Hour},
			PerLogin: config.Limit{Count: 100, Window: time.Hour},
		}
	})
	c := e.client()
	// Trois échecs depuis la même IP, avec des logins différents, bloquent l'IP.
	e.expect(e.login(c, "inconnu1", "x"), 401, "")
	e.expect(e.login(c, "inconnu2", "x"), 401, "")
	e.expect(e.login(c, "inconnu3", "x"), 401, "")
	e.expect(e.login(c, "inconnu4", "x"), 429, "")
}

func TestLoginRateLimitResetOnSuccess(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.RateLimit.Login = config.RateRule{
			PerIP:    config.Limit{Count: 100, Window: time.Hour},
			PerLogin: config.Limit{Count: 2, Window: time.Hour},
		}
	})
	e.register(e.client(), "alice")
	c := e.client()
	e.expect(e.login(c, "alice", "faux mot de passe 1"), 401, "")
	e.expect(e.login(c, "alice", pw), 303, "/") // le succès remet le compteur par login à zéro
	e.expect(e.login(c, "alice", "faux mot de passe 2"), 401, "")
	e.expect(e.login(c, "alice", "faux mot de passe 3"), 401, "") // sans remise à zéro : 429 ici
	e.expect(e.login(c, "alice", "faux mot de passe 4"), 429, "")
}

func TestRecoveryFlow(t *testing.T) {
	e := newEnv(t, nil)
	key := e.register(e.client(), "alice")
	c := e.client()
	newPW := "nouveau mot de passe long"

	form := func(k string) url.Values {
		return url.Values{"login": {"alice"}, "recovery_key": {k}, "password": {newPW}, "password2": {newPW}}
	}
	resp, page := e.do(c, "POST", "/recover", form("AAAAA-AAAAA-AAAAA"), nil)
	e.expect(resp, 401, "")
	if !strings.Contains(page, "incorrect") {
		t.Error("message d'erreur de récupération attendu")
	}
	resp, _ = e.do(c, "POST", "/recover", form(strings.ToLower(key)), nil)
	e.expect(resp, 303, "/recovery-key")
	_, page = e.do(c, "GET", "/recovery-key", nil, nil)
	m := keyRe.FindStringSubmatch(page)
	if m == nil || m[1] == key {
		t.Fatal("une nouvelle clé de récupération doit être affichée")
	}
	resp, _ = e.do(c, "POST", "/recovery-key", url.Values{"csrf": {csrf(t, page)}}, nil)
	e.expect(resp, 303, "/")

	e.expect(e.login(e.client(), "alice", pw), 401, "")
	e.expect(e.login(e.client(), "alice", newPW), 303, "/")
	resp, _ = e.do(e.client(), "POST", "/recover", form(key), nil)
	e.expect(resp, 401, "") // l'ancienne clé ne fonctionne plus
}

func TestChangePasswordClosesOtherSessions(t *testing.T) {
	e := newEnv(t, nil)
	a := e.client()
	e.register(a, "alice")
	b := e.client()
	e.expect(e.login(b, "alice", pw), 303, "/")

	_, page := e.do(a, "GET", "/settings", nil, nil)
	newPW := "nouveau mot de passe long"
	resp, _ := e.do(a, "POST", "/settings/password", url.Values{"csrf": {csrf(t, page)}, "old_password": {"faux mot de passe"}, "password": {newPW}, "password2": {newPW}}, nil)
	e.expect(resp, 401, "")
	resp, _ = e.do(a, "POST", "/settings/password", url.Values{"csrf": {csrf(t, page)}, "old_password": {pw}, "password": {newPW}, "password2": {"autre"}}, nil)
	e.expect(resp, 400, "")
	resp, body := e.do(a, "POST", "/settings/password", url.Values{"csrf": {csrf(t, page)}, "old_password": {pw}, "password": {newPW}, "password2": {newPW}}, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(body, "Mot de passe modifié") {
		t.Error("confirmation attendue")
	}
	resp, _ = e.do(a, "GET", "/settings", nil, nil)
	e.expect(resp, 200, "") // la session courante reste ouverte
	resp, _ = e.do(b, "GET", "/settings", nil, nil)
	e.expect(resp, 303, "/login") // les autres sont fermées
	e.expect(e.login(e.client(), "alice", pw), 401, "")
	e.expect(e.login(e.client(), "alice", newPW), 303, "/")
}

func TestRegenerateRecoveryKey(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	oldKey := e.register(c, "alice")
	_, page := e.do(c, "GET", "/settings", nil, nil)
	tok := csrf(t, page)

	resp, _ := e.do(c, "POST", "/settings/recovery", url.Values{"csrf": {tok}, "password": {"faux mot de passe"}}, nil)
	e.expect(resp, 401, "")
	resp, _ = e.do(c, "POST", "/settings/recovery", url.Values{"csrf": {tok}, "password": {pw}}, nil)
	e.expect(resp, 303, "/recovery-key")
	_, page = e.do(c, "GET", "/recovery-key", nil, nil)
	m := keyRe.FindStringSubmatch(page)
	if m == nil || m[1] == oldKey {
		t.Fatal("nouvelle clé attendue")
	}
}

func TestPreferencesAndThemes(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	e.register(c, "alice")
	_, page := e.do(c, "GET", "/settings", nil, nil)
	tok := csrf(t, page)
	good := func() url.Values {
		return url.Values{"csrf": {tok}, "language": {"en"}, "theme": {"dark"}, "idle_minutes": {"30"},
			"absolute_hours": {"8"}, "page_size": {"25"}, "mark_read": {"on"}}
	}

	bad := good()
	bad.Set("theme", "../../etc/passwd")
	resp, _ := e.do(c, "POST", "/settings/preferences", bad, nil)
	e.expect(resp, 400, "")
	bad = good()
	bad.Set("idle_minutes", "100000") // au-dessus de la borne de config.json
	resp, _ = e.do(c, "POST", "/settings/preferences", bad, nil)
	e.expect(resp, 400, "")

	resp, body := e.do(c, "POST", "/settings/preferences", good(), nil)
	e.expect(resp, 200, "")
	if !strings.Contains(body, "Preferences saved") || !strings.Contains(body, `/themes/dark/theme.css`) {
		t.Errorf("langue et thème non appliqués :\n%s", body)
	}

	// Les préférences survivent à une nouvelle connexion (fichier chiffré réécrit).
	c2 := e.client()
	e.expect(e.login(c2, "alice", pw), 303, "/")
	_, body = e.do(c2, "GET", "/settings", nil, nil)
	if !strings.Contains(body, `value="25"`) || !strings.Contains(body, `value="30"`) || !strings.Contains(body, "Settings") {
		t.Errorf("préférences non conservées :\n%s", body)
	}
}

func TestThemeCSS(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	resp, body := e.do(c, "GET", "/themes/light/theme.css", nil, nil)
	e.expect(resp, 200, "")
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/css") || !strings.Contains(body, "--bg") {
		t.Error("thème clair attendu")
	}
	resp, _ = e.do(c, "GET", "/themes/inexistant/theme.css", nil, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(c, "GET", "/themes/..%2f..%2fserver.key/theme.css", nil, nil)
	if resp.StatusCode == 200 {
		t.Fatal("traversée de répertoire")
	}

	// Un thème externe, déposé dans data_dir/themes, s'ajoute sans recompiler.
	dir := filepath.Join(e.cfg.DataDir, "themes", "custom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.css"), []byte(":root{--bg:#abcdef}"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, body = e.do(c, "GET", "/themes/custom/theme.css", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(body, "#abcdef") {
		t.Error("thème externe non servi")
	}
	if names := e.srv.themeNames(); strings.Join(names, ",") != "custom,dark,light,rainloop" {
		t.Errorf("thèmes %v", names)
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newEnv(t, nil)
	resp, _ := e.do(e.client(), "GET", "/login", nil, nil)
	h := resp.Header
	if !strings.Contains(h.Get("Content-Security-Policy"), "default-src 'self'") ||
		!strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") ||
		h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" ||
		h.Get("Cache-Control") != "no-store" {
		t.Errorf("en-têtes de sécurité incomplets : %v", h)
	}
}

func TestLanguageFromAcceptHeader(t *testing.T) {
	e := newEnv(t, nil)
	_, page := e.do(e.client(), "GET", "/login", nil, map[string]string{"Accept-Language": "en-GB,en;q=0.8"})
	if !strings.Contains(page, "Sign in") {
		t.Error("page en anglais attendue")
	}
}

func TestSessionCookieSecureBehindProxy(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.TrustedProxy = true })
	resp, _ := e.do(e.client(), "POST", "/register", url.Values{"login": {"alice"}, "password": {pw}, "password2": {pw}},
		map[string]string{"X-Forwarded-Proto": "https"})
	e.expect(resp, 303, "/recovery-key")
	var secure bool
	for _, ck := range resp.Cookies() {
		if ck.Name == cookieName {
			secure = ck.Secure
		}
	}
	if !secure {
		t.Error("cookie Secure attendu derrière un proxy HTTPS")
	}
}

func TestClientIP(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4567"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9")
	if got := s.clientIP(r); got != "10.0.0.5" {
		t.Errorf("sans proxy de confiance : %q", got)
	}
	s.cfg.TrustedProxy = true
	if got := s.clientIP(r); got != "203.0.113.9" {
		t.Errorf("avec proxy de confiance, dernière entrée attendue : %q", got)
	}
	r.Header.Set("X-Forwarded-For", "pas une ip")
	if got := s.clientIP(r); got != "10.0.0.5" {
		t.Errorf("en-tête invalide : %q", got)
	}
}

func TestHealthz(t *testing.T) {
	e := newEnv(t, nil)
	resp, body := e.do(e.client(), "GET", "/healthz", nil, nil)
	e.expect(resp, 200, "")
	if body != "ok\n" {
		t.Errorf("corps %q", body)
	}
}

type httpClient = http.Client

// expect1 vérifie une redirection simple après une action réussie.
func (e *env) expect1(resp *http.Response, _ string) {
	e.t.Helper()
	if resp.StatusCode != 303 {
		e.t.Fatalf("statut %d, attendu 303 (%s %s)", resp.StatusCode, resp.Request.Method, resp.Request.URL.Path)
	}
}

var idRe = regexp.MustCompile(`/accounts/([0-9a-f]{24})/edit`)

// idsFrom renvoie les identifiants de comptes mail de la page de liste, dans l'ordre et sans doublon.
func idsFrom(page string) []string {
	var out []string
	for _, m := range idRe.FindAllStringSubmatch(page, -1) {
		if len(out) == 0 || out[len(out)-1] != m[1] {
			out = append(out, m[1])
		}
	}
	return out
}

func TestLanguageChoiceOnLoginPage(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	_, page := e.do(c, "GET", "/login", nil, map[string]string{"Accept-Language": "fr"})
	if !strings.Contains(page, `href="/lang?l=en&amp;next=/login"`) || !strings.Contains(page, "<strong>Français</strong>") {
		t.Fatalf("sélecteur de langue :\n%s", page)
	}
	resp, _ := e.do(c, "GET", "/lang?l=en&next=/register", nil, nil)
	e.expect(resp, 303, "/register")
	_, page = e.do(c, "GET", "/login", nil, map[string]string{"Accept-Language": "fr"})
	if !strings.Contains(page, "<strong>English</strong>") || !strings.Contains(page, `lang="en"`) {
		t.Errorf("la langue choisie doit l'emporter sur Accept-Language")
	}
	// valeurs inconnues ignorées ; pas de redirection ouverte
	resp, _ = e.do(c, "GET", "/lang?l=xx&next=https://evil.example", nil, nil)
	e.expect(resp, 303, "/login")
	_, page = e.do(c, "GET", "/login", nil, nil)
	if !strings.Contains(page, `lang="en"`) {
		t.Error("une langue inconnue ne doit pas effacer le choix")
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"/mail?folder=INBOX": "/mail?folder=INBOX",
		"":                   "/",
		"//evil.example":     "/",
		"https://evil":       "/",
		"/login":             "/",
		"/logout":            "/",
		"/static/app.css":    "/",
		"mail":               "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, attendu %q", in, got, want)
		}
	}
}

// Les URL d'images d'un thème doivent être absolues : dans une variable CSS, une URL relative est résolue
// d'après la feuille qui l'utilise (/static/app.css), pas d'après le fichier du thème.
func TestThemeImageURLsAreAbsolute(t *testing.T) {
	css, err := staticFS.ReadFile("static/themes/rainloop/theme.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`url("/themes/rainloop/background.jpg")`, `url("/themes/rainloop/background.svg")`} {
		if !strings.Contains(string(css), want) {
			t.Errorf("%s absent du thème", want)
		}
	}
	if strings.Contains(string(css), `url("background.`) {
		t.Error("URL relative dans le thème : elle serait résolue depuis /static/")
	}
}

// Sans image personnelle dans data/themes/, le thème ne doit pas référencer background.jpg (404 inutile).
func TestThemeDropsMissingWallpaper(t *testing.T) {
	e := newEnv(t, nil)
	c, _ := e.loggedIn("alice")
	_, css := e.do(c, "GET", "/themes/rainloop/theme.css", nil, nil)
	if strings.Contains(css, "background.jpg") {
		t.Error("la couche background.jpg doit disparaître quand le fichier n'existe pas")
	}
	if !strings.Contains(css, "background.svg") {
		t.Error("l'image livrée doit rester référencée")
	}
	dir := filepath.Join(e.cfg.DataDir, "themes", "rainloop")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "background.jpg"), []byte("\xff\xd8\xff"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, css = e.do(c, "GET", "/themes/rainloop/theme.css", nil, nil)
	if !strings.Contains(css, "background.jpg") {
		t.Error("l'image personnelle doit être référencée dès qu'elle existe")
	}
}
