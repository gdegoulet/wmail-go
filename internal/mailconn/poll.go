package mailconn

import (
	"github.com/emersion/go-imap/v2"
)

// PollResult décrit l'état d'un dossier pour le sondage périodique du navigateur.
type PollResult struct {
	Unseen  uint32
	Total   uint32
	UIDNext uint32
	// New : messages non lus arrivés depuis sinceUID (les plus récents d'abord, au plus maxNew).
	New []Summary
}

const maxNewPolled = 10

// Poll renvoie les compteurs du dossier et, si sinceUID > 0, les nouveaux messages non lus d'UID >= sinceUID.
func (c *Conn) Poll(folder string, sinceUID uint32) (_ *PollResult, err error) {
	defer c.trace("sondage", false, "dossier", folder, "depuis", sinceUID)(&err)
	st, err := c.Client.Status(folder, &imap.StatusOptions{NumMessages: true, NumUnseen: true, UIDNext: true}).Wait()
	if err != nil {
		return nil, err
	}
	res := &PollResult{UIDNext: uint32(st.UIDNext)}
	if st.NumMessages != nil {
		res.Total = *st.NumMessages
	}
	if st.NumUnseen != nil {
		res.Unseen = *st.NumUnseen
	}
	if sinceUID == 0 || uint32(st.UIDNext) <= sinceUID || res.Unseen == 0 {
		return res, nil
	}
	if _, err := c.Select(folder, true); err != nil {
		return nil, err
	}
	var set imap.UIDSet
	set.AddRange(imap.UID(sinceUID), 0)
	msgs, err := c.Client.Fetch(set, &imap.FetchOptions{UID: true, Flags: true, Envelope: true, InternalDate: true}).Collect()
	if err != nil {
		return nil, err
	}
	for i := len(msgs) - 1; i >= 0 && len(res.New) < maxNewPolled; i-- {
		m := msgs[i]
		if uint32(m.UID) < sinceUID {
			continue // « n:* » renvoie toujours le dernier message, même plus ancien
		}
		if s := summaryOf(m); !s.Seen {
			res.New = append(res.New, s)
		}
	}
	return res, nil
}
