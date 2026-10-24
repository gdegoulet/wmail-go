package mailconn

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-smtp"
)

// Catégories d'erreur affichées à l'utilisateur.
const (
	KindBlocked   = "blocked"   // refusé par la politique réseau
	KindDNS       = "dns"       // nom d'hôte introuvable
	KindRefused   = "refused"   // connexion refusée
	KindTimeout   = "timeout"   // délai dépassé
	KindTLS       = "tls"       // erreur TLS ou de certificat
	KindAuth      = "auth"      // identifiants refusés
	KindLegacyTLS = "legacytls" // le serveur n'accepte que d'anciens protocoles ou chiffrements TLS
	KindProtocol  = "protocol"  // réponse inattendue du serveur
)

// TestError est une erreur de connexion classée par catégorie, avec un détail court.
type TestError struct {
	Kind   string
	Detail string
	err    error
}

func (e *TestError) Error() string { return e.Kind + " : " + e.Detail }
func (e *TestError) Unwrap() error { return e.err }

func shorten(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}

// Classify range une erreur réseau, TLS, IMAP ou SMTP dans une catégorie.
func Classify(err error) *TestError {
	var te *TestError
	if errors.As(err, &te) {
		return te
	}
	kind := KindProtocol
	var (
		blocked *BlockedError
		dns     *net.DNSError
		unknown x509.UnknownAuthorityError
		hostErr x509.HostnameError
		certErr x509.CertificateInvalidError
		verify  *tls.CertificateVerificationError
		record  tls.RecordHeaderError
		alert   tls.AlertError
		imapErr *imap.Error
		smtpErr *smtp.SMTPError
		auth    *authStage
	)
	switch {
	case errors.As(err, &blocked):
		kind = KindBlocked
	case errors.As(err, &dns):
		kind = KindDNS
	case errors.Is(err, syscall.ECONNREFUSED):
		kind = KindRefused
	case errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err):
		kind = KindTimeout
	case errors.As(err, &unknown) || errors.As(err, &hostErr) || errors.As(err, &certErr) ||
		errors.As(err, &verify) || errors.As(err, &record) || errors.As(err, &alert) ||
		strings.Contains(err.Error(), "STARTTLS") || strings.Contains(err.Error(), "tls:"):
		kind = KindTLS
	case errors.As(err, &auth):
		kind = KindAuth
	case errors.As(err, &imapErr) && imapErr.Code == imap.ResponseCodeAuthenticationFailed,
		errors.As(err, &smtpErr) && (smtpErr.Code == 535 || smtpErr.Code == 534 || smtpErr.Code == 530):
		kind = KindAuth
	}
	return &TestError{Kind: kind, Detail: shorten(err.Error()), err: err}
}

// isCertError indique une erreur de vérification du certificat (à ne pas confondre avec une négociation refusée).
func isCertError(err error) bool {
	var (
		unknown x509.UnknownAuthorityError
		host    x509.HostnameError
		inv     x509.CertificateInvalidError
		verify  *tls.CertificateVerificationError
	)
	return errors.As(err, &unknown) || errors.As(err, &host) || errors.As(err, &inv) || errors.As(err, &verify)
}
