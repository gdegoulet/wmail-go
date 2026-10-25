package web

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
)

func TestTrustedSenderAndImageProxy(t *testing.T) {
	var hits int
	img := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/pic.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(testPNG(8, 8))
		default: // pas une image : doit être refusé par le serveur mandataire
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("<html><script>alert(1)</script></html>"))
		}
	}))
	defer img.Close()

	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add([]byte("From: Pub <pub@example.org>\r\nTo: test@localhost\r\nSubject: Promo\r\nDate: Mon, 02 Mar 2026 10:00:00 +0000\r\nMessage-ID: <promo@test>\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n"+
			`<p>Bonjour</p><img src="`+img.URL+`/pic.png"><img src="`+img.URL+`/faux.png">`+"\r\n"), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	form := accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save")
	e.expect1(e.do(c, "POST", "/accounts/save", form, nil))
	_, page := e.do(c, "GET", "/?q=Promo&scope=subject", nil, nil)
	link := linkTo(t, page, "Promo")
	uid := uidOf(link)
	bodyURL := "/mail/body?folder=INBOX&uid=" + uid

	// 1. bloquées par défaut
	resp, body := e.do(c, "GET", bodyURL, nil, nil)
	if !strings.Contains(body, "data-blocked") || strings.Contains(body, img.URL) || strings.Contains(resp.Header.Get("Content-Security-Policy"), "http:") {
		t.Fatalf("images non bloquées :\n%s\n%s", body, resp.Header.Get("Content-Security-Policy"))
	}
	// 2. expéditeur de confiance : affichées d'emblée
	_, msg := e.do(c, "GET", link, nil, nil)
	if !strings.Contains(msg, `action="/mail/trust"`) {
		t.Fatal("bouton « toujours afficher » absent")
	}
	e.do(c, "POST", "/mail/trust", url.Values{"csrf": {tok}, "email": {"PUB@example.org"}, "stay": {"/"}}, nil)
	resp, body = e.do(c, "GET", bodyURL, nil, nil)
	if !strings.Contains(body, img.URL+"/pic.png") || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "http:") {
		t.Errorf("expéditeur de confiance : images toujours bloquées\n%s", body)
	}
	_, settings := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(settings, "pub@example.org") {
		t.Error("expéditeur de confiance absent de la liste des réglages")
	}
	// retrait
	e.do(c, "POST", "/mail/trust", url.Values{"csrf": {tok}, "email": {"pub@example.org"}, "action": {"remove"}}, nil)
	if _, body = e.do(c, "GET", bodyURL, nil, nil); !strings.Contains(body, "data-blocked") {
		t.Error("l'expéditeur retiré doit être bloqué à nouveau")
	}
	// 3. mandataire d'images : le lecteur ne contacte jamais l'expéditeur
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([0-9a-f]{24})/edit`).FindStringSubmatch(accounts)[1]
	edit := accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save")
	edit.Set("id", id)
	edit.Set("proxy_images", "on")
	edit.Set("block_remote", "")
	edit.Del("block_remote")
	e.do(c, "POST", "/accounts/save", edit, nil)
	hits = 0
	resp, body = e.do(c, "GET", bodyURL+"&remote=1", nil, nil)
	if strings.Contains(body, img.URL) || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "'self'") {
		t.Fatalf("les images doivent passer par le serveur :\n%s", body)
	}
	srcs := regexp.MustCompile(`src="(/mail/image\?[^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(srcs) != 2 {
		t.Fatalf("%d images réécrites :\n%s", len(srcs), body)
	}
	resp, data := e.do(c, "GET", html.UnescapeString(srcs[0][1]), nil, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(data, "\x89PNG") || hits != 1 {
		t.Errorf("image mandatée : %d %s (%d appels)", resp.StatusCode, resp.Header.Get("Content-Type"), hits)
	}
	if resp, _ = e.do(c, "GET", html.UnescapeString(srcs[1][1]), nil, nil); resp.StatusCode != 404 {
		t.Errorf("contenu qui n'est pas une image : %d", resp.StatusCode)
	}
	// signature fausse, adresse modifiée, hors session
	bad := strings.Replace(html.UnescapeString(srcs[0][1]), "sig=", "sig=00", 1)
	if resp, _ = e.do(c, "GET", bad, nil, nil); resp.StatusCode != 404 {
		t.Errorf("signature fausse : %d", resp.StatusCode)
	}
	if resp, _ = e.do(e.client(), "GET", html.UnescapeString(srcs[0][1]), nil, nil); resp.StatusCode != 303 {
		t.Errorf("sans session : %d", resp.StatusCode)
	}
}
