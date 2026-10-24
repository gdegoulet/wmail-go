// Package compose prépare et construit les messages sortants : adresses, objet, destinataires d'une réponse,
// citation et assemblage MIME. Il ne fait aucun accès réseau.
package compose

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

// ErrInvalidAddress signale une adresse inutilisable (syntaxe, caractère de contrôle).
var ErrInvalidAddress = errors.New("adresse invalide")

// hasControl indique la présence d'un caractère de contrôle (CR, LF, NUL…) : refusé partout dans les en-têtes,
// pour interdire l'injection d'en-têtes.
func hasControl(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < 0x20 && r != '\t' || r == 0x7f }) >= 0
}

// ParseList lit une liste d'adresses saisie par l'utilisateur. Les séparateurs sont la virgule, le point-virgule et
// le retour à la ligne (hors guillemets). Une liste vide est valide.
func ParseList(s string) ([]*mail.Address, error) {
	if hasControl(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")) {
		return nil, fmt.Errorf("%w : caractère de contrôle", ErrInvalidAddress)
	}
	var parts []string
	var cur strings.Builder
	inQuote, inAngle := false, false
	esc := false
	for _, r := range s {
		switch {
		case esc:
			esc = false
			cur.WriteRune(r)
			continue
		case r == '\\' && inQuote:
			esc = true
		case r == '"' && !inAngle:
			inQuote = !inQuote
		case r == '<' && !inQuote:
			inAngle = true
		case r == '>' && !inQuote:
			inAngle = false
		case (r == ',' || r == ';' || r == '\n' || r == '\r') && !inQuote && !inAngle:
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	parts = append(parts, cur.String())

	var out []*mail.Address
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		a, err := mail.ParseAddress(p)
		if err != nil {
			return nil, fmt.Errorf("%w : %q", ErrInvalidAddress, truncate(p, 60))
		}
		if hasControl(a.Name) || hasControl(a.Address) || !strings.Contains(a.Address, "@") {
			return nil, fmt.Errorf("%w : %q", ErrInvalidAddress, truncate(p, 60))
		}
		out = append(out, a)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// FormatList met une liste d'adresses en une ligne « Nom <a@b>, c@d ».
func FormatList(list []*mail.Address) string {
	parts := make([]string, 0, len(list))
	for _, a := range list {
		if a != nil {
			parts = append(parts, a.String())
		}
	}
	return strings.Join(parts, ", ")
}

// Dedup retire les doublons (adresse comparée sans casse) et les adresses de exclude (en minuscules).
func Dedup(list []*mail.Address, exclude map[string]bool) []*mail.Address {
	seen := map[string]bool{}
	var out []*mail.Address
	for _, a := range list {
		if a == nil {
			continue
		}
		k := strings.ToLower(a.Address)
		if k == "" || seen[k] || exclude[k] {
			continue
		}
		seen[k] = true
		out = append(out, a)
	}
	return out
}

// Domain renvoie la partie après « @ » d'une adresse (vide si absente).
func Domain(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return strings.TrimFunc(addr[i+1:], func(r rune) bool { return unicode.IsSpace(r) || r == '>' })
	}
	return ""
}
