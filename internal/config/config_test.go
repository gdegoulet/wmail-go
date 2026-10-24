package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultIsValid(t *testing.T) {
	d := Default()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if !d.AllowRegistration {
		t.Error("allow_registration doit valoir true par défaut")
	}
}

func TestLoadOverridesAndKeepsDefaults(t *testing.T) {
	cfg, err := Load(write(t, `{"allow_registration": false, "listen": ":9000", "rate_limit": {"login": {"per_ip": "3/1m"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowRegistration || cfg.Listen != ":9000" {
		t.Fatalf("valeurs non appliquées : %+v", cfg)
	}
	if cfg.PasswordMinLength != 12 {
		t.Error("valeur par défaut perdue")
	}
	if got := cfg.RateLimit.Login.PerIP; got.Count != 3 || got.Window != time.Minute {
		t.Errorf("limite lue : %+v", got)
	}
}

func TestLoadRejectsUnknownKeyAndBadValues(t *testing.T) {
	cases := map[string]string{
		"clé inconnue":            `{"allow_registraton": false}`,
		"mot de passe trop court": `{"password_min_length": 4}`,
		"limite mal formée":       `{"rate_limit": {"login": {"per_ip": "beaucoup"}}}`,
		"langue inconnue":         `{"default_language": "de"}`,
		"bornes de session":       `{"session": {"idle_minutes_default": 100, "idle_minutes_max": 50, "absolute_hours_default": 1, "absolute_hours_max": 1}}`,
		"mémoire argon2":          `{"kdf": {"memory_mib": 1, "iterations": 3, "parallelism": 4}}`,
		"JSON invalide":           `{`,
	}
	for name, content := range cases {
		if _, err := Load(write(t, content)); err == nil {
			t.Errorf("%s : accepté à tort", name)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("fichier absent accepté")
	}
}

func TestParseLimit(t *testing.T) {
	l, err := ParseLimit("10/15m")
	if err != nil || l.Count != 10 || l.Window != 15*time.Minute || !l.Enabled() {
		t.Fatalf("%+v %v", l, err)
	}
	if l, err := ParseLimit(""); err != nil || l.Enabled() {
		t.Fatalf("limite vide : %+v %v", l, err)
	}
	for _, bad := range []string{"10", "x/1m", "-1/1m", "5/0s", "5/abc"} {
		if _, err := ParseLimit(bad); err == nil {
			t.Errorf("%q accepté", bad)
		}
	}
	if s := (Limit{Count: 5, Window: time.Hour}).String(); !strings.HasPrefix(s, "5/") {
		t.Error(s)
	}
}

func TestSessionDurationsAreClamped(t *testing.T) {
	c := Default()
	if c.SessionIdle(0) != 60*time.Minute || c.SessionAbsolute(0) != 12*time.Hour {
		t.Error("valeurs par défaut")
	}
	if c.SessionIdle(30) != 30*time.Minute || c.SessionAbsolute(2) != 2*time.Hour {
		t.Error("valeurs de l'utilisateur")
	}
	if c.SessionIdle(100000) != 480*time.Minute || c.SessionAbsolute(100000) != 72*time.Hour {
		t.Error("les bornes de config.json doivent s'appliquer")
	}
}

func TestShippedConfigIsValid(t *testing.T) {
	if _, err := Load("../../config/config.json"); err != nil {
		t.Fatal(err)
	}
}

func TestComposeLimitsValidated(t *testing.T) {
	for _, bad := range []string{
		`{"compose": {"max_attachment_mb": 0}}`,
		`{"compose": {"max_attachment_mb": 50, "max_message_mb": 10}}`,
		`{"compose": {"max_message_mb": 500, "max_session_mb": 100}}`,
		`{"compose": {"max_attachments": 0}}`,
	} {
		if _, err := Load(write(t, bad)); err == nil {
			t.Errorf("%s devrait être refusé", bad)
		}
	}
	cfg, err := Load(write(t, `{"compose": {"max_attachment_mb": 5, "max_message_mb": 10, "max_session_mb": 20}}`))
	if err != nil || cfg.Compose.MaxAttachments != 30 || cfg.Compose.MaxAttachmentMB != 5 {
		t.Errorf("limites : %+v %v", cfg.Compose, err)
	}
}

func TestTimeoutsDefaultsAndBounds(t *testing.T) {
	c := Default()
	if c.Timeouts.Mail != 30 || c.Timeouts.Send != 90 || c.Timeouts.Dial != 20 {
		t.Errorf("délais par défaut : %+v", c.Timeouts)
	}
	for _, bad := range []Timeouts{{Mail: 1, Send: 90, Dial: 20}, {Mail: 30, Send: 5, Dial: 20}, {Mail: 30, Send: 90, Dial: 0}, {Mail: 1000, Send: 90, Dial: 20}} {
		c := Default()
		c.Timeouts = bad
		if err := c.Validate(); err == nil {
			t.Errorf("délais invalides acceptés : %+v", bad)
		}
	}
}
