package contacts

import (
	"bytes"
	"strings"
	"testing"
)

func mk(first, last, mail string) *Contact {
	c := &Contact{First: first, Last: last}
	if mail != "" {
		c.Emails = []Typed{{Value: mail}}
	}
	return c
}

func TestNormalizeAndSave(t *testing.T) {
	b := &Book{}
	if err := b.Save(&Contact{}); err != ErrEmpty {
		t.Errorf("fiche vide : %v", err)
	}
	c := &Contact{First: " Zoë\n", Emails: []Typed{{Value: "Zoe@X.org"}, {Value: "zoe@x.org"}, {Value: "pas une adresse"}}, Birthday: "n'importe quoi", Tags: []string{"A", "a", "b,c"}}
	if err := b.Save(c); err != nil {
		t.Fatal(err)
	}
	if c.First != "Zoë" || len(c.Emails) != 1 || c.Birthday != "" || len(c.Tags) != 2 || c.ID == "" || c.UID == "" {
		t.Errorf("normalisation : %+v", c)
	}
	c.Frec = 7
	b.Bump("ZOE@x.org")
	if c.Frec != 8 {
		t.Errorf("frec : %d", c.Frec)
	}
	upd := &Contact{ID: c.ID, First: "Zoé"}
	if err := b.Save(upd); err != nil || upd.Frec != 8 || upd.UID != c.UID {
		t.Errorf("mise à jour : %+v %v", upd, err)
	}
	if err := b.Save(&Contact{ID: "inconnu", First: "x"}); err != ErrNotFound {
		t.Errorf("id inconnu : %v", err)
	}
	if b.Delete(c.ID, "nope") != 1 || len(b.Contacts) != 0 {
		t.Error("suppression")
	}
}

func TestLimits(t *testing.T) {
	b := &Book{}
	for i := 0; i < MaxContacts; i++ {
		b.Contacts = append(b.Contacts, &Contact{ID: newID()})
	}
	if err := b.Save(mk("a", "b", "")); err != ErrFull {
		t.Errorf("carnet plein : %v", err)
	}
}

func TestSearchSortPage(t *testing.T) {
	b := &Book{}
	for _, c := range []*Contact{mk("Émile", "Zola", "e@z.fr"), mk("Anne", "Durand", "a@d.fr"), mk("Bob", "Éclair", "")} {
		c.Tags = []string{"Amis"}
		if err := b.Save(c); err != nil {
			t.Fatal(err)
		}
	}
	l, n := b.Search("", "", 1)
	if n != 3 || l[0].Last != "Durand" || l[1].Last != "Éclair" || l[2].Last != "Zola" {
		t.Errorf("tri : %v", l)
	}
	if l, _ := b.Search("emile", "", 1); len(l) != 1 || l[0].Last != "Zola" {
		t.Errorf("recherche sans accent : %v", l)
	}
	if l, _ := b.Search("D.FR", "amis", 1); len(l) != 1 {
		t.Errorf("recherche + étiquette : %v", l)
	}
	if l, n := b.Search("", "", 2); l != nil || n != 3 {
		t.Errorf("page vide : %v %d", l, n)
	}
	if tags := b.AllTags(); len(tags) != 1 || tags[0] != "Amis" {
		t.Errorf("étiquettes : %v", tags)
	}
}

func TestSuggestAndAutoAdd(t *testing.T) {
	b := &Book{}
	b.Save(mk("Anne", "Durand", "anne@d.fr"))
	b.Save(mk("Annabelle", "Zed", "ann@z.fr"))
	b.Bump("ann@z.fr")
	s := b.Suggest("ann", 10)
	if len(s) != 2 || s[0].Email != "ann@z.fr" || s[1].Label != "Anne Durand <anne@d.fr>" {
		t.Errorf("suggestions : %+v", s)
	}
	if len(b.Suggest("", 5)) != 0 || len(b.Suggest("ann", 1)) != 1 {
		t.Error("limites suggestions")
	}
	if got := FormatAddress(`Durand, "Anne"`, "a@d.fr"); got != `"Durand, \"Anne\"" <a@d.fr>` {
		t.Errorf("citation : %s", got)
	}
	if c, added := b.AddAddress("Jean Paul Martin", "J@M.fr"); !added || c.Last != "Martin" || c.First != "Jean Paul" {
		t.Errorf("ajout auto : %+v", c)
	}
	if _, added := b.AddAddress("", "j@m.fr"); added {
		t.Error("doublon ajouté")
	}
}

func TestPersistence(t *testing.T) {
	b := &Book{}
	b.Save(mk("A", "B", "a@b.fr"))
	data, _ := b.Marshal()
	back, err := Parse(data)
	if err != nil || len(back.Contacts) != 1 || back.Contacts[0].Emails[0].Value != "a@b.fr" {
		t.Errorf("%v %v", back, err)
	}
	if e, err := Parse(nil); err != nil || len(e.Contacts) != 0 {
		t.Error("carnet vide")
	}
	if _, err := Parse([]byte("{")); err == nil {
		t.Error("JSON invalide accepté")
	}
}

func TestCSV(t *testing.T) {
	in := "First Name;Last Name;E-mail Address;Mobile Phone;Birthday;Notes\r\nZoë;Dupont;zoe@x.fr;0600;03/02/1980;=cmd()\r\n;;;;;\r\n"
	list, skipped, err := ParseCSV(strings.NewReader(in))
	if err != nil || len(list) != 1 || skipped != 1 {
		t.Fatalf("%v %d %v", list, skipped, err)
	}
	c := list[0]
	if c.First != "Zoë" || c.Emails[0].Value != "zoe@x.fr" || c.Phones[0].Type != "cell" || c.Birthday != "1980-02-03" {
		t.Errorf("%+v", c)
	}
	// Windows-1252 + en-têtes Google
	latin := []byte("Name,E-mail 1 - Value,Phone 1 - Value\nJos\xe9 Garcia,jose@x.es,123\n")
	l2, _, err := ParseCSV(bytes.NewReader(latin))
	if err != nil || len(l2) != 1 || l2[0].First != "José" || l2[0].Last != "Garcia" {
		t.Errorf("latin1 : %+v %v", l2, err)
	}
	if _, _, err := ParseCSV(strings.NewReader("a,b\n1,2\n")); err != ErrCSV {
		t.Errorf("colonnes inconnues : %v", err)
	}
	var out bytes.Buffer
	if err := EncodeCSV(&out, list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "'=cmd()") {
		t.Errorf("injection de formule non neutralisée : %q", out.String())
	}
	back, _, err := ParseCSV(&out)
	if err != nil || len(back) != 1 || back[0].Emails[0].Value != "zoe@x.fr" {
		t.Errorf("aller-retour : %+v %v", back, err)
	}
}

func TestPhoneSearchByDigitsAndTypedCSVHeaders(t *testing.T) {
	b := &Book{}
	c := &Contact{First: "Anne", Phones: []Typed{{Type: "cell", Value: "+33 6 12 34 56 78"}}}
	if err := b.Save(c); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"0612", "612345", "+33 6 12"} {
		if l, _ := b.Search(q, "", 1); len(l) != 1 {
			t.Errorf("recherche %q : %d résultats", q, len(l))
		}
	}
	if l, _ := b.Search("99", "", 1); len(l) != 0 {
		t.Error("deux chiffres ne doivent pas suffire")
	}
	in := "First Name,Home Email,Business Email,Business Mobile,Home Fax\r\nZoé,perso@x.fr,pro@x.fr,0611,0144\r\n"
	list, _, err := ParseCSV(strings.NewReader(in))
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	got := list[0]
	if len(got.Emails) != 2 || got.Emails[0].Type != "home" || got.Emails[1].Type != "work" {
		t.Errorf("adresses typées : %+v", got.Emails)
	}
	if len(got.Phones) != 2 || got.Phones[0].Type != "cell" || got.Phones[1].Type != "fax" {
		t.Errorf("téléphones typés : %+v", got.Phones)
	}
}

func TestVCardKeepsUnknownPropertiesAndPrefFirst(t *testing.T) {
	in := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Zoé\r\nEMAIL:second@x.fr\r\nEMAIL;TYPE=PREF:premier@x.fr\r\nTEL;TYPE=MOBILE:0600\r\nTEL;TYPE=WORK,FAX:0144\r\nIMPP:xmpp:zoe@x.fr\r\nX-SOCIALPROFILE;TYPE=twitter:@zoe\r\nEND:VCARD\r\n"
	list, _, err := ParseVCards(strings.NewReader(in))
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	c := list[0]
	if c.Emails[0].Value != "premier@x.fr" {
		t.Errorf("l'adresse PREF doit venir en tête : %+v", c.Emails)
	}
	if c.Phones[0].Type != "cell" || c.Phones[1].Type != "work,fax" {
		t.Errorf("types de téléphone : %+v", c.Phones)
	}
	if len(c.Extra) != 2 {
		t.Fatalf("propriétés inconnues gardées : %v", c.Extra)
	}
	var out bytes.Buffer
	if err := EncodeVCard(&out, list); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"IMPP:xmpp:zoe@x.fr", "X-SOCIALPROFILE", "@zoe"} {
		if !strings.Contains(s, want) {
			t.Errorf("%q absent de la vCard réécrite :\n%s", want, s)
		}
	}
}
