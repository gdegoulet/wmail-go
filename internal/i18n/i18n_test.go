package i18n

import (
	"slices"
	"testing"
)

func TestLocalesHaveSameKeys(t *testing.T) {
	b, err := New("fr")
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Languages(); !slices.Equal(got, []string{"en", "fr"}) {
		t.Fatalf("langues %v", got)
	}
	if fr, en := b.Keys("fr"), b.Keys("en"); !slices.Equal(fr, en) {
		for _, k := range fr {
			if !slices.Contains(en, k) {
				t.Errorf("clé %q absente de en.json", k)
			}
		}
		for _, k := range en {
			if !slices.Contains(fr, k) {
				t.Errorf("clé %q absente de fr.json", k)
			}
		}
	}
}

func TestTranslateAndFallback(t *testing.T) {
	b, _ := New("fr")
	if got := b.T("en", "login.submit"); got != "Sign in" {
		t.Errorf("en : %q", got)
	}
	if got := b.T("de", "login.submit"); got != "Se connecter" {
		t.Errorf("repli sur la langue par défaut : %q", got)
	}
	if got := b.T("fr", "clé.inexistante"); got != "clé.inexistante" {
		t.Errorf("clé inconnue : %q", got)
	}
	if got := b.T("fr", "error.password_short", 12); got != "Le mot de passe doit contenir au moins 12 caractères." {
		t.Errorf("argument : %q", got)
	}
}

func TestMatch(t *testing.T) {
	b, _ := New("fr")
	cases := map[string]string{
		"en-US,en;q=0.9,fr;q=0.8": "en",
		"fr-CH, fr;q=0.9":         "fr",
		"de-DE,de;q=0.9":          "",
		"":                        "",
		"de, en;q=0.5":            "en",
	}
	for in, want := range cases {
		if got := b.Match(in); got != want {
			t.Errorf("Match(%q) = %q, attendu %q", in, got, want)
		}
	}
}
