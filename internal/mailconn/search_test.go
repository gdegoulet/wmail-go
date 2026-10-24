package mailconn

import (
	"sort"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

func TestSearchOperators(t *testing.T) {
	c := searchCriteria(`from:bob subject:"rapport mensuel" is:unseen,flagged date:2026.10.05 size:2MB smaller:10KB budget`, "all", false)
	if len(c.Header) != 2 || c.Header[0].Key != "From" || c.Header[1].Value != "rapport mensuel" {
		t.Errorf("en-têtes : %+v", c.Header)
	}
	if len(c.NotFlag) != 2 || c.NotFlag[0] != imap.FlagDeleted || c.NotFlag[1] != imap.FlagSeen || len(c.Flag) != 1 || c.Flag[0] != imap.FlagFlagged {
		t.Errorf("drapeaux : %+v %+v", c.NotFlag, c.Flag)
	}
	if !c.Since.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)) || !c.Before.Equal(c.Since.AddDate(0, 0, 1)) {
		t.Errorf("date : %v %v", c.Since, c.Before)
	}
	if c.Larger != 2<<20 || c.Smaller != 10<<10 {
		t.Errorf("tailles : %d %d", c.Larger, c.Smaller)
	}
	if len(c.Text) != 0 || len(c.Or) != 1 {
		t.Errorf("le texte libre se cherche dans les en-têtes, pas dans le corps : %+v", c)
	}
	// sans opérateur : comportement historique
	if c := searchCriteria("simple recherche", "all", false); len(c.Text) != 0 || len(c.Body) != 0 || len(c.Or) != 1 {
		t.Errorf("recherche simple (De, À, Cc, Objet) : %+v", c)
	}
	// opérateur inconnu ou valeur invalide : traités comme du texte
	if c := searchCriteria("http://x.fr size:abc", "all", false); len(c.Or) != 1 || c.Larger != 0 {
		t.Errorf("valeur invalide : %+v", c)
	}
	if c := searchCriteria("has:attachment", "all", false); len(c.Or) != 1 {
		t.Errorf("pièce jointe : %+v", c)
	}
	// plage de dates ouverte
	if c := searchCriteria("date:2026.01.01/2026.01.31", "all", false); c.Since.IsZero() || !c.Before.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("plage : %v %v", c.Since, c.Before)
	}
}

// C-06 : « from: adresse@exemple.fr » (une espace après les deux-points) ne renvoyait rien, car le mot-clé « from: »
// et l'adresse repartaient en texte libre : la recherche portait alors sur la chaîne entière, en-tête From compris.
func TestSearchOperatorValueAfterSpace(t *testing.T) {
	const addr = "pascalinejaouen@yahoo.fr"
	header := func(c *imap.SearchCriteria, key string) string {
		for _, h := range c.Header {
			if h.Key == key {
				return h.Value
			}
		}
		return ""
	}
	for _, q := range []string{
		"from:" + addr, "from: " + addr, `from:"` + addr + `"`, `from: "` + addr + `"`, "FROM: " + addr,
	} {
		c := searchCriteria(q, "all", false)
		if header(c, "From") != addr || len(c.Or) != 0 || len(c.Text) != 0 || len(c.Body) != 0 {
			t.Errorf("%q : en-têtes %+v, Or %+v", q, c.Header, c.Or)
		}
	}
	// les autres mots-clés acceptent aussi l'espace et une adresse complète
	if c := searchCriteria("cc: "+addr, "all", false); header(c, "Cc") != addr {
		t.Errorf("cc : %+v", c.Header)
	}
	if c := searchCriteria(`subject: "Facture 2026"`, "all", false); header(c, "Subject") != "Facture 2026" {
		t.Errorf("subject : %+v", c.Header)
	}
	// « to: » cherche dans À ou Cc : un seul Or, sans texte libre résiduel
	c := searchCriteria("to: "+addr, "all", false)
	if len(c.Or) != 1 || len(c.Header) != 0 || c.Or[0][0].Header[0].Value != addr || c.Or[0][1].Header[0].Value != addr {
		t.Errorf("to : %+v", c)
	}
	// combinaison : deux mots-clés séparés par une espace après les deux-points
	c = searchCriteria("from: "+addr+" subject: Facture", "all", false)
	if header(c, "From") != addr || header(c, "Subject") != "Facture" || len(c.Or) != 0 {
		t.Errorf("combinaison : %+v %+v", c.Header, c.Or)
	}
	// un mot-clé sans valeur, une valeur invalide ou un mot-clé inconnu restent du texte libre
	for _, q := range []string{"from:", "size: abc", "truc: machin"} {
		if c := searchCriteria(q, "all", false); len(c.Header) != 0 || c.Larger != 0 || len(c.Or) != 1 {
			t.Errorf("%q : %+v", q, c)
		}
	}
	if c := searchCriteria("size: 2MB", "all", false); c.Larger != 2<<20 || len(c.Or) != 0 {
		t.Errorf("size avec espace : %+v", c)
	}
}

func TestHighPriority(t *testing.T) {
	for in, want := range map[string]bool{
		"Importance: high\r\n\r\n":                    true,
		"X-Priority: 1 (Highest)\r\n\r\n":             true,
		"x-priority: 2\r\n\r\n":                       true,
		"X-MSMail-Priority: High\r\n\r\n":             true,
		"Importance: normal\r\nX-Priority: 3\r\n\r\n": false,
		"X-Priority: 5\r\n\r\n":                       false,
		"":                                            false,
	} {
		if got := highPriority([]byte(in)); got != want {
			t.Errorf("%q : %v", in, got)
		}
	}
}

func TestAttachmentKind(t *testing.T) {
	for _, c := range []struct{ mime, name, want string }{
		{"image/png", "a.png", "image"}, {"application/pdf", "x.bin", "pdf"}, {"application/octet-stream", "Devis.PDF", "pdf"},
		{"application/zip", "a.zip", "archive"}, {"application/vnd.ms-excel", "t.xls", "tableur"}, {"text/plain", "n.txt", "texte"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "p.pptx", "presentation"},
		{"application/octet-stream", "inconnu.xyz", ""}, {"audio/mpeg", "a.mp3", "audio"},
	} {
		if got := attachmentKind(c.mime, c.name); got != c.want {
			t.Errorf("%s %s : %q, attendu %q", c.mime, c.name, got, c.want)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	in := []string{"Projet 10", "projet 2", "Projet", "Archive 2026", "archive 9", "A1b", "A10b"}
	want := []string{"A1b", "A10b", "archive 9", "Archive 2026", "Projet", "projet 2", "Projet 10"}
	sort.SliceStable(in, func(i, j int) bool { return NaturalLess(in[i], in[j]) })
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("ordre : %v", in)
		}
	}
}
