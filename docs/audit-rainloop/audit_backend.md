# Audit des fonctionnalités côté serveur (PHP) — RainLoop 1.17.0

Périmètre : `/home/gdegoulet/Work/webmail_project/rainloop/1.17.0/app/libraries/` (aucun dossier `data/` lu, rien exécuté). Lecture seule.
Les chemins sont relatifs à ce dossier. Abréviations : `A` = `RainLoop/Actions.php`, `S` = `RainLoop/Service.php`, `SA` = `RainLoop/ServiceActions.php`, `U` = `RainLoop/Utils.php`, `CfgApp` = `RainLoop/Config/Application.php`, `Acc` = `RainLoop/Model/Account.php`, `Dom` = `RainLoop/Model/Domain.php`, `DDom` = `RainLoop/Providers/Domain/DefaultDomain.php`, `Sieve` = `RainLoop/Providers/Filters/SieveStorage.php`, `MC` = `MailSo/Mail/MailClient.php`, `IC` = `MailSo/Imap/ImapClient.php`, `SMTP` = `MailSo/Smtp/SmtpClient.php`, `MSU` = `MailSo/Base/Utils.php`, `HU` = `MailSo/Base/HtmlUtils.php`.
Chaque entrée porte un type : **[U]** utilisateur, **[A]** administrateur, **[I]** interne (mécanisme technique, utile à connaître mais invisible).

Constat général : RainLoop 1.17 est une application « un seul fichier de configuration `application.ini` + fichiers par domaine + stockage fichier par utilisateur ». Pas de base de données applicative (sauf carnet d'adresses en PDO), pas de rôle/quota applicatif par utilisateur, pas de limite de débit par IP (seulement un `sleep` sur échec), pas de journal d'audit autre que les logs et le log d'authentification. Le côté « admin » est une application séparée (route `/?admin`).

---

## 1. Authentification et sessions

Mécanique : le « jeton de session » est un tableau sérialisé (e-mail, login, **mot de passe IMAP en clair dans le jeton**, empreinte du User-Agent, jeton sign-me, e-mail parent, jeton court, proxy-auth, durée de vie, certificat client), chiffré en XXTEA avec une clé dérivée de `APP_SALT` + jeton de session cookie (`Acc:395-432`, `U:149-254`). Il n'y a pas de session PHP ni de table de sessions : le serveur est « stateless » et reconstruit le compte à chaque requête.

- **F-BE-01 : Connexion par e-mail + mot de passe (DoLogin)** — [U] Action AJAX `Login` : paramètres `Email`, `Password`, `Language`, `SignMe`, `AdditionalCode`, `AdditionalCodeSignMe`. Retourne OK ou « 2FA requise ». Mémorise la langue choisie à la connexion dans les réglages du compte. — `A:2416-2479`
- **F-BE-02 : Pipeline de connexion en 2 étapes (LoginProcess)** — [I] Hooks `step-1`, normalisation e-mail (trim, minuscules si ASCII), complétion du domaine, `step-2`, hook `filter.login-credentials`, `LoginProvide` (charge le domaine, applique la liste blanche, fabrique le compte), contrôle 2FA, puis vraie connexion IMAP de vérification (`CheckMailConnection`). — `A:2201-2399`
- **F-BE-03 : Mise en minuscules de l'identifiant** — [A] `login.login_lowercase` (défaut oui) : e-mail et login passés en minuscules (ASCII uniquement). — `A:2209, 2298` ; `CfgApp:277`
- **F-BE-04 : Détection automatique du domaine à partir de l'hôte HTTP** — [A] Si l'utilisateur tape un identifiant sans `@` et `login.determine_user_domain` actif : on teste le nom d'hôte de la requête (sans `www.`), puis ses suffixes successifs (jusqu'à 14 niveaux), puis un domaine joker `*`. — `A:2214-2274` ; `CfgApp:268`
- **F-BE-05 : Domaine par défaut** — [A] `login.default_domain` ajouté si l'identifiant n'a pas de `@` après la détection. — `A:2276-2283` ; `CfgApp:262`
- **F-BE-06 : Hooks de transformation d'identifiants** — [I] `filter.login-credentials.step-1/step-2` (e-mail, mot de passe) et `filter.login-credentials` (e-mail, login, mot de passe) permettent de mapper un identifiant saisi vers un autre login IMAP/SMTP. — `A:2206, 2286, 2303`
- **F-BE-07 : Délai artificiel après échec de connexion** — [A] `labs.login_fault_delay` (défaut 1 s) : `sleep` si la requête a duré moins d'une seconde, sur identifiant incomplet, échec de provision, échec 2FA, échec IMAP, échec de changement de mot de passe et échec admin. C'est la seule protection anti-force-brute interne. — `A:2090-2108, 2290, 2329, 2370, 2393, 3902, 5053` ; `CfgApp:413`
- **F-BE-08 : Journal d'authentification dédié pour fail2ban** — [A] `logs.auth_logging`, `auth_logging_filename` (défaut `fail2ban/auth-{date:Y-m-d}.txt`), `auth_logging_format` (défaut `[{date:Y-m-d H:i:s}] Auth failed: ip={request:ip} user={imap:login} host={imap:host} port={imap:port}`). Écrit pour échecs IMAP, 2FA et admin. Jetons disponibles : `{date:..}`, `{request:ip}`, `{request:domain}`, `{user:email|login|domain|uid|ip}`, `{imap:login|host|port}`, `{smtp:...}`, `{labs:rand:N}`. — `A:454-584, 1169-1215, 2176-2188` ; `CfgApp:350-352`
- **F-BE-09 : Jeton de session en cookie** — [I] Cookie `rlspecauth` (session navigateur, httponly) renvoyé dans l'URL sous forme `_<jeton>` pour les liens `Raw`, `Lang`, etc. Cookies : `rlspecauth`, `rlsmauth`, `rltfasmauth`, `rlmailtoauth`, `rlspeclogout`, `rlspeclogoutcmk`, `rlaauth` (admin), `rlmobiletype`, `rltoken` (30 j), `rlsession`. — `A:10-17, 625-632` ; `U:259-310, 494-528`
- **F-BE-10 : Liaison de la session au navigateur** — [I] Le jeton embarque `md5(User-Agent)` et un « jeton court » (cookie `rlsession`) validé à chaque requête ; sans correspondance, `AuthError`. Exception : `GetShortLifeSpecAuthToken` accepte un horodatage d'expiration (index 10). — `A:243-253, 1329-1372` ; `U:276-296`
- **F-BE-11 : Jeton à courte durée (60 s) pour liens externes** — [I] Utilisé pour la prévisualisation Google Docs, l'envoi vers Dropbox : le jeton est réémis avec une durée de vie limitée. — `A:243-253, 3152, 8043-8052`
- **F-BE-12 : « Se souvenir de moi » (Sign me)** — [U] Cookie `rlsmauth` (30 jours) + enregistrement côté serveur `sign_me` (jeton chiffré, horodatage) ; reconnexion silencieuse au chargement de l'application avec re-vérification IMAP. Si l'IMAP refuse, les données sign-me sont effacées. — `A:634-651, 1377-1411` ; `SA:1248-1275`
- **F-BE-13 : Politique « sign me » configurable** — [A] `login.sign_me_auto` : `DefaultOff` (défaut), `DefaultOn`, `Unused`. — `CfgApp:279-286`
- **F-BE-14 : Authentification à deux facteurs TOTP** — [U] Création d'un secret + 9 codes de secours (9 chiffres), affichage du secret, activation/désactivation, test d'un code, suppression. Stockage chiffré par compte parent. Codes via `GoogleTwoFactorAuth` (TOTP). — `A:6666-6930` ; `RainLoop/Providers/TwoFactorAuth*`
- **F-BE-15 : 2FA exigée à la connexion avec mémorisation 14 jours** — [U] Si activée : code requis sauf cookie `rltfasmauth` (hash secret+empreinte UA, 14 jours) ; un code de secours (>6 chiffres) est consommé après usage. — `A:2334-2385, 6734-6766`
- **F-BE-16 : 2FA activable et imposable** — [A] `security.allow_two_factor_auth` (défaut non), `security.force_two_factor_auth` (défaut non → drapeau `RequireTwoFactor` jusqu'à activation). Réservé aux comptes principaux. — `CfgApp:225-226` ; `A:1741-1753, 8192-8202`
- **F-BE-17 : QR code 2FA via un service tiers** — [I] L'URL `otpauth://` est transmise à Google Charts (`chart.googleapis.com`) : fuite du secret vers un tiers. À noter pour tout équivalent. — `RainLoop/Providers/TwoFactorAuth.php` (méthode `GetQRCodeGoogleUrl`)
- **F-BE-18 : Comptes supplémentaires (multi-comptes)** — [U] Ajouter/supprimer un autre compte IMAP (vérifié par connexion réelle), changer de compte en un clic (`/?/Change/<email>`), maximum de 7 comptes pour les compteurs. Stockage `accounts` (jetons chiffrés de chaque compte). — `A:2486-2538, 2890-2973` ; `SA:1167-1203`
- **F-BE-19 : Compteurs de non lus par compte** — [U] Ouvre une connexion IMAP pour chaque compte et lit `STATUS INBOX (UNSEEN)`. — `A:3445-3518` ; `MC:971-981`
- **F-BE-20 : Ordre personnalisé des comptes et identités** — [U] `accounts_identities_order`. — `A:3370-3389, 2510-2525, 2678-2699`
- **F-BE-21 : Identités d'expédition** — [U] Plusieurs identités : e-mail, nom, Reply-To, Cci par défaut, signature, position de la signature (avant/après). L'identité par défaut est celle du compte. — `A:2624-2735, 3182-3252` ; `RainLoop/Model/Identity.php`
- **F-BE-22 : Modèles de message (templates)** — [U] Liste, création/maj, suppression, lecture (corps différé). Désactivés par défaut. — `A:2545-2617, 3259-3363, 3424-3436` ; `RainLoop/Model/Template.php` ; `CfgApp:253`
- **F-BE-23 : Déconnexion** — [U] Efface le cookie de session et, si besoin, les données sign-me. — `A:3523-3556`
- **F-BE-24 : Message personnalisé affiché à la déconnexion forcée** — [I] Une exception cliente peut demander un `Logout` avec message (cookie `rlspeclogoutcmk`) affiché sur l'écran de connexion. — `A:687-704` ; `SA:195-208`
- **F-BE-25 : Rotation du jeton après changement d'état** — [I] `UpdateToken` renvoyé dans la réponse AJAX (mot de passe changé, jeton OAuth Google rafraîchi). — `A:2113-2126` ; `SA:217-221`
- **F-BE-26 : Connexion SSO à usage unique (`/?/Sso&hash=`)** — [A] Un système externe appelle `Api::GetUserSsoHash(email, pass, options)` ; le hash est stocké en cache chiffré, consommé et supprimé à la première utilisation, valable 10 s (ou illimité si `bUseTimeout` faux). Options supportées : langue. — `SA:941-1019` ; `RainLoop/Api.php:204-225`
- **F-BE-27 : Authentification REMOTE_USER (`/?/RemoteAutoLogin`)** — [A] Utilise les variables d'environnement `REMOTE_USER` et `REMOTE_PASSWORD` (authentification Apache/proxy). — `SA:1024-1054`
- **F-BE-28 : Login externe par formulaire/API (`/?/ExternalLogin`)** — [A] `labs.allow_external_login` (défaut non) ; paramètres `Email`, `Password`, `Output=Redirect|Json`. — `SA:1059-1122` ; `CfgApp:465`
- **F-BE-29 : SSO externe par clé partagée (`/?/ExternalSso`)** — [A] `labs.allow_external_sso` + `labs.external_sso_key`, retourne un hash SSO en texte ou JSON. — `SA:1127-1165` ; `CfgApp:466-467`
- **F-BE-30 : Intégration OwnCloud/Nextcloud** — [A] Route `/?/OwnCloudAuth` (identifiants en variables d'environnement), détection `RAINLOOP_OWNCLOUD`, suggestions de contacts OwnCloud, enregistrement des pièces jointes dans un dossier OwnCloud (`labs.owncloud_save_folder` défaut `Attachments`), `labs.owncloud_suggestions` défaut oui, auto-logout désactivé. — `SA:255-294` ; `U:533-544` ; `A:3086-3147, 8254-8257` ; `RainLoop/Providers/Suggestions/OwnCloudSuggestions.php`
- **F-BE-31 : Gestionnaire de liens `mailto:` (`/?/Mailto&to=`)** — [U] Mémorise l'adresse dans un cookie chiffré, ouvre la composition après connexion (si sign-me). — `SA:894-910` ; `A:1265-1276, 1615-1628`
- **F-BE-32 : Connexion et liaison par réseaux sociaux** — [U] Google, Facebook, Twitter : popup OAuth, enregistrement de l'identifiant social → e-mail/mot de passe (chiffré, stockage « nobody »), déconnexion, connexion via ce lien. Options de pilotage dans la section `social`. — `RainLoop/Social.php:31-130, 192-812` ; `A:7561-7608` ; `SA:800-820`
- **F-BE-33 : Connexion Gmail par OAuth2 (XOAUTH2)** — [U] Le « mot de passe » du compte est alors un pseudo-mot de passe `jeton d'accès + jeton de rafraîchissement` ; rafraîchi toutes les 20 min, utilisé pour IMAP **et** SMTP via `AUTHENTICATE XOAUTH2`. — `A:2020-2063` ; `Acc:97-121, 495-503, 575-583`
- **F-BE-34 : Authentification par proxy IMAP (PROXYAUTH)** — [A] Un compte peut se connecter avec des identifiants proxy (administrateur) puis s'identifier comme l'utilisateur (`PROXYAUTH`). — `Acc:480-486` ; `IC:200-330`
- **F-BE-35 : Authentification IMAP** — [I] CRAM-MD5 (si le serveur l'annonce et que `labs.imap_use_auth_cram_md5`), AUTHENTICATE PLAIN, sinon `LOGIN` ; XOAUTH2 ; pas de SCRAM, pas de SASL-IR (désactivé par `&& false`), pas de NTLM/GSSAPI. — `IC:200-380` ; `CfgApp:440-441`
- **F-BE-36 : Authentification SMTP** — [I] CRAM-MD5, PLAIN (variante en une ligne optionnelle `__USE_SINGLE_LINE_AUTH_PLAIN_COMMAND`), LOGIN, XOAUTH2. — `SMTP:189-340`
- **F-BE-37 : Authentification ManageSieve** — [I] SASL PLAIN (avec réponse initiale paramétrable `labs.sieve_auth_plain_initial`) ou LOGIN. — `MailSo/Sieve/ManageSieveClient.php:157-240`
- **F-BE-38 : Authentification du panneau admin** — [A] Login + mot de passe (`security.admin_login` / `admin_password`, défaut `admin` / `12345`), session = jeton dans le cache fichier (cookie `rlaauth`). Le mot de passe est stocké avec `password_hash` (ou MD5 salé historique). Détection du mot de passe par défaut (`WeakPassword`). — `CfgApp:95-135, 222-223` ; `A:1220-1260, 3890-3920, 3978-4012, 1802`
- **F-BE-39 : Isolement du panneau admin** — [A] Chemin secret (`security.admin_panel_key`, défaut `admin`) **ou** hôte dédié (`security.admin_panel_host`) ; désactivable (`security.allow_admin_panel`). — `S:113-138` ; `CfgApp:224, 228-229`
- **F-BE-40 : Changement de mot de passe du compte** — [U] Contrôle : mot de passe actuel, longueur ≥ 6, liste noire de mots de passe courants (`PasswordWeaknessCheck`), drapeau `labs.check_new_password_strength`. **Aucun pilote fourni dans le noyau** (le pilote vient d'un plugin) ; le noyau régénère le jeton après succès. — `RainLoop/Providers/ChangePassword.php` ; `A:5036-5061, 369-370` ; `MSU:2510-2520`
- **F-BE-41 : Clients POP3 et poppassd présents mais non branchés** — [I] `MailSo/Pop3/Pop3Client.php` (connexion, login, STAT, CAPA) et `MailSo/Poppassd/PoppassdClient.php` (changement de mot de passe) existent mais ne sont référencés par aucun code RainLoop. — `MailSo/Pop3/Pop3Client.php:74-207` ; `MailSo/Poppassd/PoppassdClient.php:62-140`

---

## 2. Configuration par domaine (IMAP / SMTP / Sieve)

Un domaine = un fichier `domains/<nom>.ini` (`*` ↔ `default.ini`, jokers `_wildcard_`), plus des alias (`.alias`) et une liste de désactivés (`disabled`). Le domaine est déduit du **domaine de l'adresse e-mail saisie**.

- **F-BE-42 : Paramètres IMAP par domaine** — [A] `imap_host`, `imap_port` (défaut 143), `imap_secure` (None/SSL/TLS), `imap_short_login` (n'envoyer que la partie locale comme login). — `Dom:188-262` ; `Acc:174-183`
- **F-BE-43 : Paramètres SMTP par domaine** — [A] `smtp_host`, `smtp_port` (défaut 25), `smtp_secure`, `smtp_short_login`, `smtp_auth` (défaut oui), `smtp_php_mail` (utiliser `mail()` de PHP au lieu d'un serveur SMTP). — `Dom:207-217, 267-288` ; `A:6163-6221`
- **F-BE-44 : Paramètres Sieve par domaine** — [A] `sieve_use`, `sieve_host`, `sieve_port` (défaut 4190), `sieve_secure`, `sieve_allow_raw` (autorise l'édition du script brut). Le login Sieve = login IMAP. — `Dom:199-205` ; `Acc:604-651`
- **F-BE-45 : Sécurité de connexion automatique par port** — [I] Mode « auto » : SSL implicite pour 993/995/465, sinon STARTTLS si annoncé (`PreferStartTlsIfAutoDetect`). Les domaines enregistrés n'ont que None/SSL/TLS. — `MailSo/Net/Enumerations/ConnectionSecurityType.php` ; `Dom:295-329`
- **F-BE-46 : Liste blanche d'utilisateurs par domaine** — [A] `white_list` : seuls les logins listés (séparés par espaces/virgules/points-virgules) peuvent se connecter ; les `utilisateur@...` sont réduits à la partie locale. — `Dom:544-560` ; `A:1296-1309`
- **F-BE-47 : Domaine par défaut « * » et jokers** — [A] `default.ini` ; fichiers avec `*` (ex. `*.exemple.fr`) résolus par motif (`Helper::ValidateWildcardValues`), cache de la liste. — `DDom:35-105, 186-200` ; `RainLoop/Plugins/Helper.php:21-71`
- **F-BE-48 : Alias de domaine** — [A] Un domaine en redirige un autre (`.alias`), `AliasName` renvoyé au compte. — `DDom:171-185, 229-240`
- **F-BE-49 : Désactivation d'un domaine sans le supprimer** — [A] Fichier `disabled` (liste CSV). — `DDom:248-278` ; `A:4059-4067`
- **F-BE-50 : CRUD de domaines + liste** — [A] Charger, lister (limite `labs.domain_list_limit` défaut 99, alias inclus), créer/modifier, supprimer, alias. — `A:4017-4093` ; `DDom:285-409`
- **F-BE-51 : Test de domaine (IMAP/SMTP/Sieve) avec chronométrage** — [A] Connexion avec délai de 5 s sans authentification ; renvoie le message du socket. Le test SMTP tient compte du mode `mail()`. — `A:4098-4235`
- **F-BE-52 : Tolérance aux fautes de frappe `smpt_*`** — [I] Correction de clés mal orthographiées dans les fichiers de domaine. — `DDom:144-167`
- **F-BE-53 : Conversion IDN (punycode)** — [I] Hôtes et e-mails convertis (UTF-8 ↔ ASCII) ; nom de fichier de domaine encodé. — `Dom:353-363` ; `DDom:35-56` ; `MSU:2410-2478`
- **F-BE-54 : TLS : vérification de certificat** — [A] `ssl.verify_certificate` (défaut non), `ssl.allow_self_signed` (défaut oui), `ssl.cafile`, `ssl.capath`, `ssl.client_cert` (certificat client PEM transmis à IMAP). — `CfgApp:234-240` ; `RainLoop/Api.php:111-136` ; `MailSo/Net/NetClient.php:243-262`
- **F-BE-55 : Variantes TLS** — [I] STARTTLS testé avec `STREAM_CRYPTO_METHOD_ANY/TLS/SSLv23_CLIENT` ; pas de choix de version minimale. — `MailSo/Net/NetClient.php:316-345`
- **F-BE-56 : Hooks de substitution des identifiants de serveur** — [I] `filter.imap-credentials`, `filter.smtp-credentials`, `filter.sieve-credentials` (modifier hôte/port/login/mot de passe, activer/désactiver la connexion ou l'authentification), événements `event.imap-pre-connect`, `-pre-login`, `-post-login`, idem `smtp` et `sieve`. — `Acc:463-515, 551-594, 621-648`
- **F-BE-57 : Chargement du domaine à la volée par plugin** — [I] Hook `filter.domain`. — `RainLoop/Providers/Domain.php` (hook) ; `RainLoop/Providers/Domain/DomainInterface.php`
- **F-BE-58 : Nom d'EHLO SMTP** — [I] Déduit de `SERVER_NAME`/`HTTP_HOST`/`gethostname()`, IPv4 entre crochets. — `SMTP:125-146`

---

## 3. Sécurité et limites

- **F-BE-59 : Protection CSRF** — [A] `security.csrf_protection` (défaut oui) : jeton `XToken` obligatoire sur les POST AJAX. — `SA:131-136` ; `U:315-318` ; `CfgApp:213`
- **F-BE-60 : En-têtes de sécurité HTTP** — [A] `security.custom_server_signature` (défaut `RainLoop`), `x_frame_options_header` (vide), `x_xss_protection_header` (défaut `1; mode=block`), `content_security_policy` (balise `<meta>`). — `S:38-54, 292-308` ; `CfgApp:216-218, 230`
- **F-BE-61 : HTTPS forcé** — [A] `labs.force_https` : redirection si non sécurisé (détecte `X-Forwarded-Proto`/`X-Forwarded-Ssl`). — `S:56-60` ; `MailSo/Base/Http.php:287-308` ; `CfgApp:462`
- **F-BE-62 : Cookies : chemin et drapeau Secure** — [A] `labs.cookie_default_path`, `labs.cookie_default_secure` ; tous les cookies `httponly`. — `RainLoop/Api.php:114-118` ; `U:494-514`
- **F-BE-63 : CSP pour contenus bruts** — [I] Réponses `Raw` (pièces jointes, aperçus) envoyées avec `script-src 'none'` et un `child-src` limité à Google (docs/apis). — `SA:508-509`
- **F-BE-64 : IP cliente derrière proxy** — [A] `labs.http_client_ip_check_proxy` (lecture de `HTTP_CLIENT_IP`/`X-Forwarded-For`) pour les journaux. — `MailSo/Base/Http.php:358-377` ; `CfgApp:468`
- **F-BE-65 : Masquage de l'en-tête X-Mailer** — [A] `security.hide_x_mailer_header` (défaut non). — `A:5824-5829` ; `CfgApp:227`
- **F-BE-66 : Restriction d'installation du noyau/paquets** — [A] `security.core_install_access_domain` : le domaine autorisé à installer/mettre à jour (`*` pour tous). — `A:4260-4266` ; `CfgApp:231`
- **F-BE-67 : Matrice de capacités (`capa`) globale** — [A] Interrupteurs : `folders` (oui), `composer` (oui), `contacts` (oui), `settings` (oui), `quota` (oui), `help` (oui), `reload` (oui), `search` (oui), `search_adv` (oui), `filters` (oui), `x-templates` (**non**), `dangerous_actions` (oui), `message_actions` (oui), `messagelist_actions` (oui), `attachments_actions` (oui). Dépendances : `dangerous_actions` n'a d'effet que si `folders` + `messagelist_actions`. — `CfgApp:242-258` ; `A:8117-8260`
- **F-BE-68 : Capacités dérivées d'autres sections** — [A] `allow_additional_accounts`, `allow_additional_identities`, `allow_themes`, `allow_user_background`, `security.openpgp`, `labs.allow_gravatar`, `labs.allow_prefetch`, `interface.show_attachment_thumbnail`, `security.allow_two_factor_auth`. Ces interrupteurs sont refusés côté serveur (retour `False`) et pas seulement masqués. — `A:8117-8260, 2894, 3263`
- **F-BE-69 : Limite de taille de pièce jointe** — [A] `webmail.attachment_size_limit` (défaut 25 Mo, 0 = illimité) vérifiée à l'upload ; contacts 5 Mo, fond d'écran 1 Mo. — `SA:343-379, 438-449` ; `CfgApp:174-176`
- **F-BE-70 : Détection de taille d'upload PHP** — [A] Le panneau admin reçoit `upload_max_filesize` et `post_max_size`. — `A:1805-1808`
- **F-BE-71 : Validation des noms de fichiers** — [I] `MainClearFileName` : espaces, points multiples, longueur ≤ 250, caractères interdits, UTF-8 nettoyé ; `ClearXss` sur le nom d'affichage. — `A:7400-7412, 9987-9988` ; `MSU:1410-1432`
- **F-BE-72 : Liens signés/chiffrés pour les ressources** — [I] Chaque pièce jointe porte un « Download » = jeton chiffré contenant version, hash du compte, dossier, UID, index MIME, type, nom ; un jeton d'un autre compte ou d'une autre version ne fonctionne pas. — `A:9779-9786, 10005-10014`
- **F-BE-73 : Plafond de longueur des collections d'adresses** — [I] 100 adresses max par collection renvoyée au client. — `A:10093-10096`
- **F-BE-74 : Protection contre les images de suivi** — [A] `labs.try_to_detect_hidden_images` (défaut non) : repère les pixels espions (balises 1×1, `display:none`, hôtes `email.microsoftemail.com/open`, `github.com/notifications/beacon/`, `mandrillapp.com/track/open`, `list-manage.com/track/open`). — `HU:1121-1171` ; `A:9864` ; `CfgApp:418`
- **F-BE-75 : Journalisation fine** — [A] `logs.enable`, `write_on_error_only`, `write_on_php_error_only`, `write_on_timeout_only` (secondes), `hide_passwords` (défaut oui), `time_offset`, `session_filter` (ne journaliser qu'une session portant un cookie donné), `filename` (motifs `{date}`, `{user:...}`, `syslog`), `sentry_dsn` (envoi des erreurs JS à Sentry). — `A:1076-1164` ; `S:292-299` ; `CfgApp:306-353`
- **F-BE-76 : Pilotes de journal** — [I] Fichier, Syslog, Inline, Callback. — `MailSo/Log/Drivers/*.php`
- **F-BE-77 : Mots secrets censurés dans les journaux** — [I] Mot de passe, jetons CRAM/PLAIN ajoutés au registre de secrets du logger. — `A:2295, 2428` ; `SA:149-157` ; `IC:226-293`
- **F-BE-78 : Mode debug** — [A] `debug.enable` : affiche les erreurs PHP, ajoute les types de log TIME, active `Sleep` de test. — `S:32-36` ; `A:1127-1131, 2430-2437`
- **F-BE-79 : Échecs de connexion IMAP : détail des alertes serveur** — [A] `labs.imap_show_login_alert` (défaut oui) : le texte `[ALERT]` du serveur est renvoyé à l'utilisateur. — `A:2155-2160` ; `CfgApp:439`
- **F-BE-80 : Détail des erreurs SMTP à l'utilisateur** — [A] `labs.smtp_show_server_errors` (défaut non). — `A:6256-6281`
- **F-BE-81 : Pages d'erreur dédiées** — [I] `/?/NoScript`, `/?/NoCookie`, `/?/BadBrowser`, page « Accès refusé » du panel admin désactivé. — `SA:861-889, 1211-1234` ; `S:131-138`
- **F-BE-82 : Dossier de données inaccessible** — [I] Test d'écriture de `data/` à chaque chargement et message d'erreur. — `S:196-204`
- **F-BE-83 : Liste de 300 mots de passe courants** — [I] Contrôle d'un nouveau mot de passe contre une liste embarquée (recherche de sous-chaîne). — `MSU:2510-2520`
- **F-BE-84 : Suppression des données d'un utilisateur (API)** — [A] `Api::ClearUserData(email)` supprime le stockage de l'utilisateur et son carnet d'adresses. — `RainLoop/Api.php:232-254`
- **F-BE-85 : Ping de maintenance** — [I] `/?/Ping` (texte « Pong »), action AJAX `Ping`/`Noop` (maintient la connexion IMAP), `Version` (comparaison de version client/serveur pour forcer le rechargement). — `SA:915-922` ; `A:5019-5031, 5085-5089`

---

## 4. Téléchargements, pièces jointes, aperçus et proxy

Toutes les routes passent par `/?/Raw/&q[]=/<jeton>/<Action>/<clé>/` (`SA:493-563`) ; l'action `Raw<Nom>` est cherchée dans `Actions`.

- **F-BE-86 : Téléchargement d'une pièce jointe (`Raw/Download`)** — [U] En-têtes `Content-Disposition: attachment`, nom RFC 2231, transcodage depuis Content-Transfer-Encoding en flux (pas de chargement complet en mémoire). — `A:8695-8697, 8468-8690`
- **F-BE-87 : Affichage en ligne (`Raw/View`)** — [U] `inline`, type MIME et nom conservés. — `A:8703-8705`
- **F-BE-88 : Support des requêtes HTTP Range** — [U] Réponse 206 + `Content-Range` (lecture en mémoire) pour lecture audio/vidéo partielle ; `Accept-Ranges: bytes`. — `A:8473-8484, 8628-8675`
- **F-BE-89 : Vignettes (`Raw/ViewThumbnail`)** — [U] Miniatures 60×60 PNG générées avec la bibliothèque Imagine/GD (formats png/gif/jpeg) ; option capa `ATTACHMENT_THUMBNAILS` (`interface.show_attachment_thumbnail` défaut oui). — `A:8571-8584, 8807-8839` ; `CfgApp:180`
- **F-BE-90 : Correction d'orientation EXIF** — [A] `labs.detect_image_exif_orientation` (défaut oui) : rotation/miroir de JPEG/PNG avant affichage. — `A:8389-8460, 8591-8609` ; `CfgApp:474`
- **F-BE-91 : Cache navigateur par ETag et en-têtes** — [A] ETag dérivé de la clé + `cache.index`, `304 Not Modified`, `Expires` `cache.http_expires` (défaut 3600 s) si `cache.http` (défaut oui). — `A:8280-8330` ; `CfgApp:392-393`
- **F-BE-92 : Téléchargement d'un message complet en `.eml`** — [U] Le « RequestHash » d'un message est un lien Raw avec type `message/rfc822`, nom = sujet + `.eml`. — `A:9779-9786` ; `MC:467-551`
- **F-BE-93 : Affichage de la source brute (`Raw/ViewAsPlain`)** — [U] Flux `text/plain` d'une partie MIME ou de l'en-tête complet. — `A:8004-8023`
- **F-BE-94 : Téléchargement groupé en ZIP** — [U] Action `AttachmentsActions` `Do=zip` : copie temporaire des pièces, création `attachments.zip` (extension PHP `ZipArchive`), nettoyage. — `A:2980-3084` ; `A:1436-1441`
- **F-BE-95 : Enregistrement des pièces jointes dans OwnCloud** — [U] `Do=owncloud`, nom unique via `SmartFileExists`. — `A:3086-3147`
- **F-BE-96 : Envoi des pièces vers Dropbox** — [U] `Do=dropbox` : URL + jeton court, hash par fichier ; nécessite `social.dropbox_enable` + `dropbox_api_key`. — `A:3149-3165, 1448-1450`
- **F-BE-97 : Aperçu de documents Office via Google Docs Viewer** — [U] Pour `doc, docx, xls, xlsx, ppt, pptx` : page iframe vers `docs.google.com/viewer?embedded=true&url=...` avec jeton court. Suppose un serveur joignable depuis Internet. — `A:8028-8068, 8796-8800`
- **F-BE-98 : Proxy d'images externes (`/?/ProxyExternal/<clé>`)** — [A] `labs.use_local_proxy_for_external_images` (défaut non) : le serveur télécharge l'image distante (types png/jpeg/jpg/bmp/gif), jeton lié à l'utilisateur, ce qui masque l'IP du lecteur à l'expéditeur. Les liens sont posés dans `data-x-additional-src`. — `SA:454-488` ; `A:9847-9867, 9867` ; `CfgApp:473`
- **F-BE-99 : Blocage par défaut des images distantes** — [U] Les `src` externes sont mis dans `data-x-src` et le message est marqué `HasExternals` ; l'option utilisateur `ShowImages` (défaut `defaults.show_images` = non) débloque. — `HU:1173-1220` ; `A:1836, 9882` ; `CfgApp:299`
- **F-BE-100 : Avatars (`Raw/Avatar`)** — [U] Gravatar (`http://gravatar.com/avatar/<md5>.jpg?s=80&d=404`, via proxy `labs.curl_proxy`), repli sur une icône par domaine (`app/resources/images/services/<domaine>.png`), sinon image vide. Désactivable : `labs.allow_gravatar`. — `A:8844-8897` ; `CfgApp:405`
- **F-BE-101 : Upload de fichiers (`/?/Upload`)** — [U] Dépôt temporaire par compte (nom `upload-post-<md5>`), codes d'erreur typés (taille, partiel, aucun fichier, dossier temporaire, type, enregistrement). — `A:7616-7710` ; `SA:343-433`
- **F-BE-102 : Joindre une pièce d'un message existant (transfert)** — [U] `MessageUploadAttachments` : copie des parties MIME choisies vers le dépôt temporaire. — `A:7417-7470`
- **F-BE-103 : Joindre depuis une URL externe** — [U] `ComposeUploadExternals` : téléchargement HTTP (timeout 60 s, proxy sortant `labs.curl_proxy`/`curl_proxy_auth`). — `A:7475-7510`
- **F-BE-104 : Joindre depuis Google Drive** — [U] `ComposeUploadDrive` avec jeton d'accès et clé API. — `A:7515-7556`
- **F-BE-105 : Fond d'écran utilisateur** — [U] Upload png/jpg 1 Mo, restitution via `Raw/UserBackground`, effacement. Activé par `webmail.allow_user_background` (défaut non). — `A:7715-7838, 8719-8750` ; `CfgApp:163`
- **F-BE-106 : Fichiers publics (`Raw/Public`)** — [I] Servir des images stockées en zone « nobody » (png/jpeg). — `A:8755-8789` ; `RainLoop/KeyPathHelper.php:PublicFile`
- **F-BE-107 : Export de contacts (`Raw/ContactsVcf`, `Raw/ContactsCsv`)** — [U] vCard et CSV. — `A:8902-8933`
- **F-BE-108 : Import de contacts CSV/vCard** — [U] Détection par le début du fichier, séparateur `,` ou `;`, en-têtes obligatoires (>3 colonnes) pour le CSV. — `A:7843-8000`
- **F-BE-109 : Dépôt d'un .eml dans un dossier (`/?/Append`)** — [A] `labs.allow_message_append` (défaut non) : import d'un message brut par `APPEND`. — `A:8075-8108` ; `SA:299-335` ; `CfgApp:411`
- **F-BE-110 : Détection d'une pièce jointe (côté serveur)** — [I] Une partie est « pièce jointe » si `Content-Disposition: attachment` **ou** si son type n'est ni `multipart/*`, ni `text/plain`, ni `text/html`. — `MailSo/Imap/BodyStructure.php:272-286`
- **F-BE-111 : Pièces « liées » (CID / Content-Location)** — [I] Parties référencées par `cid:` ou `Content-Location` dans le HTML marquées `IsLinked`/`IsInline` pour ne pas les lister comme pièces à télécharger. — `A:9962-9996` ; `HU:1178-1186`
- **F-BE-112 : Type simplifié d'une pièce** — [I] image / archive / doc / pdf d'après le type MIME ou l'extension (zip/7z/rar). — `MSU:1213-1272` ; `MailSo/Mime/Attachment.php:161-190`
- **F-BE-113 : Nom par défaut des parties sans nom** — [I] `message/rfc822` → `.eml`, `text/calendar` → `calendar<index>.ics`, sinon nom dérivé du type. — `MailSo/Mail/Attachment.php:88-110`
- **F-BE-114 : `winmail.dat` reconnu comme TNEF** — [I] Seulement une association de type MIME `application/ms-tnef` ; **aucun décodeur TNEF**. — `MSU:1031-1033` ; `MailSo/Mime/Enumerations/MimeType.php:33`
- **F-BE-115 : Table de types MIME par extension** — [I] ~150 extensions (ics/ifb → `text/calendar`, p7c/p7m/p7s, doc/odt, etc.). — `MSU:1025-1200`
- **F-BE-116 : Aucune route « imprimer »** — [I] Il n'existe pas d'action d'impression côté serveur ; l'impression est une fonction de l'interface. (Aucun `RawPrint`/`ServicePrint` dans `Actions`/`ServiceActions`.)
- **F-BE-117 : Fichiers temporaires par compte et purge** — [I] Dépôt `storage/files` par compte, GC toutes les 25 h (suppression des fichiers > 48 h) déclenché au démarrage de l'application. — `RainLoop/Providers/Files/FileStorage.php` ; `A:3561-3632`

---

## 5. Protocoles IMAP / SMTP / Sieve

### IMAP (client maison `MailSo\Imap`)
Capacités **utilisées** : STARTTLS, AUTH=PLAIN/CRAM-MD5/XOAUTH2, NAMESPACE, LIST/LSUB, LIST-STATUS, STATUS (+HIGHESTMODSEQ si CONDSTORE), SORT, THREAD (REFS/REFERENCES/ORDEREDSUBJECT), MOVE, UIDPLUS (UID EXPUNGE, APPENDUID), UNSELECT, QUOTA, X-GM-EXT-1 (recherche X-GM-RAW). **Absentes** (aucune occurrence dans le code) : IDLE, COMPRESS, ID, ACL, METADATA, BINARY, UTF8=ACCEPT, QRESYNC, ENABLE, LITERAL+, SPECIAL-USE (RFC 6154 non demandée explicitement), MULTIAPPEND, PIPELINING SMTP, 8BITMIME, SMTPUTF8, CHUNKING/BDAT.

- **F-BE-118 : NAMESPACE** — [I] Lu au démarrage de la liste de dossiers (espace personnel) et utilisé comme préfixe pour la création des dossiers système. — `IC:464-502` ; `MC:2398-2403` ; `A:5301-5302`
- **F-BE-119 : Liste des dossiers (LIST/LSUB, LIST-STATUS)** — [U] Abonnés ou tous (`labs.use_imap_list_subscribe` défaut oui), état MESSAGES/UNSEEN/UIDNEXT en une commande si LIST-STATUS. — `IC:800-900` ; `MC:2322-2420` ; `CfgApp:425`
- **F-BE-120 : Optimisation des très grandes arborescences** — [A] `labs.imap_folder_list_limit` (défaut 200) : réduction en priorité aux dossiers système (inbox, sent, drafts, junk, trash, archive, starred, important…), puis abonnés, puis noms « simples ». — `MC:2186-2320` ; `A:5280-5283` ; `CfgApp:438`
- **F-BE-121 : Ignorer l'abonnement IMAP** — [A] `labs.ignore_folders_subscription`. — `CfgApp:402`
- **F-BE-122 : Types de dossiers (rôles)** — [U] Déduits des attributs (`\Sent`, `\Drafts`, `\Junk`, `\Trash`, `\Archive`, `\All`, `\Flagged`/`\Starred`, `\Important`) puis, à défaut, par une table de noms courants (Sent Items, Deleted Items, Bulk Mail, All Mail, Brouillons… en anglais uniquement), étendue par le hook `filter.system-folders-names`. — `MailSo/Mail/Folder.php:273-312` ; `A:5124-5264`
- **F-BE-123 : Création automatique des dossiers système** — [A] `labs.autocreate_system_folders` (défaut oui) : crée Sent/Drafts/Spam/Trash/Archive s'ils manquent, selon le premier nom connu. — `A:5297-5399` ; `CfgApp:410`
- **F-BE-124 : Dossiers système choisis par l'utilisateur** — [U] `SentFolder`, `DraftFolder`, `SpamFolder`, `TrashFolder`, `ArchiveFolder`, `NullFolder` (poubelle « sans dossier ») stockés en réglages locaux. — `A:3637-3652, 1864-1869`
- **F-BE-125 : Création/renommage/suppression/abonnement de dossiers** — [U] Avec délimiteur détecté, conversion UTF-7 IMAP, abonnement automatique à la création, refus de suppression d'un dossier non vide (erreur dédiée). — `A:5415-5585` ; `MC:2422-2665` ; `MailSo/Mail/Exceptions` (NonEmptyFolder)
- **F-BE-126 : Vider un dossier** — [U] `FolderClear` (supprime tous les messages du dossier). — `A:5592-5608` ; `MC:2621-2645`
- **F-BE-127 : Dossiers « à surveiller » (checkable)** — [U] Choix de dossiers dont le compteur est rafraîchi en tâche de fond (`FolderInformationMultiply`). — `A:5477-5522, 5674-5708`
- **F-BE-128 : Informations de dossier légères et détection de nouveaux messages** — [U] `FolderInformation` : STATUS, hash de dossier, `UIDNEXT` précédent → liste des nouveaux messages non lus de l'INBOX (expéditeur + sujet) ; flags d'une liste d'UID. Désactivable `labs.check_new_messages` (défaut oui). — `A:5615-5667` ; `MC:812-942` ; `CfgApp:477`
- **F-BE-129 : Hash de dossier pour le cache** — [I] `md5(nom, nombre, UIDNEXT, HIGHESTMODSEQ, filtre permanent, client)` ; le client IMAP utilise ce hash comme validateur de cache ETag. — `MC:778-803`
- **F-BE-130 : Liste de messages paginée** — [U] Offset/limite (≤999), tri par date inversée, `UNDELETED` systématique (`MessageListUndeletedOnly`), nouveaux messages. — `A:5715-5796` ; `MC:1908-2150`
- **F-BE-131 : Tri serveur SORT** — [A] `labs.use_imap_sort` (défaut oui) ; `ESEARCH/ESORT` implémentés dans le client mais **non appelés** par `MailClient`. — `MC:1819-1906` ; `IC:1180-1420` ; `CfgApp:423`
- **F-BE-132 : Conversations (THREAD)** — [U] `THREAD=REFS|REFERENCES|ORDEREDSUBJECT` (le premier annoncé), liste de fils, ouverture d'un fil (`ThreadUid`), limite `labs.imap_large_thread_limit` (défaut 50), cache de la carte des fils. `labs.use_imap_thread` (défaut oui) et préférence utilisateur `UseThreads` (défaut `defaults.mail_use_threads` = non). — `MC:1487-1724, 2082-2145` ; `A:5771-5775, 1913` ; `CfgApp:301, 426`
- **F-BE-133 : Grammaire de recherche utilisateur** — [U] Mots-clés `from:`, `to:` (To+Cc), `email:`/`mail:` (From/To/Cc), `subject:`, `text:`/`body:`, `has:attachment|file|attachments…`, `is:flagged|starred|pinned`/`unflagged`, `is:unread|unseen`/`read|seen`, `size:`/`larger:`/`bigger:`/`minsize:`, `smaller:`/`maxsize:` (unités K/M/G), `date:AAAA.MM.JJ` ou `AAAA.MM.JJ/AAAA.MM.JJ` ; guillemets simples ou doubles ; texte libre = `OR FROM TO CC SUBJECT` (rapide, `labs.imap_message_list_fast_simple_search` défaut oui) ou `TEXT`. — `MC:1059-1454` ; `CfgApp:432`
- **F-BE-134 : « A des pièces jointes » sans extension serveur** — [I] Heuristique `HEADER Content-Type application/ | multipart/m | multipart/signed | multipart/report` (non fiable), ou `X-GM-RAW has:attachment` sur Gmail. — `MC:1286-1306`
- **F-BE-135 : Recherche Gmail native** — [I] Si `X-GM-EXT-1` : texte libre → `X-GM-RAW`. — `MC:1209, 1396-1412`
- **F-BE-136 : Recherche non-ASCII** — [I] Littéraux IMAP et `CHARSET UTF-8`. — `MC:997-1003, 1853`
- **F-BE-137 : Mise en cache des UID de recherche/tri** — [A] `cache.server_uids` (défaut oui) par dossier/critères, invalidé par le hash du dossier. — `MC:1819-1906` ; `A:9616-9621` ; `CfgApp:395`
- **F-BE-138 : Bornes et filtres globaux de liste** — [A] `labs.imap_message_list_date_filter` (mois, 0 = tous), `labs.imap_message_list_permanent_filter` (critère IMAP ajouté en permanence ; indicateur « liste filtrée » côté UI), `labs.imap_message_list_count_limit_trigger` (au-delà de N messages, désactive tri/threads et liste par numéro de séquence). — `MC:1197-1201, 1442-1445, 1946-1964, 2056-2096` ; `RainLoop/Api.php:86-99` ; `CfgApp:433-435`
- **F-BE-139 : Lecture d'un message** — [U] BODYSTRUCTURE, puis en-têtes choisis ou **tous** (`labs.imap_message_all_headers`, défaut non), parties texte `BODY.PEEK` avec limite `labs.imap_body_text_limit` (défaut 555000 octets ; drapeau `TextPartIsTrimmed`), parties de signature PGP. — `MC:367-452` ; `A:7197-7243` ; `CfgApp:431, 436`
- **F-BE-140 : En-têtes extraits** — [I] Return-Path, Received, Message-ID, From/To/Cc/Bcc/Sender/Reply-To/Delivered-To, In-Reply-To, References, Date, Subject, Sensitivity, X-MSMail-Priority/Importance/X-Priority, X-Draft-Info, Return-Receipt-To, Disposition-Notification-To, X-Confirm-Reading-To, Authentication-Results, X-DKIM-Authentication-Results, List-Unsubscribe. — `MC:163-222`
- **F-BE-141 : Priorité, sensibilité, accusés** — [U] Priorité haute/basse (mapping de 6 variantes), sensibilité Personal/Private/Company-Confidential, accusé de lecture demandé (2 en-têtes), accusé de réception. — `MailSo/Mail/Message.php:644-701`
- **F-BE-142 : Liens de désinscription (List-Unsubscribe)** — [U] Extraits et exposés (`UnsubsribeLinks`). — `MailSo/Mail/Message.php:703-718` ; `A:9764`
- **F-BE-143 : Statut DKIM/Authentication-Results** — [U] Lit `dkim=pass|fail…` (champ `header.d/i/from`) ou `X-DKIM-Authentication-Results` ; le statut est attaché à l'expéditeur dont le domaine correspond (`DkimStatus`, `DkimValue`). Pas de vérification cryptographique locale, ni SPF/DMARC dédiés. — `MailSo/Mime/HeaderCollection.php:362-466` ; `A:9915-9922`
- **F-BE-144 : PGP en ligne détecté (texte)** — [U] Détecte `-----BEGIN PGP SIGNATURE-----` et `-----BEGIN PGP MESSAGE-----` dans le texte brut ; le traitement OpenPGP est côté navigateur (`security.openpgp`). Les signatures MIME `application/pgp-signature` sont lues mais **S/MIME (`pkcs7`) n'est pas traité**. — `MailSo/Mail/Message.php:839-851` ; `MC:397-437` ; `A:9876-9878`
- **F-BE-145 : Texte formaté (format=flowed)** — [I] `DecodeFlowedFormat`, désactivé si l'encodage est base64/QP. — `MailSo/Mail/Message.php:820-823` ; `MailSo/Imap/BodyStructure.php:291-302`
- **F-BE-146 : Sujet découpé (Re:/Fwd:)** — [U] `SubjectParts` : préfixes `RE`, `FWD`, `RE[n]`, `FWD(n)`. — `A:9637-9678`
- **F-BE-147 : Date du message** — [A] Date interne IMAP (INTERNALDATE) par défaut ; `labs.date_from_headers` (défaut oui) préfère l'en-tête `Date`. — `A:9737-9745` ; `CfgApp:409`
- **F-BE-148 : Drapeaux gérés** — [U] `\Seen`, `\Flagged`, `\Answered`, `\Deleted`, `\Draft`, `\Recent`, plus deux drapeaux personnalisés : transfert `$Forwarded` (`labs.imap_forwarded_flag`) et accusé de lecture `$ReadReceipt` (`labs.imap_read_receipt_flag`) ; vérification `PERMANENTFLAGS` avant `STORE`. — `A:5632-5653, 9795-9799` ; `IC:910-935` ; `MailSo/Imap/FolderInformation.php:106` ; `CfgApp:429-430`
- **F-BE-149 : Marquer lu / non lu, suivi, tout marquer lu** — [U] Par UID, ou tout le dossier (ou un fil). — `A:7130-7190` ; `MC:236-352`
- **F-BE-150 : Suppression** — [U] `\Deleted` + `EXPUNGE` ; `UID EXPUNGE` si UIDPLUS ; `labs.use_imap_expunge_all_on_delete` (défaut non) ; `labs.use_imap_unselect` (défaut oui côté code) après opération. — `A:7250-7293` ; `MC:566-588` ; `IC:1682-1708`
- **F-BE-151 : Déplacement/copie** — [U] `MOVE` natif si `labs.use_imap_move` (défaut non dans la config, oui dans le code) sinon COPY + DELETE ; option « marquer comme lu en déplaçant ». — `A:7300-7390` ; `MC:604-655` ; `CfgApp:427`
- **F-BE-152 : Sélection forcée à l'EXAMINE** — [A] `labs.use_imap_force_selection`. — `A:8959` ; `CfgApp:424`
- **F-BE-153 : Écriture dans un dossier (APPEND) avec UID retourné** — [I] Flux de message, drapeaux, date, lecture de `APPENDUID`, repli par recherche du `Message-ID`. — `MC:682-720` ; `IC:1737-1780` ; `A:6097-6107`
- **F-BE-154 : Quota IMAP** — [U] `GETQUOTAROOT INBOX` → stockage (usage/limite en Ko) et nombre de messages ; désactivable (`capa.quota`). — `IC:1123-1170` ; `A:6556-6575`
- **F-BE-155 : Gmail : nombre de messages corrigé** — [I] Détection `imap.gmail.com` pour compter les messages réellement présents dans INBOX / `[Gmail]/…`. — `MC:760-772, 986-989`
- **F-BE-156 : Étiquettes Gmail (constantes)** — [I] `X-GM-MSGID`, `X-GM-THRID`, `X-GM-LABELS` et actions STORE de labels définis mais pas exploités par `MailClient`. — `MailSo/Imap/Enumerations/FetchType.php:39-41` ; `StoreAction.php:28-33`
- **F-BE-157 : Délais IMAP/SMTP/Sieve** — [A] `labs.imap_timeout` (300 s), `labs.smtp_timeout` (60 s), `labs.sieve_timeout` (10 s), connexion SMTP 10 s. — `RainLoop/Api.php:104-105` ; `A:6166` ; `Sieve:62` ; `CfgApp:449-451`
- **F-BE-158 : Fixation des moteurs de décodage** — [A] `labs.disable_iconv_if_mbstring_supported`. — `RainLoop/Api.php:80-84` ; `CfgApp:412`
- **F-BE-159 : Client IMAP non pooling** — [I] Une connexion IMAP par requête AJAX (connexion + login à chaque appel), fermée en fin de requête (`BootEnd`). Coût important par requête ; pas d'IDLE ni de push. — `A:8938-8961, 394-404`

### SMTP
- **F-BE-160 : Envoi SMTP** — [U] `MAIL FROM`, `RCPT TO` (destinataires To+Cc+Bcc), `DATA` en flux, `RSET`, `VRFY`, `NOOP`, `QUIT`. — `A:6138-6287` ; `SMTP:361-600`
- **F-BE-161 : DSN (accusé de remise)** — [U] Demande `RET=HDRS` et `NOTIFY=SUCCESS,FAILURE` si l'option « DSN » est cochée et le serveur l'annonce. — `A:6307` ; `SMTP:361-415`
- **F-BE-162 : SIZE** — [I] La taille maximale annoncée est lue ; `SIZE=` envoyé si fournie (RainLoop ne la fournit pas dans `MailFrom` à l'appel). — `SMTP:365-372, 720-740`
- **F-BE-163 : Envoi par `mail()` PHP** — [A] Domaine avec `smtp_php_mail` : en-têtes To/Subject nettoyés (`labs.mail_func_clear_headers`, défaut oui), option `-f` expéditeur (`labs.mail_func_additional_parameters`, défaut non). — `A:6172-6221` ; `CfgApp:453-454`
- **F-BE-164 : Destinataires cachés (plugin)** — [I] Hook `filter.smtp-hidden-rcpt` (copie cachée), `filter.smtp-from` (enveloppe), `filter.message-rcpt`, `filter.smtp-message-stream`. — `A:6144-6160`
- **F-BE-165 : Copie dans « Envoyés » après envoi** — [U] Flux ré-lu et ajouté par APPEND ; si Cci, le message est reconstruit **avec** les en-têtes Bcc pour le dossier Envoyés. — `A:6361-6406`
- **F-BE-166 : Marquage « répondu » / « transféré » après envoi** — [U] `\Answered` pour reply/reply-all ; drapeau `$Forwarded` pour forward. — `A:6330-6358`
- **F-BE-167 : Suppression du brouillon et des fichiers temporaires à l'envoi** — [U] — `A:6413-6425`
- **F-BE-168 : Fréquence d'usage des destinataires** — [U] `IncFrec` incrémente les destinataires To ; création automatique de contact selon `ContactsAutosave` (défaut `defaults.contacts_autosave` = oui). — `A:6445-6474` ; `PdoAddressBook.php:1560`
- **F-BE-169 : Accusés de lecture (envoi)** — [U] Construit un message (From identité du compte, Reply-To, sujet/texte fournis par le client) et pose `$ReadReceipt` ; mémorisation en cache pour ne plus redemander. — `A:6002-6052, 6482-6551`
- **F-BE-170 : Sauvegarde de brouillon** — [U] `SaveMessage` : APPEND avec `\Seen` dans le dossier Brouillons, suppression de l'ancienne version, retourne le nouvel UID ; brouillon automatique contrôlé par `defaults.allow_draft_autosave` (défaut oui) et option utilisateur. — `A:6057-6125` ; `CfgApp:302`
- **F-BE-171 : Informations de réponse/transfert dans l'en-tête X-Draft-Info** — [I] `type, uid, folder` en base64 pour retrouver le message d'origine. — `MailSo/Mail/Message.php:720-749` ; `MailSo/Mime/Message.php:449`

### Sieve
- **F-BE-172 : Filtres (règles) visuels** — [U] Conditions : From, Recipient (To/Cc), Subject, Header (nom libre), Size ; opérateurs : =, ≠, contient, ne contient pas, regex, plus grand, plus petit ; conjonction tous/au moins un. Actions : aucune (stop), déplacer, supprimer (discard), répondeur (vacation : message, objet, jours, adresses), rejeter (reject), rediriger (avec option « garder une copie » → `fileinto "INBOX"`), marquer comme lu (`addflag \Seen`). Les règles sont stockées dans le script avec un en-tête base64 JSON (`# RAINLOOP:SIEVE`). — `Sieve:162-545` ; `RainLoop/Providers/Filters/Classes/*`
- **F-BE-173 : Extensions Sieve détectées** — [I] `fileinto`, `regex`, `relational`, `date`, `reject`, `vacation`, `imap4flags` ; la liste des modules du serveur est lue via `CAPABILITY` et expose quelles actions sont possibles (`Modules`). `require` généré automatiquement (regex, imap4flags, fileinto, vacation, reject). — `Sieve:97-106, 505-509`
- **F-BE-174 : Script brut éditable** — [A] Si le domaine autorise `sieve_allow_raw` : fichier `rainloop.user.raw` actif au lieu du script visuel. — `Sieve:51-160`
- **F-BE-175 : Noms de scripts et dossier cible** — [I] Scripts `rainloop.user` / `rainloop.user.raw` ; dossier cible converti UTF-7→UTF-8 (`labs.sieve_utf8_folder_name`, défaut oui) ; `labs.sieve_allow_fileinto_inbox` (défaut non, exposé au client). — `Sieve:9-10, 445-453` ; `A:1544` ; `CfgApp:446, 448`
- **F-BE-176 : Hooks filtres** — [I] `filter.filters-fake` (court-circuite la lecture), `filter.filters-save`. — `A:2832-2836, 2877-2879`
- **F-BE-177 : Client ManageSieve** — [I] LISTSCRIPTS, GETSCRIPT, PUTSCRIPT, CHECKSCRIPT, SETACTIVE, DELETESCRIPT, CAPABILITY, STARTTLS. Pas de HAVESPACE/RENAMESCRIPT/NOOP automatisé. — `MailSo/Sieve/ManageSieveClient.php:114-453`

---

## 6. MIME, encodages et nettoyage HTML

- **F-BE-178 : Construction du message sortant** — [U] `multipart/alternative` (texte brut + HTML), `multipart/related` (images liées CID), `multipart/mixed` (pièces), parties en base64 en flux, `Content-Disposition` inline/attachment avec nom RFC 2231, `Content-ID`, `Content-Location`. En-têtes par défaut : Date (`gmdate('r')`), Message-ID (`<md5@hôte>`), X-Mailer, MIME-Version. Frontières `--=_RainLoop_nnn_...`. — `MailSo/Mime/Message.php:510-940`
- **F-BE-179 : Texte brut généré depuis le HTML** — [I] `ConvertHtmlToPlain` (titres, listes, liens `texte (url)`, tableaux, entités) pour la partie alternative ; hook `filter.message-plain`. — `HU:1512-1617` ; `A:5909-5914`
- **F-BE-180 : Texte → HTML avec citations** — [I] `ConvertPlainToHtml` : liens détectés (`LinkFinder`), lignes `>` en `<blockquote>`, tabulations et doubles espaces préservés. — `HU:1425-1505` ; `MailSo/Base/LinkFinder.php`
- **F-BE-181 : Images en ligne (data: URL → pièces CID)** — [U] À l'envoi, les images `data:image/...;base64` du HTML deviennent des pièces jointes `cid:` (hash MD5) ; les `data-x-src-cid` repartent en `cid:`. — `HU:1284-1417` ; `A:5942-5968`
- **F-BE-182 : Nettoyage du HTML entrant** — [I] Suppression des balises `svg, link, base, meta, title, x-script, script, bgsound, keygen, source, object, embed, applet, mocha, iframe, frame, frameset, video, audio, area, map, head, style` et des commentaires ; `iframe/frame` neutralisés ; attributs interdits (`on*`, `id`, `class`, `contenteditable`, `formaction`, `manifest`, `action`, `xmlns`, `srcset`, `data-bind`…) ; `href` non http(s)/ftp/skype/mailto → `javascript:false` (valeur d'origine gardée dans `data-x-broken-href`) ; `rel="external nofollow noopener noreferrer"` et `target="_blank"` sur les liens ; `tabindex=-1`. — `HU:368-409, 957-1111, 1173-1220`
- **F-BE-183 : Liste blanche stricte optionnelle** — [A] `labs.strict_html_parser` (défaut non dans `application.ini`, mais le code lit la valeur avec défaut `true` — comportement dépendant de la présence de la clé) : seuls ~80 attributs autorisés ; liste de balises (`HtmlStrictAllowedTags`) paramétrable en code. — `RainLoop/Api.php:142-184` ; `CfgApp:480`
- **F-BE-184 : Nettoyage des styles CSS** — [I] Retire `behavior`, `pointer-events`, `expression()`, `display:none`, `text-indent` négatif, `cursor` exotiques ; `position:fixed` → `absolute` ; `background`/`list-style`/`content` avec `url()` : `cid:` conservé, `http(s)` externe déplacé dans `data-x-style-url`, autre → bloqué. — `HU:647-772`
- **F-BE-185 : Attributs HTML hérités convertis** — [I] `bgcolor`, `background`, `link`, `text`, `topmargin`… de `<body>` convertis en styles ; couleur de lien `link` appliquée aux `<a>`. — `HU:967-1048, 1222-1246`
- **F-BE-186 : Détection des liens dans le HTML** — [I] `FindLinksInDOM` (autolink de texte brut dans le HTML reçu). — `HU:777-864`
- **F-BE-187 : `<pre>` conservé** — [I] Retours à la ligne des blocs `<pre>` remplacés par `<br />`. — `A:9858-9860`
- **F-BE-188 : Encodages de transfert** — [I] base64, quoted-printable, 7bit/8bit, uuencode non géré ; décodage en flux (`StreamWrappers\Binary`) ; encodage d'en-têtes (`B`/`Q`, RFC 2047) et paramètres RFC 2231 (continuations, `charset'lang'`). — `MSU:620-960` ; `MailSo/Base/StreamWrappers/Binary.php`
- **F-BE-189 : Jeux de caractères** — [I] Normalisation d'alias (`NormalizeCharset`), conversion par iconv ou mbstring (translit/ignore), détection automatique `CharsetDetect` (mbstring), nettoyage UTF-8 (`Utf8Clear`), UTF-7 IMAP ↔ UTF-8 pour les noms de dossiers. — `MSU:127-530, 1635-1672, 1927-2220, 2311-2332`
- **F-BE-190 : IDN (noms de domaine internationaux)** — [I] Bibliothèque IDNA2 embarquée (`Vendors/Net/IDNA2.php`), conversions sur e-mails, hôtes, domaines. — `MSU:2410-2478`
- **F-BE-191 : Analyse d'adresses** — [I] Parseur d'e-mail avec noms affichés, guillemets, commentaires, plusieurs adresses ; `Unique()` ; conversion IDN. — `MailSo/Mime/Email.php:83-215` ; `MailSo/Mime/EmailCollection.php`
- **F-BE-192 : Détection RTL** — [I] `IsRTL` pour texte de droite à gauche. — `MSU:1675-1688`
- **F-BE-193 : Parseur MIME complet en mémoire/flux** — [I] `Part::ParseFromFile/String/Stream` (arborescence, en-têtes, frontières). Sert aux tests et aux flux ; l'affichage s'appuie sur `BODYSTRUCTURE` IMAP, pas sur ce parseur. — `MailSo/Mime/Part.php:261-640`
- **F-BE-194 : Extraction de texte des pièces** — [I] **Absente** : pas d'indexation ni d'extraction de texte (PDF/Office), la recherche est entièrement déléguée au serveur IMAP. (Aucun code correspondant dans `MailSo`.)
- **F-BE-195 : Exportation de pièce vers aperçu texte de type « calendar »** — [I] `text/calendar` reçoit un nom `.ics` mais **aucune interprétation d'invitation** (pas de lecture d'événement, pas d'accept/refus). — `MailSo/Mail/Attachment.php:97-100`

---

## 7. Options de configuration globales (`application.ini`), par thème

Chaque ligne : `section.option` = défaut — commentaire du fichier source. Les valeurs ci-dessous sont les défauts de `CfgApp:151-490`. Les sections « labs » regroupent des réglages avancés non supportés officiellement (libellé source : « Experimental settings. Handle with care. »). Le fichier peut être complété par un fichier additionnel (`APP_ADDITIONAL_CONFIGURATION_NAME`) et certaines valeurs peuvent référencer des variables d'environnement (`labs.replace_env_in_configuration`, liste de clés) — `CfgApp:12-93`.

### 7.1 Interface, branding, identité du service
| Option | Défaut | Commentaire / rôle |
|---|---|---|
| webmail.title | RainLoop Webmail | Titre de page |
| webmail.loading_description | RainLoop | Texte au démarrage |
| webmail.favicon_url | (vide) | Favicon/icône tactile personnalisée |
| webmail.theme | Default | Thème par défaut |
| webmail.allow_themes | true | Choix du thème en paramètres (aussi capacité THEMES) |
| webmail.allow_user_background | false | Fond d'écran utilisateur |
| webmail.language | en | Langue par défaut |
| webmail.language_admin | en | Langue du panneau admin |
| webmail.allow_languages_on_settings | true | Choix de langue dans les paramètres |
| login.allow_languages_on_login | true | Choix de langue sur l'écran de connexion |
| login.determine_user_language | true | Langue détectée via `Accept-Language` |
| webmail.messages_per_page | 20 | Messages par page (choix utilisateur : 10, 20, 30, 50, 100, 150, 200, 300) |
| interface.show_attachment_thumbnail | true | Vignettes de pièces jointes |
| interface.use_native_scrollbars | false | Barres de défilement natives |
| interface.new_move_to_folder_button | true | Bouton « déplacer vers » |
| branding.login_logo, login_background, login_desc, login_css | (vides) | Personnalisation de l'écran de connexion |
| branding.user_css, user_logo, user_logo_title, user_logo_message, user_iframe_message | (vides) | Personnalisation de l'interface connectée ; tokens `{user:...}` autorisés dans `user_iframe_message` |
| branding.welcome_page_url | (vide) | Page d'accueil (iframe) |
| branding.welcome_page_display | none | `none`, `once` (une fois par URL), `always` |
| login.welcome_page | false | Écran d'accueil de connexion |
| login.hide_submit_button | false | Masquer le bouton de connexion |
| login.forgot_password_link_url / registration_link_url | (vides) | Liens externes affichés sur l'écran de connexion |
| labs.custom_login_link / custom_logout_link | (vides) | Redirection personnalisée connexion / déconnexion |
| labs.startup_url | (vide) | Page ouverte après connexion (tokens `{user:...}`) |
| labs.in_iframe | false | Mode intégré dans une iframe |
| labs.allow_mobile_version | true | Version mobile |
| labs.use_mobile_version_for_tablets | false | Tablettes → version mobile |
| labs.favicon_status | true | Compteur dans l'icône d'onglet |
| labs.allow_html_editor_source_button | false | Bouton « source » dans l'éditeur |
| labs.allow_html_editor_biti_buttons | false | Boutons avancés de l'éditeur |
| labs.allow_ctrl_enter_on_compose | true | Ctrl+Entrée envoie |
| labs.use_app_debug_js / use_app_debug_css | false | Versions non minifiées |
| labs.use_material_design | (non défini → vrai) | Lu en code, absent du fichier |

### 7.2 Valeurs par défaut des préférences utilisateur (`defaults`)
| Option | Défaut | Commentaire |
|---|---|---|
| defaults.view_editor_type | Html | `Plain`, `Html`, `HtmlForced`, `PlainForced` |
| defaults.view_layout | 1 | 0 sans aperçu, 1 aperçu latéral, 2 aperçu en bas |
| defaults.view_use_checkboxes | true | Cases à cocher dans la liste |
| defaults.autologout | 30 | Déconnexion automatique (minutes) — désactivé sous OwnCloud |
| defaults.show_images | false | Charger les images distantes |
| defaults.contacts_autosave | true | Créer automatiquement les contacts |
| defaults.mail_use_threads | false | Conversations |
| defaults.allow_draft_autosave | true | Brouillon automatique |
| defaults.mail_reply_same_folder | false | Réponse enregistrée dans le même dossier |

Préférences réellement stockées par utilisateur côté serveur (`DoSettingsUpdate`) : Language, Theme, MPP, Layout, EditorDefaultType, ShowImages, ContactsAutosave, DesktopNotifications, SoundNotification, UseCheckboxesInList, AllowDraftAutosave, AutoLogout, EnableTwoFactor, UseThreads, ReplySameFolder, + dossiers système, dossiers « checkable », fond d'écran, LastWelcomePage, noms sociaux. — `A:4951-5014, 1858-1921`

### 7.3 Contacts
| Option | Défaut | Commentaire |
|---|---|---|
| contacts.enable | false | Active le carnet d'adresses |
| contacts.allow_sync | false | Synchronisation CardDAV |
| contacts.sync_interval | 20 | Minutes |
| contacts.type | sqlite | `sqlite`, `mysql`, `pgsql` |
| contacts.pdo_dsn / pdo_user / pdo_password | `mysql:host=127.0.0.1;port=3306;dbname=rainloop` / root / (vide) | Connexion PDO (mot de passe masqué `APP_DUMMY` dans l'admin) |
| contacts.suggestions_limit | 30 | Nombre max de suggestions (plancher 5) |

### 7.4 Sécurité
| Option | Défaut | Commentaire |
|---|---|---|
| security.csrf_protection | true | Protection CSRF |
| security.custom_server_signature | RainLoop | En-tête `Server` |
| security.x_frame_options_header | (vide) | |
| security.x_xss_protection_header | `1; mode=block` | |
| security.openpgp | false | Autorise OpenPGP côté navigateur |
| security.admin_login / admin_password | admin / 12345 | « Login and password for web admin panel » |
| security.allow_admin_panel | true | « Access settings » |
| security.allow_two_factor_auth | false | |
| security.force_two_factor_auth | false | |
| security.hide_x_mailer_header | false | |
| security.admin_panel_host | (vide) | Hôte dédié à l'admin |
| security.admin_panel_key | admin | Segment d'URL |
| security.content_security_policy | (vide) | |
| security.core_install_access_domain | (vide) | Domaine autorisé à installer/mettre à jour |
| ssl.verify_certificate | false | « Require verification of SSL certificate used » |
| ssl.allow_self_signed | true | Exige verify_certificate |
| ssl.cafile / capath / client_cert | (vides) | Fichier CA, dossier CA haché, certificat client PEM |

### 7.5 Connexion
| Option | Défaut | Commentaire |
|---|---|---|
| login.default_domain | (vide) | |
| login.determine_user_domain | false | Détection par l'hôte HTTP |
| login.login_lowercase | true | |
| login.sign_me_auto | DefaultOff | `DefaultOff`, `DefaultOn`, `Unused` |

### 7.6 Plugins, réseaux sociaux
| Option | Défaut |
|---|---|
| plugins.enable | false |
| plugins.enabled_list | (vide, noms séparés par virgules) |
| social.google_enable / google_enable_auth / google_enable_auth_gmail / google_enable_drive / google_enable_preview | false |
| social.google_client_id / google_client_secret / google_api_key | (vides) |
| social.fb_enable, fb_app_id, fb_app_secret | false / (vides) |
| social.twitter_enable, twitter_consumer_key, twitter_consumer_secret | false / (vides) |
| social.dropbox_enable, dropbox_api_key | false / (vide) |

### 7.7 Journaux
| Option | Défaut | Commentaire |
|---|---|---|
| logs.enable | false | |
| logs.write_on_error_only | false | « Logs entire request only if error occured (php requred) » |
| logs.write_on_php_error_only | false | |
| logs.write_on_timeout_only | 0 | Secondes |
| logs.hide_passwords | true | « Required for development purposes only. Disabling this option is not recommended. » |
| logs.time_offset | 0 | |
| logs.session_filter | (vide) | `cookie:valeur` |
| logs.sentry_dsn | (vide) | |
| logs.filename | `log-{date:Y-m-d}.txt` | Motifs documentés dans la source (`{date}`, `{user:email|login|domain|uid|ip}`, `{request:ip}`, `{imap:...}`, `{smtp:...}`) |
| logs.auth_logging | false | « Enable auth logging in a separate file (for fail2ban) » |
| logs.auth_logging_filename / auth_logging_format | `fail2ban/auth-{date:Y-m-d}.txt` / `[{date:Y-m-d H:i:s}] Auth failed: ip={request:ip} user={imap:login} host={imap:host} port={imap:port}` | |
| debug.enable | false | « Special option required for development purposes » |

### 7.8 Cache et performances
| Option | Défaut | Commentaire |
|---|---|---|
| cache.enable | true | « Enables caching in the system » |
| cache.index | v1 | Clé additionnelle ; si modifiée, cache purgé |
| cache.fast_cache_driver | files | `files`, `APC`, `memcache`, `redis (beta)` |
| cache.fast_cache_index | v1 | |
| cache.http | true | Cache navigateur |
| cache.http_expires | 3600 | Secondes |
| cache.server_uids | true | Cache des UID de recherche/tri |
| labs.fast_cache_memcache_host/port | 127.0.0.1 / 11211 | |
| labs.fast_cache_redis_host/port | 127.0.0.1 / 6379 | |
| labs.cache_system_data | true | Cache des gabarits, langues, CSS, JS de plugins |

### 7.9 IMAP / SMTP / Sieve (`labs`)
| Option | Défaut |
|---|---|
| use_imap_sort | true |
| use_imap_force_selection | false |
| use_imap_list_subscribe | true |
| use_imap_thread | true |
| use_imap_move | false |
| use_imap_expunge_all_on_delete | false |
| use_imap_unselect (code seulement) | true |
| imap_forwarded_flag | `$Forwarded` |
| imap_read_receipt_flag | `$ReadReceipt` |
| imap_body_text_limit | 555000 |
| imap_message_list_fast_simple_search | true |
| imap_message_list_count_limit_trigger | 0 |
| imap_message_list_date_filter | 0 (mois) |
| imap_message_list_permanent_filter | (vide) |
| imap_message_all_headers | false |
| imap_large_thread_limit | 50 |
| imap_folder_list_limit | 200 |
| imap_show_login_alert | true |
| imap_use_auth_plain | true |
| imap_use_auth_cram_md5 | false |
| smtp_show_server_errors | false |
| smtp_use_auth_plain | true |
| smtp_use_auth_cram_md5 | false |
| sieve_allow_raw_script | false (non lu : la valeur est par domaine) |
| sieve_utf8_folder_name | true |
| sieve_auth_plain_initial | true |
| sieve_allow_fileinto_inbox | false |
| imap_timeout / smtp_timeout / sieve_timeout | 300 / 60 / 10 |
| mail_func_clear_headers | true |
| mail_func_additional_parameters | false |
| folders_spec_limit | 50 (exposé au client) |
| autocreate_system_folders | true |
| ignore_folders_subscription | false |
| allow_message_append | false |
| check_new_messages | true |
| date_from_headers | true |
| disable_iconv_if_mbstring_supported | false |
| domain_list_limit | 99 |

### 7.10 Divers (`labs`)
`allow_gravatar` true ; `allow_prefetch` true ; `allow_smart_html_links` true (lu côté client) ; `try_to_detect_hidden_images` false ; `hide_dangerous_actions` false ; `use_local_proxy_for_external_images` false ; `detect_image_exif_orientation` true ; `curl_proxy`, `curl_proxy_auth` (proxy sortant HTTP) vides ; `force_https` false ; `allow_external_login` false ; `allow_external_sso` false ; `external_sso_key` vide ; `http_client_ip_check_proxy` false ; `cookie_default_path` vide ; `cookie_default_secure` false ; `replace_env_in_configuration` vide ; `strict_html_parser` false ; `allow_cmd` false (interface de commande) ; `update_channel` stable ; `check_new_password_strength` true ; `owncloud_save_folder` Attachments ; `owncloud_suggestions` true ; `log_ajax_response_write_limit` 300 ; `dev_email`, `dev_password` vides (pré-remplissage du formulaire en développement) ; `login_fault_delay` 1.

### 7.11 Version
`version.current`, `version.saved` : mis à jour à chaque sauvegarde. — `CfgApp:137-146`

- **F-BE-196 : Fichier additionnel de configuration** — [A] `APP_ADDITIONAL_CONFIGURATION_NAME` fusionné ; clés non reconnues réécrites ; en-tête « Please don't add custom parameters here, those will be overwritten ». — `CfgApp:12-18`
- **F-BE-197 : Substitution de variables d'environnement** — [A] `labs.replace_env_in_configuration` (liste `section.option`) : les valeurs contenant `$NOM` sont remplacées par `$_SERVER`/`$_ENV`. — `CfgApp:28-93`
- **F-BE-198 : Sauvegarde de configuration depuis l'admin** — [A] Une seule action `AdminSettingsUpdate` écrit dans `application.ini` : langue, thème, TLS, proxy d'images, capacités, domaine par défaut, contacts, sécurité, social, branding. — `A:3781-3883`

---

## 8. Plugins et points d'extension

Les plugins vivent dans `plugins/<nom-en-minuscules>/index.php` (classe `<Nom>Plugin` héritant `AbstractPlugin`), activés par `plugins.enable` + `plugins.enabled_list`. Un fichier `VERSION` et une configuration typée (types : string, int, texte, mot de passe, sélection, booléen) sont gérés. — `RainLoop/Plugins/Manager.php:70-200` ; `RainLoop/Plugins/Property.php` ; `RainLoop/Enumerations/PluginPropertyType.php`

- **F-BE-199 : API plugin (`AbstractPlugin`)** — [I] `addHook`, `addJs` (admin ou utilisateur), `addTemplate`, `replaceTemplate`, `addPartHook` (nouvelle route `/?/<Nom>`), `addAjaxHook` (nouvelle action AJAX), `addTemplateHook` (injecter du HTML dans un gabarit), `UseLangs` (fichiers de langue du plugin), `FilterAppDataPluginSection`, `getUserSettings`/`saveUserSettings` (réglages par utilisateur), `ajaxResponse`, `ajaxParam`, `Supported()` (refus de chargement). — `RainLoop/Plugins/AbstractPlugin.php:47-436`
- **F-BE-200 : Hooks « filtre » (modifier des données)** — [I] `filter.application-config`, `filter.app-data`, `filter.app-data[2]`, `filter.http-query`, `filter.http-paths`, `filter.fabrica`, `filter.acount`, `filter.login-credentials(.step-1/.step-2)`, `filter.imap-credentials`, `filter.smtp-credentials`, `filter.sieve-credentials`, `filter.domain`, `filter.action-params`, `filter.ajax-response`, `filter.upload-response`, `filter.response-object`, `filter.result-message(+[2])`, `filter.folders-before`, `filter.folders-post`, `filter.folders-complete`, `filter.folders-system-types`, `filter.system-folders-names`, `filter.build-message(+[2])`, `filter.message-html`, `filter.message-plain`, `filter.save-message(+[2])`, `filter.send-message(+[2])`, `filter.sent-message-stream`, `filter.smtp-message-stream`, `filter.smtp-from`, `filter.smtp-hidden-rcpt`, `filter.message-rcpt`, `filter.read-receipt-message-plain`, `filter.build-read-receipt-message`, `filter.send-read-receipt-message`, `filter.filters-fake`, `filter.filters-save`. — `grep RunHook` dans `RainLoop/*.php` ; p.ex. `A:5274-5407, 5906-5971, 6077-6079, 6311-6313, 6372-6389`
- **F-BE-201 : Hooks « événement » (observer)** — [I] `api.bootstrap.plugins`, `event.login-pre-login-provide`, `event.login-post-login-provide`, `event.imap-pre-connect/pre-login/post-login`, `event.smtp-pre-connect/post-connect/pre-login/post-login`, `event.sieve-pre-connect/post-connect/pre-login/post-login`, `ajax.action-pre-call`, `ajax.action-post-call`, `ajax.suggestions-input-parameters/-pre/-post`, `service.app-delay-start-begin/-end`, `main.default-response-data`, `main.default-response`, `main.default-response-error-data`. — `A:2307-2320, 7061-7116, 3563-3629` ; `SA:173-181`
- **F-BE-202 : Fabrication de fournisseurs (`main.fabrica`)** — [I] Un plugin peut fournir : `files`, `storage`, `storage-local`, `settings`, `settings-local`, `login`, `domain`, `filters`, `address-book`, `suggestions` (liste), `change-password` (**aucun fournisseur par défaut**), `two-factor-auth`. Les interfaces correspondantes : `RainLoop/Providers/*/*Interface.php`. — `A:294-389`
- **F-BE-203 : Pilotes de suggestions d'adresses** — [U] Carnet interne + pilotes plugin (`ISuggestions::Process`), limite `contacts.suggestions_limit`, dédoublonnage. — `A:7052-7125` ; `RainLoop/Providers/Suggestions/*`
- **F-BE-204 : Routes et actions additionnelles** — [I] `AddAdditionalPartAction` (route HTTP libre, ex. `/?/MonPlugin`), `AddAdditionalAjaxAction` (action AJAX), `AddProcessTemplateAction` (injection dans un gabarit à un endroit nommé). — `RainLoop/Plugins/Manager.php:451-583, 503-534` ; `S:153-156` ; `SA:177-182`
- **F-BE-205 : JS/CSS/gabarits de plugin** — [I] JS concaténés et servis via `/?/Plugins/0/(Admin|User)/<hash>/` (cache), gabarits ajoutés à la compilation, CSS : fonction vide (non implémentée). — `RainLoop/Plugins/Manager.php:230-295` ; `SA:655-690`
- **F-BE-206 : Réglages de plugin** — [A] Édition dans l'admin (`AdminPluginLoad`, `AdminPluginSettingsUpdate`), exposition au JS selon `AllowedInJs`. — `A:4840-4946` ; `RainLoop/Plugins/Manager.php:304-348`
- **F-BE-207 : Données utilisateur de plugin** — [I] `GetUserPluginSettings` / `SaveUserPluginSettings` (stockage par utilisateur). — `RainLoop/Plugins/Manager.php:595-660`
- **F-BE-208 : Traductions de plugin** — [I] `ReadLang` fusionne les fichiers du plugin dans le dictionnaire (utilisateur et admin). — `RainLoop/Plugins/Manager.php:661-688`
- **F-BE-209 : Sandbox absent** — [I] Un plugin est du code PHP exécuté avec les mêmes droits que l'application ; aucun cloisonnement. Noms limités à `[a-z0-9-]`.. — `RainLoop/Plugins/Manager.php:123-135`
- **F-BE-210 : Hooks bas niveau MailSo** — [I] `\MailSo\Hooks::Add/Run` (un seul point utilisé : `Net.NetClient.StreamContextSettings/Filter`, pour injecter `cafile`/`capath`). — `MailSo/Hooks.php` ; `MailSo/Net/NetClient.php:273` ; `RainLoop/Api.php:122-135`

---

## 9. Administration (uniquement les réglages ayant un équivalent utilisateur ou déploiement)

Liste non détaillée des actions du panneau admin (`/?admin` ou hôte dédié) :
- **F-BE-211 : Réglages généraux** — [A] AdminSettingsUpdate (cf. F-BE-198) ; équivalents utilisateur : langue, thème, fond d'écran, pièces jointes, capacités. — `A:3781-3883`
- **F-BE-212 : Domaines** — [A] Load/List/Save/Delete/Disable/AliasSave/Test (cf. F-BE-47…51). — `A:4017-4235`
- **F-BE-213 : Test de la base de contacts** — [A] `AdminContactsTest` (création des tables, message d'erreur). — `A:3935-3955` ; `PdoAddressBook.php:1724`
- **F-BE-214 : Mot de passe/identifiant admin** — [A] `AdminPasswordUpdate` (exige l'ancien mot de passe). — `A:3978-4012`
- **F-BE-215 : Paquets et plugins** — [A] `AdminPackagesList`, `AdminPackageInstall` (téléchargement ZIP depuis un dépôt, extraction PclZip, refus si déjà installé), `AdminPackageDelete`, `AdminPluginList`, `AdminPluginDisable` (activer/désactiver, contrôle de support), `AdminPluginLoad`, `AdminPluginSettingsUpdate`. Dépôt distant `APP_REPOSITORY_PATH`, mis en cache 1 h, filtré par version minimale (`required`) et dépréciée (`depricated`). — `A:4268-4360, 4496-4946`
- **F-BE-216 : Mise à jour du noyau** — [A] `AdminCoreData` (version distante, canal `labs.update_channel` : `stable` ou `beta`, avertissements de version), `AdminUpdateCoreData` (téléchargement ZIP, extraction dans `rainloop/v/<version>`, remplacement de `index.php`, invalidation OPcache/APC). Conditions : `index.php` et `rainloop/` inscriptibles. Ancien code : au-delà de 5 versions conservées, la plus ancienne est purgée. — `A:4251-4258, 4364-4425, 4514-4592` ; `RainLoop/Providers/Version.php`
- **F-BE-217 : Licence** — [A] `AdminLicensing` et `AdminLicensingActivate` lèvent toujours `LicensingServerIsUnavailable` : fonctions de licence vidées dans cette version. Codes de notification existants : `LicensingServerIsUnavailable`, `LicensingExpired`, `LicensingBanned`. — `A:3960-3973` ; `RainLoop/Notifications.php:60-62`
- **F-BE-218 : Vérification de signature PGP des paquets** — [I] `PgpVerifyFile` est un squelette (« TODO », retourne vrai si fichier et clé présents) ; `FastCheck` de clés de licence (`RL<n>-...-<crc>`). — `U:34-44, 446-458`
- **F-BE-219 : Informations serveur** — [A] `/?/Info` (phpinfo) réservé à l'admin connecté ; état des pilotes PDO (mysql, sqlite, pgsql) ; domaine du serveur. — `SA:927-936` ; `A:1757-1808`
- **F-BE-220 : Nettoyage périodique** — [I] Au démarrage de l'application : GC du cache (>48 h, toutes les 23 h), des fichiers temporaires (toutes les 25 h), des anciennes versions (toutes les 30 h). — `A:3561-3632`
- **F-BE-221 : Pas de gestion des utilisateurs** — [A] Aucune liste d'utilisateurs, aucun quota/limite par utilisateur, aucune révocation de session ; seule `Api::ClearUserData` (code d'intégration) supprime les données. — `RainLoop/Api.php:232-254`

---

## 10. Divers, services HTTP et stockage

- **F-BE-222 : Liste des routes de service** — [I] `/?/Ajax`, `/?/Raw`, `/?/Upload`, `/?/UploadContacts`, `/?/UploadBackground`, `/?/Append`, `/?/ProxyExternal`, `/?/Lang`, `/?/Templates`, `/?/Plugins`, `/?/Css`, `/?/AppData`, `/?/AdminAppData`, `/?/MobileVersion`, `/?/DesktopVersion`, `/?/NoScript`, `/?/NoCookie`, `/?/BadBrowser`, `/?/Mailto`, `/?/Ping`, `/?/Info`, `/?/Sso`, `/?/RemoteAutoLogin`, `/?/ExternalLogin`, `/?/ExternalSso`, `/?/Change`, `/?/OwnCloudAuth`, `/?/SocialGoogle|Facebook|Twitter`. **Aucune route `/?/Mobile`, `/?/Print` ni `/?/Download` propre** : les téléchargements passent par `Raw`. — `SA:114-1230` ; `S:141-157`
- **F-BE-223 : Version mobile** — [U] Détection via la bibliothèque MobileDetect, cookie `rlmobiletype` (`mobile` / `desktop` forcés), thème « Mobile » unique, mise en page sans aperçu, désactivation des notifications et fonds. — `S:159-186` ; `SA:841-856` ; `A:1982-1992`
- **F-BE-224 : Thèmes (LESS compilé côté serveur)** — [U] Thèmes dans `themes/<nom>` ; thèmes personnalisés dans `<racine>/themes/<nom>` suffixés `@custom` ; compilation `lessc` en CSS compressé, cache par plugins/thème/version. — `SA:695-795` ; `A:9025-9044, 9130-9215`
- **F-BE-225 : Langues** — [U] Fichiers YAML par langue (interface et admin), repli de nom (`en` → `en_US`, `zh`→`zh_CN`, `no`→`nb_NO`, `ua`→`uk_UA`…), dictionnaires compilés en JS + locale Moment.js, cache. — `A:9052-9120, 9219-9262` ; `SA:568-610, 1323-1382`
- **F-BE-226 : Mode noyau de gabarits compilés** — [I] Gabarits HTML concaténés dans `Index.html`, minification sommaire, cache avec hash de plugins/version. — `S:206-229` ; `SA:1295-1316`
- **F-BE-227 : Stockage utilisateur** — [I] `StorageProvider` : types `USER`(1), `CONFIG`(2), `NOBODY`(3) ; fichiers par utilisateur (`storage/<e-mail>/...`) ; `storage-local` distinct (options locales). Réglages utilisateur via `SettingsProvider` (JSON). — `RainLoop/Providers/Storage/FileStorage.php` ; `RainLoop/Providers/Settings/DefaultSettings.php`
- **F-BE-228 : Cache** — [I] Pilotes fichier, APC/APCu, Memcache(d), Redis (Predis) ; verrous, minuteries, GC. — `A:992-1057` ; `MailSo/Cache/Drivers/*`
- **F-BE-229 : Carnet d'adresses PDO** — [U] (existence seulement) Contacts, étiquettes, propriétés typées (nom, e-mail, téléphone, web, anniversaire, Facebook, Skype, GitHub, note, personnalisé), suggestions, fréquence d'usage, import/export CSV/vCard, synchronisation CardDAV (client DAV, proxy `curl_proxy`), partage possible. Bases : SQLite, MySQL, PostgreSQL. — `RainLoop/Providers/AddressBook/PdoAddressBook.php:31-1879` ; `RainLoop/Providers/AddressBook/Enumerations/PropertyType.php`
- **F-BE-230 : Synchronisation CardDAV utilisateur** — [U] Activer/désactiver, URL, utilisateur, mot de passe (stocké chiffré), déclenchement manuel et périodique (`contacts.sync_interval`). — `A:6577-6664`
- **F-BE-231 : Liste complète des codes d'erreur** — [I] `InvalidToken 101`, `AuthError 102`, `AccessError 103`, `ConnectionError 104`, `CaptchaError 105`, `SocialFacebook/Twitter/GoogleLoginAccessDisable 106-108`, `DomainNotAllowed 109`, `AccountNotAllowed 110`, `AccountTwoFactorAuthRequired 120`, `AccountTwoFactorAuthError 121`, `CouldNotSaveNewPassword 130`, `CurrentPasswordIncorrect 131`, `NewPasswordShort 132`, `NewPasswordWeak 133`, `NewPasswordForbidden 134`, `ContactsSyncError 140`, `CantGetMessageList 201`, `CantGetMessage 202`, `CantDeleteMessage 203`, `CantMoveMessage 204`, `CantCopyMessage 205`, `CantSaveMessage 301`, `CantSendMessage 302`, `InvalidRecipients 303`, `CantSaveFilters 351`, `CantGetFilters 352`, `FiltersAreNotCorrect 355`, `CantCreateFolder 400`, `CantRenameFolder 401`, `CantDeleteFolder 402`, `CantSubscribeFolder 403`, `CantUnsubscribeFolder 404`, `CantDeleteNonEmptyFolder 405`, `CantSaveSettings 501`, `CantSavePluginSettings 502`, `DomainAlreadyExists 601`, `CantInstallPackage 701`, `CantDeletePackage 702`, `InvalidPluginPackage 703`, `UnsupportedPluginPackage 704`, `Licensing* 710-712`, `DemoSendMessageError 750`, `DemoAccountError 751`, `AccountAlreadyExists 801`, `AccountDoesNotExist 802`, `MailServerError 901`, `ClientViewError 902` (message serveur relayé), `InvalidInputArgument 903`, `UnknownNotification 998`, `UnknownError 999`. Notez que `CaptchaError`, `DemoSendMessageError`/`DemoAccountError`, `NewPasswordForbidden`, `AccessError` ne sont jamais levés dans ce code (vestiges : captcha et mode démo retirés). — `RainLoop/Notifications.php:7-74`
- **F-BE-232 : Réponses AJAX normalisées** — [I] Enveloppe `{Action, Result, ErrorCode, ErrorMessage, ErrorMessageAdditional, Time, UpdateToken, Logout, ClearAuth}` ; sérialisation d'objets métier (messages, dossiers, contacts, pièces) dans `responseObject`. — `A:9284-9410, 9687-10124` ; `SA:114-250`
- **F-BE-233 : Limites de l'API AJAX** — [I] Pas de pagination côté carnet au-delà de `Offset/Limit` ; liste de 100 adresses max par champ ; liste de messages ≤ 999 ; `Folders` en erreur renvoie `ClearAuth`. — `A:6935-6963, 10093-10096` ; `SA:197-200` ; `MC:1913`
- **F-BE-234 : Journal AJAX borné** — [A] `labs.log_ajax_response_write_limit` (300 caractères par réponse journalisée). — `SA:244-247` ; `CfgApp:414`
- **F-BE-235 : Indicateur de capacités exposées au client** — [I] `AppData` fournit au JS : capacités, thèmes, langues, limites (pièce jointe en octets), liens personnalisés, drapeaux IMAP (`useImapThread`, `useImapSubscribe`), `folderSpecLimit`, `allowAppendMessage`, actions de pièces disponibles (`zip`, `owncloud`, `dropbox` selon extensions et réglages). — `A:1431-1490`
- **F-BE-236 : Réglages de sécurité du serveur exposés à l'admin** — [A] Drapeaux `VerifySslCertificate`, `AllowSelfSigned`, `WeakPassword`, `CoreAccess`, limites PHP. — `A:1757-1808`
- **F-BE-237 : Protection contre mot de passe admin faible au démarrage** — [A] Comparaison avec `12345` : le panneau signale l'état « mot de passe par défaut ». — `CfgApp:121, 1802`
- **F-BE-238 : Intégration Sentry (JS)** — [A] Chargement du script Sentry CDN et initialisation si `logs.sentry_dsn` est défini. — `S:292-299`
- **F-BE-239 : Notifications de bureau, son** — [U] Drapeaux `DesktopNotifications`, `SoundNotification` stockés par utilisateur (aucune logique serveur supplémentaire ; désactivés en mobile). — `A:1885-1886, 1986-1988`
- **F-BE-240 : Interface « ligne de commande »** — [A] `labs.allow_cmd` (défaut non) expose `allowCmdInterface` au client. — `A:1478`

---

# Résumé (10 lignes)

1. Le serveur RainLoop 1.17 est « sans état » : session = jeton XXTEA contenant le mot de passe IMAP, lié au navigateur par User-Agent + cookie ; pas de table de sessions ni révocation côté serveur.
2. Authentification riche pour un webmail autonome : 2FA TOTP + codes de secours + « faire confiance 14 j », sign-me, SSO à usage unique, REMOTE_USER, login/SSO externes, OAuth Google/Gmail (XOAUTH2), comptes multiples, identités, modèles.
3. Configuration IMAP/SMTP/Sieve **par domaine** (fichiers `.ini`, jokers, alias, désactivation, liste blanche, login court, `mail()` PHP) avec détection automatique du domaine par hôte HTTP ; c'est l'élément d'administration le plus structurant.
4. Sécurité : CSRF, en-têtes, CSP, HTTPS forcé, panneau admin à chemin/hôte secret ; mais anti-force-brute limité à un `sleep` + journal fail2ban ; aucun quota ni limitation par IP ni verrouillage de compte.
5. Pièces jointes/aperçus : téléchargement en flux avec Range, vignettes GD avec correction EXIF, ZIP, envoi vers OwnCloud/Dropbox, aperçu Office via Google Docs, proxy d'images externes avec jeton, avatars Gravatar ; pas de décodage TNEF, pas d'invitation iCal, pas de route d'impression.
6. IMAP : STARTTLS, PLAIN/CRAM-MD5/XOAUTH2/PROXYAUTH, NAMESPACE, LIST-STATUS, SORT, THREAD, MOVE, UIDPLUS, UNSELECT, QUOTA, CONDSTORE (status), X-GM-RAW ; **absents** : IDLE, COMPRESS, ID, ACL, METADATA, BINARY, QRESYNC, SCRAM, SPECIAL-USE explicite ; une connexion IMAP par requête AJAX.
7. SMTP : AUTH PLAIN/LOGIN/CRAM-MD5/XOAUTH2, DSN, SIZE (lue), pas de PIPELINING/8BITMIME/SMTPUTF8 ; Sieve : filtres visuels (From/To/Subject/Header/Size ; déplacer, supprimer, répondeur, rejeter, rediriger, marquer lu) et script brut par domaine.
8. HTML entrant assaini par liste noire de balises/attributs/CSS, blocage des images distantes (avec détection optionnelle des pixels espions) ; HTML sortant avec images `data:` converties en CID, texte alternatif généré automatiquement ; DKIM affiché via `Authentication-Results` (pas de vérification locale), PGP/S-MIME : détection en ligne seulement.
9. Extensibilité : ~45 hooks `filter.*` / `event.*`, fabrique de fournisseurs (stockage, domaines, filtres, carnet, suggestions, changement de mot de passe, 2FA), routes et actions AJAX de plugin ; administration : domaines, plugins/paquets, mise à jour du noyau (canaux stable/beta), licence neutralisée, journaux, ~200 options `application.ini`.
10. Fichier produit : `/tmp/claude-1000/-home-gdegoulet-Work-webmail-project/be75865a-1a74-4b90-b555-b7c7c12ae39b/scratchpad/audit_backend.md` (240 entrées F-BE-01 à F-BE-240).
