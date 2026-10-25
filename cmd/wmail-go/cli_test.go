package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var cliKeyRe = regexp.MustCompile(`(?m)^([A-Z2-7]{5}(?:-[A-Z2-7]{1,5})+)$`)

func cliEnv(t *testing.T) (cfgPath string) {
	t.Helper()
	dir := t.TempDir()
	cfg := map[string]any{
		"data_dir": filepath.Join(dir, "data"),
		"kdf":      map[string]any{"memory_mib": 8, "iterations": 1, "parallelism": 1},
	}
	b, _ := json.Marshal(cfg)
	cfgPath = filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestUserLifecycle(t *testing.T) {
	cfg := cliEnv(t)
	const pw = "un mot de passe assez long"

	code, out, errs := runCLI(t, pw+"\n", "user", "add", "Alice", "--password-stdin", "-config", cfg)
	if code != 0 {
		t.Fatalf("user add : code %d, %s", code, errs)
	}
	key := cliKeyRe.FindString(out)
	if key == "" || !strings.Contains(out, "Compte créé : alice") {
		t.Fatalf("sortie de user add inattendue :\n%s", out)
	}

	if code, _, errs = runCLI(t, pw+"\n", "user", "add", "alice", "--password-stdin", "-config", cfg); code == 0 || !strings.Contains(errs, "existe déjà") {
		t.Errorf("doublon : code %d, %q", code, errs)
	}
	if code, _, errs = runCLI(t, "court\n", "user", "add", "bob", "--password-stdin", "-config", cfg); code == 0 || !strings.Contains(errs, "trop court") {
		t.Errorf("mot de passe court : code %d, %q", code, errs)
	}

	code, out, _ = runCLI(t, "", "user", "list", "-config", cfg)
	champs := strings.Split(strings.TrimSpace(out), "\t")
	if code != 0 || len(champs) != 2 || champs[0] != "alice" {
		t.Fatalf("user list : %d %q", code, out)
	}
	// La deuxième colonne est le fichier chiffré du compte : il doit exister.
	if _, err := os.Stat(champs[1]); err != nil {
		t.Fatalf("chemin annoncé par user list introuvable : %v", err)
	}

	newPW := "nouveau mot de passe long"
	if code, _, errs = runCLI(t, "faux mot de passe\n"+newPW+"\n", "user", "passwd", "alice", "--password-stdin", "-config", cfg); code == 0 {
		t.Errorf("passwd avec un mauvais mot de passe accepté : %q", errs)
	}
	if code, _, errs = runCLI(t, pw+"\n"+newPW+"\n", "user", "passwd", "alice", "--password-stdin", "-config", cfg); code != 0 {
		t.Fatalf("user passwd : %s", errs)
	}

	third := "troisieme mot de passe long"
	code, out, errs = runCLI(t, key+"\n"+third+"\n", "user", "recover", "alice", "--password-stdin", "-config", cfg)
	if code != 0 {
		t.Fatalf("user recover : %s", errs)
	}
	if newKey := cliKeyRe.FindString(out); newKey == "" || newKey == key {
		t.Fatalf("nouvelle clé attendue :\n%s", out)
	}
	if code, _, _ = runCLI(t, key+"\n"+third+"\n", "user", "recover", "alice", "--password-stdin", "-config", cfg); code == 0 {
		t.Error("l'ancienne clé ne doit plus fonctionner")
	}

	if code, _, _ = runCLI(t, "non\n", "user", "delete", "alice", "-config", cfg); code == 0 {
		t.Error("suppression sans confirmation acceptée")
	}
	if code, _, errs = runCLI(t, "oui\n", "user", "delete", "alice", "-config", cfg); code != 0 {
		t.Fatalf("user delete : %s", errs)
	}
	code, out, _ = runCLI(t, "", "user", "list", "-config", cfg)
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Fatalf("liste après suppression : %q", out)
	}
	if code, _, _ = runCLI(t, "", "user", "delete", "alice", "--yes", "-config", cfg); code == 0 {
		t.Error("suppression d'un compte absent acceptée")
	}
}

func TestUsageErrors(t *testing.T) {
	cfg := cliEnv(t)
	for _, args := range [][]string{
		{"nimporte"},
		{"user"},
		{"user", "add", "-config", cfg},
		{"user", "list", "extra", "-config", cfg},
		{"user", "bogus", "x", "-config", cfg},
		{"user", "add", "alice", "-config", "/inexistant.json"},
	} {
		if code, _, _ := runCLI(t, "", args...); code == 0 {
			t.Errorf("%v : code 0 inattendu", args)
		}
	}
	if code, out, _ := runCLI(t, "", "version"); code != 0 || strings.TrimSpace(out) != version {
		t.Errorf("version : %d %q", code, out)
	}
}
