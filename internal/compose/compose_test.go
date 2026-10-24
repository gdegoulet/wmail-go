package compose

import (
	"net/mail"
	"strings"
	"testing"
	"time"

	"wmail-go/internal/mailview"
)

func addrs(t *testing.T, s string) []*mail.Address {
	t.Helper()
	l, err := ParseList(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestParseList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a@b.fr", []string{"a@b.fr"}},
		{"a@b.fr, c@d.fr", []string{"a@b.fr", "c@d.fr"}},
		{"a@b.fr; c@d.fr\nE <e@f.fr>", []string{"a@b.fr", "c@d.fr", "e@f.fr"}},
		{`"Doe, John" <j@d.fr>, x@y.fr`, []string{"j@d.fr", "x@y.fr"}},
		{" ,, a@b.fr ,", []string{"a@b.fr"}},
	}
	for _, c := range cases {
		got := addrs(t, c.in)
		var emails []string
		for _, a := range got {
			emails = append(emails, a.Address)
		}
		if strings.Join(emails, "|") != strings.Join(c.want, "|") {
			t.Errorf("ParseList(%q) = %v, attendu %v", c.in, emails, c.want)
		}
	}
	for _, bad := range []string{"pas une adresse", "a@b.fr\x00", "a@b.fr, <>", "a@"} {
		if _, err := ParseList(bad); err == nil {
			t.Errorf("ParseList(%q) devrait échouer", bad)
		}
	}
}

func TestReplySubject(t *testing.T) {
	cases := []struct{ prefix, in, want string }{
		{"Re", "Bonjour", "Re: Bonjour"},
		{"Re", "Re: Bonjour", "Re: Bonjour"},
		{"Re", "RE: Re:  Bonjour", "Re: Bonjour"},
		{"Re", "Re[2]: Bonjour", "Re: Bonjour"},
		{"Re", "Fwd: Bonjour", "Re: Bonjour"},
		{"Fwd", "Re: Bonjour", "Fwd: Bonjour"},
		{"Fwd", "Fwd: Fwd: Bonjour", "Fwd: Bonjour"},
		{"Re", "", "Re:"},
		{"Re", "Résumé: du jour", "Re: Résumé: du jour"},
		{"Re", "Reunion: lundi", "Re: Reunion: lundi"},
	}
	for _, c := range cases {
		if got := ReplySubject(c.prefix, c.in); got != c.want {
			t.Errorf("ReplySubject(%q, %q) = %q, attendu %q", c.prefix, c.in, got, c.want)
		}
	}
}

func emails(l []*mail.Address) string {
	var out []string
	for _, a := range l {
		out = append(out, a.Address)
	}
	return strings.Join(out, ",")
}

func TestReplyRecipients(t *testing.T) {
	me := map[string]bool{"moi@x.fr": true}
	// Reply-To prioritaire
	got := ReplyRecipients(addrs(t, "liste@x.org"), addrs(t, "a@b.fr"), me)
	if emails(got) != "liste@x.org" {
		t.Errorf("Reply-To : %s", emails(got))
	}
	// à défaut, From
	if got := ReplyRecipients(nil, addrs(t, "a@b.fr"), me); emails(got) != "a@b.fr" {
		t.Errorf("From : %s", emails(got))
	}
	// réponse à soi-même : on recalcule sans exclusion
	if got := ReplyRecipients(nil, addrs(t, "Moi@X.fr"), me); emails(got) != "Moi@X.fr" {
		t.Errorf("à soi-même : %s", emails(got))
	}
}

func TestReplyAll(t *testing.T) {
	me := map[string]bool{"moi@x.fr": true}
	to, cc := ReplyAll(nil, addrs(t, "a@b.fr"), addrs(t, "moi@x.fr, c@d.fr, A@B.fr"), addrs(t, "e@f.fr, c@d.fr, moi@x.fr"), me)
	if emails(to) != "a@b.fr,c@d.fr" || emails(cc) != "e@f.fr" {
		t.Errorf("répondre à tous : À=%s Cc=%s", emails(to), emails(cc))
	}
}

func TestQuote(t *testing.T) {
	if got := Quote("a\nb\n\n> c\n"); got != "> a\n> b\n> \n>> c" {
		t.Errorf("Quote = %q", got)
	}
}

func TestReferences(t *testing.T) {
	got := References("<m3@x>", []string{"<m1@x>", "m2@x", "m1@x"})
	if strings.Join(got, ",") != "m1@x,m2@x,m3@x" {
		t.Errorf("References = %v", got)
	}
	var many []string
	for i := 0; i < 40; i++ {
		many = append(many, "id"+string(rune('a'+i%26))+string(rune('a'+i/26))+"@x")
	}
	if got := References("last@x", many); len(got) != 20 || got[0] != many[0] || got[19] != "last@x" {
		t.Errorf("limite : %d %v", len(got), got)
	}
}

func baseMessage(t *testing.T) *Message {
	return &Message{
		From:    &mail.Address{Name: "Gérard Dégoulet", Address: "g@sdv.fr"},
		To:      addrs(t, "a@b.fr, Zoë <z@d.fr>"),
		Cc:      addrs(t, "c@d.fr"),
		Bcc:     addrs(t, "secret@d.fr"),
		Subject: "Réunion à 14 h – ordre du jour",
		Text:    "Bonjour,\nÀ tout à l'heure. Voilà une ligne très longue " + strings.Repeat("é", 120) + "\n",
		Date:    time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
}

func TestBuildRoundTrip(t *testing.T) {
	m := baseMessage(t)
	m.InReplyTo = "<orig@x>"
	m.References = []string{"r1@x", "orig@x"}
	m.XMailer = "wmail-go"
	m.Important = true
	m.ReadReceipt = true
	m.Draft = &DraftInfo{Kind: "reply", UID: 42, Folder: "INBOX/Été"}
	raw, err := m.Build(false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "Bcc:") || strings.Contains(s, "secret@d.fr") {
		t.Errorf("le Cci ne doit pas apparaître dans le message envoyé :\n%s", s)
	}
	for _, want := range []string{"In-Reply-To: <orig@x>", "References: <r1@x> <orig@x>", "X-Mailer: wmail-go", "Importance: High", "Disposition-Notification-To:", "Mime-Version: 1.0", "Content-Transfer-Encoding: quoted-printable", "Message-Id: <"} {
		if !strings.Contains(s, want) {
			t.Errorf("en-tête manquant %q :\n%s", want, s)
		}
	}
	parsed, err := mailview.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject != m.Subject {
		t.Errorf("objet = %q", parsed.Subject)
	}
	if !strings.Contains(parsed.Text, "À tout à l'heure") || !strings.Contains(parsed.Text, strings.Repeat("é", 120)) {
		t.Errorf("texte altéré : %q", parsed.Text)
	}
	if len(parsed.From) != 1 || parsed.From[0].Name != "Gérard Dégoulet" || len(parsed.To) != 2 || parsed.To[1].Name != "Zoë" {
		t.Errorf("adresses : %+v %+v", parsed.From, parsed.To)
	}
	for _, line := range strings.Split(s, "\r\n") {
		if len(line) > 998 {
			t.Errorf("ligne trop longue (%d)", len(line))
		}
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Error("fins de ligne autres que CRLF")
	}
	// copie pour « Envoyés » : avec Cci
	withBcc, err := m.Build(true)
	if err != nil || !strings.Contains(string(withBcc), "Bcc: <secret@d.fr>") {
		t.Errorf("la copie doit contenir le Cci : %v", err)
	}
}

func TestBuildAttachments(t *testing.T) {
	m := baseMessage(t)
	m.Attachments = []Attachment{
		{Name: "../../etc/passwd", ContentType: "text/plain", Data: []byte("secret\r\n")},
		{Name: "photo é.png", ContentType: "image/png\r\nBcc: x@y.z", Data: []byte{0, 1, 2, 3, 255}},
		{Name: "m.eml", ContentType: "message/rfc822", Data: []byte("Subject: x\r\n\r\ny\r\n")},
	}
	raw, err := m.Build(false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Bcc: x@y.z") {
		t.Fatal("injection d'en-tête par le type de la pièce jointe")
	}
	p, err := mailview.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Attachments) != 3 {
		t.Fatalf("%d pièces jointes", len(p.Attachments))
	}
	if p.Attachments[0].Name != "passwd" {
		t.Errorf("nom non assaini : %q", p.Attachments[0].Name)
	}
	if p.Attachments[1].Name != "photo é.png" {
		t.Errorf("nom accentué : %q", p.Attachments[1].Name)
	}
	_, data, _ := p.Attachment(1)
	if string(data) != string([]byte{0, 1, 2, 3, 255}) {
		t.Errorf("contenu binaire altéré : %v", data)
	}
	if !strings.Contains(p.Text, "Bonjour") {
		t.Error("texte absent")
	}
}

func TestBuildHeaderInjection(t *testing.T) {
	m := baseMessage(t)
	m.Subject = "Salut\r\nBcc: pirate@evil.example"
	m.InReplyTo = "x@y>\r\nBcc: pirate@evil.example"
	m.XMailer = "a\r\nBcc: pirate@evil.example"
	raw, err := m.Build(false)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.SplitN(string(raw), "\r\n\r\n", 2)[0]
	for _, line := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "bcc:") {
			t.Fatalf("en-tête injecté : %q", line)
		}
	}
	if _, err := ParseList("a@b.fr\r\nBcc: pirate@evil.example"); err == nil {
		// deux adresses séparées par une fin de ligne : la seconde n'est pas une adresse valide
		t.Error("une ligne parasite doit être refusée")
	}
	m2 := baseMessage(t)
	m2.From = &mail.Address{Name: "x\r\nBcc: p@e.x", Address: "g@sdv.fr"}
	if _, err := m2.Build(false); err == nil {
		t.Error("expéditeur avec contrôle accepté")
	}
}

func TestRecipients(t *testing.T) {
	m := baseMessage(t)
	m.To = addrs(t, "a@b.fr, A@B.fr")
	if got := strings.Join(m.Recipients(), ","); got != "a@b.fr,c@d.fr,secret@d.fr" {
		t.Errorf("destinataires = %s", got)
	}
}

func TestDraftInfoRoundTrip(t *testing.T) {
	for _, d := range []DraftInfo{{"reply", 7, "INBOX"}, {"forward", 123456, "Dossiers/Été \"x\"; y"}} {
		got, ok := ParseDraftInfo(EncodeDraftInfo(d))
		if !ok || got != d {
			t.Errorf("aller-retour %+v → %+v (%v)", d, got, ok)
		}
	}
	for _, bad := range []string{"", "reply", "reply; uid=0; folder=41", "evil; uid=1; folder=41", "reply; uid=1; folder=zz", "reply; uid=1; folder=0a41"} {
		if _, ok := ParseDraftInfo(bad); ok {
			t.Errorf("%q devrait être refusé", bad)
		}
	}
}

func TestNewMessageID(t *testing.T) {
	a, b := NewMessageID("g@sdv.fr"), NewMessageID("g@sdv.fr")
	if a == b || !strings.HasSuffix(a, "@sdv.fr") {
		t.Errorf("identifiants : %s %s", a, b)
	}
}

func TestMDNBuild(t *testing.T) {
	m := MDN{
		From: &mail.Address{Name: "Moi", Address: "moi@sdv.fr"}, To: &mail.Address{Address: "expediteur@example.org"},
		OrigSubject: "Réunion\r\nBcc: pirate@evil.example", OrigID: "orig@x", Text: "Votre message a été affiché.\nCeci ne prouve pas qu'il a été lu.",
		SubjectLabel: "Accusé de lecture :", XMailer: "wmail-go",
	}
	raw, err := m.Build()
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"multipart/report", "report-type=disposition-notification", "message/disposition-notification", "Final-Recipient: rfc822; moi@sdv.fr",
		"Original-Message-ID: <orig@x>", "Disposition: manual-action/MDN-sent-manually; displayed", "Auto-Submitted: auto-replied", "To: <expediteur@example.org>"} {
		if !strings.Contains(s, want) {
			t.Errorf("manque %q :\n%s", want, s)
		}
	}
	head := strings.SplitN(s, "\r\n\r\n", 2)[0]
	for _, line := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "bcc:") {
			t.Fatalf("en-tête injecté par l'objet : %q", line)
		}
	}
	p, err := mailview.Parse(raw)
	if err != nil || !strings.Contains(p.Text, "Votre message a été affiché") {
		t.Errorf("relecture : %v %q", err, p.Text)
	}
	if _, err := (MDN{From: m.From, To: &mail.Address{Address: "a@b.fr\r\nBcc: x@y.z"}}).Build(); err == nil {
		t.Error("adresse avec retour à la ligne acceptée")
	}
	if _, err := (MDN{From: m.From}).Build(); err == nil {
		t.Error("destinataire manquant accepté")
	}
}

func TestParseListAcceptsLooseFormsAndIDN(t *testing.T) {
	list, err := ParseList("Jean Dupont jean@x.fr, pierre@y.fr (Pierre Martin), 'Zoé' <zoe@z.fr>, bob@b.fr.")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 {
		t.Fatalf("%d adresses : %v", len(list), FormatList(list))
	}
	for i, want := range []struct{ name, addr string }{
		{"Jean Dupont", "jean@x.fr"}, {"Pierre Martin", "pierre@y.fr"}, {"Zoé", "zoe@z.fr"}, {"", "bob@b.fr"},
	} {
		if list[i].Name != want.name || list[i].Address != want.addr {
			t.Errorf("%d : %q <%s>, attendu %q <%s>", i, list[i].Name, list[i].Address, want.name, want.addr)
		}
	}
	if _, err := ParseList("vraiment pas une adresse"); err == nil {
		t.Error("texte sans adresse accepté")
	}
	if got := ToASCIIDomain("zoé@café.fr"); got != "zoé@xn--caf-dma.fr" {
		t.Errorf("punycode : %s", got)
	}
	if got := ToASCIIDomain("a@b.fr"); got != "a@b.fr" {
		t.Errorf("domaine ASCII modifié : %s", got)
	}
}

func TestIDNInEnvelopeAndHeaders(t *testing.T) {
	m := &Message{From: &mail.Address{Name: "Zoé", Address: "zoe@café.fr"}, To: []*mail.Address{{Address: "ami@déjà.fr"}}, Subject: "x", Text: "y"}
	if r := m.Recipients(); len(r) != 1 || r[0] != "ami@xn--dj-kia8a.fr" {
		t.Errorf("enveloppe : %v", r)
	}
	raw, err := m.Build(false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "xn--caf-dma.fr") || !strings.Contains(string(raw), "xn--dj-kia8a.fr") {
		t.Errorf("en-têtes sans punycode :\n%s", raw)
	}
}
