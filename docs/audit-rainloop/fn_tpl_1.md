# Audit gabarits RainLoop 1.17 (Views/User, tranche 1) contre wmail-go

Source RainLoop : `/Work/webmail_project/rainloop/1.17.0/app/templates/Views/User/` (lu en entier : MailFolderList, MailFolderListItem, MailFolderListSystemItem, MailMessageList, MailMessageListItem, MailMessageListItemNoPreviewPane, MailMessageView, SystemDropDown, About, PhotoSwipe, PopupsCompose, ComposeAttachment, PopupsAdvancedSearch, PopupsFolderClear, PopupsFolderCreate, PopupsFolderSystem, PopupsWindowSimpleMessage, Login, LoginWelcome).
Côté wmail-go, lus : `internal/web/templates/{home,base,compose,login,login_2fa,register,recover,folders_settings,mail_message,message,fragment}.html`, `static/{mail.css,app.css,list.js,keys.js,compose.js,dnd.js,listnav.js,preview.js,msgfull.js,folders.js,menus.js,poll.js,chips.js,emldrop.js,print.js,print.css,themes/rainloop/theme.css}`, `mail.go`, `foldertree.go`, `mailtools.go`, `server.go`, locales `fr.json` (extraits). Quelques repères dans `static/js/app.js` de RainLoop (printableUnreadCount, clearListIsVisible, allowDangerousActions, selectedDates) pour confirmer un comportement ; rien d'autre du JS (hors tranche).
Limites : pas lu `static/editor.js` en entier (barre d'outils riche, hors gabarits), ni `contacts-ac.js`, ni les thèmes `dark`/`light`. Le rendu pixel exact (polices d'icônes, ombres) n'a pas été comparé sur un navigateur : les écarts visuels ci-dessous sont déduits du balisage et du CSS lus. Remarque d'ensemble : l'essentiel de la boîte (3 volets, barres de 50 px, lignes de 52 px, couleurs DarkShine) est déjà recréé ; les écarts restants sont donc surtout des détails de comportement et de rendu.

Légende priorités : P1 = quotidien et clairement visible ; P2 = utile ; P3 = cosmétique ou rare.

---

## 1. MailFolderList.html (panneau des dossiers)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-folders` classes `focused`, `single-root-inbox`, `inbox-is-starred` (1) | Racine du panneau ; état « focus clavier », racine unique INBOX, étoile Boîte de réception active | Panneau cohérent selon contexte | `base.html:196` `<aside class="folders">` ; INBOX racine gérée par `namespacePrefix` (`foldertree.go:76`) ; étoile active `fstar.on` (`base.html:204`) | PARTIEL | Pas d'état « focus » du panneau (voir ligne navigation clavier dossiers). P3 |
| Barre `.b-toolbar` (2-12) `hide-on-mobile` | Boutons Nouveau message + Contacts en tête du panneau, à gauche | Accès rapide écrire / contacts | `base.html:97-98` (`btn-new` + bouton contacts dans `.mb-left`, largeur calée sur le panneau `mail.css:27`) | OK | Rien |
| Bouton Nouveau message `buttonCompose` (3-8) : icône avion, libellé, tooltip, `btn-success` vert / `btn-warning` orange si rédaction en cours | Vert normalement, orange quand une fenêtre de rédaction est ouverte | Rappeler qu'un brouillon est en cours de saisie | `base.html:97` vert `--new-bg:#84ab04` (`themes/rainloop/theme.css`), icône `send`, `title` | PARTIEL | Pas d'état orange : sans objet tant que la rédaction est une page entière (pas de fenêtre réductible). Voir ligne « minimiser » en §9. P3 |
| Bouton Contacts `buttonContacts` (9-11) icône carnet d'adresses | Ouvre les contacts | Accès contacts | `base.html:98` icône `contacts` (rectangle + silhouette), `title` | OK | Icône dessinée différemment (silhouette sur carnet, pas la police RainLoop) : cosmétique. P3 |
| `.b-content` zone défilante `nano` + `scrollerShadows` (13-22) | Liste des dossiers avec barre de défilement fine superposée et ombres haut/bas | Défilement discret | `.folders {overflow:auto}` `mail.css:70` (barre native) | PARTIEL | Style de barre : `scrollbar-width: thin` + couleurs translucides. P3 |
| `.b-folders-system` (16) + `<hr class="b-list-delimiter">` (17) + `.b-folders-user` (18) | Dossiers système, trait séparateur, dossiers utilisateur | Regroupement | `base.html:202-208` `ul.sys`, `hr.fsep`, `ul.usr` (`mail.css:83`) | OK | Rien |
| `.move-action-content-wrapper` visible si `moveAction` (19) | Voile pendant le glisser de messages | Retour visuel du glisser-déposer | `body.dnd` + `.folder-list li.drop-over` (`mail.css:430-431`, `dnd.js:277,294`) | PARTIEL | Pas de voile global, seulement surbrillance du dossier survolé : suffisant. P3 |
| `.b-content.show-on-panel-disabled` (23) | Panneau replié : clic sur la bande le réouvre | Réafficher le panneau replié | `folders-collapsed` (`mail.css:397-402`) + bouton `data-folders-toggle` (`base.html:216`) ; plus de clic sur la bande | PARTIEL | Faire que le clic sur la colonne repliée (hors boutons) la rouvre. P3 |
| Pied `.b-footer` (24-38) | Boutons bas du panneau | Gestion dossiers | `base.html:215-219` `.folders-foot` | OK | Rien |
| Bouton `buttonResize` (26-28) icônes `resize-out/in` | Replie/déplie le panneau | Gagner de la place | `base.html:216` bouton `expand` ; JS `folders.js:498-506` (état enregistré par POST `/settings/panes`) | OK | Icône unique `expand` au lieu de deux (in/out) : P3 |
| Bouton « créer un dossier » (31-33), icône `folder-add`, spinner quand `foldersChanging` | Ouvre le popup de création | Créer un dossier depuis la liste | `base.html:217` icône `folder` simple (pas de « + »), ouvre le dialogue `#newfolder` (`base.html:222-231`, `folders.js:515`) | PARTIEL | Icône sans « + » ; pas de spinner pendant la création (page rechargée). P3 |
| Bouton « configurer les dossiers » (34-36) icône roue | Ouvre Réglages > Dossiers | Gérer abonnements/dossiers | `base.html:218` lien `/settings` (pas `/settings/folders`) | PARTIEL | Pointer vers `/settings/folders` (une ligne). P2 |
| Navigation clavier dans le panneau (`focused-poiner`, scope FolderList, item 46/54) | Marqueur de focus + flèches/Entrée dans les dossiers | Parcourir les dossiers au clavier | aucun (les liens sont tabulables, pas de flèches ; `keys.js` n'a pas de portée « dossiers ») | MANQUANT | Flèches haut/bas + Entrée sur `.folder-list a`, classe `.focused` stylée. P3 |

## 2. MailFolderListItem.html (dossier utilisateur) et MailFolderListSystemItem.html

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.e-item` visible/`i-am-inbox-wrapper` (1) | Ligne de dossier ; masquée si non abonné | Cacher les dossiers non abonnés | `foldertree.go:51,92` (`f.Hidden` écarté sauf dossier actif) | OK | Rien |
| `.e-link` droppable (2-4) : survol/drop de messages (déplacement) | Déposer des messages sur un dossier | Glisser-déposer pour déplacer / copier (Ctrl) | `dnd.js:253-322`, `base.html:203,209` `data-drop` ; copie avec Ctrl/Alt (`dnd.js:294,312`) ; dépliage au survol 500 ms (`dnd.js:297-305`) | OK | Rien |
| Classes `selected`, `selectable`, `hidden` (4) | Surbrillance du dossier actif, dossiers non sélectionnables grisés | Repérer le dossier courant | `.f.active` (`mail.css:77`), `.f.nosel` grisé (`mail.css:82`, `foldertree` `nosel` dans `base.html:209`) | OK | Rien |
| `print-count` / `unread-sub` (4) | Gras si non lus ; marque si un sous-dossier abonné a des non lus | Visualiser les non lus enfouis | `.usr .f.has-unread a {bold}` `mail.css:79` ; `unread-sub` posé dans `base.html:209`, gras `mail.css:465` ; calcul `foldertree.go:145` | OK | Rien |
| Pastille `.badge.count` `printableUnreadCount` (5) | Nombre à droite. Règle RainLoop (confirmée dans `app.js:32521-32535`) : Brouillons = nombre TOTAL de messages ; Corbeille, Archive, Envoyés = jamais de compteur ; autres = non lus | Compteurs fidèles | `base.html:204,211` : pastille `Unseen` pour tous les dossiers sans exception | PARTIEL | Appliquer la règle : Brouillons -> `Total`, pas de pastille pour Corbeille/Archive/Envoyés. Dans `folderPanel`/gabarit (rôle connu : `roleDrafts`, `roleTrash`, `roleArchive`, `roleSent` `foldertree.go:20`) et dans `poll.js:55-71` (qui réinjecte les pastilles). P2 |
| Icône de repli `collapsedCss()` (6) + `b-sub-folders` `collapsed` (50) | Flèche/signe pour replier les sous-dossiers, mémorisé | Arbre repliable | `base.html:210` bouton `.caret` ▾/▸, mémorisé via `/settings/folder-open` (`folders.js:493`), dépli auto de l'ancêtre actif | OK | Glyphes ▾▸ au lieu des icônes RainLoop : P3 |
| `focused-poiner` (7) | Repère clavier | Voir ligne clavier ci-dessus | aucun | MANQUANT | P3 (regroupé) |
| `.name` (8) | Nom du dossier (indenté selon niveau) | Libellé | `base.html:211`, indentation `.f.depth-N` (`mail.css:80-81`, hauteur 2.429rem = 34 px) | OK | Rien |
| Système : `b-folder-system-item`, `is-inbox` (SystemItem 1-2) | Ligne des 6 dossiers système, libellés traduits (`localName`) | Noms localisés | `FolderName` (`accounts.go:76-95`), `base.html:204` ; ordre Réception, Envoyés, Brouillons, Indésirables, Corbeille, Archive (`foldertree.go:64-73`) | OK | Rien |
| Étoile « Boîte de réception favoris » `inbox-star-icon` (SystemItem 6-9) | Étoile à côté de INBOX : clic = filtrer les favoris (isInboxStarred) | Vue rapide des messages suivis | `base.html:204` `a.fstar` -> `/?q=is%3Aflagged`, icône pleine/vide, `mail.css:459` | OK | Rien |
| `&nbsp;` avant le nom (SystemItem 5) | Décalage pour l'icône | Alignement | `.f` padding `.714rem` | OK | Rien |
| Clignotement `anim-action-class` / `actionBlink` | Clignote quand un déplacement y arrive | Retour d'action | aucun | MANQUANT | Animation courte au retour d'un déplacement (classe posée depuis la requête). P3 |
| Quota d'occupation | (autre gabarit : pied de liste `e-quota`, voir §3) | Occupation boîte | `base.html:220` barre + texte dans le panneau | OK | Rien |

## 3. MailMessageList.html : barre d'outils principale

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `#rl-sub-left`, `.messageList` classes (1-3) `message-selected`, `loading`, `hideMessageListCheckbox` | Carte liste + états | Masquer cases selon réglage ; indicateur de chargement | `body.hide-checks` (`base.html:14`, `mail.css:466`) ; chargement : aucun (page rechargée) | PARTIEL | Pas d'indicateur de chargement (voir bandeau animé plus bas). P3 |
| Boutons mobiles panneau/gauche (6-18) | Bascules sur mobile | Navigation mobile | `@media (max-width:900px)` empile les volets (`mail.css:209-221`) | SANS OBJET | Variante mobile RainLoop (détection serveur) |
| Bouton Nouveau (mobile) (19-23) | Idem | Mobile | — | SANS OBJET | Mobile seulement |
| Bouton Recharger `buttonReload` (25-29) : icône spinner qui tourne pendant le chargement, `btn-dark-disabled-border` | Rafraîchit la liste | Recharger le dossier | `base.html:101` lien `Back` (recharge la page entière, ferme le message ouvert, pas d'animation) | PARTIEL | Recharger la liste sans fermer le message (fetch de la liste), icône qui tourne. P2 |
| Rafraîchissement automatique du dossier ouvert (RainLoop recharge la liste quand du courrier arrive) | Nouveaux messages apparaissent seuls | Courrier en temps quasi réel | `poll.js:42-72` ne met à jour que titre, notification et pastilles, pas la liste (toutes les 120 s) | PARTIEL | Si la poll détecte de nouveaux UID dans le dossier affiché en page 1, recharger la liste en fetch (ou afficher « n nouveaux messages — actualiser »). P1 |
| Menu « Déplacer vers » `buttonMove` (32-49) : bouton dossier + caret, icônes `copy`/`folder` selon Ctrl | Liste déroulante de TOUS les dossiers : système d'abord (noms localisés), séparateur, dossiers utilisateur indentés, dossier courant et non sélectionnables désactivés ; Ctrl = copier | Déplacer rapidement | `base.html:103-106` : `MoveMenu` = noms bruts triés dans l'ordre IMAP sans indentation, sans séparateur, sans noms localisés (`foldertree.go:165-169` : liste plate de `f.Name`) ; copie = menu séparé `base.html:107-111` | PARTIEL | Générer la liste depuis `Folders` du panneau : système d'abord (libellé `FolderName`), `<hr>`, puis arbre indenté (classes `depth-N`), non sélectionnables désactivés, dossier courant grisé ; accepter Ctrl+clic = copie. P1 |
| Menu « Déplacer » version `newMoveToFolder` (51-58) | Variante mobile : fenêtre | Mobile | — | SANS OBJET | Variante tactile |
| Groupe Archive / Spam / Pas spam / Supprimer (60-81) : visibilité par dossier (archive masquée dans Archive, spam masqué dans Indésirables/Envoyés/Brouillons, « pas spam » seulement dans Indésirables), boutons groupés `first/last` | Boutons selon le dossier | Actions de tri | `base.html:112-114` : archive/spam/corbeille toujours affichés, `data-needs-selection` grise sans sélection ; action `spam` dans Indésirables ne fait rien (`mail.go:1363`, `moveTo` renvoie nil si déjà dedans) | PARTIEL | (a) masquer Archive dans Archive, Spam dans Indésirables/Envoyés/Brouillons ; (b) bouton « Pas du spam » (icône coche cerclée) dans Indésirables = déplacer vers INBOX : ajouter l'action `notspam` (liste blanche `mail.go:1317`, cas `case "notspam": return moveTo(INBOX)`) + clé i18n `mail.act.notspam`. P2 |
| Info-bulles de ces boutons `tooltip: MESSAGE_LIST/BUTTON_*` | Libellés | Aide | `title`/`aria-label` (`base.html:112-114`) | OK | Rien |
| Séparateurs `&nbsp;` entre groupes (24, 30, 59, 82) | Espacement | Rendu | `.mb-mid {gap:.857rem}` + `.tgroup` (`mail.css:18-23,29`) | OK | Rien |
| Menu « Plus » `buttonMore` icône liste (83-149) | Menu d'actions secondaires | Actions de masse | `base.html:116-143` icône `menu` (trois traits) | OK | Icône : `icon-list` RainLoop = lignes avec puces ; cosmétique. P3 |
| Entrée « Marquer non lu » (89-95) | Action sur la sélection | Marquer non lu | `base.html:120` | OK | Rien |
| Entrée « Marquer lu » (96-102) | idem | Marquer lu | `base.html:119` | OK | Rien |
| Entrée « Marquer favori » (103-109) | idem | Favori | `base.html:121` | OK | Rien |
| Entrée « Retirer le favori » (110-116) | idem | Défavori | `base.html:122` | OK | Rien |
| Entrée « Tout marquer comme lu » (117-123) | Tout le dossier | Lire tout | `base.html:123` + `markAllRead` (`mail.go:1272`) | OK | Rien |
| Entrées désactivées sans sélection `disabled: !hasCheckedOrSelectedLines` | Grisées | Éviter les actions vides | `data-needs-selection` -> `disabled` (`list.js:46`) | OK | Rien |
| Entrée « Transférer plusieurs messages » `multyForwardCommand` (126-132) | Transférer N messages joints dans un seul message | Transfert groupé | aucun (`grep multy|forward` : seule action unitaire `forwardatt`) | MANQUANT | Action `forwardmulti` : composer avec N messages en pièces jointes `.eml` (réutiliser `forwardatt` de `compose.go` avec liste d'UID). P3 |
| Entrée « Supprimer sans déplacer » `deleteWithoutMoveCommand` (134-140), disponible par défaut (`capa.dangerous_actions = true`, `Application.php:254`) | Suppression définitive depuis n'importe quel dossier | Ne pas passer par la corbeille | aucun : `delete` hors corbeille déplace toujours (`mail.go:1366-1371`) ; seule la corbeille expunge | MANQUANT | Bouton « Supprimer définitivement » dans le menu Plus + `data-confirm` ; action `purge` = `c.Expunge`. P2 |
| Entrée « Vider le dossier » `clearCommand` (141-147) | Vide le dossier courant | Vidage | `home.html:54-67` lien « Vider la corbeille/les indésirables » uniquement pour ces deux dossiers (`mail.go:534-543`) | PARTIEL | Ajouter l'entrée au menu Plus pour tout dossier (confirmation avec nom + nombre). P2 |

## 4. MailMessageList.html : seconde barre, corps de liste, pied

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Barre `second-toolbar` (153-166) : recherche à droite, case « tout » à gauche | Rangée de 50 px | Recherche + sélection | `.lp-head` (`home.html:8-52`, `mail.css:94`), case à gauche, recherche `max-width:18.4rem` | OK | Rien |
| Champ de recherche `inputSearch` (158) : placeholder, Entrée lance, focus programmatique (touche `/`) | Recherche par mots-clés (from: subject: …) | Chercher | `home.html:22-23` ; `/` focus (`keys.js:377`) ; opérateurs côté serveur (`mailconn/search.go:70`) | OK | Rien |
| Croix `.close` pour annuler la recherche (157) | Efface la recherche en un clic | Sortir de la recherche | `home.html:49` lien « effacer » seulement dans le menu avancé ; croix native `type=search` du navigateur (vide le champ sans relancer) | PARTIEL | Croix cliquable dans le champ quand `Query` non vide, lien `ClearSearchURL` (`mail.go:624`). P2 |
| Bouton caret `buttonMoreSearch` (160-162) | Ouvre la recherche avancée | Recherche avancée | `home.html:24-50` `<details class="dropdown adv">` (menu déroulant, pas de fenêtre modale) | PARTIEL | Voir §10 (PopupsAdvancedSearch). P3 |
| Case « tout sélectionner » icône 3 états (164) `checkbox-unchecked/partial/checked` | Sélection globale avec état partiel | Cocher tout | `home.html:9` input natif + `indeterminate` (`list.js:42-43`) ; menu « sélection par critère » en plus (`home.html:10-19`) | PARTIEL | Fonction OK (et plus riche) ; rendu : case native claire au lieu des icônes carrées grises de RainLoop. Styler `appearance:none` (carré 14 px, bordure #999, coche) pour les cases `.chk input`. P2 |
| Bandeau de chargement `.line-loading` animé (167) | Barre qui défile pendant le chargement | Retour visuel | aucun | MANQUANT | Bande animée en haut de la liste pendant fetch/rechargement. P3 |
| `.mainDelimiter toolbarDelimiter` (168) | Trait sous la barre | Rendu | `.lp-head {border-bottom:1px solid #ddd}` | OK | Rien |
| `.listThreadUidDesc` « ‹ Retour à la liste » (172-176) | Quitte la vue d'une conversation | Conversations | `home.html:71` `.threadbar` lien « ‹ … » | OK | Rien |
| `.listSearchDesc` (177-179) | Bandeau « Recherche : … » sous la barre quand une recherche est active | Rappeler le filtre en cours | aucun (seul `filterbar` pour « non lus », `home.html:74`) | MANQUANT | Bandeau avec la requête et un lien « effacer », sur le modèle de `filterbar`. P2 |
| `.listDragOver` « Déposez le message ici » (180-184) | Zone de dépôt d'un fichier `.eml` | Importer par glisser | `emldrop.js:561-596` + `.eml-drop` (`mail.css:460`) | OK | Libellé `mail.eml_drop` (`fr.json:658`) |
| `.listClear` « Vider le dossier » (185-187), visible pour Indésirables/Corbeille, hors recherche | Lien centré | Vider | `home.html:54-67` `.listClear` + confirmation par menu (`mail.css:225-231`) | OK | Rien (masquer pendant recherche/conversation : RainLoop le fait `app.js:35113`) ; vérifier P3 |
| `.listError` (188-190) | Message d'erreur de liste | Erreur serveur | `home.html:68-69` `.msg.error` avec détail | OK | Rien |
| `.listEmptyList` / `listEmptyListLoading` / `listEmptySearchList` (191-200) | États vides distincts | Message adapté | `home.html:99` « Ce dossier est vide » / « Aucun message ne correspond » / « non lus » | OK | Pas d'état « Chargement… » : page rechargée. P3 |
| Zone `draggable: dragAndDronHelper` + `.messageListPlace` (201-203) | Liste glissable, image du glisser | Glisser | `dnd.js:262-287` (lignes `draggable`) ; pas d'image personnalisée (compte de messages) | PARTIEL | Image de glisser « N messages » (`setDragImage`). P3 |
| Pied `b-footer` (208-215) : % de quota `e-quota` + info-bulle, pagination à droite | Quota et pages | Pagination + quota | `home.html:102-107` : « n messages » à gauche + pagination `pager` ; quota dans le panneau dossiers (`base.html:220`) | PARTIEL | Pas de % de quota dans le pied de liste avec infobulle (déjà en barre dans le panneau) ; compteur total en plus : acceptable. P3 |
| Pagination `Pagenator` (213) | Pages 1 2 3 … | Navigation | `pageLinks` (`mail.go:150-186`), `.pager` (`mail.css:114-117`) | OK | Rien |

## 5. MailMessageListItem.html et MailMessageListItemNoPreviewPane.html (ligne de message)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.messageListItem` + `lineAsCss()` (1) | Classes d'état : non lu, sélectionné, coché, favori, joint, important, conversation, supprimé… | Style par état | `home.html:83` `unread selected answered` + `checked` posée par `list.js:49` | PARTIEL | Manque les états conversation (`hasUnseenSubMessage`, `hasFlaggedSubMessage`) : non lu / favori dans une conversation repliée ne se voit pas. P3 |
| `.sidebarParent` (2-4) barre verticale de 6 px à gauche | Couleur : non lu orange, coché bleu, sélectionné bleu foncé | Repérage | `home.html:84` `.sidebar` ; `mail.css:123,127,129,131` | OK | Rien |
| `.delimiter` (5) | Filet haut de ligne | Rendu | `home.html:85`, `mail.css:122` | OK | Rien |
| `.flagParent` flagOn/flagOnHalf/flagOff (7-17) : étoile à droite, demi-étoile si seule une sous-conversation est favorite | Basculer le favori au clic | Suivre | `home.html:87` bouton étoile (formulaire POST, `mail.go:1262-1266`), `.star.on` orange (`mail.css:139-141`) | PARTIEL | Le basculement recharge toute la page (la position de défilement est restaurée, `list.js:134-157`) au lieu d'un changement instantané : envoyer en fetch et basculer la classe. Pas de demi-étoile. P2 |
| `.dateParent` (18-20) : date format court + infobulle date complète | Date à droite | Datation | `home.html:88` `{{.Date}}` (`listDate`, `mail.go`) sans `title` | PARTIEL | Ajouter `title` avec la date complète (`formatFullDate` existe `mailtools.go:117`). P3 |
| `.threadsParent` nombre + flèche `right-mini` (21-28) | Nombre de messages de la conversation | Conversations | `home.html:93` pastille grise `.threads` (`mail.css:462`) | PARTIEL | Pastille ronde grise vs « n > » RainLoop ; ajouter le chevron. P3 |
| `.checkedParent` icône case (29-31) | Cocher | Sélection | `home.html:89` case native | PARTIEL | Voir styler cases (P2 §4) |
| `.senderParent` : icônes répondu/transféré (reply/forward), expéditeur (ou destinataires dans Envoyés), infobulle e-mails complets (32-37) | Qui | Identifier | `home.html:91` icônes `reply`/`forward` (visibles si `answered`, `mail.css:149-151`) ; destinataires dans Envoyés/Brouillons (`mail.go:586-600`) ; pas de `title` | PARTIEL | Ajouter `title` avec les adresses complètes (champ `FromFull`). P3 |
| `.attachmentParent` icône selon type (38-42, `attachmentIconClass`) | Trombone / image / pdf / archive… | Repérer les pièces jointes | `home.html:92` icône `image` ou `doc`, classe `kind-*`, infobulle type | PARTIEL | Un seul glyphe « doc » pour pdf, archive, audio…, vs icônes distinctes RainLoop ; ajouter symboles (pdf, zip, audio, tableur). P3 |
| `.subjectParent` : `!` rouge importance, sujet vide traduit, préfixe `Re:`/`Fwd:` grisé (43-46) | Objet | Lisibilité | `home.html:93` `.imp` rouge, `<i>sans objet</i>`, `.pfx` grisé (`mail.css:464`) | OK | Rien |
| Poignées `actionHandle dragHandle` (18, 32, 38, 43) | Seules ces zones lancent le glisser | Éviter glisser par erreur | `dnd.js:263` toute la ligne est `draggable` | PARTIEL | Acceptable ; P3 |
| Variante sans volet de lecture (`NoPreviewPane`, 1-49) : ligne sur une seule ligne (`e-single-line`), ordre date, fils, case, étoile, expéditeur, trombone, sujet | Disposition horizontale quand la lecture est « sans volet » | Réglage « liste seule » | `home.html:2` `.listonly` + même ligne en deux lignes (`ThreePane` faux) ; pas de mise en ligne unique | PARTIEL | Ajouter style `.mailbox.listonly .msgrow` sur une ligne (flex, sujet au centre). P3 |
| Hauteur/typographie de ligne (52 px, 12 px) | Densité | Fidélité visuelle | `mail.css:120` `.msgrow {height:3.714rem; font-size:.857rem}` | OK | Rien |
| Survol / sélection / focus clavier | États visuels | Rendu | `mail.css:125-131,429` | OK | Rien |

## 6. MailMessageView.html (volet de lecture)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `#rl-sub-right .messageView` classes `message-selected`, `message-focused` (1-2) | Carte de lecture | Volet de lecture | `home.html:113-117` `.readpane` + `.msgview` (`base.html:234`) | OK | Rien |
| Barre `top-toolbar` sans volet de lecture (3-43) : fermer, modifier (brouillon), archive/spam/pas-spam/supprimer, flèches précédent/suivant | Barre d'actions quand la lecture est une page | Agir et naviguer | Barre globale `mailbar` (`base.html:95-143`) ; fermer `mh-close` (`base.html:259`) ; pas de flèches précédent/suivant (`PrevURL/NextURL` calculés `mail.go:625-634` mais non affichés, `grep` : seul `contacts.html` les utilise) | PARTIEL | Afficher deux boutons ‹ › (message précédent/suivant de la liste) dans l'en-tête quand la lecture est pleine page ; `PrevURL`/`NextURL` existent déjà pour les pages de liste : étendre aux messages (uid voisin). P3 |
| Bouton « Modifier » vert crayon pour Brouillons (12-16, 214-218) | Rouvrir le brouillon en édition | Éditer un brouillon | `base.html:268` si `IsDraft` : icône `doc` + entrée « Modifier le brouillon » (`base.html:271`) | PARTIEL | Bouton vert avec crayon (classe `.btn-new`) pour reconnaître l'action ; actuellement icône document grise. P3 |
| `.b-message-view-backdrop` « Déposez pour déplacer » (46-53) | Voile quand on glisse des messages vers le volet | Aide au glisser | `dnd.js` (surbrillance dossier uniquement) | PARTIEL | Sans objet pratique (la cible est le dossier). P3 |
| `.b-message-view-checked-helper` (54-57) : « N ✉ » quand des messages sont cochés sans message ouvert | Rappel du nombre de messages cochés | Retour d'état | `list.js:45` `#select-count` (compteur) : aucun élément n'existe dans le gabarit actuel (`select-count` absent de `home.html`) | MANQUANT | Afficher « N sélectionné(s) » (`mail.selected` existe `fr.json:192`) dans le volet vide quand N > 0. P3 |
| `.b-message-view-desc.error` (59-61) | Erreur de chargement du message | Erreur | `home.html:115` `.msg.error.pad` (`PaneError`) | OK | Rien |
| Description vide `MESSAGE_VIEW_DESC` + logo/iframe (63-79) | Texte « Sélectionnez un message » | État vide | `home.html:116` `.pane-empty` `mailbox.placeholder` | OK | Logo/iframe d'accueil : SANS OBJET (option admin) |
| Barre flottante `.message-fixed-button-toolbar` (81-221) | Bouton de réponse + menu à droite du bandeau | Répondre vite | `base.html:267-284` `.replygroup` : bouton dernier mode utilisé + menu | OK | Rien |
| Menu caret `buttonMore` (84-199) | Menu d'actions | Actions de message | `base.html:269-283` | OK | Voir lignes suivantes |
| Entrée Répondre (91-97) | Répondre | Réponse | `base.html:272` | OK | Rien |
| Entrée Répondre à tous (98-104) | idem | Réponse à tous | `base.html:273` | OK | Rien |
| Entrée Transférer (105-111) | Transférer | Transfert | `base.html:274` | OK | Rien |
| Entrée Modifier comme nouveau (112-118) | Rouvrir pour renvoyer | Rééditer | `base.html:276` | OK | Rien |
| Entrée Transférer en pièce jointe (119-125) | Transfert `.eml` | Transfert joint | `base.html:275` | OK | Rien |
| Entrées Archive / Spam / Pas spam / Supprimer dans le menu (127-156, mode volet) | Actions dans le menu | Raccourci | Barre globale (`base.html:112-114`) | OK | Rien (voir §3 pour « pas spam ») |
| Entrée Se désabonner (157-163) | Lien List-Unsubscribe | Désabonnement | `base.html:288` bandeau `unsubbar` + `UnsubscribeURL` (`mail.go:873-886`) | OK | Bandeau au lieu d'entrée de menu : acceptable. Rien |
| Entrée Imprimer (168-174) | Version imprimable | Imprimer | `base.html:279` + `mailPrint` (`mailtools.go:70-135`) | OK | Voir §12 pour le contenu |
| Entrée « Dans une nouvelle fenêtre » (175-181) | Ouvre le message seul dans une fenêtre | Lire à part | aucun (impression automatique seulement) | MANQUANT | Lien `target=_blank` vers `/mail/message?folder=&uid=` (route existe `mail.go:947`). P3 |
| Entrée Voir l'original (183-189) | Source brute | En-têtes/source | `base.html:280-281` (en-têtes + source) | OK | Rien |
| Entrée Télécharger l'original (190-196) | `.eml` | Export | `base.html:282` | OK | Rien |
| Séparateurs du menu (128, 166, 182) | Rendu | Groupes | `<hr>` (`base.html:278`) | OK | Rien |
| Boutons répondre / répondre à tous / transférer selon `lastReplyAction` (200-211) | Le bouton principal mémorise le dernier mode | Gain de clics | `mv.ReplyMode` (`mail.go:869-874`), `rememberReply` (`compose.go:1072`), `base.html:268` | OK | Rien |
| Icône ellipse si pas de composition (86) | Cas sans composer | Cas rare | — | SANS OBJET | Composition toujours permise |
| En-tête `.messageItemHeader` classe `emptySubject` (224) | Bandeau gris de l'objet | Entête | `.mh` (`mail.css:158`) | OK | Rien |
| Bouton fermer × (226) | Ferme le message | Fermer | `base.html:259` `mh-close` + Échap (`keys.js:371-374`) | OK | Rien |
| Icône info (227-229) : ouvre « informationFull » | Détails des en-têtes | Voir tous les en-têtes | `base.html:237-248` `details.msg-info` (pastille « i » italique, `mail.css:160-165`) | OK | Rien |
| Étoile favori dans l'en-tête (230-232) | Basculer | Favori | `base.html:249-256` | OK | Rien |
| `!` rouge si important (233) | Priorité | Repérage | présent seulement dans la liste (`home.html:93`) | PARTIEL | Ajouter `Important` dans `MessageView` et un `!` devant l'objet de l'en-tête (`base.html:257`). P3 |
| Objet `.subject` + double-clic = plein écran, `emptySubjectText` (234-235) | Titre, plein écran au double-clic | Lecture plein écran | `base.html:257` `h1.msg-subject` ; bouton `mh-full` (`base.html:258`) ; objet vide « — » (`mail.go:865`) | PARTIEL | Ajouter l'écouteur `dblclick` sur `.msg-subject` / zone expéditeur (`msgfull.js`). P3 |
| Photo `.fromPic` (239) | Avatar de l'expéditeur (Gravatar/contact) | Reconnaître l'expéditeur | `base.html:262` `.avatar` icône générique | PARTIEL | Photo du contact s'il en a une (contacts-ac ne la gère pas : à vérifier) ; sinon initiale colorée. P3 |
| Ligne `from` + icône DKIM + date entre parenthèses (241-252) | « Nom (date complète) » | Qui / quand | `base.html:264` lien expéditeur + `(Date)` + `✔`/`⚠` DKIM + ajouter au carnet | PARTIEL | Icône DKIM = caractère vs icône colorée ; P3 |
| Ligne « À : » (254-259) | Destinataires courts | Voir les destinataires | `base.html:265` `ToShort` (3 premiers + …) | OK | Rien |
| Ligne « Cc : » (260-265) | Copie | Voir qui est en copie | aucune : `ToShort` seulement (`mail.go:895-902`), Cc uniquement dans le popup « i » | MANQUANT | Ajouter `CcShort` (même règle) et la ligne « Cc : » sous « À : » (`base.html:265`). P2 |
| Tableau `informationFull` (270-305) : De (+taille), À, Cc, Cci, Répondre à, Date (+ « il y a … ») | Détails | En-têtes complets | `base.html:238-247` De, À, Cc, Cci, Répondre à, Date, Taille, DKIM | PARTIEL | Manque la mention relative « (il y a 3 jours) » après la date (P3) |
| `.messageItem` zone défilante, fond selon corps `bodyBackgroundColor` (308-310) | Zone de corps | Lire | `iframe.mailbody` (`base.html:312`, `mail.css:206`) ; fond blanc fixe | PARTIEL | Fond blanc de l'iframe toujours ; RainLoop adapte au fond du message. P3 |
| Bouton `buttonUp` (315-317) | Remonter en haut | Long message | aucun (le volet `.readpane` défile entièrement) | MANQUANT | Petit bouton ↑ flottant dans le volet (JS 5 lignes). P3 |
| `buttonFull` / `buttonUnFull` (318-323) | Plein écran / quitter | Plein écran | `base.html:258` + `msgfull.js` + Échap ; touche Entrée mentionnée dans le commentaire de `msgfull.js:436` | OK | Rien |
| `.line-loading` + « Chargement… » avec points animés (325-329) | Indique le chargement du message | Retour visuel | aucun (le volet est remplacé après `fetch`, `list.js:197`) | MANQUANT | Afficher « Chargement… » dans `.readpane` au début de `openPane` (`list.js:186`). P3 |
| Bandeau « Afficher les images » (334-338) | Images distantes bloquées | Vie privée | `base.html:297` `imgbar` + « faire confiance à l'expéditeur » (`/mail/trust`) | OK | Rien |
| Bandeau « Notifier la lecture » (339-343) | Accusé de réception | MDN | `base.html:289-294` `mdnbar` envoyer/ignorer (`mail.go:880-885`) | OK | Rien |
| Liste de pièces `.attachmentsPlace` (344-393) : vignette, icône type + texte extension, nom, taille, coche de sélection, glisser vers le bureau (`draggable`), lecture audio (`hasPreplay`), œil au survol | Pièces jointes | Ouvrir/télécharger | `base.html:299-306` vignette, extension, nom, taille, œil au survol (`mail.css:485-488`), visionneuse (`preview.js`) | PARTIEL | Manquent : coche de sélection par pièce, glisser vers le bureau (`DownloadURL` dans `dragstart`), icône « play » pour l'audio (lecteur en ligne). P3 |
| Roue `controls-handle` + `attachmentsControls` (392-434) : ZIP des pièces cochées, enregistrer vers OwnCloud/Dropbox | Actions groupées | Télécharger tout | `base.html:304-305` roue -> « Télécharger tout en .zip » (`ZipURL` si > 1 pièce `mail.go:955`) | PARTIEL | ZIP = toutes les pièces (pas la sélection). OwnCloud/Dropbox : SANS OBJET. Cocher + zip sélection : P3 |
| `.bodyText` corps (437-438) | Corps assaini | Lire le corps | `iframe.mailbody` `sandbox` (`base.html:312`) + CSP | OK | Rien |

## 7. SystemDropDown.html (menu utilisateur)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Bouton `system-dropdown` : icône utilisateur / spinner + caret (5-14) | Ouvre le menu | Accès compte | `base.html:147` `tbtn` avec icônes `user` + `caret` | OK | Spinner pendant le chargement des comptes : sans objet (P3) |
| Libellé `accountPlace` (adresse e-mail) à gauche du bouton (83) | Affiche la boîte active | Savoir quelle boîte | `base.html:146` `.who` (fond noir translucide, `mail.css:31-32`) | OK | Rien |
| Liste des comptes avec coche sur le courant (17-33) | Changer de boîte | Multi-comptes | `base.html:150-153` boutons, `✓` sur l'actif | OK | Icône utilisateur devant chaque entrée : P3 |
| « Ajouter un compte » (35-44) | Ajoute une boîte | Ajouter | `base.html:157` lien « Comptes » (page de gestion), pas d'ajout direct | PARTIEL | Lien « Ajouter un compte » vers `/accounts/new` dans le menu. P3 |
| « Réglages » (45-51) icône roue | Réglages | Préférences | `base.html:156` | OK | Icônes devant chaque entrée du menu : cosmétique P3 |
| « Aide » (52-58) | Aide | Aide | `base.html:158` aide des raccourcis (touche ?) | OK | Rien |
| Séparateur (59) | — | — | `base.html:159` `<hr>` | OK | Rien |
| Versions mobile/bureau (60-73) | Bascule d'interface | Mobile | — | SANS OBJET | Détection mobile serveur |
| Déconnexion (74-80) icône power | Quitte | Logout | `base.html:160` formulaire POST + CSRF | OK | Rien |
| Zone audio `audioPlace` (84-92) | Lecteur audio des pièces jointes | Écoute | aucun | SANS OBJET | Lié au lecteur audio des pièces jointes (voir §6, P3) |
| Logo (94-97) | Logo personnalisé | Branding | — | SANS OBJET | Option d'admin |
| Raccourci backtick « menu utilisateur » | Ouvre le menu | Clavier | `keys.js:399` | OK | Rien |

## 8. About.html et PhotoSwipe.html

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| About : titre « RainLoop Webmail », version, lien rainloop.net (1-5) | Page « À propos » | Information produit | aucun | SANS OBJET | Spécifique au produit RainLoop (page de réglages « À propos ») |
| PhotoSwipe : conteneur `.pswp` plein écran, fond sombre (1-9) | Visionneuse d'images | Aperçu d'image | `preview.js:396-415` `dialog.viewer` plein écran fond `rgba(0,0,0,.94)` (`mail.css:489`) | OK | Rien |
| Compteur « 1 / n » (`pswp__counter`) (16) | Position dans la galerie | Se repérer | `preview.js:420` légende « nom (1/n) » | OK | Rien |
| Bouton Fermer (17) | Quitter | Fermer | `preview.js:404,409` `×` + Échap natif + clic sur le fond | OK | Rien |
| Bouton Plein écran (18) | Navigateur plein écran | Agrandir | aucun | MANQUANT | `dlg.requestFullscreen()`. P3 |
| Bouton Zoom (19) | Zoom image | Voir les détails | aucun (image bornée à 94 vw/88 vh) | MANQUANT | Clic = taille réelle / ajustée. P3 |
| Préchargeur `pswp__preloader` (20-26) | Indicateur de chargement | Retour visuel | aucun | MANQUANT | Texte « … » avant `onload` de l'image. P3 |
| Flèches précédent/suivant (31-32), clavier ← → | Parcourir | Galerie | `preview.js:405,411-413` boutons ‹ › + flèches clavier (boucle circulaire) | OK | Rien |
| Légende `pswp__caption` (33-35) | Nom du fichier | Identifier | `preview.js:420` | OK | Rien |
| Glisser tactile, partage (28-30) | Mobile | — | — | SANS OBJET | Tactile ; partage désactivé dans RainLoop |

## 9. PopupsCompose.html (fenêtre de rédaction) et ComposeAttachment.html

Remarque d'architecture : RainLoop ouvre la rédaction en fenêtre modale par-dessus la liste (réductible) ; wmail-go ouvre une page `/compose` (`compose.html:2`, `mail.css:253-255` reproduit la boîte blanche de 850 px sur le fond d'écran).

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.modal.b-compose` `loading` (2) | Fenêtre, voile pendant envoi/enregistrement | Retour visuel | `compose.html:2-3` formulaire ; bouton Envoyer désactivé pendant l'envoi (`compose.js:165`) | PARTIEL | Pas de voile « chargement » ; P3 |
| Bouton Envoyer vert `button-send` (4-7) : icône avion / spinner, états succès/erreur/avertissement avec info-bulle d'erreur | Envoi | Envoyer | `compose.html:16` bouton vert `btn-send`, Ctrl+Entrée (`compose.js:206`), avertissements (objet vide, pièce jointe oubliée, > 20 destinataires `compose.js:171-179`) | PARTIEL | Pas de spinner, ni couleurs d'état (la page est rechargée avec message) ; ajouter spinner sur le bouton quand `submitting`. P3 |
| Bouton Enregistrer (8-11) : disquette / spinner, rouge si erreur | Brouillon | Sauvegarder | `compose.html:17` `btn-save` ; fetch (`compose.js:182-198`) ; Ctrl+S ; auto-enregistrement 60 s (`compose.js:199`) | PARTIEL | Erreur affichée dans `#cp-status` (pas de bouton rouge) ; P3 |
| Croix de fermeture (13) | Ferme avec confirmation | Fermer | `compose.html:20` `cp-close` + confirmation si modifié (`compose.js:214-217`) | OK | Rien |
| Bouton Minimiser (14) | Réduit la fenêtre, reprend plus tard | Consulter le courrier pendant la rédaction | aucun | MANQUANT | Nécessite rédaction en volet/fenêtre ; alternative : ouvrir `/compose` dans un nouvel onglet (`target=_blank` sur le bouton Nouveau avec clic milieu). Pas d'effort recommandé. P3 |
| Bouton Supprimer rouge `button-delete` (16-18) | Abandonne le brouillon | Jeter | `compose.html:18` `btn-discard` rouge, à droite (`mail.css:260`) | OK | Rien |
| Texte « Enregistré à … » `saved-text` (19) | Heure de la sauvegarde | Retour | `compose.html:19` `#cp-saved` | OK | Rien |
| Ligne De (25-48) : liste déroulante des identités | Choisir l'expéditeur | Multi-identités | `compose.html:29-39` `<select>` identités, signature suivie (`compose.js:52-69`) | OK | Rien |
| Menu options (icône liste) (50-85) : accusé de lecture, DSN, important ; OpenPGP | Options d'envoi | Options | `compose.html:69-76` menu (icône `menu`) avec trois cases | OK | OpenPGP : SANS OBJET |
| Bouton Contacts (87-91) icône carnet | Choisir un destinataire | Carnet d'adresses | `compose.html:42` lien texte « Choisir… » (`contacts.pick`) ouvrant un dialogue (`contacts-ac.js`) | PARTIEL | Icône carnet à la place du lien texte, alignée à droite de la ligne « De » comme RainLoop. P3 |
| Liens Cc / Cci / Répondre à (94-105) | Révéler les champs | Champs facultatifs | `compose.html:33-37` (+ ouverts d'office si remplis, `compose.js:27-30`) | OK | Rien |
| Champ À avec pastilles + libellé rouge si vide (111-117) | Destinataires | Saisie adresses | `compose.html:42` + `chips.js` (pastilles, glisser entre champs, autocomplétion `contacts-ac.js`) | OK | Libellé rouge si vide non fait (erreur côté serveur) : P3 |
| Champs Cc, Cci, Répondre à (120-143) | Idem | Idem | `compose.html:44-55` | OK | Rien |
| Objet (144-151) | Sujet | Sujet | `compose.html:56-59` | OK | Rien |
| Bascule texte / pièces jointes (157-171) avec compteur et état d'erreur (rouge) | Onglets | Basculer | `compose.html:63-65` onglets icône document / trombone + compteur `#cp-count` (`compose.js:73-77`) | PARTIEL | Pas de rouge si une pièce a échoué (la ligne échouée seule est rouge `mail.css:297`) ; spinner dans l'onglet. P3 |
| Bouton « + 📎 » joindre rapidement (175-180) | Choisir des fichiers sans changer d'onglet | Joindre vite | aucun : il faut passer sur l'onglet puis « Joindre » (`compose.html:98`) | MANQUANT | Bouton trombone « + » à droite de la ligne des onglets déclenchant `#cp-file` (`compose.js:138`). P2 |
| Dropbox / Google Drive (181-190) | Pièces depuis le cloud | — | — | SANS OBJET | Services externes |
| Zone `attachmentAreaParent` : dépôt de fichiers `b-attachment-place`, liste, message « aucune pièce » (199-211) | Pièces jointes | Joindre | `compose.html:95-105` zone de dépôt (`.cp-drop`, `mail.css:288-289`), liste, message vide | OK | Rien |
| Zone de texte / éditeur (213-214) | Corps | Rédiger | `compose.html:81-93` éditeur riche + texte brut (`editor.js`) | OK | Barre d'outils détaillée hors tranche |
| ComposeAttachment : `li.attachmentItem` classes `waiting`, `error` (1-3) | Ligne de pièce | État d'envoi | `compose.js:80-90` `li.uploading/failed` | PARTIEL | Voir lignes suivantes |
| Icône de type + texte extension (5-11) | Icône de la pièce | Reconnaître | aucun (nom + taille seulement, `compose.js:83-86`) | MANQUANT | Pastille avec l'extension. P3 |
| Anneau/fond de progression `iconProgress`, `iconBG` texte « % » (12-13) | Progression | Suivre l'envoi | `compose.js:100-102` « n % » en texte à la place de la taille | PARTIEL | Suffisant ; P3 |
| Bouton annuler × (17) | Annule l'envoi/retire | Retirer | `compose.js:85,97,127-132` | OK | Rien |
| Nom tronqué + taille (18-23) | Libellé | Lecture | `.cp-atts .name` ellipsis (`mail.css:294`) | OK | Rien |

## 10. PopupsAdvancedSearch.html

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Fenêtre modale avec titre et croix (2-26) | Recherche avancée en fenêtre | Critères multiples | `home.html:24-50` menu déroulant `details.adv` sous le champ (`mail.css:101-106`) | PARTIEL | Menu au lieu d'une fenêtre à deux colonnes ; fonctionnel, rendu différent. P3 |
| Champs De, À, Objet, Texte (32-267) | Critères texte | Filtrer | `home.html:39-42` `adv_from`, `adv_to`, `adv_subject`, `adv_text` (`mail.go:260-280` les traduit en opérateurs) | OK | Rien |
| Sélecteur de date (270-285) : toutes, 3 jours, 7 jours, 1 mois, 3 mois, 6 mois, 1 an (`selectedDates`, `app.js:39416-`) | Période | Filtrer par date | `home.html:43` mêmes périodes (3d, 1w, 1m, 3m, 6m, 1y) | OK | Rien |
| Cases Non lus / Favori / Avec pièce jointe (290-311) | Filtres booléens | Filtrer | `home.html:44-46` | OK | Rien |
| Enter lance, Échap ferme (238, 247…) | Clavier | Rapidité | Enter = submit natif ; Échap ferme `menus.js:535-538` | OK | Rien |
| Bouton « Rechercher » icône loupe (318-322) | Lancer | Lancer | `home.html:48` bouton texte sans icône | PARTIEL | Ajouter l'icône loupe. P3 |
| Options supplémentaires de wmail-go (portée, non lus) | — | — | `home.html:26-37` | OK | Extension, pas d'équivalent RainLoop |

## 11. PopupsFolderClear / PopupsFolderCreate / PopupsFolderSystem

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Clear : titre « Vider le dossier » / « Vidage… » (7-8) | Fenêtre de confirmation | Vider | `home.html:56` `<summary>` + menu de confirmation (`mail.css:228-231`) | PARTIEL | Pas d'état « en cours » (page rechargée). P3 |
| Clear : avertissement rouge avec icône (13-22) texte « tous les messages de [dossier] seront supprimés (n) » | Message de danger avec nom du dossier | Éviter erreur | `home.html:58` `mail.clear.confirm` avec le nombre seulement (`fr.json:224`) | PARTIEL | Ajouter le nom du dossier dans le texte. P3 |
| Clear : bouton rouge « Vider » icône feu (26-30) | Confirme | Exécuter | `home.html:63` bouton `danger-btn` rouge | OK | Rien |
| Clear : applicable à tout dossier (via menu Plus) | Vider n'importe quel dossier | Vidage | Corbeille/indésirables seulement (`mail.go:534-543`) | PARTIEL | Voir §3 « Vider le dossier ». P2 |
| Create : titre, nom, dossier parent, bouton Créer icône dossier+ (39-72) | Créer un dossier | Gestion | `base.html:222-231` dialogue (nom, parent avec « racine » + options, Créer/Annuler) ; page `folders_settings.html:142-153` | OK | Rien |
| Create : Entrée valide (53) | Clavier | Rapidité | Formulaire natif (Entrée soumet) ; Échap ferme (`folders.js:519-521`) | OK | Rien |
| Create : liste parent indentée avec dossiers système (61-62) | Choix du parent | Hiérarchie | `base.html:228` `Key` des dossiers sélectionnables ou parents | PARTIEL | Noms bruts sans indentation ; mettre le libellé indenté. P3 |
| System : choix Envoyés, Brouillons, Indésirables, Corbeille, Archive (91-135) | Dossiers système | Associer les rôles | `folders_settings.html:119-132` (rôles + option auto, + « non utilisé ») | OK | Rien (plus riche) |
| System : message d'alerte `notification` (87-89) | Retour | Information | `messages` (`base.html:194`) | OK | Rien |

## 12. PopupsWindowSimpleMessage.html (impression / fenêtre du message)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Style de page : fond blanc, Arial 13 px, liens bleus (141-157) | Rendu papier | Imprimer lisiblement | `print.css:1` (12 pt Arial) | OK | Rien |
| Tableau `.tbl` lignes De, À, Cc, Cci, Répondre à, Date, Objet, Pièces jointes (219-252) | Entêtes imprimés | En-têtes complets | `mailtools.go:111-127` : De, À, Cc, Date, Pièces ; objet en `h1` | PARTIEL | Ajouter Cci (si présent) et Répondre à (`m.ReplyTo`/`Bcc`), mettre l'objet comme dernière ligne d'en-tête. P3 |
| Corps `text-body` html/plain, `pre` monospace (170-216) | Texte brut en monospace, HTML tel quel | Fidélité d'impression | `print.css:8-9` ; texte converti `TextToHTML` (`mailtools.go:101`) | PARTIEL | Citations en bleu / vert / rouge selon le niveau en texte brut (`print.css` : noir seul). P3 |
| Titre de la fenêtre / impression automatique | `window.print` à l'ouverture | Imprimer | `print.js:545-547` impression auto + bouton « Imprimer » | OK | Rien |

## 13. Login.html et LoginWelcome.html

Rappel : wmail-go a son propre identifiant (pas l'adresse e-mail IMAP) ; les gabarits `login.html`, `login_2fa.html`, `register.html`, `recover.html` sont comparés ici.

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-login-content`, `.loginFormWrapper` (1-2) | Boîte centrée sur le fond d'écran | Page de connexion DarkShine | `login.html:2-3` `.authpage/.authwrap` centré, `.authbox` fond `rgba(0,0,0,.5)`, rayon 7 px, ombre `0 2px 10px` (`theme.css rainloop` `--login-*`, `mail.css:233-249`) | OK | Rendu aligné sur `.thm-login` du thème (`out.css:18-28`) |
| Logo et description `logoImg`, `loginDescription` (160-168) | Image/texte au-dessus | Branding | `login.html:3` titre « wmail » texte | SANS OBJET | Options admin ; le titre texte (couleur `#ddd`, ombre) correspond à `.thm-login-desc` |
| Alerte d'erreur rouge avec × (170-177) | Message d'échec au-dessus du formulaire, animé | Retour sur erreur | `login.html:5` `messages` -> `.msg.error` pleine largeur au-dessus de la boîte | PARTIEL | Pas de bouton ×, ni d'animation « tremblement » (`errorAnimated`). P3 |
| Champ e-mail avec icône enveloppe à droite (188-197) : placeholder, 40 px, 18 px de texte | Identifiant | Saisie | `login.html:7-8` champ `Identifiant`, hauteur 2.86rem, police 1.29rem (`mail.css:244`) | PARTIEL | Icône à droite du champ (enveloppe/utilisateur) absente. Ajouter un `span.add-on` avec icône `user` et `padding-right`. P3 |
| Champ mot de passe avec icône clé qui devient flèche de validation quand rempli (199-214) | Validation rapide | Se connecter | `login.html:9-10` champ simple | PARTIEL | Icône clé ; flèche cliquable dès qu'il y a une saisie (CSS+JS court). P3 |
| Champ code de vérification (215-226) et case « ne plus demander » (227-236) | 2FA dans le même formulaire | Double authentification | `login_2fa.html:8-14` page séparée : champ `code` + case `trust` « 2 semaines » | OK | Autre chemin, même fonction |
| Bouton « Se connecter » `buttonLogin` (238-242) large, animé | Connexion | Valider | `login.html:11` bouton `.authbox button` pleine largeur (`mail.css:246`) | OK | Style « glass » (bordure blanche, texte majuscule) : non appliqué car DarkShine non glass ; vérifié `out.css:251` seulement sous `html.glass`. Rien |
| Boutons sociaux Google/Facebook/Twitter (244-260) | Connexion externe | — | — | SANS OBJET | Service externe |
| Bouton langue (261-264) `language-button` (ou sélecteur bas de page si social) | Choisir la langue | Langue | `login.html:17` liste de liens de langues sous le formulaire | OK | Rien |
| Case « Rester connecté » `signMe` (267-274) | Session longue | Ne pas se reconnecter chaque jour | aucune : cookie de session sans `MaxAge` (`server.go:354-357`) donc supprimé à la fermeture du navigateur ; durée serveur seule réglable en préférences (`session.go:57,106`) | MANQUANT | Case « Rester connecté » sur `login.html` : poser `MaxAge` = durée absolue et créer la session avec TTL long (`SetTTL`). P1 |
| Liens « Mot de passe oublié ? » / « Inscription » (277-292) | Liens sous le bouton | Récupération/inscription | `login.html:13-16` liens `recover` et `register` | OK | Rien |
| Bascule version mobile/bureau (295-310) | Interface mobile | — | — | SANS OBJET | Mobile |
| Sélecteur de langue en bas si connexions sociales (311-319) | Idem | — | — | SANS OBJET | Cas social |
| Hooks plugins `{{INCLUDE/...}}` (159, 169, 185, 237, 320) | Plugins | — | — | SANS OBJET | Plugins exclus |
| Animation `submitting` (183-184, voile + disque tournant) | Pendant la connexion | Retour visuel | aucune | MANQUANT | Voile sombre + désactiver le bouton à la soumission (JS 5 lignes). P3 |
| `LoginWelcome` : icônes Google / enveloppe (1-5) | Écran d'accueil (login social) | — | — | SANS OBJET | Social |
| `register.html`, `recover.html` (pages propres) | Création de compte local / récupération par clé | Gestion des comptes wmail | `register.html:2-20`, `recover.html:2-22` | SANS OBJET | N'existe pas dans RainLoop (propre à wmail-go) |

---

# À AJOUTER (MANQUANT / PARTIEL), par priorité

## P1 (quotidien, clairement visible)
1. **Menu « Déplacer vers » structuré** : lister les dossiers comme le panneau (système d'abord avec libellés `FolderName`, `<hr>`, puis arbre indenté `depth-N`), dossier courant et non sélectionnables grisés/désactivés, Ctrl+clic = copier. Fichiers : `foldertree.go:165-169` (renvoyer des `FolderRow` au lieu de `[]string`), `base.html:103-111` (menus déplacer/copier), `mail.css` (indentations `.menu button.depth-N`).
2. **Rafraîchissement automatique de la liste** : quand `poll.js` voit de nouveaux UID (`d.new`) dans le dossier affiché (page 1, tri date), recharger le contenu de `.msglist-form` en fetch (ou bandeau « n nouveaux messages » cliquable) sans fermer le message ouvert. Même mécanisme pour le bouton Recharger.
3. **Case « Rester connecté »** sur `login.html` : si cochée, poser le cookie `wmail_session` avec `MaxAge` = durée absolue de session et créer la session avec un TTL long (`session.SetTTL`, `server.go:354`) ; sinon cookie de session actuel. Clé i18n `login.stay_signed_in`.

## P2
4. **Compteurs de dossiers selon la règle RainLoop** : Brouillons = total des messages, Corbeille/Archive/Envoyés = pas de pastille, les autres = non lus (`base.html:204,211`, `foldertree.go`, et `poll.js:55-71`).
5. **Recharger sans fermer le message** : le bouton `base.html:101` doit recharger la liste en fetch (icône qui tourne), pas naviguer vers `List.Back`.
6. **Bouton « Pas du spam »** dans le dossier Indésirables (action `notspam` = déplacer vers INBOX, `mail.go:1317,1363`) et masquer Archive dans Archive, Spam dans Indésirables/Envoyés/Brouillons (`base.html:112-113`).
7. **Menu Plus : « Supprimer sans déplacer » et « Vider le dossier »** pour tout dossier, avec confirmation (nom du dossier + nombre) : actions `purge` (`c.Expunge`) et `clear` généralisée (`mail.go:534-543,1366-1371`, `base.html:116-143`).
8. **Ligne « Cc : »** dans l'en-tête du message sous « À : » (`MessageView.CcShort`, `mail.go:895-902`, `base.html:265`).
9. **Étoile favori sans rechargement** : envoyer en fetch (`/mail/action` avec `X-Requested-With`) et basculer la classe `.on` / icône sur place (`home.html:87`, `base.html:255`, `list.js`).
10. **Bandeau de recherche active** (`listSearchDesc`) avec la requête et « effacer », et croix cliquable dans le champ (`home.html:20-50`, `ClearSearchURL`).
11. **Cases à cocher stylées** (carré 14 px, bordure #999, coche, état partiel) pour `.chk input` et `#select-all` (`mail.css:134-136`).
12. **Bouton « + trombone »** dans la ligne des onglets de rédaction pour joindre sans changer d'onglet (`compose.html:63-65`, déclenche `#cp-file`).
13. **Bouton « configurer les dossiers »** du panneau : lien vers `/settings/folders` au lieu de `/settings` (`base.html:218`).

## P3
14. Flèches message précédent/suivant (`goUp/goDown`) quand le message est affiché en pleine page (`mail.go:625-634` à étendre aux messages, `message.html`).
15. « Ouvrir dans une nouvelle fenêtre » dans le menu du message ; Cci et Répondre à dans la version imprimable ; citations colorées en texte brut.
16. Transférer plusieurs messages en un seul (`forwardmulti`) ; ZIP des pièces sélectionnées avec coches ; glisser une pièce vers le bureau ; lecteur audio inline ; fond d'iframe adaptatif.
17. Rendu : `title` sur date complète et expéditeurs (adresses), chevron des fils, icônes de pièces jointes par type, demi-étoile conversation, états non lu/favori dans les conversations, `!` importance dans l'en-tête du message, mention « il y a … » dans les détails.
18. Visionneuse d'images : zoom, plein écran, indicateur de chargement.
19. Indicateurs de chargement : bandeau animé de liste, « Chargement… » du volet de lecture, voile « connexion en cours », animation d'erreur de connexion.
20. Icônes dans les champs de login (utilisateur / clé → flèche), bouton ×, entrées du menu utilisateur avec icônes, « Ajouter un compte » direct, icônes d'entrées de menu, barre de défilement fine, clic sur la colonne repliée pour rouvrir le panneau des dossiers, navigation clavier dans les dossiers, clignotement du dossier cible.
21. Rédaction : spinner et couleurs d'état sur Envoyer/Enregistrer, libellé « À » rouge si vide, icône carnet pour les contacts, onglet pièces jointes en rouge sur erreur, icône d'extension sur les pièces en cours d'envoi, minimiser (non recommandé).
