package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/mailconn"
	"wmail-go/internal/mailview"
	"wmail-go/internal/session"
)

// mailAppend ajoute au dossier affiché un message déposé en fichier .eml (glisser-déposer sur la liste). Le fichier doit
// être un message RFC 822 lisible ; il est ajouté tel quel, marqué comme lu.
func (s *Server) mailAppend(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	reply := func(status int, v map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		reply(http.StatusConflict, map[string]any{"error": "no_account"})
		return
	}
	limit := int64(s.cfg.Compose.MaxMessageMB+s.cfg.Compose.MaxAttachmentMB) << 20
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		reply(http.StatusBadRequest, map[string]any{"error": "bad_request"})
		return
	}
	folder := r.FormValue("folder")
	f, _, err := r.FormFile("file")
	if err != nil || !mailconn.ValidFolderName(folder) {
		reply(http.StatusBadRequest, map[string]any{"error": "bad_request"})
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		reply(http.StatusRequestEntityTooLarge, map[string]any{"error": "too_large"})
		return
	}
	// Un message RFC 822 commence par des en-têtes ; on exige au moins un expéditeur, une date ou un objet.
	m, perr := mailview.Parse(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n")))
	if perr != nil || (len(m.From) == 0 && m.Subject == "" && m.Date.IsZero()) {
		reply(http.StatusUnprocessableEntity, map[string]any{"error": "not_a_message"})
		return
	}
	raw = normalizeCRLF(raw)
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var uid uint32
	err = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		uid, err = c.Append(folder, raw, m.MessageID, imap.FlagSeen)
		return
	})
	if err != nil {
		s.log.Warn("ajout d'un message impossible", "err", shortErr(err))
		reply(http.StatusBadGateway, map[string]any{"error": "imap"})
		return
	}
	reply(http.StatusOK, map[string]any{"uid": uid})
}

// normalizeCRLF convertit les fins de ligne en CRLF (IMAP les exige).
func normalizeCRLF(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
}
