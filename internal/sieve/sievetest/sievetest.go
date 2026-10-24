// Package sievetest fournit un serveur ManageSieve minimal, en mémoire, pour les tests.
package sievetest

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Identifiants acceptés par le serveur de test.
const (
	User     = "test"
	Password = "testpass"
)

// Server est un serveur ManageSieve de test.
type Server struct {
	Port int
	// Extensions annoncées dans la capacité SIEVE.
	Extensions string

	mu      sync.Mutex
	scripts map[string]string
	active  string
	tlsOnly bool // STARTTLS exigé avant l'authentification
	cfg     *tls.Config
	// Log garde les commandes reçues (sans le contenu des scripts ni les mots de passe).
	log []string
}

// Scripts renvoie une copie des scripts et le nom du script actif.
func (s *Server) Scripts() (map[string]string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for k, v := range s.scripts {
		out[k] = v
	}
	return out, s.active
}

// Put dépose un script directement (préparation d'un test).
func (s *Server) Put(name, script string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripts[name] = script
	if active {
		s.active = name
	}
}

// Commands renvoie les commandes reçues (noms seulement).
func (s *Server) Commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.log...)
}

// Start démarre le serveur. mode : « none » (en clair) ou « starttls » (STARTTLS exigé avant l'authentification).
func Start(t testing.TB, mode string) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s := &Server{Port: ln.Addr().(*net.TCPAddr).Port, scripts: map[string]string{}, Extensions: "fileinto reject vacation imap4flags regex relational date"}
	if mode == "starttls" {
		s.tlsOnly = true
		s.cfg = &tls.Config{Certificates: []tls.Certificate{cert(t)}}
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func cert(t testing.TB) tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func (s *Server) caps(tlsOn bool) string {
	var b strings.Builder
	b.WriteString("\"IMPLEMENTATION\" \"wmail-go test\"\r\n")
	b.WriteString("\"SIEVE\" \"" + s.Extensions + "\"\r\n")
	if s.cfg != nil && !tlsOn {
		b.WriteString("\"STARTTLS\"\r\n")
	}
	if s.cfg == nil || tlsOn {
		b.WriteString("\"SASL\" \"PLAIN\"\r\n")
	} else {
		b.WriteString("\"SASL\" \"\"\r\n")
	}
	b.WriteString("\"VERSION\" \"1.0\"\r\n")
	return b.String()
}

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	tlsOn, authed := false, false
	fmt.Fprint(c, s.caps(tlsOn)+"OK \"Sieve ready.\"\r\n")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		fields := splitFields(line)
		if len(fields) == 0 {
			continue
		}
		cmd := strings.ToUpper(fields[0])
		s.mu.Lock()
		s.log = append(s.log, cmd)
		s.mu.Unlock()
		switch cmd {
		case "CAPABILITY":
			fmt.Fprint(c, s.caps(tlsOn)+"OK \"Capability completed.\"\r\n")
		case "STARTTLS":
			if s.cfg == nil || tlsOn {
				fmt.Fprint(c, "NO \"TLS unavailable\"\r\n")
				continue
			}
			fmt.Fprint(c, "OK \"Begin TLS negotiation now.\"\r\n")
			tc := tls.Server(c, s.cfg)
			if err := tc.Handshake(); err != nil {
				return
			}
			c, r, tlsOn = tc, bufio.NewReader(tc), true
			fmt.Fprint(c, s.caps(tlsOn)+"OK \"Sieve ready.\"\r\n")
		case "AUTHENTICATE":
			if s.tlsOnly && !tlsOn {
				fmt.Fprint(c, "NO \"Authentication not allowed without TLS\"\r\n")
				continue
			}
			if len(fields) < 3 || strings.ToUpper(fields[1]) != "PLAIN" {
				fmt.Fprint(c, "NO \"Unsupported mechanism\"\r\n")
				continue
			}
			raw, _ := base64.StdEncoding.DecodeString(fields[2])
			parts := strings.Split(string(raw), "\x00")
			if len(parts) == 3 && parts[1] == User && parts[2] == Password {
				authed = true
				fmt.Fprint(c, "OK \"Logged in.\"\r\n")
			} else {
				fmt.Fprint(c, "NO \"Authentication failed.\"\r\n")
			}
		case "LOGOUT":
			fmt.Fprint(c, "OK \"Logout completed.\"\r\n")
			return
		default:
			if !authed {
				fmt.Fprint(c, "NO \"Authenticate first\"\r\n")
				continue
			}
			s.command(c, r, cmd, fields[1:], line)
		}
	}
}

// splitFields découpe « CMD "a b" {12+} » en champs en respectant les guillemets.
func splitFields(line string) []string {
	var out []string
	i := 0
	for i < len(line) {
		switch {
		case line[i] == ' ':
			i++
		case line[i] == '"':
			var b strings.Builder
			i++
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					i++
				}
				b.WriteByte(line[i])
				i++
			}
			i++
			out = append(out, b.String())
		default:
			j := i
			for j < len(line) && line[j] != ' ' {
				j++
			}
			out = append(out, line[i:j])
			i = j
		}
	}
	return out
}

// literal lit un littéral annoncé en fin de ligne ({n} ou {n+}).
func literal(r *bufio.Reader, line string) (string, bool) {
	i := strings.LastIndexByte(line, '{')
	if i < 0 || !strings.HasSuffix(line, "}") {
		return "", false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(line[i+1:len(line)-1], "+"))
	if err != nil || n < 0 || n > 4<<20 {
		return "", false
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", false
	}
	_, _ = r.ReadString('\n') // CRLF qui suit le littéral
	return string(buf), true
}

func quoteText(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func (s *Server) command(c io.Writer, r *bufio.Reader, cmd string, args []string, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch cmd {
	case "LISTSCRIPTS":
		for name := range s.scripts {
			if name == s.active {
				fmt.Fprintf(c, "%s ACTIVE\r\n", quoteText(name))
			} else {
				fmt.Fprintf(c, "%s\r\n", quoteText(name))
			}
		}
		fmt.Fprint(c, "OK \"Listscripts completed.\"\r\n")
	case "GETSCRIPT":
		if len(args) < 1 {
			fmt.Fprint(c, "NO \"Missing name\"\r\n")
			return
		}
		body, ok := s.scripts[args[0]]
		if !ok {
			fmt.Fprint(c, "NO (NONEXISTENT) \"There is no script by that name\"\r\n")
			return
		}
		fmt.Fprintf(c, "{%d}\r\n%s\r\nOK \"Getscript completed.\"\r\n", len(body), body)
	case "PUTSCRIPT", "CHECKSCRIPT":
		body, ok := literal(r, line)
		if !ok {
			fmt.Fprint(c, "NO \"Bad literal\"\r\n")
			return
		}
		if strings.Contains(body, "SYNTAXERROR") {
			fmt.Fprint(c, "NO \"line 1: syntax error\"\r\n")
			return
		}
		if cmd == "PUTSCRIPT" {
			if len(args) < 1 {
				fmt.Fprint(c, "NO \"Missing name\"\r\n")
				return
			}
			s.scripts[args[0]] = body
		}
		fmt.Fprint(c, "OK \""+strings.ToLower(cmd)+" completed.\"\r\n")
	case "SETACTIVE":
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if name != "" {
			if _, ok := s.scripts[name]; !ok {
				fmt.Fprint(c, "NO (NONEXISTENT) \"There is no script by that name\"\r\n")
				return
			}
		}
		s.active = name
		fmt.Fprint(c, "OK \"Setactive completed.\"\r\n")
	case "DELETESCRIPT":
		if len(args) < 1 {
			fmt.Fprint(c, "NO \"Missing name\"\r\n")
			return
		}
		if args[0] == s.active {
			fmt.Fprint(c, "NO (ACTIVE) \"You may not delete an active script\"\r\n")
			return
		}
		if _, ok := s.scripts[args[0]]; !ok {
			fmt.Fprint(c, "NO (NONEXISTENT) \"There is no script by that name\"\r\n")
			return
		}
		delete(s.scripts, args[0])
		fmt.Fprint(c, "OK \"Deletescript completed.\"\r\n")
	default:
		fmt.Fprint(c, "NO \"Unknown command\"\r\n")
	}
}
