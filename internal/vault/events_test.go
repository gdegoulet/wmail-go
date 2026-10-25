package vault

import (
	"bytes"
	"testing"
)

func TestSealOpenEvent(t *testing.T) {
	priv, pub, err := NewEventKeys()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte(`{"k":"login_fail","ip":"192.0.2.1"}`)
	data, err := SealEvent(pub, "abcdef", msg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenEvent(priv, "abcdef", data)
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("contenu %q, attendu %q", got, msg)
	}
	if bytes.Contains(data, msg) {
		t.Error("le texte clair apparaît dans le scellé")
	}
}

func TestEventBindings(t *testing.T) {
	priv, pub, _ := NewEventKeys()
	autre, _, _ := NewEventKeys()
	data, err := SealEvent(pub, "compte-a", []byte("coucou"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenEvent(priv, "compte-b", data); err == nil {
		t.Error("une entrée s'ouvre sous un autre identifiant de compte")
	}
	if _, err := OpenEvent(autre, "compte-a", data); err == nil {
		t.Error("une entrée s'ouvre avec une autre clé privée")
	}
	for i := range data {
		altere := bytes.Clone(data)
		altere[i] ^= 0x01
		if _, err := OpenEvent(priv, "compte-a", altere); err == nil {
			t.Fatalf("octet %d modifié : entrée acceptée", i)
		}
	}
	if _, err := OpenEvent(priv, "compte-a", data[:len(data)-1]); err == nil {
		t.Error("entrée tronquée acceptée")
	}
}

func TestEventNonDeterministe(t *testing.T) {
	_, pub, _ := NewEventKeys()
	a, _ := SealEvent(pub, "x", []byte("même contenu"))
	b, _ := SealEvent(pub, "x", []byte("même contenu"))
	if bytes.Equal(a, b) {
		t.Error("deux scellés identiques pour le même contenu")
	}
}

func TestEventPublicStable(t *testing.T) {
	priv, pub, _ := NewEventKeys()
	again, err := EventPublic(priv)
	if err != nil {
		t.Fatal(err)
	}
	if again != pub {
		t.Error("la clé publique recalculée diffère")
	}
}

func TestEventTropGrand(t *testing.T) {
	_, pub, _ := NewEventKeys()
	if _, err := SealEvent(pub, "x", make([]byte, maxEvent+1)); err == nil {
		t.Error("entrée hors limite acceptée")
	}
}
