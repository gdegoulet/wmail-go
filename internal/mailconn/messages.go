package mailconn

import (
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message"
	_ "wmail-go/internal/mailview" // installe le décodeur de charsets de repli (message.CharsetReader)
)

var (
	// ErrNotFound : le message n'existe plus dans ce dossier.
	ErrNotFound = errors.New("message introuvable")
	// ErrTooLarge : le message dépasse la taille maximale affichable.
	ErrTooLarge = errors.New("message trop volumineux")
)

// maxClientSort est le nombre maximal de messages triés côté serveur Go quand le serveur IMAP n'offre pas SORT.
const maxClientSort = 5000

// Conn est une connexion IMAP authentifiée prêtée par le Pool.
type Conn struct {
	Client *imapclient.Client
	log    *slog.Logger // nil : aucune trace
	server string       // hôte:port, pour les traces
}

// slowOp : au-delà, une opération de lecture est tracée en niveau info.
const slowOp = time.Second

// trace journalise une opération IMAP : au début (niveau info) pour celles qui modifient la boîte, pour qu'une
// opération bloquée laisse une trace ; à la fin avec la durée (info si modification, erreur ou lenteur, sinon debug).
// Usage : defer c.trace("move", true, "messages", n)(&err)
func (c *Conn) trace(op string, mutating bool, attrs ...any) func(*error) {
	if c.log == nil {
		return func(*error) {}
	}
	attrs = append([]any{"op", op, "serveur", c.server}, attrs...)
	if mutating {
		c.log.Info("imap début", attrs...)
	}
	start := time.Now()
	return func(errp *error) {
		d := time.Since(start)
		a := append(attrs, "ms", d.Milliseconds())
		switch {
		case errp != nil && *errp != nil:
			c.log.Warn("imap échec", append(a, "erreur", shorten((*errp).Error()))...)
		case mutating || d > slowOp:
			c.log.Info("imap", a...)
		default:
			c.log.Debug("imap", a...)
		}
	}
}

// Address est une adresse d'expéditeur ou de destinataire.
type Address struct {
	Name, Email string
}

// Short renvoie le nom, ou l'adresse à défaut.
func (a Address) Short() string {
	if a.Name != "" {
		return a.Name
	}
	return a.Email
}

// Summary est une ligne de la liste des messages.
type Summary struct {
	UID           uint32
	Subject       string
	From, To      []Address
	Date          time.Time // date de l'en-tête Date (tri)
	Arrival       time.Time // date de réception (INTERNALDATE) : c'est celle que la liste affiche, comme RainLoop
	Size          int64
	Seen, Flagged bool
	Answered      bool
	Important     bool     // priorité haute
	ThreadUIDs    []uint32 // tous les messages de la conversation (représentant compris), s'il y en a plusieurs
	ThreadUnseen  bool     // le représentant est lu, mais un autre message de la conversation ne l'est pas
	Forwarded     bool     // mot-clé $Forwarded
	HasAttachment bool
	AttachKind    string // type commun à toutes les pièces jointes (image, pdf, archive, audio, video, texte, tableur, presentation) ; vide : variés
}

// ListQuery décrit une page de la liste.
type ListQuery struct {
	Folder   string
	Page     int // à partir de 1
	PageSize int
	Sort     string // date, arrival, from, subject, size
	Desc     bool
	Search   string
	Scope    string // all, subject, from, body
	// UnreadOnly ne garde que les messages non lus (sans le drapeau \Seen).
	UnreadOnly bool
	// Threads regroupe les messages par conversation (une ligne par conversation) ; ThreadOf n'affiche que la
	// conversation contenant ce message.
	Threads  bool
	ThreadOf uint32
}

// ListResult est une page de messages.
type ListResult struct {
	Total, Page, Pages int
	Messages           []Summary
	// SortFallback : le tri demandé n'a pas pu être appliqué (dossier trop gros sans SORT sur le serveur) ;
	// la liste est triée par ordre d'arrivée.
	SortFallback bool
	// ThreadsUnsupported : le regroupement est demandé mais le serveur ne sait pas faire THREAD.
	ThreadsUnsupported bool
}

// Select ouvre un dossier.
func (c *Conn) Select(folder string, readOnly bool) (_ *imap.SelectData, err error) {
	defer c.trace("select", false, "dossier", folder)(&err)
	return c.Client.Select(folder, &imap.SelectOptions{ReadOnly: readOnly}).Wait()
}

// Folders liste les dossiers, avec les compteurs si withStatus.
func (c *Conn) Folders(withStatus bool) (_ []Folder, err error) {
	defer c.trace("dossiers", false, "statuts", withStatus)(&err)
	return listFolders(c.Client, withStatus)
}

func uidSet(uids []uint32) imap.UIDSet {
	var s imap.UIDSet
	for _, u := range uids {
		s.AddNum(imap.UID(u))
	}
	return s
}

func searchCriteria(q, scope string, unreadOnly bool) *imap.SearchCriteria {
	// Comme RainLoop, les messages marqués \Deleted (supprimés par un autre client sans purge) n'apparaissent pas.
	crit := &imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagDeleted}}
	if unreadOnly {
		crit.NotFlag = []imap.Flag{imap.FlagSeen}
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return crit
	}
	if scope == "" || scope == "all" {
		if free, found := applyOperators(crit, q); found {
			if len(free) > 0 {
				addFast(crit, strings.Join(free, " "))
			}
			return crit
		}
	}
	switch scope {
	case "subject":
		crit.Header = []imap.SearchCriteriaHeaderField{{Key: "Subject", Value: q}}
	case "from":
		crit.Header = []imap.SearchCriteriaHeaderField{{Key: "From", Value: q}}
	case "body":
		crit.Body = []string{q}
	default:
		addFast(crit, q)
	}
	return crit
}

func sortKey(s string) imapclient.SortKey {
	switch s {
	case "arrival":
		return imapclient.SortKeyArrival
	case "from":
		return imapclient.SortKeyFrom
	case "subject":
		return imapclient.SortKeySubject
	case "size":
		return imapclient.SortKeySize
	}
	return imapclient.SortKeyDate
}

// sortedUIDs renvoie les UID du dossier sélectionné qui répondent à la recherche, dans l'ordre demandé.
func (c *Conn) sortedUIDs(crit *imap.SearchCriteria, q ListQuery) (uids []uint32, fallback bool, err error) {
	if c.Client.Caps().Has(imap.CapSort) {
		uids, err = c.Client.UIDSort(&imapclient.SortOptions{
			SearchCriteria: crit,
			SortCriteria:   []imapclient.SortCriterion{{Key: sortKey(q.Sort), Reverse: q.Desc}},
		}).Wait()
		if err == nil {
			return uids, false, nil
		}
		// SORT refusé (critère non pris en charge, par exemple) : repli sur la recherche.
	}
	data, err := c.Client.UIDSearch(crit, nil).Wait()
	if err != nil {
		return nil, false, err
	}
	for _, u := range data.AllUIDs() {
		uids = append(uids, uint32(u))
	}
	slices.Sort(uids) // ordre d'arrivée croissant
	byArrival := func() []uint32 {
		if q.Desc {
			slices.Reverse(uids)
		}
		return uids
	}
	if q.Sort == "arrival" || len(uids) <= 1 {
		return byArrival(), false, nil
	}
	if len(uids) > maxClientSort {
		return byArrival(), true, nil
	}
	msgs, err := c.Client.Fetch(uidSet(uids), &imap.FetchOptions{UID: true, Envelope: true, RFC822Size: true, InternalDate: true}).Collect()
	if err != nil {
		return nil, false, err
	}
	type key struct {
		uid  uint32
		str  string
		date time.Time
		size int64
	}
	keys := make([]key, 0, len(msgs))
	for _, m := range msgs {
		k := key{uid: uint32(m.UID), size: m.RFC822Size, date: m.InternalDate}
		if m.Envelope != nil {
			if !m.Envelope.Date.IsZero() {
				k.date = m.Envelope.Date
			}
			switch q.Sort {
			case "from":
				if len(m.Envelope.From) > 0 {
					a := m.Envelope.From[0]
					k.str = strings.ToLower(cmp(a.Name, a.Mailbox+"@"+a.Host))
				}
			case "subject":
				k.str = baseSubject(m.Envelope.Subject)
			}
		}
		keys = append(keys, k)
	}
	less := func(a, b key) int {
		switch q.Sort {
		case "from", "subject":
			if c := strings.Compare(a.str, b.str); c != 0 {
				return c
			}
		case "size":
			if a.size != b.size {
				if a.size < b.size {
					return -1
				}
				return 1
			}
		default:
			if c := a.date.Compare(b.date); c != 0 {
				return c
			}
		}
		switch {
		case a.uid < b.uid:
			return -1
		case a.uid > b.uid:
			return 1
		}
		return 0
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if q.Desc {
			return less(keys[i], keys[j]) > 0
		}
		return less(keys[i], keys[j]) < 0
	})
	out := make([]uint32, len(keys))
	for i, k := range keys {
		out[i] = k.uid
	}
	return out, false, nil
}

func cmp(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// baseSubject retire les préfixes « Re: », « Fwd: » pour le tri par sujet.
func baseSubject(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for {
		trimmed := s
		for _, p := range []string{"re:", "fw:", "fwd:", "tr:", "réf:"} {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, p))
		}
		if trimmed == s {
			return s
		}
		s = trimmed
	}
}

func hasAttachment(bs imap.BodyStructure) bool {
	has, _ := attachmentInfo(bs)
	return has
}

// attachmentInfo dit s'il y a des pièces jointes et, si elles sont toutes du même genre, lequel.
func attachmentInfo(bs imap.BodyStructure) (has bool, kind string) {
	if bs == nil {
		return false, ""
	}
	mixed := false
	bs.Walk(func(_ []int, part imap.BodyStructure) bool {
		sp, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		d := sp.Disposition()
		mt := strings.ToLower(sp.Type + "/" + sp.Subtype)
		switch mt {
		case "application/pkcs7-signature", "application/x-pkcs7-signature", "application/pgp-signature":
			return true // signature numérique : pas une pièce jointe
		}
		attached := d != nil && strings.EqualFold(d.Value, "attachment")
		named := (d == nil || !strings.EqualFold(d.Value, "inline")) && sp.Filename() != ""
		// Comme RainLoop : toute partie qui n'est ni du texte brut ou HTML ni une image incorporée (cid) est une pièce jointe.
		other := mt != "text/plain" && mt != "text/html" && !(strings.HasPrefix(mt, "image/") && sp.ID != "" && !attached) && !(d != nil && strings.EqualFold(d.Value, "inline") && sp.Filename() == "")
		if !attached && !named && !other {
			return true
		}
		has = true
		k := attachmentKind(sp.Type+"/"+sp.Subtype, sp.Filename())
		switch {
		case kind == "" && !mixed:
			kind = k
		case kind != k:
			kind, mixed = "", true
		}
		return true
	})
	return has, kind
}

// attachmentKind classe une pièce jointe d'après son type MIME, à défaut son extension.
func attachmentKind(mime, name string) string {
	m := strings.ToLower(mime)
	ext := strings.ToLower(name)
	if i := strings.LastIndexByte(ext, '.'); i >= 0 {
		ext = ext[i+1:]
	} else {
		ext = ""
	}
	switch {
	case strings.HasPrefix(m, "image/"):
		return "image"
	case strings.HasPrefix(m, "audio/"):
		return "audio"
	case strings.HasPrefix(m, "video/"):
		return "video"
	case m == "application/pdf" || ext == "pdf":
		return "pdf"
	case strings.Contains(m, "zip") || strings.Contains(m, "tar") || strings.Contains(m, "rar") || strings.Contains(m, "7z") || strings.Contains(m, "gzip") || ext == "zip" || ext == "tar" || ext == "gz" || ext == "rar" || ext == "7z":
		return "archive"
	case strings.Contains(m, "spreadsheet") || strings.Contains(m, "excel") || ext == "xls" || ext == "xlsx" || ext == "ods" || ext == "csv":
		return "tableur"
	case strings.Contains(m, "presentation") || strings.Contains(m, "powerpoint") || ext == "ppt" || ext == "pptx" || ext == "odp":
		return "presentation"
	case strings.HasPrefix(m, "text/") || ext == "txt" || ext == "md" || ext == "doc" || ext == "docx" || ext == "odt" || ext == "rtf":
		return "texte"
	}
	return ""
}

// fixUTF8 relit en ISO-8859-1 un en-tête qui n'est pas de l'UTF-8 valide (serveurs ou expéditeurs non conformes),
// pour afficher « réunion » plutôt qu'un caractère de remplacement.
// decodeWords décode les mots encodés RFC 2047 que le serveur laisse tels quels dans l'ENVELOPE
// (« =?utf-8?q?Pinterest?= »), y compris entre guillemets.
func decodeWords(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	if dec, err := (&mime.WordDecoder{CharsetReader: message.CharsetReader}).DecodeHeader(s); err == nil && dec != "" {
		// Le serveur garde parfois les guillemets de la chaîne d'origine : ils n'ont plus lieu d'être.
		if len(dec) > 1 && strings.HasPrefix(dec, `"`) && strings.HasSuffix(dec, `"`) {
			dec = dec[1 : len(dec)-1]
		}
		return dec
	}
	return s
}

func fixUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	r := make([]rune, len(s))
	for i := 0; i < len(s); i++ {
		r[i] = rune(s[i])
	}
	return string(r)
}

func summaryOf(m *imapclient.FetchMessageBuffer) Summary {
	s := Summary{UID: uint32(m.UID), Size: m.RFC822Size, Date: m.InternalDate, Arrival: m.InternalDate}
	s.HasAttachment, s.AttachKind = attachmentInfo(m.BodyStructure)
	if e := m.Envelope; e != nil {
		s.Subject = decodeWords(fixUTF8(e.Subject))
		if !e.Date.IsZero() {
			s.Date = e.Date
		}
		for _, a := range e.From {
			s.From = append(s.From, Address{Name: decodeWords(fixUTF8(a.Name)), Email: fixUTF8(a.Addr())})
		}
		for _, a := range e.To {
			s.To = append(s.To, Address{Name: decodeWords(fixUTF8(a.Name)), Email: fixUTF8(a.Addr())})
		}
	}
	for _, sec := range m.BodySection {
		s.Important = s.Important || highPriority(sec.Bytes)
	}
	for _, f := range m.Flags {
		switch f {
		case imap.FlagSeen:
			s.Seen = true
		case imap.FlagFlagged:
			s.Flagged = true
		case imap.FlagAnswered:
			s.Answered = true
		case imap.Flag("$Forwarded"):
			s.Forwarded = true
		}
	}
	return s
}

// ListMessages renvoie une page de messages d'un dossier (recherche et tri côté serveur IMAP quand il le permet).
func (c *Conn) ListMessages(q ListQuery) (_ *ListResult, err error) {
	defer c.trace("liste", false, "dossier", q.Folder, "page", q.Page, "taille", q.PageSize, "recherche", q.Search != "", "non_lus", q.UnreadOnly, "portee", q.Scope)(&err)
	if q.PageSize < 1 {
		q.PageSize = 50
	}
	sel, err := c.Select(q.Folder, true)
	if err != nil {
		return nil, err
	}
	res := &ListResult{Page: 1, Pages: 1}
	if sel.NumMessages == 0 {
		return res, nil
	}
	uids, fallback, err := c.sortedUIDs(searchCriteria(q.Search, q.Scope, q.UnreadOnly), q)
	if err != nil {
		return nil, err
	}
	res.SortFallback = fallback
	var threads map[uint32][]uint32 // représentant → membres
	if q.Threads || q.ThreadOf != 0 {
		if !c.hasThreads() {
			res.ThreadsUnsupported = true
		} else if uids, threads, err = c.groupThreads(searchCriteria(q.Search, q.Scope, q.UnreadOnly), uids, q); err != nil {
			return nil, err
		}
	}
	res.Total = len(uids)
	res.Pages = max(1, (res.Total+q.PageSize-1)/q.PageSize)
	res.Page = min(max(q.Page, 1), res.Pages)
	if res.Total == 0 {
		return res, nil
	}
	lo := (res.Page - 1) * q.PageSize
	page := uids[lo:min(lo+q.PageSize, len(uids))]

	msgs, err := c.Client.Fetch(uidSet(page), &imap.FetchOptions{
		UID: true, Flags: true, Envelope: true, RFC822Size: true, InternalDate: true,
		BodyStructure: &imap.FetchItemBodyStructure{},
		BodySection:   []*imap.FetchItemBodySection{{Specifier: imap.PartSpecifierHeader, HeaderFields: []string{"Importance", "X-Priority", "X-MSMail-Priority"}, Peek: true}},
	}).Collect()
	if err != nil {
		return nil, err
	}
	byUID := make(map[uint32]*imapclient.FetchMessageBuffer, len(msgs))
	for _, m := range msgs {
		byUID[uint32(m.UID)] = m
	}
	// Conversations : un drapeau « non lu » ou « suivi » d'un membre compte pour toute la conversation.
	var members []uint32
	for _, u := range page {
		members = append(members, threads[u]...)
	}
	flags := map[uint32][]imap.Flag{}
	if len(members) > 0 {
		if fm, err := c.Client.Fetch(uidSet(members), &imap.FetchOptions{UID: true, Flags: true}).Collect(); err == nil {
			for _, m := range fm {
				flags[uint32(m.UID)] = m.Flags
			}
		}
	}
	for _, u := range page {
		if m := byUID[u]; m != nil {
			sum := summaryOf(m)
			if mem := threads[u]; len(mem) > 1 {
				sum.ThreadUIDs = mem
				repSeen := sum.Seen
				for _, mu := range mem {
					if fl, ok := flags[mu]; ok {
						if !slices.Contains(fl, imap.FlagSeen) {
							sum.Seen = false
						}
						if slices.Contains(fl, imap.FlagFlagged) {
							sum.Flagged = true
						}
					}
				}
				// Le représentant est lu mais un message plus ancien du fil ne l'est pas :
				// RainLoop grise la ligne autrement que les vrais non lus.
				sum.ThreadUnseen = repSeen && !sum.Seen
			}
			res.Messages = append(res.Messages, sum)
		}
	}
	return res, nil
}

func (c *Conn) hasThreads() bool {
	return c.Client.Caps().Has(imap.Cap("THREAD=REFERENCES")) || c.Client.Caps().Has(imap.Cap("THREAD=ORDEREDSUBJECT"))
}

func flattenThread(t imapclient.ThreadData, out *[]uint32) {
	*out = append(*out, t.Chain...)
	for _, s := range t.SubThreads {
		flattenThread(s, out)
	}
}

// groupThreads regroupe les UID triés (uids) par conversation. Sans q.ThreadOf : une entrée par conversation, au rang
// de son premier message dans l'ordre de tri, avec ses membres ; avec q.ThreadOf : les seuls membres de la conversation
// de ce message, dans l'ordre de tri.
func (c *Conn) groupThreads(crit *imap.SearchCriteria, uids []uint32, q ListQuery) ([]uint32, map[uint32][]uint32, error) {
	alg := imap.ThreadReferences
	if !c.Client.Caps().Has(imap.Cap("THREAD=REFERENCES")) {
		alg = imap.ThreadOrderedSubject
	}
	data, err := c.Client.UIDThread(&imapclient.ThreadOptions{Algorithm: alg, SearchCriteria: crit}).Wait()
	if err != nil {
		return nil, nil, err
	}
	groupOf := map[uint32]int{}
	var groups [][]uint32
	for _, t := range data {
		var mem []uint32
		flattenThread(t, &mem)
		for _, u := range mem {
			groupOf[u] = len(groups)
		}
		groups = append(groups, mem)
	}
	if q.ThreadOf != 0 {
		gi, ok := groupOf[q.ThreadOf]
		if !ok {
			return nil, nil, nil
		}
		in := map[uint32]bool{}
		for _, u := range groups[gi] {
			in[u] = true
		}
		var out []uint32
		for _, u := range uids {
			if in[u] {
				out = append(out, u)
			}
		}
		return out, nil, nil
	}
	seen := map[int]bool{}
	var out []uint32
	threads := map[uint32][]uint32{}
	for _, u := range uids {
		gi, ok := groupOf[u]
		if !ok {
			out = append(out, u)
			continue
		}
		if seen[gi] {
			continue
		}
		seen[gi] = true
		out = append(out, u)
		var mem []uint32
		for _, m := range groups[gi] { // seuls les messages de la recherche (uids) comptent
			mem = append(mem, m)
		}
		threads[u] = mem
	}
	return out, threads, nil
}

// RawMessage est un message complet.
type RawMessage struct {
	UID   uint32
	Flags []imap.Flag
	Raw   []byte
	// Arrival : date de réception (INTERNALDATE), celle que RainLoop affiche dans l'en-tête du message.
	Arrival time.Time
	// JustRead : l'ouverture vient de marquer le message comme lu (renseigné par l'appelant).
	JustRead bool
}

// HasFlag indique si le message porte le drapeau.
func (r *RawMessage) HasFlag(f imap.Flag) bool { return slices.Contains(r.Flags, f) }

// FetchMessage télécharge un message sans le marquer comme lu. maxSize borne la taille acceptée.
func (c *Conn) FetchMessage(folder string, uid uint32, readOnly bool, maxSize int64) (_ *RawMessage, err error) {
	defer c.trace("message", false, "dossier", folder, "uid", uid)(&err)
	if _, err := c.Select(folder, readOnly); err != nil {
		return nil, err
	}
	meta, err := c.Client.Fetch(uidSet([]uint32{uid}), &imap.FetchOptions{UID: true, Flags: true, RFC822Size: true, InternalDate: true}).Collect()
	if err != nil {
		return nil, err
	}
	if len(meta) == 0 || uint32(meta[0].UID) != uid {
		return nil, ErrNotFound
	}
	if maxSize > 0 && meta[0].RFC822Size > maxSize {
		return nil, fmt.Errorf("%w (%d octets)", ErrTooLarge, meta[0].RFC822Size)
	}
	body, err := c.Client.Fetch(uidSet([]uint32{uid}), &imap.FetchOptions{
		UID: true, BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	}).Collect()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || len(body[0].BodySection) == 0 {
		return nil, ErrNotFound
	}
	return &RawMessage{UID: uid, Flags: meta[0].Flags, Raw: body[0].BodySection[0].Bytes, Arrival: meta[0].InternalDate}, nil
}

// StoreFlags ajoute, retire ou remplace des drapeaux (le dossier est ouvert en écriture).
func (c *Conn) StoreFlags(folder string, uids []uint32, op imap.StoreFlagsOp, flags ...imap.Flag) (err error) {
	if len(uids) == 0 {
		return nil
	}
	defer c.trace("drapeaux", true, "dossier", folder, "messages", len(uids), "drapeaux", flags)(&err)
	sel, err := c.Select(folder, false)
	if err != nil {
		return err
	}
	flags = permittedFlags(sel, flags)
	if len(flags) == 0 {
		return nil // le serveur ne garde aucun de ces drapeaux dans ce dossier
	}
	return c.Client.Store(uidSet(uids), &imap.StoreFlags{Op: op, Silent: true, Flags: flags}, nil).Close()
}

// permittedFlags ne garde que les drapeaux que le dossier accepte durablement (PERMANENTFLAGS).
// Un serveur qui annonce \* accepte tout ; un serveur muet est supposé tout accepter.
func permittedFlags(sel *imap.SelectData, flags []imap.Flag) []imap.Flag {
	if sel == nil || len(sel.PermanentFlags) == 0 {
		return flags
	}
	allowed := map[imap.Flag]bool{}
	for _, f := range sel.PermanentFlags {
		if f == imap.FlagWildcard {
			return flags
		}
		allowed[imap.Flag(strings.ToLower(string(f)))] = true
	}
	out := flags[:0:0]
	for _, f := range flags {
		if allowed[imap.Flag(strings.ToLower(string(f)))] {
			out = append(out, f)
		}
	}
	return out
}

// Move déplace des messages vers un autre dossier (MOVE, ou COPY + suppression si le serveur n'a pas MOVE).
func (c *Conn) Move(folder string, uids []uint32, dest string) (err error) {
	if len(uids) == 0 {
		return nil
	}
	defer c.trace("deplacer", true, "dossier", folder, "vers", dest, "messages", len(uids))(&err)
	if _, err := c.Select(folder, false); err != nil {
		return err
	}
	set := uidSet(uids)
	if c.Client.Caps().Has(imap.CapMove) {
		if _, err = c.Client.Move(set, dest).Wait(); err != nil && c.createMissing(dest, err) {
			_, err = c.Client.Move(set, dest).Wait()
		}
		return err
	}
	// Sans MOVE, on enchaîne COPY, STORE puis EXPUNGE en attendant chaque réponse. Le repli de la bibliothèque
	// envoie les trois commandes d'un coup : au-delà de 128 messages, les réponses EXPUNGE remplissent un canal
	// que personne ne vide avant la fin du COPY, la lecture réseau reste bloquée et la connexion ne se ferme plus.
	if _, err = c.Client.Copy(set, dest).Wait(); err != nil {
		if !c.createMissing(dest, err) {
			return err
		}
		if _, err = c.Client.Copy(set, dest).Wait(); err != nil {
			return err
		}
	}
	return c.deleteAndExpunge(set)
}

// createMissing crée le dossier quand le serveur signale qu'il n'existe pas (TRYCREATE ou NONEXISTENT) :
// la corbeille, les archives ou les indésirables peuvent manquer à la première utilisation.
// Il renvoie vrai si le dossier a été créé et que l'opération peut être retentée.
func (c *Conn) createMissing(name string, err error) bool {
	var ie *imap.Error
	if !errors.As(err, &ie) || (ie.Code != imap.ResponseCodeTryCreate && ie.Code != imap.ResponseCodeNonExistent) {
		return false
	}
	if !ValidFolderName(name) || c.Client.Create(name, nil).Wait() != nil {
		return false
	}
	_ = c.Client.Subscribe(name).Wait()
	if c.log != nil {
		c.log.Info("dossier créé automatiquement", "dossier", name)
	}
	return true
}

// deleteAndExpunge marque les messages \Deleted puis les supprime (avec UIDPLUS, seulement ceux-là).
func (c *Conn) deleteAndExpunge(set imap.UIDSet) error {
	if err := c.Client.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
		return err
	}
	if c.Client.Caps().Has(imap.CapUIDPlus) {
		return c.Client.UIDExpunge(set).Close()
	}
	return c.Client.Expunge().Close()
}

// Expunge supprime définitivement des messages. Sans UIDPLUS, le serveur supprime tous les messages
// du dossier portant le drapeau \Deleted, y compris ceux marqués auparavant par un autre client.
func (c *Conn) Expunge(folder string, uids []uint32) (err error) {
	if len(uids) == 0 {
		return nil
	}
	defer c.trace("supprimer", true, "dossier", folder, "messages", len(uids))(&err)
	if _, err := c.Select(folder, false); err != nil {
		return err
	}
	return c.deleteAndExpunge(uidSet(uids))
}

// ClearFolder supprime définitivement tous les messages du dossier et renvoie leur nombre
// (drapeau \Deleted sur toute la plage d'UID, puis EXPUNGE).
func (c *Conn) ClearFolder(folder string) (_ int, err error) {
	defer c.trace("vider", true, "dossier", folder)(&err)
	sel, err := c.Select(folder, false)
	if err != nil {
		return 0, err
	}
	n := int(sel.NumMessages)
	if n == 0 {
		return 0, nil
	}
	var all imap.UIDSet
	all.AddRange(1, 0) // 1:*
	if err := c.Client.Store(all, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
		return 0, err
	}
	if err := c.Client.Expunge().Close(); err != nil {
		return 0, err
	}
	return n, nil
}

// ValidFolderName refuse les noms qui ne peuvent pas désigner un dossier précis.
func ValidFolderName(name string) bool {
	if name == "" || len(name) > 255 || strings.ContainsAny(name, "*%") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// EnsureFolder crée le dossier s'il n'existe pas.
func (c *Conn) EnsureFolder(name string) (err error) {
	defer c.trace("creer-dossier", false, "dossier", name)(&err)
	if !ValidFolderName(name) {
		return fmt.Errorf("nom de dossier invalide : %q", name)
	}
	list, err := c.Client.List("", name, nil).Collect()
	if err != nil {
		return err
	}
	for _, d := range list {
		if d.Mailbox == name {
			return nil
		}
	}
	return c.Client.Create(name, nil).Wait()
}

// Append dépose un message dans un dossier (brouillon, copie d'un message envoyé) et renvoie son UID, ou 0 si le
// serveur ne l'indique pas (sans UIDPLUS) et que la recherche par Message-ID ne le retrouve pas.
func (c *Conn) Append(folder string, raw []byte, messageID string, flags ...imap.Flag) (uint32, error) {
	return c.AppendAt(folder, raw, messageID, time.Time{}, flags...)
}

// AppendAt ajoute un message en fixant sa date interne (INTERNALDATE) : l'import d'un .eml garde la date d'origine.
func (c *Conn) AppendAt(folder string, raw []byte, messageID string, when time.Time, flags ...imap.Flag) (uid uint32, err error) {
	defer c.trace("ajouter", true, "dossier", folder, "octets", len(raw))(&err)
	cmd := c.Client.Append(folder, int64(len(raw)), &imap.AppendOptions{Flags: flags, Time: when})
	if _, err = cmd.Write(raw); err != nil {
		_ = cmd.Close()
		return 0, err
	}
	if err = cmd.Close(); err != nil {
		return 0, err
	}
	data, err := cmd.Wait()
	if err != nil {
		if !c.createMissing(folder, err) {
			return 0, err
		}
		return c.AppendAt(folder, raw, messageID, when, flags...)
	}
	if data != nil && data.UID != 0 {
		return uint32(data.UID), nil
	}
	if messageID == "" {
		return 0, nil
	}
	return c.findByMessageID(folder, messageID)
}

// findByMessageID cherche l'UID d'un message par son en-tête Message-ID (0 si absent).
func (c *Conn) findByMessageID(folder, messageID string) (uint32, error) {
	if _, err := c.Select(folder, true); err != nil {
		return 0, err
	}
	res, err := c.Client.UIDSearch(&imap.SearchCriteria{Header: []imap.SearchCriteriaHeaderField{{Key: "Message-Id", Value: messageID}}}, nil).Wait()
	if err != nil {
		return 0, err
	}
	uids := res.AllUIDs()
	if len(uids) == 0 {
		return 0, nil
	}
	return uint32(uids[len(uids)-1]), nil
}

// CreateFolder crée un dossier (et l'abonne : la plupart des clients s'attendent à le voir apparaître).
func (c *Conn) CreateFolder(name string) (err error) {
	defer c.trace("creer-dossier", true, "dossier", name)(&err)
	if !ValidFolderName(name) {
		return fmt.Errorf("nom de dossier invalide : %q", name)
	}
	name = c.withNamespace(name)
	if err = c.Client.Create(name, nil).Wait(); err != nil {
		return err
	}
	_ = c.Client.Subscribe(name).Wait() // sans importance si le serveur refuse
	return nil
}

// withNamespace ajoute le préfixe de l'espace de noms personnel (« INBOX. » sur Courier, « INBOX/ » sur certains
// Cyrus) quand le serveur en annonce un et que le nom ne l'a pas déjà : sans lui, la création échoue ou crée un
// dossier hors de la hiérarchie visible.
func (c *Conn) withNamespace(name string) string {
	if !c.Client.Caps().Has(imap.CapNamespace) {
		return name
	}
	data, err := c.Client.Namespace().Wait()
	if err != nil || data == nil || len(data.Personal) == 0 {
		return name
	}
	prefix := data.Personal[0].Prefix
	if prefix == "" || strings.HasPrefix(name, prefix) || strings.EqualFold(name, "INBOX") {
		return name
	}
	return prefix + name
}

// RenameFolder renomme un dossier (ses sous-dossiers suivent, selon le serveur).
func (c *Conn) RenameFolder(oldName, newName string) (err error) {
	defer c.trace("renommer-dossier", true, "dossier", oldName, "vers", newName)(&err)
	if !ValidFolderName(oldName) || !ValidFolderName(newName) || strings.EqualFold(oldName, "INBOX") {
		return fmt.Errorf("nom de dossier invalide")
	}
	return c.Client.Rename(oldName, newName, nil).Wait()
}

// DeleteFolder supprime un dossier ET ses messages. INBOX ne peut jamais être supprimée.
func (c *Conn) DeleteFolder(name string) (err error) {
	defer c.trace("supprimer-dossier", true, "dossier", name)(&err)
	if !ValidFolderName(name) || strings.EqualFold(name, "INBOX") {
		return fmt.Errorf("nom de dossier invalide")
	}
	_ = c.Client.Unsubscribe(name).Wait()
	return c.Client.Delete(name).Wait()
}

// SetSubscribed abonne ou désabonne un dossier.
func (c *Conn) SetSubscribed(name string, on bool) (err error) {
	defer c.trace("abonnement", true, "dossier", name, "abonne", on)(&err)
	if !ValidFolderName(name) {
		return fmt.Errorf("nom de dossier invalide")
	}
	if on {
		return c.Client.Subscribe(name).Wait()
	}
	return c.Client.Unsubscribe(name).Wait()
}

// Subscribed renvoie les noms des dossiers abonnés. known est faux quand le serveur ne sait pas le dire
// (sans LIST-EXTENDED) : on doit alors considérer tous les dossiers comme abonnés.
func (c *Conn) Subscribed() (set map[string]bool, known bool, err error) {
	if !c.Client.Caps().Has(imap.CapListExtended) {
		return nil, false, nil
	}
	list, err := c.Client.List("", "*", &imap.ListOptions{SelectSubscribed: true}).Collect()
	if err != nil {
		return nil, false, err
	}
	set = make(map[string]bool, len(list))
	for _, d := range list {
		if slices.Contains(d.Attrs, imap.MailboxAttrSubscribed) {
			set[d.Mailbox] = true
		}
	}
	return set, true, nil
}

// Quota renvoie l'occupation de la boîte (en octets) et sa limite quand le serveur annonce l'extension QUOTA et fixe
// une limite de stockage. ok est faux dans tous les autres cas (jamais une erreur : l'information est facultative).
func (c *Conn) Quota() (used, limit int64, ok bool) {
	if !c.Client.Caps().Has(imap.CapQuota) {
		return 0, 0, false
	}
	var err error
	defer c.trace("quota", false)(&err)
	data, err := c.Client.GetQuotaRoot("INBOX").Wait()
	if err != nil {
		return 0, 0, false
	}
	for _, d := range data {
		if r, found := d.Resources[imap.QuotaResourceStorage]; found && r.Limit > 0 {
			return r.Usage * 1024, r.Limit * 1024, true // le quota de stockage est exprimé en Kio (RFC 9208)
		}
	}
	return 0, 0, false
}

// MarkAllRead marque tous les messages d'un dossier comme lus et renvoie leur nombre.
func (c *Conn) MarkAllRead(folder string) (int, error) { return c.markAll(folder, true) }

// MarkAllUnread marque tous les messages d'un dossier comme non lus et renvoie leur nombre.
func (c *Conn) MarkAllUnread(folder string) (int, error) { return c.markAll(folder, false) }

func (c *Conn) markAll(folder string, seen bool) (n int, err error) {
	op := imap.StoreFlagsAdd
	if !seen {
		op = imap.StoreFlagsDel
	}
	defer c.trace("tout-marquer", true, "dossier", folder, "lu", seen)(&err)
	sel, err := c.Select(folder, false)
	if err != nil {
		return 0, err
	}
	if sel.NumMessages == 0 {
		return 0, nil
	}
	if len(permittedFlags(sel, []imap.Flag{imap.FlagSeen})) == 0 {
		return 0, nil
	}
	var all imap.UIDSet
	all.AddRange(1, 0) // 1:*
	if err = c.Client.Store(all, &imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{imap.FlagSeen}}, nil).Close(); err != nil {
		return 0, err
	}
	return int(sel.NumMessages), nil
}

// Copy copie des messages vers un autre dossier (ils restent dans le dossier d'origine).
func (c *Conn) Copy(folder string, uids []uint32, dest string) (err error) {
	if len(uids) == 0 {
		return nil
	}
	defer c.trace("copier", true, "dossier", folder, "vers", dest, "messages", len(uids))(&err)
	if _, err = c.Select(folder, false); err != nil {
		return err
	}
	_, err = c.Client.Copy(uidSet(uids), dest).Wait()
	return err
}

// highPriority lit les en-têtes de priorité d'un message : Importance: high, X-Priority: 1 ou 2, X-MSMail-Priority: high.
func highPriority(hdr []byte) bool {
	for _, line := range strings.Split(strings.ReplaceAll(string(hdr), "\r", ""), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.ToLower(strings.TrimSpace(v))
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "importance", "x-msmail-priority":
			if v == "high" {
				return true
			}
		case "x-priority":
			if strings.HasPrefix(v, "1") || strings.HasPrefix(v, "2") {
				return true
			}
		}
	}
	return false
}

// addFast ajoute la recherche simple de RainLoop : le texte est cherché dans l'expéditeur, les destinataires (À, Cc) ou
// l'objet, jamais dans le corps (le critère TEXT du serveur lit tous les messages : lent, et d'autres résultats). Pour le
// corps : « text:mot » ou la portée « corps ».
func addFast(crit *imap.SearchCriteria, q string) {
	h := func(k string) imap.SearchCriteria {
		return imap.SearchCriteria{Header: []imap.SearchCriteriaHeaderField{{Key: k, Value: q}}}
	}
	cc := imap.SearchCriteria{Or: [][2]imap.SearchCriteria{{h("Cc"), h("Subject")}}}
	to := imap.SearchCriteria{Or: [][2]imap.SearchCriteria{{h("To"), cc}}}
	crit.Or = append(crit.Or, [2]imap.SearchCriteria{h("From"), to})
}
