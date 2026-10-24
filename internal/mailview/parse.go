// Package mailview analyse un message MIME et prépare son affichage sûr :
// extraction du texte, du HTML et des pièces jointes, puis assainissement du HTML.
package mailview

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // jeux de caractères non UTF-8
	"github.com/emersion/go-message/mail"
)

const (
	maxTextPart   = 5 << 20  // texte ou HTML d'une partie
	maxAttachment = 25 << 20 // pièce jointe
	maxInline     = 2 << 20  // image incorporée (cid:)
	maxInlineAll  = 8 << 20  // total des images incorporées
)

// Address est une adresse de message.
type Address struct {
	Name, Email string
}

// String renvoie « Nom <adresse> » ou l'adresse seule.
func (a Address) String() string {
	if a.Name == "" {
		return a.Email
	}
	return fmt.Sprintf("%s <%s>", a.Name, a.Email)
}

// Short renvoie le nom, ou l'adresse à défaut.
func (a Address) Short() string {
	if a.Name != "" {
		return a.Name
	}
	return a.Email
}

// Attachment est une pièce jointe.
type Attachment struct {
	Index       int
	Name        string
	ContentType string
	Size        int
	data        []byte
}

// Message est un message analysé.
type Message struct {
	From, To, Cc, Bcc, ReplyTo []Address
	DeliveredTo                []Address // Delivered-To / X-Original-To : adresse réellement visée (aide à choisir l'identité)
	Subject                    string
	Date                       time.Time
	MessageID                  string
	InReplyTo                  string    // sans chevrons
	References                 []string  // sans chevrons
	ReadReceiptTo              []Address // Disposition-Notification-To : l'expéditeur demande une confirmation de lecture
	DraftInfo                  string    // en-tête privé X-Wmail-Draft-Info (brouillons)
	DKIM                       string    // pass (signature valide du domaine de l'expéditeur), warn (autre résultat) ou vide : lu dans Authentication-Results
	ListUnsubscribe            []string  // adresses de List-Unsubscribe (https:// ou mailto:), sans chevrons

	Text string // partie text/plain
	HTML string // partie text/html, non assainie

	Attachments []Attachment
	// InlineImages associe un Content-ID normalisé à une URI data: prête à l'emploi.
	InlineImages map[string]string

	Truncated bool // une partie dépasse les limites
	Partial   bool // le message est abîmé : seule une partie a pu être lue
}

func addresses(h mail.Header, key string) []Address {
	list, err := h.AddressList(key)
	if err != nil && len(list) == 0 {
		return nil
	}
	out := make([]Address, 0, len(list))
	for _, a := range list {
		out = append(out, Address{Name: a.Name, Email: a.Address})
	}
	return out
}

// NormalizeCID retire les chevrons et le préfixe « cid: » et met en minuscules.
func NormalizeCID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "cid:")
	return strings.Trim(s, "<> ")
}

var inlineImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

func readLimited(r io.Reader, limit int) ([]byte, bool, error) {
	b, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return b, false, err
	}
	if len(b) > limit {
		return b[:limit], true, nil
	}
	return b, false, nil
}

// Parse analyse un message brut (RFC 822).
func Parse(raw []byte) (*Message, error) {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil && !message.IsUnknownCharset(err) {
		return nil, fmt.Errorf("message illisible : %w", err)
	}
	m := &Message{InlineImages: map[string]string{}}
	m.From = addresses(mr.Header, "From")
	m.To = addresses(mr.Header, "To")
	m.Cc = addresses(mr.Header, "Cc")
	m.ReplyTo = addresses(mr.Header, "Reply-To")
	m.Bcc = addresses(mr.Header, "Bcc")
	m.DeliveredTo = append(addresses(mr.Header, "Delivered-To"), addresses(mr.Header, "X-Original-To")...)
	m.ReadReceiptTo = addresses(mr.Header, "Disposition-Notification-To")
	m.DraftInfo = mr.Header.Get("X-Wmail-Draft-Info")
	for _, part := range strings.Split(mr.Header.Get("List-Unsubscribe"), ",") {
		if u := strings.TrimSpace(part); strings.HasPrefix(u, "<") && strings.HasSuffix(u, ">") && len(u) < 2000 {
			u = u[1 : len(u)-1]
			if l := strings.ToLower(u); strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "mailto:") {
				m.ListUnsubscribe = append(m.ListUnsubscribe, u)
			}
		}
	}
	m.DKIM = dkimStatus(mr.Header.Values("Authentication-Results"), m.From)
	m.InReplyTo = strings.Trim(mr.Header.Get("In-Reply-To"), "<> ")
	for _, f := range strings.Fields(mr.Header.Get("References")) {
		if f = strings.Trim(f, "<>"); f != "" {
			m.References = append(m.References, f)
		}
	}
	m.Subject, _ = mr.Header.Subject()
	m.Date, _ = mr.Header.Date()
	m.MessageID = strings.Trim(mr.Header.Get("Message-Id"), "<> ")

	paths := leafPaths(raw)
	leaf := -1
	inlineTotal := 0
	for {
		part, err := mr.NextPart()
		leaf++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !(message.IsUnknownCharset(err) && part != nil) {
			m.Partial = true
			break
		}
		var hdr interface {
			ContentType() (string, map[string]string, error)
			Get(string) string
		}
		inline, filename := false, ""
		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			hdr, inline = h, true
		case *mail.AttachmentHeader:
			hdr = h
			filename, _ = h.Filename()
		default:
			continue
		}
		ct, params, _ := hdr.ContentType()
		ct = strings.ToLower(ct)
		cid := NormalizeCID(hdr.Get("Content-Id"))
		disposition := strings.ToLower(strings.TrimSpace(strings.SplitN(hdr.Get("Content-Disposition"), ";", 2)[0]))

		switch {
		case inlineImageTypes[ct] && cid != "" && disposition != "attachment":
			// Image incorporée au message (référencée par cid:) : elle n'est pas listée comme pièce jointe.
			if inlineTotal >= maxInlineAll {
				continue
			}
			b, trunc, _ := readLimited(part.Body, maxInline)
			if trunc {
				m.Truncated = true
				continue
			}
			inlineTotal += len(b)
			m.InlineImages[cid] = "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(b)
		case inline && ct == "text/plain" && m.Text == "":
			b, trunc, _ := readLimited(part.Body, maxTextPart)
			m.Text, m.Truncated = string(b), m.Truncated || trunc
			if strings.EqualFold(params["format"], "flowed") {
				m.Text = Unflow(m.Text, strings.EqualFold(params["delsp"], "yes"))
			}
		case inline && ct == "text/html" && m.HTML == "":
			b, trunc, _ := readLimited(part.Body, maxTextPart)
			m.HTML, m.Truncated = string(b), m.Truncated || trunc
		case inline && (ct == "text/plain" || ct == "text/html") && disposition != "attachment":
			// Deuxième partie texte ou HTML en ligne (signature ajoutée par le serveur ou le client, avertissement légal) :
			// elle suit le corps au lieu d'être listée comme pièce jointe.
			b, trunc, _ := readLimited(part.Body, maxTextPart)
			m.Truncated = m.Truncated || trunc
			if ct == "text/plain" {
				m.Text += "\n" + string(b)
			} else {
				m.HTML += "\n" + string(b)
			}
		case isSignaturePart(ct):
			// Signature S/MIME ou PGP détachée : ce n'est pas une pièce jointe pour l'utilisateur.
		default:
			name := filename
			if name == "" {
				name = params["name"]
			}
			if name == "" {
				name = defaultName(ct, leafPath(paths, leaf, len(m.Attachments)+1))
			}
			m.addAttachment(name, ct, part.Body)
		}
	}
	return m, nil
}

// defaultName nomme une partie sans nom comme RainLoop : « message1.2.eml », « calendar1.3.ics », « image1.2.png » (le numéro
// est celui de la partie MIME dans le message).
func defaultName(contentType, index string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case ct == "message/rfc822":
		return "message" + index + ".eml"
	case ct == "text/calendar":
		return "calendar" + index + ".ics"
	case ct == "":
		return "piece-jointe-" + index
	}
	return strings.Replace(ct, "/", index+".", 1)
}

func leafPath(paths []string, i, fallback int) string {
	if i >= 0 && i < len(paths) {
		return paths[i]
	}
	return strconv.Itoa(fallback)
}

// leafPaths numérote les parties terminales du message comme IMAP (« 1 », « 1.2 », « 2.1.3 »), dans l'ordre où
// mail.Reader les rend.
func leafPaths(raw []byte) []string {
	e, err := message.Read(bytes.NewReader(raw))
	if err != nil && !message.IsUnknownCharset(err) {
		return nil
	}
	var out []string
	var walk func(*message.Entity, string)
	walk = func(e *message.Entity, path string) {
		mr := e.MultipartReader()
		if mr == nil {
			if path == "" {
				path = "1"
			}
			out = append(out, path)
			return
		}
		defer mr.Close()
		for n := 1; ; n++ {
			child, err := mr.NextPart()
			if err != nil && !message.IsUnknownCharset(err) {
				return
			}
			p := strconv.Itoa(n)
			if path != "" {
				p = path + "." + p
			}
			walk(child, p)
		}
	}
	walk(e, "")
	return out
}

func (m *Message) addAttachment(name, ct string, r io.Reader) {
	b, trunc, _ := readLimited(r, maxAttachment)
	if trunc {
		m.Truncated = true
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	m.Attachments = append(m.Attachments, Attachment{Index: len(m.Attachments), Name: name, ContentType: ct, Size: len(b), data: b})
}

// Attachment renvoie le contenu de la pièce jointe d'index i.
func (m *Message) Attachment(i int) (Attachment, []byte, bool) {
	if i < 0 || i >= len(m.Attachments) {
		return Attachment{}, nil, false
	}
	return m.Attachments[i], m.Attachments[i].data, true
}

var (
	dkimResultRe = regexp.MustCompile(`(?i)\bdkim\s*=\s*([a-z]+)`)
	dkimDomainRe = regexp.MustCompile(`(?i)header\.(?:d|i)\s*=\s*"?@?([^\s;"]+)`)
)

// dkimStatus déduit l'état de la signature DKIM de l'en-tête Authentication-Results ajouté par le serveur de réception
// (la signature n'est pas revérifiée ici). « pass » seulement si le domaine signataire correspond à celui de l'expéditeur.
func dkimStatus(results []string, from []Address) string {
	if len(from) != 1 {
		return ""
	}
	at := strings.LastIndexByte(from[0].Email, '@')
	if at < 0 {
		return ""
	}
	fromDomain := strings.ToLower(from[0].Email[at+1:])
	status := ""
	for _, r := range results {
		for _, seg := range strings.Split(r, ";") {
			m := dkimResultRe.FindStringSubmatch(seg)
			if m == nil {
				continue
			}
			if strings.EqualFold(m[1], "pass") {
				d := dkimDomainRe.FindStringSubmatch(seg)
				if d == nil {
					if status == "" {
						status = "warn"
					}
					continue
				}
				dom := strings.ToLower(d[1])
				if i := strings.LastIndexByte(dom, '@'); i >= 0 {
					dom = dom[i+1:]
				}
				if dom == fromDomain || strings.HasSuffix(fromDomain, "."+dom) || strings.HasSuffix(dom, "."+fromDomain) {
					return "pass"
				}
			}
			status = "warn"
		}
	}
	return status
}

// isSignaturePart reconnaît les parties de signature numérique, qu'on ne liste pas comme pièces jointes.
func isSignaturePart(ct string) bool {
	switch ct {
	case "application/pkcs7-signature", "application/x-pkcs7-signature", "application/pgp-signature":
		return true
	}
	return false
}
