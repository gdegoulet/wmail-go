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

// applyOperators lit les mots-clés de RainLoop (from: to: cc: subject: text: has:attachment is: date: size: larger: smaller:)
// et les ajoute à crit ; renvoie le reste du texte libre et vrai si au moins un mot-clé a été reconnu.
func applyOperators(crit *imap.SearchCriteria, q string) (free []string, found bool) {
	for _, tok := range searchTokens(q) {
		key, val, ok := strings.Cut(tok, ":")
		key = strings.ToLower(key)
		if !ok || val == "" {
			free = append(free, tok)
			continue
		}
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
		if !recognized {
			free = append(free, tok)
			continue
		}
		found = true
	}
	return free, found
}
