// Package config lit et valide config.json (configuration globale).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Limit est une limite « nombre/durée », par exemple "10/15m".
// La valeur zéro désactive la limite.
type Limit struct {
	Count  int
	Window time.Duration
}

// ParseLimit lit "nombre/durée". Une chaîne vide donne une limite désactivée.
func ParseLimit(s string) (Limit, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Limit{}, nil
	}
	n, w, ok := strings.Cut(s, "/")
	if !ok {
		return Limit{}, fmt.Errorf("limite %q : format attendu nombre/durée (ex. 10/15m)", s)
	}
	count, err := strconv.Atoi(strings.TrimSpace(n))
	if err != nil || count < 0 {
		return Limit{}, fmt.Errorf("limite %q : nombre invalide", s)
	}
	d, err := time.ParseDuration(strings.TrimSpace(w))
	if err != nil || d <= 0 {
		return Limit{}, fmt.Errorf("limite %q : durée invalide", s)
	}
	return Limit{Count: count, Window: d}, nil
}

// Enabled indique si la limite s'applique.
func (l Limit) Enabled() bool { return l.Count > 0 && l.Window > 0 }

func (l Limit) String() string {
	if !l.Enabled() {
		return ""
	}
	return fmt.Sprintf("%d/%s", l.Count, l.Window)
}

// UnmarshalJSON accepte une chaîne "nombre/durée".
func (l *Limit) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("une limite doit être une chaîne « nombre/durée » : %w", err)
	}
	v, err := ParseLimit(s)
	if err != nil {
		return err
	}
	*l = v
	return nil
}

// RateRule regroupe les limites par IP et par login pour une action.
type RateRule struct {
	PerIP    Limit `json:"per_ip"`
	PerLogin Limit `json:"per_login"`
}

// RateLimits regroupe les règles par action.
type RateLimits struct {
	Login    RateRule `json:"login"`
	Recovery RateRule `json:"recovery"`
	Register RateRule `json:"register"`
	MailTest RateRule `json:"mail_test"` // bouton « Test » des comptes mail (connexions sortantes)
	Send     RateRule `json:"send"`      // envoi de messages (par IP et par compte)
	Sieve    RateRule `json:"sieve"`     // pages et actions des filtres (une connexion ManageSieve chacune)
}

// Compose fixe les limites de la composition de messages.
type Compose struct {
	MaxAttachmentMB int  `json:"max_attachment_mb"` // taille maximale d'une pièce jointe
	MaxMessageMB    int  `json:"max_message_mb"`    // taille maximale de l'ensemble des pièces jointes d'un message
	MaxAttachments  int  `json:"max_attachments"`   // nombre maximal de pièces jointes par message
	MaxSessionMB    int  `json:"max_session_mb"`    // mémoire maximale des pièces jointes en attente, par session
	HideXMailer     bool `json:"hide_x_mailer"`     // n'ajoute pas l'en-tête X-Mailer
	XOriginatingIP  bool `json:"x_originating_ip"`  // ajoute l'en-tête X-Originating-IP (adresse IP du poste qui écrit, comme le plugin RainLoop du même nom)
}

// Session fixe les durées par défaut et les bornes des sessions.
type Session struct {
	IdleMinutesDefault   int `json:"idle_minutes_default"`
	IdleMinutesMax       int `json:"idle_minutes_max"`
	AbsoluteHoursDefault int `json:"absolute_hours_default"`
	AbsoluteHoursMax     int `json:"absolute_hours_max"`
}

// KDF fixe les paramètres argon2id appliqués aux nouveaux fichiers.
type KDF struct {
	MemoryMiB   uint32 `json:"memory_mib"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
}

// Timeouts fixe les délais des opérations distantes (en secondes).
type Timeouts struct {
	Mail int `json:"mail_seconds"` // liste, lecture, recherche, actions IMAP
	Send int `json:"send_seconds"` // envoi d'un message par SMTP
	Dial int `json:"dial_seconds"` // établissement d'une connexion (TCP et TLS)
}

// Config est le contenu de config.json.
type Config struct {
	AllowRegistration    bool       `json:"allow_registration"`
	Listen               string     `json:"listen"`
	DataDir              string     `json:"data_dir"`
	TrustedProxy         bool       `json:"trusted_proxy"`
	MaxUsers             int        `json:"max_users"`
	BlockPrivateNetworks bool       `json:"block_private_networks"`
	AllowedPrivateHosts  []string   `json:"allowed_private_hosts"`
	MaxConcurrentKDF     int        `json:"max_concurrent_kdf"`
	PasswordMinLength    int        `json:"password_min_length"`
	Session              Session    `json:"session"`
	RateLimit            RateLimits `json:"rate_limit"`
	Compose              Compose    `json:"compose"`
	KDF                  KDF        `json:"kdf"`
	Timeouts             Timeouts   `json:"timeouts"`
	DefaultLanguage      string     `json:"default_language"`
	DefaultTheme         string     `json:"default_theme"`
	Timezone             string     `json:"timezone"` // fuseau d'affichage des dates (ex. Europe/Paris) ; vide : celui du système
	LogLevel             string     `json:"log_level"`
	LogFormat            string     `json:"log_format"`
}

func mustLimit(s string) Limit {
	l, err := ParseLimit(s)
	if err != nil {
		panic(err)
	}
	return l
}

// Default renvoie la configuration par défaut (voir le cahier des charges, 4.1 et 4.9).
func Default() Config {
	return Config{
		AllowRegistration:    true,
		Listen:               "127.0.0.1:8080",
		DataDir:              "./data",
		TrustedProxy:         false,
		MaxUsers:             0,
		BlockPrivateNetworks: true,
		AllowedPrivateHosts:  []string{},
		MaxConcurrentKDF:     4,
		PasswordMinLength:    12,
		Session: Session{
			IdleMinutesDefault:   60,
			IdleMinutesMax:       480,
			AbsoluteHoursDefault: 12,
			AbsoluteHoursMax:     72,
		},
		RateLimit: RateLimits{
			Login:    RateRule{PerIP: mustLimit("10/15m"), PerLogin: mustLimit("5/15m")},
			Recovery: RateRule{PerIP: mustLimit("5/1h"), PerLogin: mustLimit("3/1h")},
			Register: RateRule{PerIP: mustLimit("5/1h")},
			MailTest: RateRule{PerIP: mustLimit("30/10m"), PerLogin: mustLimit("20/10m")},
			Send:     RateRule{PerIP: mustLimit("60/1h"), PerLogin: mustLimit("30/1h")},
			Sieve:    RateRule{PerIP: mustLimit("200/10m"), PerLogin: mustLimit("100/10m")},
		},
		Compose:         Compose{MaxAttachmentMB: 25, MaxMessageMB: 30, MaxAttachments: 30, MaxSessionMB: 100},
		KDF:             KDF{MemoryMiB: 64, Iterations: 3, Parallelism: 4},
		Timeouts:        Timeouts{Mail: 30, Send: 90, Dial: 20},
		DefaultLanguage: "fr",
		DefaultTheme:    "rainloop",
		Timezone:        "Europe/Paris",
		LogLevel:        "info",
		LogFormat:       "text",
	}
}

// Load lit config.json. Les clés absentes prennent la valeur par défaut ;
// une clé inconnue est une erreur (elle protège des fautes de frappe sur des options de sécurité).
func Load(path string) (Config, error) {
	cfg := Default()
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("config %s : %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("config %s : %w", path, err)
	}
	return cfg, nil
}

// Validate vérifie la cohérence de la configuration.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if strings.TrimSpace(c.Listen) == "" {
		add("listen est vide")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		add("data_dir est vide")
	}
	if c.MaxUsers < 0 {
		add("max_users doit être >= 0")
	}
	if c.MaxConcurrentKDF < 1 {
		add("max_concurrent_kdf doit être >= 1")
	}
	if c.PasswordMinLength < 8 || c.PasswordMinLength > 128 {
		add("password_min_length doit être compris entre 8 et 128")
	}
	s := c.Session
	if s.IdleMinutesDefault < 1 || s.IdleMinutesMax < s.IdleMinutesDefault {
		add("session : idle_minutes_default >= 1 et idle_minutes_max >= idle_minutes_default")
	}
	if s.AbsoluteHoursDefault < 1 || s.AbsoluteHoursMax < s.AbsoluteHoursDefault {
		add("session : absolute_hours_default >= 1 et absolute_hours_max >= absolute_hours_default")
	}
	if c.KDF.MemoryMiB < 8 || c.KDF.MemoryMiB > 1024 {
		add("kdf.memory_mib doit être compris entre 8 et 1024")
	}
	if c.KDF.Iterations < 1 || c.KDF.Iterations > 20 {
		add("kdf.iterations doit être compris entre 1 et 20")
	}
	if c.KDF.Parallelism < 1 || c.KDF.Parallelism > 64 {
		add("kdf.parallelism doit être compris entre 1 et 64")
	}
	if c.Compose.MaxAttachmentMB < 1 || c.Compose.MaxAttachmentMB > 512 {
		add("compose.max_attachment_mb doit être compris entre 1 et 512")
	}
	if c.Compose.MaxMessageMB < c.Compose.MaxAttachmentMB || c.Compose.MaxMessageMB > 1024 {
		add("compose.max_message_mb doit être au moins compose.max_attachment_mb et au plus 1024")
	}
	if c.Compose.MaxAttachments < 1 || c.Compose.MaxAttachments > 500 {
		add("compose.max_attachments doit être compris entre 1 et 500")
	}
	if c.Compose.MaxSessionMB < c.Compose.MaxMessageMB || c.Compose.MaxSessionMB > 4096 {
		add("compose.max_session_mb doit être au moins compose.max_message_mb et au plus 4096")
	}
	if c.Timeouts.Mail < 5 || c.Timeouts.Mail > 600 {
		add("timeouts.mail_seconds doit être compris entre 5 et 600")
	}
	if c.Timeouts.Send < 10 || c.Timeouts.Send > 900 {
		add("timeouts.send_seconds doit être compris entre 10 et 900")
	}
	if c.Timeouts.Dial < 2 || c.Timeouts.Dial > 120 {
		add("timeouts.dial_seconds doit être compris entre 2 et 120")
	}
	if c.DefaultLanguage != "fr" && c.DefaultLanguage != "en" {
		add("default_language doit valoir fr ou en")
	}
	if strings.TrimSpace(c.DefaultTheme) == "" {
		add("default_theme est vide")
	}
	if c.Timezone != "" {
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			add("timezone inconnu : %s", c.Timezone)
		}
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		add("log_level doit valoir debug, info, warn ou error")
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		add("log_format doit valoir text ou json")
	}
	return errors.Join(errs...)
}

// SessionIdle renvoie la durée d'inactivité effective : valeur de l'utilisateur
// (0 = défaut), bornée par session.idle_minutes_max.
func (c *Config) SessionIdle(userMinutes int) time.Duration {
	m := userMinutes
	if m <= 0 {
		m = c.Session.IdleMinutesDefault
	}
	if m > c.Session.IdleMinutesMax {
		m = c.Session.IdleMinutesMax
	}
	return time.Duration(m) * time.Minute
}

// SessionAbsolute renvoie la durée maximale effective d'une session.
func (c *Config) SessionAbsolute(userHours int) time.Duration {
	h := userHours
	if h <= 0 {
		h = c.Session.AbsoluteHoursDefault
	}
	if h > c.Session.AbsoluteHoursMax {
		h = c.Session.AbsoluteHoursMax
	}
	return time.Duration(h) * time.Hour
}
