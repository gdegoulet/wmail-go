package web

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
)

// printableMessage : message complet (De, À, Cc, Cci, Répondre à, objet, deux pièces jointes, corps HTML avec liens).
func printableMessage() []byte {
	return []byte("From: Bérénice <berenice@exemple.fr>\r\nTo: test@localhost\r\nCc: Camille <camille@exemple.fr>\r\n" +
		"Bcc: discret@exemple.fr\r\nReply-To: Service <service@exemple.fr>\r\nSubject: =?utf-8?q?R=C3=A9union_de_rentr=C3=A9e?=\r\n" +
		"Date: Mon, 02 Mar 2026 10:00:00 +0000\r\nMessage-ID: <imp@test>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"B\"\r\n\r\n" +
		"--B\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" +
		"<div style=\"background:#123456;color:#fff\">Bonjour, voir <a href=\"https://exemple.fr/ordre-du-jour\">l'ordre du jour</a>.</div>\r\n" +
		"--B\r\nContent-Type: text/plain; name=\"ordre.txt\"\r\nContent-Disposition: attachment; filename=\"ordre.txt\"\r\n\r\n" +
		"ordre du jour\r\n" +
		"--B\r\nContent-Type: application/pdf; name=\"plan.pdf\"\r\nContent-Disposition: attachment; filename=\"plan.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		"JVBERi0xLjQK\r\n--B--\r\n")
}

// emptyMessage : message sans partie affichable (ni texte ni HTML) et sans objet.
func emptyMessage() []byte {
	return []byte("From: Vide <vide@exemple.fr>\r\nTo: test@localhost\r\nSubject: \r\n" +
		"Date: Mon, 02 Mar 2026 11:00:00 +0000\r\nMessage-ID: <vide@test>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: application/octet-stream\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAA\r\n")
}

// plainMessage : message en texte brut, avec citations imbriquées et une adresse web nue (impression sur deux pages).
func plainMessage() []byte {
	var body strings.Builder
	body.WriteString("Bonjour,\n\nvoici le compte rendu : https://exemple.fr/compte-rendu-de-la-reunion-de-rentree\n\n")
	for i := 1; i <= 60; i++ {
		body.WriteString("Point " + strconv.Itoa(i) + " : une ligne de compte rendu assez longue pour éprouver le retour à la ligne à l'impression.\n")
	}
	body.WriteString("\n> citation de premier niveau\n>> citation de deuxième niveau\n> retour au premier niveau\n\nBien à vous.\n")
	return []byte("From: Rapporteur <rapport@exemple.fr>\r\nTo: test@localhost\r\nSubject: Compte rendu\r\n" +
		"Date: Mon, 02 Mar 2026 12:00:00 +0000\r\nMessage-ID: <plat@test>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" + strings.ReplaceAll(body.String(), "\n", "\r\n"))
}

// printEnv ouvre une session dont la boîte contient les messages ci-dessus.
func printEnv(t *testing.T) (*env, *http.Client) {
	t.Helper()
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add(printableMessage(), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
		add(emptyMessage(), time.Date(2026, 3, 2, 11, 0, 0, 0, time.UTC))
		add(plainMessage(), time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC))
	})
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: mailtest.StartSMTP(t, "none")}, "save"), nil))
	return e, c
}

var printScriptRe = regexp.MustCompile(`<script[^>]*\bsrc="([^"]*)"|<script(?:\s[^>]*)?>`)

func TestPrintPageHeadersAndAttachments(t *testing.T) {
	e, c := printEnv(t)
	_, page := e.do(c, "GET", "/", nil, nil)
	uid := uidOf(linkTo(t, page, "Réunion de rentrée"))

	resp, pr := e.do(c, "GET", "/mail/print?folder=INBOX&uid="+uid, nil, nil)
	e.expect(resp, 200, "")
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("type de contenu %q", ct)
	}
	// En-tête complet : objet, expéditeur, destinataires, Cc, Cci, Répondre à, date.
	for _, want := range []string{
		">Objet</th>", ">De</th>", ">À</th>", ">Cc</th>", ">Cci</th>", ">Répondre à</th>", ">Date</th>",
		"Réunion de rentrée", "berenice@exemple.fr", "test@localhost", "camille@exemple.fr",
		"discret@exemple.fr", "service@exemple.fr", "02/03/2026",
	} {
		if !strings.Contains(pr, want) {
			t.Errorf("%q absent de la version imprimable :\n%s", want, pr)
		}
	}
	// Liste des pièces jointes, avec leur taille.
	if !strings.Contains(pr, ">Pièces jointes</th>") || !strings.Contains(pr, `<ul class="att">`) {
		t.Errorf("liste des pièces jointes absente :\n%s", pr)
	}
	for _, want := range []string{"ordre.txt", "plan.pdf"} {
		if !strings.Contains(pr, "<li>"+want+" <span class=\"size\">(") {
			t.Errorf("pièce jointe %q absente de la liste :\n%s", want, pr)
		}
	}
	// Corps assaini, feuille de style d'impression, aucune barre d'outils de l'application.
	for _, want := range []string{`<div class="body">`, `href="/static/print.css?v=`, "ordre du jour"} {
		if !strings.Contains(pr, want) {
			t.Errorf("%q absent de la version imprimable", want)
		}
	}
	for _, bad := range []string{"msgview", "mh-tools", "cp-toolbar", "<nav", "/static/mail.css", "/static/list.js"} {
		if strings.Contains(pr, bad) {
			t.Errorf("%q présent : la page d'impression ne doit porter aucune barre d'outils", bad)
		}
	}
	// Aucun script sauf celui de l'impression (ni script en ligne, ni script venu du message).
	for _, m := range printScriptRe.FindAllStringSubmatch(pr, -1) {
		if !strings.HasPrefix(m[1], "/static/print.js?v=") {
			t.Errorf("script inattendu %q dans la version imprimable", m[0])
		}
	}
	if n := strings.Count(pr, "<script"); n != 1 {
		t.Errorf("%d balises script, attendu 1", n)
	}
}

func TestPrintPageWithoutBody(t *testing.T) {
	e, c := printEnv(t)
	_, page := e.do(c, "GET", "/", nil, nil)
	uid := uidOf(linkTo(t, page, "(Pas d'objet)"))

	resp, pr := e.do(c, "GET", "/mail/print?folder=INBOX&uid="+uid, nil, nil)
	e.expect(resp, 200, "")
	// Ni panique ni page blanche : l'absence de contenu est écrite, les en-têtes restent là.
	for _, want := range []string{`<div class="body">`, `class="nobody"`, "vide@exemple.fr", ">De</th>", "(Pas d&#39;objet)"} {
		if !strings.Contains(pr, want) {
			t.Errorf("%q absent de la version imprimable d'un message sans corps :\n%s", want, pr)
		}
	}
	if strings.Count(pr, "<script") != 1 || !strings.Contains(pr, `src="/static/print.js?v=`) {
		t.Errorf("scripts de la page : attendu le seul script d'impression :\n%s", pr)
	}
}

func TestPrintPagePlainTextKeepsLayout(t *testing.T) {
	e, c := printEnv(t)
	_, page := e.do(c, "GET", "/", nil, nil)
	uid := uidOf(linkTo(t, page, "Compte rendu"))

	resp, pr := e.do(c, "GET", "/mail/print?folder=INBOX&uid="+uid, nil, nil)
	e.expect(resp, 200, "")
	// Le texte brut garde sa mise en page (print.css applique « white-space: pre-wrap » à div.plain) et ses citations.
	for _, want := range []string{`<div class="plain">`, "<blockquote>", "Point 60", `<a href="https://exemple.fr/compte-rendu-de-la-reunion-de-rentree"`} {
		if !strings.Contains(pr, want) {
			t.Errorf("%q absent de la version imprimable en texte brut", want)
		}
	}
	if strings.Count(pr, "<script") != 1 {
		t.Errorf("%d balises script, attendu 1", strings.Count(pr, "<script"))
	}
}
