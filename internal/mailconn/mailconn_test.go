package mailconn

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"wmail-go/internal/mailconn/mailtest"
)

// --- serveurs de test en mémoire ---

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func portOf(ln net.Listener) int { return ln.Addr().(*net.TCPAddr).Port }

var (
	startIMAP = mailtest.StartIMAP
	startSMTP = mailtest.StartSMTP
)

func localGuard(t *testing.T) *Guard {
	t.Helper()
	g, err := NewGuard(true, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func ep(port int, security string) Endpoint {
	return Endpoint{Host: "127.0.0.1", Port: port, Security: security, Login: "test", Password: "testpass"}
}

func ctx5(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return c
}

func kindOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var te *TestError
	if !errors.As(err, &te) {
		t.Fatalf("erreur non classée : %v", err)
	}
	return te.Kind
}

// --- Guard ---

func TestGuardBlocksPrivateAddressesByDefault(t *testing.T) {
	g, _ := NewGuard(true, nil)
	for _, host := range []string{
		"127.0.0.1", "10.0.0.1", "172.16.5.4", "192.168.1.5", "169.254.169.254", "0.0.0.0", "100.64.0.1",
		"::1", "fd00::1", "fe80::1", "::ffff:10.0.0.1", "::ffff:127.0.0.1", "64:ff9b::a00:1", "240.0.0.1",
	} {
		_, err := g.DialContext(ctx5(t), host, 25)
		var be *BlockedError
		if !errors.As(err, &be) {
			t.Errorf("%s : %v, attendu BlockedError", host, err)
		}
	}
}

func TestIsBlockedPublicAddresses(t *testing.T) {
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "93.184.216.34"} {
		if isBlocked(netip.MustParseAddr(s)) {
			t.Errorf("%s ne doit pas être bloquée", s)
		}
	}
}

func TestGuardAllowlist(t *testing.T) {
	ln := listen(t)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := portOf(ln)
	cases := []struct {
		allow []string
		host  string
		ok    bool
	}{
		{[]string{"127.0.0.1"}, "127.0.0.1", true},
		{[]string{"127.0.0.0/8"}, "127.0.0.1", true},
		{[]string{"localhost"}, "localhost", true},
		{[]string{"10.0.0.0/8"}, "127.0.0.1", false},
		{[]string{"autre.exemple"}, "localhost", false},
		{nil, "127.0.0.1", false},
	}
	for _, c := range cases {
		g, err := NewGuard(true, c.allow)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := g.DialContext(ctx5(t), c.host, port)
		if c.ok != (err == nil) {
			t.Errorf("allow=%v host=%s : err=%v, attendu ok=%v", c.allow, c.host, err, c.ok)
		}
		if conn != nil {
			conn.Close()
		}
	}
}

func TestGuardDisabledAllowsPrivate(t *testing.T) {
	ln := listen(t)
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()
	g, _ := NewGuard(false, nil)
	conn, err := g.DialContext(ctx5(t), "127.0.0.1", portOf(ln))
	if err != nil {
		t.Fatalf("block_private_networks=false : %v", err)
	}
	conn.Close()
}

func TestNewGuardRejectsBadEntries(t *testing.T) {
	for _, bad := range [][]string{{""}, {"10.0.0.0/99"}, {"pas valide!"}, {"a b"}} {
		if _, err := NewGuard(true, bad); err == nil {
			t.Errorf("%q accepté", bad)
		}
	}
	if _, err := NewGuard(true, []string{"imap.exemple.fr", "10.1.2.3", "192.168.0.0/16", "fd00::/8", "Mail.Interne"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidHost(t *testing.T) {
	for _, h := range []string{"imap.exemple.fr", "mail", "10.0.0.1", "::1", "a-b.c-d.org", "XN--80ak6aa92e.com"} {
		if !ValidHost(h) {
			t.Errorf("%q refusé", h)
		}
	}
	for _, h := range []string{"", "-bad.fr", "bad-.fr", "a..b", "a b", "fe80::1%eth0", "http://x", "x/y", "x:25", strings.Repeat("a", 254)} {
		if ValidHost(h) {
			t.Errorf("%q accepté", h)
		}
	}
}

// --- IMAP ---

func TestIMAPSecurityModes(t *testing.T) {
	g := localGuard(t)
	for _, mode := range []string{"none", "starttls", "tls"} {
		port := startIMAP(t, mode)
		e := ep(port, mode)
		e.VerifyCert = false
		if err := g.TestIMAP(ctx5(t), e); err != nil {
			t.Errorf("IMAP %s : %v", mode, err)
		}
	}
}

func TestIMAPFailureKinds(t *testing.T) {
	g := localGuard(t)
	plain := startIMAP(t, "none")
	secure := startIMAP(t, "tls")
	starttls := startIMAP(t, "starttls")

	bad := ep(plain, "none")
	bad.Password = "faux"
	if k := kindOf(t, g.TestIMAP(ctx5(t), bad)); k != KindAuth {
		t.Errorf("mauvais mot de passe : %q", k)
	}

	verify := ep(secure, "tls")
	verify.VerifyCert = true
	if k := kindOf(t, g.TestIMAP(ctx5(t), verify)); k != KindTLS {
		t.Errorf("certificat auto-signé vérifié : %q", k)
	}
	verify.VerifyCert = false
	if err := g.TestIMAP(ctx5(t), verify); err != nil {
		t.Errorf("certificat non vérifié : %v", err)
	}

	// TLS demandé sur un port en clair, STARTTLS demandé sur un serveur sans STARTTLS.
	if k := kindOf(t, g.TestIMAP(ctx5(t), ep(plain, "tls"))); k != KindTLS {
		t.Errorf("TLS sur port en clair : %q", k)
	}
	if k := kindOf(t, g.TestIMAP(ctx5(t), ep(plain, "starttls"))); k != KindTLS {
		t.Errorf("STARTTLS absent : %q", k)
	}
	// Aucun repli en clair : le serveur STARTTLS refuse le LOGIN avant chiffrement.
	if err := g.TestIMAP(ctx5(t), ep(starttls, "none")); err == nil {
		t.Error("connexion en clair vers un serveur qui exige STARTTLS acceptée")
	}

	ln := listen(t)
	port := portOf(ln)
	ln.Close()
	if k := kindOf(t, g.TestIMAP(ctx5(t), ep(port, "none"))); k != KindRefused {
		t.Errorf("port fermé : %q", k)
	}

	mute := listen(t) // accepte puis ne dit rien
	go func() {
		for {
			if _, err := mute.Accept(); err != nil {
				return
			}
		}
	}()
	short, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if k := kindOf(t, g.TestIMAP(short, ep(portOf(mute), "none"))); k != KindTimeout {
		t.Errorf("serveur muet : %q", k)
	}

	blocked, _ := NewGuard(true, nil)
	err := blocked.TestIMAP(ctx5(t), ep(plain, "none"))
	if k := kindOf(t, err); k != KindBlocked {
		t.Errorf("hôte privé : %q", k)
	}
	var be *BlockedError
	if !errors.As(err, &be) {
		t.Error("BlockedError attendu dans la chaîne d'erreurs")
	}

	nx := Endpoint{Host: "introuvable.invalid", Port: 143, Security: "none", Login: "a", Password: "b"}
	if k := kindOf(t, g.TestIMAP(ctx5(t), nx)); k != KindDNS {
		t.Errorf("hôte inconnu : %q", k)
	}
}

func TestListFolders(t *testing.T) {
	g := localGuard(t)
	port := startIMAP(t, "none")
	folders, err := g.ListFolders(ctx5(t), ep(port, "none"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range folders {
		names = append(names, f.Name)
		if f.Selectable() && !f.HasCount {
			t.Errorf("%s : compteurs absents", f.Name)
		}
	}
	got := strings.Join(names, ",")
	if got != "INBOX,Projets,Projets/2026,Sent,Trash" {
		t.Errorf("dossiers : %s", got)
	}
	var sub Folder
	for _, f := range folders {
		if f.Name == "Projets/2026" {
			sub = f
		}
	}
	if sub.Depth() != 1 || sub.Leaf() != "2026" {
		t.Errorf("profondeur %d, nom %q", sub.Depth(), sub.Leaf())
	}

	bad := ep(port, "none")
	bad.Password = "faux"
	if _, err := g.ListFolders(ctx5(t), bad); kindOf(t, err) != KindAuth {
		t.Errorf("authentification refusée : %v", err)
	}
}

// --- SMTP ---

func TestSMTPSecurityModesAndAuth(t *testing.T) {
	g := localGuard(t)
	for _, mode := range []string{"none", "starttls", "tls"} {
		port := startSMTP(t, mode)
		e := ep(port, mode)
		if err := g.TestSMTP(ctx5(t), e); err != nil {
			t.Errorf("SMTP %s : %v", mode, err)
		}
		bad := e
		bad.Password = "faux"
		if k := kindOf(t, g.TestSMTP(ctx5(t), bad)); k != KindAuth {
			t.Errorf("SMTP %s mauvais mot de passe : %q", mode, k)
		}
	}
	port := startSMTP(t, "tls")
	verify := ep(port, "tls")
	verify.VerifyCert = true
	if k := kindOf(t, g.TestSMTP(ctx5(t), verify)); k != KindTLS {
		t.Errorf("certificat auto-signé vérifié : %q", k)
	}
	plain := startSMTP(t, "none")
	if k := kindOf(t, g.TestSMTP(ctx5(t), ep(plain, "starttls"))); k != KindTLS {
		t.Errorf("STARTTLS absent : %q", k)
	}
	noauth := ep(plain, "none")
	noauth.Login = ""
	if err := g.TestSMTP(ctx5(t), noauth); err != nil {
		t.Errorf("SMTP sans authentification : %v", err)
	}
}

// --- utilitaires ---

func TestResolveTrash(t *testing.T) {
	folders := []Folder{
		{Name: "INBOX"},
		{Name: "Corbeille", Attrs: []imap.MailboxAttr{imap.MailboxAttrTrash}},
	}
	if got := ResolveTrash(folders, "Mes déchets"); got != "Mes déchets" {
		t.Errorf("valeur explicite : %q", got)
	}
	if got := ResolveTrash(folders, ""); got != "Corbeille" {
		t.Errorf("attribut \\Trash : %q", got)
	}
	if got := ResolveTrash([]Folder{{Name: "INBOX"}}, ""); got != DefaultTrash {
		t.Errorf("défaut : %q", got)
	}
}

func TestSortFolders(t *testing.T) {
	fs := []Folder{
		{Name: "Zèbre", Delim: '/'}, {Name: "Archive", Delim: '/', Attrs: []imap.MailboxAttr{imap.MailboxAttrArchive}},
		{Name: "Trash", Delim: '/', Attrs: []imap.MailboxAttr{imap.MailboxAttrTrash}},
		{Name: "Projets/b", Delim: '/'}, {Name: "Projets", Delim: '/'}, {Name: "Projets/a", Delim: '/'},
		{Name: "Sent", Delim: '/', Attrs: []imap.MailboxAttr{imap.MailboxAttrSent}},
		{Name: "INBOX", Delim: '/'}, {Name: "Drafts", Delim: '/', Attrs: []imap.MailboxAttr{imap.MailboxAttrDrafts}},
	}
	SortFolders(fs)
	var got []string
	for _, f := range fs {
		got = append(got, f.Name)
	}
	want := "INBOX,Sent,Drafts,Trash,Archive,Projets,Projets/a,Projets/b,Zèbre"
	if strings.Join(got, ",") != want {
		t.Errorf("ordre %s, attendu %s", strings.Join(got, ","), want)
	}
}

func TestClassifyShortensDetail(t *testing.T) {
	te := Classify(errors.New(strings.Repeat("x", 500) + "\n\nfin"))
	if len([]rune(te.Detail)) > 201 || strings.Contains(te.Detail, "\n") {
		t.Errorf("détail non réduit : %d caractères", len(te.Detail))
	}
	if again := Classify(te); again != te {
		t.Error("Classify doit conserver une erreur déjà classée")
	}
}

func TestResolveArchiveAndJunk(t *testing.T) {
	folders := []Folder{{Name: "INBOX"}, {Name: "Archives"}, {Name: "Spam"}, {Name: "Rangement", Attrs: []imap.MailboxAttr{imap.MailboxAttrArchive}}}
	if got := ResolveArchive(folders, ""); got != "Rangement" {
		t.Errorf("attribut \\Archive : %q", got)
	}
	if got := ResolveArchive(folders[:3], ""); got != "Archives" {
		t.Errorf("nom usuel : %q", got)
	}
	if got := ResolveJunk(folders, ""); got != "Spam" {
		t.Errorf("nom usuel : %q", got)
	}
	if got := ResolveJunk(folders, "Pourriel"); got != "Pourriel" {
		t.Errorf("valeur explicite : %q", got)
	}
	if got := ResolveJunk(folders[:1], ""); got != DefaultJunk || ResolveArchive(folders[:1], "") != DefaultArchive {
		t.Error("valeurs par défaut")
	}
}

func TestLegacyTLSServer(t *testing.T) {
	port := startIMAP(t, "tls-legacy")
	g := localGuard(t)
	e := ep(port, SecurityTLS)

	// Sans l'option, la poignée de main échoue (pas de suite commune) ; le test détecte que le mode ancien aboutit.
	if _, err := g.LoginIMAP(ctx5(t), e); err == nil {
		t.Fatal("le serveur ancien ne devrait pas être joignable sans l'option")
	}
	if k := kindOf(t, g.TestIMAP(ctx5(t), e)); k != KindLegacyTLS {
		t.Fatalf("catégorie = %q, attendu %q", k, KindLegacyTLS)
	}

	e.LegacyTLS = true
	if err := g.TestIMAP(ctx5(t), e); err != nil {
		t.Fatalf("avec l'option : %v", err)
	}
}

func TestLegacyTLSNotMaskingCertErrors(t *testing.T) {
	port := startIMAP(t, "tls")
	e := ep(port, SecurityTLS)
	e.VerifyCert = true
	if k := kindOf(t, localGuard(t).TestIMAP(ctx5(t), e)); k != KindTLS {
		t.Fatalf("catégorie = %q, attendu %q", k, KindTLS)
	}
}

func TestResolveSystemFoldersByUsualNames(t *testing.T) {
	mk := func(names ...string) []Folder {
		var fs []Folder
		for _, n := range names {
			fs = append(fs, Folder{Name: n, Delim: '/'})
		}
		return fs
	}
	if got := ResolveTrash(mk("INBOX", "Sent Items", "Deleted Items"), ""); got != "Deleted Items" {
		t.Errorf("corbeille Exchange : %q", got)
	}
	if got := ResolveTrash(mk("INBOX", "Deleted Items", "Trash"), ""); got != "Trash" {
		t.Errorf("« Trash » reste prioritaire : %q", got)
	}
	if got := ResolveTrash(mk("INBOX"), ""); got != "Trash" {
		t.Errorf("sans corbeille : %q", got)
	}
	if got := ResolveTrash(mk("INBOX", "Deleted Items"), "Autre"); got != "Autre" {
		t.Errorf("choix explicite : %q", got)
	}
	if got := ResolveJunk(mk("INBOX", "Junk E-mail"), ""); got != "Junk E-mail" {
		t.Errorf("indésirables Exchange : %q", got)
	}
}
