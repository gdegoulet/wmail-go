// Package davtest fournit un serveur CardDAV en mémoire (avec authentification Basic) pour tester la synchronisation.
package davtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
)

// BookPath est le chemin du carnet unique du serveur.
const BookPath = "/dav/books/default/"

// Backend garde les fiches.
type Backend struct {
	mu   sync.Mutex
	objs map[string]*carddav.AddressObject
	n    int
}

func (m *Backend) CurrentUserPrincipal(ctx context.Context) (string, error) { return "/dav/me/", nil }
func (m *Backend) AddressBookHomeSetPath(ctx context.Context) (string, error) {
	return "/dav/books/", nil
}
func (m *Backend) ListAddressBooks(ctx context.Context) ([]carddav.AddressBook, error) {
	return []carddav.AddressBook{{Path: BookPath, Name: "Défaut"}}, nil
}
func (m *Backend) GetAddressBook(ctx context.Context, p string) (*carddav.AddressBook, error) {
	return &carddav.AddressBook{Path: BookPath, Name: "Défaut"}, nil
}
func (m *Backend) CreateAddressBook(ctx context.Context, ab *carddav.AddressBook) error {
	return fmt.Errorf("non pris en charge")
}
func (m *Backend) DeleteAddressBook(ctx context.Context, p string) error {
	return fmt.Errorf("non pris en charge")
}
func (m *Backend) GetAddressObject(ctx context.Context, p string, req *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.objs[p]
	if !ok {
		return nil, webdav.NewHTTPError(404, fmt.Errorf("absent"))
	}
	c := *o
	return &c, nil
}
func (m *Backend) list() []carddav.AddressObject {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []carddav.AddressObject
	for _, o := range m.objs {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
func (m *Backend) ListAddressObjects(ctx context.Context, p string, req *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	return m.list(), nil
}
func (m *Backend) QueryAddressObjects(ctx context.Context, p string, q *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	if len(q.PropFilters) == 0 { // sans filtre : tout le carnet, comme les serveurs usuels
		return m.list(), nil
	}
	return carddav.Filter(q, m.list())
}
func (m *Backend) PutAddressObject(ctx context.Context, p string, card vcard.Card, opts *carddav.PutAddressObjectOptions) (*carddav.AddressObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n++
	o := &carddav.AddressObject{Path: p, Card: card, ModTime: time.Now(), ETag: fmt.Sprintf("e%d", m.n)}
	m.objs[p] = o
	c := *o
	return &c, nil
}
func (m *Backend) DeleteAddressObject(ctx context.Context, p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objs, p)
	return nil
}

func card(uid, name, mail string) vcard.Card {
	c := make(vcard.Card)
	c.SetValue(vcard.FieldVersion, "3.0")
	c.SetValue(vcard.FieldUID, uid)
	c.SetValue(vcard.FieldFormattedName, name)
	c.SetName(&vcard.Name{GivenName: name})
	c.SetValue(vcard.FieldEmail, mail)
	return c
}

// Put crée ou remplace la fiche d'un autre client (nom de ressource = name).
func (m *Backend) Put(name, mail string, mod time.Time) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n++
	p := BookPath + name + ".vcf"
	m.objs[p] = &carddav.AddressObject{Path: p, Card: card("uid-"+name, name, mail), ModTime: mod, ETag: fmt.Sprintf("e%d", m.n)}
	return p
}

// Edit change le prénom d'une ressource existante (autre client), avec une nouvelle date et un nouvel ETag.
func (m *Backend) Edit(p, first string, mod time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.objs[p]
	if !ok {
		return fmt.Errorf("ressource %s absente", p)
	}
	uid, mail := o.Card.Value(vcard.FieldUID), o.Card.Value(vcard.FieldEmail)
	m.n++
	m.objs[p] = &carddav.AddressObject{Path: p, Card: card(uid, first, mail), ModTime: mod, ETag: fmt.Sprintf("e%d", m.n)}
	return nil
}

// Remove supprime une ressource (autre client).
func (m *Backend) Remove(p string) {
	m.mu.Lock()
	delete(m.objs, p)
	m.mu.Unlock()
}

// Names liste les noms des fiches, triés.
func (m *Backend) Names() []string {
	var out []string
	for _, o := range m.list() {
		out = append(out, o.Card.PreferredValue(vcard.FieldFormattedName))
	}
	sort.Strings(out)
	return out
}

// Objects renvoie les ressources.
func (m *Backend) Objects() []carddav.AddressObject { return m.list() }

// Start lance le serveur (utilisateur « alice », mot de passe « secret ») ; renvoie l'adresse du carnet.
func Start(t testing.TB) (url string, be *Backend, client *http.Client) {
	t.Helper()
	be = &Backend{objs: map[string]*carddav.AddressObject{}}
	h := &carddav.Handler{Backend: be}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "alice" || p != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
			http.Error(w, "non autorisé", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + BookPath, be, srv.Client()
}
