# Routes HTTP de wmail-go

Référence exhaustive des **96 routes** servies par `internal/web`. Chaque route est enregistrée dans
`Server.Handler()` (`internal/web/server.go`) ; l'ordre de ce document suit celui du code.

Généré à la main à partir du code ; pour recouper, voir `internal/web/server.go:124` et suivants.

---

## 1. Conventions valables pour toutes les routes

### Pile de traitement

Toute requête traverse, dans cet ordre :

1. `logging` — une ligne par requête sur la sortie standard (méthode, chemin, code, durée).
2. `secure` — en-têtes de sécurité, limite de taille du corps, contrôle d'origine des POST.
3. le routeur `http.ServeMux` (motifs Go 1.22 : méthode + chemin + variables `{nom}`).
4. `auth`, pour les routes marquées **session** ci-dessous.

### En-têtes posés sur chaque réponse (`secure`)

| En-tête | Valeur |
|---|---|
| `Content-Security-Policy` | `default-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'` |
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `no-referrer` |
| `Permissions-Policy` | caméra, micro, géolocalisation, paiement, USB… tous refusés |
| `Cross-Origin-Opener-Policy` | `same-origin` |
| `Cross-Origin-Resource-Policy` | `same-origin` |
| `Strict-Transport-Security` | `max-age=31536000`, seulement si la requête est arrivée en HTTPS |

`/compose` et `/compose/*` reçoivent une CSP élargie à `style-src 'self' 'unsafe-inline'` : l'éditeur de texte
riche écrit des attributs `style`. Les scripts et les images distantes restent interdits.

`/mail/body` et `/mail/source` posent leur propre CSP, bien plus stricte (voir leurs fiches).

### Protection des écritures

Toute requête `POST` subit deux contrôles :

- **Origine** (`sameOrigin`, dans `secure`) : `Origin` ou `Referer` doit correspondre à l'hôte. Sinon **403**
  avec la clé `error.csrf`. S'applique y compris aux POST non authentifiés (`/login`, `/register`, `/recover`).
- **Jeton CSRF** (dans `auth`) : le champ de formulaire `csrf` doit égaler `session.CSRF`, comparé en temps
  constant. Sinon **403**. S'applique à toutes les routes marquées **session**.

Le corps des POST est borné par `postLimit(path)` : la limite dépend du chemin (envoi, pièces jointes et import
sont plus larges que les formulaires ordinaires).

**Le champ `csrf` n'est donc pas répété dans les tableaux de paramètres : il est obligatoire sur chaque POST
authentifié.**

### Session

- Marqué **session** : passe par `auth`. Sans session valable, `GET` redirige (303) vers
  `/login?expired=1&next=<chemin demandé>`, et `POST` vers `/login`.
- Une clé de récupération en attente (`session.PendingRecoveryKey`) détourne toute route authentifiée vers
  `/recovery-key` tant que l'utilisateur ne l'a pas acquittée. Seules les routes `/recovery-key*` échappent.
- Marqué **public** : aucune session requise.

### Limitation de débit

Réglée dans `config.json` (`rate_limit`). Les compteurs portent à la fois sur l'adresse IP et sur le login.

| Règle | Routes concernées |
|---|---|
| `login` | `POST /login`, `POST /login/2fa`, `POST /settings/password`, `POST /settings/recovery`, codes TOTP |
| `register` | `POST /register` (par IP seulement) |
| `recovery` | `POST /recover` |
| `mail_test` | `POST /accounts/save` quand il teste la connexion |
| interne | `/mail/image` (mandataire d'images), ManageSieve (`sieveLimit`) |

Un dépassement répond **429** avec le délai d'attente.

### Codes de retour courants

| Code | Sens |
|---|---|
| 200 | page ou fragment rendu |
| 303 | redirection après une écriture réussie (motif POST/Redirect/GET) |
| 400 | paramètre invalide — clé `mail.err.bad_request` ou message propre au formulaire |
| 401 | identifiants ou code refusés |
| 403 | origine différente ou jeton CSRF invalide |
| 404 | objet inconnu — clé `error.not_found` |
| 409 | conflit (login déjà pris, script Sieve étranger) |
| 413 | corps trop gros |
| 422 | contenu refusé (premier compte dont le test échoue, fichier qui n'est pas un message) |
| 429 | limitation de débit |
| 502 | le serveur IMAP, SMTP ou ManageSieve n'a pas répondu |

### Notations

- `{nom}` : variable de chemin.
- *requête* : paramètre de la chaîne de requête (`?a=b`).
- *formulaire* : champ de corps `application/x-www-form-urlencoded` ou `multipart/form-data`.
- Un champ répétable est noté `nom[]`.
- Les booléens de formulaire suivent la convention HTML : **présent et non vide = vrai, absent = faux**.

---

## 2. Service et ressources statiques

### `GET /healthz` — public

Sonde de vivacité. Répond `ok\n` en `text/plain`. N'ouvre aucune connexion IMAP et ne touche pas au disque :
convient à `HEALTHCHECK` et aux sondes d'un orchestrateur.

Aucun paramètre.

### `GET /static/` — public

Sert l'arborescence `static/` embarquée dans le binaire (`embed.FS`) : feuilles de style, scripts, icônes.
Préfixe retiré par `http.StripPrefix`, cache long posé par `staticCache`.

Aucun paramètre.

### `GET /favicon.ico` — public

Redirection **301** permanente vers `/static/favicon.svg`. Présente uniquement pour les navigateurs qui
demandent `/favicon.ico` d'office.

### `GET /themes/{name}/{file}` — public

Sert un fichier de thème.

| Paramètre | Emplacement | Filtre | Rôle |
|---|---|---|---|
| `name` | chemin | `themeNameRe` | nom du thème |
| `file` | chemin | `themeFileRe` | nom du fichier (`theme.css`, images) |

Les deux noms sont validés par expression rationnelle **avant** toute lecture : pas de traversée de chemin.
Le fichier est cherché d'abord dans `<data_dir>/themes/<name>/` (posé par l'administrateur), sinon dans
`static/themes/<name>/` embarqué. Introuvable → **404**.

`theme.css` passe par `dropMissingWallpaper` : les règles qui désignent un fond absent sont retirées, pour
éviter une requête vers une image qui n'existe pas.

Type de contenu déduit de l'extension : `.png`, `.webp`, `.jpg`/`.jpeg`, `.svg`, sinon `text/css`.
`Cache-Control: no-cache` — un thème modifié sur le disque est repris sans vider le cache du navigateur.

### `GET /user.css` — public

Feuille de style des variables propres à l'utilisateur : largeur du panneau des dossiers, largeur de la liste.
Sans session, répond une feuille vide — ce qui permet de la référencer inconditionnellement dans le gabarit.

Aucun paramètre.

---

## 3. Langue

### `GET /lang` — public

Change la langue de l'interface **avant** la connexion (après, c'est une préférence du compte).

| Paramètre | Emplacement | Filtre | Rôle |
|---|---|---|---|
| `l` | requête | doit exister dans `i18n` | code de langue (`fr`, `en`) |
| `next` | requête | **liste blanche** : `/register` ou `/recover` | page de retour |

Pose le cookie `lang` (365 jours, `SameSite=Lax`, `Secure` si HTTPS). Une langue inconnue est ignorée
silencieusement et le cookie n'est pas posé. Redirige **303** vers `next` s'il figure dans la liste blanche,
sinon vers `/login`.

---

## 4. Connexion, inscription, récupération

### `GET /login` — public

Formulaire de connexion. Si une session est déjà ouverte, redirige **303** vers `/`.

| Paramètre | Emplacement | Filtre | Rôle |
|---|---|---|---|
| `next` | requête | `safeNext` : chemin local commençant par `/`, sans `//` ni schéma | page à rouvrir après connexion |
| `expired` | requête | `=1` | affiche « votre session a expiré » |

### `POST /login` — public

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `login` | formulaire | identifiant du compte |
| `password` | formulaire | mot de passe |
| `next` | formulaire | `safeNext`, page à rouvrir |

Déroulé : limitation de débit → `store.Login` → second facteur si le compte en a un → ouverture de session.

- Bloqué par la limitation : **429**, journal serveur, événement `login_blocked` dans le journal d'activité.
- Identifiants refusés : **401**, page de connexion avec `login.error.invalid`, événement `login_fail`.
- Second facteur exigé : la page `login_2fa` est rendue et un cookie `pending` (portée `/login`) est posé.
- Succès : compteur par utilisateur remis à zéro, session ouverte, redirection **303** vers `next`.

Le login n'est jamais écrit entier dans le journal serveur : `store.ReduceLogin` le réduit.

### `POST /login/2fa` — public

Second facteur de la connexion en cours. Sans le cookie `pending`, redirige vers `/login`.

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `code` | formulaire | code TOTP à 6 chiffres **ou** un code de secours |
| `trust` | formulaire | booléen : poser un cookie « appareil de confiance » |
| `next` | formulaire | `safeNext` |

La tentative en attente expire au bout d'un délai court (`pending.attempt`) ; passé ce délai, **401** avec
`twofactor.expired`. Code refusé : **401**, événement `2fa_fail`. Limitation de débit par IP.

### `GET /register` — public

Formulaire d'inscription. Si `allow_registration` est faux dans la configuration : **404** avec
`register.error.closed` — l'ouverture du service n'est pas devinable.

### `POST /register` — public

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `login` | formulaire | identifiant souhaité |
| `password` | formulaire | mot de passe |
| `password2` | formulaire | confirmation |

Limitation par IP, **chaque tentative compte**, réussie ou non. `password != password2` → **400**
(`error.mismatch`). Login déjà pris → **409**. Nombre maximum de comptes atteint → **403**. Succès : compte
créé, session ouverte, redirection **303** vers `/recovery-key` — la clé de récupération doit être vue avant
toute autre page.

### `GET /recover` — public

Formulaire de récupération par clé. Aucun paramètre.

### `POST /recover` — public

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `login` | formulaire | identifiant |
| `recovery_key` | formulaire | clé de récupération remise à la création |
| `password` | formulaire | nouveau mot de passe |
| `password2` | formulaire | confirmation |

Clé refusée → **401** (`recover.error.invalid`), événement `login_fail` avec le détail `recover`.
Succès : **toutes les autres sessions du compte sont fermées**, événement `recovery`, une **nouvelle** clé est
générée (l'ancienne a servi) et affichée via `/recovery-key`.

### `GET /recovery-key` — session

Affiche la clé de récupération en attente, en pleine page sans barre de navigation. Sans clé en attente,
redirige **303** vers `/`.

### `GET /recovery-key.txt` — session

La même clé en fichier à télécharger (`wmail-go-recovery-key.txt`), `Cache-Control: no-store`. Sans clé en
attente : **404**. Le fichier porte l'avertissement en français et en anglais.

### `POST /recovery-key` — session

Acquitte la clé : vide `session.PendingRecoveryKey` et redirige **303** vers `/`. Tant que ce POST n'a pas eu
lieu, `auth` renvoie toute autre route vers `/recovery-key`.

### `POST /logout` — session

Journalise la déconnexion, écrit l'événement `logout`, supprime la session côté serveur **et** le cookie,
puis redirige **303** vers `/login`.

---

## 5. Boîtes mail (comptes IMAP/SMTP du compte webmail)

### `GET /accounts` — session

Liste des boîtes configurées, dans leur ordre d'affichage. Aucun paramètre.

### `GET /accounts/new` — session

Formulaire vierge, prérempli par `defaultForm()` (ports et sécurité par défaut).

### `GET /accounts/{id}/edit` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `id` | chemin | identifiant de la boîte |

Boîte inconnue → **404** (`error.not_found`). Les mots de passe ne sont **jamais** renvoyés dans le
formulaire : seuls des indicateurs `HasIMAPPassword` / `HasSMTPPassword` disent qu'un mot de passe existe.

### `POST /accounts/save` — session

Crée ou met à jour une boîte. Deux boutons partagent la même route, distingués par `action`.

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `action` | formulaire | `test` = tester seulement ; toute autre valeur = enregistrer |
| `id` | formulaire | vide = création, sinon identifiant modifié |
| `name` | formulaire | nom affiché de la boîte |
| `email` | formulaire | adresse de l'identité principale |
| `display_name` | formulaire | nom affiché dans les messages envoyés |
| `imap_host`, `imap_port`, `imap_security`, `imap_login` | formulaire | serveur de réception |
| `imap_password` | formulaire | laissé vide = mot de passe existant conservé |
| `imap_verify` | formulaire | booléen : vérifier le certificat TLS |
| `imap_legacy` | formulaire | booléen : tolérer un TLS ancien |
| `smtp_host`, `smtp_port`, `smtp_security`, `smtp_login` | formulaire | serveur d'envoi |
| `smtp_password`, `smtp_verify`, `smtp_legacy` | formulaire | idem IMAP |
| `sieve_disabled` | formulaire | booléen : ne pas utiliser ManageSieve |
| `sieve_host`, `sieve_port`, `sieve_security`, `sieve_login` | formulaire | serveur de filtres |
| `sieve_password`, `sieve_verify`, `sieve_legacy` | formulaire | idem IMAP |
| `trash_folder`, `archive_folder`, `junk_folder`, `sent_folder`, `drafts_folder` | formulaire | dossiers système |
| `signature` | formulaire | signature (CRLF normalisés en LF) |
| `signature_before` | formulaire | booléen : signature au-dessus de la citation |
| `reply_to` | formulaire | adresse de réponse |
| `bcc` | formulaire | copie cachée systématique |
| `block_remote` | formulaire | booléen : bloquer les images distantes |
| `proxy_images` | formulaire | booléen : faire passer les images par le serveur |

Contrôles :

- Boîte en doublon — même adresse **et** même serveur IMAP qu'une boîte existante → `accounts.err.duplicate`.
- Nombre maximum de boîtes atteint → `accounts.err.limit`.
- `action=test` : teste IMAP, SMTP et ManageSieve, réaffiche le formulaire avec les résultats, **sans rien
  enregistrer**. Chaque test consomme un jeton de la règle `mail_test`, qu'il réussisse ou non.
- **La toute première boîte n'est enregistrée que si le test réussit** (`accounts.err.first_must_pass`,
  **422**). Les suivantes sont enregistrées sans test obligatoire.

Succès : événement `mail_add` dans le journal d'activité, redirection **303**.

### `POST /accounts/select` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `id` | formulaire | boîte à rendre active |

Change `session.ActiveMailAccount` et redirige **303** vers `/`. Inconnue → **404**.

### `POST /accounts/reorder` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `order` | formulaire | **liste complète** des identifiants, séparés par des virgules, dans l'ordre voulu |

Envoyé par le glisser-déposer. Une liste incomplète ou contenant un identifiant inconnu est refusée (**400**).
La première boîte de la liste est celle ouverte au démarrage.

### `POST /accounts/{id}/move` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `id` | chemin | boîte à déplacer |
| `dir` | formulaire | `up` ou `down` |

Équivalent sans script de la route précédente.

### `POST /accounts/{id}/delete` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `id` | chemin | boîte à retirer |

Retire la boîte de la configuration du compte. **Rien n'est supprimé côté serveur IMAP.** Événement
`mail_del`. Redirige **303** vers `/accounts`.

---

## 6. Identités supplémentaires

### `GET /accounts/{id}/identities` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `id` | chemin | boîte concernée |
| `edit` | requête | identifiant de l'identité à ouvrir dans le formulaire |

### `POST /accounts/{id}/identities` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `id` | chemin | boîte concernée |
| `iid` | formulaire | vide = ajout, sinon identité modifiée |
| `email` | formulaire | adresse |
| `name` | formulaire | nom affiché |
| `reply_to` | formulaire | adresse de réponse |
| `bcc` | formulaire | copie cachée |
| `signature` | formulaire | signature propre à l'identité |
| `signature_before` | formulaire | booléen |

Erreurs possibles : adresse invalide (`identities.err.address`), adresse déjà utilisée
(`identities.err.duplicate`), nombre maximum atteint (`identities.err.limit`), champ trop long
(`accounts.err.too_long`).

### `POST /accounts/{id}/identities/{iid}/delete` — session

Supprime une identité supplémentaire. Boîte ou identité inconnue → **404**. L'identité **principale** n'est pas
concernée : elle vient de la boîte elle-même.

### `POST /accounts/{id}/identities/{iid}/move` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `dir` | formulaire | `up` ou `down` |

L'identité principale reste toujours la première ; seul l'ordre des identités supplémentaires change.

### `POST /settings/identities/reorder` — session

| Paramètre | Emplacement | Rôle |
|---|---|---|
| `box` | formulaire | identifiant de la boîte |
| `order` | formulaire | liste complète des identifiants d'identités, dans l'ordre |

Version glisser-déposer. Liste incomplète → **400**.

---

## 7. Liste des messages

### `GET /` — session

Page principale : panneau des dossiers, liste des messages, volet de lecture facultatif. Sans aucune boîte
configurée, redirige **303** vers `/accounts/new`.

**Paramètres de liste** (analysés par `parseListParams`, communs à `/` et `/mail/pane`) :

| Paramètre | Filtre | Défaut | Rôle |
|---|---|---|---|
| `folder` | `mailconn.ValidFolderName` | `INBOX` | dossier affiché |
| `page` | entier, `1 < n < 100000` | 1 | page de résultats |
| `sort` | `date`, `arrival`, `from`, `subject`, `size` | `date` | critère de tri |
| `dir` | `asc` ou `desc` | dépend du critère | sens du tri |
| `q` | tronqué à 200 caractères par terme, 400 au total | vide | recherche, syntaxe RainLoop (`from:`, `to:`, `subject:`, `is:`, `has:`, `date:`) |
| `scope` | `subject`, `from`, `body` | `all` | portée de la recherche simple |
| `filter` | `unread` | `all` | n'afficher que les non lus |
| `thread` | UID | 0 | n'afficher que cette conversation |

**Recherche avancée** — chaque champ est traduit en opérateur et **concaténé à `q`** :

| Paramètre | Opérateur produit |
|---|---|
| `adv_from` | `from:` |
| `adv_to` | `to:` |
| `adv_subject` | `subject:` |
| `adv_text` | `text:` |
| `adv_unread` | `is:unseen` |
| `adv_flagged` | `is:flagged` |
| `adv_attach` | `has:attachment` |
| `adv_date` | `date:<début>/<aujourd'hui>` — valeurs `3d`, `1w`, `1m`, `3m`, `6m`, `1y` |

Une valeur contenant une espace est mise entre guillemets ; les guillemets déjà présents sont retirés.
Chaque terme est borné **séparément** : tronquer la requête assemblée pourrait couper un opérateur en deux.

**Volet de lecture** (seulement si la disposition n'est pas « liste seule ») :

| Paramètre | Rôle |
|---|---|
| `uid` | UID du message ouvert dans le volet |
| `view` | `text` = forcer le texte brut, sinon HTML assaini |
| `remote` | `1` = afficher les images distantes de ce message |

Le message du volet est lu **avant** les compteurs de non lus : l'ouverture le marque comme lu, et les
compteurs doivent en tenir compte. Un échec IMAP sur les dossiers rend la page avec `FolderError` plutôt
qu'une erreur : la navigation reste possible.

La page calcule aussi le quota (si le serveur l'annonce), les cibles de déplacement, et masque les boutons
sans objet selon le dossier (pas d'archivage dans Archive, pas de « spam » dans Indésirables, Envoyés ou
Brouillons).

### `GET /mail/pane` — session

Rend **uniquement le volet de lecture**, en fragment HTML. Le script de la liste remplace le volet sans
recharger la liste : la position de défilement et la sélection sont conservées.

Prend exactement les mêmes paramètres que `GET /`.

### `GET /mail/message` — session

Affiche un message sur une page entière : disposition « liste seule », petits écrans, liens directs.

| Paramètre | Filtre | Rôle |
|---|---|---|
| `folder` | `ValidFolderName` | dossier |
| `uid` | entier non nul | message |
| `view` | `text` | forcer le texte brut |
| `remote` | `1` | afficher les images distantes |
| `back` | `safeBack` | URL de retour vers la liste |

`folder` ou `uid` invalide → **400** (`mail.err.bad_request`).

### `GET /mail/body` — session

Sert le **corps assaini** d'un message, destiné à un `iframe sandbox`.

| Paramètre | Rôle |
|---|---|
| `folder`, `uid` | message |
| `view` | `text` = conversion du HTML en texte, sinon HTML assaini |
| `remote` | `1` = ne pas bloquer les images distantes |

CSP propre à la réponse : `default-src 'none'` ; `style-src 'unsafe-inline'` ; `base-uri 'none'` ;
`form-action 'none'`. La directive `img-src` dépend du réglage :

- images bloquées → `img-src data:` seulement ;
- autorisées et mandatées → `img-src data: 'self'` : les images passent par `/mail/image`, **le navigateur ne
  contacte jamais l'expéditeur** ;
- autorisées sans mandataire → `img-src data: https: http:`.

Aucun script ne peut s'exécuter, dans aucun cas.

### `GET /mail/source` — session

Source du message.

| Paramètre | Valeurs | Rôle |
|---|---|---|
| `folder`, `uid` | | message |
| `view` | `source` | message complet en texte brut |
| | `headers` | en-têtes seuls |
| | `eml` | téléchargement du fichier `.eml` |
| | `part` | contenu décodé d'une partie MIME |
| `index` | entier | numéro de la partie, requis si `view=part` |

Toujours servi en texte brut, **jamais interprété par le navigateur** :
`Content-Security-Policy: sandbox`, `X-Content-Type-Options: nosniff`, `Cache-Control: private, no-store`.
`view=eml` passe par `http.ServeContent` (requêtes partielles possibles) et nomme le fichier d'après l'objet,
assaini. `index` hors bornes → **404**.

### `GET /mail/print` — session

Version imprimable : en-têtes, liste des pièces jointes, corps assaini.

| Paramètre | Rôle |
|---|---|
| `folder`, `uid` | message |
| `remote` | `1` = images distantes autorisées |

La page n'autorise que le script d'impression de l'application ; **le contenu du message ne peut exécuter
aucun script**.

### `GET /mail/attachment` — session

Télécharge une pièce jointe.

| Paramètre | Rôle |
|---|---|
| `folder`, `uid` | message |
| `index` | numéro de la pièce (entier) |
| `inline` | `1` = tenter un aperçu dans le navigateur |

La pièce est demandée seule au serveur IMAP d'après la structure annoncée : un message de 100 Mo ne transite
pas par la mémoire pour en extraire un fichier de 2 Mo. Si le serveur ne le permet pas, le message entier est
relu, en repli.

Règles de sécurité :

- Le nom est assaini (`safeFileName`).
- Un type non reconnu, ou contenant `html`, `xml`, `svg` ou `javascript`, devient `application/octet-stream`.
- `Content-Disposition: attachment` **par défaut, toujours**.
- `inline=1` ne donne `inline` que pour les types sûrs, **reconnus d'après les octets eux-mêmes**, jamais
  d'après l'étiquette annoncée. Un PDF reçoit une CSP sans script ni ressource.

Pièce inconnue → **404**.

### `GET /mail/attachments.zip` — session

Toutes les pièces jointes d'un message dans une archive zip.

| Paramètre | Rôle |
|---|---|
| `folder`, `uid` | message |

### `GET /mail/image` — session

Mandataire d'images distantes. **Jamais appelé directement par l'utilisateur** : les URL sont réécrites dans
le corps assaini.

| Paramètre | Filtre | Rôle |
|---|---|---|
| `u` | base64url, longueur ≤ 2000, schéma `http`/`https`, hôte présent, **pas d'identifiants dans l'URL** | adresse distante |
| `sig` | `hmac.Equal` avec `proxyMAC(session.CSRF, url)` | signature |

Toute incohérence → **404**, sans distinguer les cas. Le téléchargement passe par le garde **anti-SSRF**
(`s.guard`), la taille est bornée, et seuls les formats raster reconnus **d'après les octets** sont renvoyés.
Limitation de débit propre à la session.

### `GET /mail/avatar` — session

Photo d'un correspondant.

| Paramètre | Rôle |
|---|---|
| `email` | adresse du correspondant |

Source : d'abord le carnet d'adresses. Si l'utilisateur a choisi Gravatar, la photo est ensuite demandée à
gravatar.com **par le serveur** — le navigateur ne contacte jamais ce service, donc l'adresse de l'utilisateur
ne fuit pas vers un tiers. Sans photo : **404**, et l'interface garde la silhouette.

### `POST /mail/action` — session

Toutes les actions de masse de la liste.

| Paramètre | Rôle |
|---|---|
| `action` | `read`, `unread`, `flag`, `unflag`, `delete`, `purge`, `move`, `copy`, `archive`, `spam`, `notspam`, `readall`, `unreadall` |
| `folder` | dossier courant (`ValidFolderName`) |
| `uid[]` | UID cochés, répétable, au plus `maxActionUIDs` |
| `dest` | dossier cible de `move` et `copy` |
| `th_<uid>` | membres d'une conversation, séparés par des virgules |
| `current_uid` | message ouvert dans le volet, employé si aucune case n'est cochée |
| `move_to` | raccourci : vaut `action=move` + `dest=<valeur>` |
| `copy_to` | raccourci : vaut `action=copy` + `dest=<valeur>` |
| `star_on` | raccourci : vaut `action=flag` sur ce seul UID |
| `star_off` | raccourci : vaut `action=unflag` sur ce seul UID |
| `back` | `safeBack`, retour à la liste |
| `stay` | `safeBack`, retour en restant sur le message |
| `open_next` | `safeBack`, message à ouvrir après l'action |

Règles :

- Une **conversation cochée entraîne tous ses messages** pour `delete`, `purge`, `move`, `copy`, `archive` et
  `spam`. Pas pour les drapeaux, qui ne concernent que le message représentant.
- Les UID sont dédoublonnés.
- `readall` et `unreadall` portent sur le dossier entier, sans sélection.
- Aucune sélection, action inconnue, UID non numérique, trop d'UID → **400**.

### `POST /mail/clear` — session

Vide définitivement un dossier.

| Paramètre | Rôle |
|---|---|
| `folder` | dossier à vider |
| `back` | `safeBack` |

**Seuls la corbeille et les indésirables** peuvent être vidés. `INBOX` ne l'est **jamais**, même si le compte
la désigne comme corbeille. Tout autre dossier → erreur `errNotClearable`.

### `POST /mail/append` — session

Ajoute au dossier affiché un message déposé en glisser-déposer. **Répond en JSON.**

| Paramètre | Rôle |
|---|---|
| `folder` | dossier cible |
| `file` | fichier `.eml` (`multipart/form-data`) |

Taille bornée par `max_message_mb + max_attachment_mb`. Le contenu doit s'analyser comme un message RFC 822
comportant au moins un expéditeur, une date **ou** un objet — sinon **422** `{"error":"not_a_message"}`.
Le message est ajouté tel quel, marqué comme lu. Erreurs : `no_account` (409), `bad_request` (400),
`too_large` (413).

### `POST /mail/pagesize` — session

| Paramètre | Filtre | Rôle |
|---|---|---|
| `size` | doit figurer dans `pageSizes` | nombre de messages par page |
| `back` | `safeBack` | retour |

Enregistre la préférence dans le compte. Le retour est nettoyé de son numéro de page (`withoutPage`) :
changer la taille ramène au début. Valeur hors liste → **400**.

### `POST /mail/trust` — session

Expéditeurs dont les images distantes s'affichent d'emblée.

| Paramètre | Rôle |
|---|---|
| `email` | adresse (minuscules, 254 caractères au plus, doit contenir `@`, sans espace ni `<>,;"`) |
| `action` | `remove` pour retirer ; toute autre valeur pour ajouter |
| `stay` | `safeBack` |

La liste est bornée à `maxTrustedSenders` : au-delà, **les plus anciennes entrées s'effacent**. Un retrait
renvoie vers `/settings`, un ajout vers `stay`.

### `POST /mail/mdn` — session

Accusé de lecture (RFC 3798).

| Paramètre | Rôle |
|---|---|
| `folder`, `uid` | message concerné |
| `action` | `send` ou `ignore` — toute autre valeur → **400** |
| `stay` | `safeBack` |

Le message doit effectivement demander un accusé, sinon **400**. La décision est notée sur le message avec le
mot-clé **`$MDNSent`** (RFC 3503) : elle n'est plus redemandée, et un message déjà traité ne renvoie rien.

### `GET /mail/poll` — session

Sondage périodique de la liste. **Répond en JSON**, `Cache-Control: no-store`.

| Paramètre | Rôle |
|---|---|
| `since` | UID à partir duquel chercher les nouveaux messages (entier 32 bits) |

Réponse :

```json
{
  "unseen":  12,
  "uidnext": 4312,
  "new":     [{"uid": 4311, "from": "…", "subject": "…"}],
  "folders": {"Dossier": 3},
  "accounts": {"<id de boîte>": 7}
}
```

- `folders` : non lus des dossiers surveillés, obtenus en **une seule commande** quand le serveur annonce
  `LIST-STATUS`.
- `accounts` : non lus des autres boîtes, au plus `maxPolledAccounts`, pour le sélecteur de comptes.
- Déclenche aussi la synchronisation CardDAV différée (`maybeSyncContacts`).
- Aucune boîte → **409** `{"error":"no_account"}` ; échec IMAP → **502** `{"error":"imap"}`.

---

## 8. Composition

### `GET /compose` — session

Ouvre la page de composition.

| Paramètre | Valeurs | Rôle |
|---|---|---|
| `mode` | `new` | message vierge (défaut) |
| | `reply` | réponse à l'expéditeur |
| | `replyall` | réponse à tous |
| | `forward` | transfert, corps cité |
| | `forwardatt` | transfert en pièces jointes (groupé) |
| | `draft` | reprise d'un brouillon |
| | `editasnew` | repartir d'un message existant |
| `folder`, `uid` | | message d'origine, requis si `mode != new` |
| `uids` | | transfert groupé : UID séparés par des virgules, chacun joint en `.eml` |
| `to`, `cc`, `bcc` | | destinataires préremplis |
| `subject`, `body` | | objet et corps préremplis |
| `mailto` | | URL `mailto:` analysée et répartie dans les champs |
| `back` | `safeBack` | retour |

Un `mode` hors liste → **400**. Choisir `reply`, `replyall` ou `forward` mémorise la préférence pour les
prochains messages.

### `POST /compose/send` — session

Envoie le message, en conserve une copie et marque le message d'origine.

| Paramètre | Rôle |
|---|---|
| `compose_id` | identifiant de la composition (relie les pièces jointes déposées) |
| `identity` | identité expéditrice |
| `to`, `cc`, `bcc` | destinataires |
| `reply_to` | adresse de réponse |
| `subject` | objet |
| `body` | corps |
| `is_html` | booléen : le corps est du HTML |
| `html` | corps HTML (quand les deux versions sont envoyées) |
| `important` | booléen : priorité haute |
| `receipt` | booléen : demander un accusé de lecture |
| `dsn` | booléen : demander un avis de remise |
| `kind` | `reply` ou `forward` : nature du message d'origine |
| `orig_folder`, `orig_uid` | message d'origine, à marquer « répondu » ou « transféré » |
| `in_reply_to`, `references` | en-têtes de fil de discussion |
| `draft_folder`, `draft_uid` | brouillon à supprimer une fois l'envoi réussi |
| `back` | `safeBack` |

Après l'envoi : copie dans le dossier Envoyés (sauf si la boîte le désigne comme absent), drapeau
`\Answered` ou `$Forwarded` sur l'original, suppression du brouillon.

### `POST /compose/save` — session

Enregistre un brouillon, **en remplaçant le précédent**. Mêmes paramètres que `/compose/send`.
Appelé automatiquement par l'éditeur si l'enregistrement automatique est actif.

### `POST /compose/attach` — session

Reçoit **une** pièce jointe par requête. **Répond en JSON.**

| Paramètre | Rôle |
|---|---|
| `compose_id` | composition à laquelle rattacher le fichier |
| `file` | fichier (`multipart/form-data`) |

Taille bornée par `compose.max_attachment_mb`.

### `POST /compose/detach` — session

| Paramètre | Rôle |
|---|---|
| `compose_id` | composition |
| `id` | identifiant de la pièce à retirer |

### `POST /compose/discard` — session

Abandonne la composition : pièces jointes oubliées, brouillon enregistré supprimé.

| Paramètre | Rôle |
|---|---|
| `compose_id` | composition |
| `draft_folder`, `draft_uid` | brouillon à supprimer |
| `back` | `safeBack` |

---

## 9. Réglages généraux

### `GET /settings` — session

Page des réglages, onglet par défaut.

### `GET /settings/security` — session

Même page, onglet « Sécurité » (mot de passe, clé de récupération, second facteur).

### `POST /settings/password` — session

| Paramètre | Rôle |
|---|---|
| `old_password` | mot de passe actuel |
| `password` | nouveau |
| `password2` | confirmation |

Soumis à la limitation `login`. Confirmation différente → **400**. Mot de passe actuel faux → **401**
(`settings.error.password`) et le compteur d'échecs augmente. Succès : **toutes les autres sessions du compte
sont fermées**, événement `password`.

### `POST /settings/recovery` — session

| Paramètre | Rôle |
|---|---|
| `password` | mot de passe actuel, pour confirmer |

Régénère la clé de récupération : **l'ancienne cesse immédiatement de fonctionner**. Événement
`recovery_new`, puis redirection vers `/recovery-key` pour afficher la nouvelle.

### `POST /settings/preferences` — session

Enregistre les préférences. Avec l'en-tête **`X-Requested-With: fetch`** (enregistrement à la volée par
`settings-auto.js`), répond `{"ok":true}` ou `{"ok":false}` ; sans lui, la page est réaffichée.

| Paramètre | Filtre | Rôle |
|---|---|---|
| `language` | doit exister dans `i18n` | langue de l'interface |
| `theme` | doit exister | thème |
| `timezone` | `time.LoadLocation`, 64 caractères au plus ; vide = fuseau du serveur | fuseau horaire |
| `layout` | `three`, `list`, `bottom` ; absent = inchangé | disposition |
| `font_size` | doit figurer dans `fontSizes` ; absent = inchangé | taille du texte |
| `page_size` | 10 … `maxPageSize` | messages par page |
| `idle_minutes` | 1 … `session.idle_minutes_max` | expiration par inactivité |
| `absolute_hours` | 1 … `session.absolute_hours_max` | durée maximale d'une session |
| `editor` | `userdata.ValidEditor` | éditeur par défaut |
| `avatars` | `none` ou `gravatar` | source des photos |
| `mark_read` | booléen | marquer comme lu à l'ouverture |
| `draft_autosave` | booléen inversé | enregistrement automatique des brouillons |
| `contacts_autosave` | booléen inversé | ajout automatique au carnet |
| `reply_same_folder` | booléen | répondre en restant dans le dossier |
| `threads` | booléen | regrouper en conversations |
| `show_checkboxes` | booléen inversé | cases à cocher dans la liste |
| `animations` | booléen inversé | animations |
| `favicon_count` | booléen inversé | compteur dans l'icône de l'onglet |
| `notify_desktop` | booléen | notifications du navigateur |
| `notify_sound` | booléen | son à l'arrivée |

Un seul champ invalide fait échouer l'ensemble (**400**) : la configuration n'est jamais à moitié enregistrée.
Les durées de session sont appliquées à la session en cours sans attendre la reconnexion.

Les champs marqués « booléen inversé » enregistrent une négation (`NoAnimations`, `NoDraftAutosave`…) : coché
dans l'interface = fonction active.

### `POST /settings/panes` — session

Largeur des volets, enregistrée par le script à la fin d'un glissement (`X-Requested-With: fetch`).

| Paramètre | Bornes | Rôle |
|---|---|---|
| `folders` | 160 … 480 px, ou `0` | largeur du panneau des dossiers |
| `list` | 240 … 1000 px, ou `0` | largeur de la liste |
| `hidden` | `0` ou `1` | panneau des dossiers replié |

`0` rétablit la valeur par défaut. **Les champs absents restent inchangés** ; une requête sans aucun champ
valide → **400**.

### `POST /settings/folder-open` — session

Mémorise le dépliage d'un niveau de l'arbre des dossiers, pour la boîte active.

| Paramètre | Rôle |
|---|---|
| `folder` | nom du dossier (`ValidFolderName`) |
| `open` | `1` déplié, `0` replié — toute autre valeur → **400** |

La liste est bornée à `maxOpenFolders` : au-delà, les plus anciennes entrées s'effacent.

---

## 10. Image de fond

### `POST /settings/background` — session

| Paramètre | Rôle |
|---|---|
| `file` | image (`multipart/form-data`) |

**JPEG ou PNG seulement**, reconnus par `image.DecodeConfig` — donc d'après le contenu, pas d'après le nom.
Dimensions et poids bornés (`maxBackground`, `maxBackgroundDim`). L'image est stockée **chiffrée**, comme le
reste du compte, précédée d'un octet de format (`j` ou `p`). Refus → **400** (`settings.background.invalid`).

### `POST /settings/background/delete` — session

Supprime l'image et remet le réglage à zéro. Aucun paramètre.

### `GET /user/background` — session

Sert l'image de fond déchiffrée. `Cache-Control: private, max-age=86400`. Type déduit de l'octet de format.
Absente → **404**.

---

## 11. Journal d'activité

### `GET /settings/activity` — session

Journal d'activité du compte : connexions réussies et échouées, refus de la limitation de débit, second
facteur refusé, déconnexions, changement de mot de passe, usage et régénération de la clé de récupération,
activation et retrait du TOTP, ajout et retrait d'une boîte, modification des filtres, export du carnet.
Avec date, adresse IP et navigateur tronqué.

Aucun paramètre.

L'affichage déclenche le **versement** de la file d'attente chiffrée (`.evt`) dans l'historique (`.act`) :
les événements écrits compte fermé — les échecs de connexion, justement — n'apparaissent qu'à ce moment.
Conservation **30 jours**, 2 000 entrées au plus.

**Ce journal ne peut pas être effacé depuis l'interface, volontairement.** Un intrus qui détient le mot de
passe est un utilisateur comme un autre : lui donner un bouton « effacer » reviendrait à lui offrir le moyen
de supprimer ses propres traces. Seule la rotation automatique retire des entrées ; pour tout effacer il faut
supprimer le compte. Voir `SECURITY.md` §9 bis.

---

## 12. Second facteur (TOTP)

### `GET /settings/2fa` — session

Page du second facteur : état, QR code pendant la mise en place, codes de secours.

### `POST /settings/2fa/start` — session

Tire un nouveau secret TOTP et le garde **en session seulement** (`session.TwoFactorSetup`). Sans effet si le
second facteur est déjà actif. Rien n'est enregistré à ce stade.

### `GET /settings/2fa/qr.png` — session

QR code du secret, en PNG, `Cache-Control: no-store`.

| Paramètre | Rôle |
|---|---|
| `again` | `1` = QR du secret révélé par `/settings/2fa/show`, valable le temps de `revealTTL` |

Sans secret en cours ni révélation valable : **404**.

### `POST /settings/2fa/enable` — session

| Paramètre | Rôle |
|---|---|
| `code` | code TOTP affiché par l'application |

Active le second facteur **seulement si le code est juste** : cela prouve que l'application est bien
configurée avant de verrouiller le compte. Génère les codes de secours (affichés une fois, stockés hachés) et
une `TrustKey`. Code faux → **400** (`twofactor.bad_code`), compteur d'échecs incrémenté. Événement `2fa_on`.

### `POST /settings/2fa/codes` — session

| Paramètre | Rôle |
|---|---|
| `code` | code TOTP ou code de secours |

Régénère les codes de secours : **les anciens cessent de fonctionner**.

### `POST /settings/2fa/show` — session

| Paramètre | Rôle |
|---|---|
| `code` | code TOTP ou code de secours |

Révèle de nouveau le secret et son QR, le temps de `revealTTL` : installer l'application sur un second
appareil sans tout réinitialiser.

### `POST /settings/2fa/disable` — session

| Paramètre | Rôle |
|---|---|
| `code` | code TOTP ou code de secours |

Retire le second facteur **et renouvelle la `TrustKey`** : tous les cookies « appareil de confiance » déjà
posés deviennent inutilisables. Événement `2fa_off`.

---

## 13. Dossiers

Les quatre routes acceptent `box` (identifiant de boîte) ; absent, c'est la boîte active (`settingsBox`).

### `GET /settings/folders` — session

Arbre des dossiers de la boîte : abonnement, création, renommage, suppression, dossiers système, dossiers
surveillés.

### `POST /settings/folders/action` — session

| Paramètre | Rôle |
|---|---|
| `box` | boîte concernée |
| `action` | `create`, `rename`, `delete`, `empty`, `subscribe`, `unsubscribe` |
| `name` | dossier visé (renommage, suppression, vidage, abonnement) |
| `parent` | dossier parent, pour `create` |
| `new_name` | nouveau nom, pour `create` et `rename` |
| `delim` | séparateur hiérarchique annoncé par le serveur |
| `return_to` | retour au panneau de gauche quand l'action en vient |

### `POST /settings/folders/system` — session

| Paramètre | Valeurs | Rôle |
|---|---|---|
| `box` | | boîte concernée |
| `sent` | dossier existant, `userdata.NoFolder`, ou vide | Envoyés |
| `drafts` | idem | Brouillons |
| `junk` | dossier existant ou vide (**pas** `NoFolder`) | Indésirables |
| `trash` | idem | Corbeille |
| `archive` | idem | Archive |

Chaque valeur doit désigner un dossier **existant et sélectionnable**, sinon **400** (`folders.err.system`).
`NoFolder` n'est accepté que pour Envoyés et Brouillons : on peut choisir de ne pas conserver de copie, mais
pas de se passer de corbeille.

### `POST /settings/folders/check` — session

| Paramètre | Rôle |
|---|---|
| `box` | boîte concernée |
| `check[]` | dossiers dont le compteur de non lus est relevé en arrière-plan, répétable |

Au plus `userdata.MaxCheckFolders` (sinon **400** avec `folders.err.check_max`). Chaque dossier doit exister
et être sélectionnable. Les doublons sont retirés.

---

## 14. Filtres (Sieve / ManageSieve)

Toutes ces routes acceptent `box`. Si la boîte a `sieve_disabled`, elles redirigent vers la page des filtres
sans rien tenter. Toutes passent par `sieveLimit` (limitation de débit propre à ManageSieve) et peuvent
répondre **502** si le serveur ne répond pas.

### `GET /settings/filters` — session

Liste des filtres, avec un résumé en langue de l'utilisateur, l'état d'activation et les modules annoncés par
le serveur. Signale un **script étranger** (`Foreign`) : un script que wmail-go n'a pas écrit et qu'il ne sait
pas relire comme une liste de filtres.

### `GET /settings/filters/edit` — session

| Paramètre | Rôle |
|---|---|
| `box` | boîte |
| `id` | filtre à modifier ; vide = nouveau filtre |
| `from` | préremplit une condition « expéditeur » |
| `subject` | préremplit une condition « objet » |

`from` et `subject` servent au bouton « filtrer les messages similaires » depuis un message.

### `POST /settings/filters/save` — session

| Paramètre | Valeurs | Rôle |
|---|---|---|
| `box` | | boîte |
| `id` | | vide = création |
| `name` | | nom du filtre |
| `enabled` | booléen | filtre actif |
| `match` | `Any`, `All` | conditions en « ou » / en « et » |
| `cond_field[]` | `From`, `Recipient`, `Subject`, `Header`, `Size` | champ testé |
| `cond_type[]` | `EqualTo`, `NotEqualTo`, `Contains`, `NotContains`, `Regex`, `Over`, `Under` | comparaison |
| `cond_value[]` | | valeur comparée |
| `cond_second[]` | | nom d'en-tête quand `cond_field=Header` |
| `action` | `None`, `MoveTo`, `Discard`, `Vacation`, `Reject`, `Forward` | action |
| `move_to` | | dossier cible, si `action=MoveTo` |
| `forward_to` | | adresse, si `action=Forward` |
| `reject_msg` | | motif du refus, si `action=Reject` |
| `vac_msg` | | message d'absence, si `action=Vacation` |
| `vac_subject` | | objet de la réponse d'absence |
| `vac_days` | | intervalle entre deux réponses |
| `vac_addrs` | | adresses à reconnaître comme siennes |
| `keep` | booléen | garder aussi une copie dans la boîte |
| `stop` | booléen | arrêter l'évaluation après ce filtre |
| `mark_read` | booléen | marquer comme lu |
| `confirm_overwrite` | booléen | confirmer l'écrasement d'un script étranger |

Les quatre tableaux `cond_*` sont lus **en parallèle, par index** ; au plus 61 conditions. Une condition dont
la valeur est vide est ignorée. Seuls les champs de l'action choisie sont lus : chaque action a ses propres
noms de champ.

Un **script étranger** présent sur le serveur provoque **409** avec `filters.err.foreign` tant que
`confirm_overwrite` n'est pas envoyé : wmail-go n'écrase pas en silence un script écrit ailleurs.
Nombre maximum de filtres → `filters.err.too_many`. Succès : événement `filters`.

### `POST /settings/filters/action` — session

| Paramètre | Valeurs | Rôle |
|---|---|---|
| `box` | | boîte |
| `id` | | filtre visé |
| `action` | `toggle` | activer / désactiver |
| | `delete` | supprimer |
| | `up` / `down` | changer l'ordre |

### `GET` et `POST /settings/filters/raw` — session

Script Sieve brut. Le `GET` l'affiche, le `POST` l'enregistre : une seule fonction sert les deux.

| Paramètre | Rôle |
|---|---|
| `box` | boîte |
| `raw` | script complet (POST) ; CRLF normalisés |
| `use_raw` | booléen : le script brut devient le script actif, au lieu de celui que l'interface compose |

Script au-delà de 200 Kio → **413** (`filters.err.too_large`). Le script est soumis au serveur, qui le valide :
une erreur de syntaxe est renvoyée telle qu'il la rapporte.

---

## 15. Modèles de message

### `GET /settings/templates` — session

| Paramètre | Rôle |
|---|---|
| `edit` | identifiant du modèle à ouvrir dans le formulaire |

### `POST /settings/templates/save` — session

| Paramètre | Rôle |
|---|---|
| `id` | vide = création, sinon modèle modifié |
| `name` | nom (sans retour à la ligne, `maxTemplateName` runes au plus) |
| `body` | contenu (`maxTemplateBody` octets au plus, CRLF normalisés) |

Nom ou contenu vide → **400** (`templates.err.invalid`). `id` fourni mais inconnu → **404**. Nombre maximum
atteint → **400** (`templates.err.full`). Un nouvel identifiant est tiré au hasard (8 octets).

### `POST /settings/templates/delete` — session

| Paramètre | Rôle |
|---|---|
| `id` | modèle à supprimer |

### `POST /settings/templates/reorder` — session

| Paramètre | Rôle |
|---|---|
| `order` | liste complète des identifiants, dans l'ordre voulu |

### `GET /settings/templates/body` — session

Renvoie le texte d'un modèle, pour l'insérer dans la composition. **Répond en JSON**, `no-store`.

| Paramètre | Rôle |
|---|---|
| `id` | modèle demandé |

Les variables de date sont remplacées au moment de la requête, dans le fuseau de l'utilisateur :
`{{DATE}}` → `2006-01-02`, `{{TIME}}` → `15:04`, et les motifs `moment` reconnus par `momentRe`.

- Modèle en texte brut : `{"text": "…"}`.
- Modèle mis en forme : `{"html": "…", "text": "…"}`, le HTML **assaini comme un message**
  (`BlockRemote: true`), le texte produit par conversion du HTML assaini — pour une composition en texte brut.

Modèle inconnu → **404**.

---

## 16. Carnet d'adresses

### `GET /contacts` — session

Page du carnet : recherche, étiquettes, pagination, fiche en cours d'édition, état de la synchronisation.

| Paramètre | Rôle |
|---|---|
| `q` | recherche |
| `tag` | filtre par étiquette |
| `page` | page de résultats (défaut 1) |
| `edit` | identifiant de la fiche à ouvrir |
| `new` | ouvrir une fiche vierge |
| `email` | adresse à préremplir avec `new` |
| `info` | message affiché : `saved`, `deleted`, `imported`, `added`, `syncsaved`, `synced` — toute autre valeur est ignorée |
| `n` | nombre d'éléments traités, affiché avec `info` |
| `skipped` | nombre d'éléments ignorés, affiché avec `info` |

`q`, `tag` et `page` sont **repris par toutes les routes POST du carnet**, pour revenir à la même vue après
l'écriture.

### `POST /contacts/save` — session

| Paramètre | Rôle |
|---|---|
| `id` | vide = création |
| `prefix`, `first`, `middle`, `last`, `suffix` | nom |
| `nick` | surnom |
| `org`, `title` | organisation et fonction |
| `birthday` | date de naissance |
| `notes` | notes (CRLF normalisés) |
| `photo` | photo |
| `tags` | étiquettes séparées par des virgules |
| `email[]`, `email_type[]` | adresses et leur type, en parallèle |
| `phone[]`, `phone_type[]` | téléphones et leur type |
| `url[]` | sites |
| `adr_type[]`, `adr_street[]`, `adr_city[]`, `adr_region[]`, `adr_code[]`, `adr_country[]` | adresses postales, lues en parallèle par index |

Chaque famille de valeurs est bornée à `2 × contacts.MaxValues`. Fiche vide → **400**
(`contacts.err.empty`) ; carnet plein → **400** (`contacts.err.full`) ; `id` inconnu → **404**.

### `POST /contacts/delete` — session

| Paramètre | Rôle |
|---|---|
| `id[]` | fiches à supprimer, répétable |

### `POST /contacts/add` — session

| Paramètre | Rôle |
|---|---|
| `email` | adresse |
| `name` | nom |

Ajoute l'expéditeur d'un message au carnet, puis ouvre sa fiche.

### `POST /contacts/compose` — session

| Paramètre | Rôle |
|---|---|
| `id[]` | contacts cochés |

Ouvre une composition adressée à ces contacts, **première adresse de chacun**.

### `POST /contacts/import` — session

| Paramètre | Rôle |
|---|---|
| `file` | fichier vCard ou CSV (`multipart/form-data`) |

Format choisi d'après le contenu : un fichier qui ne contient pas `BEGIN:VCARD` dans ses 4 premiers Kio est
traité comme du CSV, quelle que soit son extension. Taille bornée par `maxContactsImport`.

**Les doublons d'adresse sont ignorés**, pas fusionnés. Redirige vers `/contacts?info=imported&n=<ajoutés>&skipped=<ignorés>`.
Fichier illisible ou format non reconnu → **400** (`contacts.err.import`).

### `GET /contacts/export` — session

| Paramètre | Rôle |
|---|---|
| `format` | `csv` ; toute autre valeur donne vCard |
| `q` | n'exporter que les résultats de cette recherche |
| `tag` | n'exporter que cette étiquette |

Sans `q` ni `tag`, exporte tout le carnet, page par page. Réponse en pièce jointe (`contacts.csv` ou
`contacts.vcf`), `Cache-Control: no-store`. **Écrit l'événement `contacts_export`** dans le journal
d'activité, avec le nombre de fiches : un export est une sortie de données, elle doit laisser une trace.

### `GET /contacts/suggest` — session

Autocomplétion des champs de destinataires. **Répond en JSON**, `no-store`.

| Paramètre | Rôle |
|---|---|
| `q` | début de nom ou d'adresse, tronqué à 100 caractères |
| `all` | `1` = lister les contacts par page de 50 (sélecteur) au lieu de suggérer |

Sans `all`, renvoie au plus 20 suggestions, comme RainLoop. Chaque entrée :
`{"name": "…", "email": "…", "label": "…"}`.

### `POST /contacts/sync/save` — session

Réglages de la synchronisation CardDAV.

| Paramètre | Rôle |
|---|---|
| `enabled` | booléen |
| `url` | adresse du carnet distant |
| `user` | identifiant (200 caractères au plus) |
| `password` | mot de passe (500 au plus) ; **laissé vide, l'existant est conservé** |
| `clear_password` | booléen : vider réellement le mot de passe |
| `interval` | minutes entre deux synchronisations, ramené dans les bornes par `clampInterval` |

L'URL passe par `contacts.ValidSyncURL` et le **garde anti-SSRF** dès que la synchronisation est activée ou
qu'une adresse est saisie. URL refusée → **400** (`contacts.sync.err.url`). Identifiants trop longs → **400**
(`contacts.sync.err.invalid`). En cas d'échec d'écriture, les réglages précédents sont **restaurés**.

### `POST /contacts/sync/run` — session

Lance une synchronisation immédiate. Aucun paramètre propre. Redirige vers
`/contacts?info=synced&n=<tirés + poussés + supprimés des deux côtés>`. Échec → **502**
(`contacts.sync.err.run`).

---

## 17. Récapitulatif

| # | Méthode | Chemin | Accès |
|---|---|---|---|
| 1 | GET | `/healthz` | public |
| 2 | GET | `/static/` | public |
| 3 | GET | `/favicon.ico` | public |
| 4 | GET | `/themes/{name}/{file}` | public |
| 5 | GET | `/user.css` | public |
| 6 | GET | `/lang` | public |
| 7 | GET | `/login` | public |
| 8 | POST | `/login` | public |
| 9 | POST | `/login/2fa` | public |
| 10 | GET | `/register` | public |
| 11 | POST | `/register` | public |
| 12 | GET | `/recover` | public |
| 13 | POST | `/recover` | public |
| 14 | GET | `/recovery-key` | session |
| 15 | GET | `/recovery-key.txt` | session |
| 16 | POST | `/recovery-key` | session |
| 17 | POST | `/logout` | session |
| 18 | GET | `/` | session |
| 19 | GET | `/accounts` | session |
| 20 | GET | `/accounts/new` | session |
| 21 | GET | `/accounts/{id}/edit` | session |
| 22 | POST | `/accounts/save` | session |
| 23 | POST | `/accounts/select` | session |
| 24 | POST | `/accounts/reorder` | session |
| 25 | POST | `/accounts/{id}/move` | session |
| 26 | POST | `/accounts/{id}/delete` | session |
| 27 | GET | `/accounts/{id}/identities` | session |
| 28 | POST | `/accounts/{id}/identities` | session |
| 29 | POST | `/accounts/{id}/identities/{iid}/delete` | session |
| 30 | POST | `/accounts/{id}/identities/{iid}/move` | session |
| 31 | POST | `/settings/identities/reorder` | session |
| 32 | GET | `/mail/message` | session |
| 33 | GET | `/mail/pane` | session |
| 34 | GET | `/mail/body` | session |
| 35 | GET | `/mail/source` | session |
| 36 | GET | `/mail/print` | session |
| 37 | GET | `/mail/attachment` | session |
| 38 | GET | `/mail/attachments.zip` | session |
| 39 | GET | `/mail/image` | session |
| 40 | GET | `/mail/avatar` | session |
| 41 | GET | `/mail/poll` | session |
| 42 | POST | `/mail/action` | session |
| 43 | POST | `/mail/clear` | session |
| 44 | POST | `/mail/append` | session |
| 45 | POST | `/mail/pagesize` | session |
| 46 | POST | `/mail/trust` | session |
| 47 | POST | `/mail/mdn` | session |
| 48 | GET | `/compose` | session |
| 49 | POST | `/compose/send` | session |
| 50 | POST | `/compose/save` | session |
| 51 | POST | `/compose/attach` | session |
| 52 | POST | `/compose/detach` | session |
| 53 | POST | `/compose/discard` | session |
| 54 | GET | `/settings` | session |
| 55 | GET | `/settings/security` | session |
| 56 | POST | `/settings/password` | session |
| 57 | POST | `/settings/recovery` | session |
| 58 | POST | `/settings/preferences` | session |
| 59 | POST | `/settings/panes` | session |
| 60 | POST | `/settings/folder-open` | session |
| 61 | POST | `/settings/background` | session |
| 62 | POST | `/settings/background/delete` | session |
| 63 | GET | `/user/background` | session |
| 64 | GET | `/settings/activity` | session |
| 65 | GET | `/settings/2fa` | session |
| 66 | GET | `/settings/2fa/qr.png` | session |
| 67 | POST | `/settings/2fa/start` | session |
| 68 | POST | `/settings/2fa/enable` | session |
| 69 | POST | `/settings/2fa/codes` | session |
| 70 | POST | `/settings/2fa/show` | session |
| 71 | POST | `/settings/2fa/disable` | session |
| 72 | GET | `/settings/folders` | session |
| 73 | POST | `/settings/folders/action` | session |
| 74 | POST | `/settings/folders/system` | session |
| 75 | POST | `/settings/folders/check` | session |
| 76 | GET | `/settings/filters` | session |
| 77 | GET | `/settings/filters/edit` | session |
| 78 | POST | `/settings/filters/save` | session |
| 79 | POST | `/settings/filters/action` | session |
| 80 | GET | `/settings/filters/raw` | session |
| 81 | POST | `/settings/filters/raw` | session |
| 82 | GET | `/settings/templates` | session |
| 83 | POST | `/settings/templates/save` | session |
| 84 | POST | `/settings/templates/delete` | session |
| 85 | POST | `/settings/templates/reorder` | session |
| 86 | GET | `/settings/templates/body` | session |
| 87 | GET | `/contacts` | session |
| 88 | POST | `/contacts/save` | session |
| 89 | POST | `/contacts/delete` | session |
| 90 | POST | `/contacts/add` | session |
| 91 | POST | `/contacts/compose` | session |
| 92 | POST | `/contacts/import` | session |
| 93 | GET | `/contacts/export` | session |
| 94 | GET | `/contacts/suggest` | session |
| 95 | POST | `/contacts/sync/save` | session |
| 96 | POST | `/contacts/sync/run` | session |

**96 routes** : 6 publiques, 90 sous session.

---

## 18. Ce que wmail-go n'expose pas

- **Aucune API REST ni JSON générale.** Les seules réponses JSON (`/mail/poll`, `/mail/append`,
  `/compose/attach`, `/contacts/suggest`, `/settings/templates/body`, et `/settings/preferences` ou
  `/settings/panes` appelés en `fetch`) servent le script de l'interface et exigent la même session et le même
  jeton CSRF que le reste. Elles ne sont pas conçues pour un client tiers et peuvent changer sans préavis.
- **Aucune route d'administration.** La création, la suppression et la réinitialisation des comptes passent
  par la ligne de commande du binaire (`wmail-go user …`), jamais par HTTP.
- **Aucune route ne rend un chemin de fichier ni un contenu arbitraire du disque.** `/themes/{name}/{file}`
  valide ses deux composantes par expression rationnelle avant toute lecture.
- **Aucune route ne prend d'URL à charger fournie par l'utilisateur**, sauf `/mail/image`, dont l'URL est
  signée par le serveur et passe par le garde anti-SSRF, et la synchronisation CardDAV, dont l'adresse est
  validée à l'enregistrement et chargée par le même garde.
