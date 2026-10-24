package mailview

import (
	"bytes"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// SanitizeOptions règle l'assainissement du HTML.
type SanitizeOptions struct {
	// BlockRemote retire les images (et tout contenu) chargées depuis un serveur distant.
	BlockRemote bool
	// InlineImages associe un Content-ID normalisé à une URI data: (voir Message.InlineImages).
	InlineImages map[string]string
	// ProxyURL, s'il est défini, remplace l'adresse des images distantes autorisées (chargement par le serveur).
	ProxyURL func(remote string) string
	// MailtoURL, s'il est défini, remplace un lien « mailto: » par une adresse de l'application (fenêtre de composition).
	MailtoURL func(mailto string) string
}

// Report résume ce que l'assainissement a trouvé.
type Report struct {
	RemoteFound   int // références à du contenu distant dans le message
	RemoteBlocked int // références retirées parce que le blocage est actif
	Trackers      int // pixels espions retirés (même quand le contenu distant est autorisé)
}

var (
	numericRe = regexp.MustCompile(`^[0-9]{1,4}(%|px)?$`)
	colorRe   = regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|[a-zA-Z]{3,20})$`)
	dataImgRe = regexp.MustCompile(`(?i)^data:image/(png|jpeg|gif|webp);base64,[a-z0-9+/=\s]+$`)
)

// Propriétés CSS autorisées dans l'attribut style. Aucune ne peut charger une ressource :
// les valeurs contenant url(), @import, expression, etc. sont refusées par safeCSSValue.
var allowedStyles = []string{
	"color", "background-color", "font-family", "font-size", "font-style", "font-weight", "text-align",
	"text-decoration", "text-transform", "text-indent", "line-height", "letter-spacing", "word-spacing",
	"margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
	"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
	"border", "border-top", "border-right", "border-bottom", "border-left", "border-color", "border-style", "border-width",
	"border-collapse", "border-spacing", "width", "height", "max-width", "min-width", "max-height", "min-height",
	"vertical-align", "white-space", "direction", "list-style-type", "table-layout",
	// mise en page et finitions courantes des courriels modernes (invitations, newsletters) ; aucune ne positionne en
	// dehors du flux (pas de position, z-index, top/left : un message ne recouvre pas l'interface)
	"display", "float", "clear", "font", "background", "box-sizing", "overflow", "overflow-x", "overflow-y", "opacity",
	"border-radius", "border-top-left-radius", "border-top-right-radius", "border-bottom-left-radius", "border-bottom-right-radius",
	"border-top-color", "border-right-color", "border-bottom-color", "border-left-color",
	"border-top-style", "border-right-style", "border-bottom-style", "border-left-style",
	"border-top-width", "border-right-width", "border-bottom-width", "border-left-width",
	"box-shadow", "text-shadow", "text-overflow", "word-break", "overflow-wrap", "word-wrap", "list-style", "list-style-position",
	"empty-cells", "caption-side", "font-variant", "font-stretch", "text-decoration-line", "text-decoration-color", "text-decoration-style",
	"padding-inline", "margin-inline", "padding-block", "margin-block", "max-inline-size", "min-inline-size",
}

var forbiddenCSS = []string{"url(", "expression", "javascript", "@import", "behavior", "binding", "\\", "/*", "<", ">", "&#", "var(", "attr("}

func safeCSSValue(v string) bool {
	if len(v) > 200 {
		return false
	}
	l := strings.ToLower(v)
	for _, bad := range forbiddenCSS {
		if strings.Contains(l, bad) {
			return false
		}
	}
	return true
}

var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("a", "abbr", "b", "big", "blockquote", "br", "caption", "center", "cite", "code", "col", "colgroup",
		"dd", "del", "dfn", "div", "dl", "dt", "em", "font", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "img", "ins",
		"kbd", "li", "mark", "ol", "p", "pre", "q", "s", "samp", "small", "span", "strike", "strong", "sub", "sup",
		"table", "tbody", "td", "tfoot", "th", "thead", "tr", "tt", "u", "ul", "var", "wbr")
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	p.AllowAttrs("width", "height").Matching(numericRe).OnElements("img", "table", "td", "th", "col", "colgroup")
	p.AllowAttrs("colspan", "rowspan").Matching(numericRe).OnElements("td", "th")
	p.AllowAttrs("border", "cellpadding", "cellspacing").Matching(numericRe).OnElements("table")
	p.AllowAttrs("bgcolor").Matching(colorRe).OnElements("table", "tr", "td", "th")
	p.AllowAttrs("color").Matching(colorRe).OnElements("font")
	p.AllowAttrs("face", "size").OnElements("font")
	p.AllowAttrs("align", "valign").Matching(regexp.MustCompile(`^[a-zA-Z]{3,10}$`)).Globally()
	p.AllowAttrs("dir").Matching(regexp.MustCompile(`^(ltr|rtl|auto)$`)).Globally()
	p.AllowAttrs("lang", "title").Globally()
	p.AllowStyles(allowedStyles...).MatchingHandler(safeCSSValue).Globally()
	p.AllowURLSchemes("http", "https", "mailto", "tel", "cid", "data")
	p.RequireParseableURLs(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.SkipElementsContent("script", "style", "head", "title", "iframe", "frame", "frameset", "object", "embed",
		"applet", "noscript", "template", "svg", "math", "form", "select", "textarea", "button", "audio", "video")
	return p
}()

func attr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func removeAttr(n *xhtml.Node, key string) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key != key {
			out = append(out, a)
		}
	}
	n.Attr = out
}

func setAttr(n *xhtml.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, xhtml.Attribute{Key: key, Val: val})
}

// Sanitize assainit du HTML de message. La sortie ne contient ni script, ni formulaire, ni cadre,
// ni appel de ressource externe si BlockRemote est vrai ; les images cid: deviennent des URI data:.
func Sanitize(src string, opts SanitizeOptions) (string, Report) {
	var rep Report
	clean := policy.Sanitize(src)

	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(clean), ctx)
	if err != nil {
		return html.EscapeString(src), rep // dernier recours : tout est du texte
	}
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "a":
				href, ok := attr(n, "href")
				switch {
				case ok && !safeLink(href):
					removeAttr(n, "href")
				case ok && opts.MailtoURL != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(href)), "mailto:"):
					// Écrire à cette adresse ouvre la composition de wmail-go, pas le logiciel de messagerie du poste.
					setAttr(n, "href", opts.MailtoURL(strings.TrimSpace(href)))
					setAttr(n, "target", "_top")
					removeAttr(n, "rel")
				}
			case "img":
				rewriteImage(n, opts, &rep)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	autoLink(root, opts)

	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		_ = xhtml.Render(&buf, c)
	}
	return buf.String(), rep
}

func safeLink(href string) bool {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto", "tel":
		return true
	}
	return false
}

func rewriteImage(n *xhtml.Node, opts SanitizeOptions, rep *Report) {
	src, ok := attr(n, "src")
	if !ok {
		return
	}
	src = strings.TrimSpace(src)
	lower := strings.ToLower(src)
	// Image jointe désignée par Content-Location : on la remplace avant tout traitement d'URL distante.
	if len(opts.InlineImages) > 0 && !strings.HasPrefix(lower, "cid:") && !strings.HasPrefix(lower, "data:") {
		for _, k := range LocationKeys(src) {
			if data, ok := opts.InlineImages[k]; ok {
				setAttr(n, "src", data)
				return
			}
		}
	}
	switch {
	case strings.HasPrefix(lower, "cid:"):
		if data, ok := opts.InlineImages[NormalizeCID(src)]; ok {
			setAttr(n, "src", data)
			return
		}
		removeAttr(n, "src")
	case strings.HasPrefix(lower, "data:"):
		if !dataImgRe.MatchString(src) || len(src) > 4<<20 {
			removeAttr(n, "src")
		}
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		if isTracker(n, lower) {
			rep.Trackers++
			removeAttr(n, "src")
			setAttr(n, "data-tracker", "1")
			return
		}
		rep.RemoteFound++
		if opts.BlockRemote {
			rep.RemoteBlocked++
			removeAttr(n, "src")
			setAttr(n, "data-blocked", "1")
		} else if opts.ProxyURL != nil {
			setAttr(n, "src", opts.ProxyURL(src))
		}
	default: // relatif, protocole relatif, autre schéma
		removeAttr(n, "src")
	}
}

// trackerHosts : services d'ouverture de courriel connus, dont les images ne sont jamais chargées.
var trackerHosts = []string{"email.microsoftemail.com/open", "github.com/notifications/beacon/", "mandrillapp.com/track/open", "list-manage.com/track/open", "open.convertkit-mail.com", "pixel.mailchimp", "track.hubspot.com/__ptq.gif", "mailstat.us/tr/"}

// isTracker repère une image espion : une taille de 0 ou 1 pixel, ou une adresse de suivi connue.
func isTracker(n *xhtml.Node, lowerSrc string) bool {
	for _, h := range trackerHosts {
		if strings.Contains(lowerSrc, h) {
			return true
		}
	}
	tiny := func(v string) bool {
		v = strings.TrimSuffix(strings.TrimSpace(strings.ToLower(v)), "px")
		n, err := strconv.Atoi(v)
		return err == nil && n <= 1
	}
	w, hasW := attr(n, "width")
	h, hasH := attr(n, "height")
	return hasW && hasH && tiny(w) && tiny(h)
}

// CollapseQuotes replie les longues citations (blockquote de premier niveau) dans un élément details : un clic les
// déplie, sans JavaScript. src doit déjà être assaini.
func CollapseQuotes(src string) string {
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(src), ctx)
	if err != nil {
		return src
	}
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	var textLen func(*xhtml.Node) int
	textLen = func(n *xhtml.Node) int {
		if n.Type == xhtml.TextNode {
			return len(strings.TrimSpace(n.Data))
		}
		t := 0
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			t += textLen(c)
		}
		return t
	}
	changed := false
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == xhtml.ElementNode && c.Data == "blockquote" {
				if textLen(c) >= 400 {
					det := &xhtml.Node{Type: xhtml.ElementNode, Data: "details", DataAtom: atom.Details, Attr: []xhtml.Attribute{{Key: "class", Val: "wm-quote"}}}
					sum := &xhtml.Node{Type: xhtml.ElementNode, Data: "summary", DataAtom: atom.Summary}
					sum.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: "…"})
					det.AppendChild(sum)
					n.InsertBefore(det, c)
					n.RemoveChild(c)
					det.AppendChild(c)
					changed = true
				}
			} else {
				walk(c)
			}
			c = next
		}
	}
	walk(root)
	if !changed {
		return src
	}
	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		_ = xhtml.Render(&buf, c)
	}
	return buf.String()
}

// autoLinkRe repère les adresses web et de courrier dans le texte d'un message.
var autoLinkRe = regexp.MustCompile(`(?i)\b((?:https?|ftp)://[^\s<>"']+|www\.[^\s<>"']+\.[^\s<>"']+)|([A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+)`)

var noAutoLink = map[string]bool{"a": true, "code": true, "pre": true, "kbd": true, "samp": true, "textarea": true, "style": true, "script": true}

// autoLink transforme en liens les adresses web et de courrier écrites en toutes lettres dans le texte, comme RainLoop.
func autoLink(root *xhtml.Node, opts SanitizeOptions) {
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			switch {
			case c.Type == xhtml.ElementNode && noAutoLink[c.Data]:
			case c.Type == xhtml.ElementNode:
				walk(c)
			case c.Type == xhtml.TextNode:
				linkTextNode(n, c, opts)
			}
			c = next
		}
	}
	walk(root)
}

func linkTextNode(parent, node *xhtml.Node, opts SanitizeOptions) {
	locs := autoLinkRe.FindAllStringSubmatchIndex(node.Data, -1)
	if len(locs) == 0 {
		return
	}
	text, last := node.Data, 0
	var frag []*xhtml.Node
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		u := trimURL(text[start:end])
		if u == "" {
			continue
		}
		end = start + len(u)
		href := u
		switch {
		case loc[4] >= 0: // adresse de courrier
			href = "mailto:" + u
			if opts.MailtoURL != nil {
				href = opts.MailtoURL(href)
			}
		case strings.HasPrefix(strings.ToLower(u), "www."):
			href = "http://" + u
		}
		if start > last {
			frag = append(frag, &xhtml.Node{Type: xhtml.TextNode, Data: text[last:start]})
		}
		a := &xhtml.Node{Type: xhtml.ElementNode, Data: "a", DataAtom: atom.A, Attr: []xhtml.Attribute{{Key: "href", Val: href}}}
		if loc[4] >= 0 && opts.MailtoURL != nil {
			a.Attr = append(a.Attr, xhtml.Attribute{Key: "target", Val: "_top"})
		} else {
			a.Attr = append(a.Attr, xhtml.Attribute{Key: "target", Val: "_blank"}, xhtml.Attribute{Key: "rel", Val: "nofollow noreferrer noopener"})
		}
		a.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: u})
		frag = append(frag, a)
		last = end
	}
	if len(frag) == 0 {
		return
	}
	if last < len(text) {
		frag = append(frag, &xhtml.Node{Type: xhtml.TextNode, Data: text[last:]})
	}
	for _, f := range frag {
		parent.InsertBefore(f, node)
	}
	parent.RemoveChild(node)
}
