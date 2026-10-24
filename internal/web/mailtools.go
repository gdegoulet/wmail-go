package web

import (
	"bytes"
	"html"
	"mime"
	"net/http"
	"strings"
	"time"

	"wmail-go/internal/mailview"
	"wmail-go/internal/session"
)

// mailSource sert la source d'un message : tout (view=source), seulement les en-têtes (view=headers) ou le fichier
// .eml en téléchargement (view=eml). La source est toujours du texte brut : jamais interprétée par le navigateur.
func (s *Server) mailSource(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	folder, uid, ok := s.messageParams(w, r, sess, q)
	if !ok {
		return
	}
	view := q.Get("view")
	if view != "source" && view != "headers" && view != "eml" {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	raw, m, err := s.fetchMessage(ctx, sess, acc, folder, uid, false)
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	data := raw.Raw
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "private, no-store")
	if view == "eml" {
		name := safeEMLName(strings.TrimSpace(m.Subject))
		if name == ".eml" {
			name = "message.eml"
		}
		h.Set("Content-Type", "message/rfc822")
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		h.Set("Accept-Ranges", "bytes")
		http.ServeContent(w, r, "message.eml", time.Time{}, bytes.NewReader(data))
		return
	}
	if view == "headers" {
		text := string(data)
		for _, sep := range []string{"\r\n\r\n", "\n\n"} {
			if i := strings.Index(text, sep); i >= 0 {
				text = text[:i]
				break
			}
		}
		data = []byte(text)
	}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(strings.ToValidUTF8(string(data), "\uFFFD")))
}

// mailPrint sert une version imprimable : en-têtes, liste des pièces jointes et corps assaini. La page n'autorise
// que le script d'impression de l'application ; le contenu du message ne peut exécuter aucun script.
func (s *Server) mailPrint(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	folder, uid, ok := s.messageParams(w, r, sess, q)
	if !ok {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	_, m, err := s.fetchMessage(ctx, sess, acc, folder, uid, false)
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	lang := s.lang(r, sess)
	t := func(k string) string { return s.i18n.T(lang, k) }
	block := s.blockRemote(sess, acc, m.From, q.Get("remote") == "1")
	var body string
	switch {
	case m.HTML != "":
		body, _ = mailview.Sanitize(m.HTML, mailview.SanitizeOptions{BlockRemote: block, InlineImages: m.InlineImages})
	case m.Text != "":
		body = mailview.TextToHTML(m.Text)
	}
	list := func(l []mailview.Address) string {
		parts := make([]string, len(l))
		for i, a := range l {
			parts[i] = html.EscapeString(a.String())
		}
		return strings.Join(parts, ", ")
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"" + lang + "\"><head><meta charset=\"utf-8\"><meta name=\"referrer\" content=\"no-referrer\"><title>" + html.EscapeString(m.Subject) + "</title>")
	b.WriteString("<link rel=\"stylesheet\" href=\"/static/print.css\"></head><body>")
	b.WriteString("<p class=\"noprint\"><button type=\"button\" id=\"print-btn\">" + html.EscapeString(t("mail.print")) + "</button></p>")
	b.WriteString("<h1>" + html.EscapeString(m.Subject) + "</h1><table class=\"hdr\">")
	row := func(label, value string) {
		if value != "" {
			b.WriteString("<tr><th>" + html.EscapeString(label) + "</th><td>" + value + "</td></tr>")
		}
	}
	row(t("mail.from"), list(m.From))
	row(t("mail.to"), list(m.To))
	row(t("mail.cc"), list(m.Cc))
	row(t("mail.date"), html.EscapeString(formatFullDate(m.Date, lang, s.userLoc(sess))))
	if len(m.Attachments) > 0 {
		names := make([]string, len(m.Attachments))
		for i, a := range m.Attachments {
			names[i] = html.EscapeString(a.Name + " (" + humanSize(int64(a.Size), lang) + ")")
		}
		row(t("mail.attachments"), strings.Join(names, ", "))
	}
	b.WriteString("</table><div class=\"body\">" + body + "</div><script src=\"/static/print.js\"></script></body></html>")
	h := w.Header()
	img := "img-src data:"
	if !block {
		img += " https: http:"
	}
	h.Set("Content-Security-Policy", "default-src 'none'; "+img+"; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "private, no-store")
	_, _ = w.Write([]byte(b.String()))
}
