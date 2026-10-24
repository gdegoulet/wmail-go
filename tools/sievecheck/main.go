// sievecheck produit des scripts Sieve avec le générateur de wmail-go et des messages d'essai, avec le résultat attendu
// de chaque couple (script, message). Le script run.sh les fait compiler et exécuter par le vrai interpréteur Pigeonhole
// (Dovecot) pour vérifier que les filtres font bien ce qu'ils annoncent.
// Usage : go run ./tools/sievecheck <dossier de sortie>
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"wmail-go/internal/sieve"
)

type msg struct {
	Name    string   `json:"name"`
	EML     string   `json:"eml"`
	Present []string `json:"present"` // fragments qui doivent figurer dans la sortie de sieve-test
	Absent  []string `json:"absent"`  // fragments qui ne doivent pas y figurer
}

type testCase struct {
	Name    string   `json:"name"`
	Filters []string `json:"-"`
	Msgs    []msg    `json:"msgs"`
}

func eml(from, to, subject, extra, body string) string {
	if body == "" {
		body = "texte"
	}
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n%sMessage-ID: <t@x>\r\n\r\n%s\r\n", from, to, subject, extra, body)
}

const (
	moved = "store message in folder: "
	keep  = "store message in folder: INBOX"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage : sievecheck <dossier>")
	}
	out := os.Args[1]
	_ = os.MkdirAll(out, 0o755)
	alice := eml("Alice <alice@x.fr>", "me@localhost", "Facture octobre", "", "")
	bob := eml("Bob <bob@y.fr>", "me@localhost", "Re: Réunion", "", "")
	big := eml("Carl <carl@z.fr>", "me@localhost", "Gros", "", strings.Repeat("0123456789abcdef", 200))

	f := func(name string, conds []sieve.Condition, match, action, value string, mod func(*sieve.Filter)) sieve.Filter {
		x := sieve.Filter{ID: strings.ToLower(strings.ReplaceAll(name, " ", "")), Name: name, Enabled: true, Conditions: conds, ConditionsType: match, ActionType: action, ActionValue: value, Keep: true, Stop: true}
		if mod != nil {
			mod(&x)
		}
		return x
	}
	c := func(field, typ, value, second string) sieve.Condition {
		return sieve.Condition{Field: field, Type: typ, Value: value, ValueSecond: second}
	}
	type spec struct {
		name    string
		filters []sieve.Filter
		msgs    []msg
	}
	specs := []spec{
		{"deplacer", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "MoveTo", "Projets", nil)},
			[]msg{{"alice", alice, []string{moved + "Projets"}, []string{keep}}, {"bob", bob, []string{keep}, []string{moved + "Projets"}}}},
		{"dossier accentue", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "MoveTo", "Comptabilité/Envoyés", nil)},
			[]msg{{"alice", alice, []string{moved + "Comptabilité/Envoyés"}, nil}}},
		{"plusieurs adresses", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "zed@z.fr, bob@y.fr", "")}, "Any", "MoveTo", "B", nil)},
			[]msg{{"bob", bob, []string{moved + "B"}, nil}, {"alice", alice, []string{keep}, []string{moved + "B"}}}},
		{"objet exclu", []sieve.Filter{f("a", []sieve.Condition{c("Subject", "NotContains", "Re:", "")}, "Any", "MoveTo", "Sans", nil)},
			[]msg{{"alice", alice, []string{moved + "Sans"}, nil}, {"bob", bob, []string{keep}, []string{moved + "Sans"}}}},
		{"toutes les conditions", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", ""), c("Subject", "Contains", "octobre", "")}, "All", "MoveTo", "Both", nil)},
			[]msg{{"alice", alice, []string{moved + "Both"}, nil}, {"autre objet", eml("Alice <alice@x.fr>", "me@localhost", "Autre", "", ""), []string{keep}, []string{moved + "Both"}}}},
		{"une des conditions", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", ""), c("From", "Contains", "bob", "")}, "Any", "MoveTo", "Either", nil)},
			[]msg{{"alice", alice, []string{moved + "Either"}, nil}, {"bob", bob, []string{moved + "Either"}, nil}, {"carl", big, []string{keep}, []string{moved + "Either"}}}},
		{"en-tete libre", []sieve.Filter{f("a", []sieve.Condition{c("Header", "EqualTo", "bulk", "Precedence")}, "Any", "MoveTo", "Masse", nil)},
			[]msg{{"bulk", eml("L <l@x.fr>", "me@localhost", "Info", "Precedence: bulk\r\n", ""), []string{moved + "Masse"}, nil}, {"normal", alice, []string{keep}, nil}}},
		{"taille", []sieve.Filter{f("a", []sieve.Condition{c("Size", "Over", "1K", "")}, "Any", "MoveTo", "Gros", nil)},
			[]msg{{"gros", big, []string{moved + "Gros"}, nil}, {"petit", alice, []string{keep}, []string{moved + "Gros"}}}},
		{"destinataires", []sieve.Filter{f("a", []sieve.Condition{c("Recipient", "Contains", "liste@x.org", "")}, "Any", "MoveTo", "Liste", nil)},
			[]msg{{"cc", eml("A <a@x.fr>", "me@localhost", "S", "Cc: liste@x.org\r\n", ""), []string{moved + "Liste"}, nil}, {"to", eml("A <a@x.fr>", "liste@x.org", "S", "", ""), []string{moved + "Liste"}, nil}, {"aucun", alice, []string{keep}, nil}}},
		{"regex", []sieve.Filter{f("a", []sieve.Condition{c("Subject", "Regex", "^Re: .*union$", "")}, "Any", "MoveTo", "Re", nil)},
			[]msg{{"bob", bob, []string{moved + "Re"}, nil}, {"alice", alice, []string{keep}, []string{moved + "Re"}}}},
		{"supprimer", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "Discard", "", nil)},
			[]msg{{"alice", alice, []string{"* discard"}, []string{keep}}, {"bob", bob, []string{keep}, []string{"* discard"}}}},
		{"rejeter", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "Reject", "Pas ici", nil)},
			[]msg{{"alice", alice, []string{"reject message"}, nil}}},
		{"transferer avec copie", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "Forward", "autre@example.org", nil)},
			[]msg{{"alice", alice, []string{"redirect message to: <autre@example.org>", keep}, nil}}},
		{"transferer sans copie", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "Forward", "autre@example.org", func(x *sieve.Filter) { x.Keep = false })},
			[]msg{{"alice", alice, []string{"redirect message to: <autre@example.org>"}, []string{keep}}}},
		{"absence", []sieve.Filter{f("a", nil, "Any", "Vacation", "Je reviens lundi.", func(x *sieve.Filter) { x.ActionValueSecond = "Absent"; x.ActionValueThird = "5"; x.Stop = false })},
			[]msg{{"alice", alice, []string{"send vacation message"}, nil}}},
		{"marquer lu", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "MoveTo", "Lus", func(x *sieve.Filter) { x.MarkAsRead = true })},
			[]msg{{"alice", alice, []string{moved + "Lus", `add IMAP flags: \seen`}, nil}}},
		{"filtre desactive", []sieve.Filter{f("a", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "Discard", "", func(x *sieve.Filter) { x.Enabled = false })},
			[]msg{{"alice", alice, []string{keep}, []string{"* discard"}}}},
		{"ordre et arret", []sieve.Filter{
			f("premier", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "MoveTo", "Un", nil),
			f("second", []sieve.Condition{c("From", "Contains", "alice", "")}, "Any", "MoveTo", "Deux", nil)},
			[]msg{{"alice", alice, []string{moved + "Un"}, []string{moved + "Deux"}}}},
		{"tous les messages", []sieve.Filter{f("a", nil, "Any", "MoveTo", "Tout", nil)},
			[]msg{{"alice", alice, []string{moved + "Tout"}, nil}, {"bob", bob, []string{moved + "Tout"}, nil}}},
		// valeurs hostiles : elles doivent rester des données
		{"injection active", []sieve.Filter{f("a", []sieve.Condition{c("Subject", "Contains", `"; discard; "`, ""), c("From", "Contains", "x\\\" ; discard; \"", "")}, "Any", "MoveTo", "A\"; discard; /*", nil)},
			[]msg{{"alice", alice, []string{keep}, []string{"* discard"}}, {"objet piege", eml("A <a@x.fr>", "me@localhost", `"; discard; "`, "", ""), []string{moved + `A"; discard; /*`}, []string{"* discard"}}}},
		{"injection desactivee", []sieve.Filter{
			f("a", []sieve.Condition{c("Subject", "Contains", "*/ discard; /*", "")}, "Any", "MoveTo", "X", func(x *sieve.Filter) { x.Enabled = false }),
			f("b", []sieve.Condition{c("From", "Contains", "bob", "")}, "Any", "MoveTo", "Bob", nil)},
			[]msg{{"alice", alice, []string{keep}, []string{"* discard"}}, {"bob", bob, []string{moved + "Bob"}, []string{"* discard"}}}},
	}
	for _, s := range specs {
		script, err := sieve.BuildScript(s.filters)
		if err != nil {
			log.Fatalf("%s : %v", s.name, err)
		}
		base := strings.ReplaceAll(s.name, " ", "_")
		must(os.WriteFile(filepath.Join(out, base+".sieve"), []byte(script), 0o644))
		for _, m := range s.msgs {
			must(os.WriteFile(filepath.Join(out, base+"."+strings.ReplaceAll(m.Name, " ", "_")+".eml"), []byte(m.EML), 0o644))
		}
		meta, _ := json.Marshal(s.msgs)
		must(os.WriteFile(filepath.Join(out, base+".json"), meta, 0o644))
	}
	fmt.Printf("%d scénarios écrits dans %s\n", len(specs), out)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
