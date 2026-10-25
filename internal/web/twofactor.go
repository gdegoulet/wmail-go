package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"

	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/store"
	"github.com/gdegoulet/wmail-go/internal/totp"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const (
	pendingCookie  = "wm_2fa"
	trustCookie    = "wm_trust"
	pendingTTL     = 5 * time.Minute
	pendingTries   = 5
	trustDuration  = 14 * 24 * time.Hour
	maxPendingAcct = 1000
)

// pending2FA garde en mémoire, quelques minutes, un compte dont le mot de passe est vérifié et dont le second facteur
// est attendu. Rien n'est écrit sur le disque ; la clé de données reste en mémoire comme pour une session.
type pending2FA struct {
	mu sync.Mutex
	m  map[string]*pendingEntry
}

type pendingEntry struct {
	acc     *store.Account
	expires time.Time
	tries   int
}

func (p *pending2FA) put(acc *store.Account, now time.Time) (string, bool) {
	var b [24]byte
	_, _ = rand.Read(b[:])
	id := base64.RawURLEncoding.EncodeToString(b[:])
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[string]*pendingEntry{}
	}
	for k, e := range p.m {
		if now.After(e.expires) {
			e.acc.Wipe()
			delete(p.m, k)
		}
	}
	if len(p.m) >= maxPendingAcct {
		return "", false
	}
	p.m[id] = &pendingEntry{acc: acc, expires: now.Add(pendingTTL)}
	return id, true
}

// attempt renvoie l'entrée et compte un essai ; au-delà de la limite ou après expiration, elle est détruite.
func (p *pending2FA) attempt(id string, now time.Time) *store.Account {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.m[id]
	if e == nil {
		return nil
	}
	e.tries++
	if now.After(e.expires) || e.tries > pendingTries {
		e.acc.Wipe()
		delete(p.m, id)
		return nil
	}
	return e.acc
}

func (p *pending2FA) done(id string) {
	p.mu.Lock()
	delete(p.m, id)
	p.mu.Unlock()
}

func (p *pending2FA) drop(id string) {
	p.mu.Lock()
	if e := p.m[id]; e != nil {
		e.acc.Wipe()
		delete(p.m, id)
	}
	p.mu.Unlock()
}

// --- cookie « ne plus demander pendant 14 jours » ---

func trustMAC(key, id string, exp int64) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(id + "|" + strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) setTrustCookie(w http.ResponseWriter, r *http.Request, acc *store.Account) {
	exp := time.Now().Add(trustDuration).Unix()
	v := strconv.FormatInt(exp, 10) + "." + trustMAC(acc.Data.TwoFactor.TrustKey, acc.ID, exp)
	http.SetCookie(w, &http.Cookie{Name: trustCookie + "_" + acc.ID[:12], Value: v, Path: "/", MaxAge: int(trustDuration.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.isHTTPS(r)})
}

func (s *Server) trusted(r *http.Request, acc *store.Account) bool {
	tf := acc.Data.TwoFactor
	if tf.TrustKey == "" {
		return false
	}
	c, err := r.Cookie(trustCookie + "_" + acc.ID[:12])
	if err != nil {
		return false
	}
	expS, mac, ok := strings.Cut(c.Value, ".")
	exp, err := strconv.ParseInt(expS, 10, 64)
	if !ok || err != nil || time.Now().Unix() > exp {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(mac), []byte(trustMAC(tf.TrustKey, acc.ID, exp))) == 1
}

func newTrustKey() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawStdEncoding.EncodeToString(b[:])
}

// --- connexion : second facteur ---

// needSecondFactor démarre l'étape du code après un mot de passe correct ; renvoie vrai si la réponse a été écrite.
func (s *Server) needSecondFactor(w http.ResponseWriter, r *http.Request, acc *store.Account, next string) bool {
	tf := acc.Data.TwoFactor
	if !tf.Enabled || s.trusted(r, acc) {
		return false
	}
	id, ok := s.pending.put(acc, time.Now())
	if !ok {
		acc.Wipe()
		s.message(w, r, nil, http.StatusServiceUnavailable, "", "error.generic")
		return true
	}
	http.SetCookie(w, &http.Cookie{Name: pendingCookie, Value: id, Path: "/login", MaxAge: int(pendingTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.isHTTPS(r)})
	s.render(w, r, nil, http.StatusOK, "login_2fa", &Page{TitleKey: "twofactor.login_title", Next: next})
	return true
}

// checkSecondFactor vérifie un code TOTP ou de secours ; il met à jour (et enregistre) les données du compte.
func (s *Server) checkSecondFactor(acc *store.Account, code string) bool {
	tf := &acc.Data.TwoFactor
	if step, ok := totp.Verify(tf.Secret, code, time.Now(), tf.LastStep); ok {
		prev := tf.LastStep
		tf.LastStep = step
		if err := s.store.SaveData(acc); err != nil {
			tf.LastStep = prev
			s.log.Warn("pas de temps du second facteur non enregistré", "err", err)
			return false
		}
		return true
	}
	if left, ok := totp.UseBackup(tf.BackupHashes, code); ok {
		prev := tf.BackupHashes
		tf.BackupHashes = left
		if err := s.store.SaveData(acc); err != nil {
			tf.BackupHashes = prev
			return false
		}
		return true
	}
	return false
}

func (s *Server) login2FAPost(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(pendingCookie)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	ipKey := "twofa:ip:" + s.clientIP(r)
	rule := s.cfg.RateLimit.Login
	if blocked, wait := s.limiter.Check(ipKey, rule.PerIP); blocked {
		s.rateLimited(w, r, nil, wait)
		return
	}
	acc := s.pending.attempt(c.Value, time.Now())
	if acc == nil {
		s.render(w, r, nil, http.StatusUnauthorized, "login", &Page{TitleKey: "login.title", ErrorKey: "twofactor.expired"})
		return
	}
	if !s.checkSecondFactor(acc, r.PostFormValue("code")) {
		s.limiter.Add(ipKey, rule.PerIP)
		s.log.Warn("échec du second facteur", "login", store.ReduceLogin(acc.Login), "ip", s.clientIP(r))
		s.trace(acc.Login, userdata.EvTwoFactorKO, r, "")
		s.render(w, r, nil, http.StatusUnauthorized, "login_2fa", &Page{TitleKey: "twofactor.login_title", ErrorKey: "twofactor.bad_code", Next: safeNext(r.PostFormValue("next"))})
		return
	}
	s.pending.done(c.Value)
	http.SetCookie(w, &http.Cookie{Name: pendingCookie, Value: "", Path: "/login", MaxAge: -1, HttpOnly: true})
	if r.PostFormValue("trust") != "" {
		s.setTrustCookie(w, r, acc)
	}
	if err := s.startSession(w, r, acc, ""); err != nil {
		acc.Wipe()
		s.serverError(w, r, nil, err)
		return
	}
	s.log.Info("connexion (second facteur)", "login", store.ReduceLogin(acc.Login), "ip", s.clientIP(r))
	http.Redirect(w, r, safeNext(r.PostFormValue("next")), http.StatusSeeOther)
}

// --- réglages ---

// TwoFactorView alimente la page de l'authentification en deux étapes.
type TwoFactorView struct {
	Enabled, Setup bool
	Secret         string
	BackupLeft     int
	NewCodes       []string
	ShowSecret     string // secret révélé après un code valide (réinstaller l'application)
}

// groupSecret coupe la clé en groupes de quatre caractères : plus facile à recopier dans l'application.
func groupSecret(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// revealTTL : durée pendant laquelle un secret révélé reste affichable (page et QR).
const revealTTL = 2 * time.Minute

// twoFactorShow révèle de nouveau le secret (et le QR) après un code valide : installer l'application sur un autre appareil.
func (s *Server) twoFactorShow(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	if !s.codeLimit(w, r, sess) {
		return
	}
	if !s.secondFactorOK(sess, r.PostFormValue("code")) {
		s.codeFailed(r, sess)
		s.twoFactorPage(w, r, sess, http.StatusUnauthorized, nil, "twofactor.bad_code")
		return
	}
	sess.Mu.Lock()
	sess.TwoFactorReveal = sess.Account.Data.TwoFactor.Secret
	sess.TwoFactorRevealUntil = time.Now().Add(revealTTL) // le QR est demandé après la page : garder le secret un court moment
	sess.Mu.Unlock()
	s.log.Info("secret du second facteur affiché de nouveau", "login", store.ReduceLogin(sess.Account.Login))
	http.Redirect(w, r, "/settings/2fa", http.StatusSeeOther)
}

func (s *Server) twoFactorPage(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, codes []string, errKey string) {
	sess.Mu.Lock()
	tf := sess.Account.Data.TwoFactor
	setup := sess.TwoFactorSetup
	reveal := sess.TwoFactorReveal
	if reveal != "" && time.Now().After(sess.TwoFactorRevealUntil) {
		reveal, sess.TwoFactorReveal = "", "" // délai passé : le secret n'est plus affiché
	}
	sess.Mu.Unlock()
	v := &TwoFactorView{Enabled: tf.Enabled, Setup: setup != "" && !tf.Enabled, Secret: groupSecret(setup), BackupLeft: len(tf.BackupHashes), NewCodes: codes, ShowSecret: groupSecret(reveal)}
	s.render(w, r, sess, status, "twofactor", &Page{TitleKey: "twofactor.title", TwoFactor: v, Section: "security", ErrorKey: errKey})
}

func (s *Server) twoFactorGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.twoFactorPage(w, r, sess, http.StatusOK, nil, "")
}

func (s *Server) twoFactorStart(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	if !sess.Account.Data.TwoFactor.Enabled {
		sess.TwoFactorSetup = totp.NewSecret()
	}
	sess.Mu.Unlock()
	http.Redirect(w, r, "/settings/2fa", http.StatusSeeOther)
}

func (s *Server) twoFactorQR(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	secret, login := sess.TwoFactorSetup, sess.Account.Login
	if secret == "" && r.URL.Query().Get("again") == "1" && sess.TwoFactorReveal != "" && time.Now().Before(sess.TwoFactorRevealUntil) {
		secret = sess.TwoFactorReveal // QR du secret révélé (visible le temps de revealTTL)
	}
	sess.Mu.Unlock()
	if secret == "" {
		http.NotFound(w, r)
		return
	}
	png, err := qrcode.Encode(totp.URI("wmail-go", login, secret), qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

// codeLimit applique la limitation des essais de code dans les réglages (même règle que la connexion).
func (s *Server) codeLimit(w http.ResponseWriter, r *http.Request, sess *session.Session) bool {
	ipKey, userKey := s.limitKeys("twofa-settings", r, sess.Account.Login)
	if wait, blocked := s.checkLimits(s.cfg.RateLimit.Login, ipKey, userKey); blocked {
		s.rateLimited(w, r, sess, wait)
		return false
	}
	return true
}

func (s *Server) codeFailed(r *http.Request, sess *session.Session) {
	ipKey, userKey := s.limitKeys("twofa-settings", r, sess.Account.Login)
	s.addFailure(s.cfg.RateLimit.Login, ipKey, userKey)
}

func (s *Server) twoFactorEnable(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	if !s.codeLimit(w, r, sess) {
		return
	}
	sess.Mu.Lock()
	secret := sess.TwoFactorSetup
	sess.Mu.Unlock()
	step, ok := totp.Verify(secret, r.PostFormValue("code"), time.Now(), 0)
	if secret == "" || !ok {
		s.codeFailed(r, sess)
		s.twoFactorPage(w, r, sess, http.StatusBadRequest, nil, "twofactor.bad_code")
		return
	}
	codes, hashes := totp.NewBackupCodes()
	sess.Mu.Lock()
	prev := sess.Account.Data.TwoFactor
	sess.Account.Data.TwoFactor = userdata.TwoFactor{Enabled: true, Secret: secret, LastStep: step, BackupHashes: hashes, TrustKey: newTrustKey()}
	err := s.store.SaveData(sess.Account)
	if err != nil {
		sess.Account.Data.TwoFactor = prev
	} else {
		sess.TwoFactorSetup = ""
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.log.Info("authentification en deux étapes activée", "login", store.ReduceLogin(sess.Account.Login))
	s.traceSession(sess, userdata.EvTwoFactorOn, r, "")
	s.twoFactorPage(w, r, sess, http.StatusOK, codes, "")
}

// secondFactorOK vérifie un code (TOTP ou de secours) saisi dans les réglages, sans consommer le pas de temps.
func (s *Server) secondFactorOK(sess *session.Session, code string) bool {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return s.checkSecondFactor(sess.Account, code)
}

func (s *Server) twoFactorRegen(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	if !s.codeLimit(w, r, sess) {
		return
	}
	if !s.secondFactorOK(sess, r.PostFormValue("code")) {
		s.codeFailed(r, sess)
		s.twoFactorPage(w, r, sess, http.StatusBadRequest, nil, "twofactor.bad_code")
		return
	}
	codes, hashes := totp.NewBackupCodes()
	sess.Mu.Lock()
	prev := sess.Account.Data.TwoFactor.BackupHashes
	sess.Account.Data.TwoFactor.BackupHashes = hashes
	err := s.store.SaveData(sess.Account)
	if err != nil {
		sess.Account.Data.TwoFactor.BackupHashes = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.twoFactorPage(w, r, sess, http.StatusOK, codes, "")
}

func (s *Server) twoFactorDisable(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	if !s.codeLimit(w, r, sess) {
		return
	}
	if !s.secondFactorOK(sess, r.PostFormValue("code")) {
		s.codeFailed(r, sess)
		s.twoFactorPage(w, r, sess, http.StatusBadRequest, nil, "twofactor.bad_code")
		return
	}
	sess.Mu.Lock()
	prev := sess.Account.Data.TwoFactor
	sess.Account.Data.TwoFactor = userdata.TwoFactor{TrustKey: newTrustKey()} // invalide aussi les cookies « de confiance »
	err := s.store.SaveData(sess.Account)
	if err != nil {
		sess.Account.Data.TwoFactor = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.log.Info("authentification en deux étapes désactivée", "login", store.ReduceLogin(sess.Account.Login))
	s.traceSession(sess, userdata.EvTwoFactorOff, r, "")
	s.twoFactorPage(w, r, sess, http.StatusOK, nil, "")
}
