// Package ical lit les invitations iCalendar (.ics, text/calendar, RFC 5545) jointes aux messages : titre, dates, lieu,
// organisateur, participants. Seule la lecture est prise en charge ; les récurrences sont signalées, pas développées.
package ical

import (
	"net/url"
	"strings"
	"time"
	_ "time/tzdata" // fuseaux horaires embarqués : un TZID n'exige pas de base de fuseaux sur la machine
)

const (
	// MaxSize : au-delà, le fichier est tronqué avant analyse. Une invitation légitime tient très en deçà ;
	// la pièce jointe, elle, peut être de n'importe quelle taille.
	MaxSize = 256 << 10
	// MaxEvents : seuls les premiers événements d'un fichier sont lus (une invitation en contient un, un agenda
	// exporté par erreur en contiendrait des milliers).
	MaxEvents = 5
	// maxAttendees borne la liste des participants d'un même événement.
	maxAttendees = 100
)

// Person est un organisateur ou un participant.
type Person struct {
	Name, Email string
	Status      string // ACCEPTED, DECLINED, TENTATIVE, NEEDS-ACTION (participants)
}

// Label donne le nom, à défaut l'adresse.
func (p Person) Label() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Email
}

// Event est un événement lu dans le fichier.
type Event struct {
	Summary, Location, Description string
	Start, End                     time.Time // End : égal à Start si absent ; pour une journée entière, jour de fin exclu (comme iCalendar)
	AllDay, HasStart               bool
	Organizer                      Person
	Attendees                      []Person
	Method, Status, Recurrence     string
}

type prop struct {
	name   string
	params map[string]string
	value  string
}

// unfold défait le repliement des lignes (RFC 5545 §3.1) et normalise les fins de ligne.
func unfold(s string) []string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(out) > 0 {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return out
}

func parseProp(line string) (prop, bool) {
	// nom;PARAM=valeur;PARAM="valeur:avec deux-points":valeur
	inQ, cut := false, -1
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQ = !inQ
		case ':':
			if !inQ {
				cut = i
			}
		}
		if cut >= 0 {
			break
		}
	}
	if cut <= 0 {
		return prop{}, false
	}
	head, value := line[:cut], line[cut+1:]
	var parts []string
	inQ, start := false, 0
	for i := 0; i < len(head); i++ {
		switch head[i] {
		case '"':
			inQ = !inQ
		case ';':
			if !inQ {
				parts = append(parts, head[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, head[start:])
	p := prop{name: strings.ToUpper(strings.TrimSpace(parts[0])), params: map[string]string{}, value: value}
	for _, kv := range parts[1:] {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			p.params[strings.ToUpper(strings.TrimSpace(k))] = strings.Trim(v, `"`)
		}
	}
	return p, true
}

// text défait l'échappement des valeurs TEXT (\n, \, \; \\).
func text(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n', 'N':
				b.WriteByte('\n')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return strings.TrimSpace(b.String())
}

// person lit un ORGANIZER ou un ATTENDEE, dont la valeur est une URI (« mailto:bob@x.fr ») et le nom un
// paramètre CN. Le préfixe est cherché n'importe où, et non en tête, car certains émetteurs le font précéder
// d'espaces ou d'un autre schéma.
func person(p prop) Person {
	const mailto = "mailto:"
	email := p.value
	if i := strings.Index(strings.ToLower(email), mailto); i >= 0 {
		email = email[i+len(mailto):]
	}
	return Person{Name: text(p.params["CN"]), Email: strings.TrimSpace(email), Status: strings.ToUpper(p.params["PARTSTAT"])}
}

// parseTime lit une date ou date-heure iCalendar : UTC (suffixe Z), TZID connu, ou « flottante » (heure locale de l'affichage).
func parseTime(p prop, local *time.Location) (t time.Time, allDay, ok bool) {
	v := strings.TrimSpace(p.value)
	if strings.EqualFold(p.params["VALUE"], "DATE") || len(v) == 8 {
		d, err := time.ParseInLocation("20060102", v, time.UTC)
		return d, true, err == nil
	}
	loc := local
	layout := "20060102T150405"
	if strings.HasSuffix(v, "Z") || strings.HasSuffix(v, "z") {
		loc, v = time.UTC, v[:len(v)-1]
	} else if tz := p.params["TZID"]; tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	d, err := time.ParseInLocation(layout, v, loc)
	return d, false, err == nil
}

// Parse lit les événements (VEVENT) d'un fichier iCalendar. local sert aux heures sans fuseau.
func Parse(data []byte, local *time.Location) []Event {
	if len(data) > MaxSize {
		data = data[:MaxSize]
	}
	if local == nil {
		local = time.Local
	}
	var events []Event
	method := ""
	var cur *Event
	depth := 0 // sous-composants (VALARM…) ignorés dans l'événement
	for _, line := range unfold(string(data)) {
		p, ok := parseProp(line)
		if !ok {
			continue
		}
		switch {
		case p.name == "METHOD" && cur == nil:
			method = strings.ToUpper(strings.TrimSpace(p.value))
		case p.name == "BEGIN" && strings.EqualFold(p.value, "VEVENT"):
			if len(events) >= MaxEvents {
				return events
			}
			cur, depth = &Event{Method: method}, 0
		case p.name == "BEGIN" && cur != nil:
			depth++
		case p.name == "END" && cur != nil && depth > 0:
			depth--
		case p.name == "END" && strings.EqualFold(p.value, "VEVENT") && cur != nil:
			if !cur.HasStart {
				cur.End = cur.Start
			}
			if cur.End.IsZero() || cur.End.Before(cur.Start) {
				cur.End = cur.Start
			}
			if cur.Method == "" {
				cur.Method = method
			}
			events = append(events, *cur)
			cur = nil
		case cur != nil && depth == 0:
			switch p.name {
			case "SUMMARY":
				cur.Summary = text(p.value)
			case "LOCATION":
				cur.Location = text(p.value)
			case "DESCRIPTION":
				cur.Description = text(p.value)
			case "STATUS":
				cur.Status = strings.ToUpper(strings.TrimSpace(p.value))
			case "RRULE":
				cur.Recurrence = strings.TrimSpace(p.value)
			case "ORGANIZER":
				cur.Organizer = person(p)
			case "ATTENDEE":
				if len(cur.Attendees) < maxAttendees {
					cur.Attendees = append(cur.Attendees, person(p))
				}
			case "DTSTART":
				if t, ad, ok := parseTime(p, local); ok {
					cur.Start, cur.AllDay, cur.HasStart = t, ad, true
				}
			case "DTEND":
				if t, _, ok := parseTime(p, local); ok {
					cur.End = t
				}
			case "DURATION":
				if d, ok := parseDuration(p.value); ok && cur.HasStart {
					cur.End = cur.Start.Add(d)
				}
			}
		}
	}
	return events
}

// parseDuration lit une durée iCalendar simple (P1DT2H30M, PT45M, P1W).
func parseDuration(s string) (time.Duration, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !strings.HasPrefix(s, "P") {
		return 0, false
	}
	s = s[1:]
	var d time.Duration
	num, inTime := 0, false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			num = num*10 + int(r-'0')
		case r == 'T':
			inTime = true
		case r == 'W':
			d += time.Duration(num) * 7 * 24 * time.Hour
			num = 0
		case r == 'D':
			d += time.Duration(num) * 24 * time.Hour
			num = 0
		case r == 'H' && inTime:
			d += time.Duration(num) * time.Hour
			num = 0
		case r == 'M' && inTime:
			d += time.Duration(num) * time.Minute
			num = 0
		case r == 'S' && inTime:
			d += time.Duration(num) * time.Second
			num = 0
		default:
			return 0, false
		}
	}
	return d, true
}

// GoogleURL est le lien « ajouter à Google Agenda ».
func (e Event) GoogleURL() string {
	if !e.HasStart {
		return ""
	}
	f := func(t time.Time) string {
		if e.AllDay {
			return t.Format("20060102")
		}
		return t.UTC().Format("20060102T150405Z")
	}
	end := e.End
	if e.AllDay && !end.After(e.Start) {
		end = e.Start.AddDate(0, 0, 1)
	}
	q := url.Values{"action": {"TEMPLATE"}, "text": {e.Summary}, "dates": {f(e.Start) + "/" + f(end)}, "location": {e.Location}}
	return "https://www.google.com/calendar/render?" + q.Encode()
}

// OutlookURL est le lien « ajouter à Outlook (web) ».
func (e Event) OutlookURL() string {
	if !e.HasStart {
		return ""
	}
	f := func(t time.Time) string {
		if e.AllDay {
			return t.Format("2006-01-02")
		}
		return t.UTC().Format("2006-01-02T15:04:05Z")
	}
	end := e.End
	if e.AllDay && !end.After(e.Start) {
		end = e.Start.AddDate(0, 0, 1)
	}
	q := url.Values{"path": {"/calendar/action/compose"}, "rru": {"addevent"}, "subject": {e.Summary}, "startdt": {f(e.Start)}, "enddt": {f(end)}, "location": {e.Location}}
	if e.AllDay {
		q.Set("allday", "true")
	}
	return "https://outlook.office.com/calendar/0/deeplink/compose?" + q.Encode()
}
