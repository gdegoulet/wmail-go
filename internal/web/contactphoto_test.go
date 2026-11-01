package web

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

// Aller-retour complet de la photo d'un contact : enregistrement par le formulaire, affichage dans la fiche
// rouverte, puis retrait. Le champ « photo » est le seul du formulaire que l'utilisateur ne saisit pas
// lui-même — il est rempli par contacts-photo.js — donc rien ne le couvrait jusqu'ici.
func TestPhotoDeContactAllerRetour(t *testing.T) {
	e := newEnv(t, nil)
	c, tok := e.loggedIn("alice")
	photo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNG(8, 8))

	form := url.Values{"csrf": {tok}, "first": {"Camille"}, "last": {"Roux"},
		"email": {"camille@example.org", ""}, "email_type": {"work", ""}, "adr_street": {""}, "photo": {photo}}
	resp, _ := e.do(c, "POST", "/contacts/save", form, nil)
	if resp.StatusCode != 303 {
		t.Fatalf("enregistrement : %d", resp.StatusCode)
	}
	id := strings.TrimPrefix(resp.Header.Get("Location"), "/contacts?info=saved&edit=")

	_, page := e.do(c, "GET", "/contacts?edit="+id, nil, nil)
	if !strings.Contains(page, `<img src="`+photo+`"`) {
		t.Errorf("la photo n'est pas affichée dans la fiche rouverte :\n%s", extraitPhoto(page))
	}
	if !strings.Contains(page, `name="photo"`) || !strings.Contains(page, `value="`+photo+`"`) {
		t.Error("la photo doit être reportée dans le champ caché, sinon un simple enregistrement l'efface")
	}

	// Retrait : le champ caché revient à vide, la fiche n'a plus d'image.
	vide := url.Values{"csrf": {tok}, "id": {id}, "first": {"Camille"}, "last": {"Roux"},
		"email": {"camille@example.org", ""}, "email_type": {"work", ""}, "adr_street": {""}, "photo": {""}}
	e.do(c, "POST", "/contacts/save", vide, nil)
	if _, page = e.do(c, "GET", "/contacts?edit="+id, nil, nil); strings.Contains(page, "data:image/png;base64,") {
		t.Error("la photo retirée reste affichée")
	}
}

func extraitPhoto(page string) string {
	i := strings.Index(page, "<p class=\"c-photo\"")
	if i < 0 {
		return "(aucune zone photo dans la page)"
	}
	return page[max(0, i-120):min(len(page), i+200)]
}
