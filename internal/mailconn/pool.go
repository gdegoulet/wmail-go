package mailconn

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
)

const dialTimeout = 20 * time.Second

// Pool garde une connexion IMAP ouverte par clé (session + compte mail), utilisée par une seule
// requête à la fois. Une connexion inactive est fermée après idle.
type Pool struct {
	// Log reçoit les traces des opérations IMAP (nil : aucune).
	Log   *slog.Logger
	guard *Guard
	idle  time.Duration
	mu    sync.Mutex
	m     map[string]*entry
}

type entry struct {
	mu     sync.Mutex // sérialise l'usage de la connexion
	client *imapclient.Client
	ep     Endpoint
	used   time.Time
}

// NewPool crée un pool. idle est la durée d'inactivité avant fermeture.
func NewPool(g *Guard, idle time.Duration) *Pool {
	return &Pool{guard: g, idle: idle, m: map[string]*entry{}}
}

func closedClient(c *imapclient.Client) bool {
	select {
	case <-c.Closed():
		return true
	default:
		return false
	}
}

func (e *entry) drop() {
	if e.client != nil {
		_ = e.client.Close()
		e.client = nil
	}
}

func (p *Pool) entry(key string) *entry {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.m[key]
	if e == nil {
		e = &entry{}
		p.m[key] = e
	}
	return e
}

// Do exécute fn sur la connexion du compte, en la (re)créant si besoin. Si une connexion réutilisée s'est
// fermée entre deux requêtes, fn est rejouée une fois sur une connexion neuve. Les erreurs renvoyées sont
// des *TestError (la cause reste accessible avec errors.Is / errors.As).
func (p *Pool) Do(ctx context.Context, key string, ep Endpoint, fn func(*Conn) error) error {
	e := p.entry(key)
	e.mu.Lock()
	defer e.mu.Unlock()
	for attempt := 0; ; attempt++ {
		reused := e.client != nil && e.ep == ep && !closedClient(e.client)
		if !reused {
			e.drop()
			dctx, cancel := context.WithTimeout(ctx, dialTimeout)
			start := time.Now()
			c, err := p.guard.LoginIMAPDetached(dctx, ep)
			cancel()
			if err != nil {
				if p.Log != nil {
					p.Log.Warn("imap connexion échouée", "serveur", ep.addr(), "securite", ep.Security, "ancien_tls", ep.LegacyTLS, "ms", time.Since(start).Milliseconds(), "erreur", shorten(err.Error()))
				}
				return classify(ctx, err)
			}
			if p.Log != nil {
				p.Log.Info("imap connexion", "serveur", ep.addr(), "securite", ep.Security, "ancien_tls", ep.LegacyTLS, "ms", time.Since(start).Milliseconds())
			}
			e.client, e.ep = c, ep
		}
		cl := e.client
		stop := context.AfterFunc(ctx, func() { _ = cl.Close() })
		err := fn(&Conn{Client: cl, log: p.Log, server: ep.addr()})
		stop()
		e.used = time.Now()
		if err == nil {
			return nil
		}
		broken := closedClient(cl)
		if broken {
			e.client = nil
			if p.Log != nil {
				p.Log.Warn("imap connexion perdue", "serveur", ep.addr(), "rejeu", reused && attempt == 0 && ctx.Err() == nil, "contexte", ctx.Err(), "erreur", shorten(err.Error()))
			}
		}
		if broken && reused && attempt == 0 && ctx.Err() == nil {
			continue
		}
		return classify(ctx, err)
	}
}

// Close ferme les connexions dont la clé commence par prefix (par exemple l'identifiant d'une session).
func (p *Pool) Close(prefix string) {
	p.mu.Lock()
	var doomed []*entry
	for k, e := range p.m {
		if strings.HasPrefix(k, prefix) {
			doomed = append(doomed, e)
			delete(p.m, k)
		}
	}
	p.mu.Unlock()
	for _, e := range doomed {
		e.mu.Lock()
		e.drop()
		e.mu.Unlock()
	}
}

// CloseAll ferme toutes les connexions.
func (p *Pool) CloseAll() { p.Close("") }

// Sweep ferme les connexions inactives depuis plus de idle.
func (p *Pool) Sweep() {
	p.mu.Lock()
	entries := make(map[string]*entry, len(p.m))
	for k, e := range p.m {
		entries[k] = e
	}
	p.mu.Unlock()
	for k, e := range entries {
		if !e.mu.TryLock() {
			continue // en cours d'usage
		}
		if e.client == nil || closedClient(e.client) || time.Since(e.used) > p.idle {
			e.drop()
			p.mu.Lock()
			if p.m[k] == e {
				delete(p.m, k)
			}
			p.mu.Unlock()
		}
		e.mu.Unlock()
	}
}

// Run nettoie périodiquement jusqu'à l'annulation du contexte, puis ferme tout.
func (p *Pool) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			p.CloseAll()
			return
		case <-t.C:
			p.Sweep()
		}
	}
}

// Len renvoie le nombre de connexions ouvertes (tests et journaux).
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.m {
		if e.mu.TryLock() {
			if e.client != nil && !closedClient(e.client) {
				n++
			}
			e.mu.Unlock()
		} else {
			n++
		}
	}
	return n
}
