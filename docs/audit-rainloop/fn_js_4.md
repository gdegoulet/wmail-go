# Analyse exhaustive RainLoop 1.17 front-end : static/js/app.js lignes 24000 à 32000

Tranche lue intégralement, sauf les polyfills core-js (lignes 26145 à 27577, vérifiés par leurs en-têtes de modules : `core-js/internals/*`, `core-js/modules/*`, `es/instance/*`, etc.), classés SANS OBJET.
Modules contenus (ids webpack) : 316 FolderSystem (fin), 317 AbstractBasic, 318 AbstractSystemDropDown, 319 Selector, 320 MessageModel, 321 js-cookie, 325-391 core-js, 392 Account, 393 Identity, 394-401 composants Knockout, 402 AppUser, 405 Base64, 406 PromisesUserPopulator, 407 FilterModel, 408 FilterConditionModel, 409 (qr externe), 426 KeyboardShortcutsHelp, 427 Contacts, 428 FolderCreate, 429 Filter, 430 TwoFactorConfiguration, 431 Template (début seulement : la tranche s'arrête ligne 32000, au milieu de `addTemplateCommand` ; la suite est dans la tranche suivante).
Fichiers wmail-go réellement lus : `internal/web/static/{keys,listnav,list,menus,poll,filters,accounts,mailto,dnd}.js`, `templates/{home,identities,account_form,filters,filter_edit,folders_settings,contacts,twofactor,templates}.html`, extraits de `templates/base.html`, `mail.go` (actions 1249-1400), `contactsweb.go`, `static/mail.css` (400-492), `static/themes/rainloop/theme.css` (en-tête), `fr.json` (clés keys.* / mail.sel.*), `/…/scratchpad/ds/out.css` (DarkShine compilé).
Limites : je n'ai pas relu les templates HTML RainLoop (autre tranche) ni son CSS de liste ; les rendus visuels de RainLoop cités ci-dessous viennent donc uniquement des noms de classes CSS posées par ce JS (`lineAsCss`) et de `out.css` (qui ne contient que la couche couleur DarkShine). Là où je ne peux pas confirmer le rendu, je le dis.

Légende : OK / PARTIEL / MANQUANT / SANS OBJET. Priorité P1 (quotidien, très visible), P2, P3.

## 1. FolderSystemPopupView (24000-24120) : popup « dossiers système à configurer »

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Constructeur : textes « choisir un dossier » / « ne pas utiliser » (24026-24033), `folderSelectList` (24037-24040) | Listes déroulantes Sent/Drafts/Spam/Trash/Archive avec options « Choisir… » et « Ne pas utiliser » | Choisir le dossier de chaque rôle | `templates/folders_settings.html:11-17` (select par rôle, options « Automatique (résolu) », « Inutilisé », liste des dossiers) ; `account_form.html` idem ; `accounts.go:503 accountFolderChoices` | OK | Rendu différent (page de réglages au lieu d'une popup) mais fonction équivalente et plus riche (valeur auto affichée) |
| Abonnements `sentFolder…archiveFolder.subscribe` + `fSaveSystemFolders` debounce 1 s (24047-24074) | Sauvegarde automatique à chaque changement de select | Enregistrer sans bouton | `folders_settings.html:19` : bouton « Enregistrer » explicite (POST `/settings/folders/system`) | PARTIEL | Écart d'UX uniquement (pas d'enregistrement auto) ; P3, ne pas faire |
| `onShow(notificationType)` (24085-24116) : message « le dossier Envoyés/Brouillons/Spam/Corbeille/Archive n'est pas défini » | Quand l'utilisateur supprime/archive/marque spam sans dossier configuré, une popup l'invite à en choisir un | Gérer l'absence de dossier système | `mail.go:1344-1371` : `moveTo` appelle `c.EnsureFolder(target)` (crée le dossier s'il manque) et `mailconn.ResolveTrash/ResolveArchive/ResolveJunk` | OK | Comportement différent mais sans blocage : aucun besoin de popup |

## 2. AbstractBasicPromises (24123-24176)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent | Verdict | Proposition |
|---|---|---|---|---|---|
| `func`, `fastResolve`, `fastReject`, `setTrigger` (24151-24173) | Utilitaires de promesses pour les appels AJAX (indicateurs de chargement) | Plomberie du client JSON/AJAX | aucun (wmail-go est rendu serveur, formulaires POST) | SANS OBJET | Architecture différente |

## 3. AbstractSystemDropDownUserView (24179-24322) : menu du compte (en haut à droite)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `logoImg`, `logoTitle` (24232-24233) | Logo utilisateur personnalisé (réglage admin) | Branding | aucun | SANS OBJET | Admin / branding multi-utilisateur |
| `mobile`, `mobileDevice`, `allowSettings`, `allowHelp`, `capaAdditionalAccounts` (24234-24243) | Drapeaux de capacités | Masquer des entrées | aucun | SANS OBJET | Capacités admin ; mono-utilisateur |
| `currentAudio` + événements `audio.stop/start` + `stopPlay` (24238, 24245-24258) | Entrée « Arrêter la lecture » dans le menu quand une pièce jointe audio joue | Couper un son lancé depuis un message | `mail.go:1067` sert l'audio, lecture native `<audio>` ; pas d'entrée de menu | PARTIEL | Aucun contrôle global. P3 : inutile en pratique (le lecteur natif a son bouton pause) |
| `accounts` / `accountEmail` : liste des comptes dans le menu (24239-24240) | Menu déroulant listant les comptes, compte courant marqué | Changer de compte | `base.html:148-150` : bouton par compte avec ✓ sur le compte actif (POST `/accounts/select`) | OK | Rendu : coche texte au lieu de la mise en forme RainLoop (non vérifiée) |
| `accountsUnreadCount` (24241) | Compteur de non lus de l'autre compte | Voir le courrier des autres comptes | aucun ; côté RainLoop `accountsCounts()` est lui-même commenté (28848-28875) | SANS OBJET | Fonction morte dans RainLoop 1.17 |
| `accountClick` (24260-24270) | Au clic gauche : état « loading » du menu 1 s pendant le changement de compte | Retour visuel | aucun (le POST recharge la page) | SANS OBJET | Inutile avec rechargement complet |
| `emailTitle` (24272-24274) | `title` = e-mail du compte sur le bouton | Infobulle | `base.html:146` `title="{{.User}}"` | OK | |
| `settingsClick` (24276-24280) | Entrée « Paramètres » | Accès réglages | `base.html:154` `<a href="/settings">` | OK | |
| `settingsHelp` (24282-24286) | Entrée « Raccourcis clavier / Aide » ouvrant la popup d'aide | Aide | `base.html:156` bouton `data-keyhelp` → overlay `#keyhelp` (`keys.js:21-24`) | OK | |
| `addAccountClick` (24288-24292) | Entrée « Ajouter un compte » ouvrant la popup compte | Ajouter une boîte | `base.html:155` lien « Comptes » vers `/accounts` (page liste, puis « nouveau ») | PARTIEL | Il faut 2 clics au lieu d'1. Proposition : ajouter dans le menu un lien direct `/accounts/new` (« Ajouter un compte »). P3 |
| `logoutClick` (24294-24296) | Déconnexion | Quitter | `base.html:158` formulaire POST `/logout` | OK | |
| `onBuild` touche `` ` `` (24301-24307) | Ouvre le menu compte au clavier (listes, message, réglages) ; sort du plein écran | Raccourci | `keys.js:60` ``case "`"`` clique `.mb-right details.dropdown summary` | OK | Ne sort pas du mode plein écran message (`msg-fullscreen`) : détail P3 |
| `onBuild` touche `shift+/` (24309-24316) | Ouvre l'aide raccourcis ; renvoie false (consomme la touche) | Raccourci | `keys.js:44-45` touche `?` (`e.key`, donc indépendante du clavier) | OK | |

## 4. Selector (24325-25005) : moteur de sélection de listes (messages, contacts)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Constructeur : `list`, `listChecked`, `isListChecked` (24394-24404) | Liste observable des éléments, sous-liste cochée, booléen « au moins une cochée » | Sélection multiple | `list.js:33-51` `boxes()`, `refresh()` (compte les cases cochées, indeterminate sur « tout ») | OK | |
| `listChecked.subscribe` (24408-24420) : dès qu'une case est cochée, l'élément ouvert est désélectionné (le volet de lecture se vide) ; quand toutes sont décochées, l'élément focalisé est rouvert si auto-sélection | Cocher = passer en mode « action groupée », lecture fermée | Éviter l'ambiguïté lecture / sélection | aucun : `list.js` `refresh()` ne touche pas au volet de lecture ; le message reste ouvert à droite pendant qu'on coche | PARTIEL | Différence visible. Proposition : dans `refresh()`, si `n>0` vider `.readpane` (placeholder `mailbox.placeholder`) ; si `n==0` rouvrir `.focused`. P3 (le comportement actuel est défendable) |
| `selectedItem.subscribe` (24421-24435) : sélectionner un message décoche toutes les cases ; appel de `itemSelected` temporisé | Ouvrir un message annule la multi-sélection | Cohérence | `list.js` clic sur `a.rowlink` → `openPane` : les cases restent cochées | PARTIEL | Proposition : dans `openPane` après succès, décocher toutes les `input[name=uid]`. P3 |
| `itemSelectedThrottle` debounce 300 ms (24407) | Une touche ↑/↓ maintenue n'ouvre que le dernier message | Éviter les rafales | `listnav.js:24-30` timer 200 ms (« une touche maintenue n'ouvre que le dernier message ») | OK | |
| `toggleSubscribeProperty` `selected` / `focused` (24436-24441) | Pose les classes `selected` et `focused` sur la ligne | Rendu de la ligne | `listnav.js:19-23 setFocus` (`.focused`), `list.js:172-184 markSelected` (`.selected`), `mail.css:420` `.msgrow.focused{outline:1px dotted #888}` | PARTIEL | Le focus est un contour pointillé gris ; le rendu RainLoop de `.focused` dans la liste n'est pas dans cette tranche (non vérifié). `out.css` DarkShine ne contient que `.thm-folders .e-link.focused` (fond blanc 30 %). P3 |
| `sLastUid` mémorisé au focus (24446-24450) | Ancre pour la sélection par plage Maj+clic | Plage | `list.js:84-92` `lastIndex` (Maj+clic sur **case** uniquement) | OK | Voir ligne « actionClick » pour le clic sur la ligne |
| `list.subscribe beforeChange` (24455-24477) | Mémorise uids, cochés, focalisé, sélectionné avant un rechargement de liste | Conserver l'état | `list.js:124-170` sessionStorage de la position de défilement ; l'état des cases est perdu au rechargement de page (liste rendue serveur) | PARTIEL | Les cases cochées ne survivent pas à une navigation (page suivante, tri). Normal avec pages serveur ; P3 |
| `list.subscribe` après changement (24478-24586) : restaure focus/coché/sélection ; si rien n'est sélectionné et `autoSelect`, sélectionne le message qui **suivait** l'ancien focalisé (après suppression/déplacement) | Après avoir supprimé/archivé un message ouvert, le suivant s'ouvre tout seul (flux de tri rapide du courrier) | Enchaîner les messages | `mail.go:1384-1390` : après delete/move le message ouvert disparaît et on retourne à `back` (liste, volet vide) ; aucun grep `autoselect/next` ne trouve d'ouverture du voisin | MANQUANT | **P2** : après une action delete/archive/spam/move sur le message ouvert (champ `open`), rediriger vers le voisin (`NextURL`, sinon `PrevURL`, déjà calculés `mail.go:628-633`) au lieu de `back` |
| `iSelectNextHelper` / `iFocusedNextHelper` (24552-24579) | Quand ↓ au dernier / ↑ au premier change de page, le premier (dernier) élément de la nouvelle page est focalisé et sélectionné, avec scroll | Parcourir plusieurs pages au clavier | `listnav.js:46-52` clique le lien de page ; sur la nouvelle page rien n'est focalisé (↓ sélectionne la 1re ligne, ↑ sans focus ne remonte pas) | PARTIEL | Proposition : stocker en `sessionStorage` `wmail.focusEdge=first|last` avant `nx.click()`/`pv.click()` et, au chargement, focaliser + ouvrir la 1re / dernière ligne. P3 |
| `itemSelected` (24591-24599) | Appelle le callback d'ouverture (liste cochée : seulement la fermeture) | Ouvrir | `list.js:186-214` `openPane` | OK | |
| `goDown` / `goUp` (24605-24615) | Utilisés par les boutons « message suivant/précédent » du volet de lecture | Navigation entre messages | `mail.go:81, 628-633` `PrevURL/NextURL` fournis à la vue ; liens rendus dans la barre du message | OK | (liens fournis ; je n'ai pas relu le gabarit `msgview` pour confirmer leur emplacement) |
| `unselect` (24617-24620) | Désélectionner/défocaliser | Échap | `keys.js:34-36` Échap ferme le message ; `list.js:106-107` Échap décoche | OK | |
| `init` : `selectstart` bloqué (24633-24636) | Empêche la sélection de texte à la souris quand on Maj+clique | Confort | aucune règle `user-select:none` sur `.msgrow` (grep : seulement `body.dragging` et `.keybox`) | PARTIEL | Maj+clic sur une ligne sélectionne du texte. Proposition : `.msglist-form{user-select:none}` ; P3 |
| `init` : clic sur la ligne `sItemSelector` (24637-24639) → `actionClick` | Clic = ouvrir ; Maj = plage ; Ctrl = ajout | Interaction souris | `list.js:216-228` : clic gauche simple ouvre le volet ; Maj/Ctrl/Meta laissent le comportement du navigateur (lien ouvert en nouvel onglet/fenêtre) | PARTIEL | Voir `actionClick` ci-dessous |
| `init` : clic sur la case (24639-24651) : Maj = plage ; sinon focus + bascule | Cocher sans ouvrir | Cocher | `list.js:84-92` plage Maj sur case ; `focused` n'est pas déplacé au clic de case | PARTIEL | Proposition : au clic d'une case, appeler `setFocus(row)` (listnav) pour que les flèches repartent de là. P3 |
| `init` : touche `enter` (24652-24660) | Ouvre l'élément focalisé s'il n'est pas déjà ouvert | Clavier | `listnav.js:58-59` Entrée ouvre (`open(cur,true)`) | OK | |
| `init` : `ctrl+up/down` neutralisés (24661-24663) | Éviter le défilement natif | Confort | aucun | PARTIEL | P3, négligeable |
| `init` : `up, shift+up, down, shift+down, home, end, pageup, pagedown, insert, space` (24664-24713) | Navigation clavier de la liste | Clavier | `listnav.js:44-76` : ↑ ↓ Home End PgUp PgDn (pas de 10 lignes), Espace/Inser coche, Maj+↑/↓ coche en passant | OK | Même pas de page (10). Différence : Espace ne déplace pas le focus (idem RainLoop) |
| `autoSelect` / `doUpUpOrDownDown` (24721-24731) | Hook : page précédente/suivante quand on dépasse la liste | Pagination clavier | `listnav.js:46-52` `pageLink("next"/"prev")` | OK | |
| `getItemUid` (24738-24747) | Identifiant d'un élément | Plomberie | `id="m{{.UID}}"` `home.html` | OK | |
| `newSelectPosition` (24755-24859) : table de transitions focus/coché/sélection, PageUp/PageDown de 10, Maj+flèche coche l'élément **quitté** | Navigation et cochage au clavier | Clavier | `listnav.js:53-76` : Maj+↑/↓ coche l'élément courant puis le suivant | OK | Écart minime : RainLoop bascule (`!checked`), wmail force `true` ; P3 |
| `scrollToFocused` (24865-24886) : décalage 20 px, top si premier | Garder la ligne visible | Défilement | `listnav.js:22` `scrollIntoView({block:"nearest"})` | OK | |
| `scrollToTop` (24893-24911) animé 200 ms | Retour en haut après changement de page | Confort | aucun ; page rechargée (haut par défaut) | OK | |
| `eventClickFunction` : plage Maj entre `sLastUid` et l'élément (24913-24950) | Cocher une plage | Plage | `list.js:84-92` | OK | |
| `actionClick` (24957-24992) : Maj+clic = plage (sans ouvrir) ; Ctrl/Cmd+clic = ajoute à la sélection, le message ouvert est coché aussi ; clic = ouvre | Sélection multiple à la souris sans passer par la case | Souris | `list.js:216-228` n'intercepte pas Ctrl/Maj : un Ctrl+clic sur une ligne ouvre un nouvel onglet | PARTIEL | **P2** : dans le gestionnaire de clic de `list.js`, si `e.shiftKey` ou `e.ctrlKey/metaKey` sur `.rowlink` → `preventDefault()` et cocher/plage via `check(row)` au lieu de naviguer |
| `on`, `selectMessageItem` (24994-25002) | Callbacks / sélection + focus + scroll | Plomberie | `listnav.js open()+setFocus()` | OK | |

## 5. MessageModel (25010-25960)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Observables d'état `deleted`, `deletedMark`, `unseen`, `flagged`, `answered`, `forwarded`, `isReadReceipt`, `focused`, `selected`, `checked` (25117-25126) + `lineAsCss` (25508-25527) : classes `deleted`, `deleted-mark`, `selected`, `checked`, `flagged`, `unseen`, `answered`, `forwarded`, `focused`, `important`, `withAttachments`, `new`, `emptySubject`, `hasUnseenSubMessage`, `hasFlaggedSubMessage` | Chaque état change le rendu de la ligne (gras non lu, étoile, icône répondu/transféré, barre latérale, etc.) | Rendu de ligne | `home.html:75` ligne `class="msgrow{{if not .Seen}} unread…selected… answered}"` ; `checked` posé par `list.js:46-50` ; `flagged` bouton étoile `home.html:79` ; répondu/transféré icônes `home.html:84` ; `.imp` `home.html:88` ; `.focused` `listnav.js` ; sujet vide `home.html:88` `mail.no_subject` en italique | PARTIEL | Manquent : `deleted-mark` (message avec drapeau IMAP `\Deleted` laissé par un autre client : RainLoop le barre), `new` (animation d'arrivée d'un nouveau message), `hasUnseenSubMessage` / `hasFlaggedSubMessage` (voir ligne dédiée). Les classes `flagged` et `forwarded` n'existent pas sur le `<li>` (seul le bouton étoile porte l'état) : fond de ligne « favori » non reproduit si RainLoop l'a (non vérifié). P3 |
| `hasUnseenSubMessage` / `hasFlaggedSubMessage` (25146-25147, 25524-25525) + `threads` / `threadsLen` (25148-25151) | En mode conversation, la ligne racine est mise en évidence si un message de la conversation est non lu / favori | Voir l'état d'une conversation sans l'ouvrir | `home.html:86` pastille `.threads` avec le nombre ; aucun état non lu/favori de conversation (grep `hasUnseen|ThreadUnseen|hasFlagged` : aucun résultat) | MANQUANT | **P2** si le mode conversation est utilisé : calculer côté Go `ThreadUnseen`/`ThreadFlagged` (OR des drapeaux des membres, déjà listés dans `ThreadUIDs`) ; classes `.msgrow.sub-unseen .threads{background:var(--unread-bar)}` et étoile |
| `newForAnimation` (25116) | Surbrillance animée des messages arrivés pendant la session | Repérer les nouveaux | aucun | MANQUANT | P3 : non lié à un rafraîchissement automatique de la liste (voir AppUser) |
| `attachmentIconClass` / `staticCombinedIconClass` (25128-25131) | Icône de pièce jointe selon le type (image, archive, pdf, doc, audio…) | Icône dans la liste | `home.html:85` : classe `clip kind-{{.AttachKind}}`, icône `image` ou `doc` seulement | PARTIEL | P3 : ajouter icônes pdf/archive/audio/vidéo dans `AttachKind` (`mail.go`) |
| `isPgpSigned/Encrypted`, `pgpSignedVerifyStatus/User` (25136-25139, 25338-25341) | Badges OpenPGP | PGP | aucun | SANS OBJET | OpenPGP exclu |
| `priority` / `isImportant` (25100, 25152-25154, 25279-25280) | Message haute priorité : marque « ! » rouge | Priorité | `home.html:88` `<b class="imp">!</b>` ; `mail.css:464` `.imp{color:#c0392b}` | OK | |
| `getEmails` / `getRecipientsEmails` (25236-25251) | Adresses uniques To+Cc | Plomberie réponse | `compose/reply.go:37-64` | OK | |
| `friendlySize` (25258-25260) | Taille lisible | Affichage | `mail.go:858` `humanSize` | OK | |
| `computeSenderEmail` (25262-25267) : dans Envoyés/Brouillons la colonne expéditeur affiche le **destinataire** | Dans Envoyés on voit « À : x » au lieu de soi-même | Lisibilité Envoyés | `mail.go:583-584` `toColumn … who = m.To` | OK | |
| `initByJson` (25274-25318) : expéditeur, objet scindé préfixe/suffixe (`SubjectParts`), date, pièces jointes, threads | Remplit la ligne de liste ; objet = préfixe `Re:` grisé + reste | Rendu de l'objet | `home.html:88` `.pfx` + `SubjectRest` ; `mail.go splitSubject` ; `mail.css:464` `.pfx{color:#888}` | OK | |
| `initUpdateByMessageJson` (25325-25353) : DraftInfo, Message-Id, In-Reply-To, References, pièces jointes, accusé de lecture, CIDs | Complète le message à l'ouverture | Lecture / réponse | `mailview/parse.go` (Message-Id, ReplyReadReceipt), `compose/build.go:128` In-Reply-To ; `mail.go:888-892` accusé de lecture | OK | |
| `initAttachmentsFromJson` + `isLinked` via CID (25360-25381) ; `hasVisibleAttachments` (25533-25537) | Les images incorporées (cid) ne sont pas listées comme pièces jointes | Liste de PJ propre | `mailview/sanitize.go:124,185-186` cid → data URI ; (je n'ai pas lu le code qui masque ces PJ de la liste des PJ) | PARTIEL | À vérifier : s'assurer que les parties cid affichées en ligne ne sont pas aussi listées en PJ. P3 |
| `hasUnsubsribeLinks` / `getFirstUnsubsribeLink` (25387-25396) | Lien « Se désabonner » (List-Unsubscribe) | Désabonnement | `mail.go:879-886`, `base.html:288` barre « se désabonner » (https direct, sinon mailto pré-rempli) | OK | |
| `initFlagsByJson` (25404-25418) | Drapeaux IMAP | Rendu | `mailconn` drapeaux ; `home.html` | OK | |
| `fromToLine`, `toToLine`, `ccToLine`, `bccToLine`, `replyToToLine` (25426-25501) : formats « ami » (nom seul) et liens | En-têtes De/À/Cc/Cci/Répondre à | En-tête du message | `mail.go:857` `addrList(m.From/To/Cc/ReplyTo/Bcc)` ; `base.html:246,264` ; `<a class="sender" href="/compose?to=…">` | OK | |
| `fromDkimData` (25438-25446) | Statut DKIM de l'expéditeur (✔ / ⚠) | Confiance | `base.html:246,264` DKIM + `mailview/parse.go:138,255-261` `dkimStatus` ; `mail.css:469-471` | OK | |
| `findAttachmentByCid`, `findAttachmentByContentLocation` (25544-25574) | Résolution `cid:` et `Content-Location` | Images incorporées | `mailview/sanitize.go:185-186` (`NormalizeCID`) ; Content-Location : aucun résultat grep | PARTIEL | P3 : images référencées par `Content-Location` (rares) non résolues |
| `messageId`, `inReplyTo`, `references`, `fromAsSingleEmail` (25580-25606) | Accesseurs | Plomberie | `compose/build.go`, `mail.go` | OK | |
| `viewLink`, `downloadLink` (25612-25622) | Liens vue/téléchargement du .eml | Télécharger le message | `mail.go` : (je n'ai pas lu la route de téléchargement .eml) | PARTIEL | Non vérifié : confirmer l'existence d'« Enregistrer le message (.eml) ». P3 |
| `replyEmails` (25630-25648) | Destinataire de « Répondre » : Reply-To, sinon From, sinon repli sans exclusion | Répondre | `compose/reply.go:37-50` `ReplyRecipients` | OK | |
| `replyAllEmails` (25656-25680) | « Répondre à tous » : To = Reply-To/From + To ; Cc = Cc, en excluant ses propres adresses | Répondre à tous | `compose/reply.go:54-64` `ReplyAll` | OK | |
| `textBodyToString`, `attachmentsToStringLine` (25686-25700) | Texte pour la citation / impression | Citation | `compose/` (citation) | OK | |
| `viewPopupMessage` / `printMessage` (25706-25729) : fenêtre d'aperçu/impression avec titre, date `LLL`, De, À, Cc | Impression / ouvrir dans une fenêtre | Imprimer | `static/print.js`, `print.css`, `mail.go:1047` | OK | (mise en page d'impression non comparée) |
| `generateUid` (25736-25738) | `dossier/uid` | Plomberie | `home.html` `id="m{{.UID}}"` | OK | |
| `populateByMessageListItem` (25745-25799) | Pré-remplit le volet avec la ligne de liste avant le chargement complet | Affichage immédiat | `list.js:186-214` charge `/mail/pane` ; pas de pré-affichage | PARTIEL | P3 : afficher l'objet/expéditeur de la ligne dans le volet pendant le `fetch` |
| `showLazyExternalImagesInBody`, `lozad`, `showExternalImages(lazy)`, `showInternalImages(lazy)` (25801-25928) : « Afficher les images » ; chargement paresseux, animation d'opacité 0,3 → 1 en 500 ms | Images distantes bloquées puis affichées sur demande ; lazy loading | Confidentialité | `remoteimages.go`, `mail.go` option `remote`, `account_form.html:89-90` (`block_remote`, `proxy_images`) | OK | Lazy loading : SANS OBJET (performance, pas de fonction) |
| `storeDataInDom`, `fetchDataFromDom`, `replacePlaneTextBody` (25930-25948) | Cache DOM du corps déjà rendu | Performance | aucun | SANS OBJET | Cache côté client RainLoop |
| `flagHash` (25954-25956) | Détecte un changement de drapeaux | Rafraîchissement | aucun | SANS OBJET | Lié au cache client |

## 6. js-cookie (25964-26143)

| Élément | But | Besoin | Équivalent | Verdict | |
|---|---|---|---|---|---|
| Bibliothèque js-cookie 2.2.1 (get/set/remove) | Cookies côté client (langue, préférences) | Stockage | `sessionStorage` / cookies serveur wmail | SANS OBJET | Bibliothèque embarquée |

## 7. core-js / polyfills (26145-27577, modules 325-391)

| Élément | But | Besoin | Équivalent | Verdict |
|---|---|---|---|---|
| `core-js/internals/*`, `es/instance/{trim,find,concat,reduce,map,filter,slice,reverse,for-each}`, `es/object/{create,keys,define-property,set-prototype-of,get-own-property-descriptor}`, `array/virtual/*` | Polyfills ES pour vieux navigateurs | Compat | aucun | SANS OBJET |

## 8. AccountPopupView (27577-27712) : popup « Ajouter / modifier un compte »

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Champs `email`, `password`, erreurs `emailError/passwordError` effacées à la frappe (27627-27639) | Popup à 2 champs ; une erreur rouge disparaît dès qu'on retape | Ajouter un compte additionnel (domaine déjà connu du serveur) | `account_form.html:8-117` : formulaire complet IMAP/SMTP/Sieve (page), validation serveur `accounts.go buildAccount` ; `accounts.js:11-17` suggère `imap.domaine`/`smtp.domaine` à partir de l'e-mail | PARTIEL | RainLoop n'a besoin que de l'e-mail + mot de passe car le serveur connaît les domaines ; wmail-go (autonome) exige les serveurs : écart assumé. P3 : préremplir aussi `imap_login` (déjà fait `accounts.js:16`) et ports (déjà) |
| `addAccountCommand` (27650-27683) : vérifie non vide, envoie, affiche l'erreur et un message additionnel, ferme et recharge la liste des comptes | Enregistrement + retour d'erreur | Valider la connexion | `accounts.go accountSave` + bouton « Tester » `account_form.html:114` (affiche IMAP / SMTP résultats `account_form.html:8-9`) | OK | Mieux (test IMAP + SMTP séparés) |
| `onShow(account)` mode édition : e-mail rempli, `isNew=false` (27696-27703) | Modifier un compte existant | Édition | `accounts.go:365 accountEdit` ; `accounts.html` liste | OK | |
| `onShowWithDelay` : focus sur l'e-mail (27705-27707) | Focus automatique | Confort | `account_form.html` `autofocus` absent sur le champ e-mail (grep : aucun `autofocus` dans ce fichier) | PARTIEL | P3 : ajouter `autofocus` au champ e-mail |

## 9. IdentityPopupView (27714-27905) : popup « Identité »

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Champs `email` (validé), `name`, `replyTo`, `bcc` (validés), `signature`, `signatureInsertBefore` (27772-27782) | Éditer une identité d'expédition | Identités multiples | `identities.html:23-37` : email, nom, `reply_to`, `bcc`, `signature`, case `signature_before` ; `identities.go:67-80` validation | OK | |
| `showBcc` / `showReplyTo` : champs Cci / Répondre-à cachés tant qu'on n'a pas cliqué un lien ou rempli une valeur (27782-27797) | Formulaire épuré | Confort | `identities.html:29-32` toujours visibles | PARTIEL | Cosmétique. P3 : `<details>` repliable |
| `signature.__fetchEditorValue()` (27807) : signature en **éditeur HTML** | Signature avec mise en forme (gras, lien, image) | Signature riche | `identities.html:33-35` : `<textarea>` texte brut ; `fr.json:288` « précédée de -- » | PARTIEL | **P2** : si la signature RainLoop actuelle est en HTML, la signature texte perd sa mise en forme. Proposition : réutiliser `editor.js` (déjà utilisé en composition) pour le champ signature, stocker un indicateur `signature_html` |
| `addOrEditIdentityCommand` : e-mail vide → erreur + focus (sauf propriétaire) ; replyTo/bcc invalides → focus (27811-27831) | Validation client | Valider | validation serveur `identities.go:76-84` (+ `required` HTML sur l'e-mail) | OK | |
| `identityUpdate` + rechargement des identités (27834-27848) | Sauvegarde | Persister | `identities.go` POST `/accounts/{id}/identities` | OK | |
| `owner` (`id === ''`) : l'identité principale n'a pas de champ e-mail modifiable (27816, 27887) | Identité du compte : on édite nom/signature uniquement | Édition limitée | `identities.html:5-6` ligne « Principale » → lien `/accounts/{id}/edit` (formulaire compte, qui contient nom et signature) | OK | |
| `fakeMd5()` id d'une nouvelle identité (27889) | Identifiant | Plomberie | `identities.go` `newID` (`accounts.go:251`) | OK | |
| `onShowWithDelay` focus e-mail hors mobile (27893-27897) | Focus auto | Confort | `identities.html` pas d'`autofocus` | PARTIEL | P3 |

## 10. Composants Knockout (27908-28312)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `SaveTriggerComponent` (27932-27996) : après un réglage enregistré, icône animée (rotation) → coche verte → disparition, ou croix rouge | Retour visuel d'enregistrement automatique sur chaque réglage | Savoir que le réglage est sauvé | wmail-go enregistre par POST + redirection avec message `{{template "messages"}}` (`settings.html`) | PARTIEL | Pas d'indicateur par champ (les réglages ne s'enregistrent pas à la volée). P3 : un `<p class="msg ok">` suffit |
| `InputComponent`, `SelectComponent`, `RadioComponent`, `TextAreaComponent`, `DateComponent` (28016-28177) | Composants de formulaire liés aux observables (`TextAreaComponent` : 5 lignes, sans correcteur par défaut) | Formulaires | éléments HTML natifs | SANS OBJET | Mécanisme Knockout |
| `ScriptComponent` (28204-28233) | Injection de `<script>` dans les gabarits | Plomberie | aucun | SANS OBJET | |
| `CheckboxMaterialDesignComponent` (28260-28311) : animations « boîte » puis « coche » 200 ms décalées | Cases à cocher style Material (liste, réglages) avec animation | Rendu des cases | `home.html:80` `<label class="chk"><input type="checkbox">` ; CSS `.chk` : style (je n'ai pas relu `.chk` dans `mail.css`) | PARTIEL | Non vérifié. P3 : cases personnalisées sans animation ; reproduire la coche animée si on veut une ressemblance visuelle exacte |

## 11. AppUser (28314-29690) : application utilisateur

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Minuteries `interval.30s … 20m` et décalées « after5m » (28449-28497) | Horloge interne pour sondages périodiques | Rafraîchir automatiquement | `poll.js:73` `setInterval(poll, 120000)` + `visibilitychange` | OK | |
| `wakeUp` après 60 min (28499-28509) : recharge si la version JS a changé ou le timestamp expire | Récupération après mise en veille | Résilience | `poll.js:74` re-sonde au retour de visibilité ; session `server.go:367` | PARTIEL | Pas de rechargement de la liste au réveil. Voir ligne « rafraîchissement de la liste » |
| `UserBackgroundHash` : image de fond personnalisée (`backstretch`, fondu 1 s) (28515-28523) | Fond d'écran utilisateur | Thème | `background.go` ; thème `rainloop` `theme.css:1-3` (fond `background.jpg` posé dans le répertoire de données) | OK | Fondu d'apparition non reproduit (P3) |
| `reload` (28535-28541) | Recharge la page | Plomberie | `location.reload()` | SANS OBJET | |
| `reloadFlagsCurrentMessageListAndMessageFromCache` (28543-28549) | Réapplique les drapeaux du cache à la liste | Cohérence | pas de cache client | SANS OBJET | |
| `reloadMessageList` (28556-28593) : recharge la page courante, message d'erreur `CANT_GET_MESSAGE_LIST` en cas d'échec ; remet à la page 1 si on a vidé | Liste toujours à jour ; erreur lisible | Affichage de la liste | rendu serveur `home.html:57-58` (`{{if .Error}}` message d'erreur) ; bouton « Actualiser » `base.html:101` (lien vers `Back`) | OK | |
| `recacheInboxMessageList` (28595-28597) | Préchauffe le cache de la boîte de réception | Perf | aucun | SANS OBJET | |
| `contactsSync` (28604-28620) + intervalle `ContactsSyncInterval` borné 5..320 min, 1er lancement à +10 s (29585-29598) | Synchronisation CardDAV automatique | Synchroniser les contacts | `contactssync.go`, `contactsweb.go:195-196`, `contacts.html:82-100` (activer, URL, user, mot de passe, intervalle 5-320, « Synchroniser maintenant », dernier état/erreur) | OK | |
| `messagesMoveTrigger` (debounce 500 ms) / `messagesMoveHelper` / `moveCache` (28622-28651) : regroupe les déplacements par (source, cible) ; marque `SPAM` / `HAM` selon le dossier cible | Déplacements groupés ; apprentissage antispam | Déplacer | `mail.go:1349-1360` actions `move`/`copy` côté serveur ; pas d'apprentissage spam | OK | Apprentissage SPAM/HAM : SANS OBJET (dépend d'un plugin serveur RainLoop) |
| `messagesCopyHelper`, `messagesDeleteHelper` (28653-28659) | Copier / supprimer | Actions | `mail.go:1356 copy` ; `Expunge` | OK | |
| `moveOrDeleteResponseHelper` (28661-28676) : met à jour les hash dossier, `alert()` si échec de déplacement/copie, recharge la liste, quota | Retour d'erreur | Erreurs | `s.mailErr(w,r,sess,err)` `mail.go:1371-1374` page d'erreur | OK | |
| `deleteMessagesFromFolderWithoutCheck` (28683-28686) | Supprimer sans confirmation | Plomberie | `Expunge` | OK | |
| `deleteMessagesFromFolder` (28695-28743) : type Spam / NotSpam / Trash / Archive ; si dossier non configuré → popup dossiers système ; si dossier d'origine = corbeille ou spam → confirmation « voulez-vous supprimer ? » et suppression définitive | Suppression sûre | Supprimer / archiver / spam | `mail.go:1344-1371` (corbeille, `Expunge` si déjà en corbeille, EnsureFolder) ; `base.html:114` `data-confirm` quand `ClearKind` ou `Msg.InTrash` ; `list.js:19-22` confirm | PARTIEL | Écart : supprimer **depuis Indésirables** déplace en corbeille chez wmail-go, alors que RainLoop demande confirmation et supprime définitivement. P3 |
| Type `NotSpam` (28707-28709) : depuis le dossier Spam, bouton « Pas du spam » déplace vers INBOX | Récupérer un faux positif | Dé-spammer | `base.html:113` le bouton `spam` est toujours « spam » ; dans le dossier Indésirables `moveTo(junk)` renvoie nil (rien) ; grep `notspam|not_spam|ham` : aucun | MANQUANT | **P2** : dans le dossier Indésirables, remplacer l'action `spam` par `notspam` (libellé « Pas un indésirable ») qui déplace vers INBOX (`mail.go` + clé i18n) |
| `moveMessagesToFolder` (28752-28770) : glisser-déposer / menu Déplacer, copie avec Ctrl | Déplacer ou copier | DnD | `dnd.js:1-3` (glisser sur un dossier ; Ctrl = copie ; dossier replié s'ouvre au survol) ; menu `base.html:103-114` | OK | |
| `foldersReload` / `foldersPromisesActionHelper` (28776-28805) | Recharge l'arbre des dossiers ; erreur affichée dans la liste de dossiers | Dossiers | arbre rendu serveur `foldertree.go`, `folders.go` ; erreurs `FolderError` | OK | |
| `reloadOpenPgpKeys` (28807-28846) | Trousseau OpenPGP | PGP | aucun | SANS OBJET | OpenPGP exclu |
| `accountsCounts` (28848-28875) | Compteurs des autres comptes (code mort : `return false`) | n/a | aucun | SANS OBJET | Code désactivé dans RainLoop |
| `accountsAndIdentities` (28877-28929) : charge comptes (le compte parent non supprimable) et identités | Menu comptes + identités | Comptes | `accounts.go accountViews`, `identities.go` | OK | |
| `templates` (28931-28944) : charge les modèles de réponse | Modèles | Modèles | `templates.html`, `templates_settings.go` | OK | |
| `quota` (28946-28952) + `quotaDebounce` 30 s après chaque déplacement, 15 min périodique | Jauge de quota | Quota | `mail.go:525-526`, `base.html:220` `<progress>` + texte, classe `full` ≥ 90 % | OK | Rafraîchi à chaque page serveur |
| `folderInformation` (28959-29023) : état d'un dossier (hash, nombre, non lus, drapeaux, UidNext, nouveaux messages) ; si le hash change et que c'est le dossier courant → **recharge la liste** ; sinon si INBOX → recache | Liste de la boîte qui se met à jour toute seule, drapeaux changés ailleurs reflétés | Voir le nouveau courrier sans recharger | `poll.js:46-70` : titre `(n)`, badges non lus par dossier, notification + son ; la **liste n'est pas rechargée** | PARTIEL | **P2** : quand `poll` détecte un nouveau `uidnext` (ou `unseen` différent) et qu'on est sur INBOX page 1 sans recherche, soit recharger la liste en douceur (fetch de `/mail?...` et remplacement de `ul.msgrows`), soit afficher une barre « n nouveaux messages — actualiser » |
| `folderInformationMultiply` (29029-29097) : même chose pour plusieurs dossiers (2 min après 5 min) | Compteurs de tous les dossiers visibles | Badges | `poll.js:57-65` boucle sur `d.folders` | OK | |
| `messageListAction` (29105-29169) : marque lu/non lu/favori **de manière optimiste** (cache) et met à jour le compteur du dossier avant la réponse | Retour instantané | Marquer | `base.html:123-126` POST + redirection ; `list.js:200-208` décrémente le badge à l'ouverture d'un message non lu | PARTIEL | Pas d'optimisme pour les boutons groupés (rechargement). P3 |
| `googleConnect`, `twitterConnect`, `facebookConnect`, `socialUsers`, `*Disconnect` (29171-29228) | Connexions sociales | Social | aucun | SANS OBJET | Social exclu |
| `getAutocomplete` (29235-29245) : suggestions de contacts à la frappe | Autocomplétion destinataires | Composer | `contacts-ac.js` (`GET /contacts/suggest`, `contactsweb.go:355`) | OK | |
| `setExpandedFolder` (29252-29267) : mémorise les dossiers dépliés (localStorage) | Arbre de dossiers dans l'état laissé | Arbre | `mail.go:1394 openFolders` (état enregistré dans le compte) ; `dnd.js` dépliage au survol | OK | |
| `initHorizontalLayoutResizer` (29269-29350) : disposition **aperçu en bas** (liste au-dessus, message dessous), poignée de hauteur 200-500 px mémorisée | Layout « bottom preview » | Disposition | `settings.html:19-22` : seulement `three` (volets côte à côte) et `list` | MANQUANT | P3 : ajouter `bottom` (grille verticale + séparateur `--list-h`) si l'utilisateur l'emploie ; sinon ignorer |
| `initVerticalLayoutResizer` (29352-29435) : largeur du panneau de dossiers 155-350 px, mémorisée, 60 px quand replié | Panneau dossiers redimensionnable | Redimensionner | `list.js:291-335` séparateurs (dossiers 160-480, liste 240-1000), POST `/settings/panes`, dblclick = défaut ; repli `body.folders-collapsed` `mail.css:400-402` | OK | |
| `logout` (29437-29443) | Déconnexion (revient au compte parent si compte additionnel) | Quitter | `/logout` | OK | |
| `bootstartTwoFactorScreen` (29445-29447) | Impose la configuration 2FA | 2FA obligatoire | `twofactor.go` (activation volontaire) | SANS OBJET | Obligation par l'admin : mono-utilisateur |
| `bootstartWelcomePopup` (29449-29451) | Page d'accueil | Admin | aucun | SANS OBJET | |
| `bootstartLoginScreen` (29453-29470), `customLoginLink` | Écran de connexion ou redirection | Connexion | `login.html` | OK | |
| `bootend`, `progressJs` (29472-29478) | Barre de progression de chargement | Confort | aucun (pas de SPA) | SANS OBJET | |
| `bootstart` : `StartupUrl` (29519-29523) | Ouvrir directement une page au démarrage (ex. un dossier) | Page de démarrage | aucun (grep `startup` : rien) | MANQUANT | P3 : réglage « Page de démarrage » (dossier) ; peu utile en mono-utilisateur |
| `bootstart` : chargement OpenPGP (29525-29557), réseaux sociaux (29562-29564, 29658-29680) | Modules optionnels | PGP/social | aucun | SANS OBJET | |
| `bootstart` : `interval.2m` → INBOX, `3m` → dossier courant, `2m-after5m` → autres, `15m` → quota, `20m` → recharge dossiers (29566-29584) | Sondage échelonné | Fraîcheur | `poll.js` : un seul sondage 2 min (INBOX + compteurs des dossiers) ; le dossier courant autre qu'INBOX n'est pas rechargé | PARTIEL | Voir ligne `folderInformation` |
| `bootstart` : `registerProtocolHandler('mailto')` (29630-29641) + `MailToEmail` ouvre la composition | wmail comme gestionnaire de `mailto:` | Liens mailto | `mailto.js:1-15` bouton d'enregistrement ; `compose` accepte `?mailto=` (`compose.go:328,387`) | PARTIEL | RainLoop l'enregistre automatiquement (si « rester connecté ») ; wmail-go demande un clic dans les réglages. P3 |
| `bootstart` : redimensionneur vertical sauf mobile (29643-29647) | Initialisation | Plomberie | `list.js:291` | OK | |
| `interval.1m` → `momentor.reload` (29682-29684) | Les dates relatives (« il y a 5 min ») se rafraîchissent chaque minute | Dates vivantes | `mail.go:362-390 listDate` : « il y a n min » calculé à la génération de la page, jamais rafraîchi | PARTIEL | P3 : script de 10 lignes qui recalcule `data-ts` chaque minute |
| `rl.auto-logout` (29618) | Déconnexion automatique sur inactivité | Sécurité | `server.go:367` inactivité de session côté serveur (`settings.html:13-14`) | OK | |
| `appDelayStart` (29614-29616) | Ping serveur de démarrage différé | Plomberie | aucun | SANS OBJET | |
| `jsVersion` / `checkTimestamp` (28504-28513) | Détecte une nouvelle version du JS | Plomberie | aucun | SANS OBJET | |

## 12. Base64 (29697-29850)

| Élément | But | Verdict |
|---|---|---|
| Module `Base64` (encode / decode / urlsafe_encode) | Encodage client des paramètres d'URL RainLoop | SANS OBJET (bibliothèque embarquée ; wmail-go passe les paramètres en clair via `url.Values`) |

## 13. PromisesUserPopulator (29856-30031)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `isFolderExpanded` / `folderResponseParseRec` (29903-29975) : arbre récursif, état replié/déplié restauré, compteurs initiaux (`MessageCount`, `MessageUnseenCount`) | Panneau de dossiers avec sous-dossiers repliables et badges | Arbre de dossiers | `foldertree.go`, `mail.go:1394 openFolders`, badges `poll.js:57-65` | OK | |
| `checkable` (29943-29947) : dossiers « à vérifier » (sondés) ; l'option n'est affichée que si > limite | Choisir les dossiers dont on suit le non-lu | Surveiller des dossiers | `folders_settings.html:25-31` section « check » (`folders.check_hint 5`) | OK | |
| `normalizeFolder` (29912-29914) | Ignore un dossier système qui n'existe plus | Robustesse | `mailconn.ResolveTrash/…` repli automatique | OK | |
| `foldersList` : `folderSpecLimit` borné 10..100 (29977-29985) | Limite de dossiers vérifiés | Plomberie | `folders_settings.html:25` limite 5 | OK | |
| `foldersAdditionalParameters` : `SystemFolders` détectés automatiquement (SPECIAL-USE) la première fois et enregistrés ; `useImapThread` ; espace de noms (29988-30025) | Auto-détection Sent/Drafts/Spam/Trash/Archive ; threads IMAP | Rôles de dossiers | `mailconn.ResolveArchive/Junk/Trash` (résolution auto, jamais figée) ; `home.html:50` `ThreadsUnsupported` | OK | |

## 14. FilterModel (30034-30345)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Observables `enabled`, `name`, `conditions`, `conditionsType` (Any par défaut) (30092-30098) | Filtre Sieve : nom, actif, conditions, ET/OU | Règles de tri | `filter_edit.html:13-24` nom, activé, `match` Any/All ; `sieve/filters.go:59-96` | OK | |
| Action : `MoveTo`, `Forward`, `Vacation`, `Reject`, `Discard`, `None` ; `actionValue`, `Second` (objet vacances), `Third` (jours), `Fourth` (adresses) ; `actionMarkAsRead`, `actionKeep`, `actionNoStop` (30100-30123) | Toutes les actions de filtre | Filtres | `filter_edit.html:51-76` : MoveTo, Forward (+`keep`), Reject, Vacation (objet, message, jours, adresses, `stop`), Discard, None ; `mark_read` | PARTIEL | « Ne pas arrêter » (`actionNoStop`) : RainLoop l'offre pour toute action (non vérifié dans ses gabarits) ; wmail-go ne l'affiche que pour Vacation (`filter_edit.html:73`). P3 : montrer `stop` pour toutes les actions si c'est le cas côté RainLoop |
| `nameSub` (30130-30163) : sous-titre « (Déplacer vers X / Rediriger vers Y / Message d'absence / Rejeter / Supprimer) » dans la liste ; dossiers affichés « A / B » | Résumé lisible de l'action | Liste de filtres | `filters.html:15` colonne « Résumé » `.Summary` (`filters.go`) | OK | Format exact non comparé |
| `actionTemplate` (30164-30195) | Choix du sous-formulaire d'action | Formulaire dynamique | `filters.js:41-46 syncAction` (affiche `.act[data-act]`) | OK | |
| Validation `verify` (30220-30254) : nom requis ; conditions valides ; valeur d'action requise pour Move/Forward/Reject/Vacation ; Forward doit contenir `@` ; adresses vacances doivent contenir `@` | Contrôle avant enregistrement | Valider | `sieve/filters.go:401 Valid(modules)` renvoie les erreurs ; `filter_edit.html:5` liste `.Errors` ; `required` HTML sur le nom | OK | |
| `toJson` / `parse` (30256-30318) (`Stop = !NoStop`, `Keep`, `MarkAsRead`) | Sérialisation vers le serveur | Persistance | `sieve/filters.go:83-150` JSON dans un en-tête du script, compatible RainLoop (`unmarshalHeader`) | OK | |
| `addCondition` / `removeCondition` (30276-30283) | Ajouter / supprimer une condition | Conditions multiples | `filters.js:25-37` (+ / ×) | OK | |
| `setRecipients` (30285-30287) : remplit « adresses concernées » avec tous les e-mails des comptes | Bouton « mes adresses » pour le répondeur d'absence | Vacances | `vac_addrs` champ libre `filter_edit.html:71` ; aucun bouton | MANQUANT | P3 : bouton « Utiliser mes adresses » (e-mails du compte + identités) |
| `cloneSelf` (30320-30340) | Édite une copie, annule sans effet | Annuler | formulaire POST (annulation = lien) | OK | |

## 15. FilterConditionModel (30348-30465)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `field` (From par défaut), `type` (Contains), `value`, `valueSecond` (30379-30384) | Une condition | Condition | `filter_edit.html:21-38` | OK | |
| `template` selon champ : Size → gabarit taille ; Header → gabarit « autre » avec nom d'en-tête ; sinon défaut (30385-30403) | Le formulaire s'adapte au champ | Formulaire dynamique | `filters.js:10-21 syncRow` : champ en-tête visible si Header ; types Over/Under seulement pour Size | OK | |
| Reset des valeurs au changement de champ (30405-30409) | Évite des valeurs incohérentes | Confort | `filters.js:10-21` ne vide pas la valeur au changement de champ | PARTIEL | P3 |
| `verify` (30418-30430) : valeur obligatoire ; nom d'en-tête obligatoire pour Header | Validation | Valider | `sieve/filters.go:401 Valid` | OK | |
| `parse`, `toJson`, `cloneSelf` (30432-30460) | Sérialisation | Persistance | `sieve/filters.go:59-66` | OK | |
| Ordre des champs : De, Destinataires, Sujet, **Taille**, En-tête (31572-31587) | Liste déroulante des champs | Libellés | `filter_edit.html:21-27` ordre De, Destinataire, Sujet, **En-tête**, Taille | PARTIEL | P3 : permuter Taille et En-tête pour garder l'ordre habituel |

## 16. Module 409 (30468-30476)

| Élément | But | Verdict |
|---|---|---|
| `window.qr` externe (bibliothèque de QR code) | Génère le QR du secret 2FA côté client | SANS OBJET (wmail-go génère le PNG côté serveur : `twofactor.html:30` `/settings/2fa/qr.png`) |

## 17. KeyboardShortcutsHelpPopupView (30495-30575)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Popup d'aide à **onglets** (liste des messages / message / composition…), clavier Tab / Maj+Tab / ← / → pour changer d'onglet (30546-30571) | Aide raccourcis navigable | Aide | `base.html:163-190` un seul tableau avec sections `keys.list / keys.message / compose.title / keys.general` | PARTIEL | Contenu équivalent (plus complet : Ctrl+Entrée, Ctrl+S, T…). Pas d'onglets ni de navigation Tab/←/→. P3 |

## 18. ContactsPopupView (30582-31283)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Disposition : popup à deux colonnes (liste à gauche avec recherche, fiche à droite) (30698-30844) | Gérer le carnet sans quitter la page ; liste + détail côte à côte | Contacts | `contacts.html` : page de réglages avec tableau puis formulaire **en dessous** (`{{if .Editing}}`) | PARTIEL | Écart visuel. P3 : mise en page en 2 colonnes (grille CSS) de `contacts.html` |
| `search` + `reloadContactList` à la frappe ; `contactsPage` / `CONTACTS_PER_PAGE` ; pagination `computedPagenatorHelper` (30703-30715, 31182-31224) | Recherche instantanée, pagination | Chercher | `contacts.html:6-12` recherche + filtre par étiquette (en plus) en GET, pagination `PrevURL/NextURL` | OK | Recherche non instantanée (bouton) ; P3 |
| `viewClearSearch` + `clearCommand` (30715, 30914-30916) | Bouton « effacer la recherche » | Confort | aucun lien d'effacement dans `contacts.html` (grep : pas de `ClearSearch`) | PARTIEL | P3 |
| `viewProperties` : Prénom, Nom (toujours), Surnom, e-mails, téléphones, sites web, anniversaire, notes ; typage (`Home`, `Mobile`…) ; champs vides auto-supprimés au défocus (30718-30784) | Fiche contact | Fiche | `contacts.html:34-70` : civilité, prénom, 2e prénom, nom, suffixe, surnom, société, poste, anniversaire, e-mails typés, téléphones typés, URL, adresses postales, étiquettes, notes | OK | Plus riche (adresses, étiquettes) |
| `addNewEmail/Phone/Web/Nickname/Notes/Birthday` (30994-31032) : boutons « + » ajoutent une ligne | Ajouter des lignes à la volée | Fiche dynamique | `contactsweb.go:102 padTyped(l, n)` : lignes vides préfabriquées en nombre fixe, pas de bouton | PARTIEL | P3 : bouton « + » JS qui clone une ligne |
| `viewHasNonEmptyRequiredProperties` (30740-30748) : enregistrement possible seulement avec un nom ou un e-mail | Éviter les fiches vides | Valider | `contactsweb.go:238 contactsSave` (validation serveur) | OK | |
| `contactsChecked`, `contactsCheckedOrSelected` (30797-30812) | Cochés ou fiche ouverte | Sélection | `contacts.html:21` cases `name=id` | OK | |
| `selector` sur la liste de contacts (30813-30826) : flèches, Maj, Espace, Entrée, clic ouvre la fiche | Navigation clavier dans les contacts | Clavier | aucun (`list.js`/`listnav.js` ne ciblent que `.msgrows`) | MANQUANT | P3 : navigation clavier dans le tableau de contacts |
| Détection de modifications `viewHash` / `watchDirty` (30828-30840) : le bouton Enregistrer ne s'active qu'après modification | Éviter les enregistrements inutiles | Confort | aucun | PARTIEL | P3 |
| `newCommand` (30848-30851) | Nouveau contact | Créer | `contacts.html:11` bouton « Nouveau » | OK | |
| `deleteCommand` / `deleteSelectedContacts` / `removeCheckedOrSelected…` / `deleteResponse` (30853-30856, 31073-31128) : supprime cochés ou sélectionné, animation de disparition 500 ms | Suppression | Supprimer | `contacts.html:27-29` `<details class="danger">` confirmation + POST `/contacts/delete` | OK | |
| `newMessageCommand` (30858-30912) : « Nouveau message » vers les contacts cochés ou ouverts ; champ cible selon le dernier champ focalisé de la composition (To/Cc/Bcc) | Écrire à plusieurs contacts d'un coup | Écrire | `contacts.html:66` « Écrire » seulement pour **un** contact en édition ; lien e-mail par ligne `contacts.html:19` ; pas d'action « écrire aux cochés » | MANQUANT | **P2** : bouton « Écrire aux sélectionnés » (GET `/compose?to=a,b,c`) |
| `saveCommand` (30918-30959) : enregistre, `viewSaveTrigger` animé (rotation → coche) | Enregistrer avec retour visuel | Enregistrer | `contactsSave` + redirection avec message | OK | |
| `syncCommand` (30961-30971) : bouton « Synchroniser » avec alerte d'erreur | Synchronisation manuelle | Sync | `contacts.html:97` « Synchroniser maintenant » ; erreur `contacts.html:95` | OK | |
| `getPropertyPlaceholder` (30973-30992) : « Nom », « Prénom », « Surnom » | Libellés d'aide | Confort | libellés `contacts.first`, `contacts.last` (labels, pas de placeholder) | OK | |
| `exportVcf`, `exportCsv` (31034-31040) | Export | Export | `contacts.html:79` liens vCard / CSV (`/contacts/export`) | OK | |
| `initUploader` (Jua) : import CSV/VCF avec indicateur, alerte si échec (31042-31071) | Import de contacts | Import | `contacts.html:72-78` formulaire fichier `.vcf,.csv` + résultat (ajoutés/ignorés) | OK | |
| `populateViewContact` : contact en lecture seule (`readOnly`) (31139-31176) | Contacts issus de la synchro non modifiables | Lecture seule | aucun indicateur `readOnly` (le code de synchro ne marque pas les contacts) | PARTIEL | P3 |
| `onBuild` : touches `delete` et `c, w` dans la liste de contacts (31232-31241) | Raccourcis contacts | Clavier | aucun | MANQUANT | P3 (regroupé avec la navigation clavier) |
| `onShow(bBackToCompose, field)` / `onHide` : ouvert depuis la composition, ajoute les contacts au champ voulu puis revient à la composition | Sélecteur de contacts de la composition | Choisir des destinataires | `contacts-ac.js:82-116` bouton « Choisir » → `<dialog class="picker">` (recherche, liste) ; `compose.html:42` | PARTIEL | Le sélecteur de wmail cible **À** (`to`) seulement (`contacts-ac.js:82`) ; RainLoop choisit To/Cc/Bcc selon le champ focalisé. P2 : lire le dernier champ focalisé (`cp-to|cp-cc|cp-bcc`) comme destination |

## 19. FolderCreateView (31290-31403)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `folderName` + liste `parentFolderSelectList` (dossiers système annotés, tête vide) (31343-31363) | Popup « Créer un dossier » avec dossier parent | Créer | `folders_settings.html:35-45` (nom, parent « racine ») ; et dialogue `#newfolder` `base.html:222` (accessible depuis le panneau de dossiers) | OK | |
| `simpleFolderNameValidation` : nom sans `\` ni `/` (31381-31383) | Valider | Nom valide | `folders.go:148-158` : refuse délimiteur, `*`, `%`, `.`/`..` | OK | |
| `createFolderCommand` + rechargement des dossiers (31370-31379) | Création | Créer | `folders.go:169` action `create` | OK | |
| Focus hors mobile (31395-31399) | Confort | | `base.html:222` dialogue (focus non vérifié) | OK | |

## 20. FilterPopupView (31410-31668)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `saveFilterCommand` : copie dossier choisi dans `actionValue` pour MoveTo, `verify`, callback (31498-31526) | Enregistrer le filtre | Enregistrer | `filters.go:272-314` | OK | |
| `populateOptions` : actions affichées selon les modules Sieve (`moveto`, `redirect`, `reject`, `vacation`, `markasread`) ; Discard toujours (31528-31570) | Ne proposer que ce que le serveur gère | Capacités serveur | `filter_edit.html:51-56` options `disabled` selon `.Modules.fileinto/reject/vacation/imap4flags` | OK | |
| Champs et types (De, Destinataires, Sujet, Taille, En-tête ; Contient, Ne contient pas, Égal, Différent, Regex si module, Plus grand, Plus petit) (31572-31615) | Libellés | Conditions | `filter_edit.html:21-37` | OK | (ordre : voir §15) |
| Option « Aucune action » volontairement commentée dans RainLoop (31529-31530) | — | — | `filter_edit.html:51` propose `None` | PARTIEL | Extra sans conséquence |
| `folderSelectList = folderMenuForFilters` (31476) | Liste de dossiers pour « Déplacer vers » | Dossier cible | `filter_edit.html:57-59` select `.Folders` | OK | |
| `onShow` / `onShowWithDelay` : focus sur le nom d'un nouveau filtre (31637-31664) | Confort | Focus | `filter_edit.html:12` pas d'`autofocus` | PARTIEL | P3 |

## 21. TwoFactorConfigurationPopupView (31675-31899)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `createTwoFactor`, `getQr` (`otpauth://totp/…`), QR côté client (31802-31804, 31839-31841) | Générer le secret et son QR | Activer la 2FA | `twofactor.html:28-30` secret + QR PNG serveur ; `twofactor.go` | OK | |
| `testTwoFactor` : popup de test du code avant activation ; l'activation n'est permise qu'après test réussi (31746-31775, 31811-31813) | Éviter de se verrouiller dehors | Valider | `twofactor.html:31-35` code demandé pour **confirmer l'activation** | OK | Même garantie, en une étape |
| `viewBackupCodes` (31742, 31853) | Codes de secours | Secours | `twofactor.html:8-12` codes affichés une fois, `backup_left` ; régénération avec code `twofactor.html:19-23` | OK | |
| `showSecret` / `hideSecret` (31790-31800, 31872-31888) : réafficher le secret et le QR après activation | Reconfigurer un autre appareil | Re-afficher | aucun (secret non réaffichable une fois activé) | MANQUANT | P3 : volontaire côté sécurité ; à ne faire que si utile (nécessite le code courant) |
| `clearTwoFactor` (31815-31823) | Supprimer la configuration | Désactiver | `twofactor.html:24-30` désactivation avec code | OK | |
| `viewEnable` interrupteur activer/désactiver sans effacer le secret (31745-31771) | Suspendre temporairement | Basculer | aucun (désactivation = suppression) | PARTIEL | P3 |
| `lock` : 2FA imposée, la popup ne se ferme qu'en rechargeant (31825-31837) | Obligation admin | Forcer | aucun | SANS OBJET | Mono-utilisateur |
| `onBuild` : chargement de l'état (31890-31895) | Affiche l'état courant | État | `twofactor.html:13-17` | OK | |

## 22. TemplatePopupView (31906-32000, début seulement)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Champs `name`, `body` (éditeur HTML `HtmlEditor`), erreurs (31958-31976) | Modèle de message réutilisable (nom + corps riche) | Modèles | `templates.html:22-26` nom + `<textarea>` corps (texte brut, variables `templates.vars`) | PARTIEL | Corps texte brut au lieu d'éditeur HTML (comme la signature). P3 (voir identités P2 : mutualiser l'éditeur) |
| `addTemplateCommand` (31982-32000…) : nom et corps non vides (corps `:HTML:` seul = vide) ; fin de la fonction au-delà de la ligne 32000 | Enregistrer le modèle | Validation | `templates_settings.go`, `templates.html:22-26` (`required`) | OK | Fin non lue (hors tranche) |

---

## À AJOUTER (MANQUANT / PARTIEL, par priorité)

### P2
1. **Ouvrir le message voisin après suppression / archivage / spam / déplacement du message ouvert** (Selector 24523-24549, `autoSelect` + `isNextFocused`). Dans `mail.go` action `default:` (l. ~1384-1390), quand `open` figure dans les UID traités, calculer le voisin (même ordre que `PrevURL/NextURL`, `mail.go:628-633` : suivant sinon précédent) et rediriger vers son URL de lecture au lieu de `back`. Sans voisin, retour à la liste. Test : 3 messages, ouvrir le 2e, supprimer → le 3e s'ouvre.
2. **Action « Pas un indésirable »** (AppUser 28707-28709). Dans `base.html:113`, si le dossier courant est le dossier Indésirables (`mailconn.ResolveJunk`), remplacer le bouton `spam` par un bouton `action=notspam` (clé i18n fr/en « Pas un indésirable ») ; dans `mail.go` ajouter `notspam` à la liste blanche (l. 1304, 1317) et au `switch` : `c.Move(folder, uids, "INBOX")`.
3. **Rafraîchissement de la liste quand du courrier arrive** (AppUser `folderInformation` 29011-29017). `poll.js` : mémoriser `uidnext` ; si `d.uidnext` augmente et que la page est INBOX sans recherche/page > 1, afficher une barre « n nouveau(x) message(s) – Afficher » en haut de `.listpane`, ou refaire un `fetch(location.href)` et remplacer `ul.msgrows` en conservant cases cochées et ligne focalisée. Fournir aussi la mise à jour des drapeaux lus ailleurs (comparer l'ensemble des UID visibles).
4. **Clic de ligne avec Ctrl/Cmd et Maj** (Selector `actionClick` 24957-24992). `list.js:216-228` : si `e.shiftKey` → `preventDefault`, cocher la plage jusqu'à la dernière ligne cliquée (réutiliser la logique `lo/hi` l. 84-93) ; si `e.ctrlKey||e.metaKey` → `preventDefault`, basculer la case de la ligne ; sans modificateur, comportement actuel.
5. **État de conversation sur la ligne racine** (`hasUnseenSubMessage` / `hasFlaggedSubMessage`, 25146-25147, 25524-25525). Dans la construction des lignes (`mail.go`, champs `ThreadUIDs/ThreadCount`) calculer `ThreadUnseen` et `ThreadFlagged` (OU des drapeaux des membres), émettre les classes `.sub-unseen` / `.sub-flagged` sur le `<li>` (`home.html:75`) et les styler : pastille `.threads` en couleur « non lu » (`--unread-bar`), étoile jaune atténuée.
6. **Écrire aux contacts cochés** (Contacts `newMessageCommand` 30858-30912). `contacts.html` : bouton « Écrire aux sélectionnés » (`formaction="/compose" formmethod="get"`) construisant `to=` à partir des e-mails des lignes cochées (premier e-mail de chaque contact ; ignorer les contacts sans e-mail). Prévoir la même chose pour `cc`/`bcc` depuis le sélecteur.
7. **Sélecteur de contacts de la composition : choisir To/Cc/Bcc** (Contacts `onShow(bBackToCompose, sLastComposeFocusedField)`, 30255-30260 ; 30889-30902). `contacts-ac.js:82` : retenir le dernier champ focalisé parmi `cp-to/cp-cc/cp-bcc` (événement `focusin`) et y insérer les adresses choisies ; autoriser la sélection multiple dans le dialogue.
8. **Signature (et modèles) en HTML** (Identity `__fetchEditorValue` 27807 ; Template éditeur HTML 31958). Réutiliser `editor.js` sur `identities.html:33` / `account_form.html` / `templates.html:25`, stocker `signature_html` booléen, injecter en HTML dans la composition en mode HTML (`compose.go`) et en version texte dans le mode texte.

### P3
9. Vider le volet de lecture quand des cases sont cochées et décocher les cases à l'ouverture d'un message (Selector 24408-24435).
10. Après changement de page au clavier (↓ au dernier / ↑ au premier), focaliser et ouvrir la première / dernière ligne de la nouvelle page (Selector 24552-24579) via `sessionStorage`.
11. Désactiver la sélection de texte sur la liste (`.msglist-form{user-select:none}`), placer le focus clavier au clic d'une case (Selector 24633, 24639-24651).
12. Layout « aperçu en bas » avec séparateur de hauteur mémorisé (AppUser 29269-29350), option `bottom` dans `settings.html:19`.
13. Page de démarrage configurable (`StartupUrl`, AppUser 29519).
14. Dates relatives rafraîchies chaque minute (AppUser 29682-29684) : script sur `data-ts`.
15. Bouton « Utiliser mes adresses » pour le répondeur d'absence (`FilterModel.setRecipients` 30285) et réordonner les champs de condition (De, Destinataires, Sujet, Taille, En-tête) ; afficher « ne pas arrêter » pour toutes les actions si RainLoop le fait (à vérifier dans ses gabarits).
16. Contacts : mise en page deux colonnes, boutons « + » pour ajouter e-mails / téléphones / sites, bouton effacer la recherche, navigation clavier (↑ ↓ Entrée, `Suppr`, `c`), désactivation du bouton Enregistrer tant que rien n'a changé, drapeau lecture seule des contacts synchronisés.
17. Aide clavier à onglets navigables (Tab / ← / →), (KeyboardShortcutsHelp 30546).
18. Classes de ligne manquantes : `deleted-mark` (message `\Deleted` barré), `new` (animation d'arrivée), icônes de pièces jointes par type (pdf, archive, audio, vidéo) (MessageModel 25508-25527, 25128-25131).
19. Entrée « Ajouter un compte » directe dans le menu utilisateur (AbstractSystemDropDown 24288) ; `autofocus` sur les champs e-mail / nom des formulaires compte, identité, filtre.
20. Supprimer depuis Indésirables : demander une confirmation et supprimer définitivement (AppUser 28733-28738) ; optionnel.
21. Pré-afficher l'objet et l'expéditeur de la ligne dans le volet pendant le chargement (`populateByMessageListItem` 25745) ; images référencées par `Content-Location` (25563-25574).
22. 2FA : afficher à nouveau le secret avec le code courant ; suspendre sans effacer (31790, 31745).
