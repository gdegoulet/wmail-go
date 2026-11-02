package web

import (
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gdegoulet/wmail-go/internal/config"
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

// Un thème copié dans static/themes/ n'est pas inscrit dans l'application pour autant : sans libellé, les
// réglages affichent le nom du dossier ; sans vignette, sa case est le carré gris neutre. Les deux gestes
// s'oublient, et rien ne les signale à l'usage. tools/themereg les fait ; ce test les exige.
func TestChaqueThemeAUnLibelleEtUneVignette(t *testing.T) {
	css, err := staticFS.ReadFile("static/settings-auto.css")
	if err != nil {
		t.Fatal(err)
	}
	var trads [][]byte
	for _, f := range []string{"../i18n/locales/fr.json", "../i18n/locales/en.json"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		trads = append(trads, b)
	}
	for _, theme := range themesEmbarques(t) {
		if !strings.Contains(string(css), `.theme-prev[data-theme-prev="`+theme+`"]`) {
			t.Errorf("thème %s : pas de vignette dans settings-auto.css (go run ./tools/themereg -ecrire)", theme)
		}
		for i, b := range trads {
			if !strings.Contains(string(b), `"theme.`+theme+`"`) {
				t.Errorf("thème %s : clé theme.%s absente de %s", theme, theme,
					[]string{"fr.json", "en.json"}[i])
			}
		}
	}
}

// default_theme = « random » fait tirer un thème au sort à chaque page, tant que l'utilisateur n'en a pas
// choisi un : c'est ce que voit le visiteur sur la page de connexion.
func TestThemeAleatoireSurLaPageDeConnexion(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.DefaultTheme = ThemeAleatoire })
	connus := map[string]bool{}
	for _, n := range e.srv.themeNames() {
		connus[n] = true
	}
	vus := map[string]bool{}
	for i := 0; i < 60; i++ {
		_, body := e.do(e.client(), "GET", "/login", nil, nil)
		m := regexp.MustCompile(`<link rel="stylesheet" href="/themes/([a-z0-9_-]+)/theme\.css">`).FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("aucune feuille de thème dans la page de connexion")
		}
		if !connus[m[1]] {
			t.Fatalf("thème tiré %q : inconnu", m[1])
		}
		// La feuille et l'attribut doivent désigner le même thème : deux appels au tirage dans la même
		// page donneraient une page peinte par l'un et annoncée comme l'autre.
		if !strings.Contains(body, `data-theme="`+m[1]+`"`) {
			t.Fatalf("thème %q dans la feuille, autre valeur dans data-theme", m[1])
		}
		vus[m[1]] = true
	}
	// 60 tirages parmi au moins deux thèmes : tomber 60 fois sur le même vaut une chance sur 2^59.
	if len(vus) < 2 && len(connus) > 1 {
		t.Errorf("60 pages de connexion, un seul thème tiré : %v", vus)
	}
}

// Un thème nommé « random » rendrait le réglage ambigu : le serveur doit refuser de démarrer.
func TestThemeNommeRandomRefuseAuDemarrage(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.DefaultTheme = ThemeAleatoire })
	dir := filepath.Join(e.cfg.DataDir, "themes", ThemeAleatoire)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.css"), []byte(":root{--bg:#000}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(e.cfg, e.srv.store, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Error("un thème nommé « random » doit être refusé au démarrage")
	}
}
