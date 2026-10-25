package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"wmail-go/internal/config"
	"wmail-go/internal/mailconn"
	"wmail-go/internal/session"
	"wmail-go/internal/store"
	"wmail-go/internal/userdata"
)

// limitKeys construit les clés de limitation d'une action : par IP et par login (identifiant hashé).
func (s *Server) limitKeys(action string, r *http.Request, login string) (ipKey, userKey string) {
	ipKey = action + ":ip:" + s.clientIP(r)
	if nl, err := store.NormalizeLogin(login); err == nil {
		userKey = action + ":user:" + s.store.ID(nl)
	}
	return ipKey, userKey
}

// checkLimits renvoie le délai d'attente si l'IP ou le login a atteint sa limite.
func (s *Server) checkLimits(rule config.RateRule, ipKey, userKey string) (time.Duration, bool) {
	if blocked, wait := s.limiter.Check(ipKey, rule.PerIP); blocked {
		return wait, true
	}
	if userKey != "" {
		if blocked, wait := s.limiter.Check(userKey, rule.PerLogin); blocked {
			return wait, true
		}
	}
	return 0, false
}

func (s *Server) addFailure(rule config.RateRule, ipKey, userKey string) {
	s.limiter.Add(ipKey, rule.PerIP)
	if userKey != "" {
		s.limiter.Add(userKey, rule.PerLogin)
	}
}

// formError remplit une page de formulaire avec une erreur de politique de mot de passe ou de login.
func (s *Server) formError(p *Page, err error) bool {
	key, _, ok := passwordErrorKey(err)
	if !ok {
		return false
	}
	p.ErrorKey = key
	if key == "error.password_short" {
		p.ErrorArg = s.cfg.PasswordMinLength
	}
	return true
}

// --- connexion ---

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if s.currentSession(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	p := &Page{TitleKey: "login.title", Next: safeNext(r.URL.Query().Get("next"))}
	if r.URL.Query().Get("expired") == "1" {
		p.InfoKey = "login.expired"
	}
	s.render(w, r, nil, http.StatusOK, "login", p)
}

func (s *Server) loginPost(w http.ResponseWriter, r *http.Request) {
	login, password := r.PostFormValue("login"), r.PostFormValue("password")
	rule := s.cfg.RateLimit.Login
	ipKey, userKey := s.limitKeys("login", r, login)
	if wait, blocked := s.checkLimits(rule, ipKey, userKey); blocked {
		s.log.Warn("connexion bloquée par la limitation de débit", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
		s.rateLimited(w, r, nil, wait)
		return
	}
	acc, err := s.store.Login(login, password)
	if err != nil {
		if errors.Is(err, store.ErrInvalidCredentials) {
			s.addFailure(rule, ipKey, userKey)
			s.log.Warn("échec de connexion", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
			s.render(w, r, nil, http.StatusUnauthorized, "login",
				&Page{TitleKey: "login.title", ErrorKey: "login.error.invalid", Login: login, Next: safeNext(r.PostFormValue("next"))})
			return
		}
		s.serverError(w, r, nil, err)
		return
	}
	if userKey != "" {
		s.limiter.Reset(userKey)
	}
	next := safeNext(r.PostFormValue("next"))
	if s.needSecondFactor(w, r, acc, next) {
		return
	}
	if err := s.startSession(w, r, acc, ""); err != nil {
		acc.Wipe()
		s.serverError(w, r, nil, err)
		return
	}
	s.log.Info("connexion", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.log.Info("déconnexion", "login", store.ReduceLogin(sess.Account.Login))
	s.sessions.Delete(sess.ID)
	s.clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- inscription ---

func (s *Server) registerForm(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowRegistration {
		s.message(w, r, nil, http.StatusNotFound, "", "register.error.closed")
		return
	}
	s.render(w, r, nil, http.StatusOK, "register", &Page{TitleKey: "register.title"})
}

func (s *Server) registerPost(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowRegistration {
		s.message(w, r, nil, http.StatusNotFound, "", "register.error.closed")
		return
	}
	ipKey := "register:ip:" + s.clientIP(r)
	rule := s.cfg.RateLimit.Register
	if blocked, wait := s.limiter.Check(ipKey, rule.PerIP); blocked {
		s.rateLimited(w, r, nil, wait)
		return
	}
	s.limiter.Add(ipKey, rule.PerIP) // chaque tentative compte

	login, pw, pw2 := r.PostFormValue("login"), r.PostFormValue("password"), r.PostFormValue("password2")
	page := &Page{TitleKey: "register.title", Login: login}
	if pw != pw2 {
		page.ErrorKey = "error.mismatch"
		s.render(w, r, nil, http.StatusBadRequest, "register", page)
		return
	}
	acc, recovery, err := s.store.Create(login, pw, false)
	switch {
	case err == nil:
	case s.formError(page, err):
		s.render(w, r, nil, http.StatusBadRequest, "register", page)
		return
	case errors.Is(err, store.ErrExists):
		page.ErrorKey = "register.error.exists"
		s.render(w, r, nil, http.StatusConflict, "register", page)
		return
	case errors.Is(err, store.ErrMaxUsers):
		s.message(w, r, nil, http.StatusForbidden, "", "register.error.closed")
		return
	default:
		s.serverError(w, r, nil, err)
		return
	}
	if err := s.startSession(w, r, acc, recovery); err != nil {
		acc.Wipe()
		s.serverError(w, r, nil, err)
		return
	}
	s.log.Info("compte créé", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
	http.Redirect(w, r, "/recovery-key", http.StatusSeeOther)
}

// --- clé de récupération (affichage unique) ---

func (s *Server) pendingKey(sess *session.Session) string {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return sess.PendingRecoveryKey
}

func (s *Server) recoveryKeyGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	key := s.pendingKey(sess)
	if key == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, sess, http.StatusOK, "recovery_key",
		&Page{TitleKey: "recovery_key.title", RecoveryKey: key, HideNav: true})
}

func (s *Server) recoveryKeyTxt(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	key := s.pendingKey(sess)
	if key == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="wmail-go-recovery-key.txt"`)
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, "wmail-go – %s\n\n%s\n\n%s\n", sess.Account.Login, key,
		"Keep this key safe. Anyone holding it has full access to the account. / Gardez cette clé en lieu sûr : elle donne un accès complet au compte.")
}

func (s *Server) recoveryKeyPost(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	sess.PendingRecoveryKey = ""
	sess.Mu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- récupération du compte ---

func (s *Server) recoverForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, nil, http.StatusOK, "recover", &Page{TitleKey: "recover.title"})
}

func (s *Server) recoverPost(w http.ResponseWriter, r *http.Request) {
	login, key := r.PostFormValue("login"), r.PostFormValue("recovery_key")
	pw, pw2 := r.PostFormValue("password"), r.PostFormValue("password2")
	rule := s.cfg.RateLimit.Recovery
	ipKey, userKey := s.limitKeys("recover", r, login)
	if wait, blocked := s.checkLimits(rule, ipKey, userKey); blocked {
		s.log.Warn("récupération bloquée par la limitation de débit", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
		s.rateLimited(w, r, nil, wait)
		return
	}
	page := &Page{TitleKey: "recover.title", Login: login}
	if pw != pw2 {
		page.ErrorKey = "error.mismatch"
		s.render(w, r, nil, http.StatusBadRequest, "recover", page)
		return
	}
	acc, newKey, err := s.store.Recover(login, key, pw)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrInvalidCredentials):
		s.addFailure(rule, ipKey, userKey)
		s.log.Warn("échec de récupération", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
		page.ErrorKey = "recover.error.invalid"
		s.render(w, r, nil, http.StatusUnauthorized, "recover", page)
		return
	case s.formError(page, err):
		s.render(w, r, nil, http.StatusBadRequest, "recover", page)
		return
	default:
		s.serverError(w, r, nil, err)
		return
	}
	if userKey != "" {
		s.limiter.Reset(userKey)
	}
	s.sessions.DeleteUser(acc.Login, "")
	// L'ancienne clé a servi : une nouvelle est générée et affichée avant de continuer.
	if err := s.startSession(w, r, acc, newKey); err != nil {
		acc.Wipe()
		s.serverError(w, r, nil, err)
		return
	}
	s.log.Info("compte récupéré", "login", store.ReduceLogin(login), "ip", s.clientIP(r))
	http.Redirect(w, r, "/recovery-key", http.StatusSeeOther)
}

// --- réglages ---

func (s *Server) settingsPage(sess *session.Session) *Page {
	sess.Mu.Lock()
	set := sess.Account.Data.Settings
	sess.Mu.Unlock()
	p := &Page{
		TitleKey:  "settings.title",
		Themes:    s.themeNames(),
		Languages: s.i18n.Languages(),
		FontSizes: fontSizes,
		IdleMax:   s.cfg.Session.IdleMinutesMax,
		AbsMax:    s.cfg.Session.AbsoluteHoursMax,
		Prefs: PrefsView{
			Language:        set.Language,
			Theme:           set.Theme,
			IdleMinutes:     int(s.cfg.SessionIdle(set.SessionIdleMinutes) / time.Minute),
			AbsoluteHours:   int(s.cfg.SessionAbsolute(set.SessionAbsoluteHours) / time.Hour),
			PageSize:        set.PageSize,
			Layout:          set.Layout,
			FontSize:        effectiveFontSize(set.FontSize),
			MarkRead:        set.MarkReadOnOpen,
			Editor:          set.Editor,
			DraftAutosave:   !set.NoDraftAutosave,
			ReplySame:       set.ReplySameFolder,
			ContactsAuto:    !set.NoContactsAutosave,
			Threads:         set.Threads,
			Timezone:        set.Timezone,
			DefaultTimezone: s.cfg.Timezone,
			HideChecks:      set.HideCheckboxes,
			TrustedSenders:  set.TrustedSenders,
			NoAnim:          set.NoAnimations,
			Version:         Version,
			SourceURL:       s.cfg.SourceURL,
			Avatars:         set.Avatars, // sans cela le sélecteur revenait toujours sur « aucune »
			FaviconCount:    !set.NoFaviconCount,
			NotifyDesktop:   set.NotifyDesktop,
			Background:      set.Background,
			NotifySound:     set.NotifySound,
		},
	}
	if !s.i18n.Has(p.Prefs.Language) {
		p.Prefs.Language = s.cfg.DefaultLanguage
	}
	if !s.themeExists(p.Prefs.Theme) {
		p.Prefs.Theme = s.cfg.DefaultTheme
	}
	if p.Prefs.PageSize < 10 {
		p.Prefs.PageSize = 50
	}
	if p.Prefs.Layout != "list" && p.Prefs.Layout != "bottom" {
		p.Prefs.Layout = "three"
	}
	return p
}

func (s *Server) settingsGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.render(w, r, sess, http.StatusOK, "settings", s.settingsPage(sess))
}

func (s *Server) settingsSecurity(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	p := s.settingsPage(sess)
	p.Section = "security"
	s.render(w, r, sess, http.StatusOK, "settings", p)
}

func (s *Server) settingsPassword(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	login := sess.Account.Login
	old, pw, pw2 := r.PostFormValue("old_password"), r.PostFormValue("password"), r.PostFormValue("password2")
	rule := s.cfg.RateLimit.Login
	ipKey, userKey := s.limitKeys("login", r, login)
	if wait, blocked := s.checkLimits(rule, ipKey, userKey); blocked {
		s.rateLimited(w, r, sess, wait)
		return
	}
	page := s.settingsPage(sess)
	page.Section = "security"
	if pw != pw2 {
		page.ErrorKey = "error.mismatch"
		s.render(w, r, sess, http.StatusBadRequest, "settings", page)
		return
	}
	err := s.store.ChangePassword(login, old, pw)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrInvalidCredentials):
		s.addFailure(rule, ipKey, userKey)
		page.ErrorKey = "settings.error.password"
		s.render(w, r, sess, http.StatusUnauthorized, "settings", page)
		return
	case s.formError(page, err):
		s.render(w, r, sess, http.StatusBadRequest, "settings", page)
		return
	default:
		s.serverError(w, r, sess, err)
		return
	}
	n := s.sessions.DeleteUser(login, sess.ID)
	s.log.Info("mot de passe modifié", "login", store.ReduceLogin(login), "sessions_fermées", n)
	page.InfoKey = "settings.password.done"
	s.render(w, r, sess, http.StatusOK, "settings", page)
}

func (s *Server) settingsRecovery(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	login := sess.Account.Login
	rule := s.cfg.RateLimit.Login
	ipKey, userKey := s.limitKeys("login", r, login)
	if wait, blocked := s.checkLimits(rule, ipKey, userKey); blocked {
		s.rateLimited(w, r, sess, wait)
		return
	}
	key, err := s.store.RegenerateRecovery(login, r.PostFormValue("password"))
	if errors.Is(err, store.ErrInvalidCredentials) {
		s.addFailure(rule, ipKey, userKey)
		page := s.settingsPage(sess)
		page.Section = "security"
		page.ErrorKey = "settings.error.password"
		s.render(w, r, sess, http.StatusUnauthorized, "settings", page)
		return
	}
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.log.Info("clé de récupération régénérée", "login", store.ReduceLogin(login))
	sess.Mu.Lock()
	sess.PendingRecoveryKey = key
	sess.Mu.Unlock()
	http.Redirect(w, r, "/recovery-key", http.StatusSeeOther)
}

func atoiIn(s string, lo, hi int) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= lo && n <= hi
}

// settingsPrefs enregistre les préférences. Avec l'en-tête « X-Requested-With: fetch » (enregistrement à la volée
// par settings-auto.js) la réponse est un petit JSON ; sans lui, la page des réglages est réaffichée comme avant.
func (s *Server) settingsPrefs(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	ajax := r.Header.Get("X-Requested-With") == "fetch"
	lang, theme := r.PostFormValue("language"), r.PostFormValue("theme")
	idle, ok1 := atoiIn(r.PostFormValue("idle_minutes"), 1, s.cfg.Session.IdleMinutesMax)
	abs, ok2 := atoiIn(r.PostFormValue("absolute_hours"), 1, s.cfg.Session.AbsoluteHoursMax)
	size, ok3 := atoiIn(r.PostFormValue("page_size"), 10, maxPageSize)
	fontSize := 0 // facultatif : absent = valeur actuelle
	if raw := r.PostFormValue("font_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || !slices.Contains(fontSizes, n) {
			ok3 = false
		}
		fontSize = n
	}
	layout := r.PostFormValue("layout") // facultatif : absent = valeur actuelle
	if layout != "" && layout != "three" && layout != "list" && layout != "bottom" {
		ok3 = false
	}
	tz := strings.TrimSpace(r.PostFormValue("timezone")) // vide : fuseau du serveur
	if tz != "" {
		if _, err := time.LoadLocation(tz); err != nil || len(tz) > 64 {
			ok3 = false
		}
	}
	if !s.i18n.Has(lang) || !s.themeExists(theme) || !ok1 || !ok2 || !ok3 {
		if ajax {
			writeJSONOK(w, http.StatusBadRequest, false)
			return
		}
		page := s.settingsPage(sess)
		page.ErrorKey = "settings.prefs.error"
		s.render(w, r, sess, http.StatusBadRequest, "settings", page)
		return
	}
	sess.Mu.Lock()
	set := &sess.Account.Data.Settings
	prev := *set
	set.Language, set.Theme = lang, theme
	set.SessionIdleMinutes, set.SessionAbsoluteHours = idle, abs
	set.PageSize = size
	if layout != "" {
		set.Layout = layout
	}
	if fontSize != 0 {
		set.FontSize = fontSize
	}
	set.MarkReadOnOpen = r.PostFormValue("mark_read") != ""
	if ed := r.PostFormValue("editor"); userdata.ValidEditor(ed) {
		set.Editor = ed
	}
	set.NoDraftAutosave = r.PostFormValue("draft_autosave") == ""
	set.ReplySameFolder = r.PostFormValue("reply_same_folder") != ""
	set.NoContactsAutosave = r.PostFormValue("contacts_autosave") == ""
	set.Threads = r.PostFormValue("threads") != ""
	set.Timezone = tz
	if a := r.PostFormValue("avatars"); a == "none" || a == "gravatar" {
		set.Avatars = a
	}
	set.HideCheckboxes = r.PostFormValue("show_checkboxes") == ""
	set.NoAnimations = r.PostFormValue("animations") == ""
	set.NoFaviconCount = r.PostFormValue("favicon_count") == ""
	set.NotifyDesktop = r.PostFormValue("notify_desktop") != ""
	set.NotifySound = r.PostFormValue("notify_sound") != ""
	err := s.store.SaveData(sess.Account)
	if err != nil {
		*set = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.sessions.SetTTL(sess, s.cfg.SessionIdle(idle), s.cfg.SessionAbsolute(abs))
	if ajax {
		writeJSONOK(w, http.StatusOK, true)
		return
	}
	page := s.settingsPage(sess)
	page.InfoKey = "settings.prefs.done"
	s.render(w, r, sess, http.StatusOK, "settings", page)
}

// writeJSONOK répond { "ok": true|false } aux requêtes faites en fetch.
func writeJSONOK(w http.ResponseWriter, status int, ok bool) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if ok {
		_, _ = io.WriteString(w, `{"ok":true}`)
		return
	}
	_, _ = io.WriteString(w, `{"ok":false}`)
}

// Bornes (en pixels) des largeurs de volets enregistrées par l'utilisateur.
const (
	foldersWidthMin, foldersWidthMax = 160, 480
	listWidthMin, listWidthMax       = 240, 1000
)

// settingsPanes enregistre la largeur des volets ; 0 rétablit la valeur par défaut. Les champs absents sont inchangés.
// Appelé par le script (en-tête X-Requested-With: fetch) ou depuis la page des réglages.
func (s *Server) settingsPanes(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	parse := func(name string, lo, hi int) (v int, present, ok bool) {
		raw := r.PostFormValue(name)
		if raw == "" {
			return 0, false, true
		}
		n, err := strconv.Atoi(raw)
		if err != nil || (n != 0 && (n < lo || n > hi)) {
			return 0, true, false
		}
		return n, true, true
	}
	f, hasF, okF := parse("folders", foldersWidthMin, foldersWidthMax)
	l, hasL, okL := parse("list", listWidthMin, listWidthMax)
	hidden := r.PostFormValue("hidden")
	hasH := hidden == "0" || hidden == "1"
	if !okF || !okL || (!hasF && !hasL && !hasH) || (hidden != "" && !hasH) {
		http.Error(w, "largeur invalide", http.StatusBadRequest)
		return
	}
	sess.Mu.Lock()
	set := &sess.Account.Data.Settings
	prev := *set
	if hasF {
		set.FoldersWidth = f
	}
	if hasL {
		set.ListWidth = l
	}
	if hasH {
		set.FoldersHidden = hidden == "1"
	}
	err := s.store.SaveData(sess.Account)
	if err != nil {
		*set = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// userCSS sert les variables CSS propres à l'utilisateur (largeur des volets). Sans session, la feuille est vide.
func (s *Server) userCSS(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder
	if sess := s.currentSession(r); sess != nil {
		sess.Mu.Lock()
		set := sess.Account.Data.Settings
		sess.Mu.Unlock()
		var vars []string
		if set.FoldersWidth >= foldersWidthMin && set.FoldersWidth <= foldersWidthMax {
			vars = append(vars, fmt.Sprintf("--folders-w:%dpx", set.FoldersWidth))
		}
		if set.ListWidth >= listWidthMin && set.ListWidth <= listWidthMax {
			vars = append(vars, fmt.Sprintf("--list-w:%dpx", set.ListWidth))
		}
		if len(vars) > 0 {
			sb.WriteString(":root{" + strings.Join(vars, ";") + "}\n")
		}
		if set.Background > 0 {
			fmt.Fprintf(&sb, "body{background:#222 url(/user/background?v=%d) center / cover fixed no-repeat !important}\n", set.Background)
		}
		if fs := effectiveFontSize(set.FontSize); fs != defaultFontSize {
			fmt.Fprintf(&sb, "html{font-size:%dpx}\n", fs)
		}
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-cache")
	_, _ = w.Write([]byte(sb.String()))
}

// Tailles de texte proposées (pixels) ; defaultFontSize est celle de la feuille de style.
var fontSizes = []int{10, 11, 12, 13, 14, 15, 16, 18}

const defaultFontSize = 14

func effectiveFontSize(v int) int {
	for _, f := range fontSizes {
		if f == v {
			return v
		}
	}
	return defaultFontSize
}

// settingsFolderOpen mémorise le dépliage ou le repliage d'un niveau de l'arbre des dossiers (boîte active).
func (s *Server) settingsFolderOpen(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	name, open := r.PostFormValue("folder"), r.PostFormValue("open")
	if !ok || !mailconn.ValidFolderName(name) || (open != "0" && open != "1") {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}
	sess.Mu.Lock()
	set := &sess.Account.Data.Settings
	prev := set.OpenFolders
	next := map[string][]string{}
	for k, v := range prev {
		next[k] = append([]string(nil), v...)
	}
	list := next[acc.ID]
	has := slices.Contains(list, name)
	switch {
	case open == "1" && !has:
		if len(list) >= maxOpenFolders {
			list = list[1:]
		}
		list = append(list, name)
	case open == "0" && has:
		list = slices.DeleteFunc(list, func(x string) bool { return x == name })
	}
	if len(list) == 0 {
		delete(next, acc.ID)
	} else {
		next[acc.ID] = list
	}
	set.OpenFolders = next
	err := s.store.SaveData(sess.Account)
	if err != nil {
		set.OpenFolders = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
