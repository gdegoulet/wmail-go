# Analyse RainLoop 1.17 `static/css/app.css` lignes 6000 à 11592 vs wmail-go

Portée : tout le fichier compilé `static/css/app.css` de la ligne 6000 à 11592 (surcharges Bootstrap, utilitaires `g-ui`, structure `#rl-*`, états `ssm`, dossiers, liste, lecture, contacts, composition, réglages, admin, animations) + correspondance des icônes de la police `icon-*` avec notre sprite SVG.

Méthode et limites (honnêteté) :
- Les règles CSS ont été lues en entier (compactées une règle par ligne avec un script awk en lecture seule). Les lignes citées côté RainLoop sont celles de `app.css` (parfois « ~ » quand je cite un bloc).
- Pour le rendu réel sous DarkShine j'ai lu `scratchpad/ds/out.css` (284 lignes) : DarkShine ne surcharge QUE fond d'écran, formulaire de connexion, menus `g-ui-menu` (fond blanc, survol #48525C), dossiers/menu réglages (texte blanc/gris #aaa, survol/sélection `rgba(255,255,255,.3)`), barres haut/bas de la liste (dégradé #f4f4f4 → #dfdfdf). Tout le reste vient donc de `app.css` tel quel (ex. barre de composition en `rgba(0,0,0,.8)`, liste claire, lecture blanche).
- Côté wmail-go j'ai lu : `internal/web/static/app.css` (67 l.), `mail.css` (492 l.), `themes/rainloop/theme.css` et `themes/dark/theme.css`, `templates/base.html`, `home.html`, `compose.html`, `login.html`, `accounts.html`, `contacts.html` (début), `folders_settings.html` (début), `static/dnd.js`, début de `list.js` et `folders.js`, `mailview/text.go`, `mailview/sanitize.go` (CollapseQuotes), `web/mail.go` (bodyDocHead, rows). Je n'ai PAS relu `filters.html`, `identities.html`, `templates.html`, `twofactor.html`, `settings.html` en entier, ni `compose.js`/`chips.js`/`editor.js` en détail : les verdicts qui en dépendent sont marqués « non relu en détail ».
- Les gabarits HTML RainLoop (MailMessageList.html, MailMessageView.html, SystemDropDown.html, SettingsPane.html) ont été lus uniquement pour comprendre qui rend quelle classe CSS.
- Conversions : 1 rem = 14 px chez nous (RainLoop = 14 px de base).
- « SANS OBJET » = admin, plugins, OpenPGP, réseaux sociaux/Dropbox/ownCloud/Google Drive, détection mobile serveur, bibliothèques embarquées, outils de debug.

Légende priorité : P1 = quotidien et très visible, P2 = visible/utile, P3 = cosmétique ou rare.

---

## 1. Boutons, infobulles, formulaires génériques (6000-6322)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.btn .svg-icon`, `.btn-small-small`, `.btn-thin`, `.btn-thin-2`, `.btn-narrow`, `.btn-large` (6004-6062) | Variantes de taille/padding des boutons Bootstrap | Boutons de toolbar de tailles différentes | `mail.css:9-11` (`.tbtn`, hauteur fixe 2,143rem, padding 0,929rem) ; une seule taille | PARTIEL | Les boutons « carrés » (padding 4/7 px, ex. flèches haut/bas, bouton caret de recherche) sont plus larges chez nous : ajouter `.tbtn.thin{padding:0 .5rem}`. P3 |
| `.btn-nowrap`, `.btn-ellipsis` (6018-6026) | Texte du bouton sur 1 ligne, coupé par « … » | Nom de compte long dans le bouton | `mail.css:28` (`.mb-left .btn-new` ellipsis) ; `.who` ellipsis (`mail.css:31`) | OK | — |
| `.btn.btn-transparent` + hover + état ouvert rouge (6027-6052) | Bouton sans fond, opacité .6, 1 au survol ; icône rouge `#BD362F` quand le menu est ouvert | Boutons fantômes dans les toolbars (ex. engrenage pièce jointe) | aucun (`.gear > summary` mail.css:204 est gris simple) | PARTIEL | Ajouter `.tbtn.transparent`, opacité .6→1 ; peu utilisé. P3 |
| Menu ouvert → icône/texte/caret en `#BD362F` : `.btn-group.open … dropdown-toggle`, `.dropdown.colored-toggle.open …` (6044-6051, 6165-6173) | Quand un menu déroulant de la barre d'outils est ouvert, son bouton passe en rouge brique (icône + caret) ; animation `bounce-me` de l'icône à l'ouverture (11400) | Retour visuel « ce menu est ouvert » | grep `\[open\]` dans `mail.css` : seulement `.msg-info[open]` et `dialog.viewer`. Aucun style `details.dropdown[open] > summary` | MANQUANT | `details.dropdown[open] > summary.tbtn{color:#bd362f}` + `svg` en `currentColor` (déjà). Visible tous les jours (dossier, copier, plus, compte). P2 |
| `.btn` base : radius 3px, `background-image:none`, bordure `rgba(0,0,0,.25)` (6069-6076) | Boutons plats gris clair | Look boutons | `mail.css:9-11` (bordure `--btn-bd`, radius .214rem) | OK | — |
| `.btn.disabled` opacité .8 + ombre (6077-6081) | Bouton inactif peu atténué | Retour « inactif » | `mail.css:13-14` : `opacity:.55` (`.tbtn:disabled`), `.tbtn.disabled` sans opacité | PARTIEL | Aligner `.tbtn:disabled{opacity:.8}` (RainLoop atténue peu). P3 |
| `html.textshadow .btn` ombre blanche 1px ; sur success/primary ombre noire (6082-6091) | Texte « gravé » | Look | `mail.css:11` (`text-shadow:0 1px 1px rgba(255,255,255,.75)`), `:15` (noire pour `.btn-new`) | OK | — |
| `.btn.btn-success` `#84AB04` (6095) | Vert du bouton « Nouveau message » / Envoyer | Couleur | `--new-bg` (`themes/rainloop/theme.css`: `#84ab04`), `mail.css:15`, `:257-258` | OK | — |
| `.btn-toolbar` marges nulles ; `.btn-group + .btn-group` marge 3 px ; `.btn-group > .btn` radius 0, `.first/.last/.single` radius 3 (6098-6164) | Groupes de boutons soudés | Alignement des groupes de la barre d'outils | `mail.css:18-23` (`.tgroup`, marge -1px, radius aux extrémités) | OK | — |
| `.btn-group { margin-right:3px }` (6159) | Espace entre groupes | Espacement toolbar | `mail.css:29` (`.mb-mid{gap:.857rem}`) | PARTIEL | Écart de 12 px chez nous contre 3 px + `&nbsp;` RainLoop (≈ 12 px) : sensiblement identique. Aucun changement. | 
| `.dropdown-menu` radius 0, ombre `0 1px 2px rgba(0,0,0,.2)` (6063, 6102) | Menus carrés avec fine ombre | Look menus | `mail.css:38-39` (radius 0, même ombre) | OK | — |
| `.tooltip` 14 px, `.tooltip-inner` max 380 px, ombre texte, `.tooltip-big` 18 px (6107-6130) | Info-bulles Bootstrap sombres rapides (data-tooltip, bulles sur tous les boutons) | Libellés des boutons icônes | Attributs `title=` natifs (`base.html:103-114`…) | PARTIEL | Info-bulle navigateur : délai ≈ 1 s, style système. Option CSS-only : `[data-tip]:hover::after`. P3 |
| `.popover` (6131-6141, 6219) | Bulles riches (aide) | Rare | aucun | SANS OBJET | Utilisé seulement par l'admin/aide. |
| `textarea, input[type=text…]` bordure #ccc, focus : fond blanc, bordure #999, sans ombre ; hauteur 20 px (6176-6200) | Champs sobres, focus discret | Look des champs | `app.css:23-27` : focus = `outline:2px solid var(--accent)` | PARTIEL | Visuel plus voyant que RainLoop. Option `input:focus{border-color:#999;outline:none}` aux endroits de type mail, en gardant un contraste suffisant (accessibilité). P3 |
| `.alert a` souligné, couleurs par type (6201-6215) | Liens dans alertes | `.msg a` | `app.css:33-36` sans règle de lien | PARTIEL | Ajouter `.msg a{text-decoration:underline;color:inherit}`. P3 |
| `.nav-tabs > li > a` (6216) | Onglets Bootstrap (admin/settings) | — | aucun (pas d'onglets) | SANS OBJET | Onglets admin. |
| `.modal-backdrop` opacité .2 (6222-6232) | Voile très léger derrière les fenêtres | Fenêtres modales sur la page | `dialog.picker::backdrop` .35 (`mail.css:419`), `dialog.viewer::backdrop` (438), `.keyhelp` et `.newfolder` SANS voile (`mail.css:370`) | PARTIEL | Ajouter un voile `rgba(0,0,0,.2)` derrière `.keyhelp`/`.newfolder` (ou les passer en `<dialog>`). P3 |
| `.popups`, `.popups .modal` marge 5 % auto, ombre `0 5px 80px rgba(0,0,0,.3)`, corps blanc ; `.modal.fade` anim. (6233-6275 ; 11451) | Fenêtres centrées, apparition par glissement de 20 px en 200 ms | Toute boîte de dialogue (compose, contacts, filtre, identité…) | Pas de modales : composition, contacts, réglages sont des pages (`compose.html`, `contacts.html`, `base.html:75`). `.keyhelp` (`mail.css:370`) et `dialog.picker` sont les seules vraies fenêtres | PARTIEL (choix d'architecture) | Choix « sans JS lourd » assumé ; conséquence : la liste n'est plus visible derrière la composition. Aucune action sauf ombre `0 5px 80px` pour `.keyhelp`. P3 |
| `.modal.loginContent .modal-body` transparent (6265) | Fenêtre de login sans fond | Connexion | `app.css`/`mail.css:234-249` | OK | — |
| `.picker.modal-dialog` z-index (6269-6275) | Sélecteur Google Drive | — | — | SANS OBJET | Google Drive. |
| `.form-horizontal.long-label` (6276-6284) | Libellé 160 px | Formulaires de réglages | `.setcontent label` blocs verticaux | PARTIEL | Les formulaires de réglage RainLoop sont en lignes « libellé à gauche / champ à droite » ; les nôtres sont empilés. P3 |
| `html.rl-mobile …` (6285-6303) | Adaptations mobile | Mobile | `@media (max-width:900px)` (`mail.css:209-221`), `@media (max-width:45rem)` (332) | OK | — |
| `.close-input-wrp` + `.close` (6304-6322) | Petite croix `×` dans le champ de recherche (visible dès qu'une recherche est active) pour la vider | Annuler la recherche en 1 clic | `form.search` (`home.html:20`) : `type=search` (croix native WebKit seulement) ; lien « Effacer la recherche » caché dans le menu avancé (`home.html:~44`) | PARTIEL | Ajouter une croix `×` (lien vers `ClearSearchURL`) dans le champ quand `.Query` non vide. P2 |

## 2. Destinataires (inputosaurus), CKEditor, utilitaires g-ui (6323-6735)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.inputosaurus-container` (6323-6343) | Boîte des destinataires : bordure #ccc, ombre interne, 99 %, focus fond blanc + bordure #999 | Champ À/Cc/Cci | `.chips-box` (`mail.css:444-446`, bordure #ccc, focus bordure bleue) | PARTIEL | Focus bleu `#6a9bd8` au lieu de gris `#999` ; mêmes dimensions. P3 |
| `.inputosaurus-container li` (6344-6379) : pastille carrée #eee, bordure #aaa, texte #555, `×` en haut à droite, sélectionnée #ddd, draggable | Adresse en « tag » gris carré | Voir les adresses séparées, les supprimer, les glisser entre champs | `.chip` (`mail.css:448-451`) : pastille arrondie bleu clair `#e6eef8`, `×`, `cursor:grab` ; glisser-déposer entre champs : `chips.js` (non relu en détail) | PARTIEL | Couleurs/forme différentes (arrondi 1rem bleu vs carré gris). Pour « identique » : `.chip{background:#eee;border:1px solid #aaa;border-radius:0;color:#555}`. P2 (compo quotidienne) |
| `li.pgp` vert (6377) | Clé PGP trouvée | PGP | — | SANS OBJET | OpenPGP. |
| `.inputosaurus-input` (6380-6395) | Champ de saisie dans la boîte | Saisie libre | `.chips-in` (`mail.css:447`) | OK | — |
| `.ui-autocomplete` z-index 2000 (6389) | Liste d'autocomplétion | Suggestions de contacts | `.ac-list` (`mail.css:411-414`), `contacts-ac.js` | OK | — |
| `.cke_chrome`, `.cke_toolgroup`, `.cke_top`, `.cke_combo*` (6397-6483) | Barre d'outils CKEditor grise `#F0F0F0`, boutons `#FBFBFB`, combo police 40 px | Barre d'édition riche | `.cp-fmt` (`mail.css:335-345`) fond `#f4f4f4`, boutons `--btn-bg`, hauteur 1,86rem | PARTIEL | Look proche (gris clair) mais jeu de boutons natif/emoji (`compose.html:~80`, ex. « 🔗 », « •≡ ») au lieu des icônes CKEditor ; fonctions couvertes dans `editor.js` (non relu en détail). P3 |
| `.cke_source` / `.cke_plain` (6444-6481) | Zone texte brut : Monaco/Menlo/Consolas 13 px/16 px, marge 10 px, pas de redimensionnement | Éditeur texte | `.cp-text textarea` (`mail.css:285`) : « Courier New », 1rem/1.45, `resize:vertical`, padding .64rem | PARTIEL | `font:13px/16px Monaco,Menlo,Consolas,"Courier New",monospace; padding:10px; resize:none`. P3 |
| `.b-compose .cke_chrome` sans bordure latérale (6484-6490 ; 10143) | Éditeur bord à bord dans la fenêtre | Mise en page | `.cp-editor` bordure complète (`mail.css:346`) | PARTIEL | Supprimer bordures gauche/droite/bas de `.cp-fmt`/`.cp-editor` : P3 |
| `.cke_wysiwyg_div` (6491-6558) : Arial 13 px/16 px, listes 40 px, `code`/`pre` fond `#f9f9f9`, `blockquote` bord gauche **2 px #444**, liens bleus/#609/rouge | Corps du message édité en HTML | WYSIWYG fidèle | `.cp-editor` (`mail.css:346-351`) : Arial 13px/1.4 ; blockquote bord `#ccc` | PARTIEL | `.cp-editor blockquote{border-left:2px solid #444;margin:5px 0 5px 5px;padding-left:5px}` ; `pre,code` fond `#f9f9f9`. P3 |
| `.cke_dialog …` (6559-6583) | Boîtes de dialogue CKEditor (lien, image) | Insérer un lien | `window.prompt` (data-prompt, `compose.html`) | PARTIEL | Fonctionnel, rendu navigateur. P3 |
| `.g-ui-user-select-allow`, `.g-ui-clearfix`, `.g-ui-link`, `.g-ui-min-height-300`, `.g-ui-100-proc-height`, `.g-ui-absolute-reset`, `.g-ui-height-100proc` (6584-6658) | Utilitaires de mise en page | Aucun besoin fonctionnel | — | SANS OBJET | Utilitaires internes. `.g-ui-link` (bleu `#336699` souligné) : voir `a{color:var(--accent)}` `app.css:6`, `.sender` `mail.css:179`, `.clearlink` 226 : OK |
| `.g-ui-menu` taille max 400×300, défilement (6616-6621) | Menus à défilement | Menu des dossiers très long | `.menu.scroll{max-height:28.6rem}` (`mail.css:41`), `max-width:21.4rem` (`:38`) | OK | — |
| `.g-ui-menu .e-link`, `.selected` fond #eee, `:hover/:focus` fond sombre texte blanc, `.disabled` gris + `not-allowed` (6622-6643) + DarkShine (`out.css`: hover `#48525C`, texte `#eee`) | Éléments de menu | États des items | `.menu a:hover` → `--menu-hover-bg:#48525c` / `--menu-hover-fg:#eee` (`themes/rainloop/theme.css`), `mail.css:42-46` ; `.current` en gras seulement (`:45`) ; désactivé : `color:#999;cursor:not-allowed` (`:46`) | PARTIEL | « élément sélectionné » = fond #eee chez RainLoop, gras chez nous. `.menu .current{font-weight:700;background:#eee}` (thème clair). P3 |
| `.g-ui-table` (6644-6655) | Mise en page tabulaire | — | `.cp-grid` (`mail.css:267`) | OK | — |
| `.g-ui-resizable-delimiter-highlight` (6659-6662, 6695) | Barre de séparation (6 px) en surbrillance pendant le glisser | Redimensionner les panneaux | `.splitter` + `.dragging .splitter.active` fond `rgba(255,255,255,.3)` (`mail.css:63-67`) | OK | — |
| `.e-pagenator .e-page` (6663-6694) : 22 px, gris `#999`, page courante 25 px `#333` soulignée 2 px | Pagination chiffrée | Pagination liste/contacts | `.pager` (`mail.css:114-117`), même tailles (1,571rem/1,786rem), courante soulignée | OK | — |
| `.settings-saved-trigger` (+ `-input`) (6698-6734) | Coche verte / croix rouge qui s'affiche à côté d'un réglage dès qu'il est enregistré automatiquement (bordure de champ verte/rouge) | Réglages sauvegardés à la volée sans bouton | Formulaire unique avec bouton « Enregistrer » : `settings.html:8-41` ; `.flashbar` vert après enregistrement (`mail.css:303`) | PARTIEL | Fonctionnement différent (envoi explicite). Pour ressembler : auto-submit `fetch` au `change` + coche verte courte. Seulement si on veut imiter ; sinon accepter. P3 |
| `.display-none` (6735) | utilitaire | — | `[hidden]` (`app.css:61`) | OK | — |
| `.e-spinner` rebond 3 points (6738-6821, `@keyframes bouncedelay`) | Écran de chargement initial « … » | Attente du démarrage de l'appli JS | Pages rendues côté serveur : pas de chargement JS | SANS OBJET | Pas d'écran de démarrage. |
| `.e-mobile-switcher`, `.e-powered` (6742-6754) | Liens « version mobile », « powered by » | Bascule mobile | — | SANS OBJET | Détection mobile serveur. |
| `.e-languages`, `.flag-name` (6755-6768) | Langue courante soulignée en pointillés ; ouvre la fenêtre de langues | Changer de langue depuis l'écran de connexion | `login.html:~17` : liste de liens `langs` (`authlinks langs`) | PARTIEL | Fonction OK, rendu : liste de liens vs « Français » pointillé + fenêtre. P3 |
| `.command.command-disabled.hide-on-disabled-command` (6822) | Cache les commandes non applicables | Boutons contextuels | `data-needs-selection` → `disabled` (`list.js:~42`) ; cache non | PARTIEL | RainLoop CACHE certains boutons (ex. « Pas spam » en dehors de Spam) ; nous : pas de bouton « pas spam ». Voir §6 (actions). P3 |

## 3. Squelette de page, panneaux, états de responsive (6825-7235)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `html/body` overflow hidden, Arial, fond #e3e3e3 + fond d'écran DarkShine `background.jpg` (6825-6845 ; out.css `.thm-body`) | Application plein écran sur fond d'écran | Fond d'écran | `app.css:4-5` (`body{background:var(--wall)}`), `themes/rainloop/theme.css` (dégradé + `background.jpg` utilisateur), `body.mailpage` (`mail.css:6`) | OK | — (le fond réel DarkShine dépend de l'image déposée par l'utilisateur) |
| `body:before` (6853-6863) | Ombre portée de 10 px au sommet de la page (`box-shadow 0 0 10px rgba(0,0,0,.6)`) | Léger effet de profondeur | aucun | MANQUANT | `body::before{content:"";position:fixed;top:-10px;left:0;width:100%;height:8px;box-shadow:0 0 10px rgba(0,0,0,.6);z-index:100}`. P3 |
| `textarea{resize:none}`, `option:disabled` (6846-6852) | Zones non redimensionnables | Look | `.cp-text textarea{resize:vertical}` (`mail.css:285`) | PARTIEL | trivial. P3 |
| `* {outline:none}` (6864) | Pas de contour de focus | Look | `app.css:27` impose un contour 2 px au focus | PARTIEL | Choix d'accessibilité volontaire. Aucune action. |
| `#rl-bg`, `#rl-content`, `#rl-center` min 600×400 (6882-6906) | Conteneurs ; taille mini | Mise en page | `body.mailpage` flex plein écran (`mail.css:6`) sans minimum | PARTIEL | Pas de taille minimale ; non gênant. P3 |
| `#rl-top` / `#rl-bottom` / `#rl-left` / `#rl-right` / `#rl-sub-left` / `#rl-sub-right` (6917-6981) | Les 3 colonnes absolues : dossiers 200 px, liste 500 px, lecture | Mise en page 3 volets | `.mailbox.three` grille 3 colonnes `var(--folders-w,14.286rem) | var(--list-w,35.714rem) | 1fr` (`mail.css:54-62`) | OK | — |
| `#rl-sub-right .b-message-view-backdrop` + `.backdrop-message` (6983-7007) | Voile noir 40 % sur le volet de lecture pendant un glisser de messages, avec « Déposez sur un dossier pour déplacer » + icône dossier | Retour visuel pendant le glisser | `body.dnd .msgrow{cursor:grabbing}` + `.folder-list li.drop-over` (`mail.css:430-431`), `dnd.js:25-30` | MANQUANT | Ajouter pendant `body.dnd` un voile `.readpane::after` + texte (clé i18n `mail.drag_hint`). P3 |
| `html.ssm-state-desktop-large` (≥1401 px) : gauche 220 / liste 500 / compose 1000 / contacts 900 ; `desktop` (1000-1400) : 200 / 400 ; `tablet/mobile` 155 / 310 (7009-7085) | Largeurs par défaut selon la fenêtre | Mise en page adaptative | Valeurs par défaut fixes 14,286rem (=200 px) / 35,714rem (=500 px) modifiables et mémorisées (`mail.css:55`, `list.js` splitters, POST `/settings/panes`) ; seul seuil 900 px (`mail.css:209`) | PARTIEL | Défaut RainLoop sous 1400 px : liste 400 px, ≥1401 : dossiers 220. Proposition : `@media (max-width:1400px){:root{--list-w:28.571rem}}` et `(min-width:1401px){:root{--folders-w:15.714rem}}` (valeurs par défaut seulement). P3 |
| `.show-on-panel-disabled`, `html.rl-left-panel-disabled/short/none` (7086-7121) | Panneau des dossiers réduit à 60 px (icônes seules) ou 10 px, texte 30 % opaque, bouton Nouveau = icône, contacts en dessous | Gagner de la place | `body.folders-collapsed` (`mail.css:397-402`, 3,3rem = 46 px), bouton `data-folders-toggle` (`base.html:~214`), `folders.js` | PARTIEL | Fonction OK ; largeur 46 px vs 60 px ; pas de mode « 10 px » ; en mode réduit RainLoop garde le bouton « Nouveau » (icône) et cache les noms de dossiers (nous : cache toute la liste, `mail.css:399`). Proposition : `3.3rem → 4.286rem` ; garder `.f` avec icône seule nécessite des icônes de dossier : P3 |
| `.ui-resizable-helper-w`, `.ui-resizable-handle:hover` fond blanc 50 % (7122-7133) | Poignée de redimensionnement visible au survol | Redimensionner | `.mailbox.resizable > .splitter:hover` fond `rgba(255,255,255,.3)` (`mail.css:65`) | OK | — (écart .3 / .5 négligeable) |
| `html.rl-no-preview-pane …` (7134-7146) | Mode « sans volet de lecture » : liste pleine largeur, message affiché à la place | Disposition « liste seule » | `.mailbox.listonly` + `Prefs.Layout="list"` (`mail.css:56`, `settings.html:20`, `web/…:304`) | OK | — |
| `html.rl-bottom-preview-pane …` (7150-7175) | Mode « volet de lecture sous la liste » : liste 300 px de haut, lecture dessous | Disposition horizontale | `Layout` n'accepte que `three`/`list` (`web/…:403-420`) | MANQUANT | Ajouter `layout=bottom` (grille 2 lignes) + option dans les réglages. P2 (dépend de l'usage de l'utilisateur ; DarkShine par défaut = côte à côte, donc P3 si l'utilisateur n'utilise pas ce mode) |
| `.visible-on-ctrl`, `.hidden-on-ctrl-btn`, `html.rl-ctrl-key-pressed …` (7177-7195) | Touche Ctrl enfoncée : l'icône « dossier » du bouton Déplacer devient « copie » (`icon-copy visible-on-ctrl-btn`) et le glisser copie | Copier au lieu de déplacer | Menu séparé « Copier vers » (`base.html:107`), Ctrl au glisser = copie (`dnd.js:44,56`) | PARTIEL | Fonction OK (menu dédié + Ctrl au glisser) ; la bascule d'icône sur Ctrl n'existe pas. P3 |
| `#rl-loading`, `#rl-loading-error`, `#rl-app`, `#rl-check`, `.rl-content-show` (7196-7225) | Écran de chargement/erreur de l'appli JS | — | — | SANS OBJET | SPA. |
| `.hide-on-mobile`, `.show-on-mobile`, `.width100-on-mobile` (7226-7237) | Utilitaires mobile | — | media queries (`mail.css:209`) | OK | — |
| `.nano … scroller-shadow-top/bottom` (7238-7270) | Ombre en haut/bas des zones défilantes quand il reste du contenu | Indication de défilement | aucun | MANQUANT | P3 (cosmétique) : `mask`/ombre interne sur `.msglist-form` ; ignorer. |

## 4. Cases à cocher / boutons radio « material design » (7274-7557)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.e-component.e-checkbox` + `.e-checkbox-icon` (7278-7296) | Case à cocher via police d'icônes (`icon-checkbox-*`) | Cases des listes et formulaires | `<input type="checkbox">` natif (`home.html:9,~84`), `.chk input{width:1rem;height:1rem}` (`mail.css:136`) | PARTIEL | Case native du navigateur au lieu des cases RainLoop (carré gris arrondi avec coche). `accent-color` ou `appearance:none` + SVG. P3 |
| `.e-component.material-design.e-checkbox …` (7304-7385) + keyframes `box-shrink`, `checkmark-expand`… (7386-7557) | Case « Material » animée, coche verte `#0F9D58` | Cases de réglages, de la liste | natif | MANQUANT | `input[type=checkbox]{accent-color:#0f9d58}` : coche verte sans animation. P3 |
| `.e-component.e-radio` (7297-7303) | Radio par police d'icônes | Choix exclusifs | `<input type=radio>` natif | PARTIEL | idem, P3 |
| `.e-component.e-select select:focus` (7274) | Focus pointillé | — | `app.css:27` | OK | — |

## 5. Menu compte (system drop down) (7558-7770)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-system-drop-down .b-toolbar` à droite, hauteur 30, padding 10/8 (7558) | Barre du haut à droite : bouton menu + nom du compte | Compte et déconnexion | `.mb-right` (`mail.css:30`), `base.html:146-147` | OK | — |
| `.accountPlace` : fond `rgba(0,0,0,.5)`, texte blanc gras 16 px, ombre noire, 29 px de haut, max 250 px, radius 4 (7623-7639) | Étiquette avec l'adresse du compte | Savoir quel compte est actif | `.who` (`mail.css:31-32`) : mêmes valeurs (1,143rem, rgba .5, gras) mais `max-width:20rem` (280 px) et pas de `border-radius` | PARTIEL | Ajouter `border-radius:.286rem` (4 px) et `max-width:17.857rem` (250 px). P3 |
| `.btn.system-dropdown` padding 10 ; bouton `icon-user` + caret ; devient `icon-spinner animated` pendant le chargement des comptes (7570 ; template SystemDropDown.html) | Bouton menu compte | Ouvrir le menu | `base.html:147` (icône `user` + `caret`) | OK | — (spinner de chargement SANS OBJET) |
| `.email-title`, `.account-item .icon-ok / .icon-user`, `.current` (7577-7656) | Dans la liste des comptes : coche verte pour le compte courant, icône utilisateur pour les autres | Changer de compte | `base.html:152` : `✓` pour le courant (`.acc-list .mark`, `mail.css:222`), pas d'icône pour les autres | PARTIEL | Ajouter l'icône `user` devant les autres comptes. P3 |
| `.counter` (7651) | Nombre de non-lus par compte (code commenté dans le gabarit) | — | — | SANS OBJET | Désactivé dans le gabarit 1.17 (commentaire HTML). |
| `.logoPlace` (7587) | Logo personnalisé (branding admin) | Branding | — | SANS OBJET | Branding admin. |
| `.audioPlace`, `.equaliser`, `@keyframes firstBar…` (7593-7770) | Égaliseur animé orange + bouton pause quand une pièce jointe audio est lue depuis le webmail | Lecture audio des pièces jointes | `mail.go:1067` classe `audio` pour le type, aucune lecture ; `poll.js:17` n'utilise AudioContext que pour le son de notification | MANQUANT | `<audio controls>` dans la visionneuse (`dialog.viewer`), égaliseur facultatif. P3 |
| Menu : « Ajouter un compte », « Réglages », « Aide », « Version mobile », « Déconnexion » (SystemDropDown.html) | Entrées du menu | Navigation | `base.html:154-161` : comptes, Réglages, Comptes, Aide clavier, Déconnexion | OK | — (« version mobile » SANS OBJET) |

## 6. Écran de connexion (7772-7945) — rendu DarkShine : boîte `rgba(0,0,0,.5)` rayon 7 px, ombre `0 2px 10px rgba(0,0,0,.5)`, texte `#eee` (out.css)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-login-content`, `.loginFormWrapper` (7772-7786) | Boîte centrée verticalement/horizontalement | Page de connexion | `.authpage` flex centré (`mail.css:235`) | OK | — |
| `.desc` titre 18 px (7790-7793) + `.thm-login-desc` blanc ombré (out.css) | Titre au-dessus de la boîte | Titre de page | `.authwrap h1` 1,29rem (`mail.css:237`), `--login-title` | OK | — |
| `.loginForm` 303 px, `.wrapper` 40/40/20 (7797-7820) | Dimensions de la boîte | Look | `.authbox` 21,64rem + padding 2,86rem 2,86rem 1,43rem (`mail.css:241`) = 303 + 40/40/20 | OK | — |
| Champs 18 px, hauteur 40 px, padding 12 (7829-7835) | Gros champs | Look | `.authbox input` 2,86rem/1,29rem (`mail.css:244`) | OK | — |
| `.input-append .add-on i` : icône mail/clé à droite de chaque champ (17 px, gris #999) ; icône de soumission `login-submit-icon` (flèche 24 px) dans le champ mot de passe (7845-7866 ; `btn-submit-icon-wrp` 7935) | Icônes dans les champs ; entrer = flèche cliquable | Aide visuelle | `login.html:7-12` : placeholders seuls | MANQUANT | Ajouter icônes `mail`/`key` (SVG) en `position:absolute;right` dans les champs et une flèche de soumission. P2 |
| `.buttonLogin` bouton « Connexion » (7867) | Bouton pleine largeur | Connexion | `.authbox button` (`mail.css:246`) | OK | — |
| `.alert` dans la boîte (7870), erreur : `.errorAnimated` secousse `login-form-shake` 400 ms + bouton en rouge gras (11311, 11380) | Retour d'erreur animé | Mauvais mot de passe | `messages` en haut (`login.html:5`) ; aucune animation | MANQUANT | `@keyframes shake` sur `.authbox` quand `.msg.error` présent (CSS `:has`). P3 |
| `.loginAfter`, `.flag-selector`, `.social-buttons`, `.language-button` (7874-7889) | Langues et connexions sociales | — | `login.html:17` langues ; social : aucun | SANS OBJET (social) / OK (langues) | — |
| `.submitting-pane.submitting:before/:after` (7903-7934) | Voile + anneau tournant blanc pendant l'envoi du formulaire | Retour pendant la connexion | aucun (rechargement de page) | MANQUANT | P3 : anneau CSS via `form:has(button:active)`… ou accepter. |
| `html.csstransitions.rl-started-trigger.no-mobile .loginFormWrapper` (11330) | Apparition en fondu et glissement de la boîte au démarrage | Look | aucun | MANQUANT | `@keyframes` d'entrée de 300 ms. P3 |
| Case « Rester connecté » `signMeLabel` (7839) | Connexion mémorisée | Persistance de session | `login.html` : aucune case ; durée de session via `Prefs.IdleMinutes/AbsoluteHours` (`settings.html:14-16`) | PARTIEL | Choix de sécurité (durée fixée dans les réglages). Voir audit connexion. P3 |
| `html.glass .thm-login …` (out.css) | Variante « verre » | — | — | SANS OBJET | Seulement si le réglage « glass » est actif (non DarkShine par défaut). |

## 7. Fenêtres modales spécifiques (7938-8440)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.popups .b-ask-content` (texte centré 18 px) (7943-7950) | Fenêtre de confirmation générique | Confirmer une action | `window.confirm` (`list.js:~15`), `.menu.confirm` pour vider la corbeille (`mail.css:228-231`), `<details class="danger">` | PARTIEL | Confirmations natives/détails au lieu d'une fenêtre ; fonction OK. P3 |
| `.b-shortcuts-content` 700 px (7951-7955) | Aide des raccourcis | `?` pour voir les raccourcis | `.keyhelp` (`mail.css:370-377`, 34rem = 476 px), `base.html:163-190` | PARTIEL | Largeur 476 px vs 700 px ; contenu complet (liste, message, composition, général). `width:min(50rem,94vw)`. P3 |
| `.b-folder-create-content`, `.b-folder-clear-content`, `.b-folder-system-content` (8200-8210) | Créer un dossier, vider un dossier, dossiers système | Gestion des dossiers | `#newfolder` (`base.html:222-235`), `.menu.confirm` (`home.html:55-66`), `folders_settings.html` rôles système | OK | — |
| `.b-filter-content` 750 px, bouton supprimer (8212-8224) | Édition d'un filtre Sieve en fenêtre | Filtres | `filter_edit.html` en page, `.filter-form` (`mail.css:358-367`) | PARTIEL | Page au lieu de fenêtre. P3 |
| `.b-languages-content` : grille de langues 180 px, langue utilisateur surlignée jaune `#fffddd`, sélection #f5f5f5 (8225-8260) | Choix de la langue | Changer de langue | `<select name="language">` (`settings.html:10`), liens (login) | PARTIEL | Grille de tuiles vs liste déroulante. P3 |
| `.b-account-add-content` (8261) | Ajouter un compte | Comptes supplémentaires | `account_form.html` | OK | — |
| `.b-template-add-content` 750 px, éditeur 300 px (8262-8270) | Modèles de message | Modèles | `templates.html` (page), `Templates` dans la composition (`compose.html:~72`) | OK | non relu en détail |
| `.b-open-pgp-*`, `.b-compose-open-pgp-content`, `.b-message-open-pgp-content` (8271-8403) | Gestion PGP | — | — | SANS OBJET | OpenPGP. |
| `.b-two-factor-content`, `.b-two-factor-test-content` (8393-8400) | Double authentification | Sécurité | `twofactor.html`, `login_2fa.html`, `recovery_key.html` | OK | non relu en détail |
| `.b-identity-content` 750 px, signature 200 px (8405-8426) | Identités | Identités multiples | `identities.html` | OK | non relu en détail |
| `.b-advanced-search-content` 750 px, labels 100 px (8427-8438) | Recherche avancée en fenêtre | Recherche multi-critères | `.adv-menu` + `.adv-fields` (`mail.css:103-106, 427-428`, `home.html:20-52`) : de, à, objet, texte, date, non lus, suivis, pièce jointe | OK | Fonction équivalente, présentation en menu déroulant au lieu d'une fenêtre. |

## 8. Panneau des dossiers (7962-8198) — DarkShine : texte blanc, grisé `#aaa`, survol/sélection `rgba(255,255,255,.3)`

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-folders .move-action-content-wrapper` (7966-7980) | Pendant le glisser : calque plein écran rayé en diagonale (opacité .05) derrière le contenu | Retour visuel du glisser | aucun | MANQUANT | P3 |
| `.b-folders .b-toolbar` (bouton Nouveau + Contacts) 30 px, padding 10/0/0/8 (7981-7985) ; `.btn.buttonCompose` (8169-8180) | Haut du panneau : « Nouveau message » + bouton contacts | Rédiger, contacts | `.mb-left` (`mail.css:27-28`, `base.html:97-98`) | OK | — |
| `.b-folders .b-footer` bas : Réglages + créer dossier (7986-7990) | Raccourcis bas | Réglages, nouveau dossier | `.folders-foot` (`mail.css:88-89`, `base.html:215-218`) | OK | — |
| `.b-content` top 58 / bottom 45 (7991) ; `.b-list-delimiter` (8016-8019) | Zone défilante ; séparateur 1 px #999 entre dossiers système et personnels | Structure | `.folders` + `.fsep` (`mail.css:70, 83`, `base.html:207`) | OK | — |
| `.e-link` hauteur 34 px, 14 px, padding 10 ; sans curseur si non sélectionnable (8025-8045) | Ligne de dossier | Look | `.f` 2,429rem (=34 px), `padding .714rem` (`mail.css:74`) | OK | — |
| `.focused-poiner` : trait blanc de 3 px à gauche + fond du dossier « ciblé au clavier » (8046, 8082-8090) | Navigation clavier dans les dossiers | Raccourcis clavier dossiers | `keys.js` (liste/message seulement ; non relu en détail pour dossiers) | MANQUANT | Focus clavier sur dossiers (`.f:focus-within{background:var(--folders-sel);box-shadow:inset 3px 0 #fff}`). P3 |
| `.inbox-star-icon` (étoile à côté de Boîte de réception pour « seulement suivis ») (8049-8067, 8123-8128) | Filtre rapide « messages suivis » | Voir les suivis | `.fstar` (`mail.css:459`, `base.html:204`, lien `?q=is:flagged`) ; étoile jaune `#e6a700` quand actif | OK | RainLoop : orange `orange`, ici `#e6a700` : négligeable |
| `.e-link.selectable` / `:hover` / `.selected` / `.focused` / `.droppableHover` (8068-8095) | États survol/sélection/dépôt (DarkShine : `rgba(255,255,255,.3)`) | Retour visuel | `.f:hover`, `.f.active` → `--folders-sel` (`mail.css:76-77`) ; dépôt : `.folder-list li.drop-over` bleu `rgba(80,130,200,.35)` (`mail.css:430`) | PARTIEL | Dépôt : bleu chez nous vs blanc translucide RainLoop. `.folder-list li.drop-over{background:var(--folders-sel)}`. P2 (rapide, visible lors du glisser) |
| `.e-link.system` texte gras gris, curseur par défaut ; `.b-folder-system-item` gras (8096-8100, 8130) | Dossiers système en gras | Hiérarchie visuelle | `.sys .f{font-weight:700}` (`mail.css:78`) | OK | — |
| `.count` / `.print-count` (nombre de non lus, gras) ; badge masqué pour dossiers système sauf boîte de réception (7970, 8101-8110) | Compteurs | Non lus | `.badge` (`mail.css:84-85`) : rond gris `#999` texte blanc ; affiché pour système et personnels (`base.html:204,~211`) | PARTIEL | RainLoop n'affiche PAS le badge sur Brouillons/Envoyés/Spam/Corbeille système (CSS 7970 `display:none !important` sur `.system .badge`, hors inbox via `print-count`) ; nous l'affichons pour tout dossier avec non lus. À vérifier pour Spam (non lus). P3 |
| `.unread-sub` gras si sous-dossier non lu (8111-8113) | Signale non lus plus bas | Arbre replié | `.folder-list li.unread-sub > .fname` (`mail.css:465`) | OK | — |
| `.e-collapsed-sign` (icônes `right-mini` / `down-mini`) (8114-8117) | Plier/déplier l'arbre | Arbre de dossiers | `.f .caret` ▸/▾ (`mail.css:392`, `folders.js:20-34`) | PARTIEL | Glyphes texte au lieu d'icônes triangulaires ; fonction OK. P3 |
| `.b-sub-folders` indentation 25/40/55/70 px (8131-8138) | Hiérarchie | Sous-dossiers | `.f.depth-1…5` (`mail.css:80-81`) 1,786/2,857/3,929/5/6,07rem = 25/40/55/70/85 px | OK | — |
| `.single-root-inbox` (8139-8160) | Boîte de réception comme racine unique | Cas IMAP où tout est sous INBOX | `foldertree.go` (profondeur) | SANS OBJET | non vérifié en détail ; sans effet visuel pour un serveur standard. |
| `.btn .btn-text-wrp`, `.buttonComposeText` masqué en panneau réduit (8166-8198) | Texte du bouton Nouveau | Look | `.mb-left .btn-new` `mail.css:28` | OK | — |
| `.e-quota` dans le pied de liste (8620-8627) : pourcentage 18 px, `cursor:help`, soulignement pointillé au survol, info-bulle usage | Occupation de la boîte | Surveiller le quota | `.folders .quota` : barre de progression + texte dans le panneau dossiers (`mail.css:382-388`, `base.html:220`) | PARTIEL | Information équivalente, emplacement/rendu différents (barre vs pourcentage seul dans le pied de la liste). P3 |

## 9. Pièces jointes (carte `attachmentItem`) (8439-8592)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.attachmentItem` (8439-8478) : carte blanche, ombre `0 1px 5px rgba(0,0,0,.2)`, radius 3, max 200 px / min 60 px | Pièce jointe sous forme de carte | Voir les pièces jointes | `.att` (`mail.css:194-196`) : mêmes ombre/dimensions (max 14,286rem) | OK | — |
| `.attachmentIconParent` 60×56 ; `.iconBG` texte de l'extension 18 px gras gris, ombre blanche (8493-8515) | Zone vignette : extension du fichier | Reconnaître le type | `.thumb` 4,286×4rem, `.ext` 1,857rem gras `#999` ombre blanche (`mail.css:197-200`) | OK | — |
| `.attachmentIcon` 36 px gris `#aaa` ; icônes typées (`file-zip`, `file-code`, `file-text`, `file-music`, `file-movie`, `file-image`, `file-excel`, `file-chart-graph`, `file-certificate`) (8528-8554 ; JS) | Icône selon le type | Reconnaître le type | `.ext` = extension texte ou icône `doc` (`base.html:~301`) ; `kind-image` pour la liste (`home.html:92`) | PARTIEL | Une seule icône générique ; seulement l'extension en texte. 6-8 icônes SVG de type (zip, code, texte, audio, vidéo, image, tableur). P3 |
| `.iconPreview` au survol (voile noir 50 % + œil / lecture) (8516-8526, 8558-8575) | Aperçu rapide d'une image/PDF/audio | Prévisualiser | `a.preview` œil sur vignette au survol (`mail.css:485-488`), visionneuse `dialog.viewer` (`mail.css:437-441, 489-492`) | OK | — |
| `.attachmentNameParent` nom + taille 12 px, fond `#fafafa`, bordure gauche #ddd (8528-8534) | Libellé | Nom/taille | `.meta`, `.name`, `.size` (`mail.css:201-203`) | OK | — |
| `.checkedParent` + `.attachmentItem.checked` (ombre bleue) + `.selection-mode` (8446-8449 ; 9430-9440) | Mode sélection : cocher des pièces jointes pour « télécharger la sélection en ZIP / enregistrer » | Téléchargement groupé choisi | `ZipURL` : « tout en ZIP » seulement (`base.html:~302`) | PARTIEL | Cases + « télécharger la sélection en ZIP ». P3 |
| `.attachmentItem.error` rouge ; `.waiting` opacité .6 ; `.iconProgress` barre de progression (8441-8460, 8503-8506) | État d'envoi (composition) | Retour de téléversement | `.cp-atts li.uploading/.failed` (`mail.css:296-297`) : texte bleu/rouge, pas de barre | PARTIEL | Ajouter une barre de progression `li.uploading::after` (largeur = %). P3 |
| `.attachmentsPlace`, `.attachmentsControls`, `.controls-handle` (engrenage) (9423-9456) | Barre grise sous l'en-tête avec les cartes et le menu de téléchargement en ZIP | Zone des pièces jointes | `.bar.attbar` (`mail.css:187, 192`), `details.gear` (`base.html:~302`) | OK | — |
| `.unselectedAttachmentsError` ombre rouge (9441) | Erreur de sélection | — | — | SANS OBJET | Lié au mode sélection. |

## 10. Liste des messages (8595-9137)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.messageList .toolbar` 30 px, `.b-footer`, `.second-toolbar` (8598-8680) ; DarkShine : dégradé `#f4f4f4 → #dfdfdf` (out.css `.thm-message-list-*`) | Barre d'outils au-dessus de la carte, barre de recherche/coche dans la carte, pied avec pagination | Structure de la liste | `.mailbar` (`mail.css:26`), `.lp-head` (`mail.css:94-95`), `.lp-foot` (`mail.css:111-112`), `--toolbar-bg` | OK | — |
| `.b-message-list-wrapper` bordure 1 px #aaa, ombre `0 2px 8px rgba(0,0,0,.2)`, radius 5 (8637-8648) | Carte de la liste | Look | `.listpane` (`mail.css:92-93`) | OK | — |
| `.inputSearch` 258 px ; `.buttonMoreSearch` (caret) (8629-8636) | Champ de recherche + bouton « avancée » | Recherche | `form.search` max 18,4rem (=258 px), bouton caret `.adv > summary` (`mail.css:97-102`) | OK | — |
| `.checkboxCkeckAll` (icône : vide / partielle / cochée) (8657-8660 ; MailMessageList.html) | Case « tout sélectionner » à trois états | Sélection de masse | `#select-all` natif, état indéterminé (`list.js:36-39`), menu `selmenu` (`home.html:10-21`) | OK | — |
| `.line-loading` + `.e-strip-animation` (rayures animées pendant le chargement de la liste) (8661-8663, 11360) | Indicateur de chargement | Attente | Pas de chargement dynamique de la liste (page rechargée) | SANS OBJET | Navigation par pages serveur. |
| `.mainDelimiter` séparateurs 1 px `#bbb` (8664-8685) | Lignes de séparation | Look | `border-bottom:1px solid #ddd` (`mail.css:94, 111`) | PARTIEL | #bbb vs #ddd. P3 |
| `.listClear` (« Vider le dossier » en lien `g-ui-link`) (8691-8696) | Vider corbeille/spam | Vider le dossier | `.listClear`, `.clearlink`, confirmation `menu.confirm` (`mail.css:225-231`, `home.html:55-66`) | OK | — |
| `.listEmptyList`, `.listEmptySearchList`, `.listEmptyListLoading`, `.listError` : texte centré 24 px gris `#999`, padding 60 px (8697-8715) ; erreur `#DA4F49` (8721) | États vides/erreur | Dossier vide, aucun résultat, erreur serveur | `<p class="muted pad">` (`home.html:99`), `.msg.error` (`home.html:~68`) | PARTIEL | Texte petit à gauche vs grand texte centré 24 px. `.msglist-empty{padding:60px 10px;text-align:center;font-size:1.714rem;line-height:30px;color:#999}`. P2 (visible à chaque dossier vide) |
| `.listDragOver` (zone « déposer un message ici » pour ajouter un `.eml`) (8716-8726) | Déposer un fichier .eml dans le dossier | Import par glisser | `.eml-drop` (`mail.css:460-461`, `emldrop.js`) | OK | — |
| `.listSearchDesc` 18 px, bordure #eee (« Résultats pour … ») (8746-8754) | Rappel du terme recherché au-dessus des résultats | Savoir qu'on est en recherche | aucun (seul `.filterbar` pour « non lus », `home.html:74`) | MANQUANT | Bandeau `Résultats pour « q »` + lien effacer au-dessus de la liste si `.Query`. P2 |
| `.listThreadUidDesc` fond #aaa texte blanc centré 16 px (« ‹ Retour à la liste ») (8755-8764) | Sortir d'un fil de discussion | Conversations | `.bar.threadbar` (`home.html:71`, `mail.css:187, 463`) : bandeau gris #eee texte lien | PARTIEL | Style barre grise vs #aaa blanc centré. P3 |
| `.messageListItem` 52 px, 12 px, interligne 21 px, fond `#f9f9f9` (8782-8800) | Ligne de message (2 lignes) | Liste | `.msgrow` 3,714rem (=52 px), `.857rem`, `line-height:1.5` (`mail.css:120-121`) | OK | — |
| `.delimiter` 1 px #999 opacité .2 ; `.wrapper` padding 5 ; `.sidebarParent` bande 6 px #eee (8802-8812, 8873) | Séparateur et barre latérale colorée | États de la ligne | `.msgrow .delimiter/.sidebar/.wrapper` (`mail.css:122-124`) | OK | — |
| `:last-child` bordure basse, `.fullThreadsParent` (8765-8780, 8777) | Dernière ligne ; bouton « tout le fil » | — | `border-bottom` chaque ligne (`mail.css:121`) ; pas de `fullThreadsParent` | PARTIEL | OK pour la bordure ; `fullThreadsParent` : lien « voir tout le fil » pour fils dans la liste : P3 |
| `.focused` fond `#f5f5f5`, barre `#ccc` (8884-8888) | Ligne ciblée au clavier | Navigation clavier | `.msgrow.focused{outline:1px dotted #888}` (`mail.css:429`) ; `:hover` fond `--row-hover` | PARTIEL | RainLoop : fond #f5f5f5 + barre latérale #ccc ; nous : contour pointillé. `.msgrow.focused{background:#f5f5f5}.msgrow.focused .sidebar{background:#ccc}`. P2 (navigation clavier quotidienne) |
| `.unseen` fond `#FFFFD9`, gras, barre orange ; focus : `#cc8400` (9049-9060) | Message non lu | Repérer les non lus | `.msgrow.unread` fond `--unread-bg:#ffffd9`, barre `--unread-bar:#ffa500`, gras (`mail.css:126-128`, thème rainloop) | OK | — (`.unread.focused` barre #cc8400 manquante : P3) |
| `.hasUnseenSubMessage` fond `#FFFFD9`, barre `#ffdb99` (focus `#cc8400`) (9040-9048) | Fil de discussion replié contenant un non lu : ligne en jaune clair | Conversations | `mail.go:~600` : `Seen: m.Seen` seulement du message en tête ; aucun indicateur de non lu dans le fil | MANQUANT | Calculer `ThreadUnseen` (si un UID du fil est non lu) et classe `.msgrow.subunread` (fond jaune, barre `#ffdb99`). P2 (si threads activés) |
| `.checked .sidebarParent` `#69a8f5`, avec focus `#217ef0` (9063-9068) | Ligne cochée | Sélection | `.msgrow.checked .sidebar{#69a8f5}` (`mail.css:129`) | OK | — |
| `.selected` fond `#DFEFFF`, barre `#398CF2`, séparateur bleu (9069-9080) | Message ouvert | Message courant | `.msgrow.selected` `--sel-bg:#dfefff`, `--sel-bar:#398cf2` (`mail.css:130-132`) | OK | — |
| `.importantMark` « ! » rouge (8891-8896, 8903) | Priorité haute | Signalement | `.imp` (`mail.css:464`, `home.html:93`) rouge `#c0392b` | OK | — (rouge `red` vs `#c0392b`) |
| `.deleted-mark` texte barré, opacité .7 (8897-8901) | Message marqué \Deleted | Messages « à supprimer » | aucun (`grep -i Deleted web/mail.go` : rien) | MANQUANT | Classe `.msgrow.deleted{opacity:.7}` + `text-decoration:line-through` si flag `\Deleted`. P3 |
| `.e-single-line` : 35 px, expéditeur 200 px à gauche, drapeau et pièce jointe à gauche (8864-8870, 8905-8925, 8940-8942) ; gabarit `MailMessageListItemNoPreviewPane` | En mode « sans volet de lecture » la liste est sur UNE ligne : [case][étoile] expéditeur | objet … date | Mode liste seule | `.mailbox.listonly` réutilise la ligne à deux lignes (`mail.css:56`, `home.html:83-95`) | MANQUANT | Mode `listonly` : `.mailbox.listonly .msgrow{height:2.5rem}` + flex en ligne (case, étoile, `senderParent` 14,286rem, `subjectParent` flex:1, date, pièce jointe). P2 (seulement si la disposition « liste » est utilisée) |
| `.checkedParent` (case) : float gauche, marge haute 11 px, padding 0/8/0/6 (8909) | Case à cocher dans la ligne | Sélection | `.checkedParent` (`mail.css:133`) 0,786rem=11 px, padding .571/.429rem | OK | — |
| `.flagParent` (étoile) à droite (8911) ; `.flagOff` opacité .5, `:hover` 1 ; `.flagOn` orange ; `.flagOnHalf` (étoile à moitié si un message du fil est suivi) (9004-9020, 9081-9091) | Suivi | Marquer / retrouver | `.star`, `.star:hover`, `.star.on` (`mail.css:137-141`) | PARTIEL | Pas d'état « demi-étoile » pour un fil contenant un message suivi (`hasFlaggedSubMessage`). P3 |
| `.dateParent` 11 px gris, à droite (8921-8924) | Date | Chronologie | `.dateParent` 0,786rem=11 px (`mail.css:138`) | OK | — |
| `.threadsParent` / `.threadsCountParent` : pastille avec le nombre (fond #eee, bordure #ccc, radius 5) ; `.threads-len-data` pastille avec icône (8926-8937, 8983-8999) | Nombre de messages du fil, clic = ouvrir le fil | Conversations | `.msgrow .threads` pastille ronde grise `#8a8a8a` texte blanc (`mail.css:462`, `home.html:93`) | PARTIEL | Fonction OK ; couleurs différentes (#eee/#ccc bordure/texte #666 vs gris foncé plein). P3 |
| `.attachmentParent` icône trombone à droite (8928-8930) | Pièce jointe | Voir qu'il y a une PJ | `.attachmentParent` (`mail.css:148`) : icône `image` ou `doc` selon le type (`home.html:92`) | PARTIEL | RainLoop : icône trombone (`icon-attachment`) ; nous : « doc »/« image » (sprite sans trombone dans la liste, `attach` existe pour la compo : `base.html:~44`). Utiliser l'icône `attach` dans la liste. P2 |
| `.senderParent` / `.subjectParent` nowrap + ellipsis ; sujet noir (8944-8963) | Texte | Lisibilité | `.senderParent`, `.subjectParent` (`mail.css:143-145`) | OK | — |
| `.emptySubjectText` italique gris (8964-8975) | « (Pas d'objet) » | Message sans sujet | `<i>` `mail.no_subject` (`home.html:93`, `.subj i` `mail.css:147`) | OK | — |
| `.subject-prefix` gris `#888` (9000) | « Re: » estompé | Lecture | `.msgrow .pfx` (`mail.css:464`) | OK | — |
| `.replyFlag`, `.forwardFlag` (9021-9028) | Icônes « répondu » / « transféré » | État du message | `.ans` + icônes `reply`/`forward` (`mail.css:149-151`, `home.html:91`) | OK | — |
| `.withAttachments .attachment` couleur #666 (9031) | Couleur de la PJ | Look | `.attachmentParent{color:#666}` (`mail.css:148`) | OK | — |
| `html.ssm-state-desktop-large … messageListItem` police 13 px (9112-9120) | Texte et date plus grands au-dessus de 1400 px | Lisibilité | `.msgrow` fixe .857rem (12 px) | PARTIEL | `@media (min-width:1401px){.msgrow{font-size:.929rem}.dateParent{font-size:.929rem}}`. P3 |
| `.messageList.message-focused …`, `.hideMessageListCheckbox` (9099-9111) | Focus de volet ; masquer les cases (option) | Réglage « cases » | `body.hide-checks` (`mail.css:466`), option `show_checkboxes` (`settings.html:35`) | OK | — |
| `.draggablePlace` (9121-9132) : pastille sombre `rgba(0,0,0,.5)` suivant le curseur pendant le glisser | Aperçu du glisser avec le nombre de messages | Retour visuel | `dnd.js:29-33` : `setData` seulement, pas de `setDragImage` | MANQUANT | `e.dataTransfer.setDragImage(el,…)` avec une pastille « N messages ». P3 |
| Animations `max-height` à l'ajout/suppression de ligne, `anim-action-class` (`highlight-folder-row`) (11420-11430) | Les lignes se replient en 400 ms quand on les supprime ; le dossier cible « pulse » | Retour visuel d'actions | pas d'animation (rechargement) ; `body.no-anim` (`mail.css:467`) | MANQUANT | P3 |

## 11. Lecture d'un message (9138-9700)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.messageView .toolbar` (9153) ; `.top-toolbar` caché en mode volet (7166) | Barre d'outils au-dessus de la lecture : fermer, éditer (brouillon), archiver, spam / pas spam, supprimer, précédent/suivant (`icon-left-middle`/`icon-right-middle`) | Actions sur le message ouvert | Pas de barre propre à la lecture : les boutons de la barre du haut agissent sur le message ouvert (`data-needs-selection`, `base.html:112-114`) ; fermer = `×` dans l'en-tête (`base.html:259`) ; précédent/suivant : `listnav.js` (non relu) | PARTIEL | Fonctionnellement couvert ; pas de « Pas spam » dans un dossier Spam, pas de flèches précédent/suivant visibles. P2 |
| `.b-content` : bordure #aaa, radius 3, blanc (9153-9170) ; DarkShine `.thm-message-view-background-color` #fff | Carte blanche de lecture à 58 px du haut | Look | `.readpane` (`mail.css:154-155`) | OK | — |
| `.b-message-view-checked-helper` : grand nombre + icône courrier quand des messages sont cochés et qu'aucun n'est ouvert (9165-9169) | « 3 messages sélectionnés » dans le volet de lecture | Retour sur la sélection multiple | `.pane-empty` « Sélectionnez un message pour le lire. » (`home.html:116`) ; compteur `select-count` (`list.js:34`) : élément non retrouvé dans les templates | MANQUANT | Afficher « N ✉ » dans `.readpane` quand N cases cochées. P2 |
| `.b-message-view-desc` texte centré 24 px gris `#999`, padding 120 px ; erreur `#DA4F49` (9170-9175) | Volet vide / erreur | État vide | `.pane-empty` 1,714rem (24 px), `margin:4rem auto`, `#999` (`mail.css:156`) ; erreur `msg error` | OK | — |
| `.message-fixed-button-toolbar`, `.infoParent` (icône « i » opacité .3→.6), `.flagParent` étoile 20 px marge (9183-9199) | Boutons d'en-tête | Détails, suivi | `.msg-info` `i` opacité .3→.6 (`mail.css:160-161`), `.starform` (`mail.css:169-170`) | OK | — |
| `.messageItemHeader` fond `#f8f8f8`, bordure basse #ddd, radius haut 5, padding 10 (9233-9245) | En-tête du message | Look | `.mh` (`mail.css:158`) | OK | — |
| `.fromPic` 50×50 : avatar de l'expéditeur (gravatar/contact), bordure #ccc radius 6 (9246-9258) | Photo de l'expéditeur | Reconnaître l'expéditeur | `.avatar` 3,571rem (=50 px) avec icône utilisateur générique (`mail.css:175-176`, `base.html:262`) | PARTIEL | Pas de vraie photo (contact/Gravatar) : P3 |
| `.subjectParent` 18 px gras, une ligne, ellipsis (9259-9268) | Sujet | Lecture | `.msg-subject` 1,286rem gras (`mail.css:171`) | OK | — |
| `.informationShort` : « de <a>expéditeur</a> à destinataires » sur une ligne, liens soulignés `#336699` ; clic = ouvre `.informationFull` (tableau De / À / Cc / Date / Taille aligné, bordure) (9269-9340) | Résumé des en-têtes avec détail | Voir destinataires/date | `.mh-who` (`mail.css:177-180`) : expéditeur (lien bleu souligné), date, « à … » ; détails complets dans `.msg-info` (popup `dl.headers`, `mail.css:164-168`, `base.html:237-248`) | PARTIEL | Contenu équivalent, présentation différente : détail ouvert via bouton « i » (flottant) au lieu d'un clic sur la ligne de résumé et d'un tableau dans l'en-tête. P3 |
| `.messageItem` zone défilante ; `.buttonUp`, `.buttonFull`, `.buttonUnFull` (carrés 30 px en bas à droite, bordure #333, opacité .5) ; `rl-desktop` : visibles seulement au survol (9345-9395, 9680-9699) | « Remonter en haut » et « plein écran » flottants | Lire en plein écran, remonter | Plein écran : `.mh-full` dans l'en-tête (`mail.css:455`, `base.html:258`, `msgfull.js`) ; pas de bouton « haut » | PARTIEL | Plein écran OK (emplacement différent) ; « Haut de page » MANQUANT (corps dans iframe défilant). P3 |
| `.loading` « chargement… » (9396) | Attente du corps | — | iframe : rendu natif | SANS OBJET | — |
| `.showImages` / `.readReceipt` / `.pgpSigned` barres cliquables : fond #eee ; accusé de réception `#ffffd9` (9410-9422) | Barres d'information | Images distantes bloquées, accusé de réception | `.bar.imgbar` (`mail.css:187-191`, `base.html:~287`), `.mdnbar` fond `#fcf8e3` (`mail.css:379`) | PARTIEL | Accusé : `#fcf8e3`+texte brun vs `#ffffd9`. P3 |
| `.rlBlockquoteSwitcher` : petit bouton `…` gris 30×14 qui replie/déplie les citations (9458-9470 ; 9503) | Masquer les citations longues | Lecture confortable | `CollapseQuotes` (`mailview/sanitize.go:235-290`) : `<details class="wm-quote"><summary>…</summary>` pour les blockquote de plus de 400 caractères ; style `bodyDocHead` (`web/mail.go:1002`) : bouton `#e8e8e8` | OK | Seuil 400 car. à comparer au seuil RainLoop (non vérifié : JS hors tranche). |
| `.bodyText` police Arial noire ; `.b-text-part a` bleu, visité `#609`, actif rouge ; `blockquote` bord gauche 2 px noir, padding 0 10 (9490-9506) | Corps du message | Lisibilité | `bodyDocHead` : `a{color:blue}a:visited{color:#609}`, `blockquote{margin:0;padding:0 10px;border-left:2px solid #000}` (`web/mail.go:1000-1001`) | OK | — (lien actif rouge : manque `a:active{color:red}`, P3) |
| `.b-text-part.html` : `body` marge 15 px, `img{max-width:100%}`, `pre/code` fond `#f9f9f9` (9507-9538) | Mise en forme du HTML | Lisibilité | `margin:15px`, `img{max-width:100%;height:auto}`, `table{max-width:100%}` (`web/mail.go:1000-1002`) ; pas de style `pre/code` | PARTIEL | `pre,code{font-family:Monaco,Menlo,Consolas,monospace;background:#f9f9f9}` + `pre{padding:5px 10px;border-radius:5px}`. P3 |
| `.b-text-part.plain` : padding 15 px, **police Monaco/Menlo/Consolas**, `blockquote` coloré par niveau : **bleu**, imbriqué **vert**, 3e niveau **rouge** ; `pre.b-plain-openpgp` (9539-9582) | Texte brut en chasse fixe avec citations colorées | Lire les mails « texte » comme dans RainLoop | `TextToHTML` (`mailview/text.go:15-36`) : un seul niveau de `blockquote` (aucun niveau imbriqué), rendu dans la police Arial et le style générique du corps (`web/mail.go:1000-1002`) | MANQUANT | (1) Détecter le niveau (`> >`) et produire des `<blockquote>` imbriqués ; (2) classe `.plain` : `font-family:Monaco,Menlo,Consolas,"Courier New",monospace;padding:15px` ; (3) `.plain blockquote{border-left:2px solid blue;color:blue}` niveau 2 vert, niveau 3 rouge. P1 (messages texte quotidiens, très visible) |
| `.b-openpgp-control` (9583-9593) | Icônes PGP | PGP | — | SANS OBJET | OpenPGP. |
| `.message-focused .b-content` ombre/bordure `#9d9d9d` (9594-9600) | Focus du volet de lecture | Look | aucun | MANQUANT | P3 |
| `.thread-controls`, `.thread-list`, `.thread-date`, `.more-threads` (9601-9633) | Menu déroulant des autres messages du fil dans l'en-tête | Naviguer dans la conversation | Fils ouverts via la liste (`ThreadURL`, `home.html:93`) ; pas de menu de fil dans la lecture | PARTIEL | Menu « fil de discussion » dans l'en-tête. P3 |
| `html.rl-resizer .b-message-view-iframe-backdrop` (9634) | Empêche l'iframe de capter la souris pendant le redimensionnement | Splitters fluides | `body.dragging iframe{pointer-events:none}` (`mail.css:67`) | OK | — |
| `html.rl-no-preview-pane .messageView .b-content` (9635-9644) ; `rl-bottom-preview-pane` (9645) | Lecture pleine page / en bas | Dispositions | `.mailbox.listonly` + page `/mail/message` ; bas : voir §3 | PARTIEL | Voir « layout bas ». |
| `html.rl-message-fullscreen …` (9661-9699) | Lecture plein écran | Plein écran | `body.msg-fullscreen` (`mail.css:456-458`), `msgfull.js` | OK | — |

## 12. Contacts (fenêtre 900 px) (9705-9985)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-contacts-content.modal` 900 px (max 700 px de haut) (9705-9718) | Fenêtre : barre noire (`rgba(0,0,0,.8)`, 40 px) + liste 220 px à gauche + fiche à droite | Gérer les contacts | Page `/contacts` dans les réglages : tableau + formulaire sous le tableau (`contacts.html:1-50`) | PARTIEL | Fonctions présentes (recherche, tags, nouveau, suppression, import/export, synchro : `contactsweb.go`, `contactssync.go`) ; présentation en deux volets absente. P2 |
| `.b-header-toolbar` noir translucide + boutons (nouveau, supprimer, importer, exporter, synchroniser, envoyer un mail) (9722-9735) | Boutons d'action | Actions | Boutons dispersés dans la page (`contacts.html:~7-14`) | PARTIEL | P3 |
| `.b-list-toolbar` recherche 200 px ; `.b-list-footer-toolbar` pagination ; `.b-list-content` 220 px (9736-9760) | Liste de contacts avec recherche et pager | Trouver un contact | `.contacts-search`, tableau paginé (`mail.css:426, 471-475`, `contacts.html:~36`) | OK | — |
| `.e-contact-item` 45 px : nom 16 px + e-mail ; barre latérale 6 px bleue si cochée/sélectionnée ; fond blanc au sélectionné (9764-9822) | Ligne de contact | Sélection | tableau (`.contacts-table`) | PARTIEL | Ligne/état de sélection en barre bleue absents. P3 |
| `.b-view-content-toolbar`, bouton « Enregistrer » vert gras quand le contact est modifié (`dirty`) (9823-9830) | Retour sur modification | Edition | bouton Enregistrer simple | PARTIEL | P3 |
| `.b-view-content` champs 18 px, bordure invisible → visible au survol/focus ; mode lecture seule (`contactValueStatic`) ; `hasError` rouge (9831-9900) | Fiche éditable en ligne | Edition | `.contact-form` (`mail.css:422-425, 476-479`) | PARTIEL | Formulaire classique avec bordures permanentes. P3 |
| `.e-share-sign`, `.e-read-only-sign`, `.shareParent` (9800-9900) | Partage de carnet | — | — | SANS OBJET | Partage/serveur CardDAV de RainLoop. |
| `html.rl-mobile .b-contacts-content …` | Mobile | — | `@media` | OK | — |

## 13. Composition (10076-10242)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-compose.modal` 850 px, marge 10 px auto (10076-10084) ; DarkShine : ombre `0 5px 80px` générique | Fenêtre « Écrire » par-dessus la liste (voile 20 %) | Composer | Page `compose-page` + `.compose` max 60,7rem (=850 px), ombre `0 2px 10px rgba(0,0,0,.5)` (`mail.css:254-255`) | PARTIEL | Largeur identique ; page séparée (la liste n'est plus visible derrière). Voir §1 (modales). P3 |
| `.b-compose .b-header-toolbar` 40 px, **fond `rgba(0,0,0,.8) !important`, texte blanc**, boutons Envoyer (vert) / Enregistrer / Fermer / Supprimer, texte « enregistré à… » (10107-10125) | Barre noire translucide en haut de la fenêtre | Actions de composition | `.cp-toolbar` : fond clair dégradé `#f4f4f4 → #dfdfdf`, bordure #ccc (`mail.css:256-262`) ; boutons Envoyer vert `#84ab04`, Enregistrer gris, Supprimer rouge `#bd362f` | PARTIEL | Écart visuel net : RainLoop = barre sombre. `.cp-toolbar{background:rgba(0,0,0,.8);border-bottom:0;color:#fff}`, `.cp-saved{color:#ddd}`, `.cp-close{color:#fff;opacity:1}`. P1 (écran quotidien, très visible ; à confirmer par capture si DarkShine est utilisé tel quel) |
| `.b-header` fond `#eee`, `.e-row` 30 px, `.e-label` à droite min 70 px, `.e-value` champs 98 % (10127-10170) | En-tête De / À / Cc / Cci / Objet | Saisie des destinataires | `.cp-grid`, `.cp-label` 5,7rem, `.cp-value` (`mail.css:267-276`) ; zone sur fond blanc `.cp-body` | PARTIEL | RainLoop : bloc d'en-tête gris `#eee` ; nous : fond blanc de la fenêtre. `.cp-body` partie haute `background:#eee`. P2 |
| `.e-identity` (identité en gras, pointillé si plusieurs) (10145-10160) | Choix de l'expéditeur | Identités | `<select name="identity">` (`compose.html:~33`) | PARTIEL | Liste déroulante au lieu d'un lien. P3 |
| `.error-desc`, `.error-to` rouge gras (10172-10173) | Erreur sur destinataire | Validation | `.chip.bad` (`mail.css:449`), `#cp-status` (`compose.html:27`) | OK | — |
| `.b-appachments .b-attacment…` liste de téléversement avec progression (10174-10192) | Suivi des envois | Pièces jointes | `.cp-atts` avec taille / « uploading » / « failed » (`mail.css:292-298`) | PARTIEL | Voir §9 (barre de progression). P3 |
| `.attachmentAreaParent` fond `#ddd`, cartes `attachmentItem`, `.no-attachments-desc` 24 px (10085-10100) | Zone des pièces jointes (onglet) | Joindre | `.cp-files`, `.cp-atts` liste simple, `.cp-noatt` (`mail.css:286-299`) | PARTIEL | Cartes vs liste. P3 |
| `.b-attachment-place` : surcouche pointillée 2 px #777 « Déposez ici » en 24 px sur toute la fenêtre pendant un glisser de fichier (10226-10236) | Dépôt de fichiers | Glisser-déposer | `.cp-drop` zone fixe pointillée (`mail.css:288-289`, `compose.js`) | PARTIEL | Zone dédiée dans l'onglet vs surcouche plein écran. `compose-page.dragover::after`. P3 |
| `.b-compose.loading .b-header-toolbar` rayures animées (11456) | Chargement | — | — | SANS OBJET | Attente d'initialisation JS. |
| `.minimize-custom` (10108) | Réduire la fenêtre | — | — | SANS OBJET | Style présent mais bouton non employé dans les gabarits que j'ai lus (non vérifié dans PopupsCompose.html). |

## 14. Administration (10243-10615) — SANS OBJET

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-admin-left/right/menu/general/branding/domains/packages/plugins/about`, `.b-domain-*`, `.b-plugin-content`, `.b-domain-alias-content`, `.b-activate-content`, `.b-admin-about .rl-logo` (10243-10613 ; 8211, 10560-10590) | Panneau d'administration RainLoop (domaines, thème/branding, paquets, plugins, licence, à propos) | Administration serveur multi-domaines | Application mono-utilisateur sans panneau d'admin | SANS OBJET | — |

## 15. Réglages utilisateur (10600-11121)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.b-settins-left` (barre 34 px, menu 68 px) ; `.b-settings-menu .e-link` 30 px, 18 px, gris `#888` (DarkShine : `#aaa`) ; survol/sélectionné `rgba(255,255,255,.3)` texte blanc (10600-10620 ; out.css) | Menu des réglages sur le fond d'écran | Navigation réglages | `.setmenu .e-item` 2,14rem, 1,29rem, `--folders-sel` (`mail.css:307-310`), `base.html:77-84` | OK | — |
| `.b-settins-right .b-toolbar` (transparent, texte blanc, 34 px) avec bouton « Retour » (`icon-left`) sur le fond d'écran, au-dessus de la carte (10616-10620 ; SettingsPane.html) | Le bouton Retour est sur le fond d'écran | Revenir à la boîte | `.settb` : barre grise dégradé DANS la carte blanche avec bouton Retour (`mail.css:312-315`, `base.html:86-90`) | PARTIEL | RainLoop : bouton au-dessus de la carte, sans barre. Rendre `.settb` transparent et le placer hors de `.setpane` (padding-top 50 px). P2 |
| `.b-settins-right .b-content` carte blanche, bordure #aaa, ombre `0 2px 8px`, radius 5, padding 20/30 (10616-10630) | Contenu | Look | `.setpane` blanc, radius .43rem, ombre `0 2px 10px rgba(0,0,0,.5)` ; `.setcontent` padding 1,43rem 2,14rem (`mail.css:311, 316`) | OK | — |
| `.legend` largeur 600 px (10632) | Titre de section avec filet | Titres | `.setcontent .legend` bordure #e5e5e5 (`mail.css:317`) | OK | — |
| `.b-settings-general` : notification refusée, sélecteur de langue pointillé (10646-10665) | Réglages généraux | Langue, notifications | `settings.html:8-41` ; `notify-setting.js` (état « refusé par le navigateur ») | OK | — |
| `.b-settings-accounts .list-table` 600 px : poignée `icon-braille` pour **réordonner par glisser**, nom, suppression révélée au survol (`button-delete` masqué puis `delete-access`), `ui-sortable-helper` (10676-10790) | Liste des comptes et identités | Gérer et ordonner les comptes | `accounts.html:6-27` : liste, boutons haut/bas (`/accounts/{id}/move`), Modifier, Identités, Supprimer (confirmation dans `<details>`) | PARTIEL | Ordre par flèches au lieu du glisser ; suppression confirmée dans `<details>` (équivalent au `delete-access`). Pas de poignée `braille`. P3 |
| `.b-settings-templates .list-table` (10795-10928) | Liste des modèles | Modèles | `templates.html` | OK | non relu en détail |
| `.b-settings-folders .list-table` 600 px : nom éditable en cliquant (soulignement pointillé), renommer en ligne, abonnement (œil), vérifier (coche), supprimer (corbeille), indentation par `folder-padding` 15 px/niveau, dossiers système en gras (10929-11019) | Gestion des dossiers | Renommer, abonner, relever, supprimer | `folders_settings.html` + `.folders-table` : lignes avec boutons d'actions (`mail.css:321-329`) ; indentation `indentN` | PARTIEL | Fonctions (créer, supprimer, abonnement, vérification) présentes ; renommage en ligne et icônes d'abonnement (œil / coche) : non confirmés dans la fin de `folders_settings.html` (non relue). P3 |
| `.b-themes-list .e-item` : vignettes 100×100 des thèmes, bordure noire au sélectionné (11020-11046) | Choix du thème | Thèmes | `<select name="theme">` (`settings.html:12`) | PARTIEL | Liste déroulante au lieu de vignettes ; 3 thèmes (`dark`, `light`, `rainloop`). P3 |
| `.b-settings-filters .list-table` : poignée, nom, sous-nom gris, suppression (11049-11120) | Liste des filtres | Filtres | `filters.html`, `.filters-table tr.off td` gris (`mail.css:357`) | OK | non relu en détail ; tri par glisser : voir `filters.js` (48 l., non relu) |
| `.b-settings-open-pgp`, `.b-settings-social` (10890-10928) | PGP / social | — | — | SANS OBJET | — |

## 16. Console de debug, animations, bibliothèques (11122-11592)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `.rl-cmd` (11122-11140) | Console de commande interne (debug) | Développeurs | — | SANS OBJET | Outil de debug. |
| `@keyframes highlight-folder-row`, `bounce-me`, `textLoadingAnimationKeyFrame`, `animate-stripes`, `login-form-shake` (11141-11300) | Animations (pulsation du dossier, rebond d'icône, points de chargement, rayures, secousse) | Retour visuel | `body.no-anim` pour couper (`mail.css:467`) ; aucune animation correspondante | MANQUANT | Voir lignes dédiées (§6, §8, §10). P3 |
| `html.csstransitions.rl-started-delay #rl-left … transition` (11335-11345) | Les panneaux se redimensionnent en 0,3 s (replier le panneau des dossiers) | Confort | pas de transition sur `grid-template-columns` | MANQUANT | `.mailbox{transition:grid-template-columns .3s}`. P3 |
| `.e-strip-animation`, `.rl-animated-inited`, `.errorAnimated`, `.afterLoginHide`, `.button-delete-transitions`, `.listDragOver`, `.e-contact-item` transitions (11346-11450) | Animations d'interface | Confort | — | MANQUANT | P3 |
| `.modal.fade` entrée glissante 200 ms (11451-11456) | Ouverture des fenêtres | Look | — | MANQUANT | P3 |
| `.textLoadingAnimationD1..3` (11470-11485) | Points « … » animés (chargement liste) | Chargement | — | SANS OBJET | Liste rendue côté serveur. |
| `.rl-view-model.RL-Login…` (11486-11489) | Écran de connexion | — | — | SANS OBJET | SPA. |
| `.mfp-container`, `.pswp__*`, `.lg-backdrop` (11490-11510, 11570) | Visionneuses d'images (Magnific, PhotoSwipe, lightGallery) | Prévisualiser images | `dialog.viewer` (`mail.css:437-441, 489-492`, `preview.js`) | OK | — |
| `.ui-resizable-e` (11491) | Poignée jQuery UI | — | `.splitter` | OK | — |
| `.opentip-container` + styles `rainloopErrorTip` (11512-11555) | Bulles d'erreur de champs | Messages de validation | `required`, messages `msg.error` | SANS OBJET | Bibliothèque d'infobulles. |
| `svg-icon`, `.svg-icon-archive` 14 px (11556-11569) | Icône d'archive en SVG (seule icône non-police) | Bouton Archiver | `i-archive` (`base.html:29, 112`) | OK | — |
| `.btn .btn-text` (11572) ; `.active-scale-transition:active{transform:scale(.95)}` (11574-11590) | Effet d'enfoncement des boutons actifs | Retour tactile | aucun | MANQUANT | `.tbtn:active{transform:scale(.95)}`. P3 |

---

## 17. Icônes : police `icon-*` de RainLoop vs sprite SVG de wmail-go

Définitions des glyphes : lignes 1036 et suivantes d'`app.css` (118 glyphes, hors de ma tranche) ; usages relevés dans `app/templates/**` (grep). Notre sprite : `templates/base.html:24-55` (symboles `send contacts refresh folder archive spam trash menu user star-o star reply forward attach save back al-left al-center al-right al-justify copy doc gear eye expand caret image info close down`).

| Glyphe RainLoop | Où utilisé (gabarit) | Équivalent chez nous | Verdict |
|---|---|---|---|
| `icon-paper-plane` | Bouton Nouveau message, envoyer (MailFolderList, MailMessageList, PopupsCompose) | `i-send` (`base.html:25`, utilisé `base.html:97`, `compose.html:16`) | OK |
| `icon-address-book` | Contacts (MailFolderList, PopupsCompose) | `i-contacts` | OK |
| `icon-folder` / `icon-folder-add` | Déplacer vers, nouveau dossier | `i-folder` (déplacer `base.html:103`, nouveau dossier `base.html:217` : même icône `folder`, pas de « + ») | PARTIEL (pas de variante « ajouter ») P3 |
| `icon-copy` | Copier vers | `i-copy` | OK |
| `icon-archive` / svg-icon-archive | Archiver | `i-archive` | OK |
| `icon-warning` | Spam | `i-spam` (triangle) | OK |
| `icon-check-mark-circle-two` | « Pas spam » (MailMessageList/View), dossier vérifié (SettingsFolderItem) | aucun | MANQUANT (bouton « pas spam » inexistant) P3 |
| `icon-trash` | Supprimer | `i-trash` | OK |
| `icon-fire` | Vider le dossier (MailMessageList, PopupsFolderClear) | aucun (lien texte `clearlink`) | PARTIEL P3 |
| `icon-list` | Menu « plus d'actions » (MailMessageList) | `i-menu` (trois traits) | OK |
| `icon-star` / `icon-star-empty` | Suivi (liste, lecture, dossier) | `i-star` / `i-star-o` | OK |
| `icon-reply` / `icon-reply-all` / `icon-forward` | Répondre, répondre à tous, transférer | `i-reply`, `i-forward` ; **pas d'`i-reply-all`** (menu texte seulement, `base.html:269-274`) | PARTIEL : ajouter `i-reply-all` si on affiche les trois boutons comme RainLoop. P2 |
| `icon-right-mini` | Icône « répondu/transféré » dans la ligne (MailMessageListItem) | `.ans` avec `reply`/`forward` | OK |
| `icon-attachment` | Pièce jointe (liste, compo) | `i-attach` (compose seulement ; liste utilise `doc`/`image`) | PARTIEL : utiliser `attach` dans la liste. P2 |
| `icon-file-*` (zip, code, text, music, movie, image, excel, chart-graph, certificate) | Types de pièces jointes (JS, `attachmentIcon`) | `i-doc` + extension texte | PARTIEL P3 |
| `icon-eye` | Aperçu pièce jointe, abonnement dossier | `i-eye` | OK |
| `icon-play` | Lecture audio | aucun | MANQUANT P3 |
| `icon-image` | Images distantes bloquées, thèmes | `i-image` | OK |
| `icon-info` | Détails de l'en-tête | `i` en texte italique Georgia (`mail.css:162`, `base.html:237`) ; sprite `i-info` existe mais non utilisé là | PARTIEL (cosmétique) P3 |
| `icon-remove` | Fermer, supprimer, erreur (très répandu) | `i-close` (fermer `base.html:259`) ; `&times;` en texte ailleurs (`compose.html:19`) | OK |
| `icon-ok` | Coche (compte courant, enregistré) | `✓` texte (`base.html:152`) | PARTIEL P3 |
| `icon-plus` | Ajouter (comptes, filtres, domaines) | aucun | MANQUANT P3 |
| `icon-pencil` | Éditer / brouillon (MailMessageView) | `i-doc` (brouillon `base.html:268`) | PARTIEL P3 |
| `icon-print` | Imprimer | aucun (lien texte `base.html:~274`) | PARTIEL P3 |
| `icon-popup` | Ouvrir dans une fenêtre | aucun | MANQUANT P3 |
| `icon-file-code` | Afficher la source | aucun (lien texte) | PARTIEL P3 |
| `icon-download` | Télécharger .eml / PJ | `i-down` (non utilisé dans les gabarits lus) | PARTIEL P3 |
| `icon-arrows-out` / `icon-arrows-in` | Plein écran / quitter | `i-expand` (`base.html:258`) pour les deux états | PARTIEL P3 (pas d'icône « quitter ») |
| `icon-up` | Haut de page | aucun | MANQUANT P3 |
| `icon-left-middle` / `icon-right-middle` | Message précédent/suivant ; panneau gauche | aucun (`listnav.js` sans icônes ?) | MANQUANT P3 |
| `icon-left` | Retour (réglages, fil) | `i-back` (`base.html:87`) | OK |
| `icon-right-dir` / `icon-down` | Flèches de menu/tri | `i-caret` | OK |
| `icon-down-mini` / `icon-right-mini` (arbre) | Plier/déplier dossiers | `▾` `▸` texte (`folders.js:20-34`) | PARTIEL P3 |
| `icon-cog` | Réglages, options pièces jointes | `i-gear` | OK |
| `icon-user` / `icon-user-add` / `icon-users` | Compte, nouvelle identité, partage | `i-user` | OK / `user-add`,`users` SANS OBJET |
| `icon-power` | Déconnexion | aucun (texte) | PARTIEL P3 |
| `icon-help` | Aide | aucun (texte « Raccourcis ») | PARTIEL P3 |
| `icon-laptop` / `icon-mobile` | Version bureau/mobile | — | SANS OBJET |
| `icon-resize-in/out` | Replier/déplier le panneau gauche | `i-expand` (`base.html:214`) | OK |
| `icon-braille` | Poignée de glisser (listes réglages) | aucun | MANQUANT P3 |
| `icon-floppy` | Enregistrer le brouillon | `i-save` | OK |
| `icon-key`, `icon-lock` | Mot de passe, sécurité | aucun (texte) | PARTIEL P3 (champs de connexion, voir §6) |
| `icon-mail` | Champ e-mail de connexion, volet « sélection » | aucun | MANQUANT P2 |
| `icon-world`, `icon-earth` | Langue, site web du contact | aucun | MANQUANT P3 |
| `icon-search` (PopupsAdvancedSearch) | Recherche | aucun (champ `type=search`) | PARTIEL P3 |
| `icon-filter` | Domaines (admin) | — | SANS OBJET |
| `icon-share`, `icon-telephone`, `icon-at`, `icon-import`, `icon-export`, `icon-sync` | Contacts (partage, téléphone, e-mail, import/export/synchro) | aucun | PARTIEL P3 |
| `icon-list-add`, `icon-wrench`, `icon-bolt`, `icon-pause`, `icon-ellipsis-alt` | PGP, dossiers (outil), plugins, audio, menu « … » | — | SANS OBJET (sauf `ellipsis-alt` : menu « … » des actions quand pas de composeur, P3) |
| `icon-checkbox-checked/unchecked/partial`, `icon-radio-*` | Cases/radios en police | cases natives | PARTIEL P3 (voir §4) |
| `icon-spinner` | Chargements | aucun | SANS OBJET (pas d'AJAX visible) |
| `icon-cloud-up/down`, `icon-dropbox`, `icon-google-drive`, `icon-google`, `icon-facebook(-alt)`, `icon-twitter`, `icon-github`, `icon-skype`, `icon-youtube`, `icon-apple`, `icon-creative-commons`, `icon-rocket`, `icon-purchase`, `icon-buy-sign`, `icon-new-sign`, `icon-home`, `icon-suitcase`, `icon-repeat`, `icon-upload`, `icon-popup`, `icon-braille`… | Nuage, réseaux, admin | — | SANS OBJET |
| `icon-angry-smiley`, `icon-happy-smiley` | (définis, non retrouvés dans les gabarits) | — | SANS OBJET |

---

## À AJOUTER (MANQUANT / PARTIEL), par priorité

### P1
1. **Texte brut : police à chasse fixe et citations colorées.** `mailview/text.go:15` + style de `bodyDocHead` (`internal/web/mail.go:1000`). Détecter le niveau de citation (compter les `>` initiaux, produire des `<blockquote>` imbriqués) ; envelopper le texte dans `<div class="plain">` ; CSS dans `bodyDocHead` : `.plain{padding:15px;font-family:Monaco,Menlo,Consolas,"Courier New",monospace}`, `.plain pre{margin:0;white-space:pre-wrap}`, `.plain blockquote{border-left:2px solid blue;color:blue;margin:0;padding:0 10px}`, imbriqué vert, niveau 3 rouge (valeurs RainLoop app.css 9539-9582).
2. **Barre de composition sombre.** `mail.css:256` : `.cp-toolbar{background:rgba(0,0,0,.8);color:#fff;border-bottom:0}` ; `.cp-saved{color:#ddd}` ; `.cp-close{color:#fff;opacity:1;font-size:1.714rem}` ; bouton Enregistrer reste clair (`--btn-bg`), Envoyer vert `#84ab04`, Supprimer rouge `#bd362f`. À confirmer par capture d'écran de DarkShine avant de modifier (barre 40 px = 2,857rem).

### P2
3. **En-tête de composition gris `#eee`** : `.cp-body` partie haute (De/À/Cc/Objet) sur fond `#eee` avec étiquettes à droite (min 70 px), éditeur sur blanc dessous (app.css 10127-10170).
4. **Menu ouvert → bouton rouge** : `details.dropdown[open] > summary{color:#bd362f}` et caret/svg en `currentColor` (app.css 6044-6051, 6165-6173).
5. **Recherche** : (a) croix `×` pour vider le champ (lien `ClearSearchURL` dans `.search`, app.css 6304-6322) ; (b) bandeau « Résultats de recherche pour « … » » au-dessus de la liste (`listSearchDesc` 18 px, bordure #eee, app.css 8746) ; clés i18n fr/en.
6. **Aperçu « N messages sélectionnés »** dans `.readpane` quand N cases sont cochées et aucun message ouvert : grand nombre 70 px + icône courrier, gris `#999`, padding-top 100 px (app.css 9165-9169) ; mettre à jour depuis `list.js:refresh()`.
7. **Liste vide centrée** : `.msglist-empty{padding:60px 10px;text-align:center;font-size:1.714rem;line-height:30px;color:#999}` pour « dossier vide », « aucun résultat », « aucun non lu » ; erreur `#DA4F49` (`home.html:99`, app.css 8697-8721).
8. **Fils non lus** : calculer `ThreadUnseen` (au moins un UID du fil non lu) dans `web/mail.go:~600`, classe `.msgrow.subunread{background:#ffffd9}` avec barre `#ffdb99` (app.css 9040-9048) ; option P3 : demi-étoile si un message du fil est suivi.
9. **Liste en une ligne pour la disposition « liste seule »** : `.mailbox.listonly .msgrow` 2,5rem (35 px), `display:flex`, expéditeur 14,286rem (200 px, non gras sauf non lu), sujet extensible, date et pièce jointe à droite, étoile à gauche (app.css 8864-8925).
10. **Ligne ciblée au clavier** : `.msgrow.focused{background:#f5f5f5}` + `.msgrow.focused .sidebar{background:#ccc}` (non lu : `#cc8400`), à la place du contour pointillé (`mail.css:429`).
11. **Dépôt sur dossier** : `.folder-list li.drop-over{background:var(--folders-sel)}` (RainLoop : `rgba(255,255,255,.3)`, app.css 8068-8075 + DarkShine) au lieu du bleu (`mail.css:430`).
12. **Icône trombone dans la liste** : utiliser `attach` au lieu de `doc`/`image` pour `.attachmentParent` (`home.html:92`) ; garder `kind-image` en classe seulement.
13. **Chips de destinataires** : style gris carré `#eee`, bordure `#aaa`, texte `#555` (`mail.css:448`) ; focus `#999` au lieu de bleu (`mail.css:446`).
14. **Boutons de réponse** : trois icônes visibles `reply`, `reply-all`, `forward` dans l'en-tête (RainLoop) au lieu du seul bouton « dernier mode » ; ajouter le symbole `i-reply-all` (`base.html` sprite).
15. **Écran de connexion** : icônes dans les champs (`mail` / `key`, 17 px gris `#999`, à droite) et flèche de soumission dans le champ mot de passe (app.css 7845-7866).
16. **Réglages : bouton Retour hors de la carte** (barre transparente au-dessus de la carte blanche, texte blanc, `padding-top` 50 px, app.css 10616 + SettingsPane.html) à la place de `.settb` en dégradé gris dans la carte (`mail.css:312`).
17. **Contacts en deux volets** (liste 220 px à gauche, fiche à droite, barre d'actions sombre) : refonte de `/contacts` ; fonctions déjà présentes.
18. **Disposition « volet de lecture en bas »** (`layout=bottom`, liste 300 px de haut) : réglage `settings.html:20` + grille 2 lignes ; seulement si l'utilisateur utilise ce mode.
19. **Barre d'outils de lecture** : « Pas spam » dans le dossier Spam, flèches précédent/suivant.

### P3 (regroupés)
- Ombre `body::before` du haut de page ; transitions de largeur des panneaux ; animations (secousse de login, pulsation de dossier, entrée des fenêtres) ; `.tbtn:active{transform:scale(.95)}`.
- Cases/radios : `accent-color:#0f9d58` ou cases SVG ; `a:active{color:red}` et `pre/code` dans le corps HTML ; focus de champ gris `#999`.
- Largeurs par défaut selon la taille d'écran (liste 400 px sous 1400 px, dossiers 220 px au-dessus) ; police de liste 13 px au-dessus de 1400 px ; `.who` rayon 4 px / 250 px ; panneau réduit 60 px.
- Icônes manquantes ou textuelles : ajouter au sprite `plus`, `pencil`, `print`, `key`, `mail`, `power`, `help`, `search`, `check-circle`, `up`, `left/right-middle`, `arrows-in`, `braille`, `reply-all`, `play`.
- Pièces jointes : icônes par type, mode sélection + ZIP partiel, barre de progression de téléversement, surcouche de dépôt plein écran, lecture audio.
- Photo de l'expéditeur (Gravatar/contact) à la place de l'icône générique ; menu de fil dans la lecture ; bouton « haut de page » ; étiquette de quota en pourcentage dans le pied de la liste.
- Réglages enregistrés à la volée avec coche verte, vignettes de thèmes, grille de langues, drag-and-drop pour trier comptes/filtres/modèles.
- Aide des raccourcis plus large (700 px) ; voile `rgba(0,0,0,.2)` derrière `.keyhelp` / `.newfolder`.
- Texte barré pour les messages `\Deleted` ; focus clavier visuel sur les dossiers ; calque rayé et pastille de comptage pendant le glisser de messages.
