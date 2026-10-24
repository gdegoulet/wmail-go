package mailconn

import (
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
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

// UnseenMany renvoie le nombre de non lus de plusieurs dossiers. Avec LIST-STATUS (ou IMAP4rev2), une seule
// commande suffit pour tous ; sinon un STATUS par dossier est envoyé, les réponses étant attendues ensemble.
// Un dossier absent de la réponse ne figure pas dans la carte renvoyée (il a pu disparaître entre-temps).
func (c *Conn) UnseenMany(folders []string) (_ map[string]uint32, err error) {
	if len(folders) == 0 {
		return map[string]uint32{}, nil
	}
	defer c.trace("non-lus", false, "dossiers", len(folders))(&err)
	out := make(map[string]uint32, len(folders))
	caps := c.Client.Caps()
	if caps.Has(imap.CapListStatus) || caps.Has(imap.CapIMAP4rev2) {
		want := make(map[string]bool, len(folders))
		for _, f := range folders {
			want[f] = true
		}
		list, err := c.Client.List("", "*", &imap.ListOptions{ReturnStatus: &imap.StatusOptions{NumUnseen: true}}).Collect()
		if err != nil {
			return nil, err
		}
		for _, d := range list {
			if want[d.Mailbox] && d.Status != nil && d.Status.NumUnseen != nil {
				out[d.Mailbox] = *d.Status.NumUnseen
			}
		}
		return out, nil
	}
	cmds := make([]*imapclient.StatusCommand, len(folders))
	for i, f := range folders {
		cmds[i] = c.Client.Status(f, &imap.StatusOptions{NumUnseen: true})
	}
	for i, cmd := range cmds {
		st, err := cmd.Wait() // un dossier disparu n'empêche pas les autres
		if err == nil && st.NumUnseen != nil {
			out[folders[i]] = *st.NumUnseen
		}
	}
	return out, nil
}

// Poll renvoie les compteurs du dossier et, si sinceUID > 0, les nouveaux messages non lus d'UID >= sinceUID.
// Le STATUS précède le SELECT : quand il montre qu'il n'y a rien de neuf (UIDNEXT inchangé ou aucun non lu),
// le dossier n'est pas ouvert du tout, ce qui rend le sondage périodique peu coûteux.
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
