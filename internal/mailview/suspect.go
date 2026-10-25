package mailview

import (
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Analyse structurelle d'un message, destinée à un bandeau d'avertissement dans le lecteur.
//
// Le principe : ne rien juger du *contenu* — ni marque, ni langue, ni mot-clé — mais relever les
// incohérences entre ce qu'un message prétend être et sa facture technique. Un avis transactionnel qui
// traque ses ouvertures sans offrir de désabonnement, dont le corps n'existe qu'en HTML et dont les images
// viennent d'un domaine nu, n'est pas ce qu'il prétend être.
//
// Ce que cela n'est pas : un antispam. Il n'y a ici ni réputation, ni base bayésienne, ni empreinte, ni
// requête réseau. Un serveur de filtrage fait tout cela bien mieux. L'intérêt de ces contrôles est d'être
// faits *côté client*, sur ce que l'utilisateur a réellement sous les yeux, et de rester muets sur la
// langue du message.
//
// Limite à garder en tête : un attaquant soigneux contourne ces contrôles en quelques minutes (ajouter un
// en-tête List-Unsubscribe et une partie texte coûte deux lignes). Ils relèvent la barre, ils ne la ferment
// pas. D'où un bandeau d'information, jamais un classement automatique.

// Motifs relevés. Les valeurs servent de clés de traduction (« mail.susp.<motif> »).
const (
	// SuspBulkNoUnsub : le message traque ses lecteurs comme du publipostage, mais n'offre aucun
	// désabonnement. Les envois légitimes en nombre portent un List-Unsubscribe (et les règles des grands
	// fournisseurs l'exigent depuis 2024).
	SuspBulkNoUnsub = "bulk_no_unsub"
	// SuspHTMLOnly : aucune partie text/plain. Les moteurs de gabarit des envois en nombre en produisent
	// une systématiquement ; un corps HTML seul trahit un message composé à la main.
	SuspHTMLOnly = "html_only"
	// SuspBareHostImage : image chargée depuis un domaine nu, sans chemin. Un hébergement d'images sert
	// des fichiers, pas une racine.
	SuspBareHostImage = "bare_host_image"
	// SuspLinkMismatch : le texte du lien annonce un domaine, l'adresse mène ailleurs.
	SuspLinkMismatch = "link_mismatch"
	// SuspPunycode : domaine en punycode (xn--), par lequel on imite un nom connu avec d'autres alphabets.
	SuspPunycode = "punycode"
	// SuspNameEmail : le nom affiché de l'expéditeur contient une adresse d'un autre domaine que la sienne.
	SuspNameEmail = "name_email"
	// SuspDKIM : le serveur de réception n'a pas validé la signature du domaine expéditeur.
	SuspDKIM = "dkim"
)

// poids de chaque motif ; le bandeau s'affiche à partir de suspThreshold.
var suspWeight = map[string]int{
	SuspBulkNoUnsub:   2,
	SuspHTMLOnly:      1,
	SuspBareHostImage: 2,
	SuspLinkMismatch:  3,
	SuspPunycode:      2,
	SuspNameEmail:     3,
	SuspDKIM:          1,
}

// suspThreshold : score à partir duquel le bandeau s'affiche. Trois points demandent soit un motif fort
// (lien trompeur, nom d'expéditeur trompeur), soit deux motifs faibles concordants. Un seul motif faible
// — un corps sans partie texte, par exemple — ne déclenche rien : c'est trop courant pour alerter.
const suspThreshold = 3

// trackingQueryLen : longueur de chaîne de requête à partir de laquelle une adresse ressemble à un lien de
// traçage. Les identifiants de campagne sont longs et opaques ; un lien ordinaire ne l'est pas. Le seuil
// évite d'avoir à tenir une liste de prestataires, qui serait toujours en retard.
const trackingQueryLen = 40

// Suspicions relève les motifs structurels douteux d'un message. La liste est ordonnée (ordre des
// constantes), vide si le score reste sous le seuil : l'appelant n'a pas à connaître la pondération.
func (m *Message) Suspicions() []string {
	found := map[string]bool{}
	from := fromDomain(m.From)

	if m.DKIM == "warn" {
		found[SuspDKIM] = true
	}
	if m.Text == "" && m.HTML != "" {
		found[SuspHTMLOnly] = true
	}
	if d := nameDomain(m.From); d != "" && from != "" && !sameSite(d, from) {
		found[SuspNameEmail] = true
	}

	tracked := false
	if m.HTML != "" {
		scanHTML(m.HTML, from, found, &tracked)
	}
	// Traçage sans désabonnement : les deux faits pris ensemble, jamais séparément.
	if tracked && len(m.ListUnsubscribe) == 0 {
		found[SuspBulkNoUnsub] = true
	}

	total := 0
	for motif := range found {
		total += suspWeight[motif]
	}
	if total < suspThreshold {
		return nil
	}
	out := make([]string, 0, len(found))
	for _, motif := range []string{SuspBulkNoUnsub, SuspHTMLOnly, SuspBareHostImage, SuspLinkMismatch, SuspPunycode, SuspNameEmail, SuspDKIM} {
		if found[motif] {
			out = append(out, motif)
		}
	}
	return out
}

// scanHTML parcourt le corps une seule fois : images et liens relèvent des mêmes contrôles d'adresse.
func scanHTML(body, from string, found map[string]bool, tracked *bool) {
	z := xhtml.NewTokenizer(strings.NewReader(body))
	inLink := false
	var linkHost, linkText string
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch atom.Lookup(name) {
			case atom.Img:
				u := parseAttr(z, hasAttr, "src")
				if u == nil {
					continue
				}
				if u.Path == "" || u.Path == "/" {
					found[SuspBareHostImage] = true
				}
				if isTracking(u, from) {
					*tracked = true
				}
			case atom.A:
				u := parseAttr(z, hasAttr, "href")
				inLink, linkText = true, ""
				linkHost = ""
				if u == nil {
					continue
				}
				linkHost = strings.ToLower(u.Hostname())
				if strings.Contains(linkHost, "xn--") {
					found[SuspPunycode] = true
				}
				if isTracking(u, from) {
					*tracked = true
				}
			}
		case xhtml.EndTagToken:
			name, _ := z.TagName()
			if atom.Lookup(name) == atom.A {
				if inLink && linkHost != "" && textPromisesOtherHost(linkText, linkHost) {
					found[SuspLinkMismatch] = true
				}
				inLink, linkText, linkHost = false, "", ""
			}
		case xhtml.TextToken:
			if inLink && len(linkText) < 300 {
				linkText += string(z.Text())
			}
		}
	}
}

// parseAttr lit un attribut de la balise courante et rend l'adresse analysée ; nil si l'attribut manque,
// n'est pas analysable, ou ne désigne pas une ressource distante (data:, cid:, mailto:…).
func parseAttr(z *xhtml.Tokenizer, hasAttr bool, want string) *url.URL {
	var raw string
	for hasAttr {
		var k, v []byte
		k, v, hasAttr = z.TagAttr()
		if string(k) == want {
			raw = strings.TrimSpace(string(v))
		}
	}
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil
	}
	return u
}

// isTracking reconnaît une adresse de traçage : un hôte autre que celui de l'expéditeur, et une chaîne de
// requête longue et opaque. Pas de liste de prestataires : elle serait toujours en retard d'un nom.
func isTracking(u *url.URL, from string) bool {
	h := strings.ToLower(u.Hostname())
	if h == "" || (from != "" && sameSite(h, from)) {
		return false
	}
	return len(u.RawQuery) >= trackingQueryLen
}

// textPromisesOtherHost dit si le texte visible d'un lien annonce un domaine qui n'est pas celui où il
// mène. On n'examine que les textes qui ressemblent vraiment à une adresse : un libellé ordinaire
// (« Consulter mon espace ») ne promet rien et ne doit donc rien déclencher.
func textPromisesOtherHost(text, host string) bool {
	t := strings.TrimSpace(strings.ToLower(text))
	if t == "" || strings.ContainsAny(t, " \t\n\r") {
		return false
	}
	t = strings.TrimPrefix(strings.TrimPrefix(t, "https://"), "http://")
	t = strings.TrimSuffix(t, ".")
	if i := strings.IndexAny(t, "/?#"); i >= 0 {
		t = t[:i]
	}
	if at := strings.LastIndexByte(t, '@'); at >= 0 { // une adresse électronique ne promet pas de destination
		return false
	}
	if !strings.Contains(t, ".") || len(t) < 4 {
		return false
	}
	// Le texte doit être un nom d'hôte plausible, sinon ce n'est pas une promesse d'adresse.
	for _, r := range t {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '.' && r != '-' {
			return false
		}
	}
	// Dernier segment fait de lettres seules : sans cela « 2.50 » ou « 1.2.3 » passeraient pour des
	// domaines, et le moindre prix dans un lien déclencherait l'alerte.
	tld := t[strings.LastIndexByte(t, '.')+1:]
	if len(tld) < 2 || len(tld) > 24 {
		return false
	}
	for _, r := range tld {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return !sameSite(t, host)
}

// sameSite compare deux hôtes en tolérant les sous-domaines : « ct.example.com » et « example.com » sont
// du même ressort. Comparaison volontairement simple : elle ne connaît pas la liste des suffixes publics,
// ce qui la rend permissive, donc silencieuse plutôt que bavarde.
func sameSite(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSuffix(a, ".")), strings.ToLower(strings.TrimSuffix(b, "."))
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

// fromDomain rend le domaine de l'expéditeur, vide s'il n'y en a pas exactement un.
func fromDomain(from []Address) string {
	if len(from) != 1 {
		return ""
	}
	at := strings.LastIndexByte(from[0].Email, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(from[0].Email[at+1:])
}

// nameDomain rend le domaine d'une adresse écrite dans le *nom affiché* de l'expéditeur. C'est le procédé
// classique : « Service client <contact@example.org> » affiché par un client qui ne montre que le nom.
func nameDomain(from []Address) string {
	if len(from) != 1 {
		return ""
	}
	name := from[0].Name
	at := strings.LastIndexByte(name, '@')
	if at < 0 || at == len(name)-1 {
		return ""
	}
	d := name[at+1:]
	if i := strings.IndexAny(d, " \t>,;\"'()"); i >= 0 {
		d = d[:i]
	}
	d = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(d), "."))
	if !strings.Contains(d, ".") || len(d) < 4 {
		return ""
	}
	return d
}
