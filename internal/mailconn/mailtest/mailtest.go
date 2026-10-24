// Package mailtest fournit des serveurs IMAP et SMTP en mémoire pour les tests.
// Compte unique : User / Password. Mode : "none", "starttls" ou "tls" (certificat auto-signé).
package mailtest

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

const (
	User     = "test"
	Password = "testpass"
)

// Folders sont les dossiers créés sur le serveur IMAP de test.
var Folders = []string{"INBOX", "Sent", "Trash", "Projets", "Projets/2026"}

func listen(t testing.TB) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func portOf(ln net.Listener) int { return ln.Addr().(*net.TCPAddr).Port }

func selfSigned(t testing.TB) tls.Certificate { return selfSignedKey(t, false) }

// legacyConfig imite un serveur ancien : certificat RSA, TLS 1.2 au plus, uniquement l'échange de clé RSA.
func legacyConfig(t testing.TB) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{selfSignedKey(t, true)},
		MaxVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256},
	}
}

func selfSignedKey(t testing.TB, useRSA bool) tls.Certificate {
	t.Helper()
	var key crypto.Signer
	var err error
	if useRSA {
		key, err = rsa.GenerateKey(rand.Reader, 2048)
	} else {
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// StartIMAP démarre un serveur IMAP et renvoie son port.
func StartIMAP(t testing.TB, mode string) int { return StartIMAPSeeded(t, mode, nil) }

// StartIMAPSeeded démarre un serveur IMAP et dépose en plus les messages de extra dans INBOX.
func StartIMAPSeeded(t testing.TB, mode string, extra func(t testing.TB, add func(raw []byte, date time.Time, flags ...imap.Flag))) int {
	t.Helper()
	mem := imapmemserver.New()
	u := imapmemserver.NewUser(User, Password)
	for _, name := range Folders {
		if err := u.Create(name, nil); err != nil {
			t.Fatal(err)
		}
	}
	Seed(t, u)
	if extra != nil {
		extra(t, func(raw []byte, date time.Time, flags ...imap.Flag) { appendMsg(t, u, raw, date, flags...) })
	}
	mem.AddUser(u)
	cert := selfSigned(t)
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	if mode == "tls-legacy" {
		cfg = legacyConfig(t)
	}
	opts := &imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		InsecureAuth: mode == "none",
		Logger:       nopLogger{},
	}
	if mode == "starttls" {
		opts.TLSConfig = cfg
	}
	if mode == "listext" { // serveur qui sait lister les abonnements (LIST-EXTENDED) : en clair
		opts.InsecureAuth = true
		opts.Caps = imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapUIDPlus: {}, imap.CapMove: {}, imap.CapListExtended: {}}
	}
	if mode == "nomove" { // serveur sans MOVE (comme beaucoup de Dovecot anciens) : en clair
		opts.InsecureAuth = true
		opts.Caps = imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapUIDPlus: {}}
	}
	srv := imapserver.New(opts)
	ln := listen(t)
	var l net.Listener = ln
	if mode == "tls" || mode == "tls-legacy" {
		l = tls.NewListener(ln, cfg)
	}
	go srv.Serve(l)
	return portOf(ln)
}

type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

// Sent est un message reçu par le serveur SMTP de test.
type Sent struct {
	From string
	Rcpt []string
	Data []byte
}

// SMTPServer est un serveur SMTP de test qui garde les messages reçus. Il refuse tout destinataire dont l'adresse
// commence par « refuse@ ».
type SMTPServer struct {
	Port int
	mu   sync.Mutex
	msgs []Sent
}

// Messages renvoie une copie des messages reçus.
func (s *SMTPServer) Messages() []Sent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Sent(nil), s.msgs...)
}

type smtpBackend struct{ srv *SMTPServer }

func (b smtpBackend) NewSession(*smtp.Conn) (smtp.Session, error) {
	return &smtpSession{srv: b.srv}, nil
}

type smtpSession struct {
	srv  *SMTPServer
	from string
	rcpt []string
}

func (*smtpSession) AuthMechanisms() []string { return []string{"PLAIN"} }
func (*smtpSession) Auth(string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(_, user, pass string) error {
		if user == User && pass == Password {
			return nil
		}
		return &smtp.SMTPError{Code: 535, EnhancedCode: smtp.EnhancedCode{5, 7, 8}, Message: "Authentication failed"}
	}), nil
}
func (s *smtpSession) Reset()                                      { s.from, s.rcpt = "", nil }
func (*smtpSession) Logout() error                                 { return nil }
func (s *smtpSession) Mail(from string, _ *smtp.MailOptions) error { s.from = from; return nil }
func (s *smtpSession) Rcpt(to string, _ *smtp.RcptOptions) error {
	if strings.HasPrefix(strings.ToLower(to), "refuse@") {
		return &smtp.SMTPError{Code: 550, EnhancedCode: smtp.EnhancedCode{5, 1, 1}, Message: "No such user"}
	}
	s.rcpt = append(s.rcpt, to)
	return nil
}
func (s *smtpSession) Data(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	s.srv.mu.Lock()
	s.srv.msgs = append(s.srv.msgs, Sent{From: s.from, Rcpt: append([]string(nil), s.rcpt...), Data: b})
	s.srv.mu.Unlock()
	return nil
}

// StartSMTP démarre un serveur SMTP et renvoie son port.
func StartSMTP(t testing.TB, mode string) int { return StartSMTPCapture(t, mode).Port }

// StartSMTPCapture démarre un serveur SMTP qui garde les messages reçus.
func StartSMTPCapture(t testing.TB, mode string) *SMTPServer {
	t.Helper()
	state := &SMTPServer{}
	srv := smtp.NewServer(smtpBackend{srv: state})
	srv.Domain = "localhost"
	srv.AllowInsecureAuth = mode == "none"
	cfg := &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}}
	if mode == "starttls" {
		srv.TLSConfig = cfg
	}
	ln := listen(t)
	var l net.Listener = ln
	if mode == "tls" {
		l = tls.NewListener(ln, cfg)
	}
	go srv.Serve(l)
	state.Port = portOf(ln)
	return state
}

// InboxCount est le nombre de messages déposés dans INBOX par Seed.
const InboxCount = 31

type literal struct{ *bytes.Reader }

func (l literal) Size() int64 { return l.Reader.Size() }

// Message construit un message simple (texte brut).
func Message(from, to, subject string, date time.Time, body string) []byte {
	return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%d@test>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		from, to, subject, date.Format(time.RFC1123Z), date.UnixNano(), body))
}

func appendMsg(t testing.TB, u *imapmemserver.User, raw []byte, date time.Time, flags ...imap.Flag) {
	t.Helper()
	if _, err := u.Append("INBOX", literal{bytes.NewReader(raw)}, &imap.AppendOptions{Flags: flags, Time: date}); err != nil {
		t.Fatal(err)
	}
}

// Seed dépose InboxCount messages dans INBOX :
//  1. « Facture octobre » (Alice, favori, avec une pièce jointe PDF, « montant 100 »)
//  2. « Re: Facture octobre » (Bob, lu, « ok merci »)
//  3. « Réunion lundi » (Carol, HTML avec image distante et script)
//     4 à 30. « Filler NN » (Zed)
func Seed(t testing.TB, u *imapmemserver.User) {
	t.Helper()
	d0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	withPDF := fmt.Sprintf("From: Alice <alice@x.fr>\r\nTo: test@localhost\r\nSubject: Facture octobre\r\nDate: %s\r\nMessage-ID: <m1@test>\r\nMIME-Version: 1.0\r\n"+
		"Content-Type: multipart/mixed; boundary=\"B\"\r\n\r\n--B\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nmontant 100\r\n"+
		"--B\r\nContent-Type: application/pdf; name=\"facture.pdf\"\r\nContent-Disposition: attachment; filename=\"facture.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQK%s\r\n--B--\r\n",
		d0.Format(time.RFC1123Z), bytes.Repeat([]byte("QUJD"), 500))
	appendMsg(t, u, []byte(withPDF), d0, imap.FlagFlagged)
	d1 := d0.Add(24 * time.Hour)
	appendMsg(t, u, Message("Bob <bob@y.fr>", "test@localhost", "Re: Facture octobre", d1, "ok merci"), d1, imap.FlagSeen)
	d2 := d0.Add(48 * time.Hour)
	html := fmt.Sprintf("From: Carol <carol@z.fr>\r\nTo: test@localhost\r\nSubject: Réunion lundi\r\nDate: %s\r\nMessage-ID: <m3@test>\r\nMIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n\r\n<p>Bonjour <b>Carol</b></p><img src=\"https://tracker.example/p.gif?id=7\"><script>alert(1)</script>\r\n", d2.Format(time.RFC1123Z))
	appendMsg(t, u, []byte(html), d2)
	// Sujet et expéditeur piégés : l'interface doit tout échapper.
	dx := d0.Add(84 * time.Hour)
	appendMsg(t, u, Message("Eve <eve@z.fr>", "test@localhost", "Gala <script>alert(1)</script> & co", dx, "<b>gras ?</b>"), dx)
	for i := 4; i <= InboxCount-1; i++ {
		d := d0.Add(time.Duration(i-1) * 24 * time.Hour)
		appendMsg(t, u, Message("Zed <zed@z.fr>", "test@localhost", fmt.Sprintf("Filler %02d", i), d, "rien à voir"), d)
	}
}
