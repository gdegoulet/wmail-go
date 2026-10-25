package web

import (
	"net/http"
	"net/mail"
	"slices"
	"strings"

	"github.com/gdegoulet/wmail-go/internal/compose"
	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const maxIdentities = 20

// IdentitiesView alimente la page des identités d'une boîte.
type IdentitiesView struct {
	AccountID, AccountEmail string
	Main                    userdata.Identity
	Extra                   []userdata.Identity
	Edit                    userdata.Identity // identité en cours d'édition (ID vide : ajout)
	Editing                 bool
	Full                    bool
}

func (s *Server) identitiesPage(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, accID string, edit userdata.Identity, errKey string, errArg ...any) {
	acc, ok := findAccount(s.mailAccounts(sess), accID)
	if !ok {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	all := acc.AllIdentities()
	v := &IdentitiesView{AccountID: acc.ID, AccountEmail: acc.Email, Main: all[0], Extra: all[1:], Edit: edit, Editing: edit.ID != "", Full: len(acc.Identities) >= maxIdentities}
	p := &Page{TitleKey: "identities.title", Identities: v, ActiveID: acc.ID, ErrorKey: errKey}
	if len(errArg) > 0 {
		p.ErrorArg = errArg[0]
	}
	s.render(w, r, sess, status, "identities", p)
}

func (s *Server) identitiesGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	accID := r.PathValue("id")
	var edit userdata.Identity
	if iid := r.URL.Query().Get("edit"); iid != "" {
		if acc, ok := findAccount(s.mailAccounts(sess), accID); ok {
			if i := slices.IndexFunc(acc.Identities, func(x userdata.Identity) bool { return x.ID == iid }); i >= 0 {
				edit = acc.Identities[i]
			}
		}
	}
	s.identitiesPage(w, r, sess, http.StatusOK, accID, edit, "")
}

// identitiesSave ajoute ou modifie une identité supplémentaire.
func (s *Server) identitiesSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	accID := r.PathValue("id")
	list := s.mailAccounts(sess)
	idx := slices.IndexFunc(list, func(a userdata.MailAccount) bool { return a.ID == accID })
	if idx < 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	acc := list[idx]
	in := userdata.Identity{
		ID:    r.PostFormValue("iid"),
		Email: strings.TrimSpace(r.PostFormValue("email")), Name: strings.TrimSpace(r.PostFormValue("name")),
		ReplyTo: strings.TrimSpace(r.PostFormValue("reply_to")), Bcc: strings.TrimSpace(r.PostFormValue("bcc")),
		Signature:       strings.ReplaceAll(r.PostFormValue("signature"), "\r\n", "\n"),
		SignatureBefore: r.PostFormValue("signature_before") != "",
	}
	fail := func(key string) { s.identitiesPage(w, r, sess, http.StatusBadRequest, accID, in, key) }
	if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email || len(in.Email) > 254 || strings.ContainsAny(in.Email, "\r\n") {
		fail("accounts.err.email")
		return
	}
	if len([]rune(in.Name)) > 100 || strings.ContainsAny(in.Name, "\r\n") || len(in.Signature) > 5000 || len(in.ReplyTo) > 500 || len(in.Bcc) > 500 {
		fail("accounts.err.too_long")
		return
	}
	for _, v := range []string{in.ReplyTo, in.Bcc} {
		if _, err := compose.ParseList(v); err != nil {
			fail("identities.err.address")
			return
		}
	}
	for _, other := range acc.AllIdentities() {
		if other.ID != in.ID && strings.EqualFold(other.Email, in.Email) {
			fail("identities.err.duplicate")
			return
		}
	}
	extra := slices.Clone(acc.Identities)
	if in.ID == "" {
		if len(extra) >= maxIdentities {
			fail("identities.err.limit")
			return
		}
		in.ID = randomID()
		extra = append(extra, in)
	} else {
		i := slices.IndexFunc(extra, func(x userdata.Identity) bool { return x.ID == in.ID })
		if i < 0 {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		extra[i] = in
	}
	list[idx].Identities = extra
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.setFlash(sess, "identities.saved")
	http.Redirect(w, r, "/accounts/"+accID+"/identities", http.StatusSeeOther)
}

func (s *Server) identitiesDelete(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	accID, iid := r.PathValue("id"), r.PathValue("iid")
	list := s.mailAccounts(sess)
	idx := slices.IndexFunc(list, func(a userdata.MailAccount) bool { return a.ID == accID })
	if idx < 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	extra := slices.DeleteFunc(slices.Clone(list[idx].Identities), func(x userdata.Identity) bool { return x.ID == iid })
	if len(extra) == len(list[idx].Identities) {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	list[idx].Identities = extra
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.setFlash(sess, "identities.deleted")
	http.Redirect(w, r, "/accounts/"+accID+"/identities", http.StatusSeeOther)
}

// identitiesReorder enregistre un ordre complet des identités supplémentaires d'une boîte (champ « box »),
// envoyé par le glisser-déposer. L'identité principale n'est pas concernée : elle reste toujours la première.
func (s *Server) identitiesReorder(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	accID := r.PostFormValue("box")
	list := s.mailAccounts(sess)
	ai := slices.IndexFunc(list, func(a userdata.MailAccount) bool { return a.ID == accID })
	if ai < 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	out, ok := reorderByIDs(list[ai].Identities, r.PostFormValue("order"), func(x userdata.Identity) string { return x.ID })
	if !ok {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	list[ai].Identities = out
	if err := s.saveAccounts(sess, list); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.reorderDone(w, r, "/accounts/"+accID+"/identities")
}

// identityMove change l'ordre des identités supplémentaires d'une boîte (l'identité principale reste la première).
func (s *Server) identityMove(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	accID, iid, dir := r.PathValue("id"), r.PathValue("iid"), r.PostFormValue("dir")
	list := s.mailAccounts(sess)
	ai := slices.IndexFunc(list, func(a userdata.MailAccount) bool { return a.ID == accID })
	if ai < 0 || (dir != "up" && dir != "down") {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	ids := slices.Clone(list[ai].Identities)
	ii := slices.IndexFunc(ids, func(x userdata.Identity) bool { return x.ID == iid })
	if ii < 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	if moveInList(ids, ii, dir, 0) {
		list[ai].Identities = ids
		if err := s.saveAccounts(sess, list); err != nil {
			s.serverError(w, r, sess, err)
			return
		}
	}
	http.Redirect(w, r, "/accounts/"+accID+"/identities", http.StatusSeeOther)
}
