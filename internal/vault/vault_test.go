package vault

import (
	"bytes"
	"errors"
	"testing"
)

var testParams = Params{Time: 1, MemoryKiB: 64, Threads: 1}

func newSvc(t *testing.T) *Service {
	t.Helper()
	s, err := NewService(testParams, 2)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const (
	id1 = "aaaa000000000000000000000000000000000000000000000000000000000001"
	id2 = "bbbb000000000000000000000000000000000000000000000000000000000002"
	pw  = "un mot de passe assez long"
)

func TestCreateOpenPassword(t *testing.T) {
	s := newSvc(t)
	plain := []byte(`{"hello":"monde"}`)
	data, dek, rec, err := s.Create(id1, pw, plain)
	if err != nil {
		t.Fatal(err)
	}
	if rec == "" {
		t.Fatal("clé de récupération vide")
	}
	dek2, got, err := s.OpenPassword(id1, data, pw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) || dek != dek2 {
		t.Fatal("contenu ou clé différents")
	}
	if bytes.Contains(data, []byte("monde")) {
		t.Fatal("le contenu apparaît en clair dans le fichier")
	}
	if _, _, err := s.OpenPassword(id1, data, pw+"x"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mauvais mot de passe : %v, attendu ErrInvalid", err)
	}
	// Le fichier est lié à son identifiant : déplacé chez un autre utilisateur, il ne s'ouvre pas.
	if _, _, err := s.OpenPassword(id2, data, pw); err == nil {
		t.Fatal("un fichier déplacé vers un autre identifiant ne doit pas s'ouvrir")
	}
}

func TestPasswordNormalizedNFC(t *testing.T) {
	s := newSvc(t)
	nfc, nfd := "café secret long", "café secret long"
	data, _, _, err := s.Create(id1, nfc, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.OpenPassword(id1, data, nfd); err != nil {
		t.Fatalf("NFC et NFD doivent donner la même clé : %v", err)
	}
}

func TestRecoveryKey(t *testing.T) {
	s := newSvc(t)
	data, dek, rec, _ := s.Create(id1, pw, []byte("{}"))
	got, _, err := s.OpenRecovery(id1, data, rec)
	if err != nil || got != dek {
		t.Fatalf("ouverture par clé de récupération : %v", err)
	}
	// Casse, espaces et tirets ignorés.
	loose := " " + string(bytes.ToLower([]byte(rec))) + " "
	if _, _, err := s.OpenRecovery(id1, data, loose); err != nil {
		t.Fatalf("saisie tolérante : %v", err)
	}
	// Une faute de frappe est détectée par la somme de contrôle.
	bad := []byte(rec)
	if bad[0] == 'A' {
		bad[0] = 'B'
	} else {
		bad[0] = 'A'
	}
	if _, _, err := s.OpenRecovery(id1, data, string(bad)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("clé altérée : %v", err)
	}
	if _, _, err := s.OpenRecovery(id1, data, "n'importe quoi"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("clé invalide : %v", err)
	}
}

func TestSetPasswordKeepsContentAndRecovery(t *testing.T) {
	s := newSvc(t)
	data, dek, rec, _ := s.Create(id1, pw, []byte(`{"a":1}`))
	before, _ := parse(data)
	out, err := s.SetPassword(id1, data, dek, "un tout nouveau mot de passe")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := parse(out)
	if !bytes.Equal(before.content, after.content) || before.nonce != after.nonce {
		t.Fatal("le changement de mot de passe ne doit pas rechiffrer le contenu")
	}
	if before.salt == after.salt {
		t.Fatal("le sel doit changer")
	}
	if _, _, err := s.OpenPassword(id1, out, pw); !errors.Is(err, ErrInvalid) {
		t.Fatal("l'ancien mot de passe doit être refusé")
	}
	if _, _, err := s.OpenPassword(id1, out, "un tout nouveau mot de passe"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.OpenRecovery(id1, out, rec); err != nil {
		t.Fatalf("la clé de récupération doit rester valable : %v", err)
	}
	var wrong DEK
	if _, err := s.SetPassword(id1, data, wrong, "x"); err == nil {
		t.Fatal("SetPassword avec une mauvaise DEK doit échouer")
	}
}

func TestNewRecoveryReplacesOldKey(t *testing.T) {
	s := newSvc(t)
	data, dek, old, _ := s.Create(id1, pw, []byte("{}"))
	out, fresh, err := s.NewRecovery(id1, data, dek)
	if err != nil {
		t.Fatal(err)
	}
	if fresh == old {
		t.Fatal("la nouvelle clé doit être différente")
	}
	if _, _, err := s.OpenRecovery(id1, out, old); !errors.Is(err, ErrInvalid) {
		t.Fatal("l'ancienne clé doit être refusée")
	}
	if _, _, err := s.OpenRecovery(id1, out, fresh); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.OpenPassword(id1, out, pw); err != nil {
		t.Fatal("le mot de passe doit rester valable")
	}
}

func TestReseal(t *testing.T) {
	s := newSvc(t)
	data, dek, _, _ := s.Create(id1, pw, []byte(`{"v":1}`))
	out, err := s.Reseal(id1, data, dek, []byte(`{"v":2}`))
	if err != nil {
		t.Fatal(err)
	}
	_, got, err := s.OpenPassword(id1, out, pw)
	if err != nil || string(got) != `{"v":2}` {
		t.Fatalf("contenu après Reseal : %q %v", got, err)
	}
}

func TestTamperDetected(t *testing.T) {
	s := newSvc(t)
	data, _, _, _ := s.Create(id1, pw, []byte(`{"v":1}`))

	flip := func(off int) []byte {
		b := bytes.Clone(data)
		b[off] ^= 0x01
		return b
	}
	if _, _, err := s.OpenPassword(id1, flip(len(data)-1), pw); !errors.Is(err, ErrFormat) {
		t.Fatalf("contenu altéré : %v, attendu ErrFormat", err)
	}
	if _, _, err := s.OpenPassword(id1, flip(30), pw); !errors.Is(err, ErrInvalid) {
		t.Fatalf("slot altéré : %v, attendu ErrInvalid", err)
	}
	if _, _, err := s.OpenPassword(id1, data[:100], pw); !errors.Is(err, ErrFormat) {
		t.Fatalf("fichier tronqué : %v, attendu ErrFormat", err)
	}
	if _, _, err := s.OpenPassword(id1, append(bytes.Clone(data), 0), pw); !errors.Is(err, ErrFormat) {
		t.Fatalf("octet en trop : %v, attendu ErrFormat", err)
	}
	bad := bytes.Clone(data)
	copy(bad, "XXXX")
	if _, _, err := s.OpenPassword(id1, bad, pw); !errors.Is(err, ErrFormat) {
		t.Fatalf("magic invalide : %v", err)
	}
	// Paramètres argon2id hors limites dans l'en-tête (mémoire énorme) : refusés avant tout calcul.
	huge := bytes.Clone(data)
	huge[9], huge[10], huge[11], huge[12] = 0xff, 0xff, 0xff, 0xff
	if _, _, err := s.OpenPassword(id1, huge, pw); !errors.Is(err, ErrFormat) {
		t.Fatalf("paramètres hors limites : %v, attendu ErrFormat", err)
	}
}

func TestParamsValidation(t *testing.T) {
	for _, p := range []Params{{0, 64, 1}, {1, 4, 1}, {1, 64, 0}, {1, 1 << 21, 1}, {21, 64, 1}} {
		if _, err := NewService(p, 1); err == nil {
			t.Errorf("paramètres %+v acceptés à tort", p)
		}
	}
}

func TestFormatParseRecoveryKey(t *testing.T) {
	var raw [32]byte
	for i := range raw {
		raw[i] = byte(i * 7)
	}
	s := FormatRecoveryKey(raw)
	got, err := ParseRecoveryKey(s)
	if err != nil || got != raw {
		t.Fatalf("aller-retour : %v", err)
	}
	if len(s) != 55+10 { // 55 caractères base32, 10 tirets
		t.Fatalf("longueur %d", len(s))
	}
}
