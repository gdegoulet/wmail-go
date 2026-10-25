package web

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdegoulet/wmail-go/internal/userdata"
)

// TestActivityJournal parcourt le cycle complet : création, échecs de connexion compte fermé, connexion,
// versement dans l'historique et affichage.
func TestActivityJournal(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	e.register(c, "alice")

	// Déconnexion, puis deux échecs : le compte est fermé, mais la file d'attente les accepte.
	_, page := e.do(c, "GET", "/settings/activity", nil, nil)
	resp, _ := e.do(c, "POST", "/logout", url.Values{"csrf": {csrf(t, page)}}, nil)
	e.expect(resp, 303, "/login")
	for range 2 {
		resp = e.login(c, "alice", "le mauvais mot de passe")
		e.expect(resp, 401, "")
	}

	resp = e.login(c, "alice", pw)
	e.expect(resp, 303, "/")

	_, page = e.do(c, "GET", "/settings/activity", nil, nil)
	for _, attendu := range []string{"Connexion", "Échec de connexion", "Déconnexion"} {
		if !strings.Contains(page, attendu) {
			t.Errorf("journal sans %q :\n%s", attendu, page)
		}
	}
	if n := strings.Count(page, "Échec de connexion"); n != 2 {
		t.Errorf("%d échecs affichés, attendu 2", n)
	}
}

// Le rappel de la connexion précédente apparaît une fois, avec le nombre d'échecs depuis.
func TestActivityNotice(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	e.register(c, "alice")
	_, page := e.do(c, "GET", "/settings/activity", nil, nil)
	e.do(c, "POST", "/logout", url.Values{"csrf": {csrf(t, page)}}, nil)

	e.expect(e.login(c, "alice", "faux"), 401, "")
	e.expect(e.login(c, "alice", pw), 303, "/")

	_, page = e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(page, "loginnotice") {
		t.Fatalf("rappel de connexion absent :\n%s", page)
	}
	if !strings.Contains(page, "Dernière connexion le") {
		t.Error("le rappel n'annonce pas la connexion précédente")
	}
	if !strings.Contains(page, "1 tentative(s) de connexion ont échoué depuis.") {
		t.Error("le rappel ne compte pas l'échec")
	}
	// Une seule fois par session.
	_, page = e.do(c, "GET", "/settings", nil, nil)
	if strings.Contains(page, "loginnotice") {
		t.Error("le rappel se répète à chaque page")
	}
}

// Le journal est chiffré : ni l'IP ni la nature des événements ne doivent se lire sur le disque.
func TestActivityChiffreSurDisque(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	e.register(c, "alice")
	e.do(c, "GET", "/settings/activity", nil, nil)

	grep := func(motif string) bool {
		trouve := false
		parcourirFichiers(t, e.cfg.DataDir, func(contenu []byte) {
			if strings.Contains(string(contenu), motif) {
				trouve = true
			}
		})
		return trouve
	}
	if grep("login_ok") {
		t.Error("la nature des événements se lit en clair dans data_dir")
	}
	if grep("127.0.0.1") {
		t.Error("l'adresse IP se lit en clair dans data_dir")
	}
}

// Le journal ne doit pas pouvoir être effacé : sinon un intrus qui détient le mot de passe
// supprime ses propres traces, et le journal ne sert plus à rien.
func TestActivityNonEffacable(t *testing.T) {
	e := newEnv(t, nil)
	c := e.client()
	e.register(c, "alice")
	_, page := e.do(c, "GET", "/settings/activity", nil, nil)
	if !strings.Contains(page, "Connexion") {
		t.Fatalf("journal vide :\n%s", page)
	}
	if strings.Contains(page, "activity/clear") {
		t.Error("la page propose encore d'effacer le journal")
	}

	// La route n'existe plus : aucune requête ne doit vider l'historique.
	resp, _ := e.do(c, "POST", "/settings/activity/clear", url.Values{"csrf": {csrf(t, page)}}, nil)
	if resp.StatusCode < 400 {
		t.Errorf("POST /settings/activity/clear répond %d, attendu une erreur", resp.StatusCode)
	}
	_, page = e.do(c, "GET", "/settings/activity", nil, nil)
	if !strings.Contains(page, "Connexion") {
		t.Errorf("journal vidé malgré tout :\n%s", page)
	}
}

func TestPruneActivity(t *testing.T) {
	maintenant := time.Now()
	hist := []userdata.Event{
		{At: maintenant.AddDate(0, 0, -(activityDays + 1)).Unix(), Kind: userdata.EvLoginOK},
		{At: maintenant.AddDate(0, 0, -1).Unix(), Kind: userdata.EvLoginFail},
		{At: maintenant.AddDate(0, 0, -2).Unix(), Kind: userdata.EvLogout},
	}
	out := pruneActivity(hist)
	if len(out) != 2 {
		t.Fatalf("%d entrées gardées, attendu 2", len(out))
	}
	if out[0].Kind != userdata.EvLogout || out[1].Kind != userdata.EvLoginFail {
		t.Errorf("entrées mal triées : %v", out)
	}

	// Plafond du nombre d'entrées.
	gros := make([]userdata.Event, activityMax+10)
	for i := range gros {
		gros[i] = userdata.Event{At: maintenant.Add(-time.Duration(len(gros)-i) * time.Second).Unix(), Kind: userdata.EvLoginOK}
	}
	if out := pruneActivity(gros); len(out) != activityMax {
		t.Errorf("%d entrées après plafond, attendu %d", len(out), activityMax)
	}
}

func TestLastLoginNotice(t *testing.T) {
	hist := []userdata.Event{
		{At: 1000, Kind: userdata.EvLoginOK},
		{At: 1100, Kind: userdata.EvLogout},
		{At: 1200, Kind: userdata.EvLoginFail},
		{At: 1300, Kind: userdata.EvTwoFactorKO},
		{At: 1400, Kind: userdata.EvLoginOK}, // la connexion courante
	}
	prev, failed := lastLoginNotice(hist)
	if prev != 1000 {
		t.Errorf("connexion précédente %d, attendu 1000", prev)
	}
	if failed != 2 {
		t.Errorf("%d échecs comptés, attendu 2", failed)
	}
	// Première connexion : aucune précédente, aucun échec.
	if prev, failed := lastLoginNotice(hist[4:]); prev != 0 || failed != 0 {
		t.Errorf("première connexion : %d, %d ; attendu 0, 0", prev, failed)
	}
}

func TestTronque(t *testing.T) {
	if got := tronque("abc", 5); got != "abc" {
		t.Errorf("%q, attendu abc", got)
	}
	if got := tronque("abcdef", 3); got != "abc…" {
		t.Errorf("%q, attendu abc…", got)
	}
	// Caractères multi-octets : la coupe se fait sur les runes, pas sur les octets.
	if got := tronque("ééééé", 2); got != "éé…" {
		t.Errorf("%q, attendu éé…", got)
	}
}

// parcourirFichiers passe chaque fichier régulier de dir à fn : sert à vérifier qu'aucune donnée ne traîne en clair.
func parcourirFichiers(t *testing.T, dir string, fn func([]byte)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fn(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
