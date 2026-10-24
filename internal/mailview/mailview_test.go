package mailview

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/url"
	"strings"
	"testing"
)

var attacks = map[string]string{
	"script":                `<script>alert(1)</script>bonjour`,
	"img onerror":           `<img src=x onerror=alert(1)>`,
	"href javascript":       `<a href="javascript:alert(1)">x</a>`,
	"href javascript maj":   `<a href="  JaVaScRiPt:alert(1)">x</a>`,
	"href data html":        `<a href="data:text/html;base64,PHNjcmlwdD4=">x</a>`,
	"svg onload":            `<svg onload=alert(1)><circle/></svg>`,
	"iframe":                `<iframe src="https://evil.example/"></iframe>`,
	"style import":          `<style>@import url(https://evil.example/x.css)</style><p>t</p>`,
	"style url":             `<div style="background:url(https://evil.example/x.png)">x</div>`,
	"style expression":      `<div style="width:expression(alert(1))">x</div>`,
	"style image":           `<p style="color:red;background-image:url('//evil.example')">x</p>`,
	"style escape":          `<p style="background:u\72l(https://evil.example/x)">x</p>`,
	"img protocole relatif": `<img src="//evil.example/p.gif">`,
	"img srcset":            `<img srcset="https://evil.example/a.png 1x" src="cid:inconnu">`,
	"table background":      `<table background="https://evil.example/b.png"><tr><td>x</td></tr></table>`,
	"form":                  `<form action="https://evil.example"><input name=p><button>ok</button></form>`,
	"meta refresh":          `<meta http-equiv="refresh" content="0;url=https://evil.example">`,
	"base":                  `<base href="https://evil.example/">`,
	"link css":              `<link rel=stylesheet href=https://evil.example/x.css>`,
	"object embed":          `<object data="x"></object><embed src="x">`,
	"math":                  `<math><mi xlink:href="javascript:alert(1)">x</mi></math>`,
	"onclick":               `<a href="https://ok.example/" onclick="alert(1)">x</a>`,
	"img data svg":          `<img src="data:image/svg+xml;base64,PHN2Zz4=">`,
	"video poster":          `<video src="https://evil.example/v.mp4" poster="https://evil.example/p.png"></video>`,
	"tags imbriqués":        `<<script>script>alert(1)<</script>/script>`,
	"noscript":              `<noscript><img src="https://evil.example/n.gif"></noscript>`,
	"template":              `<template><script>alert(1)</script></template>`,
	"mutation xss":          `<noscript><p title="</noscript><img src=x onerror=alert(1)>">`,
	"entités":               `<a href="&#106;avascript:alert(1)">x</a>`,
	"tabulation":            `<a href="java&#9;script:alert(1)">x</a>`,
	"img pixel":             `<img src="https://evil.example/track.gif?u=1" width=1 height=1>`,
}

var forbidden = []string{
	"<script", "onerror", "onclick", "onload", "javascript:", "<iframe", "<form", "<object", "<embed", "<svg",
	"<style", "<link", "<meta", "<base", "<video", "<math", "expression(", "url(", "@import", "srcset", "background=",
	"href=\"data:", "src=\"//", "src=\"http", "<input", "<button", "<noscript", "<template",
}

func TestSanitizeNeutralizesAttacks(t *testing.T) {
	for name, payload := range attacks {
		for _, block := range []bool{true, false} {
			out, _ := Sanitize(payload, SanitizeOptions{BlockRemote: block})
			l := strings.ToLower(out)
			for _, bad := range forbidden {
				if block == false && bad == "src=\"http" {
					continue // contenu distant autorisé par l'utilisateur : voir TestRemoteContent
				}
				if strings.Contains(l, bad) {
					t.Errorf("%s (blocage=%v) : %q présent dans %q", name, block, bad, out)
				}
			}
		}
	}
}

func TestSanitizeKeepsLegitimateContent(t *testing.T) {
	src := `<p style="color:#333333;font-size:14px;text-align:center">Bonjour <b>monde</b></p>` +
		`<table width="600" cellpadding="0"><tr><td bgcolor="#eeeeee" colspan="2">cellule</td></tr></table>` +
		`<a href="https://exemple.fr/page?x=1&y=2">lien</a> <a href="mailto:a@b.fr">mail</a>`
	out, _ := Sanitize(src, SanitizeOptions{BlockRemote: true})
	for _, want := range []string{"Bonjour", "<b>monde</b>", "color: #333333", "font-size: 14px", "text-align: center", `colspan="2"`,
		`bgcolor="#eeeeee"`, `href="https://exemple.fr/page?x=1&amp;y=2"`, `href="mailto:a@b.fr"`, "cellule"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q absent de %q", want, out)
		}
	}
	if !strings.Contains(out, `rel="`) || !strings.Contains(out, "noopener") || !strings.Contains(out, "noreferrer") || !strings.Contains(out, `target="_blank"`) {
		t.Errorf("liens externes sans rel/target sûrs : %q", out)
	}
}

func TestRemoteContent(t *testing.T) {
	src := `<img src="https://t.example/p.gif?id=42" alt="pixel"><img src="http://t.example/b.png">`
	out, rep := Sanitize(src, SanitizeOptions{BlockRemote: true})
	if strings.Contains(out, "t.example") || rep.RemoteFound != 2 || rep.RemoteBlocked != 2 {
		t.Errorf("blocage : %q %+v", out, rep)
	}
	if !strings.Contains(out, `alt="pixel"`) {
		t.Error("le texte alternatif doit rester")
	}
	out, rep = Sanitize(src, SanitizeOptions{BlockRemote: false})
	if !strings.Contains(out, `src="https://t.example/p.gif?id=42"`) || rep.RemoteFound != 2 || rep.RemoteBlocked != 0 {
		t.Errorf("contenu distant autorisé : %q %+v", out, rep)
	}
}

func TestInlineImages(t *testing.T) {
	png := "data:image/png;base64,iVBORw0KGgo="
	opts := SanitizeOptions{BlockRemote: true, InlineImages: map[string]string{"logo@x": png}}
	out, rep := Sanitize(`<img src="cid:Logo@X"><img src="cid:absent"><img src="`+png+`">`, opts)
	if strings.Count(out, png) != 2 {
		t.Errorf("deux images locales attendues : %q", out)
	}
	if strings.Contains(out, "absent") || rep.RemoteFound != 0 {
		t.Errorf("cid inconnu : %q %+v", out, rep)
	}
	out, _ = Sanitize(`<img src="data:image/png;base64,AAAA<script>">`, opts)
	if strings.Contains(strings.ToLower(out), "script") {
		t.Errorf("data: piégé : %q", out)
	}
}

func TestTextToHTML(t *testing.T) {
	out := TextToHTML("Salut <b>toi</b> & co\nvoir https://exemple.fr/a?b=1&c=2. Fin\n> cité\n> encore\nok")
	for _, want := range []string{"&lt;b&gt;toi&lt;/b&gt;", "&amp; co", `<a href="https://exemple.fr/a?b=1&amp;c=2"`, "</a>. Fin", "<blockquote>", "</blockquote>"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q absent de %q", want, out)
		}
	}
	if strings.Contains(out, "<b>") {
		t.Error("le HTML du texte doit être échappé")
	}
	if strings.Count(out, "<blockquote>") != 1 {
		t.Errorf("les lignes citées doivent être regroupées : %q", out)
	}
	evil := TextToHTML(`http://x.example/"onmouseover="alert(1)`)
	if strings.Contains(evil, `"onmouseover`) {
		t.Errorf("injection par URL : %q", evil)
	}
}

func TestHTMLToText(t *testing.T) {
	got := HTMLToText(`<html><head><title>T</title><style>p{}</style></head><body><p>Un</p><p>Deux<br>trois</p><script>x()</script></body></html>`)
	if got != "Un\n\nDeux\ntrois" && got != "Un\nDeux\ntrois" {
		t.Errorf("texte extrait : %q", got)
	}
	if strings.Contains(got, "x()") || strings.Contains(got, "p{}") {
		t.Errorf("script ou style dans le texte : %q", got)
	}
}

const sample = "From: =?UTF-8?Q?Andr=C3=A9?= <andre@exemple.fr>\r\n" +
	"To: Moi <moi@exemple.fr>, autre@exemple.fr\r\n" +
	"Cc: copie@exemple.fr\r\n" +
	"Subject: =?UTF-8?Q?Caf=C3=A9_du_matin?=\r\n" +
	"Date: Fri, 23 Oct 2026 10:00:00 +0200\r\n" +
	"Message-ID: <abc@exemple.fr>\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: multipart/mixed; boundary=\"B1\"\r\n\r\n" +
	"--B1\r\n" +
	"Content-Type: multipart/related; boundary=\"B2\"\r\n\r\n" +
	"--B2\r\n" +
	"Content-Type: multipart/alternative; boundary=\"B3\"\r\n\r\n" +
	"--B3\r\n" +
	"Content-Type: text/plain; charset=ISO-8859-1\r\n" +
	"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
	"Bonjour, caf=E9 !\r\n" +
	"--B3\r\n" +
	"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
	"<p>Bonjour, <img src=\"cid:logo@x\"> café !</p><script>alert(1)</script>\r\n" +
	"--B3--\r\n" +
	"--B2\r\n" +
	"Content-Type: image/png\r\n" +
	"Content-ID: <logo@x>\r\n" +
	"Content-Transfer-Encoding: base64\r\n\r\n" +
	"iVBORw0KGgo=\r\n" +
	"--B2--\r\n" +
	"--B1\r\n" +
	"Content-Type: application/pdf; name=\"facture.pdf\"\r\n" +
	"Content-Disposition: attachment; filename=\"facture.pdf\"\r\n" +
	"Content-Transfer-Encoding: base64\r\n\r\n" +
	"JVBERi0xLjQK\r\n" +
	"--B1--\r\n"

func TestParse(t *testing.T) {
	m, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject != "Café du matin" || m.MessageID != "abc@exemple.fr" || m.Date.IsZero() {
		t.Errorf("en-têtes : %q %q %v", m.Subject, m.MessageID, m.Date)
	}
	if len(m.From) != 1 || m.From[0].Name != "André" || m.From[0].Email != "andre@exemple.fr" || m.From[0].Short() != "André" {
		t.Errorf("From : %+v", m.From)
	}
	if len(m.To) != 2 || m.To[1].Short() != "autre@exemple.fr" || len(m.Cc) != 1 {
		t.Errorf("To/Cc : %+v %+v", m.To, m.Cc)
	}
	if !strings.Contains(m.Text, "café") {
		t.Errorf("texte ISO-8859-1 mal décodé : %q", m.Text)
	}
	if !strings.Contains(m.HTML, "<script>") { // brut : l'assainissement vient après
		t.Errorf("HTML brut attendu : %q", m.HTML)
	}
	if _, ok := m.InlineImages["logo@x"]; !ok || len(m.InlineImages) != 1 {
		t.Errorf("image incorporée : %v", m.InlineImages)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "facture.pdf" || m.Attachments[0].ContentType != "application/pdf" || m.Attachments[0].Size == 0 {
		t.Fatalf("pièces jointes : %+v", m.Attachments)
	}
	if _, data, ok := m.Attachment(0); !ok || !strings.HasPrefix(string(data), "%PDF") {
		t.Errorf("contenu de la pièce jointe : %q", data)
	}
	if _, _, ok := m.Attachment(5); ok {
		t.Error("index hors limites accepté")
	}

	out, _ := Sanitize(m.HTML, SanitizeOptions{BlockRemote: true, InlineImages: m.InlineImages})
	if strings.Contains(out, "script") || !strings.Contains(out, "data:image/png;base64,") || !strings.Contains(out, "café") {
		t.Errorf("rendu final : %q", out)
	}
}

func TestParsePlainAndGarbage(t *testing.T) {
	m, err := Parse([]byte("From: a@b.fr\r\nSubject: simple\r\n\r\nCorps du message\r\n"))
	if err != nil || m.Text != "Corps du message\r\n" || m.HTML != "" {
		t.Errorf("message simple : %+v %v", m, err)
	}
	for _, junk := range []string{"", "pas un message", "\x00\x01\x02", "Content-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type:"} {
		if _, err := Parse([]byte(junk)); err != nil {
			continue // une erreur propre est acceptable ; une panique ne l'est pas
		}
	}
}

func TestNormalizeCID(t *testing.T) {
	for in, want := range map[string]string{"<Logo@X>": "logo@x", "cid:Logo@X": "logo@x", " CID:<a@b> ": "a@b", "plain": "plain"} {
		if got := NormalizeCID(in); got != want {
			t.Errorf("NormalizeCID(%q) = %q, attendu %q", in, got, want)
		}
	}
}

func TestThumbnail(t *testing.T) {
	// Image large 600x200 : la miniature est carrée, recadrée au centre.
	src := image.NewRGBA(image.Rect(0, 0, 600, 200))
	for x := 0; x < 600; x++ {
		for y := 0; y < 200; y++ {
			src.Set(x, y, color.RGBA{R: uint8(x * 255 / 600), G: 40, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	uri, ok := Thumbnail(buf.Bytes(), 48)
	if !ok || !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("miniature : %v %.40q", ok, uri)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || format != "png" || cfg.Width != 48 || cfg.Height != 48 {
		t.Errorf("dimensions %dx%d (%s) : %v", cfg.Width, cfg.Height, format, err)
	}

	// JPEG et GIF.
	buf.Reset()
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := Thumbnail(buf.Bytes(), 48); !ok {
		t.Error("JPEG sans miniature")
	}
	buf.Reset()
	if err := gif.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := Thumbnail(buf.Bytes(), 48); !ok {
		t.Error("GIF sans miniature")
	}

	// Pas de miniature : texte, SVG, données tronquées, image aux dimensions démesurées.
	for name, data := range map[string][]byte{
		"texte":    []byte("pas une image"),
		"svg":      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"vide":     nil,
		"tronquée": buf.Bytes()[:20],
	} {
		if _, ok := Thumbnail(data, 48); ok {
			t.Errorf("%s : miniature produite à tort", name)
		}
	}
	huge := image.NewPaletted(image.Rect(0, 0, 20000, 3000), color.Palette{color.White, color.Black})
	buf.Reset()
	if err := png.Encode(&buf, huge); err != nil {
		t.Fatal(err)
	}
	if _, ok := Thumbnail(buf.Bytes(), 48); ok {
		t.Error("une image de 60 millions de pixels ne doit pas être décodée")
	}
	if !(Attachment{ContentType: "image/png"}).IsImage() || (Attachment{ContentType: "image/svg+xml"}).IsImage() || (Attachment{ContentType: "application/pdf"}).IsImage() {
		t.Error("IsImage")
	}
}

func TestDKIMStatus(t *testing.T) {
	from := []Address{{Email: "bob@example.org"}}
	cases := map[string]string{
		"mx.x; dkim=pass header.d=example.org header.s=sel":              "pass",
		"mx.x; dkim=pass header.i=@mail.example.org":                     "pass",
		"mx.x; dkim=pass header.d=autre.net":                             "warn",
		"mx.x; dkim=fail header.d=example.org":                           "warn",
		"mx.x; spf=pass smtp.mailfrom=example.org":                       "",
		"mx.x; dkim=pass":                                                "warn",
		"mx.x; dkim=fail header.d=x.org; dkim=pass header.d=example.org": "pass",
	}
	for in, want := range cases {
		if got := dkimStatus([]string{in}, from); got != want {
			t.Errorf("%q : %q, attendu %q", in, got, want)
		}
	}
	if dkimStatus([]string{"dkim=pass header.d=example.org"}, nil) != "" {
		t.Error("sans expéditeur unique, pas d'état")
	}
}

func TestCollapseQuotesAndTrackers(t *testing.T) {
	long := strings.Repeat("ligne citée assez longue ", 30)
	html, _ := Sanitize(`<p>Réponse</p><blockquote>`+long+`<blockquote>imbriquée</blockquote></blockquote><blockquote>court</blockquote>`, SanitizeOptions{})
	out := CollapseQuotes(html)
	if strings.Count(out, `<details class="wm-quote">`) != 1 || !strings.Contains(out, "<summary>…</summary><blockquote>ligne citée") || !strings.Contains(out, "<blockquote>court</blockquote>") {
		t.Errorf("repliage : %s", out)
	}
	if CollapseQuotes("<p>rien à replier</p>") != "<p>rien à replier</p>" {
		t.Error("texte modifié sans citation longue")
	}
	// pixels espions : retirés même quand le contenu distant est autorisé ; une vraie image reste
	out, rep := Sanitize(`<img src="https://t.example/p.gif" width="1" height="1"><img src="https://mandrillapp.com/track/open.php?x=1"><img src="https://x.example/photo.jpg" width="300" height="200">`, SanitizeOptions{})
	if rep.Trackers != 2 || strings.Contains(out, "t.example") || strings.Contains(out, "mandrillapp") || !strings.Contains(out, "photo.jpg") {
		t.Errorf("espions : %d %s", rep.Trackers, out)
	}
}

func TestExtraInlineTextPartsFollowTheBodyInsteadOfBeingAttachments(t *testing.T) {
	raw := "From: a@x.fr\r\nTo: b@x.fr\r\nSubject: Signature\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=B\r\n\r\n" +
		"--B\r\nContent-Type: multipart/alternative; boundary=A\r\n\r\n--A\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\ncorps texte\r\n" +
		"--A\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n<p>corps html</p>\r\n--A--\r\n" +
		"--B\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Disposition: inline\r\n\r\n<p>-- <b>Signature HTML</b></p>\r\n" +
		"--B\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Disposition: attachment; filename=\"notes.txt\"\r\n\r\nvraie pièce jointe\r\n--B--\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.HTML, "corps html") || !strings.Contains(m.HTML, "Signature HTML") {
		t.Errorf("la signature doit suivre le corps : %q", m.HTML)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "notes.txt" {
		t.Errorf("seule la vraie pièce jointe doit être listée : %+v", m.Attachments)
	}
}

func TestUnknownOrFancyCharsetsNeverDropTheText(t *testing.T) {
	mk := func(charset, body string) string {
		return "From: a@x.fr\r\nSubject: t\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=" + charset + "\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body + "\r\n"
	}
	for charset, c := range map[string]struct{ body, want string }{
		"x-inconnu-42": {"caf\xe9 au lait", "café au lait"},
		"asci":         {"r\xe9union", "réunion"},
		"us-asci":      {"simple", "simple"},
		"utf-7":        {"Caf+AOk- +- plus", "Café + plus"},
		"iso-8859-15":  {"\xa4 5", "€ 5"},
		"utf-8":        {"déjà", "déjà"},
	} {
		m, err := Parse([]byte(mk(charset, c.body)))
		if err != nil || !strings.Contains(m.Text, c.want) {
			t.Errorf("charset %s : %q (%v), attendu %q", charset, m.Text, err, c.want)
		}
	}
}

func TestSignatureStreamsAreNotAttachments(t *testing.T) {
	raw := "From: a@x.fr\r\nSubject: signé\r\nMIME-Version: 1.0\r\nContent-Type: multipart/signed; protocol=\"application/pkcs7-signature\"; boundary=S\r\n\r\n" +
		"--S\r\nContent-Type: text/plain\r\n\r\ncorps signé\r\n--S\r\nContent-Type: application/pkcs7-signature; name=smime.p7s\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAA\r\n--S--\r\n"
	m, err := Parse([]byte(raw))
	if err != nil || len(m.Attachments) != 0 || !strings.Contains(m.Text, "corps signé") {
		t.Errorf("%+v %v", m.Attachments, err)
	}
}

func TestPlainTextNestedQuotesLinksAndSpacing(t *testing.T) {
	out := TextToHTML("Bonjour www.exemple.fr, écris à Anne <anne@x.org>.\n> niveau 1\n>> niveau 2\n> retour 1\nfin (voir https://x.fr/a_(b)) ok")
	for _, want := range []string{`<div class="plain">`, `<a href="http://www.exemple.fr"`, `<a href="mailto:anne@x.org"`, ">niveau 1", "<blockquote>niveau 1", "<blockquote>niveau 2", `href="https://x.fr/a_(b)"`, "</a>) ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q absent de %q", want, out)
		}
	}
	if strings.Count(out, "<blockquote>") != 2 || strings.Count(out, "</blockquote>") != 2 {
		t.Errorf("citations imbriquées : %q", out)
	}
	if strings.Contains(out, "<br") {
		t.Errorf("les retours à la ligne sont gardés par la feuille de style, pas par <br> : %q", out)
	}
}

func TestUnflowRFC3676(t *testing.T) {
	in := "Une ligne longue \ncoupée par le client \nmais qui continue.\n-- \nSignature \nreste séparée\n> citation coupée \n> suite\n"
	got := Unflow(in, false)
	want := "Une ligne longue coupée par le client mais qui continue.\n-- \nSignature reste séparée\n> citation coupée suite\n"
	if got != want {
		t.Errorf("flowed :\n%q\nattendu\n%q", got, want)
	}
	if got := Unflow("mot-\ncoupé \nici", true); got != "mot-\ncoupéici" {
		t.Errorf("delsp : %q", got)
	}
	raw := "From: a@x.fr\r\nSubject: f\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8; format=flowed; delsp=no\r\n\r\nUne phrase \r\nsur deux lignes.\r\n"
	m, _ := Parse([]byte(raw))
	if !strings.Contains(m.Text, "Une phrase sur deux lignes.") {
		t.Errorf("Parse doit défaire le flowed : %q", m.Text)
	}
}

func TestUnnamedPartsGetRainLoopStyleNames(t *testing.T) {
	raw := "From: a@x.fr\r\nSubject: noms\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=M\r\n\r\n" +
		"--M\r\nContent-Type: multipart/alternative; boundary=A\r\n\r\n--A\r\nContent-Type: text/plain\r\n\r\ncorps\r\n" +
		"--A\r\nContent-Type: text/html\r\n\r\n<p>corps</p>\r\n--A\r\nContent-Type: text/calendar; method=REQUEST\r\n\r\nBEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n--A--\r\n" +
		"--M\r\nContent-Type: image/png\r\nContent-Disposition: attachment\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAA\r\n" +
		"--M\r\nContent-Type: message/rfc822\r\nContent-Disposition: attachment\r\n\r\nSubject: x\r\n\r\ny\r\n--M--\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range m.Attachments {
		names = append(names, a.Name)
	}
	if strings.Join(names, ",") != "calendar1.3.ics,image2.png,message3.eml" {
		t.Errorf("noms : %v", names)
	}
}

func TestSanitizerKeepsModernLayoutStylesButNeverPositioning(t *testing.T) {
	out, _ := Sanitize(`<div style="display:inline-block;border-radius:4px;font:14px/20px Roboto,Arial;background:#1a73e8;box-shadow:0 1px 2px #ccc;position:fixed;top:0;left:0;z-index:9999">x</div>`, SanitizeOptions{})
	for _, want := range []string{"display: inline-block", "border-radius: 4px", "font: 14px/20px Roboto,Arial", "background: #1a73e8", "box-shadow"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q perdu : %s", want, out)
		}
	}
	for _, bad := range []string{"position", "z-index", "top:", "left:"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q ne doit pas survivre : %s", bad, out)
		}
	}
	// une image de fond reste refusée
	if out, _ := Sanitize(`<div style="background:url(https://t.example/p.gif) #fff">x</div>`, SanitizeOptions{}); strings.Contains(out, "url(") {
		t.Errorf("url() CSS : %s", out)
	}
}

func TestMailtoLinksAndAutoLinksInHTML(t *testing.T) {
	compose := func(m string) string { return "/compose?mailto=" + url.QueryEscape(m) }
	in := `<p>Écrivez à <a href="mailto:anne@x.org?subject=Re">Anne</a> ou à paul@x.org.</p>` +
		`<p>Voir https://exemple.fr/a et www.exemple.fr.</p><pre>pas-de-lien@ici.fr</pre>` +
		`<p><a href="https://garde.fr">lien existant paul@x.org</a></p>`
	out, _ := Sanitize(in, SanitizeOptions{MailtoURL: compose})
	for _, want := range []string{
		`href="/compose?mailto=mailto%3Aanne%40x.org%3Fsubject%3DRe" target="_top"`,
		`href="/compose?mailto=mailto%3Apaul%40x.org" target="_top">paul@x.org</a>`,
		`<a href="https://exemple.fr/a" target="_blank" rel="nofollow noreferrer noopener">https://exemple.fr/a</a>`,
		`<a href="http://www.exemple.fr" target="_blank"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%q absent de :\n%s", want, out)
		}
	}
	if strings.Contains(out, `<pre><a`) || !strings.Contains(out, "<pre>pas-de-lien@ici.fr</pre>") {
		t.Errorf("le contenu de <pre> ne doit pas être transformé : %s", out)
	}
	if strings.Count(out, "lien existant paul@x.org</a>") != 1 {
		t.Errorf("pas de lien dans un lien : %s", out)
	}
	// sans MailtoURL, le lien reste un mailto: ordinaire
	if out, _ := Sanitize(`<a href="mailto:a@b.fr">x</a>`, SanitizeOptions{}); !strings.Contains(out, `href="mailto:a@b.fr"`) {
		t.Errorf("mailto inchangé : %s", out)
	}
}

func TestHTMLToTextQuotesListsLinksAndWrapping(t *testing.T) {
	got := HTMLToText(`<p>Bonjour,</p><blockquote><p>Question&nbsp;?</p><blockquote>Citation imbriquée</blockquote></blockquote>` +
		`<ul><li>un</li><li>deux</li></ul><hr><p>Voir <a href="https://exemple.fr/page">la page</a> et <a href="mailto:a@b.fr">a@b.fr</a>.</p>` +
		`<table><tr><td>A</td><td>B</td></tr></table>`)
	for _, want := range []string{"Bonjour,", "> Question ?", ">> Citation imbriquée", "* un", "* deux", "_______________________________", "la page (https://exemple.fr/page)", "a@b.fr", "A  B"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q absent de :\n%s", want, got)
		}
	}
	if strings.Contains(got, "mailto:a@b.fr (") {
		t.Errorf("une adresse identique au texte ne doit pas être répétée :\n%s", got)
	}
	long := HTMLToText("<p>" + strings.Repeat("mot ", 60) + "</p>")
	for _, line := range strings.Split(long, "\n") {
		if len([]rune(line)) > 100 {
			t.Errorf("ligne de %d caractères : %q", len([]rune(line)), line)
		}
	}
	if q := HTMLToText("<blockquote>" + strings.Repeat("long ", 40) + "</blockquote>"); !strings.HasPrefix(strings.Split(q, "\n")[1], "> ") {
		t.Errorf("le préfixe de citation doit être repris sur les lignes coupées :\n%s", q)
	}
}

func TestMalformedAddressHeaderKeepsReadableRecipients(t *testing.T) {
	raw := "From: a@x.fr\r\nTo: Service Client service@x.fr, \"Durand, Anne\" <anne@y.fr>; bob@z.fr\r\nSubject: s\r\n\r\ncorps\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.To) != 3 {
		t.Fatalf("%d destinataires : %+v", len(m.To), m.To)
	}
	if m.To[0].Email != "service@x.fr" || m.To[0].Name != "Service Client" || m.To[2].Email != "bob@z.fr" {
		t.Errorf("destinataires : %+v", m.To)
	}
}

// Mots encodés RFC 2047 laissés dans un nom entre guillemets : décodés quand même (comme RainLoop).
func TestQuotedEncodedWordSender(t *testing.T) {
	raw := "From: \"=?utf-8?q?Pinterest?=\" <no-reply@pinterest.com>\r\n" +
		"To: \"=?iso-8859-1?q?Andr=E9?=\" <andre@example.org>\r\n" +
		"Subject: =?utf-8?q?Id=C3=A9e_D=C3=A9co?=\r\n\r\nbonjour\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.From) != 1 || m.From[0].Name != "Pinterest" {
		t.Errorf("expéditeur : %+v", m.From)
	}
	if len(m.To) != 1 || m.To[0].Name != "André" {
		t.Errorf("destinataire : %+v", m.To)
	}
	if m.Subject != "Idée Déco" {
		t.Errorf("objet : %q", m.Subject)
	}
}

func TestExifOrientationRotatesThumbnail(t *testing.T) {
	// Image 2×1 : pixel gauche rouge, droit bleu. Orientation 6 (quart de tour horaire) : le rouge passe en haut.
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.RGBA{255, 0, 0, 255})
	src.Set(1, 0, color.RGBA{0, 0, 255, 255})
	out := applyOrientation(src, 6)
	if out.Bounds().Dx() != 1 || out.Bounds().Dy() != 2 {
		t.Fatalf("dimensions après rotation : %v", out.Bounds())
	}
	r, _, _, _ := out.At(0, 0).RGBA()
	if r < 0x8000 {
		t.Error("le pixel rouge doit se retrouver en haut")
	}
	if applyOrientation(src, 1).Bounds() != src.Bounds() {
		t.Error("orientation 1 : image inchangée")
	}
	if n := exifOrientation([]byte("pas un jpeg")); n != 0 {
		t.Errorf("données non JPEG : %d", n)
	}
}

// Image incorporée désignée par Content-Location (Outlook) : elle doit s'afficher comme une image cid:.
func TestContentLocationInlineImage(t *testing.T) {
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	raw := "MIME-Version: 1.0\r\nContent-Type: multipart/related; boundary=b\r\n\r\n" +
		"--b\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" +
		`<p><img src="http://exemple.invalid/image001.png"></p>` + "\r\n" +
		"--b\r\nContent-Type: image/png\r\nContent-Location: http://exemple.invalid/image001.png\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" + png + "\r\n--b--\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 0 {
		t.Errorf("l'image incorporée ne doit pas être listée en pièce jointe : %d", len(m.Attachments))
	}
	html, _ := Sanitize(m.HTML, SanitizeOptions{InlineImages: m.InlineImages, BlockRemote: true})
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Errorf("image non remplacée :\n%s", html)
	}
	if strings.Contains(html, "exemple.invalid") {
		t.Errorf("l'URL distante doit disparaître :\n%s", html)
	}
}
