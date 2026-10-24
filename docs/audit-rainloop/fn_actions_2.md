# Actions.php lignes 2600-5200 : analyse fonction par fonction

Périmètre : `RainLoop/Actions.php` 2600-5200 (71 fonctions de premier niveau ; les closures internes sont rattachées à leur fonction). `recFoldersTypes` (5198) commence dans la plage mais son corps dépasse 5200 : seule son intention (déduite du nom, de `systemFoldersNames` et de l'en-tête lu jusqu'à 5204) est évaluée.
Go de référence : `/home/gdegoulet/Work/webmail_project/internal/...`.

## Table 1 : identités, comptes, modèles, filtres (2602-3480)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| GetTemplateByID (2602) | Retrouve un modèle par id, sinon le premier | Insérer un modèle de message dans le composeur | web/templates_settings.go:119 `templatesBody` (GET /settings/templates/body, server.go:168) | OK | - |
| GetIdentities (2624) | Charge les identités (celle du compte + supplémentaires), applique la capa et l'ordre utilisateur | Plusieurs expéditeurs, ordre choisi | web/identities.go:30-33 (`AllIdentities()`: principale puis `Identities`), ordre via identityMove identities.go:140 | OK | - |
| GetIdentityByID (2713) | Identité par id (ou première) | Choisir l'identité d'envoi | web/compose.go:97 champ `identity` du formulaire | OK | - |
| GetAccountIdentity (2735) | Identité principale du compte | Expéditeur par défaut | `acc.AllIdentities()[0]` identities.go:31 | OK | - |
| SetAccounts (2746) | Persiste la liste des comptes additionnels (vide si seul parent) | Mémoriser les boîtes | web/accounts.go:324 `saveAccounts` | OK | - |
| SetIdentities (2773) | Persiste les identités (filtrées par capa) | Mémoriser identités | identities.go:105 appelle `saveAccounts` | OK | - |
| SetTemplates (2801) | Persiste les modèles | Mémoriser modèles | web/templates_settings.go:61-98 (`templatesSave`, stockage userdata) | OK | - |
| DoFilters (2821) | Charge les filtres Sieve (si capa) | Page Filtres | web/filters.go:193 `filtersGet`, loadSieve filters.go:58 | OK | - |
| DoFiltersSave (2850) | Enregistre les filtres + script brut (Raw/RawIsActive) | Édition filtres et script Sieve brut | web/filters.go:334 `filterSave`, 395 `filterAction`, 455 `filtersRaw` (use_raw) | OK | - |
| DoAccountSetup (2890) | Ajoute/modifie un compte additionnel : refuse si doublon (AccountAlreadyExists) ou inconnu, teste le login, mémorise | Ajouter une 2e boîte | web/accounts.go:376 `accountSave` (test IMAP/SMTP, limite `maxMailAccounts`) | PARTIEL | Aucun contrôle de doublon d'adresse/login trouvé par grep dans accountSave (376-456). Ajouter un refus si un autre compte a même IMAP login+host (message i18n `accounts.err.duplicate`). P3 |
| DoAccountDelete (2939) | Supprime un compte additionnel, bascule sur le parent si c'était l'actif | Retirer une boîte | web/accounts.go:457 `accountDelete` | OK | - |
| DoAttachmentsActions (2980) | Actions groupées sur des PJ sélectionnées : zip, OwnCloud, Dropbox | Télécharger des PJ choisies en zip | web/mail.go:1144 `mailAttachmentsZip` (zip de TOUTES les PJ d'un message, noms dédoublonnés) ; OwnCloud/Dropbox sans objet | PARTIEL | Zip : pas de sélection d'un sous-ensemble de PJ (cases à cocher + paramètre `n=` repétable dans /mail/attachments.zip). OwnCloud/Dropbox : SANS OBJET (services tiers). P3 |
| DoIdentityUpdate (3182) | Crée/modifie une identité (remplace par id, sinon ajoute) | Éditer identité | web/identities.go:50 `identitiesSave` (validation, doublon, limite 20) | OK | - |
| DoIdentityDelete (3225) | Supprime une identité | Idem | web/identities.go:113 `identitiesDelete` | OK | - |
| DoTemplateSetup (3259) | Crée/modifie un modèle (id généré) | Gérer modèles | web/templates_settings.go:61 | OK | - |
| DoTemplateDelete (3301) | Supprime un modèle | Idem | web/templates_settings.go:100 | OK | - |
| DoTemplateGetByID (3334) | Renvoie le corps d'un modèle | Insertion dans composeur | web/templates_settings.go:119 | OK | - |
| DoAccountsAndIdentitiesSortOrder (3370) | Mémorise l'ordre (glisser-déposer) des comptes et identités | Réordonner | web/accounts.go:551 `accountMove`, identities.go:140 `identityMove` (boutons haut/bas, pas drag&drop) | OK | Équivalent fonctionnel (ergonomie différente, acceptable) |
| DoAccountsAndIdentities (3396) | Liste comptes + identités pour l'UI | Sélecteur de boîte / d'expéditeur | web/accounts.go:349 `accountsList`, base.html:150-152 sélecteur de boîte | OK | - |
| DoTemplates (3424) | Liste des modèles | Liste dans réglages et composeur | web/templates_settings.go:47 `templatesGet` | OK | - |
| getAccountUnreadCountFromHash (3445) | Ouvre IMAP sur une autre boîte et renvoie le nb de non lus INBOX | Badge de non lus des autres comptes | aucun (le sélecteur base.html:74 / 150-152 n'affiche que l'adresse) | MANQUANT | Au changement de page ou via /mail/poll, interroger `Poll("INBOX",0)` des autres comptes (pool `s.pool.Do`, timeout court, cache 60 s) et afficher « (n) » dans le sélecteur. P3 (utile seulement si 2+ boîtes) |
| DoAccountsCounts (3477) | Compteurs non lus des comptes (max 7, flag Complete) pour l'UI | Idem, rafraîchi périodiquement | aucun | MANQUANT | Même proposition : étendre `pollReply` (web/poll.go:33) d'un champ `accounts` {id:unseen}. P3 |

## Table 2 : session, dossiers système, réglages utilisateur (3520-3660, 4940-5200)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ClearSignMeData (3523) | Efface cookie et stockage « se souvenir de moi » | Persistance de login longue durée | aucun cookie « sign me » ; sessions à durée réglable (handlers.go:392-393 idle/absolute) et appareil de confiance 2FA (twofactor.go:208) | SANS OBJET | Choix de conception : sessions serveur à TTL configurable remplacent « sign me ». |
| DoLogout (3539) | Déconnexion, purge cookies | Se déconnecter | web/handlers.go:107 `logout` (supprime session + cookie) | OK | - |
| DoAppDelayStart (3561) | Au démarrage différé, GC des caches (cache, fichiers, versions) | Hygiène disque | aucun équivalent web ; attachstore.go:112-124 nettoie par session | SANS OBJET | Infra PHP (caches fichiers). |
| DoSystemFoldersUpdate (3637) | Enregistre dossiers Sent/Draft/Spam/Trash/Archive | Réglages > Dossiers | web/folders.go:122-126, 278 `foldersSystem` (server.go:163) | OK | NullFolder (dossier « aucun ») sans objet. |
| setConfigFromParams (3662) | Helper : écrit une clé de config applicative | Admin | aucun | SANS OBJET | Panneau admin (config en fichier/YAML côté Go). |
| setCapaFromParams (3705) | Helper : active/désactive des capacités (comptes, identités, 2FA, gravatar, thèmes...) | Admin | aucun | SANS OBJET | Admin ; mono-utilisateur, tout activé. |
| setSettingsFromParams (3748) | Helper : écrit un réglage utilisateur typé | Réglages | web/handlers.go:390 `settingsPrefs` | OK | - |
| DoSettingsUpdate (4951) | Enregistre réglages perso : langue, thème, MPP (10,20,30,50,100,150,200,300), Layout (sans aperçu / latéral / bas), éditeur par défaut, ShowImages, autosave contacts, notif bureau/son, cases à cocher, brouillon auto, AutoLogout, 2FA, threads, répondre dans même dossier | Page Réglages | web/handlers.go:390-440 (langue, thème, page_size 10-500, layout three/list, editor, contacts autosave, notify_*, show_checkboxes, draft_autosave, threads, reply_same_folder, idle/absolute) ; ShowImages = `BlockRemote` par compte accounts.go:150 ; 2FA twofactor.go | PARTIEL | (a) Layout : seulement `three`/`list` (handlers.go:304, mail.go:488), pas d'aperçu EN BAS (RainLoop BOTTOM_PREVIEW) : ajouter `bottom` (CSS flex-direction column dans home.html, pane). (b) Taille de page : champ libre 10-500 ; le menu rapide (mail.go:40) propose 50/100/200/500 et non 10/20/30. Ajouter 10,20,30 dans `pageSizes`. P3 |
| DoNoop (5019) | Garde la connexion IMAP vivante | Éviter déconnexion pendant l'usage | web/poll.go:46 `mailPoll` (sonde périodique via pool) | OK | - |
| DoPing (5028) | Test de vie « Pong » | Supervision | web/server.go:111 `/healthz` | OK | - |
| DoChangePassword (5036) | Change le mot de passe de la BOÎTE via un fournisseur (plugin) | Changer son mot de passe mail | aucun pour le mot de passe IMAP ; web/handlers.go:320 change le mot de passe du coffre wmail seulement | SANS OBJET | Fournisseur absent par défaut dans RainLoop (plugin). Si voulu : ManageSieve/ poppassd hors périmètre. P3 non retenu. |
| DoWelcomeClose (5066) | Mémorise la fermeture de la page d'accueil de marque | Bandeau d'accueil admin | aucun | SANS OBJET | Branding admin. |
| DoVersion (5085) | Compare la version cliente et serveur (rechargement si obsolète) | Cache-busting assets | aucun (assets embarqués `/static/`, server.go:115) | SANS OBJET | Binaire unique, versions couplées. |
| recFoldersNames (5095) | Construit la liste « nom|flags|abonné » pour détecter un changement d'arbre de dossiers | Cache/rafraîchissement | aucun | SANS OBJET | Détail interne de cache PHP. |
| systemFoldersNames (5124) | Table de noms usuels → type de dossier (Sent, Sent Items, Outbox, Draft(s), Spam, Junk E-mail, Bulk Mail, Trash, Deleted Items, Bin, Archive(s), All Mail ...), avec variante sans espaces | Détection automatique des dossiers système sans SPECIAL-USE | web : mailconn/client.go:512-536 `ResolveSent/Drafts/Trash/Archive/Junk` (SPECIAL-USE puis liste d'alternatives) | PARTIEL | Listes plus courtes : manquent Outbox, Send/Sent Mail(s)/Sent Item, Draft/Draft Mail(s), Spams, Bulk Mails, Bin, All/All Mail(s) pour l'archive, et variantes sans espace (« SentItems », « DeletedItems »). Ajouter ces alias dans les `alt` de client.go:512-536 et comparer en ignorant espaces. P3 |
| recFoldersTypes (5198) | Parcourt récursivement l'arbre et assigne les types système (SPECIAL-USE puis noms de systemFoldersNames) | Même fonction que ci-dessus | mailconn/client.go:486 `ResolveFolder` + web/foldertree.go | PARTIEL | Voir systemFoldersNames (même correctif). Corps après 5200 non lu. P3 |

## Table 3 : administration, domaines, paquets, plugins (3780-4940)

Toutes ces fonctions exigent `IsAdminLoggined()` : panneau d'admin RainLoop (multi-domaines, paquets, plugins, mise à jour). Wmail-go est mono-utilisateur, configuré par fichier (config/) et sans plugins : SANS OBJET, sauf mention.

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| DoAdminSettingsUpdate (3781) | Enregistre ~80 clés de config (langues, thème par défaut, SSL, contacts PDO, capas, social, branding...) | Config serveur via UI | config/ + internal/config (fichier) | SANS OBJET | Admin ; Google/Facebook/Twitter/Dropbox = social/OAuth hors périmètre. |
| DoAdminLogin (3890) | Connexion admin (délai anti brute-force) | Accès admin | aucun | SANS OBJET | Pas d'admin. (Anti brute-force login utilisateur présent : handlers.go:29-48.) |
| DoAdminLogout (3916) | Déconnexion admin | idem | aucun | SANS OBJET | Admin. |
| DoAdminPing (3925) | Maintien de session admin | idem | aucun | SANS OBJET | Admin. |
| DoAdminContactsTest (3935) | Teste la connexion BDD contacts (PDO) | Config contacts | aucun | SANS OBJET | Contacts en stockage interne chiffré (vault). |
| DoAdminLicensing (3960) | Licence (toujours indisponible) | - | aucun | SANS OBJET | Mort. |
| DoAdminLicensingActivate (3969) | idem | - | aucun | SANS OBJET | Mort. |
| DoAdminPasswordUpdate (3978) | Change identifiants admin | - | aucun | SANS OBJET | Admin. |
| DoAdminDomainLoad (4017) | Lit la définition d'un domaine (hôtes IMAP/SMTP/Sieve) | Config serveurs par domaine | par compte : web/accounts.go:164 `buildAccount` | SANS OBJET | Pas de domaines ; paramètres saisis par compte. |
| DoAdminDomainList (4028) | Liste des domaines | idem | web/accounts.go:349 | SANS OBJET | idem |
| DoAdminDomainDelete (4048) | Supprime un domaine | idem | aucun | SANS OBJET | idem |
| DoAdminDomainDisable (4059) | Désactive un domaine | idem | aucun | SANS OBJET | idem |
| DoAdminDomainSave (4072) | Crée/modifie un domaine | idem | accounts.go:376 (saisie par compte) | SANS OBJET | idem |
| DoAdminDomainAliasSave (4085) | Alias de domaine | idem | aucun | SANS OBJET | idem |
| DoAdminDomainTest (4098) | Teste connexion IMAP, SMTP (ou mail() PHP) et Sieve, avec temps et message d'erreur | Vérifier ses paramètres de serveur avant d'enregistrer | web/accounts.go:295 `testAccount` (IMAP + SMTP en parallèle, erreurs typées accounts.go:286) | PARTIEL | Le test ne couvre pas ManageSieve (sieveEndpoint existe accounts.go:264 mais n'est pas testé dans testAccount) : ajouter un test Sieve (goroutine 3) si `!SieveDisabled`, afficher le résultat à côté de IMAP/SMTP. mail() PHP sans objet. P3 |
| rainLoopRepo (4240) | URL du dépôt de paquets | Mises à jour | aucun | SANS OBJET | Écosystème de paquets RainLoop. |
| rainLoopUpdatable (4251) | Mise à jour possible ? | idem | aucun | SANS OBJET | idem |
| rainLoopCoreAccess (4260) | Droit d'accès au core update | idem | aucun | SANS OBJET | idem |
| getRepositoryDataByUrl (4274) | Télécharge/parse l'index du dépôt | idem | aucun | SANS OBJET | idem |
| getCoreChannel (4364) | Canal de version (stable) | idem | aucun | SANS OBJET | idem |
| getCoreData (4375) | Version cœur disponible | idem | aucun | SANS OBJET | idem |
| getRepositoryData (4425) | Fusion index paquets / plugins installés | idem | aucun | SANS OBJET | idem |
| DoAdminPackagesList (4496) | Liste paquets installables | idem | aucun | SANS OBJET | idem |
| DoAdminCoreData (4514) | Infos sur le cœur et mise à jour | idem | aucun | SANS OBJET | idem |
| DoAdminUpdateCoreData (4570) | Lance la mise à jour du cœur | idem | aucun | SANS OBJET | Déploiement par Docker/binaire. |
| DoAdminPackageDelete (4597) | Désinstalle un paquet | idem | aucun | SANS OBJET | Pas de plugins. |
| downloadRemotePackageByUrl (4635) | Télécharge un paquet | idem | aucun | SANS OBJET | idem |
| DoAdminPackageInstall (4670) | Installe un paquet/plugin/ thème | idem | thèmes : web/server.go:116 `themeFile` (thèmes embarqués/dossier) | SANS OBJET | Thème DarkShine déjà fourni statiquement. |
| DoAdminPluginList (4743) | Liste plugins | idem | aucun | SANS OBJET | Pas de plugins. |
| pluginEnable (4771) | (Dés)active un plugin | idem | aucun | SANS OBJET | idem |
| DoAdminPluginDisable (4810) | Désactive un plugin | idem | aucun | SANS OBJET | idem |
| DoAdminPluginLoad (4840) | Charge la fiche d'un plugin | idem | aucun | SANS OBJET | idem |
| DoAdminPluginSettingsUpdate (4886) | Enregistre réglages d'un plugin | idem | aucun | SANS OBJET | idem |

## Décompte (71 fonctions)

- OK : 23
- PARTIEL : 6 (DoAccountSetup, DoAttachmentsActions, DoSettingsUpdate, systemFoldersNames, recFoldersTypes, DoAdminDomainTest)
- MANQUANT : 2 (getAccountUnreadCountFromHash, DoAccountsCounts)
- SANS OBJET : 40
- Total : 23 + 6 + 2 + 40 = 71.

## À AJOUTER (MANQUANT / PARTIEL, par priorité)

Aucun item P1 : tout ce que l'utilisateur emploie quotidiennement (identités, filtres, modèles, dossiers système, réglages, déconnexion) est couvert dans cette plage.

P2 : aucun.

P3 :
1. Non lus des autres boîtes dans le sélecteur de comptes (getAccountUnreadCountFromHash + DoAccountsCounts) : étendre `/mail/poll` (web/poll.go) avec `accounts:{id:unseen}` et afficher le badge dans base.html:150-152.
2. Disposition « aperçu en bas » (DoSettingsUpdate/Layout) : ajouter la valeur `bottom` aux préférences (handlers.go:304,420 ; mail.go:488) + CSS.
3. Tailles de page 10/20/30 dans le menu rapide (mail.go:40 `pageSizes`).
4. Alias de noms de dossiers système (systemFoldersNames/recFoldersTypes) : compléter mailconn/client.go:512-536 (Outbox, Sent Mail(s), Draft(s), Spams, Bulk Mails, Bin, All Mail, variantes sans espaces).
5. Zip de PJ sélectionnées (DoAttachmentsActions) : cases à cocher + paramètres multiples dans `/mail/attachments.zip`.
6. Test ManageSieve dans le test de connexion du compte (DoAdminDomainTest) : accounts.go:295.
7. Contrôle de doublon de compte à l'ajout (DoAccountSetup) : accounts.go:376.
