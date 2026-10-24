package web

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/config"
	"wmail-go/internal/mailconn"
)

// Conversations contre un vrai Dovecot (THREAD) : lignes regroupées, ouverture d'une conversation, actions sur toute la
// conversation. Ignoré sans DOVECOT_IMAP.
func TestThreadsAgainstDovecot(t *testing.T) {
	addr := os.Getenv("DOVECOT_IMAP")
	if addr == "" {
		t.Skip("DOVECOT_IMAP non défini")
	}
	host, portS, _ := strings.Cut(addr, ":")
	port, _ := strconv.Atoi(portS)
	login := fmt.Sprintf("web%d", rand.Intn(1e9))
	g, err := mailconn.NewGuard(true, []string{host})
	if err != nil {
		t.Fatal(err)
	}
	ep := mailconn.Endpoint{Host: host, Port: port, Security: mailconn.SecurityNone, Login: login, Password: "testpass"}
	pool := mailconn.NewPool(g, time.Minute)
	defer pool.CloseAll()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	msg := func(id, parent, subject string, i int) []byte {
		extra := ""
		if parent != "" {
			extra = "In-Reply-To: <" + parent + ">\r\nReferences: <" + parent + ">\r\n"
		}
		return []byte(fmt.Sprintf("From: a%d@x.fr\r\nTo: moi@example.org\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s>\r\n%sMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\ncorps\r\n",
			i, subject, base.Add(time.Duration(i)*time.Hour).Format(time.RFC1123Z), id, extra))
	}
	err = pool.Do(ctx, "w", ep, func(c *mailconn.Conn) error {
		for i, m := range [][]byte{msg("a@x", "", "Sujet A", 0), msg("b@x", "a@x", "Re: Sujet A", 1), msg("c@x", "b@x", "Re: Sujet A", 2), msg("d@x", "", "Sujet D", 3)} {
			if _, err := c.Append("INBOX", m, fmt.Sprintf("m%d", i), imap.FlagSeen); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Skipf("Dovecot injoignable : %v", err)
	}

	e := newEnv(t, func(cfg *config.Config) { cfg.AllowedPrivateHosts = append(cfg.AllowedPrivateHosts, host) })
	c, tok := e.loggedIn("alice")
	// Le premier compte doit passer le test IMAP et SMTP : une boîte de test sert de première boîte ; Dovecot est la seconde.
	first := startMail(t)
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, first, "save"), nil))
	form := accountForm(tok, mailPorts{imap: port, smtp: first.smtp}, "save")
	form.Set("email", "dovecot@localhost")
	form.Set("imap_host", host)
	form.Set("imap_login", login)
	form.Set("imap_password", "testpass")
	e.expect1(e.do(c, "POST", "/accounts/save", form, nil))
	_, list := e.do(c, "GET", "/accounts", nil, nil)
	ids := regexp.MustCompile(`/accounts/([0-9a-f]{24})/edit`).FindAllStringSubmatch(list, -1)
	if len(ids) < 2 {
		t.Fatalf("deux boîtes attendues :\n%s", list)
	}
	e.expect1(e.do(c, "POST", "/accounts/select", url.Values{"csrf": {tok}, "id": {ids[len(ids)-1][1]}, "back": {"/"}}, nil))

	_, page := e.do(c, "GET", "/", nil, nil)
	if strings.Contains(page, `class="threads"`) {
		t.Fatal("conversations affichées sans l'option")
	}
	prefs := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"rainloop"}, "idle_minutes": {"30"}, "absolute_hours": {"8"}, "page_size": {"50"}, "threads": {"1"}}
	if resp, _ := e.do(c, "POST", "/settings/preferences", prefs, nil); resp.StatusCode != 200 {
		t.Fatalf("préférences : %d", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	badge := regexp.MustCompile(`class="threads" href="([^"]+)" title="[^"]*">3<`).FindStringSubmatch(page)
	if badge == nil || rowCount(page) != 2 {
		t.Fatalf("deux lignes dont une conversation de 3 messages attendues :\n%s", page)
	}
	th := regexp.MustCompile(`name="th_(\d+)" value="([\d,]+)"`).FindStringSubmatch(page)
	if th == nil || len(strings.Split(th[2], ",")) != 3 {
		t.Fatalf("membres de la conversation : %v", th)
	}
	// ouverture de la conversation : trois messages, bandeau de retour
	_, page = e.do(c, "GET", strings.ReplaceAll(badge[1], "&amp;", "&"), nil, nil)
	if rowCount(page) != 3 || !strings.Contains(page, `class="bar threadbar"`) {
		t.Errorf("conversation ouverte :\n%s", page)
	}
	// supprimer la conversation entière depuis la liste regroupée
	del := url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"INBOX"}, "uid": {th[1]}, "th_" + th[1]: {th[2]}, "back": {"/"}}
	e.do(c, "POST", "/mail/action", del, nil)
	_, page = e.do(c, "GET", "/?threads=0", nil, nil)
	if rowCount(page) != 1 || !strings.Contains(page, "Sujet D") {
		t.Errorf("après suppression de la conversation, seul « Sujet D » doit rester :\n%s", page)
	}
}

func rowCount(page string) int {
	return len(regexp.MustCompile(`<li id="m\d+" draggable="true" class="msgrow[ "]`).FindAllString(page, -1))
}
