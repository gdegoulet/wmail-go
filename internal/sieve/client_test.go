package sieve

import (
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/gdegoulet/wmail-go/internal/sieve/sievetest"
)

func dial(t *testing.T, port int) *Client {
	t.Helper()
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(conn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestClientLifecycle(t *testing.T) {
	srv := sievetest.Start(t, "none")
	c := dial(t, srv.Port)
	if !c.HasExtension("fileinto") || !c.HasExtension("VACATION") || c.HasExtension("nonexistent") {
		t.Errorf("extensions : %v", c.Extensions())
	}
	if err := c.Authenticate(sievetest.User, "faux"); !errors.Is(err, ErrAuth) {
		t.Fatalf("mauvais mot de passe : %v", err)
	}
	if err := c.Authenticate(sievetest.User, sievetest.Password); err != nil {
		t.Fatal(err)
	}
	if list, err := c.ListScripts(); err != nil || len(list) != 0 {
		t.Fatalf("liste vide attendue : %v %v", list, err)
	}
	body := "require [\"fileinto\"];\r\nif header :contains \"Subject\" \"x\" { fileinto \"A\"; }\r\n// é \"guillemets\" et accents\r\n"
	if err := c.PutScript("mon script", body); err != nil {
		t.Fatal(err)
	}
	if err := c.SetActive("mon script"); err != nil {
		t.Fatal(err)
	}
	list, err := c.ListScripts()
	if err != nil || len(list) != 1 || list[0].Name != "mon script" || !list[0].Active {
		t.Fatalf("liste : %+v %v", list, err)
	}
	got, err := c.GetScript("mon script")
	if err != nil || got != body {
		t.Fatalf("script relu : %q %v", got, err)
	}
	// un script invalide est refusé par le serveur, avec son message
	err = c.PutScript("mauvais", "SYNTAXERROR")
	var se *ServerError
	if !errors.As(err, &se) || se.Status != "NO" || !strings.Contains(se.Text, "syntax") {
		t.Errorf("script invalide : %v", err)
	}
	if err := c.CheckScript("SYNTAXERROR"); err == nil {
		t.Error("CHECKSCRIPT devrait refuser")
	}
	if err := c.CheckScript(body); err != nil {
		t.Errorf("CHECKSCRIPT : %v", err)
	}
	// le script actif ne se supprime pas ; après désactivation si
	if err := c.DeleteScript("mon script"); err == nil {
		t.Error("suppression d'un script actif acceptée")
	}
	if err := c.SetActive(""); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteScript("mon script"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetScript("inexistant"); !errors.As(err, &se) || se.Code != "NONEXISTENT" {
		t.Errorf("script inexistant : %v", err)
	}
	// noms et contenus dangereux
	if err := c.PutScript("a\r\nLOGOUT", "x"); err == nil {
		t.Error("nom avec retour à la ligne accepté")
	}
	if err := c.PutScript(`un"nom\`, "x"); err != nil {
		t.Errorf("nom avec guillemet : %v", err)
	}
	if err := c.Logout(); err != nil {
		t.Log("logout :", err)
	}
}

func TestClientStartTLSRequiredBeforeAuth(t *testing.T) {
	srv := sievetest.Start(t, "starttls")
	c := dial(t, srv.Port)
	if _, ok := c.Capability("STARTTLS"); !ok {
		t.Fatal("STARTTLS non annoncé")
	}
	// sans STARTTLS, le serveur refuse l'authentification
	if err := c.Authenticate(sievetest.User, sievetest.Password); !errors.Is(err, ErrAuth) {
		t.Fatalf("authentification en clair : %v", err)
	}
	c2 := dial(t, srv.Port)
	if err := c2.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	if err := c2.Authenticate(sievetest.User, sievetest.Password); err != nil {
		t.Fatal(err)
	}
	if _, err := c2.ListScripts(); err != nil {
		t.Fatal(err)
	}
	// serveur sans STARTTLS
	plain := dial(t, sievetest.Start(t, "none").Port)
	if err := plain.StartTLS(&tls.Config{InsecureSkipVerify: true}); !errors.Is(err, ErrNoStartTLS) { //nolint:gosec
		t.Errorf("STARTTLS absent : %v", err)
	}
}
