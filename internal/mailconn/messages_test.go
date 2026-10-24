package mailconn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/mailconn/mailtest"
)

func newPool(t *testing.T) (*Pool, Endpoint) {
	t.Helper()
	p := NewPool(localGuard(t), time.Minute)
	t.Cleanup(p.CloseAll)
	return p, ep(startIMAP(t, "none"), "none")
}

func do(t *testing.T, p *Pool, e Endpoint, fn func(*Conn) error) error {
	t.Helper()
	return p.Do(ctx5(t), "s1/a1", e, fn)
}

func list(t *testing.T, p *Pool, e Endpoint, q ListQuery) *ListResult {
	t.Helper()
	var res *ListResult
	if err := do(t, p, e, func(c *Conn) (err error) { res, err = c.ListMessages(q); return }); err != nil {
		t.Fatalf("ListMessages(%+v) : %v", q, err)
	}
	return res
}

func subjects(r *ListResult) []string {
	var out []string
	for _, m := range r.Messages {
		out = append(out, m.Subject)
	}
	return out
}

func TestListPaginationAndDefaultOrder(t *testing.T) {
	p, e := newPool(t)
	r := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 10, Sort: "date", Desc: true})
	if r.Total != mailtest.InboxCount || r.Pages != 4 || r.Page != 1 || len(r.Messages) != 10 {
		t.Fatalf("total %d, pages %d, page %d, %d messages", r.Total, r.Pages, r.Page, len(r.Messages))
	}
	if got := r.Messages[0].Subject; got != "Filler 30" {
		t.Errorf("le plus récent d'abord : %q", got)
	}
	r4 := list(t, p, e, ListQuery{Folder: "INBOX", Page: 4, PageSize: 10, Sort: "date", Desc: true})
	if got := subjects(r4); len(got) != 1 || got[0] != "Facture octobre" {
		t.Errorf("dernière page : %v", got)
	}
	if far := list(t, p, e, ListQuery{Folder: "INBOX", Page: 99, PageSize: 10, Sort: "date", Desc: true}); far.Page != 4 {
		t.Errorf("page hors limites ramenée à %d, attendu 4", far.Page)
	}
	if zero := list(t, p, e, ListQuery{Folder: "INBOX", Page: 0, PageSize: 10, Sort: "date"}); zero.Page != 1 {
		t.Errorf("page 0 ramenée à %d", zero.Page)
	}
	empty := list(t, p, e, ListQuery{Folder: "Sent", Page: 1, PageSize: 10, Sort: "date"})
	if empty.Total != 0 || empty.Pages != 1 || len(empty.Messages) != 0 {
		t.Errorf("dossier vide : %+v", empty)
	}
}

func TestListSorting(t *testing.T) {
	p, e := newPool(t)
	asc := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date"})
	if got := subjects(asc); got[0] != "Facture octobre" || got[len(got)-1] != "Filler 30" {
		t.Errorf("date croissante : %v ... %v", got[0], got[len(got)-1])
	}
	bySubject := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "subject"})
	got := subjects(bySubject)
	if got[0] != "Facture octobre" || got[1] != "Re: Facture octobre" || got[2] != "Filler 04" || got[len(got)-1] != "Réunion lundi" || got[len(got)-2] != "Gala <script>alert(1)</script> & co" {
		t.Errorf("tri par sujet (les « Re: » sont ignorés) : %v", got)
	}
	bySize := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "size", Desc: true})
	if bySize.Messages[0].Subject != "Facture octobre" || !bySize.Messages[0].HasAttachment {
		t.Errorf("le plus gros d'abord : %+v", bySize.Messages[0])
	}
	byFrom := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "from"})
	if byFrom.Messages[0].From[0].Short() != "Alice" || byFrom.Messages[len(byFrom.Messages)-1].From[0].Short() != "Zed" {
		t.Errorf("tri par expéditeur : %v … %v", byFrom.Messages[0].From, byFrom.Messages[len(byFrom.Messages)-1].From)
	}
	byArrival := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 3, Sort: "arrival", Desc: true})
	if got := subjects(byArrival); got[0] != "Filler 30" || len(got) != 3 {
		t.Errorf("ordre d'arrivée : %v", got)
	}
}

func TestListFlagsAndAttachments(t *testing.T) {
	p, e := newPool(t)
	r := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date"})
	byS := map[string]Summary{}
	for _, m := range r.Messages {
		byS[m.Subject] = m
	}
	if m := byS["Facture octobre"]; !m.Flagged || m.Seen || !m.HasAttachment || m.UID == 0 || m.Date.IsZero() || m.Size == 0 {
		t.Errorf("Facture octobre : %+v", m)
	}
	if m := byS["Re: Facture octobre"]; !m.Seen || m.Flagged || m.HasAttachment {
		t.Errorf("Re: Facture : %+v", m)
	}
	if m := byS["Réunion lundi"]; len(m.From) != 1 || m.From[0].Name != "Carol" || m.From[0].Email != "carol@z.fr" {
		t.Errorf("expéditeur : %+v", m.From)
	}
}

func TestListSearch(t *testing.T) {
	p, e := newPool(t)
	cases := []struct {
		q, scope string
		want     int
	}{
		{"Facture", "subject", 2},
		{"bob", "from", 1},
		{"montant", "body", 1},
		{"octobre", "all", 2},
		{"introuvable-xyz", "all", 0},
		{"Filler 1", "subject", 10}, // Filler 10 à 19
	}
	for _, c := range cases {
		r := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date", Search: c.q, Scope: c.scope})
		if r.Total != c.want {
			t.Errorf("recherche %q (%s) : %d résultats, attendu %d", c.q, c.scope, r.Total, c.want)
		}
	}
	empty := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 10, Sort: "date", Search: "introuvable-xyz"})
	if empty.Pages != 1 || empty.Page != 1 {
		t.Errorf("recherche sans résultat : %+v", empty)
	}
}

func TestFetchMessage(t *testing.T) {
	p, e := newPool(t)
	r := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date"})
	uid := r.Messages[0].UID // Facture octobre

	var raw *RawMessage
	if err := do(t, p, e, func(c *Conn) (err error) { raw, err = c.FetchMessage("INBOX", uid, true, 1<<20); return }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw.Raw), "Subject: Facture octobre") || !raw.HasFlag(imap.FlagFlagged) || raw.HasFlag(imap.FlagSeen) {
		t.Errorf("message : drapeaux %v, début %.80q", raw.Flags, raw.Raw)
	}
	// Consulter ne marque pas le message comme lu.
	r = list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date"})
	if r.Messages[0].Seen {
		t.Error("FetchMessage ne doit pas marquer le message comme lu")
	}

	err := do(t, p, e, func(c *Conn) error { _, err := c.FetchMessage("INBOX", 99999, true, 1<<20); return err })
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("message absent : %v", err)
	}
	err = do(t, p, e, func(c *Conn) error { _, err := c.FetchMessage("INBOX", uid, true, 100); return err })
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("taille maximale : %v", err)
	}
}

func TestFlagsMoveExpunge(t *testing.T) {
	p, e := newPool(t)
	first := func() Summary {
		return list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date"}).Messages[0]
	}
	uid := first().UID

	if err := do(t, p, e, func(c *Conn) error { return c.StoreFlags("INBOX", []uint32{uid}, imap.StoreFlagsAdd, imap.FlagSeen) }); err != nil {
		t.Fatal(err)
	}
	if m := first(); !m.Seen || !m.Flagged {
		t.Errorf("lu ajouté, favori conservé : %+v", m)
	}
	if err := do(t, p, e, func(c *Conn) error {
		return c.StoreFlags("INBOX", []uint32{uid}, imap.StoreFlagsDel, imap.FlagSeen, imap.FlagFlagged)
	}); err != nil {
		t.Fatal(err)
	}
	if m := first(); m.Seen || m.Flagged {
		t.Errorf("drapeaux retirés : %+v", m)
	}

	// Déplacement vers un dossier à créer.
	err := do(t, p, e, func(c *Conn) error {
		if err := c.EnsureFolder("Corbeille"); err != nil {
			return err
		}
		return c.Move("INBOX", []uint32{uid}, "Corbeille")
	})
	if err != nil {
		t.Fatal(err)
	}
	inbox := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date"})
	moved := list(t, p, e, ListQuery{Folder: "Corbeille", Page: 1, PageSize: 50, Sort: "date"})
	if inbox.Total != mailtest.InboxCount-1 || moved.Total != 1 || moved.Messages[0].Subject != "Facture octobre" {
		t.Errorf("après déplacement : INBOX %d, Corbeille %d", inbox.Total, moved.Total)
	}
	var folders []Folder
	if err := do(t, p, e, func(c *Conn) (err error) { folders, err = c.Folders(false); return }); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range folders {
		found = found || f.Name == "Corbeille"
	}
	if !found {
		t.Error("le dossier créé doit apparaître dans la liste")
	}
	// EnsureFolder est idempotent.
	if err := do(t, p, e, func(c *Conn) error { return c.EnsureFolder("Corbeille") }); err != nil {
		t.Errorf("EnsureFolder sur un dossier existant : %v", err)
	}

	// Suppression définitive.
	if err := do(t, p, e, func(c *Conn) error { return c.Expunge("Corbeille", []uint32{moved.Messages[0].UID}) }); err != nil {
		t.Fatal(err)
	}
	if left := list(t, p, e, ListQuery{Folder: "Corbeille", Page: 1, PageSize: 50, Sort: "date"}); left.Total != 0 {
		t.Errorf("après suppression définitive : %d messages", left.Total)
	}
	if err := do(t, p, e, func(c *Conn) error { return c.Move("INBOX", nil, "x") }); err != nil {
		t.Errorf("liste vide : %v", err)
	}
}

func TestValidFolderName(t *testing.T) {
	for _, ok := range []string{"INBOX", "Corbeille", "Projets/2026", "Éléments supprimés"} {
		if !ValidFolderName(ok) {
			t.Errorf("%q refusé", ok)
		}
	}
	for _, bad := range []string{"", "a*", "a%b", "ligne\nsuivante", strings.Repeat("x", 256)} {
		if ValidFolderName(bad) {
			t.Errorf("%q accepté", bad)
		}
	}
	p, e := newPool(t)
	if err := do(t, p, e, func(c *Conn) error { return c.EnsureFolder("a*") }); err == nil {
		t.Error("EnsureFolder doit refuser les jokers")
	}
}

// --- Pool ---

func TestPoolReusesAndReconnects(t *testing.T) {
	p, e := newPool(t)
	for range 3 {
		if err := do(t, p, e, func(c *Conn) error { _, err := c.Select("INBOX", true); return err }); err != nil {
			t.Fatal(err)
		}
	}
	if p.Len() != 1 {
		t.Fatalf("%d connexions, attendu 1 (réutilisation)", p.Len())
	}
	first := p.m["s1/a1"].client

	// Connexion coupée entre deux requêtes : le pool se reconnecte sans erreur visible.
	_ = first.Close()
	if err := do(t, p, e, func(c *Conn) error { _, err := c.Select("INBOX", true); return err }); err != nil {
		t.Fatalf("reconnexion : %v", err)
	}
	if p.m["s1/a1"].client == first {
		t.Error("une nouvelle connexion était attendue")
	}

	// Identifiants modifiés : la connexion existante n'est pas réutilisée.
	bad := e
	bad.Password = "faux"
	err := p.Do(ctx5(t), "s1/a1", bad, func(c *Conn) error { return nil })
	if k := kindOf(t, err); k != KindAuth {
		t.Errorf("nouveaux identifiants refusés : %q", k)
	}

	// Fermeture par préfixe (fin de session).
	if err := do(t, p, e, func(c *Conn) error { return nil }); err != nil {
		t.Fatal(err)
	}
	p.Close("s1/")
	if p.Len() != 0 {
		t.Errorf("%d connexions après Close", p.Len())
	}
}

func TestPoolSweepClosesIdle(t *testing.T) {
	p, e := newPool(t)
	p.idle = time.Millisecond
	if err := do(t, p, e, func(c *Conn) error { return nil }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	p.Sweep()
	if p.Len() != 0 {
		t.Errorf("%d connexions après Sweep", p.Len())
	}
}

func TestPoolContextTimeoutDropsConnection(t *testing.T) {
	p, e := newPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	err := p.Do(ctx, "s1/a1", e, func(c *Conn) error {
		cancel()
		time.Sleep(50 * time.Millisecond)
		_, err := c.Select("INBOX", true)
		return err
	})
	if err == nil {
		t.Fatal("une requête annulée doit échouer")
	}
	// La connexion cassée est remplacée à la requête suivante.
	if err := do(t, p, e, func(c *Conn) error { _, err := c.Select("INBOX", true); return err }); err != nil {
		t.Fatalf("après annulation : %v", err)
	}
}

func TestPoolConcurrentUseIsSerialized(t *testing.T) {
	p, e := newPool(t)
	errs := make(chan error, 8)
	for range 8 {
		go func() {
			errs <- do(t, p, e, func(c *Conn) error {
				_, err := c.ListMessages(ListQuery{Folder: "INBOX", Page: 1, PageSize: 5, Sort: "date", Desc: true})
				return err
			})
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if p.Len() != 1 {
		t.Errorf("%d connexions, attendu 1", p.Len())
	}
}

func TestFixUTF8(t *testing.T) {
	if got := fixUTF8("r\xe9union"); got != "réunion" {
		t.Errorf("ISO-8859-1 relu : %q", got)
	}
	if got := fixUTF8("réunion"); got != "réunion" {
		t.Errorf("UTF-8 valide inchangé : %q", got)
	}
}

func TestListUnreadOnly(t *testing.T) {
	p, e := newPool(t)
	all := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date", Desc: true})
	unread := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date", Desc: true, UnreadOnly: true})
	if all.Total != mailtest.InboxCount || unread.Total != mailtest.InboxCount-1 {
		t.Fatalf("tous %d, non lus %d (un seul message est lu)", all.Total, unread.Total)
	}
	for _, m := range unread.Messages {
		if m.Seen || m.Subject == "Re: Facture octobre" {
			t.Errorf("message lu dans la liste des non lus : %+v", m)
		}
	}
	// Combiné avec une recherche et avec le tri côté Go.
	r := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "subject", UnreadOnly: true, Search: "Facture", Scope: "subject"})
	if r.Total != 1 || r.Messages[0].Subject != "Facture octobre" {
		t.Errorf("non lus + recherche : %v", subjects(r))
	}
	// Une fois lu, le message sort de la liste des non lus.
	uid := r.Messages[0].UID
	if err := do(t, p, e, func(c *Conn) error { return c.StoreFlags("INBOX", []uint32{uid}, imap.StoreFlagsAdd, imap.FlagSeen) }); err != nil {
		t.Fatal(err)
	}
	if r := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "date", UnreadOnly: true, Search: "Facture", Scope: "subject"}); r.Total != 0 {
		t.Errorf("après lecture : %d résultat(s)", r.Total)
	}
}

func TestClearFolder(t *testing.T) {
	p, e := newPool(t)
	// Trois messages à la corbeille, puis on la vide ; INBOX n'est pas touchée.
	all := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "arrival"})
	uids := []uint32{all.Messages[0].UID, all.Messages[1].UID, all.Messages[2].UID}
	if err := do(t, p, e, func(c *Conn) error { return c.Move("INBOX", uids, "Trash") }); err != nil {
		t.Fatal(err)
	}
	if got := list(t, p, e, ListQuery{Folder: "Trash", Page: 1, PageSize: 50, Sort: "arrival"}); got.Total != 3 {
		t.Fatalf("corbeille : %d messages", got.Total)
	}
	var n int
	if err := do(t, p, e, func(c *Conn) (err error) { n, err = c.ClearFolder("Trash"); return }); err != nil || n != 3 {
		t.Fatalf("ClearFolder : %d, %v", n, err)
	}
	if got := list(t, p, e, ListQuery{Folder: "Trash", Page: 1, PageSize: 50, Sort: "arrival"}); got.Total != 0 {
		t.Errorf("la corbeille contient encore %d messages", got.Total)
	}
	if got := list(t, p, e, ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Sort: "arrival"}); got.Total != mailtest.InboxCount-3 {
		t.Errorf("INBOX : %d messages, attendu %d", got.Total, mailtest.InboxCount-3)
	}
	// Dossier déjà vide : rien à faire, pas d'erreur.
	if err := do(t, p, e, func(c *Conn) (err error) { n, err = c.ClearFolder("Trash"); return }); err != nil || n != 0 {
		t.Errorf("dossier vide : %d, %v", n, err)
	}
}

// Sans MOVE, déplacer plus de 128 messages bloquait la connexion (voir Conn.Move).
func TestMoveWithoutMoveCapabilityManyMessages(t *testing.T) {
	const n = 400
	port := mailtest.StartIMAPSeeded(t, "nomove", func(t testing.TB, add func([]byte, time.Time, ...imap.Flag)) {
		for i := 0; i < n; i++ {
			add(mailtest.Message("a@example.org", "test@localhost", fmt.Sprintf("bulk %d", i), time.Now(), "x"), time.Now())
		}
	})
	p := NewPool(localGuard(t), time.Minute)
	t.Cleanup(p.CloseAll)
	e := ep(port, SecurityNone)

	var uids []uint32
	if err := do(t, p, e, func(c *Conn) error {
		res, err := c.ListMessages(ListQuery{Folder: "INBOX", PageSize: 500, Search: "bulk", Scope: "subject"})
		if err != nil {
			return err
		}
		for _, m := range res.Messages {
			uids = append(uids, m.UID)
		}
		if c.Client.Caps().Has(imap.CapMove) {
			t.Fatal("le serveur de test ne devrait pas annoncer MOVE")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(uids) != n {
		t.Fatalf("%d messages trouvés, attendu %d", len(uids), n)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- p.Do(ctx, "k", e, func(c *Conn) error { return c.Move("INBOX", uids, "Trash") })
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("déplacement bloqué")
	}
	if err := do(t, p, e, func(c *Conn) error {
		res, err := c.ListMessages(ListQuery{Folder: "Trash", PageSize: 500})
		if err != nil {
			return err
		}
		if res.Total != n {
			t.Errorf("corbeille : %d messages, attendu %d", res.Total, n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAppendAndSendSMTP(t *testing.T) {
	p, e := newPool(t)
	raw := []byte("From: a@example.org\r\nTo: test@localhost\r\nSubject: brouillon\r\nMessage-Id: <draft1@example.org>\r\n\r\ncorps\r\n")
	var uid uint32
	if err := do(t, p, e, func(c *Conn) (err error) {
		uid, err = c.Append("Sent", raw, "draft1@example.org", imap.FlagSeen)
		return
	}); err != nil {
		t.Fatal(err)
	}
	if uid == 0 {
		t.Error("UID du message ajouté inconnu")
	}
	got := list(t, p, e, ListQuery{Folder: "Sent", Page: 1, PageSize: 50, Sort: "arrival"})
	if got.Total != 1 || got.Messages[0].UID != uid || got.Messages[0].Subject != "brouillon" {
		t.Errorf("après ajout : %+v (uid %d)", got, uid)
	}

	srv := mailtest.StartSMTPCapture(t, "none")
	g := localGuard(t)
	se := ep(srv.Port, SecurityNone)
	if err := g.SendSMTP(ctx5(t), se, "a@example.org", []string{"x@y.fr", "z@y.fr"}, raw, false); err != nil {
		t.Fatal(err)
	}
	msgs := srv.Messages()
	if len(msgs) != 1 || msgs[0].From != "a@example.org" || len(msgs[0].Rcpt) != 2 || !strings.Contains(string(msgs[0].Data), "Subject: brouillon") {
		t.Errorf("message reçu : %+v", msgs)
	}

	// un destinataire refusé : rien n'est envoyé, l'erreur nomme l'adresse
	err := g.SendSMTP(ctx5(t), se, "a@example.org", []string{"x@y.fr", "refuse@y.fr"}, raw, false)
	var sendErr *SendError
	if !errors.As(err, &sendErr) || len(sendErr.Rejected) != 1 || sendErr.Rejected[0] != "refuse@y.fr" {
		t.Fatalf("erreur = %v", err)
	}
	if len(srv.Messages()) != 1 {
		t.Error("un message a été transmis malgré le refus d'un destinataire")
	}
	if err := g.SendSMTP(ctx5(t), se, "a@example.org", nil, raw, false); !errors.Is(err, ErrNoRecipients) {
		t.Errorf("sans destinataire : %v", err)
	}
	// mauvais identifiants
	bad := se
	bad.Password = "faux"
	if k := kindOf(t, g.SendSMTP(ctx5(t), bad, "a@example.org", []string{"x@y.fr"}, raw, false)); k != KindAuth {
		t.Errorf("catégorie = %q", k)
	}
}

func TestFolderManagement(t *testing.T) {
	p, e := newPool(t)
	names := func() []string {
		var out []string
		if err := do(t, p, e, func(c *Conn) error {
			fs, err := c.Folders(false)
			for _, f := range fs {
				out = append(out, f.Name)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	has := func(n string) bool { return slicesContains(names(), n) }

	if err := do(t, p, e, func(c *Conn) error { return c.CreateFolder("Projets/Nouveau") }); err != nil || !has("Projets/Nouveau") {
		t.Fatalf("création : %v %v", err, names())
	}
	if err := do(t, p, e, func(c *Conn) error { return c.RenameFolder("Projets/Nouveau", "Projets/Renommé") }); err != nil || has("Projets/Nouveau") || !has("Projets/Renommé") {
		t.Fatalf("renommage : %v %v", err, names())
	}
	if err := do(t, p, e, func(c *Conn) error { return c.SetSubscribed("Projets/Renommé", false) }); err != nil {
		t.Fatalf("désabonnement : %v", err)
	}
	if err := do(t, p, e, func(c *Conn) error { return c.SetSubscribed("Projets/Renommé", true) }); err != nil {
		t.Fatalf("abonnement : %v", err)
	}
	if err := do(t, p, e, func(c *Conn) error { return c.DeleteFolder("Projets/Renommé") }); err != nil || has("Projets/Renommé") {
		t.Fatalf("suppression : %v %v", err, names())
	}
	// INBOX est protégée, les noms invalides sont refusés avant tout envoi au serveur
	for _, fn := range []func(*Conn) error{
		func(c *Conn) error { return c.DeleteFolder("INBOX") },
		func(c *Conn) error { return c.DeleteFolder("inbox") },
		func(c *Conn) error { return c.RenameFolder("INBOX", "Autre") },
		func(c *Conn) error { return c.CreateFolder("a*b") },
		func(c *Conn) error { return c.CreateFolder("") },
		func(c *Conn) error { return c.RenameFolder("Sent", "x\r\ny") },
	} {
		if err := do(t, p, e, fn); err == nil {
			t.Error("opération interdite acceptée")
		}
	}
	if !has("INBOX") {
		t.Error("INBOX a disparu")
	}
}

func slicesContains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}
