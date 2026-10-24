
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

- Lignes de tableaux (unités ou groupes d'unités analysés) : voir le décompte en fin de message de l'agent ; chaque ligne cite le code RainLoop lu et le code wmail-go lu (ou « aucun »).
- Points forts constatés : énumérations de composition, sélection par critère, glisser-déposer des messages vers les dossiers (avec dépliage au survol), pastilles d'adresses, sondage et notifications, pagination, pièces jointes (miniatures, zip, aperçu), dates de liste identiques aux formats RainLoop, quota, filtres, modèles, contacts.
- Principaux écarts visibles au quotidien : menu Déplacer/Copier brut (A1), pas de « Pas un spam » (A2), liens mailto: sortent de l'application (A3), pas de liens automatiques (A4), barres de défilement natives et infobulles natives (A6, A7), liste non rafraîchie automatiquement (A10).
