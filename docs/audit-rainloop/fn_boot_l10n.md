# Analyse exhaustive : boot.js + fichiers de langue RainLoop 1.17 vs wmail-go

Périmètre lu : `static/js/boot.js` (10868 lignes ; seuls les modules « maison » sont analysés : `dev/boot.js` l.6791, `dev/Common/Booter.js` l.10056, `dev/Common/Jassl.js` l.1720, `dev/Storage/RainLoop.js` l.2920, `dev/Styles/@Boot.css` l.10734, `dev/Html/Layout.html` l.10857 ; le reste = core-js, json3, intersection-observer, modernizr, progress.js, element-dataset = vendored, ignoré), `polyfills.js` (ignoré comme demandé), `app/localization/webmail/_source.en.yml` (714 l.) et `fr_FR.yml` (717 l.) lus intégralement, comparés à `internal/i18n/locales/{fr,en}.json` (685 l. chacun, `fr.json` lu intégralement ; `en.json` a le même nombre de lignes, non relu clé par clé).
Non lu : `admin/*.yml` (hors périmètre), `en.json` ligne à ligne (supposé miroir de `fr.json`).

Constat d'architecture : RainLoop est une SPA (boot.js charge libs, langue, app JS, puis rend en Knockout). wmail-go est rendu côté serveur (`internal/web/templates/*.html` + petits scripts `static/*.js`). Il n'y a donc ni écran de chargement, ni bundle à charger : la plupart des unités de boot.js sont SANS OBJET. Dans les fichiers de langue, wmail-go n'a pas de table de codes de notification : les erreurs passent par des clés `*.err.*` ou `error.generic` (`internal/web/mail.go:662-674`, `server.go:664-675`).

Remarque utilisateur DarkShine / vocabulaire : l'utilisateur utilise RainLoop en français ; les écarts de VOCABULAIRE visibles tous les jours (Signaler/Favori, Paramètres/Réglages…) sont regroupés dans la section 3 et classés P2.

---

## 1. boot.js (code maison)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `jassl(src, async)` (Jassl.js, l.1735) | Injecte un `<script>` et renvoie une promesse | Charger dynamiquement libs/langue/app | aucun (pas de bundle : pages serveur ; `templates/base.html:11-18` charge les css, scripts `defer`) | SANS OBJET | architecture SSR |
| `isStorageSupported(name)` (l.2937) | Teste localStorage/sessionStorage | Détection stockage | aucun | SANS OBJET | |
| `__get/__set` (sessionStorage ou `window.name`) (l.2965-2985) | Mémorise le hash de compte en session | Retrouver la session après rechargement SPA | cookie de session `wmail_session` HttpOnly (`web/server.go:38,354`) | SANS OBJET | session serveur |
| `getHash/setHash/clearHash` (l.3008-3040), `__rlah*` | Hash `AuthAccountHash` envoyé dans l'URL AppData | Authentifier l'amorçage | cookie (même fichier) | SANS OBJET | |
| `checkTimestamp()` + `setInterval(setTimestamp, 60 s)` (l.3045-3060) | Efface le hash si inactif > 60 min (déconnexion côté client) | Déconnexion auto sur inactivité | réglage `settings.prefs.idle` + `...absolute` (fr.json:49-50, `templates/settings.html:13`), appliqué côté serveur | OK | |
| `dev/boot.js` : element-dataset, json3, intersection-observer, modernizr (l.6800-6830) | Polyfills navigateurs anciens | Compat | aucun | SANS OBJET | vendored |
| `progressJs` + `onbeforeend` (l.6812-6827) | Barre de progression fine en haut pendant le chargement initial, masquée à la fin | Retour visuel de chargement | aucun (`grep progress` : seule la jauge de quota `templates/base.html:220`) | SANS OBJET | pas de chargement SPA ; P3 cosmétique : barre fine lors des actions longues |
| `__runBoot` : test `navigator.cookieEnabled` → redirige `./?/NoCookie` (l.10380) | Page « Cookies requis » | Message si cookies refusés | aucun message dédié ; sans cookie la connexion échoue (`login.error.invalid`/csrf) | MANQUANT | P3 : si le POST login arrive sans cookie de formulaire/CSRF, afficher « Les cookies sont nécessaires » au lieu de `error.csrf` |
| `getComputedStyle('rl-check','display')` → classe `no-css` (l.10385) | Détecte feuille CSS non chargée, bascule en mode dégradé (@Boot.css `html.no-css`) | Rendu de secours | `static/noscript.css` (traite seulement le cas sans JS) | SANS OBJET | |
| `useJsNextBundle` (toujours `false`) (l.10139) | Choix bundle ES5/ES6 | Compat | aucun | SANS OBJET | code mort |
| `includeStyle`, `includeScr` (l.10160-10190) | Injecte CSS/JS | Injection | aucun | SANS OBJET | |
| `includeLayout()` + `Layout.html` (l.10196, 10857) : `#rl-bg`, `#rl-loading`, `#rl-loading-error`, `#rl-content`, `#rl-popups`, `#rl-center/top/left/right/bottom`, `#rl-templates`, `#rl-hidden` | Squelette DOM de la SPA | Conteneurs | `templates/base.html` | SANS OBJET | |
| `includeAppScr` : URL `./?/AppData@mobile-0/<hash>/<rand>/` (l.10225) | Charge les données d'amorçage (AppData) | Config initiale | rendu serveur dans chaque page | SANS OBJET | détection mobile serveur exclue |
| `getRainloopBootData` (meta `app-boot-data`) (l.10240) | Lit admin/mobile | Amorçage | aucun | SANS OBJET | |
| `showError(additional)` (l.10260) + texte `Layout.html` « An error occurred. Please refresh the page and try again. » | Écran d'erreur plein écran (texte centré 30px, fond thème) si le boot échoue | Message d'échec global | `error.generic` « Une erreur est survenue. Réessayez plus tard. » via `message` page (`web/server.go:664-675`) | OK | page d'erreur SSR ; le message RainLoop mentionne « rechargez la page », sans importance |
| `showDescriptionAndLoading` (l.10290) + écran `#rl-loading` : texte (LoadingDescription) + 3 boules `e-bounce` animées, fade 300 ms | Écran de chargement visible 1-2 s à chaque ouverture du webmail | Retour visuel au démarrage, aspect DarkShine | aucun (pages SSR instantanées) | SANS OBJET | rendu visible différent mais volontaire ; P3 : si on veut « même rendu », ajouter un fondu d'entrée CSS |
| `runMainBoot` (l.10310) | Lance `__APP_BOOT` ou affiche l'erreur | Démarrage | aucun | SANS OBJET | |
| `runApp` : séquence libs → langue → app → plugins → éditeur (barre 5→70 %) (l.10325-10390) | Chargement séquentiel | | aucun ; l'éditeur riche est `static/editor.js` chargé à la demande | SANS OBJET | |
| `runApp` : `IncludeBackground` + `backstretch` (l.10345) | Image de fond plein écran (admin/thème/personnelle), fondu 100 ms, centrée | Fond d'écran personnel | `web/background.go`, `static/themes/rainloop/theme.css:2-6` (`background.jpg` `center / cover no-repeat`) ; upload `settings.background.*` (fr.json:623-627) | PARTIEL | pas de fondu à l'apparition (CSS `cover` direct) ; P3. Absence d'un thème « DarkShine » dédié : voir note finale |
| `__initAppData` : `NewThemeLink`, `IncludeCss` (CSS personnalisé admin) (l.10400-10415) | Applique le thème choisi, CSS additionnel | Thèmes | `<link href="/themes/{{.Theme}}/theme.css">` + `/user.css` (`base.html:12-13`) | OK | |
| `@Boot.css` : `#rl-content{display:none}`, `.internal-hiddden`, `html.no-css …` (l.10734-10750) | Cache l'app tant que non initialisée ; mode dégradé sans CSS | | `[hidden]` natif ; `noscript.css` | SANS OBJET | |
| `Layout.html` `#rl-loading-error` | cf. `showError` | | | OK | |
| `Layout.html` `#rl-check` / noscript (`NO_SCRIPT_TITLE/DESC`, `STATIC` yml) | Message « JavaScript requis » | Prévenir si JS désactivé | wmail-go fonctionne sans JS (progressive enhancement : `<noscript>` + `noscript.css` `base.html:10`, boutons `data-js-only hidden`) | OK | meilleur que RainLoop : pas de message nécessaire |

Unités boot.js analysées : 26.

---

## 2. Fichiers de langue, groupe par groupe (`_source.en.yml`)

Légende : OK = clé et sens équivalents dans `fr.json`/`en.json` ; ≠ = existant mais libellé/sens différent (voir section 3 pour le vocabulaire).

### LOGIN (l.2-15)
| Élément | But / rendu | Équivalent wmail-go | Verdict | Proposition |
|---|---|---|---|---|
| LABEL_EMAIL / LABEL_LOGIN / LABEL_PASSWORD | champs de connexion | `login.login` « Identifiant », `login.password` (fr.json:8-9) | OK | |
| LABEL_SIGN_ME « Se souvenir de moi » | case session persistante | aucun (durée gérée par réglages idle/absolue) | MANQUANT | P3 : case « rester connecté » (cookie persistant) ; sans objet si session longue suffit |
| LABEL_VERIFICATION_CODE, LABEL_DONT_ASK_VERIFICATION_CODE | écran 2FA + « ne plus demander 2 semaines » | `twofactor.login_title`, `twofactor.trust` (fr.json:589,592) | OK | |
| BUTTON_SIGN_IN | « Se connecter » | `login.submit` « Se connecter » | OK | |
| TITLE_SIGN_IN_GOOGLE/GMAIL/FACEBOOK/TWITTER | connexion sociale | aucun | SANS OBJET | social |
| LABEL_FORGOT_PASSWORD « Mot de passe oublié » | lien | `login.recover_link` | OK | |
| LABEL_REGISTRATION « S'enregistrer » | lien | `login.register_link` « Créer un compte » | OK | |

### TOP_TOOLBAR / MOBILE
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| BUTTON_ADD_ACCOUNT « Ajouter un compte » | `accounts.add` « Ajouter un compte mail » | OK | |
| BUTTON_SETTINGS « Paramètres » | `nav.settings` « Réglages » (fr.json:3) | PARTIEL (mot) | P2 : renommer en « Paramètres » |
| BUTTON_HELP « Aide » | bouton `keys.title` « Raccourcis clavier » (`base.html:158`) ; pas d'« Aide » générique | PARTIEL | P3 : libellé « Aide » sur ce bouton (ouvre déjà l'aide des raccourcis, comme RainLoop) |
| BUTTON_LOGOUT « Déconnexion » | `nav.logout` | OK | |
| MOBILE BUTTON_MOBILE/DESKTOP_VERSION | bascule version mobile | aucun (CSS responsive) | SANS OBJET | détection mobile serveur |

### SEARCH (l.24-43)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| MAIN_INPUT_PLACEHOLDER « Recherche » | `mail.search.placeholder` « Rechercher dans ce dossier » | OK | |
| TITLE_ADV | `mail.adv.title` « Recherche avancée » | OK | |
| LABEL_ADV_FROM/TO/SUBJECT/TEXT | `mail.adv.from/to/subject/text` | OK | |
| LABEL_ADV_HAS_ATTACHMENT(S) | `mail.adv.attach` « Avec pièce jointe » (RainLoop fr : « Pièce jointe ») | OK | |
| LABEL_ADV_FLAGGED « Signalé » | `mail.adv.flagged` « Suivis » | PARTIEL (mot) | P2 : « Signalés » si on adopte Signaler (section 3) |
| LABEL_ADV_UNSEEN « Non lu » | `mail.adv.unread` « Non lus » | OK | |
| LABEL_ADV_DATE + ALL/3 jours/1 semaine/1 mois/3 mois/6 mois/1 an | `mail.adv.date`, `date_all`, `d3`,`w1`,`m1`,`m3`,`m6`,`y1` (fr.json:570-577) | OK | libellés « 3 derniers jours » vs « Jusqu'à 3 jours » : équivalent |
| BUTTON_ADV_SEARCH | `mail.search.submit` | OK | |

### PREVIEW_POPUP (l.44-53)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| FULLSCREEN, ZOOM | visionneuse `static/preview.js` : flèches/boutons ‹ › ×, Échap ; pas de zoom ni plein écran | PARTIEL | P3 : zoom au clic sur l'image |
| CLOSE « Fermer (Esc) », GALLERY_PREV/NEXT | boutons `v-prev v-next v-close` avec `aria-label="←"`, `"→"`, `"×"` (preview.js:17-19) | PARTIEL | P3 : info-bulles textuelles « Précédent (flèche gauche) » via i18n (aujourd'hui sans texte, hors i18n) |
| LOADING, GALLERY_COUNTER « %curr% de %total% » | aucun compteur « 2 sur 5 » (legende `caption` : à confirmer) | MANQUANT | P3 : compteur i18n |
| IMAGE_ERROR / AJAX_ERROR (« L'image ne peut être chargée ») | aucun | MANQUANT | P3 : message sur `img.onerror` |

### FOLDER_LIST / QUOTA
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| BUTTON_COMPOSE « Nouveau message » (fr) | `mail.new` « Nouveau » (`base.html:97`, icône + libellé) | PARTIEL (libellé) | P2 : « Nouveau message » (le title du lien est déjà `compose.title`) |
| BUTTON_CONTACTS | `contacts.title`, lien Contacts du menu | OK | |
| BUTTON_NEW_MESSAGE | idem | OK | |
| INBOX/SENT/DRAFTS/SPAM/TRASH/ARCHIVE_NAME | `folder.*` : Boîte de réception, Envoyés, Brouillons, Indésirables, Corbeille, « Archives » (RainLoop « Archive ») | OK | « Archives » vs « Archive » : P3 |
| QUOTA.TITLE « Quota » | `quota.title` « Espace utilisé sur le serveur » (fr.json:478) | PARTIEL (mot) | P3 |
| MESSAGE_LIST.QUOTA_SIZE « Vous utilisez X (P%) sur L » | `quota.text` « %s (%d %%) sur %s » + jauge `progress.bar` (`base.html:220`, mail.css:383) | OK | |

### MESSAGE_LIST (l.66-100)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| BUTTON_RELOAD | `mail.refresh` « Actualiser » (fr.json:204) | OK | |
| BUTTON_MOVE_TO / copy | `mail.move_to`, `mail.copy_to` | OK | |
| BUTTON_DELETE | `mail.act.delete` | OK | |
| BUTTON_ARCHIVE | `mail.act.archive` | OK | |
| BUTTON_SPAM « Indésirable » | `mail.act.spam` (`base.html:113`, `mail.go:1363` → dossier junk) | OK | |
| BUTTON_NOT_SPAM « Acceptable » | aucun : dans le dossier Indésirables, le bouton spam déplace encore vers Indésirables (`mail.go:1363-1364`, pas de cas « notspam ») | MANQUANT | **P2** : dans le dossier junk, remplacer le bouton par « Acceptable » (clé `mail.act.not_spam`, action `notspam` → déplacement INBOX, + flag `$NotJunk`) |
| BUTTON_EMPTY_FOLDER « Vider le dossier » | `mail.clear.trash/junk` + confirmation `mail.clear.confirm` (fr.json:222-226) ; seulement corbeille/indésirables (`mail.err.not_clearable`) | OK | |
| BUTTON_MULTY_FORWARD « Transmettre les messages » (sélection multiple en pièces jointes) | `mail.forward_att` existe pour un message (`base.html:275`) ; pas pour une sélection | PARTIEL | P3 : bouton barre d'outils « Transférer en pièces jointes » sur la sélection |
| BUTTON_DELETE_WITHOUT_MOVE | `mail.act.delete_permanent` | OK | |
| BUTTON_MORE | `mail.more` « Plus d'actions » | OK | |
| MENU_SET_SEEN / UNSET_SEEN / ALL_SEEN | `mail.act.read/unread/read_all` | OK | libellés « Marquer lu » vs « Marquer comme lu » : section 3 |
| MENU_SET_FLAG « Signaler » / UNSET « Enlever le signal » | `mail.act.flag` « Favori », `unflag` « Retirer le favori » | PARTIEL (mot) | P2 section 3 |
| MENU_SELECT_ALL/NONE/INVERT/UNSEEN/SEEN/FLAGGED/UNFLAGGED | `mail.sel.*` (fr.json:499-505) | OK | |
| EMPTY_LIST « Liste vide. » | `mail.empty` « Ce dossier est vide. » | OK | |
| EMPTY_SEARCH_LIST | `mail.no_results` | OK | |
| SEARCH_RESULT_FOR « Résultats de recherche pour "x" » | aucun en-tête (grep « Résultats » : rien) ; champ `q` conserve le terme | MANQUANT | P3 : bandeau « Résultats de recherche pour « x » » + lien `mail.search.clear` |
| BACK_TO_MESSAGE_LIST | `mail.back` | OK | |
| LIST_LOADING, MESSAGE_LOADING | indicateur de chargement | aucun (SSR) | SANS OBJET | |
| EMPTY_SUBJECT_TEXT | `mail.no_subject` « (Pas d'objet) » | OK | |
| PUT_MESSAGE_HERE « Glisser un message ici… » | `mail.eml_drop` (fr.json:658) pour fichier .eml ; glisser-déposer interne `static/dnd.js` | OK | |
| TODAY_AT / YESTERDAY_AT | `listDate` (`web/mail.go:360-392`) : « il y a 12 min », « hier 14:05 », sinon `formatDate`. Pas de « aujourd'hui à HH:MM » | PARTIEL | P3 : RainLoop affiche « aujourd'hui à 14:05 » ; wmail-go « il y a N min » pendant 4 h puis heure seule : vérifier `formatDate` |
| SEARCH_PLACEHOLDER | cf. ci-dessus | OK | |
| NEW_MESSAGE_NOTIFICATION « Vous avez %COUNT% nouveaux messages ! » | `notify.many` « Vous avez %d nouveaux messages » (`poll.js`) | OK | |

### MESSAGE (l.101-156)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| BUTTON_EDIT | `mail.edit_draft` « Modifier » | OK | |
| BUTTON_BACK / CLOSE | `mail.back`, `mail.close` | OK | |
| BUTTON_DELETE/ARCHIVE/SPAM/MOVE_TO/MORE | idem liste | OK | NOT_SPAM : voir plus haut |
| BUTTON_UNSUBSCRIBE « Se désinscrire de cette liste » | `mail.unsubscribe` « Se désabonner de cette liste » (`base.html:288`) | OK | mot : P3 |
| BUTTON_REPLY / REPLY_ALL / FORWARD | `mail.reply`, `reply_all`, `forward` | OK | |
| BUTTON_FORWARD_AS_ATTACHMENT | `mail.forward_att` « Transférer en pièce jointe » (RainLoop : « Transférer comme pièce-jointe ») | OK | |
| BUTTON_EDIT_AS_NEW « Éditer comme nouveau » | `mail.edit_as_new` « Modifier comme nouveau » | OK | mot : P3 |
| BUTTON_SHOW_IMAGES | `mail.show_images`, `mail.show_remote`, `mail.trust_sender` | OK (plus riche) | |
| BUTTON_NOTIFY_READ_RECEIPT | `mail.mdn.ask/send/ignore` (fr.json:471-475) | OK | |
| BUTTON_IN_NEW_WINDOW « Voir dans une nouvelle fenêtre » | `static/msgfull.js` (plein écran, classe `msg-fullscreen`) ; pas d'ouverture dans une fenêtre séparée | PARTIEL | P3 : lien « ouvrir dans une nouvelle fenêtre » (`target=_blank` vers la vue message) en plus du plein écran `mail.fullscreen` |
| BUTTON_THREAD_LIST / PREV / NEXT / MORE | `mail.thread_open`, `mail.thread_back`, touche T ; pas de précédent/suivant dans la conversation (grep `thread_prev` : aucun) | PARTIEL | P3 : boutons « Message précédent/suivant de la conversation » + « Plus de messages » |
| MENU_HEADERS / VIEW_ORIGINAL / DOWNLOAD_ORIGINAL / FILTER_SIMILAR / PRINT | `mail.view_headers`, `view_source`, `download_eml`, `filter_similar`, `print` (`base.html:279-281`) | OK | |
| LABEL_SUBJECT/DATE/FROM/TO/CC/BCC/REPLY_TO (+ SHORT, PRINT_*) | `mail.from/to/cc/bcc/reply_to/date/size` ; impression `static/print.css` ; « CCI » vs « Cci » | OK | |
| PRINT_LABEL_ATTACHMENTS | `mail.attachments` | OK | |
| MESSAGE_VIEW_DESC | `mailbox.placeholder` « Sélectionnez un message pour le lire. » | OK | |
| MESSAGE_VIEW_MOVE_DESC « Cliquez sur le nom du dossier… pour sélectionner la destination » | menu `mail.move_to` + glisser-déposer `dnd.js` | OK | |
| PGP_* | OpenPGP | aucun | SANS OBJET | |
| LINK_DOWNLOAD_AS_ZIP | `mail.att.zip` « Tout télécharger (.zip) » | OK | |
| LINK_SAVE_TO_OWNCLOUD/CLOUD/DROPBOX | | aucun | SANS OBJET | |

### READ_RECEIPT
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| SUBJECT « Accusé de réception (affiché) - %SUBJECT% » | `mail.mdn.subject` « Accusé de lecture : » (fr.json:476) | OK | |
| BODY | `mail.mdn.body` (fr.json:477) même note de garantie | OK | |

### SUGGESTIONS
| SEARCHING_DESC « Recherche... » (autocomplétion adresse) | `static/contacts-ac.js` | OK (message d'attente absent, P3) | |

### CONTACTS (l.166-206)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| LEGEND_CONTACTS « Carnet d'adresses », SEARCH placeholder | `contacts.title`, `contacts.search` | OK | |
| BUTTON_ADD_CONTACT / CREATE / UPDATE | `contacts.new`, `contacts.save` | OK | |
| BUTTON_IMPORT / EXPORT_VCARD / EXPORT_CSV | `contacts.import`, `export_vcf`, `export_csv` | OK | |
| ERROR_IMPORT_FILE | `contacts.err.import` | OK | |
| EMPTY_LIST / EMPTY_SEARCH / CLEAR_SEARCH | `contacts.none` (un seul message pour liste vide et recherche vide ; pas de « Effacer la recherche » repéré dans `contacts.html`) | PARTIEL | P3 : message « Aucun contact trouvé » + lien d'effacement de la recherche |
| CONTACT_VIEW_DESC | volet de lecture de contact | à confirmer (formulaire `contacts.edit` sur page séparée) | PARTIEL | P3 |
| LABEL_DISPLAY_NAME, EMAIL, PHONE, WEB, BIRTHDAY, LINK_ADD_EMAIL/PHONE, PLACEHOLDERS | `contacts.name/email/phone/url/birthday/first/last/nick` | OK | |
| LABEL_READ_ONLY, LABEL_SHARE, BUTTON_SHARE_NONE/ALL | partage de carnet entre utilisateurs | aucun | SANS OBJET | mono-utilisateur |
| ADD_MENU_* (Pseudonyme, Notes, Email, Téléphone, URL, Adresse, Anniversaire, Tags) | `contacts.nick/notes/email/phone/url/address/birthday/tags` | OK | pas de menu « Ajouter » mais champs directs |
| BUTTON_SYNC « Synchronisation (CardDAV) » | `contacts.sync.*` (fr.json:634-652) | OK | |

### COMPOSE (l.207-242)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| TITLE_FROM/TO/CC/BCC/REPLY_TO/SUBJECT | `compose.*` (« Objet » alors que RainLoop fr compose dit « Sujet ») | OK | |
| LINK_SHOW_INPUTS « voir tous les champs » | liens `data-show="cc"/"bcc"/"reply_to"` (`compose.html:34-36`) | OK | |
| BUTTON_SEND / SAVE / DELETE / CANCEL | `compose.send/save/discard/close` | OK | |
| BUTTON_MINIMIZE « Réduire » (fenêtre de rédaction popup) | aucun : la rédaction est une page | SANS OBJET | architecture |
| SAVED_TIME | `compose.saved_at` | OK | |
| SAVED_ERROR_ON_SEND | `compose.sent_not_saved` | OK | |
| DISCARD_UNSAVED_DATA | `compose.discard_confirm` | OK | |
| ATTACH_FILES / ATTACH_DROP_FILES_DESC / ATTACH_ITEM_CANCEL | `compose.attach`, `compose.drop`, `compose.remove` + abort XHR (`compose.js:97,123`) | OK | |
| DROPBOX / GOOGLE_DRIVE | | aucun | SANS OBJET | |
| REPLY_MESSAGE_TITLE « %DATETIME% %EMAIL% a écrit » | `compose.reply_attribution` « Le %s, %s a écrit : » | OK | |
| FORWARD_MESSAGE_TOP_* (De, À, CC, **Envoyé**, Objet) | `compose.fwd.*` : champ « Date » à la place de « Envoyé » | PARTIEL | P3 : libellé « Envoyé » (fr) pour fidélité |
| EMPTY_TO_ERROR_DESC | `compose.err.no_recipient` | OK | |
| NO_ATTACHMENTS_HERE_DESC | `compose.no_attachments` | OK | |
| ATTACHMENTS_ERROR_DESC / ATTACHMENTS_UPLOAD_ERROR_DESC | `compose.err.uploading`, `compose.err.attach_failed` | OK | |
| BUTTON_REQUEST_READ_RECEIPT / MARK_AS_IMPORTANT / REQUEST_DSN | `compose.opt.receipt/important/dsn` | OK | |
| BUTTON_OPEN_PGP | | aucun | SANS OBJET | |

### POPUPS_* (fenêtres modales)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| WELCOME_PAGE | | aucun | SANS OBJET | |
| ASK : BUTTON_YES/NO, DESC_WANT_CLOSE_THIS_WINDOW, DESC_WANT_DELETE_MESSAGES | confirmations ad hoc : `mail.delete_forever_confirm`, `compose.discard_confirm`, `accounts.delete_confirm`… ; suppression normale sans confirmation (comme RainLoop qui ne confirme que « définitif ») | OK | |
| LANGUAGES « Choisir la langue » | `settings.prefs.language` | OK | |
| ADD_ACCOUNT / UPDATE_ACCOUNT | `accounts.form.*` | OK | |
| IDENTITY (Email, Nom, Répondre à, Signature, Cc, Cci, signature avant citation) | `identities.*`, `accounts.form.signature_before` ; champ Cc auto absent (seulement `identities.bcc`) | PARTIEL | P3 : Cc automatique d'identité (RainLoop a LABEL_CC) |
| CREATE_FOLDER (nom, dossier parent, créer, annuler) | `folders.create/name/parent/create_button` | OK | |
| CLEAR_FOLDER : avertissement « irréversible » | `mail.clear.confirm` « … Cette action est irréversible. » | OK | |
| IMPORT/VIEW/GENERATE/COMPOSE/MESSAGE_OPEN_PGP | | aucun | SANS OBJET | |
| TWO_FACTOR_TEST (test du code avant activation) | `twofactor.confirm` « Confirmer et activer » (saisie du code avant activation) | OK | |
| FILTER (créer/modifier, conditions, actions, Fait, ajouter condition) | `filters.add/edit/conditions/add_condition` | OK | |
| FILTER actions : Aucune/Déplacer/Faire suivre/Rejeter/Absence/Exclure | `filters.act.*` (6) | OK | |
| FILTER champs : De/Destinataires/Sujet/En-tête/Taille | `filters.f.*` (5) | OK | |
| FILTER types : Contient, Ne contient pas, **Substitutions (* ?)**, **Pas de substitutions**, Regexp, **Pas Regexp**, Égal, Différent, Au-dessus, Au-dessous | `filters.t.*` : 7 types (contient, ne contient pas, égal, différent, regex, over, under) ; `sieve/filters.go:30-36` n'a pas `Matches`, `NotMatches`, `NotRegex` | PARTIEL | **P2** : ajouter TypeMatches/NotMatches (`:matches`) et NotRegex (`not … :regex`) dans `internal/sieve/filters.go`, clés `filters.t.matches` « correspond à (* et ? acceptés) », `filters.t.not_matches`, `filters.t.not_regex` ; sinon des filtres existants créés dans RainLoop deviennent « étrangers » (`filters.err.foreign`) |
| FILTER ANY / ALL | `filters.match_any/all` | OK | |
| MARK_AS_READ, REPLY_INTERVAL, KEEP, STOP, EMAIL, VACATION_*, REJECT_MESSAGE, ALL_INCOMING | `filters.mark_read/vac_days/keep/stop/address/vac_subject/vac_msg/vac_addrs/reject_msg/sum.all` | OK | |
| SYSTEM_FOLDERS (titre, « Choisir », « Ne pas utiliser », Envoyés/Brouillons/Spam/Corbeille/Archive) | `folders.system_hint`, `folders.auto`, `folders.unused`, `accounts.form.*` | OK | |
| SYSTEM_FOLDERS NOTIFICATION_* (avertissement si dossier système non choisi) | aucun avertissement par dossier ; texte unique `folders.system_hint` | PARTIEL | P3 |
| TWO_FACTOR_CFG : activer, utilisateur, statut, secret, codes de secours, créer, activer, effacer, Fait, tester, afficher/masquer secret, descriptions | `twofactor.*` (fr.json:588-610) : secret toujours affiché (pas de bouton Masquer), pas de « test » séparé | PARTIEL | P3 : bouton afficher/masquer le secret ; TWO_FACTOR_REQUIRE_DESC (imposé par admin) SANS OBJET |

### TITLES / UPLOAD / EDITOR
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| TITLES LOADING/LOGIN/MAILBOX/SETTINGS/COMPOSE (titre d'onglet) | `<title>` = « wmail-go – titre » (`base.html:7`) ; titre boîte = `home.title` « Accueil » même pour un message (`mail.go:494,986`) ; compteur non lus `(n)` (`poll.js:12`) | PARTIEL | P2 : titre d'onglet « INBOX (3) – compte – wmail-go » comme RainLoop, au lieu de « Accueil » ; clé `titles.mailbox` « Boîte mail » |
| UPLOAD.ERROR_FILE_IS_TOO_BIG | `compose.err.attach_too_big` | OK | |
| UPLOAD ERROR_FILE_PARTIALLY_UPLOADED / NO_FILE / MISSING_TEMP_FOLDER / ON_SAVING / FILE_TYPE / UNKNOWN | `compose.err.attach_failed` (message unique) | OK (agrégé) | |
| EDITOR TEXT_SWITCHER_PLAINT_TEXT / RICH_FORMATTING / CONFIRM | `compose.mode_toggle`, `compose.mode_confirm` | OK | |

### SETTINGS_LABELS (menu des réglages)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| Personnel / Général / Contacts / Dossiers / Comptes / Identité(s) / Filtres / Modèles / Sécurité / Thèmes / Mot de passe | `set.general/accounts/folders/filters/contacts/security/templates` ; thème et mot de passe dans la page Général/Sécurité (`settings.*`) | PARTIEL | P3 : `set.soon` « Bientôt disponible » existe encore : vérifier qu'il n'est plus affiché ; Social, OpenPGP SANS OBJET |
| BUTTON_BACK | `set.back` | OK | |

### SETTINGS_FILTERS
| LEGEND, Enregistrer, Ajouter, Effacer, script brut, SUBNAME_*, LOADING, DELETING_ASK, CHACHES_NEED_TO_BE_SAVED_DESC | `filters.*` ; `filters.raw*`, `filters.sum.*`, `filters.delete_confirm`, `filters.saved` | OK | |
| CAPABILITY_LABEL « Capacité » (liste des extensions Sieve du serveur) | aucun affichage | MANQUANT | P3 : ligne « Capacités du serveur : fileinto, vacation… » dans la page filtres |

### SETTINGS_IDENTITY / IDENTITIES / ACCOUNTS
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| Nom, Répondre à, Signature, ajouter signature à tous les messages | `identities.name`, `compose.reply_to`, `accounts.form.signature` ; pas d'option « ajouter à tous » (toujours ajoutée) | OK | |
| LABEL_DEFAULT / DEFAULT_IDENTITY_LABEL « défaut » | `identities.main` | OK | |
| BUTTON_ADD_IDENTITY, DELETING_ASK | `identities.add`, `identities.delete_confirm` | OK | |
| Comptes : liste, ajouter, supprimer, DELETING_ASK, LOADING | `accounts.*` + déplacer `accounts.move_up/down` | OK | |

### SETTINGS_SECURITY / GENERAL
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| CONFIGURE_TWO_FACTOR | `twofactor.manage` | OK | |
| AUTOLOGOUT Jamais / minutes / heures | `settings.prefs.idle` (nombre de minutes, max) ; pas de choix « Jamais » | PARTIEL | P3 : accepter 0 = jamais si la politique le permet (volontairement borné : sécurité) |
| LANGUAGE | `settings.prefs.language` | OK | |
| LAYOUT Pas de séparation / Séparation verticale / Séparation horizontale | `settings.layout.three` / `list` (`settings.html:19-22`) ; pas de séparation horizontale (aperçu sous la liste) | PARTIEL | P2 : option « Séparation horizontale » (liste en haut, lecture en bas) — l'utilisateur DarkShine peut l'utiliser ; sinon P3 |
| EDITOR HTML / Non-formaté / (forcé) | `settings.editor.*` | OK | |
| ANIMATION Plein/Normal/Aucune | case `settings.prefs.animations` (booléen, `settings.html:36`) | PARTIEL | P3 |
| VIEW_OPTIONS : preview pane, checkboxes, threads, reply same folder, show images, show animation, per page | `settings.layout.*`, `settings.prefs.checkboxes`, `threads`, `reply_same_folder`, `accounts.form.block_remote` (par compte), `page_size` | OK | |
| NOTIFICATIONS : son, popup, « Bloqué par le navigateur » | `settings.prefs.notify_sound/notify_desktop` (`notify-setting.js`) ; mention « (Bloqué par le navigateur) » à confirmer | PARTIEL | P3 : texte « bloqué par le navigateur » si `Notification.permission==='denied'` |

### SETTINGS_CONTACTS / THEMES / SOCIAL / FOLDERS / CHANGE_PASSWORD / OPEN_PGP
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| CONTACTS_AUTOSAVE | `settings.prefs.contacts_autosave` | OK | |
| CONTACTS_SYNC_* (serveur, URL, utilisateur, mot de passe) | `contacts.sync.*` | OK | |
| THEMES : type clair/sombre, image de fond, upload, erreurs | `settings.prefs.theme`, thèmes `dark`/`light`/`rainloop`, `settings.background.*` ; pas de « thème personnalisé » (type + fond intégré) | PARTIEL | P3 |
| SOCIAL (Google/Facebook/Twitter) | | aucun | SANS OBJET | |
| FOLDERS : créer, système, supprimer, (dés)abonner, process libellés, DELETING_ASK, TO_MANY_FOLDERS, HELP_* | `folders.*` ; `folders.check` (dossiers surveillés = HELP_CHECK_FOR_NEW_MESSAGES) ; pas de message « trop de dossiers » | OK / TO_MANY_FOLDERS PARTIEL P3 | |
| CHANGE_PASSWORD : actuel, nouveau, répéter, enregistrer, erreur mismatch | `settings.password.*`, `error.mismatch` | OK | |
| OPEN_PGP (+ LABEL_ALLOW_DRAFT_AUTOSAVE) | PGP SANS OBJET ; autosave brouillon : `settings.prefs.draft_autosave` | OK | |

### SHORTCUTS_HELP (l.587-626) — comparé à `static/keys.js:34-67` et l'aide `base.html:163-190`
| Raccourci RainLoop | Équivalent | Verdict | Proposition |
|---|---|---|---|
| OPEN_USER_DROPDOWN | « ` » (`keys.js:61`) | OK | |
| REPLY / REPLY_ALL / FORWARD / FORWARD_MULTIPLY | R, A, F, Maj+F | OK | |
| HELP | « ? » | OK | |
| CHECK_ALL | Ctrl+A (`list.js`, aide `keys.select_all`) | OK | |
| ARCHIVE / DELETE / MOVE | Z, Suppr/#, M | OK | |
| OPEN_THREAD | T | OK | |
| READ / UNREAD / IMPORTANT | Q, U, I | OK | |
| SEARCH, CANCEL_SEARCH | `/` ; Échap efface la sélection puis ferme | PARTIEL | P3 : Échap dans le champ de recherche = annuler la recherche |
| FULLSCREEN_ENTER / TOGGLE / EXIT, VIEW_MESSAGE_ENTER | `msgfull.js` (bouton) ; pas de touche dédiée dans `keys.js` | MANQUANT | P3 : touche Entrée = plein écran/ouvrir, Échap = quitter |
| SWITCH_TO_MESSAGE / FOLDER_LIST / LIST (changement de focus entre volets) | aucun (flèches ← → absentes) | MANQUANT | P3 |
| BLOCKQUOTES_TOGGLE | aucun | MANQUANT | P3 |
| THREAD_NEXT/PREV (message suivant/précédent) | aucun (navigation par flèches dans la liste ouvre le message : `keys.navigate`) | PARTIEL | P3 |
| PRINT | aucun raccourci (menu Imprimer) | MANQUANT | P3 : Ctrl+P natif |
| CLOSE_MESSAGE | Échap | OK | |
| OPEN_COMPOSE_POPUP, MINIMIZE | « C/W » ouvre la page ; minimiser SANS OBJET | OK | |
| OPEN_IDENTITIES_DROPDOWN | aucun | MANQUANT | P3 |
| SAVE_MESSAGE / SEND_MESSAGE | Ctrl+S, Ctrl+Entrée (`compose.js`, aide) | OK | |
| CLOSE_COMPOSE | aucun (Échap) | PARTIEL | P3 |

### PGP_NOTIFICATIONS : 12 messages — SANS OBJET (OpenPGP non géré).

### NOTIFICATIONS (codes d'erreur) — équivalence message par message
| Code RainLoop | Message utilisateur wmail-go | Verdict | Proposition |
|---|---|---|---|
| INVALID_TOKEN | `error.csrf` | OK | |
| AUTH_ERROR | `login.error.invalid` | OK | |
| ACCESS_ERROR | aucun message dédié (403 : `mail.err.not_clearable` seulement) | PARTIEL | P3 : message « Accès refusé » |
| CONNECTION_ERROR « Serveur injoignable » | `accounts.test.fail.dns/refused/timeout` (affichés dans le panneau dossier, `base.html:198`) | OK | |
| CAPTCHA_ERROR, SOCIAL_*_LOGIN_ACCESS_DISABLE, LICENSING_*, DEMO_*, DOMAIN_ALREADY_EXISTS, CANT_*_PACKAGE, INVALID/UNSUPPORTED_PLUGIN_PACKAGE, CANT_SAVE_PLUGIN_SETTINGS | | SANS OBJET | admin/plugins/social/licence |
| DOMAIN_NOT_ALLOWED / ACCOUNT_NOT_ALLOWED | `accounts.test.fail.blocked` (politique réseau) | OK | |
| ACCOUNT_TWO_FACTOR_AUTH_REQUIRED / ERROR | `twofactor.bad_code`, `twofactor.expired` | OK | |
| COULD_NOT_SAVE_NEW_PASSWORD | `error.generic` | PARTIEL | P3 |
| CURRENT_PASSWORD_INCORRECT | `settings.error.password` | OK | |
| NEW_PASSWORD_SHORT | `error.password_short` | OK | |
| NEW_PASSWORD_WEAK / FORBIDDENT | aucun (pas de contrôle de force/caractères) | SANS OBJET | politique volontaire |
| CONTACTS_SYNC_ERROR | `contacts.sync.err.run` + `contacts.sync.last_error` | OK | |
| CANT_GET_MESSAGE_LIST / CANT_GET_MESSAGE | `errorKey()` → not_found/too_large/`accounts.test.fail.*`/`error.generic` (`mail.go:662-674`) | PARTIEL | P3 : message dédié « Impossible d'obtenir la liste des messages » plutôt que `error.generic` |
| CANT_DELETE_MESSAGE / CANT_MOVE_MESSAGE | `s.mailErr` → mêmes clés générales | PARTIEL | P3 |
| CANT_SAVE_MESSAGE | `compose.err.no_drafts` / `error.generic` | PARTIEL | P3 |
| CANT_SEND_MESSAGE | `compose.err.send` « L'envoi a échoué : %s » | OK | |
| INVALID_RECIPIENTS | `compose.err.address`, `compose.err.rejected` | OK | |
| CANT_SAVE_FILTERS / CANT_GET_FILTERS / FILTERS_ARE_NOT_CORRECT | `filters.err.server`, `filters.err.unavailable`, `filters.err.*` | OK | |
| CANT_CREATE/RENAME/DELETE_FOLDER, SUBSCRIBE, UNSUBSCRIBE | `folders.err.server` « Le serveur a refusé l'opération : %s », `folders.err.name/protected` | OK | |
| CANT_DELETE_NON_EMPTY_FOLDER | non applicable : suppression avec messages après `folders.delete_confirm` | OK | |
| CANT_SAVE_SETTINGS | `settings.prefs.error` | OK | |
| ACCOUNT_ALREADY_EXISTS | pas de message « compte mail déjà ajouté » (`grep` aucun dans `accounts.go`) ; `register.error.exists` pour le login wmail | PARTIEL | P3 : refuser un doublon (même e-mail+serveur) avec `accounts.err.duplicate` |
| ACCOUNT_DOES_NOT_EXIST | `mail.err.not_found` | OK | |
| MAIL_SERVER_ERROR | `accounts.test.fail.protocol`, `folders.err.server` | OK | |
| INVALID_INPUT_ARGUMENT | `mail.err.bad_request` | OK | |
| UNKNOWN_ERROR | `error.generic` | OK | |

### STATIC (pages d'erreur statiques)
| Élément | Équivalent | Verdict | Proposition |
|---|---|---|---|
| BACK_LINK « Recharger » | aucun lien « Recharger » dans la page d'erreur (`message` template) | MANQUANT | P3 |
| DOMAIN_LIST_DESC, PHP_*_ERROR | | SANS OBJET | PHP |
| NO_SCRIPT_TITLE/DESC | cf. `noscript.css`, app utilisable sans JS | OK | |
| NO_COOKIE_TITLE/DESC | aucun | MANQUANT | P3 (voir boot) |
| BAD_BROWSER_TITLE/DESC | | SANS OBJET | |

---

## 3. Écarts de vocabulaire FR visibles quotidiennement (RainLoop fr_FR → wmail-go)

| Contexte | RainLoop fr_FR | wmail-go fr.json | Priorité |
|---|---|---|---|
| Drapeau | Signaler / Enlever le signal / Signalés / Non signalés / « Signalé » (rech. avancée) | « Favori », « Retirer le favori », « Favoris », « Sans favori », « Suivis » (fr.json:143-144,504-505,579) | P2 (cohérence avec l'usage quotidien) |
| Lu / non lu | « Marquer comme lu », « Marquer comme non-lu » | « Marquer lu », « Marquer non lu » (141-142) | P3 |
| Réglages | « Paramètres » | « Réglages » (3,36) | P2 |
| Bouton nouveau message | « Nouveau message » | « Nouveau » (202) | P2 |
| Brouillon/archives | « Archive » | « Archives » (75) | P3 |
| CC / CCI | « CC » / « CCI » | « Cc » / « Cci » | P3 |
| Transférer pièce jointe | « Transférer comme pièce-jointe » | « Transférer en pièce jointe » | P3 |
| Éditer comme nouveau | « Éditer comme nouveau » | « Modifier comme nouveau » (282) | P3 |
| Désinscrire | « Se désinscrire de cette liste » | « Se désabonner… » (497) | P3 |
| Quota | « Quota » / « Vous utilisez … sur … » | « Espace utilisé sur le serveur » | P3 |
| Transfert : champ date | « Envoyé » | « Date » (276) | P3 |

---

## 4. Décompte

Unités analysées : 26 (boot.js) + ~175 lignes de langue/groupes/codes (groupes SETTINGS_*, POPUPS_*, MESSAGE*, 57 codes NOTIFICATIONS, 40 raccourcis…) ; total ≈ 200.
Verdicts approximatifs (lignes de tableau) : OK ≈ 115 ; PARTIEL ≈ 38 ; MANQUANT ≈ 16 ; SANS OBJET ≈ 32.

## À AJOUTER (MANQUANT / PARTIEL, par priorité)

### P2
1. **Bouton « Acceptable » (Pas indésirable)** : dans le dossier Indésirables, remplacer l'action spam (`mail.go:1363`) par `notspam` → déplacer vers INBOX (+ flag `$NotJunk`) ; clé `mail.act.not_spam` « Acceptable » ; icône dans `templates/base.html:113`.
2. **Vocabulaire « Signaler »** : `mail.act.flag` « Signaler », `unflag` « Enlever le signal », `mail.flagged`, `mail.sel.flagged` « Signalés », `mail.sel.unflagged` « Non signalés », `mail.adv.flagged` « Signalé » (fr.json:143-151,504-505,579).
3. **« Réglages » → « Paramètres »** (`nav.settings`, `settings.title`) et bouton « Nouveau » → « Nouveau message » (`mail.new`).
4. **Types de filtre manquants** : `Matches`, `NotMatches` (`:matches`, jokers * ?) et `NotRegex` dans `internal/sieve/filters.go:30-36` + `filters.t.*`, pour que les filtres créés dans RainLoop restent éditables.
5. **Disposition « Séparation horizontale »** (liste en haut, volet de lecture en dessous) en plus de `three` et `list` (`settings.html:19-22`).
6. **Titre d'onglet** de la boîte : « Dossier (non lus) – compte – wmail-go » au lieu de « Accueil » (`mail.go:494,986`), clé `titles.mailbox`.

### P3
7. Bandeau « Résultats de recherche pour « x » » avec lien d'effacement.
8. Fil de conversation : boutons message précédent/suivant, « Plus de messages ».
9. « Voir dans une nouvelle fenêtre » (lien `_blank`) pour un message.
10. Raccourcis manquants : plein écran/Entrée, focus liste/dossiers/message, bascule des citations, identités, Échap = annuler la recherche.
11. Visionneuse d'images : compteur « n sur N », zoom, info-bulles i18n, message d'erreur de chargement (`preview.js`).
12. Transfert groupé « en pièces jointes » pour une sélection.
13. Message dédié « cookies requis » (NO_COOKIE) et lien « Recharger » sur la page d'erreur.
14. Messages d'erreur dédiés (liste, message, suppression, déplacement, enregistrement, mot de passe, doublon de compte, accès refusé) au lieu de `error.generic`.
15. Divers : « Se souvenir de moi », Cc automatique d'identité, capacités Sieve, afficher/masquer le secret TOTP, « bloqué par le navigateur » (notifications), fondu du fond d'écran, animation à trois niveaux, wording mineurs (section 3).

Note hors périmètre mais constatée : `static/themes/` ne contient que `dark`, `light`, `rainloop` ; aucune feuille « DarkShine » dédiée (la mention DarkShine apparaît dans `mail.css` et `CAHIER_DES_CHARGES.md`) ; les autres audits traitent le rendu du thème.
