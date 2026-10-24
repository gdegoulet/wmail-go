package contacts

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// ErrCSV : fichier CSV illisible ou sans colonne reconnue.
var ErrCSV = errors.New("contacts : CSV illisible ou sans colonne reconnue")

// Colonnes d'export (ordre fixe, en-têtes anglais compris par les autres logiciels).
var csvHeader = []string{"First Name", "Middle Name", "Last Name", "Title", "Suffix", "Nickname", "E-mail", "E-mail 2", "E-mail 3",
	"Mobile Phone", "Home Phone", "Business Phone", "Other Phone", "Web Page", "Company", "Job Title",
	"Street", "City", "State", "Postal Code", "Country", "Birthday", "Notes", "Categories"}

// field d'une colonne reconnue à l'import.
type colKind struct {
	name string
	typ  string
}

func headerKind(h string) colKind {
	h = fold(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
	h = strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(h)
	h = strings.Join(strings.Fields(h), " ")
	switch h {
	case "first name", "given name", "prenom", "firstname":
		return colKind{"first", ""}
	case "middle name", "additional name":
		return colKind{"middle", ""}
	case "last name", "family name", "surname", "nom", "nom de famille", "lastname":
		return colKind{"last", ""}
	case "title", "name prefix", "prefix", "civilite":
		return colKind{"prefix", ""}
	case "suffix", "name suffix":
		return colKind{"suffix", ""}
	case "nickname", "nick name", "surnom":
		return colKind{"nick", ""}
	case "name", "display name", "full name", "nom complet", "nom affiche":
		return colKind{"display", ""}
	case "company", "organization", "organisation", "societe", "entreprise":
		return colKind{"org", ""}
	case "job title", "fonction", "position":
		return colKind{"jobtitle", ""}
	case "notes", "note", "comment", "commentaire":
		return colKind{"notes", ""}
	case "birthday", "birth date", "anniversaire", "date de naissance":
		return colKind{"birthday", ""}
	case "categories", "category", "labels", "groups", "group membership", "etiquettes":
		return colKind{"tags", ""}
	case "web page", "website", "url", "site web", "personal web page", "business web page":
		return colKind{"url", ""}
	case "street", "address", "home street", "business street", "adresse", "rue":
		return colKind{"street", ""}
	case "city", "home city", "business city", "ville":
		return colKind{"city", ""}
	case "state", "region", "home state", "business state":
		return colKind{"region", ""}
	case "postal code", "zip", "home postal code", "business postal code", "code postal":
		return colKind{"code", ""}
	case "country", "home country", "business country", "pays":
		return colKind{"country", ""}
	case "mobile phone", "mobile", "cell phone", "portable", "telephone portable", "mobile number", "business mobile", "mobile telephone number", "telephone mobile":
		return colKind{"phone", "cell"}
	case "home fax", "business fax", "fax", "other fax", "telecopie", "fax number":
		return colKind{"phone", "fax"}
	case "home email", "home e mail", "personal email", "email personnel", "adresse personnelle":
		return colKind{"email", "home"}
	case "business email", "business e mail", "work email", "email professionnel", "email bureau":
		return colKind{"email", "work"}
	case "other email", "other e mail", "autre email", "email 3", "e mail 3":
		return colKind{"email", "other"}
	case "primary email", "secondary email", "email principal", "email secondaire":
		return colKind{"email", ""}
	case "home phone", "telephone domicile", "telephone fixe":
		return colKind{"phone", "home"}
	case "business phone", "work phone", "telephone bureau", "telephone professionnel":
		return colKind{"phone", "work"}
	}
	switch {
	case strings.HasPrefix(h, "e mail") || strings.HasPrefix(h, "email") || h == "mail" || h == "courriel" || h == "adresse e mail":
		if strings.HasSuffix(h, " type") || strings.HasSuffix(h, " label") {
			return colKind{}
		}
		return colKind{"email", ""}
	case strings.HasPrefix(h, "phone") || h == "tel" || h == "telephone" || strings.HasSuffix(h, " phone") || strings.HasSuffix(h, "phone number"):
		if strings.HasSuffix(h, " type") || strings.HasSuffix(h, " label") {
			return colKind{}
		}
		return colKind{"phone", ""}
	}
	return colKind{}
}

func decodeText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if utf8.Valid(data) {
		return string(data)
	}
	if s, err := charmap.Windows1252.NewDecoder().Bytes(data); err == nil {
		return string(s)
	}
	return string(data)
}

func detectSep(first string) rune {
	best, n := ',', strings.Count(first, ",")
	for _, c := range []rune{';', '\t'} {
		if k := strings.Count(first, string(c)); k > n {
			best, n = c, k
		}
	}
	return best
}

// ParseCSV lit un CSV (virgule, point-virgule ou tabulation ; UTF-8 ou Windows-1252) dont la première ligne est l'en-tête.
func ParseCSV(r io.Reader) (list []*Contact, skipped int, err error) {
	raw, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil {
		return nil, 0, err
	}
	text := decodeText(raw)
	first, _, _ := strings.Cut(text, "\n")
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = detectSep(first)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	head, err := cr.Read()
	if err != nil {
		return nil, 0, ErrCSV
	}
	kinds := make([]colKind, len(head))
	known := 0
	for i, h := range head {
		if kinds[i] = headerKind(h); kinds[i].name != "" {
			known++
		}
	}
	if known == 0 {
		return nil, 0, ErrCSV
	}
	for {
		rec, e := cr.Read()
		if errors.Is(e, io.EOF) {
			return list, skipped, nil
		}
		if e != nil {
			skipped++
			continue
		}
		c := &Contact{}
		var display string
		for i, v := range rec {
			if i >= len(kinds) || strings.TrimSpace(v) == "" {
				continue
			}
			v = strings.TrimSpace(v)
			var p *Postal
			if len(c.Postals) == 0 {
				c.Postals = []Postal{{}}
			}
			p = &c.Postals[0]
			switch k := kinds[i]; k.name {
			case "first":
				c.First = v
			case "middle":
				c.Middle = v
			case "last":
				c.Last = v
			case "prefix":
				c.Prefix = v
			case "suffix":
				c.Suffix = v
			case "nick":
				c.Nick = v
			case "display":
				display = v
			case "org":
				c.Org = v
			case "jobtitle":
				c.Title = v
			case "notes":
				c.Notes = v
			case "birthday":
				c.Birthday = normDate(v)
			case "tags":
				c.Tags = append(c.Tags, strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == ':' })...)
			case "url":
				c.URLs = append(c.URLs, Typed{Value: v})
			case "email":
				for _, a := range strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == ',' || r == ' ' }) {
					c.Emails = append(c.Emails, Typed{Type: k.typ, Value: a})
				}
			case "phone":
				c.Phones = append(c.Phones, Typed{k.typ, v})
			case "street":
				p.Street = v
			case "city":
				p.City = v
			case "region":
				p.Region = v
			case "code":
				p.Code = v
			case "country":
				p.Country = v
			}
		}
		if c.First == "" && c.Last == "" && display != "" {
			if i := strings.LastIndex(display, " "); i > 0 {
				c.First, c.Last = display[:i], display[i+1:]
			} else {
				c.First = display
			}
		}
		if c.Normalize() != nil {
			skipped++
			continue
		}
		list = append(list, c)
	}
}

// normDate accepte AAAA-MM-JJ, AAAA/MM/JJ, JJ/MM/AAAA, JJ-MM-AAAA ; sinon vide.
func normDate(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "/", "-"), ".", "-")
	p := strings.Split(s, "-")
	if len(p) != 3 {
		return ""
	}
	if len(p[0]) == 4 {
		return pad(p[0], 4) + "-" + pad(p[1], 2) + "-" + pad(p[2], 2)
	}
	if len(p[2]) == 4 {
		return pad(p[2], 4) + "-" + pad(p[1], 2) + "-" + pad(p[0], 2)
	}
	return ""
}

func pad(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}

// guard neutralise l'injection de formules à l'ouverture dans un tableur.
func guard(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// EncodeCSV écrit les contacts en CSV UTF-8 avec BOM (lisible par les tableurs).
func EncodeCSV(w io.Writer, list []*Contact) error {
	if _, err := w.Write([]byte("\xef\xbb\xbf")); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	pick := func(l []Typed, i int) string {
		if i < len(l) {
			return l[i].Value
		}
		return ""
	}
	phone := func(c *Contact, typ string) string {
		for _, p := range c.Phones {
			if p.Type == typ {
				return p.Value
			}
		}
		return ""
	}
	for _, c := range list {
		var p Postal
		if len(c.Postals) > 0 {
			p = c.Postals[0]
		}
		other := ""
		for _, ph := range c.Phones {
			if ph.Type != "cell" && ph.Type != "home" && ph.Type != "work" {
				other = ph.Value
				break
			}
		}
		row := []string{c.First, c.Middle, c.Last, c.Prefix, c.Suffix, c.Nick, pick(c.Emails, 0), pick(c.Emails, 1), pick(c.Emails, 2),
			phone(c, "cell"), phone(c, "home"), phone(c, "work"), other, pick(c.URLs, 0), c.Org, c.Title,
			p.Street, p.City, p.Region, p.Code, p.Country, c.Birthday, c.Notes, strings.Join(c.Tags, ",")}
		for i := range row {
			row[i] = guard(row[i])
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
