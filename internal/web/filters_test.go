package web

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gdegoulet/wmail-go/internal/config"
	"github.com/gdegoulet/wmail-go/internal/mailconn/mailtest"
	"github.com/gdegoulet/wmail-go/internal/sieve"
	"github.com/gdegoulet/wmail-go/internal/sieve/sievetest"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

// filtersEnv ouvre une session dont la boîte a la gestion des filtres reliée à un serveur ManageSieve de test.
func filtersEnv(t *testing.T, mode string, extra url.Values) (*env, *httpClient, string, *sievetest.Server) {
	t.Helper()
	srv := sievetest.Start(t, mode)
	form := url.Values{"sieve_enabled": {"on"}, "sieve_host": {"127.0.0.1"}, "sieve_port": {strconv.Itoa(srv.Port)}, "sieve_security": {"none"},
		"sieve_login": {sievetest.User}, "sieve_password": {sievetest.Password}, "sieve_verify": {"on"}}
	if mode == "starttls" {
		form.Set("sieve_security", "starttls")
		form.Del("sieve_verify") // certificat autosigné du serveur de test
	}
	for k, v := range extra {
		form[k] = v
	}
	e, c, tok, _ := composeEnvWith(t, func(cfg *config.Config) {}, form)
	return e, c, tok, srv
}

// expectRedirectPrefix vérifie une redirection vers la page (les réglages de filtres ajoutent « ?box=… »).
func expectRedirectPrefix(t *testing.T, resp *http.Response, prefix string) {
	t.Helper()
	if resp.StatusCode != 303 || !strings.HasPrefix(resp.Header.Get("Location"), prefix) {
		t.Fatalf("redirection %d vers %q, attendu %q…", resp.StatusCode, resp.Header.Get("Location"), prefix)
	}
}

func filterForm(tok string, extra url.Values) url.Values {
	f := url.Values{"csrf": {tok}, "name": {"Factures"}, "enabled": {"on"}, "match": {"Any"}, "action": {"MoveTo"}, "move_to": {"Projets"},
		"cond_field": {"From", "Subject"}, "cond_type": {"Contains", "NotContains"}, "cond_value": {"compta@x.fr", "Re:"}, "cond_second": {"", ""}}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

func serverFilters(t *testing.T, srv *sievetest.Server) []sieve.Filter {
	t.Helper()
	scripts, _ := srv.Scripts()
	body, ok := scripts[sieve.ScriptName]
	if !ok {
		return nil
	}
	fs, ok := sieve.ParseScript(body)
	if !ok {
		t.Fatalf("script non reconnu :\n%s", body)
	}
	return fs
}

func TestFiltersWorkWithoutAnySieveSetting(t *testing.T) {
	// aucun réglage ManageSieve : la page tente le serveur de l'IMAP (port 4190) ; ici rien n'écoute, le message est clair
	e, c, _, _ := composeEnv(t, nil)
	resp, page := e.do(c, "GET", "/settings/filters", nil, nil)
	if resp.StatusCode != 502 || !strings.Contains(page, "ne répond pas") || !strings.Contains(page, "/edit#sieve") {
		t.Errorf("serveur de filtres absent : %d\n%s", resp.StatusCode, page)
	}
	if strings.Contains(page, "Configurer ManageSieve pour activer") {
		t.Error("la page ne doit pas exiger une activation préalable")
	}
}

func TestFiltersCanBeDisabledPerMailbox(t *testing.T) {
	e, c, _, _ := composeEnvWith(t, nil, url.Values{"sieve_disabled": {"on"}})
	resp, page := e.do(c, "GET", "/settings/filters", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "désactivés pour cette boîte") || strings.Contains(page, "/settings/filters/edit") {
		t.Errorf("filtres désactivés :\n%s", page)
	}
	resp, _ = e.do(c, "GET", "/settings/filters/edit", nil, nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
}

func TestSieveEndpointDefaultsFromIMAP(t *testing.T) {
	var a userdata.MailAccount
	a.IMAP = userdata.Server{Host: "imap.example.org", Login: "moi", Password: "pw", VerifyCert: true, LegacyTLS: true}
	e := sieveEndpoint(a)
	if e.Host != "imap.example.org" || e.Port != 4190 || e.Security != "starttls" || e.Login != "moi" || e.Password != "pw" || !e.VerifyCert || !e.LegacyTLS {
		t.Errorf("valeurs par défaut : %+v", e)
	}
	a.Sieve = userdata.SieveConfig{Host: "sieve.example.org", Port: 2000, Security: "tls", Login: "autre", Password: "x", VerifyCert: false}
	if e := sieveEndpoint(a); e.Host != "sieve.example.org" || e.Port != 2000 || e.Security != "tls" || e.Login != "autre" || e.VerifyCert {
		t.Errorf("réglages propres : %+v", e)
	}
}

func TestFiltersLifecycle(t *testing.T) {
	e, c, tok, srv := filtersEnv(t, "none", nil)
	resp, page := e.do(c, "GET", "/settings/filters", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "Aucun filtre") {
		t.Errorf("liste vide :\n%s", page)
	}
	// formulaire : dossiers réels, extensions du serveur
	_, form := e.do(c, "GET", "/settings/filters/edit", nil, nil)
	if !strings.Contains(form, `<option value="Projets/2026"`) || !strings.Contains(form, `name="cond_value"`) {
		t.Errorf("formulaire :\n%s", form)
	}

	resp, _ = e.do(c, "POST", "/settings/filters/save", filterForm(tok, nil), nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	fs := serverFilters(t, srv)
	if len(fs) != 1 || fs[0].Name != "Factures" || fs[0].ActionValue != "Projets" || len(fs[0].Conditions) != 2 || fs[0].ID == "" {
		t.Fatalf("filtres enregistrés : %+v", fs)
	}
	scripts, active := srv.Scripts()
	if active != sieve.ScriptName || !strings.Contains(scripts[sieve.ScriptName], `fileinto "Projets";`) {
		t.Errorf("script actif : %q\n%s", active, scripts[sieve.ScriptName])
	}
	_, page = e.do(c, "GET", "/settings/filters", nil, nil)
	if !strings.Contains(page, "Factures") || !strings.Contains(page, "Filtres enregistrés.") || !strings.Contains(page, "2 conditions") {
		t.Errorf("liste :\n%s", page)
	}

	// second filtre, puis déplacement vers le haut
	resp, _ = e.do(c, "POST", "/settings/filters/save", filterForm(tok, url.Values{"name": {"Absence"}, "action": {"Discard"}, "cond_value": {"spam@x.fr", ""}}), nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	fs = serverFilters(t, srv)
	if len(fs) != 2 || fs[1].Name != "Absence" || len(fs[1].Conditions) != 1 {
		t.Fatalf("deux filtres attendus : %+v", fs)
	}
	resp, _ = e.do(c, "POST", "/settings/filters/action", url.Values{"csrf": {tok}, "action": {"up"}, "id": {fs[1].ID}}, nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	if fs2 := serverFilters(t, srv); fs2[0].Name != "Absence" || fs2[1].Name != "Factures" {
		t.Errorf("ordre : %v %v", fs2[0].Name, fs2[1].Name)
	}
	// désactiver : le filtre reste dans le script, enveloppé en commentaire
	resp, _ = e.do(c, "POST", "/settings/filters/action", url.Values{"csrf": {tok}, "action": {"toggle"}, "id": {fs[0].ID}}, nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	scripts, _ = srv.Scripts()
	if got := serverFilters(t, srv); !strings.Contains(scripts[sieve.ScriptName], "@Filter is disabled") || got[1].Enabled || !got[0].Enabled {
		t.Errorf("désactivation :\n%s", scripts[sieve.ScriptName])
	}
	// modification
	_, form = e.do(c, "GET", "/settings/filters/edit?id="+fs[0].ID, nil, nil)
	if !strings.Contains(form, `value="Factures"`) || !strings.Contains(form, `value="compta@x.fr"`) {
		t.Errorf("formulaire d'édition :\n%s", form)
	}
	resp, _ = e.do(c, "POST", "/settings/filters/save", filterForm(tok, url.Values{"id": {fs[0].ID}, "name": {"Renommé"}, "action": {"None"}, "enabled": {"on"}}), nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	if got := serverFilters(t, srv); len(got) != 2 || got[1].Name != "Renommé" || got[1].ID != fs[0].ID || got[1].ActionType != "None" {
		t.Errorf("modification : %+v", got)
	}
	// suppression de tout : le script disparaît du serveur
	for _, f := range serverFilters(t, srv) {
		resp, _ = e.do(c, "POST", "/settings/filters/action", url.Values{"csrf": {tok}, "action": {"delete"}, "id": {f.ID}}, nil)
		expectRedirectPrefix(t, resp, "/settings/filters")
	}
	scripts, active = srv.Scripts()
	if _, ok := scripts[sieve.ScriptName]; ok || active != "" {
		t.Errorf("le script devrait être supprimé : %v actif=%q", scripts, active)
	}
}

func TestFilterValidationKeepsInput(t *testing.T) {
	e, c, tok, srv := filtersEnv(t, "none", nil)
	for name, v := range map[string]struct {
		extra url.Values
		want  string
	}{
		"nom vide":     {url.Values{"name": {" "}}, "Donnez un nom"},
		"taille":       {url.Values{"cond_field": {"Size"}, "cond_type": {"Over"}, "cond_value": {"énorme", ""}}, "Taille invalide"},
		"over sur De":  {url.Values{"cond_type": {"Over", "Contains"}}, "ne convient pas"},
		"sans dossier": {url.Values{"move_to": {""}}, "Choisissez un dossier"},
		"adresse":      {url.Values{"action": {"Forward"}, "forward_to": {"pas une adresse"}}, "Adresse invalide"},
		"absence vide": {url.Values{"action": {"Vacation"}, "vac_msg": {""}}, "message est obligatoire"},
		"en-tête":      {url.Values{"cond_field": {"Header"}, "cond_second": {"a b", ""}}, "en-tête invalide"},
	} {
		resp, page := e.do(c, "POST", "/settings/filters/save", filterForm(tok, v.extra), nil)
		if resp.StatusCode != 400 || !strings.Contains(page, v.want) {
			t.Errorf("%s : statut %d, %q attendu\n%s", name, resp.StatusCode, v.want, page)
			continue
		}
		if name != "nom vide" && !strings.Contains(page, `value="Factures"`) {
			t.Errorf("%s : saisie non conservée", name)
		}
	}
	if len(serverFilters(t, srv)) != 0 {
		t.Error("un filtre invalide a été enregistré")
	}
}

func TestFilterScriptInjectionBlockedEndToEnd(t *testing.T) {
	e, c, tok, srv := filtersEnv(t, "none", nil)
	f := filterForm(tok, url.Values{"enabled": {}, "cond_value": {"*/ discard; /*", `"; discard; "`}, "move_to": {"Projets"}})
	resp, _ := e.do(c, "POST", "/settings/filters/save", f, nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	scripts, _ := srv.Scripts()
	body := scripts[sieve.ScriptName]
	block := strings.SplitN(strings.SplitN(body, "@Filter is disabled", 2)[1], "/* END:FILTER */", 2)[0]
	if strings.Count(block, "*/") != 1 {
		t.Errorf("un filtre désactivé ne doit contenir qu'une fin de commentaire :\n%s", block)
	}
}

func TestFiltersReadRainLoopScript(t *testing.T) {
	e, c, _, srv := filtersEnv(t, "none", nil)
	hdr := `{"ID":"abc","Enabled":true,"Name":"Depuis RainLoop","Conditions":[{"Field":"From","Type":"Contains","Value":"x@y.fr","ValueSecond":""}],"ConditionsType":"Any","ActionType":"MoveTo","ActionValue":"Projets","Keep":true,"Stop":true,"MarkAsRead":false}`
	script := "# RAINLOOP:SIEVE\r\n/*\r\nBEGIN:FILTER:abc\r\nBEGIN:HEADER\r\n" + b64(hdr) + "\r\nEND:HEADER\r\n*/\r\nif header :contains [\"From\"] \"x@y.fr\" { fileinto \"Projets\"; stop; }\r\n/* END:FILTER */\r\n"
	srv.Put(sieve.ScriptName, script, true)
	_, page := e.do(c, "GET", "/settings/filters", nil, nil)
	if !strings.Contains(page, "Depuis RainLoop") || !strings.Contains(page, "déplacer vers « Projets »") {
		t.Errorf("filtre de RainLoop absent :\n%s", page)
	}
}

func b64(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(base64.StdEncoding.EncodeToString([]byte(s)), "")
}

func TestForeignScriptIsNotOverwrittenWithoutConfirmation(t *testing.T) {
	e, c, tok, srv := filtersEnv(t, "none", nil)
	srv.Put(sieve.ScriptName, `require ["fileinto"]; if true { fileinto "X"; }`, true)
	_, page := e.do(c, "GET", "/settings/filters", nil, nil)
	if !strings.Contains(page, "client de filtres") {
		t.Errorf("avertissement absent :\n%s", page[min(len(page), 3000):])
	}
	resp, _ := e.do(c, "POST", "/settings/filters/save", filterForm(tok, nil), nil)
	if resp.StatusCode != 409 {
		t.Errorf("sans confirmation : %d", resp.StatusCode)
	}
	if scripts, _ := srv.Scripts(); !strings.Contains(scripts[sieve.ScriptName], `fileinto "X"`) {
		t.Error("le script d'origine a été écrasé")
	}
	resp, _ = e.do(c, "POST", "/settings/filters/save", filterForm(tok, url.Values{"confirm_overwrite": {"on"}}), nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	if len(serverFilters(t, srv)) != 1 {
		t.Error("le filtre n'a pas été enregistré après confirmation")
	}
}

func TestFiltersRawScript(t *testing.T) {
	e, c, tok, srv := filtersEnv(t, "none", nil)
	_, page := e.do(c, "GET", "/settings/filters/raw", nil, nil)
	if !strings.Contains(page, `name="raw"`) {
		t.Fatalf("page du script brut :\n%s", page)
	}
	good := "require [\"fileinto\"];\nif header :contains \"Subject\" \"x\" { fileinto \"Projets\"; }\n"
	resp, _ := e.do(c, "POST", "/settings/filters/raw", url.Values{"csrf": {tok}, "raw": {good}, "use_raw": {"on"}}, nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	scripts, active := srv.Scripts()
	if active != sieve.ScriptRawName || !strings.Contains(scripts[sieve.ScriptRawName], `fileinto "Projets"`) || !strings.Contains(scripts[sieve.ScriptRawName], "\r\n") {
		t.Errorf("script brut : actif=%q\n%q", active, scripts[sieve.ScriptRawName])
	}
	_, list := e.do(c, "GET", "/settings/filters", nil, nil)
	if !strings.Contains(list, "script brut est actif") {
		t.Error("l'avertissement de script brut actif manque")
	}
	// un script que le serveur refuse : message du serveur, script précédent conservé
	resp, page = e.do(c, "POST", "/settings/filters/raw", url.Values{"csrf": {tok}, "raw": {"SYNTAXERROR"}, "use_raw": {"on"}}, nil)
	if resp.StatusCode != 400 || !strings.Contains(page, "syntax error") || !strings.Contains(page, "SYNTAXERROR") {
		t.Errorf("script refusé : %d\n%s", resp.StatusCode, page)
	}
	if scripts, _ := srv.Scripts(); !strings.Contains(scripts[sieve.ScriptRawName], "fileinto") {
		t.Error("le script valide a été remplacé par un script refusé")
	}
	// retour aux filtres de la liste
	e.do(c, "POST", "/settings/filters/save", filterForm(tok, nil), nil)
	resp, _ = e.do(c, "POST", "/settings/filters/raw", url.Values{"csrf": {tok}, "raw": {good}}, nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	if _, active := srv.Scripts(); active != sieve.ScriptName {
		t.Errorf("après « ne pas utiliser le script brut » : actif = %q", active)
	}
}

func TestFiltersOverStartTLSAndErrors(t *testing.T) {
	e, c, tok, srv := filtersEnv(t, "starttls", nil)
	resp, _ := e.do(c, "POST", "/settings/filters/save", filterForm(tok, nil), nil)
	expectRedirectPrefix(t, resp, "/settings/filters")
	if len(serverFilters(t, srv)) != 1 {
		t.Error("enregistrement via STARTTLS")
	}
	cmds := strings.Join(srv.Commands(), " ")
	if !strings.Contains(cmds, "STARTTLS") || strings.Index(cmds, "STARTTLS") > strings.Index(cmds, "AUTHENTICATE") {
		t.Errorf("STARTTLS doit précéder l'authentification : %s", cmds)
	}

	// mauvais mot de passe
	e2, c2, _, _ := filtersEnv(t, "none", url.Values{"sieve_password": {"faux"}})
	resp, page := e2.do(c2, "GET", "/settings/filters", nil, nil)
	if resp.StatusCode != 502 || !strings.Contains(page, "identifiant") && !strings.Contains(page, "mot de passe") {
		t.Errorf("mauvais mot de passe : %d\n%s", resp.StatusCode, page)
	}
	// port fermé
	e3, c3, _, _ := filtersEnv(t, "none", url.Values{"sieve_port": {"1"}})
	resp, page = e3.do(c3, "GET", "/settings/filters", nil, nil)
	if resp.StatusCode != 502 || !strings.Contains(page, "ne répond pas") {
		t.Errorf("port fermé : %d\n%s", resp.StatusCode, page)
	}
}

func TestFiltersAuthAndCSRF(t *testing.T) {
	e, c, _, srv := filtersEnv(t, "none", nil)
	anon := e.client()
	for _, p := range []string{"/settings/filters", "/settings/filters/edit", "/settings/filters/raw"} {
		resp, _ := e.do(anon, "GET", p, nil, nil)
		e.expect(resp, 303, "/login")
	}
	resp, _ := e.do(c, "POST", "/settings/filters/save", filterForm("mauvais", nil), nil)
	if resp.StatusCode != 403 {
		t.Errorf("sans CSRF : %d", resp.StatusCode)
	}
	if len(srv.Commands()) > 0 && len(serverFilters(t, srv)) != 0 {
		t.Error("filtre enregistré sans jeton valide")
	}
	// action inconnue
	_, page := e.do(c, "GET", "/settings/filters", nil, nil)
	tok := csrf(t, page)
	resp, _ = e.do(c, "POST", "/settings/filters/action", url.Values{"csrf": {tok}, "action": {"x"}, "id": {"y"}}, nil)
	if resp.StatusCode != 400 {
		t.Errorf("action inconnue : %d", resp.StatusCode)
	}
}

func TestFilterSimilarLinkAndPrefill(t *testing.T) {
	e, c, _, _ := filtersEnv(t, "none", nil)
	uid := uidOfSubject(t, e, c, "INBOX", "Facture octobre")
	_, msg := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uid, nil, nil)
	if !strings.Contains(msg, "/settings/filters/edit?from=alice%40x.fr") {
		t.Errorf("lien « filtrer les messages similaires » absent")
	}
	_, form := e.do(c, "GET", "/settings/filters/edit?from=alice%40x.fr", nil, nil)
	if !strings.Contains(form, `value="alice@x.fr"`) {
		t.Errorf("formulaire non prérempli :\n%s", form)
	}
}

var _ = mailtest.User

func TestFiltersRateLimit(t *testing.T) {
	srv := sievetest.Start(t, "none")
	form := url.Values{"sieve_enabled": {"on"}, "sieve_host": {"127.0.0.1"}, "sieve_port": {strconv.Itoa(srv.Port)}, "sieve_security": {"none"},
		"sieve_login": {sievetest.User}, "sieve_password": {sievetest.Password}, "sieve_verify": {"on"}}
	e, c, _, _ := composeEnvWith(t, func(cfg *config.Config) {
		cfg.RateLimit.Sieve = config.RateRule{PerIP: config.Limit{Count: 100, Window: 600e9}, PerLogin: config.Limit{Count: 3, Window: 600e9}}
	}, form)
	for i := 0; i < 3; i++ {
		resp, _ := e.do(c, "GET", "/settings/filters", nil, nil)
		e.expect(resp, 200, "")
	}
	resp, _ := e.do(c, "GET", "/settings/filters", nil, nil)
	if resp.StatusCode != 429 || resp.Header.Get("Retry-After") == "" {
		t.Errorf("quatrième requête : %d", resp.StatusCode)
	}
}
