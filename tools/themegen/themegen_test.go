package main

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestContrasteValeursConnues(t *testing.T) {
	noir, blanc := Couleur{0, 0, 0}, Couleur{255, 255, 255}
	if got := Contraste(noir, blanc); math.Abs(got-21) > 0.01 {
		t.Errorf("noir sur blanc = %.2f, attendu 21", got)
	}
	if got := Contraste(blanc, blanc); got != 1 {
		t.Errorf("blanc sur blanc = %.2f, attendu 1", got)
	}
	// Valeur de référence des outils WCAG : #767676 est le gris le plus clair qui tienne 4,5:1 sur blanc.
	gris, _ := parseHex("#767676")
	if got := Contraste(gris, blanc); got < 4.5 || got > 4.6 {
		t.Errorf("#767676 sur blanc = %.2f, attendu entre 4,5 et 4,6", got)
	}
}

// PourContraste doit tenir sa promesse quelle que soit la couleur de départ : c'est sur elle que repose la
// lisibilité de tout thème généré. Le cas qui a échoué en vrai est celui d'un fond de luminance moyenne,
// où le blanc ne suffit pas mais le noir convient : un choix de sens fondé sur un seuil de 0,5 envoyait
// alors vers le blanc et laissait un texte à 2,5:1.
func TestPourContrasteTientSaPromesse(t *testing.T) {
	var echecs int
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 17 {
			for b := 0; b < 256; b += 51 {
				fond := Couleur{uint8(r), uint8(g), uint8(b)}
				atteignable := math.Max(
					Contraste(Couleur{255, 255, 255}, fond),
					Contraste(Couleur{0, 0, 0}, fond))
				for _, depart := range []Couleur{{128, 128, 128}, {200, 60, 40}, {30, 90, 160}, fond} {
					got := PourContraste(depart, fond, ContrasteTexte)
					obtenu := Contraste(got, fond)
					if obtenu < ContrasteTexte && atteignable >= ContrasteTexte {
						echecs++
						if echecs < 4 {
							t.Errorf("fond %s, départ %s : %.2f:1 alors que %.2f:1 était atteignable",
								fond.Hex(), depart.Hex(), obtenu, atteignable)
						}
					}
				}
			}
		}
	}
	if echecs > 0 {
		t.Errorf("%d combinaisons sous le seuil", echecs)
	}
}

// Quand la cible est hors d'atteinte — un fond de luminance moyenne ne donne jamais 7:1 — la fonction doit
// rendre le meilleur extrême, pas une couleur quelconque.
func TestPourContrasteCibleInatteignable(t *testing.T) {
	fond := Couleur{128, 128, 128}
	got := PourContraste(Couleur{120, 120, 120}, fond, 10)
	if got != (Couleur{0, 0, 0}) && got != (Couleur{255, 255, 255}) {
		t.Errorf("cible inatteignable : %s, attendu un extrême", got.Hex())
	}
	if Contraste(got, fond) < Contraste(Couleur{255, 255, 255}, fond) &&
		Contraste(got, fond) < Contraste(Couleur{0, 0, 0}, fond) {
		t.Error("l'extrême rendu n'est pas le meilleur des deux")
	}
}

func TestTSLAllerRetour(t *testing.T) {
	for _, c := range []Couleur{{0, 0, 0}, {255, 255, 255}, {128, 64, 32}, {12, 200, 90}, {200, 200, 200}} {
		got := c.TSL().Couleur()
		if abs(int(got.R)-int(c.R)) > 1 || abs(int(got.G)-int(c.G)) > 1 || abs(int(got.B)-int(c.B)) > 1 {
			t.Errorf("%s -> TSL -> %s", c.Hex(), got.Hex())
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestParseHex(t *testing.T) {
	cas := map[string]Couleur{
		"#abc":       {0xaa, 0xbb, 0xcc},
		"#A1B2C3":    {0xa1, 0xb2, 0xc3},
		"  #112233 ": {0x11, 0x22, 0x33},
		"#11223344":  {0x11, 0x22, 0x33}, // l'alpha est ignoré
	}
	for in, want := range cas {
		got, ok := parseHex(in)
		if !ok || got != want {
			t.Errorf("parseHex(%q) = %s, %v ; attendu %s", in, got.Hex(), ok, want.Hex())
		}
	}
	for _, in := range []string{"", "#", "#12", "#12345", "rouge", "rgb(1,2,3)"} {
		if _, ok := parseHex(in); ok {
			t.Errorf("parseHex(%q) accepté à tort", in)
		}
	}
}

func TestPaletteSVG(t *testing.T) {
	svg := []byte(`<svg><defs><linearGradient id="g">
	  <stop offset="0" stop-color="#140b2e"/><stop offset="1" stop-color="#f0936a"/></linearGradient></defs>
	  <rect fill="url(#g)"/><circle fill="#140b2e"/><circle fill="#140b2e"/><path stroke='#fff'/></svg>`)
	p := paletteSVG(svg)
	if len(p.Teintes) != 3 {
		t.Fatalf("%d teintes : %v", len(p.Teintes), p.Teintes)
	}
	// #140b2e apparaît trois fois sur cinq couleurs lues : c'est la dominante.
	if p.Dominante().Hex() != "#140b2e" {
		t.Errorf("dominante %s, attendu #140b2e", p.Dominante().Hex())
	}
	// url(#g) ne doit pas être pris pour une couleur.
	for _, x := range p.Teintes {
		if x.Couleur.Hex() == "#00000g" {
			t.Error("référence de dégradé lue comme une couleur")
		}
	}
}

func TestPaletteSVGSansCouleur(t *testing.T) {
	if p := paletteSVG([]byte(`<svg><rect fill="red"/></svg>`)); len(p.Teintes) != 0 {
		t.Errorf("les couleurs nommées ne sont pas lues : %v", p.Teintes)
	}
}

func imageUnie(c color.RGBA, l, h int) image.Image {
	m := image.NewRGBA(image.Rect(0, 0, l, h))
	for y := range h {
		for x := range l {
			m.Set(x, y, c)
		}
	}
	return m
}

func TestQuantifier(t *testing.T) {
	m := imageUnie(color.RGBA{200, 100, 50, 255}, 40, 30)
	p := quantifier(m, 8)
	if len(p.Teintes) != 1 {
		t.Fatalf("%d teintes pour une image unie", len(p.Teintes))
	}
	if got := p.Teintes[0].Couleur; abs(int(got.R)-200) > 2 || abs(int(got.G)-100) > 2 || abs(int(got.B)-50) > 2 {
		t.Errorf("teinte %s, attendue proche de #c86432", got.Hex())
	}
	if p.Teintes[0].Poids < 0.99 {
		t.Errorf("poids %.2f pour une image unie", p.Teintes[0].Poids)
	}
}

// Le contrat du générateur : quelle que soit l'image, le thème produit est lisible. On balaie des palettes
// couvrant la roue des teintes, du très sombre au très clair, saturé et terne.
func TestConstruireRendToujoursUnThemeLisible(t *testing.T) {
	for _, teinte := range []float64{0, 40, 120, 200, 280, 330} {
		for _, lum := range []float64{.06, .25, .5, .75, .95} {
			for _, sat := range []float64{.05, .45, .95} {
				for _, sombres := range []bool{false, true} {
					base := TSL{teinte, sat, lum}.Couleur()
					accent := TSL{math.Mod(teinte+150, 360), math.Min(1, sat+.3), .5}.Couleur()
					p := Palette{
						Teintes:   []Teinte{{base, .7}, {accent, .3}},
						Luminance: base.Luminance(),
					}
					th := Construire("essai", p, sombres, "#000")
					for _, c := range th.Controles {
						if strings.HasPrefix(c.Fond, "(") {
							continue // posé sur le fond d'écran : traité par le voile, pas par une variable
						}
						if !c.Passe() {
							t.Errorf("teinte %.0f lum %.2f sat %.2f panneaux-sombres=%v : %s à %.2f:1 (visé %.1f)",
								teinte, lum, sat, sombres, c.Quoi, c.Obtenu, c.Vise)
						}
					}
				}
			}
		}
	}
}

// Les 65 variables de docs/themes.md, dans l'ordre. Si cette liste change, docs/themes.md et
// internal/web/themes_test.go doivent changer avec elle.
var vars65 = []string{
	"--wall",
	"--login-bg", "--login-fg", "--login-border", "--login-radius", "--login-shadow",
	"--login-title", "--login-title-shadow", "--login-link",
	"--bg", "--fg", "--muted", "--card-bg", "--border", "--input-bg", "--accent", "--accent-fg", "--link",
	"--bar-bg", "--bar-fg", "--toolbar-bg", "--btn-bg", "--btn-hover", "--btn-fg", "--btn-bd",
	"--new-bg", "--new-fg", "--new-bd",
	"--menu-bg", "--menu-fg", "--menu-hover-bg", "--menu-hover-fg",
	"--folders-fg", "--folders-disabled", "--folders-sel", "--folders-sep", "--badge-bg", "--badge-fg",
	"--list-bg", "--row-bg", "--row-hover", "--row-bd", "--row-fg", "--sender", "--date",
	"--unread-bg", "--unread-bar", "--subunread-bg", "--subunread-bar", "--sel-bg", "--sel-bar",
	"--focus-ring", "--focus-tint", "--star", "--star-on",
	"--read-bg", "--read-fg",
	"--bar-gray", "--bar-line", "--danger-bg", "--danger-fg", "--info-bg", "--info-fg",
	"--warn-bg", "--warn-fg",
}

func TestConstruireRendLes65VariablesDansLOrdre(t *testing.T) {
	th := Construire("essai", Palette{Teintes: []Teinte{{Couleur{40, 60, 90}, 1}}, Luminance: .2}, false, "#000")
	var got []string
	for _, g := range th.Groupes {
		for _, v := range g.Variables {
			got = append(got, v.Nom)
			if strings.TrimSpace(v.Valeur) == "" {
				t.Errorf("%s : valeur vide", v.Nom)
			}
			if v.Role == "" {
				t.Errorf("%s : rôle non décrit, l'éditeur afficherait un réglage sans libellé", v.Nom)
			}
		}
	}
	if len(got) != len(vars65) {
		t.Fatalf("%d variables, attendu %d", len(got), len(vars65))
	}
	for i := range got {
		if got[i] != vars65[i] {
			t.Errorf("position %d : %s, attendu %s", i, got[i], vars65[i])
		}
	}
}

// Le CSS produit doit être celui qu'attend le serveur : les chemins d'image absolus, et pas une seule
// variable manquante — un thème incomplet retombe en silence sur les couleurs d'un autre.
func TestCSSProduit(t *testing.T) {
	fond := `url("/themes/essai/background.jpg") left top / cover no-repeat`
	th := Construire("essai", Palette{Teintes: []Teinte{{Couleur{40, 60, 90}, 1}}, Luminance: .2}, false, fond)
	css := th.CSS(false)
	if !strings.Contains(css, fond) {
		t.Error("le chemin du fond doit rester absolu")
	}
	if !strings.Contains(css, "color-scheme: light") {
		t.Error("color-scheme manquant : les champs natifs garderaient le mauvais thème")
	}
	for _, v := range vars65 {
		if !strings.Contains(css, "\n  "+v+":") {
			t.Errorf("%s absente du CSS", v)
		}
	}
}

func TestApercuEstAutonome(t *testing.T) {
	th := Construire("essai", Palette{Teintes: []Teinte{{Couleur{40, 60, 90}, 1}}, Luminance: .2}, false,
		`url("/themes/essai/background.jpg") left top / cover no-repeat`)
	html := th.Apercu("jpg")
	for _, interdit := range []string{"http://", "https://", "<link"} {
		if strings.Contains(html, interdit) {
			t.Errorf("la page d'aperçu doit être autonome : %q trouvé", interdit)
		}
	}
	for _, attendu := range []string{"tg-liste-contrastes", "tg-css", "--row-fg", "wm-row sel"} {
		if !strings.Contains(html, attendu) {
			t.Errorf("%q absent de la page d'aperçu", attendu)
		}
	}
}

func TestForceDuVoile(t *testing.T) {
	blanc := Couleur{255, 255, 255}
	if f := ForceDuVoile(0.02, blanc, ContrasteTexte); f != 0 {
		t.Errorf("bande déjà très sombre : voile %.2f, attendu 0", f)
	}
	f := ForceDuVoile(0.75, blanc, ContrasteTexte)
	if f <= 0 {
		t.Errorf("bande claire sous texte blanc : voile %.2f, attendu > 0", f)
	}
	if f > 0.65 {
		t.Errorf("voile %.2f : le plafond de 0,65 doit tenir", f)
	}
}

// Le voile doit assombrir le bord gauche et le haut, et laisser le centre intact : c'est là qu'est l'image.
func TestVoilerNAssombritQueLesBandes(t *testing.T) {
	src := imageUnie(color.RGBA{200, 200, 200, 255}, 200, 100)
	out := Voiler(src, 0.5)
	coin := luminanceZone(out, image.Rect(0, 0, 3, 3))
	centre := luminanceZone(out, image.Rect(120, 60, 160, 90))
	origine := luminanceZone(src, image.Rect(120, 60, 160, 90))
	if coin >= centre {
		t.Errorf("coin %.3f, centre %.3f : le bord doit être plus sombre", coin, centre)
	}
	if math.Abs(centre-origine) > 0.01 {
		t.Errorf("centre modifié : %.3f contre %.3f à l'origine", centre, origine)
	}
}

// La décision de voiler se prend sur le 90e centile, pas sur la moyenne : une image sombre traversée d'une
// zone claire doit être voilée, alors que sa moyenne ne le réclamerait pas.
func TestBandesLuminanceSuitLesTachesClaires(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			c := color.RGBA{10, 10, 10, 255}
			if y > 120 && y < 220 { // bande claire en travers de la colonne des dossiers
				c = color.RGBA{235, 235, 235, 255}
			}
			m.Set(x, y, c)
		}
	}
	gauche, _ := BandesLuminance(m)
	if gauche < 0.5 {
		t.Errorf("90e centile de la bande gauche = %.2f : la tache claire doit l'emporter sur la moyenne", gauche)
	}
}
