// Package sieve implémente un client ManageSieve (RFC 5804) et la conversion entre des filtres structurés et des
// scripts Sieve (RFC 5228), compatibles avec ceux que crée RainLoop.
package sieve

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Erreurs du client.
var (
	ErrAuth       = errors.New("authentification refusée")
	ErrNoStartTLS = errors.New("le serveur n'annonce pas STARTTLS")
	ErrProtocol   = errors.New("réponse ManageSieve inattendue")
)

// ServerError est une réponse NO ou BYE du serveur.
type ServerError struct {
	Status string // NO ou BYE
	Code   string // code entre parenthèses, par exemple QUOTA/MAXSCRIPTS
	Text   string
}

func (e *ServerError) Error() string {
	s := e.Status
	if e.Code != "" {
		s += " (" + e.Code + ")"
	}
	if e.Text != "" {
		s += " : " + e.Text
	}
	return s
}

const (
	maxLine    = 64 << 10 // ligne de réponse (hors littéral)
	maxScript  = 1 << 20  // script accepté, en lecture comme en écriture
	maxListing = 1000     // lignes de données avant la réponse finale : borne un serveur qui ne conclut jamais
	// readBuffer : les capacités annoncées à l'accueil tiennent dans ce tampon, ce qui évite de rallonger la
	// première lecture ; il est recréé après STARTTLS puisque la connexion sous-jacente change.
	readBuffer = 8192
)

// Client est une session ManageSieve.
type Client struct {
	conn net.Conn
	r    *bufio.Reader
	caps map[string]string // capacités annoncées (SIEVE, SASL, STARTTLS…) ; la valeur est celle du serveur
}

// New prend en charge une connexion déjà ouverte et lit les capacités annoncées à l'accueil.
func New(conn net.Conn) (*Client, error) {
	c := &Client{conn: conn, r: bufio.NewReaderSize(conn, readBuffer)}
	if err := c.readCapabilities(); err != nil {
		return nil, err
	}
	return c, nil
}

// Close ferme la connexion sans formalité.
func (c *Client) Close() error { return c.conn.Close() }

// Logout termine proprement la session puis ferme la connexion.
func (c *Client) Logout() error {
	_ = c.conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = io.WriteString(c.conn, "LOGOUT\r\n")
	_, _, _ = c.readResponse() // la réponse n'a pas d'importance
	return c.conn.Close()
}

// Capability renvoie la valeur d'une capacité annoncée et si elle l'est.
func (c *Client) Capability(name string) (string, bool) {
	v, ok := c.caps[strings.ToUpper(name)]
	return v, ok
}

// Extensions renvoie les extensions Sieve prises en charge par le serveur (fileinto, vacation, regex…).
func (c *Client) Extensions() []string {
	v, _ := c.Capability("SIEVE")
	return strings.Fields(strings.ToLower(v))
}

// HasExtension indique si le serveur prend en charge une extension Sieve.
func (c *Client) HasExtension(name string) bool {
	for _, e := range c.Extensions() {
		if e == strings.ToLower(name) {
			return true
		}
	}
	return false
}

// token est un élément d'une ligne de réponse : chaîne entre guillemets, littéral ou mot-clé.
type token struct {
	s    string
	atom bool // mot-clé (ACTIVE, OK, NO…) ou code entre parenthèses
}

// readLine lit une ligne de réponse et la découpe en éléments ; les littéraux {n} sont lus en entier.
func (c *Client) readLine() ([]token, error) {
	var toks []token
	line, err := c.r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) > maxLine {
		return nil, ErrProtocol
	}
	for {
		line = strings.TrimRight(line, "\r\n")
		i := 0
		for i < len(line) {
			switch ch := line[i]; {
			case ch == ' ':
				i++
			case ch == '"':
				var b strings.Builder
				i++
				for i < len(line) && line[i] != '"' {
					if line[i] == '\\' && i+1 < len(line) {
						i++
					}
					b.WriteByte(line[i])
					i++
				}
				if i >= len(line) {
					return nil, ErrProtocol
				}
				i++
				toks = append(toks, token{s: b.String()})
			case ch == '{':
				j := strings.IndexByte(line[i:], '}')
				if j < 0 || i+j != len(line)-1 {
					return nil, ErrProtocol
				}
				n, err := strconv.Atoi(strings.TrimSuffix(line[i+1:i+j], "+"))
				if err != nil || n < 0 || n > maxScript {
					return nil, ErrProtocol
				}
				buf := make([]byte, n)
				if _, err := io.ReadFull(c.r, buf); err != nil {
					return nil, err
				}
				toks = append(toks, token{s: string(buf)})
				// la ligne continue après le littéral (souvent seulement CRLF)
				rest, err := c.r.ReadString('\n')
				if err != nil {
					return nil, err
				}
				line, i = rest, 0
				line = strings.TrimRight(line, "\r\n")
				continue
			default:
				j := i
				for j < len(line) && line[j] != ' ' {
					j++
				}
				toks = append(toks, token{s: line[i:j], atom: true})
				i = j
			}
		}
		return toks, nil
	}
}

// readResponse lit les lignes de données jusqu'à la réponse finale (OK, NO ou BYE).
func (c *Client) readResponse() (data [][]token, text string, err error) {
	for {
		toks, err := c.readLine()
		if err != nil {
			return data, "", err
		}
		if len(toks) == 0 {
			continue
		}
		if toks[0].atom {
			switch strings.ToUpper(toks[0].s) {
			case "OK":
				return data, lastText(toks[1:]), nil
			case "NO", "BYE":
				se := &ServerError{Status: strings.ToUpper(toks[0].s), Text: lastText(toks[1:])}
				for _, t := range toks[1:] {
					if t.atom && strings.HasPrefix(t.s, "(") {
						se.Code = strings.Trim(t.s, "()")
					}
				}
				return data, se.Text, se
			}
		}
		if len(data) >= maxListing {
			return data, "", ErrProtocol
		}
		data = append(data, toks)
	}
}

func lastText(toks []token) string {
	for i := len(toks) - 1; i >= 0; i-- {
		if !toks[i].atom {
			return toks[i].s
		}
	}
	return ""
}

func (c *Client) readCapabilities() error {
	data, _, err := c.readResponse()
	if err != nil {
		return err
	}
	c.caps = map[string]string{}
	for _, l := range data {
		if len(l) == 0 {
			continue
		}
		v := ""
		if len(l) > 1 {
			v = l[1].s
		}
		c.caps[strings.ToUpper(l[0].s)] = v
	}
	return nil
}

func (c *Client) send(format string, args ...any) error {
	_, err := io.WriteString(c.conn, fmt.Sprintf(format, args...))
	return err
}

// quote met une valeur entre guillemets (ManageSieve : \ et " échappés ; CR, LF et NUL refusés).
func quote(s string) (string, error) {
	if strings.ContainsAny(s, "\r\n\x00") {
		return "", errors.New("caractère interdit")
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`, nil
}

func (c *Client) cmd(format string, args ...any) ([][]token, error) {
	if err := c.send(format, args...); err != nil {
		return nil, err
	}
	data, _, err := c.readResponse()
	return data, err
}

// StartTLS bascule la connexion en TLS (STARTTLS) puis relit les capacités.
func (c *Client) StartTLS(cfg *tls.Config) error {
	if _, ok := c.Capability("STARTTLS"); !ok {
		return ErrNoStartTLS
	}
	if _, err := c.cmd("STARTTLS\r\n"); err != nil {
		return err
	}
	tc := tls.Client(c.conn, cfg)
	if err := tc.Handshake(); err != nil {
		return err
	}
	c.conn, c.r = tc, bufio.NewReaderSize(tc, readBuffer)
	// Après STARTTLS, le serveur renvoie ses capacités (sans commande) ; certains attendent CAPABILITY.
	if err := c.readCapabilities(); err != nil {
		return err
	}
	return nil
}

// Authenticate s'authentifie par SASL PLAIN (le mot de passe ne doit jamais partir en clair : à appeler après TLS,
// sauf si l'appelant a choisi explicitement une connexion non chiffrée).
func (c *Client) Authenticate(login, password string) error {
	if strings.ContainsAny(login, "\x00\r\n") || strings.ContainsAny(password, "\x00\r\n") {
		return fmt.Errorf("%w : caractère interdit", ErrAuth)
	}
	if sasl, ok := c.Capability("SASL"); ok && !strings.Contains(" "+strings.ToUpper(sasl)+" ", " PLAIN ") {
		return fmt.Errorf("%w : mécanisme PLAIN non proposé (%s)", ErrAuth, sasl)
	}
	blob := base64.StdEncoding.EncodeToString([]byte("\x00" + login + "\x00" + password))
	if _, err := c.cmd("AUTHENTICATE \"PLAIN\" \"%s\"\r\n", blob); err != nil {
		var se *ServerError
		if errors.As(err, &se) {
			return fmt.Errorf("%w : %s", ErrAuth, se.Text)
		}
		return err
	}
	// Un serveur peut joindre de nouvelles capacités à son OK : readResponse les a déjà consommées comme lignes de
	// données, et celles d'avant l'authentification suffisent à l'usage qui est fait du client (extensions Sieve).
	return nil
}

// Script est un script enregistré sur le serveur.
type Script struct {
	Name   string
	Active bool
}

// ListScripts liste les scripts du serveur.
func (c *Client) ListScripts() ([]Script, error) {
	data, err := c.cmd("LISTSCRIPTS\r\n")
	if err != nil {
		return nil, err
	}
	var out []Script
	for _, l := range data {
		if len(l) == 0 {
			continue
		}
		s := Script{Name: l[0].s}
		if len(l) > 1 && l[1].atom && strings.EqualFold(l[1].s, "ACTIVE") {
			s.Active = true
		}
		out = append(out, s)
	}
	return out, nil
}

// GetScript télécharge un script.
func (c *Client) GetScript(name string) (string, error) {
	q, err := quote(name)
	if err != nil {
		return "", err
	}
	data, err := c.cmd("GETSCRIPT %s\r\n", q)
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data[0]) == 0 {
		return "", ErrProtocol
	}
	return data[0][0].s, nil
}

// withScript envoie une commande dont l'argument final est un script, transmis en littéral non synchronisant
// « {n+} » : le serveur n'a pas à accuser réception de la taille avant de recevoir le corps, ce qui évite
// un aller-retour. La réponse finale est lue avant de rendre la main.
func (c *Client) withScript(prefix, script string) error {
	if len(script) > maxScript {
		return errors.New("script trop volumineux")
	}
	if err := c.send("%s {%d+}\r\n%s\r\n", prefix, len(script), script); err != nil {
		return err
	}
	_, _, err := c.readResponse()
	return err
}

// PutScript crée ou remplace un script ; le serveur le vérifie et refuse un script invalide (*ServerError).
func (c *Client) PutScript(name, script string) error {
	q, err := quote(name)
	if err != nil {
		return err
	}
	return c.withScript("PUTSCRIPT "+q, script)
}

// CheckScript fait vérifier un script par le serveur sans l'enregistrer.
func (c *Client) CheckScript(script string) error { return c.withScript("CHECKSCRIPT", script) }

// SetActive active un script ; un nom vide désactive tous les scripts.
func (c *Client) SetActive(name string) error {
	q, err := quote(name)
	if err != nil {
		return err
	}
	_, err = c.cmd("SETACTIVE %s\r\n", q)
	return err
}

// DeleteScript supprime un script.
func (c *Client) DeleteScript(name string) error {
	q, err := quote(name)
	if err != nil {
		return err
	}
	_, err = c.cmd("DELETESCRIPT %s\r\n", q)
	return err
}
