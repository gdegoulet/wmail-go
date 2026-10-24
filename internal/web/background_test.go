package web

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

func tinyPNG(w, h int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)))
	return b.Bytes()
}

func TestPersonalBackground(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	if _, css := e.do(c, "GET", "/user.css", nil, nil); strings.Contains(css, "/user/background") {
		t.Error("fond déclaré sans image")
	}
	// refus : pas une image, mauvais format
	resp, _ := e.upload(c, "/settings/background", tok, "file", "x.png", []byte("pas une image"))
	if resp.StatusCode != 400 {
		t.Errorf("fichier invalide : %d", resp.StatusCode)
	}
	resp, _ = e.upload(c, "/settings/background", tok, "file", "x.png", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"))
	if resp.StatusCode != 400 {
		t.Errorf("GIF accepté : %d", resp.StatusCode)
	}
	resp, _ = e.upload(c, "/settings/background", tok, "file", "fond.png", tinyPNG(40, 30))
	e.expect(resp, 303, "/settings")
	_, css := e.do(c, "GET", "/user.css", nil, nil)
	if !strings.Contains(css, "url(/user/background?v=1)") {
		t.Fatalf("feuille utilisateur : %q", css)
	}
	resp, img := e.do(c, "GET", "/user/background", nil, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(img, "\x89PNG") {
		t.Errorf("image servie : %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	// jamais servi sans session ni à un autre compte
	if resp, _ = e.do(e.client(), "GET", "/user/background", nil, nil); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("sans session : %d", resp.StatusCode)
	}
	cb := e.client()
	e.register(cb, "bob")
	if resp, _ = e.do(cb, "GET", "/user/background", nil, nil); resp.StatusCode != 404 {
		t.Errorf("autre compte : %d", resp.StatusCode)
	}
	// trop grande
	resp, _ = e.upload(c, "/settings/background", tok, "file", "grand.png", tinyPNG(maxBackgroundDim+1, 1))
	if resp.StatusCode != 400 {
		t.Errorf("dimensions excessives : %d", resp.StatusCode)
	}
	// retrait
	resp, _ = e.do(c, "POST", "/settings/background/delete", map[string][]string{"csrf": {tok}}, nil)
	e.expect(resp, 303, "/settings")
	if _, css = e.do(c, "GET", "/user.css", nil, nil); strings.Contains(css, "/user/background") {
		t.Error("fond encore déclaré")
	}
	if resp, _ = e.do(c, "GET", "/user/background", nil, nil); resp.StatusCode != 404 {
		t.Errorf("image encore servie : %d", resp.StatusCode)
	}
}
