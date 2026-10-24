package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestAdvancedSearchTerms(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	v := url.Values{"q": {"budget"}, "adv_from": {"bob"}, "adv_subject": {`rapport "mensuel"`}, "adv_unread": {"1"}, "adv_flagged": {"1"}, "adv_attach": {"1"}, "adv_date": {"1w"}}
	got := strings.Join(advancedTerms(v, now), " ")
	want := "from:bob subject:\"rapport mensuel\" is:unseen,flagged has:attachment date:2026.10.16/2026.10.23"
	if got != want {
		t.Errorf("termes :\n%q\nattendu :\n%q", got, want)
	}
	if lp := parseListParams(v); !strings.HasPrefix(lp.Q, "budget from:bob ") {
		t.Errorf("requête : %q", lp.Q)
	}
	if got := advancedTerms(url.Values{"adv_date": {"nimporte"}}, now); len(got) != 0 {
		t.Errorf("période invalide : %v", got)
	}
}

func TestInboxStarLinkTogglesFlaggedFilter(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `class="fstar" href="/?q=is%3Aflagged"`) {
		t.Fatalf("étoile de la boîte de réception absente")
	}
	_, page = e.do(c, "GET", "/?q=is%3Aflagged", nil, nil)
	if !strings.Contains(page, `class="fstar on" href="/"`) {
		t.Errorf("l'étoile active doit ramener à la liste complète")
	}
}

func TestAppendEMLToFolder(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	eml := "From: Dépôt <depot@example.org>\nTo: test@localhost\nSubject: Message déposé\nDate: Mon, 05 Oct 2026 09:00:00 +0000\nMessage-ID: <depot1@x>\nMIME-Version: 1.0\nContent-Type: text/plain; charset=UTF-8\n\nbonjour depuis un fichier\n"
	resp, body := upload2(e, c, "/mail/append", map[string]string{"csrf": tok, "folder": "INBOX"}, []byte(eml))
	if resp.StatusCode != 200 || !strings.Contains(body, `"uid":`) {
		t.Fatalf("ajout : %d %s", resp.StatusCode, body)
	}
	_, page := e.do(c, "GET", "/?q=Message+d%C3%A9pos%C3%A9&scope=subject", nil, nil)
	if !strings.Contains(page, "Message déposé") {
		t.Error("message ajouté absent de la liste")
	}
	// pas un message, dossier invalide
	resp, _ = upload2(e, c, "/mail/append", map[string]string{"csrf": tok, "folder": "INBOX"}, []byte("pas un message du tout\x00\x01"))
	if resp.StatusCode != 422 {
		t.Errorf("fichier quelconque : %d", resp.StatusCode)
	}
	resp, _ = upload2(e, c, "/mail/append", map[string]string{"csrf": tok, "folder": ""}, []byte(eml))
	if resp.StatusCode != 400 {
		t.Errorf("dossier invalide : %d", resp.StatusCode)
	}
	resp, _ = upload2(e, c, "/mail/append", map[string]string{"folder": "INBOX"}, []byte(eml))
	if resp.StatusCode != 403 {
		t.Errorf("sans csrf : %d", resp.StatusCode)
	}
}

func upload2(e *env, c *httpClient, path string, fields map[string]string, data []byte) (*http.Response, string) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", "m.eml")
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", e.ts.URL+path, &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(resp.Body)
	return resp, out.String()
}

func TestListDateAndSubjectPrefix(t *testing.T) {
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, time.Local)
	for _, c := range []struct {
		t    time.Time
		lang string
		want string
	}{
		{now.Add(-20 * time.Second), "fr", "à l'instant"},
		{now.Add(-12 * time.Minute), "fr", "il y a 12 min"},
		{now.Add(-2*time.Hour - 5*time.Minute), "en", "2 h ago"},
		{now.Add(-4*time.Hour - time.Minute), "fr", "10:59"},
		{time.Date(2026, 10, 22, 9, 30, 0, 0, time.Local), "fr", "hier 09:30"},
		{time.Date(2026, 10, 22, 9, 30, 0, 0, time.Local), "en", "yesterday 09:30"},
		{now.Add(30 * time.Minute), "fr", "15:30"}, // date future : pas de « il y a »
		{time.Date(2026, 10, 14, 14, 3, 0, 0, time.Local), "fr", "14 oct."},
		{time.Date(2026, 1, 3, 14, 3, 0, 0, time.Local), "en", "3 Jan"},
		{time.Date(2025, 10, 8, 9, 0, 0, 0, time.Local), "fr", "8 octobre 2025"},
	} {
		if got := listDate(c.t, now, c.lang, time.Local); got != c.want {
			t.Errorf("listDate(%v, %s) = %q, attendu %q", c.t, c.lang, got, c.want)
		}
	}
	for in, want := range map[string][2]string{
		"Re: Réunion":         {"Re: ", "Réunion"},
		"RE[2]: Fwd: Facture": {"RE[2]: Fwd: ", "Facture"},
		"Réunion lundi":       {"", "Réunion lundi"},
		"Re:":                 {"", "Re:"},
		"Regarde ça: super":   {"", "Regarde ça: super"},
		"TR: Devis":           {"TR: ", "Devis"},
	} {
		if p, r := splitSubject(in); p != want[0] || r != want[1] {
			t.Errorf("splitSubject(%q) = %q|%q, attendu %q|%q", in, p, r, want[0], want[1])
		}
	}
}

func TestTrashedAndSpamMessagesAreMarkedReadAndButtonsFollowFolder(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/", nil, nil)
	uid := uidOf(linkTo(t, page, "Facture octobre"))
	if !strings.Contains(rowOf(page, uid), "msgrow unread") {
		t.Fatal("le message de départ doit être non lu")
	}
	e.expect1(e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"INBOX"}, "uid": {uid}, "back": {"/"}}, nil))
	_, trash := e.do(c, "GET", "/?folder=Trash", nil, nil)
	subs, _ := rows(trash)
	if len(subs) != 1 || strings.Contains(trash, "msgrow unread") {
		t.Errorf("corbeille : %v, le message doit y être marqué lu\n", subs)
	}
	if !strings.Contains(trash, `name="action" value="archive"`) || !strings.Contains(trash, `value="spam"`) {
		t.Error("dans la corbeille, archiver et spam restent proposés")
	}
	// Indésirables : « pas un indésirable » remplace « spam »
	_, page = e.do(c, "GET", "/", nil, nil)
	uid2 := uidOf(linkTo(t, page, "Re: Facture octobre"))
	e.expect1(e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"spam"}, "folder": {"INBOX"}, "uid": {uid2}, "back": {"/"}}, nil))
	_, junk := e.do(c, "GET", "/?folder=Junk", nil, nil)
	if strings.Contains(junk, "msgrow unread") || !strings.Contains(junk, `value="notspam"`) || strings.Contains(junk, `name="action" value="spam"`) {
		t.Errorf("indésirables : lu=%v pas-spam=%v spam=%v", !strings.Contains(junk, "msgrow unread"), strings.Contains(junk, `value="notspam"`), strings.Contains(junk, `name="action" value="spam"`))
	}
	_, juid := rows(junk)
	e.expect1(e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"notspam"}, "folder": {"Junk"}, "uid": {uidOf(juid[0])}, "back": {"/"}}, nil))
	_, inbox := e.do(c, "GET", "/", nil, nil)
	if s, _ := rows(inbox); !slicesContains(s, "Re: Facture octobre") {
		t.Error("« pas un indésirable » doit renvoyer le message dans la boîte de réception")
	}
}

func rowOf(page, uid string) string {
	return regexp.MustCompile(`<li id="m` + uid + `"[\s\S]*?</li>`).FindString(page)
}

func TestLongHeaderDate(t *testing.T) {
	d := time.Date(2026, 10, 14, 14, 3, 0, 0, time.Local)
	if got := formatLongDate(d, "fr", time.Local); got != "14 octobre 2026 14:03" {
		t.Errorf("fr : %q", got)
	}
	if got := formatLongDate(d, "en", time.Local); got != "14 October 2026 14:03" {
		t.Errorf("en : %q", got)
	}
}
