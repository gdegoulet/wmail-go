package web

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/config"
	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
)

func TestICSAttachmentShowsAnEventCard(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nSUMMARY:Point projet\r\nDTSTART:20261105T140000Z\r\nDTEND:20261105T150000Z\r\nLOCATION:Salle 3\r\nORGANIZER;CN=Anne Durand:mailto:anne@x.org\r\nATTENDEE;CN=Paul:mailto:paul@x.org\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add([]byte("From: Anne <anne@x.org>\r\nTo: test@localhost\r\nSubject: Invitation\r\nDate: Mon, 02 Mar 2026 10:00:00 +0000\r\nMessage-ID: <ics@test>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=B\r\n\r\n--B\r\nContent-Type: text/plain\r\n\r\nvoir l'invitation\r\n"+
			"--B\r\nContent-Type: text/calendar; method=REQUEST; name=invite.ics\r\nContent-Disposition: attachment; filename=invite.ics\r\nContent-Transfer-Encoding: base64\r\n\r\n"+b64lines([]byte(ics))+"\r\n--B--\r\n"), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save"), nil))
	_, page := e.do(c, "GET", "/?q=Invitation&scope=subject", nil, nil)
	link := linkTo(t, page, "Invitation")
	_, frag := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, nil)
	for _, want := range []string{`class="bar evbar"`, "Point projet", "05/11/2026 , 15:00", "Anne Durand", "Paul", "https://www.google.com/calendar/render?", "https://outlook.office.com/calendar/0/deeplink/compose?", `rel="noopener noreferrer"`, "invite.ics"} {
		if !strings.Contains(frag, want) {
			t.Errorf("%q absent de la fiche d'invitation", want)
		}
	}
	if strings.Index(frag, `class="bar evbar"`) > strings.Index(frag, `class="bar attbar"`) {
		t.Error("la carte de l'événement précède la liste des pièces jointes")
	}
}

func TestXOriginatingIPHeaderIsOptIn(t *testing.T) {
	for _, on := range []bool{false, true} {
		e, c, tok, smtp := composeEnv(t, func(cfg *config.Config) { cfg.Compose.XOriginatingIP = on })
		_, page := e.do(c, "GET", "/compose", nil, nil)
		f := sendForm(tok, composeID(t, page), nil)
		f.Set("identity", identityID(t, page))
		resp, _ := e.do(c, "POST", "/compose/send", f, nil)
		e.expect(resp, 303, "/")
		msgs := smtp.Messages()
		if len(msgs) != 1 {
			t.Fatalf("%d messages envoyés", len(msgs))
		}
		has := strings.Contains(string(msgs[0].Data), "X-Originating-Ip: 127.0.0.1") || strings.Contains(string(msgs[0].Data), "X-Originating-IP: 127.0.0.1")
		if has != on {
			t.Errorf("option=%v : en-tête présent=%v", on, has)
		}
	}
}
