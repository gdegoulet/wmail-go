package store

// Journal d'activité d'un compte : deux fichiers à côté du .cfg.
//
//	<id>.pub  clé publique X25519, en clair : elle permet au serveur d'écrire sans ouvrir le compte
//	<id>.evt  file d'attente, ajout seul, une entrée scellée par événement
//	<id>.act  historique compacté, chiffré par la DEK (données annexes, comme le carnet d'adresses)
//
// Tout événement passe d'abord par .evt — y compris ceux d'une session ouverte, pour n'avoir qu'un seul chemin
// d'écriture, sans relecture ni réécriture concurrente. Le passage de .evt vers .act se fait quand la DEK est
// disponible : à la connexion, et à l'affichage du journal.

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/gdegoulet/wmail-go/internal/vault"
)

const (
	pubFileExtension = ".pub"
	evtFileExtension = ".evt"
	evtMagic         = "WMEL"
	evtVersion       = 1
	evtHeaderLen     = len(evtMagic) + 1 + 4 // magie, version, nombre d'entrées perdues

	// Bornes de la file d'attente. Elle est alimentée par des tentatives de connexion, donc par n'importe qui :
	// sans plafond, un attaquant remplirait le disque. Au-delà, les entrées du milieu sont jetées et comptées.
	maxEvtBytes   = 256 << 10
	maxEvtEntries = 400
	keepFirstEvt  = 100 // les premières tentatives d'une rafale disent qui et quand
	keepLastEvt   = 200 // les dernières disent où ça en est
)

// ErrEventLogCorrupt : la file d'attente du journal est illisible.
var ErrEventLogCorrupt = errors.New("journal d'activité illisible")

func (s *Store) pubPath(id string) string {
	return strings.TrimSuffix(s.path(id), userFileExtension) + pubFileExtension
}

func (s *Store) evtPath(id string) string {
	return strings.TrimSuffix(s.path(id), userFileExtension) + evtFileExtension
}

// EnsureEventKeys dote le compte d'une paire de clés d'événements s'il n'en a pas. Elle renvoie vrai quand le
// compte a été modifié : à l'appelant d'enregistrer les données (SaveData). Les comptes créés avant le journal
// d'activité passent ici à leur première connexion.
func (s *Store) EnsureEventKeys(acc *Account) (bool, error) {
	if acc.Data.EventKey != "" {
		if _, err := os.Stat(s.pubPath(acc.ID)); err == nil {
			return false, nil
		}
		// Clé privée présente, publique perdue (restauration partielle) : on republie la publique.
		priv, err := eventPriv(acc.Data.EventKey)
		if err != nil {
			return false, err
		}
		pub, err := vault.EventPublic(priv)
		if err != nil {
			return false, err
		}
		return false, writeAtomic(s.pubPath(acc.ID), pub[:])
	}
	priv, pub, err := vault.NewEventKeys()
	if err != nil {
		return false, err
	}
	if err := writeAtomic(s.pubPath(acc.ID), pub[:]); err != nil {
		return false, err
	}
	acc.Data.EventKey = base64.StdEncoding.EncodeToString(priv[:])
	return true, nil
}

func eventPriv(encoded string) ([vault.EventKeyLen]byte, error) {
	var priv [vault.EventKeyLen]byte
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return priv, err
	}
	if len(b) != vault.EventKeyLen {
		return priv, errors.New("store : clé de journal de taille inattendue")
	}
	copy(priv[:], b)
	return priv, nil
}

// AppendEvent ajoute une entrée au journal d'un compte, sans l'ouvrir. Un login inconnu, ou un compte sans clé
// publique, ne donne pas d'erreur : il n'y a rien à écrire, et signaler la différence révélerait quels comptes
// existent.
func (s *Store) AppendEvent(login string, plain []byte) error {
	nl, err := NormalizeLogin(login)
	if err != nil {
		return nil
	}
	return s.AppendEventID(s.ID(nl), plain)
}

// AppendEventID est AppendEvent pour un compte déjà identifié (session ouverte).
func (s *Store) AppendEventID(id string, plain []byte) error {
	raw, err := os.ReadFile(s.pubPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(raw) != vault.EventKeyLen {
		return nil
	}
	var pub [vault.EventKeyLen]byte
	copy(pub[:], raw)
	sealed, err := vault.SealEvent(pub, id, plain)
	if err != nil {
		return err
	}
	return s.withLock(func() error {
		data, dropped, entries, err := s.readEvents(id)
		if err != nil {
			// File illisible (format inconnu, octets corrompus) : on repart d'une file vide plutôt que de
			// perdre l'événement courant, qui est peut-être justement celui qui expliquerait l'incident.
			data, dropped, entries = nil, 0, nil
		}
		entries = append(entries, sealed)
		entries, dropped = pruneEvents(entries, dropped, len(data)+len(sealed))
		return writeAtomic(s.evtPath(id), marshalEvents(entries, dropped))
	})
}

// readEvents lit la file d'attente : contenu brut, compteur de pertes et entrées scellées.
func (s *Store) readEvents(id string) (data []byte, dropped uint32, entries [][]byte, err error) {
	data, err = os.ReadFile(s.evtPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil, nil
	}
	if err != nil {
		return nil, 0, nil, err
	}
	if len(data) < evtHeaderLen || string(data[:len(evtMagic)]) != evtMagic || data[len(evtMagic)] != evtVersion {
		return nil, 0, nil, ErrEventLogCorrupt
	}
	dropped = binary.BigEndian.Uint32(data[len(evtMagic)+1:])
	for p := evtHeaderLen; p < len(data); {
		if p+4 > len(data) {
			return nil, 0, nil, ErrEventLogCorrupt
		}
		n := int(binary.BigEndian.Uint32(data[p:]))
		p += 4
		if n < 0 || p+n > len(data) {
			return nil, 0, nil, ErrEventLogCorrupt
		}
		entries = append(entries, data[p:p+n])
		p += n
	}
	return data, dropped, entries, nil
}

// pruneEvents applique les plafonds : au-delà, les entrées du milieu disparaissent et sont comptées. Les plus
// anciennes sont gardées autant que les plus récentes, car dans une rafale ce sont elles qui datent le début.
func pruneEvents(entries [][]byte, dropped uint32, size int) ([][]byte, uint32) {
	if len(entries) <= maxEvtEntries && size <= maxEvtBytes {
		return entries, dropped
	}
	if len(entries) <= keepFirstEvt+keepLastEvt {
		return entries, dropped
	}
	perdues := len(entries) - keepFirstEvt - keepLastEvt
	out := make([][]byte, 0, keepFirstEvt+keepLastEvt)
	out = append(out, entries[:keepFirstEvt]...)
	out = append(out, entries[len(entries)-keepLastEvt:]...)
	return out, dropped + uint32(perdues)
}

func marshalEvents(entries [][]byte, dropped uint32) []byte {
	n := evtHeaderLen
	for _, e := range entries {
		n += 4 + len(e)
	}
	out := make([]byte, 0, n)
	out = append(out, evtMagic...)
	out = append(out, evtVersion)
	out = binary.BigEndian.AppendUint32(out, dropped)
	for _, e := range entries {
		out = binary.BigEndian.AppendUint32(out, uint32(len(e)))
		out = append(out, e...)
	}
	return out
}

// DrainEvents vide la file d'attente et renvoie les entrées déchiffrées, plus le nombre d'entrées perdues par
// débordement. Les entrées illisibles sont ignorées : une seule ne doit pas emporter le journal entier.
func (s *Store) DrainEvents(acc *Account) ([][]byte, int, error) {
	if acc.Data.EventKey == "" {
		return nil, 0, nil
	}
	priv, err := eventPriv(acc.Data.EventKey)
	if err != nil {
		return nil, 0, err
	}
	var entries [][]byte
	var dropped uint32
	err = s.withLock(func() error {
		_, d, raw, err := s.readEvents(acc.ID)
		if err != nil {
			// File corrompue : on la jette, sinon elle bloque le journal pour toujours.
			dropped = 1
			return os.Remove(s.evtPath(acc.ID))
		}
		dropped = d
		entries = raw
		if len(raw) == 0 && d == 0 {
			return nil
		}
		if err := os.Remove(s.evtPath(acc.ID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([][]byte, 0, len(entries))
	for _, e := range entries {
		plain, err := vault.OpenEvent(priv, acc.ID, e)
		if err != nil {
			dropped++
			continue
		}
		out = append(out, plain)
	}
	return out, int(dropped), nil
}

// LoadActivity lit l'historique compacté du compte ; nil s'il n'y en a pas encore.
func (s *Store) LoadActivity(acc *Account) ([]byte, error) { return s.LoadBlob(acc, "activity") }

// SaveActivity réécrit l'historique compacté ; un contenu vide le supprime.
func (s *Store) SaveActivity(acc *Account, plain []byte) error {
	return s.SaveBlob(acc, "activity", plain)
}
