package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
)

// Deux zones de l'interface sont posées à même le fond d'écran, sans couleur de fond propre : la barre du
// haut et le panneau des dossiers. Sur un dégradé dessiné pour wmail-go, le fond est assombri à ces
// endroits exprès ; sur une photo quelconque, rien ne garantit que les noms de dossiers restent lisibles —
// ils disparaissent dès que la photo est claire à gauche.
//
// voiler assombrit donc une bande à gauche et une bande en haut. Le voile tient sa force sur toute la zone
// où le texte se pose, puis s'efface au-delà : s'il commençait à décroître dès le bord, les derniers noms de
// dossiers — ceux qui touchent le bord droit du panneau — ne recevraient presque rien.
//
// Les zones de texte sont celles de l'interface : --folders-w vaut 17,643 rem et --bar-h 3,571 rem, soit
// 248 et 50 pixels à 14 px. L'effacement court jusqu'à 30 % et 20 % de l'image.
const (
	partGauche = 248.0 / 1920 // largeur du panneau des dossiers, en fraction de la largeur
	partHaute  = 50.0 / 1080  // hauteur de la barre du haut, en fraction de la hauteur
	finGauche  = 0.30         // le voile gauche est nul au-delà de cette fraction de la largeur
	finHaute   = 0.20         // le voile haut est nul au-delà de cette fraction de la hauteur
)

// BandesLuminance rend, pour la bande gauche et pour la bande haute, la luminance du 90e centile — et non
// la moyenne.
//
// La lisibilité ne se perd pas sur la moyenne d'une zone mais sur ses taches claires : un portrait sombre
// dont l'épaule éclairée traverse la colonne des dossiers a une moyenne basse et des noms de dossiers
// illisibles au milieu. Le 90e centile décrit ce que l'œil voit comme « la partie claire » sans se laisser
// dicter sa valeur par quelques pixels brûlés, que le 100e centile suivrait.
func BandesLuminance(img image.Image) (gauche, haute float64) {
	b := img.Bounds()
	largeur := int(float64(b.Dx()) * partGauche)
	hauteur := int(float64(b.Dy()) * partHaute)
	gauche = luminanceZone(img, image.Rect(b.Min.X, b.Min.Y, b.Min.X+max(1, largeur), b.Max.Y))
	haute = luminanceZone(img, image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+max(1, hauteur)))
	return gauche, haute
}

func luminanceZone(img image.Image, z image.Rectangle) float64 {
	z = z.Intersect(img.Bounds())
	if z.Empty() {
		return 0.5
	}
	pas := 1
	if total := z.Dx() * z.Dy(); total > 40000 {
		pas = int(math.Sqrt(float64(total) / 40000))
	}
	var vals []float64
	for y := z.Min.Y; y < z.Max.Y; y += pas {
		for x := z.Min.X; x < z.Max.X; x += pas {
			r, g, b, _ := img.At(x, y).RGBA()
			vals = append(vals, Couleur{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}.Luminance())
		}
	}
	if len(vals) == 0 {
		return 0.5
	}
	sort.Float64s(vals)
	return vals[min(len(vals)-1, len(vals)*9/10)]
}

// Voiler rend une copie de l'image avec les deux bandes assombries. force vaut 0 à 1 : c'est l'opacité du
// noir au bord, qui décroît jusqu'à zéro vers l'intérieur de l'image.
//
// L'assombrissement est appliqué en linéaire puis réencodé, pour que le dégradé ne se voie pas par bandes.
func Voiler(src image.Image, force float64) image.Image {
	b := src.Bounds()
	out := image.NewRGBA(b)
	plateauX := math.Max(1, float64(b.Dx())*partGauche)
	plateauY := math.Max(1, float64(b.Dy())*partHaute)
	finX := math.Max(plateauX+1, float64(b.Dx())*finGauche)
	finY := math.Max(plateauY+1, float64(b.Dy())*finHaute)

	// Plein régime sur la zone de texte, puis décroissance en cosinus plutôt que linéaire : la transition
	// est invisible à l'œil, là où une rampe droite laisse voir l'arête au point où elle s'arrête.
	part := func(d, plateau, fin float64) float64 {
		switch {
		case d <= plateau:
			return 1
		case d >= fin:
			return 0
		default:
			return (math.Cos((d-plateau)/(fin-plateau)*math.Pi) + 1) / 2
		}
	}

	for y := b.Min.Y; y < b.Max.Y; y++ {
		fy := part(float64(y-b.Min.Y), plateauY, finY)
		for x := b.Min.X; x < b.Max.X; x++ {
			fx := part(float64(x-b.Min.X), plateauX, finX)
			// Les deux bandes se recouvrent dans le coin : on prend la plus forte, pas leur somme, sinon
			// le coin vire au noir franc.
			f := math.Max(fx, fy) * force
			r16, g16, b16, a16 := src.At(x, y).RGBA()
			out.Set(x, y, color.RGBA{
				R: uint8(float64(r16>>8) * (1 - f)),
				G: uint8(float64(g16>>8) * (1 - f)),
				B: uint8(float64(b16>>8) * (1 - f)),
				A: uint8(a16 >> 8),
			})
		}
	}
	return out
}

// ForceDuVoile dit quelle opacité donner au voile pour que `texte` atteigne `vise` sur une bande de cette
// luminance, et 0 s'il n'en faut pas. Le voile est plafonné : au-delà, autant choisir une autre image que
// la noircir au point de la perdre.
func ForceDuVoile(bande float64, texte Couleur, vise float64) float64 {
	const plafond = 0.65
	contrasteBande := func(f float64) float64 {
		// Une bande assombrie de f voit sa luminance multipliée par (1-f) en sRGB ; on repasse par une
		// couleur grise équivalente pour réutiliser la même formule de contraste que partout ailleurs.
		v := uint8(math.Round(math.Pow(bande, 1/2.2) * 255 * (1 - f)))
		return Contraste(texte, Couleur{v, v, v})
	}
	if contrasteBande(0) >= vise {
		return 0
	}
	for f := 0.05; f <= plafond; f += 0.05 {
		if contrasteBande(f) >= vise {
			return math.Round(f*100) / 100
		}
	}
	return plafond
}

// VoileCSS rend les couches de dégradé qui assombrissent la bande gauche et la bande haute **dans la pile
// `--wall`**, sans toucher au fichier d'image.
//
// C'est la seule voie pour un SVG : le rasteriser pour y peindre un voile le dénaturerait, et rien dans Go ne
// le dessine. Les deux couches passent devant l'image parce qu'elles sont écrites avant elle dans la pile.
// Chaque couche tient sur une ligne : dropMissingWallpaper travaille ligne par ligne, et une couche coupée en
// deux lui échapperait.
func VoileCSS(force float64) string {
	if force <= 0 {
		return ""
	}
	return fmt.Sprintf(
		"linear-gradient(90deg, rgba(0, 0, 0, %.2f) 0 %.0f%%, rgba(0, 0, 0, 0) %.0f%%),\n    "+
			"linear-gradient(180deg, rgba(0, 0, 0, %.2f) 0 %.0f%%, rgba(0, 0, 0, 0) %.0f%%),\n    ",
		force, partGauche*100, finGauche*100, force, partHaute*100, finHaute*100)
}
