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
