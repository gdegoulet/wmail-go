package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gdegoulet/wmail-go/internal/vault"
)

const goodPW = "un mot de passe assez long"

func newStore(t *testing.T, dir string, opts Options) *Store {
	t.Helper()
	svc, err := vault.NewService(vault.Params{Time: 1, MemoryKiB: 64, Threads: 1}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if opts.PasswordMinLength == 0 {
		opts.PasswordMinLength = 12
	}
	st, err := Open(dir, svc, opts)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestNormalizeLogin(t *testing.T) {
	ok := map[string]string{
		"  Alice ":        "alice",
		"BOB":             "bob",
		"café":           "café", // NFD → NFC
		"user@exemple.fr": "user@exemple.fr",
	}
	for in, want := range ok {
		got, err := NormalizeLogin(in)
		if err != nil || got != want {
			t.Errorf("NormalizeLogin(%q) = %q, %v ; attendu %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "ab", "a b c", "tab\there", "ctrl\x01char", strings.Repeat("x", 65)} {
		if _, err := NormalizeLogin(in); !errors.Is(err, ErrInvalidLogin) {
			t.Errorf("NormalizeLogin(%q) accepté à tort : %v", in, err)
		}
	}
}

func TestReduceLogin(t *testing.T) {
	cases := map[string]string{"gdegoulet": "gd***", "ab": "a***", "a": "a***", "": "***", "éric": "ér***"}
	for in, want := range cases {
		if got := ReduceLogin(in); got != want {
			t.Errorf("ReduceLogin(%q) = %q, attendu %q", in, got, want)
		}
	}
}

func TestCreateLoginList(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{})
	acc, key, err := st.Create("Alice", goodPW, false)
	if err != nil || key == "" {
		t.Fatalf("Create : %v", err)
	}
	if acc.Login != "alice" {
		t.Fatalf("login normalisé %q", acc.Login)
	}
	got, err := st.Login("ALICE", goodPW)
	if err != nil || got.Login != "alice" {
		t.Fatalf("Login : %v", err)
	}
	if _, err := st.Login("alice", goodPW+"x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("mauvais mot de passe : %v", err)
	}
	if _, err := st.Login("inconnu", goodPW); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login inconnu : %v", err)
	}
	if _, err := st.Login("x", goodPW); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login invalide : %v", err)
	}
	if _, _, err := st.Create("alice", goodPW, false); !errors.Is(err, ErrExists) {
		t.Fatalf("doublon (casse différente) : %v", err)
	}
	if _, _, err := st.Create("bob", "court", false); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("mot de passe court : %v", err)
	}
	if _, _, err := st.Create("bob", strings.Repeat("x", 2000), false); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("mot de passe trop long : %v", err)
	}
	if _, _, err := st.Create("carol", goodPW, false); err != nil {
		t.Fatal(err)
	}
	list, err := st.List()
	if err != nil || strings.Join(list, ",") != "alice,carol" {
		t.Fatalf("List = %v, %v", list, err)
	}
	if n, _ := st.Count(); n != 2 {
		t.Fatalf("Count = %d", n)
	}
}

func TestMaxUsers(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{MaxUsers: 1})
	if _, _, err := st.Create("alice", goodPW, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.Create("bob", goodPW, false); !errors.Is(err, ErrMaxUsers) {
		t.Fatalf("max_users : %v", err)
	}
	if _, _, err := st.Create("bob", goodPW, true); err != nil {
		t.Fatalf("le CLI doit contourner max_users : %v", err)
	}
}

func TestDiskLayoutHidesLogins(t *testing.T) {
	dir := t.TempDir()
	st := newStore(t, dir, Options{})
	if _, _, err := st.Create("alice-secrete", goodPW, false); err != nil {
		t.Fatal(err)
	}
	id := st.ID("alice-secrete")
	want := filepath.Join(dir, "users", id[0:2], id[2:4], id+".cfg")
	fi, err := os.Stat(want)
	if err != nil {
		t.Fatalf("fichier attendu absent : %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("droits du fichier %v, attendu 0600", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(want)); di.Mode().Perm() != 0o700 {
		t.Errorf("droits du dossier %v, attendu 0700", di.Mode().Perm())
	}
	if ki, _ := os.Stat(filepath.Join(dir, "server.key")); ki.Mode().Perm() != 0o600 {
		t.Errorf("droits de server.key %v, attendu 0600", ki.Mode().Perm())
	}
	// Aucun nom ni contenu de fichier ne contient le login en clair.
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(p, "alice") {
			t.Errorf("le chemin %s contient le login", p)
		}
		if !d.IsDir() {
			b, _ := os.ReadFile(p)
			if bytes.Contains(b, []byte("alice-secrete")) {
				t.Errorf("le fichier %s contient le login en clair", p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServerKeyPersistsAndDiffers(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	a := newStore(t, d1, Options{})
	b := newStore(t, d1, Options{})
	c := newStore(t, d2, Options{})
	if a.ID("alice") != b.ID("alice") {
		t.Fatal("même data_dir : l'identifiant doit être identique")
	}
	if a.ID("alice") == c.ID("alice") {
		t.Fatal("data_dir différent : l'identifiant doit différer (clé serveur distincte)")
	}
}

func TestChangePassword(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{})
	st.Create("alice", goodPW, false)
	if err := st.ChangePassword("alice", "mauvais mot de passe", "nouveau mot de passe long"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ancien mot de passe faux : %v", err)
	}
	if err := st.ChangePassword("alice", goodPW, "court"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("nouveau mot de passe court : %v", err)
	}
	if err := st.ChangePassword("alice", goodPW, "nouveau mot de passe long"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Login("alice", goodPW); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("l'ancien mot de passe doit être refusé")
	}
	if _, err := st.Login("alice", "nouveau mot de passe long"); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverAndRegenerate(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{})
	_, key1, _ := st.Create("alice", goodPW, false)

	if _, _, err := st.Recover("alice", "AAAAA-AAAAA", "nouveau mot de passe long"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("clé fausse : %v", err)
	}
	if _, _, err := st.Recover("inconnu", key1, "nouveau mot de passe long"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login inconnu : %v", err)
	}
	acc, key2, err := st.Recover("alice", key1, "nouveau mot de passe long")
	if err != nil {
		t.Fatal(err)
	}
	if key2 == key1 {
		t.Fatal("une nouvelle clé de récupération doit être émise")
	}
	if acc.Login != "alice" {
		t.Fatal(acc.Login)
	}
	if _, err := st.Login("alice", "nouveau mot de passe long"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.Recover("alice", key1, "autre mot de passe long"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("l'ancienne clé doit être refusée")
	}
	key3, err := st.RegenerateRecovery("alice", "nouveau mot de passe long")
	if err != nil || key3 == key2 {
		t.Fatalf("RegenerateRecovery : %v", err)
	}
	if _, err := st.RegenerateRecovery("alice", "faux mot de passe long"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("mot de passe faux")
	}
	if _, _, err := st.Recover("alice", key3, "encore un autre mot de passe"); err != nil {
		t.Fatalf("nouvelle clé : %v", err)
	}
}

func TestSaveDataPersists(t *testing.T) {
	dir := t.TempDir()
	st := newStore(t, dir, Options{})
	acc, _, _ := st.Create("alice", goodPW, false)
	acc.Data.Settings.Language = "en"
	acc.Data.Settings.PageSize = 80
	if err := st.SaveData(acc); err != nil {
		t.Fatal(err)
	}
	st2 := newStore(t, dir, Options{}) // nouveau processus simulé
	got, err := st2.Login("alice", goodPW)
	if err != nil {
		t.Fatal(err)
	}
	if got.Data.Settings.Language != "en" || got.Data.Settings.PageSize != 80 {
		t.Fatalf("préférences perdues : %+v", got.Data.Settings)
	}
	// Le mot de passe fonctionne toujours et la clé de récupération aussi (slots intacts).
	if err := st.ChangePassword("alice", goodPW, "nouveau mot de passe long"); err != nil {
		t.Fatal(err)
	}
	got, err = st2.Login("alice", "nouveau mot de passe long")
	if err != nil || got.Data.Settings.PageSize != 80 {
		t.Fatalf("contenu après changement de mot de passe : %v", err)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	st := newStore(t, dir, Options{})
	st.Create("alice", goodPW, false)
	st.Create("bob", goodPW, false)
	if err := st.Delete("alice"); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete : %v", err)
	}
	if _, err := st.Login("alice", goodPW); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("compte supprimé encore accessible")
	}
	list, _ := st.List()
	if strings.Join(list, ",") != "bob" {
		t.Fatalf("List = %v", list)
	}
	// Un login supprimé peut être recréé.
	if _, _, err := st.Create("alice", goodPW, false); err != nil {
		t.Fatal(err)
	}
}

func TestIndexIsEncrypted(t *testing.T) {
	dir := t.TempDir()
	st := newStore(t, dir, Options{})
	st.Create("utilisateur-visible", goodPW, false)
	b, err := os.ReadFile(filepath.Join(dir, "users.idx"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("utilisateur-visible")) || !bytes.HasPrefix(b, []byte("WMIX")) {
		t.Fatal("l'index doit être chiffré")
	}
	// Une autre clé serveur ne peut pas lire l'index.
	other := newStore(t, t.TempDir(), Options{})
	other.dir = dir
	if _, err := other.List(); !errors.Is(err, ErrIndexCorrupt) {
		t.Fatalf("index lu avec une mauvaise clé : %v", err)
	}
}

func TestConcurrentCreate(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{})
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := st.Create(fmt.Sprintf("user%02d", i), goodPW, false); err != nil {
				t.Errorf("Create user%02d : %v", i, err)
			}
		}()
	}
	wg.Wait()
	if n, err := st.Count(); err != nil || n != 10 {
		t.Fatalf("Count = %d, %v", n, err)
	}
}

func TestTwoProcessesShareIndex(t *testing.T) {
	// Deux Store sur le même répertoire (serveur + CLI) : l'index reste cohérent.
	dir := t.TempDir()
	a := newStore(t, dir, Options{})
	b := newStore(t, dir, Options{})
	a.Create("alice", goodPW, false)
	b.Create("bob", goodPW, false)
	list, _ := a.List()
	if strings.Join(list, ",") != "alice,bob" {
		t.Fatalf("List = %v", list)
	}
}

func TestContactsBlobLifecycle(t *testing.T) {
	st := newStore(t, t.TempDir(), Options{})
	acc, _, err := st.Create("alice", goodPW, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := st.LoadContacts(acc); err != nil || got != nil {
		t.Fatalf("carnet absent : %v %v", got, err)
	}
	plain := []byte(`{"contacts":[{"display":"Zoë"}]}`)
	if err := st.SaveContacts(acc, plain); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(st.blobPath(acc.ID, "contacts"))
	if bytes.Contains(raw, []byte("Zoë")) || !bytes.HasPrefix(raw, []byte("WMBL")) {
		t.Error("le carnet n'est pas chiffré")
	}
	got, err := st.LoadContacts(acc)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("relecture : %q %v", got, err)
	}
	// après changement de mot de passe et récupération, la même clé de données ouvre toujours le carnet
	if err := st.ChangePassword("alice", goodPW, "un autre mot de passe long"); err != nil {
		t.Fatal(err)
	}
	acc2, err := st.Login("alice", "un autre mot de passe long")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := st.LoadContacts(acc2); err != nil || !bytes.Equal(got, plain) {
		t.Errorf("après changement de mot de passe : %q %v", got, err)
	}
	// altération détectée ; fichier d'un autre compte refusé
	raw[len(raw)-1] ^= 1
	_ = os.WriteFile(st.blobPath(acc.ID, "contacts"), raw, 0o600)
	if _, err := st.LoadContacts(acc2); err == nil {
		t.Error("fichier altéré accepté")
	}
	other, _, _ := st.Create("bob", goodPW, false)
	good, _ := vault.SealBlob(acc.DEK, "contacts", acc.ID, plain)
	_ = os.WriteFile(st.blobPath(other.ID, "contacts"), good, 0o600)
	if _, err := st.LoadContacts(other); err == nil {
		t.Error("le carnet d'un compte s'ouvre sous un autre identifiant")
	}
	// suppression du compte : le carnet disparaît aussi
	if err := st.Delete("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(st.blobPath(acc.ID, "contacts")); !os.IsNotExist(err) {
		t.Error("carnet resté sur le disque après suppression du compte")
	}
}
