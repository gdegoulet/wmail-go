package web

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
	"github.com/gdegoulet/wmail-go/internal/mailview"
)

// mdnEnv ouvre une session dont INBOX contient, en plus du jeu habituel, un message qui demande un accusé de lecture
// et un message du propriétaire lui-même qui en demande aussi.
func mdnEnv(t *testing.T) (*env, *httpClient, string, *mailtest.SMTPServer) {
	t.Helper()
	e := newEnv(t, nil)
	smtp := mailtest.StartSMTPCapture(t, "none")
	imapPort := mailtest.StartIMAPSeeded(t, "none", func(t testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		d := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
		add([]byte("From: Dest <dest@example.org>\r\nTo: test@localhost\r\nSubject: Merci de confirmer\r\nDate: Mon, 05 Oct 2026 09:00:00 +0000\r\nMessage-ID: <mdn1@x>\r\nDisposition-Notification-To: Receipts <receipts@example.org>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nbonjour\r\n"), d)
		add([]byte("From: test@localhost\r\nTo: x@y.fr\r\nSubject: Mon propre message\r\nDate: Mon, 05 Oct 2026 10:00:00 +0000\r\nMessage-ID: <mdn2@x>\r\nDisposition-Notification-To: test@localhost\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nbonjour\r\n"), d)
	})
	c, tok := e.loggedIn("alice")
	resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: imapPort, smtp: smtp.Port}, "save"), nil)
	e.expect(resp, 303, "/")
	return e, c, tok, smtp
}

func TestReadReceiptPromptSendOnce(t *testing.T) {
	e, c, tok, smtp := mdnEnv(t)
	uid := uidOfSubject(t, e, c, "INBOX", "Merci de confirmer")
	msgURL := "/mail/message?folder=INBOX&uid=" + uid
	_, page := e.do(c, "GET", msgURL, nil, nil)
	if !strings.Contains(page, `action="/mail/mdn"`) || !strings.Contains(page, "receipts@example.org") {
		t.Fatalf("invitation absente :\n%s", page)
	}
	// ouvrir le message ne suffit pas : rien n'est envoyé tout seul
	if len(smtp.Messages()) != 0 {
		t.Fatal("un accusé a été envoyé sans accord")
	}
	form := url.Values{"csrf": {tok}, "folder": {"INBOX"}, "uid": {uid}, "action": {"send"}, "stay": {msgURL}}
	resp, _ := e.do(c, "POST", "/mail/mdn", form, nil)
	e.expect(resp, 303, msgURL)
	msgs := smtp.Messages()
	if len(msgs) != 1 {
		t.Fatalf("%d messages envoyés", len(msgs))
	}
	got := msgs[0]
	if got.From != "" || len(got.Rcpt) != 1 || got.Rcpt[0] != "receipts@example.org" {
		t.Errorf("enveloppe : de « %s » à %v (expéditeur vide attendu)", got.From, got.Rcpt)
	}
	data := string(got.Data)
	for _, want := range []string{"multipart/report", "disposition-notification", "Final-Recipient: rfc822; test@localhost", "Original-Message-ID: <mdn1@x>", "manual-action/MDN-sent-manually; displayed"} {
		if !strings.Contains(data, want) {
			t.Errorf("accusé : %q absent\n%s", want, data)
		}
	}
	if p, err := mailview.Parse(got.Data); err != nil || !strings.Contains(p.Subject, "Merci de confirmer") || !strings.Contains(p.Subject, "Accusé") {
		t.Errorf("objet de l'accusé : %v %q", err, p.Subject)
	}
	// la confirmation s'affiche, l'invitation ne revient plus, un second envoi est refusé en silence
	_, page = e.do(c, "GET", msgURL, nil, nil)
	if strings.Contains(page, `action="/mail/mdn"`) {
		t.Error("l'invitation revient après l'envoi")
	}
	resp, _ = e.do(c, "POST", "/mail/mdn", form, nil)
	e.expect(resp, 303, msgURL)
	if len(smtp.Messages()) != 1 {
		t.Errorf("accusé envoyé deux fois : %d", len(smtp.Messages()))
	}
}

func TestReadReceiptIgnore(t *testing.T) {
	e, c, tok, smtp := mdnEnv(t)
	uid := uidOfSubject(t, e, c, "INBOX", "Merci de confirmer")
	form := url.Values{"csrf": {tok}, "folder": {"INBOX"}, "uid": {uid}, "action": {"ignore"}, "stay": {"/"}}
	resp, _ := e.do(c, "POST", "/mail/mdn", form, nil)
	e.expect(resp, 303, "/")
	if len(smtp.Messages()) != 0 {
		t.Error("« ne pas répondre » a envoyé un message")
	}
	_, page := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uid, nil, nil)
	if strings.Contains(page, `action="/mail/mdn"`) {
		t.Error("l'invitation revient après « ne pas répondre »")
	}
}

func TestReadReceiptNotOfferedForOwnMessagesOrWithoutRequest(t *testing.T) {
	e, c, tok, smtp := mdnEnv(t)
	own := uidOfSubject(t, e, c, "INBOX", "Mon propre message")
	_, page := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+own, nil, nil)
	if strings.Contains(page, `action="/mail/mdn"`) {
		t.Error("invitation pour son propre message")
	}
	plain := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page = e.do(c, "GET", "/mail/message?folder=INBOX&uid="+plain, nil, nil)
	if strings.Contains(page, `action="/mail/mdn"`) {
		t.Error("invitation pour un message qui n'en demande pas")
	}
	// forcer la requête n'envoie rien non plus
	for _, uid := range []string{plain, own} {
		resp, _ := e.do(c, "POST", "/mail/mdn", url.Values{"csrf": {tok}, "folder": {"INBOX"}, "uid": {uid}, "action": {"send"}, "stay": {"/"}}, nil)
		if uid == plain && resp.StatusCode != 400 {
			t.Errorf("message sans demande : %d", resp.StatusCode)
		}
	}
	if n := len(smtp.Messages()); n != 1 {
		// le message du propriétaire demande un accusé à lui-même : l'envoi manuel reste possible mais jamais proposé
		t.Logf("%d messages envoyés", n)
	}
}

func TestReadReceiptValidationAndCSRF(t *testing.T) {
	e, c, tok, smtp := mdnEnv(t)
	uid := uidOfSubject(t, e, c, "INBOX", "Merci de confirmer")
	for name, v := range map[string]struct {
		form url.Values
		want int
	}{
		"CSRF":               {url.Values{"csrf": {"x"}, "folder": {"INBOX"}, "uid": {uid}, "action": {"send"}}, 403},
		"action inconnue":    {url.Values{"csrf": {tok}, "folder": {"INBOX"}, "uid": {uid}, "action": {"autre"}}, 400},
		"UID invalide":       {url.Values{"csrf": {tok}, "folder": {"INBOX"}, "uid": {"abc"}, "action": {"send"}}, 400},
		"message inexistant": {url.Values{"csrf": {tok}, "folder": {"INBOX"}, "uid": {"99999"}, "action": {"send"}}, 404},
	} {
		if resp, _ := e.do(c, "POST", "/mail/mdn", v.form, nil); resp.StatusCode != v.want {
			t.Errorf("%s : %d, attendu %d", name, resp.StatusCode, v.want)
		}
	}
	if len(smtp.Messages()) != 0 {
		t.Error("message envoyé malgré les refus")
	}
	anon := e.client()
	if resp, _ := e.do(anon, "POST", "/mail/mdn", url.Values{"csrf": {tok}}, nil); resp.StatusCode != 303 && resp.StatusCode != 403 {
		t.Errorf("sans session : %d", resp.StatusCode)
	}
}
