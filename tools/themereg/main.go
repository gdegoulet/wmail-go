// themereg inscrit dans l'application les thèmes posés dans internal/web/static/themes/ : il écrit leur
// vignette dans settings-auto.css et vérifie qu'ils ont un libellé dans les deux traductions.
//
//	go run ./tools/themereg            # rapport seul
//	go run ./tools/themereg -ecrire    # applique
//
// Un thème produit par tools/themegen est copié à la main dans static/themes/. Trois fichiers doivent alors
// suivre, et les oublier ne casse rien de visible tout de suite : la vignette des réglages, fr.json et
// en.json. Ce programme fait ces trois gestes, pour que l'oubli ne soit plus possible.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	dossierThemes = "internal/web/static/themes"
	fichierCSS    = "internal/web/static/settings-auto.css"
	debut         = "/* >>> vignettes produites par tools/themereg : ne pas modifier à la main. */"
	fin           = "/* <<< fin des vignettes produites par tools/themereg. */"
)

var (
	reRepli    = regexp.MustCompile(`linear-gradient\(160deg, (#[0-9a-fA-F]{6}) 0%, (#[0-9a-fA-F]{6}) 100%\)`)
	reVignette = regexp.MustCompile(`(?m)^\.theme-prev\[data-theme-prev="([a-z0-9_-]+)"\]`)
)

func main() {
	ecrire := flag.Bool("ecrire", false, "appliquer les changements au lieu de les décrire")
	flag.Parse()
	if err := executer(*ecrire); err != nil {
		fmt.Fprintln(os.Stderr, "erreur :", err)
		os.Exit(1)
	}
}

func executer(ecrire bool) error {
	entrees, err := os.ReadDir(dossierThemes)
	if err != nil {
		return err
	}
	var themes, retires, aRetirer []string
	replis := map[string][2]string{}
	for _, e := range entrees {
		if !e.IsDir() {
			continue
		}
		css, err := os.ReadFile(filepath.Join(dossierThemes, e.Name(), "theme.css"))
		if err != nil {
			continue
		}
		themes = append(themes, e.Name())
		// apercu.html n'est pas servi (/themes/<nom>/<fichier> ne sert que theme.css et background.*) :
		// laissé dans static/, il entre dans le binaire sans que rien ne puisse l'ouvrir.
		if apercu := filepath.Join(dossierThemes, e.Name(), "apercu.html"); existe(apercu) {
			if ecrire {
				if err := os.Remove(apercu); err != nil {
					return err
				}
				retires = append(retires, apercu)
			} else {
				aRetirer = append(aRetirer, apercu)
			}
		}
		if m := reRepli.FindStringSubmatch(string(css)); m != nil {
			replis[e.Name()] = [2]string{strings.ToLower(m[1]), strings.ToLower(m[2])}
		}
	}
	sort.Strings(themes)

	css, err := os.ReadFile(fichierCSS)
	if err != nil {
		return err
	}
	avant, bloc, apres := decouper(string(css))

	// Un thème dont la vignette est écrite à la main hors du bloc garde la sienne : elle dit mieux ce que
	// montre l'image qu'un dégradé déduit du repli.
	ecrites := map[string]bool{}
	for _, m := range reVignette.FindAllStringSubmatch(avant+apres, -1) {
		ecrites[m[1]] = true
	}

	var produit []string
	var sansRepli, sansLibelle []string
	for _, t := range themes {
		if ecrites[t] {
			continue
		}
		r, ok := replis[t]
		if !ok {
			sansRepli = append(sansRepli, t)
			continue
		}
		produit = append(produit, vignette(t, r[0], r[1]))
	}
	nouveauBloc := ""
	if len(produit) > 0 {
		nouveauBloc = debut + "\n" + strings.Join(produit, "") + fin + "\n\n"
	}

	// Libellés : sans la clé, les réglages affichent le nom du dossier. La clé s'insère **en place**, juste
	// avant la première clé theme.* existante : retrier le fichier produirait un diff de mille lignes.
	for _, f := range []string{"internal/i18n/locales/fr.json", "internal/i18n/locales/en.json"} {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		texte := string(b)
		for _, t := range themes {
			if strings.Contains(texte, `"theme.`+t+`"`) {
				continue
			}
			sansLibelle = append(sansLibelle, f+" : theme."+t)
			if !ecrire {
				continue
			}
			i := strings.Index(texte, `  "theme.`)
			if i < 0 {
				return fmt.Errorf("%s : aucune clé theme.* où insérer", f)
			}
			texte = texte[:i] + `  "theme.` + t + `": "` + titre(t) + `",` + "\n" + texte[i:]
		}
		if texte != string(b) {
			if err := os.WriteFile(f, []byte(texte), 0o644); err != nil {
				return err
			}
		}
	}

	change := nouveauBloc != bloc
	if change && ecrire {
		if err := os.WriteFile(fichierCSS, []byte(avant+nouveauBloc+apres), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("%d thème(s) : %s\n", len(themes), strings.Join(themes, ", "))
	switch {
	case !change:
		fmt.Println("vignettes : à jour")
	case ecrire:
		fmt.Printf("vignettes : %s mis à jour (%d produite(s))\n", fichierCSS, len(produit))
	default:
		fmt.Printf("vignettes : %s à mettre à jour (%d produite(s)) — relancer avec -ecrire\n", fichierCSS, len(produit))
	}
	for _, t := range sansRepli {
		fmt.Printf("  thème %s : pas de dégradé de repli « linear-gradient(160deg, …) » dans --wall ;\n"+
			"    écrivez sa vignette à la main dans %s\n", t, fichierCSS)
	}
	for _, f := range retires {
		fmt.Printf("  retiré : %s (jamais servi, il n'alourdissait que le binaire)\n", f)
	}
	for _, f := range aRetirer {
		fmt.Printf("  à retirer : %s (jamais servi, il n'alourdit que le binaire)\n", f)
	}
	for _, m := range sansLibelle {
		if ecrire {
			fmt.Printf("  libellé écrit par défaut : %s — relisez-le, c'est ce que verra l'utilisateur\n", m)
		} else {
			fmt.Printf("  libellé manquant : %s — sans lui, les réglages affichent le nom du dossier\n", m)
		}
	}
	if len(sansLibelle) > 0 && !ecrire {
		return fmt.Errorf("%d libellé(s) manquant(s)", len(sansLibelle))
	}
	return nil
}

func existe(chemin string) bool {
	_, err := os.Stat(chemin)
	return err == nil
}

// titre rend un libellé de départ à partir du nom de dossier : « mer-de-nuages » devient « Mer de nuages ».
// C'est un pis-aller, à relire : le libellé est ce que lit l'utilisateur dans les réglages.
func titre(nom string) string {
	mots := strings.FieldsFunc(nom, func(r rune) bool { return r == '-' || r == '_' })
	s := strings.Join(mots, " ")
	if s == "" {
		return nom
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func decouper(css string) (avant, bloc, apres string) {
	i := strings.Index(css, debut)
	if i < 0 {
		// Le bloc se pose juste avant la première vignette écrite à la main, ou à la fin.
		j := strings.Index(css, ".theme-prev[data-theme-prev=")
		if j < 0 {
			return css, "", ""
		}
		return css[:j], "", css[j:]
	}
	j := strings.Index(css, fin)
	if j < 0 {
		return css, "", ""
	}
	j += len(fin)
	for j < len(css) && css[j] == '\n' {
		j++
	}
	return css[:i], css[i:j], css[j:]
}

// vignette rend un carré de 64 px qui rappelle le thème : la bande sombre de la barre du haut, un halo clair
// pris de la couleur de fin, puis le dégradé de repli du thème lui-même. Aucune image n'est chargée.
func vignette(nom, a, b string) string {
	return fmt.Sprintf(`.theme-prev[data-theme-prev=%q] {
  background:
    linear-gradient(180deg, rgba(0, 0, 0, .45) 0 14px, rgba(0, 0, 0, 0) 14px),
    radial-gradient(ellipse 70%% 60%% at 35%% 40%%, %s 0%%, rgba(0, 0, 0, 0) 100%%),
    linear-gradient(160deg, %s 0%%, %s 55%%, %s 100%%);
}

`, nom, eclaircir(plusClair(a, b))+"bf", a, melanger(a, b), b)
}

func composantes(h string) [3]int {
	var c [3]int
	for i := range c {
		v, _ := strconv.ParseInt(h[1+2*i:3+2*i], 16, 0)
		c[i] = int(v)
	}
	return c
}

// plusClair rend celle des deux extrémités qui éclaire le halo : le repli va parfois du clair au sombre,
// parfois l'inverse, et un halo pris systématiquement à la fin serait tantôt lumineux, tantôt invisible.
func plusClair(a, b string) string {
	somme := func(h string) int { c := composantes(h); return c[0] + c[1] + c[2] }
	if somme(a) > somme(b) {
		return a
	}
	return b
}

func melanger(a, b string) string {
	x, y := composantes(a), composantes(b)
	return fmt.Sprintf("#%02x%02x%02x", (x[0]+y[0])/2, (x[1]+y[1])/2, (x[2]+y[2])/2)
}

func eclaircir(h string) string {
	c := composantes(h)
	for i, v := range c {
		if v = v * 145 / 100; v > 255 {
			v = 255
		}
		c[i] = v
	}
	return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
}
