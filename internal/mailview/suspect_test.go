package mailview

import (
	"slices"
	"strings"
	"testing"
)

// msg monte un message minimal pour l'analyse.
func msg(from, name, html, text string, unsub []string, dkim string) *Message {
	return &Message{
		From:            []Address{{Name: name, Email: from}},
		HTML:            html,
		Text:            text,
		ListUnsubscribe: unsub,
		DKIM:            dkim,
	}
}

// Le cas qui a motivé la fonction : un faux avis de renouvellement passé au travers d'un filtre serveur
// parce qu'il est authentifié (SPF, DKIM et DMARC valides) et ne cite aucune marque. Seule sa facture
// technique le trahit : il traque ses ouvertures, n'offre pas de désabonnement, n'a pas de partie texte,
// et charge son en-tête depuis un domaine nu.
func TestSuspicionsFauxAvisAuthentifie(t *testing.T) {
	html := `<table><tr><td style="font-size:0"><img src="https://d1v2u3h27yevyl.cloudfront.net" width="520" alt=""></td></tr>
<tr><td><p>Votre accès est prolongé d'un an</p>
<a href="https://u3960513.ct.sendgrid.net/ls/click?upn=u001.NDNg2YlskgvjysBN2fDQJUOp-2FqDdbFH7MwSj7RX3Ro29UOU9J8vvxRgF9qaqLOnUDKWa">Consulter mon espace</a></td></tr>
<tr><td><img src="https://u3960513.ct.sendgrid.net/wf/open?upn=u001.qFYK1fC4YXrXkBDjyAT0REHAO7hPEBAatVckpCuxCz-2FNHJDuS4szkoiknfidTQK4QRqP8"></td></tr></table>`
	m := msg("admin@clientes.partidasya.com", "message", html, "", nil, "pass")

	got := m.Suspicions()
	for _, want := range []string{SuspBulkNoUnsub, SuspHTMLOnly, SuspBareHostImage} {
		if !slices.Contains(got, want) {
			t.Errorf("motif %q attendu, relevés : %v", want, got)
		}
	}
	// Le libellé « Consulter mon espace » ne promet aucun domaine : pas de lien trompeur à signaler.
	if slices.Contains(got, SuspLinkMismatch) {
		t.Errorf("lien trompeur signalé à tort : %v", got)
	}
}

// Une lettre d'information ordinaire traque aussi ses lecteurs, mais offre un désabonnement et une partie
// texte : elle ne doit rien déclencher. C'est le faux positif à ne pas commettre.
func TestSuspicionsLettreLegitime(t *testing.T) {
	html := `<p>Nos nouveautés</p><img src="https://img.example.com/entete/banniere.png">
<a href="https://trk.example.com/c?id=0123456789012345678901234567890123456789012345">Voir en ligne</a>`
	m := msg("news@example.com", "Example", html, "Nos nouveautés\nVoir en ligne", []string{"https://example.com/unsub"}, "pass")
	if got := m.Suspicions(); got != nil {
		t.Errorf("aucun motif attendu, relevés : %v", got)
	}
}

// Un message transactionnel sans artifice : HTML seul, mais rien d'autre. Un motif faible isolé reste
// sous le seuil et n'affiche aucun bandeau.
func TestSuspicionsMotifFaibleIsole(t *testing.T) {
	m := msg("facture@example.com", "Example", `<p>Votre facture</p><a href="https://example.com/f/42">Télécharger</a>`, "", nil, "pass")
	if got := m.Suspicions(); got != nil {
		t.Errorf("un motif faible isolé ne doit rien déclencher, relevés : %v", got)
	}
}

func TestSuspicionsLienTrompeur(t *testing.T) {
	html := `<p>Vérifiez votre compte : <a href="https://compte-verif.example.ru/login">www.mabanque.fr</a></p>`
	m := msg("info@mabanque.fr", "Ma Banque", html, "Vérifiez votre compte", nil, "pass")
	got := m.Suspicions()
	if !slices.Contains(got, SuspLinkMismatch) {
		t.Fatalf("lien trompeur attendu, relevés : %v", got)
	}
	// Un motif fort suffit : le bandeau s'affiche sans rien d'autre.
	if len(got) == 0 {
		t.Error("le seuil devrait être atteint par le seul lien trompeur")
	}
}

// Le même domaine écrit avec un sous-domaine n'est pas un lien trompeur.
func TestSuspicionsLienSousDomaine(t *testing.T) {
	html := `<p><a href="https://www.example.com/page">example.com</a></p>`
	m := msg("info@example.com", "Example", html, "texte", []string{"https://example.com/u"}, "pass")
	if got := m.Suspicions(); slices.Contains(got, SuspLinkMismatch) {
		t.Errorf("un sous-domaine du même site n'est pas trompeur : %v", got)
	}
}

func TestSuspicionsPunycode(t *testing.T) {
	html := `<p><a href="https://xn--mabanqu-9za.fr/login">Accéder</a></p><a href="https://xn--mabanqu-9za.fr/x">suite</a>`
	m := msg("info@exemple.test", "Exemple", html, "", nil, "pass")
	if got := m.Suspicions(); !slices.Contains(got, SuspPunycode) {
		t.Errorf("punycode attendu, relevés : %v", got)
	}
}

func TestSuspicionsNomAffichéTrompeur(t *testing.T) {
	m := msg("x7f2@envoi-masse.test", "Service client <contact@mabanque.fr>", `<p>Bonjour</p>`, "Bonjour", nil, "pass")
	got := m.Suspicions()
	if !slices.Contains(got, SuspNameEmail) {
		t.Fatalf("nom affiché trompeur attendu, relevés : %v", got)
	}
}

// Le nom affiché peut légitimement contenir l'adresse réelle : ce n'est pas trompeur.
func TestSuspicionsNomAffichéHonnête(t *testing.T) {
	m := msg("contact@example.com", "contact@example.com", `<p>Bonjour</p>`, "Bonjour", nil, "pass")
	if got := m.Suspicions(); slices.Contains(got, SuspNameEmail) {
		t.Errorf("le nom reprend l'adresse réelle : %v", got)
	}
}

// Les textes de lien ordinaires ne doivent jamais être pris pour une promesse de domaine.
func TestTextePromesseDeDomaine(t *testing.T) {
	cas := []struct {
		texte, hôte string
		veut        bool
	}{
		{"www.mabanque.fr", "pirate.test", true},
		{"https://mabanque.fr/compte", "pirate.test", true},
		{"example.com", "www.example.com", false},
		{"Consulter mon espace", "pirate.test", false},
		{"Cliquez ici", "pirate.test", false},
		{"contact@example.com", "pirate.test", false},
		{"", "pirate.test", false},
		{"2.50", "pirate.test", false},
	}
	for _, c := range cas {
		if got := textPromisesOtherHost(c.texte, c.hôte); got != c.veut {
			t.Errorf("textPromisesOtherHost(%q, %q) = %v, attendu %v", c.texte, c.hôte, got, c.veut)
		}
	}
}

// Un corps sans HTML ne doit pas faire planter l'analyse, ni rien relever.
func TestSuspicionsTexteSeul(t *testing.T) {
	m := msg("ami@example.com", "Ami", "", "Salut, on se voit demain ?", nil, "pass")
	if got := m.Suspicions(); got != nil {
		t.Errorf("un message en texte brut ne doit rien relever : %v", got)
	}
}

// Du HTML abîmé ne doit ni paniquer ni boucler.
func TestSuspicionsHTMLAbime(t *testing.T) {
	m := msg("x@example.com", "X", `<a href="https://a.test/x">texte<img src="<<>`+strings.Repeat("<a", 50), "", nil, "pass")
	_ = m.Suspicions()
}
