package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"

	"github.com/gdegoulet/wmail-go/internal/contacts"
	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const maxContactsImport = 8 << 20

// loadBook renvoie le carnet en cache, le lit sur disque au premier accès. sess.ContactsMu doit être tenu.
func (s *Server) loadBook(sess *session.Session) (*contacts.Book, error) {
	if sess.Contacts != nil {
		return sess.Contacts, nil
	}
	sess.Mu.Lock()
	acc := sess.Account
	sess.Mu.Unlock()
	raw, err := s.store.LoadContacts(acc)
	if err != nil {
		return nil, err
	}
	b, err := contacts.Parse(raw)
	if err != nil {
		return nil, err
	}
	sess.Contacts = b
	return b, nil
}

// readBook exécute fn sur le carnet sans le modifier.
func (s *Server) readBook(sess *session.Session, fn func(*contacts.Book)) error {
	sess.ContactsMu.Lock()
	defer sess.ContactsMu.Unlock()
	b, err := s.loadBook(sess)
	if err != nil {
		return err
	}
	fn(b)
	return nil
}

// writeBook exécute fn puis enregistre le carnet ; si fn échoue ou si l'écriture échoue, le cache est relu.
func (s *Server) writeBook(sess *session.Session, fn func(*contacts.Book) error) error {
	sess.ContactsMu.Lock()
	defer sess.ContactsMu.Unlock()
	b, err := s.loadBook(sess)
	if err != nil {
		return err
	}
	if err := fn(b); err != nil {
		sess.Contacts = nil
		return err
	}
	sess.ContactsRev++
	raw, err := b.Marshal()
	if err == nil {
		sess.Mu.Lock()
		acc := sess.Account
		sess.Mu.Unlock()
		err = s.store.SaveContacts(acc, raw)
	}
	if err != nil {
		sess.Contacts = nil
	}
	return err
}

// ContactRow est une ligne de la liste des contacts.
type ContactRow struct {
	ID, Name, Email, Phone, Tags string
}

// ContactsView alimente la page des contacts.
type ContactsView struct {
	Rows                  []ContactRow
	Total, Page, Pages    int
	Q, Tag                string
	AllTags               []string
	Sync                  SyncView
	Edit                  *contacts.Contact // fiche affichée dans le formulaire (ID vide : création)
	Editing               bool
	EmailRows, PhoneRows  []contacts.Typed
	URLRows               []contacts.Typed
	PostalRows            []contacts.Postal
	TagsText              string
	PhotoURL              string // photo actuelle du contact (URI data:)
	Info                  string // clé de message d'information
	Added, Skipped        int
	PrevURL, NextURL      string
	ExportURL, ComposeURL string
}

func padTyped(l []contacts.Typed, n int) []contacts.Typed {
	out := append([]contacts.Typed(nil), l...)
	for i := 0; i < n; i++ {
		out = append(out, contacts.Typed{})
	}
	return out
}

func (s *Server) contactsPage(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, edit *contacts.Contact, errKey string) {
	q := r.URL.Query()
	v := &ContactsView{Q: strings.TrimSpace(q.Get("q")), Tag: strings.TrimSpace(q.Get("tag"))}
	v.Page, _ = strconv.Atoi(q.Get("page"))
	if v.Page < 1 {
		v.Page = 1
	}
	switch q.Get("info") {
	case "saved", "deleted", "imported", "added", "syncsaved", "synced":
		v.Info = "contacts.info." + q.Get("info")
	}
	v.Sync = s.syncView(sess)
	v.Added, _ = strconv.Atoi(q.Get("n"))
	v.Skipped, _ = strconv.Atoi(q.Get("skipped"))
	s.maybeSyncContacts(sess)
	err := s.readBook(sess, func(b *contacts.Book) {
		list, total := b.Search(v.Q, v.Tag, v.Page)
		v.Total, v.Pages = total, (total+contacts.PageSize-1)/contacts.PageSize
		for _, c := range list {
			row := ContactRow{ID: c.ID, Name: c.Display(), Tags: strings.Join(c.Tags, ", ")}
			if len(c.Emails) > 0 {
				row.Email = c.Emails[0].Value
			}
			if len(c.Phones) > 0 {
				row.Phone = c.Phones[0].Value
			}
			v.Rows = append(v.Rows, row)
		}
		v.AllTags = b.AllTags()
		if edit == nil {
			if id := q.Get("edit"); id != "" {
				if c := b.Get(id); c != nil {
					cp := *c
					edit = &cp
				}
			} else if q.Get("new") != "" {
				edit = &contacts.Contact{}
				if e := q.Get("email"); e != "" {
					edit.Emails = []contacts.Typed{{Value: e}}
				}
				edit.First = q.Get("name")
			}
		}
	})
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	if edit != nil {
		v.Edit, v.Editing = edit, true
		v.EmailRows, v.PhoneRows, v.URLRows = padTyped(edit.Emails, 2), padTyped(edit.Phones, 2), padTyped(edit.URLs, 1)
		v.PostalRows = append(append([]contacts.Postal(nil), edit.Postals...), contacts.Postal{})
		v.TagsText = strings.Join(edit.Tags, ", ")
		v.PhotoURL = edit.Photo
	}
	nav := url.Values{}
	if v.Q != "" {
		nav.Set("q", v.Q)
	}
	if v.Tag != "" {
		nav.Set("tag", v.Tag)
	}
	v.ExportURL = "/contacts/export?" + nav.Encode()
	pageURL := func(n int) string {
		c := url.Values{}
		for k, x := range nav {
			c[k] = x
		}
		c.Set("page", strconv.Itoa(n))
		return "/contacts?" + c.Encode()
	}
	if v.Page > 1 {
		v.PrevURL = pageURL(v.Page - 1)
	}
	if v.Page < v.Pages {
		v.NextURL = pageURL(v.Page + 1)
	}
	p := &Page{TitleKey: "contacts.title", Contacts: v, ErrorKey: errKey}
	s.render(w, r, sess, status, "contacts", p)
}

func (s *Server) contactsGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	s.contactsPage(w, r, sess, http.StatusOK, nil, "")
}

func zipTyped(types, values []string, max int) []contacts.Typed {
	var out []contacts.Typed
	for i, v := range values {
		if i >= max {
			break
		}
		t := ""
		if i < len(types) {
			t = types[i]
		}
		if strings.TrimSpace(v) != "" {
			out = append(out, contacts.Typed{Type: t, Value: v})
		}
	}
	return out
}

func contactFromForm(r *http.Request) *contacts.Contact {
	f := r.PostForm
	c := &contacts.Contact{
		ID: r.PostFormValue("id"), Prefix: r.PostFormValue("prefix"), First: r.PostFormValue("first"), Middle: r.PostFormValue("middle"),
		Last: r.PostFormValue("last"), Suffix: r.PostFormValue("suffix"), Nick: r.PostFormValue("nick"),
		Org: r.PostFormValue("org"), Title: r.PostFormValue("title"), Birthday: r.PostFormValue("birthday"),
		Notes: strings.ReplaceAll(r.PostFormValue("notes"), "\r\n", "\n"), Photo: strings.TrimSpace(r.PostFormValue("photo")),
		Tags: strings.Split(r.PostFormValue("tags"), ","),
	}
	c.Emails = zipTyped(f["email_type"], f["email"], 2*contacts.MaxValues)
	c.Phones = zipTyped(f["phone_type"], f["phone"], 2*contacts.MaxValues)
	c.URLs = zipTyped(nil, f["url"], 2*contacts.MaxValues)
	for i := range f["adr_street"] {
		get := func(k string) string {
			if l := f[k]; i < len(l) {
				return l[i]
			}
			return ""
		}
		c.Postals = append(c.Postals, contacts.Postal{Type: get("adr_type"), Street: get("adr_street"), City: get("adr_city"), Region: get("adr_region"), Code: get("adr_code"), Country: get("adr_country")})
		if i >= 2*contacts.MaxValues {
			break
		}
	}
	return c
}

func (s *Server) contactsSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	c := contactFromForm(r)
	err := s.writeBook(sess, func(b *contacts.Book) error { return b.Save(c) })
	switch {
	case errors.Is(err, contacts.ErrEmpty):
		s.contactsPage(w, r, sess, http.StatusBadRequest, c, "contacts.err.empty")
	case errors.Is(err, contacts.ErrFull):
		s.contactsPage(w, r, sess, http.StatusBadRequest, c, "contacts.err.full")
	case errors.Is(err, contacts.ErrNotFound):
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
	case err != nil:
		s.serverError(w, r, sess, err)
	default:
		http.Redirect(w, r, "/contacts?info=saved&edit="+url.QueryEscape(c.ID), http.StatusSeeOther)
	}
}

func (s *Server) contactsDelete(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	_ = r.ParseForm()
	ids := r.PostForm["id"]
	err := s.writeBook(sess, func(b *contacts.Book) error { b.Delete(ids...); return nil })
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, "/contacts?info=deleted", http.StatusSeeOther)
}

func (s *Server) contactsImport(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	if err := r.ParseMultipartForm(maxContactsImport); err != nil {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.err.import")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.err.import")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxContactsImport))
	if err != nil {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.err.import")
		return
	}
	var list []*contacts.Contact
	var ignored int
	if strings.HasSuffix(strings.ToLower(hdr.Filename), ".csv") || !bytes.Contains(bytes.ToUpper(data[:min(len(data), 4096)]), []byte("BEGIN:VCARD")) {
		list, ignored, err = contacts.ParseCSV(bytes.NewReader(data))
	} else {
		list, ignored, err = contacts.ParseVCards(bytes.NewReader(data))
	}
	if err != nil {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.err.import")
		return
	}
	added, skipped := 0, ignored
	err = s.writeBook(sess, func(b *contacts.Book) error {
		for _, c := range list {
			dup := false
			for _, e := range c.Emails {
				if b.ByEmail(e.Value) != nil {
					dup = true
				}
			}
			if dup || b.Save(c) != nil {
				skipped++
				continue
			}
			added++
		}
		return nil
	})
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	http.Redirect(w, r, "/contacts?info=imported&n="+strconv.Itoa(added)+"&skipped="+strconv.Itoa(skipped), http.StatusSeeOther)
}

func (s *Server) contactsExport(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	q := r.URL.Query()
	var all []*contacts.Contact
	err := s.readBook(sess, func(b *contacts.Book) {
		for p := 1; ; p++ {
			l, total := b.Search(q.Get("q"), q.Get("tag"), p)
			for _, c := range l {
				cp := *c
				all = append(all, &cp)
			}
			if len(all) >= total || len(l) == 0 {
				break
			}
		}
	})
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	var buf bytes.Buffer
	if q.Get("format") == "csv" {
		err = contacts.EncodeCSV(&buf, all)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="contacts.csv"`)
	} else {
		err = contacts.EncodeVCard(&buf, all)
		w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="contacts.vcf"`)
	}
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.traceSession(sess, userdata.EvContactsOut, r, strconv.Itoa(len(all)))
	_, _ = w.Write(buf.Bytes())
}

// contactsSuggest répond en JSON aux champs d'autocomplétion ; all=1 liste les contacts (sélecteur) par page de 50.
func (s *Server) contactsSuggest(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) > 100 {
		q = string([]rune(q)[:100])
	}
	out := []contacts.Suggestion{}
	_ = s.readBook(sess, func(b *contacts.Book) {
		if r.URL.Query().Get("all") == "1" {
			list, _ := b.Search(q, "", 1)
			for _, c := range list {
				for _, e := range c.Emails {
					label := e.Value
					if n := c.Display(); n != "" && n != e.Value {
						label = contacts.FormatAddress(n, e.Value)
					}
					out = append(out, contacts.Suggestion{Name: c.Display(), Email: e.Value, Label: label})
				}
			}
			return
		}
		out = b.Suggest(q, 20) // comme RainLoop
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

// contactsAdd ajoute l'expéditeur d'un message au carnet puis ouvre sa fiche.
func (s *Server) contactsAdd(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	email, name := strings.TrimSpace(r.PostFormValue("email")), strings.TrimSpace(r.PostFormValue("name"))
	var id string
	err := s.writeBook(sess, func(b *contacts.Book) error {
		c, _ := b.AddAddress(name, email)
		if c == nil {
			return contacts.ErrEmpty
		}
		id = c.ID
		return nil
	})
	if err != nil {
		s.message(w, r, sess, http.StatusBadRequest, "", "contacts.err.add")
		return
	}
	http.Redirect(w, r, "/contacts?info=added&edit="+url.QueryEscape(id), http.StatusSeeOther)
}

// contactsAfterSend met à jour fréquences et ajouts automatiques ; une erreur n'est que journalisée.
func (s *Server) contactsAfterSend(sess *session.Session, to, cc []*mail.Address) {
	sess.Mu.Lock()
	auto := !sess.Account.Data.Settings.NoContactsAutosave
	sess.Mu.Unlock()
	err := s.writeBook(sess, func(b *contacts.Book) error {
		for _, a := range to {
			b.Bump(a.Address)
		}
		if auto {
			for _, a := range append(append([]*mail.Address(nil), to...), cc...) {
				b.AddAddress(a.Name, a.Address)
			}
		}
		return nil
	})
	if err != nil {
		s.log.Warn("carnet d'adresses non mis à jour après l'envoi", "err", shortErr(err))
	}
}

// contactIDFor renvoie l'identifiant du contact portant cette adresse (vide : inconnu).
func (s *Server) contactIDFor(sess *session.Session, email string) string {
	id := ""
	_ = s.readBook(sess, func(b *contacts.Book) {
		if c := b.ByEmail(email); c != nil {
			id = c.ID
		}
	})
	return id
}

// contactPhoto renvoie la photo enregistrée dans le carnet pour cette adresse.
func (s *Server) contactPhoto(sess *session.Session, email string) (data []byte, mime string, ok bool) {
	_ = s.readBook(sess, func(b *contacts.Book) {
		if c := b.ByEmail(email); c != nil {
			data, mime, ok = c.PhotoBytes()
		}
	})
	return data, mime, ok
}

// avatarsEnabled dit si une photo de correspondant peut être affichée (carnet d'adresses ou Gravatar).
func (s *Server) avatarsEnabled(sess *session.Session) bool {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return sess.Account.Data.Settings.Avatars == "gravatar" || len(sess.Account.Data.MailAccounts) > 0
}

// contactsCompose ouvre une composition adressée aux contacts cochés (première adresse de chacun).
func (s *Server) contactsCompose(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	_ = r.ParseForm()
	ids := r.PostForm["id"]
	if len(ids) == 0 {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.err.no_selection")
		return
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var to []string
	_ = s.readBook(sess, func(b *contacts.Book) {
		for _, c := range b.Contacts {
			if !want[c.ID] || len(c.Emails) == 0 || len(to) >= 100 {
				continue
			}
			if n := c.Display(); n != "" && n != c.Emails[0].Value {
				to = append(to, contacts.FormatAddress(n, c.Emails[0].Value))
			} else {
				to = append(to, c.Emails[0].Value)
			}
		}
	})
	if len(to) == 0 {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.err.no_address")
		return
	}
	http.Redirect(w, r, "/compose?"+url.Values{"to": {strings.Join(to, ", ")}}.Encode(), http.StatusSeeOther)
}
