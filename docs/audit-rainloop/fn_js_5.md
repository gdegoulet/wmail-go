# Analyse élément par élément : RainLoop 1.17 `static/js/app.js` lignes 32000 à 40112

Périmètre : fin de `TemplatePopupView`, bibliothèques vendues (classnames, lozad, pilotes de stockage), modèles (Folder, Account, Identity, Template, OpenPgpKey, Contact), requêtes AJAX, écrans (Login, MailBox, Settings), vues (Login, FolderList, MessageList, MessageView, menus de réglages) et pages de réglages (General, Contacts, Accounts, Filters, Security, Social, ChangePassword, Templates, Folders, Themes, OpenPGP), popups (MessageOpenPgp, ComposeOpenPgp, FolderClear, AdvancedSearch, TwoFactorTest, AddOpenPgpKey, NewOpenPgpKey, ViewOpenPgpKey, WelcomePage).

Lignes citées pour RainLoop = numéros de `app.js`. Lignes citées pour wmail-go = fichiers sous `/home/gdegoulet/Work/webmail_project/internal/web/` (abrégés `templates/…`, `static/…`, `mail.go`…), numéros lus dans les fichiers.

Limites de lecture (honnêteté) : les 3 popups OpenPGP de saisie/génération/affichage de clé (39709 à 40070) et le corps de `ComposeOpenPgpPopupView.doCommand` (38933 à 39050) n'ont été parcourus que par leurs signatures de fonctions et leur rôle (chiffrement OpenPGP dans le navigateur : SANS OBJET décidé au CAHIER 14.4). Je n'ai pas lu les gabarits HTML Knoin (`templates/Views/...`) ni le CSS DarkShine pour cette tranche : les différences de rendu signalées viennent du code JS (classes, états, libellés) et des gabarits wmail-go lus, pas d'une comparaison pixel. Aucune capture n'a été faite.

Rappel d'architecture wmail-go : pages rendues côté serveur, JavaScript facultatif (`data-js-only`), actions via formulaires POST ; beaucoup de comportements RainLoop sont donc obtenus autrement (liens, `details`, formulaires).

## 1. Bibliothèques, pilotes, couche AJAX (32100 à 32950)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `classNames` (32119) | Concaténer des noms de classes CSS | Aucun besoin fonctionnel : bibliothèque vendue | aucun | SANS OBJET | Build/lib vendue |
| `lozad` (32178) | Chargement paresseux d'images (IntersectionObserver) | Images de la liste/contacts chargées au défilement | aucun (images de message : `<img>` dans iframe, `mail.go:998-1010`) | SANS OBJET | Lib vendue ; pas d'avatars dans la liste côté wmail-go |
| `CookieDriver` set/get/supported (32211 à 32265) | Stockage client par cookie (secours) | Mémoriser des préférences d'interface côté navigateur | `sessionStorage` pour le défilement (`static/list.js:131-132`) ; préférences côté serveur (`userdata.go:44-69`) | SANS OBJET | Les préférences sont dans le compte chiffré, pas en cookie |
| `LocalStorageDriver` set/get/supported (32295 à 32358) | Stockage local clé/valeur (dernière réponse, dernier « rester connecté », info d'en-tête…) | Mémoriser des états d'interface | Remplacé par réglages serveur : `LastReply` (`userdata.go:61`) ; pas d'équivalent pour « info d'en-tête » et « rester connecté » (voir §5 et §8) | SANS OBJET | Mécanisme interne ; les états visibles sont traités ligne par ligne plus bas |
| `AbstractAjaxPromises.clear/abort` (32762 à 32778) | Annuler les requêtes en cours | Éviter les réponses périmées | `static/list.js:189,196` (compteur `currentRequest`, réponse périmée ignorée) | OK | rien |
| `ajaxRequest` : jeton, `UpdateToken` (32780 à 32815) | Jeton CSRF/session renouvelé | Sécurité des POST | Jeton `csrf` dans chaque formulaire (`templates/base.html:150`, `data-csrf` `templates/home.html:2`) | OK | rien |
| `ajaxRequest` : classement Success/Abort/Error, timeout (32819 à 32858) | Gestion des états de réponse | Retour d'erreur à l'écran | Erreurs rendues par le serveur (`templates/home.html:68-69`), `.catch → location.href` (`static/list.js:213`) | OK | rien |
| `ajaxRequest` : compteurs d'erreurs, jeton invalide, déconnexion forcée (32867 à 32890) | Après N erreurs d'authentification : recharger/déconnecter | Session expirée : retour à la connexion | `static/list.js:192` (`r.redirected` → page de connexion) ; sondage : échecs comptés `static/poll.js:10,72` mais sans retour à la connexion | PARTIEL | Le sondage (`poll.js`) pourrait rediriger vers `/login` sur 401/redirect ; P3 |
| `getRequest` / `postRequest` (32905 à 32915) | Enveloppes GET/POST | Appels d'API | Formulaires et `fetch` ciblés (`static/folders.js:6-9`) | SANS OBJET | Architecture différente |

## 2. Modèles (32412 à 33223, 34314 à 34431)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `FolderModel` champs (32415 à 32445) | État d'un dossier : nom, chemin, profondeur, sélectionnable, abonné, cochable, replié, compteurs | Arbre de dossiers | `FolderRow` (`foldertree.go:100-110`), `mailconn.Folder` (`mailconn/client.go:312-313`) | OK | rien |
| `initByJson` : profondeur, `type` Inbox/User (32671 à 32691) | Construire un dossier à partir du serveur | Dossier + profondeur | `foldertree.go:104` (`Depth`, `DepthClass`) | OK | rien |
| `isInbox`, `isSystemFolder`, `canBeEdited` (32468 à 32487) | Distinguer boîte de réception/système/utilisateur | Boutons selon le type | `row.Protected` (INBOX) `folders.go:96` ; `System` dans `templates/base.html:203` | PARTIEL | RainLoop traite aussi les dossiers système (Envoyés, Corbeille…) comme non modifiables ; wmail-go ne protège que INBOX (voir `canBeDeleted`) |
| `hasSubScribedSubfolders`, `visible`, `hidden` (32471 à 32493) | Masquer un dossier non abonné sauf si un enfant l'est ; masquer les systèmes sans enfants | Arbre filtré par abonnement | `Subscribed: !known \|\| sub[...]` (`folders.go:96`), `f.Hidden` exclu des cibles `foldertree.go:~165` | OK | rien (lu partiellement ; non vérifié visuellement) |
| `selectableForFolderList` (32494) | Dossiers proposés dans les sélecteurs de dossier | Liste de choix | `MoveTargets` = sélectionnables hors dossier actif (`foldertree.go:~163-166`) | OK | rien |
| `messageCountAll/Unread` avec `notify always` (32497 à 32520) | Compteurs réactifs | Mise à jour des badges | `Unseen`/`Total` (`mailconn/client.go:376-386`), mise à jour du badge par `poll.js:52-70` | OK | rien |
| `printableUnreadCount` (32521 à 32535) | Texte du badge : **total** pour Brouillons ; **rien** pour Corbeille/Archive/Envoyés ; non lus ailleurs | Badge dossier | `templates/base.html:204,211` : `{{if gt .Unseen 0}}<span class="badge">{{.Unseen}}` pour **tous** les dossiers, sans règle de type ; `Total` n'est pas affiché | PARTIEL | Calculer `BadgeCount` par rôle : Brouillons → `Total` (>0), Corbeille/Archive/Envoyés → vide, autres → `Unseen` ; le sondage doit suivre la même règle (`poll.js:52-70`). **P1** (volet de gauche, visible chaque jour) |
| `canBeDeleted` (32536) | Supprimer seulement : non système, sans sous-dossier, pas INBOX | Garde-fou de suppression | `folders.go:188-191` : refuse INBOX seulement ; `templates/folders_settings.html:55-65` : bouton « Supprimer » pour tout dossier non protégé, y compris Envoyés/Corbeille et dossiers avec enfants | PARTIEL | Masquer « Supprimer » pour les dossiers à rôle (Role != "") et ceux qui ont des enfants ; **P2** |
| `canBeSubScribed/canBeChecked` (32541 à 32544) | Abonnement/surveillance seulement pour non système et sélectionnable | Boutons œil et surveillance | `templates/folders_settings.html:54` (abonnement pour toutes les lignes) ; surveillance : cases pour `.Check` (`templates/folders_settings.html:28`) | PARTIEL | Les dossiers système restent abonnables chez nous : sans conséquence ; **P3** (masquer l'abonnement des dossiers à rôle pour ressembler à RainLoop) |
| `localName` (32545 à 32581) | Nom traduit des dossiers système (Inbox, Envoyés, Brouillons, Spam, Corbeille, Archive) | Libellés localisés | `Page.FolderName` (`accounts.go:76-96`) clés `folder.inbox` etc. | OK | rien |
| `manageFolderSystemName` (32583 à 32623) | Suffixe « (Boîte de réception) » à côté du nom réel dans les réglages | Repérer les dossiers système | `.Role` en `<small class="role">` (`templates/folders_settings.html:50`) | OK | rien (rendu : petit texte à côté du nom, équivalent) |
| `collapsed`/`collapsedCss`/`hasSubScribedUnreadMessagesSubfolders` (32625 à 32664) | Signe replié/déplié, parent signalé s'il a des non lus dessous | Arbre repliable | `data-open`, caret ▾/▸ (`static/folders.js:13-25`, `templates/base.html:210`), `.unread-sub` en gras (`static/mail.css:465`, `foldertree.go:143-149`) | OK | rien (caractères ▾▸ au lieu des icônes `icon-down-mini` : différence de glyphe minime, **P3**) |
| événement `mailbox.inbox-unread-count` (32650 à 32654) | Titre de l'onglet et nombre du compte mis à jour | Compteur dans le titre | `static/poll.js:12,53` (`(n) titre`) | OK | rien |
| `printableFullName` « A / B » (32697) | Chemin lisible d'un dossier | Affichage | Indentation (`templates/folders_settings.html:50`) et feuille seule dans l'arbre | OK | rien |
| `AccountModel` email, count, deleteAccess, `changeAccountLink` (32951 à 32981) | Compte secondaire : lien de bascule, suppression en deux clics | Plusieurs comptes | `/accounts/select` (`templates/base.html:150-153`), suppression par `<details class="danger">` (`templates/accounts.html:12-19`) | OK | rien |
| `IdentityModel` + `formattedName` « Nom (email) » (33015 à 33043) | Identité : id, e-mail, nom, répondre à, Cci, signature, signature avant | Identités | `templates/identities.html:5,8,23-38` ; format « Nom <email> » au lieu de « Nom (email) » | OK | rien (variante typographique **P3** ignorable) |
| `TemplateModel` + `parse` (33082 à 33111) | Modèle de message id/nom/corps | Modèles | `templates/templates.html:8-15,23-24` | OK | rien |
| `OpenPgpKeyModel` (33159 à 33220) | Clé OpenPGP du trousseau navigateur | Chiffrement PGP | aucun | SANS OBJET | OpenPGP hors périmètre (CAHIER 14.4) |
| `ContactModel` champs, `getNameAndEmailHelper`, `parse` (34317 à 34386) | Contact : nom = prénom + nom, premier e-mail | Carnet | `Contact.Display()` (`internal/contacts/contacts.go:76`) | OK | rien |
| `ContactModel.srcAttr` (34392) / `lineAsCss` (34408) | Photo (URL) ; classes deleted/selected/checked/focused | Avatar + états de ligne | Photos refusées (CAHIER 14.1) ; classes de ligne dans `templates/contacts.html` (non relu ligne à ligne) | SANS OBJET | Gravatar volontairement non repris |

## 3. Écran et vue de connexion (33248 à 33716)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `LoginUserScreen.onShow` titre (33257) | Titre d'onglet vide à la connexion | Titre | `<title>` « wmail-go – … » (`templates/base.html:7`) | OK | rien |
| Champs e-mail, mot de passe, erreurs et animation de secousse `emailErrorAnimation` 500 ms (33347 à 33409) | Champ en erreur surligné et animé | Retour visuel de saisie | Validation `required` + message serveur (`templates/login.html:8-10`, `templates/base.html:194`) ; pas d'animation ni de surlignage de champ | PARTIEL | Ajouter une classe d'erreur sur le champ + courte animation CSS ; **P3** |
| Code à deux facteurs affiché dans le même formulaire (`additionalCode`, 33350 à 33357, 33544) | Champ « code » apparaît après le mot de passe | 2FA | Page séparée `templates/login_2fa.html:6-11` (champ code, case « ne plus demander ») | OK | rien (flux différent, même fonction ; « ne plus demander » = `additionalCodeSignMe`) |
| `signMe` / `LastSignMe` « Rester connecté » (33349, 33432, 33582, 33637 à 33659) | Case qui prolonge la session, mémorisée | Session longue | aucune case ; durées réglables dans les préférences (`templates/settings.html:13-16`, défaut 60 min / 12 h `config.go:148-151`) | PARTIEL | Ajouter une case « Rester connecté » sur `login.html` qui applique la durée absolue maximale (`AbsoluteHoursMax` 72 h) ; **P3** (la durée par défaut du compte couvre l'usage quotidien) |
| Connexion sociale Facebook/Google/Twitter (33438 à 33481, 33620 à 33674) | Boutons de connexion tierce | SSO | aucun | SANS OBJET | Hors périmètre |
| `submitCommand` : validation, focus sur le premier champ en erreur (33483 à 33514) | Empêche l'envoi vide et place le curseur | Saisie | `required` natif + `autofocus` sur l'identifiant (`templates/login.html:8`) | OK | rien |
| Plug-in `user-login-submit` (33524) | Point d'extension | Plugins | aucun | SANS OBJET | Plugins hors périmètre |
| Traitement de la réponse : `TwoFactorAuth`, `Admin`, `ErrorCode`, `ErrorMessageAdditional` (33542 à 33581) | Redirections et messages d'erreur détaillés | Connexion | `/login`, `/login/2fa` (`templates/login.html:6`, `login_2fa.html:6`) ; message d'erreur `messages` | OK | rien (panneau admin SANS OBJET) |
| `onShowWithDelay` : focus mot de passe si e-mail prérempli (33597 à 33607) | Curseur au bon champ | Confort | `autofocus` toujours sur l'identifiant même prérempli (`templates/login.html:8`) | PARTIEL | Mettre `autofocus` sur le mot de passe quand `.Login` est non vide ; **P3** |
| Choix de la langue sur la page de connexion + popup `Languages` (33421 à 33429, 33697) | Liste de langues | Langue avant connexion | liens de langues `templates/login.html:17` | OK | rien (liens au lieu d'une fenêtre) |
| `forgotPasswordLinkUrl`, `registrationLinkUrl` (33362 à 33363) | Liens « mot de passe oublié » / inscription | Récupération | `templates/login.html:14-15` | OK | rien |
| Logo/description de connexion, `hideSubmitButton`, `mobile`, `AdditionalLoginError` (33358 à 33361, 33345, 33450) | Personnalisation admin, mobile | — | aucun | SANS OBJET | Admin / mobile |
| `selectLanguageOnTab` (33701) | Tab depuis la langue revient à l'e-mail | Clavier | ordre de tabulation natif | SANS OBJET | Détail de widget |

## 4. Écran boîte aux lettres (33781 à 33940)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `updateWindowTitle` (33794 à 33803) | Titre : « (N) adresse – Boîte de réception » | Savoir qui/combien depuis l'onglet | `static/poll.js:12` : « (N) wmail-go – Boîte… » (`templates/base.html:7,18`) ; l'adresse du compte n'y figure pas | PARTIEL | Ajouter l'adresse active au titre (`TopUser`/`User`) comme `(N) adresse – titre` ; **P3** |
| `onShow` : focus liste, volets (33809 à 33822) | Focus clavier sur la liste à l'ouverture | Clavier | `listnav.js` écoute le document entier, aucun état de focus | OK | rien |
| `onRoute` : dossier + page + recherche + fil `~uid` dans l'URL (33832 à 33847) | Adresse = état | Liens partageables | paramètres `?folder=&page=&q=&thread=` (`mail.go:209-246`, `templates/home.html:21`) | OK | rien |
| `onStart` : composeur préchargé 500 ms, compteur Inbox → compte (33853 à 33880) | Précharger la fenêtre d'écriture | Rapidité | aucun (page `/compose` séparée) | SANS OBJET | Architecture sans popup |
| `onBuild` : redimensionnement horizontal, clic `#rl-right` referme panneau (33886 à 33896) | Séparateur de volets (disposition horizontale) | Redimensionnement | séparateurs verticaux `static/list.js:233-333` (glisser, clavier, double-clic reset) | OK | rien (séparateur horizontal : voir Layout §9) |
| `routes` (33902 à 33936) : `/dossier/pN/recherche` | Navigation | Pagination/recherche | Pagination `templates/home.html:104-106` | OK | rien |

## 5. Menu utilisateur et liste des dossiers (33965 à 34280)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `SystemDropDownMailBoxUserView` (33965 à 33977) | Menu compte : comptes, réglages, déconnexion | Menu utilisateur | `templates/base.html:147-161` (liste de comptes avec ✓, Réglages, Comptes, raccourcis, Déconnexion) | OK | rien |
| `FolderList` champs : `isInboxStarred`, autorisations (34050 à 34070) | État des boutons | Boutons | `$on` pour l'étoile (`templates/base.html:204`) | OK | rien |
| Clic dossier : sélectionner, rafraîchir si dossier courant, vider le message en mode sans aperçu (34082 à 34118) | Navigation dossier | Choix dossier | lien `.fname` (`templates/base.html:204,211`) | OK | rien |
| Clic sur le signe replier/déplier (34121 à 34131) | Replier un niveau, état mémorisé | Arbre | `static/folders.js:28-40` + POST `/settings/folder-open` | OK | rien |
| Clic sur l'étoile de la boîte de réception : filtre `is:flagged` (34132 à 34134, 34068) | Messages suivis | Filtre étoilé | `a.fstar` → `/?q=is:flagged` (`templates/base.html:204`), style `static/mail.css:459` | OK | rien |
| Clavier de la liste des dossiers : ↑↓, Entrée, Espace (replier), Échap/Tab/→ revient à la liste (34139 à 34191) | Navigation clavier dans les dossiers | Clavier | aucun (aucun état « focus dossiers » ; `keys.js` n'a rien pour les dossiers) | MANQUANT | Ajouter dans `static/folders.js` : touche ← depuis la liste = focus dossier courant ; ↑↓ déplacent `.focused`, Entrée ouvre, Espace plie ; **P3** |
| Surbrillance `focused`/`selected` selon l'état de focus (34192 à 34198) | Dossier actif visible au clavier | Rendu | `.active` seulement (`templates/base.html:203`) | PARTIEL | Style `:focus-visible` des liens `.fname` ; **P3** |
| `messagesDropOver/Out` : déplier un dossier survolé après 500 ms (34201 à 34215) | Glisser sur dossier replié | DnD | `static/dnd.js:48-55,57` (500 ms, `caret.click()`) | OK | rien |
| `scrollToFocused` (34217 à 34239) | Faire défiler pour garder le dossier focus visible | Clavier | aucun (non nécessaire sans focus dossier) | MANQUANT | Lié à la ligne « clavier des dossiers » ; **P3** |
| `messagesDrop` : déplacer, Ctrl = copier (34247 à 34257) | Déposer des messages sur un dossier | DnD déplacer/copier | `static/dnd.js:60-72` (`move_to`/`copy_to`, Ctrl ou Alt) ; cible `.drop-over` `static/mail.css:430` | OK | rien |
| Aide de glisser : bulle avec le nombre de messages (`dragAndDronHelper`, 34915 à 34933) | Image portée par le curseur « N » | Retour visuel DnD | `static/dnd.js:30-33` (`text/plain` = nombre, mais image de fantôme = la ligne) | PARTIEL | Fournir `setDragImage` avec une pastille « N » ; **P3** |
| `composeClick`, `contactsClick`, `createFolder`, `configureFolders` (34259 à 34271) | Boutons du bas du volet | Écrire, contacts, créer dossier, réglages dossiers | `templates/base.html:97-98,216-218` (bouton « Nouveau », contacts, création, réglages) ; création en boîte de dialogue `templates/base.html:222-231` | OK | rien |
| `foldersListWithSingleInboxRootFolder`, `moveAction` mobile (34056 à 34060) | Dispositions mobiles | Mobile | aucun | SANS OBJET | Mobile |

## 6. Liste des messages (34541 à 35400)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Modèle de ligne selon la disposition (`messageListItemTemplate`, 34598) | Ligne plus compacte hors aperçu latéral | Rendu de ligne | un seul gabarit `templates/home.html:83-96` | PARTIEL | Voir Layout (§9) ; **P3** |
| Texte « Résultats pour X » (`messageListSearchDesc`, 34601 à 34606) | Bandeau « Résultats de la recherche pour … » | Savoir qu'on est dans une recherche | champ conserve la valeur (`templates/home.html:23`) ; lien « Effacer la recherche » caché dans le menu d'options (`templates/home.html:49`) ; message vide « Aucun message… » (`:99`) | PARTIEL | Bandeau visible sous la barre : « Résultats pour « X » » + ✕ (même gabarit que `.filterbar` `templates/home.html:74`) ; **P2** |
| Pagination `messageListPagenator` (34607) | Pages numérotées avec ellipses | Pagination | `templates/home.html:104-106`, `pageLinks` (`mail.go:161`) | OK | rien |
| `checkAll` tri-état (34608 à 34619, 34628) | Case « tout » : cochée / indéterminée | Sélection | `static/list.js:37-45` (`checked`, `indeterminate`) | OK | rien |
| `inputProxyMessageListSearch` + `searchEnterAction` (34620 à 34627, 34878) | Recherche lancée à Entrée | Recherche | formulaire GET `templates/home.html:20-51` | OK | rien |
| `hasMessages`, `hasCheckedOrSelectedLines`, `canBeMoved` (34633 à 34638, 34682) | Boutons grisés sans sélection | Boutons actifs selon sélection | `data-needs-selection` + `refresh()` (`static/list.js:46`) | OK | rien |
| `isSpamFolder/isTrashFolder/isDraftFolder/isSentFolder/isArchiveFolder` (34639 à 34656) | Détecter le type du dossier courant | Boutons contextuels | `ClearKind` Corbeille/Indésirables seulement (`mail.go:536-542`) | PARTIEL | Exposer `IsSpam/IsArchive/IsDraft/IsSent` à la barre ; sert aux 3 lignes suivantes |
| `isArchiveVisible` (34660) : bouton Archiver masqué dans Archive et Brouillons | Pas d'archivage d'un archivé | Rendu des boutons | `templates/base.html:112` : bouton Archiver toujours là ; `mail.go:1361` `moveTo` sans effet si déjà dans le dossier | PARTIEL | Masquer le bouton dans Archive/Brouillons ; **P2** |
| `isSpamVisible`/`isUnSpamVisible` (34663 à 34668) + `notSpamCommand` (34771) : « Spam » masqué dans Spam/Envoyés/Brouillons ; dans Spam, bouton « Pas un spam » qui renvoie vers la boîte de réception | Annuler un classement en spam | Corriger un faux positif | `templates/base.html:113` : bouton « Spam » toujours là ; aucune action `notspam` (`mail.go:1256,1380` liste fermée read/unread/flag/unflag/delete/move/copy/archive/spam) | MANQUANT | Dans le dossier Indésirables : afficher « Pas un spam » (`action=notspam` → déplacer vers INBOX) à la place de « Spam » ; masquer « Spam » dans Envoyés/Brouillons ; **P2** |
| `clearListIsVisible` (35112) : « Vider » seulement en Spam/Corbeille, hors recherche, hors fil, sans erreur | Bouton de vidage | Vider un dossier | `templates/home.html:54-67` (visible dès que `ClearKind`/`ClearCount`, même en recherche ou dans un fil) ; confirmation en menu | PARTIEL | Masquer pendant une recherche ou l'ouverture d'un fil (`if and .ClearKind .ClearCount (not .Query) (not .ThreadOpen)`) ; **P3** |
| `clearCommand` + popup `FolderClear` (34735, 39264 à 39345) | Confirmer et vider ; message de danger | Vider Corbeille/Spam | `mailClear` (`mail.go:1213-1249`), refuse INBOX et tout autre dossier que Corbeille/Indésirables ; confirmation « Supprimer définitivement les N messages » (`i18n fr.json:224`) | OK | rien (confirmation en ligne au lieu de fenêtre) |
| `reloadCommand` + Ctrl+R (34741, 35199) | Vérifier le courrier | Rafraîchir | bouton rafraîchir `templates/base.html:101` ; Ctrl+R natif du navigateur recharge la page | OK | rien |
| `multyForwardCommand` + Maj+F (34747, 35273) : transférer les messages cochés en pièces jointes | Transférer plusieurs messages | Transfert groupé | `static/keys.js:49-50` : Maj+F ouvre le transfert en pièce jointe du **seul** message ouvert (`compose.go:351-359`) ; aucun bouton pour la sélection | MANQUANT | Accepter plusieurs `uid` dans `/compose?mode=forwardatt` et ajouter l'entrée au menu « plus » de la liste ; **P3** |
| `deleteWithoutMoveCommand` : Maj+Suppr, supprime sans passer par la corbeille (34753, 35180) | Suppression définitive directe | Supprimer pour de bon | aucune (seule la corbeille ou l'expunge depuis la corbeille, `mail.go:1384-1392`) | MANQUANT | Action `delete_now` avec confirmation `data-confirm` ; Maj+Suppr dans `keys.js:47` ; **P3** (action dangereuse) |
| `deleteCommand` / `archiveCommand` / `spamCommand` (34759 à 34769) | Boutons corbeille, archive, spam | Actions groupées | `templates/base.html:112-114` | OK | rien |
| Compteur « (N) » sur le bouton supprimer (`printableMessageCountForDeletion`, 34887) | Montrer combien seront supprimés (99+) | Prévenir | aucun | MANQUANT | Afficher `(n)` dans l'infobulle/bouton via `list.js refresh()` ; **P3** |
| `moveNewCommand`, `moveCommand`, menus « déplacer »/« copier » (34775 à 34796) | Menu de dossiers cible | Déplacer/copier | `templates/base.html:103-111` (menus déroulants avec liste des dossiers) | OK | rien |
| `hideLeft/showLeft` (34798 à 34808) | Masquer le volet gauche (mobile) | Volet dossiers | bouton `data-folders-toggle` (`templates/base.html:216`, `folders.js:42-51`) | OK | rien |
| `goToUpUpOrDownDown` : ↓ au bout de la page passe à la suivante (34816 à 34864) | Navigation continue entre pages | Clavier | `static/listnav.js:44,47` (`pageLink`) | OK | rien |
| `useAutoSelect` : pas d'ouverture automatique en filtre non lus (34866 à 34876) | Évite que la ligne disparaisse | Confort | `listnav.js` ouvre après 200 ms dans toute liste ; la ligne reste (pas retirée de la liste après lecture) | OK | rien |
| `cancelSearch` / `cancelThreadUid` + Échap (34892, 34897, 35290 à 35301) | Annuler la recherche ou quitter un fil au clavier | Clavier | `keys.js:37-40` : Échap ferme le message ; pas d'annulation de recherche ni de fil | MANQUANT | Échap : si `q` non vide → suivre le lien « Effacer la recherche » ; sinon si fil ouvert → `.threadbar a` ; **P2** |
| `moveSelectedMessagesToFolder` (34907) | Déplacer la sélection | Déplacer | `templates/base.html:105` | OK | rien |
| `setAction` / `setActionForAll` (34943 à 35029) : tout marquer lu, limité au fil si ouvert | Marquer tout lu | Tout lire | `markAllRead` (`mail.go:1412-1427`) : tout le dossier ; bouton `templates/base.html:123` | PARTIEL | Dans un fil ouvert, ne marquer que les messages du fil ; **P3** |
| `listSetSeen/UnsetSeen/SetFlags/UnsetFlags` (35031 à 35049) | Menu « plus » : lu, non lu, suivi, non suivi | Marquer | `templates/base.html:119-122` | OK | rien |
| `flagMessages` (35051) : étoile d'une ligne cochée = agit sur toute la sélection | Étoile en lot | Suivi en lot | `templates/home.html:87` : étoile = formulaire d'un seul `uid` (`mail.go:1264-1268`) | PARTIEL | Si la ligne est cochée, appliquer à toutes les cochées ; **P3** |
| `flagMessagesFast` touche I (35067) : **bascule** (si tous suivis → retirer, sinon suivre) | Raccourci étoile | Clavier | `static/keys.js:62` clique toujours « suivre » (`action=flag`) ; jamais « ne plus suivre » | PARTIEL | Choisir `flag` ou `unflag` selon l'état des lignes cochées/ouverte (`.star.on`) ; **P2** |
| `seenMessagesFast` touches Q/U (35083, 35259, 35265) | Lu / non lu | Clavier | `keys.js:60-61` | OK | rien |
| `gotoPage`, `gotoThread` (35099, 35105) | Page ; ouvrir la conversation | Pages, conversations | `templates/home.html:93,105`, `ThreadURL` | OK | rien |
| `gotoThread` par double-clic sur la ligne et clic sur la pastille (35142 à 35147) | Double-clic ouvre la conversation | Conversations | pastille cliquable seulement (`templates/home.html:93`) | PARTIEL | Écouteur `dblclick` sur `.msgrow` avec `.threads` ; **P3** |
| `onBuild` clics : clic sur la liste rend le focus à la liste ; case ; drapeau (35130 à 35148) | Interactions de ligne | Sélection | `static/list.js:81-92` (Maj+clic plage), `templates/home.html:87,89` | OK | rien |
| `initUploaderForAppend` : glisser un `.eml` sur la liste, zone de dépôt (35357 à 35400) | Importer un message par DnD | Ajouter un message | `static/emldrop.js:5-37` (zone `.eml-drop`, POST `/mail/append`) | OK | rien |
| `prefetchNextTick` préchargement au repos (35317) | Précharger les messages | Rapidité | aucun | SANS OBJET | Optimisation |
| `quotaTooltip` (35349) : quota en infobulle | Quota boîte | Quota | `templates/base.html:220` : barre + « utilisé/pourcentage/limite » | OK | rien |
| Raccourci Entrée en liste : plein écran si message ouvert (35163 à 35170) | Plein écran au clavier | Lecture plein écran | `listnav.js:54` : Entrée ouvre la ligne ; plein écran seulement au bouton (`msgfull.js:9-12`, le commentaire d'en-tête annonce Entrée mais aucun code ne la gère) | PARTIEL | Entrée avec message déjà ouvert = bascule `.msg-fullscreen` ; **P3** |
| Raccourci `z` archiver, `w`/`c` écrire, `/` recherche, `m` déplacer, `t` conversation, Ctrl+A (35174 à 35287) | Raccourcis | Clavier | `keys.js:43-65`, `list.js:98-109`, `listnav.js` | OK | rien |
| Raccourcis Tab/Maj+Tab/←/→ : bascule entre liste, dossiers, message (35304 à 35312) | Changer le volet actif | Clavier | aucun | MANQUANT | Voir « clavier des dossiers » ; **P3** |
| Raccourci Suppr/Maj+Suppr/Maj+3 dans la liste (35180) | Supprimer | Clavier | `keys.js:47` (`Suppr`, `#`) | PARTIEL | Maj+Suppr absent (voir ligne dédiée) |
| `popupVisibility`, `mobileChecked*`, `newMoveToFolder` (34558, 34669 à 34678) | États mobiles | Mobile | aucun | SANS OBJET | Mobile |
| `advancedSearchClick` (35343) | Ouvre la recherche avancée | Recherche avancée | menu `details.adv` (`templates/home.html:24-50`) | OK | rien |

## 7. Lecture d'un message (35510 à 36520)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `replyCommand/replyAll/forward/forwardAsAttachment/editAsNew` (35676 à 35680) | Boutons de réponse | Répondre | `templates/base.html:267-283` (bouton principal + menu) | OK | rien |
| `canBeRepliedOrForwarded` : pas en Brouillons (35670) | Dans Brouillons : modifier au lieu de répondre | Brouillons | `templates/base.html:268` (`IsDraft` → « Modifier le brouillon ») | OK | rien |
| `lastReplyAction` mémorisée (35641 à 35653) | Le bouton principal répète la dernière action | Confort | `LastReply` (`userdata.go:61`, `compose.go:1072`) | OK | rien |
| `deleteCommand/archive/spam/notSpam` depuis le message (35681 à 35685) | Boutons barre d'outils appliqués au message ouvert | Actions | mêmes boutons `form=bulk` avec `current_uid` (`mail.go:1273-1276`) | OK | rien (« Pas un spam » : voir §6) |
| `viewFromDkim*` : icône verte/rouge + infobulle (35712 à 35739) | Statut DKIM | Authenticité | `templates/base.html:246,264` (✔/⚠ + titre), `mail.go` | OK | rien |
| Bascule « infos complètes » mémorisée (`showFullInfo`, 35655, 36058 à 36062, Ctrl+I 36212) | Déplier les en-têtes complets, état conservé | En-têtes | `<details class="msg-info">` fermé à chaque message (`templates/base.html:237-248`) ; pas de mémoire ni de Ctrl+I | PARTIEL | Mémoriser l'état (`open` via réglage `POST`) et raccourci Ctrl+I ; **P3** |
| Date/taille/Cc/Cci/Répondre à/lien de désabonnement (35695 à 35707, 35791) | En-têtes affichés | Lecture | `templates/base.html:239-246,265,288` | OK | rien |
| Photo de l'expéditeur Gravatar (`viewUserPic`, 35708, 35799 à 35812) | Avatar | Reconnaître l'expéditeur | `.avatar` générique icône `user` (`templates/base.html:262`) | SANS OBJET | Gravatar refusé (CAHIER 14.1) |
| `detectDomBackgroundColor` / `bodyBackgroundColor` (35873 à 35918) | Peindre la zone du message avec la couleur de fond du courrier HTML (utile sur thème sombre) | Lisibilité des courriels HTML | `mail.go:998-1004` : corps toujours `background:#fff;color:#000` dans l'iframe | PARTIEL | Pas de détection ; le corps reste une carte blanche. Si le fond du volet est sombre, bordure autour de l'iframe ; **P3** (décision de rendu à valider avec DarkShine) |
| `fullScreen/unFullScreen/toggleFullScreen`, classe `rl-message-fullscreen` (35920 à 35934, 35832) | Lecture plein écran | Plein écran | `static/msgfull.js:1-19` (`msg-fullscreen`), bouton `templates/base.html:258` | OK | rien |
| `checkHeaderHeight` (35947) | Ajuste la hauteur du contenu sous l'en-tête | Mise en page | iframe en flux CSS | SANS OBJET | Détail d'implémentation |
| `attachmentPreview` + lightGallery (35983 à 36030) | Visionneuse d'images, flèches, miniatures | Pièces jointes images | `static/preview.js:1-47` (dialogue, ‹ ›, légende n/N, Échap) | OK | rien (miniatures de bas de visionneuse absentes : **P3**) |
| Lecture audio `showPreplay` MP3/OGG/WAV (36082 à 36105) | Lire un son sans quitter la page | Audio | aperçu dans un nouvel onglet (`mail.go:1067`, `templates/base.html:303`) | PARTIEL | Lecteur `<audio>` en ligne dans la visionneuse ; **P3** |
| Clic sur lien : `mailto:` ouvre la composition (36073 à 36077) | Liens du message | Écrire à une adresse | `mailview/sanitize.go:84,171` autorise `mailto:` ; iframe sandbox `allow-popups` (`templates/base.html:312`) ; `static/mailto.js` enregistre le gestionnaire du navigateur | PARTIEL | Dans l'iframe, un `mailto:` ouvre le client système du navigateur, pas la composition wmail-go sauf enregistrement du gestionnaire (bouton réglages `templates/settings.html:53`) ; **P3** |
| Clic sur une pièce jointe : téléchargement (36106 à 36112) | Téléchargement | Pièces | liens `a.att` (`templates/base.html:300`) | OK | rien |
| Étoile de l'en-tête et de la liste des fils (36113 à 36129) | Suivre depuis l'en-tête | Étoile | `templates/base.html:249-256` | OK | rien |
| Contrôles de pièces jointes : cases, « tout en zip », OwnCloud, Dropbox (35569 à 35639, 36379 à 36474) | Choisir des pièces et les télécharger en zip ou les envoyer sur un cloud | Zip sélectif | `ZipURL` zip de **toutes** les pièces (`templates/base.html:304-305`, `mail.go:1145`) ; pas de cases | PARTIEL | Cases de sélection + zip filtré ; OwnCloud/Dropbox SANS OBJET ; **P3** |
| `showImages` (36482) | Afficher les images distantes de ce message | Images distantes | `templates/base.html:297` (lien « Afficher les images », `ShowRemoteURL`) + « toujours pour cet expéditeur » | OK | rien |
| `readReceipt` (36504) : envoyer l'accusé de lecture | Accusé de réception | MDN | `templates/base.html:289-294` (envoyer/ignorer), `compose/mdn.go` | OK | rien |
| `printableCheckedMessageCount` (36494) | Pastille de nombre sur les boutons | Retour visuel | aucun | MANQUANT | Voir « compteur sur supprimer » ; **P3** |
| Raccourcis : `r`, `a`, `f` (36182 à 36209) | Répondre, à tous, transférer | Clavier | `static/keys.js:56-58` | OK | rien |
| Raccourci `b` : replier/déplier les citations (36220) | Citations repliables | Lecture | citations repliées en `<details class="wm-quote">` (`mail.go:1003`, CAHIER 14.3) ; pas de touche | PARTIEL | Touche `b` : aucune action possible dans l'iframe sandbox sans script ; **P3** (ignorable) |
| Raccourcis Ctrl+↑↓/←→ : message précédent/suivant (36231 à 36240) | Parcourir | Clavier | ↑↓ de `listnav.js:43-52` (équivalent, ouvre au bout de 200 ms) | OK | rien |
| Raccourci Ctrl+P (36242) : imprimer le message | Impression | Imprimer | lien « Imprimer » (`templates/base.html:279`, `static/print.js`, `print.css`) ; Ctrl+P natif imprime la page | PARTIEL | Intercepter Ctrl+P pour ouvrir `PrintURL` ; **P3** |
| Raccourcis Suppr/Maj+Suppr (36250) | Supprimer le message ouvert | Clavier | `keys.js:47` (corbeille) | PARTIEL | Maj+Suppr : voir §6 |
| Raccourcis Échap/Retour arrière : fermer, sortir du plein écran (36150 à 36176) | Fermer | Clavier | `keys.js:37-40` (Échap) + `msgfull.js:14-16` | PARTIEL | Ajouter la touche Retour arrière ; **P3** |
| Raccourci Entrée (36176) : plein écran dans le message | Plein écran | Clavier | aucun | MANQUANT | Voir ligne Entrée §6 ; **P3** |
| Tab/Maj+Tab/← : retour au focus liste (36264 à 36280) | Changer de volet | Clavier | aucun | MANQUANT | Voir §6 ; **P3** |
| Détection `isDraftFolder/isSentFolder/isSpamFolder/isArchiveFolder…` (36287 à 36337) | Boutons contextuels du message | Rendu | `IsDraft` seulement (`templates/base.html:268`) | PARTIEL | Voir « Spam/Archive visibles » §6 |
| `editMessage` brouillon (36345) | Rouvrir un brouillon | Brouillons | `ComposeURL "draft"` (`templates/base.html:268,271`) | OK | rien |
| `scrollMessageToTop/Left` (36351 à 36370) | Retour en haut à l'ouverture | Confort | `readpane.scrollTop = 0` (`static/list.js:198`) | OK | rien |
| `closeMessageCommand`, `messageVisibilityCommand` (35854 à 35858) | Fermer le message | Fermer | lien `a.mh-close` (`templates/base.html:259`) | OK | rien |
| Ancres de fils `thread-list` + `threadsDropdownTrigger` (36120 à 36129) | Liste des messages d'une conversation dans la vue | Conversations | conversation ouverte via la liste (`templates/home.html:71,93`) | PARTIEL | Pas de liste des messages du fil dans l'en-tête ; **P3** |
| `logoImg/logoIframe`, `dropboxEnabled`, `useThreads`, `layout`, `usePreviewPane` (35545 à 35558) | Options d'affichage admin | — | aucun | SANS OBJET | Admin / cloud |

## 8. Écran et pages de réglages (36586 à 38426)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `SettingsUserScreen.setupSettings` ordre du menu : Général, Contacts, Comptes, Filtres, Sécurité, [Social], Mot de passe, Modèles, Dossiers, Thèmes, OpenPGP (36620 à 36660) | Menu de gauche des réglages | Navigation réglages | `templates/base.html:77-83` : Général, Comptes, Dossiers, Filtres, Modèles, Contacts, Sécurité (ordre différent ; mot de passe dans Sécurité ; thème dans Général ; pas de page Thèmes) | PARTIEL | Réordonner : Général, Contacts, Comptes, Filtres, Sécurité, Modèles, Dossiers ; **P3** |
| Titre de page « adresse – Réglages » (36681 à 36684) | Titre d'onglet | Repère | `<title>` statique (`templates/base.html:7`) | PARTIEL | Ajouter l'adresse ; **P3** (lié au titre §4) |
| `MenuSettingsUserView` : ↑↓ dans le menu, lien « retour à la boîte » (38334 à 38343) | Navigation clavier et retour | Réglages | lien `.btn-back` (`templates/base.html:87`) ; pas de ↑↓ | PARTIEL | Clavier du menu : **P3** |
| `PaneSettingsUserView` : retour, `message(null)` (38398 à 38424) | Retour à la boîte | Navigation | `templates/base.html:87-89` | OK | rien |
| `GeneralUserSettings` : langue (36742, 36845 à 36853) | Changer de langue | Langue | `templates/settings.html:9-10` (envoi par bouton, pas à la volée) | OK | rien |
| `messagesPerPage` (36744, 36857) : liste fixe de valeurs | Messages par page | Taille de page | champ nombre 10 à 500 (`templates/settings.html:24-25`) + choix rapides dans le menu « plus » (`templates/base.html:136-141`) | OK | rien |
| `editorDefaultType` : Html / Plain / Html forcé / Plain forcé (36783 à 36798) | Éditeur par défaut | Rédaction | `templates/settings.html:27-33` (mêmes 4 valeurs) | OK | rien |
| `layout` : Pas de volet / Vertical / Horizontal (36799 à 36811, 36894) | Disposition de la boîte | Aperçu latéral, bas, ou sans | `templates/settings.html:19-23` : « three » (latéral) ou « list » (sans) seulement | MANQUANT | Option « Aperçu en bas » (`.mailbox.bottom`, séparateur horizontal) ; **P3** (DarkShine utilise l'aperçu latéral) |
| Notifications : son, bureau, `isDesktopNotificationDenied`, bouton « tester le son » (36749 à 36753, 36824, 36863 à 36877) | Régler et tester les notifications | Alertes de nouveaux messages | cases `templates/settings.html:39-40` ; demande d'autorisation `static/notify-setting.js:6-13` ; son généré `static/poll.js:15-24` | PARTIEL | Pas de bouton « Tester le son » ni message « refusé par le navigateur » ; **P3** |
| `showImages` (36754, 36859) : toujours afficher les images | Images distantes | Confidentialité | réglage par boîte `accounts.form.block_remote` (`templates/account_form.html`, `remoteimages.go:31-44`) + liste d'expéditeurs fiables (`templates/settings.html:51-52`) | OK | rien (l'emplacement du réglage diffère : dans le compte mail, pas dans Général) |
| `useCheckboxesInList` (36755, 36861) | Cases dans la liste | Affichage | `templates/settings.html:35` (`hide_checkboxes`, classe `hide-checks` `templates/base.html:14`) | OK | rien |
| `useThreads` + `threadsAllowed` (36756 à 36757, 36887) | Conversations | Regrouper | `templates/settings.html:37` | OK | rien |
| `replySameFolder` (36758, 36879) | Réponse classée dans le dossier d'origine | Envoyés | `templates/settings.html:41`, `ReplySameFolder` | OK | rien |
| Identité principale : description + bouton modifier (36770 à 36782, 36816) | Aperçu et accès à l'identité principale | Identité | `templates/identities.html:5-6` (liée au compte) | PARTIEL | Pas de rappel dans Général ; **P3** |
| Enregistrement automatique avec icône de confirmation par réglage (`saveSettingsHelper`, triggers, 36832 à 36899) | Chaque changement est enregistré tout de suite | Retour immédiat | un bouton « Enregistrer » pour tout le formulaire (`templates/settings.html:42`) | PARTIEL | Aucun besoin fonctionnel ; **P3** |
| `animations`, thème, taille de police, fond (`isAnimationSupported`, 36769) | Options d'affichage | Affichage | `templates/settings.html:11-18,36,44-50` | OK | rien |
| `ContactsUserSettings` : enregistrement auto, synchronisation CardDAV (36948 à 36973) | Réglages de contacts | Carnet | `templates/settings.html:38` (ajout auto) ; CardDAV (`internal/web/contactssync.go`, CAHIER 14.1) | OK | rien |
| `AccountsUserSettings` : liste, ajout, édition, suppression avec confirmation (37038 à 37084) | Gérer des comptes | Comptes | `templates/accounts.html:6-26` | OK | rien |
| Tri par glisser-déposer des comptes et identités (`scrollableOptions`, `accountsAndIdentitiesAfterMove`, 37030, 37106) | Ordre des comptes | Ordre | boutons ▲▼ (`templates/accounts.html:9`, `identities.html:10`) | OK | rien (équivalent fonctionnel ; rendu différent, **P3**) |
| Identités : ajout/édition/suppression (37048 à 37104) | Identités | Identités | `templates/identities.html` | OK | rien |
| `FiltersUserSettings` : liste, ajout, édition, suppression, ordre (37314 à 37343) | Filtres Sieve | Filtres | `templates/filters.html:14-33` (▲▼, activer/désactiver, éditer, supprimer) | OK | rien |
| Enregistrement global « Enregistrer » avec `haveChanges` (37224 à 37250) | Les filtres sont écrits ensemble | Sieve | chaque action est écrite immédiatement (`/settings/filters/action`) | OK | rien (plus sûr) |
| Édition du script brut + case « actif » (37198 à 37201, 37228) | Script Sieve manuel | Sieve avancé | `templates/filters_raw.html:7-12` (zone brute + « utiliser ce script ») | OK | rien |
| Erreur serveur `serverError` à l'affichage (37188, 37299 à 37301) | Message si ManageSieve injoignable | Erreurs | `templates/filters.html:6-10` (« non configuré », `ErrorKey`) | OK | rien |
| Filtre « modules » / capacités (37281, 37285) | Options selon le serveur | Sieve | aucun module affiché | SANS OBJET | Détail serveur ; l'éditeur adapte déjà les champs (`static/filters.js:15-24`) |
| `SecurityUserSettings` : déconnexion automatique 0 à 10 h (37399 à 37461) | Délai d'inactivité | Sécurité | `idle_minutes` et `absolute_hours` en nombres (`templates/settings.html:13-16`) | OK | rien (liste fixe de durées vs champ numérique : **P3**) |
| `configureTwoFactor` et popup `TwoFactorTest` (37449, 39602 à 39659) | Activer le 2FA, tester le code | 2FA | `/settings/2fa` : QR, secret, saisie du code de confirmation, codes de secours (`templates/twofactor.html:30-39,5-13`) | OK | rien (le test fait partie de l'activation) |
| `SocialUserSettings` Google/Facebook/Twitter (37503 à 37552) | Connexions tierces | SSO | aucun | SANS OBJET | Hors périmètre |
| `ChangePasswordUserSettings` : actuel + nouveau + confirmation, erreurs (37596 à 37680) | Changer le mot de passe | Mot de passe | `templates/settings.html:64-75` (+ longueur mini) ; échec « mot de passe actuel incorrect » (`fr.json:56`) | OK | rien |
| `TemplatesUserSettings` : liste, ajout, édition, suppression (37712 à 37777) | Modèles de message | Modèles | `templates/templates.html:7-29` | OK | rien |
| Corps du modèle dans un éditeur HTML (`editorSetBody`, 32033 à 32045) | Modèle riche | Modèles HTML | `<textarea>` texte brut (`templates/templates.html:24`) | PARTIEL | Texte seul ; les variables de date sont gérées. Éditeur riche **P3** |
| `FoldersUserSettings` : créer, dossiers système, renommer (Entrée/Échap), supprimer, abonnement, surveillance (37847 à 37935) | Gérer les dossiers | Dossiers | `templates/folders_settings.html` + `folders.go:137-221` | OK | rien |
| Suppression refusée si le dossier n'est pas vide (37893, 37911) | Garde-fou | Éviter la perte de messages | `DeleteFolder` envoie `DELETE` sans contrôle (`mailconn/messages.go:788-795`) ; confirmation « Supprimer » seulement | PARTIEL | Refuser (message d'erreur) quand `Total > 0` ou sous-dossiers ; **P2** |
| Textes d'aide au survol : supprimer / afficher-masquer / surveiller (37873 à 37880) | Aide contextuelle | Aide | `title` et paragraphe d'aide `folders.check_hint` (`templates/folders_settings.html:25`) | OK | rien |
| `ThemesUserSettings` : choix du thème avec aperçu (`themePreviewSrc`) (38036 à 38044) | Page « Thèmes » avec vignettes | Thème | `<select>` de noms (`templates/settings.html:11-12`) | PARTIEL | Pas de vignettes ; **P3** |
| Image de fond : téléverser, effacer, erreurs (taille/type) (38052 à 38118) | Fond personnel | Fond d'écran | `templates/settings.html:44-50`, `backgroundSave` (`server.go:169-171`), message `settings.background.invalid` (`fr.json:627`), CSS `handlers.go:526` | OK | rien (RainLoop fait un fondu `backstretch`, **P3** ignorable) |
| `OpenPgpUserSettings` : liste/ajout/génération/vue/suppression de clés, `allowDraftAutosave` (38165 à 38222) | Clés PGP ; **réglage d'enregistrement auto des brouillons placé là** | Brouillons auto | PGP SANS OBJET ; auto-brouillons : case `draft_autosave` (`templates/settings.html:34`) | OK | rien |

## 9. Popups de cette tranche (38571 à 40107)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `MessageOpenPgpPopupView` (38571 à 38705) | Choisir la clé privée et la phrase secrète pour déchiffrer | PGP | aucun | SANS OBJET | OpenPGP hors périmètre |
| `ComposeOpenPgpPopupView` (38833 à 39200, lu en partie) | Signer/chiffrer à l'écriture | PGP | aucun | SANS OBJET | idem |
| `FolderClearPopupView` : confirmation, vidage, erreur (39264 à 39345) | Vider le dossier | Corbeille/spam | voir §6 (`mail.go:1213`) | OK | rien |
| `AdvancedSearchPopupView` : champs De, À, Objet, Texte, période, pièce jointe, étoilé, non lu (39403 à 39440) | Recherche avancée | Recherche | `templates/home.html:38-47` (mêmes champs, mêmes périodes 3 j, 1 sem, 1 mois, 3 m, 6 m, 1 an) ; `advancedTerms` (`mail.go:248`) | OK | rien |
| `parseSearchStringValue` : préremplir la fenêtre depuis la recherche en cours (39456 à 39479) | Ré-ouvrir la recherche avancée avec les valeurs actuelles | Modifier une recherche | champs avancés vides à chaque ouverture (`templates/home.html:39-46`) ; la requête reste dans le champ simple | PARTIEL | Re-remplir `adv_*` en lisant les paramètres de l'URL ; **P3** |
| `buildSearchString` : `from:` `to:` `subject:` `has:attachment` `is:unseen,flagged` `date:` `text:` (39490 à 39540) | Syntaxe de recherche | Recherche | `advancedTerms` + syntaxe `from: to: subject: has:attachment is: date:` (CAHIER 14.2, `mail.go:248-281`) | OK | rien |
| `TwoFactorTestPopupView` (39602 à 39659) | Tester un code TOTP | 2FA | confirmation lors de l'activation (`templates/twofactor.html:35-39`) | OK | rien |
| `AddOpenPgpKeyPopupView`, `NewOpenPgpKeyPopupView`, `ViewOpenPgpKeyPopupView` (39709 à 40070, parcourus seulement par signatures) | Importer/générer/voir une clé PGP | PGP | aucun | SANS OBJET | OpenPGP hors périmètre |
| `WelcomePagePopupView` (40066 à 40107) | Page d'accueil facultative de l'admin | Message d'accueil | aucun | SANS OBJET | Pas d'administrateur (CAHIER 14.4) |

## À AJOUTER (MANQUANT / PARTIEL), par priorité

### P1

1. **Règle des badges de dossiers (Brouillons = total, Corbeille/Archive/Envoyés = rien, autres = non lus).** Fichiers : `internal/web/templates/base.html:204` et `:211`, `internal/web/static/poll.js:55-62`, `internal/web/foldertree.go` (champ à ajouter à `FolderRow`). Ajouter `Badge int` calculé dans `folderPanel` : si rôle = brouillons → `Total` ; si rôle ∈ {corbeille, archive, envoyés} → 0 ; sinon → `Unseen`. Afficher `<span class="badge" data-unread-badge>{{.Badge}}</span>` si `Badge > 0`. Le sondage (`poll.js`) met à jour les compteurs `Unseen` par dossier : ne pas afficher de badge pour les rôles exclus (ajouter `data-nobadge` sur la ligne), pour Brouillons ne pas toucher au total. Le gras « non lu » (`has-unread`) doit suivre `Unseen` uniquement hors rôles exclus. Rendu RainLoop : même pastille grise `.badge` (déjà en CSS `static/mail.css:84`).

### P2

2. **Bouton « Pas un spam » et visibilité contextuelle des boutons Archiver/Spam.** Fichiers : `templates/base.html:112-113`, `mail.go:1256,1361-1366,1380`. Exposer `IsJunk`, `IsArchive`, `IsDraftOrSent` dans `Page`. Masquer « Archiver » dans Archive et Brouillons ; masquer « Spam » dans Indésirables, Envoyés, Brouillons ; dans Indésirables afficher « Pas un spam » (`action=notspam`). Serveur : accepter `notspam` dans les listes de `mailAction` (`mail.go:1256` et `:1380`) et déplacer vers INBOX avec `c.Move(folder, uids, "INBOX")`. Libellé fr : « Pas un spam », en : « Not spam ».
3. **Bandeau « Résultats de la recherche pour « X » » avec ✕ et Échap qui annule.** Fichiers : `templates/home.html` (près de la ligne 74, même style que `.filterbar`), `static/keys.js:37-40`. Afficher le bandeau quand `.Query` est non vide avec lien `.ClearSearchURL` ; dans `keys.js`, sur Échap sans aide ni sélection cochée : si `.searchbar a.clear` existe → clic ; sinon si `.threadbar a` existe (fil ouvert) → clic ; sinon fermer le message. Clé i18n `mail.search.result`.
4. **Touche `i` en bascule (suivre/ne plus suivre).** Fichier : `static/keys.js:62`. Déterminer les lignes visées (cochées, sinon ligne ouverte/ciblée `.msgrow.selected`) ; si toutes ont `.star.on` → cliquer `button[form=bulk][value=unflag]`, sinon `value=flag`. Même logique côté message ouvert (`.msgview .star.on`).
5. **Garde-fous de suppression de dossier.** Fichiers : `templates/folders_settings.html:55-65`, `internal/web/folders.go:187-192`, `internal/mailconn/messages.go:788`. Ne pas proposer « Supprimer » pour un dossier à rôle système (`.Role != ""`) ni s'il a des sous-dossiers ; côté serveur refuser si `Total > 0` avec l'erreur `folders.err.not_empty` (« Le dossier n'est pas vide : déplacez ou supprimez d'abord ses messages ») et refuser les dossiers à rôle (`folders.err.system_delete`).

### P3

6. **Disposition « aperçu en bas »** (`settings.html:19-23`, `userdata.go:Layout` valeur `bottom`, CSS `.mailbox.bottom` avec séparateur horizontal et `--list-h` enregistré comme `--list-w`).
7. **Maj+Suppr : suppression définitive directe** (`mail.go:1256,1384`, `keys.js:47`, nouvelle action `delete_now` avec `data-confirm`).
8. **Transfert groupé en pièces jointes** des messages cochés (`compose.go:351`, accepter plusieurs `uid`, entrée dans le menu « plus » `templates/base.html:116`).
9. **Navigation clavier des volets** : ← depuis la liste = focus dossiers, ↑↓/Entrée/Espace dans les dossiers, Tab/Maj+Tab entre volets, Retour arrière ferme le message, Entrée avec message ouvert = plein écran (`static/folders.js`, `static/keys.js`, `static/msgfull.js`).
10. **Titre de l'onglet avec l'adresse** : `(N) adresse – titre` (`templates/base.html:7,18`, `poll.js:12`) y compris pour les réglages.
11. **Ordre du menu des réglages** : Général, Contacts, Comptes, Filtres, Sécurité, Modèles, Dossiers (`templates/base.html:77-83`).
12. **Mémorisation de « infos complètes » de l'en-tête et Ctrl+I** (`templates/base.html:237`, réglage serveur `HeaderInfo`).
13. **Case « Rester connecté » à la connexion** (`templates/login.html`, appliquer `AbsoluteHoursMax` ; mémoire du dernier choix) et focus mot de passe si identifiant prérempli.
14. **Pastille « N » à la place de l'image de glisser** (`static/dnd.js:30-33` `setDragImage`) et compteur « (n) » sur le bouton supprimer (`static/list.js refresh()`).
15. **Cases de sélection des pièces jointes + zip filtré, lecteur audio en ligne** (`templates/base.html:298-306`, `mail.go:1145`).

Autres P3 relevés : étoile d'une ligne cochée agit sur la sélection (`templates/home.html:87`), double-clic ouvre la conversation, « tout lu » limité au fil ouvert, masquer « Vider » pendant une recherche ou un fil (`templates/home.html:54`), bouton « Tester le son », vignettes de thèmes, pré-remplissage de la recherche avancée, éditeur riche pour les modèles, animation d'erreur du formulaire de connexion, détection de la couleur de fond du courrier HTML, redirection du sondage vers `/login` à l'expiration, protection d'abonnement des dossiers système.
