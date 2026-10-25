package compose

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/gdegoulet/wmail-go/internal/mailview"
)

// InlineImage est une image incorporée au corps HTML (référencée par cid:).
type InlineImage struct {
	CID         string // sans chevrons
	ContentType string
	Data        []byte
}

const (
	maxInlineImage = 4 << 20 // une image en ligne
	maxInlineTotal = 12 << 20
	maxInlineCount = 40
)

var dataURLRe = regexp.MustCompile(`(?is)^data:(image/(?:png|jpeg|gif|webp));base64,([a-z0-9+/=\s]+)$`)

// PrepareHTML assainit le HTML saisi dans l'éditeur et en extrait les images :
//   - scripts, formulaires, cadres, gestionnaires d'événements et styles dangereux sont retirés (même liste blanche
//     que l'affichage des messages) ;
//   - les images distantes sont retirées (un message ne doit pas déclencher de chargement chez le destinataire à
//     l'insu de l'auteur, et les citations ne doivent pas recopier de pisteurs) ;
//   - les images cid: sont résolues avec inline (URI data:) quand elles viennent d'un message cité ;
//   - les images data: deviennent des parties incorporées (cid:), bornées en taille et en nombre.
//
// Le résultat est un document HTML complet, prêt à être envoyé.
func PrepareHTML(src string, inline map[string]string) (string, []InlineImage) {
	clean, _ := mailview.Sanitize(src, mailview.SanitizeOptions{BlockRemote: true, InlineImages: inline})
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(clean), ctx)
	if err != nil {
		return wrapHTML(html.EscapeString(src)), nil
	}
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	var (
		images []InlineImage
		total  int
		byData = map[string]string{} // même image répétée : une seule partie
		drop   []*xhtml.Node
		walk   func(*xhtml.Node)
	)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "img" {
			src := ""
			for _, a := range n.Attr {
				if a.Key == "src" {
					src = strings.TrimSpace(a.Val)
				}
			}
			m := dataURLRe.FindStringSubmatch(src)
			var raw []byte
			if m != nil {
				raw, _ = base64.StdEncoding.DecodeString(strings.Join(strings.Fields(m[2]), ""))
			}
			switch {
			case m == nil || len(raw) == 0 || len(raw) > maxInlineImage:
				drop = append(drop, n)
			default:
				cid, seen := byData[src]
				if !seen {
					if len(images) >= maxInlineCount || total+len(raw) > maxInlineTotal {
						drop = append(drop, n)
						break
					}
					cid = randomCID()
					byData[src] = cid
					total += len(raw)
					images = append(images, InlineImage{CID: cid, ContentType: strings.ToLower(m[1]), Data: raw})
				}
				n.Attr = replaceAttr(n.Attr, "src", "cid:"+cid)
				n.Attr = removeAttrKey(n.Attr, "data-blocked")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	for _, n := range drop {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		_ = xhtml.Render(&buf, c)
	}
	return wrapHTML(buf.String()), images
}

func replaceAttr(attrs []xhtml.Attribute, key, val string) []xhtml.Attribute {
	for i := range attrs {
		if attrs[i].Key == key {
			attrs[i].Val = val
			return attrs
		}
	}
	return append(attrs, xhtml.Attribute{Key: key, Val: val})
}

func removeAttrKey(attrs []xhtml.Attribute, key string) []xhtml.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		if a.Key != key {
			out = append(out, a)
		}
	}
	return out
}

func randomCID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + "@wmail-go"
}

func wrapHTML(body string) string {
	return "<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"></head><body>" + body + "</body></html>\n"
}

// HTMLToPlain produit la partie texte d'un message HTML (alternative lisible sans HTML).
func HTMLToPlain(h string) string {
	return strings.ReplaceAll(mailview.HTMLToText(h), "\u00a0", " ")
}

// TextToHTMLBody convertit du texte brut en fragment HTML (échappé, retours à la ligne conservés), pour passer du
// texte brut à l'éditeur HTML.
func TextToHTMLBody(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	var b strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			b.WriteString("<br>")
		}
		b.WriteString(html.EscapeString(line))
	}
	return b.String()
}

// QuoteHTML met un fragment HTML en citation, avec le même style de bloc que la plupart des clients.
func QuoteHTML(fragment string) string {
	return `<blockquote style="margin:0 0 0 .8ex;border-left:2px solid #ccc;padding-left:1ex">` + fragment + `</blockquote>`
}

// Escape échappe du texte pour l'insérer dans du HTML.
func Escape(s string) string { return html.EscapeString(s) }

var _ = fmt.Sprintf
