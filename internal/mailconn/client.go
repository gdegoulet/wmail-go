package mailconn

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// Sécurité de la connexion.
const (
	SecurityTLS      = "tls"      // TLS implicite
	SecuritySTARTTLS = "starttls" // STARTTLS obligatoire (jamais de repli en clair)
	SecurityNone     = "none"     // sans chiffrement
)

// ValidSecurity indique si s est une valeur de sécurité connue.
func ValidSecurity(s string) bool {
	return s == SecurityTLS || s == SecuritySTARTTLS || s == SecurityNone
}

// Endpoint décrit un serveur IMAP ou SMTP.
type Endpoint struct {
	Host       string
	Port       int
	Security   string
	Login      string
	Password   string
	VerifyCert bool
	// LegacyTLS accepte les serveurs anciens : TLS 1.0 à 1.2 et les suites de chiffrement qui n'offrent pas la confidentialité
	// persistante (échange de clé RSA, CBC, 3DES). À réserver aux serveurs que l'on ne peut pas mettre à jour.
	LegacyTLS bool
}

func (e Endpoint) tlsConfig() *tls.Config {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Le contrôle du certificat est une option du compte (serveurs internes à certificat auto-signé).
		InsecureSkipVerify: !e.VerifyCert, //nolint:gosec
	}
	if _, err := netip.ParseAddr(e.Host); err != nil {
		cfg.ServerName = e.Host
	}
	if e.LegacyTLS {
		cfg.MinVersion = tls.VersionTLS10
		// Go retire par défaut les suites à échange de clé RSA : on les nomme explicitement.
		for _, cs := range tls.CipherSuites() {
			cfg.CipherSuites = append(cfg.CipherSuites, cs.ID)
		}
		for _, cs := range tls.InsecureCipherSuites() {
			cfg.CipherSuites = append(cfg.CipherSuites, cs.ID)
		}
	}
	return cfg
}

// connect ouvre la connexion TCP (et TLS implicite si demandé). L'expiration du contexte ferme la connexion ;
// stop() détache la connexion du contexte (connexion qui doit lui survivre, voir Pool).
func (g *Guard) connect(ctx context.Context, e Endpoint, implicitTLS bool) (net.Conn, func() bool, error) {
	raw, err := g.DialContext(ctx, e.Host, e.Port)
	if err != nil {
		return nil, nil, err
	}
	// Les bibliothèques IMAP et SMTP fixent leurs propres délais (parfois de plusieurs minutes) :
	// l'expiration du contexte ferme la connexion et débloque toute lecture en cours.
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	if !implicitTLS {
		return raw, stop, nil
	}
	tc := tls.Client(raw, e.tlsConfig())
	if err := tc.HandshakeContext(ctx); err != nil {
		stop()
		raw.Close()
		return nil, nil, err
	}
	return tc, stop, nil
}

// DialIMAP se connecte au serveur IMAP et attend le message d'accueil, sans s'authentifier.
func (g *Guard) DialIMAP(ctx context.Context, e Endpoint) (*imapclient.Client, error) {
	c, _, err := g.dialIMAP(ctx, e)
	return c, err
}

func (g *Guard) dialIMAP(ctx context.Context, e Endpoint) (*imapclient.Client, func() bool, error) {
	opts := &imapclient.Options{TLSConfig: e.tlsConfig()}
	var c *imapclient.Client
	var stop func() bool
	switch e.Security {
	case SecurityTLS:
		conn, st, err := g.connect(ctx, e, true)
		if err != nil {
			return nil, nil, err
		}
		c, stop = imapclient.New(conn, opts), st
	case SecuritySTARTTLS:
		conn, st, err := g.connect(ctx, e, false)
		if err != nil {
			return nil, nil, err
		}
		if c, err = imapclient.NewStartTLS(conn, opts); err != nil {
			st()
			return nil, nil, err
		}
		stop = st
	case SecurityNone:
		conn, st, err := g.connect(ctx, e, false)
		if err != nil {
			return nil, nil, err
		}
		c, stop = imapclient.New(conn, opts), st
	default:
		return nil, nil, fmt.Errorf("sécurité inconnue : %q", e.Security)
	}
	if err := c.WaitGreeting(); err != nil {
		stop()
		c.Close()
		return nil, nil, err
	}
	return c, stop, nil
}

// authStage marque une erreur survenue pendant l'authentification.
type authStage struct{ err error }

func (e *authStage) Error() string { return e.err.Error() }
func (e *authStage) Unwrap() error { return e.err }

// LoginIMAP se connecte et s'authentifie. La connexion est fermée quand le contexte expire.
func (g *Guard) LoginIMAP(ctx context.Context, e Endpoint) (*imapclient.Client, error) {
	c, _, err := g.dialIMAP(ctx, e)
	if err != nil {
		return nil, err
	}
	if err := c.Login(e.Login, e.Password).Wait(); err != nil {
		c.Close()
		return nil, &authStage{err}
	}
	enableUTF8(c)
	return c, nil
}

// LoginIMAPDetached se connecte et s'authentifie sous la limite de ctx, puis détache la connexion
// du contexte : elle peut être réutilisée après la fin de la requête (voir Pool).
func (g *Guard) LoginIMAPDetached(ctx context.Context, e Endpoint) (*imapclient.Client, error) {
	c, stop, err := g.dialIMAP(ctx, e)
	if err != nil {
		return nil, err
	}
	if err := c.Login(e.Login, e.Password).Wait(); err != nil {
		stop()
		c.Close()
		return nil, &authStage{err}
	}
	enableUTF8(c)
	stop()
	return c, nil
}

// enableUTF8 active UTF8=ACCEPT (RFC 6855) quand le serveur le propose. La bibliothèque go-imap écrit les noms de
// dossiers en UTF-8 dès que le serveur annonce IMAP4rev2, alors que Dovecot 2.4 l'annonce sans que la connexion soit en
// mode IMAP4rev2 tant que le client ne l'a pas demandé : sans ENABLE, un dossier au nom accentué serait refusé
// (« Mailbox name is not valid mUTF-7 »). L'échec de la commande est ignoré : le serveur reste utilisable pour les noms ASCII.
func enableUTF8(c *imapclient.Client) {
	caps := c.Caps()
	if caps.Has(imap.CapEnable) && caps.Has(imap.CapUTF8Accept) {
		_, _ = c.Enable(imap.CapUTF8Accept).Wait()
	}
}

func closeIMAP(c *imapclient.Client) {
	_ = c.Logout().Wait()
	_ = c.Close()
}

// classify classe une erreur ; si le contexte a expiré, la cause est le délai (la connexion fermée n'est qu'un effet).
func classify(ctx context.Context, err error) *TestError {
	if ctx.Err() != nil {
		err = errors.Join(ctx.Err(), err)
	}
	return Classify(err)
}

// TestIMAP vérifie la connexion, le chiffrement et l'authentification. Il renvoie un *TestError.
// Si la négociation TLS échoue, il réessaie en mode ancien : en cas de succès, l'erreur est de catégorie KindLegacyTLS
// (le serveur n'accepte que d'anciens protocoles ou chiffrements, voir Endpoint.LegacyTLS).
func (g *Guard) TestIMAP(ctx context.Context, e Endpoint) error {
	err := g.testIMAP(ctx, e)
	return g.legacyHint(err, e, func(e2 Endpoint) error { return g.testIMAP(ctx, e2) })
}

func (g *Guard) testIMAP(ctx context.Context, e Endpoint) error {
	c, err := g.LoginIMAP(ctx, e)
	if err != nil {
		return classify(ctx, err)
	}
	closeIMAP(c)
	return nil
}

// legacyHint transforme un échec TLS en KindLegacyTLS quand le même serveur accepte le mode ancien.
func (g *Guard) legacyHint(err error, e Endpoint, retry func(Endpoint) error) error {
	var te *TestError
	if err == nil || e.LegacyTLS || !errors.As(err, &te) || te.Kind != KindTLS || e.Security == SecurityNone || isCertError(te.err) {
		return err
	}
	e.LegacyTLS = true
	if retry(e) == nil {
		return &TestError{Kind: KindLegacyTLS, Detail: te.Detail, err: te.err}
	}
	return err
}

// DialSMTP se connecte au serveur SMTP, négocie le chiffrement et envoie EHLO, sans s'authentifier.
func (g *Guard) DialSMTP(ctx context.Context, e Endpoint) (*smtp.Client, error) {
	switch e.Security {
	case SecurityTLS:
		conn, _, err := g.connect(ctx, e, true)
		if err != nil {
			return nil, err
		}
		c := smtp.NewClient(conn)
		if err := c.Hello("localhost"); err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	case SecuritySTARTTLS:
		conn, _, err := g.connect(ctx, e, false)
		if err != nil {
			return nil, err
		}
		return smtp.NewClientStartTLS(conn, e.tlsConfig())
	case SecurityNone:
		conn, _, err := g.connect(ctx, e, false)
		if err != nil {
			return nil, err
		}
		c := smtp.NewClient(conn)
		if err := c.Hello("localhost"); err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}
	return nil, fmt.Errorf("sécurité inconnue : %q", e.Security)
}

// LoginSMTP se connecte et s'authentifie (si un login est renseigné).
func (g *Guard) LoginSMTP(ctx context.Context, e Endpoint) (*smtp.Client, error) {
	c, err := g.DialSMTP(ctx, e)
	if err != nil {
		return nil, err
	}
	if e.Login == "" {
		return c, nil
	}
	ok, mechs := c.Extension("AUTH")
	if !ok {
		c.Close()
		return nil, &authStage{errors.New("le serveur n'annonce pas d'authentification (AUTH)")}
	}
	var auth sasl.Client
	switch list := strings.Fields(strings.ToUpper(mechs)); {
	case slices.Contains(list, "PLAIN"):
		auth = sasl.NewPlainClient("", e.Login, e.Password)
	case slices.Contains(list, "LOGIN"):
		auth = sasl.NewLoginClient(e.Login, e.Password)
	default:
		c.Close()
		return nil, &authStage{fmt.Errorf("mécanismes d'authentification non pris en charge : %s", mechs)}
	}
	if err := c.Auth(auth); err != nil {
		c.Close()
		return nil, &authStage{err}
	}
	return c, nil
}

// TestSMTP vérifie la connexion, le chiffrement et l'authentification, sans envoyer de message.
func (g *Guard) TestSMTP(ctx context.Context, e Endpoint) error {
	err := g.testSMTP(ctx, e)
	return g.legacyHint(err, e, func(e2 Endpoint) error { return g.testSMTP(ctx, e2) })
}

func (g *Guard) testSMTP(ctx context.Context, e Endpoint) error {
	c, err := g.LoginSMTP(ctx, e)
	if err != nil {
		return classify(ctx, err)
	}
	_ = c.Quit()
	return nil
}

// Folder est un dossier IMAP avec ses compteurs.
type Folder struct {
	Name     string
	Delim    rune
	Attrs    []imap.MailboxAttr
	Total    uint32
	Unseen   uint32
	HasCount bool
	// Hidden : dossier non abonné (masqué dans le panneau de gauche, sauf s'il est affiché). Jamais vrai pour INBOX ni
	// pour un dossier à usage spécial, ni quand le serveur ne sait pas dire quels dossiers sont abonnés.
	Hidden bool
}

// HasAttr indique si le dossier porte l'attribut IMAP (par exemple \Trash).
func (f Folder) HasAttr(a imap.MailboxAttr) bool { return slices.Contains(f.Attrs, a) }

// Selectable indique si le dossier peut contenir des messages.
func (f Folder) Selectable() bool {
	return !f.HasAttr(imap.MailboxAttrNoSelect) && !f.HasAttr(imap.MailboxAttrNonExistent)
}

// Depth renvoie la profondeur dans l'arborescence.
func (f Folder) Depth() int {
	if f.Delim == 0 {
		return 0
	}
	return strings.Count(f.Name, string(f.Delim))
}

// Leaf renvoie le dernier segment du nom.
func (f Folder) Leaf() string {
	if f.Delim == 0 {
		return f.Name
	}
	if i := strings.LastIndexByte(f.Name, byte(f.Delim)); i >= 0 && f.Delim < 128 {
		return f.Name[i+1:]
	}
	return f.Name
}

// ListFolders liste les dossiers du compte avec le nombre de messages et de non lus (connexion dédiée).
func (g *Guard) ListFolders(ctx context.Context, e Endpoint) ([]Folder, error) {
	c, err := g.LoginIMAP(ctx, e)
	if err != nil {
		return nil, classify(ctx, err)
	}
	defer closeIMAP(c)
	folders, err := listFolders(c, true)
	if err != nil {
		return nil, classify(ctx, err)
	}
	return folders, nil
}

// listFolders liste les dossiers ; withStatus ajoute les compteurs (un STATUS par dossier, envoyés à la suite).
func listFolders(c *imapclient.Client, withStatus bool) ([]Folder, error) {
	list, err := c.List("", "*", nil).Collect()
	if err != nil {
		return nil, err
	}
	folders := make([]Folder, 0, len(list))
	for _, d := range list {
		folders = append(folders, Folder{Name: d.Mailbox, Delim: d.Delim, Attrs: d.Attrs})
	}
	markUnsubscribed(c, folders)
	if withStatus {
		cmds := make([]*imapclient.StatusCommand, len(folders))
		for i, f := range folders {
			if f.Selectable() {
				cmds[i] = c.Status(f.Name, &imap.StatusOptions{NumMessages: true, NumUnseen: true})
			}
		}
		for i, cmd := range cmds {
			if cmd == nil {
				continue
			}
			if st, err := cmd.Wait(); err == nil && st.NumMessages != nil {
				folders[i].Total = *st.NumMessages
				if st.NumUnseen != nil {
					folders[i].Unseen = *st.NumUnseen
				}
				folders[i].HasCount = true
			}
		}
	}
	SortFolders(folders)
	return folders, nil
}

// markUnsubscribed marque Hidden les dossiers non abonnés quand le serveur sait lister les abonnements (LIST-EXTENDED).
// Si aucun dossier n'est abonné, on ne masque rien : une liste vide serait inutilisable.
func markUnsubscribed(c *imapclient.Client, folders []Folder) {
	if !c.Caps().Has(imap.CapListExtended) {
		return
	}
	list, err := c.List("", "*", &imap.ListOptions{SelectSubscribed: true}).Collect()
	if err != nil || len(list) == 0 {
		return
	}
	sub := make(map[string]bool, len(list))
	for _, d := range list {
		// Le serveur peut aussi renvoyer un parent non abonné (\NonExistent) d'un dossier abonné : on ne compte que \Subscribed.
		if slices.Contains(d.Attrs, imap.MailboxAttrSubscribed) {
			sub[d.Mailbox] = true
		}
	}
	if len(sub) == 0 {
		return
	}
	for i, f := range folders {
		special := strings.EqualFold(f.Name, "INBOX")
		for _, a := range []imap.MailboxAttr{imap.MailboxAttrSent, imap.MailboxAttrDrafts, imap.MailboxAttrJunk, imap.MailboxAttrTrash, imap.MailboxAttrArchive} {
			special = special || f.HasAttr(a)
		}
		folders[i].Hidden = !sub[f.Name] && !special
	}
}

var specialOrder = []imap.MailboxAttr{
	imap.MailboxAttrSent, imap.MailboxAttrDrafts, imap.MailboxAttrJunk, imap.MailboxAttrTrash, imap.MailboxAttrArchive,
}

func folderRank(f Folder) int {
	if strings.EqualFold(f.Name, "INBOX") {
		return 0
	}
	for i, a := range specialOrder {
		if f.HasAttr(a) {
			return 1 + i
		}
	}
	return len(specialOrder) + 1
}

// NaturalLess compare deux noms sans tenir compte de la casse, les suites de chiffres comparées comme des nombres :
// « Dossier 2 » vient avant « Dossier 10 ».
func NaturalLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0
	for i < len(la) && j < len(lb) {
		ca, cb := la[i], lb[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(la) && isDigit(la[i]) {
				i++
			}
			for j < len(lb) && isDigit(lb[j]) {
				j++
			}
			na, nb := strings.TrimLeft(la[si:i], "0"), strings.TrimLeft(lb[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	if len(la)-i != len(lb)-j {
		return len(la)-i < len(lb)-j
	}
	return a < b
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// SortFolders place INBOX en premier, puis les dossiers spéciaux, puis les autres par nom.
// Les sous-dossiers restent groupés sous leur parent.
func SortFolders(fs []Folder) {
	top := func(f Folder) string {
		if f.Delim != 0 {
			if i := strings.IndexRune(f.Name, f.Delim); i > 0 {
				return f.Name[:i]
			}
		}
		return f.Name
	}
	rank := map[string]int{}
	for _, f := range fs {
		if r := folderRank(f); r < rank[top(f)] || rank[top(f)] == 0 && !strings.EqualFold(top(f), "INBOX") {
			rank[top(f)] = r
		}
	}
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		ra, rb := rank[top(a)], rank[top(b)]
		if strings.EqualFold(top(a), "INBOX") {
			ra = 0
		}
		if strings.EqualFold(top(b), "INBOX") {
			rb = 0
		}
		if ra != rb {
			return ra < rb
		}
		if ta, tb := top(a), top(b); !strings.EqualFold(ta, tb) {
			return NaturalLess(ta, tb)
		}
		return NaturalLess(a.Name, b.Name)
	})
}

// Noms par défaut des dossiers spéciaux quand ni la configuration du compte ni SPECIAL-USE ne les désignent.
const (
	DefaultTrash   = "Trash"
	DefaultArchive = "Archive"
	DefaultJunk    = "Junk"
)

// ResolveFolder renvoie le dossier d'un rôle : valeur explicite du compte, sinon dossier portant l'attribut IMAP
// (SPECIAL-USE), sinon un dossier existant portant un des noms usuels (alt), sinon def.
func ResolveFolder(folders []Folder, configured string, attr imap.MailboxAttr, def string, alt ...string) string {
	if configured != "" {
		return configured
	}
	for _, f := range folders {
		if f.HasAttr(attr) {
			return f.Name
		}
	}
	for _, name := range append([]string{def}, alt...) {
		for _, f := range folders {
			if strings.EqualFold(f.Name, name) {
				return f.Name
			}
		}
	}
	return def
}

// Noms par défaut des dossiers d'envoi et de brouillons.
const (
	DefaultSent   = "Sent"
	DefaultDrafts = "Drafts"
)

// ResolveSent renvoie le dossier des messages envoyés.
func ResolveSent(folders []Folder, configured string) string {
	return ResolveFolder(folders, configured, imap.MailboxAttrSent, DefaultSent, "Sent Items", "Envoyés", "INBOX.Sent")
}

// ResolveDrafts renvoie le dossier des brouillons.
func ResolveDrafts(folders []Folder, configured string) string {
	return ResolveFolder(folders, configured, imap.MailboxAttrDrafts, DefaultDrafts, "Brouillons", "INBOX.Drafts")
}

// ResolveTrash renvoie le dossier de corbeille.
func ResolveTrash(folders []Folder, configured string) string {
	return ResolveFolder(folders, configured, imap.MailboxAttrTrash, DefaultTrash,
		"Deleted Items", "Deleted Messages", "Deleted", "Corbeille", "Éléments supprimés", "INBOX.Trash", "INBOX.Deleted Items")
}

// ResolveArchive renvoie le dossier d'archives.
func ResolveArchive(folders []Folder, configured string) string {
	return ResolveFolder(folders, configured, imap.MailboxAttrArchive, DefaultArchive, "Archives")
}

// ResolveJunk renvoie le dossier des indésirables.
func ResolveJunk(folders []Folder, configured string) string {
	return ResolveFolder(folders, configured, imap.MailboxAttrJunk, DefaultJunk, "Spam", "Junk E-mail", "Junk Email", "Bulk Mail", "Indésirables", "INBOX.Junk", "INBOX.Spam")
}

// WithTimeout renvoie un contexte limité à d.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// addr renvoie « hôte:port », pour les traces.
func (e Endpoint) addr() string { return net.JoinHostPort(e.Host, strconv.Itoa(e.Port)) }

// ErrNoRecipients : l'enveloppe n'a aucun destinataire.
var ErrNoRecipients = errors.New("aucun destinataire")

// SendError est une erreur d'envoi SMTP : le serveur a refusé un destinataire ou le message (Rejected), ou la
// connexion a échoué (Rejected vide).
type SendError struct {
	Rejected []string // adresses refusées par le serveur
	Err      error
}

func (e *SendError) Error() string { return "envoi : " + e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

// SendSMTP transmet un message par SMTP : MAIL FROM, RCPT TO pour chaque destinataire, DATA. Si dsn est vrai, on
// demande des notifications d'envoi (NOTIFY=SUCCESS,FAILURE,DELAY) quand le serveur annonce DSN. Les erreurs de
// connexion sont des *TestError, les refus du serveur des *SendError.
func (g *Guard) SendSMTP(ctx context.Context, e Endpoint, from string, rcpts []string, raw []byte, dsn bool) error {
	if len(rcpts) == 0 {
		return ErrNoRecipients
	}
	c, err := g.LoginSMTP(ctx, e)
	if err != nil {
		return classify(ctx, err)
	}
	// Le contexte borne aussi l'échange : sans cela, un serveur muet bloquerait l'envoi.
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	defer c.Close()

	var mailOpts *smtp.MailOptions
	var rcptOpts *smtp.RcptOptions
	if ok, _ := c.Extension("DSN"); ok && dsn {
		rcptOpts = &smtp.RcptOptions{Notify: []smtp.DSNNotify{smtp.DSNNotifyFailure, smtp.DSNNotifySuccess, smtp.DSNNotifyDelayed}}
	}
	if err := c.Mail(from, mailOpts); err != nil {
		return &SendError{Err: err}
	}
	var rejected []string
	accepted := 0
	for _, r := range rcpts {
		if err := c.Rcpt(r, rcptOpts); err != nil {
			rejected = append(rejected, r)
			continue
		}
		accepted++
	}
	if accepted == 0 || len(rejected) > 0 {
		_ = c.Reset()
		return &SendError{Rejected: rejected, Err: fmt.Errorf("destinataire(s) refusé(s) par le serveur : %s", strings.Join(rejected, ", "))}
	}
	w, err := c.Data()
	if err != nil {
		return &SendError{Err: err}
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return classify(ctx, err)
	}
	if err := w.Close(); err != nil {
		return &SendError{Err: err}
	}
	_ = c.Quit()
	return nil
}
