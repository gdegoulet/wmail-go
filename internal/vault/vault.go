// Package vault implémente le fichier utilisateur chiffré (.cfg) :
// une clé de données (DEK) aléatoire chiffre le contenu, et deux « slots »
// contiennent la DEK chiffrée, l'un par le mot de passe (argon2id), l'autre par la clé de récupération.
//
// Format (big-endian) :
//
//	"WMCF" | version(1) | time(4) | memoryKiB(4) | threads(1) | salt(16)
//	| slot mot de passe : nonce(24) + DEK chiffrée(48)
//	| slot récupération : nonce(24) + DEK chiffrée(48)
//	| nonce contenu(24) | longueur contenu(4) | contenu chiffré
//
// Le contenu est authentifié avec le magic, la version et l'identifiant du fichier :
// un fichier ne peut pas être déplacé vers un autre utilisateur. Les slots sont
// indépendants du contenu : changer le mot de passe ne rechiffre pas le contenu.
package vault

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/text/unicode/norm"
)

const (
	magic      = "WMCF"
	version    = 1
	saltLen    = 16
	keyLen     = 32
	nonceLen   = chacha20poly1305.NonceSizeX
	wrapLen    = keyLen + chacha20poly1305.Overhead
	headerLen  = 4 + 1 + 4 + 4 + 1 + saltLen + 2*(nonceLen+wrapLen) + nonceLen + 4
	maxContent = 8 << 20
)

var (
	// ErrInvalid : mot de passe ou clé de récupération incorrect.
	ErrInvalid = errors.New("vault : identifiants invalides")
	// ErrFormat : fichier illisible ou altéré.
	ErrFormat = errors.New("vault : format de fichier invalide")
)

// DEK est la clé de données d'un fichier.
type DEK [keyLen]byte

// Wipe efface la clé de la mémoire (au mieux).
func (d *DEK) Wipe() {
	for i := range d {
		d[i] = 0
	}
}

// Params regroupe les paramètres argon2id enregistrés dans le fichier.
type Params struct {
	Time      uint32
	MemoryKiB uint32
	Threads   uint8
}

func (p Params) validate() error {
	if p.Time < 1 || p.Time > 20 || p.MemoryKiB < 8 || p.MemoryKiB > 1<<20 || p.Threads < 1 || p.Threads > 64 {
		return fmt.Errorf("%w : paramètres argon2id hors limites", ErrFormat)
	}
	return nil
}

// Service dérive les clés et limite le nombre de dérivations simultanées.
type Service struct {
	params Params
	sem    chan struct{}
}

// NewService crée un service avec les paramètres appliqués aux nouveaux fichiers.
func NewService(p Params, maxConcurrent int) (*Service, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{params: p, sem: make(chan struct{}, maxConcurrent)}, nil
}

type slot struct {
	nonce [nonceLen]byte
	ct    [wrapLen]byte
}

type file struct {
	params  Params
	salt    [saltLen]byte
	pw, rec slot
	nonce   [nonceLen]byte
	content []byte
}

func (f *file) marshal() []byte {
	b := make([]byte, 0, headerLen+len(f.content))
	b = append(b, magic...)
	b = append(b, version)
	b = binary.BigEndian.AppendUint32(b, f.params.Time)
	b = binary.BigEndian.AppendUint32(b, f.params.MemoryKiB)
	b = append(b, f.params.Threads)
	b = append(b, f.salt[:]...)
	b = append(b, f.pw.nonce[:]...)
	b = append(b, f.pw.ct[:]...)
	b = append(b, f.rec.nonce[:]...)
	b = append(b, f.rec.ct[:]...)
	b = append(b, f.nonce[:]...)
	b = binary.BigEndian.AppendUint32(b, uint32(len(f.content)))
	return append(b, f.content...)
}

func parse(b []byte) (*file, error) {
	if len(b) < headerLen || string(b[:4]) != magic || b[4] != version {
		return nil, ErrFormat
	}
	var f file
	o := 5
	f.params.Time = binary.BigEndian.Uint32(b[o:])
	o += 4
	f.params.MemoryKiB = binary.BigEndian.Uint32(b[o:])
	o += 4
	f.params.Threads = b[o]
	o++
	if err := f.params.validate(); err != nil {
		return nil, err
	}
	o += copy(f.salt[:], b[o:])
	o += copy(f.pw.nonce[:], b[o:])
	o += copy(f.pw.ct[:], b[o:])
	o += copy(f.rec.nonce[:], b[o:])
	o += copy(f.rec.ct[:], b[o:])
	o += copy(f.nonce[:], b[o:])
	n := binary.BigEndian.Uint32(b[o:])
	o += 4
	if n > maxContent || int(n) != len(b)-o {
		return nil, ErrFormat
	}
	f.content = append([]byte(nil), b[o:]...)
	return &f, nil
}

func contentAAD(id string) []byte { return []byte("wmcf-content/v1/" + id) }

func pwAAD(id string, p Params, salt [saltLen]byte) []byte {
	b := []byte("wmcf-slot-pw/v1/" + id)
	b = binary.BigEndian.AppendUint32(b, p.Time)
	b = binary.BigEndian.AppendUint32(b, p.MemoryKiB)
	b = append(b, p.Threads)
	return append(b, salt[:]...)
}

func recAAD(id string) []byte { return []byte("wmcf-slot-rec/v1/" + id) }

func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

func seal(key, aad, plain []byte) (nonce [nonceLen]byte, ct []byte, err error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nonce, nil, err
	}
	if _, err = io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nonce, nil, err
	}
	return nonce, aead.Seal(nil, nonce[:], plain, aad), nil
}

func open(key []byte, nonce [nonceLen]byte, aad, ct []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce[:], ct, aad)
}

func wrap(kek, aad []byte, dek DEK) (slot, error) {
	var s slot
	n, ct, err := seal(kek, aad, dek[:])
	if err != nil {
		return s, err
	}
	s.nonce = n
	copy(s.ct[:], ct)
	return s, nil
}

func unwrap(kek, aad []byte, s slot) (DEK, error) {
	var d DEK
	plain, err := open(kek, s.nonce, aad, s.ct[:])
	if err != nil || len(plain) != keyLen {
		return d, ErrInvalid
	}
	copy(d[:], plain)
	return d, nil
}

func (s *Service) derive(password string, salt []byte, p Params) []byte {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()
	return argon2.IDKey([]byte(norm.NFC.String(password)), salt, p.Time, p.MemoryKiB, p.Threads, keyLen)
}

// DummyDerive fait un calcul argon2id inutile : à appeler quand le login n'existe pas,
// pour que la durée de réponse ne révèle pas l'existence du compte.
func (s *Service) DummyDerive(password string) {
	salt, err := randBytes(saltLen)
	if err != nil {
		return
	}
	s.derive(password, salt, s.params)
}

func recoveryKEK(raw [keyLen]byte) []byte {
	r := hkdf.New(sha256.New, raw[:], nil, []byte("wmail-go recovery v1"))
	k := make([]byte, keyLen)
	_, _ = io.ReadFull(r, k)
	return k
}

// Create chiffre plain pour l'utilisateur d'identifiant id. Il renvoie le fichier,
// la DEK et la clé de récupération (à afficher une seule fois).
func (s *Service) Create(id, password string, plain []byte) (data []byte, dek DEK, recovery string, err error) {
	if _, err = io.ReadFull(rand.Reader, dek[:]); err != nil {
		return nil, dek, "", err
	}
	f := &file{params: s.params}
	if _, err = io.ReadFull(rand.Reader, f.salt[:]); err != nil {
		return nil, dek, "", err
	}
	kek := s.derive(password, f.salt[:], f.params)
	if f.pw, err = wrap(kek, pwAAD(id, f.params, f.salt), dek); err != nil {
		return nil, dek, "", err
	}
	raw, display, err := newRecoveryKey()
	if err != nil {
		return nil, dek, "", err
	}
	if f.rec, err = wrap(recoveryKEK(raw), recAAD(id), dek); err != nil {
		return nil, dek, "", err
	}
	var ct []byte
	if f.nonce, ct, err = seal(dek[:], contentAAD(id), plain); err != nil {
		return nil, dek, "", err
	}
	f.content = ct
	return f.marshal(), dek, display, nil
}

func openContent(id string, f *file, dek DEK) ([]byte, error) {
	plain, err := open(dek[:], f.nonce, contentAAD(id), f.content)
	if err != nil {
		return nil, ErrFormat
	}
	return plain, nil
}

// OpenPassword déchiffre le fichier avec le mot de passe.
func (s *Service) OpenPassword(id string, data []byte, password string) (DEK, []byte, error) {
	var zero DEK
	f, err := parse(data)
	if err != nil {
		return zero, nil, err
	}
	kek := s.derive(password, f.salt[:], f.params)
	dek, err := unwrap(kek, pwAAD(id, f.params, f.salt), f.pw)
	if err != nil {
		return zero, nil, err
	}
	plain, err := openContent(id, f, dek)
	if err != nil {
		dek.Wipe()
		return zero, nil, err
	}
	return dek, plain, nil
}

// OpenRecovery déchiffre le fichier avec la clé de récupération.
func (s *Service) OpenRecovery(id string, data []byte, recovery string) (DEK, []byte, error) {
	var zero DEK
	raw, err := ParseRecoveryKey(recovery)
	if err != nil {
		return zero, nil, ErrInvalid
	}
	f, err := parse(data)
	if err != nil {
		return zero, nil, err
	}
	dek, err := unwrap(recoveryKEK(raw), recAAD(id), f.rec)
	if err != nil {
		return zero, nil, err
	}
	plain, err := openContent(id, f, dek)
	if err != nil {
		dek.Wipe()
		return zero, nil, err
	}
	return dek, plain, nil
}

// SetPassword remplace le slot mot de passe (nouveau sel, paramètres courants).
// Le contenu n'est pas rechiffré.
func (s *Service) SetPassword(id string, data []byte, dek DEK, newPassword string) ([]byte, error) {
	f, err := parse(data)
	if err != nil {
		return nil, err
	}
	if _, err := openContent(id, f, dek); err != nil {
		return nil, err
	}
	f.params = s.params
	if _, err := io.ReadFull(rand.Reader, f.salt[:]); err != nil {
		return nil, err
	}
	kek := s.derive(newPassword, f.salt[:], f.params)
	if f.pw, err = wrap(kek, pwAAD(id, f.params, f.salt), dek); err != nil {
		return nil, err
	}
	return f.marshal(), nil
}

// NewRecovery remplace le slot de récupération et renvoie la nouvelle clé.
func (s *Service) NewRecovery(id string, data []byte, dek DEK) ([]byte, string, error) {
	f, err := parse(data)
	if err != nil {
		return nil, "", err
	}
	if _, err := openContent(id, f, dek); err != nil {
		return nil, "", err
	}
	raw, display, err := newRecoveryKey()
	if err != nil {
		return nil, "", err
	}
	if f.rec, err = wrap(recoveryKEK(raw), recAAD(id), dek); err != nil {
		return nil, "", err
	}
	return f.marshal(), display, nil
}

// Reseal remplace le contenu chiffré en gardant les slots.
func (s *Service) Reseal(id string, data []byte, dek DEK, plain []byte) ([]byte, error) {
	if len(plain) > maxContent {
		return nil, fmt.Errorf("vault : contenu trop grand (%d octets)", len(plain))
	}
	f, err := parse(data)
	if err != nil {
		return nil, err
	}
	if _, err := openContent(id, f, dek); err != nil {
		return nil, err
	}
	var ct []byte
	if f.nonce, ct, err = seal(dek[:], contentAAD(id), plain); err != nil {
		return nil, err
	}
	f.content = ct
	return f.marshal(), nil
}

var recoveryEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

func recoveryChecksum(raw [keyLen]byte) []byte {
	sum := sha256.Sum256(append([]byte("wmail-go recovery checksum"), raw[:]...))
	return sum[:2]
}

func newRecoveryKey() ([keyLen]byte, string, error) {
	var raw [keyLen]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return raw, "", err
	}
	return raw, FormatRecoveryKey(raw), nil
}

// FormatRecoveryKey affiche la clé en base32 par groupes de 5, avec somme de contrôle.
func FormatRecoveryKey(raw [keyLen]byte) string {
	enc := recoveryEnc.EncodeToString(append(raw[:], recoveryChecksum(raw)...))
	var sb strings.Builder
	for i := 0; i < len(enc); i += 5 {
		if i > 0 {
			sb.WriteByte('-')
		}
		sb.WriteString(enc[i:min(i+5, len(enc))])
	}
	return sb.String()
}

// ParseRecoveryKey lit une clé saisie par l'utilisateur (tirets, espaces et casse ignorés).
func ParseRecoveryKey(s string) ([keyLen]byte, error) {
	var raw [keyLen]byte
	s = strings.ToUpper(strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(s))
	b, err := recoveryEnc.DecodeString(s)
	if err != nil || len(b) != keyLen+2 {
		return raw, ErrInvalid
	}
	copy(raw[:], b[:keyLen])
	if string(b[keyLen:]) != string(recoveryChecksum(raw)) {
		return raw, ErrInvalid
	}
	return raw, nil
}

// --- données annexes (carnet d'adresses…) chiffrées avec une sous-clé de la clé de données ---

const (
	blobMagic = "WMBL"
	maxBlob   = 64 << 20
)

// subkey dérive une clé propre à un usage : compromettre le fichier d'un usage ne révèle pas la clé des autres.
func (d DEK) subkey(info string) []byte {
	k := make([]byte, keyLen)
	r := hkdf.New(sha256.New, d[:], nil, []byte("wmail-go blob/v1/"+info))
	_, _ = io.ReadFull(r, k)
	return k
}

func blobAAD(info, id string) []byte { return []byte("wmbl/v1/" + info + "/" + id) }

// SealBlob chiffre des données annexes d'un compte. info désigne l'usage (« contacts »), id le compte (liaison :
// le fichier d'un compte ne s'ouvre pas sous l'identifiant d'un autre).
func SealBlob(dek DEK, info, id string, plain []byte) ([]byte, error) {
	if len(plain) > maxBlob {
		return nil, errors.New("vault : données trop volumineuses")
	}
	nonce, ct, err := seal(dek.subkey(info), blobAAD(info, id), plain)
	if err != nil {
		return nil, err
	}
	out := append([]byte(blobMagic), 1)
	out = append(out, nonce[:]...)
	return append(out, ct...), nil
}

// OpenBlob déchiffre des données produites par SealBlob.
func OpenBlob(dek DEK, info, id string, data []byte) ([]byte, error) {
	if len(data) < len(blobMagic)+1+nonceLen+chacha20poly1305.Overhead || string(data[:len(blobMagic)]) != blobMagic || data[len(blobMagic)] != 1 {
		return nil, ErrFormat
	}
	var nonce [nonceLen]byte
	copy(nonce[:], data[len(blobMagic)+1:])
	plain, err := open(dek.subkey(info), nonce, blobAAD(info, id), data[len(blobMagic)+1+nonceLen:])
	if err != nil {
		return nil, ErrFormat
	}
	return plain, nil
}
