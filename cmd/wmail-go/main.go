// wmail-go – webmail en Go.
// Licence : GNU AGPLv3 (voir LICENSE).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdegoulet/wmail-go/internal/config"
	"github.com/gdegoulet/wmail-go/internal/store"
	"github.com/gdegoulet/wmail-go/internal/vault"
	"github.com/gdegoulet/wmail-go/internal/web"
)

const version = "0.0.2-dev"

const usage = `Usage : wmail-go <commande> [options]

Commandes :
  serve                      démarre le serveur HTTP
  healthcheck                interroge /healthz (pour HEALTHCHECK)
  user add <login>           crée un compte (affiche la clé de récupération)
  user list                  liste les comptes
  user passwd <login>        change le mot de passe
  user recover <login>       récupère un compte avec sa clé de récupération
  user delete <login>        supprime un compte
  version                    affiche la version

Options communes : -config <fichier> (défaut : $WMAIL_CONFIG ou config.json)
Options user add/passwd/recover : --password-stdin (lit les mots de passe sur l'entrée standard)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errw io.Writer) int {
	if len(args) == 0 {
		args = []string{"serve"}
	}
	var err error
	switch args[0] {
	case "serve":
		err = cmdServe(args[1:])
	case "healthcheck":
		err = cmdHealthcheck(args[1:])
	case "user":
		err = cmdUser(args[1:], in, out, errw)
	case "version", "-version", "--version":
		fmt.Fprintln(out, version)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(out, usage)
	default:
		fmt.Fprintf(errw, "commande inconnue : %s\n\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(errw, "erreur :", err)
		return 1
	}
	return 0
}

func defaultConfigPath() string {
	if p := os.Getenv("WMAIL_CONFIG"); p != "" {
		return p
	}
	return "config.json"
}

// newFlags crée un jeu d'options avec -config.
func newFlags(name string, errw io.Writer) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errw)
	return fs, fs.String("config", defaultConfigPath(), "fichier de configuration")
}

func newLogger(cfg config.Config) *slog.Logger {
	level := map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError}[cfg.LogLevel]
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

// openStore ouvre le stockage avec les paramètres de config.json.
func openStore(cfg config.Config) (*store.Store, error) {
	svc, err := vault.NewService(vault.Params{
		Time:      cfg.KDF.Iterations,
		MemoryKiB: cfg.KDF.MemoryMiB * 1024,
		Threads:   cfg.KDF.Parallelism,
	}, cfg.MaxConcurrentKDF)
	if err != nil {
		return nil, err
	}
	return store.Open(cfg.DataDir, svc, store.Options{MaxUsers: cfg.MaxUsers, PasswordMinLength: cfg.PasswordMinLength})
}

func cmdServe(args []string) error {
	fs, cfgPath := newFlags("serve", os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	if cfg.Timezone != "" {
		if loc, err := time.LoadLocation(cfg.Timezone); err == nil {
			time.Local = loc // dates du courrier, des invitations et des journaux dans le fuseau choisi
		}
	}
	st, err := openStore(cfg)
	if err != nil {
		return err
	}
	web.Version = version // affichée dans les réglages, à côté du lien vers le code source
	srv, err := web.New(cfg, st, logger)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Lecture et écriture : 5 minutes. Les opérations longues (téléversement d'une pièce jointe,
		// envoi SMTP jusqu'à 90 s, recherche sur une grosse boîte) ont leur propre délai, plus court ;
		// ces bornes-ci ne servent qu'à libérer une connexion abandonnée.
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv.Start(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	logger.Info("démarrage", "version", version, "listen", cfg.Listen, "data_dir", cfg.DataDir,
		"allow_registration", cfg.AllowRegistration, "trusted_proxy", cfg.TrustedProxy)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("arrêt")
	return nil
}

// cmdHealthcheck interroge /healthz : l'image finale n'a ni shell ni curl.
func cmdHealthcheck(args []string) error {
	fs, cfgPath := newFlags("healthcheck", os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	addr := cfg.Listen
	if addr != "" && addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz : statut %d", resp.StatusCode)
	}
	return nil
}
