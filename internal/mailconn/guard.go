// Package mailconn ouvre les connexions IMAP et SMTP vers les serveurs des utilisateurs.
// Le Guard protège contre le SSRF : le serveur Go ne se connecte pas aux réseaux privés,
// sauf aux hôtes autorisés explicitement dans config.json (allowed_private_hosts).
package mailconn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BlockedError indique qu'une connexion a été refusée par la politique réseau.
type BlockedError struct {
	Host string
	IP   netip.Addr
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("connexion vers %s (%s) refusée par la politique réseau", e.Host, e.IP)
}

var hostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// ValidHost indique si host est un nom d'hôte ou une adresse IP acceptable pour une connexion.
func ValidHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return !strings.Contains(host, "%") // pas de zone IPv6
	}
	return hostRe.MatchString(strings.ToLower(host))
}

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",       // « ce réseau »
	"100.64.0.0/10",   // CGNAT
	"192.0.0.0/24",    // IETF
	"192.0.2.0/24",    // documentation
	"198.18.0.0/15",   // tests de performance
	"198.51.100.0/24", // documentation
	"203.0.113.0/24",  // documentation
	"240.0.0.0/4",     // réservé
	"64:ff9b::/96",    // NAT64 (embarque une IPv4)
	"2002::/16",       // 6to4 (embarque une IPv4)
	"100::/64",        // discard
	"fec0::/10",       // site-local obsolète
)

func mustPrefixes(l ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(l))
	for i, s := range l {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// isBlocked indique si l'adresse est privée, locale ou réservée.
func isBlocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// Guard applique la politique réseau aux connexions sortantes.
type Guard struct {
	blockPrivate bool
	hosts        map[string]bool
	prefixes     []netip.Prefix
	resolver     *net.Resolver
	dialTimeout  time.Duration
}

// SetDialTimeout fixe le délai d'établissement d'une connexion (TCP et TLS).
func (g *Guard) SetDialTimeout(d time.Duration) {
	if d > 0 {
		g.dialTimeout = d
	}
}

// DialTimeout renvoie le délai d'établissement d'une connexion.
func (g *Guard) DialTimeout() time.Duration { return g.dialTimeout }

// NewGuard crée un Guard. allowed contient des noms d'hôtes, des adresses IP ou des réseaux CIDR
// autorisés même s'ils sont privés ; il n'a d'effet que si blockPrivate est vrai.
func NewGuard(blockPrivate bool, allowed []string) (*Guard, error) {
	g := &Guard{blockPrivate: blockPrivate, hosts: map[string]bool{}, resolver: net.DefaultResolver, dialTimeout: 10 * time.Second}
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		switch {
		case a == "":
			return nil, fmt.Errorf("allowed_private_hosts : entrée vide")
		case strings.Contains(a, "/"):
			p, err := netip.ParsePrefix(a)
			if err != nil {
				return nil, fmt.Errorf("allowed_private_hosts : réseau %q invalide : %w", a, err)
			}
			g.prefixes = append(g.prefixes, p.Masked())
		default:
			if ip, err := netip.ParseAddr(a); err == nil {
				ip = ip.Unmap()
				g.prefixes = append(g.prefixes, netip.PrefixFrom(ip, ip.BitLen()))
			} else if ValidHost(a) {
				g.hosts[a] = true
			} else {
				return nil, fmt.Errorf("allowed_private_hosts : %q n'est ni un hôte, ni une adresse IP, ni un réseau", a)
			}
		}
	}
	return g, nil
}

func (g *Guard) allowed(host string, ip netip.Addr) bool {
	if !g.blockPrivate || !isBlocked(ip) {
		return true
	}
	if g.hosts[strings.ToLower(host)] {
		return true
	}
	for _, p := range g.prefixes {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

// DialContext résout host, vérifie chaque adresse puis se connecte à l'adresse vérifiée elle-même :
// une réponse DNS qui change entre la vérification et la connexion est sans effet.
func (g *Guard) DialContext(ctx context.Context, host string, port int) (net.Conn, error) {
	var addrs []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{ip}
	} else {
		ips, err := g.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		addrs = ips
	}
	var usable []netip.Addr
	var blocked *BlockedError
	for _, a := range addrs {
		a = a.Unmap()
		if g.allowed(host, a) {
			usable = append(usable, a)
		} else if blocked == nil {
			blocked = &BlockedError{Host: host, IP: a}
		}
	}
	if len(usable) == 0 {
		if blocked != nil {
			return nil, blocked
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	d := net.Dialer{Timeout: g.dialTimeout}
	var lastErr error
	for _, a := range usable {
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(a.String(), strconv.Itoa(port)))
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

// HTTPClient renvoie un client HTTP dont chaque connexion passe par le garde (y compris après une redirection).
func (g *Guard) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				host, portS, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				port, err := strconv.Atoi(portS)
				if err != nil {
					return nil, err
				}
				return g.DialContext(ctx, host, port)
			},
			Proxy:               nil, // jamais de proxy d'environnement : le garde doit voir la destination
			TLSHandshakeTimeout: 15 * time.Second,
			MaxIdleConns:        2,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("trop de redirections")
			}
			if req.URL.Scheme != "https" && !(via[0].URL.Scheme == "http" && req.URL.Scheme == "http") {
				return errors.New("redirection vers un schéma non sécurisé")
			}
			return nil
		},
	}
}

// ExplicitlyAllowed indique si l'hôte figure dans allowed_private_hosts (nom ou adresse) : seuls ces hôtes locaux
// peuvent être joints en HTTP simple.
func (g *Guard) ExplicitlyAllowed(host string) bool {
	if g.hosts[strings.ToLower(host)] {
		return true
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		for _, p := range g.prefixes {
			if p.Contains(ip.Unmap()) {
				return true
			}
		}
	}
	return false
}
