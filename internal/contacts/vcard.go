package contacts

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/emersion/go-vcard"
)

// ErrNoCards : le flux ne contient aucune vCard.
var ErrNoCards = errors.New("contacts : aucune vCard trouvée")

// ParseVCards lit un flux vCard (2.1, 3.0 ou 4.0) ; renvoie les fiches valides et le nombre d'éléments ignorés.
func ParseVCards(r io.Reader) (list []*Contact, skipped int, err error) {
	dec := vcard.NewDecoder(r)
	for {
		card, e := dec.Decode()
		if errors.Is(e, io.EOF) {
			if len(list) == 0 && skipped == 0 {
				return nil, 0, ErrNoCards
			}
			return list, skipped, nil
		}
		if e != nil {
			if len(list) == 0 && skipped == 0 {
				return nil, 0, e
			}
			return list, skipped + 1, nil
		}
		c := fromCard(card)
		if c.Normalize() != nil {
			skipped++
			continue
		}
		list = append(list, c)
	}
}

// normType ramène les étiquettes des autres logiciels à celles que nous utilisons (« mobile » de RainLoop → « cell »).
var normType = map[string]string{"mobile": "cell", "cellular": "cell", "voice": "", "internet": "", "pref": "", "x-mobile": "cell"}

// typeOf donne l'étiquette d'une valeur (home, work, cell…) ; les types composés sont conservés (« work,fax »).
func typeOf(f *vcard.Field) string {
	var kept []string
	for _, t := range f.Params.Types() {
		t = strings.ToLower(strings.TrimSpace(t))
		if n, ok := normType[t]; ok {
			t = n
		}
		if t != "" && !slices.Contains(kept, t) {
			kept = append(kept, t)
		}
	}
	return strings.Join(kept, ",")
}

// prefFirst place en tête la valeur marquée PREF, comme le font les autres carnets.
func prefFirst(fields []*vcard.Field) []*vcard.Field {
	for i, f := range fields {
		if f.Params.Get(vcard.ParamPreferred) == "1" || slices.ContainsFunc(f.Params.Types(), func(t string) bool { return strings.EqualFold(t, "pref") }) {
			out := append([]*vcard.Field{f}, fields[:i]...)
			return append(out, fields[i+1:]...)
		}
	}
	return fields
}

func fromCard(card vcard.Card) *Contact {
	c := &Contact{UID: card.Value(vcard.FieldUID)}
	if n := card.Name(); n != nil {
		c.Prefix, c.First, c.Middle, c.Last, c.Suffix = n.HonorificPrefix, n.GivenName, n.AdditionalName, n.FamilyName, n.HonorificSuffix
	}
	if c.First == "" && c.Last == "" {
		if fn := strings.TrimSpace(card.PreferredValue(vcard.FieldFormattedName)); fn != "" {
			if i := strings.LastIndex(fn, " "); i > 0 {
				c.First, c.Last = fn[:i], fn[i+1:]
			} else {
				c.First = fn
			}
		}
	}
	c.Nick = card.Value(vcard.FieldNickname)
	for _, f := range prefFirst(card[vcard.FieldEmail]) {
		c.Emails = append(c.Emails, Typed{typeOf(f), f.Value})
	}
	for _, f := range prefFirst(card[vcard.FieldTelephone]) {
		c.Phones = append(c.Phones, Typed{typeOf(f), f.Value})
	}
	for _, f := range prefFirst(card[vcard.FieldURL]) {
		c.URLs = append(c.URLs, Typed{typeOf(f), f.Value})
	}
	for _, a := range card.Addresses() {
		c.Postals = append(c.Postals, Postal{typeOf(a.Field), strings.TrimSpace(a.StreetAddress + " " + a.ExtendedAddress), a.Locality, a.Region, a.PostalCode, a.Country})
	}
	if o := card.Value(vcard.FieldOrganization); o != "" {
		c.Org = strings.TrimSpace(strings.ReplaceAll(o, ";", " "))
	}
	c.Title = card.Value(vcard.FieldTitle)
	c.Notes = card.Value(vcard.FieldNote)
	if bd := strings.ReplaceAll(card.Value(vcard.FieldBirthday), "-", ""); len(bd) >= 8 {
		c.Birthday = bd[:4] + "-" + bd[4:6] + "-" + bd[6:8]
	}
	c.Photo = photoFromCard(card)
	c.Extra = extraFromCard(card)
	if cat := card.Value(vcard.FieldCategories); cat != "" {
		c.Tags = strings.Split(cat, ",")
	}
	return c
}

func toCard(c *Contact) vcard.Card {
	card := make(vcard.Card)
	card.SetValue(vcard.FieldVersion, "3.0")
	card.SetValue(vcard.FieldUID, c.UID)
	card.SetValue(vcard.FieldFormattedName, c.Display())
	card.SetName(&vcard.Name{FamilyName: c.Last, GivenName: c.First, AdditionalName: c.Middle, HonorificPrefix: c.Prefix, HonorificSuffix: c.Suffix})
	set := func(k, v string) {
		if v != "" {
			card.SetValue(k, v)
		}
	}
	set(vcard.FieldNickname, c.Nick)
	set(vcard.FieldOrganization, c.Org)
	set(vcard.FieldTitle, c.Title)
	set(vcard.FieldNote, c.Notes)
	set(vcard.FieldBirthday, c.Birthday)
	if len(c.Tags) > 0 {
		set(vcard.FieldCategories, strings.Join(c.Tags, ","))
	}
	add := func(k string, list []Typed) {
		for _, t := range list {
			f := &vcard.Field{Value: t.Value}
			if t.Type != "" {
				f.Params = vcard.Params{vcard.ParamType: {strings.ToUpper(t.Type)}}
			}
			card.Add(k, f)
		}
	}
	if c.Photo != "" {
		if _, mime, ok := c.PhotoBytes(); ok {
			_, b64, _ := strings.Cut(c.Photo, ",")
			card.Add(vcard.FieldPhoto, &vcard.Field{Value: strings.Join(strings.Fields(b64), ""),
				Params: vcard.Params{"ENCODING": {"b"}, vcard.ParamType: {strings.ToUpper(strings.TrimPrefix(mime, "image/"))}}})
		}
	}
	for _, line := range c.Extra { // propriétés inconnues conservées telles quelles (IMPP, X-…)
		if k, v, ok := strings.Cut(line, ":"); ok && k != "" {
			name, params, _ := strings.Cut(k, ";")
			f := &vcard.Field{Value: v, Params: vcard.Params{}}
			for _, kv := range strings.Split(params, ";") {
				if pk, pv, ok := strings.Cut(kv, "="); ok {
					f.Params.Add(strings.ToUpper(pk), pv)
				}
			}
			card.Add(strings.ToUpper(name), f)
		}
	}
	add(vcard.FieldEmail, c.Emails)
	add(vcard.FieldTelephone, c.Phones)
	add(vcard.FieldURL, c.URLs)
	for _, p := range c.Postals {
		a := &vcard.Address{StreetAddress: p.Street, Locality: p.City, Region: p.Region, PostalCode: p.Code, Country: p.Country}
		if p.Type != "" {
			a.Field = &vcard.Field{Params: vcard.Params{vcard.ParamType: {strings.ToUpper(p.Type)}}}
		}
		card.AddAddress(a)
	}
	return card
}

// EncodeVCard écrit les contacts au format vCard 3.0.
func EncodeVCard(w io.Writer, list []*Contact) error {
	enc := vcard.NewEncoder(w)
	for _, c := range list {
		if err := enc.Encode(toCard(c)); err != nil {
			return err
		}
	}
	return nil
}

// VCardBytes sérialise un seul contact (CardDAV).
func VCardBytes(c *Contact) ([]byte, error) {
	var b bytes.Buffer
	err := EncodeVCard(&b, []*Contact{c})
	return b.Bytes(), err
}

// photoFromCard lit la photo d'une vCard : « data: » directement, ou valeur en base64 avec ENCODING=b / BASE64.
func photoFromCard(card vcard.Card) string {
	f := card.Get(vcard.FieldPhoto)
	if f == nil || f.Value == "" || len(f.Value) > MaxPhoto {
		return ""
	}
	v := strings.TrimSpace(f.Value)
	if strings.HasPrefix(strings.ToLower(v), "data:image/") {
		return v
	}
	enc := strings.ToLower(f.Params.Get("ENCODING"))
	if enc != "b" && enc != "base64" {
		return "" // une URL externe n'est pas rapatriée (elle révélerait l'adresse IP du lecteur)
	}
	mime := "image/jpeg"
	switch strings.ToLower(f.Params.Get(vcard.ParamType)) {
	case "png":
		mime = "image/png"
	case "gif":
		mime = "image/gif"
	case "webp":
		mime = "image/webp"
	}
	return "data:" + mime + ";base64," + strings.Join(strings.Fields(v), "")
}

// knownFields : propriétés que ce carnet sait lire et réécrire ; les autres sont gardées telles quelles.
var knownFields = map[string]bool{
	vcard.FieldVersion: true, vcard.FieldUID: true, vcard.FieldFormattedName: true, vcard.FieldName: true,
	vcard.FieldNickname: true, vcard.FieldEmail: true, vcard.FieldTelephone: true, vcard.FieldURL: true,
	vcard.FieldAddress: true, vcard.FieldOrganization: true, vcard.FieldTitle: true, vcard.FieldNote: true,
	vcard.FieldBirthday: true, vcard.FieldCategories: true, vcard.FieldPhoto: true, "PRODID": true, "REV": true,
}

// extraFromCard garde les propriétés que nous ne connaissons pas, pour les réécrire lors d'une synchronisation CardDAV.
func extraFromCard(card vcard.Card) []string {
	var out []string
	total := 0
	for name, fields := range card {
		if knownFields[strings.ToUpper(name)] {
			continue
		}
		for _, f := range fields {
			var params []string
			for k, vals := range f.Params {
				for _, v := range vals {
					params = append(params, strings.ToUpper(k)+"="+v)
				}
			}
			sort.Strings(params)
			line := strings.ToUpper(name)
			if len(params) > 0 {
				line += ";" + strings.Join(params, ";")
			}
			line += ":" + f.Value
			if total+len(line) > MaxExtra || len(out) >= 40 {
				return out
			}
			total += len(line)
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}
