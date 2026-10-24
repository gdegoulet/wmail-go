// Package userdata définit le contenu JSON du fichier utilisateur chiffré.
package userdata

// CurrentVersion est la version du schéma JSON.
const CurrentVersion = 1

// Data est le contenu complet du compte webmail d'un utilisateur.
type Data struct {
	Version      int           `json:"version"`
	Settings     Settings      `json:"settings"`
	MailAccounts []MailAccount `json:"mail_accounts"`
	TwoFactor    TwoFactor     `json:"two_factor"`
	Templates    []Template    `json:"templates"`
	ContactsSync ContactsSync  `json:"contacts_sync"`
}

// ContactsSync configure la synchronisation du carnet d'adresses avec un serveur CardDAV.
type ContactsSync struct {
	Enabled         bool   `json:"enabled"`
	URL             string `json:"url"`
	User            string `json:"user"`
	Password        string `json:"password"`
	IntervalMinutes int    `json:"interval_minutes"` // 5 à 320, 20 par défaut
}

// Template est un modèle de message : un texte qu'on insère dans la composition.
type Template struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Body string `json:"body"`
}

// TwoFactor est l'authentification en deux étapes (TOTP) du compte webmail.
type TwoFactor struct {
	Enabled      bool     `json:"enabled"`
	Secret       string   `json:"secret"`        // base32
	LastStep     int64    `json:"last_step"`     // dernier pas de temps accepté (un code ne sert qu'une fois)
	BackupHashes []string `json:"backup_hashes"` // empreintes des codes de secours non utilisés
	TrustKey     string   `json:"trust_key"`     // clé des cookies « ne plus demander » ; changée à chaque (dés)activation
}

// Settings regroupe les préférences de l'utilisateur.
// Une valeur vide ou nulle signifie « utiliser le défaut de config.json ».
type Settings struct {
	Language             string              `json:"language"`
	Theme                string              `json:"theme"`
	SessionIdleMinutes   int                 `json:"session_idle_minutes"`
	SessionAbsoluteHours int                 `json:"session_absolute_hours"`
	MarkReadOnOpen       bool                `json:"mark_read_on_open"`
	PageSize             int                 `json:"page_size"`
	Layout               string              `json:"layout"`               // three (3 volets, défaut) ou list (liste seule)
	FontSize             int                 `json:"font_size"`            // taille du texte de l'interface en pixels (0 = défaut)
	FoldersWidth         int                 `json:"folders_width"`        // largeur du volet des dossiers en pixels (0 = défaut)
	ListWidth            int                 `json:"list_width"`           // largeur du volet de la liste en pixels (0 = défaut)
	Editor               string              `json:"editor"`               // html (défaut), plain, html_forced, plain_forced
	FoldersHidden        bool                `json:"folders_hidden"`       // volet des dossiers replié
	OpenFolders          map[string][]string `json:"open_folders"`         // dossiers dépliés dans l'arbre, par boîte mail
	NoDraftAutosave      bool                `json:"no_draft_autosave"`    // désactive l'enregistrement automatique des brouillons
	Background           int                 `json:"background"`           // version du fond d'écran personnel (0 = aucun)
	Timezone             string              `json:"timezone"`             // fuseau d'affichage des dates (IANA, ex. Europe/Paris) ; vide : celui du serveur
	TrustedSenders       []string            `json:"trusted_senders"`      // expéditeurs dont les images distantes s'affichent d'emblée
	HideCheckboxes       bool                `json:"hide_checkboxes"`      // masque les cases à cocher de la liste (la sélection reste possible au clavier)
	NoAnimations         bool                `json:"no_animations"`        // supprime les transitions de l'interface
	Threads              bool                `json:"threads"`              // regrouper les messages par conversation
	LastReply            string              `json:"last_reply"`           // dernière action de réponse : reply, replyall ou forward
	NotifyDesktop        bool                `json:"notify_desktop"`       // notification de bureau pour les nouveaux messages
	NotifySound          bool                `json:"notify_sound"`         // son pour les nouveaux messages
	NoContactsAutosave   bool                `json:"no_contacts_autosave"` // n'ajoute pas les destinataires inconnus au carnet après un envoi
	ReplySameFolder      bool                `json:"reply_same_folder"`    // copie d'une réponse dans le dossier du message d'origine
}

// SieveConfig décrit l'accès ManageSieve (filtres) d'une boîte. Les champs vides prennent la valeur de l'IMAP :
// même serveur, mêmes identifiants, même vérification du certificat. Les filtres sont proposés par défaut ; Disabled
// les masque pour une boîte dont le serveur n'a pas de ManageSieve.
type SieveConfig struct {
	Enabled    bool   `json:"enabled"` // ancien réglage, ignoré
	Disabled   bool   `json:"disabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`     // 0 : 4190
	Security   string `json:"security"` // starttls (défaut), tls, none
	Login      string `json:"login"`
	Password   string `json:"password"`
	VerifyCert bool   `json:"verify_cert"`
	LegacyTLS  bool   `json:"legacy_tls"`
}

// Valeurs de Settings.Editor.
const (
	EditorHTML        = "html"
	EditorPlain       = "plain"
	EditorHTMLForced  = "html_forced"
	EditorPlainForced = "plain_forced"
)

// ValidEditor indique si v est une valeur d'éditeur connue (vide = défaut).
func ValidEditor(v string) bool {
	switch v {
	case "", EditorHTML, EditorPlain, EditorHTMLForced, EditorPlainForced:
		return true
	}
	return false
}

// Server décrit la connexion à un serveur IMAP ou SMTP.
type Server struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Security   string `json:"security"` // tls, starttls, none
	Login      string `json:"login"`
	Password   string `json:"password"`
	VerifyCert bool   `json:"verify_cert"`
	LegacyTLS  bool   `json:"legacy_tls"` // accepte TLS 1.0 à 1.2 et les chiffrements sans confidentialité persistante
}

// Folders associe les rôles IMAP à des noms de dossiers (vide = détection automatique).
type Folders struct {
	Trash   string `json:"trash"`
	Archive string `json:"archive"`
	Junk    string `json:"junk"`
	Sent    string `json:"sent"`   // « - » : ne pas conserver de copie des messages envoyés
	Drafts  string `json:"drafts"` // « - » : pas de brouillons
}

// NoFolder désigne un rôle volontairement inutilisé (choix « Ne pas utiliser »).
const NoFolder = "-"

// Identity est une adresse d'expéditeur : l'adresse principale d'une boîte, ou une identité supplémentaire.
type Identity struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	ReplyTo         string `json:"reply_to"`
	Bcc             string `json:"bcc"`
	Signature       string `json:"signature"`
	SignatureBefore bool   `json:"signature_before"` // signature avant la citation
}

// MailOptions regroupe les options propres à un compte mail.
type MailOptions struct {
	BlockRemoteContent bool    `json:"block_remote_content"`
	ProxyRemoteImages  bool    `json:"proxy_remote_images"` // images distantes chargées par le serveur (l'expéditeur ne voit pas l'adresse IP du lecteur)
	ImageProxy         bool    `json:"image_proxy"`
	Folders            Folders `json:"folders"`
}

// MailAccount est un compte mail IMAP/SMTP.
type MailAccount struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Email       string      `json:"email"`
	DisplayName string      `json:"display_name"`
	IMAP        Server      `json:"imap"`
	SMTP        Server      `json:"smtp"`
	Sieve       SieveConfig `json:"sieve"`
	Options     MailOptions `json:"options"`
	// Signature et réglages de l'identité principale (celle de Email et DisplayName).
	Signature       string     `json:"signature"`
	SignatureBefore bool       `json:"signature_before"`
	Identities      []Identity `json:"identities"`    // identités supplémentaires
	CheckFolders    []string   `json:"check_folders"` // dossiers dont le compteur de non lus est relevé en arrière-plan (INBOX l'est toujours), 5 au plus
}

// MaxCheckFolders est le nombre de dossiers surveillés en plus de la boîte de réception.
const MaxCheckFolders = 5

// AllIdentities renvoie l'identité principale puis les identités supplémentaires. L'identifiant de la principale
// est celui de la boîte.
func (a MailAccount) AllIdentities() []Identity {
	out := []Identity{{ID: a.ID, Email: a.Email, Name: a.DisplayName, Signature: a.Signature, SignatureBefore: a.SignatureBefore}}
	return append(out, a.Identities...)
}

// New renvoie le contenu d'un compte tout juste créé.
func New() *Data {
	return &Data{
		Version:      CurrentVersion,
		Settings:     Settings{MarkReadOnOpen: true, PageSize: 50},
		MailAccounts: []MailAccount{},
	}
}
