# Sécurité de wmail-go

Document de référence : ce que le logiciel protège, comment, et ce qu'il ne protège pas. Il décrit le code tel
qu'il est (références `fichier:ligne` à l'appui), pas une intention. Dernière revue : 2026-10-24.

Sommaire : [1. Modèle de menace](#1-modèle-de-menace) · [2. Données au repos](#2-données-au-repos) ·
[3. Connexion et sessions](#3-connexion-et-sessions) · [4. Second facteur](#4-second-facteur-totp) ·
[5. Échanges réseau](#5-échanges-réseau-navigateur-serveur-imapsmtp) · [6. Affichage des messages](#6-affichage-des-messages-le-point-sensible-dun-webmail) ·
[7. Entrées et validation](#7-entrées-et-validation) · [8. Abus et déni de service](#8-abus-et-déni-de-service) ·
[9. Journaux et vie privée](#9-journaux-et-vie-privée) · [10. Ce qui manque](#10-ce-qui-manque-honnêtement) ·
[11. Déploiement](#11-déploiement-sûr) · [12. Signaler une faille](#12-signaler-une-faille)

---

## 1. Modèle de menace

**Ce qui est défendu**

| Adversaire | Ce qu'il tente | Réponse du logiciel |
|---|---|---|
| Qui lit le disque du serveur (sauvegarde volée, machine éteinte) | lire les mots de passe IMAP, les carnets d'adresses | tout le contenu utilisateur est chiffré, aucune clé n'est dérivable sans le mot de passe ou la clé de récupération (§2) |
| Qui écoute le réseau | capter le mot de passe ou le contenu | TLS obligatoire de bout en bout si vous déployez correctement (§5, §11) |
| Un site tiers visité par l'utilisateur | agir à sa place (CSRF), lire ses messages | cookie `SameSite=Strict`, jeton CSRF, contrôle `Sec-Fetch-Site`, CSP (§3, §7) |
| Un message piégé | exécuter du script, pister, exfiltrer | assainissement + `iframe` en bac à sable + CSP sans script (§6) |
| Qui devine un mot de passe | force brute | argon2id, limitation par IP et par identifiant, second facteur (§3, §8) |
| L'application elle-même utilisée comme rebond | scanner le réseau interne (SSRF) | garde sur toutes les connexions sortantes (§5) |

**Ce qui n'est pas défendu (dit franchement)**

- Un serveur **compromis en exécution** : les clés des sessions ouvertes sont en mémoire, un adversaire root les lit.
- Un **poste client compromis** (greffon de navigateur, enregistreur de frappe).
- Le **chiffrement de bout en bout** des messages : wmail-go lit le courrier en clair, comme tout webmail. Pas de
  PGP/S-MIME (une signature est seulement reconnue pour ne pas l'afficher en pièce jointe).
- La **métadonnée** : votre serveur IMAP voit tout ce que le webmail fait.

---

## 2. Données au repos

### Aucun mot de passe n'est stocké, nulle part

Il n'existe **pas de mot de passe maître**, pas de base de comptes, pas de condensat de mot de passe à voler. Le
mot de passe de l'utilisateur n'est jamais écrit sur le disque : il sert uniquement à **déchiffrer son fichier**.

Un utilisateur = un fichier `data/users/ab/cd/<identifiant>.cfg` (`internal/vault/vault.go`), de format :

```
"WMCF" | version | time(4) | memoryKiB(4) | threads(1) | sel(16)
       | emplacement mot de passe     : nonce(24) + DEK chiffrée(48)
       | emplacement clé de récupération : nonce(24) + DEK chiffrée(48)
       | nonce du contenu(24) | longueur(4) | contenu chiffré
```

- **DEK** (clé de données) : 32 octets aléatoires, chiffre le contenu en **XChaCha20-Poly1305**.
- La DEK est enveloppée deux fois : par une clé dérivée du **mot de passe** (argon2id, paramètres dans l'en-tête :
  64 Mio, 3 passes, 4 fils par défaut, sel de 16 octets, `internal/config/config.go`), et par une clé dérivée de la
  **clé de récupération** (HKDF-SHA256). Changer le mot de passe ne rechiffre que l'emplacement, jamais le contenu.
- Le contenu est **authentifié avec l'identifiant du fichier** : un `.cfg` copié sur le compte d'un autre
  utilisateur est rejeté (pas de substitution de fichier).
- Le mot de passe est normalisé en NFC avant dérivation (`vault.go:215`), pour qu'un accent saisi différemment
  ouvre quand même le compte.

Contenu chiffré : identifiants IMAP/SMTP/ManageSieve, réglages, identités, signatures, filtres, secret TOTP et
codes de secours. Les fichiers annexes (carnet d'adresses `.ctc`, fond d'écran `.bgd`) sont chiffrés avec une
**sous-clé dérivée de la DEK** (HKDF, étiquette par type) et authentifiés avec l'identifiant du compte
(`vault.go:428`).

### Le nom d'utilisateur lui-même n'est pas en clair

Le chemin du fichier vient d'un **HMAC-SHA256** du login avec une clé propre à l'instance
(`internal/store/store.go:170`) : lister `data/users/` ne donne pas la liste des adresses de vos utilisateurs.
L'index `users.idx` est chiffré avec une clé dérivée par HKDF (`store.go:93`).

### Clé de récupération

Engendrée à l'inscription, 32 octets aléatoires affichés en base32 par groupes de 5 avec somme de contrôle
(`vault.go:375-392`), **montrée une seule fois**. Elle ouvre le compte sans le mot de passe : à conserver comme un
mot de passe. La perdre **et** oublier son mot de passe = compte irrécupérable, par construction.

### Mémoire

La DEK est effacée (`Wipe`) à la fermeture de session et après un échec. Go ne garantit pas l'absence de copies
résiduelles : c'est un « au mieux », pas une promesse.

---

## 3. Connexion et sessions

### Déroulé d'une connexion (ce qui transite, et quand)

1. Le navigateur envoie `POST /login` : `login`, `password`, en TLS (le proxy termine le chiffrement).
2. Le serveur normalise le login, calcule son HMAC, lit le fichier `.cfg`. **Login inconnu : une dérivation
   argon2id factice est tout de même effectuée** (`store.go:381`, `vault.DummyDerive`) pour que le temps de réponse
   ne révèle pas l'existence du compte.
3. argon2id dérive la clé, ouvre l'emplacement, obtient la DEK, déchiffre le contenu. Échec = `401`, message
   identique quel que soit le motif.
4. Si le second facteur est actif, la session **n'est pas créée** : un jeton temporaire (cookie `Path=/login`,
   durée courte) attend le code (§4).
5. Sinon, une session est créée : identifiant de **32 octets aléatoires** (`crypto/rand`, base64url,
   `internal/session/session.go:50`), gardée **en mémoire seulement**.
6. Le mot de passe n'est **plus jamais** utilisé ensuite : la session porte la DEK, qui déchiffre à la volée les
   identifiants IMAP/SMTP.

Le mot de passe n'est ni journalisé, ni mis en cache, ni renvoyé au navigateur.

### Cookie de session

```
__Host-wmail_session=<32 octets aléatoires>; Path=/; HttpOnly; Secure; SameSite=Strict
```

- `HttpOnly` : inaccessible au JavaScript de la page.
- `SameSite=Strict` : jamais envoyé depuis un autre site (barrière CSRF principale).
- `Secure` + préfixe `__Host-` dès que la requête arrive en HTTPS (`internal/web/server.go`) : le navigateur refuse
  alors tout cookie de ce nom qui ne serait pas `Secure`, de portée `/` et sans domaine — un sous-domaine
  compromis ne peut pas en poser un à notre place.
- En HTTP (développement), le cookie reste sans préfixe et sans `Secure`, sinon il serait inutilisable.

### Peut-on voler ou rejouer une session ?

| Scénario | Réponse |
|---|---|
| Lire le cookie en JavaScript | non : `HttpOnly`, et aucune injection de script n'est possible dans la page (§6) |
| L'envoyer depuis un autre site | non : `SameSite=Strict`, plus jeton CSRF et contrôle `Sec-Fetch-Site` |
| L'intercepter sur le réseau | seulement si vous servez l'application en clair : déployez derrière TLS (§11) |
| Le rejouer après déconnexion | non : la session est supprimée de la mémoire côté serveur ; un identifiant volé devient inutile |
| Le deviner | 256 bits d'aléa ; aucun compteur, aucune information dérivée du compte |
| Le fixer avant la connexion (fixation de session) | non : l'identifiant est créé **après** l'authentification |
| Survivre à un changement de mot de passe | non : les autres sessions du compte sont fermées |
| Survivre à un redémarrage | non : les sessions sont en mémoire |

Expiration : inactivité (60 min par défaut, borne 480) **et** durée absolue (12 h, borne 72), réglables par
utilisateur dans les limites de `config.json`. Une session expirée est supprimée à la première requête suivante.

### Protection CSRF

Trois barrières indépendantes : `SameSite=Strict`, un **jeton CSRF** par session comparé en temps constant
(`subtle.ConstantTimeCompare`) sur **tout** POST, et le refus des requêtes annonçant `Sec-Fetch-Site: cross-site`
ou `same-site`. Aucune action modifiante n'est accessible en GET.

---

## 4. Second facteur (TOTP)

- TOTP standard (RFC 6238), secret de 20 octets aléatoires, pas de 30 s, fenêtre ±1 pas, 6 chiffres
  (`internal/totp/totp.go`).
- Comparaison en **temps constant**, et **un code ne sert qu'une fois** : le dernier pas accepté est mémorisé, tout
  pas inférieur ou égal est refusé (anti-rejeu).
- 9 codes de secours, stockés **hachés** (SHA-256 avec étiquette), consommés une seule fois.
- Entre le mot de passe et le code, un jeton temporaire limité dans le temps tient lieu d'état ; aucune session
  n'existe tant que le code n'est pas validé.
- « Appareil de confiance » : cookie signé, 2 semaines, révoqué par un changement de mot de passe.
- Revoir le secret après activation exige un code valide ; il reste affichable 2 minutes (le temps que le
  navigateur charge le QR) puis disparaît.
- Les essais de code sont soumis à la même limitation que la connexion.

**Limite à connaître** : le second facteur protège l'**accès au webmail**. Il ne protège pas le fichier chiffré :
qui a votre mot de passe et une copie du fichier peut l'ouvrir hors ligne. Le second facteur n'est pas un facteur
de chiffrement, c'est un contrôle d'accès.

---

## 5. Échanges réseau : navigateur ↔ serveur ↔ IMAP/SMTP

```
navigateur --TLS--> nginx --HTTP (réseau privé du conteneur)--> wmail-go --TLS--> IMAP / SMTP / ManageSieve
```

- **Navigateur ↔ serveur** : wmail-go ne fait pas de TLS lui-même ; la terminaison revient au proxy
  (`deploy/nginx.conf`, TLS 1.2/1.3). Le saut proxy → application reste sur le réseau interne du compose, sans port
  publié. `Strict-Transport-Security: max-age=31536000` est envoyé dès que l'application voit
  `X-Forwarded-Proto: https` **et** que `trusted_proxy` est actif.
- **Serveur ↔ IMAP/SMTP** : TLS implicite ou STARTTLS, **TLS 1.2 minimum**, certificat **vérifié par défaut**
  (`internal/mailconn/client.go:48`). Deux options par boîte, à n'utiliser qu'en connaissance de cause :
  « ne pas vérifier le certificat » (`InsecureSkipVerify`) et « TLS hérité » (abaisse le minimum à TLS 1.0 pour un
  vieux serveur). Les deux sont des décisions explicites de l'utilisateur, jamais par défaut.
- **Quand les identifiants IMAP circulent** : à chaque ouverture de connexion vers le serveur de courrier,
  c'est-à-dire la première fois qu'une page a besoin du serveur, puis au plus toutes les 5 minutes (durée de vie
  d'une connexion inactive dans le pool). Ils vont **uniquement** au serveur configuré, dans le tunnel TLS, par
  `LOGIN` ou `AUTHENTICATE PLAIN` quand le serveur refuse `LOGIN`. Ils ne transitent jamais vers le navigateur.
- **Homme du milieu** : impossible entre navigateur et proxy si vous utilisez un certificat valide (HSTS verrouille
  ensuite le navigateur) ; impossible entre l'application et votre serveur IMAP tant que la vérification du
  certificat reste active. Avec un certificat auto-signé, la première visite doit être acceptée à la main : faites-le
  sur un réseau de confiance, ou installez un vrai certificat.
- **Écoute réseau** : rien d'exploitable en clair, sauf si vous publiez le port 8080 de wmail-go directement.

### Anti-SSRF

Toutes les connexions sortantes (IMAP, SMTP, ManageSieve, mandataire d'images, CardDAV, Gravatar) passent par une
**garde** (`internal/mailconn/guard.go`) : résolution du nom, puis refus des adresses privées, de bouclage,
lien-local, multicast et non routables, **y compris après redirection**. `allowed_private_hosts` autorise
explicitement un hôte interne (c'est ainsi que le Dovecot de test est joignable). Sans cette garde, un utilisateur
pourrait faire scanner votre réseau interne par le serveur.

---

## 6. Affichage des messages : le point sensible d'un webmail

Un message est du contenu hostile par défaut. Trois barrières indépendantes :

1. **Assainissement serveur** (`internal/mailview/sanitize.go`, bluemonday + traitement propre) : suppression des
   `<script>`, `<iframe>`, `<object>`, `<embed>`, des gestionnaires `on*`, des URL `javascript:`, `data:` non
   images, `vbscript:`, des `<base>`, des `<form>`, `<button>`, `<select>`, `<textarea>` et de leur contenu ; filtrage des styles (aucun `position`, `z-index`,
   `behavior`, `expression`, ni `url()` distant non maîtrisé) ; réécriture des liens en `target=_blank` +
   `rel="noopener noreferrer"`. Le test `TestSecurityMessageScriptsNeutralised` sert un message réellement piégé et
   vérifie qu'aucun de ces éléments ne ressort.
2. **Document séparé** : le corps est servi par `/mail/body`, avec sa propre politique
   `default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'` **et** une
   directive `sandbox` sans `allow-scripts`. Même si un script passait, il ne s'exécuterait pas.
3. **Cadre isolé** : la page l'affiche dans une `<iframe sandbox="allow-popups allow-popups-to-escape-sandbox">`,
   sans `allow-scripts`, sans `allow-same-origin`. C'est ce qui produit le message « Blocked script execution… »
   dans la console du navigateur : la protection a fonctionné.

**Images distantes** : bloquées par défaut (option par boîte). Affichage sur décision explicite, globalement ou
« toujours pour cet expéditeur ». Un **mandataire** optionnel les charge côté serveur (votre adresse IP n'est pas
révélée à l'expéditeur), avec taille et délai bornés, derrière la garde anti-SSRF. Les **pixels espions** (images
de 1×1) sont retirés même quand les images sont autorisées.

**Page principale** : `Content-Security-Policy: default-src 'self'` (aucun script en ligne, aucune ressource
externe ; la composition ajoute `style-src 'unsafe-inline'` pour l'éditeur riche), `X-Frame-Options: DENY` +
`frame-ancestors 'none'`, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`,
`Permissions-Policy` refusant caméra, micro, position, paiement, USB…, `Cross-Origin-Opener-Policy` et
`Cross-Origin-Resource-Policy` en `same-origin`. Un test échoue si un gestionnaire d'événement en ligne réapparaît
dans un gabarit.

**Pièces jointes** : toujours servies en `Content-Disposition: attachment`, type MIME réécrit en
`application/octet-stream` dès qu'il ressemble à du HTML, XML, SVG ou JavaScript, `nosniff`, CSP `sandbox`, nom de
fichier nettoyé (pas de chemin, pas de caractère de contrôle). Seuls trois types sont affichés en ligne, après
**reconnaissance du contenu réel** (jamais de l'étiquette annoncée) : image, PDF, texte brut. Le PDF est servi avec
sa propre CSP et `X-Frame-Options: SAMEORIGIN` pour la visionneuse interne.

**Autres pièges couverts** : adresses `mailto:` réécrites vers notre composition, liens automatiques encadrés,
noms de fichiers engendrés sans collision, archives ZIP construites côté serveur, `X-Draft-Info` validé avant
usage, redirections internes filtrées (`safeBack`, `safeNext` : pas de redirection ouverte).

---

## 7. Entrées et validation

Principe : **tout ce qui vient du navigateur est hostile**, et tout ce qui vient du serveur IMAP aussi (un message
est écrit par un tiers).

- **Taille** : corps des requêtes borné par route, pièces jointes limitées par message et par session, message lu
  borné à 30 Mio, image décodée pour vignette bornée, lecture des parties MIME bornée, archive ZIP bornée.
- **Typage** : tous les paramètres sont analysés et validés (UID numérique, nom de dossier conforme IMAP sans
  caractère de contrôle ni `*` `%`, ports 1-65535, hôtes validés, taille de page bornée, langue et thème parmi une
  liste, fuseau vérifié par `time.LoadLocation`).
- **Injection IMAP** : les noms de dossiers et les critères passent par go-imap, qui encode les littéraux ; aucune
  commande n'est construite par concaténation de chaînes.
- **Injection d'en-têtes SMTP** : tout retour chariot, saut de ligne et octet nul est retiré des champs avant
  construction du message ; les adresses sont analysées, les domaines convertis en punycode.
- **Traversée de chemin** : aucun chemin ne vient de l'utilisateur ; les noms de fichiers reçus sont réduits à leur
  base, les thèmes et fichiers de thème validés par expression régulière.
- **Encodages** : charsets inconnus traités avec un repli explicite, UTF-8 validé, mots encodés RFC 2047 décodés
  (y compris entre guillemets) ; aucune donnée binaire n'est injectée telle quelle dans une page.
- **Sortie HTML** : `html/template` échappe par construction ; les seules valeurs non échappées sont des URI `data:`
  d'images que **nous** fabriquons, et le corps de message, qui vit dans son cadre isolé.

---

## 8. Abus et déni de service

- **Limitation de débit** (`config.json`, par IP **et** par identifiant) : connexion 10/15 min par IP et 5/15 min
  par compte, récupération 5/h et 3/h, inscription, test de boîte, envoi (60/h par IP, 30/h par compte), pages de
  filtres, photos de correspondants. Les compteurs sont en mémoire et balayés périodiquement.
- **argon2id** est volontairement coûteux (64 Mio) : le nombre de dérivations simultanées est borné
  (`max_concurrent_kdf`) pour qu'une rafale de connexions n'épuise pas la mémoire.
- **Bornes de travail** : tri côté serveur limité, nombre d'UID par action limité à 500, vignettes limitées par
  message, conversations limitées, cartes d'événements limitées, suggestions limitées.
- **Délais** : toute opération distante a une borne (`timeouts.mail_seconds` 30 s, `send_seconds` 90 s,
  `dial_seconds` 20 s) ; le serveur HTTP coupe à 5 minutes une requête abandonnée.
- **Déni de service distribué** : hors périmètre de l'application — c'est le rôle de votre proxy ou pare-feu.

---

## 9. Journaux et vie privée

- Les journaux vont sur la sortie standard (texte ou JSON). **Jamais** de mot de passe, de clé, de cookie de
  session, de contenu de message ni de pièce jointe.
- Le login apparaît **réduit** (`gd***`) ; l'adresse IP du client est journalisée pour les événements de sécurité
  (échec de connexion, refus CSRF, limitation).
- Les erreurs IMAP/SMTP sont tronquées avant journalisation (pas de dump de réponse serveur).
- Aucune télémétrie, aucune ressource tierce chargée par le navigateur. Gravatar, s'il est activé, est interrogé
  **par le serveur** : votre navigateur ne contacte jamais gravatar.com, et l'option est désactivée par défaut.
- CardDAV et ManageSieve ne sont contactés que si vous les configurez.

---

## 9 bis. Journal d'activité du compte (visible par l'utilisateur)

Distinct des journaux serveur ci-dessus : il est **chiffré pour le seul utilisateur**, qui le consulte dans
**Réglages → Activité**. Ni l'administrateur du serveur ni personne d'autre ne peut le lire.

**Le problème résolu.** Le reste du coffre ne s'ouvre qu'avec la clé de données, donc après une connexion réussie.
Or l'événement le plus utile — une tentative qui échoue — survient précisément compte fermé. D'où une paire de clés
X25519 par compte : la privée vit dans le fichier chiffré, la publique en clair à côté (`<id>.pub`). Le serveur
scelle un événement pour cette clé publique sans rien connaître du compte ; l'utilisateur seul le relit.

**Les fichiers.**

| Fichier | Rôle |
|---|---|
| `<id>.pub` | clé publique X25519, en clair — publier une clé publique ne coûte rien |
| `<id>.evt` | file d'attente, ajout seul, une « sealed box » par événement |
| `<id>.act` | historique compacté, chiffré par une sous-clé de la DEK, comme le carnet d'adresses |

Tout événement passe d'abord par `.evt`. Le versement dans `.act` a lieu quand la clé de données est disponible :
à la connexion et à l'affichage de la page. Conservation : **30 jours**, 2 000 entrées au plus.

**Ce qui est consigné** : connexion réussie, échec, refus par la limitation de débit, second facteur refusé,
déconnexion, changement de mot de passe, usage et régénération de la clé de récupération, activation/retrait du
TOTP, ajout/retrait d'une boîte mail, modification des filtres. Avec la date, l'adresse IP et l'agent utilisateur
tronqué. **Pas** les lectures de messages : trop de volume, et le journal deviendrait lui-même une cible.

**À la connexion**, un rappel annonce la connexion précédente et le nombre d'échecs survenus depuis. Il s'affiche
une fois par session.

**Limites, à lire avant de s'y fier :**

- **Ce journal n'est pas infalsifiable.** Qui a l'accès en écriture à `/data` détient la clé publique : il peut
  forger des entrées, et surtout en supprimer. Il détecte l'**abus de compte à distance**, pas la **compromission
  du serveur**. Les journaux serveur (§9) restent la source pour ce dernier cas.
- La file `.evt` est alimentée par des tentatives de connexion, donc par n'importe qui. Elle est **plafonnée**
  (400 entrées, 256 Kio) : au-delà, les entrées du milieu sont jetées et comptées, et le journal l'affiche. Sans ce
  plafond, un attaquant remplirait le disque.
- **Le journal ne peut pas être effacé depuis l'interface, volontairement.** Un intrus qui détient le mot de
  passe est un utilisateur comme un autre : lui donner un bouton « effacer » revient à lui offrir le moyen de
  supprimer ses propres traces, et le journal ne détecterait plus rien. Seule la rotation automatique retire des
  entrées, au bout de 30 jours.
- L'adresse IP est une donnée personnelle. Elle est ici chiffrée pour le seul utilisateur concerné et conservée
  30 jours au plus. Pour l'effacer plus tôt, il faut supprimer le compte (`user delete`), qui emporte `.act`,
  `.evt` et `.pub` avec lui.

---

## 10. Ce qui manque (honnêtement)

Comparé à ce qu'on attend d'un webmail en 2026 :

| Attendu | État | Commentaire |
|---|---|---|
| Chiffrement de bout en bout (PGP/S-MIME) | **absent** | choix assumé ; une signature est seulement reconnue pour ne pas l'afficher en pièce jointe |
| Clés de sécurité / WebAuthn (passkeys) | **absent** | TOTP seulement ; WebAuthn serait le prochain ajout utile |
| Journal des connexions visible par l'utilisateur | **présent** | chiffré pour l'utilisateur seul, 30 jours, échecs compris (§9 bis) |
| Liste des sessions actives, déconnexion à distance | **partiel** | un changement de mot de passe ferme les autres sessions ; pas de liste consultable |
| Expiration forcée du mot de passe, politique de complexité | **partiel** | longueur minimale configurable (12 par défaut), pas de règle de composition (choix délibéré) |
| Détection de connexion depuis un lieu inhabituel | **absent** | aucune géolocalisation, aucune heuristique |
| DKIM/SPF/DMARC à la réception | **partiel** | le résultat `Authentication-Results` du serveur est affiché s'il existe ; wmail-go ne vérifie pas lui-même |
| Signature DKIM à l'envoi | **absent** | c'est le rôle de votre serveur SMTP |
| Antivirus des pièces jointes | **absent** | rien n'est exécuté côté serveur ; l'analyse reste à votre MTA |
| Audit externe | **absent** | code jeune, relu par ses auteurs seulement |
| Verrouillage de compte après N échecs | **volontairement absent** | la limitation de débit est préférée (un verrouillage se retourne en déni de service contre l'utilisateur) |

Points de vigilance propres à ce déploiement :

1. `trusted_proxy: true` **sans** proxy devant : un client pourrait falsifier `X-Forwarded-For` et contourner la
   limitation par IP. Ne l'activez que derrière votre proxy.
2. « Ne pas vérifier le certificat » sur une boîte annule la protection contre l'homme du milieu pour cette boîte.
3. Les sessions vivent en mémoire : pratique (un redémarrage les invalide toutes), mais tout redémarrage déconnecte.
4. La clé de récupération est équivalente au mot de passe. Conservez-la hors ligne.

---

## 11. Déploiement sûr

1. **TLS obligatoire** : `make tls-up` (nginx + wmail-go, voir README), ou votre propre proxy. Ne publiez jamais le
   port 8080 de wmail-go.
2. `trusted_proxy: true` **et** `X-Forwarded-Proto` posé par le proxy : sans les deux, pas de cookie `Secure`, pas
   de HSTS, et l'IP réelle n'est pas vue.
3. `allow_registration: false` après avoir créé votre compte (`make tls-user-add LOGIN=…`).
4. Certificat valide (Let's Encrypt ou interne) plutôt qu'auto-signé, pour que HSTS ait un sens.
5. Sauvegardez le volume (`make backup`) : il contient les comptes chiffrés. Chiffrez aussi l'archive si elle sort
   de la machine.
6. Le conteneur tourne en utilisateur non privilégié, système de fichiers en lecture seule, toutes capacités
   abandonnées, `no-new-privileges`. Gardez-le ainsi.
7. Tenez les images à jour (`make tls-up` reconstruit) : c'est ce qui applique les corrections de sécurité de Go et
   des dépendances.

---

## 11 bis. Tenir les dépendances à jour

Le projet dépend d'une quinzaine de modules Go, de la bibliothèque standard de Go et de deux images de conteneur.
Une faille peut apparaître dans n'importe lequel **après** la dernière modification du code : la surveillance ne
peut donc pas se limiter aux contributions.

**Trois filets, automatiques :**

| Quoi | Quand | Où |
|---|---|---|
| `govulncheck` (failles connues de Go et des modules, **limitées à ce que notre code appelle réellement**) | à chaque poussée et chaque proposition | tâche `vuln` de l'intégration continue |
| `govulncheck` de veille | chaque lundi matin, et à la demande | `.github/workflows/vuln-hebdo.yml` ; ouvre un ticket étiqueté « sécurité » si une faille est atteignable |
| Dependabot (modules Go, actions, image de base) | hebdomadaire / mensuel | `.github/dependabot.yml` ; propose une contribution par mise à jour |

**À la main, quand vous le souhaitez :**

```bash
make vuln             # failles connues, atteignables depuis notre code
make deps-outdated    # dépendances directes en retard, sans rien modifier
make deps-update      # go get -u ./... + go mod tidy + construction + tests
make test-race test-js  # avant de commiter une mise à jour
```

**Doctrine retenue**

- Une faille **atteignable** (govulncheck la signale comme appelée) est traitée tout de suite, avant toute autre
  chose.
- Une faille **présente mais non atteignable** est notée, pas corrigée en urgence. Exemple actuel :
  `GO-2026-5932`, le paquet `golang.org/x/crypto/openpgp` est déclaré non maintenu ; il arrive par une dépendance
  transitive, notre code ne l'appelle jamais, et l'avis ne propose aucune version corrigée.
- Les mises à jour de confort sont groupées, jamais au fil de l'eau, et toujours suivies de la série complète de
  tests : une mise à jour non testée est un risque, pas une protection.
- La version de Go vient de `go.mod` (`go-version-file` dans l'intégration continue) et l'image de base est
  épinglée dans le `Dockerfile` : une seule ligne à changer pour monter de version, et `govulncheck` vérifie
  aussi les failles de la bibliothèque standard.

## 12. Signaler une faille

Écrivez à l'auteur du dépôt, de préférence avec une preuve de concept minimale. Merci de laisser un délai
raisonnable avant publication. Le projet est sous licence AGPLv3 : il est fourni sans garantie, mais les problèmes
de sécurité sont traités en priorité.

---

## Annexe : tests de sécurité automatiques

Lancés par `make test-race` (et à chaque modification) :

| Test | Ce qu'il vérifie |
|---|---|
| `TestSecurityMessageScriptsNeutralised` | un message piégé réel ressort sans `<script>`, `onerror`, `javascript:`, `<iframe>`, `<object>` ; CSP et bac à sable en place |
| `TestSecurityRemoteImagesAndTrackers` | images distantes bloquées, pixel 1×1 retiré même quand les images sont autorisées |
| `TestSecurityCSRFRejected` | POST sans jeton, avec un faux jeton, ou annoncé comme inter-sites → 403 |
| `TestSecurityPrivateNetworkBlocked` | serveur IMAP/SMTP sur adresse privée refusé, sauf autorisation explicite |
| `TestNoInlineEventHandlers` | aucun gestionnaire d'événement en ligne dans les gabarits |
| `TestSessionCookieSecureBehindProxy` | cookie `__Host-`, `Secure`, portée `/`, sans domaine derrière un proxy HTTPS |
| `TestSafeNext`, `safeBack` | pas de redirection ouverte |
| tests de `internal/vault` | format du fichier, mauvais mot de passe, fichier altéré, substitution d'identifiant |
| tests de `internal/mailview` | une quarantaine de charges utiles XSS (balises imbriquées, SVG, MathML, `javascript:` déguisé…) |
| tests de `internal/mailconn` | garde anti-SSRF, redirections, hôtes autorisés |
