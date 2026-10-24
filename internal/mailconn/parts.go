package mailconn

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/quotedprintable"
	"strings"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/mailview"
)

// Structure lit la structure MIME d'un message (BODYSTRUCTURE) sans télécharger son contenu : de quoi lister les pièces
// jointes et n'en charger qu'une, même pour un message de plusieurs dizaines de mégaoctets.
func (c *Conn) Structure(folder string, uid uint32) (_ []mailview.PartDesc, err error) {
	defer c.trace("structure", false, "dossier", folder, "uid", uid)(&err)
	if _, err := c.Select(folder, true); err != nil {
		return nil, err
	}
	msgs, err := c.Client.Fetch(uidSet([]uint32{uid}), &imap.FetchOptions{UID: true, BodyStructure: &imap.FetchItemBodyStructure{Extended: true}}).Collect()
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 || msgs[0].BodyStructure == nil {
		return nil, ErrNotFound
	}
	return describeParts(msgs[0].BodyStructure), nil
}

// describeParts aplatit la structure en parties terminales, dans l'ordre du message.
func describeParts(bs imap.BodyStructure) []mailview.PartDesc {
	var out []mailview.PartDesc
	bs.Walk(func(path []int, part imap.BodyStructure) bool {
		sp, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		d := mailview.PartDesc{Path: append([]int(nil), path...), Type: strings.ToLower(sp.Type + "/" + sp.Subtype),
			Filename: sp.Filename(), ContentID: sp.ID, Encoding: strings.ToLower(sp.Encoding), Size: sp.Size}
		if disp := sp.Disposition(); disp != nil {
			d.Disposition = strings.ToLower(disp.Value)
		}
		if len(d.Path) == 0 {
			d.Path = []int{1}
		}
		out = append(out, d)
		return true
	})
	return out
}

// FetchPart télécharge une seule partie du message et la décode (base64, quoted-printable). maxSize borne la taille
// acceptée ; au-delà, l'erreur ErrTooLarge est renvoyée sans rien télécharger.
func (c *Conn) FetchPart(folder string, uid uint32, path []int, encoding string, maxSize int64) (_ []byte, err error) {
	defer c.trace("partie", false, "dossier", folder, "uid", uid, "partie", len(path))(&err)
	if _, err := c.Select(folder, true); err != nil {
		return nil, err
	}
	msgs, err := c.Client.Fetch(uidSet([]uint32{uid}), &imap.FetchOptions{
		UID: true, BodySection: []*imap.FetchItemBodySection{{Part: path, Peek: true}},
	}).Collect()
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 || len(msgs[0].BodySection) == 0 {
		return nil, ErrNotFound
	}
	raw := msgs[0].BodySection[0].Bytes
	if maxSize > 0 && int64(len(raw)) > maxSize {
		return nil, fmt.Errorf("%w (%d octets)", ErrTooLarge, len(raw))
	}
	var r io.Reader = strings.NewReader(string(raw))
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, newSkipSpace(r))
	case "quoted-printable":
		r = quotedprintable.NewReader(r)
	}
	out, err := io.ReadAll(r)
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}

// newSkipSpace retire les fins de ligne d'un flux base64 (le décodeur standard les refuse).
func newSkipSpace(r io.Reader) io.Reader {
	return &spaceSkipper{r: r}
}

type spaceSkipper struct{ r io.Reader }

func (s *spaceSkipper) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	out := 0
	for i := 0; i < n; i++ {
		switch p[i] {
		case '\r', '\n', ' ', '\t':
		default:
			p[out] = p[i]
			out++
		}
	}
	if out == 0 && err == nil {
		return s.Read(p)
	}
	return out, err
}
