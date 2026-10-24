# Audit fonctionnel RainLoop 1.17.0 : réglages utilisateur et gestion des comptes

Périmètre : lecture seule de `/home/gdegoulet/Work/webmail_project/rainloop/1.17.0` (aucun dossier `data/` lu, rien exécuté).

Conventions de référence
- `app.js` = `static/js/app.js` (non minifié, bundle webpack ; les numéros de ligne sont ceux du fichier tel quel).
- `Actions.php` = `app/libraries/RainLoop/Actions.php`.
- `Application.php` = `app/libraries/RainLoop/Config/Application.php`.
- `T/` = `app/templates/Views/` (les gabarits sont cités par fichier, avec ligne quand elle est utile).
- `fr_FR.yml` = `app/localization/webmail/fr_FR.yml`.
- Les valeurs « par défaut » distinguent : défaut client (code JS avant peuplement), défaut serveur (`defaults.*` / `webmail.*` de `application.ini`) et valeur effective (celle envoyée par `AppData`, Actions.php:1499).

---------------------------------------------------------------------

## A. Cartographie des écrans (une section par écran)

### A.1 Chargement initial et menu système (SystemDropDown)
- Gabarit : `T/User/SystemDropDown.html` (même vue pour boîte aux lettres et réglages : `AbstractSystemDropDownUserView`, app.js:24225-24320).
- Contenu du menu : liste des comptes, « Ajouter un compte » (si capa ADDITIONAL_ACCOUNTS), « Paramètres » (capa SETTINGS), « Aide » (capa HELP, raccourcis clavier), bascule version mobile/grand écran, « Déconnexion ». Le logo utilisateur, l'e-mail courant et le lecteur audio (pause) sont affichés dans la même barre.
- Entrées : F-ST-01 à F-ST-12.

### A.2 Écran de connexion (Login, LoginWelcome)
- Gabarits : `T/User/Login.html`, `T/User/LoginWelcome.html` ; vue `LoginUserView` (app.js:33324-33716), écran `LoginUserScreen` (app.js:33248).
- Entrées : F-ST-13 à F-ST-32.

### A.3 Réglages : navigation (menu, volet)
- Gabarits : `SettingsMenu.html`, `SettingsPane.html` ; `SettingsUserScreen` (app.js:36589-36690) construit le menu selon les capacités ; `AbstractSettingsScreen` (app.js:22974-23180) gère routes et sous-écrans.
- Entrées : F-ST-33 à F-ST-37.

### A.4 Général (`SettingsGeneral`)
- `GeneralUserSettings` (app.js:36739-36940), store `SettingsUserStore` (app.js:8378-8444), store notifications (app.js:11864-12029).
- Entrées : F-ST-38 à F-ST-57.

### A.5 Contacts (`SettingsContacts`)
- `ContactsUserSettings` (app.js:36945-36977). Visible seulement si `ContactsIsAllowed` (carnet d'adresses actif).
- Entrées : F-ST-58 à F-ST-62.

### A.6 Comptes et identités (`SettingsAccounts`, popups Account et Identity)
- `AccountsUserSettings` (app.js:37019-37130), popups app.js:27615-27710 et 27757-27903 ; PHP Actions.php:2486-2830, 2890-2980, 3182-3260, 3370-3520.
- Entrées : F-ST-63 à F-ST-86.

### A.7 Filtres (`SettingsFilters`, popup Filter)
- `FiltersUserSettings` (app.js:37182-37362), popup (app.js:31460-31667), modèles (app.js:30085-30463), PHP `Providers/Filters/SieveStorage.php`.
- Visible seulement avec la capacité SIEVE (et non FILTERS seule, voir F-ST-87).
- Entrées : F-ST-87 à F-ST-103.

### A.8 Sécurité et double authentification (`SettingsSecurity`, popups TwoFactorConfiguration et TwoFactorTest)
- `SecurityUserSettings` (app.js:37398-37466), popups app.js:31723-31898 et 39602-39659 ; PHP Actions.php:6666-6930, 2201-2410.
- Entrées : F-ST-104 à F-ST-122.

### A.9 Social (`SettingsSocial`)
- `SocialUserSettings` (app.js:37503-37555). Visible seulement pour le compte racine et si Google (auth) / Facebook / Twitter sont activés.
- Entrées : F-ST-123 à F-ST-125.

### A.10 Mot de passe (`SettingsChangePassword`)
- `ChangePasswordUserSettings` (app.js:37596-37683) ; PHP `Providers/ChangePassword.php`, Actions.php:5036.
- Entrées : F-ST-126 à F-ST-132.

### A.11 Modèles de messages (`SettingsTemplates`, popup Template)
- `TemplatesUserSettings` (app.js:37712-37780), popup (app.js:31945-32091), PHP Actions.php:3259-3368, `Model/Template.php`.
- Entrées : F-ST-133 à F-ST-141.

### A.12 Dossiers (`SettingsFolders`, popups FolderCreate, FolderSystem, FolderClear)
- `FoldersUserSettings` (app.js:37825-37939), popups app.js:31331-31402, 24021-24118, 39264-39345 ; PHP Actions.php:5269-5615, 3637-3650.
- Entrées : F-ST-142 à F-ST-166.

### A.13 Thèmes (`SettingsThemes`)
- `ThemesUserSettings` (app.js:37988-38123), PHP Actions.php:7715-7830, 9035-9215.
- Entrées : F-ST-167 à F-ST-176.

### A.14 OpenPGP (`SettingsOpenPGP`, partie réglages)
- `OpenPgpUserSettings` (app.js:38165-38225). Les popups de clés ne sont traitées qu'en périphérie.
- Entrées : F-ST-177 à F-ST-180.

### A.15 Popups transverses (Languages, Ask, WelcomePage, KeyboardShortcutsHelp)
- `T/Common/PopupsLanguages.html`, `PopupsAsk.html`, `PopupsWelcomePage.html`, `PopupsKeyboardShortcutsHelp.html` ; vues app.js:22405-22470, 20391-20480, 40066-40107, 30530-30574.
- Entrées : F-ST-181 à F-ST-189.

### A.16 Options de configuration serveur (`application.ini`) qui pilotent les écrans
- `Application.php` sections `webmail` (l.155), `interface` (l.179), `branding` (l.185), `contacts` (l.201), `security` (l.212), `capa` (l.242), `login` (l.260), `defaults` (l.294), `labs` (l.398).
- Entrées : F-ST-190 à F-ST-205.

### A.17 Libellés, notifications et erreurs
- `fr_FR.yml` sections LOGIN, TOP_TOOLBAR, SETTINGS_*, POPUPS_*, TITLES, STATIC, NOTIFICATIONS, UPLOAD, EDITOR, PGP_NOTIFICATIONS.
- Entrées : F-ST-206 à F-ST-213.

---------------------------------------------------------------------

## B. Inventaire numéroté

### B.1 Menu système et chargement initial

F-ST-01 : Menu compte (liste des comptes) — Le bouton utilisateur ouvre un menu listant chaque compte (e-mail, coche sur le compte courant). Le clic sur un autre compte navigue vers `?/Change/<email>/` ; le serveur (`ServiceChange`, `changeAction`) vérifie la capacité ADDITIONAL_ACCOUNTS, recharge le compte depuis le jeton stocké (`GetAccountFromCustomToken`), pose un nouveau jeton d'authentification puis redirige vers `./`. Pendant 1 s après un clic gauche, l'icône devient un spinner (`accountClick`). — `T/User/SystemDropDown.html`, app.js:6327-6329 (`Links.change`), app.js:24260-24270, ServiceActions.php:1167-1203.

F-ST-02 : Raccourci d'ouverture du menu — la touche « ` » (accent grave) ouvre le menu compte dans les scopes MessageList, MessageView et Settings, et quitte le mode plein écran du message. — app.js:24298-24307.

F-ST-03 : Raccourci d'aide — `Shift + /` (équivalent « ? ») ouvre la popup d'aide des raccourcis dans MessageList, MessageView, Settings, uniquement si la vue est visible. — app.js:24309-24316.

F-ST-04 : « Ajouter un compte » — élément de menu visible seulement si capa ADDITIONAL_ACCOUNTS ; ouvre la popup Account (création). — `T/User/SystemDropDown.html`, app.js:24288-24292.

F-ST-05 : « Paramètres » et « Aide » — visibles si capas SETTINGS / HELP (HELP est supprimée en mobile côté serveur). « Paramètres » navigue vers la racine des réglages ; « Aide » ouvre la popup des raccourcis. — app.js:24276-24286, Actions.php:8117-8250 (`Capa`).

F-ST-06 : Bascule mobile/grand écran — lien `./?/MobileVersion/` ou `./?/DesktopVersion/` selon l'état courant (présent aussi sur l'écran de connexion). Option serveur `labs.allow_mobile_version` (défaut vrai), `labs.use_mobile_version_for_tablets` (défaut faux). — `T/User/SystemDropDown.html`, ServiceActions.php:841-860, Application.php:398+.

F-ST-07 : Déconnexion — appelle `Remote.logout` puis `loginAndLogoutReload(false, true, close)`. Si le compte courant est un compte additionnel (`ParentEmail` non vide), la fenêtre est fermée (`window.close`) en plus. Si `labs.custom_logout_link` est renseigné, redirection vers ce lien, sinon vers la racine. Le jeton côté client est effacé. — app.js:29437-29443, 23396-23430, Actions.php:3539-3555.

F-ST-08 : Logo utilisateur — `branding.user_logo` (image) et `user_logo_title` (infobulle) affichés dans la barre ; `branding.user_css` ajouté à l'interface authentifiée. — app.js:24232-24233, Actions.php:1499+ (`UserLogo`, `UserLogoTitle`, `UserCss`).

F-ST-09 : Compteur de non lus par compte — DÉSACTIVÉ en 1.17 : `accountsUnreadCount` renvoie toujours 0, `AppUser.accountsCounts()` retourne `false` immédiatement (code commenté), les compteurs du menu sont commentés dans le gabarit. Côté PHP, `DoAccountsCounts` existe et fonctionne (limite 7 comptes, ouvre une connexion IMAP par compte, compte courant forcé à 0), mais n'est plus appelé. — app.js:9172-9175, 28848-28875, `T/User/SystemDropDown.html` (blocs `<!-- -->`), Actions.php:3477-3520.

F-ST-10 : Lecteur audio dans la barre — affiche `currentAudio` avec animation d'égaliseur ; un clic arrête la lecture (`audio.api.stop`). — `T/User/SystemDropDown.html`, app.js:24245-24258.

F-ST-11 : Démarrage authentifié (`bootstart`) — séquence : peuplement des stores (App, Settings, Notification, Account, Contact), puis si `Auth` : si capas TWO_FACTOR + TWO_FACTOR_FORCE et `RequireTwoFactor` → popup de configuration 2FA verrouillée (F-ST-117) ; sinon rechargement des dossiers, route de démarrage `labs.startup_url`, chargement d'OpenPGP (si capa et `crypto.getRandomValues`), écrans boîte + réglages (si capa SETTINGS), puis minuteurs. — app.js:29480-29687.

F-ST-12 : Minuteurs de fond — `interval.2m` info dossier INBOX ; `interval.3m` dossier courant ; `interval.2m-after5m` dossiers « à vérifier » (F-ST-153) ; `interval.15m` quota ; `interval.20m` rechargement des dossiers ; synchro contacts toutes les `max(5, min(320, ContactsSyncInterval))` min (défaut 20 si <5), première synchro 10 s après le démarrage ; `AppDelayStart` (purge des caches serveur) à 35 s. Un `registerProtocolHandler('mailto')` est enregistré seulement si `AccountSignMe` est vrai et capa COMPOSER. Popup d'accueil (F-ST-188) 1 s après le démarrage si `WelcomePageUrl`. — app.js:29566-29641, Actions.php:3561-3635.

### B.2 Écran de connexion

F-ST-13 : Champ e-mail — `type=email`, nom `RainLoopEmail`, `autocomplete="email"`, placeholder `LOGIN/LABEL_EMAIL` (« Email »). Obligatoire : une chaîne vide après `trim` met `emailError` (animation 500 ms) et redonne le focus. Le champ « Identifiant » (`LABEL_LOGIN`) existe dans la langue mais n'est pas affiché ; `Login` est toujours envoyé vide. — `T/User/Login.html:36-44`, app.js:33483-33514, 6679-6688.

F-ST-14 : Champ mot de passe — `type=password`, `autocomplete="current-password"`, obligatoire (non vide après trim). L'icône clé à droite est un bouton d'envoi dès qu'un mot de passe est saisi (infobulle `BUTTON_SIGN_IN`). — `T/User/Login.html:46-58`.

F-ST-15 : Bouton « Se connecter » — `command: submitCommand` ; masqué si `appSettings.hideSubmitButton` (option serveur `login.hide_submit_button`, défaut faux). Désactivé pendant `submitRequest`. Pendant l'envoi les champs sont désactivés. — `T/User/Login.html:83-88`, Application.php:260+, Actions.php:1431+.

F-ST-16 : Pré-remplissage de développement — `DevEmail` / `DevPassword` (options `labs.dev_email`, `labs.dev_password`) préremplissent les champs ; focus : mot de passe si les deux sont remplis, sinon le champ vide. — app.js:33597-33607, 33661-33662, Actions.php:1499+.

F-ST-17 : Code de vérification (2FA) — champ texte `LOGIN/LABEL_VERIFICATION_CODE` (« Code de vérification »), caché par défaut. Il apparaît quand la réponse de `Login` contient `TwoFactorAuth: true` (le serveur a répondu AccountTwoFactorAuthRequired) ; focus après 100 ms ; vidé et recaché si l'e-mail est modifié. Obligatoire dès qu'il est visible. — app.js:33379-33385, 33544-33553, 33492-33495, Actions.php:2416-2470.

F-ST-18 : Case « Ne plus demander le code pendant 2 semaines » — visible uniquement avec le code de vérification ; envoyée en `AdditionalCodeSignMe`. Côté serveur : si le code est valide, un cookie `rltfasmauth` = `md5(APP_SALT . secret . empreinte navigateur)` est posé pour 14 jours (`60*60*24*14`) ; tant que ce cookie correspond (même User-Agent, même secret), le code n'est plus demandé. Le cookie n'est pas posé quand le code de secours est utilisé sans cocher la case. — `T/User/Login.html:72-81`, Actions.php:2330-2388, constante Actions.php:10.

F-ST-19 : Codes de secours à la connexion — si le code saisi fait plus de 6 caractères et que des codes de secours existent, il est comparé aux codes (suite de chiffres, séparateurs ignorés) ; une correspondance supprime ce code (usage unique) et dispense du TOTP ; sinon le code est vérifié comme TOTP. Erreur : `ACCOUNT_TWO_FACTOR_AUTH_ERROR` (« Erreur lors de la double authentification ») avec délai (F-ST-24). — Actions.php:2356-2380, 6734-6762.

F-ST-20 : Case « Se souvenir de moi » — libellé `LOGIN/LABEL_SIGN_ME`, animée. Visibilité et défaut selon `login.sign_me_auto` : `DefaultOff` (défaut, visible décochée), `DefaultOn` (visible cochée), `Unused` (cachée). Le dernier choix de l'utilisateur est mémorisé en stockage client (`LastSignMe` = `-1-`/`-0-`) et prime sur le défaut tant que le mode est DefaultOn/DefaultOff. — app.js:33429-33437, 33614-33659, 33582, `T/User/Login.html:112-120`, Application.php:279-286.

F-ST-21 : Chiffrement du cookie « se souvenir de moi » — avec `SignMe` coché, le serveur génère `generateSignMeToken` (`Md5Rand(APP_SALT . email)`), pose le cookie `rlsmauth` valable 30 jours contenant `{e: email, t: jeton}` chiffré par XXTEA (clé `md5(APP_SALT) . 'Q' . jeton de session 'rlsession'`), et enregistre côté serveur (stockage CONFIG `sign_me`) l'heure, le jeton d'authentification complet (qui contient le mot de passe IMAP chiffré) et le jeton signMe. À la reconnexion, le serveur déchiffre le cookie, compare les jetons, rejoue `GetAccountFromCustomToken`, puis `CheckMailConnection` ; en cas d'échec les données signMe sont effacées. — Actions.php:625-655, 1377-1417, 2416-2430, 3523-3535, Utils.php:207-250, 284-296, constantes Actions.php:10-17, ServiceActions.php:1242-1275.

F-ST-22 : Effets de « se souvenir de moi » sur l'application — `AccountSignMe` vrai : désactive la déconnexion automatique (F-ST-107), active l'enregistrement du gestionnaire `mailto:` du navigateur et la lecture du jeton `rlmailtoauth` pour préremplir un nouveau message. — app.js:8434, 29630-29641, Actions.php:1617-1625.

F-ST-23 : Normalisation de l'identifiant — côté serveur : `trim`, minuscules si `login.login_lowercase` (défaut vrai, ASCII seulement), si pas de `@` : (1) option `login.determine_user_domain` (défaut faux) tente de déduire le domaine depuis l'hôte HTTP (jusqu'à 14 niveaux, puis recherche générique), (2) puis `login.default_domain` est ajouté. Sans `@` ou mot de passe vide : `INVALID_INPUT_ARGUMENT` (affiché au client comme `AUTH_ERROR`). — Actions.php:2201-2296, 2430-2460, app.js:33559-33566.

F-ST-24 : Délai après échec — `labs.login_fault_delay` (défaut 1 s) : `requestSleep` fait dormir la requête si moins de 1 s s'est écoulée depuis le démarrage (échec de connexion, de code 2FA, de connexion IMAP, de changement de mot de passe). — Actions.php:2101-2108, Application.php:413.

F-ST-25 : Messages d'erreur de connexion — alerte rouge avec bouton de fermeture ; texte via `getNotificationFromResponse`. `INVALID_INPUT_ARGUMENT` est remplacé par `AUTH_ERROR` (« L'authentification a échoué ») ; code inconnu : `UNKNOWN_ERROR` (« Erreur inconnue »). Un second paragraphe affiche `ErrorMessageAdditional` (alerte IMAP) si `labs.imap_show_login_alert` (défaut vrai). Erreurs réseau/non-JSON : `UNKNOWN_ERROR`. — `T/User/Login.html:14-23`, app.js:33559-33580, Actions.php:2162-2170.

F-ST-26 : Message de déconnexion personnalisé — `AdditionalLoginError` (cookie `rlspeclogoutcmk`, lu puis supprimé) est affiché comme erreur à l'ouverture de l'écran de connexion. — app.js:33450-33452, Actions.php:1678 (`GetSpecLogoutCustomMgsWithDeletion`).

F-ST-27 : Choix de la langue à la connexion — bouton globe (tooltip `POPUPS_LANGUAGES/TITLE_LANGUAGES`) visible si `login.allow_languages_on_login` (défaut vrai) et aucun bouton social ; sinon un sélecteur « drapeau + nom » est affiché en pied. Le choix recharge les traductions (spinner) et la langue n'est envoyée à `Login` qu'après un changement (`bSendLanguage`) ; le serveur la valide (`ValidateLanguage`) et l'enregistre dans les réglages du compte si elle diffère. — `T/User/Login.html:106-110, 155-163`, app.js:33421-33428, 33676-33688, 33697-33712, Actions.php:2455-2472.

F-ST-28 : Langue détectée — sans authentification, si `login.allow_languages_on_login` ET `login.determine_user_language` (défaut vrai), la langue de l'écran de connexion est déduite de l'en-tête `Accept-Language` (première langue disponible), sinon `webmail.language` (défaut `en`). — Actions.php:1926-1935, 2068-2085, Application.php:158-175.

F-ST-29 : Connexion sociale — boutons Gmail (icône mail), Google, Facebook, Twitter ouvrent une fenêtre (`window.open`, 500 à 550 px). Visibilité : Facebook `AllowFacebookSocial` ; Twitter `AllowTwitterSocial` ; Google `AllowGoogleSocial` ET `AllowGoogleSocialAuth` ; Gmail `AllowGoogleSocial` ET `AllowGoogleSocialAuthGmail`. Les options sont forcées à faux si les identifiants (client id/secret, app id/secret, consumer key/secret) sont vides ; Facebook exige PHP ≥ 5.4. Un retour d'erreur (`fSocial`) affiche la notification correspondante (ex. `SOCIAL_*_LOGIN_ACCESS_DISABLE`). — app.js:33463-33480, 33620-33674, Actions.php:1713-1790.

F-ST-30 : Écran d'accueil de connexion (`LoginWelcome`) — si `login.welcome_page` (défaut faux) : au lieu du formulaire, deux icônes : Google/Gmail (si activé) et enveloppe qui affiche le formulaire (`displayMainForm`). — `T/User/LoginWelcome.html`, app.js:33346, 33589-33591, `T/User/Login.html:25-28`.

F-ST-31 : Liens « Mot de passe oublié » / « S'enregistrer » — affichés seulement si `login.forgot_password_link_url` / `login.registration_link_url` sont non vides (après `trim`) ; ouverts dans un nouvel onglet. — `T/User/Login.html:121-137`, Actions.php:1456-1457.

F-ST-32 : Habillage de la page de connexion — `branding.login_logo`, `login_desc` (description), `login_background` (image de fond), `login_css` ; points d'extension de plugins (BeforeLogo, AfterLogo, TopControlGroup, BottomControlGroup, BottomFooter). Le hook `user-login-submit` peut bloquer l'envoi avec un code ou un message. La redirection après succès : espace admin si `Admin`, sinon rechargement. — `T/User/Login.html`, app.js:33516-33532, 33554-33558, Actions.php:1499+.

### B.3 Navigation des réglages

F-ST-33 : Entrées du menu et conditions — Général (toujours, par défaut) ; Contacts (`ContactsIsAllowed`) ; Comptes (capa ADDITIONAL_ACCOUNTS ou IDENTITIES : libellé « Comptes » si comptes, sinon « Identités ») ; Filtres (capa **SIEVE**) ; Sécurité (capa AUTOLOGOUT ou TWO_FACTOR) ; Social (compte racine ET Google auth/Facebook/Twitter) ; Mot de passe (`ChangePasswordIsAllowed`) ; Modèles (capa TEMPLATES) ; Dossiers (capa FOLDERS) ; Thèmes (capa THEMES) ; OpenPGP (capa OPEN_PGP). Si la capa SETTINGS est absente, aucun réglage n'est enregistré. Des plugins peuvent ajouter des entrées (`SettingsCustom.html`, gabarit « CUSTOM »). — app.js:36600-36665, `T/User/SettingsCustom.html`.

F-ST-34 : Routes — `#/settings/<route>` ; routes : `general`, `contacts`, `accounts`, `filters`, `security`, `social`, `change-password`, `templates`, `folders`, `themes`, `openpgp`. Route inconnue ou entrée désactivée : retour à la route par défaut (`general`). — app.js:36600-36665, 23042-23175.

F-ST-35 : Navigation clavier du menu — flèches haut/bas dans le scope Settings ; sur mobile, un clic sur une entrée masque le panneau gauche. — app.js:38327-38335.

F-ST-36 : Bouton « Retour » — `SETTINGS_LABELS/BUTTON_BACK` ; retourne à la boîte INBOX ; le titre de fenêtre devient « email - Paramètres ». — `T/User/SettingsPane.html`, app.js:38341-38343, 36673-36684.

F-ST-37 : Panneau gauche réductible — bouton de redimensionnement (icône) pour replier le menu ; désactivé par défaut sur mobile (`leftPanelDisabled(true)` à l'ouverture). — `T/User/SettingsMenu.html`, app.js:36665-36672.

### B.4 Général

Sauvegarde : chaque réglage est envoyé individuellement à `SettingsUpdate` (`Remote.saveSettings`) ; pour les listes, un indicateur (SaveTrigger) affiche : animation, succès ou erreur pendant 1 s.

F-ST-38 : Identité principale — affiche `nom (email)` de l'identité d'id vide ; un clic ouvre la popup Identité (édition de l'identité par défaut, voir F-ST-80). Affiche `---` si la liste d'identités n'est pas chargée. — `T/User/SettingsGeneral.html:6-14`, app.js:36765-36785, 36823-36829.

F-ST-39 : Langue — sélecteur « drapeau + nom » (popup Languages) visible si `webmail.allow_languages_on_settings` (défaut vrai). Choix : recharge les traductions, indicateur de succès/échec, puis `SettingsUpdate Language`. Côté serveur, si l'option est désactivée, la langue enregistrée est remplacée par `webmail.language`. La valeur est validée par `ValidateLanguage`. — `T/User/SettingsGeneral.html:17-33`, app.js:36832-36852, Actions.php:4951-4965.

F-ST-40 : Disposition (Layout) — liste : 0 « Pas de séparation » (sans aperçu), 1 « Séparation verticale » (aperçu à droite), 2 « Séparation horizontale » (aperçu en bas) ; défaut client 1 ; défaut serveur `defaults.view_layout` = 1 ; masquée sur mobile (`hide-on-mobile`) ; sur mobile le serveur impose 0. Le changement vide la liste de messages, applique les classes CSS `rl-no-preview-pane` / `rl-side-preview-pane` / `rl-bottom-preview-pane`. Valeur serveur hors liste ramenée à 1. L'ancien libellé « Utiliser le panneau de prévisualisation » (`LABEL_USE_PREVIEW_PANE`) n'est plus affiché ; `usePreviewPane` est dérivé (`layout != 0`). — `T/User/SettingsGeneral.html:35-52`, app.js:8381-8416, 36796-36807, 36934-36939, Actions.php:4975-4980, 1832-1850, 1981-1985.

F-ST-41 : Éditeur par défaut — valeurs : `Html`, `Plain`, `HtmlForced`, `PlainForced` (libellés HTML, Non-formaté, HTML (forcé), Non-formaté (forcé)) ; défaut client `Html`, défaut serveur `defaults.view_editor_type` = « Html » ; toute valeur serveur invalide est ramenée à `Plain` à l'envoi des données. Les variantes « forcées » imposent le mode à la composition (voir l'éditeur). — `T/User/SettingsGeneral.html:53-68`, app.js:8384-8386, 36787-36797, Actions.php:1972-1973, 4983.

F-ST-42 : Messages par page — liste 10, 20, 30, 50, 100 (client) ; défaut client 20 ; défaut serveur `webmail.messages_per_page` = 20 (fallback code 25) ; le serveur accepte 10, 20, 30, 50, 100, 150, 200, 300 et ramène tout autre entier à 20 ; une valeur serveur absente de la liste client (150/200/300) est corrigée par `limitedList` côté client. — `T/User/SettingsGeneral.html:70-82`, app.js:7742-7743, 8387-8389, 36743-36745, Actions.php:4967-4970, 1833-1834.

F-ST-43 : « Toujours afficher les images dans le mail » (`ShowImages`) — case ; défaut client faux ; défaut serveur `defaults.show_images` = faux. Effet : à l'ouverture d'un message HTML contenant des images distantes, `showExternalImages(true)` est appelé automatiquement (chargement différé `lozad`) ; sinon un bandeau propose de les afficher. — `T/User/SettingsGeneral.html:89-93`, app.js:5560-5562, 8390, 36861.

F-ST-44 : « Afficher des cases à cocher dans la liste » (`UseCheckboxesInList`) — défaut vrai ; masquée sur mobile (le serveur impose vrai en mobile) ; utilisée par la liste des messages. — `T/User/SettingsGeneral.html:94-101`, app.js:8391, 30787, 34572, Actions.php:1981-1990.

F-ST-45 : « Regrouper en conversations » (`UseThreads`) — visible seulement si `threadsAllowed` : serveur `labs.use_imap_thread` (défaut vrai) ET le serveur IMAP supporte THREAD (`IsThreadsSupported`). Défaut client faux ; défaut serveur `defaults.mail_use_threads` = faux. Le changement vide la liste de messages et enregistre immédiatement. Stocké dans les réglages locaux (`settings_local`). — `T/User/SettingsGeneral.html:103-109`, app.js:8393, 29994 (threadsAllowed), 36890-36894, Actions.php:4994.

F-ST-46 : « Placer les réponses dans le dossier du message » (`ReplySameFolder`) — défaut faux (`defaults.mail_reply_same_folder`). Effet à l'envoi d'une réponse/transfert : le message envoyé est copié dans le dossier du message d'origine (`aDraftInfo[2]`) au lieu du dossier « Envoyés ». Enregistrement retardé de 3 s (`timeOutAction`). Réglage local. — `T/User/SettingsGeneral.html:110-116`, app.js:12696-12700, 36879-36885, Actions.php:4995.

F-ST-47 : Notifications : section — le bloc entier est masqué sur mobile (et le serveur force sons/pop-ups à faux en mobile) ; visible seulement si le navigateur supporte les notifications bureau OU l'audio MP3 (`isDesktopNotificationSupported() || soundNotificationIsSupported()`). — `T/User/SettingsGeneral.html:120-150`, app.js:11864-12029, Actions.php:1981-1990.

F-ST-48 : Notification bureau — case « Afficher un pop-up de notification pour les nouveaux messages » (`DesktopNotifications`). États : `NotSupported`, `Allowed`, `NotAllowed`, `Denied`. Cocher demande la permission du navigateur si elle est « par défaut » ; si refusée la case revient décochée ; si bloquée, la case est désactivée et le texte « (Bloqué par le navigateur) » s'affiche. Enregistrement retardé de 3 s. Défaut faux. — `T/User/SettingsGeneral.html:126-140`, app.js:11871-11945, 36866-36872, 36936.

F-ST-49 : Comportement des pop-ups bureau — uniquement pour de nouveaux messages détectés dans INBOX : si plus de 3 nouveaux messages, une seule notification (titre = e-mail du compte, texte `MESSAGE_LIST/NEW_MESSAGE_NOTIFICATION` avec le nombre) ; sinon une notification par message (expéditeur + sujet). Le clic ouvre le message ; fermeture automatique après 7 s. — app.js:5253-5280, 11979-12013.

F-ST-50 : Son de notification — case « Son de notification » (`SoundNotification`), visible si le navigateur lit l'audio MP3 ; défaut faux ; enregistrement retardé de 3 s ; bouton « lecture » vert pour tester (`playSoundNotification(true)` ignore le réglage). Le son est joué à chaque lot de nouveaux messages INBOX. — `T/User/SettingsGeneral.html:141-150`, app.js:10662-10678, 11964-11977, 36824-36827, 36873-36878.

F-ST-51 : Animations de l'interface — AUCUNE case dans l'écran : les libellés `LABEL_ANIMATION` (Plein/Normal/Aucune) et `LABEL_SHOW_ANIMATION` existent dans les langues mais sont inutilisés ; `InterfaceAnimation` est forcé à vrai dans les données serveur ; `isAnimationSupported` est calculé dans la classe mais non lié au gabarit. L'animation des cases à cocher (design matériel) dépend de `labs.use_material_design` (défaut vrai) et de la prise en charge des animations CSS. — app.js:36759, 23465-23472, Actions.php:1475, 1927.

F-ST-52 : Signature d'envoi / identités — aucun champ de signature dans l'écran Général de 1.17 (le libellé `SETTINGS_IDENTITY/*` est hérité) ; la signature se règle dans la popup Identité (F-ST-80). — `T/User/SettingsGeneral.html`, fr_FR.yml:457-462.

F-ST-53 : Brouillon enregistré automatiquement (`AllowDraftAutosave`) — réglage situé dans l'écran OpenPGP (voir F-ST-179) mais appartenant aux réglages généraux : défaut client vrai, défaut serveur `defaults.allow_draft_autosave` = vrai ; le brouillon est enregistré toutes les minutes (`Time1m`) si la fenêtre est ouverte, non vide, sans envoi/sauvegarde en cours et si un dossier Brouillons est défini. — app.js:8392, 12646, 12772, 12815, 12822-12825.

F-ST-54 : Persistance des réglages — `DoSettingsUpdate` : exige la capa SETTINGS ; `Language` (si autorisé), `MPP`, `Layout`, `EditorDefaultType`, `ShowImages`, `ContactsAutosave`, `DesktopNotifications`, `SoundNotification`, `UseCheckboxesInList`, `AllowDraftAutosave`, `AutoLogout`, `EnableTwoFactor` dans `settings` ; `Theme` (si capa THEMES, sinon forcé au thème par défaut), `UseThreads`, `ReplySameFolder` dans `settings_local` ; seule la clé reçue est mise à jour (`HasActionParam`). `EditorDefaultType` n'est pas validé à l'écriture, seulement à la lecture. — Actions.php:4951-5003, 3748-3776.

F-ST-55 : Priorité des valeurs — l'ordre est : défaut `application.ini` (`defaults.*`) < réglage utilisateur (`settings`) ; le thème et les options locales passent par `settings_local` ; l'écran Général n'a pas de bouton « Enregistrer » (écriture à chaque changement, en parallèle). — Actions.php:1832-1925.

F-ST-56 : Effet mobile — en version mobile le serveur impose Layout = 0, pas de sons ni de notifications, cases à cocher forcées vraies, pas d'image de fond ; seuls les thèmes de la liste `Mobile` sont proposés. — Actions.php:1981-1990, 9130-9136.

F-ST-57 : Libellés absents/non utilisés — `LABEL_ANIMATION*`, `LABEL_SHOW_ANIMATION`, `LABEL_VIEW_OPTIONS`, `LABEL_USE_PREVIEW_PANE` sont définis dans `fr_FR.yml` (l.470-498) sans être référencés par `SettingsGeneral.html`. — fr_FR.yml:470-498.

### B.5 Contacts

F-ST-58 : Ajout automatique des destinataires (`ContactsAutosave`) — case ; défaut client faux, défaut serveur `defaults.contacts_autosave` = vrai ; enregistré immédiatement via `SettingsUpdate`. Effet : à l'envoi d'un message, les destinataires inconnus sont ajoutés au carnet. — `T/User/SettingsContacts.html:7-16`, app.js:36946-36970, 8035, Actions.php:4983.

F-ST-59 : Section « Synchronisation à distance (CardDAV) » — visible si `ContactsSyncIsAllowed` (`contacts.allow_sync`, défaut faux) ET carnet actif. — `T/User/SettingsContacts.html:19`, app.js:11314, Actions.php:1595-1610.

F-ST-60 : Champs CardDAV — case « Activer », URL du carnet (placeholder `https://`, texte libre sans validation), utilisateur, mot de passe. Toute modification (chaîne concaténée `enable|url|user|pass`) est envoyée avec une temporisation de 500 ms à `SaveContactsSyncData`. Le mot de passe n'est jamais renvoyé : le serveur envoie la valeur factice `APP_DUMMY` ; si le client renvoie cette valeur, l'ancien mot de passe est conservé. — app.js:36947-36973, Actions.php:6606-6632, 1595-1610.

F-ST-61 : Intervalle de synchronisation — `contacts.sync_interval` (défaut 20 min), borné côté client à 5..320 min (valeur <5 → 20 ; >320 → 320). Si la synchronisation est activée mais échoue : notification `CONTACTS_SYNC_ERROR` (« Erreur de synchronisation des contacts »). Délai de requête 200 s. — app.js:29585-29598, 7747, Actions.php:6634-6660.

F-ST-62 : Stockage — données chiffrées (`EncodeKeyValues`) dans le stockage CONFIG `contacts_sync`, indexées par compte. `SaveContactsSyncData` retourne faux si le carnet n'est pas actif. — Actions.php:6606-6632.

### B.6 Comptes et identités

F-ST-63 : Section « Liste des comptes » — visible si capa ADDITIONAL_ACCOUNTS (`webmail.allow_additional_accounts`, défaut vrai ; fallback code faux). Spinner pendant le chargement. — `T/User/SettingsAccounts.html:1-15`, app.js:37020, Actions.php:8140-8150.

F-ST-64 : Liste des comptes — chaque ligne : icône, poignée de glisser-déposer, e-mail ; édition au clic (si `canBeEdit`), suppression avec confirmation. Le compte parent (compte racine) n'est ni éditable ni supprimable (`canBeDelete = (email != parentEmail)`). — `T/User/SettingsAccounts.html:19-48`, app.js:32943-32969, 28892-28900.

F-ST-65 : Ordre des comptes — glisser-déposer (jQuery sortable, axe Y, poignée `.drag-handle`) ; à chaque déplacement, `AccountsAndIdentitiesSortOrder` envoie la liste d'e-mails des comptes ET les identifiants d'identités. Le serveur stocke `accounts_identities_order` (JSON) et réordonne `GetAccounts` si ≥ 2 comptes et filtre les entrées dont le jeton fait ≤ 5 caractères. — app.js:37106-37108, Actions.php:3370-3388, 2486-2540.

F-ST-66 : Ajout d'un compte (popup Account) — champs : e-mail (type email, obligatoire), mot de passe (obligatoire). Validation client : `trim` non vide ; erreur de champ en rouge. Envoi `AccountSetup` (New = 1). Succès : fermeture de la popup et rechargement de la liste. — `T/User/PopupsAccount.html`, app.js:27650-27683.

F-ST-67 : Règles serveur de l'ajout — capa ADDITIONAL_ACCOUNTS requise ; erreur `ACCOUNT_ALREADY_EXISTS` (« Ce compte existe déjà ») si l'e-mail égale le compte courant, le compte parent ou une entrée existante ; sinon `LoginProcess` complet (normalisation, domaine, connexion IMAP réelle, **sans** 2FA car `bSkipTwoFactorAuth = true`) puis stockage du jeton d'authentification (contenant le mot de passe) dans `accounts`, sous la clé e-mail ; le compte parent est ajouté lui-même à la liste. Erreurs de connexion : `AUTH_ERROR` et message additionnel IMAP affiché sous l'alerte. — Actions.php:2890-2937, app.js:27664-27680.

F-ST-68 : Mise à jour d'un compte — mêmes champs, e-mail affiché en lecture seule, titre « Mettre à jour le compte ? » ; envoi `New = 0` ; erreur `ACCOUNT_DOES_NOT_EXIST` (« Ce compte n'existe pas ») si absent. Sert à resaisir un mot de passe changé. — `T/User/PopupsAccount.html`, app.js:27696-27703, Actions.php:2917-2921.

F-ST-69 : Suppression d'un compte — bouton corbeille puis confirmation « Êtes-vous sûr ? » (`deleteAccessHelper`, retour automatique) ; retrait immédiat de la liste puis `AccountDelete`. Serveur : refuse de supprimer le compte parent (faux) ; si le compte supprimé est le compte courant, bascule vers le compte parent (`AuthToken`) et répond `Reload: true`, le client recharge alors la page. — app.js:37061-37084, Actions.php:2939-2980.

F-ST-70 : Particularités des comptes additionnels — pas de double authentification pour eux (capa TWO_FACTOR exigée sur compte non additionnel), pas de page d'accueil (`WelcomePage`), pas de réglage Social (compte racine seulement). La 2FA du parent s'applique par `ParentEmailHelper`. — Actions.php:8190-8205, 1630-1650, app.js:36630-36640.

F-ST-71 : Stockage des comptes — liste JSON `accounts` (e-mail → jeton chiffré XXTEA avec mot de passe) dans le stockage CONFIG du compte racine ; si la liste ne contient qu'un seul compte (le parent), elle est effacée. — Actions.php:2486-2540, 2746-2770.

F-ST-72 : Compteurs de non lus par compte — voir F-ST-09 (désactivé côté client).

F-ST-73 : Section « Identités » — visible si capa IDENTITIES (`webmail.allow_additional_identities`, défaut vrai) ; bouton « Ajouter une identité ». — `T/User/SettingsAccounts.html:50-62`, app.js:37021.

F-ST-74 : Liste des identités — chaque ligne : nom formaté (`nom (email)` ou e-mail), poignée de tri, étiquette « (défaut) » sur la première ligne (index 0), suppression avec confirmation (absente pour l'identité principale d'id vide). — `T/User/SettingsAccounts.html:64-97`, app.js:33008-33043.

F-ST-75 : Identité par défaut — la première identité de la liste (index 0) est étiquetée « (défaut) » et c'est elle que la composition sélectionne par défaut (`identities[0]`), sauf pour une réponse/transfert où `findIdentityByMessage` choisit l'identité dont l'e-mail apparaît dans À/Cc/Cci (ou `deliveredTo`) du message d'origine, avec le plus petit index, et pour un brouillon celle de l'expéditeur ou du Reply-To. L'identité principale du compte (id `''`, e-mail = compte, e-mail non modifiable, non supprimable) est créée automatiquement côté serveur si absente (`NewInstanceFromAccount`) et placée en tête. À signaler : l'ordre est envoyé via `identitiesIDS`, calculé avec `_.compact`, qui supprime l'id vide de l'identité principale ; côté serveur `''` est traité comme `---`, absent de la liste envoyée, donc le tri `usort` avec `array_search` (faux) rend la position de l'identité principale non déterministe après un glisser-déposer (à vérifier à l'exécution). — app.js:11260-11264, 12862-12910, 12600, Actions.php:2624-2700, 3370-3388.

F-ST-76 : Popup identité : champs — E-mail (obligatoire, validé `validateEmail`, caché et lecture seule pour l'identité principale), Nom (texte libre), Répondre à (`validateSimpleEmail`, masqué tant que non demandé, liens « Répondre à » / « Cci » pour l'afficher), Cci (`validateSimpleEmail`), case « Insérer la signature avant le texte cité dans les réponses » (défaut faux), éditeur de signature (HTML ou texte). — `T/User/PopupsIdentity.html`, app.js:27765-27800.

F-ST-77 : Validation de l'identité — e-mail vide ou invalide : bloque et redonne le focus (sauf identité principale) ; Répondre à / Cci invalides : bloque avec focus ; sinon `IdentityUpdate` (`Id`, `Email`, `Name`, `ReplyTo`, `Bcc`, `Signature`, `SignatureInsertBefore` '0'/'1'). Nouvelle identité : id aléatoire `fakeMd5()` généré à l'ouverture. Succès : popup fermée, rechargement des comptes/identités ; erreur : message sous l'alerte. — app.js:27804-27849, 27875-27891.

F-ST-78 : Règles serveur de l'identité — `Identity::FromJSON` exige un `Email` non vide ; e-mail converti en ASCII (IDN) ; absence de `SignatureInsertBefore` ⇒ vrai (mais le client envoie toujours '0' ou '1'). `DoIdentityUpdate` ne vérifie PAS la capa IDENTITIES ; c'est `SetIdentities` qui ignore les identités non principales si la capa est absente. Une identité dont l'id existe est remplacée, sinon ajoutée en fin de liste. — Model/Identity.php, Actions.php:3182-3222, 2773-2795.

F-ST-79 : Suppression d'identité — `IdentityDelete` exige la capa IDENTITIES ; id vide : `UNKNOWN_ERROR`. — Actions.php:3225-3257.

F-ST-80 : Édition de l'identité principale — possible depuis Général ou depuis la liste ; nom, Répondre à, Cci, signature et position de la signature modifiables ; e-mail non modifiable. — app.js:27875-27888, 36823-36829.

F-ST-81 : Signature : texte ou HTML — le contenu est stocké tel que fourni par l'éditeur ; le HTML est préfixé `:HTML:`. À l'insertion dans un message, le préfixe détermine le mode. — app.js:13075-13091, 18364-18390.

F-ST-82 : Signature : variables — `{{FROM}}` (nom ou partie locale de l'expéditeur du message auquel on répond, vide si nouveau message), `{{FROM-FULL}}` (nom + adresse), `{{DATE}}` (format `llll` de moment), `{{TIME}}` (format `LT`), `{{MOMENT:format}}` (format moment libre ; 10 occurrences maximum, le reste est supprimé). Les retours chariot `\r` sont supprimés ; 1 ou 2 espaces précédant `{{FROM}}`/`{{FROM-FULL}}` sont consommés ; les variables non résolues sont effacées. — app.js:13013-13072.

F-ST-83 : Signature avant/après le texte cité — `signatureInsertBefore` est passé à `editor.setSignature(…, insertBefore)` à l'ouverture d'un message ou au changement d'identité d'envoi. — app.js:13075-13091.

F-ST-84 : Effets de « Répondre à » et « Cci » d'une identité — dans la fenêtre de composition, sélectionner une identité ajoute son Cci et son Reply-To aux champs Cci / Répondre à (sans doublon) et les retire quand on en change (`fEmailOutInHelper`). À l'envoi, le serveur n'utilise de l'identité choisie (`IdentityID`) que l'e-mail et le nom pour l'expéditeur ; si l'identité n'existe plus, l'expéditeur est l'e-mail du compte. Le Reply-To de l'identité par défaut sert aussi aux accusés de lecture générés. — app.js:12392-12407, 12600-12610, 12913-12916, Actions.php:5831-5840, 6008-6035.

F-ST-85 : Stockage des identités — liste JSON `identities` dans le stockage local (`StorageProvider(true)`, donc par compte et non partagée avec le compte parent), plus l'ordre dans `accounts_identities_order`. — Actions.php:2624-2700, 2773-2795.

F-ST-86 : Raccourci « identités » dans la composition — `` ` `` / `Ctrl+`` ouvre la liste de sélection d'identité (libellé `SHORTCUTS_HELP/LABEL_OPEN_IDENTITIES_DROPDOWN`). — `T/Common/PopupsKeyboardShortcutsHelp.html`.

### B.7 Filtres (Sieve)

F-ST-87 : Disponibilité — l'entrée de menu exige la capa `SIEVE` ; côté serveur `SIEVE` n'est ajoutée que si `capa.filters` (défaut vrai, mais la valeur de repli du code est faux) ET le domaine du compte a Sieve activé (`UseSieve`). La capa FILTERS seule suffit aux appels PHP mais pas au menu. — app.js:36621-36625, Actions.php:8160-8175, 2821-2848.

F-ST-88 : Chargement — `Filters` : connexion ManageSieve (timeout 10 s `labs.sieve_timeout`) ; modules détectés : `fileinto`→ « Déplacer vers » et « Faire suivre » ; `regex` ; `reject` ; `vacation` ; `imap4flags` → « Marquer comme lu » ; `relational` et `date` sont détectés mais non exploités. Erreur : bandeau rouge avec `CANT_GET_FILTERS` (« Impossible d'obtenir les filtres ») ou le code serveur ; l'interface d'édition est masquée en cas d'erreur serveur. — app.js:37261-37306, SieveStorage.php:51-107.

F-ST-89 : Liste des filtres — case d'activation (désactivé ⇒ bloc Sieve commenté), poignée de tri (glisser-déposer), nom + sous-nom de l'action (« Déplacer vers "dossier" », « Faire suivre à "adresse" », « Message d'absence », « Rejeter », « Exclure »), suppression avec confirmation « Êtes-vous sûr ? ». L'ordre de la liste est l'ordre d'exécution. — `T/User/SettingsFilters.html:70-100`, app.js:30130-30163.

F-ST-90 : Enregistrement global — les filtres ne sont pas sauvegardés individuellement : un bandeau « Ces changements doivent être enregistrés sur le serveur. » s'affiche dès qu'une modification existe ; bouton « Enregistrer » (désactivé sans changement ni pendant l'enregistrement). Erreur : infobulle rouge (`ErrorMessageAdditional` du serveur, sinon `CANT_SAVE_FILTERS`). — `T/User/SettingsFilters.html:3-45`, app.js:37224-37251.

F-ST-91 : Script brut — bouton (icône fichier de code, tooltip « Utiliser le script d'un utilisateur ») visible si `RawIsAllow`, c'est-à-dire `DomainSieveAllowRaw` (option de domaine ; `labs.sieve_allow_raw_script` défaut faux). Active un éditeur texte (300 px) et la liste de capacités du serveur (« Capacité : … »). Script actif vide → erreur de champ et blocage. À l'enregistrement : script brut enregistré sous `rainloop.user.raw` ; si actif, il remplace la liste de filtres (`rainloop.user`). — `T/User/SettingsFilters.html:17-25, 56-68`, app.js:37224-37235, SieveStorage.php:110-150.

F-ST-92 : Popup Filtre — nom (obligatoire), groupe de conditions, groupe d'actions. Titres « Créer un filtre ? » / « Mettre à jour le filtre ? » ; bouton « Fait ». Un filtre édité est cloné, remplacé dans la liste au « Fait ». — `T/User/PopupsFilter.html`, app.js:31498-31526, 31637-31654.

F-ST-93 : Conditions : opérateur global — liste « Correspondant à l'une des règles suivantes » (`Any`) ou « … à toutes les règles suivantes » (`All`) ; affichée seulement si ≥ 2 conditions ; sans condition : « Tous les messages entrants » (condition universelle). — `T/User/PopupsFilter.html:22-36`, SieveStorage.php:300-345.

F-ST-94 : Conditions : champs et opérateurs — champs : De, Destinataires (À ou Cc), Sujet, Taille, En-tête (nom d'en-tête libre + valeur) ; opérateurs : Contient, Ne contient pas, Égal à, Différent de, et « Regex » si le module `regex` existe ; pour la taille : « Au-dessus de » / « Au-dessous de » (`:over`/`:under`, valeur numérique brute). Les libellés `MATCHES`, `REGEXP` existent dans la langue sans usage. — `T/User/SettingsFiltersCondition*.html`, app.js:31572-31615, SieveStorage.php:200-300.

F-ST-95 : Conditions : validation — valeur vide ⇒ erreur ; champ En-tête sans nom d'en-tête ⇒ erreur ; plusieurs adresses séparées par des virgules acceptées pour De/Destinataires (liste Sieve). Un changement de champ vide la valeur. — app.js:30418-30430, 30405-30410, SieveStorage.php:268-285.

F-ST-96 : Actions disponibles — « Déplacer vers » (liste des dossiers abonnés), « Faire suivre à » (adresse), « Rejeter » (message), « Message d'absence », « Exclure » (discard) ; une action « Aucune » (stop) existe dans le code mais n'est pas proposée. Les trois premières dépendent des modules Sieve. — app.js:31528-31571, SieveStorage.php:340-430.

F-ST-97 : Action « Déplacer vers » — dossier cible obligatoire ; coche « Marquer comme lu » (`addflag \Seen`, si module `imap4flags`) ; script : `fileinto "<dossier>"; stop;`. Le nom de dossier est converti de UTF-7 IMAP en UTF-8 si `labs.sieve_utf8_folder_name` (défaut vrai). `labs.sieve_allow_fileinto_inbox` (défaut faux) contrôle la présence d'INBOX dans la liste des dossiers. — `T/User/SettingsFiltersActionMoveToFolder.html`, SieveStorage.php:390-425, Actions.php:1505.

F-ST-98 : Action « Faire suivre à » — adresse obligatoire contenant `@` ; cases « Garder » (`fileinto "INBOX"` avant le `redirect`), « Marquer comme lu » ; script : `[fileinto "INBOX";] redirect "<adresse>"; stop;`. — `T/User/SettingsFiltersActionForward.html`, app.js:30241-30245, SieveStorage.php:375-390.

F-ST-99 : Action « Rejeter » — message obligatoire ; script `reject "<message>"; stop;`. — `T/User/SettingsFiltersActionReject.html`, SieveStorage.php:360-375.

F-ST-100 : Action « Message d'absence » — Délai de réponse (liste 1, 2, 3, 7, 30 jours ; défaut vide traité comme 1 ; toute valeur numérique ≤ 1 ⇒ 1), Destinataires (adresses séparées par des virgules, bouton « + » pour insérer toutes les adresses de comptes ; validation : si renseigné, doit contenir `@`), Sujet (optionnel), Message (obligatoire), case « Ne pas arrêter le traitement des règles » ; script `vacation :days N [:addresses […]] [:subject "…"] "message";` suivi de `stop;` sauf si la case est cochée. — `T/User/SettingsFiltersActionVacation.html`, app.js:30246-30250, 30285-30287, SieveStorage.php:318-360.

F-ST-101 : Action « Exclure » — `discard; stop;`, sans paramètre. — SieveStorage.php:315-318.

F-ST-102 : Format de stockage — script `rainloop.user` : en-tête commentaire « RAINLOOP:SIEVE », pour chaque filtre un bloc `BEGIN:FILTER:<id>` + en-tête JSON sérialisé en base64 (74 colonnes) + corps Sieve ; un filtre désactivé est commenté `/* @Filter is disabled … */`. Le script est supprimé s'il est vide. `Stop` côté client = inverse de « Ne pas arrêter » (`Stop = noStop ? '0' : '1'`). — SieveStorage.php:476-545, app.js:30256-30274.

F-ST-103 : Contrôles de validité du filtre — nom vide ⇒ erreur ; valeur d'action obligatoire pour MoveTo/Forward/Reject/Vacation ; `@` obligatoire pour Forward ; à l'ouverture d'un nouvel ajout un nouvel id est généré (`fakeMd5`) ; ajouter un filtre désactive le mode brut. — app.js:30220-30253, 37314-37324.

### B.8 Sécurité et authentification à deux facteurs

F-ST-104 : Disponibilité de l'écran — menu si capa AUTOLOGOUT ou TWO_FACTOR. AUTOLOGOUT est présente sauf sous ownCloud. TWO_FACTOR exige `security.allow_two_factor_auth` (défaut faux) et un compte NON additionnel. — app.js:36626-36629, Actions.php:8180-8215.

F-ST-105 : Anomalie de visibilité — dans `SettingsSecurity.html`, le lien « Configurer l'authentification en deux étapes » est à l'intérieur du bloc `visible: capaAutoLogout` : si AUTOLOGOUT est absente alors que TWO_FACTOR est présente, la page est vide. — `T/User/SettingsSecurity.html:2-33`.

F-ST-106 : Déconnexion automatique : valeurs — liste : Jamais (0), 5, 10, 30, 60 minutes, 2 h (120), 5 h (300), 10 h (600). Défaut client 30 ; défaut serveur `defaults.autologout` = 30 ; si la capa AUTOLOGOUT manque, la valeur envoyée est 0. Le serveur n'impose aucune borne à l'écriture (`int`). Enregistrement immédiat avec indicateur. — `T/User/SettingsSecurity.html:6-22`, app.js:37399-37462, 8395, Actions.php:1843, 1889-1894, 5004.

F-ST-107 : Déconnexion automatique : mécanisme — un minuteur est armé à `autoLogout` minutes ; toute activité (`mousemove`, `keypress`, `click`, anti-rebond 5 s) rappelle `rl.auto-logout-refresh` qui réarme. Le minuteur est désactivé (non armé) si `AccountSignMe` est vrai ou si la valeur est 0. À l'échéance : `logout()` (même chemin que la déconnexion manuelle). — app.js:8431-8440, 23284-23286, 29618-29620.

F-ST-108 : Lien « Configurer l'authentification en deux étapes » — ouvre la popup de configuration (icône cadenas). Visible si capa TWO_FACTOR. — `T/User/SettingsSecurity.html:24-33`, app.js:37449-37451.

F-ST-109 : Prérequis serveur de la 2FA — `security.allow_two_factor_auth` (défaut faux) doit être vrai (sinon le fournisseur est nul et toutes les actions 2FA retournent faux) ; `security.force_two_factor_auth` (défaut faux) ajoute TWO_FACTOR_FORCE. Fournisseur : `GoogleTwoFactorAuth` (TOTP, bibliothèque PHPGangsta). — Actions.php:831-842, 8185-8212, Providers/TwoFactorAuth/GoogleTwoFactorAuth.php.

F-ST-110 : Popup de configuration : ouverture — `GetTwoFactorInfo` à la construction : statut (Configuré / Non configuré), utilisateur (e-mail du compte racine), case « Activer l'authentification en deux étapes » (visible si configuré), lien « test », boutons Activer / Effacer / Fait. — `T/User/PopupsTwoFactorConfiguration.html`, app.js:31890-31895, 31843-31870.

F-ST-111 : Création du secret — bouton « Activer » (appelle `CreateTwoFactorSecret`) : secret TOTP de 16 caractères (alphabet base32), **9 codes de secours** à 9 chiffres (`rand(100000000, 900000000)`), stockés chiffrés (`EncodeKeyValues`) dans `two_factor` du compte racine avec `Enable = false` ; la réponse retourne secret, codes, titre d'URL (`webmail.title`). L'interface affiche le secret, un QR code (niveau M, taille 8) `otpauth://totp/<user>?secret=<secret>&issuer=` (issuer vide), et les codes de secours séparés par deux espaces. — Actions.php:6788-6826, PHPGangsta/GoogleAuthenticator.php:14-30, app.js:31802-31805, 31838-31870.

F-ST-112 : Afficher/masquer le secret — lien « Montrer la clé secrète » : `ShowTwoFactorSecret` retourne le secret et le titre mais PAS les codes de secours (supprimés de la réponse) ; QR taille 6 ; lien « Masquer la clé secrète » efface secret, codes, URL. Texte d'aide : importer dans Google Authenticator ou un client TOTP. — Actions.php:6828-6848, app.js:31790-31800, 31872-31888.

F-ST-113 : Codes de secours — affichés une seule fois après création (zone `pre`), texte « …Après avoir fait cela, il deviendra inactif » ; chaque code est à usage unique ; le client ne les redonne plus ensuite. — `T/User/PopupsTwoFactorConfiguration.html:80-92`, Actions.php:2356-2380.

F-ST-114 : Test du code — lien « test » ouvre la popup Test : champ code (focus auto hors mobile), bouton « Test » désactivé si vide ou en cours ; succès ⇒ bouton vert et `twoFactorTested(true)` ; échec ⇒ bouton rouge. Serveur : `VerifyCode(secret, code, 8)`, tolérance ±8 intervalles de 30 s (±4 min), code à 6 chiffres. Le serveur impose une pause (`requestSleep`). — `T/User/PopupsTwoFactorTest.html`, app.js:39602-39659, Actions.php:6884-6911, GoogleAuthenticator.php:86-110.

F-ST-115 : Activation de la 2FA — la case « Activer… » n'est activée qu'après un test réussi (`twoFactorAllowedEnable = viewEnable || tested`) ; infobulle « Vous ne pouvez pas modifier ce paramètre avant de l'avoir essayé. ». Cocher envoie `EnableTwoFactor(1)` ; décocher envoie `0`. Échec serveur ⇒ la case revient décochée. Le serveur, lui, n'exige pas de test préalable. — app.js:31746-31780, Actions.php:6850-6882.

F-ST-116 : Effacement — bouton rouge « Effacer » : supprime l'entrée `two_factor` (secret, codes, état) et vide l'interface. — `T/User/PopupsTwoFactorConfiguration.html:96-100`, Actions.php:6913-6935.

F-ST-117 : 2FA obligatoire (`TWO_FACTOR_FORCE`) — si le compte (non additionnel) n'a pas de 2FA configurée et activée, `RequireTwoFactor` est vrai et la popup s'ouvre en mode verrouillé (`lock`) dès le démarrage : message « Votre compte requiert la configuration d'une vérification en deux étapes. », boutons Fermer/Fait masqués tant que la 2FA n'est pas activée, bouton « Déconnexion » proposé ; à la fermeture le client recharge la page. — app.js:29509-29511, 31825-31837, `T/User/PopupsTwoFactorConfiguration.html:3-8, 30-35, 100-110`, Actions.php:1800-1815.

F-ST-118 : Portée de la 2FA — les données sont liées à `ParentEmailHelper()` ; la vérification à la connexion s'applique au compte racine ; l'ajout d'un compte additionnel n'exige jamais de code. — Actions.php:6666-6725, 2890-2937.

F-ST-119 : Vérification à la connexion — si 2FA configurée et activée et que le cookie « ne plus demander » ne correspond pas : sans code ⇒ `ACCOUNT_TWO_FACTOR_AUTH_REQUIRED` (« Double authentification requise ») traduit par l'affichage du champ code ; avec code faux ⇒ `ACCOUNT_TWO_FACTOR_AUTH_ERROR`, délai, journal d'échec d'authentification. — Actions.php:2318-2388.

F-ST-120 : Réglage `EnableTwoFactor` — également persisté dans `settings` via `SettingsUpdate` et relu dans les données de démarrage (`EnableTwoFactor`). — Actions.php:1848, 1908.

F-ST-121 : Données retournées sans secret — `GetTwoFactorInfo` et `ClearTwoFactorInfo` appellent `getTwoFactorInfo(..., true)` qui supprime `Secret`, `UrlTitle` et `BackupCodes`. — Actions.php:6666-6725.

F-ST-122 : Le « délai de deux semaines » du libellé de connexion correspond à 14 jours (`60*60*24*14`), non paramétrable ; la durée du cookie « se souvenir de moi » est de 30 jours (non paramétrable). — Actions.php:2380-2386, 636-640.

### B.9 Social

F-ST-123 : Disponibilité — écran visible seulement pour le compte racine et si (Google avec auth) OU Facebook OU Twitter est activé. Chaque bloc a sa propre condition : Google (`googleEnable && googleEnableAuth`), Facebook (`facebookEnable`), Twitter (`twitterEnable`). — app.js:36636-36640, `T/User/SettingsSocial.html`.

F-ST-124 : Connecter / déconnecter — pour chaque réseau, bouton « Se connecter à X » (si non connecté) ouvrant la fenêtre d'autorisation, ou nom du compte lié et bouton « Se déconnecter de X » ; un indicateur « en cours » masque les boutons pendant l'action. Les noms sont lus par `SocialUsers` (`GoogleSocialName`, `FacebookSocialName`, `TwitterSocialName`) et rechargés par des fonctions globales de rappel. — app.js:37524-37552, 29171-29232, Actions.php:7561-7612.

F-ST-125 : Texte d'aide — « Après l'activation de la connexion via Google/Facebook/Twitter, vous pouvez vous connecter à ce compte en utilisant le bouton … sur l'écran de connexion. » (`MAIN_*_DESC`). Une identité sociale non liée produit `SOCIAL_*_LOGIN_ACCESS_DISABLE` à la connexion. — fr_FR.yml:519-531, 640-660.

### B.10 Mot de passe

F-ST-126 : Disponibilité — entrée « Mot de passe » visible si `ChangePasswordIsAllowed` : un pilote `change-password` doit exister (fourni par un plugin ; aucun pilote natif dans `fabrica`) ET `PasswordChangePossibility(compte)`. Sans plugin, l'écran n'apparaît jamais. — app.js:36641-36643, Actions.php:816-826, 345-380, Providers/ChangePassword.php.

F-ST-127 : Champs — mot de passe actuel, nouveau mot de passe, confirmation (3 champs `type=password`, `autocomplete` `current-password` / `new-password`). Le bouton « Enregistrer le nouveau mot de passe » est actif seulement si les trois champs sont non vides et aucune requête n'est en cours. — `T/User/SettingsChangePassword.html`, app.js:37593-37595.

F-ST-128 : Contrôles client — si nouveau ≠ confirmation : champs en erreur, message `ERROR_PASSWORD_MISMATCH` (« Les mots de passe ne correspondent pas, merci de réessayer ») sans appel serveur. — app.js:37634-37638.

F-ST-129 : Contrôles serveur — (1) mot de passe actuel différent du mot de passe de session ⇒ `CURRENT_PASSWORD_INCORRECT` (champ actuel en erreur) ; (2) nouveau mot de passe (après `trim`) de moins de 6 caractères ⇒ `NEW_PASSWORD_SHORT` ; (3) `PasswordWeaknessCheck` : refus (`NEW_PASSWORD_WEAK`) si le texte (en minuscules) est une sous-chaîne d'une longue liste interne de mots de passe courants (ex. « password », « 123456 », « admin », « azerty », « test ») ; (4) échec du pilote ⇒ `COULD_NOT_SAVE_NEW_PASSWORD`. L'option `labs.check_new_password_strength` est passée au fournisseur mais `ChangePassword()` ne la consulte pas : le contrôle est toujours appliqué. — Providers/ChangePassword.php, MailSo/Base/Utils.php:2510-2520, Actions.php:5036-5062.

F-ST-130 : Après succès — champs vidés, bouton vert, nouveau jeton d'authentification renvoyé et appliqué côté client (`setClientSideToken`) ; le mot de passe de session est mis à jour. Toute saisie ultérieure efface les états d'erreur/succès. Délai de réponse d'erreur (`loginErrorDelay`). — app.js:37659-37680, Providers/ChangePassword.php:55-62.

F-ST-131 : Messages affichés — zone rouge à droite des champs (`errorDescription`), texte via `getNotificationFromResponse` ; défaut `COULD_NOT_SAVE_NEW_PASSWORD`. — `T/User/SettingsChangePassword.html:43-46`, app.js:37677-37679.

F-ST-132 : Remise à zéro — à la sortie de l'écran (`onHide`) les trois champs et les erreurs sont vidés. — app.js:37649-37657.

### B.11 Modèles de messages

F-ST-133 : Disponibilité — capa TEMPLATES : option `capa.x-templates` (défaut FAUX dans `application.ini`) ET non mobile ET capa SETTINGS. Par défaut, les modèles sont donc désactivés. — Application.php:253, Actions.php:8147-8150.

F-ST-134 : Liste des modèles — nom, poignée de glisser-déposer, bouton de suppression avec confirmation, clic = édition. Le chargement affiche un spinner. NB : le glisser-déposer est défini (`sortable`) sans callback de sauvegarde (`afterMove`) : l'ordre n'est pas persisté (`DoTemplateSetup` replace aussi le modèle édité en fin de liste). — `T/User/SettingsTemplates.html`, app.js:37727-37733, Actions.php:3259-3300.

F-ST-135 : Popup modèle — champs : Nom (obligatoire après `trim`), corps (éditeur riche/texte, obligatoire : corps vide ou égal à `:HTML:` ⇒ erreur de champ). Titres « Ajouter/Mettre à jour le modèle ? ». Le corps est enregistré avec `getDataWithHtmlMark()` (préfixe `:HTML:` si HTML). — `T/User/PopupsTemplate.html`, app.js:31982-32009, 32027-32031.

F-ST-136 : Corps différé — la liste ne contient pas les corps (`Populated=false`, `Body=''`, toujours, car `|| true`) ; à l'ouverture d'un modèle existant `TemplateGetByID` charge le corps ; échec ⇒ erreur de champ corps et éditeur vide. — Model/Template.php:60-80, app.js:32052-32081, Actions.php:3334-3368.

F-ST-137 : Règles serveur — `TemplateSetup` : capa TEMPLATES requise, `ID`, `Name`, `Body` présents sinon `INVALID_INPUT_ARGUMENT` ; id vide ⇒ `GenerateID()` (md5 aléatoire) ; si un modèle de même id existe il est retiré puis le nouveau est ajouté en fin ; stockage JSON `templates` dans le stockage local. `TemplateDelete` : id vide ⇒ `UNKNOWN_ERROR`. `TemplateGetByID` sur un id inconnu appelle une méthode sur `false` (erreur PHP, cas limite non géré). — Actions.php:3259-3368, 2808-2819.

F-ST-138 : Insertion dans un message — AUCUNE interface d'insertion de modèle dans la composition 1.17 : `templatesNames` (store) est calculé mais jamais consommé ; le code d'insertion est commenté. Les modèles sont donc créés/édités sans pouvoir être insérés par l'interface standard. — app.js:17063-17095.

F-ST-139 : Libellés manquants — les clés `SETTINGS_TEMPLATES/LEGEND_TEMPLATES`, `BUTTON_ADD_TEMPLATE`, `DELETING_ASK`, `POPUPS_ADD_TEMPLATE/*` (titre, nom, boutons) et `SETTINGS_TEMPLETS/LOADING_PROCESS` (faute de frappe) n'existent dans aucun fichier de langue du dossier `webmail` (vérifié sur `fr_FR.yml`). L'écran affiche donc des libellés vides ou bruts. — `T/User/SettingsTemplates.html`, `T/User/PopupsTemplate.html`, app.js:37717.

F-ST-140 : Variables de modèle — aucune variable n'est interprétée dans les modèles (contrairement aux signatures, F-ST-82). — app.js:31982-32031.

F-ST-141 : Boutons de l'éditeur de modèle — même éditeur `HtmlEditor` que la composition (CKEditor) ; boutons source/bit d'`allow_html_editor_source_button` et `allow_html_editor_biti_buttons` (défaut faux). — app.js:32033-32045, Actions.php:1445-1447.

### B.12 Dossiers

F-ST-142 : Disponibilité — capa FOLDERS (`capa.folders`, défaut vrai) ; sans elle, seule la boîte INBOX est listée, la création/renommage/suppression/abonnement sont refusés et le dossier d'envoi est forcé à « non utilisé ». — app.js:36644-36648, Actions.php:5269-5290, app.js:12701-12703.

F-ST-143 : Liste des dossiers — arbre indenté (classes `deep-N`), nom localisé pour les dossiers système (suffixe « (Boîte de réception) » etc. via `manageFolderSystemName`), spinner pendant chargement/création/suppression/renommage. Si `Optimized` : alerte « Vous avez trop de dossiers ! Seulement une partie d'entre eux ont été affichés… » (limite serveur `labs.imap_folder_list_limit`, défaut 200). — `T/User/SettingsFolders.html`, `SettingsFolderItem.html`, app.js:32583-32620, 29995, Actions.php:5270-5278.

F-ST-144 : Panneau d'aide — au survol des icônes corbeille / œil / coche, un encadré d'aide affiche : « Supprimer le dossier », « Afficher/masquer le dossier », « Cocher/Décocher pour les nouveaux messages » (masqué sur mobile). — `T/User/SettingsFolders.html:38-44`, app.js:37870-37881.

F-ST-145 : Création de dossier (popup) — champs : nom (validation `/^[^\\/]+$/` : non vide, sans `\` ni `/` ; bouton inactif sinon), dossier parent (liste hiérarchique, valeur vide = racine ; s'il existe un espace de noms, les dossiers hors espace de noms sont désactivés ; racine = espace de noms sans son séparateur final si sa longueur > 1). Appel `FolderCreate(Folder, Parent)` puis fermeture immédiate de la popup ; l'erreur éventuelle est affichée dans la liste (`CANT_CREATE_FOLDER`, « Impossible de créer le dossier »). — `T/User/PopupsFolderCreate.html`, app.js:31343-31398, Actions.php:5415-5441.

F-ST-146 : Création côté serveur — `MailClient.FolderCreate` avec abonnement automatique si `labs.use_imap_list_subscribe` (défaut vrai). Toute exception ⇒ `CANT_CREATE_FOLDER`. — Actions.php:5415-5441.

F-ST-147 : Renommage — clic sur le nom d'un dossier utilisateur (`canBeEdited` : type utilisateur, existant, sélectionnable) ouvre un champ ; `Entrée` valide (nom non vide, différent, après `trim`), `Échap` annule. Le client met immédiatement le nom à jour, vide le cache de dossiers (`FoldersLashHash`) et appelle `FolderRename`. Erreur : `CANT_RENAME_FOLDER` (« Impossible de renommer le dossier »). Aucune validation de caractères (`/`) côté client dans cette zone. — `T/User/SettingsFolderItem.html:4-9`, app.js:37847-37864, Actions.php:5529-5557.

F-ST-148 : Suppression — icône corbeille puis confirmation « Êtes-vous sûr ? » ; autorisée seulement si le dossier est utilisateur (non système), sans sous-dossier, n'est pas INBOX et `privateMessageCountAll() == 0` ; sinon alerte `CANT_DELETE_NON_EMPTY_FOLDER` (« Impossible de supprimer un dossier non vide »). Serveur : `NonEmptyFolder` ⇒ `CANT_DELETE_NON_EMPTY_FOLDER` ; autre ⇒ `CANT_DELETE_FOLDER`. — app.js:37892-37914, 32536-32540, Actions.php:5559-5590.

F-ST-149 : Abonnement (afficher/masquer) — icône œil (grisée si masqué) sur chaque dossier non système, sélectionnable et non INBOX ; applique immédiatement, vide le cache et appelle `FolderSubscribe(Subscribe=1/0)`. Les erreurs ne sont pas remontées (callback `noop`). La colonne d'abonnement est masquée (classe `ignore-folder-subscribe`) si `labs.use_imap_list_subscribe` est faux (`useImapSubscribe`). — app.js:37916-37926, 32541-32543, `T/User/SettingsFolders.html:1`, Actions.php:5443-5475.

F-ST-150 : Dossiers « à vérifier » (Checkable) — coche visible pour les dossiers abonnés, non système et non INBOX ; stocke dans `settings_local` la liste JSON `CheckableFolder` (sans doublon). Dossiers cochés : leurs compteurs de messages/non lus sont rafraîchis périodiquement. — `T/User/SettingsFolderItem.html:27-36`, Actions.php:5477-5527, app.js:37928-37936.

F-ST-151 : Colonne « à vérifier » conditionnelle — affichée seulement si `displaySpecSetting` : nombre de dossiers = 0 OU supérieur à `labs.folders_spec_limit` (défaut 50, borné 10..100 côté client). En dessous du seuil la colonne est masquée et tous les dossiers sont traités comme vérifiables. — app.js:29977-29986, 29923-29940, Actions.php:1478.

F-ST-152 : Sélection des dossiers à vérifier — `getNextFolderNames` choisit au maximum 5 dossiers par passage, non INBOX, existants, sélectionnables, non vérifiés depuis > 5 minutes, et soit système, soit abonnés ET cochés ; triés du plus ancien au plus récent ; relevé toutes les 2 min (+5 min de décalage) par `FolderInformationMultiply`. Mise à jour des compteurs, détection de changement de hash (rechargement de la liste si c'est le dossier courant). — app.js:4919-4955, 29029-29097.

F-ST-153 : Dossiers système — bouton « Dossiers systèmes » ouvre la popup : cinq listes déroulantes Envoyés, Brouillons, Indésirables (Spam), Corbeille, Archive ; chaque liste propose « Faites un choix », « Ne pas utiliser » (`__UNUSE__`), puis tous les dossiers. Toute modification est sauvegardée (anti-rebond 1 s) via `SystemFoldersUpdate` (`SentFolder`, `DraftFolder`, `SpamFolder`, `TrashFolder`, `ArchiveFolder`, plus `NullFolder='NullFolder'`) et reflétée immédiatement dans les réglages client. Le serveur ne valide ni capa ni noms. — `T/User/PopupsFolderSystem.html`, app.js:24038-24075, Actions.php:3637-3652.

F-ST-154 : Notifications guidées des dossiers système — si un envoi est tenté sans dossier « Envoyés » défini, la popup s'ouvre avec un message explicatif (`NOTIFICATION_SENT/DRAFTS/SPAM/TRASH/ARCHIVE`) ; de même pour Brouillons, Spam, Corbeille et Archive lors des actions correspondantes. — app.js:24085-24116, 12701-12706, fr_FR.yml:357-378.

F-ST-155 : Auto-détection et création — au premier chargement (aucun dossier système enregistré), le serveur déduit les dossiers par attributs IMAP (SPECIAL-USE) ou par noms usuels (`recFoldersTypes`) ; avec `labs.autocreate_system_folders` (défaut vrai) et la capa FOLDERS, il crée ceux qui manquent (Envoyés, Brouillons, Indésirables, Corbeille, Archive/« All ») ; le client enregistre le résultat. — Actions.php:5290-5390, app.js:29988-30020.

F-ST-156 : Purger un dossier (popup Folder Clear) — message d'avertissement en rouge « ATTENTION ! … effacera tous les emails du dossier <nom> de manière définitive. Une fois lancé, le processus ne peut pas être interrompu ou annulé. » ; bouton « Vider » désactivé pendant l'opération ; le titre devient « Purge du dossier... » ; en cas d'échec, `clearingError` est calculé (`MAIL_SERVER_ERROR` par défaut) mais **n'est affiché nulle part** dans le gabarit. Le serveur n'exige pas la capa FOLDERS ni DANGEROUS_ACTIONS. L'action est lancée depuis le menu contextuel de la liste de dossiers de la boîte aux lettres. — `T/User/PopupsFolderClear.html`, app.js:39264-39345, Actions.php:5592-5612.

F-ST-157 : Confirmation de suppression de messages (popup Ask) — voir F-ST-186. — app.js:20391+.

F-ST-158 : Persistance et cache — chaque opération (création, suppression, renommage, abonnement) réinitialise `FoldersLashHash` côté client, ce qui force le rechargement de la liste ; le serveur calcule `FoldersHash` (md5 des noms) pour détecter les changements. — app.js:37851, 37906, Actions.php:5380-5388.

F-ST-159 : Message d'erreur global — bandeau rouge fermable (`folderList.error`) pour les erreurs des promesses (`foldersPromisesActionHelper`), avec code par défaut selon l'action (`CANT_RENAME_FOLDER`, `CANT_DELETE_FOLDER`, `CANT_CREATE_FOLDER`). — `T/User/SettingsFolders.html:26-31`, app.js:28796-28805.

F-ST-160 : Dossiers non sélectionnables / inexistants — ils ne peuvent ni être abonnés, ni cochés, ni renommés ; ils restent visibles s'ils ont des sous-dossiers abonnés. — app.js:32473-32495, 32541-32544.

F-ST-161 : Espace de noms IMAP — le champ « Dossier parent » désactive les dossiers hors de l'espace de noms ; la racine par défaut tient compte de l'espace de noms. — app.js:31354-31375, 29988-29995.

F-ST-162 : Dossier « Archive » — type `ServerFolderType.ALL` ; l'archivage est déclenché par la touche `Z` (libellé « Archiver »). — Actions.php:5198-5260, `T/Common/PopupsKeyboardShortcutsHelp.html`.

F-ST-163 : Thread / dossiers — sans objet pour les réglages ; la liste de messages utilise `labs.use_imap_thread`. — app.js:29994.

F-ST-164 : Quota — `capa.quota` (défaut vrai) active l'affichage du quota (relevé toutes les 15 min) ; pas de réglage utilisateur. — Actions.php:6556-6600, app.js:28946-28952.

F-ST-165 : Statut de chargement — l'indicateur global combine chargement, création, suppression et renommage (`loading` calculé). — app.js:37831-37837.

F-ST-166 : Mobile — l'aide au survol est masquée sur mobile ; le reste est identique. — `T/User/SettingsFolders.html:40`.

### B.13 Thèmes

F-ST-167 : Disponibilité — capa THEMES (`webmail.allow_themes`, défaut vrai ; fallback code faux ; non mobile). Sinon le thème est forcé au thème par défaut du serveur. — Actions.php:8148, 4966-4975.

F-ST-168 : Liste des thèmes — dossiers de `themes/` contenant `styles.less` (installés : A, Blurred, DarkShine, Default, Linen, Love, Snow, Squares, SquaresDark, Stripes, Wood, Xv, Clear ; `Mobile` exclu pour le bureau) plus thèmes personnalisés d'un dossier `themes/` à la racine web, suffixés `@custom`. Ordre : `Default` en premier, `Clear` en dernier, les autres triés alphabétiquement. — Actions.php:9130-9215, dossier `themes/`.

F-ST-169 : Affichage — vignette (`images/preview.png`) et nom lisible (suppression de `@custom`, découpage CamelCase en mots : « SquaresDark » → « Squares Dark ») ; la vignette du thème sélectionné est mise en évidence. — `T/User/SettingsThemes.html:10-17`, app.js:2410-2415, 6490-6499, 38035-38045.

F-ST-170 : Application d'un thème — un clic change `theme()` : chargement AJAX du CSS (`Css/0/User/<thème>/` en JSON), injection dans une balise `<style>` sans recharger, indicateur animé/réussite, puis `SettingsUpdate Theme`. L'enregistrement se fait dans `settings_local`. — app.js:37996-38015, 3029-3085, Actions.php:4966-4975.

F-ST-171 : Validation du thème — `ValidateTheme` : thème inconnu ⇒ retour à `themes.default` (section absente de la configuration, donc `Default` codé en dur) et non à `webmail.theme`. En version mobile, toujours `Mobile`. — Actions.php:9035-9044.

F-ST-172 : Image de fond personnalisée — section « Configuration du thème personnel » visible si capa USER_BACKGROUND (`webmail.allow_user_background`, défaut faux, non mobile). Bouton « Envoyer une image de fond (JPG, PNG) » (une seule image à la fois, glisser-déposer désactivé). — `T/User/SettingsThemes.html:20-43`, app.js:38000, 38064-38076, Application.php:163.

F-ST-173 : Contrôles de l'envoi — côté serveur : type MIME déduit de l'extension du nom (`image/png`, `image/jpg`, `image/jpeg` uniquement) sinon `UploadError::FILE_TYPE` (« Type de fichier invalide (JPG et PNG seulement) ») ; limite de taille = limites PHP (`upload_max_filesize`/`post_max_size` ; l'option `attachment_size_limit` ne s'y applique pas) ⇒ « Fichier trop volumineux » ; autres erreurs : message générique « Une erreur inconnue de téléversement de fichier s'est produite ». Le fichier est stocké en base64 dans le stockage CONFIG `background` avec un hash aléatoire. — Actions.php:7743-7830, 7616-7660, app.js:38077-38118.

F-ST-174 : Affichage du fond — le hash modifie le fond `#rl-bg` via `backstretch` (fondu 1 s, centré) ; vider le hash détruit l'effet. Le fond n'est pas appliqué en mobile. — app.js:38016-38030, Actions.php:1981-1990.

F-ST-175 : Suppression du fond — bouton croix à côté du nom du fichier (visible si un nom est présent) : `ClearUserBackground` supprime le stockage `background` et vide `UserBackgroundName/Hash` ; exige la capa USER_BACKGROUND. — `T/User/SettingsThemes.html:28-34`, app.js:38052-38061, Actions.php:7715-7741.

F-ST-176 : Libellés inutilisés — `LABEL_CUSTOM_TYPE`, `LABEL_CUSTOM_TYPE_LIGHT/DARK`, `LABEL_CUSTOM_BACKGROUND_IMAGE` existent dans `fr_FR.yml` sans usage dans le gabarit ; il n'y a pas de personnalisation de couleurs. — fr_FR.yml:508-518.

### B.14 OpenPGP (réglages)

F-ST-177 : Disponibilité — capa OPEN_PGP (`security.openpgp`, défaut faux, non mobile) ET chargement réussi de la bibliothèque (nécessite `crypto.getRandomValues`). — app.js:36649-36652, 29525-29557, Actions.php:8150-8153.

F-ST-178 : Boutons — « Importer la clé OpenPGP » et « Générer les clés OpenPGP » (désactivé hors HTTPS, infobulle « HTTPS seulement ») ; liste des clés privées (icône cadenas) puis publiques (icône clé, avec identifiant) ; chaque clé : voir (œil), supprimer (corbeille + confirmation) ; suppression : retire de l'anneau de clés local (`localStorage` navigateur), pas du serveur. — `T/User/SettingsOpenPGP.html`, app.js:38176-38214.

F-ST-179 : « Brouillon sauvegardé automatiquement » (`AllowDraftAutosave`) — case ; enregistrée immédiatement ; voir F-ST-53. — `T/User/SettingsOpenPGP.html`, app.js:38216-38222.

F-ST-180 : Stockage des clés — dans le navigateur uniquement (non synchronisé entre appareils) ; les messages chiffrés ne sont pas lisibles ailleurs. — app.js:38199-38211.

### B.15 Popups transverses

F-ST-181 : Popup Langues — liste de drapeaux : langue de l'utilisateur détectée mise en évidence (`user`), langue courante cochée, infobulle en anglais (`LANGS_NAMES_EN`) ; un clic applique et ferme. Utilisée par Général et par l'écran de connexion. Le nom vient de `LANGS_NAMES/LANG_<CODE>` (repli sur le code). — `T/Common/PopupsLanguages.html`, app.js:22405-22470, 2446-2452.

F-ST-182 : Langues disponibles — fichiers du dossier `app/localization/webmail/` (39 langues, dont `fr_FR`, `en_GB`, `en_US`, `de_DE`, `es_ES`, `it_IT`, `nl_NL`, `pt_BR`, `pt_PT`, `ru_RU`, `zh_CN`…), listés par `GetLanguages`. — dossier `app/localization/webmail/`, Actions.php:9219+.

F-ST-183 : Popup « Ask » — boîte de confirmation générique : texte HTML, boutons Oui/Non (libellés personnalisables), focus par défaut sur « Oui » (ou « Non » si demandé), Échap désactivé. — `T/Common/PopupsAsk.html`, app.js:20391-20480.

F-ST-184 : Messages « Ask » connus — « Êtes-vous sûr de vouloir fermer cette fenêtre ? », « Êtes-vous sûr de vouloir supprimer ce(s) message(s) ? », et dans la composition : `COMPOSE/DISCARD_UNSAVED_DATA`. — fr_FR.yml:244-248, app.js:13115.

F-ST-185 : Hooks plugins — `runSettingsViewModelHooks`, `user-login-submit`, `rl-start-login-screens`, `rl-start-user-screens`, `rl-start-screens` permettent d'ajouter des entrées ou de modifier les comportements ; `plugins.enable` (défaut faux) active le support. — app.js:8475-8560, Application.php:289-292.

F-ST-186 : Raccourcis clavier (popup d'aide) — quatre onglets navigables par Tab / Maj+Tab / flèches : Boîte aux lettres (` , R répondre, A répondre à tous, F transférer, Maj+F transférer en pièce jointe, ? aide), Liste des messages (Ctrl+A tout sélectionner, Z archiver, Suppr / Maj+Suppr / # supprimer, T ouvrir, M déplacer, Q lu, U non lu, I important, / recherche, Entrée plein écran / voir, → ou Tab message, ← ou Maj+Tab dossiers), Message (Entrée plein écran, B citations, Ctrl+P imprimer, Échap quitter / fermer / retour), Écrire (W ou C ouvrir, Ctrl+S enregistrer, Ctrl+Entrée envoyer — option `labs.allow_ctrl_enter_on_compose`, Échap réduire, Maj+Échap fermer). — `T/Common/PopupsKeyboardShortcutsHelp.html`, app.js:30546-30570, fr_FR.yml:587-626.

F-ST-187 : Anneau d'attente de focus — la popup d'aide gère son propre scope de touches (`PopupKeyboardShortcutsHelp`). — app.js:30540-30541.

F-ST-188 : Popup « Page d'accueil » — iframe plein contenu (600 × 450) affichant `branding.welcome_page_url`, bouton « Fermer » ; `branding.welcome_page_display` : `none` (jamais), `once` (une fois, mémorisé par `LastWelcomePage`), `always` ; uniquement pour le compte non additionnel ; à la fermeture `WelcomeClose` enregistre l'URL vue. — `T/Common/PopupsWelcomePage.html`, app.js:40066-40107, Actions.php:5066-5084, 1630-1650.

F-ST-189 : Titre des fenêtres — `TITLES/LOADING` (« Chargement »), `LOGIN`, `MAILBOX`, `SETTINGS` (« Paramètres »), `COMPOSE` ; le titre du navigateur combine `webmail.title` (défaut « RainLoop Webmail ») ; `webmail.loading_description`, `webmail.favicon_url`, `labs.favicon_status` (défaut vrai : compteur de non lus dans le favicon). — app.js:23354-23363, fr_FR.yml:407-412, Actions.php:1512-1520.

### B.16 Options de configuration qui pilotent les écrans (`application.ini`)

F-ST-190 : Section `webmail` (Application.php:155-178) — `title` (« RainLoop Webmail »), `loading_description` (« RainLoop »), `favicon_url`, `theme` (« Default »), `allow_themes` (vrai), `allow_user_background` (faux), `language` (« en »), `language_admin` (« en »), `allow_languages_on_settings` (vrai), `allow_additional_accounts` (vrai), `allow_additional_identities` (vrai), `messages_per_page` (20), `attachment_size_limit` (25 Mo, 0 = illimité).

F-ST-191 : Section `interface` (l.179-184) — `show_attachment_thumbnail` (vrai ; capa ATTACHMENT_THUMBNAILS), `use_native_scrollbars` (faux), `new_move_to_folder_button` (vrai).

F-ST-192 : Section `branding` (l.185-200) — `login_logo`, `login_background`, `login_desc`, `login_css`, `user_css`, `user_logo`, `user_logo_title`, `user_logo_message`, `user_iframe_message`, `welcome_page_url`, `welcome_page_display` (« none »).

F-ST-193 : Section `contacts` (l.201-211) — `enable` (faux), `allow_sync` (faux), `sync_interval` (20), `type` (sqlite), `pdo_dsn`, `pdo_user`, `pdo_password`, `suggestions_limit` (30).

F-ST-194 : Section `security` (l.212-233) — `csrf_protection` (vrai), `custom_server_signature`, `x_frame_options_header`, `x_xss_protection_header` (« 1; mode=block »), `openpgp` (faux), `admin_login`, `admin_password` (« 12345 » : valeur d'installation, indiquée comme `WeakPassword`), `allow_admin_panel` (vrai), `allow_two_factor_auth` (faux), `force_two_factor_auth` (faux), `hide_x_mailer_header`, `admin_panel_host`, `admin_panel_key` (« admin »), `content_security_policy`, `core_install_access_domain`.

F-ST-195 : Section `capa` (l.242-259) — `folders`, `composer`, `contacts`, `settings`, `quota`, `help`, `reload`, `search`, `search_adv`, `filters`, `dangerous_actions`, `message_actions`, `messagelist_actions`, `attachments_actions` (tous vrais), `x-templates` (faux) ; hiérarchie : sans `folders`, plus de MESSAGELIST_ACTIONS ni DANGEROUS_ACTIONS ; sans `composer`, plus de CONTACTS ; sans `search`, plus de SEARCH_ADV. Les capas HELP/TEMPLATES/THEMES/USER_BACKGROUND/OPEN_PGP/PREFETCH sont désactivées en mobile. Note : `attachments_actions` est vrai dans le fichier mais `Capa()` lit faux par défaut si la clé manque.

F-ST-196 : Section `login` (l.260-288) — `default_domain`, `allow_languages_on_login` (vrai), `determine_user_language` (vrai), `determine_user_domain` (faux), `welcome_page` (faux), `hide_submit_button` (faux), `forgot_password_link_url`, `registration_link_url`, `login_lowercase` (vrai), `sign_me_auto` (« DefaultOff » ; valeurs DefaultOff, DefaultOn, Unused).

F-ST-197 : Section `defaults` (l.294-305) — `view_editor_type` (« Html »), `view_layout` (1), `view_use_checkboxes` (vrai), `autologout` (30), `show_images` (faux), `contacts_autosave` (vrai), `mail_use_threads` (faux), `allow_draft_autosave` (vrai), `mail_reply_same_folder` (faux). Ces valeurs sont les valeurs initiales pour les nouveaux utilisateurs ; elles sont écrasées par les réglages utilisateur.

F-ST-198 : Section `social` (l.359-380) — `google_enable`, `google_enable_auth`, `google_enable_auth_gmail`, `google_enable_drive`, `google_enable_preview`, `google_client_id`, `google_client_secret`, `google_api_key`, `fb_enable`, `fb_app_id`, `fb_app_secret`, `twitter_enable`, `twitter_consumer_key`, `twitter_consumer_secret`, `dropbox_enable`, `dropbox_api_key` (tous faux/vides par défaut). Effets : boutons de connexion (F-ST-29), écran Social (F-ST-123), Google Drive / aperçu / Dropbox dans les actions de pièces jointes.

F-ST-199 : Section `labs` : IMAP/listes (l.398-460) — `use_imap_sort`, `use_imap_thread`, `use_imap_list_subscribe`, `imap_folder_list_limit` (200), `folders_spec_limit` (50), `ignore_folders_subscription`, `autocreate_system_folders` (vrai), `imap_forwarded_flag` (« $Forwarded »), `imap_read_receipt_flag` (« $ReadReceipt »), `imap_body_text_limit` (555000), `imap_large_thread_limit` (50), `imap_show_login_alert` (vrai), `imap_timeout` (300), `smtp_timeout` (60), `sieve_timeout` (10), `check_new_messages` (vrai).

F-ST-200 : Section `labs` : authentification (l.398-486) — `login_fault_delay` (1), `check_new_password_strength` (vrai, non utilisé par le contrôle), `allow_external_login`, `allow_external_sso`, `external_sso_key`, `custom_login_link`, `custom_logout_link`, `startup_url`, `dev_email`, `dev_password`, `allow_cmd` (faux), `cookie_default_path`, `cookie_default_secure`, `force_https`.

F-ST-201 : Section `labs` : interface (l.398-486) — `allow_gravatar` (vrai), `allow_prefetch` (vrai), `allow_html_editor_source_button`, `allow_html_editor_biti_buttons`, `allow_ctrl_enter_on_compose` (vrai), `favicon_status` (vrai), `use_app_debug_js/css`, `use_mobile_version_for_tablets`, `allow_mobile_version`, `try_to_detect_hidden_images`, `hide_dangerous_actions`, `strict_html_parser`, `detect_image_exif_orientation`, `use_local_proxy_for_external_images`, `in_iframe`.

F-ST-202 : Section `labs` : sieve et divers — `sieve_allow_raw_script`, `sieve_utf8_folder_name` (vrai), `sieve_auth_plain_initial`, `sieve_allow_fileinto_inbox`, `owncloud_save_folder` (« Attachments »), `owncloud_suggestions`, `curl_proxy`, `domain_list_limit`, `replace_env_in_configuration`.

F-ST-203 : Section `logs` (l.306-354) — activation et formats ; `auth_logging` (faux), format `[{date}] Auth failed: ip=… user=… host=… port=…` utilisé pour fail2ban. Pas d'écran utilisateur associé.

F-ST-204 : Section `cache` (l.381-397) — `enable`, `index`, `fast_cache_driver` (files), `http` (vrai), `http_expires` (3600 s), `server_uids`. Pas d'écran associé.

F-ST-205 : Section `ssl`, `plugins`, `debug` — `verify_certificate` (faux), `allow_self_signed` (vrai), `cafile`, `capath`, `client_cert` ; `plugins.enable`, `enabled_list` ; `debug.enable`. Le mot de passe « sleep@sleep.dev » en mode debug sert à tester les délais.

### B.17 Libellés, notifications et erreurs

F-ST-206 : Notifications d'authentification et de compte — `INVALID_TOKEN`, `AUTH_ERROR`, `ACCESS_ERROR`, `CONNECTION_ERROR` (« Serveur injoignable »), `CAPTCHA_ERROR`, `SOCIAL_*_LOGIN_ACCESS_DISABLE`, `DOMAIN_NOT_ALLOWED`, `ACCOUNT_NOT_ALLOWED`, `ACCOUNT_TWO_FACTOR_AUTH_REQUIRED/ERROR`, `ACCOUNT_ALREADY_EXISTS`, `ACCOUNT_DOES_NOT_EXIST`, `MAIL_SERVER_ERROR`, `INVALID_INPUT_ARGUMENT`, `UNKNOWN_ERROR`. — fr_FR.yml:640-700, Notifications.php.

F-ST-207 : Notifications de mot de passe — `COULD_NOT_SAVE_NEW_PASSWORD`, `CURRENT_PASSWORD_INCORRECT`, `NEW_PASSWORD_SHORT`, `NEW_PASSWORD_WEAK`, `NEW_PASSWORD_FORBIDDENT` (« caractères invalides », non utilisée par le contrôle fourni). — fr_FR.yml:665-669.

F-ST-208 : Notifications de réglages — `CANT_SAVE_SETTINGS` (501) existe mais l'écran Général n'affiche pas d'erreur d'enregistrement par option (seul l'indicateur d'échec des listes) ; les réponses des cases ne sont pas contrôlées. — fr_FR.yml:688, app.js:36861-36894.

F-ST-209 : Notifications de filtres et dossiers — `CANT_SAVE_FILTERS`, `CANT_GET_FILTERS`, `FILTERS_ARE_NOT_CORRECT`, `CANT_CREATE_FOLDER`, `CANT_RENAME_FOLDER`, `CANT_DELETE_FOLDER`, `CANT_DELETE_NON_EMPTY_FOLDER`, `CANT_SUBSCRIBE_FOLDER`, `CANT_UNSUBSCRIBE_FOLDER`, `CONTACTS_SYNC_ERROR`. — fr_FR.yml:671-687.

F-ST-210 : Libellés d'envoi de fichiers (UPLOAD) — « Le fichier est trop volumineux », « envoyé partiellement », « Aucun fichier téléversé », « Le fichier temporaire est manquant », « Une erreur inconnue s'est produite », « Type de fichier invalide », « Une erreur inconnue de téléversement… ». — fr_FR.yml:413-420.

F-ST-211 : Éditeur (EDITOR) — bascule HTML ↔ non formaté (`TEXT_SWITCHER_PLAINT_TEXT`), « Format riche », confirmation : « Le formatage du texte et les images seront perdus. Êtes-vous sûr de vouloir continuer ? ». — fr_FR.yml:421-424.

F-ST-212 : Pages statiques (STATIC) — pages d'erreur « JavaScript nécessaire », « cookies nécessaires », « navigateur obsolète », « extensions PHP manquantes », « version PHP inférieure à 5.3.0 », lien « Recharger ». — fr_FR.yml:701-717, ServiceActions.php:861-890.

F-ST-213 : Notifications PGP (PGP_NOTIFICATIONS) — clés publiques/privées introuvables (avec e-mail), signature valide/non vérifiée, erreur de déchiffrement, « Merci de spécifier une adresse email pour DE », « … au moins un destinataire ». — fr_FR.yml:627-639.

---------------------------------------------------------------------

## C. Constats et cas limites à surveiller (sans implémentation)

1. Compteurs de non lus par compte : fonction PHP prête mais interface désactivée (F-ST-09).
2. Modèles : aucun point d'insertion dans la rédaction, ordre non persisté, libellés absents des langues, option `x-templates` désactivée par défaut (F-ST-133 à F-ST-139).
3. Écran Sécurité : le lien 2FA dépend de la capa AUTOLOGOUT (F-ST-105).
4. Mot de passe : l'écran n'existe qu'avec un plugin de changement de mot de passe ; la vérification de robustesse utilise une recherche de sous-chaîne dans une liste, donc des mots de passe très courts ou communs sont refusés même s'ils sont longs mais contenus dans la liste (F-ST-126, F-ST-129).
5. Purge de dossier : message d'erreur calculé mais non affiché (F-ST-156) ; pas de contrôle de capa côté serveur.
6. `DoIdentityUpdate` sans contrôle de capa IDENTITIES (filtrage tardif par `SetIdentities`) (F-ST-78) ; ordre des identités après glisser-déposer incertain (F-ST-75).
7. `AccountsCounts` : seuil de 7 comptes appliqué de façon inversée (découpe quand le nombre est inférieur à la limite), sans impact car non appelé (F-ST-09).
8. Valeurs de `messages_per_page` : le serveur accepte 150/200/300 que l'interface ne propose pas (F-ST-42).
9. `ValidateTheme` se rabat sur `themes.default` (section inexistante), donc `Default` en dur, jamais sur `webmail.theme` (F-ST-171).
10. Clés de langue manquantes en `fr_FR` : `LOGIN/TITLE_SIGN_IN_GMAIL`, `SETTINGS_TEMPLATES/*`, `POPUPS_ADD_TEMPLATE/*`, `SETTINGS_TEMPLETS/LOADING_PROCESS` (F-ST-139).
11. Pas de « mot de passe applicatif » (mot de passe d'application) dans le code lu : la seule notion proche est le jeton d'authentification XXTEA contenant le mot de passe IMAP (F-ST-21, F-ST-71) et le mot de passe Google OAuth (jetons `APP_GOOGLE_ACCESS_TOKEN_PREFIX`) stocké à la place du mot de passe quand la connexion est faite par Gmail.
12. Les mots de passe des comptes additionnels sont conservés (chiffrés par XXTEA avec `APP_SALT`) dans le stockage serveur ; ils sont rejoués à chaque changement de compte (F-ST-67, F-ST-71).
