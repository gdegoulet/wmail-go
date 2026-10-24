package sieve

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
)

// Noms des scripts sur le serveur : les mêmes que RainLoop, pour que les filtres créés dans l'un soient lus par l'autre.
const (
	ScriptName    = "rainloop.user"
	ScriptRawName = "rainloop.user.raw"
)

// Champs, types de condition et actions (valeurs de RainLoop).
const (
	FieldFrom      = "From"
	FieldRecipient = "Recipient"
	FieldSubject   = "Subject"
	FieldHeader    = "Header"
	FieldSize      = "Size"

	TypeEqualTo     = "EqualTo"
	TypeNotEqualTo  = "NotEqualTo"
	TypeContains    = "Contains"
	TypeNotContains = "NotContains"
	TypeRegex       = "Regex"
	TypeOver        = "Over"
	TypeUnder       = "Under"

	MatchAny = "Any"
	MatchAll = "All"

	ActionNone     = "None"
	ActionMoveTo   = "MoveTo"
	ActionDiscard  = "Discard"
	ActionVacation = "Vacation"
	ActionReject   = "Reject"
	ActionForward  = "Forward"
)

// flexBool accepte les booléens écrits par PHP : true, false, "1", "0", 1, 0, "".
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	*b = s == "1" || s == "true"
	return nil
}

// Condition est une condition d'un filtre.
type Condition struct {
	Field       string
	Type        string
	Value       string
	ValueSecond string // nom de l'en-tête pour le champ Header
}

// Filter est un filtre : conditions, action et options. Les noms des champs JSON sont ceux de RainLoop.
type Filter struct {
	ID                string
	Name              string
	Enabled           bool
	Conditions        []Condition
	ConditionsType    string
	ActionType        string
	ActionValue       string // dossier (UTF-8), adresse, message…
	ActionValueSecond string
	ActionValueThird  string
	ActionValueFourth string
	Keep              bool
	Stop              bool
	MarkAsRead        bool
}

type jsonFilter struct {
	ID                string
	Enabled           flexBool
	Name              string
	Conditions        []Condition
	ConditionsType    string
	ActionType        string
	ActionValue       string
	ActionValueSecond string
	ActionValueThird  string
	ActionValueFourth string
	Keep              flexBool
	Stop              flexBool
	MarkAsRead        flexBool
}

// NewID fabrique un identifiant de filtre.
func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// marshalHeader sérialise un filtre pour l'en-tête d'un script. Le dossier de destination est écrit en UTF-7 modifié
// comme le fait RainLoop.
func marshalHeader(f Filter) ([]byte, error) {
	conds := f.Conditions
	if conds == nil {
		conds = []Condition{}
	}
	value := f.ActionValue
	if f.ActionType == ActionMoveTo {
		value = ToUTF7(value)
	}
	return json.Marshal(map[string]any{
		"ID": f.ID, "Enabled": f.Enabled, "Name": f.Name, "Conditions": conds, "ConditionsType": f.ConditionsType,
		"ActionType": f.ActionType, "ActionValue": value, "ActionValueSecond": f.ActionValueSecond,
		"ActionValueThird": f.ActionValueThird, "ActionValueFourth": f.ActionValueFourth,
		"Keep": f.Keep, "Stop": f.Stop, "MarkAsRead": f.MarkAsRead,
	})
}

func unmarshalHeader(data []byte) (Filter, bool) {
	var j jsonFilter
	// valeurs par défaut de RainLoop pour les clés absentes
	j.Enabled, j.Keep, j.Stop = true, true, true
	j.ConditionsType, j.ActionType = MatchAny, ActionMoveTo
	if err := json.Unmarshal(bytes.TrimSpace(data), &j); err != nil {
		return Filter{}, false
	}
	f := Filter{
		ID: j.ID, Name: j.Name, Enabled: bool(j.Enabled), Conditions: j.Conditions, ConditionsType: j.ConditionsType,
		ActionType: j.ActionType, ActionValue: j.ActionValue, ActionValueSecond: j.ActionValueSecond,
		ActionValueThird: j.ActionValueThird, ActionValueFourth: j.ActionValueFourth,
		Keep: bool(j.Keep), Stop: bool(j.Stop), MarkAsRead: bool(j.MarkAsRead),
	}
	if f.ActionType == ActionMoveTo {
		f.ActionValue = FromUTF7(f.ActionValue)
	}
	if f.ID == "" {
		f.ID = NewID()
	}
	return f, true
}

var headerBlockRe = regexp.MustCompile(`(?s)BEGIN:FILTER(.+?)BEGIN:HEADER(.+?)END:HEADER`)

// ParseScript relit les filtres d'un script écrit par RainLoop ou par wmail-go (via les en-têtes encodés).
// ok est faux si le script n'est pas un script de filtres.
func ParseScript(script string) (filters []Filter, ok bool) {
	if !strings.Contains(script, "RAINLOOP:SIEVE") {
		return nil, false
	}
	for _, m := range headerBlockRe.FindAllStringSubmatch(script, -1) {
		raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(m[2]), ""))
		if err != nil || len(raw) == 0 {
			continue
		}
		if f, ok := unmarshalHeader(raw); ok {
			filters = append(filters, f)
		}
	}
	return filters, true
}

// --- génération ---

func qs(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(s)
	// Un filtre désactivé est enveloppé dans un commentaire /* … */ : une valeur contenant « */ » le refermerait et
	// laisserait le reste s'exécuter. Une barre oblique échappée garde la même valeur en Sieve.
	s = strings.ReplaceAll(s, "*/", "*\\/")
	s = strings.ReplaceAll(s, "/*", "/\\*")
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n") // une chaîne Sieve ne contient que des CRLF
}

var (
	sizeRe       = regexp.MustCompile(`^[0-9]{1,12}[KkMmGg]?$`)
	headerNameRe = regexp.MustCompile(`^[!-9;-~]{1,100}$`)
	spacesRe     = regexp.MustCompile(` {2,}`)
)

const errFalse = "false"

// conditionScript renvoie le test Sieve d'une condition et les extensions nécessaires.
func conditionScript(c Condition, req map[string]bool) string {
	val, second := strings.TrimSpace(c.Value), strings.TrimSpace(c.ValueSecond)
	if val == "" || (c.Field == FieldHeader && second == "") {
		return errFalse
	}
	neg := ""
	word := ""
	switch c.Type {
	case TypeOver:
		word = ":over"
	case TypeUnder:
		word = ":under"
	case TypeNotEqualTo:
		neg, word = "not ", ":is"
	case TypeEqualTo:
		word = ":is"
	case TypeNotContains:
		neg, word = "not ", ":contains"
	case TypeContains:
		word = ":contains"
	case TypeRegex:
		word = ":regex"
		req["regex"] = true
	default:
		return errFalse
	}
	var test string
	switch c.Field {
	case FieldFrom:
		test = `header ` + word + ` ["From"]`
	case FieldRecipient:
		test = `header ` + word + ` ["To", "CC"]`
	case FieldSubject:
		test = `header ` + word + ` ["Subject"]`
	case FieldHeader:
		if !headerNameRe.MatchString(second) {
			return errFalse
		}
		test = `header ` + word + ` ["` + qs(second) + `"]`
	case FieldSize:
		if !sizeRe.MatchString(val) || (c.Type != TypeOver && c.Type != TypeUnder) {
			return errFalse
		}
		return spacesRe.ReplaceAllString(neg+`size `+word+` `+val, " ")
	default:
		return errFalse
	}
	if c.Type == TypeOver || c.Type == TypeUnder {
		return errFalse
	}
	if (c.Field == FieldFrom || c.Field == FieldRecipient) && strings.Contains(val, ",") {
		var parts []string
		for _, p := range strings.Split(val, ",") {
			parts = append(parts, `"`+qs(p)+`"`)
		}
		test += " [" + strings.Join(parts, ", ") + "]"
	} else {
		test += ` "` + qs(val) + `"`
	}
	return spacesRe.ReplaceAllString(neg+test, " ")
}

func filterScript(f Filter, req map[string]bool) string {
	const nl = "\r\n"
	tab := "    "
	var out []string
	all := false
	switch n := len(f.Conditions); {
	case n > 1:
		kind := "allof"
		if f.ConditionsType == MatchAny {
			kind = "anyof"
		}
		out = append(out, "if "+kind+"(")
		for i, c := range f.Conditions {
			s := tab + conditionScript(c, req)
			if i < n-1 {
				s += ","
			}
			out = append(out, s)
		}
		out = append(out, ")")
	case n == 1:
		out = append(out, "if "+conditionScript(f.Conditions[0], req))
	default:
		all = true
	}
	if !all {
		out = append(out, "{")
	} else {
		tab = ""
	}
	if f.MarkAsRead && (f.ActionType == ActionNone || f.ActionType == ActionMoveTo || f.ActionType == ActionForward) {
		req["imap4flags"] = true
		out = append(out, tab+`addflag "\\Seen";`)
	}
	switch f.ActionType {
	case ActionNone:
		out = append(out, tab+"stop;")
	case ActionDiscard:
		out = append(out, tab+"discard;", tab+"stop;")
	case ActionVacation:
		msg := strings.TrimSpace(f.ActionValue)
		if msg == "" {
			out = append(out, tab+"# @Error (vacation): empty action value")
			break
		}
		req["vacation"] = true
		days := 1
		if n, err := strconv.Atoi(strings.TrimSpace(f.ActionValueThird)); err == nil && n > 1 {
			days = min(n, 365)
		}
		subject := ""
		if s := strings.Join(strings.Fields(f.ActionValueSecond), " "); s != "" {
			subject = `:subject "` + qs(s) + `" `
		}
		addrs := ""
		var list []string
		for _, a := range strings.Split(f.ActionValueFourth, ",") {
			if a = strings.TrimSpace(a); a != "" {
				list = append(list, `"`+qs(a)+`"`)
			}
		}
		if len(list) > 0 {
			addrs = ":addresses [" + strings.Join(list, ", ") + "] "
		}
		out = append(out, tab+`vacation :days `+strconv.Itoa(days)+` `+addrs+subject+`"`+qs(msg)+`";`)
		if f.Stop {
			out = append(out, tab+"stop;")
		}
	case ActionReject:
		msg := strings.TrimSpace(f.ActionValue)
		if msg == "" {
			out = append(out, tab+"# @Error (reject): empty action value")
			break
		}
		req["reject"] = true
		out = append(out, tab+`reject "`+qs(msg)+`";`, tab+"stop;")
	case ActionForward:
		to := strings.TrimSpace(f.ActionValue)
		if to == "" {
			out = append(out, tab+"# @Error (redirect): empty action value")
			break
		}
		if f.Keep {
			req["fileinto"] = true
			out = append(out, tab+`fileinto "INBOX";`)
		}
		out = append(out, tab+`redirect "`+qs(to)+`";`, tab+"stop;")
	case ActionMoveTo:
		folder := strings.TrimSpace(f.ActionValue)
		if folder == "" {
			out = append(out, tab+"# @Error (fileinto): empty action value")
			break
		}
		req["fileinto"] = true
		out = append(out, tab+`fileinto "`+qs(folder)+`";`, tab+"stop;")
	}
	if !all {
		out = append(out, "}")
	}
	return strings.Join(out, nl)
}

// BuildScript produit le script Sieve complet pour une liste de filtres, avec les en-têtes que RainLoop relit.
func BuildScript(filters []Filter) (string, error) {
	const nl = "\r\n"
	req := map[string]bool{}
	parts := []string{"# This is RainLoop Webmail sieve script.", "# Please don't change anything here.", "# RAINLOOP:SIEVE", ""}
	for _, f := range filters {
		if f.ID == "" {
			f.ID = NewID()
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(f.ID) {
			return "", fmt.Errorf("identifiant de filtre invalide")
		}
		hdr, err := marshalHeader(f)
		if err != nil {
			return "", err
		}
		enc := base64.StdEncoding.EncodeToString(hdr)
		var chunks strings.Builder
		for len(enc) > 74 {
			chunks.WriteString(enc[:74] + nl)
			enc = enc[74:]
		}
		chunks.WriteString(enc + nl)
		block := []string{"/*", "BEGIN:FILTER:" + f.ID, "BEGIN:HEADER", chunks.String() + "END:HEADER", "*/"}
		body := filterScript(f, req)
		if f.Enabled {
			block = append(block, "", body, "")
		} else {
			block = append(block, "/* @Filter is disabled ", body, "*/")
		}
		block = append(block, "/* END:FILTER */", "")
		parts = append(parts, strings.Join(block, nl))
	}
	var caps []string
	for _, c := range []string{"fileinto", "reject", "vacation", "imap4flags", "regex"} {
		if req[c] {
			caps = append(caps, `"`+c+`"`)
		}
	}
	head := ""
	if len(caps) > 0 {
		head = nl + "require [" + strings.Join(caps, ", ") + "];" + nl
	}
	return head + nl + strings.Join(parts, nl) + nl, nil
}

// Valid renvoie les clés d'erreur d'un filtre avant enregistrement ; modules indique les extensions du serveur.
func (f Filter) Valid(modules map[string]bool) []string {
	var errs []string
	add := func(k string) { errs = append(errs, k) }
	if strings.TrimSpace(f.Name) == "" || len([]rune(f.Name)) > 100 || strings.ContainsAny(f.Name, "\r\n") {
		add("filters.err.name")
	}
	if len(f.Conditions) > 30 {
		add("filters.err.too_many")
	}
	for _, c := range f.Conditions {
		if strings.TrimSpace(c.Value) == "" {
			add("filters.err.empty_condition")
			break
		}
	}
	for _, c := range f.Conditions {
		switch {
		case c.Field == FieldSize && c.Type != TypeOver && c.Type != TypeUnder,
			c.Field != FieldSize && (c.Type == TypeOver || c.Type == TypeUnder):
			add("filters.err.condition_type")
		case c.Field == FieldSize && !sizeRe.MatchString(strings.TrimSpace(c.Value)):
			add("filters.err.size")
		case c.Field == FieldHeader && !headerNameRe.MatchString(strings.TrimSpace(c.ValueSecond)):
			add("filters.err.header")
		case c.Type == TypeRegex && !modules["regex"]:
			add("filters.err.module")
		case c.Field != FieldSize && c.Field != FieldFrom && c.Field != FieldRecipient && c.Field != FieldSubject && c.Field != FieldHeader:
			add("filters.err.condition_type")
		}
	}
	switch f.ActionType {
	case ActionNone, ActionDiscard:
	case ActionMoveTo:
		if strings.TrimSpace(f.ActionValue) == "" {
			add("filters.err.folder")
		}
		if !modules["fileinto"] {
			add("filters.err.module")
		}
	case ActionForward:
		if _, err := mail.ParseAddress(strings.TrimSpace(f.ActionValue)); err != nil || strings.ContainsAny(f.ActionValue, "\r\n") {
			add("filters.err.address")
		}
	case ActionReject:
		if strings.TrimSpace(f.ActionValue) == "" {
			add("filters.err.message")
		}
		if !modules["reject"] {
			add("filters.err.module")
		}
	case ActionVacation:
		if strings.TrimSpace(f.ActionValue) == "" {
			add("filters.err.message")
		}
		if !modules["vacation"] {
			add("filters.err.module")
		}
		for _, a := range strings.Split(f.ActionValueFourth, ",") {
			if a = strings.TrimSpace(a); a != "" {
				if _, err := mail.ParseAddress(a); err != nil {
					add("filters.err.address")
					break
				}
			}
		}
	default:
		add("filters.err.action")
	}
	if f.MarkAsRead && !modules["imap4flags"] {
		add("filters.err.module")
	}
	if f.ConditionsType != MatchAny && f.ConditionsType != MatchAll {
		add("filters.err.match")
	}
	return dedupe(errs)
}

func dedupe(l []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range l {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
