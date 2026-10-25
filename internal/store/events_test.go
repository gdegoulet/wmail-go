package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// compteTest ouvre un dépôt jetable et crée un compte doté de ses clés d'événements.
func compteTest(t *testing.T) (*Store, *Account) {
	t.Helper()
	st := newStore(t, t.TempDir(), Options{})
	acc, _, err := st.Create("alice", goodPW, false)
	if err != nil {
		t.Fatal(err)
	}
	modifie, err := st.EnsureEventKeys(acc)
	if err != nil {
		t.Fatal(err)
	}
	if !modifie {
		t.Fatal("le compte neuf devrait recevoir une clé d'événements")
	}
	if err := st.SaveData(acc); err != nil {
		t.Fatal(err)
	}
	return st, acc
}

func TestEventsAllerRetour(t *testing.T) {
	st, acc := compteTest(t)
	for i := range 3 {
		if err := st.AppendEvent("ALICE", fmt.Appendf(nil, `{"k":"login_fail","n":%d}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	entries, dropped, err := st.DrainEvents(acc)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 0 || len(entries) != 3 {
		t.Fatalf("%d entrées, %d perdues ; attendu 3 et 0", len(entries), dropped)
	}
	for i, e := range entries {
		if want := fmt.Sprintf(`{"k":"login_fail","n":%d}`, i); string(e) != want {
			t.Errorf("entrée %d = %q, attendu %q", i, e, want)
		}
	}
	// La file est vidée : un second passage ne rend rien.
	entries, _, err = st.DrainEvents(acc)
	if err != nil || len(entries) != 0 {
		t.Fatalf("file non vidée : %d entrées, %v", len(entries), err)
	}
}

func TestEventsCompteInconnu(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{})
	// Ni erreur ni fichier : un login inconnu ne doit pas se distinguer d'un login connu.
	if err := st.AppendEvent("personne", []byte(`{"k":"login_fail"}`)); err != nil {
		t.Fatalf("login inconnu : %v", err)
	}
	if err := st.AppendEvent("", []byte(`{"k":"login_fail"}`)); err != nil {
		t.Fatalf("login vide : %v", err)
	}
}

func TestEventsEnClairAbsent(t *testing.T) {
	st, acc := compteTest(t)
	secret := []byte(`{"k":"login_fail","ip":"192.0.2.77"}`)
	if err := st.AppendEvent("alice", secret); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(st.evtPath(acc.ID))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("192.0.2.77")) {
		t.Error("l'adresse IP apparaît en clair dans la file d'attente")
	}
}

func TestEventsPlafond(t *testing.T) {
	st, acc := compteTest(t)
	const n = maxEvtEntries + 50
	for i := range n {
		if err := st.AppendEvent("alice", fmt.Appendf(nil, `{"n":%d}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	entries, dropped, err := st.DrainEvents(acc)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > maxEvtEntries {
		t.Errorf("%d entrées gardées, plafond %d", len(entries), maxEvtEntries)
	}
	if dropped == 0 {
		t.Error("aucune perte comptée malgré le débordement")
	}
	if len(entries)+dropped != n {
		t.Errorf("%d gardées + %d perdues = %d, attendu %d", len(entries), dropped, len(entries)+dropped, n)
	}
	// La première et la dernière survivent : début et fin de rafale.
	if string(entries[0]) != `{"n":0}` {
		t.Errorf("première entrée %q, attendu {\"n\":0}", entries[0])
	}
	if want := fmt.Sprintf(`{"n":%d}`, n-1); string(entries[len(entries)-1]) != want {
		t.Errorf("dernière entrée %q, attendu %q", entries[len(entries)-1], want)
	}
}

func TestEventsFileCorrompue(t *testing.T) {
	st, acc := compteTest(t)
	if err := st.AppendEvent("alice", []byte(`{"n":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(st.evtPath(acc.ID), []byte("n'importe quoi"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, dropped, err := st.DrainEvents(acc)
	if err != nil {
		t.Fatalf("une file corrompue ne doit pas faire échouer la lecture : %v", err)
	}
	if dropped == 0 {
		t.Error("la perte n'est pas signalée")
	}
	// Et la file repart : l'écriture suivante fonctionne.
	if err := st.AppendEvent("alice", []byte(`{"n":2}`)); err != nil {
		t.Fatal(err)
	}
	entries, _, err := st.DrainEvents(acc)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%d entrées après reprise, %v", len(entries), err)
	}
}

func TestEventsCleAbsenteRepubliee(t *testing.T) {
	st, acc := compteTest(t)
	if err := os.Remove(st.pubPath(acc.ID)); err != nil {
		t.Fatal(err)
	}
	modifie, err := st.EnsureEventKeys(acc)
	if err != nil {
		t.Fatal(err)
	}
	if modifie {
		t.Error("la clé privée existe déjà : les données ne devraient pas changer")
	}
	if _, err := os.Stat(st.pubPath(acc.ID)); err != nil {
		t.Fatalf("clé publique non republiée : %v", err)
	}
	if err := st.AppendEvent("alice", []byte(`{"n":1}`)); err != nil {
		t.Fatal(err)
	}
	entries, _, err := st.DrainEvents(acc)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%d entrées, %v", len(entries), err)
	}
}

func TestEventsSupprimesAvecLeCompte(t *testing.T) {
	st, acc := compteTest(t)
	if err := st.AppendEvent("alice", []byte(`{"n":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveActivity(acc, []byte("historique")); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("alice"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{st.pubPath(acc.ID), st.evtPath(acc.ID), st.blobPath(acc.ID, "activity")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s subsiste après la suppression du compte", filepath.Base(p))
		}
	}
}

func TestActivityBlob(t *testing.T) {
	st, acc := compteTest(t)
	if b, err := st.LoadActivity(acc); err != nil || b != nil {
		t.Fatalf("historique initial %q, %v", b, err)
	}
	if err := st.SaveActivity(acc, []byte(`[{"k":"login_ok"}]`)); err != nil {
		t.Fatal(err)
	}
	b, err := st.LoadActivity(acc)
	if err != nil || string(b) != `[{"k":"login_ok"}]` {
		t.Fatalf("historique %q, %v", b, err)
	}
	raw, err := os.ReadFile(st.blobPath(acc.ID, "activity"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("login_ok")) {
		t.Error("l'historique n'est pas chiffré sur le disque")
	}
}
