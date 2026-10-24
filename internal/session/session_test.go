package session

import (
	"testing"
	"time"

	"wmail-go/internal/store"
	"wmail-go/internal/userdata"
)

func acct(login string) *store.Account {
	return &store.Account{Login: login, Data: userdata.New()}
}

func TestIdleAndAbsoluteExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	m := NewManager()
	m.now = func() time.Time { return now }

	s, err := m.Create(acct("alice"), 30*time.Minute, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ID) < 40 || s.ID == s.CSRF {
		t.Fatalf("identifiants de session faibles : %q %q", s.ID, s.CSRF)
	}
	now = now.Add(20 * time.Minute)
	if m.Get(s.ID) == nil {
		t.Fatal("session valide refusée")
	}
	// L'activité prolonge l'inactivité…
	now = now.Add(25 * time.Minute)
	if m.Get(s.ID) == nil {
		t.Fatal("l'activité doit prolonger la session")
	}
	// … mais pas au-delà de la durée maximale.
	for range 5 {
		now = now.Add(25 * time.Minute)
		m.Get(s.ID)
	}
	if m.Get(s.ID) != nil {
		t.Fatal("la durée maximale doit couper la session")
	}
	if m.Count() != 0 {
		t.Fatal("session expirée non supprimée")
	}

	s2, _ := m.Create(acct("bob"), 10*time.Minute, time.Hour)
	now = now.Add(11 * time.Minute)
	if m.Get(s2.ID) != nil {
		t.Fatal("l'inactivité doit couper la session")
	}
}

func TestDeleteWipesKey(t *testing.T) {
	m := NewManager()
	a := acct("alice")
	a.DEK[0] = 0xAA
	s, _ := m.Create(a, time.Hour, time.Hour)
	m.Delete(s.ID)
	if a.DEK[0] != 0 {
		t.Fatal("la clé de données doit être effacée à la fermeture")
	}
	if m.Get(s.ID) != nil || m.Get("") != nil {
		t.Fatal("session supprimée encore accessible")
	}
}

func TestDeleteUserExcept(t *testing.T) {
	m := NewManager()
	a, _ := m.Create(acct("alice"), time.Hour, time.Hour)
	b, _ := m.Create(acct("alice"), time.Hour, time.Hour)
	c, _ := m.Create(acct("bob"), time.Hour, time.Hour)
	if n := m.DeleteUser("alice", a.ID); n != 1 {
		t.Fatalf("%d sessions fermées, attendu 1", n)
	}
	if m.Get(a.ID) == nil || m.Get(b.ID) != nil || m.Get(c.ID) == nil {
		t.Fatal("mauvaises sessions fermées")
	}
}

func TestSetTTLAndSweep(t *testing.T) {
	now := time.Now()
	m := NewManager()
	m.now = func() time.Time { return now }
	s, _ := m.Create(acct("alice"), time.Hour, time.Hour)
	m.SetTTL(s, time.Minute, time.Hour)
	now = now.Add(2 * time.Minute)
	m.Sweep()
	if m.Count() != 0 {
		t.Fatal("Sweep doit supprimer les sessions expirées")
	}
}
