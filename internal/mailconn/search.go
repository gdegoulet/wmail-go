package mailconn

import (
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
)

// searchTokens découpe une recherche en mots ; les guillemets regroupent, y compris après « clé: ».
func searchTokens(q string) []string {
	var out []string
	var cur strings.Builder
	quoted, any := false, false
	flush := func() {
		if any || cur.Len() > 0 {
			out = append(out, cur.String())
		}
		cur.Reset()
		any = false
	}
	for _, r := range q {
		switch {
		case r == '"':
			quoted = !quoted
			any = true
		case (r == ' ' || r == '\t') && !quoted:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

func parseSize(s string) (int64, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	mult := int64(1)
	for _, u := range []struct {
		suf string
		m   int64
	}{{"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"G", 1 << 30}, {"M", 1 << 20}, {"K", 1 << 10}, {"B", 1}} {
		if strings.HasSuffix(s, u.suf) {
			s, mult = strings.TrimSuffix(s, u.suf), u.m
			break
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n * mult, true
}

func parseDay(s string) (time.Time, bool) {
	for _, l := range []string{"2006.01.02", "2006-01-02", "2006/01/02"} {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func hdr(key, v string) imap.SearchCriteriaHeaderField {
	return imap.SearchCriteriaHeaderField{Key: key, Value: v}
}

// operatorKeys énumère les mots-clés reconnus. Il sert à repérer la forme « from: adresse@exemple.fr », avec une
// espace après les deux-points : la valeur est alors le mot suivant.
var operatorKeys = map[string]bool{
	"from": true, "to": true, "cc": true, "email": true, "mail": true, "subject": true, "text": true, "body": true,
	"has": true, "is": true, "date": true, "size": true, "larger": true, "bigger": true, "minsize": true,
	"smaller": true, "maxsize": true,
}

// applyOperators lit les mots-clés de RainLoop (from: to: cc: subject: text: has:attachment is: date: size: larger: smaller:)
// et les ajoute à crit ; renvoie le reste du texte libre et vrai si au moins un mot-clé a été reconnu.
// « from:a@b.fr », « from: a@b.fr » et « from:"a@b.fr" » sont équivalents.
func applyOperators(crit *imap.SearchCriteria, q string) (free []string, found bool) {
	toks := searchTokens(q)
	for i := 0; i < len(toks); i++ {
		tok := toks[i]
		key, val, ok := strings.Cut(tok, ":")
		key = strings.ToLower(strings.TrimSpace(key))
		if !ok || !operatorKeys[key] {
			free = append(free, tok)
			continue
		}
		// « from: a@b.fr » : les deux-points sont suivis d'une espace, la valeur est le mot suivant.
		next, taken := "", false
		if val = strings.TrimSpace(val); val == "" && i+1 < len(toks) {
			next, taken = toks[i+1], true
			val = strings.TrimSpace(next)
		}
		if val == "" {
			free = append(free, tok)
			continue
		}
		if !applyOperator(crit, key, val) { // valeur invalide : le mot-clé redevient du texte libre
			free = append(free, tok)
			if taken {
				free = append(free, next)
			}
			continue
		}
		if taken {
			i++
		}
		found = true
	}
	return free, found
}

// applyOperator ajoute un mot-clé à crit ; faux si la valeur est inutilisable (« size:abc », « is:inconnu »).
func applyOperator(crit *imap.SearchCriteria, key, val string) bool {
	recognized := true
	switch key {
	case "from":
		crit.Header = append(crit.Header, hdr("From", val))
	case "to":
		crit.Or = append(crit.Or, [2]imap.SearchCriteria{{Header: []imap.SearchCriteriaHeaderField{hdr("To", val)}}, {Header: []imap.SearchCriteriaHeaderField{hdr("Cc", val)}}})
	case "cc":
		crit.Header = append(crit.Header, hdr("Cc", val))
	case "email", "mail":
		crit.Or = append(crit.Or, [2]imap.SearchCriteria{{Header: []imap.SearchCriteriaHeaderField{hdr("From", val)}},
			{Or: [][2]imap.SearchCriteria{{{Header: []imap.SearchCriteriaHeaderField{hdr("To", val)}}, {Header: []imap.SearchCriteriaHeaderField{hdr("Cc", val)}}}}}})
	case "subject":
		crit.Header = append(crit.Header, hdr("Subject", val))
	case "text", "body":
		crit.Body = append(crit.Body, val)
	case "has":
		switch strings.ToLower(val) {
		case "attachment", "attachments", "attach", "attaches", "file", "files":
			crit.Or = append(crit.Or, [2]imap.SearchCriteria{
				{Header: []imap.SearchCriteriaHeaderField{hdr("Content-Type", "multipart/mixed")}},
				{Or: [][2]imap.SearchCriteria{{
					{Header: []imap.SearchCriteriaHeaderField{hdr("Content-Type", "multipart/signed")}},
					{Header: []imap.SearchCriteriaHeaderField{hdr("Content-Type", "application/")}},
				}}},
			})
		default:
			recognized = false
		}
	case "is":
		for _, v := range strings.Split(strings.ToLower(val), ",") {
			switch v {
			case "unread", "unseen":
				crit.NotFlag = append(crit.NotFlag, imap.FlagSeen)
			case "read", "seen":
				crit.Flag = append(crit.Flag, imap.FlagSeen)
			case "flag", "flagged", "star", "starred", "pinned":
				crit.Flag = append(crit.Flag, imap.FlagFlagged)
			case "unflag", "unflagged", "unstar", "unstarred":
				crit.NotFlag = append(crit.NotFlag, imap.FlagFlagged)
			default:
				recognized = false
			}
		}
	case "date":
		a, b, rng := strings.Cut(val, "/")
		from, ok1 := parseDay(a)
		if !ok1 && a != "" {
			recognized = false
			break
		}
		if rng {
			if ok1 {
				crit.Since = from
			}
			if to, ok2 := parseDay(b); ok2 {
				crit.Before = to.AddDate(0, 0, 1)
			}
		} else {
			crit.Since, crit.Before = from, from.AddDate(0, 0, 1)
		}
	case "size", "larger", "bigger", "minsize":
		if n, ok := parseSize(val); ok {
			crit.Larger = n
		} else {
			recognized = false
		}
	case "smaller", "maxsize":
		if n, ok := parseSize(val); ok {
			crit.Smaller = n
		} else {
			recognized = false
		}
	default:
		recognized = false
	}
	return recognized
}
