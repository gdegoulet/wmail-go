// Package contacts gère le carnet d'adresses d'un compte webmail : modèle, recherche, suggestions,
// import/export vCard et CSV. Le stockage chiffré est assuré par le paquet store.
package contacts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	MaxContacts = 5000
	MaxValues   = 20 // adresses, téléphones, URL par contact
	MaxTags     = 20
	MaxField    = 500 // longueur d'un champ texte simple
	MaxNotes    = 10000
	PageSize    = 50
)

var (
	ErrNotFound = errors.New("contacts : contact introuvable")
	ErrFull     = errors.New("contacts : carnet plein")
	ErrEmpty    = errors.New("contacts : contact vide")
)

// Typed est une valeur étiquetée (home, work, mobile…).
type Typed struct {
	Type  string `json:"t,omitempty"`
	Value string `json:"v"`
}

// Postal est une adresse postale.
type Postal struct {
	Type    string `json:"t,omitempty"`
	Street  string `json:"s,omitempty"`
	City    string `json:"c,omitempty"`
	Region  string `json:"r,omitempty"`
	Code    string `json:"z,omitempty"`
	Country string `json:"k,omitempty"`
}

// Contact est une fiche du carnet.
type Contact struct {
	ID       string   `json:"id"`
	UID      string   `json:"uid,omitempty"` // UID vCard / CardDAV
	Prefix   string   `json:"pre,omitempty"`
	First    string   `json:"fn,omitempty"`
	Middle   string   `json:"mn,omitempty"`
	Last     string   `json:"ln,omitempty"`
	Suffix   string   `json:"suf,omitempty"`
	Nick     string   `json:"nick,omitempty"`
	Emails   []Typed  `json:"em,omitempty"`
	Phones   []Typed  `json:"ph,omitempty"`
	URLs     []Typed  `json:"url,omitempty"`
	Postals  []Postal `json:"adr,omitempty"`
	Org      string   `json:"org,omitempty"`
	Title    string   `json:"ttl,omitempty"`
	Birthday string   `json:"bd,omitempty"` // AAAA-MM-JJ
	Notes    string   `json:"note,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Frec     int      `json:"frec,omitempty"`
	Changed  int64    `json:"ch,omitempty"`
	Href     string   `json:"href,omitempty"` // ressource CardDAV
	ETag     string   `json:"etag,omitempty"`
}

// Display donne le nom affiché : prénom + nom, sinon surnom, organisation, première adresse.
func (c *Contact) Display() string {
	n := strings.TrimSpace(strings.Join(nonEmpty(c.Prefix, c.First, c.Middle, c.Last, c.Suffix), " "))
	switch {
	case n != "":
		return n
	case c.Nick != "":
		return c.Nick
	case c.Org != "":
		return c.Org
	case len(c.Emails) > 0:
		return c.Emails[0].Value
	case len(c.Phones) > 0:
		return c.Phones[0].Value
	}
	return ""
}

// SortName sert au tri : nom, prénom, puis affichage.
func (c *Contact) SortName() string {
	k := strings.Join(nonEmpty(c.Last, c.First), " ")
	if k == "" {
		k = c.Display()
	}
	return fold(k)
}

func nonEmpty(v ...string) []string {
	out := v[:0:0]
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Normalize nettoie et borne les champs ; renvoie ErrEmpty si la fiche ne contient rien d'utile.
func (c *Contact) Normalize() error {
	cut := func(s string, n int) string {
		s = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r < ' ' && r != '\n' && r != '\t' {
				return -1
			}
			return r
		}, s))
		if r := []rune(s); len(r) > n {
			s = string(r[:n])
		}
		return s
	}
	line := func(s string) string { return cut(strings.NewReplacer("\n", " ", "\t", " ").Replace(s), MaxField) }
	c.Prefix, c.First, c.Middle, c.Last, c.Suffix = line(c.Prefix), line(c.First), line(c.Middle), line(c.Last), line(c.Suffix)
	c.Nick, c.Org, c.Title = line(c.Nick), line(c.Org), line(c.Title)
	c.Notes = cut(c.Notes, MaxNotes)
	if _, err := time.Parse("2006-01-02", c.Birthday); err != nil {
		c.Birthday = ""
	}
	typed := func(in []Typed, mail bool) []Typed {
		var out []Typed
		seen := map[string]bool{}
		for _, t := range in {
			t.Value, t.Type = line(t.Value), strings.ToLower(line(t.Type))
			if t.Value == "" {
				continue
			}
			if mail {
				t.Value = strings.ToLower(t.Value)
				if !strings.Contains(t.Value, "@") || strings.ContainsAny(t.Value, " <>,;\"") {
					continue
				}
			}
			if seen[t.Value] || len(out) >= MaxValues {
				continue
			}
			seen[t.Value] = true
			out = append(out, t)
		}
		return out
	}
	c.Emails, c.Phones, c.URLs = typed(c.Emails, true), typed(c.Phones, false), typed(c.URLs, false)
	var ps []Postal
	for _, p := range c.Postals {
		p = Postal{line(p.Type), line(p.Street), line(p.City), line(p.Region), line(p.Code), line(p.Country)}
		if (p != Postal{Type: p.Type}) && len(ps) < MaxValues {
			ps = append(ps, p)
		}
	}
	c.Postals = ps
	var tags []string
	seen := map[string]bool{}
	for _, t := range c.Tags {
		t = line(strings.ReplaceAll(t, ",", " "))
		if t != "" && !seen[fold(t)] && len(tags) < MaxTags {
			seen[fold(t)] = true
			tags = append(tags, t)
		}
	}
	c.Tags = tags
	if c.Display() == "" && c.Notes == "" && len(c.Postals) == 0 && len(c.URLs) == 0 {
		return ErrEmpty
	}
	return nil
}

// Book est le carnet complet d'un compte.
type Book struct {
	Contacts []*Contact `json:"contacts"`
	// SyncedAt : instant (secondes) du dernier passage CardDAV réussi ; Deleted : ressources distantes à supprimer
	// (contacts effacés ici depuis) ; SyncError : dernier échec, vide si le dernier passage a réussi.
	SyncedAt  int64    `json:"synced,omitempty"`
	Deleted   []string `json:"deleted,omitempty"`
	SyncError string   `json:"sync_error,omitempty"`
}

// Parse décode un carnet sérialisé (nil → carnet vide).
func Parse(data []byte) (*Book, error) {
	b := &Book{}
	if len(data) == 0 {
		return b, nil
	}
	if err := json.Unmarshal(data, b); err != nil {
		return nil, err
	}
	return b, nil
}

// Marshal sérialise le carnet.
func (b *Book) Marshal() ([]byte, error) { return json.Marshal(b) }

func newID() string {
	var r [12]byte
	_, _ = rand.Read(r[:])
	return hex.EncodeToString(r[:])
}

// Get renvoie un contact par identifiant.
func (b *Book) Get(id string) *Contact {
	for _, c := range b.Contacts {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// ByEmail renvoie le contact portant cette adresse.
func (b *Book) ByEmail(addr string) *Contact {
	addr = strings.ToLower(strings.TrimSpace(addr))
	for _, c := range b.Contacts {
		for _, e := range c.Emails {
			if e.Value == addr {
				return c
			}
		}
	}
	return nil
}

// Save ajoute ou met à jour (selon ID) une fiche normalisée.
func (b *Book) Save(c *Contact) error {
	if err := c.Normalize(); err != nil {
		return err
	}
	c.Changed = time.Now().Unix()
	if c.ID == "" {
		if len(b.Contacts) >= MaxContacts {
			return ErrFull
		}
		c.ID = newID()
		if c.UID == "" {
			c.UID = newID()
		}
		b.Contacts = append(b.Contacts, c)
		return nil
	}
	for i, old := range b.Contacts {
		if old.ID == c.ID {
			c.Frec, c.UID, c.Href, c.ETag = old.Frec, firstNonEmpty(c.UID, old.UID), old.Href, old.ETag
			b.Contacts[i] = c
			return nil
		}
	}
	return ErrNotFound
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// Delete supprime des contacts ; renvoie le nombre supprimé.
func (b *Book) Delete(ids ...string) int {
	del := map[string]bool{}
	for _, id := range ids {
		del[id] = true
	}
	kept := b.Contacts[:0]
	n := 0
	for _, c := range b.Contacts {
		if del[c.ID] {
			if c.Href != "" {
				b.Deleted = append(b.Deleted, c.Href)
			}
			n++
			continue
		}
		kept = append(kept, c)
	}
	b.Contacts = kept
	return n
}

// AddAddress crée un contact minimal pour une adresse inconnue (ajout automatique après envoi).
func (b *Book) AddAddress(name, addr string) (*Contact, bool) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	if c := b.ByEmail(addr); c != nil {
		return c, false
	}
	c := &Contact{Emails: []Typed{{Value: addr}}}
	if name = strings.TrimSpace(name); name != "" && name != addr {
		if i := strings.LastIndex(name, " "); i > 0 {
			c.First, c.Last = name[:i], name[i+1:]
		} else {
			c.First = name
		}
	}
	if b.Save(c) != nil {
		return nil, false
	}
	return c, true
}

// Bump augmente la fréquence d'usage du contact portant cette adresse.
func (b *Book) Bump(addr string) {
	if c := b.ByEmail(addr); c != nil {
		c.Frec++
	}
}

var foldT = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// fold met en minuscules sans accents pour comparer.
func fold(s string) string {
	out, _, err := transform.String(foldT, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

func (c *Contact) haystack() string {
	parts := []string{c.Display(), c.Nick, c.Org, c.Notes}
	for _, t := range c.Emails {
		parts = append(parts, t.Value)
	}
	for _, t := range c.Phones {
		parts = append(parts, t.Value)
	}
	for _, t := range c.URLs {
		parts = append(parts, t.Value)
	}
	parts = append(parts, c.Tags...)
	return fold(strings.Join(parts, "\n"))
}

// Search filtre (texte et/ou étiquette) et trie par nom ; page commence à 1.
func (b *Book) Search(q, tag string, page int) (list []*Contact, total int) {
	q, tag = fold(strings.TrimSpace(q)), fold(strings.TrimSpace(tag))
	var all []*Contact
	for _, c := range b.Contacts {
		if tag != "" && !hasTag(c, tag) {
			continue
		}
		if q != "" && !strings.Contains(c.haystack(), q) {
			continue
		}
		all = append(all, c)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].SortName() < all[j].SortName() })
	total = len(all)
	if page < 1 {
		page = 1
	}
	lo := (page - 1) * PageSize
	if lo >= total {
		return nil, total
	}
	return all[lo:min(lo+PageSize, total)], total
}

func hasTag(c *Contact, folded string) bool {
	for _, t := range c.Tags {
		if fold(t) == folded {
			return true
		}
	}
	return false
}

// AllTags liste les étiquettes utilisées, triées.
func (b *Book) AllTags() []string {
	set := map[string]string{}
	for _, c := range b.Contacts {
		for _, t := range c.Tags {
			set[fold(t)] = t
		}
	}
	out := make([]string, 0, len(set))
	for _, t := range set {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return fold(out[i]) < fold(out[j]) })
	return out
}

// Suggestion est une proposition d'autocomplétion.
type Suggestion struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Label string `json:"label"` // « Nom <adresse> » prêt à insérer
}

// Suggest propose des adresses dont le nom ou l'adresse contient q, les plus utilisées d'abord.
func (b *Book) Suggest(q string, limit int) []Suggestion {
	q = fold(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	type hit struct {
		s    Suggestion
		frec int
		key  string
	}
	var hits []hit
	for _, c := range b.Contacts {
		name := c.Display()
		for _, e := range c.Emails {
			if !strings.Contains(fold(name+" "+c.Nick+" "+e.Value), q) {
				continue
			}
			label := e.Value
			if name != "" && name != e.Value {
				label = FormatAddress(name, e.Value)
			}
			hits = append(hits, hit{Suggestion{name, e.Value, label}, c.Frec, c.SortName()})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].frec != hits[j].frec {
			return hits[i].frec > hits[j].frec
		}
		return hits[i].key < hits[j].key
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Suggestion, len(hits))
	for i, h := range hits {
		out[i] = h.s
	}
	return out
}

// FormatAddress met en forme « Nom <adresse> » avec guillemets si nécessaire.
func FormatAddress(name, addr string) string {
	if strings.ContainsAny(name, `",;:<>()[]@\`) {
		name = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name) + `"`
	}
	return name + " <" + addr + ">"
}
