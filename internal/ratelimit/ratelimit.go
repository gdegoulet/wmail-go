// Package ratelimit limite le nombre d'événements par clé sur une fenêtre glissante.
package ratelimit

import (
	"sync"
	"time"

	"wmail-go/internal/config"
)

type entry struct {
	window time.Duration
	hits   []time.Time
}

// Limiter compte les événements par clé. Il ne contient jamais de login en clair :
// les clés par login sont des identifiants hashés.
type Limiter struct {
	mu  sync.Mutex
	m   map[string]*entry
	now func() time.Time
}

// New crée un limiteur.
func New() *Limiter { return &Limiter{m: map[string]*entry{}, now: time.Now} }

func (l *Limiter) prune(e *entry, now time.Time) {
	cut := now.Add(-e.window)
	i := 0
	for i < len(e.hits) && !e.hits[i].After(cut) {
		i++
	}
	e.hits = e.hits[i:]
}

// Check indique si la clé a atteint la limite. Si oui, retryAfter est le délai avant la prochaine tentative.
func (l *Limiter) Check(key string, lim config.Limit) (blocked bool, retryAfter time.Duration) {
	if !lim.Enabled() {
		return false, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[key]
	if e == nil {
		return false, 0
	}
	now := l.now()
	e.window = lim.Window
	l.prune(e, now)
	if len(e.hits) < lim.Count {
		return false, 0
	}
	return true, e.hits[len(e.hits)-lim.Count].Add(lim.Window).Sub(now)
}

// Add enregistre un événement pour la clé.
func (l *Limiter) Add(key string, lim config.Limit) {
	if !lim.Enabled() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[key]
	if e == nil {
		e = &entry{}
		l.m[key] = e
	}
	now := l.now()
	e.window = lim.Window
	l.prune(e, now)
	e.hits = append(e.hits, now)
}

// Reset efface les événements d'une clé (par exemple après une connexion réussie).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.m, key)
}

// Sweep supprime les clés dont tous les événements sont hors fenêtre.
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, e := range l.m {
		l.prune(e, now)
		if len(e.hits) == 0 {
			delete(l.m, k)
		}
	}
}
