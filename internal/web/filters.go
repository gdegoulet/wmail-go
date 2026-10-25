package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gdegoulet/wmail-go/internal/mailconn"
	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/sieve"
	"github.com/gdegoulet/wmail-go/internal/store"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const maxFilters = 100

// FilterRow est une ligne de la liste des filtres.
type FilterRow struct {
	ID, Name, Summary string
	Enabled           bool
	First, Last       bool
}

// CondRow est une ligne de condition dans le formulaire d'un filtre.
type CondRow struct {
	Field, Type, Value, Second string
}

// FiltersView alimente les pages des filtres.
type FiltersView struct {
	Rows          []FilterRow
	NotConfigured bool
	Foreign       bool // le script du même nom n'a pas été écrit par un client de filtres
	RawActive     bool
	Raw           string
	Modules       map[string]bool
	Folders       []string
	// formulaire
	Edit      sieve.Filter
	EditConds []CondRow
	IsNew     bool
	Errors    []string
}

// sieveState est l'état des filtres sur le serveur.
type sieveState struct {
	Filters   []sieve.Filter
	Foreign   bool
	RawActive bool
	Raw       string
	HasBasic  bool
	Modules   map[string]bool
}

func (s *Server) loadSieve(ctx context.Context, acc userdata.MailAccount) (*sieveState, error) {
	st := &sieveState{Modules: map[string]bool{}}
	err := s.guard.WithSieve(ctx, sieveEndpoint(acc), func(c *sieve.Client) error {
		for _, m := range c.Extensions() {
			st.Modules[m] = true
		}
		list, err := c.ListScripts()
		if err != nil {
			return err
		}
		for _, sc := range list {
			switch sc.Name {
			case sieve.ScriptName:
				body, err := c.GetScript(sc.Name)
				if err != nil {
					return err
				}
				st.HasBasic = true
				var ok bool
				if st.Filters, ok = sieve.ParseScript(body); !ok {
					st.Foreign = true
				}
			case sieve.ScriptRawName:
				body, err := c.GetScript(sc.Name)
				if err != nil {
					return err
				}
				st.Raw, st.RawActive = body, sc.Active
			}
		}
		return nil
	})
	return st, err
}

// saveSieveFilters écrit le script des filtres et l'active ; sans filtre, le script est supprimé.
func (s *Server) saveSieveFilters(ctx context.Context, acc userdata.MailAccount, filters []sieve.Filter) error {
	script, err := sieve.BuildScript(filters)
	if err != nil {
		return err
	}
	return s.guard.WithSieve(ctx, sieveEndpoint(acc), func(c *sieve.Client) error {
		list, err := c.ListScripts()
		if err != nil {
			return err
		}
		if len(filters) == 0 {
			if i := slices.IndexFunc(list, func(x sieve.Script) bool { return x.Name == sieve.ScriptName }); i >= 0 {
				if list[i].Active {
					if err := c.SetActive(""); err != nil {
						return err
					}
				}
				return c.DeleteScript(sieve.ScriptName)
			}
			return nil
		}
		if err := c.PutScript(sieve.ScriptName, script); err != nil {
			return err
		}
		return c.SetActive(sieve.ScriptName)
	})
}

// sieveErrKey traduit une erreur ManageSieve en clé de message.
func sieveErrKey(err error) (string, any) {
	var se *sieve.ServerError
	var te *mailconn.TestError
	switch {
	case errors.As(err, &se):
		return "filters.err.server", shortErr(se)
	case errors.As(err, &te) && (te.Kind == mailconn.KindRefused || te.Kind == mailconn.KindTimeout || te.Kind == mailconn.KindDNS || te.Kind == mailconn.KindBlocked):
		return "filters.err.unavailable", nil
	case errors.As(err, &te):
		return "accounts.test.fail." + te.Kind, nil
	}
	return "filters.err.server", shortErr(err)
}

// sieveLimit applique la limitation de débit des connexions ManageSieve ; il renvoie vrai si la requête est refusée.
func (s *Server) sieveLimit(w http.ResponseWriter, r *http.Request, sess *session.Session) bool {
	ipKey, userKey := s.limitKeys("sieve", r, sess.Account.Login)
	if wait, blocked := s.checkLimits(s.cfg.RateLimit.Sieve, ipKey, userKey); blocked {
		s.rateLimited(w, r, sess, wait)
		return true
	}
	s.addFailure(s.cfg.RateLimit.Sieve, ipKey, userKey)
	return false
}

func (s *Server) filtersAccount(w http.ResponseWriter, r *http.Request, sess *session.Session) (userdata.MailAccount, bool) {
	acc, ok := s.settingsBox(r, sess)
	if !ok {
		if r.FormValue("box") != "" {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return acc, false
		}
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return acc, false
	}
	return acc, true
}

func (s *Server) filtersRender(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, page string, acc userdata.MailAccount, v *FiltersView, errKey string, errArg any) {
	p := &Page{TitleKey: "set.filters", Section: "filters", ActiveID: acc.ID, Filters: v, ErrorKey: errKey, ErrorArg: errArg, Accounts: accountViews(s.mailAccounts(sess)), BoxEmail: acc.Email, Path: "/settings/filters"}
	s.render(w, r, sess, status, page, p)
}

// filterSummary décrit un filtre en une ligne.
func (s *Server) filterSummary(f sieve.Filter, lang string) string {
	t := func(k string, a ...any) string { return s.i18n.T(lang, k, a...) }
	conds := t("filters.sum.all")
	if n := len(f.Conditions); n == 1 {
		conds = t("filters.sum.one")
	} else if n > 1 {
		conds = t("filters.sum.many", n)
	}
	var act string
	switch f.ActionType {
	case sieve.ActionMoveTo:
		act = t("filters.sum.move", f.ActionValue)
	case sieve.ActionForward:
		act = t("filters.sum.forward", f.ActionValue)
	case sieve.ActionReject:
		act = t("filters.act.reject")
	case sieve.ActionVacation:
		act = t("filters.act.vacation")
	case sieve.ActionDiscard:
		act = t("filters.act.discard")
	default:
		act = t("filters.act.none")
	}
	return conds + " → " + act
}

func (s *Server) filtersGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.filtersAccount(w, r, sess)
	if !ok {
		return
	}
	v := &FiltersView{}
	if acc.Sieve.Disabled {
		v.NotConfigured = true
		s.filtersRender(w, r, sess, http.StatusOK, "filters", acc, v, "", nil)
		return
	}
	if s.sieveLimit(w, r, sess) {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	st, err := s.loadSieve(ctx, acc)
	if err != nil {
		key, arg := sieveErrKey(err)
		s.log.Warn("filtres : lecture impossible", "login", store.ReduceLogin(sess.Account.Login), "err", shortErr(err))
		s.filtersRender(w, r, sess, http.StatusBadGateway, "filters", acc, v, key, arg)
		return
	}
	lang := s.lang(r, sess)
	v.Foreign, v.RawActive, v.Raw, v.Modules = st.Foreign, st.RawActive, st.Raw, st.Modules
	for i, f := range st.Filters {
		v.Rows = append(v.Rows, FilterRow{ID: f.ID, Name: f.Name, Enabled: f.Enabled, Summary: s.filterSummary(f, lang), First: i == 0, Last: i == len(st.Filters)-1})
	}
	s.filtersRender(w, r, sess, http.StatusOK, "filters", acc, v, "", nil)
}

func (s *Server) folderNames(ctx context.Context, sess *session.Session, acc userdata.MailAccount) []string {
	var names []string
	_ = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		fs, err := c.Folders(false)
		for _, f := range fs {
			if f.Selectable() {
				names = append(names, f.Name)
			}
		}
		return err
	})
	return names
}

func condRows(f sieve.Filter) []CondRow {
	rows := make([]CondRow, 0, max(len(f.Conditions), 3))
	for _, c := range f.Conditions {
		rows = append(rows, CondRow{Field: c.Field, Type: c.Type, Value: c.Value, Second: c.ValueSecond})
	}
	for len(rows) < 3 { // lignes vides : sans JavaScript on peut tout de même saisir jusqu'à trois conditions
		rows = append(rows, CondRow{Field: sieve.FieldFrom, Type: sieve.TypeContains})
	}
	return rows
}

// filterEdit affiche le formulaire d'un filtre (nouveau ou existant, ou prérempli pour « filtrer les messages
// similaires »).
func (s *Server) filterEdit(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.filtersAccount(w, r, sess)
	if !ok {
		return
	}
	if acc.Sieve.Disabled {
		http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
		return
	}
	if s.sieveLimit(w, r, sess) {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	st, err := s.loadSieve(ctx, acc)
	if err != nil {
		key, arg := sieveErrKey(err)
		s.filtersRender(w, r, sess, http.StatusBadGateway, "filters", acc, &FiltersView{}, key, arg)
		return
	}
	q := r.URL.Query()
	f := sieve.Filter{Enabled: true, ConditionsType: sieve.MatchAny, ActionType: sieve.ActionMoveTo, Keep: true, Stop: true}
	isNew := true
	if id := q.Get("id"); id != "" {
		i := slices.IndexFunc(st.Filters, func(x sieve.Filter) bool { return x.ID == id })
		if i < 0 {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		f, isNew = st.Filters[i], false
	} else {
		if from := strings.TrimSpace(q.Get("from")); from != "" && len(from) < 300 {
			f.Conditions = append(f.Conditions, sieve.Condition{Field: sieve.FieldFrom, Type: sieve.TypeContains, Value: from})
		}
		if subj := strings.TrimSpace(q.Get("subject")); subj != "" && len(subj) < 300 {
			f.Conditions = append(f.Conditions, sieve.Condition{Field: sieve.FieldSubject, Type: sieve.TypeContains, Value: subj})
		}
		if len(f.Conditions) > 0 {
			f.ConditionsType = sieve.MatchAll
			f.Name = strings.TrimSpace(q.Get("from"))
		}
	}
	v := &FiltersView{Edit: f, EditConds: condRows(f), IsNew: isNew, Modules: st.Modules, Folders: s.folderNames(ctx, sess, acc), Foreign: st.Foreign}
	s.filtersRender(w, r, sess, http.StatusOK, "filter_edit", acc, v, "", nil)
}

// filterFromForm lit le formulaire ; les lignes de condition sans valeur sont ignorées.
func filterFromForm(r *http.Request) sieve.Filter {
	f := sieve.Filter{
		ID: r.PostFormValue("id"), Name: strings.TrimSpace(r.PostFormValue("name")), Enabled: r.PostFormValue("enabled") != "",
		ConditionsType: r.PostFormValue("match"), ActionType: r.PostFormValue("action"),
		Keep: r.PostFormValue("keep") != "", Stop: r.PostFormValue("stop") != "", MarkAsRead: r.PostFormValue("mark_read") != "",
	}
	// Chaque action a ses propres champs (noms distincts) : on ne lit que ceux de l'action choisie.
	switch f.ActionType {
	case sieve.ActionMoveTo:
		f.ActionValue = strings.TrimSpace(r.PostFormValue("move_to"))
	case sieve.ActionForward:
		f.ActionValue = strings.TrimSpace(r.PostFormValue("forward_to"))
	case sieve.ActionReject:
		f.ActionValue = r.PostFormValue("reject_msg")
	case sieve.ActionVacation:
		f.ActionValue, f.ActionValueSecond = r.PostFormValue("vac_msg"), strings.TrimSpace(r.PostFormValue("vac_subject"))
		f.ActionValueThird, f.ActionValueFourth = strings.TrimSpace(r.PostFormValue("vac_days")), strings.TrimSpace(r.PostFormValue("vac_addrs"))
	}
	fields, types, values, seconds := r.PostForm["cond_field"], r.PostForm["cond_type"], r.PostForm["cond_value"], r.PostForm["cond_second"]
	for i := range fields {
		if i >= len(types) || i >= len(values) || i > 60 {
			break
		}
		c := sieve.Condition{Field: fields[i], Type: types[i], Value: strings.TrimSpace(values[i])}
		if i < len(seconds) {
			c.ValueSecond = strings.TrimSpace(seconds[i])
		}
		if c.Value == "" {
			continue
		}
		f.Conditions = append(f.Conditions, c)
	}
	return f
}

// filterSave crée ou modifie un filtre.
func (s *Server) filterSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.filtersAccount(w, r, sess)
	if !ok {
		return
	}
	if acc.Sieve.Disabled {
		http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
		return
	}
	if s.sieveLimit(w, r, sess) {
		return
	}
	f := filterFromForm(r)
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	st, err := s.loadSieve(ctx, acc)
	if err != nil {
		key, arg := sieveErrKey(err)
		s.filtersRender(w, r, sess, http.StatusBadGateway, "filters", acc, &FiltersView{}, key, arg)
		return
	}
	redisplay := func(status int, errs []string, key string, arg any) {
		v := &FiltersView{Edit: f, EditConds: condRows(f), IsNew: f.ID == "", Modules: st.Modules, Folders: s.folderNames(ctx, sess, acc), Errors: errs, Foreign: st.Foreign}
		s.filtersRender(w, r, sess, status, "filter_edit", acc, v, key, arg)
	}
	if errs := f.Valid(st.Modules); len(errs) > 0 {
		redisplay(http.StatusBadRequest, errs, "", nil)
		return
	}
	if st.Foreign && r.PostFormValue("confirm_overwrite") == "" {
		redisplay(http.StatusConflict, []string{"filters.err.foreign"}, "", nil)
		return
	}
	filters := slices.Clone(st.Filters)
	if f.ID == "" {
		if len(filters) >= maxFilters {
			redisplay(http.StatusBadRequest, []string{"filters.err.too_many"}, "", nil)
			return
		}
		f.ID = sieve.NewID()
		filters = append(filters, f)
	} else {
		i := slices.IndexFunc(filters, func(x sieve.Filter) bool { return x.ID == f.ID })
		if i < 0 {
			s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
			return
		}
		filters[i] = f
	}
	if err := s.saveSieveFilters(ctx, acc, filters); err != nil {
		key, arg := sieveErrKey(err)
		s.log.Warn("filtres : enregistrement refusé", "login", store.ReduceLogin(sess.Account.Login), "err", shortErr(err))
		redisplay(http.StatusBadGateway, nil, key, arg)
		return
	}
	s.log.Info("filtres enregistrés", "login", store.ReduceLogin(sess.Account.Login), "filtres", len(filters))
	s.traceSession(sess, userdata.EvFilters, r, "")
	s.setFlash(sess, "filters.saved")
	http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
}

// filterAction active, désactive, supprime ou déplace un filtre.
func (s *Server) filterAction(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.filtersAccount(w, r, sess)
	if !ok {
		return
	}
	if acc.Sieve.Disabled {
		http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
		return
	}
	action, id := r.PostFormValue("action"), r.PostFormValue("id")
	if !slices.Contains([]string{"toggle", "delete", "up", "down"}, action) {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	if s.sieveLimit(w, r, sess) {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	st, err := s.loadSieve(ctx, acc)
	if err != nil {
		key, arg := sieveErrKey(err)
		s.filtersRender(w, r, sess, http.StatusBadGateway, "filters", acc, &FiltersView{}, key, arg)
		return
	}
	if st.Foreign {
		s.setFlash(sess, "filters.err.foreign")
		http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
		return
	}
	filters := slices.Clone(st.Filters)
	i := slices.IndexFunc(filters, func(x sieve.Filter) bool { return x.ID == id })
	if i < 0 {
		s.message(w, r, sess, http.StatusNotFound, "", "error.not_found")
		return
	}
	switch action {
	case "toggle":
		filters[i].Enabled = !filters[i].Enabled
	case "delete":
		filters = slices.Delete(filters, i, i+1)
	case "up":
		if i > 0 {
			filters[i], filters[i-1] = filters[i-1], filters[i]
		}
	case "down":
		if i < len(filters)-1 {
			filters[i], filters[i+1] = filters[i+1], filters[i]
		}
	}
	if err := s.saveSieveFilters(ctx, acc, filters); err != nil {
		key, arg := sieveErrKey(err)
		s.filtersRender(w, r, sess, http.StatusBadGateway, "filters", acc, &FiltersView{}, key, arg)
		return
	}
	s.setFlash(sess, "filters.saved")
	http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
}

// filtersRaw affiche et enregistre le script brut.
func (s *Server) filtersRaw(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, ok := s.filtersAccount(w, r, sess)
	if !ok {
		return
	}
	if acc.Sieve.Disabled {
		http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
		return
	}
	if s.sieveLimit(w, r, sess) {
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	st, err := s.loadSieve(ctx, acc)
	if err != nil {
		key, arg := sieveErrKey(err)
		s.filtersRender(w, r, sess, http.StatusBadGateway, "filters", acc, &FiltersView{}, key, arg)
		return
	}
	v := &FiltersView{Raw: st.Raw, RawActive: st.RawActive, Modules: st.Modules}
	if r.Method == http.MethodPost {
		v.Raw, v.RawActive = strings.ReplaceAll(r.PostFormValue("raw"), "\r\n", "\n"), r.PostFormValue("use_raw") != ""
		if len(v.Raw) > 200<<10 {
			s.filtersRender(w, r, sess, http.StatusRequestEntityTooLarge, "filters_raw", acc, v, "filters.err.too_large", nil)
			return
		}
		script := strings.ReplaceAll(v.Raw, "\n", "\r\n")
		err := s.guard.WithSieve(ctx, sieveEndpoint(acc), func(c *sieve.Client) error {
			if strings.TrimSpace(script) != "" {
				if err := c.CheckScript(script); err != nil {
					return err
				}
				if err := c.PutScript(sieve.ScriptRawName, script); err != nil {
					return err
				}
			}
			switch {
			case v.RawActive && strings.TrimSpace(script) != "":
				return c.SetActive(sieve.ScriptRawName)
			case st.HasBasic:
				return c.SetActive(sieve.ScriptName) // retour aux filtres de la liste
			default:
				return c.SetActive("")
			}
		})
		if err != nil {
			key, arg := sieveErrKey(err)
			s.filtersRender(w, r, sess, http.StatusBadRequest, "filters_raw", acc, v, key, arg)
			return
		}
		s.setFlash(sess, "filters.saved")
		http.Redirect(w, r, "/settings/filters?box="+url.QueryEscape(acc.ID), http.StatusSeeOther)
		return
	}
	if v.Raw == "" { // point de départ : le script actuel des filtres
		if script, err := sieve.BuildScript(st.Filters); err == nil && len(st.Filters) > 0 {
			v.Raw = strings.ReplaceAll(script, "\r\n", "\n")
		}
	}
	s.filtersRender(w, r, sess, http.StatusOK, "filters_raw", acc, v, "", nil)
}
