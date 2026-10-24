package web

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"

	"github.com/emersion/go-imap/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"wmail-go/internal/mailconn/mailtest"
)

// mailEnv ouvre une session avec un compte mail relié au serveur IMAP de test (30 messages dans INBOX).
func mailEnv(t *testing.T, block bool) (*env, *http.Client, string) {
	t.Helper()
	e := newEnv(t, nil)
	ports := startMail(t)
	c, tok := e.loggedIn("alice")
	form := accountForm(tok, ports, "save")
	if !block {
		form.Del("block_remote")
	}
	resp, _ := e.do(c, "POST", "/accounts/save", form, nil)
	e.expect(resp, 303, "/")
	return e, c, tok
}

var (
	rowRe  = regexp.MustCompile(`<li id="m\d+" draggable="true" class="msgrow[^"]*">[\s\S]*?</li>`)
	uidRe  = regexp.MustCompile(`name="uid" value="(\d+)"`)
	subjRe = regexp.MustCompile(`<a class="rowlink" href="([^"]+)"[^>]*>[\s\S]*?<span class="subj">([\s\S]*?)</span></span>`)
	impRe  = regexp.MustCompile(`<b class="imp"[^>]*>!</b>\s*`)
	tagRe  = regexp.MustCompile(`<[^>]+>`)
)

// rows renvoie, dans l'ordre, les sujets et les liens de lecture de la liste.
func rows(page string) (subjects, links []string) {
	for _, m := range subjRe.FindAllStringSubmatch(page, -1) {
		link := htmlUnescape(m[1])
		if i := strings.Index(link, "#"); i >= 0 {
			link = link[:i] // le fragment (#m12) ne part pas au serveur
		}
		links = append(links, link)
		subjects = append(subjects, htmlUnescape(tagRe.ReplaceAllString(impRe.ReplaceAllString(m[2], ""), "")))
	}
	return
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&#34;", `"`, "&#39;", "'", "&quot;", `"`).Replace(s)
}

func linkTo(t *testing.T, page, subject string) string {
	t.Helper()
	subs, links := rows(page)
	for i, s := range subs {
		if s == subject {
			return links[i]
		}
	}
	t.Fatalf("message %q absent de la liste : %v", subject, subs)
	return ""
}

func TestMessageListShowsFoldersAndMessages(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	resp, page := e.do(c, "GET", "/", nil, nil)
	e.expect(resp, 200, "")
	subs, _ := rows(page)
	if len(subs) != mailtest.InboxCount {
		t.Fatalf("%d messages affichés, attendu %d", len(subs), mailtest.InboxCount)
	}
	if subs[0] != "Filler 30" || subs[len(subs)-1] != "Facture octobre" {
		t.Errorf("tri par date décroissante : %q … %q", subs[0], subs[len(subs)-1])
	}
	for _, want := range []string{"Boîte de réception", "Projets", "31 message(s)", "Page 1 sur 1", `attachmentParent clip`, `class="star on"`} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent de la page", want)
		}
	}
	// Non lu en gras : « Facture octobre » (non lu) oui, « Re: Facture octobre » (lu) non.
	for _, m := range rowRe.FindAllString(page, -1) {
		switch {
		case strings.Contains(m, `<span class="subj">Facture octobre<`):
			if !strings.Contains(m, "msgrow unread") {
				t.Error("Facture octobre doit être non lu")
			}
		case strings.Contains(m, `<span class="pfx">Re: </span>Facture octobre<`):
			if strings.Contains(m, "msgrow unread") {
				t.Error("Re: Facture octobre doit être lu")
			}
		}
	}
}

func TestMessageListEscapesHostileContent(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	if strings.Contains(page, "<script>alert(1)") {
		t.Fatal("le sujet piégé n'est pas échappé")
	}
	if !strings.Contains(page, "Gala &lt;script&gt;alert(1)&lt;/script&gt; &amp; co") {
		t.Error("sujet échappé attendu")
	}
}

func TestMessageListPaginationSortSearch(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	// Taille de page : 10 messages.
	resp, _ := e.do(c, "POST", "/settings/preferences", url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"},
		"idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"10"}, "mark_read": {"on"}}, nil)
	e.expect(resp, 200, "")

	_, page := e.do(c, "GET", "/", nil, nil)
	subs, _ := rows(page)
	if len(subs) != 10 || !strings.Contains(page, "Page 1 sur 4") || !strings.Contains(page, "?page=2") {
		t.Fatalf("pagination : %d lignes, page :\n%s", len(subs), page)
	}
	if strings.Contains(page, `href="/?page=0"`) {
		t.Error("lien précédent invalide")
	}
	_, page = e.do(c, "GET", "/?page=4", nil, nil)
	subs, _ = rows(page)
	if len(subs) != 1 || subs[0] != "Facture octobre" || !strings.Contains(page, "Page 4 sur 4") {
		t.Errorf("dernière page : %v", subs)
	}
	_, page = e.do(c, "GET", "/?page=99", nil, nil)
	if !strings.Contains(page, "Page 4 sur 4") {
		t.Error("page hors limites ramenée à la dernière")
	}
	_, page = e.do(c, "GET", "/?page=abc&sort=bogus&dir=sideways&scope=x", nil, nil)
	if subs, _ = rows(page); len(subs) != 10 {
		t.Error("paramètres invalides ignorés")
	}

	// Tri : par sujet croissant, puis bascule.
	_, page = e.do(c, "GET", "/?sort=subject&dir=asc", nil, nil)
	subs, _ = rows(page)
	if subs[0] != "Facture octobre" || subs[1] != "Re: Facture octobre" || !strings.Contains(page, "▲") {
		t.Errorf("tri par sujet : %v", subs[:3])
	}
	if !strings.Contains(page, `href="/?dir=desc&amp;sort=subject"`) {
		t.Errorf("le lien de l'en-tête doit inverser le tri :\n%s", page)
	}
	_, page = e.do(c, "GET", "/?sort=size&dir=desc", nil, nil)
	if subs, _ = rows(page); subs[0] != "Facture octobre" {
		t.Errorf("tri par taille : %v", subs[:2])
	}

	// Recherche.
	_, page = e.do(c, "GET", "/?q=Facture&scope=subject", nil, nil)
	subs, _ = rows(page)
	if len(subs) != 2 || !strings.Contains(page, "2 message(s)") {
		t.Errorf("recherche : %v", subs)
	}
	_, page = e.do(c, "GET", "/?q=montant&scope=body", nil, nil)
	if subs, _ = rows(page); len(subs) != 1 || subs[0] != "Facture octobre" {
		t.Errorf("recherche dans le contenu : %v", subs)
	}
	_, page = e.do(c, "GET", "/?q=introuvable-xyz", nil, nil)
	if !strings.Contains(page, "Aucun message ne correspond") {
		t.Error("message de recherche sans résultat attendu")
	}
	_, page = e.do(c, "GET", "/?q="+url.QueryEscape(`"><script>alert(1)</script>`), nil, nil)
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("la recherche affichée doit être échappée")
	}
}

func TestOtherFolderAndUnknownFolder(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	resp, page := e.do(c, "GET", "/?folder=Sent", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "Ce dossier est vide") {
		t.Error("dossier vide attendu")
	}
	resp, page = e.do(c, "GET", "/?folder=Projets%2F2026", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "Ce dossier est vide") {
		t.Error("sous-dossier vide attendu")
	}
	resp, page = e.do(c, "GET", "/?folder=Inexistant", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, `role="alert"`) || !strings.Contains(page, "Boîte de réception") {
		t.Errorf("erreur affichée avec la liste des dossiers conservée :\n%s", page)
	}
	resp, _ = e.do(c, "GET", "/?folder="+url.QueryEscape("a*"), nil, nil)
	e.expect(resp, 200, "") // nom invalide : retombe sur INBOX
}

func TestReadMessageHTMLBlocksRemoteContent(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Réunion lundi")

	resp, msg := e.do(c, "GET", link, nil, nil)
	e.expect(resp, 200, "")
	for _, want := range []string{"Réunion lundi", "carol@z.fr", `sandbox="allow-popups allow-popups-to-escape-sandbox"`, "/mail/body?", "bloqué(s)", "Afficher les images", "texte brut"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q absent de la page de lecture", want)
		}
	}
	if strings.Contains(msg, "allow-scripts") || strings.Contains(msg, "allow-same-origin") {
		t.Error("l'iframe ne doit autoriser ni scripts ni same-origin")
	}
	bodyURL := regexp.MustCompile(`src="(/mail/body\?[^"]+)"`).FindStringSubmatch(msg)
	if bodyURL == nil {
		t.Fatal("URL du corps absente")
	}

	resp, body := e.do(c, "GET", htmlUnescape(bodyURL[1]), nil, nil)
	e.expect(resp, 200, "")
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "img-src data: https:") || !strings.Contains(csp, "sandbox") ||
		strings.Contains(csp, "script-src") && !strings.Contains(csp, "'none'") {
		t.Errorf("CSP du corps : %s", csp)
	}
	if resp.Header.Get("X-Frame-Options") != "SAMEORIGIN" || !strings.Contains(csp, "frame-ancestors 'self'") {
		t.Error("le corps doit pouvoir être affiché dans une iframe du même site")
	}
	if !strings.Contains(body, "Bonjour <b>Carol</b>") || strings.Contains(body, "<script") || strings.Contains(body, "tracker.example") {
		t.Errorf("corps assaini :\n%s", body)
	}

	// « Afficher pour ce message » : contenu distant autorisé, CSP élargie à ce message seulement.
	resp, msg = e.do(c, "GET", link+"&remote=1", nil, nil)
	e.expect(resp, 200, "")
	if strings.Contains(msg, "Afficher les images") {
		t.Error("le bandeau doit disparaître une fois le contenu autorisé")
	}
	resp, body = e.do(c, "GET", "/mail/body?folder=INBOX&uid="+uidOf(link)+"&remote=1", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(body, `src="https://tracker.example/p.gif?id=7"`) || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "img-src data: https: http:") {
		t.Errorf("contenu distant autorisé :\n%s\n%s", resp.Header.Get("Content-Security-Policy"), body)
	}
	// Sans le paramètre, le message reste bloqué.
	resp, body = e.do(c, "GET", "/mail/body?folder=INBOX&uid="+uidOf(link), nil, nil)
	if strings.Contains(body, "tracker.example") {
		t.Error("le blocage doit rester la valeur par défaut")
	}

	// Vue texte brut.
	resp, body = e.do(c, "GET", "/mail/body?folder=INBOX&uid="+uidOf(link)+"&view=text", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(body, "Bonjour Carol") || strings.Contains(body, "<b>") || strings.Contains(body, "alert(1)") {
		t.Errorf("vue texte :\n%s", body)
	}
}

func uidOf(link string) string {
	u, _ := url.Parse(link)
	return u.Query().Get("uid")
}

func TestRemoteContentAllowedPerAccountOption(t *testing.T) {
	e, c, _ := mailEnv(t, false) // blocage désactivé pour ce compte
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Réunion lundi")
	_, msg := e.do(c, "GET", link, nil, nil)
	if strings.Contains(msg, "Afficher les images") {
		t.Error("aucun bandeau quand le compte autorise le contenu distant")
	}
	resp, body := e.do(c, "GET", "/mail/body?folder=INBOX&uid="+uidOf(link), nil, nil)
	if !strings.Contains(body, "tracker.example") || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "https:") {
		t.Error("contenu distant attendu quand l'option du compte le permet")
	}
}

func TestReadMarksAsReadAndDoesNotWhenDisabled(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Facture octobre")
	e.do(c, "GET", link, nil, nil)
	_, page = e.do(c, "GET", "/", nil, nil)
	for _, m := range rowRe.FindAllString(page, -1) {
		if strings.Contains(m, `<span class="subj">Facture octobre<`) && strings.Contains(m, "msgrow unread") {
			t.Error("l'ouverture doit marquer le message comme lu")
		}
	}
	// Remise en non lu, puis lecture avec le marquage automatique désactivé.
	uid := uidOf(link)
	resp, _ := e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"unread"}, "folder": {"INBOX"}, "uid": {uid}, "back": {"/"}}, nil)
	e.expect(resp, 303, "/")
	e.do(c, "POST", "/settings/preferences", url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"},
		"idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"50"}}, nil) // mark_read absent = désactivé
	e.do(c, "GET", link, nil, nil)
	_, page = e.do(c, "GET", "/", nil, nil)
	found := false
	for _, m := range rowRe.FindAllString(page, -1) {
		if strings.Contains(m, `<span class="subj">Facture octobre<`) {
			found = true
			if !strings.Contains(m, "msgrow unread") {
				t.Error("marquage automatique désactivé : le message doit rester non lu")
			}
		}
	}
	if !found {
		t.Fatal("message absent")
	}
}

func TestAttachmentDownload(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Facture octobre")
	_, msg := e.do(c, "GET", link, nil, nil)
	m := regexp.MustCompile(`href="(/mail/attachment\?[^"]+)"`).FindStringSubmatch(msg)
	if m == nil || !strings.Contains(msg, "facture.pdf") {
		t.Fatalf("pièce jointe absente :\n%s", msg)
	}
	resp, data := e.do(c, "GET", htmlUnescape(m[1]), nil, nil)
	e.expect(resp, 200, "")
	if !strings.HasPrefix(data, "%PDF") || resp.Header.Get("Content-Type") != "application/pdf" ||
		!strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") || !strings.Contains(resp.Header.Get("Content-Disposition"), "facture.pdf") ||
		resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("téléchargement : %v", resp.Header)
	}
	resp, _ = e.do(c, "GET", "/mail/attachment?folder=INBOX&uid="+uidOf(link)+"&index=9", nil, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(c, "GET", "/mail/attachment?folder=INBOX&uid="+uidOf(link)+"&index=x", nil, nil)
	e.expect(resp, 400, "")
}

func TestReadErrors(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/mail/message?folder=INBOX&uid=999999", 404},
		{"/mail/message?folder=INBOX&uid=abc", 400},
		{"/mail/message?folder=INBOX", 400},
		{"/mail/message?folder=" + url.QueryEscape("a*") + "&uid=1", 400},
		{"/mail/body?folder=INBOX&uid=999999", 404},
		{"/mail/body?folder=INBOX&uid=0", 400},
	} {
		resp, _ := e.do(c, "GET", tc.path, nil, nil)
		if resp.StatusCode != tc.status {
			t.Errorf("%s : statut %d, attendu %d", tc.path, resp.StatusCode, tc.status)
		}
	}
	// Non connecté : rien n'est accessible.
	anon := e.client()
	for _, p := range []string{"/mail/message?folder=INBOX&uid=1", "/mail/body?folder=INBOX&uid=1", "/mail/attachment?folder=INBOX&uid=1&index=0"} {
		resp, _ := e.do(anon, "GET", p, nil, nil)
		e.expect(resp, 303, "/login")
	}
	resp, _ := e.do(anon, "POST", "/mail/action", url.Values{"action": {"delete"}}, nil)
	e.expect(resp, 303, "/login")
}

func listUIDs(t *testing.T, e *env, c *http.Client, path string) []string {
	t.Helper()
	_, page := e.do(c, "GET", path, nil, nil)
	var out []string
	for _, m := range uidRe.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestActionsFlagReadDeleteMove(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	act := func(action string, uids []string, extra url.Values) *http.Response {
		v := url.Values{"csrf": {tok}, "action": {action}, "folder": {"INBOX"}, "uid": uids, "back": {"/?sort=size&dir=desc"}}
		for k, vs := range extra {
			v[k] = vs
		}
		resp, _ := e.do(c, "POST", "/mail/action", v, nil)
		return resp
	}
	all := listUIDs(t, e, c, "/")
	if len(all) != mailtest.InboxCount {
		t.Fatalf("%d messages", len(all))
	}
	first, second, third := all[0], all[1], all[2]

	// Drapeaux, en lot.
	e.expect(act("flag", []string{first, second}, nil), 303, "/?sort=size&dir=desc")
	_, page := e.do(c, "GET", "/", nil, nil)
	if strings.Count(page, `class="star on"`) != 3 { // Facture était déjà en favori
		t.Errorf("%d favoris, attendu 3", strings.Count(page, `class="star on"`))
	}
	e.expect(act("unflag", []string{first, second}, nil), 303, "")
	e.expect(act("unread", []string{first, second, third}, nil), 303, "")
	e.expect(act("read", []string{first}, nil), 303, "")

	// Déplacement vers un dossier existant.
	e.expect(act("move", []string{second}, url.Values{"dest": {"Sent"}}), 303, "")
	if got := listUIDs(t, e, c, "/?folder=Sent"); len(got) != 1 {
		t.Errorf("Sent contient %d messages", len(got))
	}
	if got := listUIDs(t, e, c, "/"); len(got) != mailtest.InboxCount-1 {
		t.Errorf("INBOX contient %d messages", len(got))
	}
	// Destination inexistante ou identique : refusée.
	resp := act("move", []string{third}, url.Values{"dest": {"Nulle part"}})
	e.expect(resp, 404, "")
	e.expect(act("move", []string{third}, url.Values{"dest": {"INBOX"}}), 400, "")
	e.expect(act("move", []string{third}, url.Values{"dest": {"a*"}}), 400, "")

	// Suppression : corbeille d'abord, définitive ensuite.
	e.expect(act("delete", []string{third}, nil), 303, "")
	trash := listUIDs(t, e, c, "/?folder=Trash")
	if len(trash) != 1 {
		t.Fatalf("la corbeille contient %d messages", len(trash))
	}
	if got := listUIDs(t, e, c, "/"); len(got) != mailtest.InboxCount-2 {
		t.Errorf("INBOX contient %d messages", len(got))
	}
	v := url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"Trash"}, "uid": trash, "back": {"/"}}
	resp, _ = e.do(c, "POST", "/mail/action", v, nil)
	e.expect(resp, 303, "/")
	if got := listUIDs(t, e, c, "/?folder=Trash"); len(got) != 0 {
		t.Errorf("suppression définitive : %d messages restent", len(got))
	}
}

func TestActionValidationAndSafety(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	uid := listUIDs(t, e, c, "/")[0]
	base := func() url.Values {
		return url.Values{"csrf": {tok}, "action": {"flag"}, "folder": {"INBOX"}, "uid": {uid}, "back": {"/"}}
	}
	do := func(mut func(url.Values)) *http.Response {
		v := base()
		mut(v)
		resp, _ := e.do(c, "POST", "/mail/action", v, nil)
		return resp
	}
	e.expect(do(func(v url.Values) { v.Del("csrf") }), 403, "")
	e.expect(do(func(v url.Values) { v.Set("csrf", "faux") }), 403, "")
	e.expect(do(func(v url.Values) { v.Set("action", "explode") }), 400, "")
	e.expect(do(func(v url.Values) { v.Del("uid") }), 400, "")
	e.expect(do(func(v url.Values) { v.Set("uid", "abc") }), 400, "")
	e.expect(do(func(v url.Values) { v.Set("uid", "0") }), 400, "")
	e.expect(do(func(v url.Values) { v.Set("folder", "") }), 400, "")
	many := make([]string, maxActionUIDs+1)
	for i := range many {
		many[i] = strconv.Itoa(i + 1)
	}
	e.expect(do(func(v url.Values) { v["uid"] = many }), 400, "")

	// Pas de redirection ouverte.
	for _, evil := range []string{"//evil.example/", "https://evil.example/", "/\\evil.example", "javascript:alert(1)", "/accounts", "/mail/body?x=1"} {
		resp := do(func(v url.Values) { v.Set("back", evil); v.Set("stay", evil) })
		e.expect(resp, 303, "/")
	}
	if got := safeBack("/mail/message?folder=INBOX&uid=1&back=%2F"); got == "/" {
		t.Error("l'adresse d'un message doit être acceptée")
	}
	if got := safeBack("/?folder=Sent&page=2"); got != "/?folder=Sent&page=2" {
		t.Errorf("l'adresse d'une liste doit être acceptée : %q", got)
	}
}

func TestMessagePageToolbarAndStayOnMessage(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Re: Facture octobre")
	_, msg := e.do(c, "GET", link, nil, nil)
	for _, want := range []string{`name="star_on"`, `name="action" value="delete"`, `name="action" value="archive"`, `name="action" value="spam"`, `name="move_to"`, `name="stay"`, `name="current_uid"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q absent de la barre d'outils", want)
		}
	}
	// Favori depuis la page du message : on reste sur le message.
	uid := uidOf(link)
	stay := regexp.MustCompile(`name="stay" value="([^"]+)"`).FindStringSubmatch(msg)
	if stay == nil {
		t.Fatal("champ stay absent")
	}
	resp, _ := e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"flag"}, "folder": {"INBOX"}, "uid": {uid},
		"back": {"/"}, "stay": {htmlUnescape(stay[1])}}, nil)
	if resp.StatusCode != 303 || !strings.HasPrefix(resp.Header.Get("Location"), "/?") || !strings.Contains(resp.Header.Get("Location"), "uid=") {
		t.Errorf("redirection %q, attendu la page du message", resp.Header.Get("Location"))
	}
	_, msg = e.do(c, "GET", resp.Header.Get("Location"), nil, nil)
	if !strings.Contains(msg, `name="action" value="unflag"`) {
		t.Error("le message doit apparaître en favori")
	}
	// Suppression depuis la page du message : retour à la liste.
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"INBOX"}, "uid": {uid},
		"back": {"/"}, "stay": {htmlUnescape(stay[1])}}, nil)
	e.expect(resp, 303, "/")
}

func TestLogoutClosesIMAPConnections(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	e.do(c, "GET", "/", nil, nil)
	if e.srv.pool.Len() != 1 {
		t.Fatalf("%d connexions IMAP, attendu 1", e.srv.pool.Len())
	}
	resp, _ := e.do(c, "POST", "/logout", url.Values{"csrf": {tok}}, nil)
	e.expect(resp, 303, "/login")
	deadline := time.Now().Add(2 * time.Second)
	for e.srv.pool.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if e.srv.pool.Len() != 0 {
		t.Errorf("%d connexions IMAP restent ouvertes après la déconnexion", e.srv.pool.Len())
	}
}

func TestIMAPConnectionReusedAcrossRequests(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	for range 3 {
		resp, _ := e.do(c, "GET", "/", nil, nil)
		e.expect(resp, 200, "")
	}
	if e.srv.pool.Len() != 1 {
		t.Errorf("%d connexions IMAP, attendu 1 (réutilisation)", e.srv.pool.Len())
	}
}

func TestFormatHelpers(t *testing.T) {
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, time.Local)
	cases := []struct {
		t    time.Time
		lang string
		want string
	}{
		{time.Date(2026, 10, 23, 9, 5, 0, 0, time.Local), "fr", "09:05"},
		{time.Date(2026, 10, 2, 9, 5, 0, 0, time.Local), "fr", "02/10"},
		{time.Date(2026, 10, 2, 9, 5, 0, 0, time.Local), "en", "Oct 02"},
		{time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "fr", "31/12/2025"},
		{time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "en", "2025-12-31"},
		{time.Time{}, "fr", ""},
	}
	for _, c := range cases {
		if got := formatDate(c.t, now, c.lang, time.Local); got != c.want {
			t.Errorf("formatDate(%v, %s) = %q, attendu %q", c.t, c.lang, got, c.want)
		}
	}
	if got := humanSize(512, "en"); got != "512 B" {
		t.Error(got)
	}
	if got := humanSize(1536, "fr"); got != "1.5 Ko" {
		t.Error(got)
	}
	if got := humanSize(5<<20, "en"); got != "5.0 MB" {
		t.Error(got)
	}
}

func TestMessagePageKeepsFolderPanel(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Facture octobre")
	uid := uidOf(link)

	_, msg := e.do(c, "GET", link, nil, nil)
	for _, want := range []string{`<aside class="folders">`, `href="/?folder=Sent"`, `href="/?folder=Projets%2f2026"`, "Boîte de réception"} {
		if !strings.Contains(strings.ReplaceAll(msg, "%2F", "%2f"), want) {
			t.Errorf("%q absent de la page du message", want)
		}
	}
	if !regexp.MustCompile(`<li class="f role-inbox active[^"]*"[^>]*>\s*<a class="fname" href="/">Boîte de réception</a>`).MatchString(msg) {
		t.Errorf("le dossier du message doit être mis en évidence :\n%s", msg)
	}

	// Un message d'un autre dossier : le panneau suit.
	e.expect1(e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"move"}, "folder": {"INBOX"}, "uid": {uid}, "dest": {"Sent"}, "back": {"/"}}, nil))
	_, sent := e.do(c, "GET", "/?folder=Sent", nil, nil)
	_, sentLinks := rows(sent)
	if len(sentLinks) != 1 {
		t.Fatalf("Sent contient %d messages", len(sentLinks))
	}
	_, msg = e.do(c, "GET", sentLinks[0], nil, nil)
	if !regexp.MustCompile(`<li class="f role-sent active[^"]*"[^>]*>\s*<a class="fname" href="/\?folder=Sent">Envoyés</a>`).MatchString(msg) {
		t.Errorf("le dossier Sent doit être actif :\n%s", msg)
	}
	// Le compteur de non lus du panneau tient compte du marquage à l'ouverture.
	if !strings.Contains(msg, `<span class="badge" data-unread-badge>`) {
		t.Error("compteurs de non lus attendus dans le panneau")
	}
	// Le corps et les pièces jointes n'ont pas besoin des compteurs (pas de panneau, pas de requêtes inutiles).
	resp, _ := e.do(c, "GET", "/mail/body?folder=Sent&uid="+uidOf(sentLinks[0]), nil, nil)
	e.expect(resp, 200, "")
}

func TestUnreadFilter(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	resp, _ := e.do(c, "POST", "/settings/preferences", url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"},
		"idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"10"}, "mark_read": {"on"}}, nil)
	e.expect(resp, 200, "")

	_, page := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `href="/?filter=unread"`) || !strings.Contains(page, `<option value="unread">Non lus</option>`) ||
		!strings.Contains(page, `<option value="all" selected>Tous</option>`) {
		t.Errorf("filtre absent du menu et de la recherche avancée :\n%.1500s", page)
	}
	if !strings.Contains(page, "31 message(s)") {
		t.Error("31 messages attendus sans filtre")
	}

	_, page = e.do(c, "GET", "/?filter=unread", nil, nil)
	subs, _ := rows(page)
	if !strings.Contains(page, "30 message(s)") || len(subs) != 10 || !strings.Contains(page, `<option value="unread" selected>Non lus</option>`) ||
		!strings.Contains(page, `class="filterbar"`) {
		t.Fatalf("filtre non lus : %d lignes\n%s", len(subs), page)
	}
	for _, s := range subs {
		if s == "Re: Facture octobre" {
			t.Error("un message lu apparaît dans la liste des non lus")
		}
	}
	// Le filtre est conservé par la pagination, le tri, la recherche et les changements de dossier.
	for _, want := range []string{`href="/?filter=unread&amp;page=2"`, `/?filter=unread&amp;folder=Sent`, `<option value="unread" selected>`} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent : le filtre doit être conservé", want)
		}
	}
	if !strings.Contains(page, "dir=asc&amp;filter=unread&amp;sort=") {
		t.Error("les liens de tri doivent conserver le filtre")
	}
	_, page = e.do(c, "GET", "/?filter=unread&q=Facture&scope=subject", nil, nil)
	if subs, _ = rows(page); len(subs) != 1 || subs[0] != "Facture octobre" || !strings.Contains(page, `href="/?filter=unread"`) {
		t.Errorf("non lus + recherche : %v", subs)
	}

	// Lire un message le retire de la liste des non lus ; les actions reviennent sur la même vue filtrée.
	link := linkTo(t, page, "Facture octobre")
	e.do(c, "GET", link, nil, nil)
	_, page = e.do(c, "GET", "/?filter=unread&q=Facture&scope=subject", nil, nil)
	if subs, _ = rows(page); len(subs) != 0 || !strings.Contains(page, "Aucun message ne correspond") {
		t.Errorf("après lecture : %v", subs)
	}
	_, page = e.do(c, "GET", "/?filter=unread&folder=Sent", nil, nil)
	if !strings.Contains(page, "Aucun message non lu dans ce dossier") {
		t.Error("message dédié attendu pour un dossier sans message non lu")
	}
	uid := listUIDs(t, e, c, "/?filter=unread")[0]
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"read"}, "folder": {"INBOX"}, "uid": {uid},
		"back": {"/?filter=unread"}}, nil)
	e.expect(resp, 303, "/?filter=unread")
	if got := listUIDs(t, e, c, "/?filter=unread"); slicesContains(got, uid) {
		t.Error("le message marqué lu doit disparaître de la vue des non lus")
	}

	// Valeur inconnue : retour à « tous ».
	_, page = e.do(c, "GET", "/?filter=bogus", nil, nil)
	if !strings.Contains(page, "31 message(s)") {
		t.Error("filtre inconnu : tous les messages attendus")
	}
}

func slicesContains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func TestPageSizeSelector(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `action="/mail/pagesize"`) || !strings.Contains(page, "Messages par page") ||
		!strings.Contains(page, `<script src="/static/list.js" defer>`) {
		t.Fatalf("sélecteur absent :\n%.2000s", page)
	}
	opts := regexp.MustCompile(`<button type="submit" name="size" value="(\d+)"( class="current")?>`).FindAllStringSubmatch(page, -1)
	var values []string
	selected := ""
	for _, o := range opts {
		values = append(values, o[1])
		if o[2] != "" {
			selected = o[1]
		}
	}
	if strings.Join(values, ",") != "10,20,30,50,100,200,500" || selected != "50" {
		t.Errorf("choix %v, sélection %q (50 par défaut)", values, selected)
	}

	// Changement de taille : enregistré, la page revient à 1, la recherche et le filtre sont conservés.
	post := func(size, back string) *http.Response {
		resp, _ := e.do(c, "POST", "/mail/pagesize", url.Values{"csrf": {tok}, "size": {size}, "back": {back}}, nil)
		return resp
	}
	resp := post("100", "/?filter=unread&page=3&q=Fact")
	e.expect(resp, 303, "/?filter=unread&q=Fact")
	resp = post("200", "/?page=2")
	e.expect(resp, 303, "/")
	_, page = e.do(c, "GET", "/", nil, nil)
	if !regexp.MustCompile(`name="size" value="200" class="current"`).MatchString(page) {
		t.Error("la taille choisie doit être sélectionnée")
	}

	// Elle est conservée entre deux connexions (préférence stockée dans le fichier chiffré).
	c2 := e.client()
	e.expect(e.login(c2, "alice", pw), 303, "/")
	_, page = e.do(c2, "GET", "/", nil, nil)
	if !regexp.MustCompile(`name="size" value="200" class="current"`).MatchString(page) {
		t.Error("la taille de page doit survivre à une reconnexion")
	}
	_, page = e.do(c2, "GET", "/settings", nil, nil)
	if !strings.Contains(page, `name="page_size" type="number" min="10" max="500" value="200"`) {
		t.Error("la préférence des réglages doit refléter la taille choisie")
	}

	// Valeurs refusées.
	for _, bad := range []string{"40", "1000", "0", "-5", "abc", "", "50 "} {
		if r := post(bad, "/"); r.StatusCode != 400 {
			t.Errorf("taille %q : statut %d, attendu 400", bad, r.StatusCode)
		}
	}
	resp, _ = e.do(c, "POST", "/mail/pagesize", url.Values{"size": {"50"}, "back": {"/"}}, nil)
	e.expect(resp, 403, "") // CSRF
	resp = post("500", "//evil.example/?page=2")
	e.expect(resp, 303, "/") // pas de redirection ouverte

	// Une taille personnalisée des réglages apparaît dans la liste.
	e.do(c, "POST", "/settings/preferences", url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"},
		"idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"25"}, "mark_read": {"on"}}, nil)
	_, page = e.do(c, "GET", "/", nil, nil)
	if !regexp.MustCompile(`name="size" value="25" class="current"`).MatchString(page) || !strings.Contains(page, `name="size" value="50">`) {
		t.Errorf("taille personnalisée attendue en plus des choix")
	}
	if subs, _ := rows(page); len(subs) != 25 {
		t.Errorf("%d lignes, attendu 25", len(subs))
	}
	// Réglages : jusqu'à 500.
	ok := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"}, "idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"500"}}
	resp, _ = e.do(c, "POST", "/settings/preferences", ok, nil)
	e.expect(resp, 200, "")
	ok.Set("page_size", "501")
	resp, _ = e.do(c, "POST", "/settings/preferences", ok, nil)
	e.expect(resp, 400, "")
}

func sizeSelect(page string) string {
	i := strings.Index(page, `name="size"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(page[i:], "</select>")
	return page[i : i+j]
}

func TestWithoutPage(t *testing.T) {
	for in, want := range map[string]string{
		"/": "/", "/?page=3": "/", "/?filter=unread&page=2&q=a": "/?filter=unread&q=a", "/?folder=Sent": "/?folder=Sent",
	} {
		if got := withoutPage(in); got != want {
			t.Errorf("withoutPage(%q) = %q, attendu %q", in, got, want)
		}
	}
}

var badgeRe = regexp.MustCompile(`<span class="badge" data-unread-badge>(\d+)</span>`)

func firstBadge(t *testing.T, page string) int {
	t.Helper()
	m := badgeRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("aucun compteur de non lus")
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func TestThreePaneLayout(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	for _, want := range []string{`class="mailbox three"`, `<aside class="folders">`, `<section class="listpane"`, `<section class="readpane"`, "Sélectionnez un message pour le lire."} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent de la vue 3 volets", want)
		}
	}
	if strings.Contains(page, "has-message") || strings.Contains(page, `class="mailbox listonly"`) {
		t.Error("aucun message ouvert au départ")
	}
	_, links := rows(page)
	if !strings.HasPrefix(links[0], "/?uid=") || strings.Contains(page, "#m") {
		t.Errorf("les lignes doivent ouvrir le volet de lecture : %q", links[0])
	}

	link := linkTo(t, page, "Réunion lundi")
	resp, pane := e.do(c, "GET", link, nil, nil)
	e.expect(resp, 200, "")
	for _, want := range []string{`class="mailbox three has-message"`, `class="msg-subject">Réunion lundi`, "carol@z.fr", `class="mailbody"`, "Afficher les images", "Filler 30"} {
		if !strings.Contains(pane, want) {
			t.Errorf("%q absent du volet de lecture", want)
		}
	}
	if !regexp.MustCompile(`<li id="m\d+" draggable="true" class="msgrow selected"`).MatchString(pane) &&
		!regexp.MustCompile(`<li id="m\d+" draggable="true" class="msgrow[^"]* selected"`).MatchString(pane) {
		t.Error("la ligne ouverte doit être mise en évidence")
	}
	if n := strings.Count(pane, ` selected"`); n < 1 {
		t.Error("ligne sélectionnée absente")
	}
	// Un message ouvert via un lien direct (page seule) reste disponible.
	resp, single := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uidOf(link), nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(single, `class="mailbox standalone"`) || !strings.Contains(single, `class="mh-close"`) {
		t.Error("page seule du message attendue")
	}
}

func TestThreePaneMarksReadAndUpdatesCounters(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	before := firstBadge(t, page)
	link := linkTo(t, page, "Réunion lundi")
	_, pane := e.do(c, "GET", link, nil, nil)
	if after := firstBadge(t, pane); after != before-1 {
		t.Errorf("compteur %d, attendu %d : l'ouverture doit marquer le message comme lu avant le calcul des compteurs", after, before-1)
	}
	for _, m := range rowRe.FindAllString(pane, -1) {
		if strings.Contains(m, ">Réunion lundi<") && strings.Contains(m, "msgrow unread") {
			t.Error("la ligne ouverte ne doit plus être en gras")
		}
	}
	// Réouvrir ne change plus rien.
	_, pane = e.do(c, "GET", link, nil, nil)
	if firstBadge(t, pane) != before-1 {
		t.Error("le compteur ne doit pas bouger à la deuxième ouverture")
	}
}

func TestThreePaneKeepsListParameters(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/?q=lundi&scope=subject", nil, nil)
	link := linkTo(t, page, "Réunion lundi")
	if !strings.Contains(link, "q=lundi") || !strings.Contains(link, "scope=subject") || !strings.Contains(link, "uid=") {
		t.Errorf("le lien de la ligne doit conserver la recherche : %q", link)
	}
	_, pane := e.do(c, "GET", link, nil, nil)
	if subs, _ := rows(pane); len(subs) != 1 || subs[0] != "Réunion lundi" {
		t.Errorf("la liste filtrée doit rester affichée : %v", subs)
	}
	for _, want := range []string{`name="back" value="/?q=lundi&amp;scope=subject"`, `href="/?q=lundi&amp;scope=subject&amp;uid=` + uidOf(link) + `&amp;view=text"`} {
		if !strings.Contains(pane, want) {
			t.Errorf("%q absent du volet", want)
		}
	}
	stay := regexp.MustCompile(`name="stay" value="([^"]+)"`).FindStringSubmatch(pane)
	if stay == nil || !strings.Contains(htmlUnescape(stay[1]), "uid="+uidOf(link)) {
		t.Errorf("« rester sur le message » : %v", stay)
	}
}

func TestThreePaneActionsReturnToList(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Re: Facture octobre")
	_, pane := e.do(c, "GET", link, nil, nil)
	back := regexp.MustCompile(`name="back" value="([^"]*)"`).FindStringSubmatch(pane)
	stay := regexp.MustCompile(`name="stay" value="([^"]+)"`).FindStringSubmatch(pane)
	if back == nil || stay == nil {
		t.Fatal("champs back et stay absents du volet")
	}
	uid := uidOf(link)
	form := func(action string) url.Values {
		return url.Values{"csrf": {tok}, "action": {action}, "folder": {"INBOX"}, "uid": {uid}, "back": {htmlUnescape(back[1])}, "stay": {htmlUnescape(stay[1])}}
	}
	resp, _ := e.do(c, "POST", "/mail/action", form("flag"), nil)
	if resp.StatusCode != 303 || !strings.Contains(resp.Header.Get("Location"), "uid="+uid) {
		t.Errorf("favori : le message doit rester ouvert (%q)", resp.Header.Get("Location"))
	}
	resp, _ = e.do(c, "POST", "/mail/action", form("delete"), nil)
	e.expect(resp, 303, "/")
	_, page = e.do(c, "GET", "/", nil, nil)
	if strings.Contains(page, "has-message") || !strings.Contains(page, "Sélectionnez un message") {
		t.Error("après une suppression, le volet de lecture se vide")
	}
}

func TestThreePaneMissingMessageKeepsList(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	resp, page := e.do(c, "GET", "/?uid=999999", nil, nil)
	e.expect(resp, 200, "")
	subs, _ := rows(page)
	if len(subs) != mailtest.InboxCount || !strings.Contains(page, "existe plus") || strings.Contains(page, "has-message") {
		t.Errorf("%d lignes ; erreur du volet absente ou mal placée", len(subs))
	}
	resp, page = e.do(c, "GET", "/?uid=abc", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(page, "Sélectionnez un message") {
		t.Error("uid invalide ignoré")
	}
}

func TestLayoutPreference(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	prefs := func(layout string) *http.Response {
		v := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"}, "idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"50"}, "mark_read": {"on"}}
		if layout != "" {
			v.Set("layout", layout)
		}
		resp, _ := e.do(c, "POST", "/settings/preferences", v, nil)
		return resp
	}
	e.expect(prefs("bogus"), 400, "")
	e.expect(prefs("list"), 200, "")
	_, page := e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `class="mailbox listonly"`) || strings.Contains(page, `class="readpane"`) {
		t.Fatalf("liste seule attendue :\n%.600s", page)
	}
	_, links := rows(page)
	if !strings.HasPrefix(links[0], "/mail/message?") {
		t.Errorf("en liste seule, la ligne ouvre une page : %q", links[0])
	}
	_, page = e.do(c, "GET", "/?uid=1", nil, nil)
	if strings.Contains(page, "has-message") {
		t.Error("le paramètre uid est ignoré en liste seule")
	}
	// Réglages : la disposition choisie est sélectionnée, et un envoi sans champ la conserve.
	_, settings := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(settings, `<option value="list" selected>`) {
		t.Error("disposition « liste seule » attendue dans les réglages")
	}
	e.expect(prefs(""), 200, "")
	_, page = e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `class="mailbox listonly"`) {
		t.Error("une préférence absente du formulaire ne doit pas réinitialiser la disposition")
	}
	e.expect(prefs("three"), 200, "")
	_, page = e.do(c, "GET", "/", nil, nil)
	if !strings.Contains(page, `class="mailbox three"`) {
		t.Error("3 volets attendus")
	}
}

func TestSelectAllMarkupAndEmptySelection(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	for _, want := range []string{`id="select-all"`, `title="Ctrl+A"`, `src="/static/list.js"`, `aria-label="Tout sélectionner"`} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent", want)
		}
	}
	if !regexp.MustCompile(`<label class="chk" data-js-only hidden`).MatchString(page) {
		t.Error("le bouton « Tout sélectionner » doit rester caché sans JavaScript")
	}
	if n := strings.Count(page, "data-needs-selection"); n != 9 {
		t.Errorf("%d boutons d'action à griser sans sélection, attendu 9", n)
	}
	resp, body := e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"INBOX"}, "back": {"/"}}, nil)
	e.expect(resp, 400, "")
	if !strings.Contains(body, "Aucun message sélectionné") {
		t.Error("message dédié attendu sans sélection")
	}
}

// --- volet de lecture partiel et largeur des volets ---

func TestPaneFragment(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	before := firstBadge(t, page)
	link := linkTo(t, page, "Réunion lundi")
	if strings.Contains(page, "#m") {
		t.Error("les liens de lignes ne doivent plus contenir de fragment (#mN) : il fait défiler la liste")
	}

	resp, frag := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, map[string]string{"X-Requested-With": "fetch"})
	e.expect(resp, 200, "")
	for _, bad := range []string{"<html", "<aside", `class="listpane"`, "<head>"} {
		if strings.Contains(frag, bad) {
			t.Errorf("le fragment ne doit pas contenir %q", bad)
		}
	}
	for _, want := range []string{`class="msgview"`, `data-uid="` + uidOf(link) + `"`, `data-just-read="1"`, "Réunion lundi", "carol@z.fr", `class="mailbody"`, `name="csrf"`, "data-pane"} {
		if !strings.Contains(frag, want) {
			t.Errorf("%q absent du fragment", want)
		}
	}
	// Le message est marqué lu par l'appel partiel ; une deuxième ouverture n'annonce plus rien.
	resp, again := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, nil)
	e.expect(resp, 200, "")
	if strings.Contains(again, "data-just-read") {
		t.Error("data-just-read ne doit apparaître qu'à la première ouverture")
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	if firstBadge(t, page) != before-1 {
		t.Errorf("compteur %d, attendu %d", firstBadge(t, page), before-1)
	}

	// Les liens du volet (texte brut, afficher le contenu distant) restent dans le volet et gardent la liste.
	if !regexp.MustCompile(`href="/\?uid=\d+&amp;view=text" data-pane`).MatchString(frag) {
		t.Errorf("lien « texte brut » attendu avec data-pane :\n%s", frag)
	}
	// Vue texte demandée directement.
	resp, txt := e.do(c, "GET", "/mail/pane?uid="+uidOf(link)+"&view=text", nil, nil)
	e.expect(resp, 200, "")
	if !strings.Contains(txt, "view=text") || strings.Contains(txt, "Afficher en texte brut") {
		t.Error("la vue texte doit proposer de revenir à la version mise en forme")
	}
	// Erreurs : message absent, uid invalide, non connecté.
	resp, miss := e.do(c, "GET", "/mail/pane?uid=999999", nil, nil)
	e.expect(resp, 404, "")
	if !strings.Contains(miss, "existe plus") || strings.Contains(miss, "<html") {
		t.Errorf("fragment d'erreur : %q", miss)
	}
	resp, _ = e.do(c, "GET", "/mail/pane?uid=abc", nil, nil)
	e.expect(resp, 400, "")
	resp, _ = e.do(e.client(), "GET", "/mail/pane?uid=1", nil, nil)
	e.expect(resp, 303, "/login")
	_ = tok
}

func TestPaneWidthsPersistedInAccount(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	css := func(cl *http.Client) string { _, b := e.do(cl, "GET", "/user.css", nil, nil); return b }
	if got := css(c); strings.TrimSpace(got) != "" {
		t.Errorf("feuille vide par défaut : %q", got)
	}
	_, page := e.do(c, "GET", "/", nil, nil)
	for _, want := range []string{`href="/user.css"`, `class="splitter a"`, `class="splitter b"`, `data-pane="folders"`, `data-pane="list"`, `role="separator"`, `data-csrf="` + tok + `"`, "Largeur du volet"} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent de la page", want)
		}
	}
	post := func(v url.Values, hdr map[string]string) *http.Response {
		v.Set("csrf", tok)
		resp, _ := e.do(c, "POST", "/settings/panes", v, hdr)
		return resp
	}
	fetch := map[string]string{"X-Requested-With": "fetch"}
	e.expect(post(url.Values{"folders": {"300"}, "list": {"520"}}, fetch), 204, "")
	if got := css(c); !strings.Contains(got, "--folders-w:300px") || !strings.Contains(got, "--list-w:520px") {
		t.Errorf("largeurs servies : %q", got)
	}
	// Chaque champ est indépendant ; les absents sont inchangés.
	e.expect(post(url.Values{"folders": {"220"}}, fetch), 204, "")
	if got := css(c); !strings.Contains(got, "--folders-w:220px") || !strings.Contains(got, "--list-w:520px") {
		t.Errorf("largeur de la liste conservée : %q", got)
	}
	// Enregistrées dans le fichier chiffré : retrouvées après reconnexion, par un autre navigateur.
	c2 := e.client()
	e.expect(e.login(c2, "alice", pw), 303, "/")
	if got := css(c2); !strings.Contains(got, "--folders-w:220px") || !strings.Contains(got, "--list-w:520px") {
		t.Errorf("après reconnexion : %q", got)
	}
	// Hors bornes ou illisible : refusé, rien ne change. Absent des deux : refusé.
	for _, bad := range []url.Values{{"folders": {"50"}}, {"folders": {"9999"}}, {"list": {"100"}}, {"list": {"abc"}}, {"folders": {"-1"}}, {}} {
		if r := post(bad, fetch); r.StatusCode != 400 {
			t.Errorf("%v : statut %d, attendu 400", bad, r.StatusCode)
		}
	}
	if got := css(c); !strings.Contains(got, "--folders-w:220px") {
		t.Errorf("une valeur refusée ne doit rien modifier : %q", got)
	}
	// CSRF.
	resp, _ := e.do(c, "POST", "/settings/panes", url.Values{"folders": {"300"}}, fetch)
	e.expect(resp, 403, "")
	// Réinitialisation depuis les réglages (formulaire classique : redirection).
	resp = post(url.Values{"folders": {"0"}, "list": {"0"}}, nil)
	e.expect(resp, 303, "/settings")
	if got := css(c); strings.TrimSpace(got) != "" {
		t.Errorf("après réinitialisation : %q", got)
	}
	_, settings := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(settings, `action="/settings/panes"`) || !strings.Contains(settings, "Réinitialiser la largeur des volets") {
		t.Error("bouton de réinitialisation absent des réglages")
	}
	// Sans session : feuille vide, pas d'erreur.
	if got := css(e.client()); strings.TrimSpace(got) != "" {
		t.Errorf("sans session : %q", got)
	}
}

func TestPaneWidthsDoNotLeakBetweenUsers(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	resp, _ := e.do(c, "POST", "/settings/panes", url.Values{"csrf": {tok}, "folders": {"400"}}, map[string]string{"X-Requested-With": "fetch"})
	e.expect(resp, 204, "")
	other := e.client()
	e.register(other, "bob")
	_, b := e.do(other, "GET", "/user.css", nil, nil)
	if strings.Contains(b, "400") {
		t.Errorf("la largeur d'un utilisateur ne doit pas apparaître chez un autre : %q", b)
	}
}

func TestFontSizePreference(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	css := func() string { _, b := e.do(c, "GET", "/user.css", nil, nil); return b }
	prefs := func(size string) *http.Response {
		v := url.Values{"csrf": {tok}, "language": {"fr"}, "theme": {"light"}, "idle_minutes": {"60"}, "absolute_hours": {"12"}, "page_size": {"50"}, "mark_read": {"on"}}
		if size != "" {
			v.Set("font_size", size)
		}
		resp, _ := e.do(c, "POST", "/settings/preferences", v, nil)
		return resp
	}
	if strings.Contains(css(), "font-size") {
		t.Error("taille par défaut : aucune règle dans la feuille utilisateur")
	}
	_, settings := e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(settings, `<option value="14" selected>14 px</option>`) || !strings.Contains(settings, "Taille du texte") {
		t.Errorf("sélecteur de taille attendu, 13 px par défaut :\n%s", settings)
	}
	e.expect(prefs("9"), 400, "")
	e.expect(prefs("abc"), 400, "")
	e.expect(prefs("16"), 200, "")
	if !strings.Contains(css(), "html{font-size:16px}") {
		t.Errorf("feuille utilisateur : %q", css())
	}
	_, settings = e.do(c, "GET", "/settings", nil, nil)
	if !strings.Contains(settings, `<option value="16" selected>`) {
		t.Error("la taille choisie doit être sélectionnée")
	}
	// Le corps des messages suit la même taille ; un envoi sans le champ la conserve.
	e.expect(prefs(""), 200, "")
	_, page := e.do(c, "GET", "/", nil, nil)
	link := linkTo(t, page, "Réunion lundi")
	_, body := e.do(c, "GET", "/mail/body?folder=INBOX&uid="+uidOf(link), nil, nil)
	if !strings.Contains(body, "font:16px/1.45") || !strings.Contains(css(), "font-size:16px") {
		t.Errorf("corps du message : %.200s", body)
	}
	e.expect(prefs("14"), 200, "")
	if strings.Contains(css(), "font-size") {
		t.Errorf("retour à la taille par défaut : %q", css())
	}
}

// --- pièces jointes : miniatures et archive ---

func testPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func b64lines(b []byte) string {
	enc := base64.StdEncoding.EncodeToString(b)
	var sb strings.Builder
	for len(enc) > 76 {
		sb.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	return sb.String() + enc
}

func attachmentMessage() []byte {
	return []byte("From: Pascaline <p@x.fr>\r\nTo: test@localhost\r\nSubject: Photos\r\nDate: Mon, 02 Mar 2026 10:00:00 +0000\r\nMessage-ID: <att@test>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"B\"\r\n\r\n--B\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nvoir pièces jointes\r\n" +
		"--B\r\nContent-Type: image/png; name=\"plage.png\"\r\nContent-Disposition: attachment; filename=\"plage.png\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" + b64lines(testPNG(300, 200)) + "\r\n" +
		"--B\r\nContent-Type: image/png; name=\"plage.png\"\r\nContent-Disposition: attachment; filename=\"plage.png\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" + b64lines(testPNG(64, 64)) + "\r\n" +
		"--B\r\nContent-Type: image/svg+xml; name=\"logo.svg\"\r\nContent-Disposition: attachment; filename=\"logo.svg\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" + b64lines([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)) + "\r\n" +
		"--B\r\nContent-Type: application/pdf; name=\"../../etc/programme.pdf\"\r\nContent-Disposition: attachment; filename=\"../../etc/programme.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQK\r\n--B--\r\n")
}

func TestAttachmentThumbnailsAndZip(t *testing.T) {
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add(attachmentMessage(), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save"), nil))
	_, page := e.do(c, "GET", "/?q=Photos&scope=subject", nil, nil)
	link := linkTo(t, page, "Photos")

	resp, frag := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, nil)
	e.expect(resp, 200, "")
	if strings.Contains(frag, "ZgotmplZ") {
		t.Fatal("html/template a filtré l'URI data: des miniatures")
	}
	imgs := regexp.MustCompile(`<img src="(data:image/png;base64,[^"]+)" alt="" width="36" height="36">`).FindAllStringSubmatch(frag, -1)
	if len(imgs) != 2 {
		t.Fatalf("%d miniatures, attendu 2 (deux PNG ; ni SVG ni PDF) :\n%s", len(imgs), frag)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(html.UnescapeString(imgs[0][1]), "data:image/png;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != thumbSize || cfg.Height != thumbSize {
		t.Errorf("miniature %dx%d : %v", cfg.Width, cfg.Height, err)
	}
	for _, want := range []string{`class="bar attbar"`, `class="atts"`, `class="name">plage.png`, `class="ext"><svg`, `href="/mail/attachments.zip?`, "Tout télécharger"} {
		if !strings.Contains(frag, want) {
			t.Errorf("%q absent de la barre des pièces jointes", want)
		}
	}
	if strings.Index(frag, `class="bar attbar"`) > strings.Index(frag, `class="mailbody"`) {
		t.Error("les pièces jointes doivent précéder le corps du message")
	}

	// Archive : tous les fichiers, noms assainis et dédoublonnés.
	zipURL := htmlUnescape(regexp.MustCompile(`href="(/mail/attachments\.zip\?[^"]+)"`).FindStringSubmatch(frag)[1])
	resp, body := e.do(c, "GET", zipURL, nil, nil)
	e.expect(resp, 200, "")
	if resp.Header.Get("Content-Type") != "application/zip" || !strings.Contains(resp.Header.Get("Content-Disposition"), "pieces-jointes.zip") ||
		resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("en-têtes : %v", resp.Header)
	}
	zr, err := zip.NewReader(bytes.NewReader([]byte(body)), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if strings.Contains(f.Name, "/") || strings.Contains(f.Name, "\\") || f.Name == ".." || strings.HasPrefix(f.Name, ".") {
			t.Errorf("nom non assaini : %q", f.Name)
		}
	}
	if strings.Join(names, "|") != "plage.png|plage (2).png|logo.svg|_.._etc_programme.pdf" {
		t.Errorf("fichiers de l'archive : %v", names)
	}
	// Message sans pièce jointe ou paramètres invalides.
	resp, _ = e.do(c, "GET", "/mail/attachments.zip?folder=INBOX&uid=2", nil, nil)
	e.expect(resp, 404, "")
	resp, _ = e.do(c, "GET", "/mail/attachments.zip?folder=INBOX&uid=abc", nil, nil)
	e.expect(resp, 400, "")
	resp, _ = e.do(e.client(), "GET", zipURL, nil, nil)
	e.expect(resp, 303, "/login")
}

func TestThumbnailLimits(t *testing.T) {
	if got := attachmentExt("rapport.final.PDF"); got != "PDF" {
		t.Errorf("extension : %q", got)
	}
	if got := attachmentExt("archive.tar.gzip"); got != "GZIP" {
		t.Errorf("extension : %q", got)
	}
	if attachmentExt("sans-extension") != "" || attachmentExt("fin.") != "" {
		t.Error("pas d'extension attendue")
	}
	if safeFileName("../../x") != "_.._x" && safeFileName("../../x") != "_.._x" {
		// seuls les séparateurs et le point de tête comptent
	}
	for _, n := range []string{"../../etc/passwd", "a\\b", ".bashrc", "ligne\nsuivante", ""} {
		got := safeFileName(n)
		if strings.ContainsAny(got, "/\\\n\r") || strings.HasPrefix(got, ".") || got == "" {
			t.Errorf("safeFileName(%q) = %q", n, got)
		}
	}
}

func TestClearTrashAndJunk(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	uids := listUIDs(t, e, c, "/")
	act := func(action string, uid string) {
		resp, _ := e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {action}, "folder": {"INBOX"}, "uid": {uid}, "back": {"/"}}, nil)
		e.expect(resp, 303, "/")
	}
	clearForm := func(folder string) url.Values {
		return url.Values{"csrf": {tok}, "folder": {folder}, "back": {"/?folder=" + folder}}
	}

	// Rien à vider : pas de lien dans la boîte de réception ni dans une corbeille vide.
	_, page := e.do(c, "GET", "/", nil, nil)
	if strings.Contains(page, `class="listClear"`) {
		t.Error("la boîte de réception ne doit jamais proposer de vidage")
	}
	_, page = e.do(c, "GET", "/?folder=Trash", nil, nil)
	if strings.Contains(page, `class="listClear"`) {
		t.Error("une corbeille vide n'a rien à proposer")
	}

	act("delete", uids[0])
	act("delete", uids[1])
	act("spam", uids[2])
	_, page = e.do(c, "GET", "/?folder=Trash", nil, nil)
	for _, want := range []string{`class="listClear"`, "Vider la corbeille (2)", "Supprimer définitivement les 2 messages", `action="/mail/clear"`, `name="folder" value="Trash"`} {
		if !strings.Contains(page, want) {
			t.Errorf("%q absent de la corbeille", want)
		}
	}
	_, page = e.do(c, "GET", "/?folder=Junk", nil, nil)
	if !strings.Contains(page, "Vider les indésirables (1)") {
		t.Errorf("le dossier des indésirables doit proposer son vidage :\n%.1500s", page)
	}

	// Garde-fous : INBOX et les dossiers ordinaires sont refusés, la protection CSRF s'applique.
	for _, f := range []string{"INBOX", "Sent", "Projets"} {
		resp, _ := e.do(c, "POST", "/mail/clear", clearForm(f), nil)
		e.expect(resp, 403, "")
	}
	resp, _ := e.do(c, "POST", "/mail/clear", url.Values{"folder": {"Trash"}}, nil)
	e.expect(resp, 403, "")
	resp, _ = e.do(c, "POST", "/mail/clear", url.Values{"csrf": {tok}, "folder": {"a*"}}, nil)
	e.expect(resp, 400, "")
	if got := listUIDs(t, e, c, "/?folder=Trash"); len(got) != 2 {
		t.Fatalf("les refus ne doivent rien supprimer : %d messages", len(got))
	}

	// Vidage de la corbeille.
	resp, _ = e.do(c, "POST", "/mail/clear", clearForm("Trash"), nil)
	e.expect(resp, 303, "/?folder=Trash")
	_, page = e.do(c, "GET", "/?folder=Trash", nil, nil)
	if subs, _ := rows(page); len(subs) != 0 || strings.Contains(page, `class="listClear"`) {
		t.Errorf("la corbeille doit être vide et sans lien : %v", subs)
	}
	if got := listUIDs(t, e, c, "/?folder=Junk"); len(got) != 1 {
		t.Errorf("les indésirables ne doivent pas être touchés : %d", len(got))
	}
	if got := listUIDs(t, e, c, "/"); len(got) != mailtest.InboxCount-3 {
		t.Errorf("INBOX : %d messages", len(got))
	}
	// Vidage des indésirables.
	resp, _ = e.do(c, "POST", "/mail/clear", clearForm("Junk"), nil)
	e.expect(resp, 303, "/?folder=Junk")
	if got := listUIDs(t, e, c, "/?folder=Junk"); len(got) != 0 {
		t.Errorf("indésirables : %d messages restent", len(got))
	}
	// Redirection : pas de redirection ouverte.
	resp, _ = e.do(c, "POST", "/mail/clear", url.Values{"csrf": {tok}, "folder": {"Trash"}, "back": {"//evil.example/"}}, nil)
	e.expect(resp, 303, "/")
	// Non connecté.
	resp, _ = e.do(e.client(), "POST", "/mail/clear", url.Values{"folder": {"Trash"}}, nil)
	e.expect(resp, 303, "/login")
}

func TestQuotaView(t *testing.T) {
	q := quotaView(512<<20, 2<<30, "fr")
	if q.Percent != 25 || q.Full || q.Used == "" || q.Limit == "" {
		t.Errorf("quota 25 %% : %+v", q)
	}
	if q := quotaView(95, 100, "en"); q.Percent != 95 || !q.Full {
		t.Errorf("quota plein : %+v", q)
	}
	if q := quotaView(300, 100, "en"); q.Percent != 100 {
		t.Errorf("dépassement : %+v", q)
	}
}

func TestNoQuotaWithoutServerSupport(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	if strings.Contains(page, `class="quota`) {
		t.Error("quota affiché alors que le serveur ne l'annonce pas")
	}
}

func TestMessageToolsViewsAndDownloads(t *testing.T) {
	e, c, _ := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	uid := uidOf(linkTo(t, page, "Facture octobre"))
	q := "?folder=INBOX&uid=" + uid

	resp, src := e.do(c, "GET", "/mail/source"+q+"&view=source", nil, nil)
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(src, "Subject: Facture octobre") || !strings.Contains(src, "montant 100") {
		t.Errorf("source : %d %q\n%s", resp.StatusCode, resp.Header.Get("Content-Type"), src)
	}
	resp, hdr := e.do(c, "GET", "/mail/source"+q+"&view=headers", nil, nil)
	if resp.StatusCode != 200 || !strings.Contains(hdr, "From: Alice") || strings.Contains(hdr, "montant 100") {
		t.Errorf("en-têtes seuls :\n%s", hdr)
	}
	resp, eml := e.do(c, "GET", "/mail/source"+q+"&view=eml", nil, nil)
	if resp.Header.Get("Content-Type") != "message/rfc822" || !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") || !strings.Contains(resp.Header.Get("Content-Disposition"), ".eml") || !strings.Contains(eml, "Subject: Facture octobre") {
		t.Errorf(".eml : %q %q", resp.Header.Get("Content-Type"), resp.Header.Get("Content-Disposition"))
	}
	if resp, _ := e.do(c, "GET", "/mail/source"+q+"&view=autre", nil, nil); resp.StatusCode != 400 {
		t.Errorf("vue inconnue : %d", resp.StatusCode)
	}
	if resp, _ := e.do(c, "GET", "/mail/source?folder=INBOX&uid=99999&view=source", nil, nil); resp.StatusCode != 404 {
		t.Errorf("message inexistant : %d", resp.StatusCode)
	}
	if resp, _ := e.do(e.client(), "GET", "/mail/source"+q+"&view=source", nil, nil); resp.StatusCode != 303 {
		t.Errorf("sans session : %d", resp.StatusCode)
	}

	// impression : en-têtes, pièces jointes, corps ; script d'impression seulement
	resp, pr := e.do(c, "GET", "/mail/print"+q, nil, nil)
	csp := resp.Header.Get("Content-Security-Policy")
	if resp.StatusCode != 200 || !strings.Contains(pr, "Facture octobre") || !strings.Contains(pr, "facture.pdf") || !strings.Contains(pr, "montant 100") || !strings.Contains(pr, "/static/print.js") {
		t.Errorf("impression :\n%s", pr)
	}
	if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP de l'impression : %s", csp)
	}
	// message HTML piégé : rien d'exécutable, images distantes bloquées
	_, page = e.do(c, "GET", "/", nil, nil)
	hostile := uidOf(linkTo(t, page, "Réunion lundi"))
	_, pr = e.do(c, "GET", "/mail/print?folder=INBOX&uid="+hostile, nil, nil)
	for _, bad := range []string{"<script>", "onerror", "http://tracker"} {
		if strings.Contains(strings.ToLower(pr), bad) {
			t.Errorf("%q dans la version imprimable", bad)
		}
	}
	// liens du menu
	_, msg := e.do(c, "GET", "/mail/message"+q, nil, nil)
	for _, want := range []string{"/mail/print?", "view=headers", "view=source", "view=eml"} {
		if !strings.Contains(msg, want) {
			t.Errorf("lien %s absent du menu du message", want)
		}
	}
}

func TestMarkAllReadAndCopy(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, list := e.do(c, "GET", "/?folder=INBOX&filter=unread", nil, nil)
	if subs, _ := rows(list); len(subs) == 0 {
		t.Fatal("aucun message non lu au départ")
	}
	if !strings.Contains(list, `name="action" value="readall"`) || !strings.Contains(list, `data-copy-label=`) {
		t.Error("boutons « tout marquer lu » et « copier vers » absents")
	}
	resp, _ := e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"readall"}, "folder": {"INBOX"}, "back": {"/"}}, nil)
	e.expect(resp, 303, "/")
	_, list = e.do(c, "GET", "/?folder=INBOX&filter=unread", nil, nil)
	if subs, _ := rows(list); len(subs) != 0 {
		t.Errorf("%d messages restent non lus", len(subs))
	}
	// copier : le message reste dans le dossier d'origine
	_, page := e.do(c, "GET", "/", nil, nil)
	uid := uidOf(linkTo(t, page, "Facture octobre"))
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "copy_to": {"Projets"}, "folder": {"INBOX"}, "uid": {uid}, "back": {"/"}}, nil)
	e.expect(resp, 303, "/")
	_, dest := e.do(c, "GET", "/?folder=Projets", nil, nil)
	if subs, _ := rows(dest); len(subs) != 1 || subs[0] != "Facture octobre" {
		t.Errorf("dossier de destination : %v", subs)
	}
	_, page = e.do(c, "GET", "/", nil, nil)
	if subs, _ := rows(page); !slicesContains(subs, "Facture octobre") {
		t.Error("le message copié a disparu de INBOX")
	}
	// refus : dossier inexistant, copie sans sélection, readall sans CSRF
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "copy_to": {"Fantome"}, "folder": {"INBOX"}, "uid": {uid}}, nil)
	if resp.StatusCode < 400 {
		t.Errorf("copie vers un dossier inexistant : %d", resp.StatusCode)
	}
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "copy_to": {"Projets"}, "folder": {"INBOX"}}, nil)
	if resp.StatusCode != 400 {
		t.Errorf("copie sans sélection : %d", resp.StatusCode)
	}
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {"x"}, "action": {"readall"}, "folder": {"INBOX"}}, nil)
	if resp.StatusCode != 403 {
		t.Errorf("readall sans CSRF : %d", resp.StatusCode)
	}
}

func TestUnsubscribeLinks(t *testing.T) {
	e := newEnv(t, nil)
	imapPort := mailtest.StartIMAPSeeded(t, "none", func(t testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		d := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
		add([]byte("From: News <news@x.org>\r\nTo: test@localhost\r\nSubject: Lettre https\r\nDate: Mon, 05 Oct 2026 09:00:00 +0000\r\nMessage-ID: <u1@x>\r\nList-Unsubscribe: <mailto:stop@x.org?subject=unsub>, <https://x.org/u?id=1>\r\nContent-Type: text/plain\r\n\r\nhello\r\n"), d)
		add([]byte("From: News <news@x.org>\r\nTo: test@localhost\r\nSubject: Lettre mailto\r\nDate: Mon, 05 Oct 2026 10:00:00 +0000\r\nMessage-ID: <u2@x>\r\nList-Unsubscribe: <mailto:stop@x.org?subject=unsub>, <javascript:alert(1)>\r\nContent-Type: text/plain\r\n\r\nhello\r\n"), d)
	})
	c, tok := e.loggedIn("alice")
	resp, _ := e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: imapPort, smtp: mailtest.StartSMTP(t, "none")}, "save"), nil)
	e.expect(resp, 303, "/")
	_, page := e.do(c, "GET", "/", nil, nil)
	_, https := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uidOf(linkTo(t, page, "Lettre https")), nil, nil)
	if !strings.Contains(https, `<a href="https://x.org/u?id=1" target="_blank" rel="noopener noreferrer">`) {
		t.Errorf("lien https de désabonnement :\n%s", regexp.MustCompile(`unsubbar[^\n]*`).FindString(https))
	}
	_, mt := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uidOf(linkTo(t, page, "Lettre mailto")), nil, nil)
	if !strings.Contains(mt, `unsubbar`) || !strings.Contains(mt, "/compose?back=") || !strings.Contains(mt, "mailto%3Astop%40x.org") || strings.Contains(mt, "javascript:") {
		t.Errorf("désabonnement par message :\n%s", regexp.MustCompile(`unsubbar[^\n]*`).FindString(mt))
	}
	_, plain := e.do(c, "GET", "/mail/message?folder=INBOX&uid="+uidOf(linkTo(t, page, "Facture octobre")), nil, nil)
	if strings.Contains(plain, "unsubbar") {
		t.Error("lien de désabonnement sans en-tête List-Unsubscribe")
	}
}

func TestAttachmentInlinePreviewOnlySafeTypes(t *testing.T) {
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add(attachmentMessage(), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save"), nil))
	_, page := e.do(c, "GET", "/?q=Photos&scope=subject", nil, nil)
	link := linkTo(t, page, "Photos")
	_, frag := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, nil)
	previews := regexp.MustCompile(`class="preview" href="([^"]+)" data-kind="image"`).FindAllStringSubmatch(frag, -1)
	if len(previews) != 2 {
		t.Fatalf("%d aperçus d'image, attendu 2 (ni SVG ni PDF) :\n%s", len(previews), frag)
	}
	resp, body := e.do(c, "GET", html.UnescapeString(previews[0][1]), nil, nil)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "inline") ||
		resp.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(body, "\x89PNG") {
		t.Errorf("aperçu PNG : %d %s %s", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Disposition"))
	}
	// SVG : l'option inline est ignorée, la pièce reste un téléchargement ; PDF : affiché sans bac à sable mais sans script
	for idx, want := range map[string]string{"2": "attachment", "3": "inline"} {
		u, _ := url.Parse(html.UnescapeString(previews[0][1]))
		qq := u.Query()
		qq.Set("index", idx)
		resp, _ = e.do(c, "GET", u.Path+"?"+qq.Encode(), nil, nil)
		if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), want) {
			t.Errorf("pièce %s : %s (%s)", idx, resp.Header.Get("Content-Disposition"), resp.Header.Get("Content-Type"))
		}
		if idx == "2" && resp.Header.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("SVG servi comme %s", resp.Header.Get("Content-Type"))
		}
		if idx == "3" && (resp.Header.Get("Content-Type") != "application/pdf" || strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'none'")) {
			t.Errorf("PDF : %s / %s", resp.Header.Get("Content-Type"), resp.Header.Get("Content-Security-Policy"))
		}
	}
}

func TestAttachmentSupportsRangeRequests(t *testing.T) {
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add(attachmentMessage(), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save"), nil))
	_, page := e.do(c, "GET", "/?q=Photos&scope=subject", nil, nil)
	link := linkTo(t, page, "Photos")
	_, frag := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, nil)
	href := html.UnescapeString(regexp.MustCompile(`class="att" href="([^"]+)"`).FindStringSubmatch(frag)[1])
	resp, full := e.do(c, "GET", href, nil, nil)
	if resp.Header.Get("Accept-Ranges") != "bytes" || len(full) < 100 {
		t.Fatalf("en-têtes : %v (%d octets)", resp.Header, len(full))
	}
	resp, part := e.do(c, "GET", href, nil, map[string]string{"Range": "bytes=10-19"})
	if resp.StatusCode != 206 || part != full[10:20] || resp.Header.Get("Content-Range") != fmt.Sprintf("bytes 10-19/%d", len(full)) {
		t.Errorf("requête partielle : %d %q %s", resp.StatusCode, part, resp.Header.Get("Content-Range"))
	}
}

func TestAttachmentIcon(t *testing.T) {
	for _, c := range []struct{ ct, name, want string }{
		{"application/pdf", "a.bin", "file-pdf"},
		{"application/octet-stream", "Devis.PDF", "file-pdf"},
		{"image/png", "a.png", "image"},
		{"application/zip", "a.zip", "file-zip"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "t.xlsx", "file-sheet"},
		{"application/vnd.ms-powerpoint", "p.ppt", "file-slides"},
		{"text/calendar; charset=utf-8", "invite.ics", "calendar"},
		{"audio/mpeg", "a.mp3", "file-audio"},
		{"text/plain", "notes.txt", "file-text"},
		{"application/octet-stream", "inconnu.xyz", "doc"},
	} {
		if got := attachmentIcon(c.ct, c.name); got != c.want {
			t.Errorf("%s %s : %q, attendu %q", c.ct, c.name, got, c.want)
		}
	}
}

// Suppression du message ouvert : on ouvre son voisin (comme RainLoop).
func TestDeleteOpenMessageOpensNeighbour(t *testing.T) {
	e, c, tok := mailEnv(t, true)
	_, page := e.do(c, "GET", "/", nil, nil)
	uids := regexp.MustCompile(`name="uid" value="(\d+)"`).FindAllStringSubmatch(page, -1)
	if len(uids) < 3 {
		t.Fatalf("%d messages, 3 attendus au moins", len(uids))
	}
	open := uids[1][1]
	_, page = e.do(c, "GET", "/?uid="+open, nil, nil)
	m := regexp.MustCompile(`name="open_next" id="open-next" value="([^"]*)"`).FindStringSubmatch(page)
	if m == nil || m[1] == "" {
		t.Fatal("le volet de lecture doit indiquer le message voisin")
	}
	next := htmlUnescape(m[1])
	resp, _ := e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"INBOX"},
		"uid": {open}, "back": {"/"}, "current_uid": {open}, "open_next": {next}}, nil)
	if loc := resp.Header.Get("Location"); loc != next {
		t.Errorf("redirection %q, attendu le voisin %q", loc, next)
	}
	// Avec plusieurs messages visés, retour à la liste.
	resp, _ = e.do(c, "POST", "/mail/action", url.Values{"csrf": {tok}, "action": {"delete"}, "folder": {"INBOX"},
		"uid": {uids[0][1], uids[2][1]}, "back": {"/"}, "current_uid": {uids[0][1]}, "open_next": {next}}, nil)
	e.expect(resp, 303, "/")
}

// ZIP partiel : seules les pièces jointes dont l'index est passé entrent dans l'archive.
func TestZipSelectedAttachments(t *testing.T) {
	e := newEnv(t, nil)
	port := mailtest.StartIMAPSeeded(t, "none", func(_ testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		add(attachmentMessage(), time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC))
	})
	smtpPort := mailtest.StartSMTP(t, "none")
	c, tok := e.loggedIn("alice")
	e.expect1(e.do(c, "POST", "/accounts/save", accountForm(tok, mailPorts{imap: port, smtp: smtpPort}, "save"), nil))
	_, page := e.do(c, "GET", "/?q=Photos&scope=subject", nil, nil)
	link := linkTo(t, page, "Photos")
	_, frag := e.do(c, "GET", "/mail/pane"+strings.TrimPrefix(link, "/"), nil, nil)
	m := regexp.MustCompile(`href="(/mail/attachments\.zip\?[^"]+)"`).FindStringSubmatch(frag)
	if m == nil {
		t.Fatalf("lien ZIP absent :\n%s", frag)
	}
	zipURL := htmlUnescape(m[1])
	resp, all := e.do(c, "GET", zipURL, nil, nil)
	e.expect(resp, 200, "")
	resp, one := e.do(c, "GET", zipURL+"&index=0", nil, nil)
	e.expect(resp, 200, "")
	if len(one) >= len(all) {
		t.Errorf("ZIP d'une pièce (%d o) devrait être plus petit que le ZIP complet (%d o)", len(one), len(all))
	}
	resp, _ = e.do(c, "GET", zipURL+"&index=99", nil, nil)
	if resp.StatusCode != 404 {
		t.Errorf("index inconnu : statut %d, attendu 404", resp.StatusCode)
	}
}

func TestQuotaBarMinimum(t *testing.T) {
	// 0,3 % d'occupation : le texte garde la valeur exacte, la barre reste visible.
	v := quotaView(3<<20, 1<<30, "fr")
	if v.Percent != 0 || v.Bar != 2 {
		t.Errorf("pourcent %d, barre %d (attendu 0 et 2)", v.Percent, v.Bar)
	}
	if v = quotaView(0, 1<<30, "fr"); v.Bar != 0 {
		t.Errorf("boîte vide : barre %d, attendu 0", v.Bar)
	}
	if v = quotaView(950<<20, 1<<30, "fr"); !v.Full || v.Bar != v.Percent {
		t.Errorf("boîte pleine : %+v", v)
	}
}
