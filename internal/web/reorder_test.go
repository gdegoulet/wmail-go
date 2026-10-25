package web

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// dataIDs renvoie les identifiants des lignes réordonnables d'une page, dans l'ordre d'affichage.
var dataIDRe = regexp.MustCompile(`data-id="([0-9a-f]+)"`)

func dataIDs(page string) []string {
	var out []string
	for _, m := range dataIDRe.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

// threeAccounts crée trois boîtes mail et renvoie leurs identifiants dans l'ordre de la page des comptes.
func threeAccounts(t *testing.T, e *env, c *httpClient, tok string) []string {
	t.Helper()
	ports := startMail(t)
	for _, addr := range []string{"un@localhost", "deux@localhost", "trois@localhost"} {
		v := accountForm(tok, ports, "save")
		v.Set("email", addr)
		e.expect1(e.do(c, "POST", "/accounts/save", v, nil))
	}
	_, page := e.do(c, "GET", "/accounts", nil, nil)
	ids := dataIDs(page)
	if len(ids) != 3 {
		t.Fatalf("trois boîtes attendues, %v :\n%s", ids, page)
	}
	return ids
}

func accountOrder(e *env, c *httpClient) []string {
	_, page := e.do(c, "GET", "/accounts", nil, nil)
	return dataIDs(page)
}

func TestAccountsReorderDragDrop(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	ids := threeAccounts(t, e, c, tok)

	// La page porte bien les attributs attendus par le script.
	_, page := e.do(c, "GET", "/accounts", nil, nil)
	if !strings.Contains(page, `data-reorder="accounts"`) || !strings.Contains(page, `data-csrf="`+tok+`"`) ||
		!strings.Contains(page, `src="/static/reorder.js?v=`) {
		t.Errorf("attributs de réordonnancement absents :\n%s", page)
	}

	// Ordre complet : la liste suit exactement ce qui a été envoyé.
	want := []string{ids[2], ids[0], ids[1]}
	resp, _ := e.do(c, "POST", "/accounts/reorder", url.Values{"csrf": {tok}, "order": {strings.Join(want, ",")}}, nil)
	e.expect(resp, 303, "/accounts")
	if got := accountOrder(e, c); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordre %v, attendu %v", got, want)
	}

	// Appel depuis le script : réponse JSON, pas de redirection.
	want2 := []string{want[1], want[2], want[0]}
	resp, body := e.do(c, "POST", "/accounts/reorder", url.Values{"csrf": {tok}, "order": {strings.Join(want2, ",")}},
		map[string]string{"X-Requested-With": "fetch"})
	e.expect(resp, 200, "")
	if strings.TrimSpace(body) != `{"ok":true}` {
		t.Errorf("réponse JSON %q", body)
	}
	if got := accountOrder(e, c); strings.Join(got, ",") != strings.Join(want2, ",") {
		t.Fatalf("ordre %v, attendu %v", got, want2)
	}
}

func TestAccountsReorderRejected(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	ids := threeAccounts(t, e, c, tok)
	before := strings.Join(ids, ",")

	cases := map[string]string{
		"liste incomplète":    strings.Join(ids[:2], ","),
		"liste trop longue":   before + "," + ids[0],
		"identifiant inconnu": ids[0] + ",ffffffffffffffffffffffff," + ids[2],
		"doublon":             ids[0] + "," + ids[0] + "," + ids[1],
		"liste vide":          "",
		"identifiant vide":    ids[0] + ",," + ids[2],
		"identité d'un autre": "a,b,c",
	}
	for name, order := range cases {
		resp, page := e.do(c, "POST", "/accounts/reorder", url.Values{"csrf": {tok}, "order": {order}}, nil)
		if resp.StatusCode != 400 {
			t.Errorf("%s : statut %d, attendu 400", name, resp.StatusCode)
		}
		if !strings.Contains(page, `role="alert"`) {
			t.Errorf("%s : message d'erreur absent", name)
		}
		if got := strings.Join(accountOrder(e, c), ","); got != before {
			t.Fatalf("%s : ordre modifié (%s, attendu %s)", name, got, before)
		}
	}
}

func TestReorderNeedsCSRF(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	ids := threeAccounts(t, e, c, tok)
	order := strings.Join([]string{ids[2], ids[1], ids[0]}, ",")

	for _, path := range []string{"/accounts/reorder", "/settings/identities/reorder", "/settings/templates/reorder"} {
		for name, v := range map[string]url.Values{
			"sans jeton":   {"order": {order}},
			"jeton erroné": {"csrf": {"faux"}, "order": {order}},
		} {
			resp, _ := e.do(c, "POST", path, v, nil)
			if resp.StatusCode != 403 {
				t.Errorf("%s %s : statut %d, attendu 403", path, name, resp.StatusCode)
			}
		}
	}
	if got := strings.Join(accountOrder(e, c), ","); got != strings.Join(ids, ",") {
		t.Errorf("ordre modifié sans jeton CSRF : %s", got)
	}
}

func TestIdentitiesReorderDragDrop(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	acc := threeAccounts(t, e, c, tok)[0]
	for _, addr := range []string{"a@x.fr", "b@x.fr", "c@x.fr"} {
		e.expect1(e.do(c, "POST", "/accounts/"+acc+"/identities", url.Values{"csrf": {tok}, "email": {addr}, "name": {addr}}, nil))
	}
	order := func() []string {
		_, page := e.do(c, "GET", "/accounts/"+acc+"/identities", nil, nil)
		return dataIDs(page)
	}
	ids := order()
	if len(ids) != 3 {
		t.Fatalf("trois identités supplémentaires attendues, %v", ids)
	}
	want := []string{ids[2], ids[1], ids[0]}
	resp, _ := e.do(c, "POST", "/settings/identities/reorder",
		url.Values{"csrf": {tok}, "box": {acc}, "order": {strings.Join(want, ",")}}, nil)
	e.expect(resp, 303, "/accounts/"+acc+"/identities")
	if got := order(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordre %v, attendu %v", got, want)
	}

	// Liste invalide : refusée, ordre inchangé.
	resp, _ = e.do(c, "POST", "/settings/identities/reorder",
		url.Values{"csrf": {tok}, "box": {acc}, "order": {want[0] + "," + want[0] + "," + want[1]}}, nil)
	if resp.StatusCode != 400 {
		t.Errorf("doublon : statut %d, attendu 400", resp.StatusCode)
	}
	if got := order(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ordre modifié : %v", got)
	}

	// Boîte inconnue : 404.
	resp, _ = e.do(c, "POST", "/settings/identities/reorder",
		url.Values{"csrf": {tok}, "box": {"inconnue"}, "order": {strings.Join(want, ",")}}, nil)
	if resp.StatusCode != 404 {
		t.Errorf("boîte inconnue : statut %d, attendu 404", resp.StatusCode)
	}
}

func TestTemplatesReorderDragDrop(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	for _, n := range []string{"Un", "Deux", "Trois"} {
		e.expect1(e.do(c, "POST", "/settings/templates/save", url.Values{"csrf": {tok}, "name": {n}, "body": {"corps " + n}}, nil))
	}
	order := func() []string {
		_, page := e.do(c, "GET", "/settings/templates", nil, nil)
		return dataIDs(page)
	}
	ids := order()
	if len(ids) != 3 {
		t.Fatalf("trois modèles attendus, %v", ids)
	}
	want := []string{ids[1], ids[2], ids[0]}
	resp, body := e.do(c, "POST", "/settings/templates/reorder",
		url.Values{"csrf": {tok}, "order": {strings.Join(want, ",")}}, map[string]string{"X-Requested-With": "fetch"})
	e.expect(resp, 200, "")
	if strings.TrimSpace(body) != `{"ok":true}` {
		t.Errorf("réponse JSON %q", body)
	}
	if got := order(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordre %v, attendu %v", got, want)
	}

	resp, _ = e.do(c, "POST", "/settings/templates/reorder",
		url.Values{"csrf": {tok}, "order": {strings.Join(want[:2], ",")}}, nil)
	if resp.StatusCode != 400 {
		t.Errorf("liste incomplète : statut %d, attendu 400", resp.StatusCode)
	}
	if got := order(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ordre modifié : %v", got)
	}
}
