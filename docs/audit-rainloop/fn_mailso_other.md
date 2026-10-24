# MailSo (hors Imap/Net/Mail/Cache/Mime/Base/Vendors) : analyse fonction par fonction vs wmail-go

Sources lues : MailSo/Smtp/SmtpClient.php, Smtp/Exceptions, Sieve/ManageSieveClient.php, Sieve/Exceptions, Pop3/*, Poppassd/*, Log/*, MailSo.php, Config.php, Hooks.php, Version.php.
Les protocoles SMTP/ManageSieve sont faits par go-smtp / go-sasl / notre client `internal/sieve/client.go`. On ne signale que les comportements et fonctionnalités.
Contexte Go : `internal/mailconn/client.go` (DialSMTP l.227, LoginSMTP l.262, SendSMTP l.561), `internal/mailconn/sieve.go` (WithSieve), `internal/sieve/client.go`, `internal/web/filters.go`.

## 1. Smtp/SmtpClient.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (73) / NewInstance (93) | Initialise l'état (capacités, drapeaux bHelo/bMail/bRcpt/bData) | Plomberie | smtp.NewClient (client.go:234) | SANS OBJET | go-smtp gère l'état |
| IsSupported (101) | Capacité EHLO annoncée ? | Adapter le comportement aux extensions (SIZE, DSN, STARTTLS) | `c.Extension("DSN")` client.go:576 ; `Extension("AUTH")` client.go:270 | OK | Utilisé pour DSN et AUTH |
| IsAuthSupported (109) | Mécanisme AUTH annoncé ? | Choisir PLAIN/LOGIN/CRAM | liste des mécanismes client.go:276-281 | OK | |
| HasSupportedAuth (117) | PLAIN ou LOGIN dispo ? Sert à décider si le STARTTLS opportuniste est requis | Ne pas rester en clair si on doit s'authentifier | aucun repli en clair : SecuritySTARTTLS obligatoire client.go:25, 240-245 | OK (voulu : plus strict) | |
| EhloHelper (125) | Calcule le nom envoyé à EHLO : SERVER_NAME, sinon HTTP_HOST, sinon hostname, IP entre crochets | Un EHLO au nom réel du serveur évite le rejet par des MTA (reject_non_fqdn_helo, filtrage anti-spam) | `c.Hello("localhost")` en dur client.go:235, 252 ; `NewClientStartTLS` utilise aussi « localhost » par défaut (client.go:245) | PARTIEL | Ajouter `Server.EhloHost` (config `smtp_ehlo_host`, défaut = hostname de l'URL publique, sinon `os.Hostname()`) et passer par `smtp.Client.Hello(...)` / `NewClientStartTLS` puis Hello. P3 (la plupart des serveurs de soumission acceptent « localhost ») |
| Connect (162) | Connexion + lecture 220 + STARTTLS auto-détecté + EHLO/HELO | Connexion SMTP chiffrée | DialSMTP client.go:227-256 (TLS implicite, STARTTLS, aucun) | PARTIEL | Pas d'AUTO_DETECT (STARTTLS opportuniste si annoncé) : choix volontaire (jamais de repli en clair, client.go:25). Pas de repli HELO si EHLO échoue (go-smtp fait EHLO puis HELO seul : à vérifier, sans importance). P3 : rien à faire |
| Login (189) | AUTH CRAM-MD5 > PLAIN (mode une ligne) > LOGIN ; login passé en IDN->ASCII | S'authentifier sur SMTP avec les mécanismes courants | LoginSMTP client.go:262-291 : PLAIN puis LOGIN ; CRAM-MD5 absent ; pas d'IDN->ASCII sur le login | PARTIEL | (a) ajouter `sasl` CRAM-MD5 : go-sasl v0.0.0-2024 n'a que PLAIN/LOGIN/ANONYMOUS/OAUTHBEARER/EXTERNAL, donc écrire 25 lignes (HMAC-MD5) ; P3, car presque aucun serveur moderne n'impose CRAM-MD5 seul. (b) Login IDN : voir IdnToAscii dans « À AJOUTER » P3 |
| LoginWithXOauth2 (325) | AUTH XOAUTH2 avec jeton | Comptes Gmail/Outlook via OAuth | aucun (go-sasl a OAUTHBEARER, non utilisé) | SANS OBJET | L'appli ne gère pas OAuth (mot de passe d'application) ; le cahier des charges ne le demande pas |
| MailFrom (361) | MAIL FROM ; ajoute SIZE=n si annoncé, RET=HDRS si DSN ; adresse IDN->ASCII | Éviter l'envoi d'un message trop gros (rejet précoce 552) et DSN qui ne renvoie que les en-têtes | SendSMTP client.go:574,583 : `mailOpts` toujours nil ; DSN : seulement NOTIFY sur RCPT (client.go:577) | PARTIEL | Renseigner `smtp.MailOptions{Size: int64(len(raw)), Return: smtp.DSNReturnHeaders}` (si DSN) quand l'extension SIZE/DSN est annoncée. 4 lignes. P3 (SIZE : le serveur rejette de toute façon après DATA ; RET=HDRS : évite que les DSN d'échec ramènent tout le message) |
| Rcpt (397) | RCPT TO ; NOTIFY=SUCCESS,FAILURE si DSN ; message « Failed to add recipient » ; IDN->ASCII | Accusé de livraison (DSN) + signaler quel destinataire est refusé | client.go:576-577 (NOTIFY SUCCESS, FAILURE, DELAY) ; liste des refusés `SendError.Rejected` client.go:547-585 | OK | Mieux que RainLoop (liste précise des refus). IDN non traité, voir ci-dessous |
| MailTo (434) | Alias de Rcpt | | idem Rcpt | OK | |
| Data (447) | DATA depuis une chaîne ; refuse vide | Envoi du message | `c.Data()` client.go:593 | OK | |
| DataWithStream (471) | DATA en flux ; dot-stuffing ; remise à zéro du délai PHP | Envoi de gros messages | w.Write(raw) / Close client.go:597-606 (go-smtp fait le dot-stuffing) | OK | Message entièrement en mémoire (raw []byte) : acceptable, limite max_message_mb config.go:85 |
| Rset (531) | RSET | Annuler l'enveloppe après erreur | `c.Reset()` client.go:592 | OK | |
| Vrfy (548) | VRFY utilisateur | Vérifier une adresse (jamais utilisé par RainLoop côté UI) | `c.Verify` : aucun | SANS OBJET | VRFY désactivé presque partout, aucune fonction UI |
| Noop (564) | NOOP keepalive | Garder la session | aucun (connexion SMTP ouverte par envoi, une seule fois) | SANS OBJET | Pas de session SMTP longue |
| Logout (577) | QUIT | Fermeture propre | `c.Quit()` client.go:290, 608 | OK | |
| preLoginStartTLSAndEhloProcess (597) | EHLO, STARTTLS si annoncé/choisi, nouvel EHLO ; erreur « STARTTLS not supported » si exigé | Chiffrement | NewClientStartTLS client.go:245 (échoue si absent) ; classé KindTLS errors.go | OK | |
| sendRequest (636) | Envoi d'une commande, masquage des secrets dans le journal | Journal sans mot de passe | go-smtp : pas de journal de protocole ; aucun | SANS OBJET | Voir section Journal (débogage protocole) |
| sendRequestWithCheck (671) | Envoi + validation du code attendu | Plomberie | go-smtp | SANS OBJET | |
| ehloOrHelo (682) / ehlo (711) / helo (755) | EHLO, repli HELO, analyse des capacités AUTH et SIZE | Détecter capacités | go-smtp Extension() | SANS OBJET | go-smtp. La valeur SIZE annoncée (iSizeCapaValue) n'est pas utilisée par RainLoop ici non plus |
| validateResponse (771) | Lecture réponses multi-lignes, code attendu, exceptions Negative/Response | Remonter le texte d'erreur du serveur | `*smtp.SMTPError` classé errors.go:55,82-84 ; SendError client.go:550 | OK | Le texte serveur est affiché (shorten, errors.go:30) |
| getLogName (820) / SetLogger (832) | Nom/Logger | Plomberie | aucun | SANS OBJET | |

### Smtp/Exceptions
| Fonction (ligne) | But | Besoin | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| ResponseException::__construct (32), GetResponses (45), GetLastResponse (53) | Conserve les lignes de réponse SMTP pour affichage | Message d'erreur précis à l'utilisateur | `TestError.Detail` + SendError.Err (errors.go:25, client.go:550) | OK | |
| Exception, LoginBadMethodException, LoginBadCredentialsException, LoginException, NegativeResponseException, RuntimeException (classes vides, 19 lignes chacune) | Hiérarchie d'erreurs | Message « identifiants refusés » vs « méthode non gérée » | KindAuth / KindProtocol errors.go:19-22 ; authStage client.go:138 | OK | |

## 2. Sieve/ManageSieveClient.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (43) / NewInstance (58) | État initial | Plomberie | sieve.New client.go:57 | OK | |
| IsSupported (68) | Capacité annoncée ? | Savoir ce que le serveur offre | Capability sieve/client.go:77 | OK | |
| IsModuleSupported (78) | Extension Sieve (fileinto, vacation…) supportée ? | Griser les actions non supportées dans l'éditeur de filtres | HasExtension sieve/client.go:89 ; `Modules` map web/filters.go:58-62 | OK | |
| Modules (86) | Liste des extensions | Idem | Extensions() sieve/client.go:83, utilisé web/filters.go:61 | OK | |
| IsAuthSupported (96) | SASL annoncé et mécanisme présent | Choix PLAIN/LOGIN | test PLAIN seul sieve/client.go:277 | PARTIEL | Pas de repli LOGIN. P3 : ajouter `AUTHENTICATE "LOGIN"` si PLAIN absent (rare, Dovecot/Cyrus annoncent PLAIN) |
| Connect (114) | Connexion, lecture bannière, STARTTLS si annoncé/choisi | Connexion chiffrée au port 4190 | mailconn/sieve.go:21-41 (TLS implicite ou StartTLS sieve/client.go:252) | OK | Pas d'AUTO-DETECT : voulu |
| Login (157) | AUTHENTICATE PLAIN (initial response) ou LOGIN ; `sLoginAuthKey` = identité d'autorisation (authzid) | Auth ManageSieve avec le même login/mot de passe qu'IMAP | Authenticate sieve/client.go:273-290 (PLAIN, réponse initiale) | PARTIEL | Pas de LOGIN, pas d'authzid (utile seulement si login admin proxy : hors cas d'usage). P3 |
| Logout (241) | LOGOUT | Fermer | Logout sieve/client.go:69 | OK | |
| ListScripts (258) | LISTSCRIPTS -> nom => actif | Voir les scripts et le script actif | ListScripts sieve/client.go:299, utilisé web/filters.go:64,100 | OK | |
| Capability (288) | Commande CAPABILITY | Relire les capacités | readCapabilities sieve/client.go:211 (après STARTTLS) | OK | |
| Noop (304) | NOOP | Keepalive | aucun | SANS OBJET | Sessions Sieve courtes (WithSieve ouvre/ferme) |
| GetScript (319) | GETSCRIPT | Lire script pour l'éditeur | GetScript sieve/client.go:319 ; web/filters.go:71,81 | OK | |
| PutScript (353) | PUTSCRIPT (littéral non synchronisant) | Enregistrer script | PutScript sieve/client.go:347 ; web/filters.go:115,488 | OK | |
| CheckScript (369) | CHECKSCRIPT | Valider avant d'enregistrer (éditeur « script brut ») | CheckScript sieve/client.go:356 ; web/filters.go:485 | OK | Mais : les filtres structurés (saveSieveFilters web/filters.go:96-118) ne font pas CheckScript : PutScript valide déjà côté serveur, équivalent |
| SetActiveScript (385) | SETACTIVE | Activer un script, "" désactive | SetActive sieve/client.go:359 ; web/filters.go:107,118,494-498 | OK | |
| DeleteScript (400) | DELETESCRIPT | Supprimer | DeleteScript sieve/client.go:369 ; web/filters.go:111 | OK | |
| GetActiveScriptName (413) / IsActiveScript (438) | Nom du script actif, test d'activité | Savoir si les filtres sont actifs | champ `Active` de Script, lu web/filters.go:84 (`sc.Active`) | OK | |
| parseLine (447) / parseStartupResponse (486) | Analyse des lignes entre guillemets, capacités SASL et SIEVE | Plomberie | readLine/readCapabilities sieve/client.go:105,211 | SANS OBJET | Réimplémenté proprement (tokens) |
| sendRequest (523) / sendRequestWithCheck (546) / convertEndOfLine (557) / parseResponse (586) / validateResponse (621) | Envoi, littéraux {n}, réponse OK/NO | Plomberie + message d'erreur serveur | cmd/readResponse sieve/client.go:172,243 ; ServerError l.25-32 ; message affiché via sieveErrKey web/filters.go:121 | OK | |
| getLogName (635) / SetLogger (647) | Journal | Plomberie | aucun | SANS OBJET | |

### Sieve/Exceptions
| Fonction (ligne) | But | Besoin | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| ResponseException::__construct (32), GetResponses (45), GetLastResponse (53) | Garde les lignes de réponse NO pour afficher la raison (script invalide : ligne et erreur) | Afficher l'erreur Sieve du serveur dans l'éditeur | ServerError.Text sieve/client.go:25-32 ; renvoyée telle quelle mailconn/sieve.go:46-47 | OK | |
| 7 classes d'exception vides (Exception, Login*, Negative*, Runtime) | Hiérarchie | | ErrAuth / ErrNoStartTL / ServerError sieve/client.go:18-30 | OK | |

## 3. Pop3/ (Pop3Client.php + Exceptions)

Fonctions : __construct (43), NewInstance (56), Connect (74), Login (112), IsLoggined (146), Noop (157), Status (169), Capa (185), Logout (207), sendRequest (227), secureRequestParams (259), sendRequestWithCheck (285), validateResponse (297), readMultilineResponse (328), getLogName (356), SetLogger (367) ; exceptions : ResponseException (ctor, GetResponses, GetLastResponse) + 4 classes vides.

| Fonction (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| Toutes les fonctions du client POP3 (16 fonctions + 7 exceptions) | Client POP3 minimal (USER/PASS, STAT, CAPA, NOOP, QUIT). RainLoop ne s'en sert que pour tester une connexion POP3 / driver de relève externe (non utilisé en 1.17 pour la lecture) | Pas de besoin : wmail-go est IMAP seul (cahier des charges : IMAP + SMTP + Sieve) | aucun (grep POP3 : aucune occurrence dans internal) | SANS OBJET | POP3 hors périmètre |

## 4. Poppassd/ (PoppassdClient.php + Exceptions)

| Fonction (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (33) / NewInstance (44) / Connect (62) / Login (85) / Logout (121) / NewPass (140) / secureRequestParams (162) / sendRequest (188) / sendRequestWithCheck (226) / validateResponse (242) / getLogName (278) / SetLogger (290) | Protocole poppassd (port 106) : user, pass, newpass, quit. Sert au plugin « Changer le mot de passe de la boîte » | Changer le mot de passe du compte de messagerie depuis le webmail | aucun (grep passwd/chpass : seulement `store.ChangePassword` web/handlers.go:336 = mot de passe du webmail local, pas celui de la boîte IMAP) | MANQUANT (mais driver poppassd SANS OBJET) | Le protocole poppassd lui-même est SANS OBJET. Le besoin « changer le mot de passe de la boîte » n'est pas couvert ; pour un utilisateur unique il le fait ailleurs. Proposition P3 : si voulu, un champ « nouveau mot de passe IMAP » qui met à jour le coffre (vault) après changement fait ailleurs, rien côté serveur. Ne pas implémenter poppassd. |
| ResponseException (ctor, GetResponses, GetLastResponse) + 4 classes vides | Erreurs | | | SANS OBJET | |

## 5. Log/ (Logger, Driver, Drivers/*, Enumerations/Type)

Journal protocolaire de RainLoop (activé en admin, « Logs » avec option `log_level`, `show_passwords`, `hide_passwords`) : trace chaque échange IMAP/SMTP/Sieve avec masquage des mots de passe, pour le dépannage.
Équivalent Go : slog (`log_level` debug/info/warn/error et `log_format` text/json, config.go:123-124, 244-252 ; `c.log.Debug("imap", ...)` mailconn/messages.go:58 pour les opérations IMAP avec durée ; requêtes HTTP server.go:229-236).

| Fonction (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Logger::__construct (50), NewInstance (73), SingletonInstance (83) | Création du journal, gestionnaire d'erreurs PHP | Plomberie | slog.Logger passé à web.New server.go:58 | SANS OBJET | |
| DateHelper (101), IsSystemEnabled (110), SystemLog (119), Guid (132), Ping (146), IsEnabled (154) | Utilitaires : date avec décalage, journal système global, identifiant de requête (préfixe) | Corréler les lignes d'une même requête | aucun identifiant de requête dans nos logs (grep server.go:229-236 : method, path, status, ip seulement) | PARTIEL | Ajouter un `req_id` court dans les logs de requête et d'opérations IMAP (middleware + context). P3 |
| AddSecret (164), SetShowSecter (178), IsShowSecter (198) | Remplace les mots de passe par ******* dans le journal ; option pour les montrer | Ne jamais fuir le mot de passe dans le journal de débogage | go-imap/go-smtp ne journalisent pas les protocoles ; nous ne loguons pas les secrets (aucun) | OK (par construction) | Si un jour un trace protocolaire est ajouté (voir ci-dessous), masquer AUTH/LOGIN |
| HideErrorNotices (189), __phpErrorHandler (234), __loggerShutDown (256) | Erreurs PHP et pic mémoire/temps dans le journal | | `s.log.Error("erreur interne"...)` server.go:673 | SANS OBJET | Spécifique PHP |
| AddForbiddenType (208), RemoveForbiddenType (220) | Filtrer par type de ligne (INFO/NOTE/TIME/MEMORY…) | Niveaux de journal | log_level config.go:123 | OK | |
| WriteEmptyLine (280), Write (302), WriteDump (339), WriteException (354), WriteExceptionShort (381), WriteMixed (408) | Écriture d'un message/objet/exception sur tous les pilotes, déduplication des exceptions | Journalisation | slog (`s.log.*`) | OK | |
| Enumerations/Type (INFO, NOTICE, WARNING, ERROR, SECURE, NOTE, TIME, MEMORY, TIME_DELTA, *_PHP) | Types de lignes | Niveaux | slog niveaux | OK | |
| Driver::__construct (88), SetTimeOffset (127), DisableGuidPrefix (136), DisableTimePrefix (145), DisableTypedPrefix (192), getTimeWithMicroSec (241), getTypedPrefix (254), loggerLineImplementation (220), GetNewLine (379), writeEmptyLineImplementation (207), localWriteImplementation (266), clearImplementation (233), writeImplementation (202), Clear (388), WriteEmptyLine (397) | Mise en forme (préfixes heure/guid/type), écriture, purge | Format de ligne | slog text/json (config.go:124) | OK | Purge du journal : voir ci-dessous |
| Driver::WriteOnErrorOnly (156), WriteOnPhpErrorOnly (167), WriteOnTimeoutOnly (178), Write (297) | Mode « ne vider le tampon que si erreur / dépassement de délai » : le journal complet de la requête n'est écrit que s'il y a eu un problème | Garder des logs légers mais obtenir le détail au moment d'une panne | `mailconn/messages.go:38` : « à la fin avec la durée (info si modification, erreur ou lenteur, sinon debug) » ; requête lente server.go:229 | PARTIEL | Équivalent fonctionnel déjà présent pour IMAP et HTTP. Rien pour SMTP/Sieve (aucune trace lente). P3 : logguer SendSMTP et WithSieve en slog (durée, erreur classée, sans secret) |
| Drivers/File : __construct (32), SetLoggerFileName (43), NewInstance (54), writeImplementation (64), clearImplementation (72), writeToLogFile (82) | Journal fichier (par jour dans data/logs) | Consulter un journal | slog -> stdout (Docker logs) | SANS OBJET | Journal de conteneur ; pas de fichier à gérer |
| Drivers/Syslog : __construct (26), NewInstance (49), writeImplementation (59), clearImplementation (77) | Syslog | | aucun | SANS OBJET | Driver de journal |
| Drivers/Inline : __construct (32), NewInstance (46), writeImplementation (56), clearImplementation (80) | Affiche le journal dans la page (mode debug) | | aucun | SANS OBJET | Fuite possible, inutile |
| Drivers/Callback : __construct (37), NewInstance (51), writeImplementation (61), clearImplementation (74) | Journal vers fonction utilisateur | | aucun | SANS OBJET | |
| Absence de trace protocole IMAP/SMTP/Sieve (équivalent du « log IMAP complet » de RainLoop, utile en dépannage d'un serveur capricieux) | | Dépanner un serveur de mail | aucun (`c.log.Debug("imap")` ne trace que l'opération, pas les commandes brutes) | PARTIEL | Optionnel : option `log_protocol` (debug) enveloppant la `net.Conn` du `Guard.connect` (client.go:71) avec un tee masquant les lignes `LOGIN`/`AUTHENTICATE`/`AUTH`. P3 |

## 6. MailSo.php, Version.php, Hooks.php, Config.php

| Fonction / paramètre (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| MailSoSplAutoloadRegisterFunction (24) | Autoloader PHP | Plomberie | packages Go | SANS OBJET | |
| Version::AppVersion (32) | Version de la lib | | aucun | SANS OBJET | |
| Version::XMailer (40) | Valeur de l'en-tête X-Mailer (« MailSo/2.0.1 ») | En-tête X-Mailer, désactivable (`hide_x_mailer_header`) | `X-Mailer: wmail-go` compose/build.go:144 ; désactivable `HideXMailer` config.go:88, web/compose.go:544 | OK | |
| Version::Signature (48) | Signature PHAR | | | SANS OBJET | |
| Hooks::Run (28) / Hooks::Add (43) | Points d'extension plugins (ex. filtre le message avant envoi, recherche d'identité) | Plugins | aucun (plugins : voir cahier des charges l.16/l.73 : cité parmi les fonctions de RainLoop, pas implémenté) | SANS OBJET | Pas de système de plugins ; fonctions utiles déjà natives |
| Config::$ICONV (21), $MBSTRING (26), $FixIconvByMbstring (46) | Choix des jeux de caractères PHP | Décoder tous les charsets | golang.org/x/text (go.mod:16) | SANS OBJET | Plomberie |
| Config::$HtmlStrictAllowedTags (31) / $HtmlStrictAllowedAttributes (36) / $HtmlStrictDebug (41) | Listes blanches du nettoyeur HTML | Sécurité de l'affichage HTML | mailview/sanitize.go | OK | |
| Config::$MessageListFastSimpleSearch (51) | Recherche simple rapide | Recherche | internal/mailconn/search.go | OK | |
| Config::$MessageListCountLimitTrigger (56) | Seuil pour limiter le décompte | Perf. sur gros dossiers | aucun | SANS OBJET | |
| Config::$MessageListUndeletedOnly (61) | Cache les messages marqués \Deleted dans la liste | Ne pas voir les messages supprimés par un autre client (Thunderbird) avant expunge | aucun filtre `UNDELETED` (grep : \Deleted seulement mailconn/messages.go:648,684 pour la suppression) | MANQUANT | Ajouter `NotFlag: \Deleted` aux critères de recherche/liste (SEARCH ... UNDELETED). P2 (visible si un autre client est utilisé) |
| Config::$MessageListDateFilter (66) / $MessageListPermanentFilter (71) | Filtre permanent (date, texte) appliqué à toutes les listes | Fonction avancée d'admin | aucun | SANS OBJET | Réglage admin rarement utilisé |
| Config::$LargeThreadLimit (76) | Limite de messages d'un fil pour afficher les conversations | Conversations | Threads dans mailconn/messages.go:86,103-116 | OK | |
| Config::$MessageAllHeaders (81) | Affiche tous les en-têtes | Voir les en-têtes bruts | `/mail/source?view=headers` web/mailtools.go:15-29 | OK | |
| Config::$LogSimpleLiterals (86) | Journal : littéraux IMAP | | | SANS OBJET | |
| Config::$CheckNewMessages (91) | Vérifie les nouveaux messages | Notification de nouveaux messages | mailconn/poll.go | OK | |
| Config::$PreferStartTlsIfAutoDetect (96) | Préfère STARTTLS en auto-détection | | choix explicite TLS/STARTTLS/aucun client.go:25 | OK | |
| Config::$BoundaryPrefix (101) | Préfixe des frontières MIME | | compose/multipart.go:18 | OK | |
| Config::$ImapTimeout (106) | Délai IMAP 300 s | Ne pas bloquer | dialTimeout pool.go:13 ; contextes avec timeout `WithTimeout` client.go:537 | OK | |
| Config::$SystemLogger (111) | Journal système | | slog | OK | |

## À AJOUTER (MANQUANT / PARTIEL)

Priorité P2
1. Masquer les messages `\Deleted` dans les listes (Config::$MessageListUndeletedOnly, défaut vrai dans RainLoop) : ajouter UNDELETED aux critères de liste et de recherche (mailconn/search.go, messages.go liste). Utile si un autre client marque sans expunge.
2. IDN : conversion nom de domaine en punycode (RainLoop IdnToAscii dans MailFrom l.363, Rcpt l.406, Login l.191). Aucun `idna` dans le code (grep). Utiliser `golang.org/x/net/idna` (déjà en dépendance indirecte, go.mod:14) pour l'enveloppe SMTP (SendSMTP) et pour les en-têtes To/Cc, sans quoi l'envoi vers une adresse avec domaine accentué est refusé par un serveur sans SMTPUTF8. Je classe P2 (rare mais bloquant quand cela arrive).

Priorité P3
3. EHLO avec le vrai nom d'hôte au lieu de « localhost » (client.go:235,252, EhloHelper l.125).
4. `MAIL FROM` : paramètres SIZE (si annoncé) et RET=HDRS (si DSN) via `smtp.MailOptions` (client.go:574, SmtpClient::MailFrom l.361).
5. SMTP AUTH CRAM-MD5 (SmtpClient::Login l.193) : écrire le SASL client (go-sasl ne le fournit pas) et l'essayer en priorité si PLAIN/LOGIN absents.
6. Sieve : repli `AUTHENTICATE "LOGIN"` quand PLAIN absent (ManageSieveClient::Login l.191) et authzid optionnel.
7. Journalisation : `req_id` corrélant les lignes, trace lente/erreur pour SendSMTP et WithSieve, option `log_protocol` avec masquage des secrets (Log/*).
8. Changer le mot de passe de la boîte (Poppassd, plugin RainLoop) : non couvert ; l'utilisateur unique peut le faire ailleurs. Ne pas implémenter poppassd ; au mieux mettre à jour le coffre.
