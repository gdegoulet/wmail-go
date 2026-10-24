package sieve

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// Test d'intégration contre un vrai Dovecot (Pigeonhole) : make sieve-up, puis make test. Ignoré si DOVECOT_SIEVE n'est
// pas défini ou si le serveur ne répond pas.
func realServer(t *testing.T) *Client {
	t.Helper()
	addr := os.Getenv("DOVECOT_SIEVE")
	if addr == "" {
		t.Skip("DOVECOT_SIEVE non défini")
	}
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Skipf("serveur ManageSieve de test injoignable (%v) : make sieve-up", err)
	}
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	c, err := New(conn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if err := c.Authenticate("test", "testpass"); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRealDovecotManageSieve(t *testing.T) {
	c := realServer(t)
	for _, ext := range []string{"fileinto", "vacation", "regex", "imap4flags", "reject"} {
		if !c.HasExtension(ext) {
			t.Errorf("extension %s non annoncée : %v", ext, c.Extensions())
		}
	}
	// repartir de zéro (le serveur de test garde ses scripts entre deux essais)
	_ = c.SetActive("")
	if list, _ := c.ListScripts(); len(list) > 0 {
		for _, s := range list {
			_ = c.DeleteScript(s.Name)
		}
	}

	filters := []Filter{
		{ID: "r1", Name: "Factures é", Enabled: true, ConditionsType: MatchAny, ActionType: ActionMoveTo, ActionValue: "Comptabilité/Envoyés", Keep: true, Stop: true, MarkAsRead: true,
			Conditions: []Condition{{Field: FieldFrom, Type: TypeContains, Value: "a@x.fr, b@y.fr"}, {Field: FieldSize, Type: TypeOver, Value: "100K"}, {Field: FieldHeader, Type: TypeRegex, Value: "^bulk", ValueSecond: "Precedence"}}},
		{ID: "r2", Name: "Absence", Enabled: true, ConditionsType: MatchAll, ActionType: ActionVacation, ActionValue: "Je reviens.\nÀ lundi.", ActionValueSecond: "Absent", ActionValueThird: "7", ActionValueFourth: "me@x.fr", Stop: true},
		{ID: "r3", Name: "Désactivé", Enabled: false, ConditionsType: MatchAny, ActionType: ActionDiscard, Conditions: []Condition{{Field: FieldSubject, Type: TypeContains, Value: "*/ x /*"}}},
		{ID: "r4", Name: "Rejet", Enabled: true, ActionType: ActionReject, ActionValue: "Non."},
	}
	script, err := BuildScript(filters)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CheckScript(script); err != nil {
		t.Fatalf("le vrai serveur refuse le script généré : %v\n%s", err, script)
	}
	if err := c.PutScript(ScriptName, script); err != nil {
		t.Fatal(err)
	}
	if err := c.SetActive(ScriptName); err != nil {
		t.Fatal(err)
	}
	list, err := c.ListScripts()
	if err != nil || len(list) != 1 || list[0].Name != ScriptName || !list[0].Active {
		t.Fatalf("liste : %+v %v", list, err)
	}
	got, err := c.GetScript(ScriptName)
	if err != nil {
		t.Fatal(err)
	}
	back, ok := ParseScript(got)
	if !ok || len(back) != len(filters) {
		t.Fatalf("relecture : %v %d filtres\n%s", ok, len(back), got)
	}
	for i := range filters {
		if back[i].Name != filters[i].Name || back[i].ActionType != filters[i].ActionType || back[i].Enabled != filters[i].Enabled {
			t.Errorf("filtre %d : %+v", i, back[i])
		}
	}
	if back[0].ActionValue != "Comptabilité/Envoyés" {
		t.Errorf("dossier après UTF-7 : %q", back[0].ActionValue)
	}

	// un script invalide est refusé avec un message du serveur
	err = c.CheckScript("require [\"fileinto\"];\nif header :contains \"Subject\" { fileinto \"x\"; }\n")
	if se, ok := err.(*ServerError); !ok || se.Status != "NO" || se.Text == "" {
		t.Errorf("script invalide : %v", err)
	}
	if err := c.PutScript("mauvais", "ceci n'est pas un script"); err == nil {
		t.Error("PUTSCRIPT d'un script invalide accepté")
	}
	// un grand script
	var big strings.Builder
	for i := 0; i < 400; i++ {
		big.WriteString("# ligne de commentaire numéro " + strings.Repeat("x", 100) + "\r\n")
	}
	if err := c.PutScript("grand", big.String()+"keep;\r\n"); err != nil {
		t.Errorf("grand script : %v", err)
	}
	// nettoyage : désactiver avant de supprimer
	_ = c.SetActive("")
	for _, n := range []string{ScriptName, "grand"} {
		if err := c.DeleteScript(n); err != nil {
			t.Errorf("suppression de %s : %v", n, err)
		}
	}
	_ = c.Logout()
}
