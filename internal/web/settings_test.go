package web

import (
	"github.com/gdegoulet/wmail-go/internal/config"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
)

func folderNames(page string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`<td class="fname[^"]*">([^<]*)`).FindAllStringSubmatch(page, -1) {
		out = append(out, strings.TrimSpace(htmlUnescape(m[1])))
	}
	return out
}

func TestSettingsShellAndSections(t *testing.T) {
	e, c, _, _ := composeEnv(t, nil)
	for path, want := range map[string]string{
		"/settings": `name="page_size"`, "/settings/security": `action="/settings/password"`,
		"/accounts": `/accounts/new`, "/settings/folders": `action="/settings/folders/system"`,
	} {
		resp, page := e.do(c, "GET", path, nil, nil)
		e.expect(resp, 200, "")
		if !strings.Contains(page, want) {
			t.Errorf("%s : %q absent", path, want)
		}
		if !strings.Contains(page, `class="setmenu"`) || strings.Contains(page, `class="topbar"`) {
			t.Errorf("%s : cadre des réglages absent", path)
		}
	}
	// chaque page ne montre que sa section
	_, general := e.do(c, "GET", "/settings", nil, nil)
	if strings.Contains(general, `action="/settings/password"`) {
		t.Error("le mot de passe doit être dans la section Sécurité")
	}
	_, sec := e.do(c, "GET", "/settings/security", nil, nil)
	if strings.Contains(sec, `name="page_size"`) {
		t.Error("les préférences doivent être dans la section Général")
	}
	anon := e.client()
	for _, p := range []string{"/settings/folders", "/settings/security", "/accounts/x/identities"} {
		resp, _ := e.do(anon, "GET", p, nil, nil)
		e.expect(resp, 303, "/login")
	}
}

func TestFoldersSettingsLifecycle(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/settings/folders", nil, nil)
	names := folderNames(page)
	for _, want := range []string{"INBOX", "Sent", "Trash", "Projets", "2026"} {
		if !slicesContains(names, want) {
			t.Errorf("dossier %q absent : %v", want, names)
		}
	}
	// INBOX : ni renommer ni supprimer
	inboxRow := regexp.MustCompile(`(?s)<tr>\s*<td class="fname[^"]*">INBOX.*?</tr>`).FindString(page)
	if inboxRow == "" || strings.Contains(inboxRow, `value="delete"`) || strings.Contains(inboxRow, `value="rename"`) {
		t.Errorf("ligne INBOX : %s", inboxRow)
	}
	act := func(v url.Values) (int, string) {
		v.Set("csrf", tok)
		resp, body := e.do(c, "POST", "/settings/folders/action", v, nil)
		return resp.StatusCode, body
	}
	if st, _ := act(url.Values{"action": {"create"}, "new_name": {"Archives 2026"}, "parent": {"Projets"}, "delim": {"/"}}); st != 303 {
		t.Fatalf("création : %d", st)
	}
	_, page = e.do(c, "GET", "/settings/folders", nil, nil)
	if !slicesContains(folderNames(page), "Archives 2026") {
		t.Fatalf("dossier créé absent : %v", folderNames(page))
	}
	if !strings.Contains(page, "Dossier créé.") {
		t.Error("message de confirmation absent")
	}
	if st, _ := act(url.Values{"action": {"rename"}, "name": {"Projets/Archives 2026"}, "new_name": {"Vieux"}, "delim": {"/"}}); st != 303 {
		t.Fatalf("renommage : %d", st)
	}
	_, page = e.do(c, "GET", "/settings/folders", nil, nil)
	if n := folderNames(page); !slicesContains(n, "Vieux") || slicesContains(n, "Archives 2026") {
		t.Fatalf("après renommage : %v", n)
	}
	if st, _ := act(url.Values{"action": {"delete"}, "name": {"Projets/Vieux"}}); st != 303 {
		t.Fatalf("suppression : %d", st)
	}
	_, page = e.do(c, "GET", "/settings/folders", nil, nil)
	if slicesContains(folderNames(page), "Vieux") {
		t.Fatal("dossier supprimé encore présent")
	}
	// refus
	for _, tc := range []struct {
		name string
		v    url.Values
		want int
	}{
		{"supprimer INBOX", url.Values{"action": {"delete"}, "name": {"INBOX"}}, 400},
		{"supprimer inbox", url.Values{"action": {"delete"}, "name": {"inbox"}}, 400},
		{"renommer INBOX", url.Values{"action": {"rename"}, "name": {"INBOX"}, "new_name": {"x"}}, 400},
		{"séparateur dans le nom", url.Values{"action": {"create"}, "new_name": {"a/b"}, "delim": {"/"}}, 400},
		{"joker dans le nom", url.Values{"action": {"create"}, "new_name": {"a*b"}}, 400},
		{"nom vide", url.Values{"action": {"create"}, "new_name": {"  "}}, 400},
		{"point point", url.Values{"action": {"create"}, "new_name": {".."}}, 400},
		{"action inconnue", url.Values{"action": {"detruire"}, "name": {"Sent"}}, 400},
		{"dossier inexistant", url.Values{"action": {"delete"}, "name": {"Fantome"}}, 502},
	} {
		if st, _ := act(tc.v); st != tc.want {
			t.Errorf("%s : statut %d, attendu %d", tc.name, st, tc.want)
		}
	}
	_, page = e.do(c, "GET", "/settings/folders", nil, nil)
	if !slicesContains(folderNames(page), "INBOX") {
		t.Error("INBOX a disparu")
	}
	// jeton CSRF obligatoire
	resp, _ := e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {"x"}, "action": {"delete"}, "name": {"Sent"}}, nil)
	if resp.StatusCode != 403 {
		t.Errorf("sans CSRF : %d", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/settings/folders", nil, nil)
	if !slicesContains(folderNames(page), "Sent") {
		t.Error("Sent supprimé sans jeton valide")
	}
}

func TestFoldersSystemChoice(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	post := func(v url.Values) int {
		v.Set("csrf", tok)
		resp, _ := e.do(c, "POST", "/settings/folders/system", v, nil)
		return resp.StatusCode
	}
	if st := post(url.Values{"sent": {"Projets"}, "drafts": {"-"}, "trash": {"Trash"}, "junk": {""}, "archive": {""}}); st != 303 {
		t.Fatalf("enregistrement : %d", st)
	}
	_, form := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([a-f0-9]+)/edit`).FindStringSubmatch(form)[1]
	_, edit := e.do(c, "GET", "/accounts/"+id+"/edit", nil, nil)
	if selectedValue(edit, "sent_folder") != "Projets" || selectedValue(edit, "drafts_folder") != "-" {
		t.Errorf("valeurs enregistrées : sent=%q drafts=%q", selectedValue(edit, "sent_folder"), selectedValue(edit, "drafts_folder"))
	}
	// ce sont des sélecteurs de dossiers existants, pas des zones de texte libres
	for _, f := range []string{"trash_folder", "archive_folder", "junk_folder", "sent_folder", "drafts_folder"} {
		if !strings.Contains(edit, `<select name="`+f+`">`) || strings.Contains(edit, `<input name="`+f+`"`) {
			t.Errorf("%s n'est pas un sélecteur", f)
		}
	}
	if !strings.Contains(edit, `<option value="Projets/2026"`) {
		t.Error("les dossiers du serveur ne sont pas proposés")
	}
	// la composition en tient compte : plus de brouillons, copie dans « Projets »
	_, page := e.do(c, "GET", "/compose", nil, nil)
	if strings.Contains(page, `id="cp-save"`) {
		t.Error("bouton Enregistrer présent sans dossier de brouillons")
	}
	resp, _ := e.do(c, "POST", "/compose/send", sendForm(tok, composeID(t, page), url.Values{"identity": {identityID(t, page)}}), nil)
	e.expect(resp, 303, "/")
	_, list := e.do(c, "GET", "/?folder=Projets&size=50", nil, nil)
	if subs, _ := rows(list); len(subs) != 1 {
		t.Errorf("copie dans le dossier choisi : %v", subs)
	}
	// refus : « ne pas utiliser » pour la corbeille, dossier inexistant
	if st := post(url.Values{"sent": {""}, "drafts": {""}, "trash": {"-"}}); st != 400 {
		t.Errorf("corbeille « ne pas utiliser » : %d", st)
	}
	if st := post(url.Values{"sent": {"Fantome"}}); st != 400 {
		t.Errorf("dossier inexistant : %d", st)
	}
	// un dossier désigné ne se supprime pas depuis les réglages (rôle système)
	e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "action": {"delete"}, "name": {"Projets/2026"}}, nil)
	post(url.Values{"sent": {"Projets"}, "drafts": {""}, "trash": {""}})
	if resp, _ := e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "action": {"delete"}, "name": {"Projets"}}, nil); resp.StatusCode != 400 {
		t.Errorf("un dossier système ne doit pas être supprimable : %d", resp.StatusCode)
	}
	// renommé ailleurs (autre client) : le réglage retombe sur la détection automatique
	post(url.Values{"sent": {"Projets"}, "drafts": {""}, "trash": {""}})
	e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "action": {"rename"}, "name": {"Projets"}, "new_name": {"Ailleurs"}, "delim": {"/"}}, nil)
	_, edit = e.do(c, "GET", "/accounts/"+id+"/edit", nil, nil)
	if selectedValue(edit, "sent_folder") != "" {
		t.Errorf("dossier supprimé encore désigné : %q", selectedValue(edit, "sent_folder"))
	}
}

func accountIDOf(t *testing.T, e *env, c interface {
}, page string) string {
	t.Helper()
	m := regexp.MustCompile(`/accounts/([a-f0-9]+)/edit`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("identifiant de compte introuvable")
	}
	return m[1]
}

func TestIdentitiesLifecycleAndUseInCompose(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([a-f0-9]+)/identities`).FindStringSubmatch(accounts)[1]
	base := "/accounts/" + id + "/identities"

	resp, page := e.do(c, "GET", base, nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "test@localhost") {
		t.Error("identité principale absente")
	}
	add := url.Values{"csrf": {tok}, "email": {"alias@example.org"}, "name": {"Alias é"}, "reply_to": {"retour@example.org"}, "bcc": {"archive@example.org"}, "signature": {"-- Alias"}}
	resp, _ = e.do(c, "POST", base, add, nil)
	e.expect(resp, 303, base)
	_, page = e.do(c, "GET", base, nil, nil)
	if !strings.Contains(page, "alias@example.org") || !strings.Contains(page, "Identité enregistrée.") {
		t.Fatalf("identité ajoutée absente :\n%s", page)
	}
	iid := regexp.MustCompile(`identities\?edit=([a-f0-9]+)`).FindStringSubmatch(page)[1]

	// refus : doublon (même casse ignorée), adresse invalide, injection
	for name, v := range map[string]url.Values{
		"doublon":   {"csrf": {tok}, "email": {"ALIAS@example.org"}},
		"principal": {"csrf": {tok}, "email": {"test@localhost"}},
		"invalide":  {"csrf": {tok}, "email": {"pas une adresse"}},
		"injection": {"csrf": {tok}, "email": {"a@b.fr"}, "name": {"x\r\nBcc: p@e.x"}},
		"reply_to":  {"csrf": {tok}, "email": {"b@b.fr"}, "reply_to": {"pas une adresse"}},
	} {
		resp, _ := e.do(c, "POST", base, v, nil)
		if resp.StatusCode != 400 {
			t.Errorf("%s : statut %d", name, resp.StatusCode)
		}
	}

	// la composition propose l'identité ; l'envoi l'utilise (enveloppe, De, Répondre à, Cci automatique)
	_, comp := e.do(c, "GET", "/compose", nil, nil)
	if !strings.Contains(comp, `value="`+iid+`"`) || !strings.Contains(comp, `data-signature="-- Alias"`) {
		t.Fatalf("identité absente de la composition :\n%s", comp)
	}
	f := sendForm(tok, composeID(t, comp), url.Values{"identity": {iid}})
	resp, _ = e.do(c, "POST", "/compose/send", f, nil)
	e.expect(resp, 303, "/")
	got := smtp.Messages()[0]
	data := string(got.Data)
	if got.From != "alias@example.org" || !strings.Contains(data, "alias@example.org") || !strings.Contains(data, "retour@example.org") {
		t.Errorf("identité non utilisée : from=%s\n%s", got.From, data)
	}
	if len(got.Rcpt) != 2 || strings.Contains(strings.ToLower(data), "bcc:") || strings.Contains(data, "archive@example.org") {
		t.Errorf("Cci automatique : %v", got.Rcpt)
	}

	// modification puis suppression
	edit := url.Values{"csrf": {tok}, "iid": {iid}, "email": {"alias@example.org"}, "name": {"Nouveau nom"}}
	resp, _ = e.do(c, "POST", base, edit, nil)
	e.expect(resp, 303, base)
	_, page = e.do(c, "GET", base, nil, nil)
	if !strings.Contains(page, "Nouveau nom") {
		t.Error("modification non enregistrée")
	}
	resp, _ = e.do(c, "POST", base+"/"+iid+"/delete", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 303, base)
	_, comp = e.do(c, "GET", "/compose", nil, nil)
	if strings.Contains(comp, "alias@example.org") {
		t.Error("identité supprimée encore proposée")
	}
	resp, _ = e.do(c, "POST", base+"/"+iid+"/delete", url.Values{"csrf": {tok}}, nil)
	if resp.StatusCode != 404 {
		t.Errorf("double suppression : %d", resp.StatusCode)
	}
}

func TestIdentityChosenFromMessageRecipient(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([a-f0-9]+)/identities`).FindStringSubmatch(accounts)[1]
	// une identité qui est le destinataire du message « Facture octobre » (To: test@localhost est l'identité principale) :
	// on ajoute un alias et on vérifie que la principale reste choisie pour répondre à ce message
	e.do(c, "POST", "/accounts/"+id+"/identities", url.Values{"csrf": {tok}, "email": {"alias@example.org"}}, nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, page := e.do(c, "GET", "/compose?mode=reply&folder=INBOX&uid="+uid, nil, nil)
	if !regexp.MustCompile(`<option value="` + id + `"[^>]*selected>`).MatchString(page) {
		t.Errorf("l'identité du destinataire d'origine n'est pas sélectionnée :\n%s", page)
	}
}

func TestIdentitiesAreIsolatedBetweenUsers(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([a-f0-9]+)/identities`).FindStringSubmatch(accounts)[1]
	e.do(c, "POST", "/accounts/"+id+"/identities", url.Values{"csrf": {tok}, "email": {"alias@example.org"}}, nil)

	other, otherTok := e.loggedIn("bob")
	resp, _ := e.do(other, "GET", "/accounts/"+id+"/identities", nil, nil)
	if resp.StatusCode != 404 {
		t.Errorf("lecture par un autre utilisateur : %d", resp.StatusCode)
	}
	resp, _ = e.do(other, "POST", "/accounts/"+id+"/identities", url.Values{"csrf": {otherTok}, "email": {"pirate@example.org"}}, nil)
	if resp.StatusCode != 404 {
		t.Errorf("écriture par un autre utilisateur : %d", resp.StatusCode)
	}
	resp, _ = e.do(c, "POST", "/accounts/"+id+"/identities", url.Values{"csrf": {"mauvais"}, "email": {"x@y.fr"}}, nil)
	if resp.StatusCode != 403 {
		t.Errorf("sans CSRF : %d", resp.StatusCode)
	}
}

func TestEditingAccountKeepsIdentities(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([a-f0-9]+)/identities`).FindStringSubmatch(accounts)[1]
	e.do(c, "POST", "/accounts/"+id+"/identities", url.Values{"csrf": {tok}, "email": {"alias@example.org"}}, nil)
	_, edit := e.do(c, "GET", "/accounts/"+id+"/edit", nil, nil)
	form := url.Values{"csrf": {tok}, "action": {"save"}, "id": {id}, "email": {"test@localhost"}, "display_name": {"Nouveau"},
		"imap_host": {fieldValue(edit, "imap_host")}, "imap_port": {fieldValue(edit, "imap_port")}, "imap_security": {"none"}, "imap_login": {fieldValue(edit, "imap_login")},
		"smtp_host": {fieldValue(edit, "smtp_host")}, "smtp_port": {fieldValue(edit, "smtp_port")}, "smtp_security": {"none"}, "block_remote": {"on"}}
	resp, _ := e.do(c, "POST", "/accounts/save", form, nil)
	e.expect(resp, 303, "")
	_, comp := e.do(c, "GET", "/compose", nil, nil)
	if !strings.Contains(comp, "alias@example.org") || !strings.Contains(comp, "Nouveau &lt;test@localhost&gt;") {
		t.Errorf("identités après modification du compte :\n%s", comp)
	}
}

func TestUnsubscribedFolderHiddenInPanel(t *testing.T) {
	e := newEnv(t, nil)
	ports := mailPorts{imap: mailtest.StartIMAP(t, "listext"), smtp: mailtest.StartSMTP(t, "none")}
	c, tok := e.loggedIn("alice")
	resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, ports, "save"), nil)
	e.expect(resp, 303, "/")
	panel := func() string { _, p := e.do(c, "GET", "/", nil, nil); return p }
	// Le serveur de test ne garde aucun abonnement au départ : sans abonnement, rien n'est masqué.
	if !strings.Contains(panel(), `folder=Projets"`) {
		t.Fatal("dossier absent du panneau alors qu'aucun abonnement n'existe")
	}
	for _, n := range []string{"Projets", "Sent"} {
		e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "action": {"subscribe"}, "name": {n}}, nil)
	}
	resp, _ = e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "action": {"unsubscribe"}, "name": {"Projets"}}, nil)
	expectRedirectPrefix(t, resp, "/settings/folders")
	if p := panel(); strings.Contains(p, `folder=Projets"`) || !strings.Contains(p, `folder=Sent"`) {
		t.Errorf("après désabonnement, Projets doit être masqué et Sent visible :\n%s", regexp.MustCompile(`href="[^"]*folder=[^"]*"`).FindAllString(p, -1))
	}
	// il reste listé dans les réglages, avec l'action « s'abonner »
	_, page := e.do(c, "GET", "/settings/folders", nil, nil)
	if !slicesContains(folderNames(page), "Projets") || !strings.Contains(page, `value="subscribe"`) {
		t.Error("dossier désabonné absent des réglages")
	}
	// sa lecture directe reste possible
	resp, _ = e.do(c, "GET", "/?folder=Projets", nil, nil)
	e.expect(resp, 200, "")
	e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "action": {"subscribe"}, "name": {"Projets"}}, nil)
	if !strings.Contains(panel(), `folder=Projets"`) {
		t.Error("dossier réabonné absent du panneau")
	}
}

// selectedValue renvoie la valeur de l'option choisie d'un sélecteur.
func selectedValue(page, name string) string {
	m := regexp.MustCompile(`(?s)<select name="` + name + `">(.*?)</select>`).FindStringSubmatch(page)
	if m == nil {
		return "(absent)"
	}
	o := regexp.MustCompile(`<option value="([^"]*)"[^>]*selected`).FindStringSubmatch(m[1])
	if o == nil {
		return ""
	}
	return htmlUnescape(o[1])
}

func TestAccountFormFolderSelectorsKeepMissingValue(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([a-f0-9]+)/edit`).FindStringSubmatch(accounts)[1]
	_, edit := e.do(c, "GET", "/accounts/"+id+"/edit", nil, nil)
	form := url.Values{"csrf": {tok}, "action": {"save"}, "id": {id}, "email": {"test@localhost"},
		"imap_host": {fieldValue(edit, "imap_host")}, "imap_port": {fieldValue(edit, "imap_port")}, "imap_security": {"none"}, "imap_login": {fieldValue(edit, "imap_login")},
		"smtp_host": {fieldValue(edit, "smtp_host")}, "smtp_port": {fieldValue(edit, "smtp_port")}, "smtp_security": {"none"}, "block_remote": {"on"},
		"trash_folder": {"Dossier disparu"}}
	resp, _ := e.do(c, "POST", "/accounts/save", form, nil)
	e.expect(resp, 303, "")
	_, edit = e.do(c, "GET", "/accounts/"+id+"/edit", nil, nil)
	if selectedValue(edit, "trash_folder") != "Dossier disparu" || !strings.Contains(edit, "introuvable sur le serveur") {
		t.Errorf("une valeur absente du serveur doit rester visible et signalée :\n%s", regexp.MustCompile(`(?s)<select name="trash_folder">.*?</select>`).FindString(edit))
	}
}

func TestFolderTreeStateAndPaneHiddenArePersisted(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `data-key="Projets"`) || !strings.Contains(page, `data-open="0"`) || !strings.Contains(page, `data-key="Projets/2026" data-depth="1"`) {
		t.Fatalf("arbre absent :\n%s", regexp.MustCompile(`(?s)<ul class="folder-list usr".*?</ul>`).FindString(page))
	}
	if !regexp.MustCompile(`data-key="Projets/2026" data-depth="1"[^>]* hidden`).MatchString(page) {
		t.Error("un sous-dossier d'un niveau replié doit être caché au départ")
	}
	post := func(path string, v url.Values) int {
		v.Set("csrf", tok)
		resp, _ := e.do(c, "POST", path, v, map[string]string{"X-Requested-With": "fetch"})
		return resp.StatusCode
	}
	if st := post("/settings/folder-open", url.Values{"folder": {"Projets"}, "open": {"1"}}); st != 204 {
		t.Fatalf("dépliage : %d", st)
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	if regexp.MustCompile(`data-key="Projets/2026" data-depth="1"[^>]* hidden`).MatchString(page) || !strings.Contains(page, `data-key="Projets" data-depth="0" data-open="1"`) {
		t.Error("l'état déplié n'est pas conservé")
	}
	if st := post("/settings/folder-open", url.Values{"folder": {"Projets"}, "open": {"0"}}); st != 204 {
		t.Fatalf("repliage : %d", st)
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `data-key="Projets" data-depth="0" data-open="0"`) {
		t.Error("l'état replié n'est pas conservé")
	}
	// requêtes invalides
	for _, v := range []url.Values{{"folder": {""}, "open": {"1"}}, {"folder": {"x"}, "open": {"2"}}, {"folder": {"a\r\nb"}, "open": {"1"}}} {
		if st := post("/settings/folder-open", v); st != 400 {
			t.Errorf("%v : %d", v, st)
		}
	}
	if resp, _ := e.do(c, "POST", "/settings/folder-open", url.Values{"csrf": {"x"}, "folder": {"Projets"}, "open": {"1"}}, nil); resp.StatusCode != 403 {
		t.Errorf("sans CSRF : %d", resp.StatusCode)
	}

	// panneau masqué
	if strings.Contains(page, "folders-collapsed") {
		t.Error("panneau masqué au départ")
	}
	if st := post("/settings/panes", url.Values{"hidden": {"1"}}); st != 204 {
		t.Fatalf("masquer : %d", st)
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `<body class="mailpage folders-collapsed">`) {
		t.Errorf("classe du panneau masqué absente : %s", regexp.MustCompile(`<body[^>]*>`).FindString(page))
	}
	if st := post("/settings/panes", url.Values{"hidden": {"0"}}); st != 204 {
		t.Fatalf("afficher : %d", st)
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	if strings.Contains(page, "folders-collapsed") {
		t.Error("panneau resté masqué")
	}
	if st := post("/settings/panes", url.Values{"hidden": {"oui"}}); st != 400 {
		t.Errorf("valeur invalide : %d", st)
	}
	// la largeur seule fonctionne toujours
	if st := post("/settings/panes", url.Values{"folders": {"300"}}); st != 204 {
		t.Errorf("largeur : %d", st)
	}
}

func TestOpenStateIsPerMailbox(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	e.do(c, "POST", "/settings/folder-open", url.Values{"csrf": {tok}, "folder": {"Projets"}, "open": {"1"}}, nil)
	// une seconde boîte mail n'hérite pas de l'état
	ports := startMail(t)
	form := accountForm(tok, ports, "save")
	form.Set("email", "second@localhost")
	resp, _ := e.do(c, "POST", "/accounts/save", form, nil)
	e.expect(resp, 303, "")
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	ids := regexp.MustCompile(`/accounts/([a-f0-9]+)/edit`).FindAllStringSubmatch(accounts, -1)
	if len(ids) != 2 {
		t.Fatalf("%d boîtes", len(ids))
	}
	e.do(c, "POST", "/accounts/select", url.Values{"csrf": {tok}, "id": {ids[1][1]}}, nil)
	_, page := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `data-key="Projets" data-depth="0" data-open="0"`) {
		t.Error("l'état de dépliage d'une boîte ne doit pas s'appliquer à une autre")
	}
}

func TestCreateFolderFromThePanel(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/", nil, nil)
	for _, want := range []string{`id="newfolder"`, `name="new_name"`, `<select name="parent">`, `<option value="Projets/2026">`, `data-newfolder`, `href="/settings/folders#create"`} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent du panneau", want)
		}
	}
	form := url.Values{"csrf": {tok}, "action": {"create"}, "new_name": {"Archives"}, "parent": {"Projets"}, "delim": {"/"}, "return_to": {"/?folder=Sent"}}
	resp, _ := e.do(c, "POST", "/settings/folders/action", form, nil)
	e.expect(resp, 303, "/?folder=Sent")
	_, page = e.do(c, "GET", "/?folder=Projets%2FArchives", nil, nil)
	if !strings.Contains(page, `data-key="Projets/Archives"`) || !strings.Contains(page, "Dossier créé.") {
		t.Error("dossier créé absent du panneau ou confirmation manquante")
	}
	// retour hors application refusé : redirection ramenée à « / »
	form.Set("new_name", "Autre")
	form.Set("return_to", "https://evil.example/")
	resp, _ = e.do(c, "POST", "/settings/folders/action", form, nil)
	e.expect(resp, 303, "/")
}

func TestFolderSettingsAreExplicitlyPerMailbox(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	ports := startMail(t)
	form := accountForm(tok, ports, "save")
	form.Set("email", "second@localhost")
	e.do(c, "POST", "/accounts/save", form, nil)
	_, accounts := e.do(c, "GET", "/accounts", nil, nil)
	ids := regexp.MustCompile(`/accounts/([a-f0-9]+)/edit`).FindAllStringSubmatch(accounts, -1)
	first, second := ids[0][1], ids[1][1]

	_, page := e.do(c, "GET", "/settings/folders?box="+second, nil, nil)
	if !strings.Contains(page, "second@localhost") || !strings.Contains(page, `name="box" value="`+second+`"`) || !strings.Contains(page, `/settings/folders?box=`+first) {
		t.Errorf("la page doit nommer la boîte, la mémoriser dans ses formulaires et proposer l'autre :\n%s", regexp.MustCompile(`(?s)<h1.*?</p>`).FindString(page))
	}
	// choisir un dossier système pour la 2e boîte ne touche pas la 1re
	resp, _ := e.do(c, "POST", "/settings/folders/system", url.Values{"csrf": {tok}, "box": {second}, "sent": {"Projets"}, "drafts": {""}, "trash": {""}, "junk": {""}, "archive": {""}}, nil)
	expectRedirectPrefix(t, resp, "/settings/folders?box="+second)
	_, e1 := e.do(c, "GET", "/accounts/"+first+"/edit", nil, nil)
	_, e2 := e.do(c, "GET", "/accounts/"+second+"/edit", nil, nil)
	if selectedValue(e1, "sent_folder") != "" || selectedValue(e2, "sent_folder") != "Projets" {
		t.Errorf("dossier des envoyés : boîte 1 = %q, boîte 2 = %q", selectedValue(e1, "sent_folder"), selectedValue(e2, "sent_folder"))
	}
	// boîte inconnue ou d'un autre utilisateur
	if resp, _ := e.do(c, "GET", "/settings/folders?box=inconnue", nil, nil); resp.StatusCode != 404 {
		t.Errorf("boîte inconnue : %d", resp.StatusCode)
	}
	other, otherTok := e.loggedIn("bob")
	if resp, _ := e.do(other, "POST", "/settings/folders/system", url.Values{"csrf": {otherTok}, "box": {first}, "sent": {"X"}}, nil); resp.StatusCode != 404 {
		t.Errorf("boîte d'un autre utilisateur : %d", resp.StatusCode)
	}
}

func TestDisplayPreferencesSetBodyClasses(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	_, page := e.do(c, "GET", "/settings", nil, nil)
	if !regexp.MustCompile(`<body class="[^"]*">`).MatchString(page) || strings.Contains(page, "hide-checks") || strings.Contains(page, "no-anim") {
		t.Fatalf("par défaut : cases et animations actives")
	}
	prefs := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"rainloop"}, "idle_minutes": {"30"}, "absolute_hours": {"8"}, "page_size": {"50"}}
	e.do(c, "POST", "/settings/preferences", prefs, nil) // cases et animations non cochées : désactivées
	_, page = e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(page, "hide-checks") || !strings.Contains(page, "no-anim") {
		t.Errorf("classes absentes après désactivation :\n%s", page[:min(len(page), 900)])
	}
	prefs.Set("show_checkboxes", "1")
	prefs.Set("animations", "1")
	e.do(c, "POST", "/settings/preferences", prefs, nil)
	_, page = e.do(c, "GET", "/settings", nil, nil)
	if strings.Contains(page, "hide-checks") || strings.Contains(page, "no-anim") {
		t.Error("classes encore présentes après réactivation")
	}
}

func TestPerUserTimezoneChangesDisplayedDates(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Timezone = "Europe/Paris" })
	c, tok := e.loggedIn("alice")
	_, page := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(page, `name="timezone"`) || !strings.Contains(page, `placeholder="Europe/Paris"`) {
		t.Fatalf("champ du fuseau absent ou défaut non indiqué")
	}
	prefs := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"rainloop"}, "idle_minutes": {"30"}, "absolute_hours": {"8"}, "page_size": {"50"}, "show_checkboxes": {"1"}, "animations": {"1"}, "timezone": {"Mars/Olympus"}}
	if resp, _ := e.do(c, "POST", "/settings/preferences", prefs, nil); resp.StatusCode != 400 {
		t.Errorf("fuseau inconnu accepté : %d", resp.StatusCode)
	}
	prefs.Set("timezone", "America/Montreal")
	if resp, _ := e.do(c, "POST", "/settings/preferences", prefs, nil); resp.StatusCode != 200 {
		t.Fatalf("fuseau valide refusé : %d", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(page, `value="America/Montreal"`) {
		t.Error("fuseau non conservé")
	}
}

func TestUserLocPrefersUserThenConfigThenSystem(t *testing.T) {
	e := newEnv(t, func(cfg *config.Config) { cfg.Timezone = "Europe/Paris" })
	if got := e.srv.userLoc(nil).String(); got != "Europe/Paris" {
		t.Errorf("config : %s", got)
	}
	e.srv.cfg.Timezone = "n'existe pas"
	if got := e.srv.userLoc(nil); got != time.Local {
		t.Errorf("repli système : %v", got)
	}
}

func TestFolderDeleteGuards(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/settings/folders", nil, nil)
	box := regexp.MustCompile(`name="box" value="([^"]+)"`).FindStringSubmatch(page)[1]
	// dossiers système et dossiers avec des messages : pas de bouton supprimer
	rowOfFolder := func(page, name string) string {
		for _, row := range strings.Split(page, "<tr>") {
			if i := strings.Index(row, "</tr>"); i >= 0 && strings.Contains(row[:i], `value="`+name+`"`) {
				return row[:i]
			}
		}
		return ""
	}
	for _, name := range []string{"Trash", "Sent", "Drafts"} {
		if r := rowOfFolder(page, name); r != "" && strings.Contains(r, `value="delete"`) {
			t.Errorf("%s : suppression proposée pour un dossier système", name)
		}
	}
	del := func(name string) (int, string) {
		resp, body := e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "box": {box}, "action": {"delete"}, "name": {name}}, nil)
		return resp.StatusCode, body
	}
	if code, body := del("Trash"); code != 400 || !strings.Contains(body, "rôle système") {
		t.Errorf("suppression de la corbeille : %d", code)
	}
	if code, body := del("Projets"); code != 400 || !strings.Contains(body, "sous-dossiers") {
		t.Errorf("dossier avec sous-dossiers : %d %s", code, body[:min(len(body), 200)])
	}
	if code, body := del("INBOX"); code != 400 || !strings.Contains(body, "protégé") && !strings.Contains(body, "système") {
		t.Errorf("INBOX : %d", code)
	}
	// un dossier vide et sans enfant se supprime
	e.do(c, "POST", "/settings/folders/action", url.Values{"csrf": {tok}, "box": {box}, "action": {"create"}, "new_name": {"Jetable"}, "delim": {"/"}}, nil)
	if code, body := del("Jetable"); code != 303 {
		t.Errorf("dossier vide : %d %s", code, body[:min(len(body), 200)])
	}
}

func TestMainIdentityReplyToAndBcc(t *testing.T) {
	e, c, tok, smtp := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/accounts", nil, nil)
	id := regexp.MustCompile(`/accounts/([0-9a-f]{24})/edit`).FindStringSubmatch(page)[1]
	_, edit := e.do(c, "GET", "/accounts/"+id+"/edit", nil, nil)
	if !strings.Contains(edit, `name="reply_to"`) || !strings.Contains(edit, `name="bcc"`) {
		t.Fatal("champs « Répondre à » et « Cci » absents du formulaire de boîte")
	}
	form := accountForm(tok, mailPorts{}, "save")
	form.Set("id", id)
	for _, k := range []string{"imap_host", "imap_port", "imap_security", "imap_login", "smtp_host", "smtp_port", "smtp_security"} {
		form.Set(k, fieldValue(edit, k))
	}
	form.Set("imap_security", "none")
	form.Set("smtp_security", "none")
	form.Set("reply_to", "pas une adresse")
	if resp, _ := e.do(c, "POST", "/accounts/save", form, nil); resp.StatusCode != 400 {
		t.Errorf("adresse de réponse invalide : %d", resp.StatusCode)
	}
	form.Set("reply_to", "Retour <retour@x.fr>")
	form.Set("bcc", "archive@x.fr")
	e.expect1(e.do(c, "POST", "/accounts/save", form, nil))
	// un message envoyé avec l'identité principale porte Reply-To et la copie cachée
	_, page = e.do(c, "GET", "/compose", nil, nil)
	f := sendForm(tok, composeID(t, page), url.Values{"identity": {identityID(t, page)}, "to": {"dest@example.org"}})
	e.expect1(e.do(c, "POST", "/compose/send", f, nil))
	msgs := smtp.Messages()
	last := msgs[len(msgs)-1]
	if !strings.Contains(string(last.Data), "Reply-To: \"Retour\" <retour@x.fr>") && !strings.Contains(string(last.Data), "Reply-To: Retour <retour@x.fr>") {
		t.Errorf("en-tête Reply-To absent :\n%s", headOf(string(last.Data)))
	}
	if !slicesContains(last.Rcpt, "archive@x.fr") {
		t.Errorf("copie cachée absente de l'enveloppe : %v", last.Rcpt)
	}
	if strings.Contains(headOf(string(last.Data)), "archive@x.fr") {
		t.Error("la copie cachée ne doit pas figurer dans les en-têtes envoyés")
	}
}

func headOf(raw string) string {
	if i := strings.Index(raw, "\r\n\r\n"); i >= 0 {
		return raw[:i]
	}
	return raw
}

// Le choix « Photo du correspondant » doit être conservé et réaffiché.
func TestAvatarsPreferenceRoundTrip(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	form := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"rainloop"}, "idle_minutes": {"30"},
		"absolute_hours": {"8"}, "page_size": {"50"}, "avatars": {"gravatar"}}
	resp, _ := e.do(c, "POST", "/settings/preferences", form, nil)
	if resp.StatusCode >= 400 {
		t.Fatalf("enregistrement : statut %d", resp.StatusCode)
	}
	_, page := e.do(c, "GET", "/settings", nil, nil)
	if !regexp.MustCompile(`<option value="gravatar" selected`).MatchString(page) {
		t.Errorf("le choix « gravatar » doit rester sélectionné :\n%s", page)
	}
	form.Set("avatars", "none")
	e.do(c, "POST", "/settings/preferences", form, nil)
	_, page = e.do(c, "GET", "/settings", nil, nil)
	if !regexp.MustCompile(`<option value="none" selected`).MatchString(page) {
		t.Error("le retour à « aucune » doit être conservé")
	}
}
