package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"wmail-go/internal/config"
	"wmail-go/internal/mailview"
	"wmail-go/internal/session"
	"wmail-go/internal/userdata"
)

const (
	maxTrustedSenders = 300
	maxProxyImage     = 5 << 20
	proxyTimeout      = 10 * time.Second
)

var proxyLimit = config.Limit{Count: 400, Window: time.Minute}

// blockRemote dit si les images distantes d'un message sont bloquées : option de la boîte, sauf demande de l'utilisateur
// pour ce message ou expéditeur de confiance.
func (s *Server) blockRemote(sess *session.Session, acc userdata.MailAccount, from []mailview.Address, remote bool) bool {
	if !acc.Options.BlockRemoteContent || remote {
		return false
	}
	if len(from) == 1 {
		sess.Mu.Lock()
		trusted := slices.Contains(sess.Account.Data.Settings.TrustedSenders, strings.ToLower(from[0].Email))
		sess.Mu.Unlock()
		if trusted {
			return false
		}
	}
	return true
}

func proxyMAC(key, remote string) string {
	m := hmac.New(sha256.New, []byte("wmail-go image proxy/v1:"+key))
	m.Write([]byte(remote))
	return hex.EncodeToString(m.Sum(nil))
}

// proxyURLFunc renvoie la fonction qui réécrit les images distantes vers /mail/image (signée pour cette session), ou nil si
// la boîte charge les images directement.
func (s *Server) proxyURLFunc(sess *session.Session, acc userdata.MailAccount) func(string) string {
	if !acc.Options.ProxyRemoteImages {
		return nil
	}
	key := sess.CSRF
	return func(remote string) string {
		return "/mail/image?" + url.Values{"u": {base64.RawURLEncoding.EncodeToString([]byte(remote))}, "sig": {proxyMAC(key, remote)}}.Encode()
	}
}

// mailImage charge une image distante pour le lecteur : adresse signée, connexion par le garde anti-SSRF, taille bornée,
// seuls les formats raster reconnus d'après les octets sont rendus.
func (s *Server) mailImage(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	raw, err := base64.RawURLEncoding.DecodeString(r.URL.Query().Get("u"))
	remote := string(raw)
	u, perr := url.Parse(remote)
	if err != nil || perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || len(remote) > 2000 ||
		!hmac.Equal([]byte(proxyMAC(sess.CSRF, remote)), []byte(r.URL.Query().Get("sig"))) {
		http.NotFound(w, r)
		return
	}
	key := "imgproxy:" + sess.ID
	if blocked, _ := s.limiter.Check(key, proxyLimit); blocked {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	s.limiter.Add(key, proxyLimit)
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, remote, nil)
	req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/webp")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; wmail-go image proxy)")
	resp, err := s.guard.HTTPClient(proxyTimeout).Do(req)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyImage+1))
	if err != nil || len(data) > maxProxyImage {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	ct := http.DetectContentType(data[:min(len(data), 512)])
	switch ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}

// trustSender ajoute ou retire un expéditeur dont les images distantes s'affichent d'emblée.
func (s *Server) trustSender(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	email := strings.ToLower(strings.TrimSpace(r.PostFormValue("email")))
	remove := r.PostFormValue("action") == "remove"
	if email == "" || len(email) > 254 || !strings.Contains(email, "@") || strings.ContainsAny(email, " <>,;\"\r\n") {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	sess.Mu.Lock()
	set := &sess.Account.Data.Settings
	prev := slices.Clone(set.TrustedSenders)
	switch {
	case remove:
		set.TrustedSenders = slices.DeleteFunc(set.TrustedSenders, func(e string) bool { return e == email })
	case !slices.Contains(set.TrustedSenders, email):
		if len(set.TrustedSenders) >= maxTrustedSenders {
			set.TrustedSenders = set.TrustedSenders[1:] // les plus anciens s'effacent
		}
		set.TrustedSenders = append(set.TrustedSenders, email)
	}
	err := s.store.SaveData(sess.Account)
	if err != nil {
		set.TrustedSenders = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	next := safeBack(r.PostFormValue("stay"))
	if remove {
		next = "/settings"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}
