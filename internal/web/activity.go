package web

// Journal d'activité du compte webmail.
//
// Tout passe par la file d'attente scellée du dépôt (store.AppendEvent) : un échec de connexion s'écrit sans que le
// compte soit ouvert, et une action de session ne demande ni relecture ni réécriture de l'historique. Le versement
// dans l'historique chiffré a lieu quand la clé de données est disponible : à la connexion, et à l'affichage de la
// page. L'écriture ne doit jamais faire échouer la requête en cours : une erreur est journalisée, rien de plus.

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/store"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const (
	activityDays     = 30   // durée de conservation
	activityMax      = 2000 // plafond d'entrées gardées, au cas où 30 jours seraient très chargés
	activityAgentMax = 120  // l'agent utilisateur est tronqué : le reste n'apprend rien
	// evDropped n'est pas un événement du serveur : c'est la trace d'entrées perdues, écrite au versement.
	evDropped = "dropped"
)

// ActivityView présente le journal à la page des réglages.
type ActivityView struct {
	Rows []ActivityRow
	Days int
}

// ActivityRow est une ligne du journal, déjà mise en forme pour l'affichage.
type ActivityRow struct {
	When   string // date et heure dans le fuseau de l'utilisateur
	Kind   string // clé de traduction (activity.kind.*)
	IP     string
	Agent  string
	Detail string
	Warn   bool // échec ou refus : la ligne est signalée
}

// trace consigne un événement pour un login, compte fermé (échec de connexion, blocage).
func (s *Server) trace(login, kind string, r *http.Request, detail string) {
	raw, ok := s.eventJSON(kind, r, detail)
	if !ok {
		return
	}
	if err := s.store.AppendEvent(login, raw); err != nil {
		s.log.Warn("journal d'activité : écriture impossible", "login", store.ReduceLogin(login), "err", err)
	}
}

// traceSession consigne un événement d'une session ouverte.
func (s *Server) traceSession(sess *session.Session, kind string, r *http.Request, detail string) {
	if sess == nil || sess.Account == nil {
		return
	}
	raw, ok := s.eventJSON(kind, r, detail)
	if !ok {
		return
	}
	if err := s.store.AppendEventID(sess.Account.ID, raw); err != nil {
		s.log.Warn("journal d'activité : écriture impossible", "err", err)
	}
}

func (s *Server) eventJSON(kind string, r *http.Request, detail string) ([]byte, bool) {
	ev := userdata.Event{At: time.Now().Unix(), Kind: kind, Detail: detail}
	if r != nil {
		ev.IP = s.clientIP(r)
		ev.Agent = tronque(r.UserAgent(), activityAgentMax)
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		s.log.Warn("journal d'activité : encodage impossible", "err", err)
		return nil, false
	}
	return raw, true
}

func tronque(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// mergeActivity verse la file d'attente dans l'historique, élague et enregistre. Elle renvoie l'historique à jour,
// du plus ancien au plus récent.
func (s *Server) mergeActivity(acc *store.Account) ([]userdata.Event, error) {
	hist, err := s.readActivity(acc)
	if err != nil {
		return nil, err
	}
	entries, dropped, err := s.store.DrainEvents(acc)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 && dropped == 0 {
		return hist, nil
	}
	for _, raw := range entries {
		var ev userdata.Event
		if err := json.Unmarshal(raw, &ev); err != nil || ev.At == 0 {
			dropped++
			continue
		}
		hist = append(hist, ev)
	}
	if dropped > 0 {
		hist = append(hist, userdata.Event{At: time.Now().Unix(), Kind: evDropped, Detail: strconv.Itoa(dropped)})
	}
	hist = pruneActivity(hist)
	raw, err := json.Marshal(hist)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveActivity(acc, raw); err != nil {
		return nil, err
	}
	return hist, nil
}

func (s *Server) readActivity(acc *store.Account) ([]userdata.Event, error) {
	raw, err := s.store.LoadActivity(acc)
	if err != nil || len(raw) == 0 {
		// Historique illisible : on repart de zéro plutôt que de bloquer la connexion pour un journal.
		if err != nil {
			s.log.Warn("journal d'activité : historique illisible, remis à zéro", "err", err)
		}
		return nil, nil
	}
	var hist []userdata.Event
	if err := json.Unmarshal(raw, &hist); err != nil {
		s.log.Warn("journal d'activité : historique illisible, remis à zéro", "err", err)
		return nil, nil
	}
	return hist, nil
}

// pruneActivity trie par date, jette ce qui dépasse la durée de conservation, puis applique le plafond.
func pruneActivity(hist []userdata.Event) []userdata.Event {
	slices.SortStableFunc(hist, func(a, b userdata.Event) int { return cmp.Compare(a.At, b.At) })
	limite := time.Now().AddDate(0, 0, -activityDays).Unix()
	i := 0
	for i < len(hist) && hist[i].At < limite {
		i++
	}
	hist = hist[i:]
	if len(hist) > activityMax {
		hist = hist[len(hist)-activityMax:]
	}
	return slices.Clip(hist)
}

// lastLoginNotice résume, pour la bannière de connexion, la connexion précédente et les échecs survenus depuis.
// hist doit se terminer par la connexion courante.
func lastLoginNotice(hist []userdata.Event) (prev int64, failed int) {
	vu := false
	for i := len(hist) - 1; i >= 0; i-- {
		switch hist[i].Kind {
		case userdata.EvLoginOK:
			if !vu { // la connexion courante
				vu = true
				continue
			}
			return hist[i].At, failed
		case userdata.EvLoginFail, userdata.EvLoginBlocked, userdata.EvTwoFactorKO:
			failed++
		}
	}
	return 0, failed
}

// noteLogin consigne la connexion, verse la file et garnit la session de quoi afficher la bannière.
func (s *Server) noteLogin(sess *session.Session, acc *store.Account, r *http.Request) {
	s.traceSession(sess, userdata.EvLoginOK, r, "")
	hist, err := s.mergeActivity(acc)
	if err != nil {
		s.log.Warn("journal d'activité : versement impossible", "err", err)
		return
	}
	prev, failed := lastLoginNotice(hist)
	sess.Mu.Lock()
	sess.LastLogin, sess.FailedSince = prev, failed
	sess.Mu.Unlock()
}

// ensureEventKeys dote un compte ancien de ses clés de journal, à sa première connexion.
func (s *Server) ensureEventKeys(acc *store.Account) {
	modifie, err := s.store.EnsureEventKeys(acc)
	if err != nil {
		s.log.Warn("journal d'activité : clés indisponibles", "err", err)
		return
	}
	if modifie {
		if err := s.store.SaveData(acc); err != nil {
			s.log.Warn("journal d'activité : clé privée non enregistrée", "err", err)
		}
	}
}

// --- page des réglages ---

func (s *Server) activityGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	acc := sess.Account
	sess.Mu.Unlock()
	hist, err := s.mergeActivity(acc)
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	p := s.settingsPage(sess)
	p.Section = "activity"
	p.Activity = &ActivityView{Rows: s.activityRows(sess, hist), Days: activityDays}
	s.render(w, r, sess, http.StatusOK, "activity", p)
}

// activityClear efface l'historique à la demande de l'utilisateur.
func (s *Server) activityClear(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	sess.Mu.Lock()
	acc := sess.Account
	sess.Mu.Unlock()
	if _, _, err := s.store.DrainEvents(acc); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	if err := s.store.SaveActivity(acc, nil); err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	s.setFlash(sess, "activity.cleared")
	http.Redirect(w, r, "/settings/activity", http.StatusSeeOther)
}

// activityRows met en forme l'historique, du plus récent au plus ancien.
func (s *Server) activityRows(sess *session.Session, hist []userdata.Event) []ActivityRow {
	loc := s.userLoc(sess)
	rows := make([]ActivityRow, 0, len(hist))
	for i := len(hist) - 1; i >= 0; i-- {
		ev := hist[i]
		rows = append(rows, ActivityRow{
			When:   time.Unix(ev.At, 0).In(loc).Format("02/01/2006 15:04:05"),
			Kind:   "activity.kind." + ev.Kind,
			IP:     ev.IP,
			Agent:  ev.Agent,
			Detail: ev.Detail,
			Warn:   ev.Kind == userdata.EvLoginFail || ev.Kind == userdata.EvLoginBlocked || ev.Kind == userdata.EvTwoFactorKO || ev.Kind == evDropped,
		})
	}
	return rows
}

// LoginNotice rappelle la connexion précédente et les échecs survenus depuis. Il s'affiche une fois par session :
// l'intérêt est de voir l'anomalie en arrivant, pas de la répéter à chaque page.
type LoginNotice struct {
	Last   string // date de la connexion précédente, vide à la première
	Failed int
}

// loginNotice rend le rappel une seule fois, puis le marque comme vu.
func (s *Server) loginNotice(sess *session.Session) *LoginNotice {
	sess.Mu.Lock()
	vu, dernier, echecs := sess.NoticeSeen, sess.LastLogin, sess.FailedSince
	sess.NoticeSeen = true
	sess.Mu.Unlock()
	if vu || (dernier == 0 && echecs == 0) {
		return nil
	}
	n := &LoginNotice{Failed: echecs}
	if dernier != 0 {
		// userLoc prend sess.Mu : le verrou doit être relâché avant de l'appeler.
		n.Last = time.Unix(dernier, 0).In(s.userLoc(sess)).Format("02/01/2006 15:04")
	}
	return n
}
