package web

import (
	"errors"
	"testing"
)

func TestAttachStoreLimitsAndCleanup(t *testing.T) {
	s := newAttachStore(attachLimits{Each: 10, Message: 15, Session: 20, Count: 3})
	add := func(comp string, n int) (*pendingAttachment, error) {
		return s.Add("s1", comp, pendingAttachment{Name: "f", Data: make([]byte, n)})
	}
	if _, err := add("c1", 11); !errors.Is(err, errAttachTooBig) {
		t.Errorf("trop grosse : %v", err)
	}
	a, err := add("c1", 8)
	if err != nil || a.ID == "" {
		t.Fatal(err)
	}
	if _, err := add("c1", 8); !errors.Is(err, errAttachMsgLimit) {
		t.Errorf("limite du message : %v", err)
	}
	if _, err := add("c2", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := add("c3", 5); !errors.Is(err, errAttachSession) {
		t.Errorf("limite de session : %v", err)
	}
	if !s.Remove("s1", "c1", a.ID) || s.Remove("s1", "c1", a.ID) {
		t.Error("Remove")
	}
	if _, err := add("c3", 5); err != nil {
		t.Errorf("la place libérée doit être réutilisable : %v", err)
	}
	for i := 0; i < 2; i++ {
		_, _ = add("c3", 1)
	}
	if _, err := add("c3", 1); !errors.Is(err, errAttachTooMany) {
		t.Errorf("nombre : %v", err)
	}
	// une autre session n'est pas affectée
	if _, err := s.Add("s2", "c1", pendingAttachment{Data: make([]byte, 10)}); err != nil {
		t.Error(err)
	}
	s.Clear("s1", "c2")
	if got := s.Compositions("s1"); len(got) != 1 || got[0] != "c3" {
		t.Errorf("compositions = %v", got)
	}
	s.ClearSession("s1")
	if len(s.List("s1", "c3")) != 0 || len(s.Compositions("s1")) != 0 {
		t.Error("ClearSession")
	}
	if s.sizeOf["s1"] != 0 || s.sizeOf["s2"] != 10 {
		t.Errorf("comptage : %v", s.sizeOf)
	}
}
