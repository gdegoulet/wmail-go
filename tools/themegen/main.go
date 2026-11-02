// themegen déduit un thème wmail-go d'une image : il en tire une palette, en compose les 65 variables CSS
// en garantissant les contrastes, puis écrit le thème et une page d'aperçu modifiable.
//
//	go run ./tools/themegen -image fond.jpg -nom foret
//
// Le passage obligé est docs/themes.md, qui décrit ce que peint chaque variable. Ce programme ne fait que
// produire un premier jet défendable : la page d'aperçu est là pour le retoucher.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"
)

var reNom = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

func main() {
	var (
		img      = flag.String("image", "", "image de fond : .jpg, .jpeg, .png, .webp, .gif ou .svg")
		nom      = flag.String("nom", "", "nom du thème (minuscules, chiffres, tiret, souligné ; 32 au plus)")
		dest     = flag.String("dest", "static", "où écrire : « static » (embarqué, recompilation) ou un chemin de dossier")
		panneaux = flag.String("panneaux", "auto", "panneaux opaques : « clair », « sombre » ou « auto » (d'après l'image)")
		voile    = flag.String("voile", "auto", "assombrir les bandes où se posent dossiers et barre : « auto », « non » ou une force de 0 à 0,65")
		force    = flag.Bool("force", false, "écraser un thème existant")
	)
	flag.Parse()
	if err := executer(*img, *nom, *dest, *panneaux, *voile, *force); err != nil {
		fmt.Fprintln(os.Stderr, "erreur :", err)
		os.Exit(1)
	}
}

func executer(chemin, nom, dest, panneaux, voile string, force bool) error {
	if chemin == "" || nom == "" {
		flag.Usage()
		return fmt.Errorf("-image et -nom sont obligatoires")
	}
	if !reNom.MatchString(nom) {
		return fmt.Errorf("nom %q : attendu ^[a-z0-9_-]{1,32}$ (c'est le contrôle que fait le serveur)", nom)
	}
	src, err := os.ReadFile(chemin)
	if err != nil {
		return err
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(chemin), "."))
	if ext == "jpeg" {
		ext = "jpg"
	}

	var p Palette
	var matricielle image.Image
	switch ext {
	case "svg":
		p = paletteSVG(src)
		if len(p.Teintes) == 0 {
			return fmt.Errorf("aucune couleur lisible dans ce SVG : les couleurs nommées et les fonctions " +
				"rgb() ne sont pas lues, et un dessin fait de filtres ou d'images incorporées n'en déclare pas")
		}
	case "jpg", "png", "webp", "gif":
		m, _, err := image.Decode(strings.NewReader(string(src)))
		if err != nil {
			return fmt.Errorf("lecture de l'image : %w", err)
		}
		p = quantifier(m, 12)
		matricielle = m
	default:
		return fmt.Errorf("extension %q non gérée : .jpg, .jpeg, .png, .webp, .gif ou .svg", ext)
	}

	sombres := p.Sombre()
	switch panneaux {
	case "clair":
		sombres = false
	case "sombre":
		sombres = true
	case "auto":
		// Fond sombre, panneaux clairs : c'est le parti de DarkShine, et il évite qu'un thème entier
		// disparaisse dans le noir. On ne passe en panneaux sombres que sur une image très claire, où
		// des panneaux clairs ne se détacheraient plus du tout.
		sombres = p.Luminance > 0.72
	default:
		return fmt.Errorf("-panneaux %q : attendu clair, sombre ou auto", panneaux)
	}

	dossier := dest
	if dest == "static" {
		dossier = filepath.Join("internal", "web", "static", "themes")
	}
	dossier = filepath.Join(dossier, nom)
	if _, err := os.Stat(filepath.Join(dossier, "theme.css")); err == nil && !force {
		return fmt.Errorf("%s existe déjà ; -force pour l'écraser", filepath.Join(dossier, "theme.css"))
	}
	if err := os.MkdirAll(dossier, 0o755); err != nil {
		return err
	}

	fondURL := fmt.Sprintf("url(\"/themes/%s/background.%s\") left top / cover no-repeat", nom, ext)
	t := Construire(nom, p, sombres, fondURL+",\n    "+degradeDeRepli(p))

	// Un SVG n'est pas rasterisé ici : son voile de lisibilité se pose en CSS, dans la pile --wall, et non
	// dans le fichier. La luminance de référence est celle de la palette entière, faute de pouvoir mesurer
	// la bande gauche — c'est une approximation, que la page d'aperçu permet de corriger.
	voileSVG := 0.0
	if ext == "svg" && voile != "non" {
		switch voile {
		case "auto":
			voileSVG = math.Max(
				ForceDuVoile(p.Luminance, couleurDe(t, "--folders-fg"), ContrasteTexte),
				ForceDuVoile(p.Luminance, couleurDe(t, "--bar-fg"), ContrasteTexte))
		default:
			f, err := strconv.ParseFloat(strings.Replace(voile, ",", ".", 1), 64)
			if err != nil || f < 0 || f > 0.65 {
				return fmt.Errorf("-voile %q : attendu auto, non, ou une force de 0 à 0,65", voile)
			}
			voileSVG = f
		}
		if couches := VoileCSS(voileSVG); couches != "" {
			t = Construire(nom, p, sombres, couches+fondURL+",\n    "+degradeDeRepli(p))
		}
	}

	// Voile de lisibilité : il s'applique à l'image, pas au CSS, parce que la feuille de style n'a aucun
	// moyen de poser un fond derrière le panneau des dossiers — il est transparent par construction.
	forceVoile, sortieImage := 0.0, src
	if matricielle != nil && voile != "non" {
		texteDossiers := couleurDe(t, "--folders-fg")
		gauche, haute := BandesLuminance(matricielle)
		switch voile {
		case "auto":
			forceVoile = math.Max(
				ForceDuVoile(gauche, texteDossiers, ContrasteTexte),
				ForceDuVoile(haute, couleurDe(t, "--bar-fg"), ContrasteTexte))
		default:
			f, err := strconv.ParseFloat(strings.Replace(voile, ",", ".", 1), 64)
			if err != nil || f < 0 || f > 0.65 {
				return fmt.Errorf("-voile %q : attendu auto, non, ou une force de 0 à 0,65", voile)
			}
			forceVoile = f
		}
		if forceVoile > 0 {
			var buf bytes.Buffer
			voilee := Voiler(matricielle, forceVoile)
			switch ext {
			case "jpg":
				err = jpeg.Encode(&buf, voilee, &jpeg.Options{Quality: 88})
			default: // png, webp et gif sont réencodés en PNG : sans perte, et toujours servi par le serveur
				err = png.Encode(&buf, voilee)
				ext = "png"
			}
			if err != nil {
				return fmt.Errorf("réencodage de l'image voilée : %w", err)
			}
			sortieImage = buf.Bytes()
			// Le nom du fichier a pu changer (webp réencodé en png) : le thème doit citer le bon.
			fondURL = fmt.Sprintf("url(\"/themes/%s/background.%s\") left top / cover no-repeat", nom, ext)
			t = Construire(nom, p, sombres, fondURL+",\n    "+degradeDeRepli(p))
		}
	}

	if err := os.WriteFile(filepath.Join(dossier, "background."+ext), sortieImage, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dossier, "theme.css"), []byte(t.CSS(sombres)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dossier, "apercu.html"), []byte(t.Apercu(ext)), 0o644); err != nil {
		return err
	}

	fmt.Printf("thème « %s » écrit dans %s\n", nom, dossier)
	fmt.Printf("  palette : ")
	for i, x := range p.Teintes {
		if i == 6 {
			break
		}
		fmt.Printf("%s(%.0f%%) ", x.Couleur.Hex(), x.Poids*100)
	}
	fmt.Printf("\n  luminance moyenne : %.2f — panneaux %s\n", p.Luminance, map[bool]string{true: "sombres", false: "clairs"}[sombres])
	if forceVoile > 0 {
		fmt.Printf("  voile de lisibilité : %.0f %% au bord gauche et en haut, sinon les noms de dossiers\n"+
			"    disparaîtraient sur cette image (-voile=non pour garder l'image telle quelle)\n", forceVoile*100)
	} else if matricielle != nil && voile != "non" {
		fmt.Println("  voile de lisibilité : inutile, l'image est assez sombre là où se posent dossiers et barre")
	}
	if ext == "svg" {
		fmt.Println("  (SVG : poids comptés par occurrence d'attribut, pas par surface couverte)")
		if voileSVG > 0 {
			fmt.Printf("  voile de lisibilité : %.0f %% en CSS, au bord gauche et en haut — le fichier SVG n'est\n"+
				"    pas retouché (-voile=non pour l'enlever, -voile=0,30 pour l'ajuster)\n", voileSVG*100)
		} else {
			fmt.Println("  voile de lisibilité : inutile d'après la palette ; vérifiez quand même à l'œil que les")
			fmt.Println("    noms de dossiers restent lisibles en haut à gauche, là où ils se posent à même le fond")
		}
	}

	fmt.Println("\n  contrastes :")
	echecs := 0
	for _, c := range t.Controles {
		marque := "ok  "
		if !c.Passe() {
			marque, echecs = "FAIBLE", echecs+1
		}
		fmt.Printf("    %-6s %5.2f:1 (visé %.1f)  %s\n", marque, c.Obtenu, c.Vise, c.Quoi)
	}
	if echecs > 0 {
		fmt.Printf("\n  %d contraste(s) sous le seuil : à reprendre dans %s\n", echecs, filepath.Join(dossier, "apercu.html"))
	}
	fmt.Printf("\nAperçu et retouche : ouvrez %s dans un navigateur.\n", filepath.Join(dossier, "apercu.html"))
	if dest == "static" {
		fmt.Printf("Pensez à ajouter \"theme.%s\" dans internal/i18n/locales/fr.json et en.json, puis « make test ».\n", nom)
	}
	return nil
}

// CSS rend le fichier theme.css, dans l'ordre de docs/themes.md.
func (t *Theme) CSS(sombres bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/* Thème « %s », déduit d'une image par tools/themegen.\n", t.Nom)
	b.WriteString("   Les 65 variables et ce qu'elles peignent : docs/themes.md. Les chemins d'image doivent rester\n")
	b.WriteString("   absolus : une URL relative dans une variable CSS est résolue d'après la feuille qui l'utilise\n")
	b.WriteString("   (/static/app.css), pas d'après ce fichier. */\n")
	b.WriteString(":root {\n")
	fmt.Fprintf(&b, "  color-scheme: %s;\n", map[bool]string{true: "dark", false: "light"}[sombres])
	for _, g := range t.Groupes {
		fmt.Fprintf(&b, "\n  /* %s */\n", g.Titre)
		for _, v := range g.Variables {
			fmt.Fprintf(&b, "  %s: %s;\n", v.Nom, v.Valeur)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

// couleurDe retrouve une couleur du thème par son nom de variable.
func couleurDe(t *Theme, nom string) Couleur {
	for _, g := range t.Groupes {
		for _, v := range g.Variables {
			if v.Nom == nom && v.Couleur != nil {
				return *v.Couleur
			}
		}
	}
	return Couleur{255, 255, 255}
}
