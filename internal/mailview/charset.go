package mailview

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/emersion/go-message"
	gocharset "github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/charmap"
)

// Les messages réels portent des noms de jeux de caractères fantaisistes (« asci », « us-asci », « utf-7 »…) ou
// inconnus : au lieu d'ignorer la partie (le texte disparaîtrait), on la lit avec un repli sûr.
func init() {
	message.CharsetReader = func(name string, input io.Reader) (io.Reader, error) {
		switch n := strings.ToLower(strings.TrimSpace(name)); n {
		case "asci", "us-asci", "ascii", "ansi", "unknown-8bit", "x-unknown", "default", "x-user-defined", "":
			return charmap.Windows1252.NewDecoder().Reader(input), nil
		case "utf-7", "utf7", "unicode-1-1-utf-7":
			b, err := io.ReadAll(io.LimitReader(input, 8<<20))
			if err != nil {
				return nil, err
			}
			return strings.NewReader(decodeUTF7(string(b))), nil
		}
		if r, err := gocharset.Reader(name, input); err == nil {
			return r, nil
		}
		// jeu inconnu : l'octet est lu comme Windows-1252 plutôt que de perdre le texte
		return charmap.Windows1252.NewDecoder().Reader(input), nil
	}
}

// decodeUTF7 décode l'UTF-7 de RFC 2152 (+base64- pour les caractères hors ASCII).
func decodeUTF7(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '+' {
			out.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] >= 'A' && s[j] <= 'Z' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= '0' && s[j] <= '9' || s[j] == '+' || s[j] == '/') {
			j++
		}
		if j == i+1 { // « +- » = un plus littéral
			out.WriteByte('+')
			if j < len(s) && s[j] == '-' {
				j++
			}
			i = j - 1
			continue
		}
		b64 := s[i+1 : j]
		b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(b64, "="))
		if err != nil && len(b) == 0 {
			out.WriteString(s[i:j])
		} else {
			for k := 0; k+1 < len(b); k += 2 {
				out.WriteRune(rune(uint16(b[k])<<8 | uint16(b[k+1])))
			}
		}
		if j < len(s) && s[j] == '-' {
			j++
		}
		i = j - 1
	}
	return out.String()
}
