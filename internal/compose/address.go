// Package compose prépare et construit les messages sortants : adresses, objet, destinataires d'une réponse,
// citation et assemblage MIME. Il ne fait aucun accès réseau.
package compose

import (
	"errors"
	"fmt"
	"golang.org/x/net/idna"

	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"wmail-go/internal/mailview"
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
			if a, err = parseLoose(p); err != nil {
				return nil, fmt.Errorf("%w : %q", ErrInvalidAddress, truncate(p, 60))
			}
		}
		// « 'Nom' <a@b> » : les apostrophes entourant le nom sont décoratives (RainLoop les retire aussi).
		if n := a.Name; len(n) > 1 && strings.HasPrefix(n, "'") && strings.HasSuffix(n, "'") {
			a.Name = strings.TrimSpace(n[1 : len(n)-1])
		}
		if hasControl(a.Name) || hasControl(a.Address) || !strings.Contains(a.Address, "@") {
			return nil, fmt.Errorf("%w : %q", ErrInvalidAddress, truncate(p, 60))
		}
		out = append(out, a)
	}
	return out, nil
}

var (
	looseMailRe    = regexp.MustCompile(`[^\s<>,;"'()]+@[^\s<>,;"'()]+\.[^\s<>,;"'()]+`)
	looseCommentRe = regexp.MustCompile(`\(([^()]*)\)`)
	headerLikeRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z-]{1,30}:`)
)

// parseLoose lit les formes qu'un analyseur strict refuse mais qu'un utilisateur écrit couramment :
// « Nom a@b.fr », « a@b.fr (Nom) », « 'Nom' <a@b.fr> », une adresse suivie d'un point final.
func parseLoose(p string) (*mail.Address, error) {
	name := ""
	rest := looseCommentRe.ReplaceAllStringFunc(p, func(m string) string {
		if name == "" {
			name = strings.TrimSpace(m[1 : len(m)-1])
		}
		return " "
	})
	m := looseMailRe.FindStringIndex(rest)
	if m == nil {
		return nil, ErrInvalidAddress
	}
	addr := strings.Trim(rest[m[0]:m[1]], ".,;")
	if name == "" {
		name = strings.TrimSpace(rest[:m[0]] + " " + rest[m[1]:])
		name = strings.Trim(name, " \t<>,;\"'")
	}
	// « Bcc: pirate@x » et les autres formes qui ressemblent à un en-tête sont refusées (tentative d'injection).
	if headerLikeRe.MatchString(name) || hasControl(name) || hasControl(addr) || !strings.Contains(addr, "@") {
		return nil, ErrInvalidAddress
	}
	return &mail.Address{Name: name, Address: addr}, nil
}

// ToASCIIDomain convertit en punycode le domaine d'une adresse (RFC 5890) : les serveurs SMTP n'acceptent pas un domaine
// en Unicode sans l'extension SMTPUTF8. La partie locale est laissée telle quelle.
func ToASCIIDomain(addr string) string {
	at := strings.LastIndexByte(addr, '@')
	if at < 0 {
		return addr
	}
	domain := addr[at+1:]
	if isASCII(domain) {
		return addr
	}
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil || ascii == "" {
		return addr
	}
	return addr[:at+1] + ascii
}

// ToUnicodeDomain rend lisible un domaine en punycode (affichage seulement) ; voir mailview.UnicodeDomain.
func ToUnicodeDomain(addr string) string { return mailview.UnicodeDomain(addr) }

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
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
