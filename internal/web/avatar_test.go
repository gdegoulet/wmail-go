package web

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
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

func smallPNG() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	return b.Bytes()
}

func TestSenderAvatarFromContactThenGravatar(t *testing.T) {
	var calls int
	gravatar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if !strings.Contains(r.URL.RawQuery, "d=404") {
			t.Errorf("Gravatar doit être interrogé avec d=404 : %s", r.URL)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(smallPNG())
	}))
	defer gravatar.Close()

	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add([]byte("From: Zoé <zoe@example.org>\r\nTo: test@localhost\r\nSubject: Photo\r\nDate: Mon, 02 Mar 2026 10:00:00 +0000\r\nMessage-ID: <av@test>\r\n\r\ncorps\r\n"), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save"), nil))

	// sans photo ni Gravatar : 404, l'interface garde la silhouette
	resp, _ := e.do(c, "GET", "/mail/avatar?email=zoe@example.org", nil, nil)
	if resp.StatusCode != 404 {
		t.Errorf("sans photo : %d", resp.StatusCode)
	}
	// photo du carnet d'adresses : servie sans contacter personne
	e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}, "first": {"Zoé"}, "email": {"zoe@example.org"}}, nil)
	// photo enregistrée dans la fiche : servie telle quelle, sans contacter personne
	_, page := e.do(c, "GET", "/contacts?q=zoe", nil, nil)
	id := regexp.MustCompile(`/contacts\?edit=([0-9a-f]+)`).FindStringSubmatch(page)[1]
	photo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(smallPNG())
	e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}, "id": {id}, "first": {"Zoé"}, "email": {"zoe@example.org"}, "photo": {photo}}, nil)
	resp, body := e.do(c, "GET", "/mail/avatar?email=ZOE@example.org", nil, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(body, "\x89PNG") {
		t.Errorf("photo du carnet : %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	// une photo trop grande ou d'un type inattendu est ignorée
	e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}, "id": {id}, "first": {"Zoé"}, "email": {"zoe@example.org"}, "photo": {"javascript:alert(1)"}}, nil)
	if resp, _ := e.do(c, "GET", "/mail/avatar?email=zoe@example.org", nil, nil); resp.StatusCode != 404 {
		t.Errorf("photo invalide acceptée : %d", resp.StatusCode)
	}
	if calls != 0 {
		t.Errorf("Gravatar interrogé alors que l'option est désactivée (%d appels)", calls)
	}
	// adresse invalide
	if resp, _ := e.do(c, "GET", "/mail/avatar?email=pas-une-adresse", nil, nil); resp.StatusCode != 404 {
		t.Errorf("adresse invalide : %d", resp.StatusCode)
	}
}

func TestGravatarHash(t *testing.T) {
	// valeur de référence de la documentation Gravatar
	if got := gravatarHash("  MyEmailAddress@example.com "); got != "0bc83cb571cd1c50ba6f3e8a78ef1346" {
		t.Errorf("empreinte : %s", got)
	}
}

// Aucun gestionnaire d'événement en ligne dans les gabarits : la politique de sécurité du contenu les refuse.
func TestNoInlineEventHandlers(t *testing.T) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		t.Fatal(err)
	}
	bad := regexp.MustCompile(`\son(error|click|load|change|submit|input|focus|blur)\s*=`)
	for _, e := range entries {
		b, err := templateFS.ReadFile("templates/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if m := bad.Find(b); m != nil {
			t.Errorf("%s : gestionnaire en ligne %q interdit par la CSP", e.Name(), m)
		}
	}
}
