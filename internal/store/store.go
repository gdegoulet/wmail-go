// Package store gère les fichiers utilisateur chiffrés sur disque :
// arborescence hashée, clé serveur, index chiffré des logins, verrouillage et écriture atomique.
package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/text/unicode/norm"

	"wmail-go/internal/userdata"
	"wmail-go/internal/vault"
)

// Erreurs du stockage. ErrInvalidCredentials est volontairement indistincte : à l'ouverture d'un compte, un login
// inconnu, un fichier absent et un mauvais mot de passe donnent tous cette même erreur, pour ne pas révéler
// quels logins existent. Les autres ne surviennent qu'une fois l'appelant déjà authentifié ou en ligne de commande.
var (
	ErrInvalidCredentials = errors.New("identifiants invalides")
	ErrInvalidLogin       = errors.New("login invalide")
	ErrPasswordTooShort   = errors.New("mot de passe trop court")
	ErrPasswordTooLong    = errors.New("mot de passe trop long")
	ErrExists             = errors.New("login déjà utilisé")
	ErrMaxUsers           = errors.New("nombre maximal d'utilisateurs atteint")
	ErrNotFound           = errors.New("utilisateur introuvable")
	ErrIndexCorrupt       = errors.New("index des utilisateurs illisible")
)

const (
	minLoginRunes     = 3
	maxLoginRunes     = 64
	maxPasswordBytes  = 1024
	serverKeyFile     = "server.key"
	indexFile         = "users.idx"
	lockFile          = ".lock"
	indexMagic        = "WMIX"
	indexVersion      = 1
	indexInfo         = "wmail-go users.idx v1"
	usersDirName      = "users"
	userFileExtension = ".cfg"
)

// Options regroupe les réglages de config.json utilisés par le stockage.
type Options struct {
	MaxUsers          int // 0 = illimité
	PasswordMinLength int
}

// Store donne accès aux comptes webmail stockés dans data_dir.
type Store struct {
	dir    string
	key    []byte // server.key (32 octets)
	idxKey []byte
	vault  *vault.Service
	opts   Options
	mu     sync.Mutex
}

// Account est un compte ouvert : la DEK permet de réécrire le fichier.
type Account struct {
	Login string // login normalisé
	ID    string // identifiant hashé (nom de fichier)
	DEK   vault.DEK
	Data  *userdata.Data
}

// Wipe efface la clé de données.
func (a *Account) Wipe() { a.DEK.Wipe() }

// Open prépare le répertoire de données et charge (ou crée) server.key.
func Open(dataDir string, v *vault.Service, opts Options) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	key, err := loadOrCreateServerKey(filepath.Join(dataDir, serverKeyFile))
	if err != nil {
		return nil, err
	}
	r := hkdf.New(sha256.New, key, nil, []byte(indexInfo))
	idxKey := make([]byte, 32)
	if _, err := io.ReadFull(r, idxKey); err != nil {
		return nil, err
	}
	return &Store{dir: dataDir, key: key, idxKey: idxKey, vault: v, opts: opts}, nil
}

func loadOrCreateServerKey(path string) ([]byte, error) {
	for range 3 {
		b, err := os.ReadFile(path)
		if err == nil {
			if len(b) != 32 {
				return nil, fmt.Errorf("%s : taille %d, attendu 32 octets", path, len(b))
			}
			return b, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue // un autre processus l'a créée : relire
		}
		if err != nil {
			return nil, err
		}
		_, werr := f.Write(key)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			os.Remove(path)
			return nil, werr
		}
		return key, nil
	}
	return nil, fmt.Errorf("%s : création impossible", path)
}

// NormalizeLogin applique la normalisation : trim, NFC, minuscules.
func NormalizeLogin(login string) (string, error) {
	l := norm.NFC.String(strings.ToLower(norm.NFC.String(strings.TrimSpace(login))))
	if !utf8.ValidString(l) {
		return "", ErrInvalidLogin
	}
	n := utf8.RuneCountInString(l)
	if n < minLoginRunes || n > maxLoginRunes {
		return "", ErrInvalidLogin
	}
	for _, r := range l {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrInvalidLogin
		}
	}
	return l, nil
}

// ReduceLogin donne la forme réduite d'un login pour les journaux (deux premiers caractères + ***).
func ReduceLogin(login string) string {
	r := []rune(strings.TrimSpace(login))
	switch {
	case len(r) == 0:
		return "***"
	case len(r) <= 2:
		return string(r[:1]) + "***"
	default:
		return string(r[:2]) + "***"
	}
}

// ID calcule l'identifiant hashé d'un login déjà normalisé.
func (s *Store) ID(normalizedLogin string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(normalizedLogin))
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, usersDirName, id[0:2], id[2:4], id+userFileExtension)
}

func (s *Store) checkPassword(pw string) error {
	if len(pw) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	if utf8.RuneCountInString(norm.NFC.String(pw)) < s.opts.PasswordMinLength {
		return ErrPasswordTooShort
	}
	return nil
}

// withLock sérialise les écritures (entre processus : flock ; dans le processus : mutex).
func (s *Store) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.dir, lockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	return fn()
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return cleanup(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	if d, err := os.Open(dir); err == nil {
		d.Sync() //nolint:errcheck
		d.Close()
	}
	return nil
}

// --- index chiffré des logins ---

type indexContent struct {
	Logins []string `json:"logins"`
}

func (s *Store) readIndex() ([]string, error) {
	b, err := os.ReadFile(filepath.Join(s.dir, indexFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	hdr := len(indexMagic) + 1 + chacha20poly1305.NonceSizeX
	if len(b) < hdr+chacha20poly1305.Overhead || string(b[:4]) != indexMagic || b[4] != indexVersion {
		return nil, ErrIndexCorrupt
	}
	aead, err := chacha20poly1305.NewX(s.idxKey)
	if err != nil {
		return nil, err
	}
	plain, err := aead.Open(nil, b[5:hdr], b[hdr:], []byte(indexInfo))
	if err != nil {
		return nil, ErrIndexCorrupt
	}
	var c indexContent
	if err := json.Unmarshal(plain, &c); err != nil {
		return nil, ErrIndexCorrupt
	}
	return c.Logins, nil
}

func (s *Store) writeIndex(logins []string) error {
	slices.Sort(logins)
	plain, err := json.Marshal(indexContent{Logins: logins})
	if err != nil {
		return err
	}
	aead, err := chacha20poly1305.NewX(s.idxKey)
	if err != nil {
		return err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	out := append([]byte(indexMagic), indexVersion)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plain, []byte(indexInfo))
	return writeAtomic(filepath.Join(s.dir, indexFile), out)
}

// List renvoie les logins (normalisés) connus de l'index, triés.
func (s *Store) List() ([]string, error) {
	l, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	out := slices.Clone(l)
	slices.Sort(out)
	return out, nil
}

// Count renvoie le nombre d'utilisateurs.
func (s *Store) Count() (int, error) {
	l, err := s.readIndex()
	return len(l), err
}

// --- opérations sur les comptes ---

func decodeData(plain []byte) (*userdata.Data, error) {
	d := userdata.New()
	if err := json.Unmarshal(plain, d); err != nil {
		return nil, fmt.Errorf("contenu utilisateur illisible : %w", err)
	}
	if d.MailAccounts == nil {
		d.MailAccounts = []userdata.MailAccount{}
	}
	return d, nil
}

// Create crée un compte. ignoreMax contourne max_users (outil en ligne de commande).
// Elle renvoie la clé de récupération, à afficher une seule fois.
func (s *Store) Create(login, password string, ignoreMax bool) (*Account, string, error) {
	nl, err := NormalizeLogin(login)
	if err != nil {
		return nil, "", err
	}
	if err := s.checkPassword(password); err != nil {
		return nil, "", err
	}
	id := s.ID(nl)
	data := userdata.New()
	plain, err := json.Marshal(data)
	if err != nil {
		return nil, "", err
	}
	file, dek, recovery, err := s.vault.Create(id, password, plain)
	if err != nil {
		return nil, "", err
	}
	err = s.withLock(func() error {
		if _, err := os.Stat(s.path(id)); err == nil {
			return ErrExists
		}
		logins, err := s.readIndex()
		if err != nil {
			return err
		}
		if !ignoreMax && s.opts.MaxUsers > 0 && len(logins) >= s.opts.MaxUsers {
			return ErrMaxUsers
		}
		if err := writeAtomic(s.path(id), file); err != nil {
			return err
		}
		if !slices.Contains(logins, nl) {
			logins = append(logins, nl)
		}
		if err := s.writeIndex(logins); err != nil {
			os.Remove(s.path(id))
			return err
		}
		return nil
	})
	if err != nil {
		dek.Wipe()
		return nil, "", err
	}
	return &Account{Login: nl, ID: id, DEK: dek, Data: data}, recovery, nil
}

// Login ouvre un compte avec son mot de passe. Toute erreur d'identifiant donne ErrInvalidCredentials.
func (s *Store) Login(login, password string) (*Account, error) {
	nl, err := NormalizeLogin(login)
	if err != nil {
		s.vault.DummyDerive(password)
		return nil, ErrInvalidCredentials
	}
	id := s.ID(nl)
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		s.vault.DummyDerive(password)
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, ErrInvalidCredentials
	}
	dek, plain, err := s.vault.OpenPassword(id, b, password)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	data, err := decodeData(plain)
	if err != nil {
		dek.Wipe()
		return nil, err
	}
	return &Account{Login: nl, ID: id, DEK: dek, Data: data}, nil
}

// ChangePassword remplace le mot de passe (ancien mot de passe obligatoire).
func (s *Store) ChangePassword(login, oldPassword, newPassword string) error {
	if err := s.checkPassword(newPassword); err != nil {
		return err
	}
	acc, err := s.Login(login, oldPassword)
	if err != nil {
		return err
	}
	defer acc.Wipe()
	return s.withLock(func() error {
		cur, err := os.ReadFile(s.path(acc.ID))
		if err != nil {
			return ErrNotFound
		}
		out, err := s.vault.SetPassword(acc.ID, cur, acc.DEK, newPassword)
		if err != nil {
			return err
		}
		return writeAtomic(s.path(acc.ID), out)
	})
}

// Recover ouvre un compte avec la clé de récupération, définit un nouveau mot de passe
// et remplace la clé de récupération. Elle renvoie le compte ouvert et la nouvelle clé.
func (s *Store) Recover(login, recoveryKey, newPassword string) (*Account, string, error) {
	if err := s.checkPassword(newPassword); err != nil {
		return nil, "", err
	}
	nl, err := NormalizeLogin(login)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	id := s.ID(nl)
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	dek, plain, err := s.vault.OpenRecovery(id, b, recoveryKey)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	data, err := decodeData(plain)
	if err != nil {
		dek.Wipe()
		return nil, "", err
	}
	var newKey string
	err = s.withLock(func() error {
		cur, err := os.ReadFile(s.path(id))
		if err != nil {
			return ErrNotFound
		}
		out, err := s.vault.SetPassword(id, cur, dek, newPassword)
		if err != nil {
			return err
		}
		out, newKey, err = s.vault.NewRecovery(id, out, dek)
		if err != nil {
			return err
		}
		return writeAtomic(s.path(id), out)
	})
	if err != nil {
		dek.Wipe()
		return nil, "", err
	}
	return &Account{Login: nl, ID: id, DEK: dek, Data: data}, newKey, nil
}

// RegenerateRecovery remplace la clé de récupération (mot de passe obligatoire).
func (s *Store) RegenerateRecovery(login, password string) (string, error) {
	acc, err := s.Login(login, password)
	if err != nil {
		return "", err
	}
	defer acc.Wipe()
	var key string
	err = s.withLock(func() error {
		cur, err := os.ReadFile(s.path(acc.ID))
		if err != nil {
			return ErrNotFound
		}
		out, k, err := s.vault.NewRecovery(acc.ID, cur, acc.DEK)
		if err != nil {
			return err
		}
		key = k
		return writeAtomic(s.path(acc.ID), out)
	})
	return key, err
}

// SaveData réécrit le contenu du compte (les slots restent inchangés).
func (s *Store) SaveData(acc *Account) error {
	plain, err := json.Marshal(acc.Data)
	if err != nil {
		return err
	}
	return s.withLock(func() error {
		cur, err := os.ReadFile(s.path(acc.ID))
		if err != nil {
			return ErrNotFound
		}
		out, err := s.vault.Reseal(acc.ID, cur, acc.DEK, plain)
		if err != nil {
			return err
		}
		return writeAtomic(s.path(acc.ID), out)
	})
}

// Données annexes chiffrées d'un compte (carnet d'adresses, fond d'écran…) : un fichier par usage à côté du fichier du
// compte, chiffré avec une sous-clé de la clé de données et lié à l'identifiant du compte.
var blobExtensions = map[string]string{"contacts": ".ctc", "background": ".bgd"}

func (s *Store) blobPath(id, kind string) string {
	return strings.TrimSuffix(s.path(id), userFileExtension) + blobExtensions[kind]
}

// LoadBlob lit une donnée annexe ; nil si elle n'existe pas encore.
func (s *Store) LoadBlob(acc *Account, kind string) ([]byte, error) {
	if _, ok := blobExtensions[kind]; !ok {
		return nil, errors.New("store : usage inconnu")
	}
	data, err := os.ReadFile(s.blobPath(acc.ID, kind))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return vault.OpenBlob(acc.DEK, kind, acc.ID, data)
}

// SaveBlob réécrit une donnée annexe ; un contenu vide la supprime.
func (s *Store) SaveBlob(acc *Account, kind string, plain []byte) error {
	if _, ok := blobExtensions[kind]; !ok {
		return errors.New("store : usage inconnu")
	}
	var data []byte
	if len(plain) > 0 {
		var err error
		if data, err = vault.SealBlob(acc.DEK, kind, acc.ID, plain); err != nil {
			return err
		}
	}
	return s.withLock(func() error {
		if _, err := os.Stat(s.path(acc.ID)); err != nil {
			return ErrNotFound
		}
		if data == nil {
			if err := os.Remove(s.blobPath(acc.ID, kind)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			return nil
		}
		return writeAtomic(s.blobPath(acc.ID, kind), data)
	})
}

// LoadContacts lit le carnet d'adresses du compte ; nil s'il n'y en a pas encore.
func (s *Store) LoadContacts(acc *Account) ([]byte, error) { return s.LoadBlob(acc, "contacts") }

// SaveContacts réécrit le carnet d'adresses du compte ; un contenu vide le supprime.
func (s *Store) SaveContacts(acc *Account, plain []byte) error {
	return s.SaveBlob(acc, "contacts", plain)
}

// Delete supprime un compte et son entrée d'index.
func (s *Store) Delete(login string) error {
	nl, err := NormalizeLogin(login)
	if err != nil {
		return ErrNotFound
	}
	id := s.ID(nl)
	return s.withLock(func() error {
		if err := os.Remove(s.path(id)); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return ErrNotFound
			}
			return err
		}
		for kind := range blobExtensions {
			_ = os.Remove(s.blobPath(id, kind))
		}
		// Supprime les dossiers de hash devenus vides (sans erreur s'ils ne le sont pas).
		d2 := filepath.Dir(s.path(id))
		os.Remove(d2)
		os.Remove(filepath.Dir(d2))
		logins, err := s.readIndex()
		if err != nil {
			return err
		}
		logins = slices.DeleteFunc(logins, func(l string) bool { return l == nl })
		return s.writeIndex(logins)
	})
}
