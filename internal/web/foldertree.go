package web

import (
	"sort"
	"strings"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/mailconn"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

const maxOpenFolders = 300

// Rôles des dossiers système, dans l'ordre d'affichage de RainLoop : boîte de réception, envoyés, brouillons,
// indésirables, corbeille, archives.
const (
	roleInbox   = "inbox"
	roleSent    = "sent"
	roleDrafts  = "drafts"
	roleJunk    = "junk"
	roleTrash   = "trash"
	roleArchive = "archive"
)

// namespacePrefix renvoie le préfixe d'espace de noms personnel quand tous les dossiers vivent sous la boîte de
// réception (Dovecot avec « INBOX. » ou « INBOX/ ») : ils sont alors présentés sans ce préfixe, comme dans RainLoop.
func namespacePrefix(folders []mailconn.Folder) string {
	for _, f := range folders {
		if f.Delim != 0 && len(f.Name) > 5 && strings.EqualFold(f.Name[:5], "INBOX") && rune(f.Name[5]) == f.Delim {
			return f.Name[:6]
		}
	}
	return ""
}

// folderPanel prépare le panneau de gauche : les dossiers système d'abord (dans l'ordre de RainLoop, d'après les
// choix de la boîte, l'attribut IMAP ou le nom usuel), puis tous les autres en arbre trié, dont les niveaux se
// replient. open est l'ensemble des dossiers dépliés ; les ancêtres du dossier affiché le sont toujours.
func folderPanel(folders []mailconn.Folder, cfg userdata.Folders, active, filter string, open map[string]bool) (rows []FolderRow, moveTargets []string) {
	byName := map[string]mailconn.Folder{}
	for _, f := range folders {
		byName[f.Name] = f
	}
	url := func(name string) string {
		return listParams{Folder: name, Page: 1, Sort: "date", Desc: true, Scope: "all", Filter: filter}.URL()
	}
	system := map[string]bool{}
	addSystem := func(role, name string) {
		f, ok := byName[name]
		if !ok || system[name] || (f.Hidden && name != active && role != roleInbox) {
			return
		}
		system[name] = true
		row := FolderRow{Folder: f, Active: name == active, System: true, Role: role, URL: url(name), Key: name}
		rows = append(rows, row)
	}
	inbox := "INBOX"
	for _, f := range folders {
		if strings.EqualFold(f.Name, "INBOX") {
			inbox = f.Name
		}
	}
	addSystem(roleInbox, inbox)
	if cfg.Sent != userdata.NoFolder {
		addSystem(roleSent, mailconn.ResolveSent(folders, cfg.Sent))
	}
	if cfg.Drafts != userdata.NoFolder {
		addSystem(roleDrafts, mailconn.ResolveDrafts(folders, cfg.Drafts))
	}
	addSystem(roleJunk, mailconn.ResolveJunk(folders, cfg.Junk))
	addSystem(roleTrash, mailconn.ResolveTrash(folders, cfg.Trash))
	addSystem(roleArchive, mailconn.ResolveArchive(folders, cfg.Archive))

	// les autres dossiers : arbre
	prefix := namespacePrefix(folders)
	type node struct {
		f         mailconn.Folder
		path      []string // segments sans le préfixe d'espace de noms
		synthetic bool
	}
	var nodes []node
	seen := map[string]bool{}
	split := func(f mailconn.Folder) []string {
		n := strings.TrimPrefix(f.Name, prefix)
		if f.Delim == 0 {
			return []string{n}
		}
		return strings.Split(n, string(f.Delim))
	}
	for _, f := range folders {
		if system[f.Name] || strings.EqualFold(f.Name, "INBOX") || (f.Hidden && f.Name != active) {
			continue
		}
		nodes = append(nodes, node{f: f, path: split(f)})
		seen[f.Name] = true
	}
	// parents manquants (« A/B » sans « A ») : ajoutés comme dossiers non sélectionnables
	for _, n := range append([]node(nil), nodes...) {
		if n.f.Delim == 0 {
			continue
		}
		for i := 1; i < len(n.path); i++ {
			full := prefix + strings.Join(n.path[:i], string(n.f.Delim))
			if seen[full] || system[full] {
				continue
			}
			seen[full] = true
			nodes = append(nodes, node{f: mailconn.Folder{Name: full, Delim: n.f.Delim, Attrs: []imap.MailboxAttr{imap.MailboxAttrNoSelect}}, path: n.path[:i], synthetic: true})
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i].path, nodes[j].path
		for k := 0; k < len(a) && k < len(b); k++ {
			if !strings.EqualFold(a[k], b[k]) {
				return mailconn.NaturalLess(a[k], b[k]) // « Projet 2 » avant « Projet 10 »
			}
		}
		return len(a) < len(b)
	})
	// ancêtres du dossier actif : toujours dépliés
	forced := map[string]bool{}
	if a, ok := byName[active]; ok && a.Delim != 0 && !system[active] {
		parts := strings.Split(strings.TrimPrefix(active, prefix), string(a.Delim))
		for i := 1; i < len(parts); i++ {
			forced[prefix+strings.Join(parts[:i], string(a.Delim))] = true
		}
	}
	start := len(rows)
	for _, n := range nodes {
		depth := len(n.path) - 1
		row := FolderRow{Folder: n.f, DepthClass: min(depth, 5), Depth: depth, Active: n.f.Name == active, Key: n.f.Name, Label: n.path[len(n.path)-1]}
		if n.f.Selectable() {
			row.URL = url(n.f.Name)
		}
		rows = append(rows, row)
	}
	// enfants, état ouvert, visibilité
	tree := rows[start:]
	for i := range tree {
		if i+1 < len(tree) && tree[i+1].Depth > tree[i].Depth {
			tree[i].HasChildren = true
			tree[i].Open = open[tree[i].Key] || forced[tree[i].Key]
			for j := i + 1; j < len(tree) && tree[j].Depth > tree[i].Depth; j++ { // un descendant non lu : le parent le signale
				if tree[j].Unseen > 0 {
					tree[i].SubUnread = true
					break
				}
			}
		}
	}
	openAt := map[int]bool{}
	for i := range tree {
		d := tree[i].Depth
		visible := true
		for k := 0; k < d; k++ {
			if !openAt[k] {
				visible = false
				break
			}
		}
		tree[i].Collapsed = !visible
		openAt[d] = tree[i].HasChildren && tree[i].Open
	}
	for i := range rows {
		rows[i].Badge, rows[i].Bold = folderBadge(rows[i])
	}
	for _, f := range folders {
		if f.Selectable() && f.Name != active && !(f.Hidden) {
			moveTargets = append(moveTargets, f.Name)
		}
	}
	return rows, moveTargets
}

// openFolderSet renvoie les dossiers dépliés d'une boîte mail.
func openFolderSet(set userdata.Settings, account string) map[string]bool {
	m := map[string]bool{}
	for _, n := range set.OpenFolders[account] {
		m[n] = true
	}
	return m
}

// moveRows prépare le menu « déplacer vers » / « copier vers » comme le panneau : dossiers système d'abord (noms traduits),
// un séparateur, puis l'arbre indenté ; le dossier affiché est grisé (on ne déplace pas un message vers lui-même).
func moveRows(rows []FolderRow) []FolderRow {
	var out []FolderRow
	sepDone := false
	for _, r := range rows {
		if !r.Selectable() || r.Hidden {
			continue
		}
		r.Collapsed = false
		if !r.System && !sepDone && len(out) > 0 {
			r.SepBefore = true
		}
		if !r.System {
			sepDone = true
		}
		out = append(out, r)
	}
	return out
}

// folderBadge applique la règle de RainLoop : les brouillons affichent le nombre total de messages, la corbeille,
// l'archive et les messages envoyés n'affichent rien, les autres dossiers le nombre de messages non lus.
func folderBadge(r FolderRow) (badge int, bold bool) {
	switch r.Role {
	case roleDrafts:
		return int(r.Total), false
	case roleTrash, roleArchive, roleSent:
		return 0, false
	}
	return int(r.Unseen), r.Unseen > 0
}
