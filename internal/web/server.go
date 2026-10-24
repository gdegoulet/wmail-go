// Package web contient le serveur HTTP : routes, pages, sécurité des requêtes.
package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"wmail-go/internal/config"
	"wmail-go/internal/i18n"
	"wmail-go/internal/mailconn"
	"wmail-go/internal/ratelimit"
	"wmail-go/internal/session"
	"wmail-go/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const cookieName = "wmail_session"

var themeNameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// Server regroupe les dépendances du serveur HTTP.
type Server struct {
	cfg      config.Config
	store    *store.Store
	sessions *session.Manager
	limiter  *ratelimit.Limiter
	i18n     *i18n.Bundle
	log      *slog.Logger
	guard    *mailconn.Guard
	pool     *mailconn.Pool
	pages    map[string]*template.Template
	attach   *attachStore
	pending  pending2FA
	avatars  avatarCache
}

// New construit le serveur et charge les modèles.
func New(cfg config.Config, st *store.Store, log *slog.Logger) (*Server, error) {
	bundle, err := i18n.New(cfg.DefaultLanguage)
	if err != nil {
		return nil, err
	}
	guard, err := mailconn.NewGuard(cfg.BlockPrivateNetworks, cfg.AllowedPrivateHosts)
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg: cfg, store: st, log: log, i18n: bundle, guard: guard, pool: newPool(guard, log),
		sessions: session.NewManager(), limiter: ratelimit.New(),
		pages: map[string]*template.Template{},
	}
	s.attach = newAttachStore(s.composeLimits())
	s.sessions.OnClose = func(id string) { s.pool.Close(id + "/"); s.attach.ClearSession(id) }
	for _, name := range []string{"login", "register", "recover", "recovery_key", "home", "settings", "message", "accounts", "account_form", "mail_message", "fragment", "compose", "folders_settings", "identities", "filters", "filter_edit", "filters_raw", "contacts", "login_2fa", "twofactor", "templates"} {
		t, err := template.New(name).Funcs(template.FuncMap{"inList": func(l []string, v string) bool { return slices.Contains(l, v) }, "sub1": func(n int) int { return n - 1 }, "replykey": func(m string) string {
			return map[string]string{"reply": "reply", "replyall": "reply_all", "forward": "forward"}[m]
		}}).ParseFS(templateFS, "templates/base.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("modèle %s : %w", name, err)
		}
		s.pages[name] = t
	}
	if !s.themeExists(cfg.DefaultTheme) {
		return nil, fmt.Errorf("default_theme %q : thème introuvable", cfg.DefaultTheme)
	}
	return s, nil
}

// Start lance le nettoyage périodique des sessions et des compteurs jusqu'à l'annulation du contexte.
func (s *Server) Start(ctx context.Context) {
	go s.sessions.Run(ctx, time.Minute)
	go s.pool.Run(ctx, time.Minute)
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.limiter.Sweep()
			}
		}
	}()
}

// Handler renvoie le gestionnaire HTTP complet.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(staticFS, "static")
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticCache(http.FileServerFS(static))))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /themes/{name}/{file}", s.themeFile)

	mux.HandleFunc("GET /{$}", s.auth(s.home))
	mux.HandleFunc("GET /lang", s.setLang)
	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.loginPost)
	mux.HandleFunc("GET /register", s.registerForm)
	mux.HandleFunc("POST /register", s.registerPost)
	mux.HandleFunc("GET /recover", s.recoverForm)
	mux.HandleFunc("POST /recover", s.recoverPost)
	mux.HandleFunc("POST /login/2fa", s.login2FAPost)
	mux.HandleFunc("POST /logout", s.auth(s.logout))
	mux.HandleFunc("GET /recovery-key", s.auth(s.recoveryKeyGet))
	mux.HandleFunc("GET /recovery-key.txt", s.auth(s.recoveryKeyTxt))
	mux.HandleFunc("POST /recovery-key", s.auth(s.recoveryKeyPost))
	mux.HandleFunc("GET /accounts", s.auth(s.accountsList))
	mux.HandleFunc("GET /accounts/new", s.auth(s.accountNew))
	mux.HandleFunc("GET /accounts/{id}/edit", s.auth(s.accountEdit))
	mux.HandleFunc("POST /accounts/save", s.auth(s.accountSave))
	mux.HandleFunc("POST /accounts/select", s.auth(s.accountSelect))
	mux.HandleFunc("POST /accounts/{id}/move", s.auth(s.accountMove))
	mux.HandleFunc("POST /accounts/{id}/identities/{iid}/move", s.auth(s.identityMove))
	mux.HandleFunc("POST /accounts/{id}/delete", s.auth(s.accountDelete))
	mux.HandleFunc("GET /mail/message", s.auth(s.mailMessage))
	mux.HandleFunc("GET /mail/pane", s.auth(s.mailPane))
	mux.HandleFunc("GET /mail/body", s.auth(s.mailBody))
	mux.HandleFunc("GET /mail/attachment", s.auth(s.mailAttachment))
	mux.HandleFunc("GET /mail/attachments.zip", s.auth(s.mailAttachmentsZip))
	mux.HandleFunc("POST /mail/action", s.auth(s.mailAction))
	mux.HandleFunc("POST /mail/pagesize", s.auth(s.mailPageSize))
	mux.HandleFunc("POST /mail/clear", s.auth(s.mailClear))
	mux.HandleFunc("POST /mail/append", s.auth(s.mailAppend))
	mux.HandleFunc("GET /mail/image", s.auth(s.mailImage))
	mux.HandleFunc("GET /mail/avatar", s.auth(s.mailAvatar))
	mux.HandleFunc("POST /mail/trust", s.auth(s.trustSender))
	mux.HandleFunc("POST /mail/mdn", s.auth(s.mailMDN))
	mux.HandleFunc("GET /mail/source", s.auth(s.mailSource))
	mux.HandleFunc("GET /mail/print", s.auth(s.mailPrint))
	mux.HandleFunc("GET /compose", s.auth(s.composeGet))
	mux.HandleFunc("POST /compose/send", s.auth(s.composeSend))
	mux.HandleFunc("POST /compose/save", s.auth(s.composeSave))
	mux.HandleFunc("POST /compose/attach", s.auth(s.composeAttach))
	mux.HandleFunc("POST /compose/detach", s.auth(s.composeDetach))
	mux.HandleFunc("POST /compose/discard", s.auth(s.composeDiscard))
	mux.HandleFunc("GET /settings", s.auth(s.settingsGet))
	mux.HandleFunc("GET /settings/security", s.auth(s.settingsSecurity))
	mux.HandleFunc("GET /settings/folders", s.auth(s.foldersGet))
	mux.HandleFunc("POST /settings/folders/action", s.auth(s.foldersAction))
	mux.HandleFunc("POST /settings/folders/system", s.auth(s.foldersSystem))
	mux.HandleFunc("POST /settings/folders/check", s.auth(s.foldersCheck))
	mux.HandleFunc("GET /settings/templates", s.auth(s.templatesGet))
	mux.HandleFunc("POST /settings/templates/save", s.auth(s.templatesSave))
	mux.HandleFunc("POST /settings/templates/delete", s.auth(s.templatesDelete))
	mux.HandleFunc("GET /settings/templates/body", s.auth(s.templatesBody))
	mux.HandleFunc("POST /settings/background", s.auth(s.backgroundSave))
	mux.HandleFunc("POST /settings/background/delete", s.auth(s.backgroundDelete))
	mux.HandleFunc("GET /user/background", s.auth(s.backgroundGet))
	mux.HandleFunc("GET /settings/2fa", s.auth(s.twoFactorGet))
	mux.HandleFunc("GET /settings/2fa/qr.png", s.auth(s.twoFactorQR))
	mux.HandleFunc("POST /settings/2fa/start", s.auth(s.twoFactorStart))
	mux.HandleFunc("POST /settings/2fa/enable", s.auth(s.twoFactorEnable))
	mux.HandleFunc("POST /settings/2fa/codes", s.auth(s.twoFactorRegen))
	mux.HandleFunc("POST /settings/2fa/disable", s.auth(s.twoFactorDisable))
	mux.HandleFunc("GET /settings/filters", s.auth(s.filtersGet))
	mux.HandleFunc("GET /settings/filters/edit", s.auth(s.filterEdit))
	mux.HandleFunc("POST /settings/filters/save", s.auth(s.filterSave))
	mux.HandleFunc("POST /settings/filters/action", s.auth(s.filterAction))
	mux.HandleFunc("GET /settings/filters/raw", s.auth(s.filtersRaw))
	mux.HandleFunc("POST /settings/filters/raw", s.auth(s.filtersRaw))
	mux.HandleFunc("GET /accounts/{id}/identities", s.auth(s.identitiesGet))
	mux.HandleFunc("POST /accounts/{id}/identities", s.auth(s.identitiesSave))
	mux.HandleFunc("POST /accounts/{id}/identities/{iid}/delete", s.auth(s.identitiesDelete))
	mux.HandleFunc("GET /mail/poll", s.auth(s.mailPoll))
	mux.HandleFunc("GET /contacts", s.auth(s.contactsGet))
	mux.HandleFunc("POST /contacts/save", s.auth(s.contactsSave))
	mux.HandleFunc("POST /contacts/delete", s.auth(s.contactsDelete))
	mux.HandleFunc("POST /contacts/compose", s.auth(s.contactsCompose))
	mux.HandleFunc("POST /contacts/import", s.auth(s.contactsImport))
	mux.HandleFunc("GET /contacts/export", s.auth(s.contactsExport))
	mux.HandleFunc("GET /contacts/suggest", s.auth(s.contactsSuggest))
	mux.HandleFunc("POST /contacts/add", s.auth(s.contactsAdd))
	mux.HandleFunc("POST /contacts/sync/save", s.auth(s.contactsSyncSave))
	mux.HandleFunc("POST /contacts/sync/run", s.auth(s.contactsSyncRun))
	mux.HandleFunc("POST /settings/password", s.auth(s.settingsPassword))
	mux.HandleFunc("POST /settings/recovery", s.auth(s.settingsRecovery))
	mux.HandleFunc("POST /settings/preferences", s.auth(s.settingsPrefs))
	mux.HandleFunc("POST /settings/panes", s.auth(s.settingsPanes))
	mux.HandleFunc("POST /settings/folder-open", s.auth(s.settingsFolderOpen))
	mux.HandleFunc("GET /user.css", s.userCSS)

	return s.logging(s.secure(mux))
}

// --- middlewares ---

// slowRequest : durée au-delà de laquelle une requête encore en cours est signalée dans le journal.
const slowRequest = 5 * time.Second

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		p := r.URL.Path
		// Une requête n'est journalisée qu'à sa fin : celle qui reste bloquée laisserait le journal muet.
		slow := time.AfterFunc(slowRequest, func() {
			s.log.Warn("requête lente en cours", "method", r.Method, "path", p, "depuis_ms", time.Since(start).Milliseconds(), "ip", s.clientIP(r))
		})
		next.ServeHTTP(sw, r)
		slow.Stop()
		if p == "/healthz" || strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/themes/") {
			return
		}
		s.log.Info("requête", "method", r.Method, "path", p, "status", sw.status,
			"ms", time.Since(start).Milliseconds(), "ip", s.clientIP(r))
	})
}

func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		csp := "default-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
		if r.URL.Path == "/compose" || strings.HasPrefix(r.URL.Path, "/compose/") {
			// L'éditeur de texte riche écrit des attributs style (couleurs, tailles, alignement) : sans cela, le
			// navigateur les ignorerait et les boutons de mise en forme ne fonctionneraient pas. Les scripts restent
			// interdits et les images distantes aussi (img-src), donc une feuille de style ne peut rien charger.
			csp = "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
		}
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if s.isHTTPS(r) {
			// Le proxy termine TLS : demander au navigateur de ne plus jamais tenter le HTTP en clair.
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, s.postLimit(r.URL.Path))
			if !s.sameOrigin(r) {
				s.log.Warn("POST refusé : origine différente", "path", r.URL.Path, "ip", s.clientIP(r))
				s.message(w, r, nil, http.StatusForbidden, "", "error.csrf")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// auth exige une session valide, applique la page de clé de récupération en attente
// et contrôle le jeton CSRF des requêtes POST.
func (s *Server) auth(h func(http.ResponseWriter, *http.Request, *session.Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := s.currentSession(r)
		if sess == nil {
			to := "/login"
			if r.Method == http.MethodGet {
				// La session a expiré : le dire, et revenir où l'utilisateur était après la reconnexion.
				q := url.Values{"expired": {"1"}}
				if n := safeNext(r.URL.RequestURI()); n != "/" {
					q.Set("next", n)
				}
				to += "?" + q.Encode()
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		sess.Mu.Lock()
		pending := sess.PendingRecoveryKey != ""
		sess.Mu.Unlock()
		if pending && !strings.HasPrefix(r.URL.Path, "/recovery-key") {
			http.Redirect(w, r, "/recovery-key", http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost {
			got := r.PostFormValue("csrf")
			if subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRF)) != 1 {
				s.message(w, r, sess, http.StatusForbidden, "", "error.csrf")
				return
			}
		}
		h(w, r, sess)
	}
}

// --- requête : IP, origine, cookie ---

// clientIP renvoie l'IP du client. Derrière un proxy de confiance, c'est la dernière entrée de
// X-Forwarded-For (celle ajoutée par le proxy), non falsifiable par le client.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if s.cfg.TrustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); ip != nil {
				return ip.String()
			}
		}
	}
	return host
}

func (s *Server) isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return s.cfg.TrustedProxy && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// sameOrigin refuse les POST venant d'un autre site.
func (s *Server) sameOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin":
		return true
	case "cross-site", "same-site":
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // client hors navigateur
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	host := r.Host
	if s.cfg.TrustedProxy {
		if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
			host = strings.TrimSpace(strings.Split(xfh, ",")[0])
		}
	}
	return strings.EqualFold(u.Host, host)
}

func (s *Server) currentSession(r *http.Request) *session.Session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	return s.sessions.Get(c.Value)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, id string) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: s.isHTTPS(r)})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: s.isHTTPS(r)})
}

// startSession ouvre une session pour un compte et pose le cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, acc *store.Account, pendingKey string) error {
	set := acc.Data.Settings
	sess, err := s.sessions.Create(acc, s.cfg.SessionIdle(set.SessionIdleMinutes), s.cfg.SessionAbsolute(set.SessionAbsoluteHours))
	if err != nil {
		return err
	}
	sess.PendingRecoveryKey = pendingKey
	s.setSessionCookie(w, r, sess.ID)
	return nil
}

// --- thèmes ---

func (s *Server) externalThemePath(name string) string {
	return filepath.Join(s.cfg.DataDir, "themes", name, "theme.css")
}

func (s *Server) themeExists(name string) bool {
	if !themeNameRe.MatchString(name) {
		return false
	}
	if _, err := fs.Stat(staticFS, "static/themes/"+name+"/theme.css"); err == nil {
		return true
	}
	fi, err := os.Stat(s.externalThemePath(name))
	return err == nil && fi.Mode().IsRegular()
}

// themeNames liste les thèmes embarqués et ceux du dossier data_dir/themes.
func (s *Server) themeNames() []string {
	set := map[string]bool{}
	if entries, err := fs.ReadDir(staticFS, "static/themes"); err == nil {
		for _, e := range entries {
			if e.IsDir() && s.themeExists(e.Name()) {
				set[e.Name()] = true
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(s.cfg.DataDir, "themes")); err == nil {
		for _, e := range entries {
			if e.IsDir() && s.themeExists(e.Name()) {
				set[e.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

var themeFileRe = regexp.MustCompile(`^(theme\.css|background\.(jpe?g|png|webp))$`)

// themeFile sert theme.css (dossier externe d'abord, puis thème embarqué) et l'image de fond background.* du dossier externe
// <data_dir>/themes/<nom>/ : chacun peut y poser sa propre image sans recompiler.
func (s *Server) themeFile(w http.ResponseWriter, r *http.Request) {
	name, file := r.PathValue("name"), r.PathValue("file")
	if !themeNameRe.MatchString(name) || !themeFileRe.MatchString(file) {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(filepath.Join(s.cfg.DataDir, "themes", name, file))
	if err != nil && file == "theme.css" {
		data, err = fs.ReadFile(staticFS, "static/themes/"+name+"/theme.css")
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := "text/css; charset=utf-8"
	switch {
	case strings.HasSuffix(file, ".png"):
		ct = "image/png"
	case strings.HasSuffix(file, ".webp"):
		ct = "image/webp"
	case strings.HasSuffix(file, ".jpg"), strings.HasSuffix(file, ".jpeg"):
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// --- rendu ---

// Page regroupe les données communes aux modèles.
type Page struct {
	MoveMenu []string    // dossiers proposés par le bouton « déplacer vers » (noms)
	MoveRows []FolderRow // les mêmes, avec leur présentation (menus déplacer et copier)
	Mail     bool        // page du courrier : barre d'outils du courrier
	TopUser  string      // adresse du compte mail actif, affichée en haut à droite
	bundle   *i18n.Bundle

	Lang, Theme, TitleKey, CSRF, User string
	HideNav, AllowRegistration        bool
	ErrorKey                          string
	ErrorArg                          any
	InfoKey                           string
	Login                             string
	Next                              string // page demandée avant la reconnexion
	MinPassword                       int
	RecoveryKey                       string
	Prefs                             PrefsView
	Themes, Languages                 []string
	FontSizes                         []int
	IdleMax, AbsMax                   int

	// Comptes mail
	Accounts      []AccountView
	Form          *AccountForm
	IMAPResult    *TestResult
	SMTPResult    *TestResult
	FormErrorKeys []string
	ActiveID      string
	Folders       []FolderRow
	FolderError   *TestResult
	FirstAccount  bool

	// Messages
	List    *ListView
	Msg     *MessageView
	Compose *ComposeView

	Section       string // entrée du menu des réglages
	FoldersView   *FoldersView
	Identities    *IdentitiesView
	Filters       *FiltersView
	Quota         *QuotaView
	Contacts      *ContactsView
	Poll          *PollInfo
	Langs         []LangChoice // langues proposées sur les pages d'accueil (sans session)
	TwoFactor     *TwoFactorView
	Templates     *TemplatesView
	HideChecks    bool   // cases à cocher de la liste masquées
	NoAnim        bool   // animations désactivées
	FoldersHidden bool   // volet des dossiers replié
	Path          string // chemin de la page (liens du sélecteur de boîte)
	BoxEmail      string // adresse de la boîte mail dont on règle les dossiers ou les filtres
}

// PrefsView présente les préférences avec leurs valeurs effectives.
type PrefsView struct {
	FontSize                   int
	Layout                     string
	FaviconCount               bool
	Language, Theme            string
	IdleMinutes, AbsoluteHours int
	TrustedSenders             []string
	DefaultTimezone            string // fuseau de config.json, proposé quand le réglage est vide
	PageSize                   int
	MarkRead                   bool
	DraftAutosave, ReplySame   bool
	ContactsAuto, Threads      bool
	Avatars                    string
	Timezone                   string
	HideChecks, NoAnim         bool
	NotifyDesktop, NotifySound bool
	Background                 int
	Editor                     string
}

// ThemeLabel donne le nom affiché d'un thème (« rainloop » est le thème DarkShine de RainLoop).
func (p *Page) ThemeLabel(name string) string {
	if l := p.T("theme." + name); l != "theme."+name {
		return l
	}
	return name
}

// T traduit une clé dans la langue de la page.
func (p *Page) T(key string, args ...any) string { return p.bundle.T(p.Lang, key, args...) }

// Err renvoie le message d'erreur traduit.
func (p *Page) Err() string {
	if p.ErrorKey == "" {
		return ""
	}
	if p.ErrorArg == nil {
		return p.T(p.ErrorKey)
	}
	return p.T(p.ErrorKey, p.ErrorArg)
}

func (s *Server) lang(r *http.Request, sess *session.Session) string {
	if sess != nil {
		sess.Mu.Lock()
		l := sess.Account.Data.Settings.Language
		sess.Mu.Unlock()
		if s.i18n.Has(l) {
			return l
		}
	}
	if c, err := r.Cookie(langCookie); err == nil && s.i18n.Has(c.Value) { // choix fait sur la page de connexion
		return c.Value
	}
	if l := s.i18n.Match(r.Header.Get("Accept-Language")); l != "" {
		return l
	}
	return s.cfg.DefaultLanguage
}

const langCookie = "wm_lang"

// languageNames : noms des langues dans leur propre langue (sélecteur de la page de connexion).
var languageNames = map[string]string{"fr": "Français", "en": "English"}

// LangChoice est une langue proposée sur les pages d'accueil.
type LangChoice struct {
	Code, Name string
	Current    bool
}

func (s *Server) langChoices(current string) []LangChoice {
	var out []LangChoice
	for _, l := range s.i18n.Languages() {
		n := languageNames[l]
		if n == "" {
			n = strings.ToUpper(l)
		}
		out = append(out, LangChoice{Code: l, Name: n, Current: l == current})
	}
	return out
}

// setLang mémorise la langue choisie avant la connexion, puis revient à la page d'accueil demandée.
// safeNext ne garde qu'un chemin local (« /mail?… ») : jamais d'URL absolue ni de « // » qui mènerait ailleurs.
func safeNext(v string) string {
	if v == "" || !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.Contains(v, "\\") {
		return "/"
	}
	if u, err := url.Parse(v); err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	switch {
	case v == "/login", strings.HasPrefix(v, "/login/"), strings.HasPrefix(v, "/logout"),
		strings.HasPrefix(v, "/static/"), strings.HasPrefix(v, "/themes/"):
		return "/"
	}
	return v
}

// staticCache ajoute un ETag (empreinte du contenu) et autorise la mise en cache des fichiers embarqués :
// ils ne changent qu'avec le binaire, et le navigateur revalide avec If-None-Match.
func staticCache(next http.Handler) http.Handler {
	var tags sync.Map
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		tag, ok := tags.Load(name)
		if !ok {
			if b, err := fs.ReadFile(staticFS, "static/"+name); err == nil {
				sum := sha256.Sum256(b)
				tag = `"` + hex.EncodeToString(sum[:8]) + `"`
				tags.Store(name, tag)
			}
		}
		if t, _ := tag.(string); t != "" {
			w.Header().Set("ETag", t)
			w.Header().Set("Cache-Control", "public, max-age=600, must-revalidate")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setLang(w http.ResponseWriter, r *http.Request) {
	l := r.URL.Query().Get("l")
	if s.i18n.Has(l) {
		http.SetCookie(w, &http.Cookie{Name: langCookie, Value: l, Path: "/", MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode, Secure: s.isHTTPS(r)})
	}
	next := "/login"
	switch r.URL.Query().Get("next") {
	case "/register", "/recover":
		next = r.URL.Query().Get("next")
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) theme(sess *session.Session) string {
	if sess != nil {
		sess.Mu.Lock()
		t := sess.Account.Data.Settings.Theme
		sess.Mu.Unlock()
		if t != "" && s.themeExists(t) {
			return t
		}
	}
	return s.cfg.DefaultTheme
}

// userLoc donne le fuseau horaire d'affichage des dates : celui de l'utilisateur, à défaut celui de config.json, à défaut
// celui du système.
func (s *Server) userLoc(sess *session.Session) *time.Location {
	name := ""
	if sess != nil {
		sess.Mu.Lock()
		name = sess.Account.Data.Settings.Timezone
		sess.Mu.Unlock()
	}
	for _, n := range []string{name, s.cfg.Timezone} {
		if n == "" {
			continue
		}
		if loc, err := time.LoadLocation(n); err == nil {
			return loc
		}
	}
	return time.Local
}

// prepare renseigne les champs communs d'une page.
func (s *Server) prepare(r *http.Request, sess *session.Session, p *Page) {
	p.bundle = s.i18n
	p.Lang = s.lang(r, sess)
	p.Theme = s.theme(sess)
	p.AllowRegistration = s.cfg.AllowRegistration
	if sess == nil {
		p.Langs = s.langChoices(p.Lang)
	}
	p.MinPassword = s.cfg.PasswordMinLength
	if sess != nil {
		p.CSRF = sess.CSRF
		p.User = sess.Account.Login
		sess.Mu.Lock()
		p.FoldersHidden = sess.Account.Data.Settings.FoldersHidden
		p.HideChecks, p.NoAnim = sess.Account.Data.Settings.HideCheckboxes, sess.Account.Data.Settings.NoAnimations
		sess.Mu.Unlock()
		if p.InfoKey == "" && p.ErrorKey == "" {
			sess.Mu.Lock()
			p.InfoKey, sess.Flash = sess.Flash, ""
			sess.Mu.Unlock()
		}
	}
	if p.TitleKey == "" {
		p.TitleKey = "app.name"
	}
}

func (s *Server) execute(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, page, tmpl string, p *Page) {
	s.prepare(r, sess, p)
	var buf bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&buf, tmpl, p); err != nil {
		s.log.Error("rendu de page", "page", page, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// settingsSections associe les pages du cadre des réglages à leur entrée de menu (la barre habituelle y est masquée).
var settingsSections = map[string]string{"settings": "general", "accounts": "accounts", "account_form": "accounts", "identities": "accounts", "folders_settings": "folders", "filters": "filters", "filter_edit": "filters", "filters_raw": "filters", "contacts": "contacts", "twofactor": "security", "templates": "templates"}

func (s *Server) render(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, name string, p *Page) {
	if sec, ok := settingsSections[name]; ok {
		p.HideNav = true
		if p.Section == "" {
			p.Section = sec
		}
	}
	s.execute(w, r, sess, status, name, "base", p)
}

// renderFragment renvoie un morceau de page (sans l'enveloppe HTML), pour les mises à jour partielles.
func (s *Server) renderFragment(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, tmpl string, p *Page) {
	s.execute(w, r, sess, status, "fragment", tmpl, p)
}

// message affiche une page avec un seul message d'erreur.
func (s *Server) message(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, titleKey, errKey string, arg ...any) {
	p := &Page{TitleKey: titleKey, ErrorKey: errKey}
	if len(arg) > 0 {
		p.ErrorArg = arg[0]
	}
	s.render(w, r, sess, status, "message", p)
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, sess *session.Session, err error) {
	s.log.Error("erreur interne", "path", r.URL.Path, "err", err)
	s.message(w, r, sess, http.StatusInternalServerError, "", "error.generic")
}

// rateLimited répond 429 avec Retry-After.
func (s *Server) rateLimited(w http.ResponseWriter, r *http.Request, sess *session.Session, wait time.Duration) {
	secs := int(wait.Seconds()) + 1
	w.Header().Set("Retry-After", fmt.Sprint(secs))
	s.message(w, r, sess, http.StatusTooManyRequests, "", "error.rate", secs)
}

// passwordErrorKey convertit une erreur de politique de mot de passe ou de login en clé de message.
func passwordErrorKey(err error) (key string, arg any, ok bool) {
	switch {
	case errors.Is(err, store.ErrPasswordTooShort):
		return "error.password_short", nil, true
	case errors.Is(err, store.ErrPasswordTooLong):
		return "error.password_long", nil, true
	case errors.Is(err, store.ErrInvalidLogin):
		return "error.login_invalid", nil, true
	}
	return "", nil, false
}

func newPool(g *mailconn.Guard, log *slog.Logger) *mailconn.Pool {
	p := mailconn.NewPool(g, 5*time.Minute)
	p.Log = log
	return p
}

// postLimit renvoie la taille maximale du corps d'une requête POST.
func (s *Server) postLimit(path string) int64 {
	switch path {
	case "/compose/attach":
		return int64(s.cfg.Compose.MaxAttachmentMB+1) << 20
	case "/mail/append":
		return int64(s.cfg.Compose.MaxMessageMB+s.cfg.Compose.MaxAttachmentMB) << 20
	case "/settings/background":
		return maxBackground + 64<<10
	case "/contacts/import":
		return maxContactsImport + 64<<10
	case "/compose/send", "/compose/save":
		return maxComposeBody + 64<<10
	}
	return 64 << 10
}
