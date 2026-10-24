# Analyse fonction par fonction de RainLoop 1.17 (partie « divers ») contre wmail-go

Périmètre : `RainLoop/` hors `Actions.php` et hors `Providers/AddressBook*` : ServiceActions, Service, Api, Utils, Notifications, KeyPathHelper, Settings, Social, Model/*, Config/*, Enumerations, Exceptions, Common/*, Plugins/*, Providers (Domain, Files, Storage, Version, Filters, Suggestions, TwoFactorAuth, ChangePassword, Settings, AbstractProvider).
Sources lues en lecture seule (aucun code ni texte repris). Aucun dossier `data/` lu. Les numéros de ligne PHP sont ceux du fichier source ; les références Go sont `fichier:ligne` dans `/home/gdegoulet/Work/webmail_project/internal/` (abrégé `web/…`, `mailconn/…`, etc.).
Verdicts : OK, PARTIEL, MANQUANT, SANS OBJET. Priorités : P1 (visible chaque jour), P2, P3. Analyse complète du périmètre, aucun arrêt anticipé.

Constats transversaux (utiles pour lire les tableaux) :
- wmail-go est un serveur de pages HTML (pas d'API AJAX `Do*`) : chaque route est explicite (`web/server.go:111-202`) ; la couche `Service*` de RainLoop (aiguillage, cache LESS, modèles JS, langues JS) n'a donc pas d'équivalent à reproduire.
- Pas de notion de « domaine » administrateur : les paramètres IMAP/SMTP/Sieve sont saisis par boîte mail (`userdata/userdata.go:104-160`, `web/accounts.go`).
- Les secrets de session sont des identifiants aléatoires côté serveur (`session/session.go`), le fichier utilisateur est chiffré par XChaCha20-Poly1305 avec clé dérivée Argon2id (`vault/vault.go`) : tout le jeu de fonctions de chiffrement « XXTEA / RSA / jetons sérialisés » de RainLoop n'a pas lieu d'exister.

---

## 1. ServiceActions.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (33), `Logger` (44), `Plugins` (52), `Config` (60), `Cacher` (68), `StorageProvider` (76), `SettingsProvider` (84), `SetPaths` (94), `SetQuery` (105) | Injection de dépendances et état de la requête (chemin découpé, chaîne de requête) | Infrastructure interne PHP | `web.Server` porte config/store/log (`web/server.go`) ; routage par `http.ServeMux` (`web/server.go:111-202`) | SANS OBJET | Mécanique interne de PHP |
| `ServiceAjax` (114) | Point d'entrée AJAX unique : lit `Action`, contrôle le jeton CSRF des POST, masque les mots de passe dans le journal, appelle `Do<Action>`, renvoie JSON (+ code d'erreur, `Logout` forcé, jeton de mise à jour) | Protection CSRF, journal sans secret, déconnexion forcée si erreur d'authentification | CSRF : `web/server.go:283-288` (jeton de session, comparaison en temps constant) + origine `web/server.go:257` ; journal sans mot de passe (`web/handlers.go:76-103` ne journalise que le login réduit `store/store.go:156`) ; session expirée → redirection `/login` (`web/server.go:271-274`) | OK | Pas d'API JSON : le modèle formulaire/redirection tient lieu. Rien à ajouter |
| `ServiceOwnCloudAuth` (255) | Connexion automatique depuis ownCloud | Intégration ownCloud | aucun | SANS OBJET | Intégration tierce |
| `ServiceAppend` (299) | Dépôt d'un message RFC822 (IMAP APPEND) depuis le navigateur, réponse `1`/`0` | Glisser un `.eml` dans la liste | `web/mailappend.go:18` (`POST /mail/append`, `web/server.go:147`), message marqué lu (`web/mailappend.go:57`) | OK | — |
| `privateUpload` (343) | Réception d'un fichier (champ `uploader`), contrôle de taille (config), codes d'erreur (taille, vide, ini), réponse JSON | Envoi de pièces jointes, import de contacts, image de fond avec limites | Limites par route : `web/server.go:703-718` (`postLimit`) ; pièces : `web/attachstore.go:57` (limites par fichier, message, session) ; erreurs traduites `i18n/locales/fr.json:258-268` ; **mais** `ReadTimeout`/`WriteTimeout` du serveur fixés à 30 s (`cmd/wmail-go/main.go:133-136`) alors que l'envoi de 25 Mo sur une liaison lente ou l'envoi SMTP (`web/compose.go:29`, 90 s) dépasse facilement 30 s : la connexion serait coupée | PARTIEL | **P2** : dans `compose.go` (attach, send, save), `mail.go` (attachment, attachments.zip, source) appeler `http.NewResponseController(w).SetReadDeadline/SetWriteDeadline` avec 5 minutes (ou supprimer WriteTimeout global et le gérer par route). Cas limite : conserver 10 s pour l'en-tête (anti-slowloris) |
| `ServiceUpload` (430) | Cible `Upload` de la composition (limite `attachment_size_limit`) | Pièces jointes à la composition | `web/compose.go` `composeAttach` (`web/server.go:156`), limite `cfg.Compose.MaxAttachmentMB` (`config/config.go:84`) | OK | Voir `privateUpload` pour les délais |
| `ServiceUploadContacts` (438) | Import de contacts, limite 5 Mo | Import vCard/CSV | `web/contactsweb.go` `contactsImport` (`web/server.go:191`), limite `maxContactsImport` (`web/server.go:713`) | OK | — |
| `ServiceUploadBackground` (446) | Envoi de l'image de fond personnelle, limite 1 Mo | Fond d'écran utilisateur | `web/background.go:23` (`POST /settings/background`), 2 Mo, 6000×6000 (décision 152) | OK | — |
| `ServiceProxyExternal` (454) | Proxy des images externes (jeton, PNG/JPEG/BMP/GIF seulement) pour que l'expéditeur ne voie pas l'IP | Confidentialité à la lecture | `web/remoteimages.go:66` (`/mail/image`, adresse signée `web/remoteimages.go:46`, garde SSRF, formats d'après les octets, 5 Mo) | OK | Plus strict que RainLoop (BMP non repris : voulu) |
| `ServiceRaw` (493) | Aiguillage `Raw<Action>` : téléchargement de pièces jointes, aperçu, source, zip, vignettes, export de contacts… avec en-tête CSP `script-src 'none'` | Téléchargements/aperçus sûrs | Routes dédiées : `/mail/attachment`, `/mail/attachments.zip`, `/mail/source`, `/contacts/export` (`web/server.go:142-151,192`) ; CSP `sandbox` (`web/mail.go:1138,1194`, `web/mailtools.go:43`) et `nosniff` (`web/mail.go:1124`) | OK | — |
| `ServiceLang` (568) | Fournit le dictionnaire de langue + la locale `moment` au navigateur, avec cache | Interface traduite | Dictionnaires côté serveur (`i18n/i18n.go`, `i18n/locales/fr.json`, `en.json`) ; choix de langue `web/server.go:543-551` ; **seulement fr et en** (RainLoop : ~30 langues) | OK | Suffisant pour un utilisateur francophone |
| `ServiceTemplates` (615) | Compile les gabarits Knockout en JS | Rendu de l'interface | Gabarits `html/template` côté serveur (`web/templates/*.html`) | SANS OBJET | Architecture différente |
| `ServicePlugins` (655) | Fournit le JS des plugins | Plugins | aucun | SANS OBJET | Plugins hors périmètre (décision 14.4) |
| `ServiceCss` (695) | Compile la feuille du thème (LESS : valeurs + `styles.less` + `template.less`), thèmes personnalisés `@custom` | Thèmes (DarkShine) | `GET /themes/{name}/{file}` (`web/server.go:116,422-448`) : thème embarqué ou `<data_dir>/themes/<nom>/theme.css` ; thème `rainloop` = couleurs DarkShine (`web/static/themes/rainloop/theme.css:1`) ; thèmes `dark`, `light` | OK | — |
| `ServiceSocialGoogle` (800), `ServiceSocialFacebook` (809), `ServiceSocialTwitter` (817) | Fenêtre de connexion OAuth sociale | Connexion par réseau social | aucun | SANS OBJET | Hors périmètre (décision 14.4) |
| `ServiceAppData` (825), `ServiceAdminAppData` (833) | Données d'initialisation JS (réglages, comptes, capacités) | Démarrage de l'interface | Page rendue côté serveur : `web/server.go` `render` (`Page`) | SANS OBJET | Architecture différente |
| `ServiceMobileVersion` (841), `ServiceDesktopVersion` (851) | Basculer entre version mobile et bureau (cookie) | Utilisation sur téléphone | Pas de bascule ; mise en page adaptative par media queries (`web/static/mail.css:209,301,332`) | PARTIEL | **P3** : vérifier à la main le rendu à 400 px de la liste, de la lecture et de la composition ; pas de bascule à ajouter |
| `ServiceNoScript` (861) | Page « JavaScript requis » | Message clair sans JS | Le site fonctionne sans JS (formulaires, `web/static/noscript.css`) | OK | — |
| `ServiceNoCookie` (869) | Page « cookies requis » | Message clair quand le navigateur refuse les cookies | aucun : sans cookie, la connexion redirige en boucle vers `/login` sans explication (`web/server.go:271-274`) | MANQUANT | **P3** : après un POST /login réussi, poser aussi un cookie témoin ; si `GET /` arrive sans cookie de session ni témoin, afficher « Activez les cookies » (clé i18n `login.nocookie`) |
| `ServiceBadBrowser` (877) | Page « navigateur trop ancien » | Compatibilité | aucun | SANS OBJET | Navigateurs modernes uniquement |
| `ServiceMailto` (894) | Lien `mailto:` : mémorise la demande si déjà connecté, redirige vers l'application | Ouvrir wmail comme gestionnaire de mailto | `GET /compose?mailto=` (`web/compose.go:328`, analyse RFC 6068 `web/compose.go:963-971`) + bouton `registerProtocolHandler` (`web/static/mailto.js`, décision 138). Sans session, la redirection vers `/login` perd la demande (`web/server.go:273`) | PARTIEL | **P3** : mémoriser `next` (chemin /compose?mailto=… borné) dans un cookie court avant la redirection vers `/login`, puis le rejouer après connexion ; RainLoop, lui, ignore la demande si on n'est pas connecté |
| `ServicePing` (915) | Test de vie « Pong » | Supervision | `GET /healthz` (`web/server.go:111`) | OK | — |
| `ServiceInfo` (927) | `phpinfo` pour l'administrateur | Diagnostic | aucun | SANS OBJET | PHP |
| `ServiceSso` (941) | Connexion unique par hachage à usage unique (10 s) | SSO pour intégrations | aucun | SANS OBJET | Intégration, aucun administrateur |
| `ServiceRemoteAutoLogin` (1024) | Connexion par variables `REMOTE_USER`/`REMOTE_PASSWORD` | Auth HTTP en amont | aucun | SANS OBJET | Sécurité : exposerait le mot de passe ; un seul utilisateur |
| `ServiceExternalLogin` (1059) | Connexion par appel externe (e-mail + mot de passe) | Intégration | aucun | SANS OBJET | Idem |
| `ServiceExternalSso` (1127) | Obtention d'un hachage SSO avec clé partagée | Intégration | aucun | SANS OBJET | Idem |
| `changeAction` (1167), `ServiceChange` (1198) | Bascule vers un compte supplémentaire par adresse | Comptes multiples | `POST /accounts/select` (`web/accounts.go:473`, `web/server.go:135`) | OK | RainLoop le fait par lien GET ; wmail-go par POST + CSRF (meilleur) |
| `ErrorTemplates` (1211), `localError` (1230) | Page d'erreur générique avec lien « retour » | Erreurs lisibles | `s.message(...)` (`web/server.go:259`, `web/handlers.go`) rendu par `base.html` | OK | — |
| `localAppData` (1242) | Prépare les données de démarrage, rétablit la session « se souvenir de moi » (jeton SignMe) | Rester connecté après fermeture du navigateur | Session en mémoire avec durée d'inactivité et durée maximale réglables par l'utilisateur (`web/handlers.go:392-446`, `config/config.go:93-96`, 60 min/480 min par défaut) ; pas de persistance après redémarrage du serveur : la clé de données dépend du mot de passe | PARTIEL | **P3** : limite structurelle (le fichier utilisateur est chiffré par le mot de passe, pas de jeton long). Option envisageable : durée maximale jusqu'à 30 jours (`absolute_hours_max`) ; ne pas stocker de clé sur disque |
| `compileTemplates` (1295) | Assemble les gabarits JS | Rendu | — | SANS OBJET | Architecture différente |
| `convertLanguageNameToMomentLanguageName` (1323) | Correspondance langue → locale `moment` | Dates localisées | Dates formatées côté serveur (`web/mail.go`, « il y a 12 min », « hier 09:30 », décision 14.2) | OK | — |
| `compileLanguage` (1338) | Compose le dictionnaire (sources anglaises + langue + plugins) | Traductions avec repli sur l'anglais | `i18n.Bundle` repli sur langue par défaut (`i18n/i18n.go:24` `New(def)`) | OK | — |
| `compileAppData` (1390) | Encapsule les données d'init en JS | Rendu | — | SANS OBJET | Architecture différente |

---

## 2. Service.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (25) | Démarrage : active le débogage, pose les en-têtes `Server`, `X-Frame-Options`, `X-XSS-Protection`, **force HTTPS** par redirection, puis traite la requête | En-têtes de sécurité, HTTPS obligatoire | En-têtes dans `secure()` : CSP, `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer` (`web/server.go:241-254`) ; pas d'en-tête `Server` ; cookie `Secure` selon `isHTTPS` (`web/server.go:355`) ; **aucune** redirection HTTP→HTTPS ni `Strict-Transport-Security` (grep : aucun) | PARTIEL | **P2** : dans `secure()`, si `s.isHTTPS(r)` ajouter `Strict-Transport-Security: max-age=15552000` (option `config.hsts`, défaut actif seulement derrière HTTPS) ; redirection HTTP→HTTPS laissée au proxy (documenter) |
| `RunResult` (68), `Handle` (77) | Singleton d'exécution | Infrastructure | `cmd/wmail-go/main.go` | SANS OBJET | PHP |
| `localHandle` (97) | Aiguillage : chemin `Service<X>`, accès au panneau d'administration par clé (`admin`), détection mobile, page d'index mise en cache, contrôle d'écriture du dossier `data` | Routage, anti-cache sur POST | `ServeMux` (`web/server.go:111-202`) ; `Cache-Control: no-store` sur pages sensibles (`web/handlers.go:196`, `web/mail.go:1051`…) ; absence d'admin voulue ; dossier de données vérifié par `store.Open` (`store/store.go:85-99`) | OK | — |
| `staticPath` (265) | URL des fichiers statiques versionnée | Cache navigateur par version | `GET /static/` via `http.FileServerFS` sur FS embarqué (`web/server.go:115`) ; **pas** d'en-tête `Cache-Control` ni de date de modification (FS embarqué sans `ModTime`) : le navigateur ne met pas en cache durablement ; pas de compression gzip (grep : aucun) | PARTIEL | **P3** : envelopper `/static/` pour ajouter `Cache-Control: public, max-age=3600` et un `ETag` (hachage du contenu calculé au démarrage) ; compression à laisser au proxy ou ajouter un middleware gzip pour `text/css`, `application/javascript` |
| `indexTemplateParameters` (277) | Paramètres de la page d'accueil : titre, **favicon**, icône tactile, manifeste, viewport (`width=950` ou mobile), CSP, langue, thème, Sentry | Onglet reconnaissable, installation en application | `<meta viewport>` (`web/templates/base.html:5`), `data-theme` (`web/templates/base.html:2`) ; **ni favicon, ni apple-touch-icon, ni manifest** (grep : aucun) ; l'onglet n'a donc pas d'icône | MANQUANT | **P3** (visible : onglet sans icône) : ajouter `web/static/favicon.png`/`.svg` + `<link rel="icon">` et `<link rel="apple-touch-icon">` dans `base.html`, route `/favicon.ico` ; manifeste facultatif |

---

## 3. Api.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (10), `RunResult` (17), `Handle` (26), `Actions` (46), `Config` (60), `Logger` (68), `Version` (191), `ExitOnEnd` (268) | Singletons et accès aux composants, version | Infrastructure | `web.New(cfg, st, logger)` (`cmd/wmail-go/main.go:126`) | SANS OBJET | PHP |
| `SetupDefaultMailSoConfig` (76) | Réglages de la bibliothèque IMAP : recherche simple rapide, limite de déclenchement de liste, filtre de date permanent, filtre permanent, tous les en-têtes, limite de gros fils (50), **délai IMAP (300 s)**, CA SSL (`cafile`/`capath`), cookies (`path`, `secure`), liste blanche d'attributs HTML (`strict_html_parser`), vérification de nouveaux messages | Réglages fins : (a) CA privée pour serveurs internes, (b) cookies sécurisés, (c) assainissement HTML strict, (d) délais | (a) Pas de CA personnalisée : vérification de certificat par le magasin système ou désactivée par boîte (`mailconn/client.go:48-52`, `VerifyCert`) ; (b) cookie `Secure` auto + `SameSite=Strict` (`web/server.go:354-357`) ; (c) assainissement par liste blanche `bluemonday` (`mailview/sanitize.go:68-88`) ; (d) délais `mailTimeout` 30 s (`web/mail.go:32`), connexion 10-20 s (`mailconn/guard.go:91`, `mailconn/pool.go:13`) | PARTIEL | **P3** : ajouter à `userdata.Server` un champ `ca_pem` (certificat racine privé, chiffré avec le compte) utilisé comme `RootCAs` dans `tlsConfig()` (`mailconn/client.go:48`) pour vérifier un serveur à CA interne sans désactiver la vérification ; le reste est couvert |
| `GetUserSsoHash` (204), `ClearUserSsoHash` (222) | Création/suppression d'un jeton SSO | SSO | aucun | SANS OBJET | Voir `ServiceSso` |
| `ClearUserData` (232) | Efface le stockage et le carnet d'un utilisateur | Droit à l'effacement d'un compte | `store.Delete` (`store/store.go:573`) supprime fichier utilisateur et blobs (contacts, fond) ; accessible par `wmail-go user delete` (`cmd/wmail-go/cli.go:181`) ; pas de bouton « supprimer mon compte » dans l'interface (voulu : action d'administration) | OK | — |
| `LogoutCurrentLogginedUser` (259) | Efface le cookie de session | Déconnexion | `POST /logout` (`web/handlers.go:107`, `web/server.go:127`), cookie vidé (`web/server.go:359`) | OK | — |

---

## 4. Utils.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (24) | Classe statique | — | — | SANS OBJET | PHP |
| `PgpVerifyFile` (34) | Vérifie une signature PGP de paquet (mises à jour) | Mises à jour du cœur | aucun | SANS OBJET | Mises à jour/administrateur |
| `RsaPrivateKey` (49), `EncryptStringRSA` (68), `DecryptStringRSA` (111) | Chiffrement RSA des jetons (désactivé par défaut) | Protéger les jetons | Remplacés par le coffre XChaCha20 (`vault/vault.go`, `SealBlob` `vault/vault.go:437`) | SANS OBJET | Mieux couvert |
| `EncryptString` (149), `DecryptString` (160), `EncryptStringQ` (171), `DecryptStringQ` (189), `EncodeKeyValues` (207), `DecodeKeyValues` (219), `EncodeKeyValuesQ` (234), `DecodeKeyValuesQ` (247) | Chiffrement XXTEA + sérialisation PHP des jetons d'authentification, d'accès aux images externes, d'URL de pièces jointes | Jetons opaques côté client | Session serveur à identifiant aléatoire (`session/session.go:57`), adresses d'images signées par HMAC (`web/remoteimages.go:46`), jeton de confiance 2FA signé (`web/twofactor.go:102`) | OK | Jeton non sérialisé (pas de `unserialize` : meilleure sécurité) |
| `GetConnectionToken` (259) | Jeton de connexion (cookie `rltoken`, 30 jours) : base du CSRF et du proxy d'images | CSRF | Jeton CSRF lié à la session (`session/session.go:67`, `web/server.go:283`) | OK | — |
| `Fingerprint` (276) | Empreinte du navigateur (User-Agent) liée au jeton d'authentification | Détection de vol de jeton | aucun (cookie HttpOnly + SameSite=Strict, `web/server.go:354`) | SANS OBJET | Empreinte UA triviale à imiter ; protection jugée suffisante |
| `GetShortToken` (284) | Jeton de session navigateur (cookie `rlsession`) | Lier le chiffrement au navigateur | `wmail_session` (`web/server.go:38`) | OK | — |
| `UpdateConnectionToken` (301) | Prolonge de 30 jours le cookie de connexion | Durée de vie | Expiration glissante côté serveur (`session/session.go:74-98`) | OK | — |
| `GetCsrfToken` (315) | Jeton CSRF dérivé | CSRF | `sess.CSRF` + champ `csrf` (`web/server.go:283-288`) | OK | — |
| `PathMD5` (325) | Empreinte d'un dossier (invalidation de cache) | Cache | — | SANS OBJET | PHP |
| `ReadAndAddLang` (348) | Lit un fichier de langue YAML/INI | Traductions | `i18n.New` lit les JSON embarqués (`i18n/i18n.go:24`) | OK | — |
| `FolderFiles` (400) | Liste les fichiers d'un dossier | Infrastructure | — | SANS OBJET | PHP |
| `ClearHtmlOutput` (430) | Compresse le HTML de la page | Poids de page | aucun (gabarits non compactés, pas de gzip) | SANS OBJET | Gain négligeable ; voir compression en `Service.php` |
| `FastCheck` (446) | Contrôle de clé de licence RLxxx | Licence commerciale | aucun | SANS OBJET | Licence |
| `CompileTemplates` (465) | Rassemble les gabarits | Rendu | — | SANS OBJET | PHP |
| `GetCookie` (484), `SetCookie` (494), `ClearCookie` (516) | Gestion des cookies : chemin par défaut, drapeau `Secure` par défaut, `HttpOnly` | Cookies sûrs | `http.SetCookie` avec `HttpOnly`, `SameSite=Strict`, `Secure` si HTTPS (`web/server.go:354-362`), cookie de langue `SameSite=Lax` (`web/server.go:581`) ; chemin fixé à `/` : pas de déploiement sous un préfixe d'URL (toutes les routes sont absolues, grep `base_path` : aucun) | PARTIEL | **P3** : si wmail-go doit vivre sous `https://hote/webmail/`, ajouter `base_path` (config) appliqué aux liens des gabarits et au `Path` des cookies ; sinon documenter « à la racine d'un nom d'hôte » |
| `IsOwnCloud` (533), `IsOwnCloudLoggedIn` (541) | Détection de l'hébergement ownCloud | Intégration ownCloud | aucun | SANS OBJET | ownCloud |
| `UrlEncode` (552) | Encodage d'URL facultatif | Infrastructure | `net/url` | SANS OBJET | — |
| `WebPath` (560), `WebVersionPath` (586), `WebStaticPath` (594) | Chemins web de l'application et des statiques versionnés | URL des ressources | `/static/` fixe (`web/server.go:115`) | SANS OBJET | Voir le point cache de `staticPath` |
| `RemoveSuggestionDuplicates` (604) | Dédoublonne les suggestions d'adresses | Autocomplétion sans doublon | `GET /contacts/suggest` (`web/server.go:193`) ; unicité des adresses à l'import (décision 147) | OK | — |
| `CustomParseIniFile` (631) | Lecture d'un fichier INI | Config | `config.Load` (JSON, `config/config.go:172`) | SANS OBJET | Format différent |
| `CustomBaseConvert` (642) | Conversion de base numérique (clés de licence) | Licence | — | SANS OBJET | — |

---

## 5. Notifications.php

| Fonction / constantes (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Constantes d'authentification : `InvalidToken` (7), `AuthError` (8), `AccessError` (9), `ConnectionError` (10), `CaptchaError` (11), `DomainNotAllowed` (15), `AccountNotAllowed` (16) | Codes d'erreur de connexion : jeton CSRF, identifiants refusés, accès, serveur injoignable, domaine/compte interdit | Message adapté à la cause | CSRF `error.csrf` (`fr.json:62`), identifiants `login.error.invalid` (`fr.json:13`), connexion au serveur de la boîte : tests classés `accounts.test.fail.*` (`fr.json:112-118,229`) ; pas de captcha ni de domaine interdit (inscription limitée `register.error.closed`) | OK | Captcha et liste blanche de domaines : sans objet pour un utilisateur |
| `SocialFacebook/Twitter/GoogleLoginAccessDisable` (12-14) | Connexions sociales désactivées | Réseaux sociaux | aucun | SANS OBJET | Hors périmètre |
| `AccountTwoFactorAuthRequired` (18), `AccountTwoFactorAuthError` (19) | 2FA requise / code faux | Double authentification | `web/twofactor.go:141` (`needSecondFactor`), `login_2fa.html`, `twofactor.*` (`fr.json`) | OK | — |
| `CouldNotSaveNewPassword` (21), `CurrentPasswordIncorrect` (22), `NewPasswordShort` (23), `NewPasswordWeak` (24), `NewPasswordForbidden` (25) | Erreurs de changement de mot de passe | Changement de mot de passe | `settings.error.password` (`fr.json:56`), `error.password_short` (`fr.json:58`), `error.mismatch` ; pas de contrôle de « faiblesse » (liste de mots de passe courants) | PARTIEL | **P3** : ajouter dans `store.checkPassword` (`store/store.go:179`) un refus des mots de passe de la liste des 1000 plus courants, ou de ceux égaux au login |
| `ContactsSyncError` (27) | Erreur de synchronisation des contacts | Synchronisation CardDAV | `web/contactssync.go` affiche l'erreur de passage | OK | — |
| `CantGetMessageList` (29), `CantGetMessage` (30), `CantDeleteMessage` (31), `CantMoveMessage` (32), `CantCopyMessage` (33) | Échecs d'opérations sur les messages | Erreur explicite à la place d'un écran blanc | `mail.err.*` (`fr.json:175-193,226`), page « corriger ce compte » (`web/templates/base.html:200`) ; erreurs IMAP du serveur transmises comme `folders.err.server` | OK | — |
| `CantSaveMessage` (35), `CantSendMessage` (36), `InvalidRecipients` (37) | Échec d'enregistrement du brouillon / d'envoi / destinataires invalides | Erreur d'envoi précise | `compose.err.send`, `compose.err.rejected`, `compose.err.address` (`fr.json:261-265`) | OK | — |
| `CantSaveFilters` (39), `CantGetFilters` (40), `FiltersAreNotCorrect` (41) | Erreurs de filtres | Filtres Sieve | `filters.err.*` (`fr.json:442-456,486`) | OK | — |
| `CantCreateFolder` … `CantDeleteNonEmptyFolder` (43-48) | Erreurs de dossiers | Gestion des dossiers | `folders.err.*` (`fr.json:316-319`) ; refus par le serveur affiché | OK | — |
| `CantSaveSettings` (50), `CantSavePluginSettings` (51) | Échec d'enregistrement des réglages | Réglages | `settings.prefs.error` (`fr.json:55`), `s.serverError` | OK | — |
| `DomainAlreadyExists` (53) | Domaine existant (admin) | Administration | aucun | SANS OBJET | Pas de domaines |
| `CantInstallPackage` … `UnsupportedPluginPackage` (55-58), `Licensing*` (60-62), `Demo*` (64-65) | Paquets/plugins, licences, mode démo | Administration | aucun | SANS OBJET | Hors périmètre |
| `AccountAlreadyExists` (67), `AccountDoesNotExist` (68) | Compte supplémentaire déjà ajouté / inexistant | Comptes multiples | `register.error.exists` (`fr.json:19`) pour l'utilisateur ; pour les boîtes : seule la limite de nombre est contrôlée (`accounts.err.limit` `fr.json:129`), pas l'unicité adresse+serveur | PARTIEL | **P3** : contrôle d'unicité d'une boîte (même adresse + même serveur IMAP) à l'ajout, avec message `accounts.err.duplicate` |
| `MailServerError` (70), `ClientViewError` (71), `InvalidInputArgument` (72), `UnknownNotification` (73), `UnknownError` (74) | Erreurs génériques | Repli | `error.generic` (`fr.json:63`), `error.not_found`, `mail.err.bad_request` | OK | — |
| `GetNotificationsMessage` (84) | Traduit un code en message (avec l'exception précédente pour `ClientViewError`) | Messages explicites | clés i18n + `ErrorArg` (`web/server.go:521-531`) | OK | — |

---

## 6. KeyPathHelper.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `PublicFile` (12) | Clé de stockage de fichiers publics | Partage de fichiers | aucun | SANS OBJET | Fonction non utilisée en pratique |
| `SsoCacherKey` (22) | Clé du cache SSO | SSO | aucun | SANS OBJET | Voir SSO |
| `RsaCacherKey` (32) | Clé du cache RSA | Chiffrement des jetons | aucun | SANS OBJET | — |
| `LicensingDomainKeyValue` (42), `LicensingDomainKeyOtherValue` (52) | Clés de licence | Licence | aucun | SANS OBJET | — |
| `RepositoryCacheFile` (63), `RepositoryCacheCore` (73) | Cache du dépôt de plugins/cœur | Mises à jour | aucun | SANS OBJET | — |
| `ReadReceiptCache` (85) | Clé qui mémorise qu'un accusé de lecture a été envoyé pour un message | Ne pas reproposer l'accusé | Mot-clé IMAP `$MDNSent` posé sur le message (décision 139, `web/compose.go:1000` `mailMDN`, `web/server.go:150`) | OK | Méthode plus robuste (côté serveur IMAP) |
| `LangCache` (97), `TemplatesCache` (108), `PluginsJsCache` (118), `CssCache` (130) | Clés de cache des langues, gabarits, plugins, CSS | Performance | — | SANS OBJET | Pas de compilation à mettre en cache |
| `SessionAdminKey` (140) | Clé de session administrateur | Administration | aucun | SANS OBJET | — |

---

## 7. Settings.php (réglages utilisateur) et Providers/Settings*

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Settings::__construct` (20), `InitData` (31), `DataAsArray` (44), `IsLocal` (52), `GetConf` (63), `SetConf` (74) | Conteneur clé/valeur des préférences de l'utilisateur | Mémoriser langue, thème, présentation, etc. | `userdata.Settings` typé, chiffré avec le compte (`userdata/userdata.go:44-68`), enregistré par `store.SaveData` (`store/store.go:499`) | OK | — |
| `Providers\Settings::__construct` (6), `Load` (10), `Save` (16), `IsActive` (20) | Fournisseur de chargement/enregistrement des préférences | Persistance | `store.Login`/`SaveData` (`store/store.go:378,499`) | OK | — |
| `DefaultSettings::__construct` (8), `Load` (12), `Save` (31), `Delete` (40) | Réglages sous forme de JSON dans le stockage `settings` ou `settings_local` | Persistance, réglage local | JSON dans le coffre chiffré ; pas de distinction local/distant | OK | — |
| `ISettings::Load/Save/Delete` | Interface | — | — | SANS OBJET | PHP |
| `AbstractProvider::IsActive` (7), `SetAccount` (11), `SetLogger` (15), `Logger` (22) | Base des fournisseurs | Infrastructure | — | SANS OBJET | PHP |

---

## 8. Model/Account.php (compte de messagerie)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (63), `NewInstance` (88) | Crée un compte : e-mail (IDN→ASCII), identifiant, mot de passe, domaine, jeton « se souvenir », authentification proxy, certificat client | Adresses internationales (IDN), identifiants distincts | `userdata.MailAccount` (`userdata/userdata.go:146-160`) ; identifiant IMAP libre (`web/accounts.go:202`) ; **pas de conversion IDN/punycode** (grep `idna` : aucun dans le code) ; pas de certificat client ni d'authentification proxy | PARTIEL | **P3** : convertir le domaine des adresses saisies en ASCII via `golang.org/x/net/idna` (`x/net` déjà dans `go.mod`) pour hôtes et adresses d'expéditeur ; certificat client TLS et proxy d'auth : SANS OBJET |
| `GenerateTokensPassword` (97), `ParseTokensPassword` (105) | Encode/décode un couple jeton d'accès/de rafraîchissement OAuth Google dans le champ « mot de passe » | Connexion Gmail par OAuth | aucun | SANS OBJET | OAuth/Google hors périmètre |
| `Email` (126), `ParentEmail` (134), `ParentEmailHelper` (158), `IsAdditionalAccount` (166), `SetParentEmail` (279) | Adresse du compte et lien vers le compte principal (comptes supplémentaires) | Comptes multiples liés à un principal | Les boîtes sont des enfants du compte wmail (`userdata.MailAccount`), pas des comptes de connexion : `web/accounts.go` | OK | — |
| `ProxyAuthUser` (142), `ProxyAuthPassword` (150), `SetProxyAuthUser` (289), `SetProxyAuthPassword` (299) | Authentification par utilisateur proxy IMAP (SASL « proxy ») | Se connecter comme un autre utilisateur (administration) | aucun | SANS OBJET | Cas d'administration |
| `IncLogin` (174), `OutLogin` (196), `Login` (210) | Identifiant IMAP/SMTP, éventuellement réduit à la partie locale de l'adresse (« short login ») | Serveurs qui attendent `user` et non `user@domaine` | Identifiants IMAP et SMTP saisis librement (`web/accounts.go:202-216`) : équivalent, sans règle automatique | OK | — |
| `IncPassword` (188), `Password` (218), `SetPassword` (269) | Mot de passe du compte | Authentification | `userdata.Server.Password`, chiffré dans le coffre ; SMTP vide = mot de passe IMAP (`web/accounts.go:208-216`) | OK | — |
| `ClientCert` (226) | Certificat client TLS | Authentification par certificat | aucun | SANS OBJET | Rare |
| `SignMe` (234), `SignMeToken` (242) | Indique si la connexion doit être retenue | Rester connecté | Voir `localAppData` | PARTIEL | **P3** : mêmes limites ; sessions plus longues réglables |
| `Domain` (250), `DomainIncHost` (307), `DomainIncPort` (315), `DomainIncSecure` (323), `DomainOutHost` (331), `DomainOutPort` (339), `DomainOutSecure` (347), `DomainOutAuth` (355), `DomainSieveHost` (363), `DomainSievePort` (371), `DomainSieveSecure` (379), `DomainSieveAllowRaw` (387) | Accès aux paramètres serveur du domaine de l'utilisateur | Paramètres de connexion IMAP/SMTP/Sieve ; **SMTP sans authentification** possible (`OutAuth` faux) ; édition du script brut autorisée ou non | IMAP/SMTP/Sieve par boîte (`userdata.Server`, `SieveConfig` `userdata/userdata.go:73-83,104-112`) ; script brut toujours permis (`web/filters.go:455`) ; **l'authentification SMTP ne peut pas être désactivée** : identifiant vide → repris de l'IMAP (`web/accounts.go:208-216`) | PARTIEL | **P3** : case « le serveur SMTP n'exige pas d'authentification » (`userdata.Server.NoAuth`) ; `LoginSMTP` (`mailconn/client.go:258-262`) saute déjà l'authentification quand l'identifiant est vide : il suffit de ne pas recopier celui de l'IMAP si la case est cochée |
| `Hash` (258) | Empreinte du compte (e-mail, serveur, mot de passe) | Détecter un changement de compte/mot de passe | `MailAccount.ID` aléatoire (`web/accounts.go:430-435`) | OK | — |
| `GetAuthToken` (395), `GetAuthTokenQ` (416) | Jeton d'authentification chiffré (cookie) contenant e-mail, mot de passe, empreinte, etc. | Rester authentifié | Session serveur (`session/session.go`), aucun mot de passe dans le cookie | OK | Plus sûr |
| `IncConnectAndLoginHelper` (442) | Connexion IMAP : mécanismes AUTH PLAIN/CRAM-MD5/XOAUTH2, vérification de certificat, auto-signé accepté, certificat client, proxy | Connexion robuste, vérification TLS configurable | `Guard.dialIMAP` (STARTTLS jamais en repli, `mailconn/client.go:91-125`), `LOGIN` (`mailconn/client.go:147,162`), vérification par boîte, TLS ancien réglable (`mailconn/client.go:36-66`) ; **pas** de CRAM-MD5 ni de XOAUTH2 (le `LOGIN` suffit sur Dovecot) | PARTIEL | **P3** : si le serveur annonce `LOGINDISABLED` ou `AUTH=PLAIN` seul, utiliser `AUTHENTICATE PLAIN` via `go-sasl` (déjà utilisé côté SMTP). CRAM-MD5 obsolète : ne pas faire |
| `OutConnectAndLoginHelper` (529) | Connexion SMTP : EHLO avec le nom d'hôte, PLAIN/CRAM-MD5/XOAUTH2, option `mail()` PHP | Envoi authentifié | `Guard.LoginSMTP` : PLAIN puis LOGIN (`mailconn/client.go:258-290`) ; EHLO envoie `localhost` (`mailconn/client.go:235,252`) au lieu du nom de la machine ; pas d'envoi par `mail()` (non retenu, décision 11.3) | PARTIEL | **P3** : envoyer EHLO avec un nom de domaine réel (`config.hostname`, sinon `os.Hostname()` si contient un point, sinon le domaine de l'adresse d'identité) : certains MTA (Postfix `reject_non_fqdn_helo_hostname`) refusent `localhost` |
| `SieveConnectAndLoginHelper` (604) | Connexion ManageSieve (auth PLAIN initial, TLS) | Filtres Sieve | `Guard.WithSieve` (`mailconn/sieve.go:16-58`), `Client.Authenticate` PLAIN (`sieve/client.go:273-290`), STARTTLS (`sieve/client.go:252`) | OK | — |

---

## 9. Model/Domain.php (domaine administrateur)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (119), `NewInstance` (169), `NewInstanceFromDomainConfigArray` (188), `Normalize` (241), `ToIniString` (267), `UpdateInstance` (347), `encodeIniString` (236) | Création/mise à jour/sérialisation INI d'un domaine (hôtes, ports, sécurité, Sieve, SMTP, liste blanche) ; ports par défaut 143/4190/25 | Administrateur configure le serveur de ses utilisateurs | Saisie par boîte : valeurs par défaut de ports côté formulaire (`web/accounts.go` `parsePort`, `SieveConfig.Port` 0 = 4190 `userdata/userdata.go:78`) | SANS OBJET | Pas d'administrateur ; couvert par le formulaire de boîte |
| `StrConnectionSecurityTypeToCons` (295), `ConstConnectionSecurityTypeToStr` (315) | Correspondance SSL/TLS/aucun ↔ constantes | Choix de sécurité | `SecurityTLS/STARTTLS/None` (`mailconn/client.go:22-32`) | OK | — |
| `Name` (378), `IncHost` (386), `IncPort` (394), `IncSecure` (402), `IncShortLogin` (410), `UseSieve` (418), `SieveHost` (426), `SievePort` (434), `SieveSecure` (442), `SieveAllowRaw` (450), `SetSieveAllowRaw` (458), `OutHost` (466), `OutPort` (474), `OutSecure` (482), `OutShortLogin` (490), `OutAuth` (498), `OutUsePhpMail` (506), `WhiteList` (514), `AliasName` (522), `SetAliasName` (531) | Lecteurs des propriétés du domaine | Paramètres de connexion | Champs de `userdata.Server`/`SieveConfig` (`userdata/userdata.go:73-112`) ; `OutAuth` : voir Account ; `OutUsePhpMail` : non retenu ; alias et liste blanche : administrateur | OK | `OutAuth` traité en P3 ci-dessus |
| `ValidateWhiteList` (544) | Restreint les comptes autorisés sur un domaine | Contrôle d'accès administrateur | aucun (inscription ouverte/fermée `allow_registration`, `max_users` `config/config.go:108-112`) | SANS OBJET | Un seul utilisateur |
| `ToSimpleJSON` (567) | Export JSON du domaine | Interface d'administration | — | SANS OBJET | Admin |

---

## 10. Model/Identity.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (48), `NewInstance` (62), `NewInstanceFromAccount` (72) | Identité vide ou issue du compte | Identité principale | `MailAccount.AllIdentities()` produit l'identité principale (`userdata/userdata.go:167-170`) | OK | — |
| `Id` (82), `Email` (90), `SetEmail` (100), `Name` (110), `ReplyTo` (118), `Bcc` (126), `Signature` (134), `SignatureInsertBefore` (142) | Propriétés d'une identité (adresse, nom, adresse de réponse, Cci, signature, « avant la citation ») | Identités d'expédition multiples | `userdata.Identity` (`userdata/userdata.go:127-135`) ; page `web/identities.go:28-162` | OK | — |
| `FromJSON` (153) | Lit une identité : adresse obligatoire, signature « avant » vrai par défaut | Création/modification | `identitiesSave` (`web/identities.go:62-120`) : adresse validée, doublons refusés, 20 au plus | OK | Différence mineure : la case « signature avant la citation » est décochée par défaut à la création (RainLoop : cochée) ; sans conséquence |
| `ToSimpleJSON` (177) | Export | Interface | — | SANS OBJET | — |
| `Validate` (193) | Une identité doit avoir une adresse | Validation | `identitiesSave` (`web/identities.go:72`) | OK | — |
| `IsAccountIdentities` (201) | Distingue l'identité principale des autres | Édition de l'identité principale | Principale éditée dans le formulaire de boîte (`web/accounts.go:243`) | OK | — |

---

## 11. Model/Template.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (34), `NewInstance` (49) | Crée un modèle (identifiant, nom, corps) | Modèles de message | `userdata.Template` (`userdata/userdata.go:27-31`) | OK | — |
| `Id` (57), `Name` (65), `Body` (73), `SetPopulateAlways` (81) | Propriétés ; le corps n'est pas renvoyé dans la liste (chargé à la demande) | Liste légère | `templatesPage` fournit un aperçu de 100 caractères (`web/templates_settings.go:34-44`), corps chargé à la demande `templatesBody` (`web/templates_settings.go:119`) | OK | — |
| `FromJSON` (92) | Lit un modèle (ID, nom, corps requis) | Enregistrement | `templatesSave` (`web/templates_settings.go:61-100`) : 50 au plus, 20 000 caractères | OK | — |
| `ToSimpleJSON` (111) | Export (corps masqué en AJAX) | Interface | — | SANS OBJET | — |
| `GenerateID` (142) | Identifiant aléatoire | Création | `rand.Read` 8 octets (`web/templates_settings.go:90-91`) | OK | — |
| `Validate` (150) | Corps non vide | Validation | `templatesSave` (`web/templates_settings.go:62-64`) | OK | — |

---

## 12. Config/AbstractConfig.php, Config/Application.php, Config/Plugin.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `AbstractConfig::__construct` (39), `defaultValues` (58), `IsInited` (63), `Get` (75), `Set` (92), `IsFileExists` (212) | Valeurs par défaut typées, lecture/écriture | Configuration | `config.Default()` (`config/config.go:136`) | OK | — |
| `cacheKey` (126), `loadDataFromCache` (134), `storeDataToCache` (167), `clearCache` (194) | Cache APC de la configuration | Performance | — | SANS OBJET | PHP |
| `Load` (220), `Save` (277) | Lecture/écriture du fichier INI (avec commentaires) | Fichier de configuration | `config.Load` (JSON, `config/config.go:172`) + `Validate` (`config/config.go:191`) ; pas d'écriture par l'interface (voulu : pas d'administrateur) | OK | — |
| `Application::__construct` (12), `Load` (28), `Get` (61), `Save` (140) | Fichier `application.ini`, substitution de variables d'environnement `$VAR` | Configuration par environnement | `config.json` seulement ; pas de remplacement d'environnement (`docker-compose.yaml` monte le fichier) | PARTIEL | **P3** : surcharge de `listen`, `data_dir`, `trusted_proxy` par variables `WMAIL_*` dans `config.Load` (pratique en conteneur) |
| `SetPassword` (100), `ValidatePassword` (114) | Mot de passe de l'administrateur | Administration | aucun | SANS OBJET | Pas d'administrateur |
| `defaultValues` (151) : section `webmail` (titre, favicon, thème par défaut, thèmes permis, fond utilisateur, langue par défaut, langue sur réglages, comptes/identités supplémentaires, messages par page, taille max de pièce) | Valeurs par défaut du produit | Réglages de base | `default_language`, `default_theme` (`config/config.go:121-122`), `PageSize` (`userdata/userdata.go:50`), `max_attachment_mb` (`config/config.go:84`), fond personnel (`web/background.go`) ; favicon : absent | PARTIEL | Favicon : voir `Service.php::indexTemplateParameters` (P3) |
| `defaultValues` : section `interface` (vignettes de pièces jointes, barres natives, bouton « déplacer vers » nouveau) | Détails d'interface | Vignettes affichées | Vignettes : `mailview/thumb.go:23` ; barres de défilement natives : défaut du navigateur ; menu « déplacer vers » présent (`web/static/menus.js`) | OK | — |
| `defaultValues` : section `branding` (logo, fond de connexion, CSS, message d'accueil…) | Personnalisation par l'administrateur | Habillage | Thème personnalisable par fichier (`web/server.go:422`) | SANS OBJET | Pas d'administrateur |
| `defaultValues` : section `contacts` (activation, synchro, intervalle, PDO, suggestions) | Carnet d'adresses | Contacts | `internal/contacts`, synchro CardDAV 5-320 min (`web/contactssync.go`), suggestions 8 réponses (`web/server.go:193`) | OK | RainLoop renvoie 30 suggestions : 8 suffisent |
| `defaultValues` : section `security` : `csrf_protection`, signature de serveur, `x_frame_options_header`, `x_xss_protection_header`, `openpgp`, `allow_two_factor_auth`, `force_two_factor_auth`, `hide_x_mailer_header`, CSP | Protections | Durcissement | CSRF `web/server.go:283` ; pas d'en-tête `Server` ; `X-Frame-Options: DENY` et CSP stricte (`web/server.go:241-254`) ; 2FA `web/twofactor.go` ; `hide_x_mailer` (`config/config.go:88`) ; OpenPGP hors périmètre ; **2FA obligatoire** : aucun | OK | 2FA obligatoire : SANS OBJET (un utilisateur) |
| `defaultValues` : `security.admin_*`, `allow_admin_panel`, `admin_panel_host/key`, `core_install_access_domain` | Panneau d'administration | Administration | aucun | SANS OBJET | — |
| `defaultValues` : section `ssl` (`verify_certificate`, `allow_self_signed`, `cafile`, `capath`, `client_cert`) | Vérification TLS globale | Serveurs à certificat interne | Vérification par boîte (`VerifyCert`), TLS ancien (`LegacyTLS`) ; pas de `cafile`/`capath` ni de certificat client | PARTIEL | **P3** : voir `ca_pem` par boîte (Api.php::SetupDefaultMailSoConfig) |
| `defaultValues` : section `capa` (interrupteurs de fonctionnalités) | Désactiver des fonctions pour des profils | Contrôle d'accès par fonction | aucun | SANS OBJET | Un seul utilisateur, tout actif |
| `defaultValues` : section `login` (domaine par défaut, langue sur la page de connexion, détection de langue/domaine, bouton, liens « mot de passe oublié »/inscription, minuscules, **se souvenir de moi**) | Page de connexion | Connexion pratique | Langue sur la connexion `GET /lang` (`web/server.go:119,579`), détection `Accept-Language` (`web/server.go:548`), mise en minuscules (`store/store.go:139`), liens récupération/inscription (`web/templates/login.html`) ; se souvenir de moi : voir `localAppData` | OK | — |
| `defaultValues` : section `plugins` | Plugins | — | aucun | SANS OBJET | — |
| `defaultValues` : section `defaults` : `view_editor_type` | Éditeur par défaut | Composition | `Settings.Editor` (`userdata/userdata.go:55`, décision 126) | OK | — |
| `defaultValues` : `view_layout` (0 sans aperçu, 1 aperçu à droite, **2 aperçu en bas**) | Disposition de la liste et de la lecture | Confort d'affichage | `three` (3 volets) et `list` seulement (`web/templates/settings.html:19-22`, `web/handlers.go:403-405`) ; aperçu en bas : aucun | PARTIEL | **P2** : ajouter `Layout="bottom"` : `settings.layout.bottom`, classe `layout-bottom` sur le conteneur de `home.html` (liste au-dessus, lecture dessous), règles dans `web/static/mail.css`, validation dans `settingsPrefs` (`web/handlers.go:403`) |
| `defaultValues` : `view_use_checkboxes` | Cases à cocher de la liste | Sélection multiple | `HideCheckboxes` (`userdata/userdata.go:61`) | OK | — |
| `defaultValues` : `autologout` (30 min) | Déconnexion après inactivité | Sécurité du poste | `SessionIdleMinutes` (`userdata/userdata.go:47`, `web/handlers.go:392`) | OK | — |
| `defaultValues` : `show_images` | Afficher les images distantes d'emblée | Confort/confidentialité | `BlockRemoteContent` par boîte (`userdata/userdata.go:139`), expéditeurs de confiance (`web/remoteimages.go:115`) | OK | — |
| `defaultValues` : `contacts_autosave`, `mail_use_threads`, `allow_draft_autosave`, `mail_reply_same_folder` | Préférences | Comportements courants | `NoContactsAutosave`, `Threads`, `NoDraftAutosave`, `ReplySameFolder` (`userdata/userdata.go:63,58,67,68`) | OK | — |
| `defaultValues` : section `logs` (activation, écriture sur erreur seulement, masquage des mots de passe, décalage horaire, filtre de session, nom de fichier à motifs, **journal d'authentification pour fail2ban**) | Journalisation et protection contre la force brute | Journal exploitable, bannissement d'IP | `slog` (`log_level`, `log_format` `config/config.go:123-124`), journal « échec de connexion » avec IP (`web/handlers.go:84`), login réduit (`store/store.go:156`), limitation de débit intégrée (`ratelimit/ratelimit.go`) | OK | Le journal est exploitable par fail2ban (ligne d'échec avec IP) |
| `defaultValues` : section `debug`, `social` | Débogage, réseaux sociaux/Dropbox | — | aucun | SANS OBJET | Hors périmètre |
| `defaultValues` : section `cache` (activation, index, pilote rapide, cache HTTP navigateur et durée, UID serveur) | Performance et cache navigateur | Chargement rapide | Pas de cache de fichiers ; cache navigateur des statiques absent (voir `staticPath`) | PARTIEL | **P3** : en-têtes `Cache-Control` pour `/static/` |
| `defaultValues` : `labs.allow_mobile_version`, `use_mobile_version_for_tablets` | Version mobile | Mobile | media queries `mail.css:209` | PARTIEL | Voir `ServiceMobileVersion` (P3) |
| `defaultValues` : `labs.ignore_folders_subscription` | Ignorer l'abonnement IMAP pour l'affichage | Voir tous les dossiers | Dossiers non abonnés masqués seulement si le serveur sait lister les abonnements et qu'au moins un est abonné (décision 124, `web/foldertree.go`) ; pas de réglage pour tout afficher | PARTIEL | **P3** : case « afficher aussi les dossiers non abonnés » dans `/settings/folders` (champ `ShowUnsubscribed` dans `Settings`) |
| `defaultValues` : `check_new_password_strength` | Contrôle de force des mots de passe | Mot de passe fort | longueur minimale `password_min_length` (`config/config.go:116`) ; pas de liste de mots courants | PARTIEL | **P3** (voir Notifications `NewPasswordWeak`) |
| `defaultValues` : `update_channel` | Mises à jour | Maintenance | aucun | SANS OBJET | — |
| `defaultValues` : `allow_gravatar` | Photos Gravatar des contacts | Confort | aucun (refus de principe, §14.1) | SANS OBJET | Envoie les adresses à un tiers |
| `defaultValues` : `allow_prefetch` | Précharge le message suivant | Lecture plus rapide | aucun | MANQUANT | **P3** : dans `list.js`/`listnav.js`, après un délai d'inactivité de 1 s sur un message, `fetch('/mail/body?...')` du suivant avec `Cache-Control: private, max-age=60` ; à ne faire que si le message n'est pas marqué lu automatiquement (la prélecture ne doit pas le marquer lu) |
| `defaultValues` : `allow_smart_html_links` | Transformer en liens les adresses en clair dans les messages HTML | Confort de lecture | Liens automatiques pour le texte brut seulement (`mailview/text.go:29,38`) ; pas dans le HTML (`mailview/sanitize.go`) | PARTIEL | **P3** : dans `sanitize.go`, après assainissement, parcourir les nœuds de texte hors `<a>` et envelopper `https?://…` en `<a href rel="noreferrer">` via `safeLink` (`mailview/sanitize.go:163`) |
| `defaultValues` : `cache_system_data`, `use_app_debug_js/css`, `fast_cache_*`, `owncloud_*`, `curl_proxy*`, `in_iframe`, `custom_login/logout_link`, `allow_external_login/sso`, `external_sso_key`, `replace_env_in_configuration`, `startup_url`, `allow_cmd`, `dev_*`, `http_client_ip_check_proxy` | Options d'infrastructure/intégration | Hébergement | `trusted_proxy` (`config/config.go:111`, IP client `web/server.go:300-318`) | SANS OBJET | Mécanique PHP / intégrations ; `http_client_ip_check_proxy` équivalent à `trusted_proxy` |
| `defaultValues` : `date_from_headers` | Date de l'en-tête `Date` plutôt que date de réception | Date affichée | `summaryOf` prend l'enveloppe si non nulle (`mailconn/messages.go:387-389`) | OK | — |
| `defaultValues` : `autocreate_system_folders` | Crée les dossiers système manquants | Envoi/archivage sans erreur | `ResolveSent/ResolveDrafts` (`mailconn/client.go:511-516`), dossiers retrouvés ou créés (§14.2 dernière ligne), Envoyés créé si absent (décision 120) | OK | — |
| `defaultValues` : `allow_message_append` | Autorise l'`APPEND` de messages | Dépôt de `.eml` | `web/mailappend.go:18` (toujours actif) | OK | — |
| `defaultValues` : `login_fault_delay` | Pause (1 s) après un échec de connexion | Ralentir la force brute | Limitation de débit par IP et par login (`web/handlers.go:29-47,74-79`, `config/config.go:74`) | OK | Plus efficace qu'une pause fixe |
| `defaultValues` : `allow_ctrl_enter_on_compose` | Ctrl+Entrée envoie | Composition | `web/static/keys.js`, `compose.js` (décision 137) | OK | — |
| `defaultValues` : `allow_html_editor_source_button`, `allow_html_editor_biti_buttons` | Boutons « source » et gras/italique de l'éditeur | Éditeur | Éditeur `web/static/editor.js` (barre sans bouton source) | OK | Bouton « source HTML » volontairement non repris (risque d'injection avant assainissement serveur) |
| `defaultValues` : `try_to_detect_hidden_images` | Détection des pixels espions | Confidentialité | `mailview/sanitize.go` retire les pixels de 0/1 px (décision 152) | OK | — |
| `defaultValues` : `hide_dangerous_actions` | Masque suppression définitive/vidage | Éviter les erreurs | Confirmation avant suppression définitive (§14.2) | OK | — |
| `defaultValues` : `use_imap_sort` | Tri par le serveur (SORT) | Tris rapides | SORT utilisé (`mailconn/search.go`, tests Dovecot décision 140) | OK | — |
| `defaultValues` : `use_imap_force_selection` | Forcer le SELECT avant certaines commandes | Compat. serveurs | Sélection par commande (`mailconn`) | OK | — |
| `defaultValues` : `use_imap_list_subscribe` | Utiliser LSUB/LIST-EXTENDED | Dossiers abonnés | LIST-EXTENDED (`web/foldertree.go`, décision 124) | OK | — |
| `defaultValues` : `use_imap_thread` | Conversations par THREAD | Conversations | Décision 154 (`mailconn`, `web/mail.go`) | OK | — |
| `defaultValues` : `use_imap_move` | MOVE au lieu de COPY+suppression | Déplacements | MOVE si annoncé, sinon COPY+STORE+EXPUNGE par lots (`mailconn/messages.go:634-654`) | OK | — |
| `defaultValues` : `use_imap_expunge_all_on_delete` | EXPUNGE global à la suppression | Compat. serveurs | UID EXPUNGE ciblé (UIDPLUS) sinon EXPUNGE (`mailconn/messages.go:647-654`) | OK | Plus sûr |
| `defaultValues` : `imap_forwarded_flag`, `imap_read_receipt_flag` | Noms des drapeaux « transféré » et « accusé envoyé » | Marquage | `$Forwarded` (`mailconn/messages.go:408`), `$MDNSent` (décision 139) | OK | — |
| `defaultValues` : `imap_body_text_limit` | Limite de lecture du corps texte | Gros messages | `mail.err.too_large` (`fr.json:176`) | OK | — |
| `defaultValues` : `imap_message_list_fast_simple_search`, `imap_message_list_count_limit_trigger`, `imap_message_list_date_filter`, `imap_message_list_permanent_filter` | Réglages avancés de la liste (filtre de date/permanent) | Liste sur gros dossiers | Recherche par critères IMAP (`mailconn/search.go`) ; pas de filtre permanent | SANS OBJET | Usage d'administrateur |
| `defaultValues` : `imap_message_all_headers` | Récupère tous les en-têtes | Voir les en-têtes | « Voir la source/en-têtes » (`web/mailtools.go`, `/mail/source`) | OK | — |
| `defaultValues` : `imap_large_thread_limit` | Seuil de gros fils | Conversations | Fils bornés par `THREAD` serveur (décision 154) | OK | — |
| `defaultValues` : `imap_folder_list_limit`, `folders_spec_limit` | Limite d'affichage de dossiers | Boîtes à très nombreux dossiers | arbre repliable (décision 142) ; pas de limite | SANS OBJET | L'arbre est replié par défaut |
| `defaultValues` : `imap_show_login_alert` | Affiche l'alerte `[ALERT]` du serveur IMAP | Information du serveur | aucun (grep `ALERT` : aucun) | MANQUANT | **P3** : dans `mailconn` après `Login`, relayer les réponses `* OK [ALERT] …` (go-imap : `Options.UnilateralDataHandler`) vers un message d'information de la page |
| `defaultValues` : `imap_use_auth_plain`, `imap_use_auth_cram_md5`, `smtp_use_auth_plain`, `smtp_use_auth_cram_md5` | Mécanismes d'authentification | Compatibilité | SMTP PLAIN/LOGIN (`mailconn/client.go:274-281`) ; IMAP `LOGIN` | PARTIEL | Voir `IncConnectAndLoginHelper` (P3) |
| `defaultValues` : `smtp_show_server_errors` | Montre le message d'erreur du serveur SMTP | Diagnostic d'envoi | `compose.err.send` affiche l'erreur (`fr.json:264`) | OK | — |
| `defaultValues` : `sieve_allow_raw_script`, `sieve_utf8_folder_name`, `sieve_auth_plain_initial`, `sieve_allow_fileinto_inbox`, `sieve_timeout` | Réglages Sieve | Filtres | script brut toujours permis (`web/filters.go:455`) ; noms de dossier en UTF-8 seulement (limite connue, décision 134) ; authentification PLAIN (`sieve/client.go:273`) ; `fileinto "INBOX"` produit pour « garder une copie » (`sieve/filters.go:337-340`) ; délai : contexte 30 s | PARTIEL | **P3** : option par boîte « noms de dossier Sieve en UTF-7 modifié » (`SieveConfig.UTF7`) pour les serveurs sans `sieve_utf8` : convertir `ActionValue` dans `filterScript` (`sieve/filters.go:340`) avec `sieve/utf7.go` (déjà présent) |
| `defaultValues` : `imap_timeout`, `smtp_timeout` | Délais de connexion | Robustesse | IMAP 30 s (`web/mail.go:32`), SMTP 90 s (`web/compose.go:29`), mais coupés à 30 s par `WriteTimeout` du serveur HTTP (`cmd/wmail-go/main.go:135`) | PARTIEL | Voir `privateUpload` (P2) |
| `defaultValues` : `domain_list_limit`, `mail_func_*` | Administration / `mail()` PHP | — | aucun | SANS OBJET | — |
| `defaultValues` : `favicon_status` | Compteur de non lus dans l'icône de l'onglet | Voir le nombre de non lus sans ouvrir l'onglet | Titre de l'onglet « (N) … » (décision 151, `web/static/poll.js`) ; pas de pastille sur l'icône (pas d'icône) | PARTIEL | **P3** : après ajout d'un favicon, `poll.js` peut le redessiner sur un `<canvas>` avec le compteur |
| `defaultValues` : `detect_image_exif_orientation` | Corrige l'orientation EXIF des vignettes | Vignettes bien orientées | `mailview/thumb.go` ne lit pas l'EXIF (grep `orient` : aucun) | MANQUANT | **P3** : lire la balise Orientation (0x0112) des JPEG dans `Thumbnail` (`mailview/thumb.go:23`) et pivoter l'image avant recadrage ; sans dépendance : analyse simple des premiers Ko |
| `defaultValues` : `cookie_default_path`, `cookie_default_secure` | Cookies : chemin et `Secure` | Cookies sûrs | `Secure` automatique (`web/server.go:355`) ; chemin `/` | OK | Chemin : voir `base_path` (P3) |
| `defaultValues` : `check_new_messages` | Vérification de nouveaux messages | Notifications | `/mail/poll` toutes les 2 minutes (`web/poll.go:44`, décision 151) | OK | — |
| `defaultValues` : `strict_html_parser` | Assainissement HTML strict | Sécurité de lecture | Liste blanche stricte `bluemonday` (`mailview/sanitize.go:68-88`) | OK | — |
| `defaultValues` : `force_https` | Redirige vers HTTPS | HTTPS obligatoire | aucun | PARTIEL | Voir `Service::__construct` (HSTS P2) |
| `defaultValues` : `version` | Version installée | Mises à jour | `/healthz` (version non exposée) | SANS OBJET | — |
| `Plugin::__construct` (15), `convertConfigMap` (26), `defaultValues` (55) | Configuration par plugin | Plugins | aucun | SANS OBJET | — |

---

## 13. Enumerations/* et Exceptions/*

| Élément | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Capa` : 27 capacités (2FA, 2FA forcée, OpenPGP, prélecture, Gravatar, thèmes, fond utilisateur, Sieve, filtres, vignettes, comptes/identités supplémentaires, dossiers, composition, contacts, rechargement, recherche (avancée), réglages, quota, aide, modèles, actions de message/liste/pièces, actions dangereuses, déconnexion automatique) | Interrupteurs de fonctions | Voir §12 | Toutes présentes sauf : OpenPGP (hors périmètre), Gravatar (refusé), prélecture (MANQUANT P3), aide intégrée (aide des raccourcis `?` `web/static/keys.js`) | OK | Voir lignes détaillées |
| `Layout` : `NO_PREVIEW`, `SIDE_PREVIEW`, `BOTTOM_PREVIEW` | Trois dispositions | Disposition | `list` ≈ sans aperçu, `three` ≈ aperçu à droite ; pas d'aperçu en bas | PARTIEL | **P2** (voir `view_layout`) |
| `PluginPropertyType`, `SignMeType`, `TimeFormat`, `UploadClientError`, `UploadError` | Constantes de plugins, « se souvenir de moi », format d'heure, erreurs d'envoi | Types d'erreurs d'envoi de fichier | Erreurs d'envoi : `compose.err.attach_*` (`fr.json:258-268`) ; format 12/24 h : suit la langue | OK | — |
| `ClientException` : `__construct` (21), `getAdditionalMessage` (37), `getLogoutOnException` (45), `setLogoutOnException` (55) | Exception destinée au client, avec déconnexion forcée en option | Déconnecter si l'authentification IMAP échoue en cours de route | Erreur d'authentification de la boîte → page « corriger ce compte » (`web/templates/base.html:200`) ; la session wmail reste ouverte (voulu : la boîte peut être corrigée) | OK | — |
| `Exception`, `InvalidArgumentException`, `RuntimeException` | Hiérarchie d'exceptions | Infrastructure | `error` Go | SANS OBJET | — |
| `Common/BackwardCapability/Account.php` (`Account` vide) | Compatibilité de classe | — | — | SANS OBJET | — |

---

## 14. Common/* (PDO, mbstring, Facebook)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `PdoAbstract::IsSupported` (35), `SetLogger` (43), `getPdoAccessData` (51), `sqliteNoCaseCollationHelper` (62), `getPDO` (73), `lastInsertId` (150), `beginTransaction` (165), `commit` (173), `rollBack` (181), `prepareAndExecute` (194), `prepareAndExplain` (237), `writeLog` (269), `getUserId` (284), `quoteValue` (340), `getSystemValue` (352), `getVersion` (390), `setVersion` (401), `initSystemTables` (433), `dataBaseUpgrade` (528) | Couche SQL (carnet d'adresses : MySQL/PostgreSQL/SQLite), versions de schéma, identifiant d'utilisateur | Stockage du carnet | Carnet dans un fichier chiffré par compte (`store/store.go:541-568`, `internal/contacts`) ; version de format du fichier `userdata.CurrentVersion` | SANS OBJET | Pas de SQL |
| `MbStringFix` : `mb_strtoupper` (5), `mb_strcut` (409), `mb_detect_encoding` (421), `mb_check_encoding` (454) | Polyfills de fonctions mbstring | Compat. PHP | Go natif UTF-8 | SANS OBJET | PHP |
| `FacebookRainLoopPersistentDataHandler::__construct` (22), `get` (41), `set` (61), `generateSessionVariableName` (77) ; `RainLoopFacebookRedirectLoginHelper::__construct` (27), `GetRLAppId` (40), `initRainLoopData` (45), `storeState` (82), `loadState` (103), `generateSessionVariableName` (123) | Connexion Facebook | Réseaux sociaux | aucun | SANS OBJET | Hors périmètre |

---

## 15. Social.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (22), `GoogleDisconnect` (31), `FacebookDisconnect` (63), `TwitterDisconnect` (96), `popupServiceResult` (128), `GoogleRefreshToken` (174), `GooglePopupService` (192), `FacebookPopupService` (353), `TwitterPopupService` (458), `GoogleConnector` (635), `TwitterConnector` (678), `FacebookConnector` (709), `GoogleUserLoginStorageKey` (757), `FacebookUserLoginStorageKey` (765), `TwitterUserLoginStorageKey` (773), `loginProcess` (785) | Connexion et liaison de comptes Google/Facebook/Twitter, Gmail par OAuth, Drive/Dropbox | Authentification sociale | aucun | SANS OBJET | Décision §14.4 (hors périmètre : sécurité et maintenance) |

---

## 16. Plugins/*

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `AbstractPlugin` : `__construct` (47), `Config` (63), `Manager` (83), `Path` (91), `Name` (99), `Version` (107), `UseLangs` (116), `configMapping` (129), `Hash` (137), `Supported` (145), `ConfigMap` (154), `SetPath` (173), `SetName` (185), `SetVersion` (197), `SetPluginManager` (212), `SetPluginConfig` (224), `PreInit` (234), `Init` (242), `FilterAppDataPluginSection` (254), `addHook` (265), `addJs` (281), `addTemplate` (297), `replaceTemplate` (313), `addPartHook` (329), `addAjaxHook` (345), `addTemplateHook` (363), `ajaxResponse` (382), `ajaxParam` (399), `getUserSettings` (412), `saveUserSettings` (427) | Base des plugins (crochets, JS, gabarits, réglages par utilisateur) | Extension du webmail | aucun | SANS OBJET | Plugins hors périmètre (§14.4) |
| `Helper::__construct` (10), `ValidateWildcardValues` (21) | Compare une valeur à une liste avec jokers (utilisée aussi pour les domaines `*`) | Domaines génériques | aucun | SANS OBJET | Domaines administrateur |
| `Manager` : `__construct` (70), `CreatePluginByName` (120), `InstalledPlugins` (160), `convertPluginFolderNameToClassName` (195), `Actions` (206), `Hash` (214), `HaveJs` (230), `CompileJs` (247), `CompileCss` (271), `CompileTemplate` (281), `InitAppData` (304), `AddHook` (356), `AddJs` (377), `AddTemplate` (400), `RunHook` (424), `AddAdditionalPartAction` (451), `RunAdditionalPart` (473), `AddProcessTemplateAction` (503), `AddAdditionalAjaxAction` (536), `HasAdditionalAjax` (556), `RunAdditionalAjax` (566), `AjaxResponseHelper` (585), `GetUserPluginSettings` (595), `SaveUserPluginSettings` (620), `ReadLang` (661), `ProcessTemplate` (689), `bIsEnabled` (711), `Count` (719), `SetLogger` (731), `WriteLog` (749), `WriteException` (763) | Gestionnaire de plugins | Extension | aucun | SANS OBJET | Idem |
| `Property` : `__construct` (42), `NewInstance` (58), `SetType` (68), `SetDefaultValue` (80), `SetPlaceholder` (92), `SetLabel` (104), `SetDescription` (116), `SetAllowedInJs` (127), `Name` (137), `AllowedInJs` (145), `Description` (153), `Label` (161), `Type` (169), `DefaultValue` (177), `Placeholder` (185), `ToArray` (193) | Descripteur de propriété de configuration de plugin | Plugins | aucun | SANS OBJET | Idem |

---

## 17. Providers/Domain.php et Providers/Domain/*

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Domain::__construct` (27), `IsAdmin` (38), `Load` (51), `Save` (67), `SaveAlias` (78), `Delete` (93), `Disable` (104), `GetList` (117), `Count` (139), `LoadOrCreateNewFromAction` (150), `CreateNewAliasFromAction` (223), `IsActive` (293) | Gestion des domaines (CRUD, alias, désactivation, liste) pilotée par l'administrateur | Administration de domaines | aucun : chaque boîte porte ses serveurs (`userdata.MailAccount`) | SANS OBJET | Pas d'administrateur |
| `DefaultDomain::__construct` (23), `codeFileName` (35), `wildcardDomainsCacheKey` (61), `getWildcardDomainsLine` (69), `Load` (115), `Save` (210), `SaveAlias` (229), `Disable` (248), `Delete` (285), `GetList` (323), `Count` (406) | Domaines dans des fichiers `.ini`, domaine par défaut `*`, alias, jokers, liste désactivée ; côté utilisateur : le serveur est **déduit du domaine de l'adresse** à la connexion | Résolution du serveur d'un utilisateur d'après son domaine | aucun : pas de **pré-remplissage** d'une nouvelle boîte d'après le domaine de l'adresse (grep `autoconfig` : aucun) | MANQUANT | **P3** : dans `web/accounts.go` `accountNew`, remplir imap/smtp/sieve avec `imap.<domaine>`/`smtp.<domaine>` et ports usuels (993 TLS, 587 STARTTLS) en simple suggestion modifiable ; ne pas faire de requête réseau tierce |
| `DomainAdminInterface` (14-55), `DomainInterface` | Interfaces | — | — | SANS OBJET | PHP |

---

## 18. Providers/Files*, Providers/Storage*

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Files::__construct`, `PutFile`, `MoveUploadedFile`, `GetFile`, `GetFileName`, `Clear`, `FileSize`, `FileExists`, `GC`, `CloseAllOpenedFiles`, `GenerateLocalFullFileName`, `IsActive` (`Providers/Files.php`) | Façade du stockage temporaire des fichiers envoyés (pièces jointes en attente) | Pièces jointes temporaires | `attachStore` en mémoire : `Add` (`web/attachstore.go:57`), `List` (83), `Remove` (90), `Clear` (112), `ClearSession` (124), `Compositions` (132) | OK | — |
| `Files\FileStorage::__construct` (7), `GenerateLocalFullFileName`, `PutFile`, `MoveUploadedFile`, `GetFile`, `GetFileName`, `Clear`, `FileSize`, `FileExists`, `GC` (nettoie après 24 h), `CloseAllOpenedFiles`, `generateFullFileName` (111) | Stockage sur disque par compte, nom haché, nettoyage des fichiers de plus de 24 h | Ne pas laisser de pièces temporaires traîner | Mémoire seulement, jamais sur disque (décision 117) ; libération à l'envoi/abandon et à la fermeture ou l'expiration de la session : `sessions.OnClose` appelle `attach.ClearSession` (`web/server.go:73`), balayage périodique `Sweep` (`session/session.go:144-152`) | OK | Plus sûr (rien sur disque) |
| `Files\IFiles` | Interface | — | — | SANS OBJET | PHP |
| `Storage::__construct`, `verifyAccount` (10), `Put`, `Get`, `Clear`, `DeleteStorage`, `IsActive`, `IsLocal` | Façade du stockage de données utilisateur | Persistance | `store.Store` : `SaveData` (`store/store.go:499`), `LoadBlob/SaveBlob` (526,541), `Delete` (573) | OK | — |
| `Storage\FileStorage::__construct`, `Put`, `Get`, `Clear`, `DeleteStorage`, `IsLocal`, `generateFileName` (59), `SetLogger` | Fichiers `cfg/` et `data/` par adresse (nom assaini), suppression du dossier d'un compte | Données utilisateur sur disque | Fichier unique chiffré par compte, nom haché HMAC (`store/store.go:169-178`), écriture atomique (`store/store.go:205`), suppression complète `Delete` (`store/store.go:573-596`) | OK | Plus sûr (nom non réversible, contenu chiffré) |
| `Storage\TemproryApcStorage::Put`, `Get`, `Clear`, `DeleteStorage`, `generateFileName` | Stockage temporaire en mémoire APC par jeton de connexion | Données temporaires de session | `session.Session` en mémoire (`session/session.go`) | OK | — |
| `Storage\IStorage`, `StorageType::{USER, CONFIG, NOBODY}` | Interface et types de stockage | — | — | SANS OBJET | PHP |

---

## 19. Providers/Version.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (10), `IsActive` (19) | Initialisation | Mises à jour | — | SANS OBJET | — |
| `ClearOldVersion` (24) | Supprime les anciennes versions installées (garde les 5 dernières) | Maintenance du dossier d'installation | aucun (binaire unique, image Docker) | SANS OBJET | Déploiement par image |
| `UpdateCore` (82) | Télécharge, extrait et installe une mise à jour du cœur | Mise à jour automatique | aucun (reconstruction de l'image) | SANS OBJET | Réduit la surface d'attaque |

---

## 20. Providers/Filters.php, Filters/*, Filters/Classes/*, Filters/Enumerations/*

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Filters::__construct` (15), `IsActive` (80) | Fournisseur Sieve | Filtres | `web/filters.go` | OK | — |
| `Filters::Load` (26) | Lit les filtres ; erreur de connexion distincte de l'erreur de lecture | Message adapté (serveur injoignable ou autre) | `sieveErrKey` : `filters.err.unavailable` pour refus/délai/DNS/bloqué, sinon `accounts.test.fail.*` (`web/filters.go:123-136`, `fr.json:486`) | OK | — |
| `Filters::Save` (52) | Enregistre ; refus du serveur renvoyé tel quel à l'utilisateur | Voir pourquoi le script est refusé | `filters.err.server` avec le texte du serveur (`web/filters.go:123-128`) ; vérification préalable `CHECKSCRIPT` pour le script brut (`web/filters.go:485`) | OK | — |
| `FiltersInterface::Load/Save` | Interface | — | — | SANS OBJET | PHP |
| `SieveStorage::__construct` (35) | Lit `sieve_utf8_folder_name` | Dossiers en UTF-8 dans le script | UTF-8 toujours (décision 131/134) | PARTIEL | **P3** : option UTF-7 (voir `defaultValues` Sieve) |
| `SieveStorage::Load` (51) | Lit `rainloop.user` (filtres de la liste) et `rainloop.user.raw` ; détermine quel script est actif ; renvoie les modules (`fileinto`, `regex`, `relational`, `date`, `reject`, `vacation`, `imap4flags`) | Page de filtres avec options grisées selon le serveur | `loadSieve` (`web/filters.go:58-92`), modules par `c.Extensions()` (`web/filters.go:61`) ; mêmes noms de scripts (`sieve/filters.go:17-21`) ; détail : RainLoop masque la liste si aucun des deux scripts n'est actif, wmail-go l'affiche toujours (plus utile) | OK | — |
| `SieveStorage::Save` (118) | Écrit `rainloop.user` et l'active, ou le script brut ; supprime le script quand il n'y a plus de filtre | Enregistrer/supprimer | `saveSieveFilters` (`web/filters.go:94-121`), `filtersRaw` (`web/filters.go:455-516`) | OK | — |
| `SieveStorage::conditionToSieveScript` (167) | Condition → test Sieve : `:over`, `:under`, `:is`, `:contains`, `:regex`, `not`, champs From/To+CC/Subject/en-tête/taille, listes séparées par virgule, valeur vide → `false` | Création de règles de tri | `conditionScript` (`sieve/filters.go:190-250`), avec en plus nom d'en-tête restreint et taille validée | OK | — |
| `SieveStorage::filterToSieveScript` (271) | Filtre → bloc Sieve : `anyof`/`allof`, `addflag "\\Seen"`, actions stop/discard/vacation/reject/redirect (+copie)/fileinto | Actions de filtre | `filterScript` (`sieve/filters.go:252-352`) | OK | — |
| `SieveStorage::collectionToFileString` (476) | Assemble le script complet : en-tête `# RAINLOOP:SIEVE`, bloc JSON base64 par filtre, filtres désactivés commentés, `require` | Relecture par RainLoop et par wmail-go | `BuildScript` (`sieve/filters.go:355-399`), `ParseScript` (`sieve/filters.go:152`) ; neutralisation de `*/` (décision 133) | OK | — |
| `SieveStorage::fileStringToCollection` (517) | Relit les filtres d'un script (balises BEGIN:FILTER/HEADER, base64, JSON) | Modifier les filtres existants | `ParseScript`, `unmarshalHeader` (`sieve/filters.go:125-170`), booléens PHP tolérés (`flexBool` `sieve/filters.go:50`) | OK | — |
| `SieveStorage::quote` (552) | Échappe `\` et `"` | Pas d'injection de script | `qs` (`sieve/filters.go:170`) | OK | — |
| `SieveStorage::SetLogger` (560) | Journalisation | — | `slog` | SANS OBJET | — |
| `Filter::__construct` (77), `Clear` (82) | Valeurs par défaut : activé, `Any`, `MoveTo`, `Keep` vrai, `Stop` vrai | Nouveau filtre | `filterFromForm`/`filterEdit` (`web/filters.go:251-332`) | OK | — |
| `Filter::ID` (107), `Enabled` (115), `Name` (123), `Conditions` (131), `ConditionsType` (139), `ActionType` (147), `ActionValue` (155), `ActionValueSecond` (163), `ActionValueThird` (171), `ActionValueFourth` (179), `MarkAsRead` (187), `Stop` (195), `Keep` (203) | Propriétés | Filtres | `sieve.Filter` (`sieve/filters.go:67-81`) | OK | — |
| `Filter::serializeToJson` (211), `unserializeFromJson` (219), `FromJSON` (235), `ToSimpleJSON` (273) | (Dé)sérialisation JSON | Compatibilité RainLoop | `marshalHeader`/`unmarshalHeader` (`sieve/filters.go:108-146`) | OK | — |
| `FilterCondition::__construct` (27), `Clear` (32), `Field` (43), `Type` (51), `Value` (59), `ValueSecond` (67), `FromJSON` (77), `ToSimpleJSON` (101), `CollectionFromJSON` (114) | Condition (champ, type, valeur, nom d'en-tête) | Conditions | `sieve.Condition` (`sieve/filters.go:59-64`), `condRows` (`web/filters.go:238`) | OK | — |
| `ActionType`, `ConditionField`, `ConditionsType`, `ConditionType` | Énumérations (None/MoveTo/Discard/Vacation/Reject/Forward ; From/Recipient/Subject/Header/Size ; All/Any ; EqualTo/NotEqualTo/Contains/NotContains/Regex/Over/Under) | Valeurs permises | Mêmes valeurs (`sieve/filters.go:23-49`) | OK | — |

---

## 21. Providers/Suggestions*, TwoFactorAuth*, ChangePassword*

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Suggestions::__construct` (6), `Process` (16), `IsActive` (42) | Agrège les suggestions d'adresses de plusieurs sources, sans doublon, limitées | Autocomplétion des destinataires | `GET /contacts/suggest` (`web/server.go:193`, `web/contactsweb.go`), pastilles `web/static/contacts-ac.js`, `chips.js` | OK | — |
| `ISuggestions::Process` | Interface | — | — | SANS OBJET | PHP |
| `OwnCloudSuggestions::Process` (6), `SetLogger` (83) | Suggestions depuis les contacts ownCloud | Intégration ownCloud | aucun | SANS OBJET | ownCloud |
| `TestSuggestions::Process` (5) | Suggestions de test | Test | — | SANS OBJET | — |
| `TwoFactorAuth::__construct` (6), `IsActive` (10) | Fournisseur TOTP | 2FA | `internal/totp` | OK | — |
| `TwoFactorAuth::GetQRCodeGoogleUrl` (14) | Code QR **par un service Google** (envoie le secret à un tiers) | Configurer l'application d'authentification | QR généré localement (`web/twofactor.go:252`, `totp.URI` `totp/totp.go:88`) | OK | Plus sûr |
| `TwoFactorAuth::CreateSecret` (19) | Secret TOTP | Activation | `totp.NewSecret` (`totp/totp.go:32`) | OK | — |
| `TwoFactorAuth::VerifyCode` (28) ; `GoogleTwoFactorAuth::VerifyCode` (7), `CreateSecret` (13) ; `AbstractTwoFactorAuth::Label` (5), `VerifyCode` (9) ; `TwoFactorAuthInterface::VerifyCode` | Vérifie un code avec une fenêtre de ±8 pas (≈ ±4 min) | Tolérance à l'horloge | `totp.Verify` : ±1 pas, un code ne sert qu'une fois (`totp/totp.go:63`) | OK | Plus strict (voulu) ; élargir à ±2 si l'horloge du téléphone dérive |
| `ChangePassword::__construct` (8), `PasswordChangePossibility` (14), `ChangePassword` (21), `IsActive` (54) | Change le mot de passe **du compte de messagerie** via un pilote (plugin) : mot de passe actuel, longueur minimale 6, test de faiblesse | Changer le mot de passe de sa boîte depuis le webmail | Changement du mot de passe **wmail** (pas de la boîte) : `settingsPassword` (`web/server.go:197`, `store/store.go:406`) ; le mot de passe IMAP se modifie dans le formulaire de boîte (`web/accounts.go`) mais pas chez le serveur de messagerie | SANS OBJET | Dans RainLoop standard aucun pilote n'est fourni (plugins externes : poppassd, etc.) |
| `ChangePasswordInterface::PasswordChangePossibility/ChangePassword` | Interface | — | — | SANS OBJET | PHP |

---

# À AJOUTER (MANQUANT et PARTIEL), par priorité

## P2
1. **Disposition « aperçu en bas »** (`view_layout`=2) : ajouter `bottom` aux dispositions `three`/`list` (`web/handlers.go:403`, `web/templates/settings.html:19`, `web/static/mail.css`, `home.html`).
2. **Délais HTTP trop courts pour envois et téléchargements** : `ReadTimeout`/`WriteTimeout` à 30 s (`cmd/wmail-go/main.go:133-136`) coupent l'envoi d'une pièce de 25 Mo sur liaison lente, l'envoi SMTP (90 s prévus, `web/compose.go:29`) et les gros téléchargements ; prolonger par route avec `http.NewResponseController`.
3. **HSTS** : ajouter `Strict-Transport-Security` quand la requête arrive en HTTPS (`web/server.go:241-254`) ; la redirection HTTP→HTTPS reste au proxy.

## P3
4. **Favicon / icône tactile / manifeste** absents (`web/templates/base.html`).
5. **Cache navigateur et compression des statiques** (`Cache-Control`, `ETag`, gzip) pour `/static/`.
6. **Pré-remplissage d'une nouvelle boîte** d'après le domaine de l'adresse (`imap.`, `smtp.`, ports usuels), sans requête réseau.
7. **SMTP sans authentification** (case « aucune authentification » par boîte, `web/accounts.go:208-216`).
8. **EHLO avec un vrai nom de domaine** au lieu de `localhost` (`mailconn/client.go:235,252`).
9. **CA privée par boîte** (`ca_pem`) pour vérifier un serveur à certificat interne sans désactiver la vérification (`mailconn/client.go:48`).
10. **`AUTHENTICATE PLAIN` IMAP** quand `LOGINDISABLED` ou mécanisme PLAIN seul (`mailconn/client.go:147,162`).
11. **Adresses/domaines internationaux (IDN)** : conversion punycode via `x/net/idna`.
12. **Noms de dossiers Sieve en UTF-7** (option par boîte) pour serveurs sans `sieve_utf8` (`sieve/filters.go:340`, `sieve/utf7.go`).
13. **Alerte IMAP `[ALERT]`** du serveur relayée à l'utilisateur.
14. **Page « cookies requis »** quand le navigateur refuse le cookie de session.
15. **Prélecture du message suivant** (`allow_prefetch`).
16. **Liens automatiques dans les messages HTML** (`allow_smart_html_links`) (`mailview/sanitize.go`).
17. **Orientation EXIF des vignettes** (`mailview/thumb.go:23`).
18. **Compteur de non lus dans le favicon** (dépend du point 4).
19. **Réglage « afficher aussi les dossiers non abonnés »** (`ignore_folders_subscription`).
20. **Contrôle de faiblesse des mots de passe** (liste de mots courants) dans `store.checkPassword` (`store/store.go:179`).
21. **Unicité d'une boîte mail** (même adresse et même serveur) à l'ajout, message dédié.
22. **Reprise de la demande `mailto:` après connexion** (cookie `next` borné).
23. **Surcharge de la configuration par variables d'environnement** (`WMAIL_*`) dans `config.Load` (`config/config.go:172`).
24. **Chemin de base configurable** (`base_path`) pour un déploiement sous un préfixe d'URL.
25. **Session plus longue (« se souvenir de moi »)** : limite structurelle (clé dérivée du mot de passe) ; seule la durée maximale réglable est possible.
26. **Mise en page mobile** : pas de bascule, à vérifier à la main à 400 px.

Aucun écart de priorité P1 (usage quotidien) n'a été trouvé dans ce périmètre : tout ce qui est visible chaque jour (liste, lecture, filtres Sieve, 2FA, thèmes, modèles, identités, sondage, images distantes) est couvert.
