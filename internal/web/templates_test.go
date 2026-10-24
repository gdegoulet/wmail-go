package web

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestMessageTemplatesLifecycle(t *testing.T) {
	e, c, tok, _ := composeEnv(t, nil)
	_, page := e.do(c, "GET", "/compose", nil, nil)
	if strings.Contains(page, "data-template=") {
		t.Error("bouton de modèle sans modèle")
	}
	resp, page := e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "name": {""}, "body": {"x"}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "obligatoires") {
		t.Errorf("nom vide : %d", resp.StatusCode)
	}
	resp, _ = e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "name": {"Accusé <b>"}, "body": {"Bonjour,\r\nLe {{MOMENT:YYYY}}.\r\n"}}, nil)
	e.expect(resp, 303, "/settings/templates")
	_, page = e.do(c, "GET", "/settings/templates", nil, nil)
	m := regexp.MustCompile(`edit=([0-9a-f]{16})#form`).FindStringSubmatch(page)
	if m == nil || !strings.Contains(page, "Accusé &lt;b&gt;") {
		t.Fatalf("liste :\n%s", page)
	}
	id := m[1]
	_, page = e.do(c, "GET", "/compose", nil, nil)
	if !strings.Contains(page, `data-template="`+id+`"`) {
		t.Error("le modèle n'est pas proposé dans la composition")
	}
	resp, body := e.do(c, "GET", "/settings/templates/body?id="+id, nil, nil)
	if resp.StatusCode != 200 || !strings.HasPrefix(body, "Bonjour,\nLe 20") || strings.Contains(body, "{{") {
		t.Errorf("corps : %q", body)
	}
	if resp, _ = e.do(c, "GET", "/settings/templates/body?id=nope", nil, nil); resp.StatusCode != 404 {
		t.Errorf("modèle inconnu : %d", resp.StatusCode)
	}
	// modification
	_, page = e.do(c, "GET", "/settings/templates?edit="+id, nil, nil)
	if !strings.Contains(page, `name="id" value="`+id+`"`) {
		t.Error("formulaire d'édition non prérempli")
	}
	e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "id": {id}, "name": {"Renommé"}, "body": {"Nouveau"}}, nil)
	_, body = e.do(c, "GET", "/settings/templates/body?id="+id, nil, nil)
	if body != "Nouveau" {
		t.Errorf("après modification : %q", body)
	}
	if resp, _ = e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "id": {"inconnu"}, "name": {"x"}, "body": {"y"}}, nil); resp.StatusCode != 404 {
		t.Errorf("id inconnu : %d", resp.StatusCode)
	}
	// suppression
	e.do(c, "POST", "/settings/templates/delete", url.Values{"csrf": {tok}, "id": {id}}, nil)
	if resp, _ = e.do(c, "GET", "/settings/templates/body?id="+id, nil, nil); resp.StatusCode != 404 {
		t.Error("modèle encore présent après suppression")
	}
}

func TestMessageTemplatesLimit(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	for i := 0; i < maxTemplates; i++ {
		resp, _ := e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "name": {"m"}, "body": {"b"}}, nil)
		e.expect(resp, 303, "")
	}
	resp, page := e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "name": {"m"}, "body": {"b"}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "Nombre maximal") {
		t.Errorf("limite : %d", resp.StatusCode)
	}
}

func TestMailboxAndIdentityOrder(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	first := startMail(t)
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, first, "save"), nil))
	second := accountForm(tok, first, "save")
	second.Set("email", "second@localhost")
	e.expect1(e.do(c, "POST", "/accounts/save", second, nil))
	ids := func() []string {
		_, page := e.do(c, "GET", "/accounts", nil, nil)
		var out []string
		for _, m := range regexp.MustCompile(`action="/accounts/([0-9a-f]{24})/move"`).FindAllStringSubmatch(page, -1) {
			out = append(out, m[1])
		}
		return out
	}
	before := ids()
	if len(before) != 2 {
		t.Fatalf("boîtes : %v", before)
	}
	e.expect1(e.do(c, "POST", "/accounts/"+before[1]+"/move", url.Values{"csrf": {tok}, "dir": {"up"}}, nil))
	after := ids()
	if after[0] != before[1] || after[1] != before[0] {
		t.Errorf("ordre après « monter » : %v, attendu %v", after, []string{before[1], before[0]})
	}
	// déjà premier : sans effet ; direction invalide : refusée
	e.expect1(e.do(c, "POST", "/accounts/"+after[0]+"/move", url.Values{"csrf": {tok}, "dir": {"up"}}, nil))
	if again := ids(); again[0] != after[0] {
		t.Error("le premier ne peut pas monter")
	}
	if resp, _ := e.do(c, "POST", "/accounts/"+after[0]+"/move", url.Values{"csrf": {tok}, "dir": {"sideways"}}, nil); resp.StatusCode != 404 {
		t.Errorf("direction invalide : %d", resp.StatusCode)
	}
	// identités supplémentaires
	for _, mail := range []string{"a@x.fr", "b@x.fr"} {
		e.do(c, "POST", "/accounts/"+after[0]+"/identities", url.Values{"csrf": {tok}, "email": {mail}, "name": {mail}}, nil)
	}
	_, page := e.do(c, "GET", "/accounts/"+after[0]+"/identities", nil, nil)
	if strings.Index(page, "a@x.fr") > strings.Index(page, "b@x.fr") {
		t.Fatal("ordre initial des identités")
	}
	iid := regexp.MustCompile(`identities/([0-9a-f]+)/move`).FindAllStringSubmatch(page, -1)
	e.expect1(e.do(c, "POST", "/accounts/"+after[0]+"/identities/"+iid[1][1]+"/move", url.Values{"csrf": {tok}, "dir": {"up"}}, nil))
	_, page = e.do(c, "GET", "/accounts/"+after[0]+"/identities", nil, nil)
	if strings.Index(page, "b@x.fr") > strings.Index(page, "a@x.fr") {
		t.Error("identité non remontée")
	}
}
