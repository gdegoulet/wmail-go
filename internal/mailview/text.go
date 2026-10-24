package mailview

import (
	"html"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

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

// balanced retire les ponctuations finales d'une adresse, mais garde une « ) » qui ferme une « ( » de l'adresse.
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

// HTMLToText extrait le texte visible d'un document HTML (pour la vue « texte »).
func HTMLToText(src string) string {
	z := xhtml.NewTokenizer(strings.NewReader(src))
	var sb strings.Builder
	skip := 0
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return strings.TrimSpace(collapseBlank(sb.String()))
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "title":
				skip++
			case "br", "p", "div", "tr", "li", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote":
				sb.WriteByte('\n')
			}
		case xhtml.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "title":
				if skip > 0 {
					skip--
				}
			case "p", "div", "tr", "li", "table":
				sb.WriteByte('\n')
			}
		case xhtml.TextToken:
			if skip == 0 {
				sb.Write(z.Text())
			}
		}
	}
}

var blankRe = regexp.MustCompile(`\n{3,}`)

func collapseBlank(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return blankRe.ReplaceAllString(s, "\n\n")
}
