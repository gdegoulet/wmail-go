package mailview

import (
	"html"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

// textWidth est la largeur à laquelle HTMLToText coupe les lignes, comme RainLoop.
const textWidth = 100

var urlRe = regexp.MustCompile(`(?i)\b((?:https?|ftp)://[^\s<>"']+|www\.[^\s<>"']+\.[^\s<>"']+)|([A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+)`)

// quoteDepth compte les « > » en tête de ligne (les espaces entre eux sont permis) et renvoie la ligne sans ces marques.
func quoteDepth(line string) (int, string) {
	depth, i := 0, 0
	for i < len(line) {
		switch line[i] {
		case '>':
			depth++
			i++
		case ' ':
			if i+1 < len(line) && (line[i+1] == '>' || line[i+1] == ' ') && depth > 0 {
				i++
				continue
			}
			if depth > 0 && i+1 <= len(line) {
				return depth, line[i+1:]
			}
			return depth, line[i:]
		default:
			return depth, line[i:]
		}
	}
	return depth, ""
}

// TextToHTML transforme du texte brut en HTML sûr : tout est échappé, la mise en page (espaces, retours à la ligne) est
// conservée par la feuille de style (« pre-wrap »), les adresses web et e-mail deviennent des liens, les lignes citées
// (« > », « >> »…) sont rendues en citations imbriquées.
func TextToHTML(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	var out strings.Builder
	out.WriteString(`<div class="plain">`)
	depth := 0
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for n, line := range lines {
		d, body := quoteDepth(line)
		for depth < d {
			out.WriteString("<blockquote>")
			depth++
		}
		for depth > d {
			out.WriteString("</blockquote>")
			depth--
		}
		out.WriteString(linkify(body))
		if n < len(lines)-1 && !(depth > 0 && quoteDepthOf(lines[n+1]) < depth) {
			out.WriteByte('\n')
		}
	}
	for ; depth > 0; depth-- {
		out.WriteString("</blockquote>")
	}
	out.WriteString("</div>")
	return out.String()
}

func quoteDepthOf(line string) int { d, _ := quoteDepth(line); return d }

// trimURL retire les ponctuations finales d'une adresse détectée dans du texte (« voir http://x/y. »), mais garde
// une parenthèse ou un crochet fermants qui équilibrent un ouvrant de l'adresse elle-même.
func trimURL(u string) string {
	for len(u) > 0 {
		c := u[len(u)-1]
		switch {
		case strings.IndexByte(".,;:!?'\"", c) >= 0:
			u = u[:len(u)-1]
		case c == ')' && strings.Count(u, "(") < strings.Count(u, ")"):
			u = u[:len(u)-1]
		case c == ']' && strings.Count(u, "[") < strings.Count(u, "]"):
			u = u[:len(u)-1]
		default:
			return u
		}
	}
	return u
}

func linkify(line string) string {
	var sb strings.Builder
	last := 0
	for _, loc := range urlRe.FindAllStringSubmatchIndex(line, -1) {
		start, end := loc[0], loc[1]
		u := trimURL(line[start:end])
		if u == "" {
			continue
		}
		end = start + len(u)
		href := u
		switch {
		case loc[4] >= 0: // adresse e-mail
			href = "mailto:" + u
		case strings.HasPrefix(strings.ToLower(u), "www."):
			href = "http://" + u
		}
		sb.WriteString(html.EscapeString(line[last:start]))
		sb.WriteString(`<a href="` + html.EscapeString(href) + `" target="_blank" rel="noopener noreferrer nofollow">` + html.EscapeString(u) + `</a>`)
		last = end
	}
	sb.WriteString(html.EscapeString(line[last:]))
	return sb.String()
}

// Unflow défait le formatage « format=flowed » (RFC 3676) des messages de Thunderbird, Apple Mail, K-9… : une ligne qui
// se termine par une espace se prolonge sur la suivante (même niveau de citation). delsp retire cette espace.
func Unflow(text string, delsp bool) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		depth, body := quoteDepth(lines[i])
		body = strings.TrimPrefix(body, " ") // « space-stuffing »
		prefix := strings.Repeat(">", depth)
		if depth > 0 {
			prefix += " "
		}
		cur := body
		for body != "-- " && strings.HasSuffix(cur, " ") && i+1 < len(lines) {
			nd, nb := quoteDepth(lines[i+1])
			if nd != depth {
				break
			}
			if delsp {
				cur = cur[:len(cur)-1]
			}
			cur += strings.TrimPrefix(nb, " ")
			i++
		}
		out = append(out, prefix+cur)
	}
	return strings.Join(out, "\n")
}

// HTMLToText extrait le texte visible d'un document HTML (vue « texte », partie texte d'un envoi HTML, citation d'une
// réponse en texte brut). Comme RainLoop : les citations sont préfixées par « > », les éléments de liste par « * »,
// les liens gardent leur adresse entre parenthèses, les cellules d'un tableau sont séparées, et les lignes sont
// coupées à 100 colonnes.
func HTMLToText(src string) string {
	z := xhtml.NewTokenizer(strings.NewReader(src))
	var sb strings.Builder
	skip, quote := 0, 0
	var href string
	var linkText strings.Builder
	inLink := false
	nl := func(n int) {
		cur := sb.String()
		have := len(cur) - len(strings.TrimRight(cur, "\n"))
		for ; have < n; have++ {
			sb.WriteByte('\n')
		}
	}
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return wrapText(trimLineEnds(strings.TrimSpace(collapseBlank(sb.String()))), textWidth)
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)
			switch tag {
			case "script", "style", "head", "title":
				skip++
			case "blockquote":
				quote++
				nl(2)
				sb.WriteString(strings.Repeat(">", quote) + " ")
			case "li":
				nl(1)
				sb.WriteString(strings.Repeat(">", quote))
				if quote > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString("* ")
			case "hr":
				nl(1)
				sb.WriteString("_______________________________")
				nl(1)
			case "br":
				nl(1)
				if quote > 0 {
					sb.WriteString(strings.Repeat(">", quote) + " ")
				}
			case "td", "th":
				if strings.TrimRight(sb.String(), " ") != "" && !strings.HasSuffix(sb.String(), "\n") {
					sb.WriteString("  ")
				}
			case "a":
				href, inLink = "", true
				linkText.Reset()
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					if string(k) == "href" {
						href = strings.TrimSpace(string(v))
					}
				}
			case "p", "div", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "table":
				nl(2)
				if quote > 0 {
					sb.WriteString(strings.Repeat(">", quote) + " ")
				}
			}
		case xhtml.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "title":
				if skip > 0 {
					skip--
				}
			case "blockquote":
				if quote > 0 {
					quote--
				}
				nl(2)
			case "a":
				t := strings.TrimSpace(linkText.String())
				switch {
				case !inLink:
				case href == "" || strings.EqualFold(href, t) || strings.HasPrefix(strings.ToLower(href), "mailto:"+strings.ToLower(t)):
					sb.WriteString(t)
				case t == "":
					sb.WriteString(href)
				default:
					sb.WriteString(t + " (" + href + ")")
				}
				inLink = false
			case "p", "div", "tr", "li", "table", "h1", "h2", "h3", "h4", "h5", "h6":
				nl(1)
			}
		case xhtml.TextToken:
			if skip > 0 {
				continue
			}
			txt := spaceRe.ReplaceAllString(string(z.Text()), " ")
			if inLink {
				linkText.WriteString(txt)
				continue
			}
			sb.WriteString(txt)
		}
	}
}

var spaceRe = regexp.MustCompile(`[ \t\r\n]+`)

// trimLineEnds retire les espaces de fin de chaque ligne. Elles viennent des espaces insérés entre deux balises
// par le tokenizer ; laissées en place, elles feraient passer la ligne pour du « format=flowed » à la relecture.
func trimLineEnds(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// wrapText coupe les lignes trop longues aux espaces, en gardant le préfixe de citation.
func wrapText(s string, width int) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		prefix := ""
		if i := strings.IndexFunc(line, func(r rune) bool { return r != '>' && r != ' ' }); i > 0 && strings.HasPrefix(line, ">") {
			prefix = line[:i]
		}
		for len([]rune(line)) > width {
			cut := -1
			r := []rune(line)
			for i := width; i > len(prefix); i-- {
				if r[i] == ' ' {
					cut = i
					break
				}
			}
			if cut <= len(prefix) {
				break
			}
			out = append(out, strings.TrimRight(string(r[:cut]), " "))
			line = prefix + strings.TrimLeft(string(r[cut:]), " ")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

var blankRe = regexp.MustCompile(`\n{3,}`)

// collapseBlank ramène les suites de lignes vides à une seule et remplace les espaces insécables (U+00A0, très
// fréquentes dans le HTML des infolettres) par des espaces ordinaires : en texte brut elles ne se voient pas,
// mais elles empêcheraient wrapText de couper la ligne et seraient recopiées telles quelles dans une réponse.
func collapseBlank(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return blankRe.ReplaceAllString(s, "\n\n")
}
