package sieve

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
)

// Les noms de dossiers que RainLoop enregistre dans ses filtres sont en UTF-7 modifié (RFC 3501, 5.1.3) : le
// préfixe est « & » et non « + », et l'alphabet base64 se termine par « +, » au lieu de « +/ ».
//
// À ne pas confondre avec mailview.decodeUTF7, qui décode l'UTF-7 ordinaire (RFC 2152) des corps de messages :
// les deux encodages se ressemblent mais ne sont pas interchangeables.

var utf7b64 = base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+,").WithPadding(base64.NoPadding)

// ToUTF7 convertit un nom UTF-8 en UTF-7 modifié.
func ToUTF7(s string) string {
	var b strings.Builder
	var pending []uint16
	flush := func() {
		if len(pending) == 0 {
			return
		}
		raw := make([]byte, 0, len(pending)*2)
		for _, u := range pending {
			raw = append(raw, byte(u>>8), byte(u))
		}
		b.WriteByte('&')
		b.WriteString(utf7b64.EncodeToString(raw))
		b.WriteByte('-')
		pending = pending[:0]
	}
	for _, r := range s {
		if r >= 0x20 && r <= 0x7e {
			flush()
			if r == '&' {
				b.WriteString("&-")
			} else {
				b.WriteRune(r)
			}
			continue
		}
		pending = append(pending, utf16.Encode([]rune{r})...)
	}
	flush()
	return b.String()
}

// FromUTF7 convertit un nom en UTF-7 modifié vers UTF-8. Une entrée invalide est renvoyée telle quelle.
func FromUTF7(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			b.WriteByte(s[i])
			continue
		}
		j := strings.IndexByte(s[i:], '-')
		if j < 0 {
			return s
		}
		chunk := s[i+1 : i+j]
		i += j
		if chunk == "" {
			b.WriteByte('&')
			continue
		}
		raw, err := utf7b64.DecodeString(chunk)
		if err != nil || len(raw)%2 != 0 {
			return s
		}
		u := make([]uint16, len(raw)/2)
		for k := range u {
			u[k] = uint16(raw[2*k])<<8 | uint16(raw[2*k+1])
		}
		b.WriteString(string(utf16.Decode(u)))
	}
	return b.String()
}
