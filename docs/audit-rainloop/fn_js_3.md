# Analyse RainLoop 1.17 front-end : static/js/app.js lignes 16000 à 24000 (tranche JS n°3)

Source lue : `/Work/webmail_project/rainloop/1.17.0/static/js/app.js` (bundle webpack, 40112 lignes ; tranche 16000-24000, avec lecture du début du module Tokenizer qui commence en 15698 et de la fin du module FolderSystem qui finit en 24122 pour comprendre les unités coupées).
Équivalents wmail-go lus dans `/home/gdegoulet/Work/webmail_project/internal/web` (templates `base.html`, `compose.html`, `contacts.html`, `templates.html`, `settings.html`, `login.html` ; `static/compose.js`, `editor.js`, `chips.js`, `preview.js`, `poll.js`, `menus.js`, `list.js`, `keys.js`, `mail.css`) et `internal/compose`, `internal/mailview`, `internal/config`.

## 0. Cartographie de la tranche (honnêteté sur ce qui a été lu)

La tranche contient 133 modules webpack. Seuls 20 sont du code RainLoop (`./dev/...`), 3 sont des « externals » (`window.Jua`, `window.Autolinker`, `window.ifvisible`), 1 est Pikaday, le reste (~109) est du **core-js / @babel/runtime** (polyfills), SANS OBJET. J'ai lu en entier les 20 modules `./dev/...` ; pour core-js j'ai vérifié par les titres de module (`!*** ./node_modules/... ***!`) et par l'en-tête de chaque module qu'il s'agit bien de polyfills standards, sans les relire ligne à ligne (aucun comportement visible).
Non lu : rien de la tranche n'est resté illisible. Les modules `Component/SaveTrigger`, `Input`, `Select`, `TextArea`, `Date`, `Script` sont enregistrés en 23457-23464 mais leur code est hors tranche (autre agent).

Modules RainLoop de la tranche (ligne de début) : Stores/User/Template 17044 · Model/Attachment 17105 · Component/Checkbox 18141 · Common/HtmlEditor 18171 · External/Opentip 18617 · Common/Mime 19972 · Stores/User/Filter 20140 · View/Popup/Ask 20361 · Stores/User/Quota 20521 · View/Popup/Languages 22375 · bootstrap 22479 · Stores/AbstractApp 22596 · Remote/AbstractAjax 22708 · Screen/AbstractSettings 22974 · App/Abstract 23185 · Knoin/AbstractBoot 23557 · Component/AbstractRadio 23580 · Model/ComposeAttachment 23678 · Model/ContactProperty 23826 · View/Popup/FolderSystem 23983.

Légende verdicts : OK · PARTIEL · MANQUANT · SANS OBJET. Priorités P1 (quotidien, très visible) / P2 / P3.

---

## 1. Analyseur d'adresses (emailjs-addressparser, fin de module : lignes 15880-16008)

Utilisé par `EmailModel` (hors tranche) pour lire les champs À/Cc/Cci collés ou saisis. Unités : OPERATORS, Tokenizer (constructeur), tokenize, checkChar (+ `parse` / `_handleAddress` lus en 15730-15880 pour le contexte).

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| OPERATORS (15889-15905) | Table des délimiteurs `" ( < , : ;` et de leur fermeture | Découper une saisie en jetons : séparateurs `,` et `;`, groupes `Nom: a, b;`, commentaires `(…)` | `web/static/chips.js:13-26` (`split`) et `internal/compose/address.go:27-58` (`ParseList`) : séparateurs `, ; \n` hors guillemets et chevrons | PARTIEL | Pas de gestion des groupes `Equipe: a@x, b@y;` ni des commentaires `(Nom)`. Le `;` interne d'un groupe coupe la liste. P3 |
| Tokenizer ctor (15910-15925) | État du découpage | Interne | idem | SANS OBJET | Interne à la lib |
| tokenize (15933-15950) | Boucle caractère par caractère, supprime les jetons vides | Interne | `chips.js:13-26` | OK | Équivalent fonctionnel |
| checkChar (15961-16007) | Gère l'échappement `\`, les opérateurs attendus, concatène le texte | Citations `"Doe, John" <j@d>` et `\"` | `chips.js:16` (échappement dans guillemets) ; `address.go:41-44` (`esc`) | OK | Le cas guillemets + échappement est couvert côté client et serveur |
| `parse`/`_handleAddress` (contexte 15730-15880, hors tranche mais lu) | Tolère `jean@x.fr (Jean)` , `Jean Dupont jean@x.fr` (sans chevrons), détecte l'adresse dans le texte libre, nom = commentaire | Coller une liste « sale » depuis un autre logiciel sans erreur | `chips.js:27-37` (`parse`) n'accepte que `Nom <a@b>` ou `a@b` ; le reste devient une pastille « bad » ; `address.go:60-66` renvoie `ErrInvalidAddress` | PARTIEL | Rendre `parse` de chips.js tolérant : (1) si pas de `<>`, chercher le dernier mot `\S+@\S+`, le reste = nom ; (2) `(texte)` après l'adresse = nom. Même tolérance dans `ParseList`. P2 |
| Regex EMAIL (chips.js:8) | — | Valider la pastille | `/^[^\s@<>,;"]+@[^\s@<>,;"]+\.[^\s@<>,;"]+$/` exige un point dans le domaine : rejette `root@localhost` | PARTIEL | Accepter un domaine sans point (adresses locales d'un intranet). P3 |

---

## 2. Bibliothèques fournies (core-js, @babel/runtime, externals)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| core-js internals (propriétés, descripteurs, to-length, is-forced, classof, iterators-core, own-keys, species-constructor, object-keys, etc.) 16018-16470, 17633-18083, 20174-20358 | Polyfills ES (une cinquantaine de modules) | Compatibilité anciens navigateurs | aucun (JS moderne natif) | SANS OBJET | Polyfills vendus |
| core-js-pure Promise (es.promise, all-settled, any, finally, aggregate-error, species…) 16470-16620, 20783-21980 | Polyfill `Promise` | idem | aucun | SANS OBJET | Polyfill |
| core-js string/regexp (es.string.match/split/search/link/iterator, regexp-flags, sticky helpers, get-substitution, string-multibyte) 16841-17030, 17852-17890, 18055-18138, 21943-22047, 23938-23980 | Polyfills String/RegExp | idem | aucun | SANS OBJET | Polyfill |
| core-js array/dom (es.array.iterator, es.array.filter, web.dom-collections.iterator, dom-iterables) 16776-16834, 17936-17994, 17890-17929, 20698, 23912 | Polyfills Array/NodeList | idem | aucun | SANS OBJET | Polyfill |
| core-js-pure set-timeout / set-interval (stable) 20742, 22147 (+ 16226-16258 task/timers) | `setTimeout` / `setInterval` avec arguments | idem | aucun | SANS OBJET | Polyfill |
| @babel/runtime-corejs3 (create, set-prototype-of, for-each, keys, reduce, reverse, slice, define-property, instance/map, instance/keys) 22551-22705, 18610, 23667 | Helpers de transpilation Babel | Compilation des classes | aucun | SANS OBJET | Build |
| `window.Jua` (17037-17041) | Bibliothèque d'envoi de fichiers : bouton, zone de dépôt, XHR avec progression, annulation | Joindre des fichiers (clic, glisser-déposer) avec barre de progression | `compose.js:92-153` (XHR, progression en %, annulation, dépôt sur la page, zone `#cp-drop`) | OK | Fonction présente ; l'affichage diffère (voir §10 ComposeAttachment) |
| `window.Autolinker` (19961-19967) | Lien automatique des URL et adresses e-mail dans les messages texte | Rendre cliquables `http(s)://…` et `adresse@x.fr` (usage lu dans `findEmailAndLinks`, ligne 2606 : `urls:true, email:true`, URL sans `http` ignorées, `newWindow:true`, pas de téléphone/hashtag) | `internal/mailview/text.go:11-48` : seules les URL `https?://` sont liées (`urlRe` ligne 11), `target=_blank rel=noopener noreferrer nofollow` | PARTIEL | Les adresses e-mail ne deviennent pas des liens `mailto:`. Ajouter dans `linkify` une regex e-mail (`[\w.+-]+@[\w-]+(\.[\w-]+)+`) produisant `<a href="mailto:…">` (le clic est déjà intercepté par `static/mailto.js` pour ouvrir la composition). P2 |
| `window.ifvisible` (20570-20576) | Détection onglet visible/inactif, déclenche la relève quand l'onglet est réactivé, idle 10 s (Magics ifvisibleIdle10s=10, usage 35152 hors tranche) | Rafraîchir la liste quand on revient sur l'onglet ; pause des requêtes en arrière-plan | `static/poll.js:77` (`visibilitychange` → poll) et `poll.js:42` (pas de poll si caché après 3 échecs) | PARTIEL | Le poll met à jour compteurs et titre, mais **ne recharge pas la liste des messages** ouverte (nouveau mail invisible tant qu'on n'actualise pas). Voir « À AJOUTER » n°3 (rafraîchissement de la liste). P1 |
| Pikaday (18667-19958) + binding `pikaday` (ligne 790, hors tranche) | Calendrier déroulant sur un `<input>` | Choisir une date (composant `Date`, template `Views/Components/Date.html`) | `contacts.html:46` utilise `<input type="date">` natif | SANS OBJET | Le composant `Date` n'est utilisé dans aucun template des vues User (grep `Components/Date` / `name: 'Date'` sans résultat) ; l'anniversaire des contacts est couvert par le champ natif |

---

## 3. Stores/User/Template.js (17044-17102) : modèles de messages

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| TemplateUserStore ctor : `templates` (17057) | Liste observable des modèles | Garder la liste des modèles de réponse | `web/templates_settings.go` + `templates/templates.html:5-16` (liste Nom + aperçu, éditer, supprimer avec confirmation) | OK | — |
| `templates.loading` (throttle 100 ms, 17059) | Indicateur de chargement | Spinner pendant le chargement | aucun (rendu serveur, pas d'attente) | SANS OBJET | Rendu côté serveur |
| `templatesNames` (throttle 1000 ms, 17063) | Liste de noms triable | Alimenter le menu « Insérer un modèle » en composition | `compose.html:67-68` (menu déroulant `data-template`), `compose.js:221-248` (insertion au curseur) | OK | — |
| `subscribers` + tri par glisser (17075-17090, code de tri **commenté** : `Remote.templatesSortOrder`) | Recalcule les noms ; tri par drag non actif dans cette version | — | aucun | SANS OBJET | Fonction désactivée dans RainLoop 1.17 |

---

## 4. Model/Attachment.js (17105-17626) : pièces jointes d'un message reçu

Rendu RainLoop (template `MailMessageView.html:344-395` + CSS `app.min.css` `.attachmentItem`) : cartes blanches 60 px de vignette + nom (ellipsis) + taille grise, ombre `0 1px 5px rgba(0,0,0,.1)`, coin arrondi 3 px, icône par type de fichier (police d'icônes), survol de la vignette = overlay sombre avec œil (aperçu) ou « play » (audio), case à cocher en haut à droite (mode sélection par roue crantée), bouton « tout télécharger en ZIP ».
Notre rendu : `templates/base.html:298-307` + `static/mail.css:192-198` (même gabarit de carte : 4,286 rem × 4 rem, ombre `0 1px 5px rgba(0,0,0,.2)`, nom + taille), aperçu œil `mail.css:485-488`.

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `staticFileType` (17110-17175) | Classe un fichier : image, audio, vidéo, code, eml, texte, html, archive, pdf, certificat, bin cert., Word, tableur, présentation, par extension et type MIME | Choisir l'icône et les capacités d'aperçu | `web/mail.go:829-839` (`attachmentExt`, simple extension en majuscules, 4 car. max) et `mail.go:1060-1075` (`previewKind` : image/pdf/audio/text par détection de contenu) | PARTIEL | Pas de classification par type pour l'icône (voir `staticIconClass`). P3 |
| `staticIconClass` (17183-17238) | Icône par type : `icon-file-text` (txt, eml, Word), `icon-file-code` (html, code), `icon-file-image`, `-music`, `-movie`, `-zip`, `-certificate`, `-excel`, `-chart-graph` (présentation), pour PDF : texte « pdf » dans un cadre vide | Reconnaître d'un coup d'œil le type de pièce jointe sans vignette | `base.html:301` : sans vignette, affiche l'extension en gros caractères gris (`mail.css:197`, `.ext`), ou l'icône « doc » générique si pas d'extension | PARTIEL | Visuellement différent : RainLoop montre un pictogramme par type, wmail-go le texte `.PDF`/`.ZIP`. Ajouter une table extension→symbole SVG (sprite `icons` de `base.html:24-55`) et un champ `Icon` dans `AttachmentView` (mail.go:94). P3 |
| `staticCombinedIconClass` (17246-17297) | Icône trombone de la **liste** de messages, remplacée par l'icône du type si toutes les pièces jointes sont du même type | Voir dans la liste si le message contient « un PDF », « une image »… | `templates/home.html` (ligne de liste) : à vérifier | PARTIEL | Je n'ai pas relu la ligne de liste complète dans cette tranche ; l'icône attachement existe (`base.html:38`, symbole `i-attach`), pas de variante par type. P3 (icône par type en liste) |
| AttachmentModel ctor (17300-17325) | Champs : checked, mimeType, fileName, taille, isInline, isLinked, isThumbnail, cid, download, folder, uid, mimeIndex, framed | Modèle de données | `mail.go:90-100` (`AttachmentView` : Name, Type, Size, URL, Ext, Thumb, PreviewURL, PreviewKind) | OK | Les cid/linked sont gérés côté serveur (images en ligne) |
| `newInstanceFromJson` / `initByJson` (17333-17418) | Lit le JSON serveur, calcule taille lisible, extension, type | Initialisation | `mail.go:925-941` (boucle `m.Attachments`) | OK | — |
| `isImage` (17424) | Type image | Choisir l'aperçu image | `mail.go:1063-1064` (png, jpeg, gif, webp) | PARTIEL | RainLoop traite aussi bmp ; wmail-go ne prévisualise pas bmp/svg/tiff (volontaire côté sécurité pour svg). P3 |
| `isMp3` / `isOgg` / `isWav` (17432-17450) | Audio lisible par jPlayer | Lecture en ligne de l'audio | `mail.go:1067-1068` (kind « audio »), mais ouverture dans un nouvel onglet (`preview.js` ne traite que `kind=image`) | PARTIEL | RainLoop : bouton « play » dans la vignette, lecture dans la page. wmail-go : ouvre le fichier brut. Ajouter une `<audio controls>` dans le visionneur (`preview.js`) pour `data-kind=audio`. P3 |
| `hasThumbnail` / `linkThumbnail` / `linkThumbnailPreviewStyle` (17456-17531) | Vignette serveur en `background:url()` du carré d'icône | Aperçu visuel des images jointes | `mail.go:933-939` (vignette `data:` générée par `mailview.Thumbnail`, limitée à `maxThumbnails` et `maxThumbSource`) ; `base.html:301` | OK | Limites de nombre/taille : dernières images n'ont pas de vignette (icône extension). Acceptable |
| `isText` (17464) | Texte, eml, certificat, html, code | Aperçu texte brut du serveur (`linkPreviewAsPlain`) | `mail.go:1069-1072` : seulement `text/plain`, `text/csv`, `text/markdown` | PARTIEL | .eml, .html, .log, .json, .php/js/css, .asc non prévisualisables. P3 (étendre `previewKind` au contenu texte détecté) |
| `isPdf` (17472) | PDF | Aperçu PDF dans le navigateur | `mail.go:1065-1066` kind « pdf » ; œil `base.html:303` ouvre `?inline=1` en nouvel onglet | OK | — |
| `isFramed` (17480) | Aperçu Google Docs (Word/Excel/PPT) | Prévisualiser Office via Google | aucun | SANS OBJET | Service Google externe (`AllowGoogleSocial`), envoi du contenu à un tiers : à ne pas reproduire |
| `hasPreview` (17488) / `linkPreviewMain` (17553-17573) | Choisit s'il y a un œil et vers quelle URL (image/pdf inline, texte « as plain », framed) | Bouton aperçu | `base.html:303` (`PreviewURL` si `previewKind` ≠ vide) ; `preview.js:36-46` (visionneuse modale pour les images avec ← → Échap, légende `nom (i/n)`) | OK | wmail-go fait mieux pour les images (navigation entre images) |
| `hasPreplay` (17496) | Lecture audio dans la page selon les formats supportés | idem isMp3 | voir isMp3 | PARTIEL | cf. ci-dessus P3 |
| `linkDownload` / `linkPreview` / `linkFramed` / `linkPreviewAsPlain` (17504-17547) | Construction des URL serveur | — | `mail.go:925` (`/mail/attachment?folder=&uid=&index=`), `&inline=1` | OK | — |
| `generateTransferDownloadUrl` + `eventDragStart` (17579-17603) | `dragstart` : `dataTransfer.setData('DownloadURL', 'mime:nom:URLabsolue')` sur chaque `<li draggable>` | **Glisser une pièce jointe vers le bureau / un gestionnaire de fichiers** pour l'enregistrer (Chrome/Chromium) | aucun (`grep DownloadURL static/*.js` vide ; `base.html:300` : `<a class="att">` sans `draggable` géré) | MANQUANT | Ajouter dans `preview.js` (ou nouveau `attdrag.js`) : `document.addEventListener("dragstart", e => { var a = e.target.closest("a.att"); if (a && e.dataTransfer) e.dataTransfer.setData("DownloadURL", type+":"+name+":"+new URL(a.href,location).href) })` ; exposer `data-type` et `data-name` sur `a.att` (`base.html:300`). Chrome seulement. P3 |
| `iconClass` / `iconText` (17609-17619) | Accesseurs pour le template | — | voir staticIconClass | PARTIEL | cf. ci-dessus |
| Case à cocher par pièce jointe + « télécharger la sélection en ZIP » (`checked`, ligne 17301 ; template 344-395, mode sélection via roue crantée) | Sélection multiple de pièces jointes, ZIP des sélectionnées | Ne télécharger qu'une partie des fichiers | `base.html:304-305` : roue crantée → un seul lien « ZIP » pour **toutes** les pièces (`mail.go:944-946`) | PARTIEL | Ajouter des cases à cocher dans `.att` en mode sélection et un paramètre `index=1,3` à `/mail/attachments.zip`. P3 |
| Style de la carte : coin arrondi 3 px, état `.checked` (bordure bleue), `.error` (rouge), `.waiting` (opacité .6) | Fidélité visuelle | — | `mail.css:194-196` : pas de `border-radius` | PARTIEL | Ajouter `border-radius:3px` à `.att` et `.cp-atts li` pour coller à RainLoop. P3 |

---

## 5. Component/Checkbox.js (18141-18168)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| CheckboxComponent (18156-18166), template `Views/Components/Checkbox.html` | Case à cocher « icône » (glyphes `icon-checkbox-checked/unchecked` de la police d'icônes RainLoop), focusable (`tabindex=0`), Espace bascule, libellé facultatif | Cases cohérentes avec le thème (réglages, popups, liste) | Cases **natives** du navigateur : `mail.css:134-136` (`.chk input` 1 rem), `home.html:9` ; réglages `<label class="check"><input type=checkbox>` | PARTIEL | Rendu différent (case native bleue/grise du navigateur vs carré clair de la police d'icônes sur fond DarkShine). Appliquer `appearance:none` + bordure 1 px, fond clair, coche en `::after` dans `app.css`/thèmes pour `input[type=checkbox]`. P2 (cases de la liste visibles tous les jours) |
| AbstractRadio (23618-23660) : `values`, `inline`, `readOnly`, `click` (désactivé si readOnly) | Boutons radio « icône » (`icon-radio-checked`) | Choix exclusifs (options de réglages) | `<input type=radio>` natifs dans les réglages | PARTIEL | Même remarque que la case à cocher : style natif. P3 |

---

## 6. Common/HtmlEditor.js (18171-18603) : éditeur de composition (CKEditor)

Éditeur chez nous : `static/editor.js` (contenteditable + `execCommand`), barre `compose.html:82-90`.

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ctor (18216-18246) : élément, onBlur, onReady, onModeChange, resize throttlé 100 ms | Crée l'éditeur dans un conteneur | Initialisation | `editor.js:3-20` (ids `cp-editor`, `cp-fmt`) | OK | — |
| `runOnBlur` / `blurTrigger` (délai 200 ms) / `focusTrigger` (18250-18271) | Appelle le callback au blur de l'éditeur (déclenche notamment le calcul de « brouillon modifié » / redimensionnement) | Détecter la sortie de l'éditeur | aucun listener blur ; l'état « modifié » vient de l'événement `input` du formulaire (`compose.js:167`) | OK | Équivalent fonctionnel via `input` |
| `isHtml` (18277) | Mode WYSIWYG vs texte | Savoir quel format envoyer | `editor.js:12` (`html` booléen) + `formdata` (`editor.js:23-26`, champ `is_html`) | OK | — |
| `clearCachedSignature` / `setSignature(signature, html, insertBefore)` (18285-18311) | Insère/remplace la signature au bon endroit (avant ou après la citation selon l'option de l'identité) ; remplace l'ancienne si on change d'identité | Signature liée à l'identité | Initial : `web/compose.go:344,349,393` (`withSignature(..., ident.SignatureBefore)`) ; changement d'identité : texte `compose.js:52-69`, HTML `editor.js:134-156` | PARTIEL | Au **changement d'identité** les deux scripts ajoutent la signature toujours **en fin** (`compose.js:64`, `editor.js:151`) et ignorent l'option « signature avant la citation » (`templates/identities.html:37`). Exposer `data-signature-before` sur chaque `<option>` (`compose.html:32`) et insérer avant le premier bloc citation (`blockquote` / ligne `>`) quand il vaut 1. P3 |
| `checkDirty` / `resetDirty` (18318-18326) | « Contenu modifié depuis le dernier enregistrement » | Avertir à la fermeture, brouillon auto | `compose.js:15,117,167,195,211-217` (drapeau `dirty`, `beforeunload`, confirmation à la croix) | OK | — |
| `getData(wrapIsHtml)` (18333-18352) | Récupère le HTML ; à l'envoi enveloppe dans `<div data-html-editor-font-wrapper="true" style="font-family: arial, sans-serif; font-size: 13px;">…</div>` | Que le destinataire voie le texte avec la même police que dans l'éditeur (Arial 13 px) | `editor.js:25` envoie `editor.innerHTML` brut ; `internal/compose/html.go:136-138` (`wrapHTML` : `<body>` nu, aucun style) ; l'éditeur affiche Arial 13 px (`mail.css:346`) | PARTIEL | L'aspect à l'écran (Arial 13 px) diffère du message reçu (police par défaut du client du destinataire, souvent Times 16 px). Envelopper le corps dans `<div style="font-family: arial, sans-serif; font-size: 13px;">…</div>` dans `wrapHTML` (ou `PrepareHTML`). P2 |
| `getDataWithHtmlMark` (18359-18365) | Préfixe `:HTML:` utilisé pour stocker brouillons/signatures | Interne | aucun (champ `is_html` séparé) | SANS OBJET | Détail de stockage RainLoop |
| `modeToggle(plain, resize)` (18367-18384) | Bascule WYSIWYG ⇄ texte brut | Passer HTML ⇄ texte | `editor.js:63-75` (bouton `#cp-mode`, confirmation si du contenu mis en forme risque d'être perdu) | OK | — |
| `setHtmlOrPlain` / `setHtml` / `setPlain` (18386-18437) | Charge un contenu ; `setHtml` supprime les `<p></p>` vides ; efface la signature en cache | Pré-remplir une réponse / un brouillon | côté serveur (`compose.go`, `{{.HTML}}` dans `compose.html:91`) | OK | — |
| `replaceHtml` (18411-18418) | Remplace un marqueur `[img=ID]` par `<img src="data:…">` après lecture du fichier déposé | Image déposée dans l'éditeur → intégrée | `editor.js:28-55` (collage et dépôt d'images png/jpeg/gif/webp ≤ 3 Mo en `data:`, `insertImage`) | OK | Le serveur incorpore les images au message (`compose/html.go`, `PrepareHTML`) |
| `init` : config CKEditor (18440-18544) : bouton Source/bidi activables par l'admin, `enterMode = BR`, `shiftEnterMode = P`, langue de l'UI | Entrée = `<br>`, Maj+Entrée = paragraphe | Comportement de la touche Entrée | contenteditable standard : Entrée crée un `<div>` (Chrome) | PARTIEL | Différence invisible à l'écran mais HTML envoyé différent (`<div>` au lieu de `<br>`). Forcer `document.execCommand("defaultParagraphSeparator", false, "br")` n'existe pas ; alternative : intercepter Entrée et insérer `insertLineBreak`. P3 |
| Événement `key` Tab (18472-18478) | Tab ne produit pas de tabulation : le focus quitte l'éditeur | Naviguer au clavier | contenteditable : Tab déplace le focus par défaut | OK | — |
| Événements `mode`, `focus`, `blur` (18480-18494) | Remonte le changement de mode | Mise à jour de l'UI (barre d'outils texte/HTML) | `editor.js:14-19` (`show()`) | OK | — |
| Événement `drop` (18496-18517) | Image déposée lue en dataURL | idem replaceHtml | `editor.js:49-55` | OK | — |
| `instanceReady` : retire « Couper/Copier/Coller » du menu contextuel (18519-18536) | Menu contextuel de l'éditeur sans ces entrées | — | menu contextuel natif du navigateur | SANS OBJET | Détail CKEditor ; le navigateur fournit son menu |
| `focus` / `hasFocus` / `blur` (18547-18574) | Gestion du focus | Placer le curseur au début d'une réponse | `editor.js:158-164` (curseur en haut de la réponse), `compose.js:23` (mode texte) | OK | — |
| `resizeEditor` (18576-18583) | Remplit la hauteur disponible | Zone de texte qui suit la fenêtre | `mail.css:346` : `min-height:22rem; height:55vh` | PARTIEL | Hauteur fixée en `vh`, pas ajustée au reste de l'interface. P3 |
| `setReadOnly` (18585-18592) | Verrouille l'éditeur pendant l'envoi | Éviter une saisie pendant l'envoi | `compose.js:165` désactive seulement le bouton Envoyer | PARTIEL | Rendre l'éditeur `contenteditable=false` / textarea `readonly` pendant l'envoi. P3 |
| `clear` (18594) | Vide le contenu | Nouveau message | côté serveur | OK | — |

Barre d'outils CKEditor (config hors tranche) : notre barre `compose.html:83-89` propose police, taille, gras/italique/…, couleurs, listes, alignements, lien, effacer la mise en forme ; non vérifié bouton par bouton face à la config RainLoop (hors tranche).

---

## 7. External/Opentip.js (18617-18660) : info-bulles

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Style `rainloop` (18631-18648) | Bulle blanche, bordure 1 px `#999`, rayon 2 px, sans ombre, apparition en fondu 0,2 s, délai 0,2 s, au survol ou au clic, retirée du DOM à la fermeture | Info-bulle sur tous les boutons de la barre d'outils, icônes, etc. | Info-bulle **native** `title=` partout (`base.html:97-116` …). Aucun CSS d'info-bulle (`grep tooltip static/*.css` vide) | PARTIEL | Rendu natif du système (jaune pâle / gris selon l'OS, délai ~1 s) au lieu de la bulle blanche à bordure grise. Implémenter un petit script (`tooltip.js`) : sur `mouseover` d'un `[title]`, déplacer le titre dans `data-tip`, afficher un `<div class="tip">` (fond `#fff`, bordure `#999`, rayon 2 px, `transition: opacity .2s`, délai 200 ms). P3 |
| Style `rainloopTip` (18649-18653) | Même bulle, délai 0,4 s, groupe `rainloopTips` (une seule affichée à la fois) | Éviter l'empilement | idem | PARTIEL | idem, délai 400 ms. P3 |
| Style `rainloopErrorTip` (18654-18657) | Bulle d'erreur (classe `rainloopErrorTip`, typiquement rouge) sur les champs invalides / pièces jointes en erreur de composition | Signaler une erreur de champ sans bloquer | aucun : erreurs en bandeau `#cp-status` (`compose.html:26`, `compose.js:17-20`) | PARTIEL | Pas de bulle rouge sur le bouton trombone quand une pièce échoue ; l'état « échec » est dans la liste (`mail.css:297`). P3 |

---

## 8. Common/Mime.js (19972-20137) : table extension → type MIME

Usage : `mimeContentType(nom)` (fichiers joints en composition : icône + type), `staticFileType`. Table d'environ 150 entrées.

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Table `Mime` (19984-20134) : texte (eml, txt, log, csv, ics, vcf, html, xml, json…), scripts (js, php, css…), images (png, jpg, gif, bmp, ico, svg, tif, webp, djvu…), archives (zip, 7z, rar, gz, tgz, bz2, deb, exe…), polices, audio (mp3, wav, ogg, wma, midi…), vidéo (mp4, avi, mov, mkv…), PDF/Adobe, Office (doc/docx/xls/xlsx/ppt/pptx), OpenOffice (odt/ods) | Déduire le type d'un fichier d'après son extension, même quand le navigateur n'en fournit pas | Joindre un fichier avec le bon `Content-Type` | `web/compose.go:837-838` : type = en-tête de la partie du formulaire (fourni par le navigateur) ; `internal/compose/build.go:205-213` : vide ou invalide → `application/octet-stream` ; `grep TypeByExtension` : aucun | PARTIEL | Sous Linux le navigateur envoie souvent un type vide pour .ics, .eml, .odt, .vcf… : le destinataire reçoit `application/octet-stream` (pas d'ouverture directe, ex. invitation .ics). Dans `composeAttach` (compose.go:837) : si `ct==""` ou `application/octet-stream`, essayer `mime.TypeByExtension(ext)` puis `http.DetectContentType`, puis une petite table locale (ics, vcf, eml, odt, ods, docx…). P2 |
| Utilisation côté affichage de la liste de pièces en composition (icône) | Icône par type | cf. §4 `staticIconClass` | `compose.js:80-90` (aucune icône) | PARTIEL | cf. §10 |

---

## 9. Petits stores et popups

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| FilterUserStore (20152-20163) : `capa`, `modules`, `filters`, `filters.loading/saving` (200 ms), `raw` | État de l'écran Filtres (Sieve) : liste, éditeur brut ManageSieve | Gérer règles Sieve + script brut | `templates/filters.html`, `filter_edit.html`, `filters_raw.html`, `web/filters.go`, `internal/sieve` | OK | Fonction présente côté serveur ; voir les autres tranches pour le détail de l'écran |
| AskPopupView ctor (20397-20412) : `askDesc`, boutons Oui/Non, focus, `bDisabeCloseOnEsc=true` | Boîte de dialogue de confirmation modale (texte + deux boutons, Oui sélectionné par défaut) | Confirmer suppression, abandon, vidage de dossier… | `window.confirm()` natif : `list.js:20-21` (`data-confirm`), `compose.js:163,215`, `editor.js:65` | PARTIEL | Fenêtre native de l'OS au lieu du popup thémé (fond sombre DarkShine, boutons « Oui » / « Non » libellés). Écrire un `<dialog>` générique `confirm.js` remplaçant les 4 `window.confirm` (texte = attribut `data-confirm`, boutons i18n `popup.yes`/`popup.no`, focus sur Oui, Tab/Maj+Tab/←/→ alternent, Échap = Non). P2 |
| `clearPopup` (20416-20424) | Remet textes par défaut | Interne | — | SANS OBJET | Interne |
| `yesClick` / `noClick` (20426-20440) | Ferme puis exécute le callback | Interne | — | OK | Couvert par `confirm()` |
| `onShow(askDesc, fYes, fNo, yesButton, noButton, focusYes)` (20452-20487) | Textes de boutons personnalisables (ex. « Supprimer », « Annuler ») | Boutons explicites | `confirm()` : OK/Annuler du navigateur | PARTIEL | Voir proposition `confirm.js` : attributs `data-yes`/`data-no`. P2 |
| `onShowWithDelay` (20489) | Donne le focus à « Oui » après l'animation | Entrée valide | `confirm()` natif : Entrée = OK | OK | — |
| `onBuild` raccourcis (20495-20511) : `tab, shift+tab, right, left` alternent Oui/Non ; `Esc` = Non | Clavier dans la boîte | Navigation clavier | `confirm()` natif : Tab, Entrée, Échap standards | OK | cf. `confirm.js` |
| QuotaUserStore (20536-20563) : `quota`, `usage` (en Ko × 1024), `percentage = ceil(usage/quota*100)` | Pourcentage d'occupation de la boîte | Afficher le quota | `web/mail.go:1392-1402` (`quotaView` : `int(used/limit*100)` arrondi **vers le bas**, plafonné à 100, `Full` ≥ 90 %), `templates/base.html:220` (barre de progression dans le volet des dossiers) ; CSS `mail.css:382-388` | PARTIEL | (1) Arrondi : RainLoop arrondit **au supérieur** (`ceil`), wmail-go à l'inférieur (99,5 % affiche 99). (2) Emplacement/rendu : RainLoop affiche seulement « N % » en texte dans la **barre du bas de la liste de messages** (`MailMessageList.html:209`, `visible: 0 < userUsageProc`, info-bulle avec détail), wmail-go une barre dans le volet gauche. Remplacer par `math.Ceil` et, si on veut la parité visuelle, ajouter un `<span class="e-quota">N %</span>` dans le pied de liste, `title` = « utilisé / limite ». P3 |
| `populateData` (20557) | Convertit Ko → octets | — | `mailconn` (quota IMAP) | OK | — |
| LanguagesPopupView ctor/`languages` (22411-22436) | Popup listant les langues avec drapeau, langue courante marquée, nom complet traduit | Choisir la langue de l'interface (écran de connexion et réglages) | `templates/login.html:17` (liens en ligne par langue, `<strong>` la courante) ; `settings.html:9-10` (liste déroulante) ; `internal/i18n/locales/{fr,en}.json` | PARTIEL | Fonction OK mais pas de popup à grille de langues avec drapeaux (le choix est limité à fr/en). Sans objet pour 2 langues. P3 |
| `languageTooltipName`, `setLanguageSelection`, `onBeforeShow`, `onShow`, `changeLanguage` (22440-22470) | Info-bulle anglaise, sélection, fermeture au clic | idem | `login.html:17` | OK | Équivalent fonctionnel |
| FolderSystemPopupView ctor (24027-24077) | Popup « Choisissez le dossier des envoyés/brouillons/indésirables/corbeille/archives » (liste des dossiers + « Ne pas utiliser »), enregistrement automatique 1 s après chaque choix (`SaveSystemFolders`) | Associer les rôles système aux dossiers | `templates/folders_settings.html:11-17` (formulaire des rôles) + `web/folders.go:125,231,277` (résolution automatique `mailconn.ResolveTrash`…) ; `compose.html:25` note « aucune copie conservée » si Envoyés = aucun | PARTIEL | RainLoop ouvre le popup **au moment où le besoin apparaît** (envoi sans dossier Envoyés, suppression sans corbeille, archivage sans Archive). wmail-go résout par détection (special-use) et n'a pas de popup contextuel : si un dossier système n'est pas détectable, l'action échoue ou se rabat sans proposer de choix. Au minimum, afficher un lien « choisir le dossier » vers `/settings/folders` dans le message d'erreur. P3 |
| `folderSelectList` (24038-24040) | Liste des dossiers + « Choisir un dossier » + « Ne pas utiliser » | Choix d'un rôle | `folders_settings.html:13-17` (liste de dossiers, option vide) | OK | — |
| `onShow(notificationType)` (24085-24116) | Message d'explication selon le rôle manquant | Expliquer pourquoi le choix est demandé | aucun | MANQUANT | cf. ligne ci-dessus. P3 |

---

## 10. Model/ComposeAttachment.js (23678-23823) : pièces jointes en composition

Rendu RainLoop (`ComposeAttachment.html`) : même carte que la réception (`attachmentItem`), vignette 60×56 avec icône de type, **barre de progression sur la vignette** (`iconProgress`, largeur = %), texte « NN % » au centre pendant l'envoi, nom (ellipsis), taille, croix `×` d'annulation à droite, carte semi-transparente tant qu'elle attend (`waiting`), texte rouge si erreur avec le message en info-bulle.
Rendu wmail-go : `compose.html:101-104` + `mail.css:292-298` : simple liste (nom, taille, « × »), progression en texte « N % » (`compose.js:100-102`), ligne rouge clair en erreur (`mail.css:297`).

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ctor : `id, fileName, size, isInline, isLinked, CID, contentLocation, fromMessage, tempName, waiting, uploading, enabled, complete, error` (23710-23779) | État d'une pièce jointe de composition | Suivre chaque envoi | `compose.js:80-125` (li `uploading`, `failed`, `data-id`) ; `web/attachstore.go` (`pendingAttachment`) | OK | — |
| `progressText` (23749-23753) : `''` si 0, `100` si > 98, sinon `N%` | Texte de progression | Retour visuel | `compose.js:100-102` (`Math.floor(loaded/total*100) + " %"`) | PARTIEL | Pas de plafond à 100 % pour > 98 (reste à « 99 % » pendant le traitement serveur) ; équivalent sinon. P3 |
| `progressStyle` (23754-23758) : `width:N%` | Remplissage de la vignette | Barre de progression | aucun | MANQUANT | Ajouter un fond de progression dans le `<li>` : `li.uploading { background: linear-gradient(...) }` piloté par `style.setProperty('--p', N + '%')` dans `onprogress` (`compose.js:100`). P2 |
| `title` (23759-23763) : erreur sinon nom | Info-bulle | Voir l'erreur au survol | `compose.js:105` (le message d'erreur remplace la taille dans la ligne) | OK | — |
| `friendlySize` (23764-23768) | Taille lisible | Afficher la taille | `compose.js:115` (`res.size` fourni par le serveur) | OK | — |
| `mimeType` / `fileExt` (23769-23775) | Type d'après l'extension (table Mime) | Icône | aucun | PARTIEL | cf. §4/§8. P3 |
| `initByUploadJson` (23788-23800) | Renseigne nom, taille, nom temporaire | Fin de l'envoi | `compose.js:112-118` | OK | — |
| `iconClass` / `iconText` (23806-23816) | Icône + texte « pdf » | Pictogramme dans la carte | `compose.html:102` : aucune icône | PARTIEL | Passer la liste de composition au gabarit « carte » (même rendu que `.att`, `mail.css:194-198`) avec vignette d'icône par type. P2 |
| Annulation pendant l'envoi (template : bouton `×` → `cancel`) | Interrompre un envoi | Annuler un téléversement | `compose.js:97` (`xhr.abort()`), `compose.js:123` | OK | — |

---

## 11. Model/ContactProperty.js (23826-23906)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ContactPropertyModel ctor : `type`, `typeStr` (libellé « Maison/Travail »), `focused`, `value`, `placeholder` (23858-23886) | Une ligne de propriété (e-mail, téléphone, web, anniversaire, note, etc.) | Éditer un contact champ par champ avec type | `templates/contacts.html:36-56` : champs fixes (préfixe, prénom, 2e prénom, nom, suffixe, surnom, organisation, fonction, anniversaire), e-mails et téléphones avec type (`datalist` maison/travail/…), URL, adresses postales, étiquettes, notes | OK | wmail-go couvre plus que RainLoop (adresse postale, étiquettes). Absents : propriétés RainLoop `Facebook` (90), `Skype` (91), `GitHub` (92) et `Custom` (250) (enum ligne 520-538) |
| `placeholderValue` (23887-23891) | Texte d'aide traduit | Aide à la saisie | `<label>` visibles (`contacts.html:38-46`) | OK | — |
| `largeValue` (23892-23894) : vrai si type = Note | Zone de texte multi-lignes pour les notes | Notes longues | `contacts.html:57` (`<textarea name="notes" rows="4">`) | OK | — |
| Types Facebook / Skype / GitHub / Custom | Champs additionnels du contact | Garder des identifiants de messagerie instantanée | aucun | MANQUANT | Ajouter un fieldset « Autres » (nom de champ libre + valeur) stocké en `X-` vCard (`internal/contacts/vcard.go`). P3 |

---

## 12. dev/bootstrap.js (22479-22548)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `keydown` → `killCtrlACtrlS` (22506 ; fonction ligne 2337) | Bloque **Ctrl+S** (boîte « enregistrer la page » du navigateur) partout ; bloque Ctrl+A hors champs de saisie (vide la sélection au lieu de sélectionner toute la page) | Ne pas déclencher les commandes du navigateur dans l'application | Ctrl+S intercepté seulement en composition (`compose.js:207`) ; Ctrl+A = tout sélectionner dans la liste (`static/list.js:100`) | PARTIEL | Sur la liste/message Ctrl+S ouvre « Enregistrer sous… » du navigateur. Ajouter dans `menus.js` (chargé partout, `base.html:19`) : `keydown` Ctrl+S sans Maj/Alt → `preventDefault()` hors composition. P3 |
| `unload` → `data.bUnload = true` (22507) | Évite d'afficher des erreurs AJAX parasites à la fermeture | Silence pendant le déchargement | `compose.js:211-217` | SANS OBJET | Pas d'AJAX d'état dans wmail-go (hors poll silencieux) |
| `$html.addClass('mobile' / 'no-mobile')` + `click.dropdown` → `detectDropdownVisibility` (22509) | Classe de mode mobile ; ferme/détecte les menus au clic | Menus déroulants | `static/menus.js:5-18` (un seul menu ouvert, clic extérieur / Échap ferment) | OK | — |
| Export `window.rl` (i18n, createCommand, hooks, settingsGet, EmailModel, Enums) (22510-22521) | API publique pour les plugins | Plugins | aucun | SANS OBJET | Plugins RainLoop |
| `__APP_BOOT` : attend le DOM, charge les templates `rainloopTEMPLATES`, appelle `bootstart`, sinon `fErrorCallback` (22523-22545) | Démarrage de l'application, écran d'erreur si les templates manquent | Chargement | rendu serveur | SANS OBJET | Architecture SPA |

---

## 13. Stores/AbstractApp.js (22596-22639)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `allowLanguagesOnSettings` / `allowLanguagesOnLogin` (22614-22615, 22628-22629) | Autorise le choix de langue (admin) | Réglage admin | toujours actif : `login.html:17`, `settings.html:9` | SANS OBJET | Réglage d'administration |
| `newMoveToFolder` (22616, 22630) | Style de « déplacer vers » (menu déroulant de dossiers vs ancien) | Déplacer des messages | `base.html:103-106` (menu déroulant « Déplacer vers »), glisser-déposer `static/dnd.js` | OK | — |
| `interfaceAnimation` (22617-22622, 22631) : classes `rl-anim` / `no-rl-anim` sur `<html>` (désactivé sur mobile) | Activer/désactiver les animations | Option « animations de l'interface » | `handlers.go:289,434` (`NoAnimations`), `base.html:14` (classe `no-anim`), `mail.css:467` (supprime transitions et animations) | OK | — |

---

## 14. Remote/AbstractAjax.js (22708-22971)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `defaultResponse` (22757-22818) : compteurs d'erreurs d'authentification/d'accès/de connexion (`iAjaxErrorCount`), compteur de jetons invalides (`iTokenErrorCount`) ; au-delà des limites → déconnexion/rechargement vers l'écran de connexion ; `ClearAuth` / `Logout` du serveur ; erreurs réseau retardées de 300 ms ; hook `ajax-default-response` | Quand la session expire pendant l'usage, l'application revient proprement à la connexion et signale les erreurs | Gestion de session expirée / erreur réseau | `server.go:269-275` (`auth` : redirection 303 vers `/login`) ; `compose.js:186-197` (autosave : `catch` silencieux, `r.json()` échoue sur la page de connexion renvoyée) ; `poll.js:72` (échecs comptés, arrêt si onglet caché) | PARTIEL | Si la session a expiré, l'enregistrement automatique du brouillon échoue **sans aucun message** (`compose.js:197`) : l'utilisateur croit son brouillon sauvegardé. Afficher un message (`showError`) en cas d'échec réseau ou de réponse non JSON, même en mode silencieux (hors erreur transitoire), et proposer un lien « se reconnecter ». P2 |
| `ajaxRequest` (22829-22920) : POST avec jeton `XToken`, GET, timeout 20 s, annulation des requêtes précédentes de même action (`abortActions`), mise à jour du jeton `UpdateToken`, détection « réponse en cache » (`Time`) | Éviter les réponses obsolètes, renouveler le jeton | Requêtes asynchrones sûres | CSRF par formulaire (`server.go:282-287`, champ `csrf`) ; envois uniques `saving` (`compose.js:181-184`) ; aucune annulation de requêtes obsolètes (navigation par pages) | OK | Pas de SPA : aucune requête concurrente à annuler |
| `defaultRequest` (22931-22937) | Ajoute `Action` et le hook `ajax-default-request` | Appels de l'API | — | SANS OBJET | API JSON RainLoop |
| `noop` (22943) | Appel vide pour garder la session vivante | Maintien de session | `poll.js:44,76` (poll toutes les 120 s) ; durée d'inactivité côté serveur `internal/config/config.go:93,148,257` (60 min par défaut, réglable) | OK | À vérifier : que `/mail/poll` prolonge ou non la session ; RainLoop réinitialise le délai d'inactivité sur mouvement de souris/clic/frappe (voir §15 `rl.auto-logout-refresh`) |
| `getPublicKey` (22951) | Clé publique pour chiffrer le mot de passe de connexion | Connexion | — | SANS OBJET | Spécifique RainLoop (RSA côté client) ; wmail-go utilise HTTPS + CSRF |
| `jsVersion` (22960) | Compare la version JS à celle du serveur et recharge | Mise à jour de l'application | — | SANS OBJET | Pas de SPA en cache |

---

## 15. App/Abstract.js (23185-23554) : application de base

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ctor : iframe cachée `internal-hiddden` (23252) | Cible des téléchargements | Téléchargement sans quitter la page | liens directs `/mail/attachment` avec `Content-Disposition` (`mail.go:1079`) | OK | — |
| ctor : événement `window.resize` → `window.resize.real` (23253-23265, throttle 50 ms) | Recalcule les hauteurs/largeurs | Mise en page adaptative | CSS flex (`mail.css`) | SANS OBJET | CSS suffit |
| ctor : classe `rl-ctrl-key-pressed` sur `<html>` quand Ctrl est enfoncé (23275-23283) | Le curseur/aspect change quand on glisse avec Ctrl (copie au lieu de déplacement) | Indication visuelle de la copie en glisser-déposer | `static/dnd.js:45,63` (`dropEffect = "copy"` avec Ctrl/Alt) | OK | Le curseur système indique déjà la copie ; pas de classe CSS dédiée |
| ctor : `mousemove keypress click` (debounce 5 s) → `rl.auto-logout-refresh` (23284-23286) | Remet à zéro le minuteur de **déconnexion automatique** par inactivité côté client | Déconnexion automatique après N minutes d'inactivité | Inactivité côté **serveur** : `config.go:93,148,257` (par défaut 60 min, borne `idle_minutes_max`, valeur utilisateur) | PARTIEL | RainLoop mesure l'activité réelle (souris/clavier) ; wmail-go ne voit que les requêtes : un utilisateur qui lit un long message sans requête est déconnecté, et le poll de 2 min peut au contraire prolonger la session d'un onglet oublié (à vérifier dans `session`). Ajouter un `activity.js` qui envoie un ping `/mail/poll` (ou `/session/touch`) au plus toutes les 5 min **seulement si** activité détectée, et ne plus compter le poll comme activité. P3 |
| ctor : raccourcis `esc, enter` → `detectDropdownVisibility` (23287-23289) | Referme les menus | Clavier | `menus.js:13-17` (Échap) | OK | — |
| `getApplicationConfiguration` (23303) | Lecture de la config | — | — | SANS OBJET | Interne |
| `download(link)` (23312-23336) : Chrome → clic simulé, mobile → `window.open(_self)`, sinon iframe | Télécharger un fichier | Pièces jointes, EML | liens `<a href>` simples | OK | — |
| `googlePreviewSupported` (23342) | Active l'aperçu Google des pièces Office | — | — | SANS OBJET | Service tiers |
| `setWindowTitle(title)` (23354-23363) : `titre - Titre du site` | Titre de l'onglet | Titre de la page (dossier, nombre de non lus) | `base.html:7` (`<title>Nom – page</title>`), `poll.js:12` (`(N) Nom – page` pour les non lus) | PARTIEL | RainLoop met `Dossier (non lus) - adresse - Titre` en liste (titre contextuel). wmail-go : titre statique « Nom – clé de page » + compteur global, sans nom de dossier ni adresse du compte. Ajouter le nom du dossier courant et le compte au `TitleKey`. P3 |
| `redirectToAdminPanel` (23365) | Aller au panneau d'admin | — | — | SANS OBJET | Admin |
| `clearClientSideToken` / `setClientSideToken` (23371-23388) | Jeton d'authentification côté client | — | cookie de session serveur | SANS OBJET | Architecture différente |
| `loginAndLogoutReload` (23396-23447) : déconnexion → redirection vers lien de déconnexion personnalisé ou racine ; rechargement sinon ; gère l'iframe parente | Déconnexion | Se déconnecter | `base.html:160` (formulaire POST `/logout` avec CSRF) | OK | — |
| `historyBack` (23449) | Retour arrière | — | liens `Back` (`mail.go`, `base.html:259`) | OK | — |
| `bootstart` : enregistre composants `SaveTrigger`, `Input`, `Select`, `Radio`, `TextArea`, `Date`, `x-script`, `Checkbox`/`CheckboxSimple` (variante Material si `materialDesign`) (23453-23473) | Composants de formulaire de RainLoop ; `SaveTrigger` = icône de statut « enregistré / erreur » à côté des réglages | Retour visuel à l'enregistrement d'un réglage | `messages` (`base.html:194`, bandeau « enregistré ») après POST ; aucune icône d'état par champ | PARTIEL | RainLoop enregistre chaque réglage dès le changement (coche verte qui apparaît/disparaît). wmail-go : bouton Enregistrer + bandeau. Fonctionnellement équivalent ; visuellement différent. P3 |
| Points de rupture `ssm` : mobile < 768 px, tablette 768-999, desktop 1000-1400, grand > 1401 ; classes `ssm-state-*`, `rl-desktop` (23486-23529) | Mise en page responsive, panneau gauche masqué en mobile (`leftPanelDisabled`) | Interface utilisable sur petit écran | `mail.css:209` (un seul `@media (max-width: 900px)`), autres : 40 rem, 45 rem, 30 rem | PARTIEL | Un seul point de rupture pour la vue mail (900 px) ; pas de gabarit tablette/grand écran. Sans impact pour un usage bureau unique. P3 |
| `leftPanelDisabled` / `leftPanelType` (`rl-left-panel-disabled/enabled/none/short`) (23535-23543) | Volet des dossiers masquable, mode « court » (icônes) | Gagner de la largeur | `folders.js` + `base.html:216` (bouton `data-folders-toggle`) + classe `folders-collapsed` (`base.html:14`, `handlers`) | PARTIEL | wmail-go a une bascule ; non vérifié : mode « short » (icônes seules) de RainLoop. P3 |

---

## 16. Screen/AbstractSettings.js (22974-23182) et Knoin/AbstractBoot.js (23557-23575)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ctor, `setupSettings` (23013-23040) | Prépare le menu des réglages | Menu de gauche | `base.html:75-84` (`setopen` : Général, Comptes, Dossiers, Filtres, Modèles, Contacts, Sécurité) | OK | — |
| `onRoute(subName)` (23042-23130) : crée la sous-vue à la demande, masque l'actuelle, marque l'item du menu `selected`, remonte le contenu en haut (`scrollTop(0)`), route inconnue → page par défaut | Navigation entre sections de réglages | Changer de section | pages séparées (`/settings`, `/accounts`, `/settings/folders`, …) avec `class="e-item selected"` (`base.html:77-83`) ; défaut = `/settings` | OK | Le défilement revient en haut naturellement (nouvelle page) |
| `onHide` (23133) | Masque la sous-vue | — | — | SANS OBJET | SPA |
| `onBuild` (23140-23159) : construit le menu depuis les vues déclarées, tient compte de `settings-removed` / `settings-disabled` | Sections désactivées par l'admin ou par plugin | Menu dynamique | menu statique `base.html:75-84` | SANS OBJET | Sections propres à chaque installation ; admin/plugins |
| `routes` (23161-23175) : `{subname}/`, `{subname}`, `''` | Adresses `#/settings/xxx` | Liens profonds | URL serveur | OK | — |
| AbstractBoot (23565-23575) | Classe de base vide (`bootstart` sans effet) | — | — | SANS OBJET | Squelette |

Remarque : le menu de réglages RainLoop compte aussi Thèmes, Sécurité (2FA, OpenPGP), Social, Mot de passe, Contacts, Dossiers, Filtres, Modèles, Identités, Général (autres tranches) ; ce menu est ici seulement la mécanique de routage.

---

## 17. Hors tranche (début/fin coupés)

* Fin du module `Tokenizer`/addressparser (15698-16008) : lu pour la partie visible dans la tranche et son contexte immédiat (§1).
* `./dev/Promises/AbstractBasic.js` (24123-24176) et `View/User/AbstractSystemDropDown.js` (24179+) commencent après la ligne 24000 : non analysés ici (autre tranche).

---

# À AJOUTER (MANQUANT / PARTIEL), par priorité

## P1

1. **Rafraîchir la liste des messages ouverte quand de nouveaux messages arrivent** (équivalent `ifvisible` + relève automatique de RainLoop).
   - État : `static/poll.js:41-77` appelle `/mail/poll?since=` toutes les 120 s, met à jour le titre et les pastilles de dossiers mais ne recharge pas la liste (`.lp` des messages). Un nouveau message n'apparaît qu'après un clic sur « Actualiser » (`base.html:101`).
   - À faire : quand `d.new.length > 0` ou que `d.unseen` change, et si le dossier courant est INBOX et que l'utilisateur est en page 1 sans sélection ni recherche, recharger le panneau de liste en ajax (même appel que `data-pane` / `ListNav`) ou afficher une barre « N nouveaux messages — cliquer pour afficher » en haut de la liste ; déclencher aussi un poll immédiat au retour de l'onglet (déjà fait `poll.js:77`) puis le rafraîchissement. Réduire l'intervalle à 60 s si l'onglet est visible.
   - Je n'ai pas relu `list.js` dans cette tranche pour la fonction exacte de rechargement de panneau ; à raccorder avec ce qui existe (`data-pane`).

## P2

2. **Popup de confirmation thémé au lieu de `window.confirm`** (AskPopupView 20391-20511).
   - Fichiers : `static/list.js:20-21`, `compose.js:163,215`, `editor.js:65` ; nouveau `static/confirm.js` + CSS thème.
   - `<dialog class="ask">` : texte, boutons « Oui » (focus par défaut) / « Non » (i18n `popup.yes`, `popup.no` ; libellés personnalisables via `data-yes`/`data-no`), flèches/Tab alternent le focus, Échap = Non, fond assombri `::backdrop`, style DarkShine.
3. **Envelopper le HTML envoyé dans la police de l'éditeur** (HtmlEditor.getData 18333-18352).
   - `internal/compose/html.go:136-138` (`wrapHTML`) : placer le corps dans `<div style="font-family: arial, sans-serif; font-size: 13px;">…</div>` (uniquement pour le HTML composé, pas pour les messages transférés tels quels).
4. **Pièces jointes de composition en « cartes » avec icône et barre de progression** (ComposeAttachment 23698-23819).
   - `compose.html:101-104`, `compose.js:80-125`, `mail.css:292-298` : reprendre le gabarit `.att` (`mail.css:194-198`) : vignette 4,286×4 rem avec symbole de type (extension), remplissage de progression par variable CSS `--p` mise à jour dans `xhr.upload.onprogress` (plafonner à 100 % au-delà de 98 %), nom ellipsé, taille, croix à droite, opacité .6 tant qu'en attente, rouge + `title` = message en cas d'échec, `border-radius:3px`.
5. **Tolérance de l'analyse d'adresses collées** (addressparser `_handleAddress` 15730-15880).
   - `static/chips.js:27-37` (`parse`) et `internal/compose/address.go:60-66` : accepter `Nom prenom@x.fr` (adresse repérée par `\S+@\S+`, le reste = nom) et `a@x.fr (Nom)` (commentaire = nom si pas de nom) ; accepter les domaines sans point (`root@localhost`).
6. **Liens `mailto:` automatiques dans les messages texte** (Autolinker `email:true`, ligne 2606).
   - `internal/mailview/text.go:11-48` (`linkify`) : ajouter une regex d'e-mail ; produire `<a href="mailto:…">` ; ne pas relier une adresse déjà dans une URL.
7. **Type MIME des pièces jointes d'après l'extension** (Mime.js 19984-20134).
   - `web/compose.go:837-838` : si `Content-Type` vide ou `application/octet-stream`, utiliser `mime.TypeByExtension`, puis une table locale pour .ics .vcf .eml .odt .ods .docx .xlsx .pptx, puis `http.DetectContentType` (texte seulement).
8. **Style des cases à cocher** (Checkbox component 18156-18166, template Checkbox.html).
   - `static/app.css` / thèmes (`themes/rainloop/theme.css`) : `input[type=checkbox]{appearance:none; width:1rem; height:1rem; border:1px solid #bbb; background:#fff; border-radius:2px}` + `:checked::after` coche ; focus visible ; appliquer à `.chk input` (`mail.css:136`) et aux `.check input`.
9. **Signaler l'échec de l'enregistrement automatique du brouillon** (AbstractAjax.defaultResponse 22757-22818).
   - `static/compose.js:186-198` : si `fetch` échoue, si la réponse n'est pas du JSON (session expirée → page de connexion) ou `res.error` : afficher `showError` (message i18n `compose.err.autosave`) même quand `quiet`, avec lien vers `/login` ; conserver `dirty = true`.

## P3

10. **Icône par type de pièce jointe** (staticIconClass 17183-17238) : champ `Icon` dans `AttachmentView` (`mail.go:94`), symboles SVG (texte, code, image, audio, vidéo, zip, certificat, tableur, présentation, pdf = texte « pdf ») ajoutés au sprite `base.html:24-55`, utilisés dans `base.html:301` et en liste (`staticCombinedIconClass` 17246-17297).
11. **Glisser une pièce jointe vers le bureau** (eventDragStart 17595-17603) : `dragstart` sur `a.att` avec `DownloadURL = "type:nom:url_absolue"` (`data-type`, `data-name` dans `base.html:300`).
12. **Aperçu étendu** : audio lisible dans le visionneur (`preview.js` : `<audio controls>` pour `data-kind=audio`) ; `previewKind` (`mail.go:1060-1075`) étendu aux textes .eml/.html/.log/.json/code (affichés en `text/plain`).
13. **Sélection de pièces jointes + ZIP partiel** (`checked` 17301 ; template 344-395) : cases à cocher en mode sélection, paramètre `index=` sur `/mail/attachments.zip`.
14. **Info-bulles style Opentip** (18631-18657) : script `tooltip.js` (bulle blanche, bordure #999, rayon 2 px, fondu 0,2 s, délai 0,2/0,4 s) remplaçant le `title` natif ; variante erreur rouge.
15. **Quota** (QuotaUserStore 20536-20563) : `math.Ceil` à la place de `int()` (`mail.go:1400`) ; option : afficher « N % » dans le pied de la liste de messages avec info-bulle « utilisé / limite » comme `e-quota` (`MailMessageList.html:209`).
16. **Ctrl+S bloqué sur toutes les pages** (killCtrlACtrlS 2337-2362, appelé 22506) : listener global dans `menus.js`.
17. **Signature avant/après la citation au changement d'identité** (HtmlEditor.setSignature 18300-18311) : `data-signature-before` sur `<option>` (`compose.html:32`), logique dans `compose.js:58-69` et `editor.js:141-156`.
18. **Entrée = saut de ligne `<br>`** (CKEditor `enterMode = BR`, 18462) et **éditeur en lecture seule pendant l'envoi** (`setReadOnly` 18585) dans `editor.js`.
19. **Popup contextuel des dossiers système** (FolderSystemPopupView 24027-24116) : lien « choisir le dossier » vers `/settings/folders` dans les erreurs d'envoi/suppression/archivage sans dossier résolu.
20. **Activité réelle pour la déconnexion automatique** (23284-23286) : ping d'activité débouncé (5 min) ; ne pas laisser le poll prolonger la session.
21. **Titre d'onglet contextuel** (`setWindowTitle` 23354) : `Dossier (non lus) - compte - Nom`.
22. **Divers visuels** : coins arrondis 3 px des cartes de pièces jointes (`mail.css:194`), points de rupture tablette/grand écran (23486-23529), propriétés de contact Facebook/Skype/GitHub/Custom (ContactProperty 23848), groupes d'adresses `Nom: a, b;` (OPERATORS 15889), langue : popup à drapeaux (22405), icônes d'état d'enregistrement `SaveTrigger` (23457), radios en icônes (AbstractRadio 23618).
