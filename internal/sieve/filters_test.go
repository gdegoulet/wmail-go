package sieve

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	gosieve "github.com/foxcpp/go-sieve"
)

func mustBuild(t *testing.T, fs ...Filter) string {
	t.Helper()
	s, err := BuildScript(fs)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// compiles vérifie qu'un script est syntaxiquement valide pour l'interpréteur de test (qui ne connaît ni
// vacation ni regex : les scripts qui les utilisent ne passent pas par ici).
func compiles(t *testing.T, script string) {
	t.Helper()
	if _, err := gosieve.Load(strings.NewReader(script), gosieve.DefaultOptions()); err != nil {
		t.Errorf("script invalide : %v\n%s", err, script)
	}
}

var sample = Filter{
	ID: "f1", Name: "Factures", Enabled: true, ConditionsType: MatchAny, ActionType: ActionMoveTo, ActionValue: "Comptabilité/Envoyés",
	Keep: true, Stop: true, MarkAsRead: true,
	Conditions: []Condition{
		{Field: FieldFrom, Type: TypeContains, Value: "factur@x.fr, compta@y.fr"},
		{Field: FieldSubject, Type: TypeNotContains, Value: `Re: "devis"`},
		{Field: FieldHeader, Type: TypeEqualTo, Value: "bulk", ValueSecond: "Precedence"},
		{Field: FieldSize, Type: TypeOver, Value: "100K"},
	},
}

func TestBuildScriptGolden(t *testing.T) {
	got := mustBuild(t, sample)
	for _, want := range []string{
		"require [\"fileinto\", \"imap4flags\"];",
		"# RAINLOOP:SIEVE",
		"BEGIN:FILTER:f1",
		"if anyof(",
		`header :contains ["From"] ["factur@x.fr", "compta@y.fr"],`,
		`not header :contains ["Subject"] "Re: \"devis\"",`,
		`header :is ["Precedence"] "bulk",`,
		`size :over 100K`,
		`addflag "\\Seen";`,
		`fileinto "Comptabilité/Envoyés";`,
		"stop;",
		"/* END:FILTER */",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("manque %q dans :\n%s", want, got)
		}
	}
	compiles(t, got)
}

func TestBuildScriptActions(t *testing.T) {
	cases := []struct {
		name  string
		f     Filter
		want  []string
		check bool // validable par l'interpréteur de test
	}{
		{"aucune", Filter{ActionType: ActionNone}, []string{"stop;"}, true},
		{"supprimer", Filter{ActionType: ActionDiscard}, []string{"discard;", "stop;"}, true},
		{"rejeter", Filter{ActionType: ActionReject, ActionValue: "Pas ici"}, []string{`reject "Pas ici";`, `require ["reject"]`}, true},
		{"transférer", Filter{ActionType: ActionForward, ActionValue: "x@y.fr", Keep: true}, []string{`fileinto "INBOX";`, `redirect "x@y.fr";`}, true},
		{"transférer sans copie", Filter{ActionType: ActionForward, ActionValue: "x@y.fr"}, []string{`redirect "x@y.fr";`}, true},
		{"absence", Filter{ActionType: ActionVacation, ActionValue: "Absent", ActionValueSecond: "Vacances", ActionValueThird: "7", ActionValueFourth: "a@b.fr, c@d.fr", Stop: true},
			[]string{`vacation :days 7 :addresses ["a@b.fr", "c@d.fr"] :subject "Vacances" "Absent";`, `require ["vacation"]`}, false},
		{"absence 1 jour", Filter{ActionType: ActionVacation, ActionValue: "x", ActionValueThird: "0"}, []string{`vacation :days 1 "x";`}, false},
		{"lu sans conditions", Filter{ActionType: ActionMoveTo, ActionValue: "A", MarkAsRead: true}, []string{`addflag "\\Seen";`, `fileinto "A";`}, true},
	}
	for _, c := range cases {
		c.f.ID, c.f.Name, c.f.Enabled, c.f.ConditionsType = "id", "n", true, MatchAll
		got := mustBuild(t, c.f)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s : %q absent :\n%s", c.name, w, got)
			}
		}
		if strings.Contains(got, "if ") {
			t.Errorf("%s : un filtre sans condition ne doit pas avoir de « if »", c.name)
		}
		if c.check {
			compiles(t, got)
		}
	}
	// aucune condition : l'action s'applique à tous les messages, sans accolades
	got := mustBuild(t, Filter{ID: "id", Name: "n", Enabled: true, ActionType: ActionDiscard})
	if strings.Contains(got, "{") && strings.Contains(strings.SplitN(got, "END:HEADER", 2)[1], "{") {
		t.Errorf("accolades inutiles :\n%s", got)
	}
}

func TestBuildScriptAllOf(t *testing.T) {
	f := Filter{ID: "a", Name: "n", Enabled: true, ConditionsType: MatchAll, ActionType: ActionMoveTo, ActionValue: "X",
		Conditions: []Condition{{Field: FieldFrom, Type: TypeEqualTo, Value: "a@b.fr"}, {Field: FieldRecipient, Type: TypeContains, Value: "c@d.fr"}}}
	got := mustBuild(t, f)
	if !strings.Contains(got, "if allof(") || !strings.Contains(got, `header :contains ["To", "CC"] "c@d.fr"`) {
		t.Errorf("allof :\n%s", got)
	}
	compiles(t, got)
	f.Conditions = f.Conditions[:1]
	got = mustBuild(t, f)
	if strings.Contains(got, "anyof") || strings.Contains(got, "allof") || !strings.Contains(got, `if header :is ["From"] "a@b.fr"`) {
		t.Errorf("une condition :\n%s", got)
	}
}

func TestRoundTrip(t *testing.T) {
	other := Filter{ID: "f2", Name: "Absence", Enabled: false, ConditionsType: MatchAll, ActionType: ActionVacation, ActionValue: "Je suis absent.\nRetour lundi.",
		ActionValueSecond: "Absent", ActionValueThird: "3", Keep: false, Stop: true,
		Conditions: []Condition{{Field: FieldRecipient, Type: TypeRegex, Value: `^a.*\.fr$`}}}
	got, ok := ParseScript(mustBuild(t, sample, other))
	if !ok || len(got) != 2 {
		t.Fatalf("filtres relus : %v %d", ok, len(got))
	}
	if !reflect.DeepEqual(got[0], sample) || !reflect.DeepEqual(got[1], other) {
		t.Errorf("aller-retour :\n%+v\n%+v", got[0], got[1])
	}
	if _, ok := ParseScript("require [\"fileinto\"]; fileinto \"x\";"); ok {
		t.Error("un script quelconque n'est pas un script de filtres")
	}
}

// Script tel que le produit RainLoop (en-tête JSON de PHP, dossier en UTF-7 modifié, booléens PHP).
func TestParseRainLoopScript(t *testing.T) {
	hdr := `{"ID":"abc123","Enabled":true,"Name":"Envoyés","Conditions":[{"Field":"From","Type":"Contains","Value":"x@y.fr","ValueSecond":""}],"ConditionsType":"Any","ActionType":"MoveTo","ActionValue":"INBOX.Envoy&AOk-s","ActionValueSecond":"","ActionValueThird":"","ActionValueFourth":"","Keep":true,"Stop":true,"MarkAsRead":false}`
	hdr2 := `{"ID":"def","Enabled":"1","Name":"Vieux format","Conditions":[],"ActionType":"Discard","Keep":"","Stop":"1","MarkAsRead":"1"}`
	enc := func(s string) string {
		b := base64.StdEncoding.EncodeToString([]byte(s))
		var out strings.Builder
		for len(b) > 74 {
			out.WriteString(b[:74] + "\r\n")
			b = b[74:]
		}
		return out.String() + b + "\r\n"
	}
	script := "require [\"fileinto\"];\r\n# RAINLOOP:SIEVE\r\n\r\n/*\r\nBEGIN:FILTER:abc123\r\nBEGIN:HEADER\r\n" + enc(hdr) + "END:HEADER\r\n*/\r\n\r\nif header :contains [\"From\"] \"x@y.fr\"\r\n{\r\n    fileinto \"INBOX.Envoyés\";\r\n    stop;\r\n}\r\n/* END:FILTER */\r\n" +
		"/*\r\nBEGIN:FILTER:def\r\nBEGIN:HEADER\r\n" + enc(hdr2) + "END:HEADER\r\n*/\r\n/* @Filter is disabled \r\ndiscard;\r\n*/\r\n/* END:FILTER */\r\n"
	got, ok := ParseScript(script)
	if !ok || len(got) != 2 {
		t.Fatalf("relecture : %v %+v", ok, got)
	}
	if got[0].ActionValue != "INBOX.Envoyés" || got[0].Name != "Envoyés" || !got[0].Enabled || got[0].ConditionsType != MatchAny {
		t.Errorf("premier filtre : %+v", got[0])
	}
	if got[1].ActionType != ActionDiscard || !got[1].Enabled || got[1].Keep || !got[1].MarkAsRead || got[1].ConditionsType != MatchAny {
		t.Errorf("second filtre (valeurs PHP) : %+v", got[1])
	}
}

func TestScriptInjectionIsNeutralized(t *testing.T) {
	evil := Filter{ID: "e", Name: "x", Enabled: false, ConditionsType: MatchAll, ActionType: ActionMoveTo, ActionValue: "A\"; discard; /* ",
		Conditions: []Condition{
			{Field: FieldSubject, Type: TypeContains, Value: "*/ discard; /*"},
			{Field: FieldFrom, Type: TypeContains, Value: "a\\\" ; discard; \""},
			{Field: FieldHeader, Type: TypeContains, Value: "v", ValueSecond: "X\"]; discard; [\""},
			{Field: FieldSize, Type: TypeOver, Value: "1; discard"},
		}}
	got := mustBuild(t, evil)
	body := strings.SplitN(got, "/* @Filter is disabled", 2)[1]
	body = strings.SplitN(body, "/* END:FILTER */", 2)[0]
	if n := strings.Count(body, "*/"); n != 1 {
		t.Errorf("« */ » dans un filtre désactivé : %d occurrences (1 attendue : la fin du bloc) :\n%s", n, body)
	}
	if strings.Contains(body, `; discard;`) && !strings.Contains(body, `\"; discard`) {
		t.Errorf("injection possible :\n%s", body)
	}
	if strings.Count(body, "false") < 2 {
		t.Errorf("les conditions invalides doivent devenir « false » :\n%s", body)
	}
	// avec un filtre actif, une valeur ne sort jamais de sa chaîne
	evil.Enabled = true
	evil.Conditions = evil.Conditions[:2]
	evil.ActionValue = "Dossier"
	compiles(t, mustBuild(t, evil))
}

func TestMultilineMessagesUseCRLF(t *testing.T) {
	f := Filter{ID: "v", Name: "v", Enabled: true, ActionType: ActionVacation, ActionValue: "a\nb\rc\r\nd"}
	got := mustBuild(t, f)
	i := strings.Index(got, `"a`)
	body := got[i : i+strings.Index(got[i:], `";`)]
	if strings.Contains(strings.ReplaceAll(body, "\r\n", ""), "\n") || strings.Contains(strings.ReplaceAll(body, "\r\n", ""), "\r") {
		t.Errorf("fins de ligne non normalisées : %q", body)
	}
}

func TestInvalidIDRejected(t *testing.T) {
	if _, err := BuildScript([]Filter{{ID: "a b\r\nBEGIN:FILTER:x", Name: "n", ActionType: ActionNone}}); err == nil {
		t.Error("identifiant invalide accepté")
	}
}

func TestValid(t *testing.T) {
	all := map[string]bool{"fileinto": true, "reject": true, "vacation": true, "imap4flags": true, "regex": true}
	ok := Filter{Name: "n", ConditionsType: MatchAny, ActionType: ActionMoveTo, ActionValue: "A", Conditions: []Condition{{Field: FieldFrom, Type: TypeContains, Value: "x"}}}
	if errs := ok.Valid(all); len(errs) != 0 {
		t.Errorf("filtre valide refusé : %v", errs)
	}
	bad := func(mut func(*Filter), modules map[string]bool, want string) {
		t.Helper()
		f := ok
		f.Conditions = append([]Condition(nil), ok.Conditions...)
		mut(&f)
		for _, e := range f.Valid(modules) {
			if e == want {
				return
			}
		}
		t.Errorf("erreur %q attendue, obtenu %v", want, f.Valid(modules))
	}
	bad(func(f *Filter) { f.Name = " " }, all, "filters.err.name")
	bad(func(f *Filter) { f.Conditions[0].Value = "" }, all, "filters.err.empty_condition")
	bad(func(f *Filter) { f.Conditions[0].Type = TypeOver }, all, "filters.err.condition_type")
	bad(func(f *Filter) { f.Conditions[0] = Condition{Field: FieldSize, Type: TypeContains, Value: "1"} }, all, "filters.err.condition_type")
	bad(func(f *Filter) { f.Conditions[0] = Condition{Field: FieldSize, Type: TypeOver, Value: "grand"} }, all, "filters.err.size")
	bad(func(f *Filter) {
		f.Conditions[0] = Condition{Field: FieldHeader, Type: TypeContains, Value: "v", ValueSecond: "a b"}
	}, all, "filters.err.header")
	bad(func(f *Filter) { f.Conditions[0].Type = TypeRegex }, map[string]bool{"fileinto": true}, "filters.err.module")
	bad(func(f *Filter) { f.ActionValue = "" }, all, "filters.err.folder")
	bad(func(f *Filter) {}, map[string]bool{}, "filters.err.module")
	bad(func(f *Filter) { f.ActionType, f.ActionValue = ActionForward, "pas une adresse" }, all, "filters.err.address")
	bad(func(f *Filter) { f.ActionType, f.ActionValue = ActionReject, "" }, all, "filters.err.message")
	bad(func(f *Filter) { f.ActionType, f.ActionValue = ActionVacation, "x" }, map[string]bool{}, "filters.err.module")
	bad(func(f *Filter) { f.ActionType, f.ActionValue, f.ActionValueFourth = ActionVacation, "x", "oups" }, all, "filters.err.address")
	bad(func(f *Filter) { f.ActionType = "Detruire" }, all, "filters.err.action")
	bad(func(f *Filter) { f.ConditionsType = "Peut-être" }, all, "filters.err.match")
	bad(func(f *Filter) { f.MarkAsRead = true }, map[string]bool{"fileinto": true}, "filters.err.module")
}

func TestUTF7(t *testing.T) {
	for in, want := range map[string]string{
		"INBOX": "INBOX", "Envoyés": "Envoy&AOk-s", "R&D": "R&-D", "日本語": "&ZeVnLIqe-", "a/é/b": "a/&AOk-/b", "😀": "&2D3eAA-",
	} {
		if got := ToUTF7(in); got != want {
			t.Errorf("ToUTF7(%q) = %q, attendu %q", in, got, want)
		}
		if got := FromUTF7(want); got != in {
			t.Errorf("FromUTF7(%q) = %q, attendu %q", want, got, in)
		}
	}
	if got := FromUTF7("&p!s-"); got != "&p!s-" {
		t.Errorf("entrée invalide : %q", got)
	}
	if got := FromUTF7("a&b"); got != "a&b" {
		t.Errorf("« & » sans fin : %q", got)
	}
}
