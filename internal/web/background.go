package web

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"wmail-go/internal/session"
)

const (
	maxBackground    = 2 << 20
	maxBackgroundDim = 6000
)

var _ = jpeg.Decode // enregistre le décodeur JPEG pour image.DecodeConfig
var _ = png.Decode

// backgroundSave enregistre l'image de fond personnelle (JPEG ou PNG), chiffrée comme le reste du compte.
func (s *Server) backgroundSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	fail := func() {
		p := s.settingsPage(sess)
		p.ErrorKey = "settings.background.invalid"
		s.render(w, r, sess, http.StatusBadRequest, "settings", p)
	}
	if err := r.ParseMultipartForm(maxBackground); err != nil {
		fail()
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		fail()
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBackground+1))
	if err != nil || len(data) > maxBackground {
		fail()
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxBackgroundDim || cfg.Height > maxBackgroundDim {
		fail()
		return
	}
	kind := byte('j')
	if format == "png" {
		kind = 'p'
	}
	sess.Mu.Lock()
	acc := sess.Account
	sess.Mu.Unlock()
	if err := s.store.SaveBlob(acc, "background", append([]byte{kind}, data...)); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	sess.Mu.Lock()
	acc.Data.Settings.Background++
	err = s.store.SaveData(acc)
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) backgroundDelete(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	acc := sess.Account
	sess.Mu.Unlock()
	if err := s.store.SaveBlob(acc, "background", nil); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	sess.Mu.Lock()
	acc.Data.Settings.Background = 0
	err := s.store.SaveData(acc)
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) backgroundGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	acc := sess.Account
	sess.Mu.Unlock()
	data, err := s.store.LoadBlob(acc, "background")
	if err != nil || len(data) < 2 {
		http.NotFound(w, r)
		return
	}
	ct := "image/jpeg"
	if data[0] == 'p' {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data[1:])
}
