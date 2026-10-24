// Package totp implémente les mots de passe à usage unique basés sur le temps (RFC 6238, HMAC-SHA1, 6 chiffres, 30 s),
// compatibles avec Google Authenticator, FreeOTP, Aegis, etc., et les codes de secours.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	Period = 30
	Digits = 6
	// Window : nombre de pas acceptés avant et après l'instant courant (horloges légèrement décalées).
	Window = 1
	// BackupCount : nombre de codes de secours créés.
	BackupCount = 9
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret crée un secret de 160 bits encodé en base32.
func NewSecret() string {
	var b [20]byte
	_, _ = rand.Read(b[:])
	return b32.EncodeToString(b[:])
}

func hotp(key []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, code%1000000)
}

// Step renvoie le numéro du pas de temps de t.
func Step(t time.Time) int64 { return t.Unix() / Period }

// Code calcule le code du pas donné.
func Code(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil || len(key) < 10 {
		return "", fmt.Errorf("totp : secret invalide")
	}
	return hotp(key, uint64(step)), nil
}

// Verify vérifie un code saisi. Il renvoie le pas accepté ; l'appelant doit refuser tout pas inférieur ou égal au
// dernier pas accepté (un code ne sert qu'une fois).
func Verify(secret, input string, now time.Time, lastStep int64) (step int64, ok bool) {
	input = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, input)
	if len(input) != Digits {
		return 0, false
	}
	cur := Step(now)
	for d := int64(-Window); d <= Window; d++ {
		st := cur + d
		want, err := Code(secret, st)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(input)) == 1 && st > lastStep {
			return st, true
		}
	}
	return 0, false
}

// URI construit l'adresse otpauth:// lue par les applications (par le code QR).
func URI(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {fmt.Sprint(Digits)}, "period": {fmt.Sprint(Period)}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// NewBackupCodes crée des codes de secours à 10 chiffres (« 1234-5678-90 ») et leurs empreintes à enregistrer.
func NewBackupCodes() (codes, hashes []string) {
	for i := 0; i < BackupCount; i++ {
		var b [8]byte
		_, _ = rand.Read(b[:])
		n := binary.BigEndian.Uint64(b[:]) % 10_000_000_000
		d := fmt.Sprintf("%010d", n)
		codes = append(codes, d[:4]+"-"+d[4:8]+"-"+d[8:])
		hashes = append(hashes, HashBackup(d))
	}
	return codes, hashes
}

// HashBackup donne l'empreinte d'un code de secours (chiffres seuls).
func HashBackup(digits string) string {
	h := sha256.Sum256([]byte("wmail-go backup/v1:" + digits))
	return hex.EncodeToString(h[:])
}

// UseBackup cherche le code de secours parmi les empreintes ; renvoie les empreintes restantes et vrai si trouvé.
func UseBackup(hashes []string, input string) ([]string, bool) {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, input)
	if len(d) != 10 {
		return hashes, false
	}
	want := HashBackup(d)
	found := -1
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			found = i
		}
	}
	if found < 0 {
		return hashes, false
	}
	out := append([]string(nil), hashes[:found]...)
	return append(out, hashes[found+1:]...), true
}
