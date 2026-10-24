package compose

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	gomail "github.com/emersion/go-message/mail"
)

// Attachment est une pièce jointe à envoyer.
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// DraftInfo relie un brouillon au message auquel il répond ou qu'il transfère.
type DraftInfo struct {
	Kind   string // « reply » ou « forward »
	UID    uint32
	Folder string
}

// Message est un message sortant.
type Message struct {
	From          *mail.Address
	To, Cc, Bcc   []*mail.Address
	ReplyTo       []*mail.Address
	Subject       string
	Text          string
	HTML          string
	Images        []InlineImage
	InReplyTo     string   // sans chevrons
	References    []string // sans chevrons
	Date          time.Time
	MessageID     string // sans chevrons ; généré si vide
	XMailer       string // vide : aucun en-tête
	OriginatingIP string // vide : aucun en-tête X-Originating-IP
	Important     bool
	ReadReceipt   bool // demande de confirmation de lecture, adressée à l'expéditeur
	Draft         *DraftInfo
	Attachments   []Attachment
}

// ErrNoRecipient : ni À, ni Cc, ni Cci.
var ErrNoRecipient = errors.New("aucun destinataire")

// Recipients renvoie les adresses d'enveloppe (À, Cc, Cci), sans doublon.
func asciiAddr(a *mail.Address) *mail.Address {
	if a == nil {
		return nil
	}
	c := *a
	c.Address = ToASCIIDomain(c.Address)
	return &c
}

func asciiList(l []*mail.Address) []*mail.Address {
	out := make([]*mail.Address, 0, len(l))
	for _, a := range l {
		out = append(out, asciiAddr(a))
	}
	return out
}

func (m *Message) Recipients() []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range [][]*mail.Address{m.To, m.Cc, m.Bcc} {
		for _, a := range l {
			addr := ToASCIIDomain(a.Address) // domaine en punycode : l'enveloppe SMTP n'accepte pas l'Unicode
			k := strings.ToLower(addr)
			if !seen[k] {
				seen[k] = true
				out = append(out, addr)
			}
		}
	}
	return out
}

// NewMessageID fabrique un identifiant unique dont le domaine est celui de l'expéditeur.
func NewMessageID(fromAddr string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	d := Domain(fromAddr)
	if d == "" {
		d = "localhost"
	}
	return fmt.Sprintf("%d.%s@%s", time.Now().UnixNano(), hex.EncodeToString(b), d)
}

func cleanLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\r' || r == '\n' || r == 0 }), " ")
}

// safeFilename garde un nom de fichier simple : pas de chemin, pas de caractère de contrôle.
func safeFilename(name string) string {
	name = cleanLine(name)
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		return "piece-jointe"
	}
	return name
}

// Build assemble le message au format RFC 5322. includeBcc ajoute l'en-tête Cci : à réserver à la copie conservée
// dans « Envoyés » (le message transmis par SMTP ne doit jamais l'afficher).
func (m *Message) Build(includeBcc bool) ([]byte, error) {
	if m.From == nil || m.From.Address == "" {
		return nil, errors.New("expéditeur manquant")
	}
	if hasControl(m.From.Name) || hasControl(m.From.Address) {
		return nil, fmt.Errorf("%w : expéditeur", ErrInvalidAddress)
	}

	var h gomail.Header
	h.SetDate(orNow(m.Date))
	h.SetAddressList("From", []*gomail.Address{asciiAddr(m.From)})
	setList := func(key string, l []*mail.Address) {
		if len(l) > 0 {
			h.SetAddressList(key, asciiList(l)) // domaines en punycode dans les en-têtes aussi
		}
	}
	setList("To", m.To)
	setList("Cc", m.Cc)
	if includeBcc {
		setList("Bcc", m.Bcc)
	}
	setList("Reply-To", m.ReplyTo)
	h.SetSubject(cleanLine(m.Subject))
	id := strings.Trim(cleanLine(m.MessageID), "<> ")
	if id == "" {
		id = NewMessageID(m.From.Address)
	}
	h.Set("Message-Id", "<"+id+">")
	if v := strings.Trim(cleanLine(m.InReplyTo), "<> "); v != "" {
		h.Set("In-Reply-To", "<"+v+">")
	}
	if len(m.References) > 0 {
		ids := make([]string, 0, len(m.References))
		for _, r := range m.References {
			if r = strings.Trim(cleanLine(r), "<> "); r != "" {
				ids = append(ids, "<"+r+">")
			}
		}
		if len(ids) > 0 {
			h.Set("References", strings.Join(ids, " "))
		}
	}
	h.Set("MIME-Version", "1.0")
	if m.XMailer != "" {
		h.Set("X-Mailer", cleanLine(m.XMailer))
	}
	if m.OriginatingIP != "" {
		h.Set("X-Originating-IP", cleanLine(m.OriginatingIP))
	}
	if m.Important {
		h.Set("Importance", "High")
		h.Set("X-Priority", "1 (Highest)")
	}
	if m.ReadReceipt {
		h.SetAddressList("Disposition-Notification-To", []*gomail.Address{asciiAddr(m.From)})
	}
	if m.Draft != nil && m.Draft.Kind != "" {
		h.Set("X-Wmail-Draft-Info", EncodeDraftInfo(*m.Draft))
	}

	var buf bytes.Buffer
	text := strings.ReplaceAll(strings.ReplaceAll(m.Text, "\r\n", "\n"), "\r", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")

	if m.HTML != "" {
		return m.buildHTML(&h, text)
	}

	if len(m.Attachments) == 0 {
		var th message.Header
		th.Set("Content-Type", "text/plain; charset=utf-8")
		th.Set("Content-Transfer-Encoding", "quoted-printable")
		// En-têtes globaux et type du texte fusionnés : un seul niveau, pas de multipart.
		for f := h.Fields(); f.Next(); {
			th.Add(f.Key(), f.Value())
		}
		w, err := message.CreateWriter(&buf, th)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, text); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	mw, err := gomail.CreateWriter(&buf, h)
	if err != nil {
		return nil, err
	}
	var ih gomail.InlineHeader
	ih.Set("Content-Type", "text/plain; charset=utf-8")
	ih.Set("Content-Transfer-Encoding", "quoted-printable")
	tw, err := mw.CreateSingleInline(ih)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(tw, text); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	for _, a := range m.Attachments {
		var ah gomail.AttachmentHeader
		ct := a.ContentType
		if ct == "" || strings.ContainsAny(ct, "\r\n") {
			ct = "application/octet-stream"
		}
		if mt, _, err := mime.ParseMediaType(ct); err == nil {
			ct = mt
		} else {
			ct = "application/octet-stream"
		}
		ah.Set("Content-Type", ct)
		ah.Set("Content-Transfer-Encoding", "base64")
		ah.SetFilename(safeFilename(a.Name))
		aw, err := mw.CreateAttachment(ah)
		if err != nil {
			return nil, err
		}
		if _, err := aw.Write(a.Data); err != nil {
			return nil, err
		}
		if err := aw.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

// EncodeDraftInfo sérialise le lien d'un brouillon avec son message d'origine (dans un en-tête privé).
func EncodeDraftInfo(d DraftInfo) string {
	return mime.FormatMediaType(d.Kind, map[string]string{
		"uid":    fmt.Sprint(d.UID),
		"folder": hex.EncodeToString([]byte(d.Folder)),
	})
}

// ParseDraftInfo relit l'en-tête X-Wmail-Draft-Info ; ok est faux s'il est absent ou invalide.
func ParseDraftInfo(v string) (DraftInfo, bool) {
	kind, params, err := mime.ParseMediaType(strings.TrimSpace(v))
	if err != nil || (kind != "reply" && kind != "forward") {
		return DraftInfo{}, false
	}
	var uid uint32
	if _, err := fmt.Sscanf(params["uid"], "%d", &uid); err != nil || uid == 0 {
		return DraftInfo{}, false
	}
	b, err := hex.DecodeString(params["folder"])
	if err != nil || len(b) == 0 || hasControl(string(b)) {
		return DraftInfo{}, false
	}
	return DraftInfo{Kind: kind, UID: uid, Folder: string(b)}, true
}
