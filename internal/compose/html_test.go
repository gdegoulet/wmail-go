package compose

import (
	"encoding/base64"
	"net/mail"
	"strings"
	"testing"
	"time"

	"wmail-go/internal/mailview"
)

const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestPrepareHTMLSanitizes(t *testing.T) {
	src := `<p onclick="x()" style="color:red;background:url(http://evil/x.png)">Bonjour <b>toi</b></p>` +
		`<script>alert(1)</script><iframe src="http://evil"></iframe><form action="http://evil"><input></form>` +
		`<a href="javascript:alert(1)">piège</a> <a href="https://ok.example/">ok</a>` +
		`<img src="http://tracker.example/p.gif"><img src="cid:inconnu"><img src="relative.png">`
	out, imgs := PrepareHTML(src, nil)
	for _, bad := range []string{"<script", "onclick", "iframe", "<form", "javascript:", "tracker.example", "url(", "relative.png", "cid:inconnu", "<input"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Errorf("%q devrait avoir disparu :\n%s", bad, out)
		}
	}
	for _, good := range []string{"<b>toi</b>", `href="https://ok.example/"`, "color: red", "<!DOCTYPE html>", `charset="utf-8"`} {
		if !strings.Contains(out, good) {
			t.Errorf("%q absent :\n%s", good, out)
		}
	}
	if len(imgs) != 0 || strings.Contains(out, "<img") {
		t.Errorf("aucune image valide attendue : %d\n%s", len(imgs), out)
	}
}

func TestPrepareHTMLExtractsDataImages(t *testing.T) {
	src := `<p>Voici</p><img src="data:image/png;base64,` + pngB64 + `" alt="a"><img src="data:image/png;base64,` + pngB64 + `">` +
		`<img src="data:text/html;base64,PHNjcmlwdD4="><img src="data:image/svg+xml;base64,PHN2Zy8+">`
	out, imgs := PrepareHTML(src, nil)
	if len(imgs) != 1 {
		t.Fatalf("une image attendue (doublon fusionné), obtenu %d\n%s", len(imgs), out)
	}
	if imgs[0].ContentType != "image/png" || len(imgs[0].Data) == 0 {
		t.Errorf("image : %+v", imgs[0])
	}
	if strings.Count(out, "cid:"+imgs[0].CID) != 2 || strings.Contains(out, "data:") || strings.Contains(out, "svg") {
		t.Errorf("références :\n%s", out)
	}
	raw, _ := base64.StdEncoding.DecodeString(pngB64)
	if string(imgs[0].Data) != string(raw) {
		t.Error("contenu de l'image altéré")
	}
}

func TestPrepareHTMLResolvesQuotedCID(t *testing.T) {
	data := "data:image/png;base64," + pngB64
	out, imgs := PrepareHTML(`<img src="cid:Logo@x">`, map[string]string{"logo@x": data})
	if len(imgs) != 1 || !strings.Contains(out, "cid:"+imgs[0].CID) {
		t.Errorf("cid d'un message cité non repris : %d\n%s", len(imgs), out)
	}
}

func TestPrepareHTMLLimitsImages(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxInlineCount+5; i++ {
		raw := append([]byte{0x89, 'P', 'N', 'G'}, byte(i), byte(i>>8), 1, 2, 3)
		b.WriteString(`<img src="data:image/png;base64,` + base64.StdEncoding.EncodeToString(raw) + `">`)
	}
	_, imgs := PrepareHTML(b.String(), nil)
	if len(imgs) != maxInlineCount {
		t.Errorf("%d images conservées, limite %d", len(imgs), maxInlineCount)
	}
}

func TestBuildHTMLStructure(t *testing.T) {
	raw := func(m *Message) *mailview.Message {
		b, err := m.Build(false)
		if err != nil {
			t.Fatal(err)
		}
		p, err := mailview.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	html, imgs := PrepareHTML(`<p>Salut <b>Zoë</b></p><img src="data:image/png;base64,`+pngB64+`">`, nil)
	m := &Message{
		From: &mail.Address{Name: "Gérard", Address: "g@sdv.fr"}, To: []*mail.Address{{Address: "a@b.fr"}},
		Subject: "HTML é", Text: HTMLToPlain(html), HTML: html, Images: imgs, Date: time.Now(),
	}
	p := raw(m)
	if !strings.Contains(p.HTML, "<b>Zoë</b>") || !strings.Contains(p.Text, "Salut") || strings.Contains(p.Text, "<b>") {
		t.Errorf("parties : HTML=%q Text=%q", p.HTML, p.Text)
	}
	if len(p.InlineImages) != 1 || len(p.Attachments) != 0 {
		t.Errorf("images en ligne : %d, pièces jointes : %d", len(p.InlineImages), len(p.Attachments))
	}
	// avec pièces jointes : mixed > alternative > related
	m.Attachments = []Attachment{{Name: "a.txt", ContentType: "text/plain", Data: []byte("pj")}}
	b, _ := m.Build(false)
	s := string(b)
	for _, want := range []string{"multipart/mixed", "multipart/alternative", "multipart/related", "Content-Id: <" + imgs[0].CID + ">", "Content-Disposition: attachment"} {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(want)) {
			t.Errorf("structure : %q absent", want)
		}
	}
	p = raw(m)
	if len(p.Attachments) != 1 || p.Attachments[0].Name != "a.txt" || len(p.InlineImages) != 1 || !strings.Contains(p.HTML, "Zoë") {
		t.Errorf("relecture : %+v %d %q", p.Attachments, len(p.InlineImages), p.HTML)
	}
	// sans image : alternative simple
	m2 := &Message{From: m.From, To: m.To, Subject: "s", Text: "t", HTML: "<p>t</p>"}
	b, _ = m2.Build(false)
	if strings.Contains(string(b), "multipart/related") || strings.Contains(string(b), "multipart/mixed") || !strings.Contains(string(b), "multipart/alternative") {
		t.Errorf("structure sans image :\n%s", b)
	}
	// aucune ligne trop longue, CRLF partout
	for _, line := range strings.Split(string(b), "\r\n") {
		if len(line) > 998 {
			t.Error("ligne trop longue")
		}
	}
}

func TestTextToHTMLBody(t *testing.T) {
	if got := TextToHTMLBody("a <b>\r\nc & d\n\ne"); got != "a &lt;b&gt;<br>c &amp; d<br><br>e" {
		t.Errorf("TextToHTMLBody = %q", got)
	}
}

func TestHTMLToPlainNormalizesNonBreakingSpaces(t *testing.T) {
	if got := HTMLToPlain("<p>a&nbsp;b</p>"); !strings.Contains(got, "a b") || strings.Contains(got, "\u00a0") {
		t.Errorf("HTMLToPlain = %q", got)
	}
}
