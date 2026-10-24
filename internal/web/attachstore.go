package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
)

// Pièces jointes en attente d'envoi, gardées en mémoire (jamais écrites en clair sur le disque du serveur), par
// session puis par composition. Elles disparaissent à l'envoi, à l'abandon, à la fermeture de la session.

var (
	errAttachTooBig   = errors.New("pièce jointe trop volumineuse")
	errAttachTooMany  = errors.New("trop de pièces jointes")
	errAttachMsgLimit = errors.New("ensemble des pièces jointes trop volumineux")
	errAttachSession  = errors.New("mémoire des pièces jointes de la session épuisée")
)

type pendingAttachment struct {
	ID, Name, ContentType string
	Data                  []byte
	FromMessage           bool // reprise d'un message existant (transfert, brouillon)
}

type attachLimits struct {
	Each, Message, Session int64
	Count                  int
}

type attachStore struct {
	mu     sync.Mutex
	lim    attachLimits
	sess   map[string]map[string][]*pendingAttachment // session → composition → pièces
	sizeOf map[string]int64                           // session → octets
}

func newAttachStore(l attachLimits) *attachStore {
	return &attachStore{lim: l, sess: map[string]map[string][]*pendingAttachment{}, sizeOf: map[string]int64{}}
}

func randomID() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sum(list []*pendingAttachment) (n int64) {
	for _, a := range list {
		n += int64(len(a.Data))
	}
	return n
}

// Add ajoute une pièce à une composition en respectant les limites.
func (s *attachStore) Add(sessID, compID string, a pendingAttachment) (*pendingAttachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	size := int64(len(a.Data))
	list := s.sess[sessID][compID]
	switch {
	case size > s.lim.Each:
		return nil, errAttachTooBig
	case len(list) >= s.lim.Count:
		return nil, errAttachTooMany
	case sum(list)+size > s.lim.Message:
		return nil, errAttachMsgLimit
	case s.sizeOf[sessID]+size > s.lim.Session:
		return nil, errAttachSession
	}
	if s.sess[sessID] == nil {
		s.sess[sessID] = map[string][]*pendingAttachment{}
	}
	a.ID = randomID()
	p := &a
	s.sess[sessID][compID] = append(list, p)
	s.sizeOf[sessID] += size
	return p, nil
}

// List renvoie les pièces d'une composition, dans l'ordre d'ajout.
func (s *attachStore) List(sessID, compID string) []*pendingAttachment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*pendingAttachment(nil), s.sess[sessID][compID]...)
}

// Remove retire une pièce ; il renvoie faux si elle n'existe pas.
func (s *attachStore) Remove(sessID, compID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.sess[sessID][compID]
	for i, a := range list {
		if a.ID == id {
			s.sizeOf[sessID] -= int64(len(a.Data))
			s.sess[sessID][compID] = append(list[:i:i], list[i+1:]...)
			if len(s.sess[sessID][compID]) == 0 {
				delete(s.sess[sessID], compID)
			}
			if len(s.sess[sessID]) == 0 {
				delete(s.sess, sessID)
				delete(s.sizeOf, sessID)
			}
			return true
		}
	}
	return false
}

// Clear oublie toutes les pièces d'une composition.
func (s *attachStore) Clear(sessID, compID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sizeOf[sessID] -= sum(s.sess[sessID][compID])
	delete(s.sess[sessID], compID)
	if len(s.sess[sessID]) == 0 {
		delete(s.sess, sessID)
		delete(s.sizeOf, sessID)
	}
}

// ClearSession oublie tout ce qui appartient à une session.
func (s *attachStore) ClearSession(sessID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sess, sessID)
	delete(s.sizeOf, sessID)
}

// Compositions renvoie les identifiants des compositions d'une session (triés, pour les tests).
func (s *attachStore) Compositions(sessID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.sess[sessID] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
