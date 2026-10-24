package web

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/ical"
	"wmail-go/internal/mailconn"
	"wmail-go/internal/mailview"
	"wmail-go/internal/session"
	"wmail-go/internal/store"
	"wmail-go/internal/userdata"
)

const (
	maxMessageBytes = 30 << 20
	// maxAttachmentBytes : taille maximale d'une pièce jointe téléchargée seule (le message entier n'est pas lu).
	maxAttachmentBytes = 200 << 20
	maxActionUIDs      = 500
	mailTimeout        = 30 * time.Second
	maxPageSize        = 500
	maxThumbnails      = 12      // miniatures calculées par message
	maxThumbSource     = 8 << 20 // taille maximale d'une image décodée pour sa miniature
	thumbSize          = 72      // côté des miniatures (affichées à 36 px, nettes sur écrans à forte densité)
)

// pageSizes sont les tailles de page proposées dans la liste (50 par défaut).
var pageSizes = []int{10, 20, 30, 50, 100, 200, 500}

// MessageRow est une ligne de la liste des messages.
type MessageRow struct {
	Answered, Forwarded          bool
	AttachKind                   string // genre commun des pièces jointes (voir mailconn.Summary)
	Important                    bool   // priorité haute (en-têtes Importance, X-Priority)
	SubjectPrefix, SubjectRest   string // « Re: » / « Fwd: » séparés du reste de l'objet
	SubUnread                    bool   // conversation dont seul un message plus ancien est non lu
	ThreadCount                  int    // messages de la conversation (0 ou 1 : message isolé)
	ThreadUIDs, ThreadURL        string // membres (séparés par des virgules) et lien vers la conversation
	Selected                     bool
	UID                          uint32
	Subject, From, Date, Size    string
	Seen, Flagged, HasAttachment bool
	ReadURL                      string
}

// ListView regroupe la liste des messages d'un dossier.
type ListView struct {
	ClearKind                      string // trash ou junk : le dossier affiché peut être vidé
	ClearCount                     int
	PageLinks                      []PageLink
	Stay                           string // adresse où revenir après une action qui laisse le message ouvert
	TotalLabel                     string
	Folder                         string
	Rows                           []MessageRow
	Total, Page, Pages             int
	Query, Scope                   string
	Filter                         string // all ou unread
	PageSize                       int
	PageSizes                      []int
	ThreePane                      bool
	BottomPane                     bool // disposition « aperçu en bas »
	Selected                       uint32
	PaneError                      string // clé de message : le message demandé n'a pas pu être chargé
	FilterAllURL                   string
	FilterUnreadURL                string
	ClearSearchURL                 string
	UnreadCount                    int
	Sort                           string
	SortURLs                       map[string]string
	SortMark                       map[string]string
	PrevURL, NextURL               string
	Back                           string // URL de la liste, pour revenir après une action
	MoveTargets                    []string
	SortFallback                   bool
	Error                          *TestResult
	NotSpam, HideSpam, HideArchive bool   // boutons « pas du spam », « spam » et « archiver » selon le dossier
	ThreadMode                     bool   // regroupement par conversation actif
	ThreadOpen                     bool   // une conversation est ouverte
	ThreadsUnsupported             bool   // le serveur ne sait pas regrouper
	ThreadBackURL                  string // retour à la liste complète
}

// AttachmentView est une pièce jointe affichée sous un message.
type AttachmentView struct {
	Thumb                   template.URL // URI data: d'une miniature (images), produite par le serveur donc de confiance
	Ext                     string       // extension en capitales (autres fichiers)
	Name, Type, Size, URL   string
	Index                   int    // position de la partie dans le message (ZIP partiel)
	Icon                    string // symbole du sprite, selon le type de fichier
	PreviewURL, PreviewKind string // aperçu dans le navigateur : « image » ou « text » ; vide : téléchargement seulement
}

// MessageView regroupe l'affichage d'un message.
type MessageView struct {
	SenderName, SenderEmail, ToShort string
	ZipURL                           string
	FolderLabel                      string
	JustRead                         bool
	UID                              uint32
	Folder                           string
	Subject, From, To, Cc            string
	Date                             string
	Attachments                      []AttachmentView
	BodyURL, TextURL, HTMLURL        string
	ShowRemoteURL                    string
	ViewText, HasHTML                bool
	RemoteBlocked                    int
	Flagged, Seen                    bool
	Truncated, Partial               bool
	NoBody                           bool
	Back, Self                       string
	PrevMsgURL, NextMsgURL           string // message précédent et suivant dans la liste affichée
	MoveTargets                      []string
	InTrash, IsDraft                 bool
	ReadReceipt                      string      // adresse qui demande une confirmation de lecture (vide : rien à traiter)
	UnsubscribeURL                   string      // lien de désabonnement (https ou composition d'un mailto) ; vide : aucun
	DKIM, ReplyTo, Bcc, Size         string      // DKIM : pass, warn ou vide
	Events                           []EventView // invitations .ics jointes
	ReplyMode                        string      // action du bouton principal : reply, replyall ou forward (la dernière utilisée)
	AvatarURL                        string      // photo du correspondant (carnet d'adresses ou Gravatar) ; vide : silhouette
	ContactID                        string      // fiche du contact expéditeur (vide : inconnu)
	UnsubscribeExternal              bool        // le lien sort de l'application
}

// SourceURL renvoie le lien vers la source (view = « source » ou « headers ») ou vers le téléchargement .eml (« eml »).
func (m MessageView) SourceURL(view string) string {
	v := url.Values{"folder": {m.Folder}, "uid": {strconv.FormatUint(uint64(m.UID), 10)}, "view": {view}}
	return "/mail/source?" + v.Encode()
}

// PrintURL renvoie le lien vers la version imprimable.
func (m MessageView) PrintURL() string {
	v := url.Values{"folder": {m.Folder}, "uid": {strconv.FormatUint(uint64(m.UID), 10)}}
	return "/mail/print?" + v.Encode()
}

// FilterURL renvoie le lien de création d'un filtre prérempli avec l'expéditeur du message.
func (m MessageView) FilterURL() string {
	return "/settings/filters/edit?" + url.Values{"from": {m.SenderEmail}}.Encode()
}

// ComposeURL renvoie le lien de composition (répondre, transférer, modifier…) pour ce message.
func (m MessageView) ComposeURL(mode string) string {
	v := url.Values{"mode": {mode}, "folder": {m.Folder}, "uid": {strconv.FormatUint(uint64(m.UID), 10)}, "back": {m.Back}}
	return "/compose?" + v.Encode()
}

// PageLink est un numéro de page du pied de liste (ou une ellipse).
type PageLink struct {
	Label        string
	URL          string
	Current, Gap bool
}

// pageLinks construit « 1 2 3 4 5 … 102 » : les cinq premières pages, ou les deux voisines de la page courante,
// plus la première et la dernière.
func pageLinks(cur, total int, urlFor func(int) string) []PageLink {
	if total <= 1 {
		return nil
	}
	start, end := max(1, cur-2), min(total, cur+2)
	if cur <= 3 {
		start, end = 1, min(total, 5)
	}
	if cur >= total-2 {
		start, end = max(1, total-4), total
	}
	var pages []int
	if start > 1 {
		pages = append(pages, 1)
	}
	for p := start; p <= end; p++ {
		pages = append(pages, p)
	}
	if end < total {
		pages = append(pages, total)
	}
	var out []PageLink
	prev := 0
	for _, p := range pages {
		if prev != 0 && p > prev+1 {
			out = append(out, PageLink{Label: "…", Gap: true})
		}
		out = append(out, PageLink{Label: strconv.Itoa(p), URL: urlFor(p), Current: p == cur})
		prev = p
	}
	return out
}

// --- paramètres de la liste ---

type listParams struct {
	Folder string
	Page   int
	Sort   string
	Desc   bool
	Q      string
	Scope  string
	Filter string // all ou unread
	Thread uint32 // UID d'un message : seule sa conversation est affichée
}

func defaultDesc(sort string) bool { return sort != "from" && sort != "subject" }

func parseListParams(v url.Values) listParams {
	lp := listParams{Folder: "INBOX", Page: 1, Sort: "date", Desc: true, Scope: "all", Filter: "all"}
	if f := v.Get("folder"); mailconn.ValidFolderName(f) {
		lp.Folder = f
	}
	if n, err := strconv.Atoi(v.Get("page")); err == nil && n > 1 && n < 100000 {
		lp.Page = n
	}
	switch s := v.Get("sort"); s {
	case "date", "arrival", "from", "subject", "size":
		lp.Sort = s
	}
	lp.Desc = defaultDesc(lp.Sort)
	switch v.Get("dir") {
	case "asc":
		lp.Desc = false
	case "desc":
		lp.Desc = true
	}
	if q := strings.TrimSpace(strings.Join(append([]string{v.Get("q")}, advancedTerms(v, time.Now())...), " ")); q != "" {
		if r := []rune(q); len(r) > 200 {
			q = string(r[:200])
		}
		lp.Q = q
	}
	switch s := v.Get("scope"); s {
	case "subject", "from", "body":
		lp.Scope = s
	}
	if v.Get("filter") == "unread" {
		lp.Filter = "unread"
	}
	if u, ok := parseUID(v.Get("thread")); ok {
		lp.Thread = u
	}
	return lp
}

// advancedTerms traduit les champs de la recherche avancée (adv_*) en opérateurs de recherche.
func advancedTerms(v url.Values, now time.Time) []string {
	quote := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(s, `"`, ""))
		if strings.ContainsAny(s, " \t") {
			return `"` + s + `"`
		}
		return s
	}
	var out []string
	for _, f := range []struct{ field, op string }{{"adv_from", "from"}, {"adv_to", "to"}, {"adv_subject", "subject"}, {"adv_text", "text"}} {
		if x := quote(v.Get(f.field)); x != "" {
			out = append(out, f.op+":"+x)
		}
	}
	var is []string
	if v.Get("adv_unread") != "" {
		is = append(is, "unseen")
	}
	if v.Get("adv_flagged") != "" {
		is = append(is, "flagged")
	}
	if len(is) > 0 {
		out = append(out, "is:"+strings.Join(is, ","))
	}
	if v.Get("adv_attach") != "" {
		out = append(out, "has:attachment")
	}
	days := map[string]int{"3d": 3, "1w": 7, "1m": 30, "3m": 91, "6m": 182, "1y": 365}[v.Get("adv_date")]
	if days > 0 {
		out = append(out, "date:"+now.AddDate(0, 0, -days).Format("2006.01.02")+"/"+now.Format("2006.01.02"))
	}
	return out
}

func (lp listParams) values() url.Values {
	q := url.Values{}
	if lp.Folder != "INBOX" {
		q.Set("folder", lp.Folder)
	}
	if lp.Page > 1 {
		q.Set("page", strconv.Itoa(lp.Page))
	}
	if lp.Sort != "date" || !lp.Desc {
		q.Set("sort", lp.Sort)
		q.Set("dir", map[bool]string{true: "desc", false: "asc"}[lp.Desc])
	}
	if lp.Q != "" {
		q.Set("q", lp.Q)
		if lp.Scope != "all" {
			q.Set("scope", lp.Scope)
		}
	}
	if lp.Filter == "unread" {
		q.Set("filter", "unread")
	}
	if lp.Thread != 0 {
		q.Set("thread", strconv.FormatUint(uint64(lp.Thread), 10))
	}
	return q
}

// URL construit l'adresse de la liste ; seules les valeurs non par défaut y figurent.
func (lp listParams) URL() string {
	q := lp.values()
	if len(q) == 0 {
		return "/"
	}
	return "/?" + q.Encode()
}

// PaneURL construit l'adresse de la liste avec un message ouvert dans le volet de lecture.
func (lp listParams) PaneURL(uid uint32, extra url.Values) string {
	q := lp.values()
	q.Set("uid", strconv.FormatUint(uint64(uid), 10))
	for k, v := range extra {
		q[k] = v
	}
	return "/?" + q.Encode()
}

// safeBack n'accepte qu'une adresse locale de la liste ou d'un message (pas de redirection ouverte).
func safeBack(v string) string {
	if v == "" || !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.ContainsAny(v, "\\\r\n") {
		return "/"
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "" || u.Host != "" || (u.Path != "/" && u.Path != "/mail/message") {
		return "/"
	}
	return v
}

// --- mise en forme ---

func formatDate(t, now time.Time, lang string, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	t, now = t.In(loc), now.In(loc)
	switch {
	case t.Year() == now.Year() && t.YearDay() == now.YearDay():
		return t.Format("15:04")
	case t.Year() == now.Year() && lang == "fr":
		return t.Format("02/01")
	case t.Year() == now.Year():
		return t.Format("Jan 02")
	case lang == "fr":
		return t.Format("02/01/2006")
	}
	return t.Format("2006-01-02")
}

// listDate est la date d'une ligne de la liste : relative pendant les 4 premières heures (« il y a 12 min »), « hier »
// avec l'heure la veille, sinon comme formatDate.
func listDate(t, now time.Time, lang string, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	if d := now.Sub(t); d >= 0 && d < 4*time.Hour {
		fr := lang == "fr"
		switch {
		case d < time.Minute:
			if fr {
				return "à l'instant"
			}
			return "just now"
		case d < time.Hour:
			if fr {
				return fmt.Sprintf("il y a %d min", int(d.Minutes()))
			}
			return fmt.Sprintf("%d min ago", int(d.Minutes()))
		}
		if fr {
			return fmt.Sprintf("il y a %d h", int(d.Hours()))
		}
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	lt, ln := t.In(loc), now.In(loc)
	if y := ln.AddDate(0, 0, -1); lt.Year() == y.Year() && lt.YearDay() == y.YearDay() {
		if lang == "fr" {
			return "hier " + lt.Format("15:04")
		}
		return "yesterday " + lt.Format("15:04")
	}
	if lt.Year() == ln.Year() && lt.YearDay() == ln.YearDay() {
		return lt.Format("15:04")
	}
	if lt.Year() == ln.Year() {
		if lang == "fr" {
			return fmt.Sprintf("%d %s", lt.Day(), frMonthsShort[lt.Month()-1])
		}
		return lt.Format("2 Jan")
	}
	if lang == "fr" {
		return fmt.Sprintf("%d %s %d", lt.Day(), frMonths[lt.Month()-1], lt.Year())
	}
	return lt.Format("2 January 2006")
}

func firstTime(a, b time.Time) time.Time {
	if !a.IsZero() {
		return a
	}
	return b
}

var frMonths = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
var frMonthsShort = []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}

// formatLongDate : « 14 octobre 2026 14:03 » (comme l'en-tête d'un message RainLoop).
func formatLongDate(t time.Time, lang string, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	t = t.In(loc)
	if lang == "fr" {
		return fmt.Sprintf("%d %s %d %s", t.Day(), frMonths[t.Month()-1], t.Year(), t.Format("15:04"))
	}
	return t.Format("2 January 2006 15:04")
}

func formatFullDate(t time.Time, lang string, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	if lang == "fr" {
		return t.In(loc).Format("02/01/2006 15:04")
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

func humanSize(n int64, lang string) string {
	units := []string{"B", "KB", "MB", "GB"}
	if lang == "fr" {
		units = []string{"o", "Ko", "Mo", "Go"}
	}
	f, i := float64(n), 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[0])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func addrList(l []mailview.Address) string {
	parts := make([]string, len(l))
	for i, a := range l {
		parts[i] = a.String()
	}
	return strings.Join(parts, ", ")
}

// --- accès IMAP ---

func (s *Server) poolKey(sess *session.Session, acc userdata.MailAccount) string {
	return sess.ID + "/" + acc.ID
}

func (s *Server) mailCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), mailTimeout)
}

// mailErr répond à une erreur d'accès au serveur IMAP.
func (s *Server) mailErr(w http.ResponseWriter, r *http.Request, sess *session.Session, err error) {
	var te *mailconn.TestError
	switch {
	case errors.Is(err, mailconn.ErrNotFound):
		s.message(w, r, sess, http.StatusNotFound, "", "mail.err.not_found")
	case errors.Is(err, mailconn.ErrTooLarge):
		s.message(w, r, sess, http.StatusRequestEntityTooLarge, "", "mail.err.too_large")
	case errors.Is(err, errNotClearable):
		s.message(w, r, sess, http.StatusForbidden, "", "mail.err.not_clearable")
	case errors.As(err, &te):
		s.log.Warn("erreur IMAP", "login", store.ReduceLogin(sess.Account.Login), "kind", te.Kind, "path", r.URL.Path)
		s.message(w, r, sess, http.StatusBadGateway, "", "accounts.test.fail."+te.Kind)
	default:
		s.serverError(w, r, sess, err)
	}
}

// isSystemFolder : la boîte de réception et les dossiers à usage spécial (envoyés, brouillons, indésirables, corbeille, archives).
func isSystemFolder(f mailconn.Folder) bool {
	if strings.EqualFold(f.Name, "INBOX") {
		return true
	}
	for _, a := range []imap.MailboxAttr{imap.MailboxAttrSent, imap.MailboxAttrDrafts, imap.MailboxAttrJunk, imap.MailboxAttrTrash, imap.MailboxAttrArchive} {
		if f.HasAttr(a) {
			return true
		}
	}
	return false
}

// --- liste des messages (accueil) ---

func (s *Server) home(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, list, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	q := r.URL.Query()
	lp := parseListParams(q)
	lang := s.lang(r, sess)
	sess.Mu.Lock()
	set := sess.Account.Data.Settings
	sess.Mu.Unlock()
	pageSize := set.PageSize
	if pageSize < 10 || pageSize > maxPageSize {
		pageSize = 50
	}
	threePane := set.Layout != "list"
	bottomPane := set.Layout == "bottom" // volet de lecture sous la liste
	var selUID uint32
	if threePane {
		selUID, _ = parseUID(q.Get("uid"))
	}

	p := &Page{TitleKey: "home.title", Accounts: accountViews(list), ActiveID: acc.ID, Mail: true, TopUser: acc.Email}
	lv := &ListView{Folder: lp.Folder, Query: lp.Q, Scope: lp.Scope, Filter: lp.Filter, Sort: lp.Sort, Back: lp.URL(),
		SortURLs: map[string]string{}, SortMark: map[string]string{}, ThreePane: threePane, BottomPane: bottomPane, Selected: selUID}
	p.List = lv
	lv.PageSize = pageSize
	lv.PageSizes = pageSizeChoices(pageSize)

	ctx, cancel := s.mailCtx(r)
	defer cancel()
	key, ep := s.poolKey(sess, acc), imapEndpoint(acc)

	// 1. Le message du volet de lecture, avant les compteurs : l'ouverture le marque comme lu.
	var raw *mailconn.RawMessage
	var parsed *mailview.Message
	var paneErr error
	if selUID > 0 {
		raw, parsed, paneErr = s.fetchMessage(ctx, sess, acc, lp.Folder, selUID, set.MarkReadOnOpen)
	}

	// 2. Les dossiers et leurs compteurs.
	var folders []mailconn.Folder
	if err := s.pool.Do(ctx, key, ep, func(c *mailconn.Conn) (err error) { folders, err = c.Folders(true); return }); err != nil {
		p.FolderError = toResult(err)
		s.log.Warn("liste des dossiers impossible", "login", store.ReduceLogin(sess.Account.Login), "account", acc.ID, "kind", p.FolderError.Kind)
		s.render(w, r, sess, http.StatusOK, "home", p)
		return
	}
	p.Poll = pollInfo(folders, set)
	p.Folders, lv.MoveTargets = folderPanel(folders, acc.Options.Folders, lp.Folder, lp.Filter, s.openFolders(sess, acc.ID))
	p.MoveMenu = lv.MoveTargets
	p.MoveRows = moveRows(p.Folders)
	_ = s.pool.Do(ctx, key, ep, func(c *mailconn.Conn) error {
		if used, limit, ok := c.Quota(); ok {
			p.Quota = quotaView(used, limit, lang)
		}
		return nil
	})
	trashFolder := mailconn.ResolveTrash(folders, acc.Options.Folders.Trash)
	junkFolder := mailconn.ResolveJunk(folders, acc.Options.Folders.Junk)
	for _, f := range folders {
		if f.Name != lp.Folder {
			continue
		}
		switch {
		case strings.EqualFold(f.Name, "INBOX"):
		case strings.EqualFold(f.Name, trashFolder):
			lv.ClearKind, lv.ClearCount = "trash", int(f.Total)
		case strings.EqualFold(f.Name, junkFolder):
			lv.ClearKind, lv.ClearCount = "junk", int(f.Total)
		}
	}
	for _, f := range folders {
		if f.Name == lp.Folder {
			lv.UnreadCount = int(f.Unseen)
		}
	}
	// Boutons selon le dossier, comme RainLoop : pas d'archivage dans l'archive, pas de « spam » dans Indésirables, Envoyés
	// et Brouillons ; « pas du spam » dans Indésirables.
	sentFolder := mailconn.ResolveSent(folders, acc.Options.Folders.Sent)
	inDrafts := acc.Options.Folders.Drafts != userdata.NoFolder && strings.EqualFold(lp.Folder, mailconn.ResolveDrafts(folders, acc.Options.Folders.Drafts))
	lv.NotSpam = strings.EqualFold(lp.Folder, junkFolder)
	lv.HideSpam = lv.NotSpam || inDrafts || strings.EqualFold(lp.Folder, sentFolder)
	lv.HideArchive = inDrafts || strings.EqualFold(lp.Folder, mailconn.ResolveArchive(folders, acc.Options.Folders.Archive))

	// 3. La page de messages.
	var res *mailconn.ListResult
	err := s.pool.Do(ctx, key, ep, func(c *mailconn.Conn) (err error) {
		res, err = c.ListMessages(mailconn.ListQuery{Folder: lp.Folder, Page: lp.Page, PageSize: pageSize,
			Sort: lp.Sort, Desc: lp.Desc, Search: lp.Q, Scope: lp.Scope, UnreadOnly: lp.Filter == "unread",
			Threads: set.Threads && lp.Thread == 0, ThreadOf: lp.Thread})
		return
	})
	if err != nil {
		lv.Error = toResult(err)
		s.log.Warn("liste des messages impossible", "login", store.ReduceLogin(sess.Account.Login), "folder", lp.Folder, "kind", lv.Error.Kind)
		s.render(w, r, sess, http.StatusOK, "home", p)
		return
	}
	lv.Total, lv.Page, lv.Pages, lv.SortFallback = res.Total, res.Page, res.Pages, res.SortFallback
	lp.Page = res.Page
	lv.Back = lp.URL()
	for filter, dst := range map[string]*string{"all": &lv.FilterAllURL, "unread": &lv.FilterUnreadURL} {
		f := lp
		f.Page, f.Filter = 1, filter
		*dst = f.URL()
	}
	clear := lp
	clear.Page, clear.Q, clear.Scope = 1, "", "all"
	lv.ClearSearchURL = clear.URL()

	loc := s.userLoc(sess)
	now := time.Now()
	// Dans Envoyés et Brouillons, la colonne désigne les destinataires, comme dans RainLoop.
	toColumn := strings.EqualFold(lp.Folder, mailconn.ResolveSent(folders, acc.Options.Folders.Sent)) ||
		acc.Options.Folders.Drafts != userdata.NoFolder && strings.EqualFold(lp.Folder, mailconn.ResolveDrafts(folders, acc.Options.Folders.Drafts))
	for _, m := range res.Messages {
		from := ""
		who := m.From
		if toColumn && len(m.To) > 0 {
			who = m.To
		}
		for i, a := range who {
			if i == 3 {
				from += ", …"
				break
			}
			if i > 0 {
				from += ", "
			}
			from += a.Short()
		}
		readURL := lp.PaneURL(m.UID, nil)
		if !threePane {
			rb := url.Values{"folder": {lp.Folder}, "uid": {strconv.FormatUint(uint64(m.UID), 10)}, "back": {lv.Back}}
			readURL = "/mail/message?" + rb.Encode()
		}
		row := MessageRow{
			UID: m.UID, Subject: m.Subject, From: from, Date: listDate(firstTime(m.Arrival, m.Date), now, lang, loc), Size: humanSize(m.Size, lang),
			AttachKind: m.AttachKind, Important: m.Important, Seen: m.Seen, Flagged: m.Flagged, Answered: m.Answered, Forwarded: m.Forwarded, HasAttachment: m.HasAttachment, Selected: m.UID == selUID, ReadURL: readURL,
		}
		if m.ThreadUnseen {
			// le message en tête du fil est lu : ligne non grasse, fond atténué
			row.Seen, row.SubUnread = true, true
		}
		row.SubjectPrefix, row.SubjectRest = splitSubject(m.Subject)
		if len(m.ThreadUIDs) > 1 {
			ids := make([]string, len(m.ThreadUIDs))
			for i, u := range m.ThreadUIDs {
				ids[i] = strconv.FormatUint(uint64(u), 10)
			}
			tl := lp
			tl.Page, tl.Thread = 1, m.UID
			row.ThreadCount, row.ThreadUIDs, row.ThreadURL = len(ids), strings.Join(ids, ","), tl.URL()
		}
		lv.Rows = append(lv.Rows, row)
	}
	lv.ThreadMode, lv.ThreadOpen, lv.ThreadsUnsupported = set.Threads, lp.Thread != 0, res.ThreadsUnsupported
	if lp.Thread != 0 {
		back := lp
		back.Thread, back.Page = 0, 1
		lv.ThreadBackURL = back.URL()
	}
	lv.PageLinks = pageLinks(res.Page, res.Pages, func(n int) string { q := lp; q.Page = n; return q.URL() })
	lv.Stay = lv.Back
	if res.Page > 1 {
		prev := lp
		prev.Page = res.Page - 1
		lv.PrevURL = prev.URL()
	}
	if res.Page < res.Pages {
		next := lp
		next.Page = res.Page + 1
		lv.NextURL = next.URL()
	}
	for _, col := range []string{"from", "subject", "date", "size"} {
		h := lp
		h.Page = 1
		h.Sort = col
		if lp.Sort == col {
			h.Desc = !lp.Desc
			lv.SortMark[col] = map[bool]string{true: "▼", false: "▲"}[lp.Desc]
		} else {
			h.Desc = defaultDesc(col)
		}
		lv.SortURLs[col] = h.URL()
	}

	// 4. Le volet de lecture.
	if selUID > 0 {
		if paneErr != nil {
			lv.PaneError = errorKey(paneErr)
			s.log.Warn("message du volet de lecture impossible", "login", store.ReduceLogin(sess.Account.Login), "folder", lp.Folder)
		} else {
			p.Msg = s.buildMessageView(sess, acc, lp.Folder, selUID, raw, parsed, folders, lang, q.Get("remote") == "1", q.Get("view") == "text", lv.Back,
				func(extra url.Values) string { return lp.PaneURL(selUID, extra) })
			lv.Stay = p.Msg.Self
			// Flèches « précédent / suivant » : voisins du message dans la page affichée.
			for i, row := range lv.Rows {
				if row.UID != selUID {
					continue
				}
				if i > 0 {
					p.Msg.PrevMsgURL = lv.Rows[i-1].ReadURL
				}
				if i+1 < len(lv.Rows) {
					p.Msg.NextMsgURL = lv.Rows[i+1].ReadURL
				}
				break
			}
		}
	}
	s.render(w, r, sess, http.StatusOK, "home", p)
}

// errorKey renvoie la clé de message qui décrit une erreur d'accès au serveur IMAP.
func errorKey(err error) string {
	var te *mailconn.TestError
	switch {
	case errors.Is(err, mailconn.ErrNotFound):
		return "mail.err.not_found"
	case errors.Is(err, mailconn.ErrTooLarge):
		return "mail.err.too_large"
	case errors.As(err, &te):
		return "accounts.test.fail." + te.Kind
	}
	return "error.generic"
}

// pageSizeChoices renvoie les tailles proposées, plus la taille actuelle si elle est personnalisée (réglages).
func pageSizeChoices(current int) []int {
	out := slices.Clone(pageSizes)
	if !slices.Contains(out, current) {
		out = append(out, current)
		slices.Sort(out)
	}
	return out
}

// withoutPage retire le numéro de page d'une adresse de liste (la taille de page change la pagination).
func withoutPage(back string) string {
	u, err := url.Parse(back)
	if err != nil {
		return "/"
	}
	q := u.Query()
	q.Del("page")
	if len(q) == 0 {
		return u.Path
	}
	return u.Path + "?" + q.Encode()
}

// mailPageSize enregistre la taille de page choisie dans la liste (préférence de l'utilisateur).
func (s *Server) mailPageSize(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	size, err := strconv.Atoi(r.PostFormValue("size"))
	if err != nil || !slices.Contains(pageSizes, size) {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	sess.Mu.Lock()
	prev := sess.Account.Data.Settings.PageSize
	sess.Account.Data.Settings.PageSize = size
	err = s.store.SaveData(sess.Account)
	if err != nil {
		sess.Account.Data.Settings.PageSize = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, withoutPage(safeBack(r.PostFormValue("back"))), http.StatusSeeOther)
}

// mailPane renvoie uniquement le volet de lecture (fragment HTML) : le script de la liste remplace le volet
// sans recharger la liste, ce qui conserve sa position de défilement.
func (s *Server) mailPane(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	lp := parseListParams(q)
	uid, ok := parseUID(q.Get("uid"))
	if !ok {
		http.Error(w, "uid invalide", http.StatusBadRequest)
		return
	}
	lang := s.lang(r, sess)
	sess.Mu.Lock()
	markRead := sess.Account.Data.Settings.MarkReadOnOpen
	sess.Mu.Unlock()

	ctx, cancel := s.mailCtx(r)
	defer cancel()
	p := &Page{List: &ListView{}, ActiveID: acc.ID}
	status := http.StatusOK
	raw, m, err := s.fetchMessage(ctx, sess, acc, lp.Folder, uid, markRead)
	var folders []mailconn.Folder
	if err == nil {
		err = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
			folders, err = c.Folders(false)
			return
		})
	}
	if err != nil {
		p.List.PaneError = errorKey(err)
		status = http.StatusBadGateway
		if errors.Is(err, mailconn.ErrNotFound) {
			status = http.StatusNotFound
		}
	} else {
		p.Msg = s.buildMessageView(sess, acc, lp.Folder, uid, raw, m, folders, lang, q.Get("remote") == "1", q.Get("view") == "text", lp.URL(),
			func(extra url.Values) string { return lp.PaneURL(uid, extra) })
	}
	s.renderFragment(w, r, sess, status, "fragment", p)
}

// --- lecture d'un message ---

func parseUID(s string) (uint32, bool) {
	n, err := strconv.ParseUint(s, 10, 32)
	return uint32(n), err == nil && n > 0
}

// fetchMessage télécharge et analyse un message (sans lister les dossiers). L'ouverture marque le message
// comme lu si markRead est vrai ; sinon le message est consulté sans modifier ses drapeaux.
func (s *Server) fetchMessage(ctx context.Context, sess *session.Session, acc userdata.MailAccount, folder string, uid uint32, markRead bool) (*mailconn.RawMessage, *mailview.Message, error) {
	var raw *mailconn.RawMessage
	err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		if raw, err = c.FetchMessage(folder, uid, !markRead, maxMessageBytes); err != nil {
			return err
		}
		if markRead && !raw.HasFlag(imap.FlagSeen) {
			if err := c.StoreFlags(folder, []uint32{uid}, imap.StoreFlagsAdd, imap.FlagSeen); err != nil {
				return err
			}
			raw.Flags = append(raw.Flags, imap.FlagSeen)
			raw.JustRead = true
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	m, err := mailview.Parse(raw.Raw)
	if err != nil {
		return nil, nil, err
	}
	return raw, m, nil
}

// fetchParsed est fetchMessage suivi de la liste des dossiers (avec compteurs si withCounts).
func (s *Server) fetchParsed(ctx context.Context, sess *session.Session, acc userdata.MailAccount, folder string, uid uint32, markRead, withCounts bool) (*mailconn.RawMessage, *mailview.Message, []mailconn.Folder, error) {
	raw, m, err := s.fetchMessage(ctx, sess, acc, folder, uid, markRead)
	if err != nil {
		return nil, nil, nil, err
	}
	var folders []mailconn.Folder
	err = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		folders, err = c.Folders(withCounts)
		return
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return raw, m, folders, nil
}

func (s *Server) messageParams(w http.ResponseWriter, r *http.Request, sess *session.Session, v url.Values) (folder string, uid uint32, ok bool) {
	folder = v.Get("folder")
	uid, uidOK := parseUID(v.Get("uid"))
	if !mailconn.ValidFolderName(folder) || !uidOK {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return "", 0, false
	}
	return folder, uid, true
}

// attachmentIcon choisit le symbole du sprite selon le type MIME, et à défaut l'extension du nom.
func attachmentIcon(ct, name string) string {
	mt, _, _ := strings.Cut(strings.ToLower(ct), ";")
	mt = strings.TrimSpace(mt)
	main, _, _ := strings.Cut(mt, "/")
	switch main {
	case "image":
		return "image"
	case "audio":
		return "file-audio"
	case "video":
		return "file-video"
	}
	switch mt {
	case "application/pdf":
		return "file-pdf"
	case "text/calendar":
		return "calendar"
	case "application/zip", "application/x-7z-compressed", "application/x-rar-compressed", "application/vnd.rar",
		"application/gzip", "application/x-tar", "application/x-bzip2":
		return "file-zip"
	}
	switch strings.ToLower(attachmentExt(name)) {
	case "pdf":
		return "file-pdf"
	case "zip", "rar", "7z", "gz", "tgz", "bz2", "xz":
		return "file-zip"
	case "xls", "xlsx", "ods", "csv", "tsv":
		return "file-sheet"
	case "ppt", "pptx", "odp":
		return "file-slides"
	case "mp3", "ogg", "wav", "flac", "m4a":
		return "file-audio"
	case "mp4", "mkv", "avi", "mov", "webm":
		return "file-video"
	case "txt", "md", "log":
		return "file-text"
	case "ics":
		return "calendar"
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg":
		return "image"
	}
	switch {
	case strings.Contains(mt, "spreadsheet"), strings.Contains(mt, "ms-excel"):
		return "file-sheet"
	case strings.Contains(mt, "presentation"), strings.Contains(mt, "powerpoint"):
		return "file-slides"
	case main == "text":
		return "file-text"
	}
	return "doc"
}

// attachmentExt renvoie l'extension d'un nom de fichier en capitales (4 caractères au plus).
func attachmentExt(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 || i == len(name)-1 {
		return ""
	}
	e := strings.ToUpper(name[i+1:])
	if len(e) > 4 {
		e = e[:4]
	}
	return e
}

// buildMessageView prépare l'affichage d'un message. urlFor construit l'adresse de la page qui l'affiche
// (volet de lecture ou page seule) avec des paramètres en plus.
func (s *Server) buildMessageView(sess *session.Session, acc userdata.MailAccount, folder string, uid uint32, raw *mailconn.RawMessage, m *mailview.Message,
	folders []mailconn.Folder, lang string, remote, viewText bool, back string, urlFor func(url.Values) string) *MessageView {
	viewText = viewText || m.HTML == ""
	block := s.blockRemote(sess, acc, m.From, remote)
	uidStr := strconv.FormatUint(uint64(uid), 10)

	state := url.Values{}
	if viewText && m.HTML != "" {
		state.Set("view", "text")
	}
	if remote {
		state.Set("remote", "1")
	}
	mv := &MessageView{
		UID: uid, Folder: folder, Subject: m.Subject, From: addrList(m.From), To: addrList(m.To), Cc: addrList(m.Cc),
		DKIM: m.DKIM, ReplyTo: addrList(m.ReplyTo), Bcc: addrList(m.Bcc), Size: humanSize(int64(len(raw.Raw)), lang),
		Date: formatLongDate(firstTime(raw.Arrival, m.Date), lang, s.userLoc(sess)), HasHTML: m.HTML != "", ViewText: viewText,
		Flagged: raw.HasFlag(imap.FlagFlagged), Seen: raw.HasFlag(imap.FlagSeen),
		Truncated: m.Truncated, Partial: m.Partial, NoBody: m.HTML == "" && m.Text == "",
		Back: back, Self: urlFor(state), JustRead: raw.JustRead,
		InTrash: strings.EqualFold(folder, mailconn.ResolveTrash(folders, acc.Options.Folders.Trash)),
		IsDraft: acc.Options.Folders.Drafts != userdata.NoFolder && strings.EqualFold(folder, mailconn.ResolveDrafts(folders, acc.Options.Folders.Drafts)),
	}
	if mv.Subject == "" {
		mv.Subject = "—"
	}
	sess.Mu.Lock()
	mv.ReplyMode = sess.Account.Data.Settings.LastReply
	sess.Mu.Unlock()
	if mv.ReplyMode != "replyall" && mv.ReplyMode != "forward" {
		mv.ReplyMode = "reply"
	}
	if len(m.From) > 0 {
		mv.SenderName, mv.SenderEmail = m.From[0].Short(), m.From[0].Email
		mv.ContactID = s.contactIDFor(sess, mv.SenderEmail)
		if s.avatarsEnabled(sess) {
			mv.AvatarURL = "/mail/avatar?" + url.Values{"email": {mv.SenderEmail}}.Encode()
		}
	}
	for _, u := range m.ListUnsubscribe { // https d'abord : un clic suffit ; sinon un message pré-rempli
		if strings.HasPrefix(strings.ToLower(u), "https://") {
			mv.UnsubscribeURL, mv.UnsubscribeExternal = u, true
			break
		}
	}
	if mv.UnsubscribeURL == "" && len(m.ListUnsubscribe) > 0 {
		mv.UnsubscribeURL = "/compose?" + url.Values{"mailto": {m.ListUnsubscribe[0]}, "back": {back}}.Encode()
	}
	// Demande de confirmation de lecture : proposée une seule fois (le mot-clé $MDNSent est posé ensuite), jamais pour
	// ses propres messages ni pour un brouillon.
	if len(m.ReadReceiptTo) > 0 && !raw.HasFlag(flagMDNSent) && !raw.HasFlag(imap.FlagDraft) && !mv.IsDraft &&
		(mv.SenderEmail == "" || !ownEmails(acc)[strings.ToLower(mv.SenderEmail)]) {
		mv.ReadReceipt = m.ReadReceiptTo[0].Email
	}
	var to []string
	for i, a := range m.To {
		if i == 3 {
			to = append(to, "…")
			break
		}
		to = append(to, a.String())
	}
	mv.ToShort = strings.Join(to, ", ")
	body := url.Values{"folder": {folder}, "uid": {uidStr}}
	if viewText {
		body.Set("view", "text")
	}
	if remote {
		body.Set("remote", "1")
	}
	mv.BodyURL = "/mail/body?" + body.Encode()
	withRemote := func(v url.Values) url.Values {
		if remote {
			v.Set("remote", "1")
		}
		return v
	}
	mv.HTMLURL = urlFor(withRemote(url.Values{}))
	mv.TextURL = urlFor(withRemote(url.Values{"view": {"text"}}))
	if !viewText && m.HTML != "" {
		_, rep := mailview.Sanitize(m.HTML, mailview.SanitizeOptions{BlockRemote: block, InlineImages: m.InlineImages})
		mv.RemoteBlocked = rep.RemoteBlocked
		if rep.RemoteBlocked > 0 {
			rq := url.Values{"remote": {"1"}}
			mv.ShowRemoteURL = urlFor(rq)
		}
	}
	thumbs := 0
	seenEvents := map[string]bool{}
	for _, a := range m.Attachments {
		aq := url.Values{"folder": {folder}, "uid": {uidStr}, "index": {strconv.Itoa(a.Index)}}
		av := AttachmentView{Name: a.Name, Type: a.ContentType, Size: humanSize(int64(a.Size), lang), URL: "/mail/attachment?" + aq.Encode(), Ext: attachmentExt(a.Name), Icon: attachmentIcon(a.ContentType, a.Name), Index: a.Index}
		if kind := previewKind(a.ContentType, a.Data()); kind != "" {
			av.PreviewURL, av.PreviewKind = av.URL+"&inline=1", kind
		}
		if a.IsImage() && thumbs < maxThumbnails && a.Size <= maxThumbSource {
			if uri, ok := mailview.Thumbnail(a.Data(), thumbSize); ok {
				av.Thumb = template.URL(uri) // html/template refuserait sinon le schéma data: (« #ZgotmplZ »)
				thumbs++
			}
		}
		mv.Attachments = append(mv.Attachments, av)
		if len(mv.Events) < maxEventCards && (strings.EqualFold(a.ContentType, "text/calendar") || strings.HasSuffix(strings.ToLower(a.Name), ".ics")) {
			for _, ev := range ical.Parse(a.Data(), s.userLoc(sess)) {
				key := ev.Summary + "|" + ev.Start.Format(time.RFC3339) + "|" + ev.End.Format(time.RFC3339)
				if len(mv.Events) < maxEventCards && !seenEvents[key] { // la même invitation jointe deux fois : une seule carte
					seenEvents[key] = true
					mv.Events = append(mv.Events, eventView(ev, lang, s.userLoc(sess)))
				}
			}
		}
	}
	if len(m.Attachments) > 1 {
		mv.ZipURL = "/mail/attachments.zip?" + url.Values{"folder": {folder}, "uid": {uidStr}}.Encode()
	}
	_, mv.MoveTargets = folderPanel(folders, acc.Options.Folders, folder, "all", nil)
	return mv
}

// mailMessage affiche un message sur une page seule (réglage « liste seule », petits écrans, liens directs).
func (s *Server) mailMessage(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	q := r.URL.Query()
	folder, uid, ok := s.messageParams(w, r, sess, q)
	if !ok {
		return
	}
	lang := s.lang(r, sess)
	sess.Mu.Lock()
	markRead := sess.Account.Data.Settings.MarkReadOnOpen
	sess.Mu.Unlock()

	ctx, cancel := s.mailCtx(r)
	defer cancel()
	raw, m, folders, err := s.fetchParsed(ctx, sess, acc, folder, uid, markRead, true)
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	back := safeBack(q.Get("back"))
	base := url.Values{"folder": {folder}, "uid": {strconv.FormatUint(uint64(uid), 10)}, "back": {back}}
	urlFor := func(extra url.Values) string {
		v := url.Values{}
		for k, vals := range base {
			v[k] = vals
		}
		for k, vals := range extra {
			v[k] = vals
		}
		return "/mail/message?" + v.Encode()
	}
	mv := s.buildMessageView(sess, acc, folder, uid, raw, m, folders, lang, q.Get("remote") == "1", q.Get("view") == "text", back, urlFor)
	p := &Page{TitleKey: "home.title", Msg: mv, ActiveID: acc.ID, List: &ListView{Stay: mv.Self, Back: back}, Mail: true, TopUser: acc.Email}
	_, accounts, _ := s.activeAccount(sess)
	p.Accounts = accountViews(accounts)
	sess.Mu.Lock()
	setNow := sess.Account.Data.Settings
	sess.Mu.Unlock()
	p.Poll = pollInfo(folders, setNow)
	p.Folders, p.MoveMenu = folderPanel(folders, acc.Options.Folders, folder, "all", s.openFolders(sess, acc.ID))
	p.MoveRows = moveRows(p.Folders)
	s.render(w, r, sess, http.StatusOK, "mail_message", p)
}

// bodyDocHead ouvre le document du corps d'un message ; px est la taille de texte de l'utilisateur.
func bodyDocHead(px int) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="color-scheme" content="light">` +
		fmt.Sprintf(`<style>body{font:%dpx/1.45 Arial,"Liberation Sans",Verdana,Geneva,sans-serif;margin:15px;color:#000;background:#fff;overflow-wrap:anywhere}`, px) +
		`a{color:blue}a:visited{color:#609}img{max-width:100%;height:auto}blockquote{margin:0;padding:0 10px;border-left:2px solid #000}` +
		`table{max-width:100%}pre{white-space:pre-wrap}div.plain{white-space:pre-wrap;font-family:Monaco,Menlo,Consolas,'Courier New',monospace;font-size:.93em}div.plain blockquote{border-left:2px solid #00f;color:#00f;margin:0;padding:0 10px}div.plain blockquote blockquote{border-color:#080;color:#080}div.plain blockquote blockquote blockquote{border-color:#c00;color:#c00}details.wm-quote>summary{cursor:pointer;display:inline-block;padding:0 .5em;margin:.3em 0;background:#e8e8e8;border-radius:3px;color:#555;list-style:none}details.wm-quote[open]>summary{margin-bottom:.3em}img[data-blocked]{display:inline-block;min-width:1em;min-height:1em;outline:1px dashed #bbb}</style>` +
		`</head><body>`
}

// mailBody sert le corps assaini d'un message, destiné à un iframe sandbox : CSP restrictive, sans script ni
// ressource externe sauf si l'utilisateur a autorisé le contenu distant.
func (s *Server) mailBody(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	folder, uid, ok := s.messageParams(w, r, sess, q)
	if !ok {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	_, m, err := s.fetchMessage(ctx, sess, acc, folder, uid, false)
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	remote := q.Get("remote") == "1"
	block := s.blockRemote(sess, acc, m.From, remote)
	var body string
	switch {
	case q.Get("view") != "text" && m.HTML != "":
		body, _ = mailview.Sanitize(m.HTML, mailview.SanitizeOptions{BlockRemote: block, InlineImages: m.InlineImages, ProxyURL: s.proxyURLFunc(sess, acc), MailtoURL: composeMailtoURL})
		body = mailview.CollapseQuotes(body)
	case m.Text != "":
		body = mailview.TextToHTML(m.Text)
	case m.HTML != "":
		body = mailview.TextToHTML(mailview.HTMLToText(m.HTML))
	}

	img := "img-src data:"
	switch {
	case !block && acc.Options.ProxyRemoteImages:
		img += " 'self'" // les images passent par le serveur ; aucune connexion directe vers l'expéditeur
	case !block:
		img += " https: http:"
	}
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; "+img+"; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; "+
		"frame-ancestors 'self'; sandbox allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "private, no-store")
	sess.Mu.Lock()
	px := effectiveFontSize(sess.Account.Data.Settings.FontSize)
	sess.Mu.Unlock()
	_, _ = w.Write([]byte(bodyDocHead(px) + body + "</body></html>"))
}

// previewKind dit si la pièce peut s'afficher dans le navigateur sans risque : images raster reconnues d'après leurs
// octets (pas de SVG) et texte brut.
func previewKind(declared string, data []byte) string {
	head := data[:min(len(data), 512)]
	switch sniff := http.DetectContentType(head); sniff {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return "image"
	case "application/pdf":
		return "pdf"
	case "audio/mpeg", "audio/ogg", "audio/wave", "audio/aiff":
		return "audio"
	case "text/plain; charset=utf-8":
		if d := strings.ToLower(declared); strings.HasPrefix(d, "text/plain") || d == "text/csv" || d == "text/markdown" {
			return "text"
		}
	}
	return ""
}

// composeMailtoURL ouvre la composition de wmail-go pour un lien « mailto: » cliqué dans un message.
func composeMailtoURL(mailto string) string {
	return "/compose?" + url.Values{"mailto": {mailto}}.Encode()
}

// fetchAttachmentPart télécharge la pièce jointe n° idx sans charger le message entier.
func (s *Server) fetchAttachmentPart(ctx context.Context, sess *session.Session, acc userdata.MailAccount, folder string, uid uint32, idx int) (name, ctype string, data []byte, err error) {
	err = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		parts, err := c.Structure(folder, uid)
		if err != nil {
			return err
		}
		list := mailview.AttachmentParts(parts)
		if idx < 0 || idx >= len(list) {
			return mailconn.ErrNotFound
		}
		a := list[idx]
		body, err := c.FetchPart(folder, uid, a.Path, a.Encoding, maxAttachmentBytes)
		if err != nil {
			return err
		}
		name, ctype, data = a.Name, a.Type, body
		return nil
	})
	return name, ctype, data, err
}

var safeMediaType = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]*/[a-z0-9][a-z0-9.+-]*$`)

// mailAttachment télécharge une pièce jointe (toujours en téléchargement, jamais affichée dans la page).
func (s *Server) mailAttachment(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	folder, uid, ok := s.messageParams(w, r, sess, q)
	if !ok {
		return
	}
	idx, err := strconv.Atoi(q.Get("index"))
	if err != nil {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	// La pièce est téléchargée seule, d'après la structure annoncée par le serveur : un message de 100 Mo ne passe pas
	// par la mémoire pour en extraire un fichier de 2 Mo. Si le serveur ne s'y prête pas, le message entier est relu.
	name, ctype, data, err := s.fetchAttachmentPart(ctx, sess, acc, folder, uid, idx)
	if err != nil {
		_, m, ferr := s.fetchMessage(ctx, sess, acc, folder, uid, false)
		if ferr != nil {
			s.mailErr(w, r, sess, ferr)
			return
		}
		att, body, ok := m.Attachment(idx)
		if !ok {
			s.message(w, r, sess, http.StatusNotFound, "", "mail.err.not_found")
			return
		}
		name, ctype, data = att.Name, att.ContentType, body
	}
	ct := strings.ToLower(ctype)
	switch {
	case !safeMediaType.MatchString(ct), strings.Contains(ct, "html"), strings.Contains(ct, "xml"), strings.Contains(ct, "svg"), strings.Contains(ct, "javascript"):
		ct = "application/octet-stream"
	}
	name = safeFileName(name)
	h := w.Header()
	disposition := "attachment"
	if q.Get("inline") == "1" {
		// Aperçu : seulement pour les types sûrs, reconnus d'après le contenu lui-même (jamais d'après l'étiquette).
		if kind := previewKind(ctype, data); kind != "" {
			disposition = "inline"
			ct = http.DetectContentType(data)
			if kind == "text" {
				ct = "text/plain; charset=utf-8"
			}
			h.Set("X-Content-Type-Options", "nosniff")
			if kind == "pdf" { // le lecteur PDF du navigateur ne s'ouvre pas dans un document « sandbox » : CSP sans script ni ressource
				h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:")
				h.Set("Accept-Ranges", "bytes")
				h.Set("Content-Type", ct)
				h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
				h.Set("Cache-Control", "private, no-store")
				http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
				return
			}
		}
	}
	h.Set("Content-Type", ct)
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "private, no-store")
	h.Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data)) // gère aussi les requêtes partielles (reprise d'un téléchargement)
}

// mailAttachmentsZip télécharge toutes les pièces jointes d'un message dans une archive zip.
func (s *Server) mailAttachmentsZip(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.NotFound(w, r)
		return
	}
	folder, uid, ok := s.messageParams(w, r, sess, r.URL.Query())
	if !ok {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	_, m, err := s.fetchMessage(ctx, sess, acc, folder, uid, false)
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	if len(m.Attachments) == 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "mail.err.not_found")
		return
	}
	// « index » répété : seulement les pièces jointes cochées ; absent : toutes.
	want := map[int]bool{}
	for _, v := range r.URL.Query()["index"] {
		if n, err := strconv.Atoi(v); err == nil {
			want[n] = true
		}
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	used := map[string]int{}
	picked := 0
	for _, a := range m.Attachments {
		if len(want) > 0 && !want[a.Index] {
			continue
		}
		picked++
		name := safeFileName(a.Name)
		if n := used[strings.ToLower(name)]; n > 0 { // noms en double : « nom (2).ext »
			ext := path.Ext(name)
			name = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n+1, ext)
		}
		used[strings.ToLower(safeFileName(a.Name))]++
		f, err := zw.Create(name)
		if err != nil {
			s.serverError(w, r, sess, err)
			return
		}
		if _, err := f.Write(a.Data()); err != nil {
			s.serverError(w, r, sess, err)
			return
		}
	}
	if picked == 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "mail.err.not_found")
		return
	}
	if err := zw.Close(); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "pieces-jointes.zip"}))
	h.Set("Content-Length", strconv.Itoa(buf.Len()))
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "private, no-store")
	_, _ = w.Write(buf.Bytes())
}

// safeFileName retire d'un nom de fichier tout ce qui permettrait de sortir d'un dossier ou de casser un en-tête.
func safeFileName(n string) string {
	n = strings.NewReplacer("/", "_", "\\", "_", "\r", "", "\n", "", "\x00", "").Replace(n)
	n = strings.TrimSpace(strings.TrimLeft(n, "."))
	if n == "" {
		return "fichier"
	}
	return n
}

// errNotClearable : seuls la corbeille et les indésirables peuvent être vidés d'un coup.
var errNotClearable = errors.New("ce dossier ne peut pas être vidé")

// mailClear vide définitivement la corbeille ou le dossier des indésirables.
func (s *Server) mailClear(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	folder, back := r.PostFormValue("folder"), safeBack(r.PostFormValue("back"))
	if !mailconn.ValidFolderName(folder) {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var n int
	err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		folders, err := c.Folders(false)
		if err != nil {
			return err
		}
		trash := mailconn.ResolveTrash(folders, acc.Options.Folders.Trash)
		junk := mailconn.ResolveJunk(folders, acc.Options.Folders.Junk)
		// La boîte de réception n'est jamais vidée, même si le compte la désigne comme corbeille.
		if strings.EqualFold(folder, "INBOX") || (!strings.EqualFold(folder, trash) && !strings.EqualFold(folder, junk)) {
			return errNotClearable
		}
		n, err = c.ClearFolder(folder)
		return err
	})
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	s.log.Info("dossier vidé", "login", store.ReduceLogin(sess.Account.Login), "messages", n)
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// --- actions ---

func (s *Server) mailAction(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	action, folder, dest := r.PostFormValue("action"), r.PostFormValue("folder"), r.PostFormValue("dest")
	back, stay := safeBack(r.PostFormValue("back")), safeBack(r.PostFormValue("stay"))
	raw := r.PostForm["uid"]
	if v := r.PostFormValue("move_to"); v != "" { // menu « déplacer vers » : un bouton par dossier
		action, dest = "move", v
	}
	if v := r.PostFormValue("star_on"); v != "" { // étoile d'une ligne de la liste
		action, raw = "flag", []string{v}
	} else if v := r.PostFormValue("star_off"); v != "" {
		action, raw = "unflag", []string{v}
	}
	open := r.PostFormValue("current_uid") // message ouvert dans le volet de lecture
	if len(raw) == 0 && open != "" {
		raw = []string{open}
	}
	if (action == "readall" || action == "unreadall") && mailconn.ValidFolderName(folder) { // tout le dossier, sans sélection
		s.markAllSeen(w, r, sess, acc, folder, back, action == "readall")
		return
	}
	if v := r.PostFormValue("copy_to"); v != "" { // menu « copier vers »
		action, dest = "copy", v
	}
	if len(raw) == 0 && mailconn.ValidFolderName(folder) {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.no_selection")
		return
	}
	if !mailconn.ValidFolderName(folder) || len(raw) > maxActionUIDs {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	uids := make([]uint32, 0, len(raw))
	seen := map[uint32]bool{}
	add := func(u uint32) {
		if !seen[u] {
			seen[u] = true
			uids = append(uids, u)
		}
	}
	for _, v := range raw {
		u, ok := parseUID(v)
		if !ok {
			s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
			return
		}
		add(u)
		// Une conversation cochée entraîne tous ses messages pour supprimer, déplacer, copier, archiver : pas pour les
		// drapeaux, qui ne concernent que le message représentant.
		if slices.Contains([]string{"delete", "purge", "move", "copy", "archive", "spam"}, action) {
			if members := r.PostFormValue("th_" + v); members != "" {
				for _, m := range strings.Split(members, ",") {
					mu, ok := parseUID(m)
					if !ok || len(uids) >= maxActionUIDs {
						s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
						return
					}
					add(mu)
				}
			}
		}
	}
	if !slices.Contains([]string{"read", "unread", "flag", "unflag", "delete", "purge", "move", "copy", "archive", "spam", "notspam"}, action) ||
		((action == "move" || action == "copy") && (!mailconn.ValidFolderName(dest) || (action == "move" && dest == folder))) {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}

	ctx, cancel := s.mailCtx(r)
	defer cancel()
	err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		switch action {
		case "read":
			return c.StoreFlags(folder, uids, imap.StoreFlagsAdd, imap.FlagSeen)
		case "unread":
			return c.StoreFlags(folder, uids, imap.StoreFlagsDel, imap.FlagSeen)
		case "flag":
			return c.StoreFlags(folder, uids, imap.StoreFlagsAdd, imap.FlagFlagged)
		case "unflag":
			return c.StoreFlags(folder, uids, imap.StoreFlagsDel, imap.FlagFlagged)
		}
		folders, err := c.Folders(false)
		if err != nil {
			return err
		}
		// moveTo déplace vers un dossier de rôle (créé s'il n'existe pas) ; rien à faire si on y est déjà.
		trash := mailconn.ResolveTrash(folders, acc.Options.Folders.Trash)
		junk := mailconn.ResolveJunk(folders, acc.Options.Folders.Junk)
		moveTo := func(target string) error {
			if strings.EqualFold(folder, target) {
				return nil
			}
			if err := c.EnsureFolder(target); err != nil {
				return err
			}
			// Vers la corbeille ou les indésirables, les messages sont d'abord marqués lus (sinon leurs compteurs de
			// non lus grimpent), comme dans RainLoop.
			if strings.EqualFold(target, trash) || strings.EqualFold(target, junk) {
				if err := c.StoreFlags(folder, uids, imap.StoreFlagsAdd, imap.FlagSeen); err != nil {
					return err
				}
			}
			return c.Move(folder, uids, target)
		}
		switch action {
		case "move":
			if !slices.ContainsFunc(folders, func(f mailconn.Folder) bool { return f.Name == dest && f.Selectable() }) {
				return fmt.Errorf("%w : dossier %q", mailconn.ErrNotFound, dest)
			}
			return moveTo(dest)
		case "copy":
			if !slices.ContainsFunc(folders, func(f mailconn.Folder) bool { return f.Name == dest && f.Selectable() }) {
				return fmt.Errorf("%w : dossier %q", mailconn.ErrNotFound, dest)
			}
			return c.Copy(folder, uids, dest)
		case "archive":
			return moveTo(mailconn.ResolveArchive(folders, acc.Options.Folders.Archive))
		case "spam":
			return moveTo(junk)
		case "notspam":
			return moveTo("INBOX")
		case "purge": // Maj+Suppr : suppression définitive, sans passer par la corbeille
			return c.Expunge(folder, uids)
		default: // delete : corbeille, ou suppression définitive si le dossier est déjà la corbeille
			if strings.EqualFold(folder, trash) {
				return c.Expunge(folder, uids)
			}
			return moveTo(trash)
		}
	})
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	s.log.Info("action sur des messages", "login", store.ReduceLogin(sess.Account.Login), "action", action, "messages", len(uids))
	next := back
	switch action {
	case "read", "unread", "flag", "unflag":
		if stay != "/" {
			next = stay
		}
	default: // déplacement ou suppression : le message ouvert disparaît, sauf si l'action vise d'autres messages
		if stay != "/" && open != "" && !slices.Contains(raw, open) {
			next = stay
		} else if open != "" && len(raw) == 1 && raw[0] == open {
			// Seul le message ouvert vient de partir : ouvrir son voisin, comme RainLoop (suivant, sinon précédent).
			if n := safeBack(r.PostFormValue("open_next")); n != "/" {
				next = n
			}
		}
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// QuotaView est l'occupation de la boîte, prête à afficher.
type QuotaView struct {
	Used, Limit string
	Percent     int
	Full        bool // 90 % ou plus
}

func quotaView(used, limit int64, lang string) *QuotaView {
	pct := int(float64(used) / float64(limit) * 100)
	return &QuotaView{Used: humanSize(used, lang), Limit: humanSize(limit, lang), Percent: min(pct, 100), Full: pct >= 90}
}

func (s *Server) openFolders(sess *session.Session, account string) map[string]bool {
	sess.Mu.Lock()
	set := sess.Account.Data.Settings
	sess.Mu.Unlock()
	return openFolderSet(set, account)
}

// markAllSeen marque tous les messages d'un dossier comme lus (seen) ou comme non lus.
func (s *Server) markAllSeen(w http.ResponseWriter, r *http.Request, sess *session.Session, acc userdata.MailAccount, folder, back string, seen bool) {
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	n := 0
	if err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		if seen {
			n, err = c.MarkAllRead(folder)
		} else {
			n, err = c.MarkAllUnread(folder)
		}
		return
	}); err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	s.log.Info("tous les messages marqués", "login", store.ReduceLogin(sess.Account.Login), "lu", seen, "messages", n)
	http.Redirect(w, r, back, http.StatusSeeOther)
}

var subjectPrefixRe = regexp.MustCompile(`(?i)^((?:(?:re|fwd?|tr|sv|aw)\s*(?:\[\d+\]|\(\d+\))?\s*:\s*)+)(.*)$`)

// splitSubject sépare les préfixes de réponse ou de transfert (« Re: », « Fwd: », « RE[2]: »…) du reste de l'objet.
func splitSubject(s string) (prefix, rest string) {
	if m := subjectPrefixRe.FindStringSubmatch(s); m != nil && m[2] != "" {
		return m[1], m[2]
	}
	return "", s
}

const maxEventCards = 3

// EventView est l'aperçu d'une invitation iCalendar jointe à un message.
type EventView struct {
	Summary, When, Organizer, Attendees string
	GoogleURL, OutlookURL               string
}

// eventWhen donne le début de l'événement, « 05/11/2026 , 14:00 » (jour entier : « 05/11/2026 »), comme le plugin d'origine.
func eventWhen(e ical.Event, lang string, loc *time.Location) string {
	if !e.HasStart {
		return ""
	}
	st := e.Start
	if st.Location() == time.UTC && !e.AllDay { // heure UTC : dans le fuseau configuré ; un TZID reste dans son fuseau
		st = st.In(loc)
	}
	if e.AllDay {
		return st.Format("02/01/2006")
	}
	return st.Format("02/01/2006 , 15:04")
}

func eventView(e ical.Event, lang string, loc *time.Location) EventView {
	v := EventView{Summary: e.Summary, When: eventWhen(e, lang, loc), GoogleURL: e.GoogleURL(), OutlookURL: e.OutlookURL()}
	if v.Summary == "" {
		v.Summary = "Invitation"
	}
	v.Organizer = e.Organizer.Label()
	var names []string
	for _, a := range e.Attendees {
		names = append(names, a.Label())
	}
	v.Attendees = strings.Join(names, ", ")
	return v
}
