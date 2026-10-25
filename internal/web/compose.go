package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/compose"
	"github.com/gdegoulet/wmail-go/internal/mailconn"
	"github.com/gdegoulet/wmail-go/internal/mailview"
	"github.com/gdegoulet/wmail-go/internal/session"
	"github.com/gdegoulet/wmail-go/internal/store"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const (
	maxComposeBody  = 2 << 20 // texte d'un message
	maxComposeField = 4000    // adresses, objet
	flagForwarded   = imap.Flag("$Forwarded")
	flagMDNSent     = imap.Flag("$MDNSent")
)

// Modes d'ouverture de la composition.
const (
	modeNew       = "new"
	modeReply     = "reply"
	modeReplyAll  = "replyall"
	modeForward   = "forward"
	modeForwardAt = "forwardatt"
	modeDraft     = "draft"
	modeEditNew   = "editasnew"
)

// IdentityView est une identité proposée dans le menu « De ».
type IdentityView struct {
	ID, Label, Signature string
	SignatureHTML        string // signature en HTML assaini (pour l'éditeur HTML)
	Selected             bool
}

// AttView est une pièce jointe affichée dans la composition.
type AttView struct {
	ID, Name, Size string
	FromMessage    bool
}

// ComposeView contient tout ce que la page de composition affiche.
type ComposeView struct {
	ID, Mode                      string
	Identities                    []IdentityView
	To, Cc, Bcc, ReplyTo, Subject string
	Body                          string
	ShowCc, ShowBcc, ShowReplyTo  bool
	Attachments                   []AttView
	DraftFolder, DraftUID         string
	Kind, OrigFolder, OrigUID     string // lien avec le message d'origine (réponse ou transfert)
	InReplyTo, References         string
	Receipt, DSN, Important       bool
	Autosave                      bool
	Back                          string
	MaxAttachMB                   int
	Saved                         string // « Enregistré à hh:mm »
	NoSent, NoDrafts              bool
	HTMLMode, ForceMode           bool          // éditeur HTML actif ; bascule interdite (réglage « forcé »)
	HTML                          template.HTML // contenu initial de l'éditeur HTML (déjà assaini)
	Templates                     []userdata.Template
}

// composeForm est ce que le navigateur envoie à l'envoi ou à l'enregistrement.
type composeForm struct {
	ID, Identity                  string
	To, Cc, Bcc, ReplyTo, Subject string
	Body                          string
	DraftFolder, DraftUID         string
	Kind, OrigFolder, OrigUID     string
	InReplyTo, References         string
	Receipt, DSN, Important       bool
	IsHTML                        bool
	HTML                          string
	ClientIP                      string // adresse du poste qui écrit (X-Originating-IP), renseignée par le gestionnaire
}

func readComposeForm(r *http.Request) (composeForm, bool) {
	f := composeForm{
		ID: r.PostFormValue("compose_id"), Identity: r.PostFormValue("identity"),
		To: r.PostFormValue("to"), Cc: r.PostFormValue("cc"), Bcc: r.PostFormValue("bcc"), ReplyTo: r.PostFormValue("reply_to"),
		Subject: r.PostFormValue("subject"), Body: r.PostFormValue("body"),
		DraftFolder: r.PostFormValue("draft_folder"), DraftUID: r.PostFormValue("draft_uid"),
		Kind: r.PostFormValue("kind"), OrigFolder: r.PostFormValue("orig_folder"), OrigUID: r.PostFormValue("orig_uid"),
		InReplyTo: r.PostFormValue("in_reply_to"), References: r.PostFormValue("references"),
		Receipt: r.PostFormValue("receipt") != "", DSN: r.PostFormValue("dsn") != "", Important: r.PostFormValue("important") != "",
		IsHTML: r.PostFormValue("is_html") == "1", HTML: r.PostFormValue("html"),
	}
	for _, v := range []string{f.ID, f.Identity, f.To, f.Cc, f.Bcc, f.ReplyTo, f.Subject, f.DraftFolder, f.DraftUID, f.Kind, f.OrigFolder, f.OrigUID, f.InReplyTo, f.References} {
		if len(v) > maxComposeField {
			return f, false
		}
	}
	return f, len(f.Body) <= maxComposeBody && len(f.HTML) <= maxComposeBody
}

func netAddrs(l []mailview.Address) []*mail.Address {
	out := make([]*mail.Address, 0, len(l))
	for _, a := range l {
		if a.Email != "" {
			out = append(out, &mail.Address{Name: a.Name, Address: a.Email})
		}
	}
	return out
}

// ownEmails renvoie les adresses de l'utilisateur (identités, identifiant IMAP), en minuscules.
func ownEmails(acc userdata.MailAccount) map[string]bool {
	own := map[string]bool{}
	for _, id := range acc.AllIdentities() {
		own[strings.ToLower(id.Email)] = true
	}
	if strings.Contains(acc.IMAP.Login, "@") {
		own[strings.ToLower(acc.IMAP.Login)] = true
	}
	return own
}

// pickIdentity choisit l'identité d'après le message d'origine : celle qui apparaît le plus tôt dans la liste parmi
// les destinataires (réponse, transfert) ou l'expéditeur (brouillon) ; à défaut l'identité principale.
func pickIdentity(ids []userdata.Identity, m *mailview.Message, mode string) string {
	if m != nil {
		var cands []mailview.Address
		switch mode {
		case modeReply, modeReplyAll, modeForward, modeForwardAt:
			cands = append(append(append(cands, m.To...), m.Cc...), m.DeliveredTo...)
		case modeDraft, modeEditNew:
			cands = append(append(cands, m.From...), m.ReplyTo...)
		}
		best, bestIdx := "", len(ids)
		for _, c := range cands {
			for i, id := range ids {
				if strings.EqualFold(id.Email, c.Email) && i < bestIdx {
					best, bestIdx = id.ID, i
				}
			}
		}
		if best != "" {
			return best
		}
	}
	return ids[0].ID
}

func findIdentity(ids []userdata.Identity, id string) (userdata.Identity, bool) {
	for _, i := range ids {
		if i.ID == id {
			return i, true
		}
	}
	return userdata.Identity{}, false
}

func identityLabel(id userdata.Identity) string {
	if id.Name == "" {
		return id.Email
	}
	return id.Name + " <" + id.Email + ">"
}

// plainSignature renvoie la signature en texte brut (une signature « :HTML: » est convertie) avec ses variables
// remplacées : {{FROM}}, {{FROM-FULL}} (expéditeur du message d'origine), {{DATE}}, {{TIME}}.
func plainSignature(sig string, orig *mailview.Message, lang string, loc *time.Location) string {
	sig = strings.ReplaceAll(sig, "\r", "")
	if strings.HasPrefix(sig, ":HTML:") {
		sig = mailview.HTMLToText(sig[6:])
	}
	from, full := "", ""
	if orig != nil && len(orig.From) > 0 {
		full = orig.From[0].String()
		from = orig.From[0].Short()
		if !strings.Contains(from, " ") && strings.Contains(from, "@") {
			from = from[:strings.Index(from, "@")]
		}
	}
	now := time.Now().In(loc)
	r := strings.NewReplacer("{{FROM-FULL}}", full, "{{FROM}}", from, "{{DATE}}", formatDate(now, now.Add(-72*time.Hour), lang, loc), "{{TIME}}", now.Format("15:04"))
	sig = momentRe.ReplaceAllStringFunc(r.Replace(sig), func(m string) string {
		return momentFormat(momentRe.FindStringSubmatch(m)[1], now)
	})
	return strings.TrimRight(sig, "\n ")
}

var momentRe = regexp.MustCompile(`\{\{MOMENT:([^}]{0,60})\}\}`)

// momentFormat applique un format à la manière de moment.js (jeu de jetons usuel : YYYY, YY, MM, M, DD, D, HH, H, mm, ss).
func momentFormat(layout string, t time.Time) string {
	r := strings.NewReplacer("YYYY", t.Format("2006"), "YY", t.Format("06"), "MM", t.Format("01"), "DD", t.Format("02"),
		"HH", t.Format("15"), "mm", t.Format("04"), "ss", t.Format("05"),
		"M", strconv.Itoa(int(t.Month())), "D", strconv.Itoa(t.Day()), "H", strconv.Itoa(t.Hour()))
	return r.Replace(layout)
}

// withSignature place la signature (séparateur « -- ») après le texte, ou avant la citation.
func withSignature(sig, text string, before bool) string {
	if sig == "" {
		return text
	}
	block := "-- \n" + sig
	if before {
		return "\n\n" + block + "\n" + text
	}
	return text + "\n\n" + block + "\n"
}

func (s *Server) composeLimits() attachLimits {
	c := s.cfg.Compose
	return attachLimits{Each: int64(c.MaxAttachmentMB) << 20, Message: int64(c.MaxMessageMB) << 20, Session: int64(c.MaxSessionMB) << 20, Count: c.MaxAttachments}
}

func (s *Server) attView(list []*pendingAttachment, lang string) []AttView {
	out := make([]AttView, len(list))
	for i, a := range list {
		out[i] = AttView{ID: a.ID, Name: a.Name, Size: humanSize(int64(len(a.Data)), lang), FromMessage: a.FromMessage}
	}
	return out
}

func identityViews(ids []userdata.Identity, selected string, orig *mailview.Message, lang string, loc *time.Location) []IdentityView {
	out := make([]IdentityView, len(ids))
	for i, id := range ids {
		out[i] = IdentityView{ID: id.ID, Label: identityLabel(id), Selected: id.ID == selected, Signature: plainSignature(id.Signature, orig, lang, loc), SignatureHTML: htmlSignature(id.Signature, orig, lang, loc)}
	}
	return out
}

// composeGet ouvre la page de composition : nouveau message, réponse, transfert, reprise de brouillon…
func (s *Server) composeGet(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	q := r.URL.Query()
	mode := q.Get("mode")
	if mode == "" {
		mode = modeNew
	}
	if mode == modeReply || mode == modeReplyAll || mode == modeForward {
		s.rememberReply(sess, mode)
	}
	switch mode {
	case modeNew, modeReply, modeReplyAll, modeForward, modeForwardAt, modeDraft, modeEditNew:
	default:
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	lang := s.lang(r, sess)
	ids := acc.AllIdentities()
	v := &ComposeView{ID: randomID(), Mode: mode, Back: safeBack(q.Get("back")), MaxAttachMB: s.cfg.Compose.MaxAttachmentMB}
	sess.Mu.Lock()
	set := sess.Account.Data.Settings
	sess.Mu.Unlock()
	v.Autosave = !set.NoDraftAutosave
	v.NoSent = acc.Options.Folders.Sent == userdata.NoFolder
	v.NoDrafts = acc.Options.Folders.Drafts == userdata.NoFolder

	var orig *mailview.Message
	var rawOrig *mailconn.RawMessage
	var folder string
	type fwdMsg struct {
		raw     *mailconn.RawMessage
		subject string
	}
	var moreRaw []fwdMsg // transfert groupé : messages suivants, joints en .eml
	if mode != modeNew {
		var uid uint32
		var okp bool
		if folder, uid, okp = s.messageParams(w, r, sess, q); !okp {
			return
		}
		ctx, cancel := s.mailCtx(r)
		defer cancel()
		var err error
		rawOrig, orig, err = s.fetchMessage(ctx, sess, acc, folder, uid, false)
		if err != nil {
			s.mailErr(w, r, sess, err)
			return
		}
		v.OrigFolder, v.OrigUID = folder, fmt.Sprint(uid)
		if mode == modeForwardAt {
			// « uids » : plusieurs messages transférés d'un coup, chacun en pièce jointe.
			for _, u := range extraUIDs(q.Get("uids"), uid) {
				rm, pm, err := s.fetchMessage(ctx, sess, acc, folder, u, false)
				if err != nil {
					s.mailErr(w, r, sess, err)
					return
				}
				moreRaw = append(moreRaw, fwdMsg{raw: rm, subject: pm.Subject})
			}
		}
	}
	selected := pickIdentity(ids, orig, mode)
	ident, _ := findIdentity(ids, selected)
	loc := s.userLoc(sess)
	sig := plainSignature(ident.Signature, orig, lang, loc)
	own := ownEmails(acc)
	origText := ""
	if orig != nil {
		origText = orig.Text
		if origText == "" && orig.HTML != "" {
			origText = mailview.HTMLToText(orig.HTML)
		}
	}
	addAtt := func(name, ct string, data []byte, fromMsg bool) {
		_, _ = s.attach.Add(sess.ID, v.ID, pendingAttachment{Name: name, ContentType: ct, Data: data, FromMessage: fromMsg})
	}
	copyAttachments := func() {
		for i := range orig.Attachments {
			if a, data, ok := orig.Attachment(i); ok {
				addAtt(a.Name, a.ContentType, data, true)
			}
		}
	}
	whoDate := func() string {
		who := ""
		if len(orig.From) > 0 {
			who = orig.From[0].String()
		}
		return s.i18n.T(lang, "compose.reply_attribution", orig.Date.In(loc).Format("02/01/2006 15:04"), who)
	}

	switch mode {
	case modeNew:
		v.To, v.Cc, v.Bcc = clip(q.Get("to")), clip(q.Get("cc")), clip(q.Get("bcc"))
		v.Subject = clip(q.Get("subject"))
		bodyText := clipBody(q.Get("body"))
		if m := q.Get("mailto"); m != "" {
			mt := parseMailto(m)
			v.To, v.Cc, v.Bcc, v.Subject, bodyText = mt.To, mt.Cc, mt.Bcc, mt.Subject, mt.Body
		}
		v.Body = withSignature(sig, bodyText, false)
	case modeReply, modeReplyAll:
		if mode == modeReply {
			v.To = compose.FormatList(compose.ReplyRecipients(netAddrs(orig.ReplyTo), netAddrs(orig.From), own))
		} else {
			to, cc := compose.ReplyAll(netAddrs(orig.ReplyTo), netAddrs(orig.From), netAddrs(orig.To), netAddrs(orig.Cc), own)
			v.To, v.Cc = compose.FormatList(to), compose.FormatList(cc)
		}
		v.Subject = compose.ReplySubject("Re", orig.Subject)
		v.Kind, v.InReplyTo = "reply", orig.MessageID
		v.References = strings.Join(compose.References(orig.MessageID, orig.References), " ")
		quote := "\n\n" + whoDate() + "\n" + compose.Quote(origText) + "\n"
		v.Body = withSignature(sig, quote, ident.SignatureBefore)
	case modeForward:
		v.Subject = compose.ReplySubject("Fwd", orig.Subject)
		v.Kind, v.InReplyTo = "forward", orig.MessageID
		v.References = strings.Join(compose.References(orig.MessageID, orig.References), " ")
		v.Body = withSignature(sig, compose.ForwardText(s.forwardHeader(orig, lang, loc), origText), ident.SignatureBefore)
		copyAttachments()
	case modeForwardAt:
		v.Subject = compose.ReplySubject("Fwd", orig.Subject)
		v.Kind, v.InReplyTo = "forward", orig.MessageID
		v.References = strings.Join(compose.References(orig.MessageID, orig.References), " ")
		v.Body = withSignature(sig, "\n", false)
		name := strings.TrimSpace(orig.Subject)
		if name == "" {
			name = "message"
		}
		addAtt(safeEMLName(name), "message/rfc822", rawOrig.Raw, true)
		for i, fm := range moreRaw {
			n := strings.TrimSpace(fm.subject)
			if n == "" {
				n = fmt.Sprintf("message-%d", i+2)
			}
			addAtt(safeEMLName(n), "message/rfc822", fm.raw.Raw, true)
		}
		if len(moreRaw) > 0 {
			v.Subject = s.i18n.T(lang, "compose.forward_many", len(moreRaw)+1)
		}
	case modeDraft, modeEditNew:
		v.To, v.Cc, v.Bcc = compose.FormatList(netAddrs(orig.To)), compose.FormatList(netAddrs(orig.Cc)), compose.FormatList(netAddrs(orig.Bcc))
		v.ReplyTo = compose.FormatList(netAddrs(orig.ReplyTo))
		v.Subject = orig.Subject
		v.Body = origText
		copyAttachments()
		if mode == modeDraft {
			v.DraftFolder, v.DraftUID = folder, v.OrigUID
			v.InReplyTo = orig.InReplyTo
			v.References = strings.Join(orig.References, " ")
			v.OrigFolder, v.OrigUID = "", ""
			if di, ok := compose.ParseDraftInfo(orig.DraftInfo); ok {
				v.Kind, v.OrigFolder, v.OrigUID = di.Kind, di.Folder, fmt.Sprint(di.UID)
			}
			v.Receipt = len(orig.ReadReceiptTo) > 0
		} else {
			v.OrigFolder, v.OrigUID = "", ""
		}
	}
	v.HTMLMode, v.ForceMode = editorMode(set.Editor, orig, mode)
	if v.HTMLMode {
		sigH := sigBlock(htmlSignature(ident.Signature, orig, lang, loc))
		var body string
		switch mode {
		case modeNew:
			bodyText := clipBody(q.Get("body"))
			if m := q.Get("mailto"); m != "" {
				bodyText = parseMailto(m).Body
			}
			body = "<br>" + compose.TextToHTMLBody(bodyText) + "<br>" + sigH
		case modeReply, modeReplyAll:
			quote := "<br><br><div>" + compose.Escape(whoDate()) + "</div>" + compose.QuoteHTML(quotedHTML(orig))
			if ident.SignatureBefore {
				body = "<br><br>" + sigH + quote
			} else {
				body = quote + "<br>" + sigH
			}
		case modeForward:
			head := compose.TextToHTMLBody(strings.TrimSpace(compose.ForwardText(s.forwardHeader(orig, lang, loc), "")))
			quote := "<br><br>" + head + "<br><br>" + quotedHTML(orig)
			if ident.SignatureBefore {
				body = "<br><br>" + sigH + quote
			} else {
				body = quote + "<br>" + sigH
			}
		case modeForwardAt:
			body = "<br>" + sigH
		default: // brouillon, modifier comme nouveau
			body = quotedHTML(orig)
		}
		v.HTML = template.HTML(body)
	}
	v.ShowCc, v.ShowBcc, v.ShowReplyTo = v.Cc != "", v.Bcc != "", v.ReplyTo != ""
	v.Identities = identityViews(ids, selected, orig, lang, loc)
	v.Attachments = s.attView(s.attach.List(sess.ID, v.ID), lang)
	s.renderCompose(w, r, sess, http.StatusOK, acc, v, "")
}

func clip(s string) string {
	if len(s) > maxComposeField {
		return s[:maxComposeField]
	}
	return s
}

func clipBody(s string) string {
	if len(s) > 20000 {
		return s[:20000]
	}
	return s
}

// extraUIDs lit la liste « uids » (séparés par des virgules) sans le message déjà chargé ; 20 au plus.
func extraUIDs(list string, first uint32) []uint32 {
	var out []uint32
	for _, part := range strings.Split(list, ",") {
		u, ok := parseUID(strings.TrimSpace(part))
		if !ok || u == first || slices.Contains(out, u) {
			continue
		}
		out = append(out, u)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func safeEMLName(subject string) string {
	subject = strings.Map(func(r rune) rune {
		if r < 0x20 || r == '/' || r == '\\' || r == ':' || r == '"' {
			return ' '
		}
		return r
	}, subject)
	if r := []rune(strings.TrimSpace(subject)); len(r) > 80 {
		subject = string(r[:80])
	}
	return strings.TrimSpace(subject) + ".eml"
}

func (s *Server) forwardHeader(m *mailview.Message, lang string, loc *time.Location) compose.ForwardHeader {
	list := func(l []mailview.Address) string {
		parts := make([]string, len(l))
		for i, a := range l {
			parts[i] = a.String()
		}
		return strings.Join(parts, ", ")
	}
	t := func(k string) string { return s.i18n.T(lang, k) }
	return compose.ForwardHeader{
		Title: t("compose.fwd.title"), From: t("compose.fwd.from"), To: t("compose.fwd.to"), Cc: t("compose.fwd.cc"), Date: t("compose.fwd.date"), Subject: t("compose.fwd.subject"),
		FromV: list(m.From), ToV: list(m.To), CcV: list(m.Cc), SubjectV: m.Subject, DateV: m.Date.In(loc),
	}
}

func (s *Server) renderCompose(w http.ResponseWriter, r *http.Request, sess *session.Session, status int, acc userdata.MailAccount, v *ComposeView, errKey string, errArg ...any) {
	sess.Mu.Lock()
	v.Templates = append([]userdata.Template(nil), sess.Account.Data.Templates...)
	sess.Mu.Unlock()
	p := &Page{TitleKey: "compose.title", Compose: v, ActiveID: acc.ID, HideNav: true, ErrorKey: errKey}
	if len(errArg) > 0 {
		p.ErrorArg = errArg[0]
	}
	s.render(w, r, sess, status, "compose", p)
}

// viewFromForm reconstruit la page à partir de ce que l'utilisateur avait saisi (après une erreur).
func (s *Server) viewFromForm(r *http.Request, sess *session.Session, acc userdata.MailAccount, f composeForm) *ComposeView {
	lang := s.lang(r, sess)
	sess.Mu.Lock()
	set := sess.Account.Data.Settings
	sess.Mu.Unlock()
	v := &ComposeView{
		ID: f.ID, Mode: modeNew, To: f.To, Cc: f.Cc, Bcc: f.Bcc, ReplyTo: f.ReplyTo, Subject: f.Subject, Body: f.Body,
		DraftFolder: f.DraftFolder, DraftUID: f.DraftUID, Kind: f.Kind, OrigFolder: f.OrigFolder, OrigUID: f.OrigUID,
		InReplyTo: f.InReplyTo, References: f.References, Receipt: f.Receipt, DSN: f.DSN, Important: f.Important,
		Back: safeBack(r.PostFormValue("back")), MaxAttachMB: s.cfg.Compose.MaxAttachmentMB, Autosave: !set.NoDraftAutosave,
		NoSent: acc.Options.Folders.Sent == userdata.NoFolder, NoDrafts: acc.Options.Folders.Drafts == userdata.NoFolder,
	}
	v.ShowCc, v.ShowBcc, v.ShowReplyTo = f.Cc != "", f.Bcc != "", f.ReplyTo != ""
	v.HTMLMode = f.IsHTML
	v.ForceMode = set.Editor == userdata.EditorHTMLForced || set.Editor == userdata.EditorPlainForced
	if f.IsHTML {
		clean, _ := mailview.Sanitize(f.HTML, mailview.SanitizeOptions{BlockRemote: true})
		v.HTML = template.HTML(clean)
	}
	v.Identities = identityViews(acc.AllIdentities(), f.Identity, nil, lang, s.userLoc(sess))
	v.Attachments = s.attView(s.attach.List(sess.ID, f.ID), lang)
	return v
}

// composeError traduit une erreur de composition en clé de message.
func composeErrKey(err error) (key string, arg any) {
	var (
		te   *mailconn.TestError
		se   *mailconn.SendError
		size *sizeError
	)
	switch {
	case errors.Is(err, compose.ErrNoRecipient), errors.Is(err, mailconn.ErrNoRecipients):
		return "compose.err.no_recipient", nil
	case errors.Is(err, compose.ErrInvalidAddress):
		return "compose.err.address", err.Error()
	case errors.As(err, &size):
		return "compose.err.too_large", size.limit
	case errors.As(err, &se) && len(se.Rejected) > 0:
		return "compose.err.rejected", strings.Join(se.Rejected, ", ")
	case errors.As(err, &se):
		return "compose.err.send", shortErr(se.Err)
	case errors.As(err, &te):
		return "accounts.test.fail." + te.Kind, nil
	}
	return "compose.err.send", shortErr(err)
}

func shortErr(err error) string {
	s := strings.Join(strings.Fields(err.Error()), " ")
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}

type sizeError struct{ limit string }

func (e *sizeError) Error() string { return "message trop volumineux" }

// buildOutgoing assemble le message à partir du formulaire.
func (s *Server) buildOutgoing(sess *session.Session, acc userdata.MailAccount, f composeForm, forDraft bool) (*compose.Message, userdata.Identity, error) {
	ident, ok := findIdentity(acc.AllIdentities(), f.Identity)
	if !ok {
		return nil, ident, fmt.Errorf("%w : identité inconnue", compose.ErrInvalidAddress)
	}
	m := &compose.Message{
		From:    &mail.Address{Name: ident.Name, Address: ident.Email},
		Subject: f.Subject, Text: f.Body, Important: f.Important, ReadReceipt: f.Receipt,
		InReplyTo: f.InReplyTo, References: strings.Fields(f.References),
	}
	if !s.cfg.Compose.HideXMailer {
		m.XMailer = "wmail-go"
	}
	if s.cfg.Compose.XOriginatingIP {
		m.OriginatingIP = f.ClientIP
		if m.OriginatingIP == "::1" {
			m.OriginatingIP = "127.0.0.1"
		}
	}
	var err error
	for _, p := range []struct {
		dst *[]*mail.Address
		src string
	}{{&m.To, f.To}, {&m.Cc, f.Cc}, {&m.Bcc, f.Bcc}, {&m.ReplyTo, f.ReplyTo}} {
		if *p.dst, err = compose.ParseList(p.src); err != nil {
			return nil, ident, err
		}
	}
	// Identité : adresse de réponse et Cci automatiques si le message n'en précise pas.
	if len(m.ReplyTo) == 0 && ident.ReplyTo != "" {
		m.ReplyTo, _ = compose.ParseList(ident.ReplyTo)
	}
	if ident.Bcc != "" && !forDraft {
		extra, _ := compose.ParseList(ident.Bcc)
		m.Bcc = compose.Dedup(append(m.Bcc, extra...), nil)
	}
	if !forDraft && len(m.Recipients()) == 0 {
		return nil, ident, compose.ErrNoRecipient
	}
	if f.Kind != "" && f.OrigFolder != "" {
		if uid, ok := parseUID(f.OrigUID); ok && mailconn.ValidFolderName(f.OrigFolder) && (f.Kind == "reply" || f.Kind == "forward") {
			m.Draft = &compose.DraftInfo{Kind: f.Kind, UID: uid, Folder: f.OrigFolder}
		}
	}
	var total int64
	for _, a := range s.attach.List(sess.ID, f.ID) {
		total += int64(len(a.Data))
		m.Attachments = append(m.Attachments, compose.Attachment{Name: a.Name, ContentType: a.ContentType, Data: a.Data})
	}
	bodyLen := int64(len(f.Body))
	if f.IsHTML {
		html, imgs := compose.PrepareHTML(f.HTML, nil)
		m.HTML, m.Images, m.Text = html, imgs, compose.HTMLToPlain(html)
		bodyLen = int64(len(html))
		for _, im := range imgs {
			bodyLen += int64(len(im.Data))
		}
	}
	if limit := int64(s.cfg.Compose.MaxMessageMB) << 20; total+bodyLen > limit {
		return nil, ident, &sizeError{limit: fmt.Sprint(s.cfg.Compose.MaxMessageMB)}
	}
	return m, ident, nil
}

// composeSend envoie le message, en conserve une copie et marque le message d'origine.
func (s *Server) composeSend(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	f, ok := readComposeForm(r)
	f.ClientIP = s.clientIP(r)
	if !ok {
		s.message(w, r, sess, http.StatusRequestEntityTooLarge, "", "mail.err.too_large")
		return
	}
	login := store.ReduceLogin(sess.Account.Login)
	ipKey, userKey := s.limitKeys("send", r, sess.Account.Login)
	if wait, blocked := s.checkLimits(s.cfg.RateLimit.Send, ipKey, userKey); blocked {
		s.log.Warn("envoi bloqué par la limitation de débit", "login", login, "ip", s.clientIP(r))
		s.rateLimited(w, r, sess, wait)
		return
	}
	fail := func(status int, err error) {
		key, arg := composeErrKey(err)
		s.log.Warn("envoi refusé", "login", login, "raison", key)
		s.renderCompose(w, r, sess, status, acc, s.viewFromForm(r, sess, acc, f), key, arg)
	}
	msg, ident, err := s.buildOutgoing(sess, acc, f, false)
	if err != nil {
		fail(http.StatusBadRequest, err)
		return
	}
	if f.ID == "" {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	raw, err := msg.Build(false)
	if err != nil {
		fail(http.StatusBadRequest, err)
		return
	}
	copyRaw := raw
	if len(msg.Bcc) > 0 {
		if copyRaw, err = msg.Build(true); err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
	}

	start := time.Now()
	s.addFailure(s.cfg.RateLimit.Send, ipKey, userKey) // compte chaque envoi (pas seulement les échecs)
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.Timeouts.Send)*time.Second)
	defer cancel()
	if err := s.guard.SendSMTP(ctx, smtpEndpoint(acc), ident.Email, msg.Recipients(), raw, f.DSN); err != nil {
		fail(http.StatusBadGateway, err)
		return
	}
	s.log.Info("message envoyé", "login", login, "destinataires", len(msg.Recipients()), "pieces_jointes", len(msg.Attachments), "octets", len(raw), "ms", time.Since(start).Milliseconds())

	s.contactsAfterSend(sess, msg.To, msg.Cc)

	// Suites de l'envoi : copie dans « Envoyés », drapeaux du message d'origine, suppression du brouillon.
	// Un échec ici n'annule pas l'envoi : il est signalé à l'utilisateur.
	copyFailed := false
	if err := s.afterSend(ctx, sess, acc, f, msg, copyRaw); err != nil {
		copyFailed = true
		s.log.Warn("suites de l'envoi en échec", "login", login, "err", shortErr(err))
	}
	s.attach.Clear(sess.ID, f.ID)
	flash := "compose.sent"
	if copyFailed {
		flash = "compose.sent_not_saved"
	}
	s.setFlash(sess, flash)
	http.Redirect(w, r, safeBack(r.PostFormValue("back")), http.StatusSeeOther)
}

func (s *Server) afterSend(ctx context.Context, sess *session.Session, acc userdata.MailAccount, f composeForm, msg *compose.Message, copyRaw []byte) error {
	sess.Mu.Lock()
	replySame := sess.Account.Data.Settings.ReplySameFolder
	sess.Mu.Unlock()
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	err := s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		folders, err := c.Folders(false)
		if err != nil {
			return err
		}
		target := mailconn.ResolveSent(folders, acc.Options.Folders.Sent)
		if replySame && msg.Draft != nil && msg.Draft.Kind == "reply" && mailconn.ValidFolderName(msg.Draft.Folder) {
			target = msg.Draft.Folder
		}
		if acc.Options.Folders.Sent != userdata.NoFolder {
			if err := c.EnsureFolder(target); err != nil {
				note(err)
			} else if _, err := c.Append(target, copyRaw, msg.MessageID, imap.FlagSeen); err != nil {
				note(err)
			}
		}
		if msg.Draft != nil {
			flag := imap.FlagAnswered
			if msg.Draft.Kind == "forward" {
				flag = flagForwarded
			}
			// Un message d'origine disparu ou déplacé n'est pas une erreur d'envoi.
			_ = c.StoreFlags(msg.Draft.Folder, []uint32{msg.Draft.UID}, imap.StoreFlagsAdd, flag)
		}
		if uid, ok := parseUID(f.DraftUID); ok && mailconn.ValidFolderName(f.DraftFolder) {
			_ = c.Expunge(f.DraftFolder, []uint32{uid})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return firstErr
}

// composeSave enregistre un brouillon dans le dossier des brouillons (en remplaçant le précédent).
func (s *Server) composeSave(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	ajax := r.Header.Get("X-Requested-With") == "fetch"
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	reply := func(status int, key string, arg any, view *ComposeView) {
		if ajax {
			lang := s.lang(r, sess)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			msg := ""
			if key != "" {
				if arg != nil {
					msg = s.i18n.T(lang, key, arg)
				} else {
					msg = s.i18n.T(lang, key)
				}
			}
			out := map[string]any{"error": msg}
			if view != nil {
				out["folder"], out["uid"], out["saved"] = view.DraftFolder, view.DraftUID, view.Saved
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		if view == nil {
			s.renderCompose(w, r, sess, status, acc, s.viewFromForm(r, sess, acc, composeForm{ID: r.PostFormValue("compose_id")}), key, arg)
			return
		}
		s.renderCompose(w, r, sess, status, acc, view, key, arg)
	}
	f, ok := readComposeForm(r)
	f.ClientIP = s.clientIP(r)
	if !ok || f.ID == "" {
		reply(http.StatusRequestEntityTooLarge, "mail.err.too_large", nil, nil)
		return
	}
	if acc.Options.Folders.Drafts == userdata.NoFolder {
		reply(http.StatusBadRequest, "compose.err.no_drafts", nil, s.viewFromForm(r, sess, acc, f))
		return
	}
	msg, _, err := s.buildOutgoing(sess, acc, f, true)
	if err != nil {
		key, arg := composeErrKey(err)
		reply(http.StatusBadRequest, key, arg, s.viewFromForm(r, sess, acc, f))
		return
	}
	msg.MessageID = compose.NewMessageID(msg.From.Address)
	raw, err := msg.Build(true)
	if err != nil {
		key, arg := composeErrKey(err)
		reply(http.StatusBadRequest, key, arg, s.viewFromForm(r, sess, acc, f))
		return
	}
	ctx, cancel := s.mailCtx(r)
	defer cancel()
	var folder string
	var uid uint32
	err = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		folders, err := c.Folders(false)
		if err != nil {
			return err
		}
		folder = mailconn.ResolveDrafts(folders, acc.Options.Folders.Drafts)
		if err := c.EnsureFolder(folder); err != nil {
			return err
		}
		if uid, err = c.Append(folder, raw, msg.MessageID, imap.FlagSeen, imap.FlagDraft); err != nil {
			return err
		}
		if old, ok := parseUID(f.DraftUID); ok && mailconn.ValidFolderName(f.DraftFolder) {
			_ = c.Expunge(f.DraftFolder, []uint32{old}) // l'ancienne version est remplacée
		}
		return nil
	})
	if err != nil {
		key, arg := composeErrKey(err)
		s.log.Warn("brouillon non enregistré", "login", store.ReduceLogin(sess.Account.Login), "err", shortErr(err))
		reply(http.StatusBadGateway, key, arg, s.viewFromForm(r, sess, acc, f))
		return
	}
	f.DraftFolder = folder
	f.DraftUID = ""
	if uid != 0 {
		f.DraftUID = fmt.Sprint(uid)
	}
	view := s.viewFromForm(r, sess, acc, f)
	view.Saved = time.Now().Format("15:04")
	reply(http.StatusOK, "", nil, view)
}

// composeAttach reçoit une pièce jointe (un fichier par requête) et répond en JSON.
func (s *Server) composeAttach(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	respond := func(status int, v map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	lang := s.lang(r, sess)
	fail := func(status int, key string, arg ...any) {
		respond(status, map[string]any{"error": s.i18n.T(lang, key, arg...)})
	}
	compID := r.PostFormValue("compose_id")
	if compID == "" || len(compID) > 64 {
		fail(http.StatusBadRequest, "mail.err.bad_request")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			fail(http.StatusRequestEntityTooLarge, "compose.err.attach_too_big", s.cfg.Compose.MaxAttachmentMB)
			return
		}
		fail(http.StatusBadRequest, "mail.err.bad_request")
		return
	}
	defer file.Close()
	limit := int64(s.cfg.Compose.MaxAttachmentMB) << 20
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		fail(http.StatusRequestEntityTooLarge, "compose.err.attach_too_big", s.cfg.Compose.MaxAttachmentMB)
		return
	}
	ct := hdr.Header.Get("Content-Type")
	a, err := s.attach.Add(sess.ID, compID, pendingAttachment{Name: hdr.Filename, ContentType: ct, Data: data})
	switch {
	case errors.Is(err, errAttachTooBig):
		fail(http.StatusRequestEntityTooLarge, "compose.err.attach_too_big", s.cfg.Compose.MaxAttachmentMB)
	case errors.Is(err, errAttachTooMany):
		fail(http.StatusRequestEntityTooLarge, "compose.err.attach_count", s.cfg.Compose.MaxAttachments)
	case errors.Is(err, errAttachMsgLimit):
		fail(http.StatusRequestEntityTooLarge, "compose.err.too_large", s.cfg.Compose.MaxMessageMB)
	case errors.Is(err, errAttachSession):
		fail(http.StatusInsufficientStorage, "compose.err.attach_session")
	case err != nil:
		fail(http.StatusInternalServerError, "error.generic")
	default:
		respond(http.StatusOK, map[string]any{"id": a.ID, "name": a.Name, "size": humanSize(int64(len(a.Data)), lang)})
	}
}

// composeDetach retire une pièce jointe de la composition.
func (s *Server) composeDetach(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	ok := s.attach.Remove(sess.ID, r.PostFormValue("compose_id"), r.PostFormValue("id"))
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
	}
	_, _ = io.WriteString(w, "{}")
}

// composeDiscard abandonne la composition : pièces jointes oubliées, brouillon enregistré supprimé.
func (s *Server) composeDiscard(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	back := safeBack(r.PostFormValue("back"))
	compID := r.PostFormValue("compose_id")
	if compID != "" {
		s.attach.Clear(sess.ID, compID)
	}
	folder, uid, valid := r.PostFormValue("draft_folder"), r.PostFormValue("draft_uid"), false
	var u uint32
	if u, valid = parseUID(uid); ok && valid && mailconn.ValidFolderName(folder) {
		ctx, cancel := s.mailCtx(r)
		defer cancel()
		_ = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
			return c.Expunge(folder, []uint32{u})
		})
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// setFlash mémorise un message (clé de traduction) à afficher sur la prochaine page.
func (s *Server) setFlash(sess *session.Session, key string) {
	sess.Mu.Lock()
	sess.Flash = key
	sess.Mu.Unlock()
}

// htmlSignature renvoie la signature en HTML assaini : une signature « :HTML: » est assainie, une signature en texte
// brut est échappée avec ses retours à la ligne.
func htmlSignature(sig string, orig *mailview.Message, lang string, loc *time.Location) string {
	sig = strings.ReplaceAll(sig, "\r", "")
	if strings.TrimSpace(sig) == "" {
		return ""
	}
	if strings.HasPrefix(sig, ":HTML:") {
		h := sig[6:]
		h = strings.NewReplacer("{{FROM-FULL}}", compose.Escape(fromFull(orig)), "{{FROM}}", compose.Escape(fromShort(orig))).Replace(h)
		out, _ := mailview.Sanitize(h, mailview.SanitizeOptions{BlockRemote: true})
		return out
	}
	return compose.TextToHTMLBody(plainSignature(sig, orig, lang, loc))
}

func fromFull(m *mailview.Message) string {
	if m == nil || len(m.From) == 0 {
		return ""
	}
	return m.From[0].String()
}

func fromShort(m *mailview.Message) string {
	f := fromFull(m)
	if m == nil || len(m.From) == 0 {
		return f
	}
	f = m.From[0].Short()
	if !strings.Contains(f, " ") && strings.Contains(f, "@") {
		f = f[:strings.Index(f, "@")]
	}
	return f
}

// sigBlock enveloppe une signature HTML dans le bloc repéré par l'éditeur (pour la remplacer au changement d'identité).
func sigBlock(sigHTML string) string {
	if sigHTML == "" {
		return ""
	}
	return `<div data-wmail-sig="1">-- <br>` + sigHTML + `</div>`
}

// editorMode choisit l'éditeur à l'ouverture, comme RainLoop : « forcé » impose le mode ; sinon un message d'origine
// en texte brut donne une réponse en texte brut, et un nouveau message suit le réglage par défaut.
func editorMode(set string, orig *mailview.Message, mode string) (html, forced bool) {
	switch set {
	case userdata.EditorHTMLForced:
		return true, true
	case userdata.EditorPlainForced:
		return false, true
	}
	if orig != nil && mode != modeNew {
		return orig.HTML != "", false
	}
	return set != userdata.EditorPlain, false
}

// quotedHTML renvoie le contenu HTML assaini du message d'origine (son texte brut converti à défaut).
func quotedHTML(orig *mailview.Message) string {
	src := orig.HTML
	if src == "" {
		src = compose.TextToHTMLBody(orig.Text)
	}
	out, _ := mailview.Sanitize(src, mailview.SanitizeOptions{BlockRemote: true, InlineImages: orig.InlineImages})
	return out
}

// _ garde l'import de template pour les vues.
var _ template.HTML

// mailtoFields est le contenu d'un lien mailto: (RFC 6068).
type mailtoFields struct {
	To, Cc, Bcc, Subject, Body string
}

// parseMailto lit un lien mailto: : adresses du chemin et paramètres to, cc, bcc, subject, body. Les retours à la ligne
// sont retirés des champs d'adresse et d'objet (aucune injection d'en-tête) ; tout est borné en taille. Un lien qui
// n'est pas mailto: donne des champs vides.
func parseMailto(raw string) mailtoFields {
	var m mailtoFields
	raw = strings.TrimSpace(raw)
	if len(raw) < 7 || !strings.EqualFold(raw[:7], "mailto:") || len(raw) > 8000 {
		return m
	}
	rest := raw[7:]
	path, query := rest, ""
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		path, query = rest[:i], rest[i+1:]
	}
	oneLine := func(s string) string {
		return clip(strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\r' || r == '\n' || r == 0 }), " "))
	}
	to, _ := url.PathUnescape(path)
	vals, _ := url.ParseQuery(query)
	if extra := vals.Get("to"); extra != "" {
		if to != "" {
			to += ", "
		}
		to += extra
	}
	m.To, m.Cc, m.Bcc, m.Subject = oneLine(to), oneLine(vals.Get("cc")), oneLine(vals.Get("bcc")), oneLine(vals.Get("subject"))
	m.Body = clipBody(vals.Get("body"))
	return m
}

// mailMDN envoie (ou ignore) l'accusé de lecture demandé par un message, puis note la décision sur le message avec le
// mot-clé $MDNSent (RFC 3503) pour ne plus la demander.
func (s *Server) mailMDN(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	acc, _, ok := s.activeAccount(sess)
	if !ok {
		http.Redirect(w, r, "/accounts/new", http.StatusSeeOther)
		return
	}
	action := r.PostFormValue("action")
	folder, uid, ok := s.messageParams(w, r, sess, r.PostForm)
	if !ok {
		return
	}
	if action != "send" && action != "ignore" {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	back := safeBack(r.PostFormValue("stay"))
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.cfg.Timeouts.Send)*time.Second)
	defer cancel()
	raw, m, err := s.fetchMessage(ctx, sess, acc, folder, uid, false)
	if err != nil {
		s.mailErr(w, r, sess, err)
		return
	}
	if len(m.ReadReceiptTo) == 0 || m.ReadReceiptTo[0].Email == "" {
		s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
		return
	}
	if raw.HasFlag(flagMDNSent) { // déjà traité : on ne renvoie rien
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	login := store.ReduceLogin(sess.Account.Login)
	if action == "send" {
		ipKey, userKey := s.limitKeys("send", r, sess.Account.Login)
		if wait, blocked := s.checkLimits(s.cfg.RateLimit.Send, ipKey, userKey); blocked {
			s.rateLimited(w, r, sess, wait)
			return
		}
		s.addFailure(s.cfg.RateLimit.Send, ipKey, userKey)
		ids := acc.AllIdentities()
		ident, _ := findIdentity(ids, pickIdentity(ids, m, modeReply))
		lang := s.lang(r, sess)
		mdn := compose.MDN{
			From: &mail.Address{Name: ident.Name, Address: ident.Email}, To: &mail.Address{Name: m.ReadReceiptTo[0].Name, Address: m.ReadReceiptTo[0].Email},
			OrigSubject: m.Subject, OrigID: m.MessageID, Text: s.i18n.T(lang, "mail.mdn.body"), SubjectLabel: s.i18n.T(lang, "mail.mdn.subject"),
		}
		if !s.cfg.Compose.HideXMailer {
			mdn.XMailer = "wmail-go"
		}
		msg, err := mdn.Build()
		if err != nil {
			s.message(w, r, sess, http.StatusBadRequest, "", "mail.err.bad_request")
			return
		}
		// Enveloppe : expéditeur vide (<>), comme l'exige la norme pour ne jamais provoquer de réponse automatique.
		if err := s.guard.SendSMTP(ctx, smtpEndpoint(acc), "", []string{m.ReadReceiptTo[0].Email}, msg, false); err != nil {
			key, arg := composeErrKey(err)
			s.log.Warn("accusé de lecture non envoyé", "login", login, "raison", key)
			s.message(w, r, sess, http.StatusBadGateway, "", key, arg)
			return
		}
		s.log.Info("accusé de lecture envoyé", "login", login)
		s.setFlash(sess, "mail.mdn.done")
	} else {
		s.setFlash(sess, "mail.mdn.ignored")
	}
	_ = s.pool.Do(ctx, s.poolKey(sess, acc), imapEndpoint(acc), func(c *mailconn.Conn) error {
		return c.StoreFlags(folder, []uint32{uid}, imap.StoreFlagsAdd, flagMDNSent)
	})
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// rememberReply garde la dernière action de réponse (le bouton principal du message la propose ensuite).
func (s *Server) rememberReply(sess *session.Session, mode string) {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	set := &sess.Account.Data.Settings
	if set.LastReply == mode {
		return
	}
	prev := set.LastReply
	set.LastReply = mode
	if err := s.store.SaveData(sess.Account); err != nil {
		set.LastReply = prev
	}
}
