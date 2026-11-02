package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Couleur est une couleur opaque en composantes 0-255. Le générateur ne manipule que de l'opaque : les
// quelques valeurs translucides des thèmes (voiles, ombres) sont composées à la fin, à partir d'une couleur
// opaque et d'un alpha choisi, ce qui évite d'avoir à raisonner sur un fond inconnu.
type Couleur struct{ R, G, B uint8 }

func (c Couleur) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// RGBA rend la couleur avec un alpha, pour les voiles et les ombres.
func (c Couleur) RGBA(alpha float64) string {
	a := strings.TrimPrefix(strconv.FormatFloat(alpha, 'f', -1, 64), "0")
	if a == "" || a == "." {
		a = "0"
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, a)
}

func parseHex(s string) (Couleur, bool) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	switch len(s) {
	case 3: // forme courte : #abc vaut #aabbcc
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	case 8: // #rrggbbaa : l'alpha est ignoré, le générateur ne garde que la teinte
		s = s[:6]
	default:
		return Couleur{}, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Couleur{}, false
	}
	return Couleur{uint8(n >> 16), uint8(n >> 8), uint8(n)}, true
}

// --- luminance et contraste (WCAG 2.1) ---

// canalLineaire défait la correction gamma du sRGB. Sans elle, un contraste calculé sur les composantes
// brutes se trompe lourdement dans les tons moyens.
func canalLineaire(v uint8) float64 {
	f := float64(v) / 255
	if f <= 0.04045 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

// Luminance est la luminance relative, de 0 (noir) à 1 (blanc).
func (c Couleur) Luminance() float64 {
	return 0.2126*canalLineaire(c.R) + 0.7152*canalLineaire(c.G) + 0.0722*canalLineaire(c.B)
}

// Contraste est le rapport de contraste entre deux couleurs, de 1 (identiques) à 21 (noir sur blanc).
// Les seuils de la WCAG : 4,5 pour du texte courant, 3 pour du gros texte et les contours d'éléments.
func Contraste(a, b Couleur) float64 {
	la, lb := a.Luminance(), b.Luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

const (
	ContrasteTexte   = 4.5 // texte courant sur son fond
	ContrasteElement = 3.0 // gros texte, contours, barres
)

// --- teinte, saturation, luminosité ---

type TSL struct{ T, S, L float64 } // teinte 0-360, saturation et luminosité 0-1

func (c Couleur) TSL() TSL {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	max, min := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (max + min) / 2
	if max == min {
		return TSL{0, 0, l} // gris : la teinte n'a pas de sens
	}
	d := max - min
	s := d / (2 - max - min)
	if l < 0.5 {
		s = d / (max + min)
	}
	var t float64
	switch max {
	case r:
		t = (g - b) / d
		if g < b {
			t += 6
		}
	case g:
		t = (b-r)/d + 2
	default:
		t = (r-g)/d + 4
	}
	return TSL{t * 60, s, l}
}

func (h TSL) Couleur() Couleur {
	t := math.Mod(math.Mod(h.T, 360)+360, 360) / 360
	s, l := clamp01(h.S), clamp01(h.L)
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return Couleur{v, v, v}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	canal := func(d float64) uint8 {
		if d < 0 {
			d++
		}
		if d > 1 {
			d--
		}
		switch {
		case d < 1.0/6:
			return uint8(math.Round((p + (q-p)*6*d) * 255))
		case d < 1.0/2:
			return uint8(math.Round(q * 255))
		case d < 2.0/3:
			return uint8(math.Round((p + (q-p)*(2.0/3-d)*6) * 255))
		}
		return uint8(math.Round(p * 255))
	}
	return Couleur{canal(t + 1.0/3), canal(t), canal(t - 1.0/3)}
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// Melange interpole deux couleurs dans l'espace sRGB. Suffisant pour teinter un gris d'une pointe de la
// couleur dominante ; on ne cherche pas ici la justesse perceptuelle d'un espace Lab.
func Melange(a, b Couleur, part float64) Couleur {
	part = clamp01(part)
	m := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x)*(1-part) + float64(y)*part))
	}
	return Couleur{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B)}
}

// PourContraste éclaircit ou assombrit `couleur` jusqu'à ce qu'elle atteigne `vise` face à `fond`, en
// gardant sa teinte et sa saturation.
//
// Le sens n'est PAS déduit de la luminance du fond. Sur un fond de luminance moyenne — 0,35 par exemple —
// le blanc ne donne que 2,6:1 alors que le noir donne 8:1 : un seuil à 0,5 enverrait vers le blanc, donc
// vers un texte illisible que rien ne rattrape ensuite. On regarde donc ce que chaque extrême atteint
// vraiment, et on ne garde que les sens qui permettent d'atteindre la cible. Si les deux conviennent, on
// retient la couleur la plus proche de celle demandée, pour s'éloigner le moins possible de la palette.
//
// C'est la fonction qui sépare un thème utilisable d'un thème joli et illisible : toute couleur de texte
// produite par le générateur y passe.
func PourContraste(couleur, fond Couleur, vise float64) Couleur {
	if Contraste(couleur, fond) >= vise {
		return couleur
	}
	h := couleur.TSL()
	blanc, noir := Couleur{255, 255, 255}, Couleur{0, 0, 0}
	versBlanc := Contraste(blanc, fond) >= vise
	versNoir := Contraste(noir, fond) >= vise
	if !versBlanc && !versNoir {
		// Cible inatteignable quelle que soit la couleur : on rend l'extrême le moins mauvais.
		if Contraste(blanc, fond) >= Contraste(noir, fond) {
			return blanc
		}
		return noir
	}

	// bissecter cherche la luminosité la plus proche de l'originale qui atteigne la cible.
	bissecter := func(versClair bool) (Couleur, bool) {
		bas, haut := h.L, 1.0
		extreme := blanc
		if !versClair {
			bas, haut, extreme = 0.0, h.L, noir
		}
		if Contraste(extreme, fond) < vise {
			return Couleur{}, false
		}
		meilleure := extreme
		for range 24 {
			milieu := (bas + haut) / 2
			essai := TSL{h.T, h.S, milieu}.Couleur()
			if Contraste(essai, fond) >= vise {
				meilleure = essai
				if versClair {
					haut = milieu
				} else {
					bas = milieu
				}
			} else if versClair {
				bas = milieu
			} else {
				haut = milieu
			}
		}
		return meilleure, true
	}

	clair, okClair := bissecter(true)
	sombre, okSombre := bissecter(false)
	switch {
	case okClair && okSombre:
		// Les deux marchent : on garde l'écart de luminosité le plus faible.
		if math.Abs(clair.TSL().L-h.L) <= math.Abs(sombre.TSL().L-h.L) {
			return clair
		}
		return sombre
	case okClair:
		return clair
	case okSombre:
		return sombre
	}
	return blanc // inatteignable : écarté plus haut
}

// SurFond rend la couleur de texte la plus lisible sur ce fond : du quasi-noir ou du quasi-blanc, teinté de
// la couleur du fond pour que l'ensemble reste d'une seule famille.
func SurFond(fond, teinte Couleur) Couleur {
	if fond.Luminance() < 0.45 {
		return Melange(Couleur{245, 245, 245}, teinte, .08)
	}
	return Melange(Couleur{26, 26, 26}, teinte, .08)
}
