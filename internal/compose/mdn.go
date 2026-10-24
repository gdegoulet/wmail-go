package compose

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

// MDN décrit un accusé de lecture (RFC 8098) à envoyer à la demande de l'utilisateur.
type MDN struct {
	From         *mail.Address // identité qui a reçu le message
	To           *mail.Address // adresse de Disposition-Notification-To
	OrigSubject  string
	OrigID       string // Message-ID d'origine, sans chevrons
	Text         string // explication lisible (traduite par l'appelant)
	SubjectLabel string // préfixe de l'objet, par exemple « Accusé de lecture : »
	XMailer      string
	Date         time.Time
}

// Build assemble le message multipart/report : explication lisible puis message/disposition-notification.
func (m MDN) Build() ([]byte, error) {
	if m.From == nil || m.To == nil || m.From.Address == "" || m.To.Address == "" {
		return nil, fmt.Errorf("%w : accusé de lecture", ErrInvalidAddress)
	}
	if hasControl(m.From.Address) || hasControl(m.To.Address) || hasControl(m.From.Name) || hasControl(m.To.Name) {
		return nil, fmt.Errorf("%w : accusé de lecture", ErrInvalidAddress)
	}
	var h gomail.Header
	h.SetDate(orNow(m.Date))
	h.SetAddressList("From", []*gomail.Address{m.From})
	h.SetAddressList("To", []*gomail.Address{m.To})
	h.SetSubject(cleanLine(strings.TrimSpace(m.SubjectLabel + " " + m.OrigSubject)))
	h.Set("Message-Id", "<"+NewMessageID(m.From.Address)+">")
	h.Set("Mime-Version", "1.0")
	h.Set("Auto-Submitted", "auto-replied")
	if m.XMailer != "" {
		h.Set("X-Mailer", cleanLine(m.XMailer))
	}
	bnd := boundary()
	h.Set("Content-Type", fmt.Sprintf("multipart/report; report-type=disposition-notification; boundary=\"%s\"", bnd))

	var body bytes.Buffer
	fmt.Fprintf(&body, "--%s\r\n", bnd)
	writePartHeader(&body, partHeader{{"Content-Type", "text/plain; charset=utf-8"}, {"Content-Transfer-Encoding", "quoted-printable"}})
	writeQP(&body, strings.ReplaceAll(strings.ReplaceAll(m.Text, "\r\n", "\n"), "\n", "\r\n"))
	fmt.Fprintf(&body, "--%s\r\n", bnd)
	writePartHeader(&body, partHeader{{"Content-Type", "message/disposition-notification"}})
	lines := []string{
		"Reporting-UA: wmail-go; webmail",
		"Final-Recipient: rfc822; " + m.From.Address,
	}
	if id := strings.Trim(cleanLine(m.OrigID), "<> "); id != "" {
		lines = append(lines, "Original-Message-ID: <"+id+">")
	}
	lines = append(lines, "Disposition: manual-action/MDN-sent-manually; displayed")
	body.WriteString(strings.Join(lines, "\r\n") + "\r\n")
	fmt.Fprintf(&body, "--%s--\r\n", bnd)
	return assemble(&h, body.Bytes()), nil
}
