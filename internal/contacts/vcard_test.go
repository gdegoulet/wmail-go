package contacts

import (
	"bytes"
	"strings"
	"testing"
)

const sample = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:u1\r\nFN:Zoë Dupont\r\nN:Dupont;Zoë;;;\r\nEMAIL;TYPE=WORK:Zoe@Example.org\r\nEMAIL:zoe@example.org\r\nTEL;TYPE=CELL:+33 6 00 00 00 00\r\nADR;TYPE=HOME:;;1 rue X;Lyon;;69000;France\r\nBDAY:1980-02-03\r\nCATEGORIES:amis,travail\r\nNOTE:ligne1\\nligne2\r\nEND:VCARD\r\nBEGIN:VCARD\r\nVERSION:3.0\r\nFN:\r\nEND:VCARD\r\n"

func TestVCardRoundTrip(t *testing.T) {
	list, skipped, err := ParseVCards(strings.NewReader(sample))
	if err != nil || len(list) != 1 || skipped != 1 {
		t.Fatalf("parse : %v %d %v", list, skipped, err)
	}
	c := list[0]
	if c.Display() != "Zoë Dupont" || len(c.Emails) != 1 || c.Emails[0].Value != "zoe@example.org" || c.Emails[0].Type != "work" {
		t.Errorf("fiche : %+v", c)
	}
	if c.Phones[0].Type != "cell" || c.Postals[0].City != "Lyon" || c.Birthday != "1980-02-03" || len(c.Tags) != 2 || c.Notes != "ligne1\nligne2" {
		t.Errorf("champs : %+v", c)
	}
	var out bytes.Buffer
	if err := EncodeVCard(&out, list); err != nil {
		t.Fatal(err)
	}
	back, _, err := ParseVCards(&out)
	if err != nil || len(back) != 1 || back[0].Display() != c.Display() || back[0].Postals[0].Street != "1 rue X" || back[0].UID != "u1" {
		t.Errorf("aller-retour : %+v %v", back, err)
	}
	if _, _, err := ParseVCards(strings.NewReader("pas une vCard")); err == nil {
		t.Error("entrée invalide acceptée")
	}
}

func TestVCardPhotoRoundTrip(t *testing.T) {
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	in := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Zoé\r\nEMAIL:zoe@x.fr\r\nPHOTO;ENCODING=b;TYPE=PNG:" + png + "\r\nEND:VCARD\r\n"
	list, _, err := ParseVCards(strings.NewReader(in))
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if list[0].Photo != "data:image/png;base64,"+png {
		t.Fatalf("photo lue : %.40s", list[0].Photo)
	}
	data, mime, ok := list[0].PhotoBytes()
	if !ok || mime != "image/png" || len(data) == 0 {
		t.Errorf("décodage : %v %q %d", ok, mime, len(data))
	}
	var out bytes.Buffer
	if err := EncodeVCard(&out, list); err != nil {
		t.Fatal(err)
	}
	back, _, _ := ParseVCards(&out)
	if len(back) != 1 || back[0].Photo != list[0].Photo {
		t.Errorf("aller-retour : %.40s", back[0].Photo)
	}
	// une photo donnée par une adresse web n'est pas rapatriée (elle révélerait l'adresse IP du lecteur)
	ext := "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:X\r\nEMAIL:x@y.fr\r\nPHOTO;VALUE=URI:https://exemple.fr/p.png\r\nEND:VCARD\r\n"
	l2, _, _ := ParseVCards(strings.NewReader(ext))
	if len(l2) != 1 || l2[0].Photo != "" {
		t.Errorf("photo externe : %q", l2[0].Photo)
	}
}
