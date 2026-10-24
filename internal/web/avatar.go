package web

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"wmail-go/internal/config"
	"wmail-go/internal/session"
)

const (
	avatarSize     = 80
	maxAvatarBytes = 512 << 10
	avatarTTL      = 24 * time.Hour
	avatarCacheMax = 500
)

var avatarLimit = config.Limit{Count: 120, Window: time.Minute}

// avatarCache garde en mémoire les images déjà demandées (et les absences), pour ne pas interroger Gravatar à chaque
// ouverture de message. Rien n'est écrit sur le disque.
type avatarCache struct {
	mu sync.Mutex
	m  map[string]avatarEntry
}

type avatarEntry struct {
	data    []byte
	ctype   string
	expires time.Time
}

func (c *avatarCache) get(key string) (avatarEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.expires) {
		return avatarEntry{}, false
	}
	return e, true
}

func (c *avatarCache) put(key string, e avatarEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]avatarEntry{}
	}
	if len(c.m) >= avatarCacheMax {
		now := time.Now()
		for k, v := range c.m {
			if now.After(v.expires) {
				delete(c.m, k)
			}
		}
		for k := range c.m { // toujours plein : on fait de la place au hasard
			if len(c.m) < avatarCacheMax {
				break
			}
			delete(c.m, k)
		}
	}
	c.m[key] = e
}

// gravatarHash : empreinte MD5 de l'adresse en minuscules, comme l'attend Gravatar.
func gravatarHash(email string) string {
	sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// avatarPossible dit s'il vaut la peine de demander une photo : une photo du carnet, ou Gravatar tant que le cache
// ne nous a pas appris que ce correspondant n'en a pas. Évite une requête (et un 404 dans la console) par message.
func (s *Server) avatarPossible(sess *session.Session, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return false
	}
	if _, _, ok := s.contactPhoto(sess, email); ok {
		return true
	}
	sess.Mu.Lock()
	mode := sess.Account.Data.Settings.Avatars
	sess.Mu.Unlock()
	if mode != "gravatar" {
		return false
	}
	if e, ok := s.avatars.get(gravatarHash(email)); ok {
		return len(e.data) > 0
	}
	return true // jamais demandé : on tente une fois, le résultat est mis en cache
}

// mailAvatar sert la photo d'un correspondant. Elle vient d'abord du carnet d'adresses ; si l'utilisateur a autorisé
// Gravatar, elle est ensuite demandée à gravatar.com **par le serveur** (le navigateur ne contacte jamais ce service).
// Sans photo, la réponse est 404 et l'interface garde la silhouette.
func (s *Server) mailAvatar(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	if email == "" || len(email) > 254 || !strings.Contains(email, "@") || strings.ContainsAny(email, " <>\r\n") {
		http.NotFound(w, r)
		return
	}
	if photo, ctype, ok := s.contactPhoto(sess, email); ok {
		serveAvatar(w, ctype, photo)
		return
	}
	sess.Mu.Lock()
	mode := sess.Account.Data.Settings.Avatars
	sess.Mu.Unlock()
	if mode != "gravatar" {
		http.NotFound(w, r)
		return
	}
	key := gravatarHash(email)
	if e, ok := s.avatars.get(key); ok {
		if len(e.data) == 0 {
			http.NotFound(w, r)
			return
		}
		serveAvatar(w, e.ctype, e.data)
		return
	}
	limitKey := "avatar:" + sess.ID
	if blocked, _ := s.limiter.Check(limitKey, avatarLimit); blocked {
		http.NotFound(w, r)
		return
	}
	s.limiter.Add(limitKey, avatarLimit)
	data, ctype := s.fetchGravatar(r, key)
	s.avatars.put(key, avatarEntry{data: data, ctype: ctype, expires: time.Now().Add(avatarTTL)})
	if len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	serveAvatar(w, ctype, data)
}

func serveAvatar(w http.ResponseWriter, ctype string, data []byte) {
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data)
}

// fetchGravatar interroge gravatar.com ; « d=404 » évite l'image générique, pour garder notre silhouette.
func (s *Server) fetchGravatar(r *http.Request, hash string) ([]byte, string) {
	url := "https://www.gravatar.com/avatar/" + hash + "?s=" + itoa(avatarSize) + "&d=404"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, ""
	}
	req.Header.Set("Accept", "image/png,image/jpeg")
	resp, err := s.guard.HTTPClient(proxyTimeout).Do(req)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAvatarBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxAvatarBytes {
		return nil, ""
	}
	switch ct := http.DetectContentType(data[:min(len(data), 512)]); ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return data, ct
	}
	return nil, ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
