package web

import (
	"bytes"
	"encoding/json"
	"github.com/gdegoulet/wmail-go/internal/contacts/davtest"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func (e *env) upload(c *http.Client, path, tok, field, name string, data []byte) (*http.Response, string) {
	e.t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("csrf", tok)
	fw, _ := mw.CreateFormFile(field, name)
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

func TestContactsLifecycle(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	_, page := e.do(c, "GET", "/contacts", nil, nil)
	if !strings.Contains(page, "Aucun contact") {
		t.Fatalf("carnet non vide :\n%s", page)
	}
	form := url.Values{"csrf": {tok}, "first": {"Zoë"}, "last": {"Dupont"}, "email": {"Zoe@Example.org", ""}, "email_type": {"work", ""}, "tags": {"amis, travail"}, "adr_street": {""}}
	resp, _ := e.do(c, "POST", "/contacts/save", form, nil)
	if resp.StatusCode != 303 || !strings.HasPrefix(resp.Header.Get("Location"), "/contacts?info=saved&edit=") {
		t.Fatalf("enregistrement : %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	id := strings.TrimPrefix(resp.Header.Get("Location"), "/contacts?info=saved&edit=")
	_, page = e.do(c, "GET", "/contacts?edit="+id, nil, nil)
	for _, want := range []string{"Zoë Dupont", "zoe@example.org", "amis, travail"} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent de la fiche", want)
		}
	}
	// recherche sans accent, étiquette
	_, page = e.do(c, "GET", "/contacts?q=zoe&tag=amis", nil, nil)
	if !strings.Contains(page, "Zoë Dupont") {
		t.Error("recherche")
	}
	_, page = e.do(c, "GET", "/contacts?q=inconnu", nil, nil)
	if strings.Contains(page, "Zoë Dupont") {
		t.Error("recherche trop large")
	}
	// suggestion JSON
	_, js := e.do(c, "GET", "/contacts/suggest?q=zo", nil, nil)
	var sug []struct{ Name, Email, Label string }
	if err := json.Unmarshal([]byte(js), &sug); err != nil || len(sug) != 1 || sug[0].Email != "zoe@example.org" || sug[0].Label != "Zoë Dupont <zoe@example.org>" {
		t.Errorf("suggestions : %s %v", js, err)
	}
	// fiche vide refusée
	resp, page = e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "est vide") {
		t.Errorf("fiche vide : %d", resp.StatusCode)
	}
	// ajout depuis un expéditeur, doublon sans effet
	resp, _ = e.do(c, "POST", "/contacts/add", url.Values{"csrf": {tok}, "email": {"ZOE@example.org"}, "name": {"Autre"}}, nil)
	if resp.Header.Get("Location") != "/contacts?info=added&edit="+id {
		t.Errorf("ajout doublon : %s", resp.Header.Get("Location"))
	}
	// export vCard et CSV
	resp, vcf := e.do(c, "GET", "/contacts/export?format=vcf", nil, nil)
	if !strings.Contains(vcf, "BEGIN:VCARD") || !strings.Contains(vcf, "zoe@example.org") || !strings.Contains(resp.Header.Get("Content-Disposition"), "contacts.vcf") {
		t.Errorf("export vcf : %s", vcf)
	}
	_, csv := e.do(c, "GET", "/contacts/export?format=csv", nil, nil)
	if !strings.Contains(csv, "First Name") || !strings.Contains(csv, "zoe@example.org") {
		t.Errorf("export csv : %s", csv)
	}
	// import : le doublon est ignoré, le nouveau ajouté
	card := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Zoé Bis\r\nEMAIL:zoe@example.org\r\nEND:VCARD\r\nBEGIN:VCARD\r\nVERSION:3.0\r\nFN:Paul Martin\r\nEMAIL:paul@example.org\r\nEND:VCARD\r\n"
	resp, _ = e.upload(c, "/contacts/import", tok, "file", "c.vcf", []byte(card))
	if resp.Header.Get("Location") != "/contacts?info=imported&n=1&skipped=1" {
		t.Errorf("import : %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp, _ = e.upload(c, "/contacts/import", tok, "file", "x.csv", []byte("n'importe quoi\n"))
	if resp.StatusCode != 400 {
		t.Errorf("import invalide : %d", resp.StatusCode)
	}
	// suppression
	resp, _ = e.do(c, "POST", "/contacts/delete", url.Values{"csrf": {tok}, "id": {id}}, nil)
	e.expect(resp, 303, "/contacts?info=deleted")
	_, page = e.do(c, "GET", "/contacts", nil, nil)
	if strings.Contains(page, "Zoë Dupont") || !strings.Contains(page, "Paul Martin") {
		t.Error("suppression")
	}
}

func TestContactsPersistAcrossLoginAndAreIsolated(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}, "first": {"Secret"}, "email": {"s@x.org"}}, nil)
	// nouvelle session du même compte
	c2 := e.client()
	e.login(c2, "alice", pw)
	_, page := e.do(c2, "GET", "/contacts", nil, nil)
	if !strings.Contains(page, "Secret") {
		t.Error("carnet perdu à la reconnexion")
	}
	// autre compte : carnet distinct
	cb := e.client()
	e.register(cb, "bob")
	_, page = e.do(cb, "GET", "/contacts", nil, nil)
	if strings.Contains(page, "Secret") {
		t.Error("fuite entre comptes")
	}
	// CSRF obligatoire
	resp, _ := e.do(c, "POST", "/contacts/delete", url.Values{"id": {"x"}}, nil)
	if resp.StatusCode != 403 {
		t.Errorf("csrf : %d", resp.StatusCode)
	}
	// anonyme
	resp, _ = e.do(e.client(), "GET", "/contacts/suggest?q=s", nil, nil)
	e.expect(resp, 303, "/login")
}

func TestContactsAutoAddAndFrecencyAfterSend(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}, "first": {"Connu"}, "email": {"connu@x.fr"}}, nil)
	send := func(to, cc string) {
		_, page := e.do(c, "GET", "/compose", nil, nil)
		f := sendForm(tok, composeID(t, page), url.Values{"identity": {identityID(t, page)}, "to": {to}, "cc": {cc}})
		resp, _ := e.do(c, "POST", "/compose/send", f, nil)
		e.expect(resp, 303, "/")
	}
	send("Connu <connu@x.fr>, Nouveau Venu <nouveau@x.fr>", "cc@x.fr")
	_, page := e.do(c, "GET", "/contacts", nil, nil)
	for _, want := range []string{"Connu", "Nouveau Venu", "cc@x.fr"} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent après envoi", want)
		}
	}
	// le contact connu, plus utilisé, passe avant à l'autocomplétion
	_, js := e.do(c, "GET", "/contacts/suggest?q=x.fr", nil, nil)
	if !strings.HasPrefix(js, `[{"name":"Connu"`) {
		t.Errorf("ordre par fréquence : %s", js)
	}
	// désactivation : plus d'ajout automatique
	set := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"rainloop"}, "idle_minutes": {"30"}, "absolute_hours": {"8"}, "page_size": {"50"}}
	resp, _ := e.do(c, "POST", "/settings/preferences", set, nil)
	e.expect(resp, 200, "")
	send("autre@x.fr", "")
	_, page = e.do(c, "GET", "/contacts", nil, nil)
	if strings.Contains(page, "autre@x.fr") {
		t.Error("ajout automatique malgré le réglage désactivé")
	}
}

func TestMailPoll(t *testing.T) {
	e, c, _, _ := mdnEnv(t)
	_, js := e.do(c, "GET", "/mail/poll?since=0", nil, nil)
	var base struct {
		Unseen, UIDNext uint32
		New             []struct {
			UID           uint32
			From, Subject string
		}
	}
	if err := json.Unmarshal([]byte(js), &base); err != nil || base.Unseen == 0 || base.UIDNext == 0 || len(base.New) != 0 {
		t.Fatalf("sondage initial : %s %v", js, err)
	}
	_, js = e.do(c, "GET", "/mail/poll?since=1", nil, nil)
	var again struct {
		New []struct {
			UID           uint32
			From, Subject string
		}
	}
	if err := json.Unmarshal([]byte(js), &again); err != nil || len(again.New) == 0 || again.New[0].Subject == "" {
		t.Errorf("nouveaux messages : %s %v", js, err)
	}
	// rien de nouveau depuis l'UID suivant
	_, js = e.do(c, "GET", "/mail/poll?since="+strconv.Itoa(int(base.UIDNext)), nil, nil)
	if !strings.Contains(js, `"new":[]`) {
		t.Errorf("rien de nouveau attendu : %s", js)
	}
	resp, _ := e.do(e.client(), "GET", "/mail/poll", nil, nil)
	e.expect(resp, 303, "/login")
}

func TestContactsCardDAVSyncFromSettings(t *testing.T) {
	e := newEnv(t, nil)
	davURL, be, _ := davtest.Start(t)
	c, tok := e.loggedIn("alice")
	be.Put("Anne", "anne@x.org", time.Now().Add(-time.Hour))
	e.do(c, "POST", "/contacts/save", url.Values{"csrf": {tok}, "first": {"Local"}, "email": {"local@x.org"}}, nil)

	// URL en http vers un hôte non autorisé : refusée ; https exigé
	form := url.Values{"csrf": {tok}, "enabled": {"1"}, "url": {"http://exemple.invalid/dav"}, "user": {"alice"}, "password": {"secret"}, "interval": {"1"}}
	resp, page := e.do(c, "POST", "/contacts/sync/save", form, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "https://") {
		t.Fatalf("url http refusée : %d", resp.StatusCode)
	}
	form.Set("url", davURL) // 127.0.0.1 figure dans allowed_private_hosts du test : http toléré
	resp, _ = e.do(c, "POST", "/contacts/sync/save", form, nil)
	e.expect(resp, 303, "/contacts?info=syncsaved")
	_, page = e.do(c, "GET", "/contacts", nil, nil)
	if !strings.Contains(page, `value="5"`) || strings.Contains(page, "secret") || !strings.Contains(page, "••••") {
		t.Errorf("réglages affichés (intervalle ramené à 5, mot de passe jamais réaffiché) :\n%s", page)
	}
	// L'affichage de la page a pu lancer le passage automatique : on attend le résultat, qu'il vienne de lui ou du bouton.
	e.do(c, "POST", "/contacts/sync/run", url.Values{"csrf": {tok}}, nil)
	deadline := time.Now().Add(5 * time.Second)
	for strings.Join(be.Names(), ",") != "Anne,Local" && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		e.do(c, "POST", "/contacts/sync/run", url.Values{"csrf": {tok}}, nil)
	}
	if got := strings.Join(be.Names(), ","); got != "Anne,Local" {
		t.Fatalf("serveur : %s", got)
	}
	_, page = e.do(c, "GET", "/contacts", nil, nil)
	if !strings.Contains(page, "Anne") || !strings.Contains(page, "Dernière synchronisation") {
		t.Error("contact distant absent après synchronisation")
	}
	// mot de passe laissé vide : conservé ; effacé sur demande
	form.Set("password", "")
	e.do(c, "POST", "/contacts/sync/save", form, nil)
	if resp, _ = e.do(c, "POST", "/contacts/sync/run", url.Values{"csrf": {tok}}, nil); resp.StatusCode != 303 {
		t.Errorf("mot de passe perdu : %d", resp.StatusCode)
	}
	form.Set("clear_password", "1")
	e.do(c, "POST", "/contacts/sync/save", form, nil)
	if resp, _ = e.do(c, "POST", "/contacts/sync/run", url.Values{"csrf": {tok}}, nil); resp.StatusCode != 502 {
		t.Errorf("sans mot de passe, la synchronisation doit échouer : %d", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/contacts", nil, nil)
	if !strings.Contains(page, "Dernier échec") {
		t.Error("l'échec n'est pas affiché")
	}
}

func TestWatchedFoldersInPoll(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/settings/folders", nil, nil)
	if !strings.Contains(page, `name="check" value="Sent"`) || strings.Contains(page, `name="check" value="INBOX"`) {
		t.Fatalf("liste de surveillance :\n%s", page)
	}
	acc := regexp.MustCompile(`name="box" value="([^"]+)"`).FindStringSubmatch(page)[1]
	resp, _ := e.do(c, "POST", "/settings/folders/check", url.Values{"csrf": {tok}, "box": {acc}, "check": {"Sent", "Inconnu"}}, nil)
	if resp.StatusCode != 400 {
		t.Errorf("dossier inconnu : %d", resp.StatusCode)
	}
	resp, _ = e.do(c, "POST", "/settings/folders/check", url.Values{"csrf": {tok}, "box": {acc}, "check": {"Sent"}}, nil)
	e.expect(resp, 303, "/settings/folders?box="+acc)
	_, js := e.do(c, "GET", "/mail/poll?since=0", nil, nil)
	if !strings.Contains(js, `"folders":{"Sent":`) {
		t.Errorf("sondage : %s", js)
	}
	many := url.Values{"csrf": {tok}, "box": {acc}, "check": {"a", "b", "c", "d", "e", "f"}}
	if resp, _ = e.do(c, "POST", "/settings/folders/check", many, nil); resp.StatusCode != 400 {
		t.Errorf("trop de dossiers : %d", resp.StatusCode)
	}
}

func TestWriteToSelectedContacts(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	ids := []string{}
	for _, v := range []struct{ first, mail string }{{"Anne", "anne@x.fr"}, {"Paul", "paul@x.fr"}, {"Sans", ""}} {
		f := url.Values{"csrf": {tok}, "first": {v.first}}
		if v.mail != "" {
			f.Set("email", v.mail)
		}
		resp, _ := e.do(c, "POST", "/contacts/save", f, nil)
		ids = append(ids, strings.TrimPrefix(resp.Header.Get("Location"), "/contacts?info=saved&edit="))
	}
	resp, page := e.do(c, "POST", "/contacts/compose", url.Values{"csrf": {tok}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "Cochez") {
		t.Errorf("sans sélection : %d", resp.StatusCode)
	}
	resp, page = e.do(c, "POST", "/contacts/compose", url.Values{"csrf": {tok}, "id": {ids[2]}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "adresse") {
		t.Errorf("contact sans adresse : %d", resp.StatusCode)
	}
	resp, _ = e.do(c, "POST", "/contacts/compose", url.Values{"csrf": {tok}, "id": {ids[0], ids[1]}}, nil)
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != 303 || loc.Path != "/compose" || loc.Query().Get("to") != "Anne <anne@x.fr>, Paul <paul@x.fr>" {
		t.Errorf("redirection : %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	_, page = e.do(c, "GET", "/contacts", nil, nil)
	if !strings.Contains(page, "data-needs-contacts") {
		t.Error("bouton « Écrire à la sélection » absent")
	}
}
