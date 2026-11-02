# Thèmes

Un thème de wmail-go est **un dossier contenant un `theme.css`** qui déclare des variables CSS sur `:root`.
Il n'y a pas de code à écrire : toute la mise en page vit dans `internal/web/static/*.css`, et un thème ne
fait que lui fournir ses couleurs.

```
<nom du thème>/
├── theme.css          obligatoire : les 65 variables
└── background.jpg     facultatif : .jpeg, .png, .webp ou .svg
```

---

## 1. Où poser un thème

| Emplacement | Effet | Pour qui |
|---|---|---|
| `internal/web/static/themes/<nom>/` | embarqué dans le binaire, **recompilation nécessaire** | thèmes livrés avec le produit |
| `<data_dir>/themes/<nom>/` | servi depuis le volume, **effet immédiat** | votre instance, sans rebâtir l'image |

Le dossier externe l'emporte **fichier par fichier** : un `theme.css` posé dans le volume remplace celui du
binaire, et un `background.jpg` posé à côté passe devant l'image livrée sans toucher au reste.

Contraintes :

- le nom doit satisfaire `^[a-z0-9_-]{1,32}$` — minuscules, chiffres, tiret, souligné ;
- seuls `theme.css` et `background.{jpg,jpeg,png,webp,svg}` sont servis par `/themes/<nom>/<fichier>` ;
- le libellé affiché dans les réglages vient de la clé i18n `theme.<nom>` (`internal/i18n/locales/*.json`).
  Sans elle, c'est le nom du dossier qui s'affiche — acceptable pour un thème personnel.

Le thème par défaut, celui de la page de connexion et des comptes qui n'ont rien choisi, est
`default_theme` dans `config.json`. Attention : `config.Load` part des valeurs par défaut du code **puis**
le fichier monté les écrase. Changer la valeur par défaut dans `internal/config/config.go` ne touche donc
que les instances neuves.

---

## 2. Le fond d'écran

`--wall` est une pile de couches `background`, de la plus haute à la plus basse. La convention des thèmes
livrés :

```css
--wall:
  url("/themes/<nom>/background.jpg") left top / cover no-repeat,  /* image personnelle, si posée */
  url("/themes/<nom>/background.svg") left top / cover no-repeat,  /* image livrée */
  linear-gradient(...);                                            /* dernier recours */
```

Trois points qui ont coûté du temps au moins une fois :

- **Les chemins doivent être absolus.** Une URL relative dans une variable CSS est résolue d'après la
  feuille qui *l'utilise* (`/static/app.css`), pas d'après le fichier du thème.
- **La couche `.jpg` est retirée à la volée** quand personne n'a posé l'image (`dropMissingWallpaper`),
  pour éviter une erreur 404 dans la console du navigateur. Cela ne vaut que pour `jpg`, `jpeg`, `png` et
  `webp` : un `background.svg` cité par le thème doit exister. Le retrait se fait **couche par couche**, et
  non ligne par ligne : la première couche porte aussi le `--wall:`, et l'emporter laissait une déclaration
  invalide, donc plus aucun fond d'écran nulle part.
- **Le fond personnel de l'utilisateur**, envoyé depuis les réglages, est écrit par `/user.css` avec
  `!important` et passe donc devant `--wall`.

Un SVG de fond doit être autonome : ni script, ni appel vers l'extérieur. La CSP de l'application est
`default-src 'self'; img-src 'self' data:`, donc une feuille de style ne peut de toute façon rien charger
d'un autre domaine — mais un test le vérifie aussi pour les fonds livrés.

### Le voile de lisibilité

Deux zones de l'interface sont posées **à même le fond**, sans couleur de fond propre : le panneau des
dossiers (`--folders-w`, 248 px) et la barre du haut (`--bar-h`, 50 px, par-dessus `--bar-bg`). Sur un fond
clair à ces endroits, les noms de dossiers disparaissent, et aucun test de feuille de style ne peut le voir :
il faut peindre l'image et la relire.

Le remède est une couche de dégradé noir en tête de `--wall`, à pleine force sur la zone de texte puis
effacée au-delà :

```css
--wall:
  linear-gradient(90deg, rgba(0, 0, 0, .55) 0 13%, rgba(0, 0, 0, 0) 30%),   /* dossiers */
  linear-gradient(180deg, rgba(0, 0, 0, .35) 0 5%, rgba(0, 0, 0, 0) 20%),   /* barre du haut */
  url("/themes/<nom>/background.svg") left top / cover no-repeat,
  linear-gradient(...);
```

Le palier compte : un dégradé qui décroît dès le bord ne donne presque rien aux derniers noms de dossiers,
ceux qui touchent le bord droit du panneau.

`make theme-check` mesure ces deux zones dans Chromium, fond réellement peint, et donne la ligne à poser
quand le contraste tombe sous 4,5:1. C'est le seul contrôle de thème qui demande un navigateur ; il exige
`make up`.

---

## 3. Les 65 variables

Les cinq thèmes livrés déclarent **exactement le même jeu** ; deux tests le vérifient
(`TestTousLesThemesDeclarentLeMemeJeu`, `TestVariablesDeThemeToutesUtiliseesEtToutesDeclarees`). Il manque
une variable à un thème, et celui-ci retombe en silence sur la valeur de repli de la feuille de style,
c'est-à-dire sur la couleur pensée pour un autre thème.

Les valeurs données ici sont celles du thème `rainloop`.

### Fond d'écran

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--wall` | fond de la fenêtre entière, derrière les trois volets | pile d'images et de dégradés |

### Page de connexion

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--login-bg` | fond de la carte de connexion | `rgba(0, 0, 0, .5)` |
| `--login-fg` | texte dans la carte | `#eeeeee` |
| `--login-border` | bordure de la carte (valeur `border` complète) | `none` |
| `--login-radius` | arrondi de la carte | `.5rem` |
| `--login-shadow` | ombre portée de la carte | `0 2px 10px rgba(0, 0, 0, .5)` |
| `--login-title` | titre « Connexion » | `#dddddd` |
| `--login-title-shadow` | ombre du titre (valeur `text-shadow`) | `none` |
| `--login-link` | liens sous le formulaire | `#dddddd` |

### Couleurs de base

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--bg` | fond des pages hors courrier (réglages, contacts) | `#e3e3e3` |
| `--fg` | texte courant | `#333333` |
| `--muted` | texte secondaire, mentions discrètes | `#777777` |
| `--card-bg` | fond des cartes blanches (réglages, contacts) | `#ffffff` |
| `--border` | bordures des cartes et des champs | `#d5d5d5` |
| `--input-bg` | fond des champs de saisie | `#ffffff` |
| `--accent` | couleur d'accentuation, contour de focus | `#336699` |
| `--accent-fg` | texte posé sur `--accent` | `#ffffff` |
| `--link` | liens | `#336699` |

### Barres et boutons

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--bar-bg` | barre du haut, sur le fond d'écran | `rgba(0, 0, 0, .5)` |
| `--bar-fg` | texte de la barre du haut | `#ffffff` |
| `--toolbar-bg` | barres d'outils de la liste (en-tête et pied) | `linear-gradient(#f4f4f4, #dfdfdf)` |
| `--btn-bg` | fond des boutons clairs | `linear-gradient(#ffffff, #e6e6e6)` |
| `--btn-hover` | bouton clair survolé | `#e6e6e6` |
| `--btn-fg` | texte des boutons clairs | `#333333` |
| `--btn-bd` | bordure des boutons clairs | `rgba(0, 0, 0, .25)` |
| `--new-bg` | bouton « Nouveau message » | `#84ab04` |
| `--new-fg` | texte du bouton « Nouveau message » | `#ffffff` |
| `--new-bd` | bordure du bouton « Nouveau message » | `rgba(0, 0, 0, .25)` |

### Menus déroulants

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--menu-bg` | fond d'un menu | `#ffffff` |
| `--menu-fg` | texte d'une entrée de menu | `#333333` |
| `--menu-hover-bg` | entrée survolée ou au clavier | `#48525c` |
| `--menu-hover-fg` | texte de l'entrée survolée | `#eeeeee` |

### Panneau des dossiers

Le panneau est posé **à même le fond d'écran** : il n'a pas de couleur de fond propre.

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--folders-fg` | nom d'un dossier | `#ffffff` |
| `--folders-disabled` | dossier non sélectionnable | `#aaaaaa` |
| `--folders-sel` | fond du dossier courant | `rgba(255, 255, 255, .3)` |
| `--folders-sep` | trait entre dossiers système et dossiers personnels | `#999999` |
| `--badge-bg` | pastille du nombre de non-lus | `#999999` |
| `--badge-fg` | chiffre dans la pastille | `#ffffff` |

### Liste des messages

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--list-bg` | fond du volet, sous les lignes | `#ffffff` |
| `--row-bg` | fond d'une ligne lue | `#f9f9f9` |
| `--row-hover` | ligne survolée | `#f5f5f5` |
| `--row-bd` | trait entre deux lignes | `#e5e5e5` |
| `--row-fg` | objet du message | `#000000` |
| `--sender` | nom de l'expéditeur | `#000000` |
| `--date` | date à droite de la ligne | `#999999` |
| `--unread-bg` | fond d'une ligne non lue | `#ffffd9` |
| `--unread-bar` | barre verticale d'une ligne non lue | `#ffa500` |
| `--subunread-bg` | conversation repliée contenant des non-lus | `#ffffd9` |
| `--subunread-bar` | barre verticale de cette conversation | `#ffdb99` |
| `--sel-bg` | fond de la ligne ouverte | `#dfefff` |
| `--sel-bar` | barre verticale de la ligne ouverte | `#398cf2` |
| `--focus-ring` | contour de la ligne atteinte au clavier | `#cc2f26` |
| `--focus-tint` | voile posé sur cette ligne (doit être translucide) | `rgba(204, 47, 38, .08)` |
| `--star` | étoile éteinte | `#666666` |
| `--star-on` | étoile allumée | `#ffa500` |

### Volet de lecture

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--read-bg` | fond du volet de lecture | `#ffffff` |
| `--read-fg` | texte des en-têtes du message | `#333333` |

Le **corps** du message n'est pas concerné : il est servi dans un iframe isolé, avec sa propre feuille
(`bodyDocHead`), toujours noir sur blanc. Un thème sombre ne noircit donc pas le courrier reçu — c'est
voulu, un message est du contenu, pas de l'interface.

### Bandeaux d'information

| Variable | Ce qu'elle peint | `rainloop` |
|---|---|---|
| `--bar-gray` | fond d'un bandeau neutre | `#eeeeee` |
| `--bar-line` | trait sous un bandeau | `#dddddd` |
| `--danger-bg` / `--danger-fg` | message d'erreur | `#fff5f5` / `#b02a37` |
| `--info-bg` / `--info-fg` | message d'information | `#ebf8ff` / `#1a5276` |
| `--warn-bg` / `--warn-fg` | avertissement, bandeau d'anomalies | `#fffaf0` / `#8a5a00` |

### `color-scheme`

À déclarer aussi : `light` ou `dark`. Elle commande ce que le **navigateur** peint lui-même — cases à
cocher, listes déroulantes natives, barres de défilement. Un thème sombre qui l'oublie garde des champs de
formulaire blancs.

---

## 4. Ce qui ne relève pas du thème

**Réglé par utilisateur**, écrit par `/user.css` par-dessus le thème :

| Variable | Réglage |
|---|---|
| `--folders-w` | largeur du panneau des dossiers (glissement du séparateur, ou réglages) |
| `--list-w` | largeur de la liste des messages |
| `html { font-size }` | taille du texte, de 10 à 18 px |
| `body { background }` | fond d'écran personnel, avec `!important` |

**Fixé par la feuille de style**, hors de portée d'un thème : `--bar-h` (hauteur des barres),
`--list-w-default`, `--list-h`, et `--wm-focus` qui n'est qu'un alias interne à une règle. Ces cinq noms
sont listés dans `varsHorsTheme` (`internal/web/themes_test.go`) : si vous ajoutez une **dimension**
réglable, ajoutez-la là ; si vous ajoutez une **couleur**, elle doit entrer dans les 65 et dans les cinq
thèmes, sinon le test échoue.

Pas paramétrable du tout : la typographie (Helvetica Neue, Helvetica, Arial), les espacements, les arrondis
hors page de connexion, et les icônes — un `<symbol>` SVG par glyphe dans `base.html`.

---

## 5. Ajouter un thème

### Par le générateur

```
make theme-new IMAGE=~/Images/foret.jpg NAME=foret
```

`tools/themegen` lit l'image, en tire une palette, en déduit les 65 variables **en garantissant les
contrastes**, écrit le thème et une page d'aperçu modifiable (section 6).

| Variable | Défaut | Effet |
|---|---|---|
| `IMAGE` | — | `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif` ou `.svg` |
| `NAME` | — | nom du thème, `^[a-z0-9_-]{1,32}$` |
| `DEST` | `static` | `static` écrit dans `internal/web/static/themes/` ; sinon **un chemin relatif au dépôt**, car l'outil tourne dans le conteneur de développement et ne voit que lui |
| `PANNEAUX` | `auto` | `clair` ou `sombre` pour forcer le parti des volets opaques |
| `VOILE` | `auto` | `non` pour garder l'image telle quelle, ou une force de 0 à 0,65 |
| `FORCE` | — | `FORCE=1` écrase un thème existant |

Comment il procède :

1. **Palette.** Image matricielle : les pixels sont regroupés dans une grille grossière de l'espace RGB et
   les cases les plus peuplées donnent les teintes. SVG : les attributs `fill`, `stroke`, `stop-color` et
   `flood-color` sont lus — wmail-go n'embarque aucun moteur de rendu et n'en ajoutera pas un pour cela.
   Le poids d'une couleur y est donc son nombre d'occurrences, pas sa surface, et les couleurs nommées
   (`red`) comme les fonctions (`rgb()`) ne sont pas lues.
2. **Rôles.** Le fond d'écran est l'image. Seules la barre du haut et la colonne des dossiers s'y posent :
   elles reçoivent un voile translucide. Tout le reste — liste, lecture, menus, cartes — est opaque et se
   construit à partir d'un neutre teinté de la couleur dominante. La couleur vive ne sert qu'aux accents
   (bouton « Nouveau », ligne ouverte, étoile). Un thème qui peindrait la liste avec la couleur vive serait
   joli en capture d'écran et épuisant à l'usage.
3. **Contrastes.** Chaque couleur de texte passe par un ajustement de luminosité jusqu'à atteindre 4,5:1
   (3:1 pour les contours et le texte secondaire), à teinte et saturation constantes. Le thème produit est
   donc lisible par construction, et un test balaie 180 palettes pour le vérifier.
4. **Voile de lisibilité.** La colonne des dossiers n'a pas de fond : sur une photo claire, les noms
   disparaissent. Sur une photo, l'outil assombrit l'**image elle-même** : plein régime sur la zone de
   texte, puis décroissance en cosinus. La décision se prend sur le **90e centile** de luminance de la
   bande, pas sur sa moyenne : une photo sombre traversée d'une épaule éclairée a une moyenne basse et des
   dossiers illisibles. Un SVG n'est pas retouché — il reçoit le même voile, mais **en CSS**, en tête de
   `--wall` (§ 2). La force est alors déduite de la palette entière, faute de pouvoir mesurer la bande :
   c'est une approximation, que `make theme-check` corrige.

### Inscrire un thème produit par le générateur

`make theme-new DEST=...` écrit un dossier de thème ; le copier dans `internal/web/static/themes/` ne suffit
pas à l'inscrire dans l'application. Trois gestes suivent, qui s'oublient sans que rien ne casse tout de
suite — `make theme-register ECRIRE=1` les fait, et un test les exige :

| Geste | Sans lui |
|---|---|
| vignette dans `settings-auto.css` | la case du thème est le carré gris neutre |
| `"theme.<nom>"` dans `fr.json` **et** `en.json` | les réglages affichent le nom du dossier |
| retrait de `apercu.html` | le fichier entre dans le binaire sans que rien ne puisse l'ouvrir |

Le libellé écrit par défaut est le nom du dossier : relisez-le, c'est ce que lit l'utilisateur. La vignette
est déduite du dégradé de repli de `--wall` ; pour l'écrire à la main, posez la règle `.theme-prev[...]`
**hors** du bloc produit, et `theme-register` la respectera.

Ensuite : `make test`, puis `make up` et `make theme-check`.

### À la main

1. Copier un thème proche : `cp -r internal/web/static/themes/light internal/web/static/themes/<nom>`
   — ou dans `<data_dir>/themes/<nom>/` pour un effet immédiat sans recompiler.
2. Régler les valeurs. Garder les 65 variables et l'ordre : `make test` le vérifie.
3. `make theme-register ECRIRE=1`, puis relire le libellé.
4. `make test`, puis `make up` et un tour dans les réglages.

---

## 6. La page d'aperçu

`make theme-new` écrit, à côté du thème, un fichier `apercu.html` autonome : une maquette de l'interface
(barre du haut, dossiers, liste avec lignes lues, non lues et sélectionnée, volet de lecture, menu, carte de
connexion, bandeaux) peinte par le thème.

Elle sert aussi d'**éditeur** : un réglage par variable, le rendu change à la frappe, les rapports de
contraste sont recalculés et signalés sous 4,5:1, et un bouton donne le `theme.css` final à copier.

Elle n'a besoin d'aucun serveur : ouvrez-la dans un navigateur.
