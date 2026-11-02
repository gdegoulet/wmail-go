package main

import (
	"image"
	"math"
	"regexp"
	"sort"
)

// Teinte est une couleur de la palette, avec le poids qu'elle occupe dans l'image (0-1).
type Teinte struct {
	Couleur Couleur
	Poids   float64
}

// Palette est le résultat de l'analyse d'une image : les teintes par poids décroissant, et la luminance
// moyenne de l'image entière.
type Palette struct {
	Teintes   []Teinte
	Luminance float64
}

// Dominante est la teinte la plus présente. Elle donne le ton général du thème.
func (p Palette) Dominante() Couleur {
	if len(p.Teintes) == 0 {
		return Couleur{128, 128, 128}
	}
	return p.Teintes[0].Couleur
}

// Accent choisit la couleur d'accentuation : la plus saturée parmi celles qui pèsent assez pour avoir été
// voulues. Une couleur vive présente sur trois pixels ne représente pas l'image.
//
// Le score mêle saturation et poids plutôt que de trier sur la seule saturation : sans cela, le liseré
// d'un aplat compressé en JPEG l'emporterait sur la couleur du sujet.
func (p Palette) Accent() Couleur {
	meilleur, score := p.Dominante(), -1.0
	for _, t := range p.Teintes {
		if t.Poids < 0.01 {
			continue
		}
		h := t.Couleur.TSL()
		// On écarte le presque-noir et le presque-blanc : ils ne font pas un accent.
		if h.L < 0.12 || h.L > 0.93 {
			continue
		}
		s := h.S * math.Sqrt(t.Poids)
		if s > score {
			meilleur, score = t.Couleur, s
		}
	}
	return meilleur
}

// Sombre dit si l'image est globalement sombre, ce qui décide du traitement du panneau des dossiers et de
// la barre du haut : ils sont posés à même le fond d'écran.
func (p Palette) Sombre() bool { return p.Luminance < 0.4 }

// --- quantification d'une image matricielle ---

// quantifier regroupe les pixels dans une grille grossière de l'espace RGB, puis rend les cases les plus
// peuplées. Une grille de 6 niveaux par canal (216 cases) suffit pour un fond d'écran : on cherche
// l'ambiance, pas une palette d'imprimeur. Un k-means donnerait des centres plus justes pour un coût et une
// complexité sans rapport avec l'usage.
func quantifier(img image.Image, maxTeintes int) Palette {
	const n = 6
	type case_ struct {
		r, g, b, poids float64
	}
	cases := make(map[int]*case_, 256)

	b := img.Bounds()
	// Échantillonnage : au plus 160 000 pixels, suffisant et borné quelle que soit la taille de l'image.
	pas := 1
	if total := b.Dx() * b.Dy(); total > 160000 {
		pas = int(math.Sqrt(float64(total) / 160000))
	}
	var somme, compte float64
	for y := b.Min.Y; y < b.Max.Y; y += pas {
		for x := b.Min.X; x < b.Max.X; x += pas {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8000 {
				continue // pixel transparent : il ne participe pas au fond
			}
			c := Couleur{uint8(r16 >> 8), uint8(g16 >> 8), uint8(b16 >> 8)}
			somme += c.Luminance()
			compte++
			idx := int(c.R)*n/256*n*n + int(c.G)*n/256*n + int(c.B)*n/256
			k := cases[idx]
			if k == nil {
				k = &case_{}
				cases[idx] = k
			}
			k.r += float64(c.R)
			k.g += float64(c.G)
			k.b += float64(c.B)
			k.poids++
		}
	}
	if compte == 0 {
		return Palette{Luminance: 0.5}
	}
	teintes := make([]Teinte, 0, len(cases))
	for _, k := range cases {
		teintes = append(teintes, Teinte{
			Couleur: Couleur{uint8(k.r / k.poids), uint8(k.g / k.poids), uint8(k.b / k.poids)},
			Poids:   k.poids / compte,
		})
	}
	trierTeintes(teintes)
	if len(teintes) > maxTeintes {
		teintes = teintes[:maxTeintes]
	}
	return Palette{Teintes: teintes, Luminance: somme / compte}
}

func trierTeintes(t []Teinte) {
	sort.SliceStable(t, func(i, j int) bool { return t[i].Poids > t[j].Poids })
}

// --- couleurs d'un SVG ---

var reCouleurSVG = regexp.MustCompile(`(?i)(?:fill|stop-color|stroke|flood-color)\s*[=:]\s*["']?\s*(#[0-9a-f]{3,8})`)

// paletteSVG lit les couleurs déclarées dans un SVG, sans le rastériser.
//
// wmail-go n'embarque aucun moteur de rendu SVG et n'en ajoutera pas un pour générer un thème. On lit donc
// les attributs de couleur du fichier. Les limites sont assumées et affichées à l'utilisateur :
//
//   - le poids d'une couleur est son nombre d'occurrences, pas la surface qu'elle couvre ;
//   - un SVG qui peint par filtres, masques ou images incorporées donnera une palette pauvre ;
//   - les couleurs nommées (« red ») et les fonctions (« rgb() ») ne sont pas lues.
//
// Sur les fonds livrés avec wmail-go, faits de dégradés et de cercles, le résultat est fidèle.
func paletteSVG(src []byte) Palette {
	compte := map[Couleur]float64{}
	var total float64
	for _, m := range reCouleurSVG.FindAllSubmatch(src, -1) {
		c, ok := parseHex(string(m[1]))
		if !ok {
			continue
		}
		compte[c]++
		total++
	}
	if total == 0 {
		return Palette{Luminance: 0.5}
	}
	teintes := make([]Teinte, 0, len(compte))
	var somme float64
	for c, n := range compte {
		teintes = append(teintes, Teinte{Couleur: c, Poids: n / total})
		somme += c.Luminance() * n / total
	}
	trierTeintes(teintes)
	if len(teintes) > 16 {
		teintes = teintes[:16]
	}
	return Palette{Teintes: teintes, Luminance: somme}
}
