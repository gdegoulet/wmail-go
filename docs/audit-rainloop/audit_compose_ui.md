# Audit fonctionnel RainLoop 1.17.0 : composition de messages et plomberie de l'interface

Périmètre : lecture seule de `static/js/app.js` (non minifié, webpack dev), `libs.js`, `boot.js`, `static/ckeditor/` (plugins `plain`, `signature`, config), gabarits `app/templates`, `static/css/app.css`, `fr_FR.yml`. Aucun dossier `data/` lu, aucun code exécuté ni copié.

## Conventions de référence

- `A` = `/home/gdegoulet/Work/webmail_project/rainloop/1.17.0/static/js/app.js` (le numéro est la ligne du fichier).
- `L` = `.../static/js/libs.js` ; `B` = `.../static/js/boot.js`.
- `CKP` = `.../static/ckeditor/plugins/plain/plugin.js` ; `CKS` = `.../static/ckeditor/plugins/signature/plugin.js` ; `CKC` = `.../static/ckeditor/ckeditor.js` (minifié, vérifié par motifs).
- `T:` = `.../app/templates/` ; `CSS` = `.../static/css/app.css` ; `YML` = `.../app/localization/webmail/fr_FR.yml`.
- Réglages "serveur" = clés lues par `settingsGet` (réglage utilisateur) ou `appSettingsGet` (configuration applicative envoyée par PHP).

## Plan des sections

1. Composition : ouverture, types et champs d'adresse
2. Pastilles d'adresses (inputosaurus / emailsTags)
3. Éditeur riche CKEditor, mode texte brut, conversions
4. Pièces jointes, envoi de fichiers, glisser-déposer
5. Identités et signatures
6. Brouillon, enregistrement automatique, minimisation, fermeture, reprise
7. Envoi, avertissements, erreurs, en-têtes et options
8. OpenPGP dans la composition
9. Plomberie : Knoin (écrans, popups, routage), commandes, focus et raccourcis
10. Plomberie : mobile/bureau, disposition, redimensionnement, plein écran, défilement, infobulles, animations
11. Plomberie : notifications, titre, favicon, réseau, session, rafraîchissement, caches
12. Plomberie : thème, langue, impression, liens, accessibilité, démarrage
13. Fonctions absentes ou inertes constatées (utile pour une comparaison)

---

## 1. Composition : ouverture, types, champs

**F-CU-1 : Fenêtre de composition en popup modale** — La composition est la popup Knoin `View/Popup/Compose` (gabarit `PopupsCompose`), modale à fond statique (`data-backdrop="static"`, clavier Bootstrap désactivé). Largeur 850 px par défaut, 1000 px en états `ssm-state-desktop` et `desktop-large`, 720 px en tablette/mobile ; marge haute 10 px. Pendant un envoi ou un enregistrement la classe `loading` anime des rayures dans la barre d'outils (si animations actives). — A:12367, T:Views/User/PopupsCompose.html:1-3, CSS:7021-7080, 10076, 11465.

**F-CU-2 : Sept types de composition (ComposeType)** — `empty`, `reply`, `replyall`, `forward`, `forward-as-attachment`, `draft`, `editasnew`. Le type pilote destinataires, objet, corps, pièces jointes, en-têtes de chaînage et choix d'éditeur. — A:295, A:13180 (`initOnShow`).

**F-CU-3 : Points d'entrée de la composition** — Bouton "Nouveau message" (liste de dossiers, bouton `buttonCompose` de la liste mobile) ; raccourcis `W` ou `C` (listes et vue message, uniquement si la capacité Composer est active) ; boutons Répondre / Répondre à tous / Transférer / Transférer en pièce jointe / Modifier comme nouveau / Modifier le brouillon dans la vue message ; touche `R`, `A`, `F` ; `Shift+F` (transfert en pièces jointes de la sélection) ; liens `mailto:` ; sélection de contacts ; enregistrement du gestionnaire de protocole `mailto`. — A:34261, A:34749, A:34812, A:35215, A:35941, A:36339-36348, A:30907.

**F-CU-4 : Bouton "Nouveau message" reflète une composition en cours** — Le bouton passe en `btn-warning` (orange) quand `composeInEdit` est vrai, `btn-success` (vert) sinon, aussi bien dans la liste des dossiers que dans la barre mobile de la liste de messages. — T:Views/User/MailFolderList.html:3, T:Views/User/MailMessageList.html:20, A:8034.

**F-CU-5 : Réouverture d'une composition minimisée / fusion de contacts** — Si `composeInEdit` est vrai et que `onShow` reçoit un type `empty` (clic "Nouveau message", `mailto:`, retour de la popup Contacts), la composition existante est conservée : les adresses fournies sont ajoutées à Á/Cc/Cci (séparées par ", ", dédoublonnées entre elles) et l'objet n'est posé que s'il est fourni et que l'objet courant est vide. Le corps n'est pas touché. — A:13105-13130, A:13145 (`addEmailsTo`).

**F-CU-6 : Écrasement d'une composition en cours par un autre type** — Si `composeInEdit` est vrai et que le type n'est pas `empty` (Répondre, Transférer...), une boîte "Abandonner les données non enregistrées ?" s'affiche (focus laissé sur aucun bouton, `isFocusYesOnShow=false`) ; "Oui" relance `initOnShow` et remplace tout ; l'état précédent n'est pas sauvegardé. — A:13111-13117, YML:206-243 (`DISCARD_UNSAVED_DATA`).

**F-CU-7 : Préchauffage de la popup** — 500 ms après le démarrage de l'écran Boîte mail, `warmUpScreenPopup` construit la popup invisible (`onWarmUp` bascule l'éditeur en texte brut pour initialiser CKEditor tôt). Évite le délai de création à la première ouverture. — A:33864-33866, A:4216, A:13132.

**F-CU-8 : Champs de l'en-tête** — De (liste d'identités), À, Cc, Cci, Répondre à, Objet. Cc, Cci et Répondre à sont masqués par défaut ; chacun a un lien texte qui l'affiche (le lien disparaît une fois le champ visible) ; ils s'affichent aussi automatiquement dès que leur valeur devient non vide (brouillon, réponse à tous, identité avec Cci/Reply-To). — T:Views/User/PopupsCompose.html:77-120, A:12512-12532.

**F-CU-9 : Champ Objet** — Champ texte `textInput` lié à `subject`, `autocomplete=off`, correcteur du navigateur actif (pas d'attribut `spellcheck`). Prérempli en `Re:` / `Fwd:` selon le type. — T:Views/User/PopupsCompose.html:127-133.

**F-CU-10 : Normalisation du préfixe d'objet (Re / Fwd)** — `replySubjectAdd` collapse les espaces, retire les préfixes `RE:`/`FWD:` successifs (y compris `RE[2]`, `FWD(3)`) et reconstruit au plus un `Re:` et un `Fwd:` : "Re: Fwd: sujet" est possible, "Re: Re: Re:" n'existe pas. Réponse = `Re: ` seul ; transfert = `Fwd: ` seul (l'ancien `Re:` est conservé après `Fwd:` si présent). Pas de localisation du préfixe. — A:2232-2260.

**F-CU-11 : Focus initial** — Hors appareil mobile (UA android/iphone/ipod/ipad/blackberry), 100 ms après l'ouverture : focus sur À si vide, sinon dans l'éditeur (sauf si À a déjà le focus). Sur mobile aucun focus automatique (évite le clavier virtuel). — A:13417-13431.

**F-CU-12 : Réponse (Reply)** — Destinataire = `Reply-To` du message, sinon `From`, en excluant l'adresse du compte et celle de l'identité choisie (`excludeEmail`) ; si la liste est vide après exclusion, recalcule sans exclusion (répondre à soi-même). Objet `Re:`. `aDraftInfo = ['reply', uid, dossier]`, `In-Reply-To = Message-ID`, `References = In-Reply-To + References du message`. Seules les pièces jointes "liées" (images inline `cid:`) sont reprises. — A:13245-13253, A:25630-25655, A:13914-13919.

**F-CU-13 : Réponse à tous (ReplyAll)** — À = Reply-To ou From + tous les To d'origine ; Cc = Cc d'origine ; mêmes exclusions ; si À vide, repli sur la liste sans exclusion. `References` utilise `message.references()` (liste de références interne) au lieu de la valeur brute. — A:13254-13263, A:25656-25682.

**F-CU-14 : Transfert (Forward)** — Objet `Fwd:`, `aDraftInfo=['forward',...]`, `In-Reply-To`/`References` posés comme pour la réponse. Corps HTML : ligne vide, "-------- Message transféré -------", puis De / À / (CC si présent) / Envoyé (date au format moment `FULL`) / Objet (HTML-échappés) puis le corps d'origine. Toutes les pièces jointes sont reprises (`add=true`). — A:13265-13271, A:13321-13326.

**F-CU-15 : Transfert en pièce jointe (.eml)** — Objet `Fwd:`, corps vide ; le message d'origine devient une pièce `sujet.eml` (extension ajoutée si absente), id = `requestHash`, `fromMessage=true`, taille = taille du message. Avec une sélection de plusieurs messages (`Shift+F`, tableau de plus d'un élément), chaque message devient une pièce `.eml` et le corps reste vide. — A:13273-13279, A:13798-13808, A:13363-13380, A:34749.

**F-CU-16 : Modification de brouillon (Draft)** — Reprend À, Cc, Cci, Répondre à, Objet, pièces jointes (toutes), `aDraftInfo` d'origine s'il est valide (3 éléments), `In-Reply-To`/`References` d'origine ; mémorise `draftFolder`/`draftUid`, `bFromDraft=true` (permet de vider le panneau de lecture à l'enregistrement). L'identité est devinée d'après `From`/`Reply-To` du brouillon ; la signature n'est pas réinsérée. — A:13281-13294, A:13341, A:12901-12904.

**F-CU-17 : Modifier comme nouveau (EditAsNew)** — Reprend À/Cc/Cci/Répondre à/Objet/pièces jointes/chaînage mais sans lien avec le brouillon (pas de `draftUid`) ; pas de réinsertion de signature. — A:13296-13306.

**F-CU-18 : Bouton Répondre mémorisé** — La vue message mémorise la dernière action (Répondre, Répondre à tous, Transférer) dans le stockage client (`LastReplyAction`) pour l'action par défaut du bouton ; Modifier comme nouveau / Transférer en pièce jointe ne sont pas retenus. Répondre/Transférer sont désactivés dans le dossier Brouillons. — A:35641-35653, A:35670-35674, A:334 (`LastReplyAction`).

**F-CU-19 : Choix du format d'éditeur à l'ouverture** — Réglage `EditorDefaultType` : `Html` (défaut), `Plain`, `HtmlForced`, `PlainForced`. Pour une réponse/transfert : texte brut si `PlainForced`, ou si le message d'origine n'est pas HTML et que le réglage n'est pas `HtmlForced` ; sinon HTML. Pour un message vide / pièces `.eml` : HTML seulement si `Html` ou `HtmlForced`. Brouillons : la bascule suit les mêmes règles que les réponses. — A:13337-13339, A:13353-13355, A:13371-13373, A:416, A:36783-36798.

**F-CU-20 : Position des boutons de format riche** — L'utilisateur peut changer le format à tout moment par le bouton "HTML <-> Non-formaté" de la barre CKEditor ; l'état de la composition se rafraîchit via `onModeChange` (observable `isHtml`). — CKP:138-143, A:12999-13005.

---

## 2. Pastilles d'adresses (emailsTags, inputosaurus)

**F-CU-21 : Champs À/Cc/Cci/Répondre à en pastilles** — Chaque champ est un `input` transformé par le widget jQuery UI `inputosaurus` v0.1.6 (modifié RainLoop) via le binding `emailsTags` ; la valeur de l'observable est la liste des adresses jointes par ",". — A:1430-1491, L:1341-2018.

**F-CU-22 : Séparateurs de saisie** — `,` `;` et retour à la ligne. La saisie n'est découpée que si le texte (trimé) se termine par un séparateur (`splitHook`) ; sinon la saisie reste dans le champ. Entrée valide la saisie courante (si le menu d'autocomplétion n'a pas d'élément actif). `parseOnBlur` : la perte de focus valide aussi. — A:1438-1459, L:1539-1578.

**F-CU-23 : Analyse d'adresses** — `emailjs-addressparser` analyse les formats `Nom <a@b>`, `"Nom" <a@b>`, `a@b`, listes ; `splitEmailLine` reconstitue des chaînes `"Nom" <email>` ; les chevrons superflus (`<<a@b>>`) sont retirés ; un nom identique à l'adresse est vidé. — A:10562-10593, A:10478.

**F-CU-24 : Texte non reconnu supprimé silencieusement** — `parseHook` ne produit un objet que si l'analyseur reconnaît une adresse ; un élément sans objet (`[texte, null]`) est refusé par `_setChosen` (condition `a[1]`). Une chaîne qui n'est pas une adresse est donc effacée du champ à la validation, sans message d'erreur ; il n'y a aucune validation de domaine ni de syntaxe stricte côté client. — A:1460-1467, L:1798.

**F-CU-25 : Collage multi-adresses** — Un collage contenant plusieurs adresses (virgule, point-virgule, retour ligne) est traité par `splitHook`/`parseHook` et crée une pastille par adresse ; non dédoublonné contre l'autre champ ; dédoublonné dans le même champ sur la valeur texte (`allowDuplicates=false`). — A:1451-1467, L:1789-1798.

**F-CU-26 : Affichage des pastilles** — Texte court = nom si présent, sinon adresse ; infobulle `title` = `"Nom" <email>` ; croix `✖` pour supprimer ; le champ de saisie s'agrandit entre 50 et 500 px selon le texte (`fakeSpan`). — L:1849-1875, L:1616-1626.

**F-CU-27 : Édition d'une pastille** — Double-clic (ou `doubletap` tactile) sur une pastille la retire et replace sa valeur complète dans la zone de saisie (texte sélectionné) ; la position d'origine est mémorisée (`lastEdit`) pour réinsérer au même rang. — L:1655-1698, L:2004-2005.

**F-CU-28 : Navigation clavier entre pastilles** — Depuis la zone de saisie : `Backspace` ou `←` en début de champ focalise la dernière pastille. Sur une pastille : `Suppr`/`Backspace` la supprime, `Entrée` ou `E` l'édite, `←`/`→` naviguent, `↓` retourne à la zone de saisie. Clic dans le conteneur focalise la saisie. — L:1586-1609, L:1641-1653, L:1700-1755, L:2002-2008.

**F-CU-29 : Glisser-déposer de pastilles entre champs** — `allowDragAndDrop` : chaque pastille est `draggable` (retour animé 200 ms si dépôt invalide), chaque conteneur est `droppable`. Un dépôt retire la pastille de la source et l'ajoute à la cible (À ↔ Cc ↔ Cci ↔ Répondre à). — A:1447, L:1405-1421, L:1860-1869, L:1912-1933.

**F-CU-30 : Autocomplétion** — Source `Suggestions` (requête serveur avec abandon des requêtes précédentes) ; `minLength=1`, `autoFocus=true` ; les suggestions sont affichées sous forme `"Nom" <email>` ; la largeur du menu suit celle du conteneur ; la sélection insère et valide ; `focus` du menu n'écrit pas dans la saisie. Dépend de la capacité Contacts/serveur. — A:12831-12837, A:29235-29245, A:7460-7466, L:1481-1531.

**F-CU-31 : Focus piloté** — `to.focused`, `cc.focused`, `bcc.focused` (observables) pilotent le focus des widgets ; le dernier champ focalisé parmi À/Cc/Cci est retenu (`sLastFocusedField`) pour l'import de contacts. — A:12431-12450, A:1439-1478.

**F-CU-32 : Valeur synchronisée en continu** — L'observable est rafraîchie à chaque ajout/suppression de pastille (`change`) ; un changement externe de l'observable (ouverture, identité) redessine les pastilles (`refresh`) sans doublons ; les attributs `autocomplete/autocorrect/autocapitalize=off`, `spellcheck=false` sont posés. — A:1468-1490, T:Views/User/PopupsCompose.html:102-133.

**F-CU-33 : Bouton Carnet d'adresses** — Bouton "Contacts" (visible si les contacts sont autorisés) : minimise la composition (`skipCommand`, enregistrement du brouillon inclus), puis 200 ms après ouvre la popup Contacts en mode sélection avec le champ visé (À/Cc/Cci). La popup Contacts rappelle ensuite la composition (`ComposeType.Empty` + listes) qui fusionne les adresses dans le champ visé. — A:12779-12789, A:30885-30912, T:...PopupsCompose.html:71-77.

**F-CU-34 : Contrôle des destinataires avant envoi (minimal)** — Seule vérification client : au moins un des champs À, Cc ou Cci est non vide ; sinon l'étiquette "À" passe en rouge (`error-to`) avec info-bulle "Merci de spécifier au moins un destinataire". La syntaxe des adresses et les destinataires invalides sont contrôlés côté serveur (erreur 303 "Destinataires invalides"). — A:12691-12693, A:12490-12493, A:619, YML:640+ (`INVALID_RECIPIENTS`).

---

## 3. Éditeur CKEditor, mode texte brut, conversions

**F-CU-35 : Éditeur CKEditor 4 en mode "classique" (iframe)** — Créé par `CKEDITOR.appendTo` ; `enterMode = BR`, `shiftEnterMode = P` (Entrée insère `<br>`, Maj+Entrée crée un paragraphe) ; `allowedContent: true`, `fillEmptyBlocks:false`, `ignoreEmptyParagraph:true`, vérificateur orthographique natif du navigateur actif, CKEditor forcé compatible (`env.isCompatible=true`), titre et feuille de style désactivés. — A:3584-3615, A:18462-18470.

**F-CU-36 : Chargement différé et langue de l'éditeur** — CKEditor est chargé après l'application (`StaticEditorJsLink`) ; `HtmlEditor.init` attend `window.__initEditor`. La langue vient du réglage `Language` via la table `htmlEditorLangsMap` (29 correspondances, ex. `fr_fr→fr`, `pt_br→pt-br`, `zh_tw→zh`) ; défaut `en`. — B:10358-10362, A:3630-3660, A:18464, A:18539-18543.

**F-CU-37 : Barre d'outils : groupes** — `spec`, `styles`, `basicstyles` (groupes basicstyles/cleanup/bidi), `colors`, `paragraph` (list, indent, blocks, align : vide sur appareil mobile), `links`, `insert`, `document` (mode, document, doctools), `others`. Le build CKEditor est un preset "basic" avec les plugins `about, autolink, base64image, basicstyles, bidi, blockquote, clipboard, colorbutton, colordialog, divarea, enterkey, entities, floatingspace, font, image, image2, indentlist, link, list, maximize, pastebase64, quicktable, removeformat, sourcearea, toolbar, undo, wysiwygarea` + `plain`, `signature`. — A:3584-3615, `.../ckeditor/build-config.js:48-80`, `.../ckeditor/config.js`.

**F-CU-38 : Barre d'outils : liste effective des boutons** — Bouton `plain` (bascule HTML/texte brut, groupe `spec`) ; Police et Taille (liste de polices CKEditor par défaut ; tailles 10, 12, 13, 14, 16, 18, 20, 24, 28, 36, 48 px ; libellés par défaut Arial et 13) ; Gras, Italique, Souligné ; Supprimer la mise en forme (RemoveFormat) ; Direction LTR/RTL (uniquement si le réglage applicatif `allowHtmlEditorBitiButtons` ; sinon le plugin `bidi` est retiré) ; Couleur du texte et Couleur de fond (sans option "Automatique", avec "Plus de couleurs" via `colordialog`) ; Liste numérotée, Liste à puces ; Diminuer / Augmenter le retrait ; Citation (Blockquote) ; Lien et Supprimer le lien ; Image base64 (plugin `base64image`) ; Tableau (plugin `quicktable`, grille de sélection + "plus"). Bouton Source seulement si le réglage `allowHtmlEditorSourceButton`. — A:3584-3615, A:18447-18460, CKC (addButton motifs), CKP:138-143.

**F-CU-39 : Boutons retirés explicitement** — `Format, Undo, Redo, Cut, Copy, Paste, Anchor, Strike, Subscript, Superscript, Image, SelectAll, Source` (Source réactivé par réglage) et le plugin `liststyle`. Les boutons `Maximize` (groupe `tools`) et `About` (groupe `about`) existent dans le build mais leurs groupes ne sont pas dans `toolbarGroups` : ils ne sont donc pas affichés. Aucun plugin d'alignement (`justify`), de règle horizontale, de caractères spéciaux ni de recherche. Les annuler/rétablir restent disponibles par raccourcis clavier CKEditor (Ctrl+Z / Ctrl+Y) car le plugin `undo` est chargé. — A:3584-3615, CKC.

**F-CU-40 : Onglets de dialogue retirés** — `link:advanced`, `link:target`, `image:advanced`, `images:advanced` ; les liens n'ont donc pas de choix de cible dans l'éditeur. — A:3607.

**F-CU-41 : Menu contextuel allégé** — À `instanceReady`, les entrées "Couper / Copier / Coller" du menu contextuel CKEditor sont supprimées ; en mode texte brut, la classe `cke_enable_context_menu` laisse le menu natif du navigateur. — A:18519-18527, CKP:121-123.

**F-CU-42 : Touche Tab** — Les événements `key` avec Tab sont annulés (`return false`) : Tab ne se comporte pas comme une indentation dans l'éditeur. — A:18472-18478.

**F-CU-43 : Collage d'images** — Plugin `pastebase64` : une image collée est insérée en `<img src="data:...">` ; Le serveur transforme les images `data:image/...;base64` du corps HTML en pièces inline (CID) à l'envoi (vérifié dans `Actions.php:5948-5963`). — CKC, `.../app/libraries/RainLoop/Actions.php:5948`.

**F-CU-44 : Dépôt d'image dans l'éditeur** — Si `FileReader` existe, l'événement `drop` de CKEditor lit la première image déposée (`file.type` `image/*`) en data URL, remplace un marqueur `[img=id]` par `<img src="…" />`. Cas limite : en pratique le survol d'un fichier sur la fenêtre bascule la composition sur l'onglet Pièces jointes (voir F-CU-61) donc cette voie est rarement atteinte. — A:18496-18517, A:13693-13696.

**F-CU-45 : Insertion d'image base64 par dialogue** — Le plugin `base64image` (dialogue `base64image.js`) permet d'insérer une image locale convertie en base64 ou une URL externe, sans traitement serveur. — `.../ckeditor/plugins/base64image/README.md`, CKC.

**F-CU-46 : Liens automatiques** — Plugin `autolink` : détection de liens/e-mails tapés dans l'éditeur WYSIWYG. — CKC (plugin `autolink`).

**F-CU-47 : Bascule HTML <-> texte brut** — Bouton `plain` : `setMode('plain')` crée un `<textarea class="cke_plain">` (100 % de large/haut, sans redimensionnement, `tab-size 4`, `dir=ltr`) ; état du bouton `ON` en mode brut. Aucune confirmation n'est demandée (le libellé `EDITOR/TEXT_SWITCHER_CONFIRM` existe dans les traductions mais n'est référencé nulle part). Le focus est repris et le curseur placé en tête. — CKP:81-158, YML:421-424.

**F-CU-48 : Conversion HTML → texte brut (`htmlToPlain`)** — Retire `<p></p>` vides ; `<pre>` conservé avec sauts de ligne ; espaces blancs écrasés ; `<br>` → `\n` ; fin de `h1..h6`/`p` → saut simple/double ; `<ul>` + `<li>` → ` * ` ; `<hr>` → ligne de 31 `_` ; divs aplaties ; `blockquote` → préfixe `> ` imbriqué (marqueurs `__bq__`, boucle limitée à 800 itérations) ; liens `<a>` → texte seul (l'URL est perdue) ; entités `&nbsp; &quot; &gt; &lt; &amp;` décodées ; coupure automatique des lignes à 100 caractères sur espace/saut de ligne (`splitPlainText`). Le plugin dispose d'un repli plus simple (`simpleHtmlToPlain`) si la fonction globale manque. — A:2624-2719, A:2110-2130, A:2722, CKP:30-52.

**F-CU-49 : Conversion texte brut → HTML (`plainToHtml`)** — Normalise `\r`, regroupe `>> >` en `>>>` ; convertit les lignes préfixées `>` en `<blockquote>` imbriqués (itérations successives) ; échappe `&`, `<`, `>` ; `\n` → `<br />` ; avec le drapeau `findEmailAndLinksInText` (utilisé par l'éditeur), passe par Autolinker. — A:2721-2780, A:2605-2620, CKP:62-70.

**F-CU-50 : Autolinker sur texte brut** — Liens `http(s)://` et adresses e-mail transformés en `<a>` avec `newWindow:true`, `stripPrefix:false`, sans mention/téléphone/hashtag ; les URL ne commençant pas par `http` (ex. `www.`, `ftp:`) sont ignorées (`replaceFn` renvoie false). — A:2605-2620.

**F-CU-51 : Contenu envoyé** — En HTML : `getData(true)` enveloppe le corps dans `<div data-html-editor-font-wrapper="true" style="font-family: arial, sans-serif; font-size: 13px;">…</div>` ; en texte brut : texte brut du textarea (`getRawData`), `TextIsHtml=0`. L'attribut wrapper est retiré lors d'une réponse (`clearBqSwitcher`). — A:18333-18351, A:2478-2483, A:12731.

**F-CU-52 : Nettoyage au chargement** — `setHtml` supprime `<p></p>` vides, remet à zéro le cache de signature, force le mode HTML ; `setPlain` force le mode texte brut ; `HtmlEditor.clear` = `setHtml('')`. — A:18394-18438, A:18594.

**F-CU-53 : Redimensionnement de l'éditeur** — L'éditeur occupe la hauteur restante de la fenêtre (binding `initResizeTrigger` : hauteur min 200 px, marge 40 px ; zone pièces jointes marge 57 px) ; mises à jour déclenchées par l'ouverture des champs Cc/Cci/Répondre à, l'ajout de pièces, `window.resize.real` (immédiat et débouncé 50 ms) et un minuteur de 5 s tant que la popup est visible. — A:1198-1228, A:12630-12638, A:13503-13519, T:...PopupsCompose.html:171-188.

**F-CU-54 : Contrôle de la perte/prise de focus** — L'éditeur appelle `onBlur` après 200 ms (annulé si le focus revient) : utilisé par les autres écrans (signature d'identité) pour synchroniser leur observable ; la composition ne s'en sert pas. Changement de mode → `blurTrigger` + `onModeChange`. — A:18250-18271, A:18480-18490, A:734-762 (binding `editor`).

---

## 4. Pièces jointes, envoi de fichiers, glisser-déposer

**F-CU-55 : Onglets Éditeur / Pièces jointes** — Deux boutons radio : icône document (éditeur) et trombone avec compteur (nombre total) ; le trombone devient rouge (`btn-danger`) s'il y a au moins une pièce en erreur, et affiche un spinner tant qu'une pièce est en cours de transfert. Une info-bulle d'erreur résume le problème. — T:...PopupsCompose.html:135-160, A:12494-12511, A:12539-12562.

**F-CU-56 : Bouton "Joindre des fichiers" (+ trombone)** — `Jua` v1 (module interne de `libs.js`) crée un champ `<input type=file multiple>` invisible superposé au bouton (`clickElement`). Visible seulement si le téléversement est supporté. — A:13670-13683, A:13771, L:2059 (Jua), T:...PopupsCompose.html:164-170.

**F-CU-57 : Paramètres de téléversement** — Nom du champ `uploader`, action `Links.upload()`, file d'attente de 2 transferts simultanés, `multipleSizeLimit` 50 fichiers par sélection (au-delà, l'événement `onLimitReached` n'est pas géré, les fichiers excédentaires sont ignorés sans message), téléversement XHR/FormData ; repli iframe pour anciens navigateurs. — A:13676-13683, L:2059.

**F-CU-58 : Limite de taille côté client** — `AttachmentLimit` (réglage serveur, octets) ; si la taille annoncée dépasse la limite, la pièce apparaît immédiatement en erreur "Le fichier est trop volumineux" (`complete=true`, `error`) et n'est pas transférée ; la limite vaut aussi pour Dropbox et Google Drive. — A:13713-13729, A:13833-13841, A:13868-13876, YML:413+.

**F-CU-59 : États et progression d'une pièce** — `ComposeAttachmentModel` : `waiting`, `uploading`, `complete`, `error`, `progress` (0-100, affichée 100 % dès 98 %), `tempName`, `enabled`, `fromMessage`, `isInline`, `isLinked`, `CID`, `contentLocation`. Icône selon extension/mime (mime déduit de l'extension via `Common/Mime`, `winmail.dat` → `ms-tnef`), taille formatée (B, KB, MB, GB), titre = erreur sinon nom. — A:23698-23818, A:13699-13712.

**F-CU-60 : Annulation d'une pièce** — Le bouton "×" appelle `cancel` : retire la pièce de la liste, détruit l'objet et appelle `Jua.cancel(id)` (abandon XHR). — A:13650-13668, T:Views/User/ComposeAttachment.html:12.

**F-CU-61 : Zone de dépôt de fichiers** — Un fichier glissé au-dessus de la fenêtre (`onBodyDragEnter` sur le document) fait afficher l'onglet Pièces jointes et la zone "Glisser les fichiers ici" ; la zone passe en surbrillance (`dragAndDropOver`) au survol ; `onSelect` après dépôt. Hors de la zone, les dépôts de fichiers sont neutralisés (`dropEffect=none`). Il y a donc un second consommateur du même événement dans la liste des messages (dépôt de `.eml`, F-CU-100). — A:13689-13698, L:2059 (Jua), T:...PopupsCompose.html:161-181.

**F-CU-62 : Retour serveur d'un téléversement** — `onComplete` : `ErrorCode` → `getUploadErrorDescByCode` (messages : trop volumineux, partiel, aucun fichier, dossier temporaire manquant, erreur d'enregistrement, type invalide, inconnue) ; absence de `Attachment` → "erreur inconnue" ; succès → `initByUploadJson` (nom, taille, `tempName`, `isInline=false`). Timeouts par défaut de 30 s ne s'appliquent pas au Jua (XHR propre). — A:13745-13770, A:4655-4689, YML:413-420.

**F-CU-63 : Pièces issues d'un message** — Transfert/brouillon/modifier comme nouveau : les pièces d'origine sont déclarées (`fromMessage=true`, `waiting=false`, `uploading=true`, `complete=false`) puis `MessageUploadAttachments` (timeout 999 s) copie côté serveur et renvoie les noms temporaires (`tempName`) ; en cas d'échec toutes les pièces d'origine passent en erreur. Les pièces inline CID/location sont réémises avec leur CID. — A:13385-13415, A:13902-13939, A:13952-13958, A:6997-7002.

**F-CU-64 : Dropbox** — Bouton visible si `AllowDropboxSocial` est actif (script Dropbox chargé à la construction) ; sélecteur `Dropbox.choose` (lien direct, sélection unique) ; le serveur télécharge le lien (`ComposeUploadExternals`, timeout 999 s). — A:13791-13807, A:13832-13859, A:7027-7032, T:...PopupsCompose.html:171.

**F-CU-65 : Google Drive** — Visible si XHR + `AllowGoogleSocial` + `AllowGoogleSocialDrive` + `GoogleClientID` + `GoogleApiKey` et si l'API `apis.google.com/js/api.js` a chargé ; sélecteur Picker (vue Documents, `drive.readonly`, jeton obtenu en silencieux puis interactif). Les documents Google sont exportés : Docs → .docx, Sheets → .xlsx, Dessin → .png, Présentation → .pptx, sinon PDF. Transfert serveur `ComposeUploadDrive` avec jeton d'accès ; la taille serveur remplace la taille annoncée. — A:12657-12661, A:13507-13513, A:13522-13637, A:13867-13895, A:7039-7045.

**F-CU-66 : Pièces envoyées/enregistrées** — Seules les pièces complètes, avec `tempName` non vide et `enabled=true` sont transmises, sous la forme `{tempName: [nom, '1'|'0' (inline), CID, contentLocation]}`. Une pièce encore en cours ou en erreur est exclue d'un enregistrement de brouillon mais bloque l'envoi. — A:13782-13792, A:12683-12689.

**F-CU-67 : Fonction morte** — `removeLinkedAttachments` (retrait des images liées) n'est jamais appelée. — A:13941-13950.

---

## 5. Identités et signatures

**F-CU-68 : Sélecteur d'expéditeur (De)** — Menu déroulant affichant `Nom (email)` (ou l'e-mail) ; n'est un vrai menu que s'il y a plus d'une identité ; la liste vient de `IdentityUserStore` (identité principale d'identifiant vide + identités supplémentaires, capacité `Identities`). La valeur courante est transmise comme `IdentityID`. — T:...PopupsCompose.html:33-52, A:12590-12616, A:33039-33043.

**F-CU-69 : Ouverture du menu des identités au clavier** — `` ` `` (touche accent grave) ouvre le menu si ni l'éditeur ni un champ n'a le focus ; `Ctrl+`` ` l'ouvre toujours (portée Compose). — A:13460-13473, T:Views/Common/PopupsKeyboardShortcutsHelp.html (onglet Écrire).

**F-CU-70 : Choix automatique de l'identité** — Pour Répondre/Répondre à tous/Transférer : première identité (par ordre de liste) dont l'adresse figure dans les destinataires To/Cc/Bcc du message ; sinon dans `Delivered-To` ; pour un brouillon : From/Reply-To ; pour un message vide : identité n°1. Repli final : première identité. (La clé de stockage `ComposeLastIdentityID` est déclarée mais jamais utilisée.) — A:12865-12910, A:334-345.

**F-CU-71 : Changement d'identité : Cci et Reply-To suivis** — À chaque changement, l'ancien Cci/Reply-To de l'identité est retiré des champs (`fEmailOutInHelper` filtre l'adresse) puis ceux de la nouvelle sont ajoutés ; Les champs Cci/Reply-To s'ouvrent s'ils deviennent non vides. — A:12392-12407, A:12602-12610.

**F-CU-72 : Remplacement de la signature au changement d'identité** — `selectIdentity` pose l'identité et recalcule la signature. La commande `insertSignature` retire l'ancienne signature uniquement si elle est reconnue intacte : en HTML, comparaison du texte nettoyé de chaque bloc `<signature>…</signature>` ; en texte brut, suppression exacte de la chaîne (jusqu'à 2 occurrences). Si la signature a été modifiée ou introuvable (`skipInsert`), la nouvelle n'est pas insérée. — A:12912-12917, CKS:8-70.

**F-CU-73 : Insertion de la signature** — Signature précédée de deux sauts de ligne (`<br />` en HTML) et, en HTML, balisée `<signature>…</signature>` ; insérée à la fin du corps ou avant lui selon `signatureInsertBefore` de l'identité. Mémorisée comme "signature précédente" seulement si sa longueur dépasse 10 caractères. Le cache est vidé à chaque `setHtml`/`setPlain`. — CKS:70-90, A:18285-18312.

**F-CU-74 : Signature HTML ou texte** — Une signature stockée avec le préfixe `:HTML:` est traitée comme HTML ; sinon texte brut. La conversion croisée (HTML ↔ texte) suit le mode courant de l'éditeur (`htmlToPlain`/`plainToHtml`). — A:13075-13093, CKS:110-130.

**F-CU-75 : Jetons dynamiques de signature** — `{{DATE}}` (format moment `llll`), `{{TIME}}` (`LT`), `{{MOMENT:format}}` (jusqu'à 10 occurrences, format moment libre), `{{FROM}}` (nom, ou partie locale de l'adresse, de l'expéditeur du message cité), `{{FROM-FULL}}` (expéditeur du message cité tel que `toLine(friendly=true)` : le nom si présent, sinon l'adresse ; `{{FROM}}` en est dérivé en retirant `@domaine` quand la ligne ne contient ni espace ni nom). Sans message cité, `{{FROM}}`/`{{FROM-FULL}}` et les 1-2 espaces qui les précèdent sont supprimés ; les `\r` sont retirés. — A:13013-13073.

**F-CU-76 : Signature non appliquée selon le type** — Pas de signature pour les brouillons (`draft`) et les `editasnew` ; appliquée pour reply/forward/empty/ pièces .eml. — A:13341-13343, A:13357-13359, A:13375-13377.

**F-CU-77 : Édition des identités (hors composition)** — Popup d'identité : e-mail validé, nom, Répondre à et Cci (validation `validateSimpleEmail`), signature (éditeur CKEditor lié), case "insérer avant le texte". L'identité principale a un `id` vide, son e-mail n'est pas modifiable. — A:27757-27903, A:33008-33046.

---

## 6. Brouillon, enregistrement automatique, minimisation, fermeture, reprise

**F-CU-78 : Bouton Enregistrer** — Visible seulement si la capacité Dossiers est active ; enregistre dans le dossier Brouillons système ; si aucun dossier Brouillons n'est défini, ouvre la popup de choix des dossiers système (notification `Draft`). Pendant l'action : spinner, `saving`, bouton verrouillé. Dossier et UID sont renvoyés et mémorisés ; le brouillon remplaçant l'ancien (identifié par `MessageFolder`/`MessageUid`). — A:12736-12752, T:...PopupsCompose.html:12-16.

**F-CU-79 : Informations d'enregistrement** — "Enregistré à HH:mm" (`COMPOSE/SAVED_TIME`, masqué sur mobile) ; en cas d'échec le bouton Enregistrer devient rouge avec une info-bulle ("Impossible d'enregistrer le message"). Ctrl/Cmd+S enregistre aussi (si la capacité Dossiers est active). — A:12484-12489, A:12946-12978, A:13480-13486.

**F-CU-80 : Enregistrement automatique** — Réglage `AllowDraftAutosave` (case à cocher dans Paramètres > OpenPGP/Sécurité : gabarit `SettingsOpenPGP.html:28`, défaut vrai). Deux déclencheurs cumulés : minuteur propre de 60 s relancé à chaque ouverture/enregistrement/autosave, et abonnement à l'événement global `interval.2m`. Conditions : popup visible, dossier Brouillons configuré, formulaire non vide (hors pièces en cours), ni envoi ni enregistrement en cours, pas d'erreur d'enregistrement antérieure (`savedError`). — A:12645-12649, A:12814-12829, A:8428, A:38170, T:Views/User/SettingsOpenPGP.html:28.

**F-CU-81 : Formulaire vide** — `isEmptyForm` : tous les champs d'adresses, l'objet, les pièces et le corps (`getData()` vide) sont vides. Pièces "en cours" comptées sauf pour l'autosave. — A:13965-13972.

**F-CU-82 : Minimisation** — Bouton "−" (`minimize-custom`), touche `Échap` ou `Ctrl/Cmd+↓` : `skipCommand` pose `bSkipNextHide`, enregistre un brouillon (si réglage activé, dossier configuré, pas déjà en cours) puis ferme la popup *sans* réinitialiser (`composeInEdit` reste vrai, texte conservé). Aucune barre de composition réduite ; la reprise se fait par "Nouveau message" (bouton orange). Impossible pendant un envoi/enregistrement (commande désactivée). — A:12769-12777, A:13474-13478, A:12980-12991, T:...PopupsCompose.html:22.

**F-CU-83 : Fermeture** — Bouton "×" ou `Maj+Échap` : `tryToClosePopup` (débouncé 200 ms) ; ferme directement si `bSkipNextHide` ou si le formulaire est vide *et* sans brouillon ; sinon boîte "Êtes-vous sûr de vouloir fermer cette fenêtre ?" (Oui/Non). Fermer ne sauvegarde pas, ne propose pas "enregistrer ?" et ne supprime pas le brouillon existant. — A:13437-13453, A:13496-13502, YML:244+.

**F-CU-84 : Fermeture par Échap désactivée dans la popup** — `bDisabeCloseOnEsc=true` : `Échap` ne ferme pas mais minimise ; `Ctrl+Q`/`Ctrl+W` (et Cmd) sont capturés (`noopFalse`) sous la portée Compose pour éviter la fermeture involontaire par le navigateur (selon navigateur). — A:12663, A:13459.

**F-CU-85 : Suppression du brouillon** — Bouton corbeille rouge visible (animé) seulement quand la composition provient d'un brouillon enregistré (`draftFolder` + `draftUid`) ; confirmation "supprimer ce(s) message(s) ?" ; supprime définitivement du dossier Brouillons (`deleteMessagesFromFolderWithoutCheck`, sans passer par la corbeille) puis ferme la composition. — A:12754-12767, T:...PopupsCompose.html:20-23, CSS:10123-10136.

**F-CU-86 : Rechargement du dossier Brouillons** — Après chaque envoi ou enregistrement : invalidation du hash du dossier, puis soit rechargement de la liste si c'est le dossier courant, soit `folderInformation`. Lors de l'enregistrement d'un brouillon affiché dans le panneau de lecture, le panneau est vidé. — A:12851-12863, A:12954-12968.

**F-CU-87 : Réinitialisation (`reset`)** — Vide tous les champs et indicateurs, détruit les pièces (`onDestroy`), efface `draftFolder/Uid`, ferme Cc/Cci/Reply-To, `aDraftInfo`, `In-Reply-To`, `References`, erreurs, minuteurs ; vide l'éditeur. Appelée à l'ouverture d'une nouvelle composition et à la fermeture définitive. — A:13974-14008, A:12980-12991.

**F-CU-88 : Routage suspendu pendant la composition** — `onShow` appelle `routeOff()` (le changement de hash n'est plus interprété) et `onHide` `routeOn()`. — A:13108, A:12990, A:4065-4075.

**F-CU-89 : Retour arrière bloqué dans les popups** — Dans toute popup visible, `Backspace` hors champ de saisie est annulé (évite la navigation arrière du navigateur). — A:7822-7840.

---

## 7. Envoi, avertissements, erreurs, en-têtes, options

**F-CU-90 : Bouton Envoyer** — Vert par défaut, rouge (`btn-danger`) après une erreur d'envoi, orange (`btn-warning`) si l'envoi a réussi mais l'enregistrement dans Envoyés a échoué ; icône avion / spinner ; `Ctrl/Cmd+Entrée` n'envoie que si le réglage applicatif `allowCtrlEnterOnCompose` est vrai (défaut config `true`, mais la raccourci n'est pas listé dans l'aide, la ligne est commentée). — T:...PopupsCompose.html:5-9, A:13488-13494, A:12460-12462, `.../app/libraries/RainLoop/Config/Application.php:417`.

**F-CU-91 : Validations avant envoi (ordre exact)** — 1) pièces en transfert → drapeau `attachmentsInProcessError` + bascule sur l'onglet Pièces ; 2) pièces en erreur → `attachmentsInErrorError` + bascule ; 3) aucun destinataire → `emptyToError`. Si l'un des trois, l'envoi est stoppé. Il n'y a ni avertissement d'objet vide, ni détection de pièce oubliée, ni confirmation d'envoi à de nombreux destinataires. — A:12674-12695.

**F-CU-92 : Dossier d'envoi** — Dossier Envoyés système ; si le réglage `ReplySameFolder` est actif et que `aDraftInfo` a 3 éléments, l'e-mail est copié dans le dossier du message d'origine ; sans capacité Dossiers, valeur `__UNUSE__` → envoi sans enregistrement ; si aucun dossier Envoyés n'est configuré (vide), la popup "dossiers système" s'ouvre avec la notification `Sent` et l'envoi n'a pas lieu ; la valeur `__UNUSE__` choisie par l'utilisateur signifie "ne pas enregistrer" (envoyée en chaîne vide). — A:12696-12731.

**F-CU-93 : Marquage répondu/transféré** — Au lancement de l'envoi, les drapeaux du message d'origine sont mis à jour localement dans le cache (index 2 = répondu, 3 = transféré), la liste et le message sont rafraîchis depuis le cache et le hash du dossier d'origine est invalidé ; la confirmation réelle vient du serveur. — A:12712-12726.

**F-CU-94 : Paramètres envoyés (SendMessage)** — `IdentityID, MessageFolder, MessageUid` (brouillon à supprimer), `SentFolder, To, Cc, Bcc, ReplyTo, Subject, TextIsHtml, Text, DraftInfo, InReplyTo, References, Dsn, ReadReceiptRequest, MarkAsImportant, Attachments` ; délai maximal 300 s (envoi) ou 200 s (brouillon). `SaveMessage` : mêmes champs sans Dsn/ReadReceipt. Les champs `To/Cc/Bcc` du brouillon ne sont pas "trimés" (seul l'envoi fait `trim`). — A:7181-7292, A:7746-7750, A:12731-12748.

**F-CU-95 : Succès d'envoi** — Si `Result` vrai et popup visible : `closeCommand` (fermeture sans demande), `composeInEdit=false`, `reset()`. Puis rechargement du dossier Brouillons. — A:12919-12944, A:12980-12991.

**F-CU-96 : Échec d'envoi** — Message traduit selon le code d'erreur (303, 750 démo, 901 serveur de courrier, 104 connexion...) ou `ErrorMessage` du serveur (code `ClientViewError`) ; info-bulle d'erreur sur le bouton Envoyer, bouton rouge ; la composition reste ouverte. Cas particulier code 301 `CantSaveMessage` : le message est parti mais pas copié dans Envoyés ; bouton orange avec "Le message a été envoyé mais n'a pas été enregistré dans le dossier des messages envoyés" ; popup non fermée. — A:12932-12941, A:593-652 (codes), YML:206-243.

**F-CU-97 : Options du menu "liste"** — Demander une confirmation de lecture (`ReadReceiptRequest` → en-tête de notification de disposition), Demander un accusé de réception (`Dsn`, notification d'état de remise), Marquer comme important (`MarkAsImportant`), puis séparateur et OpenPGP (visible si `capaOpenPGP`, désactivé en HTML). Les options sont réinitialisées à chaque ouverture. — T:...PopupsCompose.html:58-96, A:13980-13982.

**F-CU-98 : Données d'en-tête fournies par le client** — `In-Reply-To` (= Message-ID du message cité), `References` (Message-ID + références existantes), `DraftInfo` [type, uid, dossier] (pour marquer répondu/transféré côté serveur), `ReplyTo`, `MarkAsImportant`, `ReadReceiptRequest`, `Dsn`. Les valeurs sont posées côté client dans les champs ci-dessus ; la construction MIME et les en-têtes réels sont côté serveur (hors périmètre). — A:13249-13251, A:13266-13270, A:7181-7292.

**F-CU-99 : Accusé de lecture envoyé en retour (réception)** — Lorsque l'utilisateur ouvre un message avec demande d'accusé, un bouton "envoyer l'accusé" crée un message avec objet `Accusé de réception (affiché) - %SUBJECT%` et un corps type, marque `isReadReceipt`. — A:36504-36517, YML:156-160.

---

## 8. OpenPGP dans la composition

**F-CU-100 : Disponibilité** — Bouton OpenPGP dans le menu "liste" : visible si la capacité OpenPGP est active, que `window.crypto.getRandomValues` existe et que la bibliothèque OpenPGP.js s'est chargée (chargement paresseux, worker si possible). Utilisable seulement en texte brut ("OpenPGP (Texte non formaté uniquement)"). — T:...PopupsCompose.html:90-96, A:12839-12849, A:29525-29557.

**F-CU-101 : Popup Signer/Chiffrer** — Cases "Signer" et "Chiffrer" ; liste de clés de signature (clé privée trouvée pour l'identité) avec mot de passe ; liste de clés publiques de chiffrement préremplie avec les clés correspondant aux destinataires À/Cc/Cci (adresses extraites) et à l'identité ; menus pour ajouter des clés publiques ou changer la clé privée ; suppression par corbeille (impossible pour la clé de signature elle-même). Le bouton d'action s'intitule Signer, Chiffrer ou Signer et chiffrer selon les cases, activé seulement si une action est choisie et, pour le chiffrement, au moins une clé. — T:Views/User/PopupsComposeOpenPgp.html, A:38833-39217.

**F-CU-102 : Exécution** — Déchiffrement de la clé privée avec le mot de passe (erreur "Aucune clé privée" si échec) ; erreurs "Aucune clé publique pour X" ; chiffrement/signature via OpenPGP.js ; le texte brut de l'éditeur est remplacé par le résultat (`setPlain`). Tab / Maj+Tab alternent mot de passe ↔ bouton ; `Entrée` dans le mot de passe lance l'action. — A:38933-39051, A:39115-39134.

---

## 9. Plomberie : Knoin, commandes, focus, raccourcis

**F-CU-103 : Écrans (Screens) et routage par hash** — `startScreens` instancie les écrans (le premier est l'écran par défaut), initialise `hasher` + `crossroads` : route générale `^([a-zA-Z0-9-]*)/?(.*)$` → `screenOnRoute(nomEcran, sousPartie)` ; un nom inconnu bascule sur l'écran par défaut avec le nom comme sous-partie. Le `hash` est de la forme `#/mailbox/...`, `#/settings/...`. — A:4128-4240, A:4241-4290.

**F-CU-104 : Routes de la boîte mail** — `dossier`, `dossier/pN`, `dossier/pN/recherche`, `dossier/recherche` ; `dossier~uidFil` pour les discussions (threads) ; paramètres URL-décodés ; page ≥ 1. Les messages sont ouverts par `onRoute` (dossier connu dans le cache sinon ignoré). — A:33832-33936.

**F-CU-105 : Cycle de vie des vues** — `buildViewModel` construit une seule fois chaque vue/popup : `onBuild`, `onShow`, `onShowWithDelay` (200 ms écrans, 500 ms popups), `onHide`, `onHideWithDelay`, déclencheurs `onShowTrigger/onHideTrigger`, hooks de plugins (`view-model-pre-build`…). — A:3991-4075, A:4128-4240.

**F-CU-106 : Popups modales empilées** — Chaque popup visible : `z-index = 3000 + n + 10` (retombe à 2000 à la fermeture), portée clavier mémorisée puis restaurée, `html.rl-modal` posé tant qu'au moins une popup est ouverte, masquage DOM 300 ms après. — A:4012-4045, A:3700-3705 (popupVisibility).

**F-CU-107 : Boîtes de confirmation (Ask)** — Popup `View/Popup/Ask` : texte HTML (`html:` binding), boutons Oui/Non traduits ou libellés personnalisés ; focus sur Oui par défaut (`isFocusYesOnShow`) ; `Tab`, `Maj+Tab`, `←`, `→` alternent Oui/Non ; `Échap` = Non ; ne se ferme pas seule par Échap (`bDisabeCloseOnEsc`). Usages : fermer la composition, supprimer brouillon/messages, abandonner des données non enregistrées. — A:20391-20515, T:Views/Common/PopupsAsk.html, YML:244-298.

**F-CU-108 : Système de commandes** — `command()` décorateur (méthodes se terminant par `Command`) et `createCommand` : chaque commande a `canExecute` (calculé, observable), `enabled`, retourne toujours `false` (pas de navigation). Le binding `command` pose les classes `command`, `command-not-enabled`, `command-can-not-be-execute`, `command-disabled disable disabled` (ou `no-disabled`) et `prop disabled` sur `input`/`button` ; appelle `click` (ou `submit` pour un `form`). — A:4365-4390 (décorateur), A:1492-1538, A:2370-2400.

**F-CU-109 : Commandes de la composition** — `sendCommand`, `saveCommand` (verrouillées pendant envoi/enregistrement), `deleteCommand` (brouillon), `skipCommand`, `contactsCommand` (contacts autorisés), `dropboxCommand`, `driveCommand`. — A:12367-12386.

**F-CU-110 : Portées clavier (KeyState)** — `all, none, contact-list, message-list, folder-list, message-view, compose, settings, menu, compose-open-pgp, message-open-pgp, view-open-pgp, popup-keyboard-shortcuts-help, popup-ask`. `keymaster` (v1 de L) filtre les événements. — A:228-244, A:3707-3762, L:2060+.

**F-CU-111 : Filtre de touches selon portée** — Portée Compose : les raccourcis fonctionnent même dans les champs (`filter = useKeyboardShortcuts()`). Autres portées : ignorés si la cible est `INPUT`, `SELECT`, `TEXTAREA` ou un `DIV` éditable (`contentEditable='true'` ou classe `editorHtmlArea`). `useKeyboardShortcuts` est mis à `false` pendant l'affichage de la galerie d'images. — A:3709-3760, A:36008-36014.

**F-CU-112 : Menus déroulants et clavier** — Dès qu'un menu Bootstrap est ouvert (`dropdownVisibility`), la portée passe à `menu` (les raccourcis de listes sont inactifs) ; elle revient à la portée précédente à la fermeture ; `Échap`/`Entrée` déclenchent une re-détection de l'état des menus. — A:3741-3762, A:23318-23323, A:2919.

**F-CU-113 : Raccourcis de la composition** — `Échap` / `Ctrl+↓` (minimiser), `Maj+Échap` (fermer), `Ctrl/Cmd+S` (enregistrer), `Ctrl/Cmd+Entrée` (envoyer, conditionnel), `` ` `` / `Ctrl+`` ` (identités), `W` ou `C` (ouvrir depuis liste/message). Aide dans l'onglet "Écrire". — A:13455-13502, T:Views/Common/PopupsKeyboardShortcutsHelp.html:4-100, YML:587-626.

**F-CU-114 : Aide des raccourcis** — Touche `?` (`Maj+/`) ou menu utilisateur : popup à 4 onglets (Boîte, Liste, Message, Écrire) ; `Tab`/`←`/`→` changent d'onglet. — A:24298-24317, A:30530-30575.

**F-CU-115 : Raccourcis globaux et de liste (aide complète)** — Menu utilisateur `` ` `` ; `R`, `A`, `F`, `Maj+F` ; `Ctrl+A`, `Z`, `Suppr`/`Maj+Suppr`/`#`, `T`, `M`, `Q`, `U`, `I`, `/`, `Entrée`, `→`/`Tab`, `←`/`Maj+Tab`, `Ctrl+R` (recharger), `B` (citations), `Ctrl+I` (infos message), `Ctrl+P` (imprimer), `Ctrl+↑/↓/←/→` (message précédent/suivant), `Échap`, `Suppr`. Liste : flèches, `Maj+flèches`, `Home/End/PageUp/PageDown/Insert/Espace`. — A:35160-35314, A:36170-36281, A:24652-24710.

**F-CU-116 : Blocage de Ctrl+S et Ctrl+A globaux** — `killCtrlACtrlS` : `Ctrl+S` toujours annulé (pas de boîte "Enregistrer sous") ; `Ctrl+A` hors `input`/`textarea`/contenteditable : annule la sélection et le comportement natif (la sélection de tout le texte de la page est neutralisée). — A:2337-2368, A:22506 (`$win.on('keydown', killCtrlACtrlS)`).

**F-CU-117 : Focus et sélection** — `inFocus()` détecte `input,textarea,iframe,.cke_editable` (résultat mis en cache sur l'élément) ; `removeInFocus`/`removeSelection` ; `Focused` (None/MessageList/MessageView/FolderList) pilote la portée clavier et, en mobile, le panneau gauche. — A:2175-2215, A:7987-8050.

**F-CU-118 : Liste à sélection au clavier (Selector)** — Flèches, `Maj+flèches` (sélection multiple), Home/End, PageUp/PageDown, Insert, Espace, Entrée, `Ctrl+↑/↓` ; utilisé pour la liste de messages et celle des contacts. — A:24364-24720.

---

## 10. Plomberie : mobile/bureau, disposition, redimensionnement, plein écran, défilement, infobulles, animations

**F-CU-119 : Version mobile côté serveur** — Si `labs.allow_mobile_version` est actif, détection `MobileDetect` (option `use_mobile_version_for_tablets`) ; le cookie `rlmobiletype` (`mobile`/`desktop`) force le mode ; liens "Version mobile / Version grand écran" dans le menu utilisateur et la page de connexion (`./?/MobileVersion/`, `./?/DesktopVersion/`) ; meta `viewport` `width=device-width,initial-scale=1,user-scalable=no` en mobile, `width=950,maximum-scale=2` en bureau. — `.../app/libraries/RainLoop/Service.php:159-182,306`, `.../ServiceActions.php:841-855`, T:Views/User/SystemDropDown.html:60-75.

**F-CU-120 : États de largeur côté client (ssm)** — Hors mode mobile serveur, `ssm` pose les classes `ssm-state-mobile` (≤767 px), `tablet` (768-999), `desktop` (1000-1400), `desktop-large` (≥1401) et publie `ssm.mobile-enter/leave` (replie/déplie le panneau gauche) ; en mode mobile serveur : classes `rl-mobile` et `ssm-state-mobile` fixes. Le CSS pilote largeurs de colonnes (200/155 px, volets de liste 310/400/500 px). — A:23479-23545, CSS:7021-7080.

**F-CU-121 : Détection d'appareil mobile (UA)** — `bMobileDevice` (android, iphone, ipod, ipad, blackberry) : désactive barres nano, animations, infobulles (sauf `data-tooltip-mobile=on`), glisser-déposer jQuery UI, aperçu PDF, lecteur audio, focus automatique, fondu des popups ; ajoute `html.mobile` ou `no-mobile` ; `bIE`, `bChrome`, `bSafari` servent à audio et téléchargement. — A:3559-3565, A:1240-1356, A:1363.

**F-CU-122 : Disposition de la boîte mail** — Réglage `Layout` : 0 sans panneau (`rl-no-preview-pane`), 1 volet vertical/à côté (`rl-side-preview-pane`, défaut), 2 volet horizontal/en bas (`rl-bottom-preview-pane`). Applique des classes sur `<html>`, vide la liste au changement, enregistre le réglage, choisit le gabarit d'élément de liste (`MailMessageListItem` ou `…NoPreviewPane` ; mobile = avec aperçu). Auto-sélection du premier message désactivée en mode sans panneau ou recherche `is:unseen`. — A:8378-8460, A:34599, A:34866-34876, A:36894-36899.

**F-CU-123 : Séparateurs redimensionnables** — Volet vertical gauche : poignée `e`, 155 à 350 px, largeur mémorisée sous `FolderListSize` (défaut 155) ; replié à 60 px quand le panneau gauche est désactivé ; désactivé sur appareil mobile. Volet horizontal (disposition basse) : poignée `s`, 200 à 500 px, défaut 300 px, mémorisé sous `MessageListSize` ; détruit lorsque la disposition change. Pendant le glissement la classe `rl-resizer` est posée. — A:29269-29350, A:29352-29435, A:33886-33891, A:29643-29647.

**F-CU-124 : Panneau gauche (dossiers)** — États `rl-left-panel-disabled/enabled`, types `none` et `short` (sans capacité Dossiers : "short" si Composer ou Contacts, sinon "none") ; `moveAction` (mode déplacement) désactive le panneau, et inversement. Mobile : le panneau est replié quand le focus passe sur la liste ou le message. — A:23535-23545, A:3730-3745, A:33814-33822, A:7993-8020.

**F-CU-125 : Plein écran du message** — `rl-message-fullscreen` sur `<html>`, activé par `Entrée` dans la vue message (ou dans la liste, avec aperçu et sélection), quitté par `Échap` (retour focus liste) ou `` ` `` ; recalcul de hauteur d'en-tête et redimensionnement de fenêtre. — A:35832-35834, A:35920-35934, A:36150-36180, A:24303.

**F-CU-126 : Redimensionnement global** — `window.resize` → événement `window.resize` throttlé 50 ms → `window.resize.real` seulement si la taille a changé (note : la comparaison utilise deux fois la hauteur, `iW = $win.height()`, donc un changement de largeur seul n'est pas détecté) ; `windowResize` débouncé 50 ms, relancé au bout d'1 s au démarrage. — A:23300-23312, A:3338-3360.

**F-CU-127 : Barres de défilement nano** — Binding `nano` : plugin `nanoScroller` (sans défilement natif iOS) sauf mobile ou réglage `useNativeScrollbars`. — A:1358-1370.

**F-CU-128 : Ombres de défilement** — Binding `scrollerShadows` : classes `scroller-shadow-top` / `scroller-shadow-bottom` selon la position (seuil 8 px, throttle 100 ms) sur le contenu `[data-scroller-shadows-content]`, aussi à `resize`. Utilisé dans la zone pièces jointes de la composition. — A:771-789, T:...PopupsCompose.html:161-166.

**F-CU-129 : Infobulles (Opentip)** — Style `rainloop` (fond blanc, bordure grise, fondu 0,2 s, délai 0,2 s ; `rainloopTip` : délai 0,4 s, groupe commun) ; affichage au survol ou clic ; le texte est traduit (clé i18n) et se met à jour au changement de langue ; masquée à l'ouverture d'un menu ; désactivée sur mobile ; la jointure (`data-tooltip-join`) place la flèche. — A:826-907, A:18631-18660.

**F-CU-130 : Infobulles d'erreur** — Binding `tooltipErrorTip` (style `rainloopErrorTip`) : s'affiche 100 ms après changement de valeur non vide si l'élément est visible, ferme au clic/sortie de souris. Utilisé pour erreurs d'envoi/enregistrement, destinataire manquant, pièces. — A:908-953.

**F-CU-131 : Animations** — Réglage serveur `InterfaceAnimation` + CSS3 : classes `rl-anim`/`no-rl-anim` ; désactivées sur mobile ; transitions de modales (fondu + translation 20 px, 200 ms), `rl-modal-animation` (overflow masqué 500 ms), bascule du bouton corbeille, rayures de chargement de la composition, animation de lettres à la traduction (plugin `letterfx`), composant Case à cocher "Material Design" si `materialDesign` et animations CSS supportées. — A:22612-22625, A:1100-1133, A:4551-4560, A:23465-23475, CSS:11400-11470.

**F-CU-132 : Glisser-déposer de messages** — Éléments de liste `draggable` (poignée `.dragHandle`, distance 20 px) avec aide visuelle (`draggablePlace` affichant le nombre de messages et icône copie/déplacement selon `Ctrl`) ; dossiers `droppable` ; défilement automatique des dossiers aux bords (zone de 100 px, 3 px / 10 ms) ; `Ctrl` classe `rl-ctrl-key-pressed` bascule déplacer/copier. — A:1235-1357, A:2457, A:34915-34940, A:23330-23345.

**F-CU-133 : Dépôt d'un fichier .eml dans la liste** — Si `allowAppendMessage` : `Jua` (action `append`, champ `AppendFile`, 1 fichier) ; seuls les fichiers `message/rfc822` sont acceptés ; zone affichée au survol du corps de page ; rechargement de la liste après ajout. — A:35357-35401.

---

## 11. Plomberie : notifications, titre, réseau, session, rafraîchissement, caches

**F-CU-134 : Notification sonore** — Réglage `SoundNotification` ; `Audio.playNotification` joue `new-mail.mp3` (dossier `static/sounds`) ; requiert un appareil non mobile, un navigateur non Safari, support MP3 ; bouton "Tester le son" dans les paramètres ; lecture aussi pour chaque nouveau message de la boîte de réception détecté. — A:10652-10790, A:11975-11985, A:5254-5262, A:36824-36826.

**F-CU-135 : Notifications de bureau** — Réglage `DesktopNotifications` ; permission demandée à l'activation (`Notification.requestPermission`), états Autorisé/Non demandé/Refusé/Non supporté ; pour 1 à 3 nouveaux messages une notification par message (expéditeur, objet), au-delà une seule "N nouveaux messages" ; fermée après 7 s ; le clic ramène la fenêtre au premier plan et ouvre le message (`mailbox.message.show`). — A:11864-12040, A:5254-5290.

**F-CU-136 : Titre de l'onglet** — `setWindowTitle` : `titre - Titre applicatif` (réglage `Title`) ; écrit deux fois (avec " ...") pour forcer le rafraîchissement ; boîte mail : `(N) adresse - Boîte mail` (N = non lus de la boîte de réception, ignoré si `listPermanentFiltered`), paramètres : `adresse - Paramètres`, chargement : "Chargement", connexion : "Identifiant" ; le titre `TITLES/COMPOSE` existe en traduction mais n'est pas utilisé par la popup. — A:23354-23365, A:33794-33803, A:29504-29513, A:36683, YML:407-412.

**F-CU-137 : Favicon** — Pas de compteur dynamique : aucun code de manipulation du favicon n'existe dans `app.js`/`boot.js`. Le serveur émet des balises `<link>` PNG/Apple touch (`BaseAppFaviconPngLinkTag`, `BaseAppFaviconTouchLinkTag`) ; fichiers `favicon.ico/png`, `favicon-light.ico/png`, `android-icon.png`, `apple-touch-icon.png` et `manifest.json` fournis. — `.../app/templates/Index.html:17-20`, `.../static/` (listing).

**F-CU-138 : Gestion des erreurs réseau (pas de bandeau hors-ligne)** — `defaultResponse` : un échec est retardé de 300 ms puis renvoyé à l'appelant en `StorageResultType.Error` ; les échecs pendant `unload` sont convertis en `Unload` (silencieux) ; les messages utilisateur sont propres à chaque vue (ex. liste : "Impossible d'obtenir la liste des messages"). Il n'y a ni détection `navigator.onLine`, ni bannière de connexion perdue, ni reconnexion automatique. — A:22757-22818, A:28578-28592.

**F-CU-139 : Seuils de déconnexion automatique par erreurs** — Compteurs globaux : `iAjaxErrorCount` +1 à chaque réponse d'erreur parmi AuthError, AccessError, ConnectionError, DomainNotAllowed, AccountNotAllowed, MailServerError, UnknownNotification, UnknownError ; remis à 0 au premier succès ; > 7 (`AJAX_ERROR_LIMIT`) : effacement du jeton client et rechargement vers la connexion. `InvalidToken` incrémente `iTokenErrorCount` ; > 10 (`TOKEN_ERROR_LIMIT`) : déconnexion + rechargement. `ClearAuth` / `Logout` renvoyés par le serveur déclenchent l'effacement immédiat. Un `UpdateToken` renouvelle le jeton client. — A:22764-22790, A:22886-22890, A:32868-32891, A:7757-7760.

**F-CU-140 : Délais d'attente réseau** — Défaut 30 s ; recherche 300 s ; envoi 300 s ; enregistrement brouillon 200 s ; synchro contacts 200 s ; copies de pièces 999 s ; erreurs de timeout converties en `AjaxTimeout` (953) dans la couche promesses. — A:7742-7753, A:32848-32851.

**F-CU-141 : Expiration de session et déconnexion automatique** — Réglage `AutoLogout` (minutes, défaut 30, capacité `AUTOLOGOUT`), ignoré si "se souvenir de moi" est actif (`AccountSignMe`) ; chaque `mousemove`/`keypress`/`click` (débouncé 5 s) réarme le minuteur ; à l'échéance, `rl.auto-logout` appelle la déconnexion serveur puis recharge. Si l'application est suspendue plus d'1 h (`$.wakeUp`, intervalle 60 min), l'horodatage de session expiré efface le hash et recharge la page ; au réveil, une requête `Version` recharge aussi si la version applicative a changé. — A:8378-8450, A:8402-8450 (déb. minuteur), A:23333-23345 (activité), A:28499-28513, A:15136-15252.

**F-CU-142 : Stockage de session** — Le hash d'authentification (`__rlA`) et l'horodatage (`__rlT`, rafraîchi chaque minute) sont conservés dans `sessionStorage` (repli sur `window.name` JSON) ; le jeton est transmis dans les URL de requêtes (`populateAuthSuffix`). — A:15136-15252, A:23382-23395, B:10095-10110.

**F-CU-143 : Rafraîchissement périodique** — Bus d'événements `interval.30s/1m/2m/3m/5m/10m/15m/20m` et variantes décalées de 5 min. Utilisations : informations de la boîte de réception toutes les 2 min ; dossier courant toutes les 3 min ; dossiers suivants (lots) toutes les 2 min après 5 min ; quota toutes les 15 min (et 30 s débouncé après déplacement/suppression) ; rechargement de l'arbre de dossiers toutes les 20 min ; synchro des contacts (5 à 320 min, défaut 20) ; mise à jour des libellés de date chaque minute ; requête `appDelayStart` à 35 s ; autosave du brouillon (F-CU-80). — A:28449-28497, A:29566-29616, A:29682-29684, A:28445.

**F-CU-144 : Pré-chargement des messages** — Capacité `Prefetch` (hors mobile) : après 10 s d'inactivité (`ifvisible`), charge un à un (pause 1 s) les messages de la liste non encore demandés. — A:35152-35157, A:35317-35341.

**F-CU-145 : Cache local** — Mémoire : hashes de dossiers, drapeaux de messages, `UidNext`, messages demandés, corps de messages (15 conservés, `MESSAGE_BODY_CACHE_LIMIT`). Stockage client persistant (`localStorage` JSON `rlcsc`, préfixe `p`, repli cookie 30 jours) pour : hash de dossiers, taille panneau dossiers/liste, dossiers déployés, dernière action de réponse, affichage complet de l'en-tête, contrôles de pièces. — A:7757, A:9527-9572, A:32190-32360, A:334-348.

**F-CU-146 : Mise à jour des drapeaux sans rechargement** — `folderInformation` / `folderInformationMultiply` comparent les hashes serveur au cache : mettent à jour compteurs, drapeaux (lu, suivi, répondu, transféré, accusé), déclenchent `reloadMessageList` ou `recacheInboxMessageList` en cas de différence ; les nouveaux messages (UID > UidNext) déclenchent son/notification. — A:28959-29096, A:5254-5290.

---

## 12. Plomberie : thème, langue, impression, liens, accessibilité, démarrage

**F-CU-147 : Thème dynamique** — `changeTheme` récupère le CSS du thème en JSON (`/Css/.../-/{theme}/-/…/Json/`), remplace le lien `#app-theme-link` par une balise `<style id="app-theme-style">` (annule la requête précédente), sans recharger ; indicateur d'état (animation/réussite). Images de fond utilisateur par `backstretch` (fondu 1 s si animations). Thèmes présents : A, Blurred, Clear, DarkShine, Default, Linen, Love, Mobile, Snow, Squares, SquaresDark, Stripes, Wood, Xv. — A:3029-3085, A:28515-28523, A:11812-11830, `.../themes/`.

**F-CU-148 : Langue dynamique** — `Translator.reload` charge le script de langue (`dataType: script`, cache), réapplique `[data-i18n]` (formes texte, `[html]`, `[placeholder]`, `[title]`), met à jour les infobulles et les notifications d'erreur, recharge les dates (moment), pose `rl-changing-language` puis `rl-rtl`/`rl-ltr` (RTL pour ar, ar_sa, he, he_he, ur, ur_ir). Réglage enregistré après chargement. CKEditor ne change pas de langue en cours de session (langue lue à la création). — A:4541-4620, A:4695-4717, A:36845-36852.

**F-CU-149 : Textes d'erreur localisés** — `getNotification(code, message, defCode)` : table de ~55 codes d'erreur vers les clés `NOTIFICATIONS/*` (YML:640-700) ; le code `ClientViewError` (902) affiche le message du serveur tel quel. — A:4482, A:4619-4650, YML:640-700.

**F-CU-150 : Impression** — `Ctrl/Cmd+P` (liste/message) ou bouton : ouvre une fenêtre vierge, écrit le gabarit `PreviewMessage.html` (titre, objet, date, De, À, Cc, corps nettoyé des commutateurs de citations), lance `print()` après 100 ms. Aucune règle CSS `@media print` dans `app.css`. — A:2493-2525, A:25689-25730, A:36242-36247.

**F-CU-151 : Citations repliables (lecture)** — Les `<blockquote>` de premier niveau dont la hauteur dépasse 100 px (ou nulle) sont repliés (`rl-bq-switcher hidden-bq`) avec une icône "…" cliquable ; `B` bascule tous les replis dans le message ouvert ; `clearBqSwitcher` les neutralise avant réponse/impression. Pas de repli dans la composition. — A:5416-5445, A:2478-2483, A:36220-36230.

**F-CU-152 : Liens `mailto:` dans les messages** — Clic (sauf bouton du milieu) sur un lien dont l'`href` commence par `mailto:` : si Composer est actif, ouvre la composition avec À (adresse + paramètre `to`, dédoublonné et préférant la variante avec nom), `cc`, `bcc`, `subject`, `body` ; sinon le clic est bloqué. Les paramètres sont décodés avec `decodeURIComponent` (le `+` n'est pas converti en espace ; une séquence `%` invalide lève une exception) ; le corps passe par `plainToHtml` sans détection de liens ; le format de l'éditeur suit le réglage par défaut. — A:3268-3320, A:36073-36080.

**F-CU-153 : Gestionnaire de protocole mailto** — Après connexion avec "se souvenir de moi" et capacité Composer : `registerProtocolHandler('mailto', …?mailto&to=%s, Titre)` ; au retour, si le serveur a transmis `MailToEmail`, la composition s'ouvre préremplie (`mailToHelper`). — A:29630-29641, `.../ServiceActions.php:894-910`, `.../Actions.php:1265-1275,1617-1621`.

**F-CU-154 : Liens externes (affichage)** — Les liens générés côté client pour des adresses (`toLine(wrapWithLink)`) portent `target="_blank" tabindex="-1"` et `href="mailto:…"` ; `Autolinker` ajoute les ouvertures en nouvelle fenêtre dans l'éditeur. Aucun avertissement de sortie vers un site externe, ni `rel="noopener"` explicite dans le code client (le nettoyage HTML se fait côté serveur). — A:10520-10560, A:2605-2620.

**F-CU-155 : Téléchargements** — `download(link)` : sous Chrome, déclenche un lien ; sur mobile `window.open(link, '_self')` ; sinon via un `iframe` caché. Utilisé pour les pièces et archives zip. — A:23312-23332, A:36106-36112.

**F-CU-156 : Accessibilité** — Rôles ARIA dans les gabarits (`role="menu"`, `menuitem`, `presentation`, `checkbox`, `radio`, `aria-labelledby` du menu identités) ; composants Case à cocher et Radio focalisables (`tabindex=0`) et activables à la barre d'espace (`onSpace`) ; `tabindex=-1` sur les déclencheurs de menus ; pastilles d'adresses navigables au clavier ; champs d'adresses sans correction automatique. Pas de lien d'évitement ni de région ARIA live pour les erreurs. — T:Views/Components/*.html, T:...PopupsCompose.html:33-50.

**F-CU-157 : Composants de formulaire (Components/*)** — `Checkbox` (icône ou Material Design avec animation de case/coche, libellé animé, mode inversé), `CheckboxClassic`, `Input`, `TextArea`, `Select`, `Radio` (valeurs libellées, en ligne), `Date` (Pikaday), `SaveTrigger` (spinner / croix / coche pour l'état d'enregistrement des réglages). Tous déclarés par `ko.components.register` au démarrage. — T:Views/Components/*.html, A:23453-23475, A:27910-28200.

**F-CU-158 : Pagination et utilitaires** — `computedPagenatorHelper` génère les pages (fenêtre de 5 à 7, "…" aux intervalles, première/dernière) ; `friendlySize` (B/KB/MB/GB) ; `convertThemeName`, `convertLangName`. — A:3091-3150, A:2282-2300.

**F-CU-159 : Démarrage et écran de chargement** — `Index.html` : redirection `?/NoScript` si JS absent, `?/BadBrowser` si IE ≤ 8, redirection `?/NoCookie` si cookies désactivés, `meta viewport` serveur, CSP, `manifest`, `mobile-web-app-capable`. `boot.js` : données de démarrage lues de `#app-boot-data`, mise en page (`rl-bg`, spinner `rl-loading` avec message), barre de progression `progressJs` (5 → 20 → 30 → 50 → 70 → 90 → 100), chargement séquentiel : bibliothèques, langue, application, plugins, puis éditeur ; image de fond `backstretch` ; en cas d'échec : "An error occurred. Please refresh the page and try again." (non traduit). Classes `rl-booted`, `rl-started`, `rl-started-delay` (animations d'apparition). — `.../app/templates/Index.html`, B:6791-6830, B:10056-10420, B:10865, A:23477-23490, A:4258-4263.

**F-CU-160 : Pages statiques d'erreur** — `Error.html` (titre, en-tête, description, lien Retour/Recharger), `BadBrowser.html` (logos Chrome/Firefox/Opera), `Social.html` (popup de connexion sociale avec icône, redirection `RefreshMeta`). Textes `STATIC/*` traduits (JS requis, cookies requis, navigateur obsolète). — `.../app/templates/Error.html`, `BadBrowser.html`, `Social.html`, YML:701-720.

**F-CU-161 : Événements publics et plugins** — `rl` global (`i18n`, `createCommand`, `addSettingsViewModel`, `addHook`, `settingsGet`, `pluginSettingsGet`, `pluginRemoteRequest`, `EmailModel`, `Enums`) ; bus `pub/sub` (`Events`) ; hooks (`view-model-*`, `screen-*`, `ajax-default-*`, `rl-start-*`, `rl-pub`). — A:22479-22520 (bootstrap), A:7654-7712, A:8451-8650.

**F-CU-162 : Mode collage et Ctrl pressé** — Un indicateur `rl-ctrl-key-pressed` sur `<html>` suit la touche Ctrl (utile aux aides visuelles de copie). — A:23330-23345.

---

## 13. Fonctions absentes, inertes ou dormantes (constats)

**F-CU-163 : Raccourci Ctrl+Entrée non documenté** — La ligne d'aide correspondante est commentée dans le gabarit ; le raccourci n'est actif que si la configuration serveur l'autorise. — T:Views/Common/PopupsKeyboardShortcutsHelp.html (ligne de Ctrl+Entrée commentée), A:13488.

**F-CU-164 : Confirmation de bascule texte brut non utilisée** — `EDITOR/TEXT_SWITCHER_CONFIRM` ("Le formatage du texte et les images seront perdus...") et `EDITOR/TEXT_SWITCHER_RICH_FORMATTING` ne sont pas référencés dans les sources lues. — YML:421-424, A (aucune occurrence).

**F-CU-165 : Vue "Cmd" (`#` ligne de commande)** — Le gabarit `Views/Common/Cmd.html` (historique, saisie, aide) n'est relié à aucune vue ni script dans `app.js` : fonctionnalité dormante. — T:Views/Common/Cmd.html, A (aucune occurrence `rl-cmd`).

**F-CU-166 : Modèles de messages dans la composition** — La capacité `TEMPLATES` et les réglages de modèles existent mais la popup de composition n'offre aucun bouton ni raccourci d'insertion de modèle. — A:196-222, T:...PopupsCompose.html (absence).

**F-CU-167 : Pas de vérification orthographique propre, pas de rappel de pièce jointe, pas d'envoi différé, pas de glisser d'adresse depuis la liste de messages vers la composition** — Absents du code lu ; l'orthographe repose sur le navigateur (`disableNativeSpellChecker: false`). — A:3600.

**F-CU-168 : Titre de fenêtre de composition inutilisé et favicon sans compteur** — voir F-CU-136 et F-CU-137.

**F-CU-169 : Détection de largeur de fenêtre approximative** — `window.resize.real` n'est émis que si la hauteur change (bogue de comparaison hauteur/hauteur). — A:23300-23312.

**F-CU-170 : Raccourcis clavier dans l'iframe CKEditor** — Les raccourcis `keymaster` (Ctrl+S, Échap, Ctrl+Entrée) sont enregistrés sur le document principal ; le comportement de ces touches lorsque le focus est dans l'iframe de CKEditor n'est pas démontrable par la lecture seule (à vérifier à l'exécution ; le filtre prévoit pourtant la portée Compose "ouverte" aux champs). — A:3709-3730, A:18470.

---

# Résumé (10 lignes)

1. La composition est une popup modale Knoin (`ComposePopupView`, A:12367) à sept types (empty, reply, replyall, forward, forward-as-attachment, draft, editasnew) avec fusion des adresses lorsqu'une composition est déjà en cours.
2. Les adresses sont saisies en pastilles `inputosaurus` : séparateurs `, ; ↵`, collage multi-adresses, double-clic pour éditer, navigation clavier, glisser-déposer entre À/Cc/Cci/Répondre à, autocomplétion serveur ; le texte non reconnu est jeté sans erreur.
3. L'éditeur est CKEditor 4 en mode Entrée=`<br>` avec plugins `plain` (bascule texte brut sans confirmation) et `signature` ; barre : police/taille, gras/italique/souligné, couleurs, listes, retraits, citation, lien, image base64, tableau, bidi/source sous réglage.
4. Pièces jointes via Jua (2 transferts parallèles, 50 fichiers, limite `AttachmentLimit`), glisser-déposer, pièces de messages d'origine, Dropbox et Google Drive ; l'envoi est bloqué tant qu'une pièce est en cours ou en erreur.
5. Identités : sélection automatique selon destinataires/Delivered-To, Cci et Reply-To suivis, signature texte/HTML avant ou après, jetons `{{DATE}} {{TIME}} {{MOMENT:…}} {{FROM}} {{FROM-FULL}}`, remplacement conditionné à l'intégrité de l'ancienne signature.
6. Brouillon : enregistrement manuel (Ctrl+S) et automatique (minuteur 60 s + événement 2 min), minimisation sur Échap/Ctrl+↓ (avec sauvegarde), fermeture avec confirmation, suppression du brouillon, bouton "Nouveau message" orange pendant une composition.
7. Envoi : seules vérifications client = pièces en cours/en erreur et destinataire absent ; erreurs distinctes envoi / enregistrement dans Envoyés ; options accusé de lecture, DSN, importance ; chaînage In-Reply-To/References ; OpenPGP en texte brut.
8. Plomberie : routage hash/crossroads, popups empilées avec portées clavier (KeyState, filtre selon champs, menu), commandes (`canExecute`), Ask (Oui/Non, Échap=Non), mobile serveur (cookie, MobileDetect) et états `ssm`, trois dispositions, séparateurs mémorisés, plein écran, nano, infobulles Opentip.
9. Réseau et session : seuils de 7 erreurs/10 jetons invalides avant déconnexion, minuterie d'inactivité, expiration de session 1 h au réveil, intervalles de rafraîchissement (2/3/15/20 min), préchargement, notifications son/bureau, titre `(N)` ; pas de bandeau hors-ligne ni de compteur de favicon.
10. Constats : confirmation de bascule texte brut et vue `Cmd` non câblées, `removeLinkedAttachments` mort, `ComposeLastIdentityID` inutilisé, comparaison hauteur/hauteur dans `window.resize.real`, aucun avertissement d'envoi (objet vide, pièce oubliée).

Fichier : /tmp/claude-1000/-home-gdegoulet-Work-webmail-project/be75865a-1a74-4b90-b555-b7c7c12ae39b/scratchpad/audit_compose_ui.md
