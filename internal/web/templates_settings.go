package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"
	"time"

	"wmail-go/internal/session"
	"wmail-go/internal/userdata"
)

const (
	maxTemplates    = 50
	maxTemplateName = 100
	maxTemplateBody = 20000
)

// TemplateRow est une ligne de la liste des modèles.
type TemplateRow struct{ ID, Name, Preview string }

// TemplatesView alimente la page des modèles de message.
type TemplatesView struct {
	Rows    []TemplateRow
	Edit    userdata.Template
	Editing bool
	Full    bool
}

func (s *Server) templatesPage(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, edit userdata.Template, errKey string) {
	sess.Mu.Lock()
	list := slices.Clone(sess.Account.Data.Templates)
	sess.Mu.Unlock()
	v := &TemplatesView{Edit: edit, Editing: edit.ID != "", Full: len(list) >= maxTemplates}
	for _, t := range list {
		pv := strings.Join(strings.Fields(t.Body), " ")
		if r := []rune(pv); len(r) > 100 {
			pv = string(r[:100]) + "…"
		}
		v.Rows = append(v.Rows, TemplateRow{t.ID, t.Name, pv})
	}
	s.render(w, r, sess, status, "templates", &Page{TitleKey: "set.templates", Templates: v, ErrorKey: errKey})
}

func (s *Server) templatesGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	var edit userdata.Template
	if id := r.URL.Query().Get("edit"); id != "" {
		sess.Mu.Lock()
		for _, t := range sess.Account.Data.Templates {
			if t.ID == id {
				edit = t
			}
		}
		sess.Mu.Unlock()
	}
	s.templatesPage(w, r, sess, http.StatusOK, edit, "")
}

func (s *Server) templatesSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	in := userdata.Template{ID: r.PostFormValue("id"), Name: strings.TrimSpace(r.PostFormValue("name")), Body: strings.ReplaceAll(r.PostFormValue("body"), "\r\n", "\n")}
	if in.Name == "" || strings.TrimSpace(in.Body) == "" || strings.ContainsAny(in.Name, "\r\n") || len([]rune(in.Name)) > maxTemplateName || len(in.Body) > maxTemplateBody {
		s.templatesPage(w, r, sess, http.StatusBadRequest, in, "templates.err.invalid")
		return
	}
	sess.Mu.Lock()
	data := sess.Account.Data
	prev := slices.Clone(data.Templates)
	idx := slices.IndexFunc(data.Templates, func(t userdata.Template) bool { return in.ID != "" && t.ID == in.ID })
	switch {
	case idx >= 0:
		data.Templates[idx] = in
	case in.ID != "":
		sess.Mu.Unlock()
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	case len(data.Templates) >= maxTemplates:
		sess.Mu.Unlock()
		s.templatesPage(w, r, sess, http.StatusBadRequest, in, "templates.err.full")
		return
	default:
		var b [8]byte
		_, _ = rand.Read(b[:])
		in.ID = hex.EncodeToString(b[:])
		data.Templates = append(data.Templates, in)
	}
	err := s.store.SaveData(sess.Account)
	if err != nil {
		data.Templates = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, "/settings/templates", http.StatusSeeOther)
}

func (s *Server) templatesDelete(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	id := r.PostFormValue("id")
	sess.Mu.Lock()
	data := sess.Account.Data
	prev := slices.Clone(data.Templates)
	data.Templates = slices.DeleteFunc(data.Templates, func(t userdata.Template) bool { return t.ID == id })
	err := s.store.SaveData(sess.Account)
	if err != nil {
		data.Templates = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, "/settings/templates", http.StatusSeeOther)
}

// templatesBody renvoie le texte d'un modèle (variables de date remplacées), pour l'insérer dans la composition.
func (s *Server) templatesBody(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	id := r.URL.Query().Get("id")
	var body string
	found := false
	sess.Mu.Lock()
	for _, t := range sess.Account.Data.Templates {
		if t.ID == id {
			body, found = t.Body, true
		}
	}
	sess.Mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}
	now := time.Now()
	body = strings.NewReplacer("{{DATE}}", now.Format("2006-01-02"), "{{TIME}}", now.Format("15:04")).Replace(body)
	body = momentRe.ReplaceAllStringFunc(body, func(m string) string { return momentFormat(momentRe.FindStringSubmatch(m)[1], now) })
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(body))
}
