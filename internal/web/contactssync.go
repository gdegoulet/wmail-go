package web

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wmail-go/internal/contacts"
	"wmail-go/internal/session"
	"wmail-go/internal/userdata"
)

const (
	syncDefaultMinutes = 20
	syncMinMinutes     = 5
	syncMaxMinutes     = 320
	syncTimeout        = 90 * time.Second
	syncMinGap         = 2 * time.Minute // entre deux essais, réussis ou non
)

// SyncView décrit la synchronisation CardDAV sur la page des contacts.
type SyncView struct {
	Enabled         bool
	URL, User       string
	HasPassword     bool
	IntervalMinutes int
	LastSync        string
	Error           string
	Running         bool
}

func clampInterval(n int) int {
	if n < syncMinMinutes {
		if n == 0 {
			return syncDefaultMinutes
		}
		return syncMinMinutes
	}
	return min(n, syncMaxMinutes)
}

func (s *Server) syncView(sess *session.Session) SyncView {
	sess.Mu.Lock()
	cs := sess.Account.Data.ContactsSync
	sess.Mu.Unlock()
	v := SyncView{Enabled: cs.Enabled, URL: cs.URL, User: cs.User, HasPassword: cs.Password != "", IntervalMinutes: clampInterval(cs.IntervalMinutes)}
	sess.ContactsMu.Lock()
	v.Running = sess.ContactsSyncing
	if b, err := s.loadBook(sess); err == nil {
		if b.SyncedAt > 0 {
			v.LastSync = time.Unix(b.SyncedAt, 0).In(s.userLoc(sess)).Format("2006-01-02 15:04")
		}
		v.Error = b.SyncError
	}
	sess.ContactsMu.Unlock()
	return v
}

// maybeSyncContacts lance en arrière-plan une synchronisation si elle est activée et due.
func (s *Server) maybeSyncContacts(sess *session.Session) {
	sess.Mu.Lock()
	cs := sess.Account.Data.ContactsSync
	sess.Mu.Unlock()
	if !cs.Enabled || cs.URL == "" {
		return
	}
	sess.ContactsMu.Lock()
	b, err := s.loadBook(sess)
	due := err == nil && time.Since(time.Unix(b.SyncedAt, 0)) >= time.Duration(clampInterval(cs.IntervalMinutes))*time.Minute &&
		time.Since(sess.ContactsTried) >= syncMinGap && !sess.ContactsSyncing
	sess.ContactsMu.Unlock()
	if due {
		go func() { _, _ = s.runContactsSync(sess, false) }()
	}
}

// runContactsSync synchronise le carnet ; le réseau est utilisé hors verrou, sur une copie. Si le carnet a été modifié
// entre-temps, le résultat est écarté (le passage suivant reprend, les liens se retrouvent par UID).
func (s *Server) runContactsSync(sess *session.Session, force bool) (contacts.SyncResult, error) {
	var res contacts.SyncResult
	sess.Mu.Lock()
	cs := sess.Account.Data.ContactsSync
	sess.Mu.Unlock()
	if !cs.Enabled || cs.URL == "" {
		return res, nil
	}
	sess.ContactsMu.Lock()
	if sess.ContactsSyncing || (!force && time.Since(sess.ContactsTried) < syncMinGap) {
		sess.ContactsMu.Unlock()
		return res, nil
	}
	live, err := s.loadBook(sess)
	var work *contacts.Book
	var rev int
	if err == nil {
		var raw []byte
		if raw, err = live.Marshal(); err == nil {
			work, err = contacts.Parse(raw)
		}
	}
	if err != nil {
		sess.ContactsMu.Unlock()
		return res, err
	}
	rev = sess.ContactsRev
	sess.ContactsSyncing, sess.ContactsTried = true, time.Now()
	sess.ContactsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()
	res, syncErr := work.SyncCardDAV(ctx, s.guard.HTTPClient(syncTimeout), contacts.SyncConfig{URL: cs.URL, User: cs.User, Password: cs.Password})

	sess.ContactsMu.Lock()
	defer sess.ContactsMu.Unlock()
	sess.ContactsSyncing = false
	if syncErr != nil {
		live.SyncError = shortErr(syncErr)
		s.persistBook(sess, live)
		s.log.Warn("synchronisation des contacts en échec", "err", shortErr(syncErr))
		return res, syncErr
	}
	if sess.ContactsRev != rev || sess.Contacts != live {
		s.log.Info("synchronisation des contacts écartée : carnet modifié pendant le passage")
		return res, nil
	}
	sess.Contacts = work
	s.persistBook(sess, work)
	s.log.Info("contacts synchronisés", "résultat", res.String())
	return res, nil
}

func (s *Server) persistBook(sess *session.Session, b *contacts.Book) {
	sess.Mu.Lock()
	a := sess.Account
	sess.Mu.Unlock()
	if raw, err := b.Marshal(); err == nil {
		if err := s.store.SaveContacts(a, raw); err != nil {
			s.log.Warn("carnet d'adresses non enregistré après synchronisation", "err", shortErr(err))
		}
	}
}

func (s *Server) contactsSyncSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	in := userdata.ContactsSync{
		Enabled: r.PostFormValue("enabled") != "", URL: strings.TrimSpace(r.PostFormValue("url")), User: strings.TrimSpace(r.PostFormValue("user")),
		Password: r.PostFormValue("password"),
	}
	in.IntervalMinutes, _ = strconv.Atoi(r.PostFormValue("interval"))
	in.IntervalMinutes = clampInterval(in.IntervalMinutes)
	if len(in.User) > 200 || len(in.Password) > 500 {
		s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.sync.err.invalid")
		return
	}
	if in.Enabled || in.URL != "" {
		if err := contacts.ValidSyncURL(in.URL, s.guard.ExplicitlyAllowed); err != nil {
			s.contactsPage(w, r, sess, http.StatusBadRequest, nil, "contacts.sync.err.url")
			return
		}
	}
	sess.Mu.Lock()
	cur := &sess.Account.Data.ContactsSync
	prev := *cur
	if in.Password == "" && r.PostFormValue("clear_password") == "" {
		in.Password = cur.Password // champ laissé vide : on garde le mot de passe enregistré
	}
	*cur = in
	err := s.store.SaveData(sess.Account)
	if err != nil {
		*cur = prev
	}
	sess.Mu.Unlock()
	if err != nil {
		s.serverError(w, r, sess, err)
		return
	}
	sess.ContactsMu.Lock()
	sess.ContactsTried = time.Time{}
	sess.ContactsMu.Unlock()
	http.Redirect(w, r, "/contacts?info=syncsaved", http.StatusSeeOther)
}

func (s *Server) contactsSyncRun(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	res, err := s.runContactsSync(sess, true)
	if err != nil {
		s.contactsPage(w, r, sess, http.StatusBadGateway, nil, "contacts.sync.err.run")
		return
	}
	q := url.Values{"info": {"synced"}, "n": {strconv.Itoa(res.Pulled + res.Pushed + res.Deleted + res.RemoteDeleted)}}
	http.Redirect(w, r, "/contacts?"+q.Encode(), http.StatusSeeOther)
}
