package mailview

import (
	"strconv"
	"strings"
)

// PartDesc décrit une partie MIME telle que le serveur IMAP l'annonce (BODYSTRUCTURE), sans télécharger le message.
type PartDesc struct {
	Path        []int // chemin IMAP de la partie (1, 1.2…)
	Type        string
	Filename    string
	Disposition string
	ContentID   string
	Encoding    string
	Size        uint32
}

// AttachmentPart est une pièce jointe repérée dans la structure du message.
type AttachmentPart struct {
	Index int // même rang que dans Message.Attachments
	PartDesc
	Name string // nom affiché (celui du message, ou un nom construit comme Parse)
}

// AttachmentParts applique aux descriptions de parties la même règle que Parse : une partie qui n'est ni le corps
// (premier texte brut, premier HTML, textes en ligne suivants), ni une image incorporée (cid:), ni une signature
// détachée est une pièce jointe. L'ordre et les noms sont ceux de Parse : l'indice sert à retrouver la même pièce.
func AttachmentParts(parts []PartDesc) []AttachmentPart {
	var out []AttachmentPart
	haveText, haveHTML := false, false
	for _, p := range parts {
		ct := strings.ToLower(strings.TrimSpace(p.Type))
		disp := strings.ToLower(strings.TrimSpace(p.Disposition))
		inline := disp != "attachment"
		cid := NormalizeCID(p.ContentID)
		switch {
		case inlineImageTypes[ct] && cid != "" && disp != "attachment":
		case inline && ct == "text/plain" && !haveText:
			haveText = true
		case inline && ct == "text/html" && !haveHTML:
			haveHTML = true
		case inline && (ct == "text/plain" || ct == "text/html") && disp != "attachment":
		case isSignaturePart(ct):
		default:
			name := p.Filename
			if name == "" {
				name = defaultName(ct, pathString(p.Path))
			}
			out = append(out, AttachmentPart{Index: len(out), PartDesc: p, Name: name})
		}
	}
	return out
}

// pathString écrit un chemin de partie à la manière d'IMAP (« 1 », « 1.2 »). Un chemin vide désigne un message
// sans structure multipart, dont l'unique partie porte le numéro 1.
func pathString(path []int) string {
	if len(path) == 0 {
		return "1"
	}
	parts := make([]string, len(path))
	for i, n := range path {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}
