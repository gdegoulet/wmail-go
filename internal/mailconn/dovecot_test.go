package mailconn

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"

	"wmail-go/internal/mailview"
)

// Tests d'intégration contre un vrai Dovecot (make sieve-up). Ignorés si DOVECOT_IMAP n'est pas défini ou injoignable.
// Chaque exécution utilise un utilisateur neuf : sa boîte est vide au départ.
func dovecot(t *testing.T) (*Pool, Endpoint) {
	t.Helper()
	addr := os.Getenv("DOVECOT_IMAP")
	if addr == "" {
		t.Skip("DOVECOT_IMAP non défini")
	}
	host, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.Atoi(portStr)
	g, err := NewGuard(true, []string{host})
	if err != nil {
		t.Fatal(err)
	}
	e := Endpoint{Host: host, Port: port, Security: SecurityNone, Login: fmt.Sprintf("it%d", rand.Intn(1e9)), Password: "testpass"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.TestIMAP(ctx, e); err != nil {
		t.Skipf("Dovecot de test injoignable ou refusant la connexion : %v (make sieve-up)", err)
	}
	p := NewPool(g, time.Minute)
	t.Cleanup(p.CloseAll)
	return p, e
}

func dv(t *testing.T, p *Pool, e Endpoint, fn func(*Conn) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := p.Do(ctx, "it", e, fn); err != nil {
		t.Fatal(err)
	}
}

func rawMsg(i int, from, subject, body string, when time.Time) []byte {
	return []byte(fmt.Sprintf("From: %s\r\nTo: moi@example.org\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <it%d@test>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		from, subject, when.Format(time.RFC1123Z), i, body))
}

func TestRealDovecotMailboxOperations(t *testing.T) {
	p, e := dovecot(t)
	base := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	var uids []uint32
	dv(t, p, e, func(c *Conn) error {
		for i := 0; i < 40; i++ {
			from := fmt.Sprintf("Zoé %02d <z%02d@x.fr>", 39-i, 39-i) // l'ordre alphabétique est l'inverse de l'ordre d'arrivée
			subj := fmt.Sprintf("Sujet %02d é", i)
			body := "corps ordinaire"
			if i == 7 {
				body = "mot-rare-dans-le-corps"
			}
			uid, err := c.Append("INBOX", rawMsg(i, from, subj, body, base.Add(time.Duration(i)*time.Hour)), fmt.Sprintf("it%d@test", i))
			if err != nil {
				return err
			}
			if uid == 0 {
				return fmt.Errorf("UID inconnu pour le message %d", i)
			}
			uids = append(uids, uid)
		}
		return nil
	})

	list := func(q ListQuery) *ListResult {
		var res *ListResult
		dv(t, p, e, func(c *Conn) (err error) { res, err = c.ListMessages(q); return })
		return res
	}
	r := list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 10, Sort: "date", Desc: true})
	if r.Total != 40 || r.Pages != 4 || len(r.Messages) != 10 || r.Messages[0].Subject != "Sujet 39 é" {
		t.Fatalf("page 1 par date décroissante : total=%d pages=%d premier=%q", r.Total, r.Pages, r.Messages[0].Subject)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 4, PageSize: 10, Sort: "date", Desc: true}); r.Messages[len(r.Messages)-1].Subject != "Sujet 00 é" {
		t.Errorf("dernière page : %q", r.Messages[len(r.Messages)-1].Subject)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 5, Sort: "from"}); !strings.HasPrefix(r.Messages[0].Subject, "Sujet 39") {
		t.Errorf("tri par expéditeur : premier = %q (SortFallback=%v)", r.Messages[0].Subject, r.SortFallback)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 5, Sort: "subject", Desc: true}); r.Messages[0].Subject != "Sujet 39 é" {
		t.Errorf("tri par sujet : %q", r.Messages[0].Subject)
	}
	// recherche : objet accentué, corps, expéditeur
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Search: "Sujet 07", Scope: "subject"}); r.Total != 1 {
		t.Errorf("recherche par objet : %d", r.Total)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Search: "mot-rare-dans-le-corps", Scope: "body"}); r.Total != 1 || r.Messages[0].Subject != "Sujet 07 é" {
		t.Errorf("recherche dans le corps : %d", r.Total)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Search: "z05@x.fr", Scope: "from"}); r.Total != 1 {
		t.Errorf("recherche par expéditeur : %d", r.Total)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Search: "é", Scope: "all"}); r.Total != 40 {
		t.Errorf("recherche d'un caractère accentué : %d", r.Total)
	}
	// opérateurs à la manière de RainLoop
	for q, want := range map[string]int{
		"from:z05@x.fr": 1, `subject:"Sujet 07"`: 1, "text:mot-rare-dans-le-corps": 1, "is:unseen": 40, "is:seen": 0, "is:flagged": 0,
		"z05": 1, "moi@example.org": 40, "mot-rare-dans-le-corps": 0, "larger:1MB": 0, "smaller:1MB": 40, "has:attachment": 0, "from:z05@x.fr subject:inconnu": 0, "to:moi@example.org": 40,
	} {
		if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Search: q, Scope: "all"}); r.Total != want {
			t.Errorf("recherche %q : %d résultats, attendu %d", q, r.Total, want)
		}
	}

	// drapeaux, non lus, compteurs
	dv(t, p, e, func(c *Conn) error {
		if err := c.StoreFlags("INBOX", uids[:10], imap.StoreFlagsAdd, imap.FlagSeen); err != nil {
			return err
		}
		return c.StoreFlags("INBOX", uids[10:12], imap.StoreFlagsAdd, imap.FlagFlagged)
	})
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, UnreadOnly: true}); r.Total != 30 {
		t.Errorf("non lus : %d", r.Total)
	}
	dv(t, p, e, func(c *Conn) error {
		fs, err := c.Folders(true)
		for _, f := range fs {
			if f.Name == "INBOX" && (f.Total != 40 || f.Unseen != 30) {
				t.Errorf("compteurs de INBOX : %d messages, %d non lus", f.Total, f.Unseen)
			}
		}
		return err
	})

	// lecture complète d'un message
	dv(t, p, e, func(c *Conn) error {
		m, err := c.FetchMessage("INBOX", uids[7], true, 1<<20)
		if err != nil {
			return err
		}
		if !strings.Contains(string(m.Raw), "mot-rare-dans-le-corps") {
			t.Errorf("message relu : %q", m.Raw)
		}
		return nil
	})

	// déplacement vers un dossier créé à la volée (MOVE), suppression définitive, vidage
	dv(t, p, e, func(c *Conn) error {
		if err := c.EnsureFolder("Trash"); err != nil {
			return err
		}
		if err := c.Move("INBOX", uids[20:30], "Trash"); err != nil {
			return err
		}
		return nil
	})
	if r = list(ListQuery{Folder: "Trash", Page: 1, PageSize: 50}); r.Total != 10 {
		t.Errorf("après déplacement : %d dans Trash", r.Total)
	}
	if r = list(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50}); r.Total != 30 {
		t.Errorf("après déplacement : %d dans INBOX", r.Total)
	}
	trash := list(ListQuery{Folder: "Trash", Page: 1, PageSize: 50})
	dv(t, p, e, func(c *Conn) error {
		if err := c.Expunge("Trash", []uint32{trash.Messages[0].UID, trash.Messages[1].UID}); err != nil {
			return err
		}
		n, err := c.ClearFolder("Trash")
		if n != 8 {
			t.Errorf("ClearFolder : %d messages supprimés (8 attendus)", n)
		}
		return err
	})
	if r = list(ListQuery{Folder: "Trash", Page: 1, PageSize: 50}); r.Total != 0 {
		t.Errorf("Trash après vidage : %d", r.Total)
	}

	// dossiers : création, renommage, abonnement, suppression
	dv(t, p, e, func(c *Conn) error {
		if err := c.CreateFolder("Projets/Été"); err != nil {
			return err
		}
		if err := c.RenameFolder("Projets/Été", "Projets/Hiver"); err != nil {
			return err
		}
		if err := c.SetSubscribed("Projets/Hiver", false); err != nil {
			return err
		}
		sub, known, err := c.Subscribed()
		if err != nil {
			return err
		}
		if known && sub["Projets/Hiver"] {
			t.Error("dossier désabonné encore dans la liste des abonnements")
		}
		fs, err := c.Folders(false)
		if err != nil {
			return err
		}
		found := false
		for _, f := range fs {
			if f.Name == "Projets/Hiver" {
				found, _ = true, f
				if known && !f.Hidden {
					t.Error("dossier désabonné non marqué masqué")
				}
			}
		}
		if !found {
			t.Errorf("dossier absent après renommage : %v", fs)
		}
		return c.DeleteFolder("Projets/Hiver")
	})

	// dossiers au nom accentué : Dovecot 2.4 annonce IMAP4rev2 sans que la connexion soit en mode IMAP4rev2 ; sans
	// UTF8=ACCEPT activé par le client, ces noms étaient refusés (régression : enableUTF8)
	dv(t, p, e, func(c *Conn) error {
		const name = "Éléments supprimés/Été 日本"
		if err := c.CreateFolder(name); err != nil {
			return fmt.Errorf("création : %w", err)
		}
		if err := c.Move("INBOX", uids[30:32], name); err != nil {
			return fmt.Errorf("déplacement : %w", err)
		}
		res, err := c.ListMessages(ListQuery{Folder: name, Page: 1, PageSize: 10})
		if err != nil {
			return fmt.Errorf("lecture : %w", err)
		}
		if res.Total != 2 {
			t.Errorf("%d messages dans le dossier accentué (2 attendus)", res.Total)
		}
		fs, err := c.Folders(true)
		for _, f := range fs {
			if f.Name == name && f.Total != 2 {
				t.Errorf("compteur du dossier accentué : %d", f.Total)
			}
		}
		return err
	})

	// pas de quota configuré sur ce serveur de test
	dv(t, p, e, func(c *Conn) error {
		if _, _, ok := c.Quota(); ok {
			t.Log("quota annoncé par le serveur de test")
		}
		return nil
	})
}

// C-06 : contre un vrai serveur, « from: adresse@exemple.fr » doit trouver autant de messages que la même adresse
// cherchée sans mot-clé, que les deux-points soient suivis d'une espace ou non, et avec ou sans guillemets.
func TestRealDovecotSearchOperatorWithFullAddress(t *testing.T) {
	p, e := dovecot(t)
	when := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	const addr = "pascalinejaouen@yahoo.fr"
	raw := "From: =?UTF-8?Q?Pascaline_Jaouen?= <" + addr + ">\r\nTo: moi@example.org\r\nCc: =?iso-8859-1?Q?St=E9phane?= <stephane@exemple.fr>\r\n" +
		"Subject: Facture octobre\r\nDate: " + when.Format(time.RFC1123Z) + "\r\nMessage-ID: <c06@test>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\ncorps\r\n"
	dv(t, p, e, func(c *Conn) error {
		if _, err := c.Append("INBOX", []byte(raw), "c06@test"); err != nil {
			return err
		}
		_, err := c.Append("INBOX", rawMsg(2, "autre@exemple.fr", "Autre", "corps", when.Add(time.Hour)), "c06b@test")
		return err
	})
	list := func(q string) int {
		var res *ListResult
		dv(t, p, e, func(c *Conn) (err error) {
			res, err = c.ListMessages(ListQuery{Folder: "INBOX", Page: 1, PageSize: 50, Search: q, Scope: "all"})
			return
		})
		return res.Total
	}
	for q, want := range map[string]int{
		addr: 1, "from:" + addr: 1, "from: " + addr: 1, `from:"` + addr + `"`: 1, `from: "` + addr + `"`: 1,
		"cc: stephane@exemple.fr": 1, "to: moi@example.org": 2, `subject: "Facture octobre"`: 1,
		"from: " + addr + " subject: Facture": 1, "from: autre@exemple.fr": 1, "from: inconnu@exemple.fr": 0,
	} {
		if got := list(q); got != want {
			t.Errorf("recherche %q : %d résultats, attendu %d", q, got, want)
		}
	}
}

func TestRealDovecotBulkMoveDoesNotBlock(t *testing.T) {
	p, e := dovecot(t)
	const n = 520
	var uids []uint32
	dv(t, p, e, func(c *Conn) error {
		if err := c.EnsureFolder("Trash"); err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			uid, err := c.Append("INBOX", rawMsg(i, "A <a@x.fr>", fmt.Sprintf("Lot %d", i), "x", time.Now()), "")
			if err != nil {
				return err
			}
			uids = append(uids, uid)
		}
		return nil
	})
	start := time.Now()
	dv(t, p, e, func(c *Conn) error { return c.Move("INBOX", uids[:500], "Trash") })
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("déplacement de 500 messages : %v", d)
	}
	var res *ListResult
	dv(t, p, e, func(c *Conn) (err error) {
		res, err = c.ListMessages(ListQuery{Folder: "Trash", Page: 1, PageSize: 10})
		return
	})
	if res.Total != 500 {
		t.Errorf("%d messages dans Trash (500 attendus)", res.Total)
	}
	// suppression définitive de 500 messages d'un coup
	start = time.Now()
	dv(t, p, e, func(c *Conn) error {
		var all []uint32
		r, err := c.ListMessages(ListQuery{Folder: "Trash", Page: 1, PageSize: 500})
		if err != nil {
			return err
		}
		for _, m := range r.Messages {
			all = append(all, m.UID)
		}
		return c.Expunge("Trash", all)
	})
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("suppression de 500 messages : %v", d)
	}
}

func TestRealDovecotThreads(t *testing.T) {
	p, e := dovecot(t)
	var uids []uint32
	base := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	msg := func(id, inReplyTo, subject string, i int) []byte {
		extra := ""
		if inReplyTo != "" {
			extra = "In-Reply-To: <" + inReplyTo + ">\r\nReferences: <" + inReplyTo + ">\r\n"
		}
		return []byte(fmt.Sprintf("From: a%d@x.fr\r\nTo: moi@example.org\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s>\r\n%sMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\ncorps\r\n",
			i, subject, base.Add(time.Duration(i)*time.Hour).Format(time.RFC1123Z), id, extra))
	}
	folder := "Fils"
	dv(t, p, e, func(c *Conn) error {
		if err := c.CreateFolder(folder); err != nil {
			return err
		}
		for i, m := range [][]byte{
			msg("t1@x", "", "Projet", 0), msg("t2@x", "t1@x", "Re: Projet", 1), msg("t3@x", "t2@x", "Re: Projet", 2),
			msg("solo@x", "", "Autre sujet", 3), msg("t4@x", "t3@x", "Re: Projet", 4),
		} {
			uid, err := c.Append(folder, m, fmt.Sprintf("m%d", i), imap.FlagSeen)
			if err != nil {
				return err
			}
			uids = append(uids, uid)
		}
		// le dernier message de la conversation n'est pas lu
		return c.StoreFlags(folder, []uint32{uids[4]}, imap.StoreFlagsDel, imap.FlagSeen)
	})
	list := func(q ListQuery) *ListResult {
		var res *ListResult
		dv(t, p, e, func(c *Conn) (err error) { res, err = c.ListMessages(q); return })
		return res
	}
	flat := list(ListQuery{Folder: folder, Page: 1, PageSize: 50, Sort: "date", Desc: true})
	if flat.Total != 5 {
		t.Fatalf("sans conversations : %d messages", flat.Total)
	}
	r := list(ListQuery{Folder: folder, Page: 1, PageSize: 50, Sort: "date", Desc: true, Threads: true})
	if r.ThreadsUnsupported || r.Total != 2 || len(r.Messages) != 2 {
		t.Fatalf("avec conversations : %+v", r)
	}
	// la conversation « Projet » (4 messages) est représentée par son message le plus récent, non lu
	top := r.Messages[0]
	if top.UID != uids[4] || len(top.ThreadUIDs) != 4 || top.Seen {
		t.Errorf("conversation : représentant %d, %d membres, lu=%v", top.UID, len(top.ThreadUIDs), top.Seen)
	}
	if r.Messages[1].Subject != "Autre sujet" || len(r.Messages[1].ThreadUIDs) != 0 {
		t.Errorf("message isolé : %+v", r.Messages[1])
	}
	// ouverture d'une conversation : ses seuls membres
	th := list(ListQuery{Folder: folder, Page: 1, PageSize: 50, Sort: "date", Desc: true, ThreadOf: uids[1]})
	if th.Total != 4 || th.Messages[0].UID != uids[4] {
		t.Errorf("membres de la conversation : %d", th.Total)
	}
	// la recherche réduit aussi les conversations
	if rs := list(ListQuery{Folder: folder, Page: 1, PageSize: 50, Sort: "date", Desc: true, Threads: true, Search: "Autre", Scope: "subject"}); rs.Total != 1 {
		t.Errorf("recherche avec conversations : %d", rs.Total)
	}
}

func TestRealDovecotImportantAndForwardedFlags(t *testing.T) {
	p, e := dovecot(t)
	when := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	dv(t, p, e, func(c *Conn) error {
		hi := strings.Replace(string(rawMsg(1, "a@x.fr", "Urgent", "corps", when)), "MIME-Version", "X-Priority: 1 (Highest)\r\nMIME-Version", 1)
		if _, err := c.Append("INBOX", []byte(hi), "it1@test", imap.FlagSeen, imap.Flag("$Forwarded")); err != nil {
			return err
		}
		_, err := c.Append("INBOX", rawMsg(2, "b@x.fr", "Normal", "corps", when.Add(time.Hour)), "it2@test", imap.FlagSeen)
		return err
	})
	var res *ListResult
	dv(t, p, e, func(c *Conn) (err error) {
		res, err = c.ListMessages(ListQuery{Folder: "INBOX", Page: 1, PageSize: 10, Sort: "date", Desc: true})
		return
	})
	if len(res.Messages) != 2 {
		t.Fatalf("%d messages", len(res.Messages))
	}
	normal, urgent := res.Messages[0], res.Messages[1]
	if !urgent.Important || normal.Important || !urgent.Forwarded || normal.Forwarded || len(urgent.To) != 1 || urgent.To[0].Email != "moi@example.org" {
		t.Errorf("urgent=%+v normal=%+v", urgent, normal)
	}
}

// Les pièces jointes listées d'après la structure IMAP doivent correspondre, dans le même ordre et avec les mêmes noms,
// à celles qu'on obtient en analysant le message entier : c'est ce qui permet de n'en télécharger qu'une.
func TestRealDovecotPartListMatchesFullParse(t *testing.T) {
	p, e := dovecot(t)
	body := "From: a@x.fr\r\nTo: moi@example.org\r\nSubject: Parties\r\nDate: Mon, 05 Oct 2026 09:00:00 +0000\r\nMessage-ID: <parts@test>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=M\r\n\r\n" +
		"--M\r\nContent-Type: multipart/alternative; boundary=A\r\n\r\n" +
		"--A\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\ncorps texte\r\n" +
		"--A\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n<p>corps html</p>\r\n--A--\r\n" +
		"--M\r\nContent-Type: image/png\r\nContent-ID: <logo@x>\r\nContent-Transfer-Encoding: base64\r\n\r\niVBORw0KGgo=\r\n" +
		"--M\r\nContent-Type: text/calendar; method=REQUEST\r\n\r\nBEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n" +
		"--M\r\nContent-Type: application/pdf; name=\"devis.pdf\"\r\nContent-Disposition: attachment; filename=\"devis.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQK\r\n" +
		"--M\r\nContent-Type: application/pkcs7-signature; name=smime.p7s\r\nContent-Transfer-Encoding: base64\r\n\r\nAAAA\r\n--M--\r\n"
	var uid uint32
	dv(t, p, e, func(c *Conn) (err error) {
		uid, err = c.Append("INBOX", []byte(body), "parts@test", imap.FlagSeen)
		return
	})
	var parts []mailview.PartDesc
	var full *mailview.Message
	var pdf []byte
	dv(t, p, e, func(c *Conn) error {
		var err error
		if parts, err = c.Structure("INBOX", uid); err != nil {
			return err
		}
		raw, err := c.FetchMessage("INBOX", uid, true, 0)
		if err != nil {
			return err
		}
		if full, err = mailview.Parse(raw.Raw); err != nil {
			return err
		}
		list := mailview.AttachmentParts(parts)
		for _, a := range list {
			if a.Name == "devis.pdf" {
				pdf, err = c.FetchPart("INBOX", uid, a.Path, a.Encoding, 0)
				return err
			}
		}
		return nil
	})
	got := mailview.AttachmentParts(parts)
	if len(got) != len(full.Attachments) {
		t.Fatalf("%d pièces d'après la structure, %d d'après le message : %+v / %+v", len(got), len(full.Attachments), got, full.Attachments)
	}
	for i, a := range got {
		if a.Name != full.Attachments[i].Name {
			t.Errorf("pièce %d : %q d'après la structure, %q d'après le message", i, a.Name, full.Attachments[i].Name)
		}
	}
	if string(pdf) != "%PDF-1.4\n" {
		t.Errorf("contenu de la pièce téléchargée seule : %q", pdf)
	}
}
