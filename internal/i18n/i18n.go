// Package i18n fournit les textes de l'interface (français et anglais).
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

//go:embed locales/*.json
var localeFS embed.FS

// Bundle contient les traductions par langue.
type Bundle struct {
	msgs map[string]map[string]string
	def  string
}

// New charge les fichiers de langue embarqués. def est la langue de repli.
func New(def string) (*Bundle, error) {
	entries, err := fs.ReadDir(localeFS, "locales")
	if err != nil {
		return nil, err
	}
	b := &Bundle{msgs: map[string]map[string]string{}, def: def}
	for _, e := range entries {
		data, err := localeFS.ReadFile(path.Join("locales", e.Name()))
		if err != nil {
			return nil, err
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("%s : %w", e.Name(), err)
		}
		b.msgs[strings.TrimSuffix(e.Name(), ".json")] = m
	}
	if _, ok := b.msgs[def]; !ok {
		return nil, fmt.Errorf("langue par défaut %q absente", def)
	}
	return b, nil
}

// Languages renvoie les codes de langue disponibles, triés.
func (b *Bundle) Languages() []string {
	out := make([]string, 0, len(b.msgs))
	for k := range b.msgs {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Has indique si la langue existe.
func (b *Bundle) Has(lang string) bool { _, ok := b.msgs[lang]; return ok }

// T renvoie le texte de la clé dans la langue, avec repli sur la langue par défaut puis sur la clé.
func (b *Bundle) T(lang, key string, args ...any) string {
	s, ok := b.msgs[lang][key]
	if !ok {
		if s, ok = b.msgs[b.def][key]; !ok {
			return key
		}
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// Match choisit une langue d'après un en-tête Accept-Language ; renvoie "" si aucune ne convient.
func (b *Bundle) Match(accept string) string {
	for _, part := range strings.Split(accept, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.ToLower(strings.TrimSpace(tag))
		if i := strings.IndexAny(tag, "-_"); i > 0 {
			tag = tag[:i]
		}
		if b.Has(tag) {
			return tag
		}
	}
	return ""
}

// Keys renvoie les clés d'une langue (pour vérifier la cohérence des fichiers).
func (b *Bundle) Keys(lang string) []string {
	out := make([]string, 0, len(b.msgs[lang]))
	for k := range b.msgs[lang] {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
