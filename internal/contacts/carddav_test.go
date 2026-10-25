package contacts

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-vcard"

	"github.com/gdegoulet/wmail-go/internal/contacts/davtest"
)

// touch simule « une modification faite après le dernier passage » : le temps recule d'une minute pour tout le monde
// sauf pour le contact modifié (les dates sont à la seconde).
func touch(b *Book, c *Contact) {
	b.SyncedAt -= 60
	for _, o := range b.Contacts {
		o.Changed -= 60
	}
	c.Changed = b.SyncedAt + 30
}

func TestCardDAVSyncBothWays(t *testing.T) {
	url, be, hc := davtest.Start(t)
	cfg := SyncConfig{URL: url, User: "alice", Password: "secret"}
	old := time.Now().Add(-time.Hour)
	be.Put("Anne", "anne@x.org", old)
	be.Put("Bob", "bob@x.org", old)
	b := &Book{}
	ctx := context.Background()

	res, err := b.SyncCardDAV(ctx, hc, cfg)
	if err != nil || res.Pulled != 2 || len(b.Contacts) != 2 {
		t.Fatalf("premier passage : %v %v (%d contacts)", res, err, len(b.Contacts))
	}
	for _, c := range b.Contacts {
		if c.Href == "" || c.ETag == "" {
			t.Errorf("contact sans lien : %+v", c)
		}
	}
	// un deuxième passage ne change rien
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil || res.Pulled != 0 || res.Pushed != 0 || res.Deleted != 0 {
		t.Fatalf("passage à vide : %v %v", res, err)
	}

	// contact créé ici → envoyé
	nc := &Contact{First: "Carla", Emails: []Typed{{Value: "carla@x.org"}}}
	if err := b.Save(nc); err != nil {
		t.Fatal(err)
	}
	touch(b, nc)
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil || res.Pushed != 1 {
		t.Fatalf("envoi : %v %v", res, err)
	}
	if got := strings.Join(be.Names(), ","); got != "Anne,Bob,Carla" {
		t.Errorf("serveur : %s", got)
	}
	if nc.Href == "" || nc.ETag == "" {
		t.Errorf("contact envoyé non lié : %+v", nc)
	}

	// modification ici → renvoyée
	anne := b.ByEmail("anne@x.org")
	anne.Notes = "modifiée ici"
	touch(b, anne)
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil || res.Pushed != 1 {
		t.Fatalf("modification locale : %v %v", res, err)
	}
	o, _ := be.GetAddressObject(ctx, anne.Href, nil)
	if o.Card.PreferredValue(vcard.FieldNote) != "modifiée ici" {
		t.Errorf("note non transmise : %v", o.Card[vcard.FieldNote])
	}

	// modification à distance → reprise ici
	be.Put("Bob", "bob2@x.org", time.Now().Add(time.Minute))
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil || res.Pulled != 1 {
		t.Fatalf("modification distante : %v %v", res, err)
	}
	if b.ByEmail("bob2@x.org") == nil || b.ByEmail("bob@x.org") != nil || len(b.Contacts) != 3 {
		t.Errorf("Bob non mis à jour : %d contacts", len(b.Contacts))
	}

	// conflit : modifié des deux côtés, le plus récent (le distant) l'emporte
	carla := b.ByEmail("carla@x.org")
	carla.First = "Carla locale"
	touch(b, carla)
	be.Edit(carla.Href, "Carla distante", time.Now().Add(2*time.Hour))
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil {
		t.Fatal(err)
	}
	if b.ByEmail("carla@x.org").First != "Carla distante" {
		t.Errorf("conflit : %q, la version distante plus récente devait l'emporter", b.ByEmail("carla@x.org").First)
	}
	// conflit inverse : la modification locale est plus récente que celle du serveur
	carla = b.ByEmail("carla@x.org")
	be.Edit(carla.Href, "Ancienne distante", time.Now().Add(-48*time.Hour))
	carla.First = "Carla locale récente"
	touch(b, carla)
	if _, err = b.SyncCardDAV(ctx, hc, cfg); err != nil {
		t.Fatal(err)
	}
	if o, _ := be.GetAddressObject(ctx, carla.Href, nil); !strings.Contains(o.Card.PreferredValue(vcard.FieldFormattedName), "Carla locale récente") {
		t.Errorf("la version locale plus récente devait être envoyée : %s", o.Card.PreferredValue(vcard.FieldFormattedName))
	}

	// suppression ici → supprimée là-bas
	b.Delete(b.ByEmail("anne@x.org").ID)
	if len(b.Deleted) != 1 {
		t.Fatalf("pierre tombale absente : %v", b.Deleted)
	}
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil || res.RemoteDeleted != 1 || len(b.Deleted) != 0 {
		t.Fatalf("suppression : %v %v", res, err)
	}
	if got := strings.Join(be.Names(), ","); strings.Contains(got, "Anne") {
		t.Errorf("Anne encore sur le serveur : %s", got)
	}

	// suppression à distance → supprimée ici
	for _, o := range be.Objects() {
		if strings.Contains(o.Path, "Bob") {
			be.Remove(o.Path)
		}
	}
	if res, err = b.SyncCardDAV(ctx, hc, cfg); err != nil || res.Deleted != 1 || b.ByEmail("bob2@x.org") != nil {
		t.Fatalf("suppression distante : %v %v", res, err)
	}
}

func TestCardDAVBadCredentialsAndURL(t *testing.T) {
	url, _, hc := davtest.Start(t)
	cfg := SyncConfig{URL: url, User: "alice", Password: "faux"}
	if _, err := (&Book{}).SyncCardDAV(context.Background(), hc, cfg); err == nil {
		t.Error("mauvais mot de passe accepté")
	}
	for in, ok := range map[string]bool{"https://dav.example.org/x": true, "http://dav.example.org/x": false, "ftp://x/y": false, "https://u:p@x/y": false, "": false, "https://": false} {
		if err := ValidSyncURL(in, nil); (err == nil) != ok {
			t.Errorf("%q : %v", in, err)
		}
	}
	if err := ValidSyncURL("http://greenmail/x", func(h string) bool { return h == "greenmail" }); err != nil {
		t.Errorf("hôte local autorisé : %v", err)
	}
}
