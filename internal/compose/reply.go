package compose

import (
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var prefixRe = regexp.MustCompile(`(?i)^(re|fwd?|tr)(\[\d+\]|\(\d+\))?$`)

// ReplySubject ajoute le préfixe « Re » ou « Fwd » à un objet sans empiler les préfixes : les « Re: », « Fwd: »,
// « Re[2]: » déjà présents sont ramenés à un seul. prefix vaut « Re » ou « Fwd ».
func ReplySubject(prefix, subject string) string {
	prefix = strings.TrimSpace(prefix)
	fwd := strings.EqualFold(prefix, "Fwd")
	subject = strings.Join(strings.Fields(subject), " ")
	// On retire les préfixes déjà présents en tête.
	for {
		i := strings.Index(subject, ":")
		if i < 0 || !prefixRe.MatchString(strings.TrimSpace(subject[:i])) {
			break
		}
		subject = strings.TrimSpace(subject[i+1:])
	}
	if fwd {
		prefix = "Fwd"
	} else {
		prefix = "Re"
	}
	if subject == "" {
		return prefix + ":"
	}
	return prefix + ": " + subject
}

// ReplyRecipients renvoie le destinataire d'une réponse : Reply-To, à défaut l'expéditeur, sans les adresses de
// exclude. Si le résultat est vide (réponse à soi-même), on le recalcule sans exclusion.
func ReplyRecipients(replyTo, from []*mail.Address, exclude map[string]bool) []*mail.Address {
	pick := func(ex map[string]bool) []*mail.Address {
		if r := Dedup(replyTo, ex); len(r) > 0 {
			return r
		}
		return Dedup(from, ex)
	}
	if r := pick(exclude); len(r) > 0 {
		return r
	}
	return pick(nil)
}

// ReplyAll renvoie les listes À et Cc d'une réponse à tous : destinataire principal comme ReplyRecipients, plus les
// autres destinataires À (en À) et Cc (en Cc), sans doublon ni adresse de exclude.
func ReplyAll(replyTo, from, to, cc []*mail.Address, exclude map[string]bool) (outTo, outCc []*mail.Address) {
	seen := map[string]bool{}
	for k := range exclude {
		seen[k] = true
	}
	main := Dedup(replyTo, seen)
	if len(main) == 0 {
		main = Dedup(from, seen)
	}
	if len(main) == 0 { // réponse à soi-même
		main = ReplyRecipients(replyTo, from, nil)
	}
	for _, a := range main {
		seen[strings.ToLower(a.Address)] = true
	}
	outTo = append(outTo, main...)
	for _, a := range Dedup(to, seen) {
		seen[strings.ToLower(a.Address)] = true
		outTo = append(outTo, a)
	}
	for _, a := range Dedup(cc, seen) {
		seen[strings.ToLower(a.Address)] = true
		outCc = append(outCc, a)
	}
	return outTo, outCc
}

// Quote met chaque ligne du texte en citation (« > »).
func Quote(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, ">") {
			lines[i] = ">" + l
		} else {
			lines[i] = "> " + l
		}
	}
	return strings.Join(lines, "\n")
}

// ForwardHeader décrit le message d'origine en tête d'un transfert. Les libellés viennent de l'appelant (traduits).
type ForwardHeader struct {
	Title, From, To, Cc, Date, Subject string // libellés
	FromV, ToV, CcV, SubjectV          string // valeurs
	DateV                              time.Time
}

// ForwardText compose le texte d'un transfert : ligne de titre, en-têtes du message d'origine, puis son texte.
func ForwardText(h ForwardHeader, body string) string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(h.Title + "\n")
	b.WriteString(h.From + ": " + h.FromV + "\n")
	if h.ToV != "" {
		b.WriteString(h.To + ": " + h.ToV + "\n")
	}
	if h.CcV != "" {
		b.WriteString(h.Cc + ": " + h.CcV + "\n")
	}
	if !h.DateV.IsZero() {
		b.WriteString(h.Date + ": " + h.DateV.Format("Mon, 02 Jan 2006 15:04") + "\n")
	}
	b.WriteString(h.Subject + ": " + h.SubjectV + "\n\n")
	b.WriteString(strings.TrimRight(body, "\n"))
	b.WriteString("\n")
	return b.String()
}

// References construit l'en-tête References d'une réponse (RFC 5322) : références du message d'origine, puis son
// identifiant, sans doublon. Au-delà de 20 entrées, on garde la première (début du fil) et les plus récentes.
func References(messageID string, refs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range append(append([]string{}, refs...), messageID) {
		id = strings.Trim(strings.TrimSpace(id), "<>")
		if id == "" || seen[id] || strings.ContainsAny(id, " \r\n<>") {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) > 20 {
		out = append([]string{out[0]}, out[len(out)-19:]...)
	}
	return out
}
