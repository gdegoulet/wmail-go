package web

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
)

// entête commune aux messages de ce fichier.
func entete(sujet, id, ct string) string {
	return "From: message <admin@clientes.partidasya.test>\r\nTo: test@localhost\r\nSubject: " + sujet +
		"\r\nDate: Sun, 25 Oct 2026 21:28:41 +0000\r\nMessage-ID: <" + id + "@test>\r\nMIME-Version: 1.0\r\nContent-Type: " + ct + "\r\n\r\n"
}

// Le bandeau doit apparaître sur un faux avis de renouvellement — authentifié, sans marque citée, mais
// qui traque ses lecteurs sans offrir de désabonnement — et rester absent d'une lettre d'information
// ordinaire. Le second cas est le plus important : un bandeau qui se trompe serait vite ignoré.
func TestBandeauAnomalies(t *testing.T) {
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add([]byte(entete("Adhesion enregistree", "phish", "text/html; charset=UTF-8")+
			`<img src="https://d1v2u3h27yevyl.cloudfront.test" width="520" alt="">`+
			`<p>Votre acces est prolonge d'un an.</p>`+
			`<a href="https://ct.sendgrid.test/ls/click?upn=u001.NDNg2YlskgvjysBN2fDQJUOp2FqDdbFH7MwSj7RX3Ro29UOU9J8">Consulter mon espace</a>`+
			`<img src="https://ct.sendgrid.test/wf/open?upn=u001.qFYK1fC4YXrXkBDjyAT0REHAO7hPEBAatVckpCuxCz2FNHJD">`+"\r\n"),
			time.Date(2026, 10, 25, 21, 28, 41, 0, time.UTC))

		corps := "--f\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nNos nouveautes\r\n--f\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n\r\n<p>Nos nouveautes</p>" +
			`<img src="https://img.example.test/entete/banniere.png">` +
			`<a href="https://trk.example.test/c?id=01234567890123456789012345678901234567890123">Voir en ligne</a>` +
			"\r\n--f--\r\n"
		add([]byte("From: Example <news@example.test>\r\nTo: test@localhost\r\nSubject: Nouveautes\r\n"+
			"Date: Sun, 25 Oct 2026 21:00:00 +0000\r\nMessage-ID: <news@test>\r\nMIME-Version: 1.0\r\n"+
			"List-Unsubscribe: <https://example.test/unsub>\r\n"+
			"Content-Type: multipart/alternative; boundary=f\r\n\r\n"+corps),
			time.Date(2026, 10, 25, 21, 0, 0, 0, time.UTC))
	})
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: mailtest.StartSMTP(t, "none")}, "save"), nil))

	// 1. le faux avis : bandeau, motifs, et bouton vers les indésirables.
	_, page := e.do(c, "GET", "/?q=Adhesion&scope=subject", nil, nil)
	_, msg := e.do(c, "GET", linkTo(t, page, "Adhesion enregistree"), nil, nil)
	if !strings.Contains(msg, "suspbar") {
		t.Fatalf("bandeau d'anomalies absent :\n%s", msg)
	}
	for _, attendu := range []string{
		"Ce message présente des anomalies techniques",
		"aucun lien de désabonnement", // apostrophes échappées par html/template : on n'en cherche pas
		"sans version texte",
		"domaine nu",
	} {
		if !strings.Contains(msg, attendu) {
			t.Errorf("motif %q absent du bandeau", attendu)
		}
	}
	if !strings.Contains(msg, `value="spam"`) || !strings.Contains(msg, "Déplacer vers les indésirables") {
		t.Error("bouton « déplacer vers les indésirables » absent du bandeau")
	}

	// 2. la lettre d'information : rien.
	_, page = e.do(c, "GET", "/?q=Nouveautes&scope=subject", nil, nil)
	_, msg = e.do(c, "GET", linkTo(t, page, "Nouveautes"), nil, nil)
	if strings.Contains(msg, "suspbar") {
		t.Errorf("bandeau affiché à tort sur une lettre d'information légitime :\n%s", msg)
	}
}

// Le bouton du bandeau doit réellement déplacer le message vers les indésirables.
func TestBandeauDeplaceVersIndesirables(t *testing.T) {
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add([]byte(entete("Facture a regler", "ph2", "text/html; charset=UTF-8")+
			`<img src="https://cdn.example.test" alt="">`+
			`<a href="https://trk.test/ls/click?upn=0123456789012345678901234567890123456789012">Payer</a>`+
			`<img src="https://trk.test/wf/open?upn=0123456789012345678901234567890123456789012">`+"\r\n"),
			time.Date(2026, 10, 25, 20, 0, 0, 0, time.UTC))
	})
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: mailtest.StartSMTP(t, "none")}, "save"), nil))

	_, page := e.do(c, "GET", "/?q=Facture&scope=subject", nil, nil)
	link := linkTo(t, page, "Facture a regler")
	_, msg := e.do(c, "GET", link, nil, nil)
	if !strings.Contains(msg, "suspbar") {
		t.Fatalf("bandeau attendu :\n%s", msg)
	}
	resp, _ := e.do(c, "POST", "/mail/action", url.Values{
		"csrf": {tok}, "action": {"spam"}, "folder": {"INBOX"}, "uid": {uidOf(link)}, "back": {"/"},
	}, nil)
	if resp.StatusCode != 303 {
		t.Fatalf("statut %d, attendu 303", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/?q=Facture&scope=subject", nil, nil)
	if strings.Contains(page, "Facture a regler") {
		t.Error("le message est resté dans la boîte de réception")
	}
}
