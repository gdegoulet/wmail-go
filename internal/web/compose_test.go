package web

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gdegoulet/wmail-go/internal/config"
	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
	"github.com/gdegoulet/wmail-go/internal/mailview"
)

// composeEnv ouvre une session reliée à un IMAP de test et à un SMTP de test qui garde les messages envoyés.
func composeEnv(t *testing.T, mutate func(*config.Config)) (*env, *http.Client, string, *mailtest.SMTPServer) {
	return composeEnvWith(t, mutate, nil)
}

// composeEnvWith ajoute des champs au formulaire du compte mail (signature, dossiers…).
func composeEnvWith(t *testing.T, mutate func(*config.Config), extra url.Values) (*env, *http.Client, string, *mailtest.SMTPServer) {
	t.Helper()
	e := newEnv(t, mutate)
	smtp := mailtest.StartSMTPCapture(t, "none")
	ports := mailPorts{imap: mailtest.StartIMAP(t, "none"), smtp: smtp.Port}
	c, tok := e.loggedIn("alice")
	form := accountForm(tok, ports, "save")
	for k, v := range extra {
		form[k] = v
	}
	resp, _ := e.do(c, "POST", "/accounts/save", form, nil)
	e.expect(resp, 303, "/")
	return e, c, tok, smtp
}

var composeIDRe = regexp.MustCompile(`name="compose_id" value="([^"]+)"`)

func composeID(t *testing.T, page string) string {
	t.Helper()
	m := composeIDRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("compose_id absent :\n%s", page)
	}
	return m[1]
}

func fieldValue(page, name string) string {
	m := regexp.MustCompile(`name="` + name + `"[^>]*?value="([^"]*)"`).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return htmlUnescape(m[1])
}

func textareaValue(page string) string {
	m := regexp.MustCompile(`(?s)<textarea[^>]*name="body"[^>]*>(.*?)</textarea>`).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return htmlUnescape(m[1])
}

// uidOfSubject renvoie l'UID d'un message de INBOX d'après son sujet.
func uidOfSubject(t *testing.T, e *env, c *http.Client, folder, subject string) string {
	t.Helper()
	_, page := e.do(c, "GET", "/?folder="+url.QueryEscape(folder)+"&size=50", nil, nil)
	return uidOf(linkTo(t, page, subject))
}

func sendForm(tok, id string, extra url.Values) url.Values {
	f := url.Values{"csrf": {tok}, "compose_id": {id}, "identity": {""}, "to": {"dest@example.org"}, "subject": {"Bonjour"}, "body": {"Texte du message"}, "back": {"/"}}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

func identityID(t *testing.T, page string) string {
	t.Helper()
	m := regexp.MustCompile(`<option value="([^"]+)" data-signature="[^"]*" data-signature-html="[^"]*" selected>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("identité absente :\n%s", page)
	}
	return m[1]
}

func postFile(t *testing.T, e *env, c *http.Client, tok, id, name string, data []byte) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("csrf", tok)
	_ = mw.WriteField("compose_id", id)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", e.ts.URL+"/compose/attach", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return resp, out
}

func TestComposeNewPage(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	resp, page := e.do(c, "GET", "/compose", nil, nil)
	e.expect(resp, 200, "")
	for _, want := range []string{`action="/compose/send"`, `name="to"`, `name="subject"`, `name="body"`, `name="identity"`, `test@localhost`, `/static/compose.js`} {
		if !strings.Contains(page, want) {
			t.Errorf("élément absent : %s", want)
		}
	}
	if strings.Contains(page, `class="topbar"`) {
		t.Error("la barre de navigation ne doit pas apparaître dans la composition")
	}
}

func TestComposeRequiresLoginAndCSRF(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	anon := e.client()
	for _, p := range []string{"/compose", "/compose?mode=reply&folder=INBOX&uid=1"} {
		resp, _ := e.do(anon, "GET", p, nil, nil)
		e.expect(resp, 303, "/login")
	}
	_, page := e.do(c, "GET", "/compose", nil, nil)
	f := sendForm("mauvais", composeID(t, page), url.Values{"identity": {identityID(t, page)}})
	resp, _ := e.do(c, "POST", "/compose/send", f, nil)
	if resp.StatusCode != 403 {
		t.Errorf("sans jeton CSRF valide : %d", resp.StatusCode)
	}
	_ = tok
}

func TestComposeReplyPrefill(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	resp, page := e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+uid+"&back=%2F", nil, nil)
	e.expect(resp, 200, "")
	if fieldValue(page, "to") != "\"Alice\" <alice@x.fr>" && !strings.Contains(fieldValue(page, "to"), "alice@x.fr") {
		t.Errorf("destinataire = %q", fieldValue(page, "to"))
	}
	if got := fieldValue(page, "subject"); got != "Re: Facture octobre" {
		t.Errorf("objet = %q", got)
	}
	body := textareaValue(page)
	if !strings.Contains(body, "> montant 100") || !strings.Contains(body, "Alice") || !strings.Contains(body, "a écrit") {
		t.Errorf("citation :\n%s", body)
	}
	if fieldValue(page, "kind") != "reply" || fieldValue(page, "in_reply_to") != "m1@test" || fieldValue(page, "orig_uid") != uid {
		t.Errorf("lien avec l'original : kind=%q in_reply_to=%q uid=%q", fieldValue(page, "kind"), fieldValue(page, "in_reply_to"), fieldValue(page, "orig_uid"))
	}
	// la lecture pour répondre ne marque pas le message comme lu : il est resté non lu dans la liste
	_, list := e.do(c, "GET", "/?folder=INBOX&size=50", nil, nil)
	if !strings.Contains(list, `class="msgrow unread`) {
		t.Error("la liste devrait garder des messages non lus")
	}
}

func TestComposeForwardCopiesAttachmentsAndForwardAsAttachment(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")

	_, page := e.do(c, "GET", "/compose?mode=forward&folder=INBOX&uid="+uid, nil, nil)
	if got := fieldValue(page, "subject"); got != "Fwd: Facture octobre" {
		t.Errorf("objet = %q", got)
	}
	if !strings.Contains(textareaValue(page), "Message transféré") || !strings.Contains(page, "facture.pdf") {
		t.Errorf("transfert : pièce jointe ou en-tête absent :\n%s", page)
	}
	id := composeID(t, page)
	f := sendForm(tok, id, url.Values{"identity": {identityID(t, page)}, "kind": {"forward"}, "orig_folder": {"INBOX"}, "orig_uid": {uid}})
	resp, _ := e.do(c, "POST", "/compose/send", f, nil)
	e.expect(resp, 303, "/")
	msgs := smtp.Messages()
	if len(msgs) != 1 {
		t.Fatalf("%d messages envoyés", len(msgs))
	}
	m, err := mailview.Parse(msgs[0].Data)
	if err != nil || len(m.Attachments) != 1 || m.Attachments[0].Name != "facture.pdf" {
		t.Errorf("pièce jointe transférée : %+v %v", m.Attachments, err)
	}

	_, page = e.do(c, "GET", "/compose?mode=forwardatt&folder=INBOX&uid="+uid, nil, nil)
	if !strings.Contains(page, "Facture octobre.eml") {
		t.Errorf("message joint en .eml absent :\n%s", page)
	}
	id = composeID(t, page)
	f = sendForm(tok, id, url.Values{"identity": {identityID(t, page)}})
	resp, _ = e.do(c, "POST", "/compose/send", f, nil)
	e.expect(resp, 303, "/")
	m, err = mailview.Parse(smtp.Messages()[1].Data)
	if err != nil || len(m.Attachments) != 1 || m.Attachments[0].ContentType != "message/rfc822" {
		t.Errorf("message joint : %+v %v", m.Attachments, err)
	}
	_, _, ok := m.Attachment(0)
	if !ok {
		t.Error("pièce jointe illisible")
	}
}

func TestComposeSendCopiesToSentAndMarksAnswered(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page := e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+uid, nil, nil)
	id, ident := composeID(t, page), identityID(t, page)

	f := sendForm(tok, id, url.Values{
		"identity": {ident}, "to": {"Alice <alice@x.fr>"}, "cc": {"cc@example.org"}, "bcc": {"secret@example.org"},
		"subject": {"Re: Facture octobre"}, "body": {"Merci, c'est réglé.\n"},
		"kind": {"reply"}, "orig_folder": {"INBOX"}, "orig_uid": {uid}, "in_reply_to": {"m1@test"}, "references": {"m1@test"},
		"receipt": {"on"}, "important": {"on"},
	})
	resp, _ := e.do(c, "POST", "/compose/send", f, nil)
	e.expect(resp, 303, "/")
	// message d'information affiché une seule fois sur la page suivante
	_, next := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(next, "Message envoyé.") {
		t.Error("confirmation d'envoi absente")
	}
	_, again := e.do(c, "GET", "/", nil, nil)
	if strings.Contains(again, "Message envoyé.") {
		t.Error("la confirmation ne doit s'afficher qu'une fois")
	}

	msgs := smtp.Messages()
	if len(msgs) != 1 {
		t.Fatalf("%d messages envoyés", len(msgs))
	}
	got := msgs[0]
	if got.From != "test@localhost" || len(got.Rcpt) != 3 {
		t.Errorf("enveloppe : %+v", got)
	}
	data := string(got.Data)
	if strings.Contains(data, "secret@example.org") || strings.Contains(strings.ToLower(data), "\r\nbcc:") {
		t.Errorf("le Cci apparaît dans le message envoyé :\n%s", data)
	}
	for _, want := range []string{"In-Reply-To: <m1@test>", "References: <m1@test>", "Importance: High", "Disposition-Notification-To:", "X-Mailer: wmail-go", "X-Wmail-Draft-Info: reply"} {
		if !strings.Contains(data, want) {
			t.Errorf("en-tête manquant : %s", want)
		}
	}
	// copie dans « Envoyés » (dossier créé au besoin) : avec le Cci, lue
	_, sent := e.do(c, "GET", "/?folder=Sent&size=50", nil, nil)
	subs, links := rows(sent)
	if len(subs) != 1 || subs[0] != "Re: Facture octobre" {
		t.Fatalf("dossier Envoyés : %v", subs)
	}
	_, view := e.do(c, "GET", "/mail/body?folder=Sent&uid="+uidOf(links[0]), nil, nil)
	if !strings.Contains(view, "Merci, c") {
		t.Errorf("copie illisible :\n%s", view)
	}
	// le message d'origine est marqué « répondu »
	_, list := e.do(c, "GET", "/?folder=INBOX&size=50", nil, nil)
	row := regexp.MustCompile(`<li id="m` + uid + `" draggable="true" class="([^"]*)"`).FindStringSubmatch(list)
	if row == nil || !strings.Contains(row[1], "answered") {
		t.Errorf("message d'origine non marqué répondu : %v", row)
	}
}

func TestComposeSendValidation(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id, ident := composeID(t, page), identityID(t, page)
	cases := []struct {
		name  string
		extra url.Values
		want  int
		text  string
	}{
		{"sans destinataire", url.Values{"to": {""}, "cc": {""}}, 400, "au moins un destinataire"},
		{"adresse invalide", url.Values{"to": {"pas une adresse"}}, 400, "Adresse invalide"},
		{"injection d'en-tête", url.Values{"to": {"a@b.fr\r\nBcc: pirate@evil.example"}}, 400, "Adresse invalide"},
		{"identité étrangère", url.Values{"identity": {"autre"}}, 400, "Adresse invalide"},
		{"destinataire refusé", url.Values{"to": {"refuse@example.org"}}, 502, "refusé"},
	}
	for _, tc := range cases {
		extra := url.Values{"identity": {ident}}
		for k, v := range tc.extra {
			extra[k] = v
		}
		resp, body := e.do(c, "POST", "/compose/send", sendForm(tok, id, extra), nil)
		if resp.StatusCode != tc.want || !strings.Contains(body, tc.text) {
			t.Errorf("%s : statut %d, texte %q attendu\n%s", tc.name, resp.StatusCode, tc.text, body)
		}
		if resp.StatusCode == tc.want && !strings.Contains(body, `name="subject" value="Bonjour"`) {
			t.Errorf("%s : la saisie n'est pas conservée", tc.name)
		}
	}
	if n := len(smtp.Messages()); n != 0 {
		t.Errorf("%d messages envoyés malgré les erreurs", n)
	}
	// l'objet avec un saut de ligne ne crée pas d'en-tête
	resp, _ := e.do(c, "POST", "/compose/send", sendForm(tok, id, url.Values{"identity": {ident}, "subject": {"a\r\nBcc: pirate@evil.example"}}), nil)
	e.expect(resp, 303, "/")
	data := string(smtp.Messages()[0].Data)
	if strings.Contains(strings.ToLower(data), "\r\nbcc:") {
		t.Errorf("injection par l'objet :\n%s", data)
	}
}

func TestComposeDraftSaveReopenAndSend(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id, ident := composeID(t, page), identityID(t, page)
	fetch := map[string]string{"X-Requested-With": "fetch"}

	f := sendForm(tok, id, url.Values{"identity": {ident}, "to": {"dest@example.org"}, "bcc": {"cache@example.org"}, "subject": {"Brouillon d'essai"}, "body": {"à finir"}})
	resp, body := e.do(c, "POST", "/compose/save", f, fetch)
	if resp.StatusCode != 200 {
		t.Fatalf("enregistrement : %d %s", resp.StatusCode, body)
	}
	var saved struct{ Folder, Uid, Saved, Error string }
	_ = json.Unmarshal([]byte(body), &saved)
	if saved.Folder != "Drafts" || saved.Uid == "" || saved.Error != "" {
		t.Fatalf("réponse = %s", body)
	}
	// un second enregistrement remplace le premier
	f.Set("draft_folder", saved.Folder)
	f.Set("draft_uid", saved.Uid)
	f.Set("body", "à finir, version 2")
	resp, body = e.do(c, "POST", "/compose/save", f, fetch)
	_ = json.Unmarshal([]byte(body), &saved)
	_, list := e.do(c, "GET", "/?folder=Drafts&size=50", nil, nil)
	subs, links := rows(list)
	if len(subs) != 1 || subs[0] != "Brouillon d'essai" {
		t.Fatalf("dossier Brouillons après deux enregistrements : %v", subs)
	}

	// le message affiche « Modifier » ; la reprise restaure les champs, y compris le Cci
	_, view := e.do(c, "GET", "/mail/message?folder=Drafts&uid="+uidOf(links[0]), nil, nil)
	if !strings.Contains(view, "mode=draft") {
		t.Error("lien « Modifier » absent de la lecture d'un brouillon")
	}
	_, page = e.do(c, "GET", "/compose?mode=draft&folder=Drafts&uid="+uidOf(links[0]), nil, nil)
	if fieldValue(page, "subject") != "Brouillon d'essai" || !strings.Contains(fieldValue(page, "bcc"), "cache@example.org") || !strings.Contains(textareaValue(page), "version 2") {
		t.Errorf("reprise du brouillon :\n%s", page)
	}
	if fieldValue(page, "draft_uid") != uidOf(links[0]) || fieldValue(page, "draft_folder") != "Drafts" {
		t.Error("le brouillon repris n'est pas lié à la composition")
	}

	// l'envoi supprime le brouillon
	id2 := composeID(t, page)
	f2 := sendForm(tok, id2, url.Values{"identity": {ident}, "draft_folder": {"Drafts"}, "draft_uid": {uidOf(links[0])}})
	resp, _ = e.do(c, "POST", "/compose/send", f2, nil)
	e.expect(resp, 303, "/")
	_, list = e.do(c, "GET", "/?folder=Drafts&size=50", nil, nil)
	if subs, _ := rows(list); len(subs) != 0 {
		t.Errorf("le brouillon devrait avoir disparu : %v", subs)
	}
	if len(smtp.Messages()) != 1 {
		t.Error("message non envoyé")
	}
}

func TestComposeDiscardDeletesDraft(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id, ident := composeID(t, page), identityID(t, page)
	f := sendForm(tok, id, url.Values{"identity": {ident}})
	_, body := e.do(c, "POST", "/compose/save", f, map[string]string{"X-Requested-With": "fetch"})
	var saved struct{ Folder, Uid string }
	_ = json.Unmarshal([]byte(body), &saved)
	resp, _ := e.do(c, "POST", "/compose/discard", url.Values{"csrf": {tok}, "compose_id": {id}, "draft_folder": {saved.Folder}, "draft_uid": {saved.Uid}, "back": {"/"}}, nil)
	e.expect(resp, 303, "/")
	_, list := e.do(c, "GET", "/?folder=Drafts&size=50", nil, nil)
	if subs, _ := rows(list); len(subs) != 0 {
		t.Errorf("brouillon non supprimé : %v", subs)
	}
}

func TestComposeAttachmentsUploadSendAndLimits(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, func(cfg *config.Config) {
		cfg.Compose = config.Compose{MaxAttachmentMB: 1, MaxMessageMB: 2, MaxAttachments: 2, MaxSessionMB: 4}
	})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id, ident := composeID(t, page), identityID(t, page)

	resp, up := postFile(t, e, c, tok, id, "rapport é.txt", []byte("contenu du rapport"))
	if resp.StatusCode != 200 || up["id"] == nil || up["name"] != "rapport é.txt" {
		t.Fatalf("envoi de la pièce jointe : %d %v", resp.StatusCode, up)
	}
	if resp, up = postFile(t, e, c, tok, id, "gros.bin", make([]byte, 1<<20+10)); resp.StatusCode != 413 || up["error"] == nil {
		t.Errorf("fichier trop gros : %d %v", resp.StatusCode, up)
	}
	_, second := postFile(t, e, c, tok, id, "b.txt", []byte("b"))
	if resp, _ = postFile(t, e, c, tok, id, "c.txt", []byte("c")); resp.StatusCode != 413 {
		t.Errorf("trop de pièces jointes : %d", resp.StatusCode)
	}
	// retrait d'une pièce
	resp, _ = e.do(c, "POST", "/compose/detach", url.Values{"csrf": {tok}, "compose_id": {id}, "id": {second["id"].(string)}}, nil)
	e.expect(resp, 200, "")

	resp, _ = e.do(c, "POST", "/compose/send", sendForm(tok, id, url.Values{"identity": {ident}}), nil)
	e.expect(resp, 303, "/")
	m, err := mailview.Parse(smtp.Messages()[0].Data)
	if err != nil || len(m.Attachments) != 1 || m.Attachments[0].Name != "rapport é.txt" {
		t.Fatalf("pièces jointes envoyées : %+v %v", m.Attachments, err)
	}
	if _, data, _ := m.Attachment(0); string(data) != "contenu du rapport" {
		t.Errorf("contenu = %q", data)
	}
	// les pièces sont oubliées après l'envoi
	if n := len(e.srv.attach.List(sessionOf(t, e, c), id)); n != 0 {
		t.Errorf("%d pièces encore en mémoire", n)
	}
}

// sessionOf renvoie l'identifiant de la session du client (cookie).
func sessionOf(t *testing.T, e *env, c *http.Client) string {
	t.Helper()
	u, _ := url.Parse(e.ts.URL)
	for _, ck := range c.Jar.Cookies(u) {
		if ck.Name == "wmail_session" {
			return ck.Value
		}
	}
	t.Fatal("pas de session")
	return ""
}

func TestComposeAttachmentsAreIsolatedBetweenSessions(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id := composeID(t, page)
	_, up := postFile(t, e, c, tok, id, "prive.txt", []byte("secret"))

	other, otherTok := e.loggedIn("bob")
	resp, _ := e.do(other, "POST", "/compose/detach", url.Values{"csrf": {otherTok}, "compose_id": {id}, "id": {up["id"].(string)}}, nil)
	if resp.StatusCode != 404 {
		t.Errorf("un autre utilisateur a pu retirer la pièce : %d", resp.StatusCode)
	}
	if len(e.srv.attach.List(sessionOf(t, e, c), id)) != 1 {
		t.Error("la pièce a disparu")
	}
}

func TestComposeAttachmentsClearedOnLogout(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id := composeID(t, page)
	postFile(t, e, c, tok, id, "x.txt", []byte("x"))
	sid := sessionOf(t, e, c)
	resp, _ := e.do(c, "POST", "/logout", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 303, "")
	time.Sleep(50 * time.Millisecond) // OnClose s'exécute dans une goroutine
	if len(e.srv.attach.List(sid, id)) != 0 {
		t.Error("pièces jointes conservées après la déconnexion")
	}
}

func TestComposeSendRateLimit(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, func(cfg *config.Config) {
		cfg.RateLimit.Send = config.RateRule{PerIP: config.Limit{Count: 100, Window: time.Hour}, PerLogin: config.Limit{Count: 2, Window: time.Hour}}
	})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	ident := identityID(t, page)
	for i := 0; i < 2; i++ {
		resp, _ := e.do(c, "POST", "/compose/send", sendForm(tok, composeID(t, page), url.Values{"identity": {ident}}), nil)
		e.expect(resp, 303, "/")
	}
	resp, _ := e.do(c, "POST", "/compose/send", sendForm(tok, composeID(t, page), url.Values{"identity": {ident}}), nil)
	if resp.StatusCode != 429 || resp.Header.Get("Retry-After") == "" {
		t.Errorf("troisième envoi : %d", resp.StatusCode)
	}
	if len(smtp.Messages()) != 2 {
		t.Errorf("%d messages envoyés", len(smtp.Messages()))
	}
}

func TestComposeBadParameters(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	for _, p := range []string{
		"/compose?mode=hack", "/compose?mode=reply", "/compose?mode=reply&folder=INBOX&uid=abc",
		"/compose?mode=reply&folder=..%2Fx&uid=1", "/compose?mode=reply&folder=INBOX&uid=99999",
	} {
		resp, _ := e.do(c, "GET", p, nil, nil)
		if resp.StatusCode < 400 {
			t.Errorf("%s : statut %d", p, resp.StatusCode)
		}
	}
	// le retour est limité aux pages de l'application
	_, page := e.do(c, "GET", "/compose?back=https%3A%2F%2Fevil.example%2F", nil, nil)
	if fieldValue(page, "back") != "/" {
		t.Errorf("back = %q", fieldValue(page, "back"))
	}
}

func TestMessageViewHasReplyMenu(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uid, nil, nil)
	for _, mode := range []string{"reply", "replyall", "forward", "forwardatt", "editasnew"} {
		if !strings.Contains(page, "mode="+mode) {
			t.Errorf("lien %s absent", mode)
		}
	}
	_, list := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(list, `href="/compose?back=`) {
		t.Error("bouton « Nouveau » sans lien")
	}
}

func TestComposeSignature(t *testing.T) {
	e, c, _, _ := composeEnvWith(t, nil, url.Values{"signature": {"Gérard\r\nSDV – {{FROM}}"}})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	body := textareaValue(page)
	if !strings.Contains(body, "-- \nGérard\nSDV –\n") {
		t.Errorf("signature d'un nouveau message :\n%q", body)
	}
	if !strings.Contains(page, `data-signature="Gérard`) {
		t.Error("signature absente du menu des identités")
	}
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page = e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+uid, nil, nil)
	body = textareaValue(page)
	iq, is := strings.Index(body, "> montant 100"), strings.Index(body, "-- \nGérard\nSDV – Alice")
	if iq < 0 || is < 0 || is < iq {
		t.Errorf("signature après la citation, variable {{FROM}} remplacée :\n%q", body)
	}
}

func TestComposeSignatureBeforeQuote(t *testing.T) {
	e, c, _, _ := composeEnvWith(t, nil, url.Values{"signature": {"Cordialement"}, "signature_before": {"on"}})
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page := e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+uid, nil, nil)
	body := textareaValue(page)
	if is, iq := strings.Index(body, "-- \nCordialement"), strings.Index(body, "> montant 100"); is < 0 || iq < 0 || is > iq {
		t.Errorf("signature avant la citation :\n%q", body)
	}
}

func TestComposeNoSentCopy(t *testing.T) {
	e, c, tok, smtp := composeEnvWith(t, nil, url.Values{"sent_folder": {"-"}})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	if !strings.Contains(page, "Aucune copie ne sera conservée") {
		t.Error("avertissement absent")
	}
	resp, _ := e.do(c, "POST", "/compose/send", sendForm(tok, composeID(t, page), url.Values{"identity": {identityID(t, page)}}), nil)
	e.expect(resp, 303, "/")
	if len(smtp.Messages()) != 1 {
		t.Fatal("message non envoyé")
	}
	_, list := e.do(c, "GET", "/?folder=Sent&size=50", nil, nil)
	if subs, _ := rows(list); len(subs) != 0 {
		t.Errorf("une copie a été conservée : %v", subs)
	}
}

func TestComposeNoDraftsFolder(t *testing.T) {
	e, c, tok, _ := composeEnvWith(t, nil, url.Values{"drafts_folder": {"-"}})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	if strings.Contains(page, `id="cp-save"`) {
		t.Error("le bouton Enregistrer ne doit pas apparaître sans dossier de brouillons")
	}
	resp, body := e.do(c, "POST", "/compose/save", sendForm(tok, composeID(t, page), url.Values{"identity": {identityID(t, page)}}), map[string]string{"X-Requested-With": "fetch"})
	if resp.StatusCode != 400 || !strings.Contains(body, "brouillons") {
		t.Errorf("enregistrement sans dossier : %d %s", resp.StatusCode, body)
	}
}

func TestEditAccountKeepsIdentitiesAndComposePrefs(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(page, `name="draft_autosave"`) || !strings.Contains(page, `name="reply_same_folder"`) {
		t.Fatal("réglages de composition absents")
	}
	prefs := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"}, "page_size": {"50"}, "idle_minutes": {"60"}, "absolute_hours": {"12"}, "reply_same_folder": {"on"}, "mark_read": {"on"}}
	resp, _ := e.do(c, "POST", "/settings/preferences", prefs, nil)
	if resp.StatusCode != 303 && resp.StatusCode != 200 {
		t.Fatalf("préférences : %d", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/compose", nil, nil)
	if strings.Contains(page, `data-autosave="1"`) {
		t.Error("l'enregistrement automatique devrait être désactivé")
	}
}

func editorHTML(page string) string {
	m := regexp.MustCompile(`(?s)<div id="cp-editor"[^>]*>(.*?)</div>\s*<textarea`).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return m[1]
}

func paneMode(page string) string {
	m := regexp.MustCompile(`id="cp-pane-text" data-mode="(\w+)"`).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return m[1]
}

func setEditor(t *testing.T, e *env, c *http.Client, tok, editor string) {
	t.Helper()
	prefs := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"}, "page_size": {"50"}, "idle_minutes": {"60"}, "absolute_hours": {"12"}, "mark_read": {"on"}, "draft_autosave": {"on"}, "editor": {editor}}
	if resp, _ := e.do(c, "POST", "/settings/preferences", prefs, nil); resp.StatusCode != 200 && resp.StatusCode != 303 {
		t.Fatalf("préférences : %d", resp.StatusCode)
	}
}

func TestComposeEditorModeFollowsSettingAndOriginal(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	plainUID := uidOfSubject(t, e, c, "INBOX", "Facture octobre") // texte brut
	htmlUID := uidOfSubject(t, e, c, "INBOX", "Réunion lundi")    // HTML

	// par défaut : HTML pour un nouveau message et pour la réponse à un message HTML, texte brut pour un message texte
	_, page := e.do(c, "GET", "/compose", nil, nil)
	if paneMode(page) != "html" || !strings.Contains(page, `id="cp-mode"`) {
		t.Errorf("nouveau message : mode %q", paneMode(page))
	}
	_, page = e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+plainUID, nil, nil)
	if paneMode(page) != "plain" {
		t.Errorf("réponse à un message texte : mode %q", paneMode(page))
	}
	_, page = e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+htmlUID, nil, nil)
	if paneMode(page) != "html" {
		t.Errorf("réponse à un message HTML : mode %q", paneMode(page))
	}
	for setting, want := range map[string][2]string{
		"plain": {"plain", "new"}, "plain_forced": {"plain", "forced"}, "html_forced": {"html", "forced"},
	} {
		setEditor(t, e, c, tok, setting)
		_, page = e.do(c, "GET", "/compose", nil, nil)
		if paneMode(page) != want[0] {
			t.Errorf("réglage %s : mode %q", setting, paneMode(page))
		}
		if forced := !strings.Contains(page, `id="cp-mode"`); forced != (want[1] == "forced") {
			t.Errorf("réglage %s : bouton de bascule %v", setting, !forced)
		}
	}
	// html_forced : même la réponse à un message texte s'ouvre en HTML
	setEditor(t, e, c, tok, "html_forced")
	_, page = e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+plainUID, nil, nil)
	if paneMode(page) != "html" || !strings.Contains(editorHTML(page), "montant 100") {
		t.Errorf("html_forced sur un message texte : %q", paneMode(page))
	}
	// valeur inconnue ignorée
	setEditor(t, e, c, tok, "n'importe quoi")
	_, page = e.do(c, "GET", "/compose", nil, nil)
	if paneMode(page) != "html" {
		t.Errorf("valeur inconnue : mode %q", paneMode(page))
	}
}

func TestComposeHTMLReplyQuotesSanitizedOriginal(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Réunion lundi")
	_, page := e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+uid, nil, nil)
	h := editorHTML(page)
	if !strings.Contains(h, "<blockquote") || !strings.Contains(h, "a écrit") {
		t.Errorf("citation absente :\n%s", h)
	}
	for _, bad := range []string{"<script", "onerror", "onload", "http://", "https://tracker"} {
		if strings.Contains(strings.ToLower(h), bad) {
			t.Errorf("%q dans la citation HTML :\n%s", bad, h)
		}
	}
	// la version texte du champ « body » reste disponible (secours sans JavaScript)
	if textareaValue(page) == "" {
		t.Error("texte brut absent")
	}
}

func TestComposeSendHTMLProducesAlternativeAndInlineImages(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id, ident := composeID(t, page), identityID(t, page)
	html := `<p style="color:red">Bonjour <b>Zoë</b></p><script>alert(1)</script><img src="data:image/png;base64,` + pngForTest + `"><img src="http://tracker.example/p.gif">` +
		`<a href="javascript:alert(1)" onclick="x()">lien</a>`
	f := sendForm(tok, id, url.Values{"identity": {ident}, "is_html": {"1"}, "html": {html}, "body": {"ignoré en mode HTML"}})
	resp, _ := e.do(c, "POST", "/compose/send", f, nil)
	e.expect(resp, 303, "/")
	data := smtp.Messages()[0].Data
	s := string(data)
	for _, want := range []string{"multipart/alternative", "multipart/related", "text/html", "text/plain", "Content-Id: <"} {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(want)) {
			t.Errorf("structure : %q absent", want)
		}
	}
	p, err := mailview.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.HTML, "<b>Zoë</b>") || !strings.Contains(p.Text, "Bonjour") || strings.Contains(p.Text, "ignoré") {
		t.Errorf("parties : HTML=%q TEXT=%q", p.HTML, p.Text)
	}
	for _, bad := range []string{"<script", "tracker.example", "javascript:", "onclick"} {
		if strings.Contains(strings.ToLower(p.HTML), bad) {
			t.Errorf("%q dans le HTML envoyé :\n%s", bad, p.HTML)
		}
	}
	if len(p.InlineImages) != 1 || len(p.Attachments) != 0 {
		t.Errorf("images en ligne : %d, pièces jointes : %d", len(p.InlineImages), len(p.Attachments))
	}
	// la copie conservée dans « Envoyés » est lisible en HTML
	_, sent := e.do(c, "GET", "/?folder=Sent&size=50", nil, nil)
	_, links := rows(sent)
	_, body := e.do(c, "GET", "/mail/body?folder=Sent&uid="+uidOf(links[0]), nil, nil)
	if !strings.Contains(body, "<b>Zoë</b>") {
		t.Errorf("copie envoyée :\n%s", body)
	}
}

const pngForTest = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestComposeHTMLDraftRoundTrip(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	id, ident := composeID(t, page), identityID(t, page)
	f := sendForm(tok, id, url.Values{"identity": {ident}, "is_html": {"1"}, "html": {`<p>Brouillon <i>riche</i></p><img src="data:image/png;base64,` + pngForTest + `">`}})
	resp, body := e.do(c, "POST", "/compose/save", f, map[string]string{"X-Requested-With": "fetch"})
	if resp.StatusCode != 200 {
		t.Fatalf("enregistrement : %d %s", resp.StatusCode, body)
	}
	_, list := e.do(c, "GET", "/?folder=Drafts&size=50", nil, nil)
	_, links := rows(list)
	_, page = e.do(c, "GET", "/compose?mode=draft&folder=Drafts&uid="+uidOf(links[0]), nil, nil)
	if paneMode(page) != "html" {
		t.Fatalf("un brouillon HTML doit rouvrir en HTML : %q", paneMode(page))
	}
	h := editorHTML(page)
	if !strings.Contains(h, "<i>riche</i>") || !strings.Contains(h, `src="data:image/png;base64,`) {
		t.Errorf("contenu du brouillon :\n%s", h)
	}
}

func TestComposeHTMLSignature(t *testing.T) {
	e, c, _, _ := composeEnvWith(t, nil, url.Values{"signature": {":HTML:<b>Gérard</b><script>x()</script><br>SDV"}})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	h := editorHTML(page)
	if !strings.Contains(h, `data-wmail-sig="1"`) || !strings.Contains(h, "<b>Gérard</b>") || strings.Contains(h, "script") {
		t.Errorf("signature HTML :\n%s", h)
	}
	if !strings.Contains(page, `data-signature-html="`) || strings.Contains(page, "<script>x()") {
		t.Error("signature HTML des identités")
	}
	// la version texte brut est convertie (pas de balises)
	if body := textareaValue(page); !strings.Contains(body, "Gérard") || strings.Contains(body, "<b>") {
		t.Errorf("signature en texte brut :\n%q", body)
	}
}

func TestComposeHTMLBodyTooLarge(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, func(cfg *config.Config) {
		cfg.Compose = config.Compose{MaxAttachmentMB: 1, MaxMessageMB: 1, MaxAttachments: 5, MaxSessionMB: 5}
	})
	_, page := e.do(c, "GET", "/compose", nil, nil)
	f := sendForm(tok, composeID(t, page), url.Values{"identity": {identityID(t, page)}, "is_html": {"1"}, "html": {"<p>" + strings.Repeat("x", 1100000) + "</p>"}})
	resp, body := e.do(c, "POST", "/compose/send", f, nil)
	if resp.StatusCode != 400 || !strings.Contains(body, "taille maximale") {
		t.Errorf("message HTML trop gros : %d", resp.StatusCode)
	}
	if len(smtp.Messages()) != 0 {
		t.Error("message envoyé malgré la taille")
	}
}

func TestComposeCSPAllowsInlineStylesOnlyThere(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	resp, _ := e.do(c, "GET", "/compose", nil, nil)
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") || strings.Contains(csp, "script-src") {
		t.Errorf("CSP de la composition : %s", csp)
	}
	if strings.Contains(csp, "'unsafe-eval'") || !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "img-src 'self' data:") {
		t.Errorf("CSP trop permissive : %s", csp)
	}
	for _, p := range []string{"/", "/settings", "/accounts"} {
		resp, _ := e.do(c, "GET", p, nil, nil)
		if strings.Contains(resp.Header.Get("Content-Security-Policy"), "unsafe-inline") {
			t.Errorf("%s : styles en ligne autorisés hors de la composition", p)
		}
	}
}

func TestComposeMailtoLink(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	link := "mailto:a@b.fr,c@d.fr?subject=Bonjour%20%C3%A0%20tous&cc=x@y.fr&bcc=z@y.fr&body=Salut%0ALigne%202"
	_, page := e.do(c, "GET", "/compose?mailto="+url.QueryEscape(link), nil, nil)
	if fieldValue(page, "to") != "a@b.fr,c@d.fr" || fieldValue(page, "cc") != "x@y.fr" || fieldValue(page, "bcc") != "z@y.fr" || fieldValue(page, "subject") != "Bonjour à tous" {
		t.Errorf("champs : to=%q cc=%q bcc=%q subject=%q", fieldValue(page, "to"), fieldValue(page, "cc"), fieldValue(page, "bcc"), fieldValue(page, "subject"))
	}
	if body := textareaValue(page); !strings.Contains(body, "Salut\nLigne 2") {
		t.Errorf("corps : %q", body)
	}
	if !strings.Contains(editorHTML(page), "Salut<br>Ligne 2") {
		t.Errorf("corps HTML : %q", editorHTML(page))
	}
	// Cc et Cci remplis s'affichent d'emblée
	if !strings.Contains(page, `data-show="cc" hidden`) {
		t.Error("le champ Cc rempli devrait être affiché")
	}
	// hostile : retours à la ligne retirés, jamais d'en-tête injecté
	evil := "mailto:a@b.fr%0D%0ABcc:pirate@evil.example?subject=x%0D%0ABcc:p@e.x&cc=%0A%0Dz@y.fr"
	_, page = e.do(c, "GET", "/compose?mailto="+url.QueryEscape(evil), nil, nil)
	if strings.ContainsAny(fieldValue(page, "to")+fieldValue(page, "subject")+fieldValue(page, "cc"), "\r\n") {
		t.Errorf("retour à la ligne conservé : %q %q", fieldValue(page, "to"), fieldValue(page, "subject"))
	}
	// pas un lien mailto: : ignoré
	_, page = e.do(c, "GET", "/compose?mailto="+url.QueryEscape("javascript:alert(1)"), nil, nil)
	if fieldValue(page, "to") != "" || strings.Contains(page, "javascript:alert") {
		t.Errorf("lien non mailto accepté : %q", fieldValue(page, "to"))
	}
	// l'envoi refuse la saisie piégée (« Bcc: » n'est pas une adresse)
	_, tokPage := e.do(c, "GET", "/settings", nil, nil)
	f := sendForm(csrf(t, tokPage), composeID(t, page), url.Values{"identity": {identityID(t, page)}, "to": {"a@b.fr Bcc:pirate@evil.example"}})
	resp, _ := e.do(c, "POST", "/compose/send", f, nil)
	if resp.StatusCode != 400 {
		t.Errorf("adresse piégée : %d", resp.StatusCode)
	}
}

func TestSettingsPageOffersMailtoHandler(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(page, `id="mailto-register"`) || !strings.Contains(page, `/static/mailto.js`) || !strings.Contains(page, `data-uri="/compose?mailto=%s"`) {
		t.Error("gestionnaire mailto absent des réglages")
	}
}

func TestSenderLinkOpensComposition(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uid, nil, nil)
	if !strings.Contains(page, `class="sender" href="/compose?to=alice%40x.fr&amp;back=`) {
		t.Errorf("lien de l'expéditeur :\n%s", regexp.MustCompile(`class="sender"[^>]*`).FindString(page))
	}
	_, form := e.do(c, "GET", "/compose?to=alice%40x.fr", nil, nil)
	if fieldValue(form, "to") != "alice@x.fr" {
		t.Errorf("destinataire prérempli : %q", fieldValue(form, "to"))
	}
}

func TestSignatureMomentVariable(t *testing.T) {
	got := plainSignature("Cordialement {{MOMENT:DD/MM/YYYY HH:mm}} {{FROM}}", nil, "fr", time.UTC)
	if matched, _ := regexp.MatchString(`^Cordialement \d\d/\d\d/\d{4} \d\d:\d\d $`, got+" "); !matched {
		t.Errorf("signature : %q", got)
	}
	if got := momentFormat("YY-M-D", time.Date(2026, 3, 7, 9, 5, 0, 0, time.UTC)); got != "26-3-7" {
		t.Errorf("format court : %q", got)
	}
}

func TestLastReplyActionBecomesMainButton(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	msgURL := "/mail/message?folder=INBOX&uid=" + uid
	_, page := e.do(c, "GET", msgURL, nil, nil)
	if !strings.Contains(page, `class="reply" href="/compose?`) || !strings.Contains(page, "mode=reply&amp;") && !strings.Contains(page, "mode=reply\"") {
		t.Fatalf("bouton par défaut : répondre\n%s", page)
	}
	e.do(c, "GET", "/compose?mode=forward&folder=INBOX&uid="+uid, nil, nil)
	_, page = e.do(c, "GET", msgURL, nil, nil)
	if !regexp.MustCompile(`class="reply" href="[^"]*mode=forward`).MatchString(page) {
		t.Errorf("le bouton principal doit transférer après un transfert :\n%s", page)
	}
	e.do(c, "GET", "/compose?mode=replyall&folder=INBOX&uid="+uid, nil, nil)
	_, page = e.do(c, "GET", msgURL, nil, nil)
	if !regexp.MustCompile(`class="reply" href="[^"]*mode=replyall`).MatchString(page) {
		t.Errorf("le bouton principal doit répondre à tous :\n%s", page)
	}
}
