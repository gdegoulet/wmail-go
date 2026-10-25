package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdegoulet/wmail-go/internal/compose"
	"github.com/gdegoulet/wmail-go/internal/mailconn"
	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/store"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const (
	maxMailAccounts = 20
	testTimeout     = 20 * time.Second
)

// AccountView est une ligne de la liste des comptes mail.
type AccountView struct {
	ID, Name, Email, IMAPHost string
}

// FolderRow est un dossier prêt à afficher.
type FolderRow struct {
	mailconn.Folder
	DepthClass                   int // 0 à 5, pour la classe CSS d'indentation
	URL                          string
	Active                       bool
	System                       bool   // boîte de réception ou dossier à usage spécial
	Role                         string // rôle système (inbox, sent, drafts, junk, trash, archive) ou vide
	Key                          string // identifiant stable du dossier (son nom complet)
	Label                        string // nom affiché dans l'arbre (dernier segment, sans préfixe d'espace de noms)
	Depth                        int
	Badge                        int  // nombre affiché dans la pastille (0 : aucune) ; règle RainLoop par rôle
	Bold                         bool // nom en gras : des messages non lus, hors dossiers à compteur masqué
	SepBefore                    bool // menu de déplacement : séparateur avant ce dossier
	SubUnread                    bool // un sous-dossier contient des messages non lus
	HasChildren, Open, Collapsed bool // arbre : sous-dossiers, niveau déplié, ligne cachée par un ancêtre replié
}

// TestResult est le résultat d'un test de connexion.
type TestResult struct {
	OK     bool
	Kind   string
	Detail string
}

// settingsBox renvoie la boîte mail visée par une page de réglages : celle du paramètre « box » (adresse ou formulaire),
// à défaut la boîte active. Les réglages de dossiers et de filtres appartiennent à une boîte mail précise.
func (s *Server) settingsBox(r *http.Request, sess *session.Session) (userdata.MailAccount, bool) {
	list := s.mailAccounts(sess)
	if id := r.FormValue("box"); id != "" {
		a, ok := findAccount(list, id)
		return a, ok
	}
	a, _, ok := s.activeAccount(sess)
	return a, ok
}

// FolderDelim renvoie le séparateur de niveaux de dossiers du serveur (« / » à défaut).
func (p *Page) FolderDelim() string {
	for _, f := range p.Folders {
		if f.Delim != 0 {
			return string(f.Delim)
		}
	}
	return "/"
}

// FolderName renvoie le nom affiché : libellé traduit pour les dossiers système, sinon le nom dans l'arbre.
func (p *Page) FolderName(f FolderRow) string {
	switch f.Role {
	case roleInbox:
		return p.T("folder.inbox")
	case roleSent:
		return p.T("folder.sent")
	case roleDrafts:
		return p.T("folder.drafts")
	case roleJunk:
		return p.T("folder.junk")
	case roleTrash:
		return p.T("folder.trash")
	case roleArchive:
		return p.T("folder.archive")
	}
	if f.Label != "" {
		return f.Label
	}
	return f.Leaf()
}

// AccountForm contient les champs du formulaire de compte mail.
type AccountForm struct {
	ID, Name, Email, DisplayName                              string
	IMAPHost, IMAPPort, IMAPSecurity, IMAPLogin, IMAPPassword string
	SMTPHost, SMTPPort, SMTPSecurity, SMTPLogin, SMTPPassword string
	IMAPVerify, SMTPVerify, BlockRemote, ProxyImages          bool
	IMAPLegacy, SMTPLegacy                                    bool
	TrashFolder                                               string
	ArchiveFolder, JunkFolder                                 string
	SentFolder, DraftsFolder, Signature                       string
	ReplyTo, Bcc                                              string
	SieveDisabled, SieveVerify, SieveLegacy                   bool
	SieveHost, SievePort, SieveSecurity, SieveLogin           string
	SievePassword                                             string
	HasSievePassword                                          bool
	SignatureBefore                                           bool
	IsNew, HasIMAPPassword, HasSMTPPassword                   bool
}

func defaultForm() *AccountForm {
	return &AccountForm{
		IsNew: true, IMAPPort: "993", IMAPSecurity: mailconn.SecurityTLS, IMAPVerify: true,
		SMTPPort: "587", SMTPSecurity: mailconn.SecuritySTARTTLS, SMTPVerify: true, BlockRemote: true, SieveVerify: true,
	}
}

func formFromAccount(a userdata.MailAccount) *AccountForm {
	return &AccountForm{
		ID: a.ID, Name: a.Name, Email: a.Email, DisplayName: a.DisplayName,
		IMAPHost: a.IMAP.Host, IMAPPort: strconv.Itoa(a.IMAP.Port), IMAPSecurity: a.IMAP.Security, IMAPLogin: a.IMAP.Login, IMAPVerify: a.IMAP.VerifyCert, IMAPLegacy: a.IMAP.LegacyTLS,
		SMTPHost: a.SMTP.Host, SMTPPort: strconv.Itoa(a.SMTP.Port), SMTPSecurity: a.SMTP.Security, SMTPLogin: a.SMTP.Login, SMTPVerify: a.SMTP.VerifyCert, SMTPLegacy: a.SMTP.LegacyTLS,
		BlockRemote: a.Options.BlockRemoteContent, ProxyImages: a.Options.ProxyRemoteImages, TrashFolder: a.Options.Folders.Trash, SentFolder: a.Options.Folders.Sent, DraftsFolder: a.Options.Folders.Drafts, Signature: a.Signature, SignatureBefore: a.SignatureBefore, ReplyTo: a.ReplyTo, Bcc: a.Bcc,
		ArchiveFolder: a.Options.Folders.Archive, JunkFolder: a.Options.Folders.Junk,
		HasIMAPPassword: a.IMAP.Password != "", HasSMTPPassword: a.SMTP.Password != "",
		SieveDisabled: a.Sieve.Disabled, SieveHost: a.Sieve.Host, SievePort: itoaOrEmpty(a.Sieve.Port), SieveSecurity: a.Sieve.Security, SieveLogin: a.Sieve.Login,
		SieveVerify: a.Sieve.VerifyCert || a.Sieve.Host == "", SieveLegacy: a.Sieve.LegacyTLS, HasSievePassword: a.Sieve.Password != "",
	}
}

func itoaOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func formFromRequest(r *http.Request) *AccountForm {
	v := func(k string) string { return strings.TrimSpace(r.PostFormValue(k)) }
	return &AccountForm{
		ID: v("id"), Name: v("name"), Email: v("email"), DisplayName: v("display_name"),
		IMAPHost: v("imap_host"), IMAPPort: v("imap_port"), IMAPSecurity: v("imap_security"), IMAPLogin: v("imap_login"),
		IMAPPassword: r.PostFormValue("imap_password"), IMAPVerify: r.PostFormValue("imap_verify") != "", IMAPLegacy: r.PostFormValue("imap_legacy") != "",
		SMTPHost: v("smtp_host"), SMTPPort: v("smtp_port"), SMTPSecurity: v("smtp_security"), SMTPLogin: v("smtp_login"),
		SMTPPassword: r.PostFormValue("smtp_password"), SMTPVerify: r.PostFormValue("smtp_verify") != "", SMTPLegacy: r.PostFormValue("smtp_legacy") != "",
		BlockRemote: r.PostFormValue("block_remote") != "", ProxyImages: r.PostFormValue("proxy_images") != "", TrashFolder: v("trash_folder"),
		SieveDisabled: r.PostFormValue("sieve_disabled") != "", SieveHost: strings.ToLower(v("sieve_host")), SievePort: v("sieve_port"), SieveSecurity: v("sieve_security"),
		SieveLogin: v("sieve_login"), SievePassword: r.PostFormValue("sieve_password"), SieveVerify: r.PostFormValue("sieve_verify") != "", SieveLegacy: r.PostFormValue("sieve_legacy") != "",
		ArchiveFolder: v("archive_folder"), JunkFolder: v("junk_folder"), SentFolder: v("sent_folder"), DraftsFolder: v("drafts_folder"),
		Signature: strings.ReplaceAll(r.PostFormValue("signature"), "\r\n", "\n"), SignatureBefore: r.PostFormValue("signature_before") != "",
		ReplyTo: strings.TrimSpace(r.PostFormValue("reply_to")), Bcc: strings.TrimSpace(r.PostFormValue("bcc")),
	}
}

func parsePort(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1 && n <= 65535
}

// buildAccount valide le formulaire et construit le compte. existing sert à conserver les mots de passe laissés vides.
func buildAccount(f *AccountForm, existing *userdata.MailAccount) (userdata.MailAccount, []string) {
	var errs []string
	acc := userdata.MailAccount{ID: f.ID, Name: f.Name, DisplayName: f.DisplayName}

	if addr, err := mail.ParseAddress(f.Email); err != nil || addr.Address != f.Email || len(f.Email) > 254 {
		errs = append(errs, "accounts.err.email")
	}
	acc.Email = f.Email
	if acc.Name == "" {
		acc.Name = f.Email
	}
	for _, v := range []string{f.ReplyTo, f.Bcc} {
		if v == "" {
			continue
		}
		if _, err := compose.ParseList(v); err != nil || len(v) > 500 {
			errs = append(errs, "accounts.err.email")
			break
		}
	}
	if len([]rune(acc.Name)) > 100 || len([]rune(acc.DisplayName)) > 100 || len(f.TrashFolder) > 200 || len(f.ArchiveFolder) > 200 || len(f.JunkFolder) > 200 || len(f.SentFolder) > 200 || len(f.DraftsFolder) > 200 || len(f.Signature) > 5000 {
		errs = append(errs, "accounts.err.too_long")
	}

	server := func(host, port, sec, login, pw string, verify, legacy bool, prefix string, old *userdata.Server) userdata.Server {
		sv := userdata.Server{Host: strings.ToLower(host), Security: sec, Login: login, Password: pw, VerifyCert: verify, LegacyTLS: legacy}
		if !mailconn.ValidHost(sv.Host) {
			errs = append(errs, "accounts.err."+prefix+"_host")
		}
		if n, ok := parsePort(port); ok {
			sv.Port = n
		} else {
			errs = append(errs, "accounts.err."+prefix+"_port")
		}
		if !mailconn.ValidSecurity(sec) {
			errs = append(errs, "accounts.err."+prefix+"_security")
		}
		if sv.Password == "" && old != nil {
			sv.Password = old.Password
		}
		return sv
	}
	var oldIMAP, oldSMTP *userdata.Server
	if existing != nil {
		oldIMAP, oldSMTP = &existing.IMAP, &existing.SMTP
	}
	acc.IMAP = server(f.IMAPHost, f.IMAPPort, f.IMAPSecurity, f.IMAPLogin, f.IMAPPassword, f.IMAPVerify, f.IMAPLegacy, "imap", oldIMAP)
	if acc.IMAP.Login == "" {
		errs = append(errs, "accounts.err.imap_login")
	}
	if acc.IMAP.Password == "" {
		errs = append(errs, "accounts.err.imap_password")
	}
	// SMTP : identifiants laissés vides = ceux de l'IMAP (comme Thunderbird).
	smtpLogin, smtpPW := f.SMTPLogin, f.SMTPPassword
	if smtpLogin == "" {
		smtpLogin = acc.IMAP.Login
	}
	if smtpPW == "" && (oldSMTP == nil || oldSMTP.Password == "" || f.SMTPLogin == "") {
		smtpPW = acc.IMAP.Password
	}
	acc.SMTP = server(f.SMTPHost, f.SMTPPort, f.SMTPSecurity, smtpLogin, smtpPW, f.SMTPVerify, f.SMTPLegacy, "smtp", oldSMTP)

	acc.Options = userdata.MailOptions{
		BlockRemoteContent: f.BlockRemote, ProxyRemoteImages: f.ProxyImages,
		ImageProxy: false,
		Folders:    userdata.Folders{Trash: f.TrashFolder, Archive: f.ArchiveFolder, Junk: f.JunkFolder, Sent: f.SentFolder, Drafts: f.DraftsFolder},
	}
	acc.Sieve = userdata.SieveConfig{Disabled: f.SieveDisabled, Host: f.SieveHost, Security: f.SieveSecurity, Login: f.SieveLogin, Password: f.SievePassword, VerifyCert: f.SieveVerify, LegacyTLS: f.SieveLegacy}
	if f.SieveHost != "" && !mailconn.ValidHost(f.SieveHost) {
		errs = append(errs, "accounts.err.sieve_host")
	}
	if f.SievePort != "" {
		if n, ok := parsePort(f.SievePort); ok {
			acc.Sieve.Port = n
		} else {
			errs = append(errs, "accounts.err.sieve_port")
		}
	}
	if f.SieveSecurity != "" && !mailconn.ValidSecurity(f.SieveSecurity) {
		errs = append(errs, "accounts.err.sieve_security")
	}
	if len(f.SieveLogin) > 254 || len(f.SievePassword) > 1000 {
		errs = append(errs, "accounts.err.too_long")
	}
	if acc.Sieve.Password == "" && existing != nil && f.SieveLogin == existing.Sieve.Login {
		acc.Sieve.Password = existing.Sieve.Password
	}
	acc.Signature, acc.SignatureBefore = f.Signature, f.SignatureBefore
	acc.ReplyTo, acc.Bcc = f.ReplyTo, f.Bcc
	if existing != nil {
		acc.Options.ImageProxy = existing.Options.ImageProxy
		acc.Identities = existing.Identities
	}
	return acc, errs
}

func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func imapEndpoint(a userdata.MailAccount) mailconn.Endpoint {
	return mailconn.Endpoint{Host: a.IMAP.Host, Port: a.IMAP.Port, Security: a.IMAP.Security, Login: a.IMAP.Login, Password: a.IMAP.Password, VerifyCert: a.IMAP.VerifyCert, LegacyTLS: a.IMAP.LegacyTLS}
}

// sieveEndpoint renvoie l'accès ManageSieve de la boîte ; les champs vides prennent les valeurs de l'IMAP.
func sieveEndpoint(a userdata.MailAccount) mailconn.Endpoint {
	c := a.Sieve
	e := mailconn.Endpoint{Host: c.Host, Port: c.Port, Security: c.Security, Login: c.Login, Password: c.Password, VerifyCert: c.VerifyCert, LegacyTLS: c.LegacyTLS}
	if e.Host == "" { // même serveur que l'IMAP : on reprend aussi sa vérification de certificat et son mode ancien
		e.Host, e.VerifyCert, e.LegacyTLS = a.IMAP.Host, a.IMAP.VerifyCert, a.IMAP.LegacyTLS
	}
	if e.Port == 0 {
		e.Port = mailconn.DefaultSievePort
	}
	if e.Security == "" {
		e.Security = mailconn.SecuritySTARTTLS
	}
	if e.Login == "" {
		e.Login, e.Password = a.IMAP.Login, a.IMAP.Password
	}
	return e
}

func smtpEndpoint(a userdata.MailAccount) mailconn.Endpoint {
	return mailconn.Endpoint{Host: a.SMTP.Host, Port: a.SMTP.Port, Security: a.SMTP.Security, Login: a.SMTP.Login, Password: a.SMTP.Password, VerifyCert: a.SMTP.VerifyCert, LegacyTLS: a.SMTP.LegacyTLS}
}

func toResult(err error) *TestResult {
	if err == nil {
		return &TestResult{OK: true}
	}
	te := mailconn.Classify(err)
	return &TestResult{Kind: te.Kind, Detail: te.Detail}
}

// testAccount teste l'IMAP, le SMTP et (si les filtres sont actifs) ManageSieve, en parallèle.
func (s *Server) testAccount(ctx context.Context, a userdata.MailAccount) (imapRes, smtpRes, sieveRes *TestResult) {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); imapRes = toResult(s.guard.TestIMAP(ctx, imapEndpoint(a))) }()
	go func() { defer wg.Done(); smtpRes = toResult(s.guard.TestSMTP(ctx, smtpEndpoint(a))) }()
	if !a.Sieve.Disabled {
		wg.Add(1)
		go func() { defer wg.Done(); sieveRes = toResult(s.guard.TestSieve(ctx, sieveEndpoint(a))) }()
	}
	wg.Wait()
	return imapRes, smtpRes, sieveRes
}

// --- accès aux comptes de la session ---

func (s *Server) mailAccounts(sess *session.Session) []userdata.MailAccount {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return slices.Clone(sess.Account.Data.MailAccounts)
}

func findAccount(list []userdata.MailAccount, id string) (userdata.MailAccount, bool) {
	for _, a := range list {
		if a.ID == id {
			return a, true
		}
	}
	return userdata.MailAccount{}, false
}

// saveAccounts remplace la liste des comptes mail et réécrit le fichier chiffré ; la liste est restaurée en cas d'échec.
func (s *Server) saveAccounts(sess *session.Session, list []userdata.MailAccount) error {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	prev := sess.Account.Data.MailAccounts
	sess.Account.Data.MailAccounts = list
	if err := s.store.SaveData(sess.Account); err != nil {
		sess.Account.Data.MailAccounts = prev
		return err
	}
	if _, ok := findAccount(list, sess.ActiveMailAccount); !ok {
		sess.ActiveMailAccount = ""
	}
	return nil
}

func accountViews(list []userdata.MailAccount) []AccountView {
	out := make([]AccountView, len(list))
	for i, a := range list {
		out[i] = AccountView{ID: a.ID, Name: a.Name, Email: a.Email, IMAPHost: a.IMAP.Host}
	}
	return out
}

// --- pages ---

func (s *Server) accountsList(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.render(w, r, sess, http.StatusOK, "accounts",
		&Page{TitleKey: "accounts.title", Accounts: accountViews(s.mailAccounts(sess))})
}

func (s *Server) accountForm(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, p *Page) {
	p.TitleKey = "accounts.form.title"
	p.FirstAccount = len(s.mailAccounts(sess)) == 0
	p.FoldersView = s.accountFolderChoices(r, sess, p.Form)
	s.render(w, r, sess, status, "account_form", p)
}

func (s *Server) accountNew(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.accountForm(w, r, sess, http.StatusOK, &Page{Form: defaultForm()})
}

func (s *Server) accountEdit(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := findAccount(s.mailAccounts(sess), r.PathValue("id"))
	if !ok {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	s.accountForm(w, r, sess, http.StatusOK, &Page{Form: formFromAccount(acc)})
}

// accountSave traite les boutons « Test » et « Enregistrer ».
// Le premier compte n'est enregistré que si le test réussit.
func (s *Server) accountSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	form := formFromRequest(r)
	list := s.mailAccounts(sess)
	var existing *userdata.MailAccount
	if form.ID != "" {
		a, ok := findAccount(list, form.ID)
		if !ok {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		existing = &a
	} else {
		form.IsNew = true
	}
	if existing != nil {
		form.HasIMAPPassword = existing.IMAP.Password != ""
		form.HasSMTPPassword = existing.SMTP.Password != ""
	}

	acc, errs := buildAccount(form, existing)
	if existing == nil && len(list) >= maxMailAccounts {
		errs = append(errs, "accounts.err.limit")
	}
	// Même adresse sur le même serveur IMAP : c'est la même boîte, déjà ajoutée.
	for _, a := range list {
		if a.ID != acc.ID && strings.EqualFold(a.Email, acc.Email) && strings.EqualFold(a.IMAP.Host, acc.IMAP.Host) {
			errs = append(errs, "accounts.err.duplicate")
			break
		}
	}
	if len(errs) > 0 {
		s.accountForm(w, r, sess, http.StatusBadRequest, &Page{Form: form, FormErrorKeys: errs})
		return
	}

	action := r.PostFormValue("action")
	first := len(list) == 0
	var imapRes, smtpRes, sieveRes *TestResult
	if action == "test" || first {
		rule := s.cfg.RateLimit.MailTest
		ipKey, userKey := s.limitKeys("mailtest", r, sess.Account.Login)
		if wait, blocked := s.checkLimits(rule, ipKey, userKey); blocked {
			s.rateLimited(w, r, sess, wait)
			return
		}
		s.addFailure(rule, ipKey, userKey) // chaque test compte, réussi ou non
		imapRes, smtpRes, sieveRes = s.testAccount(r.Context(), acc)
		s.log.Info("test de compte mail", "login", store.ReduceLogin(sess.Account.Login),
			"imap_ok", imapRes.OK, "smtp_ok", smtpRes.OK, "imap_kind", imapRes.Kind, "smtp_kind", smtpRes.Kind)
		if action == "test" {
			s.accountForm(w, r, sess, http.StatusOK, &Page{Form: form, IMAPResult: imapRes, SMTPResult: smtpRes, SieveResult: sieveRes})
			return
		}
		if !imapRes.OK || !smtpRes.OK {
			s.accountForm(w, r, sess, http.StatusUnprocessableEntity, &Page{
				Form: form, IMAPResult: imapRes, SMTPResult: smtpRes, SieveResult: sieveRes, FormErrorKeys: []string{"accounts.err.first_must_pass"}})
			return
		}
	}

	if existing == nil {
		id, err := newID()
		if err != nil {
			s.serverError(w, r, sess, err)
			return
		}
		acc.ID = id
		list = append(list, acc)
	} else {
		for i := range list {
			if list[i].ID == acc.ID {
				list[i] = acc // le compte garde sa place dans la liste
			}
		}
	}
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	sess.Mu.Lock()
	if sess.ActiveMailAccount == "" {
		sess.ActiveMailAccount = acc.ID
	}
	sess.Mu.Unlock()
	s.log.Info("compte mail enregistré", "login", store.ReduceLogin(sess.Account.Login), "account", acc.ID, "nouveau", existing == nil)
	if existing == nil {
		s.traceSession(sess, userdata.EvMailAdd, r, acc.Email)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) accountDelete(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	id := r.PathValue("id")
	list := s.mailAccounts(sess)
	if _, ok := findAccount(list, id); !ok {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	list = slices.DeleteFunc(list, func(a userdata.MailAccount) bool { return a.ID == id })
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.log.Info("compte mail supprimé", "login", store.ReduceLogin(sess.Account.Login), "account", id)
	s.traceSession(sess, userdata.EvMailDelete, r, "")
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}

func (s *Server) accountSelect(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	id := r.PostFormValue("id")
	if _, ok := findAccount(s.mailAccounts(sess), id); !ok {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	sess.Mu.Lock()
	sess.ActiveMailAccount = id
	sess.Mu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// activeAccount renvoie le compte mail actif (le premier par défaut).
func (s *Server) activeAccount(sess *session.Session) (userdata.MailAccount, []userdata.MailAccount, bool) {
	list := s.mailAccounts(sess)
	if len(list) == 0 {
		return userdata.MailAccount{}, nil, false
	}
	sess.Mu.Lock()
	id := sess.ActiveMailAccount
	sess.Mu.Unlock()
	if a, ok := findAccount(list, id); ok {
		return a, list, true
	}
	return list[0], list, true
}

// accountFolderChoices prépare les sélecteurs de dossiers système du formulaire : les dossiers existants sont lus avec
// les identifiants déjà enregistrés de la boîte (pas ceux du formulaire, qui n'ont peut-être pas été testés). Pour une
// nouvelle boîte, ou si le serveur ne répond pas, seuls « automatique » et la valeur actuelle sont proposés.
func (s *Server) accountFolderChoices(r *http.Request, sess *session.Session, f *AccountForm) *FoldersView {
	v := &FoldersView{}
	var folders []mailconn.Folder
	if f != nil && f.ID != "" {
		if acc, ok := findAccount(s.mailAccounts(sess), f.ID); ok {
			ctx, cancel := s.mailCtx(r)
			defer cancel()
			if err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
				folders, err = c.Folders(false)
				return
			}); err != nil {
				folders = nil
			}
		}
	}
	for _, fo := range folders {
		if fo.Selectable() {
			v.Names = append(v.Names, fo.Name)
		}
	}
	cur := AccountForm{}
	if f != nil {
		cur = *f
	}
	v.Roles = []RoleChoice{
		{Field: "trash_folder", LabelKey: "accounts.form.trash", Current: cur.TrashFolder, Resolved: mailconn.ResolveTrash(folders, "")},
		{Field: "archive_folder", LabelKey: "accounts.form.archive", Current: cur.ArchiveFolder, Resolved: mailconn.ResolveArchive(folders, "")},
		{Field: "junk_folder", LabelKey: "accounts.form.junk", Current: cur.JunkFolder, Resolved: mailconn.ResolveJunk(folders, "")},
		{Field: "sent_folder", LabelKey: "accounts.form.sent", Current: cur.SentFolder, Resolved: mailconn.ResolveSent(folders, ""), AllowNone: true},
		{Field: "drafts_folder", LabelKey: "accounts.form.drafts", Current: cur.DraftsFolder, Resolved: mailconn.ResolveDrafts(folders, ""), AllowNone: true},
	}
	return v
}

// moveInList déplace l'élément d'indice i d'un cran vers le haut (dir « up ») ou le bas ; faux si impossible.
func moveInList[T any](list []T, i int, dir string, lo int) bool {
	j := i - 1
	if dir == "down" {
		j = i + 1
	}
	if i < lo || j < lo || j >= len(list) {
		return false
	}
	list[i], list[j] = list[j], list[i]
	return true
}

// reorderByIDs renvoie la liste rangée dans l'ordre donné par « order » (identifiants séparés par des virgules).
// L'ordre doit décrire exactement la liste existante : même nombre d'éléments, mêmes identifiants, aucun doublon.
// Faux sinon : la liste d'origine n'est pas touchée.
func reorderByIDs[T any](list []T, order string, id func(T) string) ([]T, bool) {
	var ids []string
	if order = strings.TrimSpace(order); order != "" {
		ids = strings.Split(order, ",")
	}
	if len(ids) != len(list) {
		return nil, false
	}
	out := make([]T, 0, len(list))
	seen := make(map[string]bool, len(ids))
	for _, raw := range ids {
		key := strings.TrimSpace(raw)
		if key == "" || seen[key] {
			return nil, false
		}
		i := slices.IndexFunc(list, func(x T) bool { return id(x) == key })
		if i < 0 {
			return nil, false
		}
		seen[key] = true
		out = append(out, list[i])
	}
	return out, true
}

// reorderDone conclut un réordonnancement : une courte réponse JSON pour le script, une redirection sinon
// (sans JavaScript la page reste utilisable avec les flèches ▲ ▼).
func (s *Server) reorderDone(w http.ResponseWriter, r *http.Request, back string) {
	if r.Header.Get("X-Requested-With") == "fetch" {
		h := w.Header()
		h.Set("Content-Type", "application/json")
		h.Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// accountsReorder enregistre un ordre complet des boîtes mail, envoyé par le glisser-déposer de la page des comptes.
func (s *Server) accountsReorder(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	list := s.mailAccounts(sess)
	out, ok := reorderByIDs(list, r.PostFormValue("order"), func(a userdata.MailAccount) string { return a.ID })
	if !ok {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	if err := s.saveAccounts(sess, out); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.log.Info("ordre des comptes mail modifié", "login", store.ReduceLogin(sess.Account.Login), "nombre", len(out))
	s.reorderDone(w, r, "/accounts")
}

// accountMove change l'ordre des boîtes mail (la première est la boîte ouverte au départ).
func (s *Server) accountMove(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	id, dir := r.PathValue("id"), r.PostFormValue("dir")
	list := s.mailAccounts(sess)
	i := slices.IndexFunc(list, func(a userdata.MailAccount) bool { return a.ID == id })
	if i < 0 || (dir != "up" && dir != "down") {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	if moveInList(list, i, dir, 0) {
		if err := s.saveAccounts(sess, list); err != nil {
			s.serverError(w, r, sess, err)
			return
		}
	}
	http.Redirect(w, r, "/accounts", http.StatusSeeOther)
}
