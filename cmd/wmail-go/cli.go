package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"wmail-go/internal/config"
	"wmail-go/internal/store"
)

// prompter lit les mots de passe : sans écho sur un terminal, ligne par ligne sinon.
type prompter struct {
	in        io.Reader
	errw      io.Writer
	lineMode  bool
	reader    *bufio.Reader
	interactV bool
}

func newPrompter(in io.Reader, errw io.Writer, forceLine bool) *prompter {
	p := &prompter{in: in, errw: errw, reader: bufio.NewReader(in)}
	if f, ok := in.(*os.File); ok && !forceLine && term.IsTerminal(int(f.Fd())) {
		p.interactV = true
	} else {
		p.lineMode = true
	}
	return p
}

func (p *prompter) readLine() (string, error) {
	line, err := p.reader.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("entrée standard : %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// secret demande une valeur secrète.
func (p *prompter) secret(label string) (string, error) {
	if p.interactV {
		fmt.Fprint(p.errw, label+" : ")
		b, err := term.ReadPassword(int(p.in.(*os.File).Fd()))
		fmt.Fprintln(p.errw)
		return string(b), err
	}
	return p.readLine()
}

// newSecret demande un nouveau mot de passe, avec confirmation en mode interactif.
func (p *prompter) newSecret(label string) (string, error) {
	pw, err := p.secret(label)
	if err != nil || !p.interactV {
		return pw, err
	}
	again, err := p.secret("Confirmer")
	if err != nil {
		return "", err
	}
	if pw != again {
		return "", errors.New("les deux mots de passe ne sont pas identiques")
	}
	return pw, nil
}

// parseMixed accepte les options avant ou après les arguments positionnels.
func parseMixed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func cmdUser(args []string, in io.Reader, out, errw io.Writer) error {
	if len(args) == 0 {
		return errors.New("sous-commande attendue : add, list, passwd, recover, delete")
	}
	sub, rest := args[0], args[1:]
	fs, cfgPath := newFlags("user "+sub, errw)
	stdin := fs.Bool("password-stdin", false, "lire les mots de passe sur l'entrée standard")
	yes := fs.Bool("yes", false, "ne pas demander de confirmation (delete)")
	pos, err := parseMixed(fs, rest)
	if err != nil {
		return err
	}
	needLogin := sub != "list"
	if needLogin && len(pos) != 1 {
		return fmt.Errorf("user %s : un seul login attendu", sub)
	}
	if !needLogin && len(pos) != 0 {
		return fmt.Errorf("user %s : aucun argument attendu", sub)
	}
	switch sub {
	case "add", "list", "passwd", "recover", "delete":
	default:
		return fmt.Errorf("sous-commande inconnue : user %s", sub)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	st, err := openStore(cfg)
	if err != nil {
		return err
	}
	pr := newPrompter(in, errw, *stdin)

	switch sub {
	case "list":
		logins, err := st.List()
		if err != nil {
			return err
		}
		for _, l := range logins {
			fmt.Fprintln(out, l)
		}
		fmt.Fprintf(errw, "%d utilisateur(s)\n", len(logins))
		return nil

	case "add":
		pw, err := pr.newSecret("Mot de passe")
		if err != nil {
			return err
		}
		acc, key, err := st.Create(pos[0], pw, true)
		if err != nil {
			return userError(err, cfg)
		}
		acc.Wipe()
		fmt.Fprintf(out, "Compte créé : %s\n\nClé de récupération (affichée une seule fois) :\n%s\n\n", acc.Login, key)
		fmt.Fprintln(errw, "Notez cette clé en lieu sûr : sans le mot de passe ni cette clé, les données sont définitivement perdues.")
		return nil

	case "passwd":
		old, err := pr.secret("Mot de passe actuel")
		if err != nil {
			return err
		}
		pw, err := pr.newSecret("Nouveau mot de passe")
		if err != nil {
			return err
		}
		if err := st.ChangePassword(pos[0], old, pw); err != nil {
			return userError(err, cfg)
		}
		fmt.Fprintln(out, "Mot de passe modifié.")
		fmt.Fprintln(errw, "Les sessions déjà ouvertes dans le serveur restent valides jusqu'à leur expiration.")
		return nil

	case "recover":
		key, err := pr.secret("Clé de récupération")
		if err != nil {
			return err
		}
		pw, err := pr.newSecret("Nouveau mot de passe")
		if err != nil {
			return err
		}
		acc, newKey, err := st.Recover(pos[0], key, pw)
		if err != nil {
			return userError(err, cfg)
		}
		acc.Wipe()
		fmt.Fprintf(out, "Compte récupéré : %s\n\nNouvelle clé de récupération (l'ancienne ne fonctionne plus) :\n%s\n", acc.Login, newKey)
		return nil

	case "delete":
		if !*yes {
			fmt.Fprintf(errw, "Supprimer %q et toutes ses données ? Tapez « oui » pour confirmer : ", pos[0])
			ans, err := pr.readLine()
			if err != nil {
				return err
			}
			if a := strings.ToLower(strings.TrimSpace(ans)); a != "oui" && a != "yes" {
				return errors.New("suppression annulée")
			}
		}
		if err := st.Delete(pos[0]); err != nil {
			return userError(err, cfg)
		}
		fmt.Fprintln(out, "Compte supprimé.")
		fmt.Fprintln(errw, "Les sessions déjà ouvertes dans le serveur restent valides jusqu'à leur expiration.")
		return nil
	}
	return nil
}

// userError traduit les erreurs du stockage en messages clairs.
func userError(err error, cfg config.Config) error {
	switch {
	case errors.Is(err, store.ErrInvalidLogin):
		return errors.New("login invalide : 3 à 64 caractères, sans espace")
	case errors.Is(err, store.ErrPasswordTooShort):
		return fmt.Errorf("mot de passe trop court : %d caractères minimum", cfg.PasswordMinLength)
	case errors.Is(err, store.ErrPasswordTooLong):
		return errors.New("mot de passe trop long")
	case errors.Is(err, store.ErrExists):
		return errors.New("ce login existe déjà")
	case errors.Is(err, store.ErrNotFound):
		return errors.New("utilisateur introuvable")
	case errors.Is(err, store.ErrInvalidCredentials):
		return errors.New("identifiants invalides")
	}
	return err
}
