package web

import (
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"

	"github.com/gdegoulet/wmail-go/internal/mailconn"
	"github.com/gdegoulet/wmail-go/internal/userdata"
)

func mkFolders(delim rune, names ...string) []mailconn.Folder {
	fs := make([]mailconn.Folder, len(names))
	for i, n := range names {
		fs[i] = mailconn.Folder{Name: n, Delim: delim}
	}
	return fs
}

func keys(rows []FolderRow) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Key)
	}
	return strings.Join(out, "|")
}

func TestFolderPanelSystemOrderLikeRainLoop(t *testing.T) {
	fs := mkFolders('/', "Archive", "Zèbre", "Deleted Items", "Brouillons", "Perso", "INBOX", "Sent Items", "Junk E-mail")
	fs[3].Attrs = []imap.MailboxAttr{imap.MailboxAttrDrafts}
	rows, _ := folderPanel(fs, userdata.Folders{}, "INBOX", "all", nil)
	var sys []string
	for _, r := range rows {
		if r.System {
			sys = append(sys, r.Role+":"+r.Key)
		}
	}
	want := "inbox:INBOX,sent:Sent Items,drafts:Brouillons,junk:Junk E-mail,trash:Deleted Items,archive:Archive"
	if got := strings.Join(sys, ","); got != want {
		t.Errorf("dossiers système :\n got %s\nwant %s", got, want)
	}
	if got := keys(rows[len(sys):]); got != "Perso|Zèbre" {
		t.Errorf("les autres dossiers, triés : %s", got)
	}
	// un choix explicite prime ; « - » retire un dossier du panneau
	rows, _ = folderPanel(fs, userdata.Folders{Trash: "Perso", Sent: userdata.NoFolder}, "INBOX", "all", nil)
	var roles []string
	for _, r := range rows {
		if r.System {
			roles = append(roles, r.Role+":"+r.Key)
		}
	}
	if got := strings.Join(roles, ","); !strings.Contains(got, "trash:Perso") || strings.Contains(got, "sent:") {
		t.Errorf("choix explicites : %s", got)
	}
}

func TestFolderPanelTreeAndCollapse(t *testing.T) {
	fs := mkFolders('/', "INBOX", "Projets", "Projets/2025", "Projets/2026", "Projets/2026/Q1", "Autre", "Orphelin/Fils")
	rows, _ := folderPanel(fs, userdata.Folders{}, "INBOX", "all", nil)
	tree := rows[1:]
	if got := keys(tree); got != "Autre|Orphelin|Orphelin/Fils|Projets|Projets/2025|Projets/2026|Projets/2026/Q1" {
		t.Errorf("ordre de l'arbre : %s", got)
	}
	byKey := map[string]FolderRow{}
	for _, r := range tree {
		byKey[r.Key] = r
	}
	if r := byKey["Orphelin"]; r.Selectable() || !r.HasChildren {
		t.Errorf("parent manquant : %+v", r)
	}
	if r := byKey["Projets/2026"]; !r.HasChildren || r.Depth != 1 || r.Label != "2026" {
		t.Errorf("niveau 2 : %+v", r)
	}
	// tout est replié au départ : seuls les niveaux 0 sont visibles
	for _, r := range tree {
		if (r.Depth == 0) == r.Collapsed {
			t.Errorf("%s : visibilité initiale (collapsed=%v)", r.Key, r.Collapsed)
		}
	}
	// un parent ouvert montre ses enfants directs, pas ses petits-enfants
	rows, _ = folderPanel(fs, userdata.Folders{}, "INBOX", "all", map[string]bool{"Projets": true})
	byKey = map[string]FolderRow{}
	for _, r := range rows {
		byKey[r.Key] = r
	}
	if byKey["Projets/2025"].Collapsed || byKey["Projets/2026"].Collapsed || !byKey["Projets/2026/Q1"].Collapsed {
		t.Errorf("Projets ouvert : 2025=%v 2026=%v Q1=%v", byKey["Projets/2025"].Collapsed, byKey["Projets/2026"].Collapsed, byKey["Projets/2026/Q1"].Collapsed)
	}
	// le dossier affiché déplie ses ancêtres, même sans état enregistré
	rows, _ = folderPanel(fs, userdata.Folders{}, "Projets/2026/Q1", "all", nil)
	byKey = map[string]FolderRow{}
	for _, r := range rows {
		byKey[r.Key] = r
	}
	if r := byKey["Projets/2026/Q1"]; r.Collapsed || !r.Active {
		t.Errorf("dossier actif caché : %+v", r)
	}
	if !byKey["Projets"].Open || !byKey["Projets/2026"].Open {
		t.Error("les ancêtres du dossier actif doivent être dépliés")
	}
}

func TestFolderPanelNamespacePrefixIsStripped(t *testing.T) {
	// Dovecot avec l'espace de noms « INBOX. » : les dossiers sont présentés sans ce préfixe
	fs := mkFolders('.', "INBOX", "INBOX.Proxmox", "INBOX.Proxmox.Logs", "INBOX.Sent", "INBOX.Divers")
	rows, targets := folderPanel(fs, userdata.Folders{}, "INBOX", "all", nil)
	var user []FolderRow
	for _, r := range rows {
		if !r.System {
			user = append(user, r)
		}
	}
	if len(user) != 3 || user[0].Label != "Divers" || user[1].Label != "Proxmox" || user[1].Depth != 0 || user[2].Label != "Logs" || user[2].Depth != 1 {
		t.Errorf("arbre sans préfixe : %+v", user)
	}
	if user[2].Key != "INBOX.Proxmox.Logs" || !user[1].HasChildren {
		t.Errorf("clés complètes conservées : %+v", user[2])
	}
	if !strings.Contains(strings.Join(targets, ","), "INBOX.Proxmox.Logs") {
		t.Errorf("cibles de déplacement : %v", targets)
	}
	// le dossier « Sent » sous INBOX. est un dossier système, pas un doublon dans l'arbre
	for _, r := range user {
		if r.Key == "INBOX.Sent" {
			t.Error("le dossier système apparaît aussi dans l'arbre")
		}
	}
}

func TestFolderPanelHidesUnsubscribedExceptActive(t *testing.T) {
	fs := mkFolders('/', "INBOX", "A", "B")
	fs[1].Hidden = true
	rows, _ := folderPanel(fs, userdata.Folders{}, "INBOX", "all", nil)
	if strings.Contains(keys(rows), "A") {
		t.Errorf("dossier non abonné affiché : %s", keys(rows))
	}
	rows, _ = folderPanel(fs, userdata.Folders{}, "A", "all", nil)
	if !strings.Contains(keys(rows), "A") {
		t.Errorf("le dossier affiché doit rester visible : %s", keys(rows))
	}
}

func TestParentOfUnreadSubfolderIsFlagged(t *testing.T) {
	folders := []mailconn.Folder{
		{Name: "INBOX", Delim: '/', HasCount: true},
		{Name: "Projets", Delim: '/', HasCount: true},
		{Name: "Projets/2026", Delim: '/', HasCount: true, Unseen: 3},
		{Name: "Archives", Delim: '/', HasCount: true},
		{Name: "Archives/Vieux", Delim: '/', HasCount: true},
	}
	rows, _ := folderPanel(folders, userdata.Folders{}, "INBOX", "all", nil)
	flag := map[string]bool{}
	for _, r := range rows {
		flag[r.Key] = r.SubUnread
	}
	if !flag["Projets"] || flag["Archives"] || flag["Projets/2026"] {
		t.Errorf("SubUnread : %v", flag)
	}
}

func TestFolderBadgeFollowsRainLoopRules(t *testing.T) {
	folders := []mailconn.Folder{
		{Name: "INBOX", Delim: '/', HasCount: true, Total: 50, Unseen: 4},
		{Name: "Sent", Delim: '/', HasCount: true, Total: 20, Unseen: 2},
		{Name: "Drafts", Delim: '/', HasCount: true, Total: 3},
		{Name: "Trash", Delim: '/', HasCount: true, Total: 9, Unseen: 9},
		{Name: "Archive", Delim: '/', HasCount: true, Total: 100, Unseen: 7},
		{Name: "Junk", Delim: '/', HasCount: true, Total: 5, Unseen: 5},
		{Name: "Projets", Delim: '/', HasCount: true, Total: 12, Unseen: 3},
	}
	rows, _ := folderPanel(folders, userdata.Folders{Sent: "Sent", Drafts: "Drafts", Trash: "Trash", Archive: "Archive", Junk: "Junk"}, "INBOX", "all", nil)
	badge := map[string]int{}
	bold := map[string]bool{}
	for _, r := range rows {
		badge[r.Key], bold[r.Key] = r.Badge, r.Bold
	}
	want := map[string]int{"INBOX": 4, "Sent": 0, "Drafts": 3, "Trash": 0, "Archive": 0, "Junk": 5, "Projets": 3}
	for name, n := range want {
		if badge[name] != n {
			t.Errorf("pastille de %s : %d, attendu %d", name, badge[name], n)
		}
	}
	if !bold["INBOX"] || bold["Drafts"] || bold["Trash"] || bold["Archive"] || bold["Sent"] {
		t.Errorf("gras : %v", bold)
	}
}
