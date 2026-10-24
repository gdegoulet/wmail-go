package totp

import (
	"strings"
	"testing"
	"time"
)

// Vecteurs de la RFC 6238 (SHA-1, secret « 12345678901234567890 », codes à 6 chiffres = 8 chiffres tronqués).
func TestRFC6238Vectors(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1111111111: "050471", 1234567890: "005924", 2000000000: "279037"} {
		got, err := Code(secret, ts/Period)
		if err != nil || got != want {
			t.Errorf("t=%d : %s (attendu %s) %v", ts, got, want, err)
		}
	}
}

func TestVerifyWindowAndReplay(t *testing.T) {
	secret := NewSecret()
	now := time.Unix(1_800_000_000, 0)
	cur := Step(now)
	code, _ := Code(secret, cur)
	st, ok := Verify(secret, code, now, 0)
	if !ok || st != cur {
		t.Fatalf("code courant refusé")
	}
	if _, ok := Verify(secret, code, now, st); ok {
		t.Error("un code ne doit servir qu'une fois")
	}
	prev, _ := Code(secret, cur-1)
	if _, ok := Verify(secret, prev, now, 0); !ok {
		t.Error("le pas précédent est toléré")
	}
	old, _ := Code(secret, cur-3)
	if _, ok := Verify(secret, old, now, 0); ok {
		t.Error("un code trop ancien est accepté")
	}
	if _, ok := Verify(secret, " "+code[:3]+" "+code[3:], now, 0); !ok {
		t.Error("les espaces dans la saisie sont ignorés")
	}
	for _, bad := range []string{"", "12345", "abcdef", "1234567"} {
		if _, ok := Verify(secret, bad, now, 0); ok {
			t.Errorf("%q accepté", bad)
		}
	}
	if _, ok := Verify("pas du base32 !", "123456", now, 0); ok {
		t.Error("secret invalide accepté")
	}
}

func TestBackupCodes(t *testing.T) {
	codes, hashes := NewBackupCodes()
	if len(codes) != BackupCount || len(hashes) != BackupCount || !strings.Contains(codes[0], "-") {
		t.Fatalf("codes : %v", codes)
	}
	left, ok := UseBackup(hashes, codes[3])
	if !ok || len(left) != BackupCount-1 {
		t.Fatal("code de secours refusé")
	}
	if _, ok := UseBackup(left, codes[3]); ok {
		t.Error("code de secours réutilisé")
	}
	if _, ok := UseBackup(left, "0000-0000-00"); ok {
		t.Error("faux code accepté")
	}
	if _, ok := UseBackup(left, strings.ReplaceAll(codes[0], "-", "")); !ok {
		t.Error("code sans tirets refusé")
	}
}

func TestURI(t *testing.T) {
	u := URI("wmail-go", "alice", "ABC")
	if !strings.HasPrefix(u, "otpauth://totp/wmail-go:alice?") || !strings.Contains(u, "secret=ABC") || !strings.Contains(u, "issuer=wmail-go") {
		t.Errorf("uri : %s", u)
	}
}
