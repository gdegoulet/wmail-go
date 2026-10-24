package ratelimit

import (
	"testing"
	"time"

	"wmail-go/internal/config"
)

func TestSlidingWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := New()
	l.now = func() time.Time { return now }
	lim := config.Limit{Count: 3, Window: 10 * time.Minute}

	for i := range 3 {
		if blocked, _ := l.Check("k", lim); blocked {
			t.Fatalf("bloqué trop tôt (%d)", i)
		}
		l.Add("k", lim)
		now = now.Add(time.Minute)
	}
	blocked, wait := l.Check("k", lim)
	if !blocked {
		t.Fatal("doit être bloqué après 3 événements")
	}
	// Le premier événement date de 3 min : il sort de la fenêtre dans 7 min.
	if wait != 7*time.Minute {
		t.Fatalf("délai %v, attendu 7m", wait)
	}
	now = now.Add(7 * time.Minute)
	if blocked, _ := l.Check("k", lim); blocked {
		t.Fatal("doit être débloqué quand le plus ancien événement sort de la fenêtre")
	}
}

func TestKeysAreIndependentAndReset(t *testing.T) {
	l := New()
	lim := config.Limit{Count: 1, Window: time.Hour}
	l.Add("a", lim)
	if b, _ := l.Check("a", lim); !b {
		t.Fatal("a doit être bloquée")
	}
	if b, _ := l.Check("b", lim); b {
		t.Fatal("b ne doit pas être bloquée")
	}
	l.Reset("a")
	if b, _ := l.Check("a", lim); b {
		t.Fatal("a doit être débloquée après Reset")
	}
}

func TestDisabledLimit(t *testing.T) {
	l := New()
	for range 100 {
		l.Add("k", config.Limit{})
	}
	if b, _ := l.Check("k", config.Limit{}); b {
		t.Fatal("limite désactivée : jamais bloqué")
	}
}

func TestSweep(t *testing.T) {
	now := time.Now()
	l := New()
	l.now = func() time.Time { return now }
	l.Add("k", config.Limit{Count: 1, Window: time.Minute})
	now = now.Add(2 * time.Minute)
	l.Sweep()
	if len(l.m) != 0 {
		t.Fatal("Sweep doit supprimer les clés expirées")
	}
}
