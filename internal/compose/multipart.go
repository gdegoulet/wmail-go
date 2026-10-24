package compose

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"strings"

	gomail "github.com/emersion/go-message/mail"
	"github.com/emersion/go-message/textproto"
)

func boundary() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "=_wmail_" + hex.EncodeToString(b)
}

// writeBase64 écrit des données en base64 sur des lignes de 76 caractères.
func writeBase64(w io.Writer, data []byte) {
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		_, _ = io.WriteString(w, enc[:76]+"\r\n")
		enc = enc[76:]
	}
	_, _ = io.WriteString(w, enc+"\r\n")
}

func writeQP(w io.Writer, text string) {
	qw := quotedprintable.NewWriter(w)
	_, _ = io.WriteString(qw, text)
	_ = qw.Close()
	_, _ = io.WriteString(w, "\r\n")
}

type partHeader [][2]string

func writePartHeader(w io.Writer, h partHeader) {
	for _, kv := range h {
		_, _ = fmt.Fprintf(w, "%s: %s\r\n", kv[0], kv[1])
	}
	_, _ = io.WriteString(w, "\r\n")
}

func multipartType(sub, bnd string) string {
	return mime.FormatMediaType("multipart/"+sub, map[string]string{"boundary": bnd})
}

// buildHTML assemble un message avec corps HTML :
//
//	multipart/mixed (si pièces jointes)
//	  multipart/alternative
//	    text/plain
//	    multipart/related (si images en ligne) ou text/html
//	      text/html
//	      image/* (Content-ID)
//	  pièces jointes
func (m *Message) buildHTML(h *gomail.Header, text string) ([]byte, error) {
	htmlCRLF := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(m.HTML, "\r\n", "\n"), "\r", "\n"), "\n", "\r\n")
	var body bytes.Buffer
	altB, relB, mixB := boundary(), boundary(), boundary()
	hasAtt := len(m.Attachments) > 0

	// partie alternative
	var alt bytes.Buffer
	fmt.Fprintf(&alt, "--%s\r\n", altB)
	writePartHeader(&alt, partHeader{{"Content-Type", "text/plain; charset=utf-8"}, {"Content-Transfer-Encoding", "quoted-printable"}})
	writeQP(&alt, text)
	fmt.Fprintf(&alt, "--%s\r\n", altB)
	if len(m.Images) > 0 {
		writePartHeader(&alt, partHeader{{"Content-Type", multipartType("related", relB)}})
		fmt.Fprintf(&alt, "--%s\r\n", relB)
		writePartHeader(&alt, partHeader{{"Content-Type", "text/html; charset=utf-8"}, {"Content-Transfer-Encoding", "quoted-printable"}})
		writeQP(&alt, htmlCRLF)
		for i, img := range m.Images {
			fmt.Fprintf(&alt, "--%s\r\n", relB)
			ct := img.ContentType
			writePartHeader(&alt, partHeader{
				{"Content-Type", ct + `; name="image` + fmt.Sprint(i+1) + extFor(ct) + `"`},
				{"Content-Transfer-Encoding", "base64"},
				{"Content-Disposition", "inline"},
				{"Content-ID", "<" + img.CID + ">"},
			})
			writeBase64(&alt, img.Data)
		}
		fmt.Fprintf(&alt, "--%s--\r\n", relB)
	} else {
		writePartHeader(&alt, partHeader{{"Content-Type", "text/html; charset=utf-8"}, {"Content-Transfer-Encoding", "quoted-printable"}})
		writeQP(&alt, htmlCRLF)
	}
	fmt.Fprintf(&alt, "--%s--\r\n", altB)

	if !hasAtt {
		h.Set("Content-Type", multipartType("alternative", altB))
		return assemble(h, alt.Bytes()), nil
	}
	h.Set("Content-Type", multipartType("mixed", mixB))
	fmt.Fprintf(&body, "--%s\r\n", mixB)
	writePartHeader(&body, partHeader{{"Content-Type", multipartType("alternative", altB)}})
	body.Write(altPartBody(alt.Bytes()))
	for _, a := range m.Attachments {
		ct := a.ContentType
		if mt, _, err := mime.ParseMediaType(ct); err == nil && !strings.ContainsAny(ct, "\r\n") {
			ct = mt
		} else {
			ct = "application/octet-stream"
		}
		name := safeFilename(a.Name)
		fmt.Fprintf(&body, "--%s\r\n", mixB)
		writePartHeader(&body, partHeader{
			{"Content-Type", mime.FormatMediaType(ct, map[string]string{"name": name})},
			{"Content-Transfer-Encoding", "base64"},
			{"Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name})},
		})
		writeBase64(&body, a.Data)
	}
	fmt.Fprintf(&body, "--%s--\r\n", mixB)
	return assemble(h, body.Bytes()), nil
}

// altPartBody renvoie le contenu de la partie alternative (déjà délimitée) tel quel.
func altPartBody(b []byte) []byte { return b }

func assemble(h *gomail.Header, body []byte) []byte {
	var out bytes.Buffer
	_ = textproto.WriteHeader(&out, h.Header.Header)
	out.Write(body)
	return out.Bytes()
}

func extFor(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}
