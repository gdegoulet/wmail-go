
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
