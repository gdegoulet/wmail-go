# Analyse RainLoop 1.17 front-end : `static/js/app.js` lignes 1 à 8000 (modules webpack 0 à 27)

Lecture réelle de `/Work/webmail_project/rainloop/1.17.0/static/js/app.js` (lignes 1-8000), comparée au code lu de wmail-go (`internal/web/static/*.js`, `internal/web/templates/*.html`, `internal/web/*.go`, `internal/mailview`, `internal/compose`). Les numéros de ligne RainLoop sont ceux de `app.js` ; les références wmail-go sont `fichier:ligne` sous `/home/gdegoulet/Work/webmail_project/internal/…`.

Limites honnêtes : le module 27 (`Stores/User/App.js`) est coupé à la ligne 8000 (suite dans la tranche suivante). Le module `Common/Momentor`, `Common/Plugins`, `Model/*`, les vues et `App/User` sont hors tranche : je ne les cite que quand le code de la tranche les appelle. Je n'ai pas lancé l'application : les verdicts « rendu » viennent de la lecture du CSS/HTML (et du CSS RainLoop `static/css/app.css`), pas de captures.

## Découpage de la tranche

| Module | Contenu | Nature |
|---|---|---|
| 0 (l.89) | `Common/Enums` : 34 énumérations | première partie |
| 1 (l.655) | `External/ko` : bindings et extenders knockout | première partie |
| 2 (l.1768) | `Common/Utils` : ~70 fonctions | première partie |
| 3, 4, 7, 21 | `window._` (underscore), `window`, `window.jQuery`, `window.key` (keymaster) | bibliothèques externes (nommées, non analysées) |
| 10, 19 | helpers Babel `inheritsLoose`, `applyDecoratedDescriptor` | build |
| 12, 17, 20 | core-js `find`, `map`, `getOwnPropertyDescriptor` | build |
| 18 | module vide | build |
| 5 | `Storage/Settings` | première partie |
| 6 | `Common/Globals` | première partie |
| 8 | `Knoin/Knoin` (routage, écrans, popups) | première partie |
| 9 | `Common/Translator` | première partie |
| 11 | `Stores/User/Folder` | première partie |
| 13 | `Stores/User/Message` | première partie |
| 14 | `Common/Cache` | première partie |
| 15 | `Common/Links` | première partie |
| 16 | `Remote/User/Ajax` | première partie |
| 22, 23, 24, 25, 26 | `Helper/Apps/User`, `Common/Events`, `Common/Consts`, `Knoin/AbstractViewNext`, `Stores/Social` | première partie |
| 27 (l.7963-8000) | début de `Stores/User/App` | première partie (coupé) |

Architecture wmail-go à garder en tête : application **rendue côté serveur** (Go templates + petits scripts de confort), sans SPA, sans knockout, sans hash-routing, sans popups modales (composition, contacts, réglages = pages). Beaucoup d'unités RainLoop (framework knockout, vues, commandes) sont donc « SANS OBJET : colle technique », mais leur **effet visible** est vérifié séparément quand il existe.

Légende des priorités : P1 = quotidien et clairement visible, P2 = utile/visible mais pas bloquant, P3 = détail ou à confirmer.

---

## 1. Module 0 : `Common/Enums` (l.89-653)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `FileType` (l.103) : unknown, text, html, code, eml, word-text, pdf, image, audio, video, sheet, presentation, certificate, certificate-bin, archive | classe de la pièce jointe : choisit l'icône de type (fontello colorée) et le mode d'aperçu | pièce jointe affichée avec une icône par famille de fichier, aperçu selon le type | Pastille texte avec l'extension (4 caractères max) `mail.go:829-839`, rendue `templates/base.html:301` ; aperçu images/pdf/audio/texte `mail.go:1060-1074`, visionneuse images `static/preview.js` ; pas d'icône par type ; vidéo/eml/certificat non aperçus | PARTIEL | Différence visuelle : badge « PDF » au lieu d'une icône de type. Ajouter une classe CSS par famille (déduite de l'extension et du type MIME) pour colorer/iconifier la pastille. P3 |
| `StorageResultType` (l.123) : success, abort, error, unload | état d'une requête Ajax | savoir si une requête a réussi/avorté | aucun (formulaires et fetch simples, `static/list.js:190-213` gère l'échec par rechargement) | SANS OBJET | Colle Ajax de la SPA. |
| `Focused` (l.133) : none, message-list, message-view, folder-list | zone qui a le focus clavier | raccourcis différents selon la zone active | Curseur de liste `.msgrow.focused` `static/listnav.js:13-17` ; pas de zone « dossiers » au clavier (voir `KeyState`) | PARTIEL | Voir « Navigation clavier dans le panneau des dossiers ». P3 |
| `State` (l.141) : Empty 10, Login 20, Auth 30 | état de l'application | écran de connexion vs boîte | pages `/login`, `/` côté serveur (`server.go:118-121`) | SANS OBJET | Routage serveur. |
| `StateType` (l.149) : Webmail, Admin | distingue l'espace admin | | aucun | SANS OBJET | Panneau d'administration hors périmètre. |
| `Capa` (l.157) : TWO_FACTOR(_FORCE), OPEN_PGP, PREFETCH, GRAVATAR, FOLDERS, COMPOSER, CONTACTS, RELOAD, SEARCH, SEARCH_ADV, MESSAGE(LIST)_ACTIONS, ATTACHMENTS_ACTIONS, DANGEROUS_ACTIONS, SETTINGS, HELP, THEMES, USER_BACKGROUND, SIEVE, FILTERS, ATTACHMENT_THUMBNAILS, TEMPLATES, AUTOLOGOUT, ADDITIONAL_ACCOUNTS, IDENTITIES | interrupteurs de fonctions pilotés par l'administrateur | masquer des fonctions | Toutes les fonctions sont toujours actives (utilisateur unique) : 2FA `server.go:172-177`, recherche avancée `templates/home.html:24-43`, aide `static/keys.js:23-32`, thèmes `handlers.go:268`, fond `server.go:169-171`, Sieve `server.go:178-183`, miniatures `mail.go:934-939`, modèles `server.go:165-168`, déconnexion auto `templates/settings.html:13-16`, comptes `server.go:131-138`, identités `server.go:184-186` | OK | `PREFETCH` (préchargement du message suivant) et `GRAVATAR` : absents (décision documentée `CAHIER_DES_CHARGES.md:595, 893`). `OPEN_PGP`, `TWO_FACTOR_FORCE` : SANS OBJET. Aucune action. |
| `KeyState` (l.173) : all, none, contact-list, message-list, folder-list, message-view, compose, settings, menu, popup-* | portée des raccourcis clavier | un raccourci n'agit que dans sa zone | Une seule portée globale, filtrée par `typing()` `static/keys.js:7-9,35` ; menus déroulants capturent Échap `static/menus.js:13-16` ; pas de portée « liste de dossiers » | PARTIEL | Voir « Navigation clavier dans le panneau des dossiers ». P3 |
| `FolderType` (l.195) : Inbox 10, SentItems 11, Draft 12, Trash 13, Spam 14, Archive 15, NotSpam 80, User 99 | rôle d'un dossier ; `NotSpam` = bouton « Pas un spam » dans le dossier Indésirables | rôles système + bouton NotSpam | Rôles `foldertree.go:58-73` (inbox, sent, drafts, junk, trash, archive) ; **aucun** bouton « pas un spam » : `templates/base.html:113` affiche toujours « spam » (qui, dans Indésirables, ne fait rien : `mail.go:1341-1349` `moveTo` sort si déjà dans la cible) | PARTIEL | Ajouter le bouton « Pas un spam » (déplacement vers INBOX) quand le dossier courant est Indésirables et masquer « spam » dans Brouillons/Envoyés/Indésirables (comportement RainLoop `isSpamVisible`/`isUnSpamVisible`, l.34663-34667). P2 |
| `ServerFolderType` (l.206) : USER, INBOX, SENT, DRAFTS, JUNK, TRASH, IMPORTANT, FLAGGED, ALL | type annoncé par le serveur (SPECIAL-USE) | reconnaissance automatique des dossiers | SPECIAL-USE Sent/Drafts/Junk/Trash/Archive `mail.go:462` et `mailconn.Resolve*` ; IMPORTANT/FLAGGED/ALL (Gmail) non traités | PARTIEL | Seulement utile si le serveur expose \Important/\Flagged/\All ; à confirmer avec le serveur de l'utilisateur. P3 |
| `LoginSignMeTypeAsString` (l.222), `LoginSignMeType` (l.232) | case « se souvenir de moi » | session persistante à la demande | aucune case dans `templates/login.html:8-10` ; durées de session réglables `templates/settings.html:13-16` | PARTIEL | Pas de case ; choix de conception (sessions à durée réglable). À confirmer si l'utilisateur la veut. P3 |
| `ComposeType` (l.241) : empty, reply, replyall, forward, forward-as-attachment, draft, editasnew | mode d'ouverture de la composition | tous les modes | `compose.go:36-44` (reply, replyall, forward, forwardatt, draft, editasnew) | OK | |
| `UploadErrorCode` (l.255) | codes d'erreur d'envoi de fichier | messages d'erreur d'envoi | `fr.json:266-268` (trop gros, trop nombreux, session) + `compose.err.attach_failed` | OK | |
| `SetSystemFoldersNotification` (l.268) | invite à configurer un dossier système quand une action en a besoin | message « choisissez le dossier Corbeille/Spam/… » | Création automatique du dossier de rôle `mail.go:1341-1348` (`EnsureFolder`) | OK | Approche différente, sans invite ; équivalent fonctionnel. |
| `ClientSideKeyName` (l.280) : FoldersLashHash, MessagesInboxLastHash, MailBoxListSize, ExpandedFolders, FolderListSize, MessageListSize, LastReplyAction, LastSignMe, ComposeLastIdentityID, MessageHeaderFullInfo, MessageAttachmnetControls | préférences d'interface mémorisées dans le navigateur | mémoriser largeurs de volets, dossiers dépliés, dernière réponse, détail des en-têtes… | Largeurs des volets : compte (`static/list.js:275-283`, `/settings/panes`) ; dossiers dépliés `static/folders.js:36-42` ; dernière action de réponse `mail.go:870` ; identité choisie d'après le message `compose.go:136-160` ; scroll de liste `static/list.js:124-157` ; **non mémorisés** : ouverture du détail d'en-têtes (`<details class="msg-info">` `templates/base.html:237`), dernière identité utilisée | PARTIEL | Mémoriser l'état ouvert/fermé de `msg-info` (localStorage) ; P3 |
| `EventKeyCode` (l.294) | codes de touches | | Utilisation de `e.key` partout | SANS OBJET | |
| `MessageSetAction` (l.314) : SetSeen, UnsetSeen, SetFlag, UnsetFlag | actions lu/non lu/étoile | marquer | `mail.go:1327-1334` | OK | |
| `MessageSelectAction` (l.325) : All, None, Invert, Unseen, Seen, Flagged, Unflagged | menu « sélectionner » à côté de la case | sélection par critère | `static/list.js:61-79` (all, none, unseen, seen, flagged, unflagged, invert) et `templates/home.html:15` | OK | |
| `DesktopNotification` (l.339) : Allowed, NotAllowed, Denied, NotSupported | état de la permission | demander la permission | `static/notify-setting.js:6-13`, `static/poll.js:29` | OK | |
| `MessagePriority` (l.350) : Low 5, Normal 3, High 1 | priorité du message | « marquer important » | seulement « important » `templates/compose.html:74`, lu `mail.go:46` | OK | RainLoop 1.17 n'expose que l'importance haute dans la composition (`MarkAsImportant`, l.7197). |
| `EditorDefaultType` (l.358) : Html, Plain, HtmlForced, PlainForced | mode d'éditeur par défaut | réglage | `templates/settings.html:28-34` | OK | |
| `ServerSecure` (l.368) : None, SSL, TLS | sécurité IMAP/SMTP | formulaire de compte | `accounts.err.imap_security` (`fr.json:123`), `templates/account_form.html` | OK | |
| `SearchDateType` (l.376) : All, 3 jours, 7 jours, 30 jours | filtre de date de la recherche avancée | recherche par date | `templates/home.html:43` (`adv_date` : tous, 3j, 1 semaine, 1 mois…) | OK | |
| `SaveSettingsStep` (l.385) : Animate, Idle, TrueResult, FalseResult | état du témoin d'enregistrement des réglages | retour visuel d'enregistrement | enregistrement par bouton + message flash `templates/settings.html:3`, `compose.go:886` | PARTIEL | Voir « Enregistrement automatique des réglages ». P3 |
| `Layout` (l.394) : NoPreview 0, SidePreview 1, BottomPreview 2 | disposition de la boîte : liste seule, aperçu à droite, **aperçu en bas** | choix de disposition | `templates/settings.html:20-23` : seulement `three` (aperçu à droite) et `list` ; aucun aperçu en bas | PARTIEL | Ajouter la disposition « aperçu en bas » (liste au-dessus, lecture dessous) si l'utilisateur l'utilise. À confirmer. P3 |
| `FilterConditionField` (l.404), `FilterConditionType` (l.414), `FiltersAction` (l.427), `FilterRulesType` (l.438) | énumérations des filtres Sieve | champs From/Recipient/Subject/Header/Size ; Contains…Under ; MoveTo/Discard/Vacation/Reject/Forward ; All/Any | `templates/filter_edit.html:18-75` (From, Recipient, Subject, Header, Size ; Any/All ; mark_read), `fr.json:442-456` | OK | Couvert plus finement dans la tranche des filtres. |
| `SignedVerifyStatus` (l.446) | statut de vérification OpenPGP | | aucun | SANS OBJET | OpenPGP. |
| `ContactPropertyType` (l.457) : FullName, FirstName, LastName, MiddleName, Nick, NamePrefix, NameSuffix, Email, Phone, Web, Birthday, **Facebook, Skype, GitHub**, Note, Custom | champs d'un contact | fiche contact complète | `contacts/contacts.go:51-71` (préfixe, prénom, 2e prénom, nom, suffixe, surnom, emails, téléphones, URLs, adresses postales, organisation, titre, date de naissance, notes, étiquettes) | PARTIEL | Pas de champs Skype/GitHub/Facebook (à stocker comme URL typées ou notes). P3 |
| `Magics` (l.480) | constantes numériques/durées | | | SANS OBJET | Constantes internes. |
| `Ports` (l.529) : 143/993/25/465/587 | ports par défaut du formulaire de compte | préremplissage selon la sécurité | `templates/account_form.html` (à vérifier dans la tranche comptes) | OK | Non relu ici en détail. |
| `Notification` (l.542) : codes 101-999 | codes d'erreur Ajax traduits en phrases | messages d'erreur clairs par type de problème | voir tableau « Notification » ci-dessous | PARTIEL | voir ci-dessous |

### Détail `Notification` (l.542-607) → messages wmail-go

| Code(s) RainLoop | Texte attendu côté RainLoop | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|
| 101 InvalidToken, 102 AuthError, 103 AccessError, 104 ConnectionError | session expirée, identifiants refusés, accès refusé, connexion impossible | `fr.json:13 login.error.invalid`, `error.csrf :62`, `accounts.test.fail.*` via `mail.go:449-451` | OK | |
| 105 CaptchaError, 106-108 Social*, 109 DomainNotAllowed, 110 AccountNotAllowed | captcha, connexion sociale, domaine/compte interdit | aucun | SANS OBJET | Captcha, social, restrictions de domaine (admin) hors périmètre. |
| 120-121 TwoFactor | code 2FA requis/invalide | `templates/login_2fa.html`, `server.go:126` | OK | |
| 130-134 mot de passe | échec/incorrect/trop court/faible/interdit | `fr.json:56-59` | OK | |
| 140 ContactsSyncError | échec de synchro CardDAV | `fr.json:652 contacts.sync.err.run` | OK | |
| 201-205 CantGetMessageList/Message/Delete/Move/Copy | échecs lecture/suppression/déplacement/copie | liste : erreur dans la zone liste `templates/base.html:198` ; actions : **page d'erreur pleine page** `mail.go:440-455` (`s.message`) | PARTIEL | RainLoop affiche une bannière non bloquante et garde la liste. Faire afficher l'erreur d'action en bandeau au-dessus de la liste (redirection avec `?err=` ou rendu partiel). P2 |
| 301-303 CantSave/CantSend/InvalidRecipients | échec d'enregistrement, d'envoi, destinataires invalides | `fr.json:260-265` | OK | |
| 351/352/355 filtres | erreurs de filtres | `fr.json:442-456` | OK | |
| 400-405 dossiers | création/renommage/suppression/(dés)abonnement/dossier non vide | `fr.json:316-319` | OK | |
| 501, 502 CantSaveSettings/PluginSettings | échec d'enregistrement | `settings.prefs.error` (`fr.json:55`) | OK | |
| 601, 701-712 domaines, paquets, licence | admin | aucun | SANS OBJET | |
| 750-751 Demo* | mode démo | aucun | SANS OBJET | |
| 801-802 AccountAlreadyExists/DoesNotExist | compte déjà présent | `fr.json:129 accounts.err.limit`, `identities.err.duplicate :337` | OK | |
| 901 MailServerError, 902 ClientViewError, 903 InvalidInputArgument, 950-953 Ajax*, 999 Unknown | erreur serveur mail, argument invalide, Ajax | `error.generic :63`, `mail.err.bad_request :177`, `s.serverError` | OK | |

---

## 2. Module 5 `Storage/Settings`, module 22 `Helper/Apps/User`, 23 `Events`, 24 `Consts`

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Initialisation `SETTINGS`/`APP_SETTINGS` (l.3407-3410) | réglages injectés dans la page (`__rlah_data`) | configuration disponible côté client | valeurs rendues dans les templates (`Page` struct `server.go:454-492`) | SANS OBJET | Rendu serveur. |
| `settingsGet` (l.3416), `settingsSet` (l.3424), `appSettingsGet` (l.3432), `capa` (l.3440) | accès aux réglages et aux capacités | | idem + `Capa` = toujours actif | SANS OBJET | |
| `getApp` (l.7647) | accès à l'objet application | | | SANS OBJET | |
| `Events.sub` (l.7677), `pub` (l.7698) | bus d'évènements (+ hook plugins `rl-pub`) | découplage des vues | évènement DOM `wmail:pane` (`static/list.js:111,211`) | SANS OBJET | Bus interne ; plugins hors périmètre. |
| `MESSAGES_PER_PAGE=20`, `MESSAGES_PER_PAGE_VALUES=[10,20,30,50,100]` (l.7742-7743) | taille de page par défaut 20 ; choix 10/20/30/50/100 | choix du nombre de messages par page | défaut 50 `mail.go:485-487`, menu 50/100/200/500 + valeur courante `mail.go:40,678`, champ libre 10-500 `templates/settings.html:24-25` | PARTIEL | Le menu de la barre ne propose pas 10/20/30. Si l'utilisateur utilise 20, ajouter 10/20/30 au menu (`pageSizes`). P3 |
| `CONTACTS_PER_PAGE=50` (l.7744) | 50 contacts par page | pagination contacts | `contacts.PageSize` (`contactsweb.go:127`) | OK | |
| `DEFAULT_AJAX_TIMEOUT=30000`, `SEARCH_AJAX_TIMEOUT=300000`, `SEND/SAVE_MESSAGE_AJAX_TIMEOUT`, `CONTACTS_SYNC_AJAX_TIMEOUT` (l.7745-7749) | délais : 30 s, **300 s pour la recherche**, 300 s envoi, 200 s brouillon/synchro | recherche longue possible | `mailTimeout = 30s` pour tout y compris la recherche `mail.go:32,435-436` ; envoi 90 s `compose.go:29` | PARTIEL | Donner 300 s aux requêtes de recherche (`q` non vide) : une recherche sur le corps d'un grand dossier dépasse 30 s. P3 |
| `UNUSED_OPTION_VALUE='__UNUSE__'` (l.7750) | valeur « dossier non utilisé » | choix « ne pas enregistrer les envoyés/brouillons » | `userdata.NoFolder` / `-` `templates/folders_settings.html:16` | OK | |
| `CLIENT_SIDE_STORAGE_INDEX_NAME`, `IMAP_DEFAULT_PORT`…`SIEVE_DEFAULT_PORT`, `MESSAGE_BODY_CACHE_LIMIT=15`, `AJAX_ERROR_LIMIT=7`, `TOKEN_ERROR_LIMIT=10`, `RAINLOOP_TRIAL_KEY` (l.7751-7760) | constantes de stockage, ports, cache des corps, seuils d'erreur | | ports : formulaire de compte ; le reste : SPA | SANS OBJET | |
| `DATA_IMAGE_USER_DOT_PIC`, `DATA_IMAGE_TRANSP_PIC`, `DATA_IMAGE_LAZY_PLACEHOLDER_PIC` (l.7763-7765) | images en data-URI (pastille utilisateur, pixel, placeholder) | | icône SVG `user` `templates/base.html:262` | SANS OBJET | |

---

## 3. Module 6 `Common/Globals` (l.3446-3766)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `$win,$doc,$html,$body,$div,$hcont`, `startMicrotime` (l.3510-3522) | références DOM globales | | | SANS OBJET | |
| `dropdownVisibility` (l.3527) | « un menu déroulant est ouvert » : bascule la portée clavier `menu` | les raccourcis ne se déclenchent pas quand un menu est ouvert | `static/menus.js:5-19` (un seul menu ouvert, Échap/clic extérieur ferment) ; `keys.js` n'est pas désactivé quand un menu est ouvert | PARTIEL | Ignorer les raccourcis de `keys.js` quand `details[data-dropdown][open]` existe (sinon `c`, `r`… agissent derrière le menu). P3 |
| `useKeyboardShortcuts` (l.3534) | activer/désactiver les raccourcis (réglage) | réglage « raccourcis clavier » | aucun réglage ; toujours actifs | PARTIEL | Ajouter une case « raccourcis clavier » dans Préférences si souhaité. P3 |
| `sUserAgent,bIE,bChrome,bSafari,bMobileDevice,bDisableNanoScroll,bAnimationSupported,bXMLHttpRequestSupported,bIsHttps` (l.3539-3579) | détection navigateur/mobile | adaptations mobiles | CSS responsive / `ThreePane` `mail.go:490` | SANS OBJET | Détection de mobile côté serveur : hors périmètre (utilisation poste de travail). |
| `htmlEditorDefaultConfig` (l.3584-3625) : barre CKEditor (styles, gras/italique/souligné + nettoyage + bidi, couleurs, listes/retrait/blocs/alignement, liens, insertion, document), boutons retirés (Format, Undo, Redo, Cut, Copy, Paste, Anchor, Strike, Subscript, Superscript, Image, SelectAll, Source), police par défaut Arial, taille 13, tailles 10-48 | contenu de la barre de l'éditeur riche | éditeur riche équivalent | `templates/compose.html:85-92` : gras, italique, souligné, listes, alignements, retrait, lien, citation, trait, effacer le formatage, couleurs, police (Arial par défaut), taille 10/12/13/14/16/18/20/24/28/36/48 `editor.js:100-150` | PARTIEL | Manque : boutons de direction du texte (bidi) ; mode « signature » (plugin `signature`) : voir tranche composition. Les tailles, polices par défaut et boutons retirés correspondent. P3 |
| `htmlEditorLangsMap` (l.3630-3661) | langue de l'éditeur | | éditeur natif navigateur | SANS OBJET | |
| `bAllowPdfPreview` (l.3666-3677) | aperçu PDF si le navigateur le sait | aperçu pièce PDF | `previewKind` `mail.go:1060-1066` renvoie `pdf` | OK | |
| `VIEW_MODELS` (l.3679) | registre des écrans de réglages | | | SANS OBJET | |
| `moveAction, leftPanelDisabled, leftPanelType, leftPanelWidth` (l.3684-3696) | panneau de gauche masquable (mode déplacement) | masquer le panneau des dossiers | `static/folders.js:47-57` (bouton, mémorisé `/settings/panes hidden`) | OK | |
| `popupVisibilityNames, popupVisibility` (l.3699-3704) | classe `rl-modal` sur `<html>` quand un popup est ouvert | assombrir/figer l'arrière-plan | `<dialog>` (`#newfolder`, visionneuse) | PARTIEL | Voir `modal` (module 1) : P3 |
| `keyScopeReal/Fake`, `keyScope` (l.3707-3753) : filtre de touches (désactivé dans INPUT/SELECT/TEXTAREA/éditeur), scope `menu` prioritaire | portée clavier | pas de raccourci pendant la saisie | `typing()` `static/keys.js:7-9`, `listnav.js:9-11` | OK | |
| `data` (l.3758) : compteurs d'erreurs, dropdowns, cache de corps | état global | | | SANS OBJET | |

---

## 4. Module 8 `Knoin/Knoin` + module 25 `AbstractViewNext` (l.3779-4423, 7768-7846)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `ViewType` (l.3853) | emplacement d'une vue (popup, gauche, droite, centre) | | | SANS OBJET | |
| `hideLoading` (l.3863) | masque l'écran de chargement (spinner « bounce ») | écran d'attente au démarrage | aucun (rendu serveur immédiat) | SANS OBJET | |
| `createCommand` (l.3873), `createCommandLegacy` (Utils l.2370), `commandDecorator` (l.4365) | commande knockout (activée/désactivée) | bouton grisé quand l'action est impossible | `data-needs-selection` + `disabled` `static/list.js:29,46` | OK | Effet visible repris. |
| `addSettingsViewModel`, `removeSettingsViewModel`, `disableSettingsViewModel` (l.3889-3917) | enregistrement des écrans de réglages | | routes `/settings/*` `server.go:159-183` | SANS OBJET | |
| `routeOff/routeOn/setHash` (l.3922-4312) | routage par hash `#/mailbox/…` | URL partageables, bouton retour | URLs serveur `/?folder=…&page=…&uid=…`, `history.pushState` `static/list.js:201` | OK | |
| `screen`, `getScreenPopup`, `hideScreenPopup`, `warmUpScreenPopup`, `isPopupVisible`, `showScreenPopup` (l.3937-4121) | gestion des popups | fenêtres modales (composition, contacts, réglages de dossier…) | pages dédiées (compose, contacts) ; `<dialog>` pour nouveau dossier `static/folders.js:50-67` | SANS OBJET | Choix d'architecture : pages au lieu de popups. |
| `vmRunHook`, `Plugins.runHook` (l.3976) | points d'accroche plugins | | | SANS OBJET | Plugins hors périmètre. |
| `buildViewModel` (l.3991-4070) : z-index 3000+n des popups, `viewModelDom.hide` après 300 ms, `registerPopupKeyDown` | construction des vues, empilement et animation des popups | | | SANS OBJET | |
| `screenOnRoute` (l.4128-4235), `startScreens` (l.4241-4282) | changement d'écran, classes `rl-started` | transitions d'écran | | SANS OBJET | |
| `viewDecorator`, `popupDecorator` (l.4318-4358) | décorateurs | | | SANS OBJET | |
| `settingsMenuKeysHandler` (l.4403-4423) | ↑/↓ naviguent dans le menu des réglages | clavier dans la liste des écrans de réglages | aucun | MANQUANT | Navigation ↑/↓ dans le menu latéral des réglages : P3 (rarement utilisé). |
| `AbstractViewNext.storeAndSetKeyScope/restoreKeyScope` (l.7805-7816) | popup = nouvelle portée clavier, restauration à la fermeture | raccourcis liste suspendus pendant un popup | `keys.js:34-35` ignore `defaultPrevented` ; `listnav.js:10` ignore `dialog[open]` ; `keys.js` n'ignore pas `dialog[open]` | PARTIEL | Ignorer aussi `dialog[open]` dans `keys.js` `typing()`. P3 |
| `AbstractViewNext.registerPopupKeyDown` (l.7822-7837) : **Échap ferme le popup** ; **Retour arrière hors champ bloqué** | fermer au clavier, empêcher le « retour navigateur » | Échap ferme la fenêtre ; Backspace ne quitte pas la page | Échap ferme dialogue dossier `static/folders.js:64-66`, aide `keys.js:38`, visionneuse (natif `<dialog>`) ; Backspace non géré (inutile : pas de hash-routing) | OK | |
| `cancelCommand/closeCommand` (l.7839-7842) | vides par défaut | | | SANS OBJET | |

---

## 5. Module 9 `Common/Translator` (l.4428-4720)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `I18N_DATA`, `i18n(key, valueList, default)` (l.4491) avec `%NOM%` | traduction avec paramètres nommés | libellés traduits | `$.T` (templates) avec `%d`/`%s` ; `i18n/locales/{fr,en}.json` | OK | Mécanisme différent (printf), même effet. |
| `i18nToNode(s)` (l.4510-4562) : `data-i18n` (`[html]`, `[placeholder]`, `[title]`) + animation `letterfx` | applique les textes au DOM, animation à chaque changement de langue | | rendu serveur | SANS OBJET | Animation de changement de langue : P3, non reproduite (rechargement de page). |
| `reloadData`, `reload(admin, language)` (l.4564-4717) : charge le fichier de langue sans recharger la page, classes `rl-changing-language`, `rl-rtl/rl-ltr` (ar, he, ur) | changement de langue à chaud | changer la langue | `/lang` `server.go:119`, réglage `settings.html:8-9` (rechargement) | PARTIEL | Pas d'application à chaud ; les langues RTL ne sont pas gérées (seulement fr/en). P3 |
| `initNotificationLanguage`, `getNotification`, `getNotificationFromResponse` (l.4579-4649) | code d'erreur → phrase | | voir tableau Notification | OK | |
| `initOnStartOrLangChange` (l.4589) | abonnement au changement de langue | | | SANS OBJET | |
| `getUploadErrorDescByCode` (l.4655) | message d'erreur de téléversement | | `fr.json:266-268` | OK | |
| init `rl-ltr` (l.4719) | classe de direction | | `<html lang>` `templates/base.html:2` | SANS OBJET | |

---

## 6. Module 11 `Stores/User/Folder` (l.4744-4966)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Observables `sentFolder, draftFolder, spamFolder, trashFolder, archiveFolder`, `namespace`, `folderList` (l.4779-4797) | dossiers système choisis par l'utilisateur ; liste des dossiers | réglage des dossiers système | `templates/folders_settings.html:14-18`, `foldertree.go:64-73`, `userdata.Folders` | OK | |
| `foldersLoading/Creating/Deleting/Renaming`, `foldersChanging` (l.4789-4831) | indicateur « opération en cours » sur le panneau des dossiers | retour visuel pendant une opération | opérations par formulaires (rechargement) `server.go:162-164` | SANS OBJET | |
| `folderList.error` (l.4788) | erreur de chargement des dossiers | message dans le panneau | `templates/base.html:198` (`FolderError`) | OK | |
| `foldersInboxUnreadCount` (l.4793) | compteur non lus de la boîte de réception (titre d'onglet, pastille) | compteur | `static/poll.js:12,61-70` | OK | |
| `currentFolder`+`toggleSubscribeProperty selected` (l.4794) | dossier courant marqué sélectionné | surbrillance | classe `active` `templates/base.html:203` | OK | |
| `sieveAllowFileintoInbox` (l.4797) | autorise « déplacer vers INBOX » dans un filtre | filtre vers la boîte de réception | `filter_edit.html` (à vérifier tranche filtres) | OK | Non relu en détail ici. |
| `draftFolderNotEnabled` (l.4807) | brouillons désactivés | message « pas de dossier brouillons » | `compose.err.no_drafts` `fr.json:265` | OK | |
| `foldersListWithSingleInboxRootFolder` (l.4810) | seule INBOX racine (adapte l'arbre) | | | SANS OBJET | |
| `currentFolderFullNameRaw/FullName/FullNameHash` (l.4815-4823) | noms du dossier courant | | paramètre `folder` | SANS OBJET | |
| `folderListSystemNames`, `folderListSystem` (l.4832-4869) | liste ordonnée des dossiers système (Inbox, Envoyés, Brouillons, Spam, Corbeille, Archive) | ordre fixe des dossiers système en tête | `foldertree.go:64-73` (même ordre) | OK | |
| `folderMenuForMove` (l.4870) : `folderListOptionsBuilder` avec dossiers système d'abord, séparateur, arbre indenté, dossier courant désactivé, noms locaux | menu « Déplacer vers » | menu de déplacement lisible et ordonné | `templates/base.html:101-107` (`{{range .MoveMenu}}` : noms IMAP bruts, ordre IMAP) et `foldertree.go:165-169` | PARTIEL | Voir « Menu Déplacer/Copier ». **P1** |
| `folderMenuForFilters` (l.4875) | liste de dossiers pour un filtre (avec ligne vide, INBOX exclue sauf option) | choix du dossier cible d'un filtre | `filter_edit.html` : à vérifier (liste brute probable) | PARTIEL | Même constructeur que le menu de déplacement. P3 |
| `subscribers` (l.4882-4912) : changer un dossier système retire le type de l'ancien et le pose sur le nouveau | cohérence des rôles | | `folders/system` `server.go:163` | OK | |
| `getNextFolderNames` (l.4919-4961) : jusqu'à 5 dossiers système ou « à relever » non vérifiés depuis 5 min | relève périodique des compteurs des autres dossiers | compteurs à jour dans l'arbre | `acc.CheckFolders` interrogés à chaque sondage `poll.go:55-62` (`folders` max 5 : `fr.json:655`), applique badge `static/poll.js:52-60` | OK | Sélection manuelle (réglage « dossiers à relever ») au lieu d'une rotation ; équivalent. |

---

## 7. Module 13 `Stores/User/Message` (l.4980-5760)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Observables de liste : `messageList, messageListCount, messageListSearch, messageListThreadUid, messageListPage, messageListError…` (l.5065-5090) | état de la liste (page, recherche, fil) | | `listParams` `mail.go:196-205` | SANS OBJET | |
| `messageListCompleteLoadingThrottle(ForAnimation)` (l.5081-5086) | spinner de chargement de liste (700 ms) | indicateur de chargement | aucun indicateur : chargement par navigation | SANS OBJET | |
| `messageListDisableAutoSelect` (l.5087) | pas d'ouverture auto du premier message après chargement | | pas d'auto-sélection `static/list.js` | OK | |
| `selectorMessageSelected/Focused`, `message` (l.5090-5093) | message sélectionné vs curseur | surbrillance de la ligne ouverte et curseur clavier | `.msgrow.selected` / `.focused` `static/list.js:172-184`, `listnav.js:14-18` | OK | |
| `messageFullScreenMode` (l.5100) | lecture plein écran | plein écran | `static/msgfull.js` | OK | |
| `messagesBodiesDom`, `messageActiveDom`, `staticMessage` (l.5065, 5101-5102) | cache DOM des corps | | iframe `/mail/body` `mail.go:1008` | SANS OBJET | |
| `messageListPageCount` (l.5120) | nombre de pages | | `Pages` `mail.go:564` | OK | |
| `mainMessageListSearch` (l.5124) : écrire = changer le hash | champ de recherche | | champ `q` `templates/home.html` | OK | |
| `messageListChecked`, `hasCheckedMessages`, `messageListCheckedOrSelected` (l.5139-5163) | messages cochés, sinon sélectionné, sinon curseur : cibles des actions | actions sur « cochés ou ouvert » | `data-needs-selection` actif si coché **ou message ouvert** `static/list.js:40,46` ; `bulk` serveur | OK | |
| `messageListCheckedOrSelectedUidsWithSubMails` (l.5164) | inclut tous les messages du fil | actions sur un fil entier | `th_` membres `mail.go:1301-1313` (supprimer, déplacer, copier, archiver, spam) | OK | |
| `subscribers` (l.5181-5225) : sans aperçu, ouvrir un message donne le focus à la vue ; fermer vide le plein écran ; changement de dossier ferme le message ouvert | | | `mail.go:1378-1390` (`next`) | OK | |
| `purgeMessageBodyCache` (l.5228) | purge du cache de 15 corps | | | SANS OBJET | |
| `initUidNextAndNewMessages` (l.5254-5283) : son + notification bureau ; au-delà de 3 nouveaux : une seule notification « N nouveaux messages » avec **icône** et titre = adresse du compte ; sinon une par message (titre = expéditeur, corps = objet) | alerte de nouveaux messages | | `static/poll.js:27-39` : son (bip synthétisé 880 Hz), notification sans icône, cas >3 sans titre de compte | PARTIEL | Ajouter l'icône (enveloppe) et le titre = adresse du compte pour le cas >3 ; son : RainLoop joue `sounds/new-mail.mp3`. P3 |
| `hideMessageBodies` (l.5285) | | | | SANS OBJET | |
| `removeMessagesFromList` (l.5300-5412) : met à jour les compteurs total/non lus des dossiers source et cible, **fait clignoter** le dossier cible (`actionBlink`), anime la disparition des lignes (350 ms), invalide le hash des dossiers, gère la sortie d'un fil | retour visuel d'un déplacement/suppression | compteurs justes immédiatement, dossier cible qui clignote, lignes qui disparaissent | rechargement serveur : compteurs exacts après redirection `mail.go:1378-1390` ; ni clignotement ni animation | PARTIEL | Retour visuel : lignes qui s'effacent, dossier cible qui clignote (classe CSS animée). Cosmétique. P3 |
| `initBlockquoteSwitcher` (l.5418-5446) : citations de 1er niveau de **plus de 100 px** (texte non vide) repliées derrière un bouton `…` 30x14 px gris (#eee, bord #999, opacité .5) inséré avant la citation | replier les longues citations | | `CollapseQuotes` `mailview/sanitize.go:235-290` : seuil **400 caractères de texte**, `<details class="wm-quote">` + résumé « … » stylé `mail.go:1002` (fond #e8e8e8, rayon 3, sans bord) | PARTIEL | Fonction OK ; seuil différent (caractères vs hauteur) et bouton différent. Aligner le style (30x14, bord #999, opacité .5/1 au survol, `cursor:pointer`). P3 |
| `initOpenPgpControls` (l.5453) | contrôles PGP | | | SANS OBJET | OpenPGP. |
| `setMessage` (l.5461-5616) : corps HTML sinon texte (`plainToHtml` + `<pre>`), `findEmailAndLinks`, `hasImages` (HasExternals), `showInternalImages` (HasInternals), images externes affichées si réglage « afficher les images », marque **lu automatiquement**, gère les messages d'un fil | affichage du message | | `mailBody` `mail.go:1008-1055`, `Sanitize`, images distantes bloquées par défaut + bandeau `templates/base.html:297`, option par compte `BlockRemoteContent`, lecture auto `MarkReadOnOpen` `handlers.go:425` | PARTIEL | Voir lignes « findEmailAndLinks », « plainToHtml » ; le reste OK. |
| `selectMessage`, `selectMessageByFolderAndUid`, `populateMessageBody`, `onMessageResponse` (l.5618-5665) : chargement du message, erreur → message dans la vue | | | `mailPane` `server.go:140` ; erreur dans le volet `templates/home.html:115` (`PaneError`) | OK | |
| `calculateMessageListHash`, `setMessageList` (l.5672-5755) : met à jour compteurs, détecte nouveaux messages (**animation `newForAnimation` sur au plus 5 lignes**), cache des drapeaux, erreur de liste | rafraîchissement de la liste | nouveaux messages surlignés/animés | pas de rafraîchissement automatique de la liste (le sondage ne met à jour que titre/badges, `poll.js:41-71`) ; pas d'animation | PARTIEL | RainLoop ajoute les nouveaux messages dans la liste ouverte sans action. Ajouter au sondage un bandeau « N nouveaux messages — afficher » ou un rafraîchissement si on est en page 1 de la boîte de réception. P2 |

---

## 8. Module 1 `External/ko` : bindings, extenders, fonctions knockout (l.655-1765)

Ces unités sont de la colle knockout ; je vérifie pour chacune l'effet visible/comportemental et je cite ce que wmail-go fait à la place.

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `fDisposalTooltipHelper` (l.707) | libère l'infobulle à la destruction de l'élément | | | SANS OBJET | |
| `updateWidth` (l.715) | recopie la largeur d'un élément dans une variable (redimensionnement) | volets redimensionnés | volets à largeur variable via variable CSS `static/list.js:244-283` | OK | |
| `editor` (l.734) | branche l'éditeur riche (CKEditor) sur une variable | éditeur riche | `static/editor.js` (contenteditable) | OK | |
| `json` (l.763) | affiche du JSON dans un nœud | | | SANS OBJET | |
| `scrollerShadows` (l.771) | ombre en haut/en bas d'une zone défilante quand il reste du contenu (classes `scroller-shadow-top/bottom`, ombre 20 px rgba(0,0,0,.4), CSS l.7238-7256) sur liste, dossiers, message, composition, contacts | indice visuel de défilement | aucun (`mail.css` sans ombres) | MANQUANT | Ajouter un `::before/::after` d'ombre sur `.msglist-form`, `.folders`, `.readpane` selon `scrollTop` (petit JS ou `animation-timeline`). P3 |
| `pikaday` (l.790) | sélecteur de date | date de naissance d'un contact | `<input type="date">` `templates/contacts.html:46` | OK | Sélecteur natif au lieu de Pikaday (rendu différent). |
| `visibleAnimated` (l.801) | affichage/masquage avec fondu (message d'erreur de connexion, alertes) | alertes animées | alerte statique `templates/login.html` | PARTIEL | Cosmétique : fondu des alertes. P3 |
| `tooltip` (l.826) : Opentip style `rainloopTip` (l.18649-18653 : fond blanc, bord 1 px #999, rayon 2, ombre non, délai 0,4 s, fondu 0,2 s, police 13 px, flèche), traduit, masqué si un menu est ouvert, désactivé sur mobile | infobulle sur chaque bouton de la barre d'outils | infobulle stylée au survol | infobulles natives `title=` (ex. `templates/base.html:97-114`) ; aucune CSS d'infobulle (grep `tooltip` vide dans `static/*.css`) | PARTIEL | Différence visuelle : bulle native du navigateur (délai long, style OS) au lieu d'une bulle blanche à bord gris et flèche. Ajouter une CSS `[data-tip]`/`[title]` stylée (pseudo-élément) en gardant `title` pour l'accessibilité. P2 |
| `tooltipErrorTip` (l.908) | bulle rouge (`rainloopErrorTip`, texte rouge) sur un champ en erreur | erreur contextuelle sur champ | messages d'erreur en bloc `class="msg error"` | PARTIEL | Cosmétique. P3 |
| `registrateBootstrapDropdown` (l.954), `openDropdownTrigger` (l.967), `dropdownCloser` (l.984) | menus Bootstrap : suivi d'ouverture, ouverture programmée, fermeture au clic sur un élément | menus déroulants | `<details data-dropdown>` `static/menus.js` (un seul ouvert, clic ailleurs/Échap ferment, soumission ferme) | OK | |
| `popover` (l.991) | bulle Bootstrap | | | SANS OBJET | |
| `csstext` (l.999) | injecte du CSS dans une balise `<style>` (thème, fond utilisateur) | thème/fond appliqué | `/user.css` `server.go:202`, `/themes/{name}/{file}` `server.go:116` | OK | |
| `resizecrop` (l.1009) | recadre une image (photo de contact/identité 100x100, coins 10 px) | photo carrée | aucune photo de contact | MANQUANT | Voir « Photo de contact ». P3 |
| `onKeyDown/onEnter/onSpace/onTab/onEsc` (l.1027-1092) | gestionnaires clavier sur un champ | Entrée valide, Échap annule | `chips.js:104-115` (Entrée/Tab), `keys.js` | OK | |
| `clickOnTrue` (l.1093) | clic programmatique | | | SANS OBJET | |
| `modal` (l.1100) : popup Bootstrap, animation `fade`, `rl-modal-animation` 500 ms, recalcul de taille | fenêtre modale animée avec fond assombri | modales | `<dialog>`/pages | PARTIEL | Les `<dialog>` (nouveau dossier, visionneuse) n'ont pas le fondu RainLoop ; cosmétique. P3 |
| `moment` (l.1134) : date relative ou absolue **rafraîchie périodiquement** (Momentor) | « il y a 5 min » qui vieillit | date relative | `listDate` `mail.go:361-390` calculé au rendu (même formats : « à l'instant », « il y a N min/h », « hier hh:mm », date) | PARTIEL | La date ne vieillit pas sans rechargement ; le sondage pourrait réécrire les dates relatives (P3). |
| `i18nInit`, `translatorInit`, `i18nUpdate` (l.1142-1157) | applique les traductions | | `$.T` | SANS OBJET | |
| `link`, `title`, `textF`, `initDom`, `appendDom` (l.1159-1233) | liaisons d'attributs/texte/DOM | | | SANS OBJET | |
| `initFixedTrigger` (l.1179), `initResizeTrigger` (l.1198) | hauteur d'un conteneur calculée d'après la fenêtre | mise en page pleine hauteur | CSS (`.mailbox` flex/hauteur) | OK | |
| `draggable` (l.1235-1314) : jQuery UI, poignée `.dragHandle`, distance 20 px, curseur décalé (22,3), **assistant de glissement** (`draggablePlace` : bulle avec le nombre de messages, icône copie si Ctrl, icône courrier sinon), **défilement automatique** de la liste de dossiers à moins de 100 px des bords | glisser des messages vers un dossier avec retour visuel | glisser-déposer riche | HTML5 DnD natif `static/dnd.js:16-38` (aucune `setDragImage`, fantôme du navigateur) ; survol d'un dossier = `drop-over` ; Ctrl/Alt = copie `dnd.js:45` | PARTIEL | Ajouter `dataTransfer.setDragImage` avec une bulle « N message(s) » + icône copie/déplacement selon Ctrl, et défilement automatique du panneau des dossiers près des bords. P2 |
| `droppable` (l.1316-1357) : classe `droppableHover`, callbacks over/out | cible de dépôt, dossier surligné | | `data-drop` + `drop-over` `static/dnd.js:40-71` ; ouverture d'un dossier replié après 500 ms `dnd.js:53-58` | OK | |
| `nano` (l.1358) : barre de défilement **fine superposée** (5 px, `rgba(0,0,0,.3)`, visible au survol, fond au survol .15, CSS app.css l.1422-1500), désactivée sur mobile ou si `useNativeScrollbars` | rendu des barres de défilement | barres fines discrètes | barres natives du navigateur (aucun `scrollbar-*` ni `::-webkit-scrollbar` dans `static/*.css`/`themes/rainloop/theme.css`) | PARTIEL | Visible en permanence : ajouter `scrollbar-width: thin; scrollbar-color: rgba(0,0,0,.3) transparent` (et `::-webkit-scrollbar` 5 px) sur les zones défilantes du thème « rainloop ». P2 |
| `saveTrigger` (l.1371-1429) : à côté d'un champ de réglage, **spinner, croix rouge ou coche verte** selon l'enregistrement ; champs texte : bordure verte/rouge | retour d'enregistrement automatique | réglages enregistrés dès la modification | bouton « Enregistrer » unique `templates/settings.html:42` + message flash | PARTIEL | Voir « Enregistrement automatique des réglages ». P3 |
| `emailsTags` (l.1430-1490) : inputosaurus (pastilles, séparateurs `, ; \n`, validation à la sortie, glisser-déposer entre champs, auto-complétion, collage de listes avec découpage intelligent, focus callback) | champs À/Cc/Cci | pastilles d'adresses | `static/chips.js` (même ensemble : séparateurs `chips.js:21,102`, parse à la sortie `:116`, glisser entre champs `:74-130`, retour arrière `:111-114`, double-clic = éditer `:73`) + `contacts-ac.js` | OK | Détail : une adresse sans point dans le domaine (ex. `moi@localhost`) est marquée invalide par `chips.js:8` (RainLoop accepte `^[^@\s]+@[^@\s]+$`, l.1724). Assouplir la regex. P3 |
| `command` (l.1492-1538) : classes `command-disabled disable disabled`, `disabled` sur bouton | boutons grisés | | `button:disabled` via `list.js:46` | OK | |
| `extenders.trimmer` (l.1541) | champ nettoyé des espaces | | `strings.TrimSpace` côté serveur (ex. `identities.go:67`) | OK | |
| `extenders.posInterer` (l.1554) | entier positif avec valeur par défaut | taille de page | `pageSize` hors de 10 à `maxPageSize` : repli sur 50 `mail.go:485-487` | OK | |
| `extenders.limitedList`, `reversible`, `toggleSubscribe`, `toggleSubscribeProperty`, `falseTimeout`, `specialThrottle`, `idleTrigger` (l.1577-1705), `fn.idleTrigger` (l.1708), `validateNone` (l.1714) | utilitaires d'observables | | | SANS OBJET | |
| `fn.validateEmail` (l.1719), `validateSimpleEmail` (l.1730), `validateFunc` (l.1750) | validation d'adresse (`^[^@\s]+@[^@\s]+$`) | champ erroné signalé | validation serveur `compose.err.address`, `identities.err.address` | OK | |
| `fn.deleteAccessHelper` (l.1741) : bouton « supprimer » à deux temps (3 s) | confirmation en deux clics | éviter les suppressions accidentelles | `window.confirm` `static/list.js:19-22` (corbeille/indésirables), confirmations ailleurs | PARTIEL | Confirmation native au lieu d'un bouton « Êtes-vous sûr ? » ; cosmétique. P3 |

---

## 9. Module 2 `Common/Utils` (l.1768-3365)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Ré-exports (l.1777-1923) : `Jassl`, `Mime`, etc. | réexporte des modules | | | SANS OBJET | |
| `trim, inArray, isArray, isObject, isFunc, isUnd, isNull, has, bind, noop, noopTrue, noopFalse, silentTryCatch, isNormal, isPosNumeric, pInt, pString, pBool, boolToAjax, isNonEmptyArray` (l.1924-2023) | utilitaires de type | | | SANS OBJET | Utilitaires JS. |
| `encodeURIComponent, decodeURIComponent, decodeURI, encodeURI` (l.2029-2055) | | | | SANS OBJET | |
| `simpleQueryParser` (l.2061) | analyse `a=b&c=d` (mailto) | | `url.ParseQuery` `compose.go:984` | OK | |
| `fakeMd5` (l.2080), `deModule` (l.2168), `microtime` (l.2429), `timestamp` (l.2436), `log` (l.2303), `timeOutAction/Second` (l.2141-2160), `delegateRun` (l.2315), `domReady` (l.3325), `windowResize(Callback)` (l.3338-3353), `substr` (l.3354) | utilitaires | | | SANS OBJET | |
| `encodeHtml` (l.2101) | échappement HTML | | `html.EscapeString` | OK | |
| `splitPlainText` (l.2110) : coupe à 100 colonnes aux espaces/retours | texte brut à 100 colonnes | texte sortant lisible | pas de coupure à 100 colonnes dans `compose.HTMLToPlain` `compose/html.go:141` ni pour le texte brut saisi | PARTIEL | Voir ligne `htmlToPlain`. P3 |
| `inFocus` (l.2175), `removeInFocus` (l.2194) | détecte/retire le focus d'un champ | pas de raccourci pendant la saisie | `typing()` `static/keys.js:7-9` | OK | |
| `removeSelection` (l.2212), `selectElement` (l.2903) | efface/sélectionne du texte | | | SANS OBJET | |
| `replySubjectAdd` (l.2232-2266) : « Re: » ou « Fwd: » sans empiler ; **conserve l'autre préfixe** (répondre à « Fwd: x » donne « Re: Fwd: x » ; transférer « Re: x » donne « Fwd: Re: x ») | objet de réponse/transfert | | `compose.ReplySubject` `compose/reply.go:14-37` : retire tous les préfixes (`re`, `fwd`, `fw`, `tr`, avec `[n]`) et remet un seul préfixe | PARTIEL | Différence : RainLoop garde « Fwd: » dans une réponse (et « Re: » dans un transfert). Pour copier : après nettoyage, conserver le type de préfixe opposé s'il était présent. P3 |
| `roundNumber` (l.2274) | | | | SANS OBJET | |
| `friendlySize` (l.2282) : `1.5MB`, `512KB` (0 décimale), `123B`, **sans espace, unités anglaises** | tailles lisibles | | `humanSize` `mail.go:405-422` : `1.5 Mo` (espace, unités françaises, 1 décimale partout sauf octets) | PARTIEL | Différence de rendu : « 12,5 Ko » vs « 13KB ». À aligner seulement si l'utilisateur veut le rendu strict (choix de langue assumé). P3 |
| `killCtrlACtrlS` (l.2337) : bloque Ctrl+S (enregistrer la page) et Ctrl+A hors champ | pas de dialogue « Enregistrer sous » ; Ctrl+A sélectionne les messages | | Ctrl+A = tout sélectionner `static/list.js:98-108` ; Ctrl+S = enregistrer le brouillon en composition `static/compose.js:204-208` ; Ctrl+S dans la liste non intercepté | OK | Détail négligeable. |
| `convertThemeName` (l.2410) : « DarkShine » → « Dark Shine », `@custom` retiré | libellé lisible des thèmes | liste des thèmes présentable | `templates/settings.html:11-12` affiche le nom de dossier brut (`rainloop`, `dark`, `light`) | PARTIEL | Ajouter des libellés (i18n `theme.<nom>`) : « DarkShine », « Sombre », « Clair ». P3 |
| `quoteName` (l.2422) | échappe les guillemets d'un nom | | | SANS OBJET | |
| `convertLangName` (l.2446) | nom de langue | | `lang.fr`, `lang.en` `templates/settings.html:10` | OK | |
| `draggablePlace` (l.2457) | bulle de glisser-déposer | | voir `draggable` | PARTIEL | voir `draggable` (P2). |
| `defautOptionsAfterRender` (l.2466) | options désactivées | | | SANS OBJET | |
| `clearBqSwitcher` (l.2478) | retire les bascules de citation avant impression | | impression : page dédiée `/mail/print` `server.go:152` avec `print.css` | OK | |
| `previewMessage` (l.2493-2517) : ouvre une fenêtre avec le gabarit `PreviewMessage.html` (titre, objet, date, de, à, cc) et lance l'impression | impression d'un message | | `/mail/print` + `static/print.js` (impression automatique) | OK | Contenu des en-têtes imprimés non comparé ligne à ligne. |
| `settingsSaveHelperFunction`, `…SimpleFunction`, `…SubscribeFunction` (l.2526-2599) | enregistrement automatique des réglages avec retour d'état | | formulaire unique | PARTIEL | Voir `saveTrigger` ; P3. |
| `findEmailAndLinks` (l.2605-2618) : Autolinker sur **le HTML et le texte** : URL (`http`) **et adresses e-mail** deviennent des liens (`target=_blank`) | liens cliquables dans le corps | | texte brut : seulement `http(s)://` `mailview/text.go:11-48` ; HTML : aucun lien automatique (bluemonday ne linkifie pas, `mailview/sanitize.go:66-90`) ; adresses e-mail jamais | MANQUANT | Ajouter une passe de linkification sur les nœuds texte du HTML assaini (hors `<a>`) pour `https?://…` et `adresse@domaine` (en `mailto:`), et ajouter les e-mails au texte brut. P2 |
| `htmlToPlain` (l.2624-2713) : conserve les citations en `> ` (imbriquées), liens → texte, `<li>` → ` * `, `<hr>` → ligne `____`, blocs `<div>`/`<p>` → lignes vides, espaces blancs réduits, coupe à 100 colonnes | version texte d'un HTML (partie texte sortante, bascule HTML→texte, citation en réponse texte) | | `mailview.HTMLToText` `mailview/text.go:52-85` : nouvelles lignes aux blocs seulement ; **pas de `> ` pour les citations**, pas de `* ` pour les listes, pas de réduction des espaces/sauts de ligne source, pas de coupe à 100 colonnes ; appelé par `compose.HTMLToPlain` `compose/html.go:141`, `compose.go:183,302,580`, `mail.go:1036` | PARTIEL | Améliorer `HTMLToText` : `blockquote` → préfixer `> ` (imbriqué), `li` → `* `, `hr` → ligne, réduire `\s+` des nœuds texte à un espace (hors `<pre>`), option de coupe à ~100 colonnes. Effet : partie texte des messages envoyés, mode « texte » d'un HTML, citation dans une réponse en texte brut. P2 |
| `plainToHtml` (l.2721-2779) : citations **imbriquées** (`>>`) en `<blockquote>` emboîtés, liens et e-mails | texte brut affiché joliment | | `mailview.TextToHTML` `mailview/text.go:15-35` : un seul niveau de `<blockquote>` (toute ligne commençant par `>`), pas d'emboîtement, e-mails non liés | PARTIEL | Gérer la profondeur (`>`, `> >`, `>>`) en `<blockquote>` emboîtés. P3 |
| `window.rainloop_Utils_htmlToPlain/plainToHtml` (l.2780-2782) | exposition aux plugins | | | SANS OBJET | |
| `folderListOptionsBuilder` (l.2798-2897) : liste de `<option>` — en-têtes, dossiers système (noms locaux), séparateur `---`, arbre indenté par 3 espaces insécables, **désactive** les dossiers non sélectionnables ou exclus, n'affiche que les abonnés | sélecteurs de dossier (déplacer, filtre, réglages) | | menus `MoveMenu` bruts `foldertree.go:165-169`, `templates/folders_settings.html:17` : liste plate de noms IMAP (`{{range $.FoldersView.Names}}`) | PARTIEL | Voir « Menu Déplacer/Copier » (P1) et appliquer le même générateur aux sélecteurs `folders_settings.html:17,40` et filtres. |
| `detectDropdownVisibility` (l.2919) | suit les menus ouverts (portée clavier) | | voir `dropdownVisibility` | PARTIEL | P3. |
| `triggerAutocompleteInputChange` (l.2928) | détecte le remplissage automatique du navigateur | | formulaire HTML standard | SANS OBJET | |
| `getConfigurationFromScriptTag` (l.2949) | lit une configuration JSON injectée | | | SANS OBJET | |
| `disposeOne/disposeObject/delegateRunOnDestroy` (l.2966-3001) | nettoyage knockout | | | SANS OBJET | |
| `appendStyles` (l.3008), `changeTheme` (l.3029-3086) : applique le thème **sans recharger** (JSON CSS), indicateur d'état | thème appliqué à chaud depuis les réglages | | thème pris en compte après enregistrement + rechargement `templates/settings.html:11` ; `data-theme` `templates/base.html:2` | PARTIEL | Aperçu immédiat : changer `data-theme`/lien CSS au `change` du sélecteur (feuille `/themes/<nom>/theme.css`). P3 |
| `computedPagenatorHelper` (l.3091-3180) : fenêtre ±2 autour de la page (±4 près des bords), première/dernière page, **« … » cliquable** qui saute à la page médiane, la page 2 (ou n-1) affichée directement quand un seul numéro manque | pagination de la liste | | `pageLinks` `mail.go:161-192` : même fenêtre, première/dernière ; l'ellipse est un `<span class="gap">` **non cliquable** `templates/home.html:105` et s'affiche même quand un seul numéro est caché (ex. `1 … 3 4`) | PARTIEL | Rendre « … » cliquable vers `(prev+1)/2` ou `(next+total)/2` et afficher le numéro unique manquant au lieu de « … ». P3 |
| `getFileExtension` (l.3186) | extension d'un nom | | `attachmentExt` `mail.go:829` | OK | |
| `mimeContentType` (l.3196) : table `Mime`, `winmail.dat` → `application/ms-tnef` | type MIME d'après le nom | | `http.DetectContentType` + type déclaré `mail.go:1060-1075` ; winmail.dat (TNEF) non géré | PARTIEL | Pas de décodage TNEF (winmail.dat) : P3, à n'ajouter que si l'utilisateur en reçoit. |
| `isTransparent` (l.3218), `getRealHeight` (l.3226) | mesures DOM | | | SANS OBJET | |
| `resizeAndCrop` (l.3238) : recadre/redimensionne une image carrée en JPEG (photo de contact) | photo carrée | | aucune | MANQUANT | Voir « Photo de contact ». P3 |
| `mailToHelper` (l.3268-3319) : lien `mailto:` → ouvre la **composition** avec à (chemin + `to=` dédoublonné par adresse), cc, bcc, objet, corps | cliquer une adresse e-mail dans un message ouvre sa propre composition | | `parseMailto` `compose.go:971-998` gère `to/cc/bcc/subject/body` (`/compose?mailto=`), **mais** les liens `mailto:` du corps ne sont pas réécrits : `mailview/sanitize.go:171` les laisse tels quels dans l'iframe sandbox → ouverture du client mail du système | MANQUANT | Réécrire dans `Sanitize` les `href="mailto:…"` en `/compose?mailto=<url-encodé>` + `target="_blank"` (l'iframe n'a pas `allow-top-navigation`) ; idem pour le texte brut lié. Dédoublonner par adresse (garder celle qui a un nom). P2 |
| `substr` polyfill (l.3354-3363) | | | | SANS OBJET | |

---

## 10. Modules 14 `Common/Cache` et 15 `Common/Links` (l.5763-6600)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `clear` (l.5824), `getMessageKey` (l.5847), `add/hasRequestedMessage` (l.5855-5866), `add/hasNewMessageAndRemoveFromCache`, `clearNewMessageCache` (l.5872-5894) | caches clients (messages demandés, nouveaux messages) | | | SANS OBJET | Cache de SPA. |
| `getUserPic(email, callback)` (l.5837) : avatar Gravatar si activé, sinon vide | photo de l'expéditeur | | silhouette fixe `templates/base.html:262` ; pas de Gravatar (décision CAHIER `:595,893`) | OK | Si `GRAVATAR` est désactivé côté RainLoop, rendu identique (silhouette). À confirmer par l'utilisateur. |
| `getFolderInboxName` (l.5899), `get/setFolderFullNameRaw` (l.5907-5921) | nom de la boîte de réception (INBOX insensible à la casse) | | `foldertree.go:58-64` | OK | |
| `get/setFolderHash` (l.5927-5938), `get/setFolderUidNext` (l.5945-5955) | hash de dossier pour éviter de recharger une liste identique ; UID suivant pour détecter les nouveaux | détection de nouveaux messages | `Poll("INBOX", since)` `poll.go:53-62` | OK | |
| `get/set/removeFolderFromCacheList` (l.5961-5978) | | | | SANS OBJET | |
| `get/set/clearMessageFlagsFromCache…`, `initMessageFlagsFromCache`, `storeMessageFlagsToCache…` (l.5985-6110) : drapeaux (non lu, étoile, répondu, transféré, accusé de lecture, **supprimé**) et agrégats de fil (`hasUnseenSubMessage`, `hasFlaggedSubMessage`) | état des drapeaux dans la liste, y compris fils | drapeaux affichés | répondu/transféré/important/étoile `templates/home.html:83,91` ; **drapeau `\Deleted` non rendu** (grep `Deleted` : aucun dans `mail.go`/`home.html`) | PARTIEL | Rendre `\Deleted` : ligne à 70 % d'opacité, expéditeur/objet barrés (CSS RainLoop app.css l.8852-8860). P3 |
| `populateAuthSuffix`, `subQueryPrefix`, `root`, `rootAdmin`, `rootUser`, `admin`, `about`, `phpInfo` (l.6194-6554) | URL de l'application, admin, à propos | | | SANS OBJET | Admin / à propos. |
| `attachmentRaw/Download/Preview/ThumbnailPreview/PreviewAsPlain/Framed` (l.6237-6285) | liens de téléchargement / aperçu / miniature / « voir en texte » / visionneuse intégrée | | `/mail/attachment?…[&inline=1]` `mail.go:925-928`, miniature en data-URI `mail.go:934-939`, zip `server.go:143` ; pas de « voir comme texte » | OK | Visionneuse Office (`FramedView`, Google/Office) : SANS OBJET. |
| `serverRequest`, `upload`, `uploadContacts`, `uploadBackground`, `append`, `change`, `ajax` (l.6291-6337) | URLs d'envoi de fichiers, d'ajout EML, de changement de compte | | `/compose/attach`, `/contacts/import`, `/settings/background`, `/mail/append`, `/accounts/select` `server.go:135,156,169,147,191` | OK | |
| `messageViewLink`, `messageDownloadLink` (l.6343-6353) | voir/télécharger la source | | `/mail/source` + téléchargement `.eml` `MessageView.SourceURL` `mail.go:130` | OK | |
| `avatarLink` (l.6359), `publicLink` (l.6367) | avatar, lien public | | | SANS OBJET | |
| `userBackground` (l.6375) | image de fond utilisateur | | `/user/background` `server.go:171` | OK | |
| `langLink` (l.6391) | fichier de langue | | | SANS OBJET | |
| `exportContactsVcf/Csv` (l.6398-6407) | export contacts | | `/contacts/export?format=csv ou vcf` `contactsweb.go:337-344` | OK | |
| `socialGoogle/Twitter/Facebook` (l.6413-6432) | connexion sociale | | | SANS OBJET | |
| `staticPrefix`, `emptyContactPic`, `sound`, `notificationMailIcon` (l.6439-6463) | ressources statiques : photo vide, son (`new-mail.mp3/ogg`), icône de notification | son de nouveau message, icône | bip synthétisé `static/poll.js:15-25` ; pas d'icône de notification | PARTIEL | Voir `initUidNextAndNewMessages` : P3. |
| `openPgpJs/WorkerJs/WorkerPath` (l.6468-6482) | | | | SANS OBJET | OpenPGP. |
| `themePreviewLink` (l.6490) : aperçu image du thème | aperçu des thèmes dans les réglages | | aucun aperçu de thème | MANQUANT | Imagettes d'aperçu des thèmes (statiques) dans le sélecteur. P3 |
| `inbox`, `settings`, `mailBox(folder,page,search,threadUid)` (l.6505-6597) : `#/mailbox/<dossier>~<fil>/p<page>/<recherche>` | URL de la liste | | `listParams.URL()` `mail.go:196-206` (`?folder=&page=&q=&thread=`) | OK | |

---

## 11. Module 16 `Remote/User/Ajax` : chaque requête serveur (l.6602-7535)

| Requête (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `folders` (l.6658) | charge l'arbre des dossiers + dossiers système | arbre des dossiers | `folderPanel` `foldertree.go:40` | OK | |
| `login` (l.6679) : email, login, mot de passe, SignMe, langue, code 2FA, « se souvenir du 2FA » | connexion | | `POST /login`, `/login/2fa` `server.go:121,126` ; pas de « SignMe » ni « se souvenir du 2FA » | PARTIEL | P3 (voir `LoginSignMeType`). |
| `getTwoFactor, createTwoFactor, clearTwoFactor, showTwoFactorSecret, testTwoFactor, enableTwoFactor, clearTwoFactorInfo` (l.6695-6751) | gestion 2FA | | `/settings/2fa*` `server.go:172-177` (QR, activer, codes de secours, désactiver) | OK | |
| `contactsSync` (l.6757), `saveContactsSyncData` (l.6769) | synchro CardDAV | | `/contacts/sync/run`, `/contacts/sync/save` `server.go:195-196` | OK | |
| `accountSetup`, `accountDelete`, `accountsAndIdentitiesSortOrder` (l.6785-6819) | comptes additionnels | | `/accounts/save`, `/delete`, `/move` `server.go:131-138` | OK | |
| `identityUpdate`, `identityDelete` (l.6832-6853) : id, email, nom, reply-to, **bcc**, signature, « signature avant » | identités | | `identitiesSave` : nom, e-mail, reply-to, bcc, signature `identities.go:67-80` ; « signature avant le texte cité » `accounts.go:127` (`SignatureBefore`) | OK | |
| `accountsAndIdentities` (l.6859) | liste comptes/identités | | | OK | |
| `accountsCounts` (l.6867) : **non lus des autres comptes** affichés dans le menu des comptes | pastille de non lus par compte | | menu des comptes sans compteurs `templates/base.html:152` ; le sondage ne couvre que le compte actif `poll.go:44-62` | MANQUANT | Ajouter un compteur de non lus (STATUS UNSEEN de INBOX) par autre compte dans le menu des comptes, rafraîchi par le sondage. P2 si plusieurs comptes sont utilisés (à confirmer), sinon P3. |
| `filtersSave`, `filtersGet` (l.6878-6893) | filtres Sieve | | `/settings/filters*` `server.go:178-183` | OK | |
| `templates`, `templateGetById`, `templateDelete`, `templateSetup` (l.6900-6939) | modèles de message | | `/settings/templates*` `server.go:165-168`, insertion `static/compose.js:220-249` | OK | |
| `messageList` (l.6951-6990) : dossier, offset, limite, recherche, UidNext, fils, **hash de dossier pour éviter un rechargement identique**, timeout 300 s en recherche | liste | | `home` `mail.go:472-650` ; pas de cache par hash (rendu serveur) ; timeout 30 s partout | PARTIEL | Timeout de recherche : voir `SEARCH_AJAX_TIMEOUT` (P3). |
| `messageUploadAttachments` (l.6997) | joindre les pièces d'un message (transfert « en pièce jointe ») | | `modeForwardAt` `compose.go:42` | OK | |
| `message` (l.7010) | charge un message (clé base64 dossier/uid) | | `mailPane` `mail.go:724` | OK | |
| `composeUploadExternals` (l.7027) | télécharge des images externes pour les intégrer | | images collées en data-URI `static/editor.js:28-58` | OK | |
| `composeUploadDrive` (l.7039) | Google Drive | | | SANS OBJET | Google Drive. |
| `folderInformation` (l.7052-7091), `folderInformationMultiply` (l.7098) : compteurs et drapeaux d'un ou plusieurs dossiers | relève des compteurs | | `mailPoll` `poll.go:44-62` (INBOX + `CheckFolders`) | OK | |
| `logout` (l.7108) | déconnexion | | `POST /logout` `server.go:127` | OK | |
| `messageSetFlagged`, `messageSetSeen` (l.7119-7140) | étoile, lu/non lu | | `action=flag, unflag, read, unread` `mail.go:1327-1334` | OK | |
| `messageSetSeenToAll` (l.7149) : **tout marquer lu** dans le dossier (ou un fil) | « Marquer tout comme lu » | | `action=readall` `mail.go:1272` ; bouton `templates/base.html:123` | OK | |
| `saveMessage` (l.7181) : brouillon avec identité, dossier, destinataires, objet, HTML/texte, pièces, DraftInfo, in-reply-to, references, important | enregistrer le brouillon | | `/compose/save` `server.go:155`, autosave 60 s `static/compose.js:198-200` | OK | |
| `sendReadReceiptMessage` (l.7211) | envoi d'accusé de lecture | | `/mail/mdn` `server.go:150`, `compose.go:1010-` | OK | |
| `sendMessage` (l.7243) : DSN, demande d'accusé, important | envoi | | `/compose/send` ; options `dsn`, `receipt`, `important` `compose.go:103`, `templates/compose.html:73-74` | OK | |
| `saveSystemFolders` (l.7271) | dossiers système | | `/settings/folders/system` `server.go:163` | OK | |
| `saveSettings`, `saveSettingsHelper` (l.7280-7298) : un réglage à la fois | enregistrement | | `/settings/preferences` (formulaire complet) `server.go:199` | PARTIEL | Voir « Enregistrement automatique des réglages ». P3 |
| `changePassword` (l.7306) | mot de passe | | `/settings/password` `server.go:197` | OK | |
| `folderClear` (l.7318) | vider un dossier (corbeille/spam) | | `/mail/clear` `server.go:146` | OK | |
| `folderSetSubscribe` (l.7330), `folderSetCheckable` (l.7343) | (dés)abonner ; marquer « à relever » | | `/settings/folders/action`, `/settings/folders/check` `server.go:162,164` | OK | |
| `messagesMove` (l.7359) : `Learning` (SPAM/HAM), `MarkAsRead`, retire de la liste | déplacer | | `action=move` `mail.go:1351-1355` | OK | `Learning` : apprentissage anti-spam côté serveur (plugin) → SANS OBJET. |
| `messagesCopy` (l.7376) | copier | | `action=copy` `mail.go:1356-1360` | OK | |
| `messagesDelete` (l.7390) | supprimer (corbeille ou définitif) | | `action=delete` `mail.go:1365-1372` | OK | |
| `appDelayStart` (l.7401) | démarrage différé de l'application | | | SANS OBJET | |
| `quota` (l.7409) | occupation de la boîte | | `quotaView` `mail.go:1392-1402`, barre `templates/base.html:220` | OK | |
| `contacts` (l.7420) : pagination + recherche | liste de contacts | | `/contacts` `server.go:188`, `contactsweb.go:110-128` | OK | |
| `contactSave`, `contactsDelete` (l.7435-7448) | enregistrer/supprimer | | `/contacts/save`, `/delete` `server.go:189-190` | OK | |
| `suggestions` (l.7460) : requête + page | auto-complétion d'adresse | | `/contacts/suggest` `server.go:193`, `static/contacts-ac.js` | OK | |
| `clearUserBackground` (l.7471) | retirer le fond | | `/settings/background/delete` `server.go:170` | OK | |
| `facebookUser/Disconnect`, `twitterUser/Disconnect`, `googleUser/Disconnect`, `socialUsers` (l.7479-7527) | comptes sociaux | | | SANS OBJET | |

---

## 12. Module 26 `Stores/Social` et module 27 `Stores/User/App` (début)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `SocialStore` : google/facebook/twitter/dropbox, `populate`, `appendDropbox` (l.7869-7955) | connexions sociales, Dropbox | | | SANS OBJET | Connexion sociale, Dropbox/Google Drive/OwnCloud. |
| `AppUserStore` début (l.7987-8000) : `currentAudio`, `focusedState` → bascule la portée clavier (MessageList / MessageView / FolderList) | focus clavier | | voir `Focused`/`KeyState` | PARTIEL | Suite dans la tranche suivante (non lue ici). |

---

## 13. Bootstrap webpack et bibliothèques/build (nommés, non analysés)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Bootstrap webpack (l.1-88) | chargeur de modules | | | SANS OBJET | Build. |
| Module 3 `window._` (l.3366) | underscore | | | SANS OBJET | Bibliothèque externe. |
| Module 4 `window` (l.3377) | objet global | | | SANS OBJET | |
| Module 7 `window.jQuery` (l.3768) | jQuery | | | SANS OBJET | Bibliothèque externe. |
| Module 21 `window.key` (l.7623) | keymaster (raccourcis clavier, portées, filtre) | raccourcis | `static/keys.js`, `static/listnav.js` en JS natif | SANS OBJET | Comportement couvert dans `Globals.keyScope`. |
| Module 10 `inheritsLoose`, 19 `applyDecoratedDescriptor` (l.4722, 7549) | helpers Babel | | | SANS OBJET | Build. |
| Modules 12, 17, 20 core-js `find`, `map`, `getOwnPropertyDescriptor` (l.4969, 7537, 7612) | polyfills | | | SANS OBJET | Build. |
| Module 18 (l.7548) | module vide | | | SANS OBJET | |

---

# À AJOUTER (PARTIEL et MANQUANT uniquement, par priorité)

Chaque fiche est autonome : elle dit quoi changer, où, et le comportement RainLoop à reproduire.

## P1 — quotidien et clairement visible

### A1. Menu « Déplacer vers » / « Copier vers » : ordre, noms et indentation comme RainLoop
- Comportement RainLoop (`folderMenuForMove` l.4870, `folderListOptionsBuilder` l.2798-2897) : en tête les dossiers système dans l'ordre Boîte de réception, Envoyés, Brouillons, Indésirables, Corbeille, Archives (noms **traduits**) ; un séparateur ; puis les dossiers utilisateur dans l'ordre de l'arbre, **indentés** de trois espaces par niveau, avec le seul nom du dossier (pas le chemin complet) ; le dossier courant apparaît **grisé** (désactivé), les dossiers non sélectionnables aussi ; seuls les dossiers abonnés sont proposés.
- État wmail-go : `foldertree.go:165-169` ne renvoie que des noms IMAP bruts dans l'ordre du serveur (`[]string`), affichés tels quels dans `templates/base.html:101-107` (boutons `move_to`/`copy_to`) ; le dossier courant est omis ; aucun libellé traduit ni indentation.
- À faire : (1) dans `folderPanel`, produire une liste d'options `{Name, Label, Depth, Sep, Disabled}` en réutilisant l'ordre de `rows` : d'abord les lignes `System` (déjà dans l'ordre voulu, libellé via la même fonction que `$.FolderName`, `base.html:204`), un séparateur, puis les lignes de l'arbre avec `Label` = dernier segment et `Depth` ; marquer `Disabled` pour le dossier actif et pour les nœuds non sélectionnables (parents synthétiques). (2) remplacer `MoveTargets []string` (`mail.go:83,119,522-523,945`) et `ListView.MoveMenu` (`server.go:454`) par ce type. (3) dans `base.html:101-107`, rendre `<hr>` pour `Sep`, `padding-left: calc(var(--d) * 1em)` pour l'indentation, `disabled` pour les options grisées. (4) appliquer le même constructeur au `<select>` de `templates/folders_settings.html:17,40` et au choix de dossier des filtres (`filter_edit.html`).
- Priorité : P1.

## P2 — utile et visible

### A2. Bouton « Pas un spam » et visibilité du bouton « Spam »
- RainLoop (l.34663-34667, `deleteMessagesFromFolder` l.28707-28710) : dans le dossier Indésirables, le bouton « Spam » est remplacé par « Pas un spam », qui déplace vers INBOX ; le bouton Spam est masqué dans Brouillons, Envoyés et Indésirables.
- wmail-go : `templates/base.html:113` toujours affiché ; dans Indésirables l'action `spam` ne fait rien (`mail.go:1341-1349`).
- À faire : ajouter l'action `notspam` (liste blanche `mail.go:1317-1319`, bouton dans `mailbar`) qui fait `c.Move(folder, uids, "INBOX")` ; masquer `spam` quand le dossier courant est Brouillons/Envoyés/Indésirables (le template dispose de `.List.ClearKind` pour Indésirables ; ajouter `IsSent`/`IsDrafts`). Icône et libellé `mail.act.notspam` (fr « Pas un spam »).

### A3. Liens `mailto:` d'un message : ouvrir la composition de wmail-go
- RainLoop (`mailToHelper` l.3268-3319) : cliquer une adresse dans un message ouvre sa composition avec à/cc/bcc/objet/corps renseignés.
- wmail-go : `parseMailto` existe (`compose.go:971-998`, route `/compose?mailto=`), mais `mailview/sanitize.go:171` laisse les `href="mailto:…"` tels quels : l'iframe sandboxé lance le client mail du système.
- À faire : dans `Sanitize` (nœud `a` de `walk`, `sanitize.go:142-146`), réécrire `mailto:X` en `/compose?mailto=` + `url.QueryEscape("mailto:X")` et poser `target="_blank"` (l'iframe n'autorise pas `allow-top-navigation`, `mail.go:1047-1048`). Même réécriture dans `linkify` du texte brut. Dédoublonner les adresses de `to=` et du chemin en gardant celle qui a un nom.

### A4. Liens automatiques (URL et adresses e-mail) dans le HTML et le texte
- RainLoop (`findEmailAndLinks` l.2605, appliqué dans `setMessage` l.5545 au HTML **et** au texte) : toute URL `http…` et toute adresse e-mail dans le texte devient un lien (`target=_blank`).
- wmail-go : texte brut seulement, URLs seulement (`mailview/text.go:11-48`) ; HTML : rien ; e-mails : jamais.
- À faire : après l'assainissement, parcourir les nœuds texte hors `<a>` (comme `CollapseQuotes`, `sanitize.go:235`) et entourer `https?://…` et `[\w.+-]+@[\w-]+(\.[\w-]+)+` d'un `<a>` (les e-mails vers `/compose?mailto=…`, voir A3) ; ajouter les e-mails à `linkify` (`text.go:38`).

### A5. `HTMLToText` : citations, listes, espaces, coupe à 100 colonnes
- RainLoop (`htmlToPlain` l.2624-2713, `splitPlainText` l.2110) : `blockquote` → lignes préfixées `> ` (imbriquées `>> `), `<li>` → ` * `, `<hr>` → `_______________________________`, `\s+` des nœuds texte réduit à un espace, `<p>` → ligne vide, coupe à 100 colonnes aux espaces.
- wmail-go : `mailview/text.go:52-85` ne fait que des sauts de ligne aux blocs ; utilisé pour la partie texte des messages HTML envoyés (`compose/html.go:141`, `compose.go:580`), pour la signature (`compose.go:183`), pour citer un HTML dans une réponse en texte brut (`compose.go:302`) et pour la vue « texte » (`mail.go:1036`).
- À faire : étendre `HTMLToText` : empiler la profondeur de `blockquote` et préfixer chaque ligne de texte par `> ` × profondeur ; `li` → `* ` ; `hr` → ligne ; réduire les espaces/sauts de ligne des nœuds texte (hors `pre`) à un espace ; paramètre optionnel de largeur (100) pour la partie texte sortante.

### A6. Barres de défilement fines et discrètes (thème « rainloop »)
- RainLoop (`nano`, app.css l.1422-1500) : barre de 5 px superposée, `rgba(0,0,0,.3)`, visible surtout au survol (`.4` au survol, fond `.15`), pas de barre native visible.
- wmail-go : aucune règle `scrollbar-*` / `::-webkit-scrollbar` (`static/*.css`, `themes/rainloop/theme.css`) : barres natives épaisses.
- À faire : dans `themes/rainloop/theme.css`, sur `.msglist-form`, `.folders`, `.readpane`, `.menu.scroll` : `scrollbar-width: thin; scrollbar-color: rgba(0,0,0,.3) transparent;` et pour WebKit `::-webkit-scrollbar{width:5px;height:5px}` `::-webkit-scrollbar-thumb{background:rgba(0,0,0,.3);border-radius:3px}` (`:hover` → `.4`).

### A7. Infobulles stylées comme RainLoop (Opentip `rainloopTip`)
- RainLoop (`tooltip` l.826-906 ; style l.18649-18653 et app.css l.11515) : bulle fond blanc, bord 1 px `#999`, rayon 2 px, police 13 px, flèche (stem), délai d'apparition 0,4 s, fondu 0,2 s, positionnée sous l'élément ; masquée quand un menu est ouvert.
- wmail-go : `title=` natif partout (ex. `templates/base.html:97-114`).
- À faire : CSS sans JS : `[data-tip]:hover::after` (contenu `attr(data-tip)`, `transition-delay:.4s`) et ajouter `data-tip` à côté de `title` sur les boutons de la barre d'outils, ou petit script qui lit `title` au survol (en retirant `title` pour éviter le doublon). Masquer les bulles quand `details[open]` existe.

### A8. Glisser-déposer : bulle de glisser avec compteur et icône, défilement automatique
- RainLoop (`draggable` l.1235-1314, `draggablePlace` l.2457) : pendant le glissement, une bulle suit le curseur (décalée de 22 px/3 px) avec le nombre de messages et une icône **copie** si Ctrl est enfoncé, **courrier** sinon ; la liste de dossiers défile quand le curseur approche à moins de 100 px de ses bords (3 px/10 ms).
- wmail-go : `static/dnd.js:16-38` n'appelle pas `setDragImage` (fantôme du navigateur) ; pas de défilement automatique.
- À faire : dans `dragstart`, construire un `<div class="draggablePlace">` hors écran contenant « N » et l'icône, l'appeler `e.dataTransfer.setDragImage(el, 3, 22)` ; mettre à jour l'icône selon `e.ctrlKey` dans `dragover` (CSS : `.dnd.copy`). Ajouter un `dragover` sur `.folders` qui fait `scrollTop ±=3` par pas de 10 ms quand `clientY` est à moins de 100 px des bords.

### A9. Erreurs d'action non bloquantes
- RainLoop (`Notification` 201-205, `removeMessagesFromList`) : en cas d'échec de suppression/déplacement/copie, un message s'affiche, la liste reste en place.
- wmail-go : `mailErr` (`mail.go:440-455`) rend une page d'erreur pleine page (`s.message`).
- À faire : pour les POST `/mail/action`, `/mail/clear`, rediriger vers `back` avec `?err=<clé>` et afficher la clé via le bloc `messages` existant au-dessus de la liste (`templates/home.html`), ou rendre un bandeau dans le volet liste.

### A10. Nouveaux messages dans la liste ouverte
- RainLoop (`setMessageList` l.5678-5755, `initUidNextAndNewMessages` l.5254) : le rafraîchissement périodique intègre les nouveaux messages dans la liste affichée (animation `newForAnimation` sur au plus 5 lignes).
- wmail-go : `static/poll.js:41-71` ne met à jour que le titre, les badges et les notifications ; la liste reste figée jusqu'à un rechargement.
- À faire : quand le sondage détecte des nouveaux (`d.new`), si on est sur la page 1 de la boîte de réception, afficher un bandeau « N nouveaux messages — afficher » (lien vers l'URL courante) ou recharger discrètement le fragment de liste (comme `/mail/pane`) et mettre la classe `new` (animation CSS) sur les nouvelles lignes.

### A11. Compteurs de non lus des autres comptes
- RainLoop (`accountsCounts` l.6867) : pastille de non lus par compte dans le menu des comptes.
- wmail-go : `templates/base.html:152` sans compteurs ; `poll.go:44-62` interroge seulement le compte actif.
- À faire : dans `mailPoll`, ajouter `accounts: {id: unseen}` (STATUS UNSEEN de INBOX pour chaque autre compte, avec la même mise en pool) et afficher un `<span class="badge">` dans `base.html:152`. P2 si l'utilisateur utilise plusieurs comptes (à confirmer), sinon P3.

## P3 — détails et à confirmer

### A12. Disposition « aperçu en bas »
`Layout.BottomPreview` (l.394). Ajouter `value="bottom"` à `templates/settings.html:20-23`, `Layout` dans `userdata.Settings`, classe `.mailbox.bottom` (liste au-dessus, lecture dessous) et séparateur horizontal glissable. À confirmer si l'utilisateur l'utilise.

### A13. Menu des tailles de page 10/20/30
RainLoop propose 10, 20, 30, 50, 100 (l.7743, défaut 20). `pageSizes` (`mail.go:40`) → `[]int{10, 20, 30, 50, 100, 200, 500}` (garder la valeur courante via `pageSizeChoices`). Défaut éventuellement 20.

### A14. Délai de recherche
`SEARCH_AJAX_TIMEOUT` = 300 s (l.7746) contre 30 s (`mail.go:32`, `mailCtx` `:435`). Utiliser un contexte de 300 s quand `lp.Q != ""` (`mail.go:501`).

### A15. Notification de bureau : icône, titre de compte, son RainLoop
`static/poll.js:27-39` : ajouter `icon` (enveloppe), pour plus de 3 nouveaux un titre = adresse du compte (champ `data-account` sur `#poll`), et éventuellement un son de fichier (`/static/new-mail.ogg` fourni par l'utilisateur) à la place du bip synthétisé (`poll.js:15-25`).

### A16. Pagination : « … » cliquable
`computedPagenatorHelper` (l.3091-3180) : l'ellipse est un lien vers la page médiane (`round((prev-1)/2)` à gauche, `round((total+next)/2)` à droite), et un numéro isolé (ex. page 2 ou n-1) est affiché au lieu de « … ». `mail.go:161-192` + `templates/home.html:105`.

### A17. Style du bouton de citation repliée
`mail.go:1002` : `details.wm-quote>summary` → 30 px x 14 px, `line-height:14px`, `background:#eee`, `border:1px solid #999`, `opacity:.5` (`1` au survol), `margin:10px 0`, texte « … ». Seuil : replier quand le bloc dépasse ~100 px de hauteur (RainLoop) plutôt que 400 caractères (`sanitize.go:263`) ; non réalisable sans JS dans l'iframe : garder le seuil caractères mais l'abaisser vers 250 pour se rapprocher.

### A18. `\Deleted` visible
Rendre le drapeau `\Deleted` dans la liste : champ `Deleted` dans la ligne (`mail.go:603`), classe `.deleted` : `opacity:.7`, `text-decoration:line-through` sur `.sender`, `.subject` (CSS RainLoop app.css l.8852-8860).

### A19. Préfixe d'objet : conserver l'autre préfixe
`compose/reply.go:14-37` : répondre à « Fwd: x » donne « Re: Fwd: x », transférer « Re: x » donne « Fwd: Re: x » (RainLoop l.2232-2266). Après nettoyage, mémoriser si les préfixes retirés contenaient `fwd` (pour une réponse) ou `re` (pour un transfert) et les remettre derrière le préfixe voulu, un seul de chaque.

### A20. Format des tailles
`humanSize` (`mail.go:405-422`) : RainLoop affiche `13KB`, `1.5MB`, `1.2GB` (KB arrondi à l'entier, pas d'espace, unités anglaises). À aligner si l'utilisateur veut le rendu strict (sinon garder « Ko/Mo » : choix de langue).

### A21. Citations imbriquées en texte brut
`mailview/text.go:15-35` : produire un `<blockquote>` par niveau (`>`, `> >`, `>>`) comme `plainToHtml` (l.2721-2779).

### A22. Champs de contact Skype / GitHub / Facebook
`ContactPropertyType` (l.457) : ajouter trois URL typées ou un champ « Réseaux » dans `contacts.Contact` (`contacts/contacts.go:51-71`) avec formulaire `templates/contacts.html` et export vCard (`X-SKYPE`, `X-GITHUB`, `X-FACEBOOK`). À confirmer.

### A23. Photo de contact / d'expéditeur
`resizecrop`, `resizeAndCrop` (l.1009, 3238), `getUserPic` (l.5837) : photo carrée 100x100 (JPEG) pour un contact et dans l'en-tête du message (`templates/base.html:262` : silhouette fixe). Lire `PHOTO` de la vCard (le CardDAV la perd aujourd'hui, `contacts/carddav.go:50`), la recadrer côté serveur, l'afficher dans `.avatar`. Gravatar : exclu (décision CAHIER `:595`).

### A24. Libellés des thèmes, aperçu et application immédiate
`convertThemeName` (l.2410), `themePreviewLink` (l.6490), `changeTheme` (l.3029). `templates/settings.html:11-12` affiche `rainloop`/`dark`/`light` : ajouter les libellés (« DarkShine », « Sombre », « Clair »), une imagette d'aperçu, et appliquer `data-theme`/la feuille `/themes/<nom>/theme.css` dès le `change` du sélecteur.

### A25. Enregistrement automatique des réglages avec témoin
`saveTrigger` (l.1371-1429), `settingsSaveHelper*` (l.2526-2599) : chaque réglage s'enregistre à la modification ; à côté du champ : spinner, coche verte ou croix rouge (≈1 s). Script : au `change` d'un champ de `templates/settings.html`, `fetch` POST `/settings/preferences` (champ unique ou formulaire complet), afficher l'état à côté. Garder le bouton sans JavaScript.

### A26. Navigation clavier dans le panneau des dossiers et dans le menu des réglages
`KeyState.FolderList`/`Focused.FolderList` (l.173, 133 ; RainLoop `app.js:34139-34187`) : ↑/↓ déplacent le curseur dans les dossiers, Entrée ouvre, Espace plie/déplie, →/Tab/Échap rend le focus à la liste. `settingsMenuKeysHandler` (l.4403) : ↑/↓ dans le menu des réglages. À ajouter dans `static/listnav.js` (zone `.folders`) et un petit script de réglages.

### A27. Raccourcis ignorés quand un menu ou une boîte de dialogue est ouvert ; réglage d'activation
`dropdownVisibility`, `keyScope Menu`, `useKeyboardShortcuts` (l.3527-3534, 3707-3753) : `static/keys.js:35` ne teste pas `details[data-dropdown][open]` ni `dialog[open]` (alors que `listnav.js:10` teste `dialog[open]`) : `c`, `r`, `Suppr` agissent derrière un menu ouvert. Ajouter ces deux tests à `typing()`. Optionnel : case « Raccourcis clavier » dans les préférences.

### A28. Icônes de type de pièce jointe
`FileType` (l.103) : classer l'extension/type MIME en familles (pdf, image, audio, video, texte, code, tableur, présentation, archive, certificat, eml, word) et colorer la pastille `.ext` (`templates/base.html:301`, `mail.go:829-839`) ou afficher un symbole par famille.

### A29. Mémoriser le détail des en-têtes
`ClientSideKeyName.MessageHeaderFullInfo` (l.280) : mémoriser l'état ouvert de `details.msg-info` (`templates/base.html:237`) dans `localStorage` et le réappliquer au rechargement du volet (`wmail:pane`).

### A30. Divers cosmétiques RainLoop non reproduits (un seul lot, P3)
- Ombres haut/bas des zones défilantes (`scrollerShadows` l.771, app.css l.7238-7256).
- Fondu des alertes, des modales (`visibleAnimated` l.801, `modal` l.1100), message d'erreur en bulle rouge (`tooltipErrorTip` l.908).
- Animation de disparition des lignes déplacées et clignotement du dossier cible (`removeMessagesFromList` l.5351-5377).
- Dates relatives rafraîchies sans rechargement (`moment` l.1134).
- Confirmation à deux temps pour les suppressions de réglages (`deleteAccessHelper` l.1741) au lieu de `window.confirm`.
- Regex d'adresse des pastilles (`static/chips.js:8`) : accepter `x@localhost` comme RainLoop (l.1724).
- Case « se souvenir de moi » à la connexion (`LoginSignMeType` l.232) : à confirmer.
- Décodage TNEF `winmail.dat` (`mimeContentType` l.3201) : seulement si l'utilisateur en reçoit.
- Dossiers serveur IMPORTANT/FLAGGED/ALL (`ServerFolderType` l.206) : seulement si le serveur de l'utilisateur les expose.
- Bouton « bidi » (direction du texte) de l'éditeur (`htmlEditorDefaultConfig` l.3584).

---

## Synthèse

- 256 lignes de verdict (une ligne = une unité ou un petit groupe d'unités de même nature, soit environ 400 éléments nommés : 34 énumérations, 54 bindings/extenders knockout, ~70 fonctions Utils, ~60 requêtes Ajax, etc.). Décompte : OK 112, PARTIEL 55, MANQUANT 8, SANS OBJET 81. Chaque ligne cite le code RainLoop lu et le code wmail-go lu (ou « aucun »).
- Points forts constatés : énumérations de composition, sélection par critère, glisser-déposer des messages vers les dossiers (avec dépliage au survol), pastilles d'adresses, sondage et notifications, pagination, pièces jointes (miniatures, zip, aperçu), dates de liste identiques aux formats RainLoop, quota, filtres, modèles, contacts.
- Principaux écarts visibles au quotidien : menu Déplacer/Copier brut (A1), pas de « Pas un spam » (A2), liens mailto: sortent de l'application (A3), pas de liens automatiques (A4), barres de défilement natives et infobulles natives (A6, A7), liste non rafraîchie automatiquement (A10).
