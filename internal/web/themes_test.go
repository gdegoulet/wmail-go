package web

import (
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// Variables CSS qui ne relèvent pas du thème et n'ont donc pas à y figurer : des dimensions, réglées par
// utilisateur (/user.css) ou par le glissement des séparateurs (list.js), et un alias interne à une règle.
var varsHorsTheme = []string{
	"--bar-h", "--folders-w", "--list-h", "--list-w", "--list-w-default", "--wm-focus",
}

var (
	reVarDecl = regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`)
	reVarUse  = regexp.MustCompile(`var\((--[a-z0-9-]+)`)
)

func varsDeclarees(t *testing.T, theme string) []string {
	t.Helper()
	css, err := staticFS.ReadFile("static/themes/" + theme + "/theme.css")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range reVarDecl.FindAllStringSubmatch(string(css), -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return slices.Compact(out)
}

func themesEmbarques(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(staticFS, "static/themes")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// Tous les thèmes livrés doivent déclarer exactement le même jeu de variables. Un thème à qui il en manque
// une retombe silencieusement sur la valeur de repli de la feuille de style, c'est-à-dire sur la couleur
// d'un autre thème : c'est ainsi que « dark » a longtemps affiché une liste de messages sur fond blanc.
func TestTousLesThemesDeclarentLeMemeJeu(t *testing.T) {
	themes := themesEmbarques(t)
	if len(themes) < 2 {
		t.Fatalf("un seul thème embarqué : %v", themes)
	}
	ref := varsDeclarees(t, "rainloop")
	for _, theme := range themes {
		got := varsDeclarees(t, theme)
		if slices.Equal(got, ref) {
			continue
		}
		var manquantes, superflues []string
		for _, v := range ref {
			if !slices.Contains(got, v) {
				manquantes = append(manquantes, v)
			}
		}
		for _, v := range got {
			if !slices.Contains(ref, v) {
				superflues = append(superflues, v)
			}
		}
		t.Errorf("thème %s : manque %v, en trop %v", theme, manquantes, superflues)
	}
}

// Aucune variable déclarée par les thèmes ne doit être inutilisée, et aucune variable de couleur lue par la
// feuille de style ne doit manquer aux thèmes. Les deux sens comptent :
//
//   - déclarée sans être lue : la régler dans un thème ne produit aucun effet, et l'éditeur de thème
//     proposerait un réglage qui ne fait rien ;
//   - lue sans être déclarée : la couleur est réglable sans que personne ne le sache, et tous les thèmes
//     héritent du même repli, donc d'une couleur pensée pour un seul d'entre eux.
func TestVariablesDeThemeToutesUtiliseesEtToutesDeclarees(t *testing.T) {
	declarees := varsDeclarees(t, "rainloop")

	var utilisees []string
	entries, err := fs.ReadDir(staticFS, "static")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".css") {
			continue
		}
		css, err := staticFS.ReadFile("static/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range reVarUse.FindAllStringSubmatch(string(css), -1) {
			utilisees = append(utilisees, m[1])
		}
	}
	sort.Strings(utilisees)
	utilisees = slices.Compact(utilisees)

	for _, v := range declarees {
		if !slices.Contains(utilisees, v) {
			t.Errorf("%s est déclarée par les thèmes mais lue par aucune feuille de style", v)
		}
	}
	for _, v := range utilisees {
		if !slices.Contains(declarees, v) && !slices.Contains(varsHorsTheme, v) {
			t.Errorf("%s est lue par une feuille de style mais déclarée par aucun thème "+
				"(si c'est une dimension et non une couleur, l'ajouter à varsHorsTheme)", v)
		}
	}
}
