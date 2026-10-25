package vault

// Journal d'activité : écriture possible compte fermé.
//
// Le reste du coffre n'est accessible qu'avec la DEK, donc qu'après une connexion réussie. Or l'événement le plus
// utile — une tentative de connexion qui échoue — se produit précisément quand le compte reste fermé. D'où une paire
// de clés X25519 par compte : la privée vit dans le fichier chiffré, la publique en clair à côté. Le serveur scelle
// un événement pour cette clé publique sans rien connaître du compte ; l'utilisateur seul le relit, à sa connexion
// suivante.
//
// Le scellé est une « sealed box » : clé éphémère tirée à chaque écriture, secret partagé X25519, clé de chiffrement
// dérivée par HKDF en liant les deux clés publiques, puis XChaCha20-Poly1305. L'écrivain ne peut pas relire ce qu'il
// a écrit (il jette sa clé éphémère), et deux événements identiques donnent des octets différents.
//
// Ce que cela ne donne pas : l'authenticité. Qui peut écrire dans /data détient la clé publique, donc peut forger
// des entrées, et surtout en supprimer. Ce journal détecte l'abus de compte à distance, pas la compromission du
// serveur.

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	eventMagic   = "WMEV"
	eventVersion = 1
	// Taille d'une clé X25519, privée comme publique.
	EventKeyLen = 32
	maxEvent    = 64 << 10
)

// ErrEventFormat : entrée de journal illisible ou altérée.
var ErrEventFormat = errors.New("vault : entrée de journal invalide")

// NewEventKeys tire une paire de clés d'événements : la privée est à ranger dans le fichier chiffré du compte,
// la publique à côté, en clair.
func NewEventKeys() (priv, pub [EventKeyLen]byte, err error) {
	if _, err = io.ReadFull(rand.Reader, priv[:]); err != nil {
		return priv, pub, err
	}
	pub, err = EventPublic(priv)
	return priv, pub, err
}

// EventPublic recalcule la clé publique correspondant à une clé privée.
func EventPublic(priv [EventKeyLen]byte) ([EventKeyLen]byte, error) {
	var pub [EventKeyLen]byte
	b, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return pub, err
	}
	copy(pub[:], b)
	return pub, nil
}

// eventKey dérive la clé de chiffrement d'une entrée. Les deux clés publiques entrent dans l'info HKDF : un secret
// partagé réutilisé sous d'autres clés ne donnerait pas la même clé de chiffrement.
func eventKey(shared, epk, pub []byte, id string) []byte {
	info := make([]byte, 0, len(epk)+len(pub)+len(id)+24)
	info = append(info, "wmail-go event/v1/"...)
	info = append(info, id...)
	info = append(info, epk...)
	info = append(info, pub...)
	k := make([]byte, keyLen)
	r := hkdf.New(sha256.New, shared, nil, info)
	_, _ = io.ReadFull(r, k)
	return k
}

func eventAAD(id string) []byte { return []byte("wmev/v1/" + id) }

// SealEvent chiffre une entrée de journal pour le détenteur de pub. id lie l'entrée au compte : elle ne s'ouvre
// pas sous l'identifiant d'un autre.
func SealEvent(pub [EventKeyLen]byte, id string, plain []byte) ([]byte, error) {
	if len(plain) > maxEvent {
		return nil, errors.New("vault : entrée de journal trop grande")
	}
	esk, epk, err := NewEventKeys()
	if err != nil {
		return nil, err
	}
	shared, err := curve25519.X25519(esk[:], pub[:])
	if err != nil {
		return nil, err
	}
	nonce, ct, err := seal(eventKey(shared, epk[:], pub[:], id), eventAAD(id), plain)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(eventMagic)+1+EventKeyLen+nonceLen+len(ct))
	out = append(out, eventMagic...)
	out = append(out, eventVersion)
	out = append(out, epk[:]...)
	out = append(out, nonce[:]...)
	return append(out, ct...), nil
}

// OpenEvent déchiffre une entrée produite par SealEvent.
func OpenEvent(priv [EventKeyLen]byte, id string, data []byte) ([]byte, error) {
	hdr := len(eventMagic) + 1 + EventKeyLen + nonceLen
	if len(data) < hdr+chacha20poly1305.Overhead || string(data[:len(eventMagic)]) != eventMagic || data[len(eventMagic)] != eventVersion {
		return nil, ErrEventFormat
	}
	var epk [EventKeyLen]byte
	var nonce [nonceLen]byte
	copy(epk[:], data[len(eventMagic)+1:])
	copy(nonce[:], data[len(eventMagic)+1+EventKeyLen:])
	pub, err := EventPublic(priv)
	if err != nil {
		return nil, err
	}
	shared, err := curve25519.X25519(priv[:], epk[:])
	if err != nil {
		return nil, ErrEventFormat
	}
	plain, err := open(eventKey(shared, epk[:], pub[:], id), nonce, eventAAD(id), data[hdr:])
	if err != nil {
		return nil, ErrEventFormat
	}
	return plain, nil
}
