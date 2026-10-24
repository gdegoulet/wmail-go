// Package session gère les sessions en mémoire (équivalent d'une session PHP).
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"sync"
	"time"
	"wmail-go/internal/contacts"

	"wmail-go/internal/store"
)

// Session contient le compte déchiffré et l'état de navigation de l'utilisateur.
type Session struct {
	Mu                 sync.Mutex // protège Account, PendingRecoveryKey et ActiveMailAccount
	ID                 string
	Account            *store.Account
	CSRF               string
	Created            time.Time
	LastSeen           time.Time // protégé par le Manager
	Idle, Abs          time.Duration
	PendingRecoveryKey string // clé à afficher puis confirmer avant de continuer
	ActiveMailAccount  string
	ContactsMu         sync.Mutex     // sérialise les accès au carnet d'adresses
	ContactsRev        int            // incrémenté à chaque écriture du carnet (détecte une modification pendant une synchronisation)
	ContactsSyncing    bool           // une synchronisation est en cours
	ContactsTried      time.Time      // dernier essai de synchronisation (automatique ou manuel)
	Contacts           *contacts.Book // carnet en cache (protégé par ContactsMu)
	TwoFactorSetup     string         // secret en cours d'installation (pas encore confirmé), protégé par Mu
	TwoFactorReveal    string         // secret révélé après un code valide, affiché une seule fois, protégé par Mu
	Flash              string         // clé d'un message à afficher une seule fois (protégée par Mu)
}

// Manager garde les sessions actives.
type Manager struct {
	mu  sync.Mutex
	m   map[string]*Session
	now func() time.Time
	// OnClose est appelée (dans une goroutine) avec l'identifiant d'une session fermée ou expirée.
	OnClose func(id string)
}

// NewManager crée un gestionnaire vide.
func NewManager() *Manager { return &Manager{m: map[string]*Session{}, now: time.Now} }

func token() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create ouvre une session pour un compte.
func (m *Manager) Create(acc *store.Account, idle, abs time.Duration) (*Session, error) {
	id, err := token()
	if err != nil {
		return nil, err
	}
	csrf, err := token()
	if err != nil {
		return nil, err
	}
	now := m.now()
	s := &Session{ID: id, Account: acc, CSRF: csrf, Created: now, LastSeen: now, Idle: idle, Abs: abs}
	m.mu.Lock()
	m.m[id] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) expired(s *Session, now time.Time) bool {
	return now.Sub(s.Created) > s.Abs || now.Sub(s.LastSeen) > s.Idle
}

func (m *Manager) drop(id string, s *Session) {
	delete(m.m, id)
	s.Account.Wipe()
	if m.OnClose != nil {
		go m.OnClose(id)
	}
}

// Get renvoie la session si elle est valide et prolonge l'inactivité.
func (m *Manager) Get(id string) *Session {
	if id == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.m[id]
	if s == nil {
		return nil
	}
	now := m.now()
	if m.expired(s, now) {
		m.drop(id, s)
		return nil
	}
	s.LastSeen = now
	return s
}

// SetTTL change les durées d'inactivité et maximale d'une session.
func (m *Manager) SetTTL(s *Session, idle, abs time.Duration) {
	m.mu.Lock()
	s.Idle, s.Abs = idle, abs
	m.mu.Unlock()
}

// Delete ferme une session.
func (m *Manager) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.m[id]; s != nil {
		m.drop(id, s)
	}
}

// DeleteUser ferme toutes les sessions d'un login, sauf exceptID (vide = aucune exception).
func (m *Manager) DeleteUser(login, exceptID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, s := range m.m {
		if s.Account.Login == login && id != exceptID {
			m.drop(id, s)
			n++
		}
	}
	return n
}

// Count renvoie le nombre de sessions (utile aux tests et aux journaux).
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.m)
}

// Sweep ferme les sessions expirées.
func (m *Manager) Sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	for id, s := range m.m {
		if m.expired(s, now) {
			m.drop(id, s)
		}
	}
}

// Run nettoie périodiquement jusqu'à l'annulation du contexte.
func (m *Manager) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Sweep()
		}
	}
}
