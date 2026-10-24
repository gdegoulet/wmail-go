package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/mailconn"
	"wmail-go/internal/session"
	"wmail-go/internal/store"
	"wmail-go/internal/userdata"
)

// FolderSetting est une ligne de la liste des dossiers (réglages).
type FolderSetting struct {
	mailconn.Folder
	Indent     int
	Role       string // clé de traduction du rôle (vide : aucun)
	Subscribed bool
	Protected  bool // INBOX : ni renommée ni supprimée
	Leaf       string
	Parent     string
}

// RoleChoice est une liste de choix pour un dossier système.
type RoleChoice struct {
	Field, LabelKey string
	Current         string // valeur enregistrée (vide : automatique)
	Resolved        string // dossier effectivement utilisé
	AllowNone       bool
}

// FoldersView alimente la page des dossiers.
// CheckRow est un dossier proposé à la surveillance des nouveaux messages.
type CheckRow struct {
	Name    string
	Checked bool
}

type FoldersView struct {
	Rows         []FolderSetting
	Names        []string // noms de tous les dossiers sélectionnables, pour les listes
	Roles        []RoleChoice
	SubKnown     bool
	Check        []CheckRow // dossiers pouvant être surveillés
	CheckFull    bool
	DelimDefault string
}

func (s *Server) foldersPage(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, errKey string, errArg ...any) {
	acc, ok := s.settingsBox(r, sess)
	if !ok {
		if r.FormValue("box") != "" {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var folders []mailconn.Folder
	var sub map[string]bool
	var known bool
	err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		if folders, err = c.Folders(true); err != nil {
			return err
		}
		sub, known, err = c.Subscribed()
		return err
	})
	p := &Page{TitleKey: "set.folders", Section: "folders", ActiveID: acc.ID, ErrorKey: errKey, Accounts: accountViews(s.mailAccounts(sess)), BoxEmail: acc.Email, Path: "/settings/folders"}
	if len(errArg) > 0 {
		p.ErrorArg = errArg[0]
	}
	if err != nil {
		var te *mailconn.TestError
		if errors.As(err, &te) {
			p.FolderError = &TestResult{Kind: te.Kind, Detail: te.Detail}
			s.render(w, r, sess, http.StatusBadGateway, "folders_settings", p)
			return
		}
		s.serverError(w, r, sess, err)
		return
	}
	v := &FoldersView{SubKnown: known, DelimDefault: "/"}
	for _, f := range folders {
		if f.Delim != 0 {
			v.DelimDefault = string(f.Delim)
		}
		row := FolderSetting{Folder: f, Indent: min(f.Depth(), 5), Subscribed: !known || sub[f.Name], Protected: strings.EqualFold(f.Name, "INBOX"), Leaf: f.Leaf()}
		switch {
		case row.Protected:
			row.Role = "folder.inbox"
		case f.HasAttr(imap.MailboxAttrSent):
			row.Role = "folder.sent"
		case f.HasAttr(imap.MailboxAttrDrafts):
			row.Role = "folder.drafts"
		case f.HasAttr(imap.MailboxAttrJunk):
			row.Role = "folder.junk"
		case f.HasAttr(imap.MailboxAttrTrash):
			row.Role = "folder.trash"
		case f.HasAttr(imap.MailboxAttrArchive):
			row.Role = "folder.archive"
		}
		if f.Selectable() {
			v.Names = append(v.Names, f.Name)
			if !strings.EqualFold(f.Name, "INBOX") {
				on := slices.Contains(acc.CheckFolders, f.Name)
				v.Check = append(v.Check, CheckRow{Name: f.Name, Checked: on})
			}
		}
		v.Rows = append(v.Rows, row)
	}
	of := acc.Options.Folders
	v.Roles = []RoleChoice{
		{Field: "sent", LabelKey: "folder.sent", Current: of.Sent, Resolved: mailconn.ResolveSent(folders, of.Sent), AllowNone: true},
		{Field: "drafts", LabelKey: "folder.drafts", Current: of.Drafts, Resolved: mailconn.ResolveDrafts(folders, of.Drafts), AllowNone: true},
		{Field: "junk", LabelKey: "folder.junk", Current: of.Junk, Resolved: mailconn.ResolveJunk(folders, of.Junk)},
		{Field: "trash", LabelKey: "folder.trash", Current: of.Trash, Resolved: mailconn.ResolveTrash(folders, of.Trash)},
		{Field: "archive", LabelKey: "folder.archive", Current: of.Archive, Resolved: mailconn.ResolveArchive(folders, of.Archive)},
	}
	p.FoldersView = v
	s.render(w, r, sess, status, "folders_settings", p)
}

func (s *Server) foldersGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.foldersPage(w, r, sess, http.StatusOK, "")
}

// foldersAction crée, renomme, supprime un dossier ou change son abonnement.
func (s *Server) foldersAction(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.settingsBox(r, sess)
	if !ok {
		if r.FormValue("box") != "" {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	action, name := r.PostFormValue("action"), r.PostFormValue("name")
	delim := r.PostFormValue("delim")
	if len(delim) > 1 || strings.ContainsAny(delim, "*%\r\n") {
		delim = ""
	}
	if delim == "" {
		delim = "/"
	}
	// join compose un nom complet à partir d'un dossier parent et d'un nom simple.
	join := func(parent, leaf string) (string, bool) {
		leaf = strings.TrimSpace(leaf)
		if leaf == "" || strings.Contains(leaf, delim) || strings.ContainsAny(leaf, "*%") || leaf == "." || leaf == ".." {
			return "", false
		}
		if parent == "" {
			return leaf, mailconn.ValidFolderName(leaf)
		}
		full := parent + delim + leaf
		return full, mailconn.ValidFolderName(parent) && mailconn.ValidFolderName(full)
	}
	var run func(c *mailconn.Conn) error
	switch action {
	case "create":
		full, ok := join(r.PostFormValue("parent"), r.PostFormValue("new_name"))
		if !ok {
			s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.name")
			return
		}
		run = func(c *mailconn.Conn) error { return c.CreateFolder(full) }
	case "rename":
		parent := ""
		if i := strings.LastIndex(name, delim); i > 0 {
			parent = name[:i]
		}
		full, ok := join(parent, r.PostFormValue("new_name"))
		if !ok || !mailconn.ValidFolderName(name) || strings.EqualFold(name, "INBOX") {
			s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.name")
			return
		}
		run = func(c *mailconn.Conn) error { return c.RenameFolder(name, full) }
	case "delete":
		if !mailconn.ValidFolderName(name) || strings.EqualFold(name, "INBOX") {
			s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.protected")
			return
		}
		run = func(c *mailconn.Conn) error { return c.DeleteFolder(name) }
	case "subscribe", "unsubscribe":
		if !mailconn.ValidFolderName(name) {
			s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.name")
			return
		}
		run = func(c *mailconn.Conn) error { return c.SetSubscribed(name, action == "subscribe") }
	default:
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	if err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), run); err != nil {
		s.log.Warn("action sur un dossier refusée", "login", store.ReduceLogin(sess.Account.Login), "action", action, "err", shortErr(err))
		s.foldersPage(w, r, sess, http.StatusBadGateway, "folders.err.server", shortErr(err))
		return
	}
	// Un dossier désigné comme corbeille, envoyés… qui a été renommé ou supprimé n'est plus valable.
	if action == "delete" || action == "rename" {
		s.forgetRoleFolder(sess, acc.ID, name)
	}
	s.setFlash(sess, "folders.done."+action)
	if stay := r.PostFormValue("return_to"); stay != "" { // créé depuis le panneau de gauche : on y retourne
		http.Redirect(w, r, safeBack(stay), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/folders?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
}

// forgetRoleFolder retire des rôles enregistrés un dossier qui n'existe plus sous ce nom.
func (s *Server) forgetRoleFolder(sess *session.Session, accID, name string) {
	list := s.mailAccounts(sess)
	for i, a := range list {
		if a.ID != accID {
			continue
		}
		f := &list[i].Options.Folders
		changed := false
		for _, p := range []*string{&f.Trash, &f.Archive, &f.Junk, &f.Sent, &f.Drafts} {
			if *p == name {
				*p, changed = "", true
			}
		}
		if changed {
			_ = s.saveAccounts(sess, list)
		}
	}
}

// foldersSystem enregistre le choix des dossiers système de la boîte active.
func (s *Server) foldersSystem(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.settingsBox(r, sess)
	if !ok {
		if r.FormValue("box") != "" {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var folders []mailconn.Folder
	if err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		folders, err = c.Folders(false)
		return
	}); err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	valid := func(v string, allowNone bool) (string, bool) {
		switch {
		case v == "":
			return "", true
		case v == userdata.NoFolder:
			return v, allowNone
		}
		return v, slices.ContainsFunc(folders, func(f mailconn.Folder) bool { return f.Name == v && f.Selectable() })
	}
	var f userdata.Folders
	for _, d := range []struct {
		dst       *string
		field     string
		allowNone bool
	}{{&f.Sent, "sent", true}, {&f.Drafts, "drafts", true}, {&f.Junk, "junk", false}, {&f.Trash, "trash", false}, {&f.Archive, "archive", false}} {
		v, ok := valid(r.PostFormValue(d.field), d.allowNone)
		if !ok {
			s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.system")
			return
		}
		*d.dst = v
	}
	list := s.mailAccounts(sess)
	for i := range list {
		if list[i].ID == acc.ID {
			list[i].Options.Folders = f
		}
	}
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.setFlash(sess, "folders.done.system")
	http.Redirect(w, r, "/settings/folders?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
}

// foldersCheck enregistre les dossiers dont le compteur de non lus est relevé en arrière-plan.
func (s *Server) foldersCheck(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.settingsBox(r, sess)
	if !ok {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	_ = r.ParseForm()
	want := r.PostForm["check"]
	if len(want) > userdata.MaxCheckFolders {
		s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.check_max", userdata.MaxCheckFolders)
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var folders []mailconn.Folder
	if err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		folders, err = c.Folders(false)
		return
	}); err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	var keep []string
	for _, n := range want {
		if !slices.ContainsFunc(folders, func(f mailconn.Folder) bool { return f.Name == n && f.Selectable() }) {
			s.foldersPage(w, r, sess, http.StatusBadRequest, "folders.err.system")
			return
		}
		if !slices.Contains(keep, n) {
			keep = append(keep, n)
		}
	}
	list := s.mailAccounts(sess)
	for i := range list {
		if list[i].ID == acc.ID {
			list[i].CheckFolders = keep
		}
	}
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.setFlash(sess, "folders.done.check")
	http.Redirect(w, r, "/settings/folders?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
}

var _ = context.Background
