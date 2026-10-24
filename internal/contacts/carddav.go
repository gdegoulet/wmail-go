package contacts

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
)

// SyncConfig décrit le carnet CardDAV distant.
type SyncConfig struct {
	URL, User, Password string
}

// SyncResult résume un passage.
type SyncResult struct {
	Pulled, Pushed, Deleted, RemoteDeleted int
}

func (r SyncResult) String() string {
	return fmt.Sprintf("reçus %d, envoyés %d, supprimés ici %d, supprimés là-bas %d", r.Pulled, r.Pushed, r.Deleted, r.RemoteDeleted)
}

// ValidSyncURL vérifie l'adresse du carnet (https, ou http si allowHTTP, sans identifiants intégrés).
func ValidSyncURL(raw string, allowHTTP func(host string) bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || len(raw) > 500 {
		return errors.New("adresse invalide")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if allowHTTP == nil || !allowHTTP(u.Hostname()) {
			return errors.New("l'adresse doit commencer par https://")
		}
	default:
		return errors.New("l'adresse doit commencer par https://")
	}
	return nil
}

// SyncCardDAV synchronise b avec le carnet distant : les contacts nouveaux ou modifiés de chaque côté sont repris de
// l'autre ; en cas de modification des deux côtés, la plus récente l'emporte. Les propriétés vCard que ce carnet ne
// connaît pas (photo, champs privés) sont perdues quand un contact modifié ici est renvoyé.
func (b *Book) SyncCardDAV(ctx context.Context, httpc webdav.HTTPClient, cfg SyncConfig) (SyncResult, error) {
	var res SyncResult
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return res, err
	}
	client, err := carddav.NewClient(webdav.HTTPClientWithBasicAuth(httpc, cfg.User, cfg.Password), cfg.URL)
	if err != nil {
		return res, err
	}
	collection := u.Path
	objs, err := client.QueryAddressBook(ctx, collection, &carddav.AddressBookQuery{DataRequest: carddav.AddressDataRequest{AllProp: true}})
	if err != nil {
		// L'adresse n'est peut-être pas celle d'un carnet : découverte du premier carnet de l'utilisateur.
		home, e1 := client.FindAddressBookHomeSet(ctx, "")
		if e1 != nil {
			return res, err
		}
		books, e2 := client.FindAddressBooks(ctx, home)
		if e2 != nil || len(books) == 0 {
			return res, err
		}
		collection = books[0].Path
		if objs, err = client.QueryAddressBook(ctx, collection, &carddav.AddressBookQuery{DataRequest: carddav.AddressDataRequest{AllProp: true}}); err != nil {
			return res, err
		}
	}
	start := time.Now().Unix()
	prevSync := b.SyncedAt
	locallyChanged := func(c *Contact) bool { return c.Changed > prevSync }

	remote := map[string]carddav.AddressObject{}
	for _, o := range objs {
		remote[o.Path] = o
	}
	tomb := map[string]bool{}
	for _, h := range b.Deleted {
		tomb[h] = true
	}
	byUID := map[string]*Contact{}
	for _, c := range b.Contacts {
		if c.UID != "" {
			byUID[c.UID] = c
		}
	}

	// Distant → local.
	pulledNow := map[*Contact]bool{}
	for p, o := range remote {
		if tomb[p] {
			continue
		}
		rc := fromCard(o.Card)
		if rc.Normalize() != nil {
			continue
		}
		local := (*Contact)(nil)
		for _, c := range b.Contacts {
			if c.Href == p {
				local = c
				break
			}
		}
		if local == nil && rc.UID != "" {
			local = byUID[rc.UID] // contact déjà connu (importé ou envoyé avant) mais sans lien
		}
		changed := min(prevSync, start)
		switch {
		case local == nil:
			if len(b.Contacts) >= MaxContacts {
				continue
			}
			rc.ID, rc.Href, rc.ETag, rc.Changed = newID(), p, o.ETag, changed
			if rc.UID == "" {
				rc.UID = newID()
			}
			b.Contacts = append(b.Contacts, rc)
			res.Pulled++
		case local.Href == p && local.ETag == o.ETag && o.ETag != "":
			// inchangé à distance
		default:
			if locallyChanged(local) && local.Href == p && o.ModTime.Unix() < local.Changed {
				local.ETag = o.ETag // le plus récent est ici : sera renvoyé plus bas
				continue
			}
			rc.ID, rc.Frec = local.ID, local.Frec
			rc.Href, rc.ETag, rc.Changed = p, o.ETag, changed
			if rc.UID == "" {
				rc.UID = local.UID
			}
			*local = *rc
			pulledNow[local] = true
			res.Pulled++
		}
	}

	// Contacts liés dont la ressource a disparu à distance.
	kept := b.Contacts[:0]
	for _, c := range b.Contacts {
		if c.Href != "" {
			if _, ok := remote[c.Href]; !ok {
				if !locallyChanged(c) {
					res.Deleted++
					continue
				}
				c.Href, c.ETag = "", "" // modifié ici : recréé là-bas
			}
		}
		kept = append(kept, c)
	}
	b.Contacts = kept

	// Local → distant.
	for _, c := range b.Contacts {
		if c.Href != "" && (!locallyChanged(c) || pulledNow[c]) {
			continue
		}
		if c.UID == "" {
			c.UID = newID()
		}
		target := c.Href
		if target == "" {
			target = path.Join(collection, safeName(c.UID)+".vcf")
		}
		card := toCard(c)
		ao, err := client.PutAddressObject(ctx, target, card)
		if err != nil {
			return res, fmt.Errorf("envoi de %q : %w", c.Display(), err)
		}
		c.Href, c.ETag = target, ao.ETag
		if c.ETag == "" {
			if got, err := client.GetAddressObject(ctx, target); err == nil {
				c.ETag = got.ETag
			}
		}
		res.Pushed++
	}

	// Suppressions faites ici.
	for _, h := range b.Deleted {
		if _, still := remote[h]; !still {
			continue // déjà supprimée là-bas
		}
		if err := client.RemoveAll(ctx, h); err != nil {
			return res, fmt.Errorf("suppression distante : %w", err)
		}
		res.RemoteDeleted++
	}
	b.Deleted = nil
	b.SyncedAt = start
	b.SyncError = ""
	return res, nil
}

func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
