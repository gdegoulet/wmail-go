package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"wmail-go/internal/mailconn"
	"wmail-go/internal/session"
	"wmail-go/internal/userdata"
)

// PollInfo configure le script de sondage d'une page du courrier.
type PollInfo struct {
	Unseen         uint32
	Desktop, Sound bool
}

func pollInfo(folders []mailconn.Folder, set userdata.Settings) *PollInfo {
	pi := &PollInfo{Desktop: set.NotifyDesktop, Sound: set.NotifySound}
	for _, f := range folders {
		if strings.EqualFold(f.Name, "INBOX") {
			pi.Unseen = f.Unseen
		}
	}
	return pi
}

type pollNew struct {
	UID     uint32 `json:"uid"`
	From    string `json:"from"`
	Subject string `json:"subject"`
}

type pollReply struct {
	Unseen  uint32            `json:"unseen"`
	UIDNext uint32            `json:"uidnext"`
	New     []pollNew         `json:"new"`
	Folders map[string]uint32 `json:"folders"` // non lus des dossiers surveillés
}

// mailPoll répond au sondage périodique : non lus de la boîte de réception et nouveaux messages depuis `since`.
func (s *Server) mailPoll(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Error(w, `{"error":"no_account"}`, http.StatusConflict)
		return
	}
	s.maybeSyncContacts(sess)
	since64, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 32)
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var res *mailconn.PollResult
	watched := map[string]uint32{}
	err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) (err error) {
		if res, err = c.Poll("INBOX", uint32(since64)); err != nil {
			return err
		}
		for _, f := range acc.CheckFolders {
			if r, e := c.Poll(f, 0); e == nil { // un dossier disparu ou illisible n'empêche pas le reste
				watched[f] = r.Unseen
			}
		}
		return nil
	})
	if err != nil {
		http.Error(w, `{"error":"imap"}`, http.StatusBadGateway)
		return
	}
	out := pollReply{Unseen: res.Unseen, UIDNext: res.UIDNext, New: []pollNew{}, Folders: watched}
	for _, m := range res.New {
		from := ""
		if len(m.From) > 0 {
			from = m.From[0].Short()
		}
		out.New = append(out.New, pollNew{UID: m.UID, From: from, Subject: m.Subject})
	}
	_ = json.NewEncoder(w).Encode(out)
}
