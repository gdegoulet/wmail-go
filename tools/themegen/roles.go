package main

import "fmt"

// Groupe est une section du thème ; l'ordre des groupes et des variables est celui de docs/themes.md, et
// celui qu'affiche l'éditeur d'aperçu.
type Groupe struct {
	Titre     string
	Variables []Variable
}

// Variable est une déclaration du thème. Couleur vaut nil pour les valeurs qui ne sont pas une couleur unie
// (pile du fond d'écran, bordure, arrondi, ombre) : l'éditeur les présente alors en texte libre.
type Variable struct {
	Nom     string
	Valeur  string
	Couleur *Couleur
	Role    string // ce que la variable peint, en une ligne, repris dans l'éditeur
}

// Controle est un couple à vérifier : un texte sur son fond, avec le rapport exigé.
type Controle struct {
	Quoi         string
	Texte, Fond  string // noms de variables
	Vise, Obtenu float64
}

func (c Controle) Passe() bool { return c.Obtenu >= c.Vise }

// Theme est un thème complet, prêt à écrire.
type Theme struct {
	Nom       string
	Groupes   []Groupe
	Controles []Controle
	Palette   Palette
}

func coul(nom string, c Couleur, role string) Variable {
	cc := c
	return Variable{Nom: nom, Valeur: c.Hex(), Couleur: &cc, Role: role}
}

func libre(nom, valeur, role string) Variable {
	return Variable{Nom: nom, Valeur: valeur, Role: role}
}

// Construire déduit les 65 variables d'une palette.
//
// Le principe : le fond d'écran est l'image, et deux zones seulement sont posées dessus — la barre du haut
// et le panneau des dossiers. Tout le reste (liste, lecture, menus, cartes) est opaque et se construit à
// partir d'un neutre teinté de la couleur dominante. La couleur vive de l'image ne sert qu'aux accents :
// bouton « Nouveau », ligne ouverte, étoile allumée. Un thème qui peindrait la liste avec la couleur vive
// serait joli en capture d'écran et épuisant à l'usage.
//
// Chaque couleur de texte passe par PourContraste, donc le thème produit est lisible par construction, pas
// par chance.
func Construire(nom string, p Palette, panneauxSombres bool, fondURL string) *Theme {
	dominante := p.Dominante()
	accent := p.Accent()
	hDom := dominante.TSL()

	// --- les panneaux opaques ---
	// Un neutre très peu saturé, teinté de la dominante : les panneaux appartiennent visuellement à
	// l'image sans en reprendre la vivacité.
	var panneau, ligne, ligneSurvol, lecture Couleur
	if panneauxSombres {
		panneau = TSL{hDom.T, min01(hDom.S, .10), .09}.Couleur()
		ligne = TSL{hDom.T, min01(hDom.S, .08), .13}.Couleur()
		ligneSurvol = TSL{hDom.T, min01(hDom.S, .08), .17}.Couleur()
		lecture = TSL{hDom.T, min01(hDom.S, .06), .11}.Couleur()
	} else {
		panneau = TSL{hDom.T, min01(hDom.S, .12), .99}.Couleur()
		ligne = TSL{hDom.T, min01(hDom.S, .10), .975}.Couleur()
		ligneSurvol = TSL{hDom.T, min01(hDom.S, .12), .95}.Couleur()
		lecture = TSL{hDom.T, min01(hDom.S, .06), .995}.Couleur()
	}
	texte := SurFond(panneau, dominante)
	texteDiscret := PourContraste(Melange(texte, panneau, .45), panneau, ContrasteElement)
	bordure := Melange(panneau, texte, .18)

	// --- ce qui est posé sur le fond d'écran ---
	// La barre du haut est un voile : sur une image, une couleur unie « flotte », un voile épouse l'image.
	// Son texte doit contraster avec le voile une fois composé sur l'image, d'où le calcul sur la couleur
	// résultante plutôt que sur le voile seul.
	voile := Couleur{0, 0, 0}
	alphaVoile := .5
	if p.Sombre() {
		// Fond déjà sombre : un voile noir n'y ajoute rien de lisible, on l'éclaircit à peine.
		voile, alphaVoile = Couleur{0, 0, 0}, .35
	}
	barreComposee := Melange(moyenne(p), voile, alphaVoile)
	barreTexte := PourContraste(SurFond(barreComposee, dominante), barreComposee, ContrasteTexte)
	dossiersTexte := PourContraste(SurFond(barreComposee, dominante), barreComposee, ContrasteTexte)

	// --- accents ---
	accentLisible := PourContraste(accent, panneau, ContrasteElement)
	lien := PourContraste(accent, panneau, ContrasteTexte)
	nouveau := accent
	if accent.TSL().S < .25 { // image terne : un bouton d'action doit rester repérable
		nouveau = TSL{hDom.T, .55, .42}.Couleur()
	}
	nouveauTexte := PourContraste(SurFond(nouveau, nouveau), nouveau, ContrasteTexte)

	// Non lus, sélection, anomalies : des teintes conventionnelles, juste accordées au thème. Le jaune des
	// non-lus et le bleu de la sélection sont des repères appris, on ne les remplace pas par la dominante.
	nonLu := TSL{52, .85, .93}.Couleur()
	nonLuBarre := TSL{38, 1, .5}.Couleur()
	selection := TSL{210, .95, .94}.Couleur()
	selectionBarre := TSL{212, .88, .59}.Couleur()
	if panneauxSombres {
		nonLu = TSL{52, .30, .12}.Couleur()
		nonLuBarre = TSL{38, .55, .40}.Couleur()
		selection = TSL{210, .35, .16}.Couleur()
		selectionBarre = TSL{212, .70, .52}.Couleur()
	}

	fond := fondURL
	if fond == "" {
		fond = degradeDeRepli(p)
	}

	t := &Theme{Nom: nom, Palette: p}
	t.Groupes = []Groupe{
		{"Fond d'écran", []Variable{
			libre("--wall", fond, "fond de la fenêtre entière, derrière les trois volets"),
		}},
		{"Page de connexion", []Variable{
			libre("--login-bg", voile.RGBA(.5), "fond de la carte de connexion"),
			coul("--login-fg", PourContraste(Couleur{238, 238, 238}, Melange(moyenne(p), voile, .5), ContrasteTexte), "texte dans la carte"),
			libre("--login-border", "none", "bordure de la carte"),
			libre("--login-radius", ".5rem", "arrondi de la carte"),
			libre("--login-shadow", "0 2px 10px "+Couleur{0, 0, 0}.RGBA(.5), "ombre portée de la carte"),
			coul("--login-title", Couleur{221, 221, 221}, "titre « Connexion »"),
			libre("--login-title-shadow", "none", "ombre du titre"),
			coul("--login-link", Couleur{221, 221, 221}, "liens sous le formulaire"),
		}},
		{"Couleurs de base", []Variable{
			coul("--bg", Melange(panneau, texte, .09), "fond des pages hors courrier"),
			coul("--fg", texte, "texte courant"),
			coul("--muted", texteDiscret, "texte secondaire"),
			coul("--card-bg", panneau, "fond des cartes (réglages, contacts)"),
			coul("--border", bordure, "bordures des cartes et des champs"),
			coul("--input-bg", panneau, "fond des champs de saisie"),
			coul("--accent", accentLisible, "accentuation, contour de focus"),
			coul("--accent-fg", PourContraste(SurFond(accentLisible, accentLisible), accentLisible, ContrasteTexte), "texte posé sur l'accent"),
			coul("--link", lien, "liens"),
		}},
		{"Barres et boutons", []Variable{
			libre("--bar-bg", voile.RGBA(alphaVoile), "barre du haut, posée sur le fond d'écran"),
			coul("--bar-fg", barreTexte, "texte de la barre du haut"),
			libre("--toolbar-bg", fmt.Sprintf("linear-gradient(%s, %s)", Melange(panneau, texte, .04).Hex(), Melange(panneau, texte, .13).Hex()), "barres d'outils de la liste"),
			libre("--btn-bg", fmt.Sprintf("linear-gradient(%s, %s)", panneau.Hex(), Melange(panneau, texte, .10).Hex()), "fond des boutons clairs"),
			coul("--btn-hover", Melange(panneau, texte, .10), "bouton clair survolé"),
			coul("--btn-fg", texte, "texte des boutons clairs"),
			libre("--btn-bd", Couleur{0, 0, 0}.RGBA(.25), "bordure des boutons clairs"),
			coul("--new-bg", nouveau, "bouton « Nouveau message »"),
			coul("--new-fg", nouveauTexte, "texte du bouton « Nouveau message »"),
			libre("--new-bd", Couleur{0, 0, 0}.RGBA(.25), "bordure du bouton « Nouveau message »"),
		}},
		{"Menus déroulants", []Variable{
			coul("--menu-bg", panneau, "fond d'un menu"),
			coul("--menu-fg", texte, "texte d'une entrée de menu"),
			coul("--menu-hover-bg", Melange(panneau, accentLisible, .85), "entrée survolée ou au clavier"),
			coul("--menu-hover-fg", PourContraste(SurFond(Melange(panneau, accentLisible, .85), accentLisible), Melange(panneau, accentLisible, .85), ContrasteTexte), "texte de l'entrée survolée"),
		}},
		{"Panneau des dossiers", []Variable{
			coul("--folders-fg", dossiersTexte, "nom d'un dossier, sur le fond d'écran"),
			coul("--folders-disabled", Melange(dossiersTexte, barreComposee, .40), "dossier non sélectionnable"),
			libre("--folders-sel", dossiersTexte.RGBA(.3), "fond du dossier courant"),
			coul("--folders-sep", Melange(dossiersTexte, barreComposee, .45), "trait entre dossiers système et personnels"),
			coul("--badge-bg", Melange(dossiersTexte, barreComposee, .45), "pastille du nombre de non-lus"),
			coul("--badge-fg", PourContraste(SurFond(Melange(dossiersTexte, barreComposee, .45), dominante), Melange(dossiersTexte, barreComposee, .45), ContrasteTexte), "chiffre dans la pastille"),
		}},
		{"Liste des messages", []Variable{
			coul("--list-bg", panneau, "fond du volet, sous les lignes"),
			coul("--row-bg", ligne, "fond d'une ligne lue"),
			coul("--row-hover", ligneSurvol, "ligne survolée"),
			coul("--row-bd", Melange(ligne, texte, .12), "trait entre deux lignes"),
			coul("--row-fg", PourContraste(texte, ligne, ContrasteTexte), "objet du message"),
			coul("--sender", PourContraste(texte, ligne, ContrasteTexte), "nom de l'expéditeur"),
			coul("--date", PourContraste(Melange(texte, ligne, .45), ligne, ContrasteElement), "date à droite de la ligne"),
			coul("--unread-bg", nonLu, "fond d'une ligne non lue"),
			coul("--unread-bar", nonLuBarre, "barre verticale d'une ligne non lue"),
			coul("--subunread-bg", nonLu, "conversation repliée contenant des non-lus"),
			coul("--subunread-bar", Melange(nonLuBarre, nonLu, .40), "barre verticale de cette conversation"),
			coul("--sel-bg", selection, "fond de la ligne ouverte"),
			coul("--sel-bar", selectionBarre, "barre verticale de la ligne ouverte"),
			coul("--focus-ring", TSL{2, .68, .48}.Couleur(), "contour de la ligne atteinte au clavier"),
			libre("--focus-tint", TSL{2, .68, .48}.Couleur().RGBA(.08), "voile posé sur cette ligne"),
			coul("--star", PourContraste(Melange(texte, ligne, .35), ligne, ContrasteElement), "étoile éteinte"),
			coul("--star-on", TSL{38, 1, .5}.Couleur(), "étoile allumée"),
		}},
		{"Volet de lecture", []Variable{
			coul("--read-bg", lecture, "fond du volet de lecture"),
			coul("--read-fg", PourContraste(texte, lecture, ContrasteTexte), "texte des en-têtes du message"),
		}},
		{"Bandeaux d'information", []Variable{
			coul("--bar-gray", Melange(panneau, texte, .07), "fond d'un bandeau neutre"),
			coul("--bar-line", Melange(panneau, texte, .14), "trait sous un bandeau"),
			coul("--danger-bg", teinteBandeau(0, panneauxSombres), "fond d'un message d'erreur"),
			coul("--danger-fg", PourContraste(TSL{354, .62, .43}.Couleur(), teinteBandeau(0, panneauxSombres), ContrasteTexte), "texte d'un message d'erreur"),
			coul("--info-bg", teinteBandeau(205, panneauxSombres), "fond d'un message d'information"),
			coul("--info-fg", PourContraste(TSL{205, .53, .28}.Couleur(), teinteBandeau(205, panneauxSombres), ContrasteTexte), "texte d'un message d'information"),
			coul("--warn-bg", teinteBandeau(38, panneauxSombres), "fond d'un avertissement"),
			coul("--warn-fg", PourContraste(TSL{40, 1, .27}.Couleur(), teinteBandeau(38, panneauxSombres), ContrasteTexte), "texte d'un avertissement"),
		}},
	}
	t.Controles = t.verifier(barreComposee)
	return t
}

func teinteBandeau(teinte float64, sombre bool) Couleur {
	if sombre {
		return TSL{teinte, .35, .13}.Couleur()
	}
	return TSL{teinte, .85, .97}.Couleur()
}

func min01(v, max float64) float64 {
	if v > max {
		return max
	}
	return v
}

// moyenne rend la couleur moyenne de l'image, utilisée pour composer les voiles posés sur le fond d'écran.
func moyenne(p Palette) Couleur {
	if len(p.Teintes) == 0 {
		return Couleur{128, 128, 128}
	}
	var r, g, b, poids float64
	for _, t := range p.Teintes {
		r += float64(t.Couleur.R) * t.Poids
		g += float64(t.Couleur.G) * t.Poids
		b += float64(t.Couleur.B) * t.Poids
		poids += t.Poids
	}
	if poids == 0 {
		return Couleur{128, 128, 128}
	}
	return Couleur{uint8(r / poids), uint8(g / poids), uint8(b / poids)}
}

// degradeDeRepli compose un dégradé à partir de la palette, pour servir de dernière couche sous l'image.
func degradeDeRepli(p Palette) string {
	if len(p.Teintes) == 0 {
		return "#e9ecef"
	}
	a := p.Teintes[0].Couleur
	b := a
	if len(p.Teintes) > 1 {
		b = p.Teintes[1].Couleur
	}
	return fmt.Sprintf("linear-gradient(160deg, %s 0%%, %s 100%%)", a.Hex(), b.Hex())
}

// verifier recalcule les contrastes des couples qui comptent. Ils sont affichés après la génération et
// recalculés en direct par l'éditeur d'aperçu : c'est le seul moyen de voir qu'une retouche à la main vient
// de rendre un texte illisible.
func (t *Theme) verifier(barre Couleur) []Controle {
	v := func(nom string) Couleur {
		for _, g := range t.Groupes {
			for _, x := range g.Variables {
				if x.Nom == nom && x.Couleur != nil {
					return *x.Couleur
				}
			}
		}
		return Couleur{128, 128, 128}
	}
	couples := []struct {
		quoi, texte, fond string
		vise              float64
	}{
		{"objet d'un message lu", "--row-fg", "--row-bg", ContrasteTexte},
		{"expéditeur", "--sender", "--row-bg", ContrasteTexte},
		{"date d'une ligne", "--date", "--row-bg", ContrasteElement},
		{"objet sur une ligne non lue", "--row-fg", "--unread-bg", ContrasteTexte},
		{"objet sur la ligne ouverte", "--row-fg", "--sel-bg", ContrasteTexte},
		{"en-têtes du message", "--read-fg", "--read-bg", ContrasteTexte},
		{"texte courant", "--fg", "--card-bg", ContrasteTexte},
		{"texte secondaire", "--muted", "--card-bg", ContrasteElement},
		{"liens", "--link", "--card-bg", ContrasteTexte},
		{"entrée de menu", "--menu-fg", "--menu-bg", ContrasteTexte},
		{"entrée de menu survolée", "--menu-hover-fg", "--menu-hover-bg", ContrasteTexte},
		{"bouton « Nouveau message »", "--new-fg", "--new-bg", ContrasteTexte},
		{"bouton clair", "--btn-fg", "--btn-hover", ContrasteTexte},
		{"pastille de non-lus", "--badge-fg", "--badge-bg", ContrasteTexte},
		{"message d'erreur", "--danger-fg", "--danger-bg", ContrasteTexte},
		{"message d'information", "--info-fg", "--info-bg", ContrasteTexte},
		{"avertissement", "--warn-fg", "--warn-bg", ContrasteTexte},
	}
	out := make([]Controle, 0, len(couples)+2)
	for _, c := range couples {
		out = append(out, Controle{c.quoi, c.texte, c.fond, c.vise, Contraste(v(c.texte), v(c.fond))})
	}
	// Ces deux-là sont posés sur le fond d'écran : leur fond n'est pas une variable, mais le voile composé
	// sur la couleur moyenne de l'image. Le contrôle reste indicatif — une image n'a pas une seule couleur.
	out = append(out,
		Controle{"nom de dossier sur le fond d'écran", "--folders-fg", "(fond d'écran)", ContrasteTexte, Contraste(v("--folders-fg"), barre)},
		Controle{"barre du haut sur le fond d'écran", "--bar-fg", "(fond d'écran)", ContrasteTexte, Contraste(v("--bar-fg"), barre)},
	)
	return out
}
