# wmail-go – Cahier des charges (webmail en Go, successeur inspiré de RainLoop)

Document de travail. Il regroupe le contexte, les demandes, les décisions prises, les choix techniques proposés, les points de vigilance et les questions encore ouvertes.

Dernière mise à jour : 2026-10-23, fin de session : voir la section 9 « Reste à faire » (ajouts précédents : index des utilisateurs, emplacement de `server.key`, périmètre v1 « lecture », blocage du contenu distant par compte mail)

---

## 1. Contexte

### 1.1 Point de départ : RainLoop (édition Community / gratuite)

- Webmail PHP, client IMAP/SMTP, interface moderne en AJAX et responsive.
- Fonctionne sans base de données (fichiers plats dans `data/`). Les contacts peuvent utiliser SQLite, MySQL ou PostgreSQL.
- Licence AGPLv3 (édition Community). Des éditions payantes (Standard / Pro) existaient.
- Fonctions : multi-comptes, identités, filtres Sieve, thèmes, plugins, PGP, 2FA, multi-domaines, conversations en fils.
- Administration sur `/?admin`, identifiants par défaut `admin` / `12345` (à changer immédiatement).

### 1.2 Constat

- Projet abandonné : dernière version Community 1.17.0 (fin 2022), dépôt officiel non maintenu.
- Failles XSS connues (dont du XSS stocké, par exemple CVE-2022-29360 et des trouvailles de Sonar), sans correctif officiel.
- Compatibilité avec les versions récentes de PHP (8.x) fragile. Monter la version de PHP finit par casser RainLoop, et rester sur un vieux PHP signifie un PHP sans correctifs de sécurité.
- Le webmail ne peut donc plus suivre les évolutions ni les corrections de PHP. Point confirmé par l'utilisateur.

### 1.3 Décision de l'utilisateur

- RainLoop est utilisé tous les jours.
- L'utilisateur ne souhaite pas basculer sur SnappyMail (fork actif de RainLoop), ni sur une autre solution existante.
- Mesures de réduction du risque proposées en attendant (si RainLoop reste en service) :
  - Ne pas l'exposer directement sur Internet : VPN, ou reverse proxy avec authentification supplémentaire, restriction par IP, HTTPS et HSTS.
  - Changer les identifiants admin, bloquer `/?admin` hors IP locale, interdire l'accès web à `data/`.
  - Atténuer le XSS : en-tête `Content-Security-Policy` strict (à tester), cookies `Secure`, `HttpOnly`, `SameSite=Strict`, blocage des images distantes.
  - Figer la version de PHP, isoler dans un conteneur, utilisateur PHP-FPM dédié, `open_basedir`, `disable_functions` (`exec`, `system`, `shell_exec`, `passthru`).
  - Sauvegarder `data/`, surveiller les journaux, fail2ban sur le login, mots de passe mail uniques.

### 1.4 Idée retenue

Écrire un nouveau webmail avec le « look and feel » de RainLoop, en Go.

---

## 2. Objectifs et périmètre

- Serveur HTTP écrit en Go.
- Interface basée sur des templates HTML et une bibliothèque JavaScript légère.
- Pas de terminaison HTTPS dans le serveur : un logiciel en mode proxy HTTP (nginx, HAProxy, etc.) s'en charge.
- HTTP/2 : abandonné par l'utilisateur. Rappel technique : les navigateurs ne parlent HTTP/2 qu'en TLS. Derrière un proxy, la liaison navigateur vers proxy peut être en h2, et la liaison proxy vers Go reste en HTTP/1.1. Le h2c dans Go (`golang.org/x/net/http2/h2c`) n'apporte presque rien dans ce schéma.
- Un compte webmail gère plusieurs comptes mail IMAP/SMTP.
- Navigation compte par compte, comme RainLoop (pas de boîte de réception unifiée).

---

## 3. Faisabilité (évaluation)

Possible, mais projet conséquent. Aucun blocage technique.

**Facile** : serveur HTTP Go, `html/template`, sessions, routes, envoi SMTP, connexion IMAP basique.

**Moyen**
- IMAP avec `emersion/go-imap` (v2) : dossiers, liste, drapeaux, recherche, IDLE pour les nouveaux messages.
- Analyse MIME avec `emersion/go-message` ou `jhillyerd/enmime` : pièces jointes, multipart, encodages.
- Frontend : trois colonnes, mises à jour en AJAX. Candidats : htmx + Alpine.js, ou Svelte / Vue léger. Nouveaux messages via SSE ou WebSocket.
- Éditeur de composition riche : Squire, TipTap ou Quill.
- Contacts : SQLite.

**Difficile**
- Assainissement du HTML des messages (`bluemonday`, iframe en sandbox, CSP). C'est la source des XSS de RainLoop, à soigner dès le début.
- Messages réels mal formés : jeux de caractères, MIME cassé, `winmail.dat`, images en ligne (`cid:`), citations propres en réponse et transfert.
- Particularités IMAP selon le serveur (Dovecot, Gmail, Exchange, OVH…).
- Gros dossiers : pagination, tri et fils de discussion côté serveur (SORT / THREAD ou cache local).
- Stockage sûr des identifiants IMAP en session.
- Extras : Sieve (ManageSieve), PGP, plugins.

**Estimation grossière (un développeur)**
- MVP (connexion, dossiers, lecture, réponse, envoi, pièces jointes) : 1 à 2 mois à mi-temps.
- Usage quotidien confortable (recherche, brouillons, contacts, fils, robustesse) : 4 à 6 mois.

**Licence** : ne pas copier le CSS, les images ni le JavaScript de RainLoop (AGPLv3, la licence serait héritée). Recréer un design inspiré.

---

## 4. Spécifications fonctionnelles

### 4.1 Configuration globale (`config.json`)

Format JSON. Première option : autoriser ou non la création de compte utilisateur, `true` par défaut.

Exemple proposé :

```json
{
  "allow_registration": true,
  "listen": "127.0.0.1:8080",
  "data_dir": "./data",
  "trusted_proxy": true,
  "session_idle_minutes": 60,
  "max_users": 0,
  "block_private_networks": true,
  "allowed_private_hosts": [],
  "max_concurrent_kdf": 4,
  "password_min_length": 12,
  "session": {
    "idle_minutes_default": 60,
    "idle_minutes_max": 480,
    "absolute_hours_default": 12,
    "absolute_hours_max": 72
  },
  "rate_limit": {
    "login":    {"per_ip": "10/15m", "per_login": "5/15m"},
    "recovery": {"per_ip": "5/1h",   "per_login": "3/1h"},
    "register": {"per_ip": "5/1h"},
    "mail_test": {"per_ip": "30/10m", "per_login": "20/10m"}
  },
  "default_language": "fr",
  "default_theme": "light",
  "kdf": {"memory_mib": 64, "iterations": 3, "parallelism": 4},
  "log_level": "info",
  "log_format": "text"
}
```

- `allow_registration` : demandé par l'utilisateur, défaut `true`.
- `max_users` (0 = illimité) et `block_private_networks` : ajouts proposés (voir section 7).
- `trusted_proxy` : permet de lire `X-Forwarded-Proto` pour décider du drapeau `Secure` des cookies.

### 4.2 Compte utilisateur webmail

- À la création, l'utilisateur saisit uniquement un login et un mot de passe.
- La création produit un fichier de configuration utilisateur, chiffré avec le mot de passe, contenant toute la configuration du compte en JSON.
- Aucun mot de passe ni hachage du mot de passe webmail n'est stocké. Le succès du déchiffrement vaut authentification.
- Inscription libre par défaut. Elle peut être désactivée dans la configuration globale.
- Un outil en ligne de commande permet de créer des comptes, y compris quand l'inscription web est désactivée.

### 4.3 Connexion et session

- À la connexion, le serveur crée l'équivalent d'une session PHP avec les informations utiles au fonctionnement.
- Contenu proposé : login, configuration déchiffrée, clé de données (DEK) pour réécrire le fichier, compte mail actif, connexions IMAP ouvertes.
- Stockage en mémoire côté serveur. Un redémarrage du serveur invalide les sessions (acceptable).
- Identifiant de session aléatoire de 256 bits. Cookie `HttpOnly`, `SameSite=Strict`, `Secure` derrière un proxy HTTPS.
- Délai d'inactivité et durée maximale. Jeton CSRF sur toute action qui modifie l'état.

### 4.4 Comptes mail (IMAP/SMTP)

Un compte webmail contient plusieurs comptes mail. Structure proposée dans le JSON utilisateur :

```json
{
  "mail_accounts": [{
    "id": "uuid",
    "name": "Perso",
    "email": "moi@exemple.fr",
    "display_name": "",
    "imap": {
      "host": "", "port": 993,
      "security": "tls|starttls|none",
      "login": "", "password": "", "verify_cert": true
    },
    "smtp": {
      "host": "", "port": 587,
      "security": "starttls",
      "login": "", "password": "", "verify_cert": true
    },
    "options": {
      "block_remote_content": true,
      "image_proxy": false,
      "folders": {
        "trash": ""
      }
    }
  }]
}
```

Comportement demandé :

- Au premier login, l'utilisateur se voit proposer de configurer un premier compte mail avec un formulaire complet, à la manière de Thunderbird : adresse e-mail, serveur IMAP, protocole, port, login, mot de passe, etc., et la même chose pour SMTP.
- Un bouton « Test » valide la configuration et un bouton « Enregistrer » la sauvegarde.
- Tant qu'aucun compte mail valide n'est configuré, ce formulaire est proposé à chaque connexion. Proposition : « Enregistrer » exige un test réussi pour le premier compte.
- Si un compte mail est configuré, l'affichage des messages du compte suit la connexion.
- L'interface permet d'ajouter, modifier et supprimer des comptes mail.
- Le test = connexion et authentification IMAP et SMTP, sans envoi de message.
- Prévoir un champ d'authentification extensible (`password` / `oauth2`) pour une version ultérieure : Gmail et Microsoft 365 n'acceptent plus le mot de passe simple.
- Chaque compte mail porte ses propres options (`options`). Première option : `block_remote_content` (booléen, `true` par défaut), voir 4.8.

### 4.5 Navigation compte par compte

- Sélecteur de compte en haut du panneau gauche, dossiers du compte actif dessous.
- Compte actif mémorisé en session.
- Une connexion IMAP par compte, ouverte à la demande, fermée après inactivité.
- Envoi toujours via le SMTP du compte actif (identités multiples : version ultérieure).
- Écran Réglages : liste des comptes mail, ajout, modification, suppression, test.

### 4.6 Changement de mot de passe

Demandé et décidé : le changement de mot de passe est prévu.

1. Saisie de l'ancien mot de passe, puis du nouveau (deux fois).
2. Vérification : le déchiffrement du slot mot de passe avec l'ancien mot de passe réussit.
3. Nouveau sel aléatoire, nouvelle clé dérivée, nouveau slot mot de passe. Grâce à l'enveloppe de clé, le contenu n'est pas rechiffré.
4. Écriture atomique (fichier temporaire, `fsync`, `rename`).
5. Invalidation des autres sessions du même utilisateur (elles détiennent l'ancien slot).

### 4.7 Perte de mot de passe : clé de récupération (option 2 retenue)

Constat : sans mot de passe stocké, un mot de passe perdu rend le fichier illisible, y compris pour l'administrateur. Un reset par l'administrateur est rejeté (il exigerait de stocker une clé côté serveur, ce qui casse le principe).

Décision : **option 2, clé de récupération**.

- À la création du compte (web et CLI), génération d'une clé aléatoire de 256 bits, affichée une seule fois.
- Format lisible : 32 octets en base32 groupés (`XXXX-XXXX-…`) avec somme de contrôle contre les fautes de frappe.
- Bouton « J'ai noté ma clé » avant de continuer, téléchargement possible en `.txt`.
- Jamais stockée en clair. Seul le slot chiffré figure dans le fichier.
- Page « Récupération » : login + clé de récupération + nouveau mot de passe. Déchiffrement du slot récupération, création d'un nouveau slot mot de passe.
- Régénération de la clé depuis Réglages (mot de passe requis). Ancien slot récupération remplacé. À proposer aussi après usage de la récupération.
- Limitation de débit identique à celle du login. Message d'erreur unique.
- Avertissement à l'utilisateur : la clé de récupération donne un accès total, comme le mot de passe.

Options écartées : option 1 (aucune récupération) et reset administrateur.

### 4.8 Vues mail – périmètre v1 : lecture

Décision : la première version porte sur la **lecture**. La composition (éditeur riche ou texte brut), les brouillons et l'envoi avec pièces jointes sont traités dans une étape ultérieure. Les pièces jointes en réception (téléchargement) font partie de la lecture.

**Liste des messages**
- Pagination (taille de page configurable). Cible : UID IMAP, `SORT` / `ESEARCH` quand le serveur les gère, repli côté serveur Go sinon.
- Tri : date, expéditeur, sujet, taille, croissant ou décroissant.
- Recherche : sujet, expéditeur, destinataire, corps, tout. Portée : dossier courant (tous dossiers : plus tard). Traduite en `SEARCH` IMAP.
- Affichage : expéditeur, sujet, date, aperçu éventuel, drapeaux (lu / non lu, favori), indicateur de pièce jointe.
- Navigation par dossiers du compte mail actif (liste des dossiers, compteurs de non lus).

**Lecture d'un message**
- En-têtes principaux (de, à, cc, date, sujet) et pièces jointes listées, téléchargeables.
- Corps : version HTML assainie, ou texte brut s'il n'existe pas de HTML (ou au choix de l'utilisateur).
- Marquage automatique comme lu à l'ouverture, désactivable dans les réglages.

**Actions sur les messages (v1)**
- Marquer lu / non lu, favori.
- Supprimer : déplacement vers la corbeille du compte mail. Le dossier de corbeille est un **mappage IMAP paramétrable par compte mail** (`options.folders.trash`). Ordre de résolution : valeur explicite du compte, sinon dossier portant l'attribut IMAP `\Trash` (SPECIAL-USE, RFC 6154), sinon valeur par défaut `Trash`. Si le dossier résolu n'existe pas, il est créé automatiquement.
- Déplacer vers un dossier.
- Pas de répondre / transférer en v1 (ils dépendent de la composition).

**Dossiers (v1)** : consultation seule (pas de création, renommage ni suppression).

**Reporté après la v1** : composition, brouillons, envoi, pièces jointes à l'envoi, fils de discussion, aperçu du corps dans la liste, recherche multi-dossiers, gestion des dossiers, contacts, Sieve, PGP.

**Assainissement du HTML**
- Assainissement côté serveur avec `bluemonday` (liste blanche stricte) : pas de `script`, de gestionnaires d'événements, de `form`, d'`iframe`, d'`object` / `embed`, de `meta refresh`, de `base`.
- Rendu dans une `iframe` avec attribut `sandbox` (sans `allow-scripts`) servie depuis un point d'entrée dédié, avec sa propre CSP.
- Liens : `target="_blank"`, `rel="noopener noreferrer"`, schémas limités (`http`, `https`, `mailto`), URL réelle visible au survol.
- Images en ligne (`cid:`) : autorisées, elles viennent du message lui-même et sont servies par le serveur.

**Blocage du contenu distant**
- Bloqué par défaut. Option **par compte mail** : `block_remote_content` (voir 4.4), modifiable dans les réglages de chaque compte mail.
- Contenu concerné : images (`img`, `srcset`, `background`), feuilles de style et `url()` en CSS, polices, `@import`, audio, vidéo, et tout élément qui provoque une requête vers un hôte tiers (pixels espions inclus).
- Mise en œuvre : réécriture lors de l'assainissement, plus CSP de l'iframe (`img-src` limité au serveur, `data:` et `cid:`). Le blocage ne dépend pas que de la réécriture.
- Quand le blocage est actif : bandeau « Contenu distant bloqué » avec bouton « Afficher pour ce message » (dérogation ponctuelle, sans changer l'option du compte).
- Quand `block_remote_content` vaut `false` : le contenu distant est chargé par le navigateur de l'utilisateur. Cela expose son adresse IP et confirme l'ouverture du message à l'expéditeur. À mentionner dans l'interface.
- Proxy d'images côté serveur (masquage de l'IP de l'utilisateur) : **plus tard, hors v1**, et **en option par compte mail** (`options.image_proxy`, `false` par défaut). Le format de configuration le prévoit dès maintenant.

### 4.9 Préférences utilisateur, langues, thèmes, journalisation

**Préférences stockées dans le fichier chiffré de l'utilisateur** (clé `settings`) :

```json
{
  "settings": {
    "language": "fr",
    "theme": "light",
    "session_idle_minutes": 60,
    "session_absolute_hours": 12,
    "mark_read_on_open": true,
    "page_size": 50
  }
}
```

- Les durées de session sont réglables par l'utilisateur, mais bornées par `session.*_max` dans `config.json`. Valeurs par défaut : `session.*_default`.
- Si une valeur est absente ou invalide, la valeur par défaut de `config.json` s'applique.

**Langues** : français et anglais dès la v1. Textes dans des fichiers de langue (pas de texte en dur dans les templates), choix par utilisateur, langue par défaut dans `config.json`. Ajouter une langue = ajouter un fichier.

**Thèmes** : thème clair et thème sombre dès la v1. Le système doit permettre d'en ajouter d'autres plus tard :
- toutes les couleurs et espacements passent par des variables CSS ;
- un thème = un dossier (`themes/<nom>/` : `theme.css` + `theme.json` avec nom et description) ;
- thèmes de base embarqués dans le binaire ; dossier optionnel de thèmes externes dans `data_dir`, sans recompilation ;
- le thème sombre ne doit pas dupliquer le CSS des composants, seulement redéfinir les variables.

**Journalisation et administration**
- Pas de page d'administration au départ.
- Journaux sur la sortie standard du conteneur (format lisible ou JSON, niveau configurable) : démarrage, connexions, échecs d'authentification, limitation de débit, erreurs IMAP/SMTP.
- Ne jamais journaliser de mots de passe, de clés de récupération, ni de contenu de messages. Le login webmail est journalisé en forme réduite : deux premiers caractères suivis de `***` (par exemple `gd***`) ; un seul caractère pour un login d'un ou deux caractères.
- Administration par le CLI (`webmail user …`).

**Sécurité des mots de passe** : longueur minimale 12 caractères par défaut, valeur modifiable (`password_min_length`) ; contrôle appliqué à la création, au changement de mot de passe et à la récupération.

**Limitation de débit** : seuils modifiables dans `config.json` (`rate_limit`), par IP et par login. Valeurs par défaut retenues (notation `nombre/durée`, fenêtre glissante) :

| Action | Par IP | Par login |
| --- | --- | --- |
| Connexion | 10 / 15 min | 5 / 15 min |
| Récupération (clé) | 5 / 1 h | 3 / 1 h |
| Inscription | 5 / 1 h | sans objet |
| Test de compte mail (connexions sortantes) | 30 / 10 min | 20 / 10 min |

- Dépassement : réponse « trop de tentatives » avec délai croissant, pas de verrouillage dur (un attaquant ne doit pas pouvoir bloquer durablement le compte d'un autre).
- Compteur par login calculé sur le HMAC du login normalisé, y compris pour les logins inexistants (pas d'énumération de comptes).
- IP lue dans `X-Forwarded-For` uniquement si `trusted_proxy` est actif.
- `max_concurrent_kdf` (4 par défaut) limite le nombre de calculs argon2id simultanés, pour éviter un déni de service par épuisement de mémoire ou de CPU.
- Ces valeurs sont des points de départ, à ajuster à l'usage.

---

## 5. Spécifications techniques

### 5.1 Format du fichier utilisateur (`.cfg`) – enveloppe de clé

```
magic "WMCF" | version
slots :
  slot 0 (mot de passe) : sel, paramètres argon2id, nonce, DEK chiffrée
  slot 1 (récupération)  : nonce, DEK chiffrée par la clé de récupération
nonce du contenu | contenu JSON chiffré par la DEK (AEAD)
```

- `DEK` : 256 bits aléatoires, générée à la création.
- Slot mot de passe : `KEK = argon2id(mot de passe, sel)`, DEK chiffrée par la KEK (AEAD).
- Slot récupération : la clé est déjà aléatoire de 256 bits, donc une dérivation HKDF suffit (pas de KDF coûteux).
- AEAD : XChaCha20-Poly1305 ou AES-256-GCM.
- Les en-têtes (version, paramètres, slots) sont authentifiés comme données associées (AAD) du contenu : toute altération fait échouer le déchiffrement.
- Paramètres argon2id à régler (point de départ : 64 Mo, 3 passes). Longueur minimale de mot de passe conseillée : 12 caractères.
- Écriture atomique (fichier temporaire + `rename`), verrou par utilisateur.
- Avantages : changement de mot de passe sans rechiffrer le contenu, ajout du slot de récupération sans changer le format du contenu, autres slots possibles plus tard (comme LUKS).
- Limite connue : un fichier volé peut être attaqué hors ligne. Contre-mesures : argon2id coûteux et chemin dérivé d'un HMAC secret.

### 5.2 Arborescence hashée

Demande : les fichiers de configuration client chiffrés sont déposés dans une arborescence hashée.

- `h = hex(HMAC-SHA256(clé_serveur, login_normalisé))`
- Normalisation du login : minuscules, trim, Unicode NFC (sinon `Bob` et `bob` donneraient deux comptes).
- `clé_serveur` : secret aléatoire de 32 octets, généré au premier démarrage, stocké dans `data_dir/server.key` (droits `0600`). Un simple SHA-256 permettrait de deviner les logins par dictionnaire en lisant le disque.
- Chemin proposé : `data/users/ab/cd/abcdef0123…(64 hex).cfg`, deux niveaux de 2 caractères hex (256 × 256 dossiers), un fichier par utilisateur dans le dernier niveau.
- Dossiers créés à la demande, droits `0700`.
- Décision : `server.key` reste dans `data_dir` (`data_dir/server.key`), pas de chemin séparé. Conseil d'exploitation : le sauvegarder à part de `users/`.

**Index des utilisateurs (`data_dir/users.idx`)**
- Décision : un index chiffré par `server.key` (AEAD) liste les logins normalisés, pour permettre `webmail user list`.
- Mis à jour à la création et à la suppression d'un compte (web et CLI), écriture atomique, verrou fichier.
- Compromis : quiconque détient `server.key` et `users.idx` peut lire la liste des logins. L'index ne contient ni mot de passe, ni clé de récupération, ni contenu de compte.
- L'index doit rester cohérent avec `users/` : prévoir une commande de reconstruction impossible (les logins ne se déduisent pas des hachages), donc `users.idx` est à sauvegarder avec `server.key`.

Conséquences :

- Perdre `server.key` rend tous les comptes inaccessibles : l'inclure dans les sauvegardes, séparément de `users/`.
- Une sauvegarde de `users/` seule est inutilisable sans `server.key`.
- Le login est fixe : en changer exigerait de déplacer le fichier.
- Aucun login lisible sur le disque.

### 5.3 Outil en ligne de commande

Binaire unique avec sous-commandes :

```
webmail serve   -config config.json
webmail user add <login>        # mot de passe demandé sans écho (ou --password-stdin)
webmail user passwd <login>     # ancien + nouveau mot de passe
webmail user recover <login>    # clé de récupération + nouveau mot de passe
webmail user delete <login>
webmail user list               # lit users.idx
```

- `user add` utilise le même code que l'inscription web (même KDF, même format, même chemin hashé) et affiche la clé de récupération. Il lit `config.json` pour trouver `data_dir`.
- Jamais de mot de passe en argument (visible dans `ps`) : saisie interactive ou `--password-stdin`.
- Le CLI peut tourner en même temps que le serveur : verrou fichier et écriture atomique.
- `user delete` supprime le fichier. Les sessions actives de cet utilisateur restent valides jusqu'à expiration, car le CLI ne peut pas les purger dans un autre processus (à documenter).
- `user passwd` exige l'ancien mot de passe. Pas de reset administrateur : l'administrateur ne peut que supprimer et recréer, ce qui efface les comptes mail de l'utilisateur.
- `user list` : possible grâce à l'index chiffré `users.idx` (voir 5.2). Sans cet index, impossible avec le HMAC seul.

### 5.4 Bibliothèques envisagées

| Besoin | Bibliothèque |
| --- | --- |
| IMAP | `emersion/go-imap` (v2) |
| MIME | `emersion/go-message` ou `jhillyerd/enmime` |
| SMTP | `emersion/go-smtp` (client) ou `net/smtp` |
| Assainissement HTML | `microcosm-cc/bluemonday` |
| KDF et AEAD | `golang.org/x/crypto` (argon2, chacha20poly1305, hkdf) |
| Contacts | SQLite |
| Sieve (plus tard) | ManageSieve |

### 5.5 Environnement d'exécution et de développement : conteneurs

Demande : le webmail est embarqué dans un conteneur (`Dockerfile`) avec un `docker-compose.yaml` pour démarrer et tester sur le poste de travail. **Le développement et les tests se font dans des conteneurs, pas directement sur le poste de travail** (aucune installation de la chaîne Go sur l'hôte, aucun test lancé hors conteneur).

Proposition d'organisation :

- **`Dockerfile`** multi-étapes :
  - étape `dev` : image Go officielle, outils de développement (tests, lint, rechargement à chaud éventuel), utilisée pour compiler et tester ;
  - étape `build` : compilation statique (`CGO_ENABLED=0`), fichiers statiques et templates embarqués (`go:embed`) ;
  - étape finale `runtime` : image minimale (distroless ou Alpine), utilisateur non root, sans shell si possible.
- **`docker-compose.yaml`** :
  - service `webmail` : le serveur, port publié uniquement sur `127.0.0.1`, `config.json` monté en lecture seule, `data_dir` dans un volume nommé ;
  - service `dev` (profil `dev`) : code source monté en volume, pour compiler, lancer `go test ./...`, `go vet` et le CLI (`webmail user add …`) ;
  - service `mailserver` (profil `test`) : serveur IMAP/SMTP de test, pour ne pas dépendre d'une vraie boîte mail ;
  - service `proxy` (profil optionnel) : nginx ou HAProxy pour tester le fonctionnement derrière un proxy (`trusted_proxy`, cookies `Secure`).
- **Serveur de test IMAP/SMTP** : GreenMail (léger, IMAP + SMTP, boîtes créées à la volée) pour les tests automatisés ; pas de Dovecot dans le `docker-compose` : l'utilisateur teste lui-même, à la main, avec une vraie boîte sur un serveur Dovecot/Postfix. Le conteneur doit donc pouvoir joindre ce serveur (réseau, DNS) ; le serveur de l'utilisateur étant joignable en IP publique, `block_private_networks` peut rester actif ; `allowed_private_hosts` (voir 4.1 et section 7) reste disponible pour un serveur sur réseau privé. Jeux de messages de test (MIME difficile, HTML piégé, XSS, pixels espions) servis par ce serveur.
- **Durcissement du conteneur** `runtime` : utilisateur non root, système de fichiers en lecture seule (`read_only: true`), `tmpfs` pour `/tmp`, `cap_drop: [ALL]`, `no-new-privileges`. Seul `data_dir` est inscriptible.
- **Commandes** : un `Makefile` (ou script) enveloppe `docker compose` : `make up`, `make test`, `make shell`, `make user-add`, afin de ne jamais appeler `go` sur l'hôte.
- **Données** : `data_dir` dans un volume nommé. `server.key` et `users.idx` y vivent (voir 5.2) : à sauvegarder ensemble ; ne jamais les inclure dans l'image.
- **Moteur du poste de travail** : constaté le 2026-10-23, la commande `docker` est un émulateur de **Podman 5.7.0** (Docker Compose 2.40.3 comme fournisseur de compose). À prendre en compte : conteneurs probablement sans privilèges (rootless), correspondance des UID sur les volumes montés (`userns`, option `:U` ou `keep-id`), noms d'images complets (`docker.io/library/…`), ports inférieurs à 1024 non publiables sans réglage. Les fichiers `Dockerfile` et `docker-compose.yaml` doivent fonctionner avec Podman comme avec Docker.
- **Reproductibilité** : versions des images figées (tags précis, idéalement digest), dépendances Go verrouillées (`go.sum`).

---

## 6. Journal des décisions

| # | Décision | Statut |
| --- | --- | --- |
| 1 | RainLoop n'est plus maintenu, ne suit plus PHP : constat partagé | Acté |
| 2 | Pas de bascule vers SnappyMail | Acté |
| 3 | Écrire un nouveau webmail en Go avec le look RainLoop | Acté |
| 4 | Templates HTML + bibliothèque JS, serveur HTTP Go | Acté |
| 5 | Pas de TLS dans le serveur, proxy HTTP en frontal | Acté |
| 6 | HTTP/2 abandonné | Acté |
| 7 | Configuration globale en JSON, première option : autoriser la création de compte (défaut `true`) | Acté |
| 8 | Création de compte : login et mot de passe uniquement | Acté |
| 9 | Fichier de configuration utilisateur JSON, chiffré, sans stockage de mot de passe | Acté |
| 10 | Session côté serveur, équivalent d'une session PHP | Acté |
| 11 | Formulaire de premier compte mail à la Thunderbird, boutons Test et Enregistrer, répété tant qu'aucun compte valide | Acté |
| 12 | Un compte webmail gère plusieurs comptes mail ; ajout, modification, suppression | Acté |
| 13 | Fichiers chiffrés dans une arborescence hashée | Acté (détails proposés en 5.2) |
| 14 | Changement de mot de passe prévu | Acté |
| 15 | Navigation compte par compte (pas de boîte unifiée) | Acté |
| 16 | Inscription libre + outil CLI pour créer des comptes | Acté |
| 17 | Authentification par déchiffrement réussi, sans mot de passe ni hachage stocké | Acté |
| 18 | Perte de mot de passe : clé de récupération (option 2), enveloppe de clé | Acté |
| 19 | `user list` via un index chiffré par `server.key` (`users.idx`) | Acté |
| 20 | `server.key` dans `data_dir` | Acté |
| 21 | v1 = lecture : liste (pagination, tri, recherche) et lecture (HTML assaini) ; composition, brouillons, pièces jointes à l'envoi : plus tard | Acté |
| 22 | Contenu distant (images, etc.) bloqué par défaut, option `block_remote_content` par compte mail | Acté |
| 23 | Périmètre v1 détaillé validé : 3 colonnes ; liste paginée (50, tri, recherche dans le dossier) ; lecture HTML assaini ; actions marquer lu/non lu, favori, supprimer (corbeille), déplacer ; marquage lu automatique désactivable ; dossiers en consultation seule ; pas d'aperçu ni de fils en v1 | Acté |
| 24 | Livraison en conteneur (`Dockerfile` + `docker-compose.yaml`) ; développement et tests uniquement dans des conteneurs, jamais directement sur le poste de travail | Acté (détails proposés en 5.5) |
| 25 | Arborescence hashée : 2 niveaux de 2 caractères hex | Acté |
| 26 | Suppression d'un message = déplacement vers la corbeille ; mappage du dossier de corbeille avec valeur par défaut, paramétrable par compte mail | Acté (création automatique si absente, décision 38) |
| 27 | Proxy d'images : plus tard, option par compte mail | Acté |
| 28 | Frontend : htmx + Alpine.js, rendu serveur | Acté |
| 29 | Langues : français et anglais | Acté |
| 30 | Pas de page d'administration au départ ; journaux sur la sortie standard du conteneur ; outils en ligne de commande | Acté |
| 31 | Déploiement : ressources embarquées (`go:embed`), construction locale de l'image, rien d'autre à maintenir (choix « le plus simple à maintenir ») | Acté |
| 32 | Serveur IMAP/SMTP de test : GreenMail | Acté |
| 33 | Mot de passe webmail : 12 caractères minimum, valeur configurable dans `config.json` | Acté |
| 34 | Limitation de débit : seuils modifiables dans `config.json` | Acté |
| 35 | Durées de session paramétrables par l'utilisateur dans son compte webmail, avec défauts et bornes dans `config.json` | Acté |
| 36 | Thèmes : clair et sombre au départ, architecture permettant d'ajouter des thèmes | Acté |
| 37 | Nom du projet : ~~Rainmail~~ (abandonné le 2026-10-23, voir 48) | Remplacé |
| 38 | Corbeille absente : création automatique du dossier | Acté |
| 39 | Limitation de débit : valeurs par défaut proposées en 4.9, laissées au choix de l'assistant, modifiables dans `config.json` | Acté |
| 40 | Sessions : 60 min d'inactivité et 12 h maximum par défaut ; bornes configurables (480 min et 72 h proposées) | Acté |
| 41 | Look : reproduire l'esprit et la disposition de RainLoop (trois colonnes) pour le thème clair initial ; détails (couleurs, icônes) à préciser plus tard | Acté (détails ouverts) |
| 42 | Tests : GreenMail pour les tests automatisés ; test manuel de l'utilisateur sur une vraie boîte Dovecot/Postfix, hors `docker-compose` | Acté |
| 43 | Licence du code : **AGPLv3** (aucun code ni ressource de RainLoop ne doit être copié) | Acté |
| 44 | Journalisation du login webmail en forme réduite (deux premiers caractères + `***`) | Acté |
| 45 | Le serveur Dovecot/Postfix de test de l'utilisateur est joignable en IP publique : `block_private_networks` peut rester actif pour ce test | Acté |
| 46 | Vérification du nom « Rainmail » le 2026-10-23 : risque de confusion avec Rainmail Server (Carizen Software), produit existant du même secteur | Acté (conduit à la décision 48) |
| 47 | Étape 3 lancée : environnement conteneurisé (`Dockerfile`, `docker-compose.yaml`, `Makefile`, GreenMail), vérifié avec Podman le 2026-10-23 : `go test` et `go vet` dans le conteneur dev, image `runtime` (distroless, non root, lecture seule) healthy, GreenMail répond en IMAP (3143) | Fait |
| 48 | Nom du projet : **wmail-go** ; module Go `wmail-go`, binaire `wmail-go`, variable d'environnement `WMAIL_CONFIG`, images `localhost/wmail-go*`, projet compose `wmail-go` | Acté |
| 49 | Login : 3 à 64 caractères, sans espace ni caractère de contrôle, normalisé (NFC puis minuscules) | Mis en œuvre (choix de l'assistant, à valider) |
| 50 | Mot de passe normalisé en NFC avant la dérivation (même mot de passe saisi sous Linux, macOS ou Windows) ; 1024 octets au maximum | Mis en œuvre (à valider) |
| 51 | Après une récupération de compte, une nouvelle clé de récupération est générée et affichée automatiquement (l'ancienne a été utilisée) | Mis en œuvre (précise 4.7, à valider) |
| 52 | Après l'inscription, la récupération ou la régénération, l'écran de la clé est obligatoire tant que l'utilisateur n'a pas confirmé l'avoir noté | Mis en œuvre |
| 53 | Limitation de débit : blocage avec `Retry-After` (pas encore de délai croissant) ; connexion et récupération comptent les échecs, l'inscription compte chaque tentative ; un succès remet à zéro le compteur par login | Mis en œuvre (version simple de 4.9) |
| 54 | Protection CSRF : jeton par session sur les actions connectées, plus refus des POST dont `Sec-Fetch-Site` ou `Origin` indiquent un autre site | Mis en œuvre |
| 55 | IP du client : dernière entrée de `X-Forwarded-For` si `trusted_proxy` est actif (celle ajoutée par le proxy) | Mis en œuvre |
| 56 | `config.json` : toute clé inconnue est une erreur de démarrage (protège des fautes de frappe sur les options de sécurité) ; nouvelles clés `kdf`, `log_level`, `log_format` | Mis en œuvre |
| 57 | Go 1.26 (exigé par la version actuelle de `golang.org/x/crypto`) ; image `golang:1.26-alpine` | Mis en œuvre |
| 58 | CLI : `serve`, `healthcheck`, `user add / list / passwd / recover / delete`, `--password-stdin`, confirmation avant suppression ; service compose `cli` sur le même volume que le serveur | Mis en œuvre |
| 59 | Formulaire de compte mail : e-mail, IMAP et SMTP (hôte, port, sécurité TLS / STARTTLS / aucune, identifiant, mot de passe, vérification du certificat), options du compte, boutons Tester et Enregistrer ; aide à la saisie en JavaScript facultatif (suggestion `imap.` / `smtp.` + domaine, port selon la sécurité) | Mis en œuvre |
| 60 | Le premier compte mail n'est enregistré que si le test IMAP et SMTP réussit ; les suivants s'enregistrent sans test | Mis en œuvre (4.4) |
| 61 | Identifiants SMTP laissés vides = ceux de l'IMAP ; mot de passe laissé vide à la modification = conservé ; les mots de passe stockés ne sont jamais renvoyés dans une page | Mis en œuvre |
| 62 | Protection SSRF : résolution DNS puis connexion à l'adresse vérifiée (pas de rebond DNS) ; refus des adresses privées, locales, CGNAT, réservées, NAT64 et 6to4 ; exceptions par `allowed_private_hosts` (nom d'hôte, IP ou CIDR) | Mis en œuvre |
| 63 | Pas de repli en clair : STARTTLS demandé mais absent = échec ; erreurs classées (réseau interdit, DNS, refus, délai, TLS, authentification, protocole) avec détail court | Mis en œuvre |
| 64 | Limitation du bouton Tester : `rate_limit.mail_test` (30 / 10 min par IP, 20 / 10 min par login) ; maximum 20 comptes mail par utilisateur | Mis en œuvre (valeurs à valider) |
| 65 | Accueil : sélecteur de compte mail et liste des dossiers (compteurs de non lus, INBOX et dossiers spéciaux en premier, sous-dossiers groupés) ; sans compte mail, redirection vers le formulaire du premier compte | Mis en œuvre |
| 66 | Résolution de la corbeille (`ResolveTrash`) : valeur du compte, sinon attribut `\Trash`, sinon `Trash` ; création du dossier à faire lors de la suppression de message | Fonction prête, usage à venir |
| 67 | Liste des messages : pagination (taille de page des préférences), tri par date, expéditeur, sujet, taille ou arrivée, recherche dans le dossier (tout, sujet, expéditeur, contenu) ; SORT et SEARCH du serveur IMAP, repli du tri côté Go jusqu'à 5 000 messages sans SORT | Mis en œuvre |
| 68 | Pool de connexions IMAP par session et par compte mail : réutilisation, reconnexion automatique si la connexion est coupée, fermeture à la déconnexion et après 5 min d'inactivité ; un seul usage à la fois par connexion | Mis en œuvre |
| 69 | Lecture : le HTML est assaini (liste blanche d'éléments et de propriétés CSS sans `url()`), puis servi dans une iframe `sandbox` sans scripts ni same-origin, avec sa propre CSP (`default-src 'none'`) ; les images `cid:` deviennent des URI `data:` ; texte brut avec liens et citations | Mis en œuvre (4.8) |
| 70 | Contenu distant bloqué selon l'option du compte ; le bouton « Afficher pour ce message » ajoute `remote=1` à ce seul affichage, et la CSP du corps s'élargit en conséquence | Mis en œuvre (4.8) |
| 71 | Pièces jointes : téléchargement forcé (`attachment`), type de contenu limité aux types sûrs (HTML, XML, SVG, script remplacés par `application/octet-stream`), `nosniff` et `Content-Security-Policy: sandbox` | Mis en œuvre |
| 72 | Actions : marquer lu / non lu, favori, supprimer, déplacer ; en lot depuis la liste (500 messages au plus) et depuis la page d'un message ; supprimer = déplacer vers la corbeille (créée si absente), suppression définitive si le dossier est déjà la corbeille | Mis en œuvre (4.8) |
| 73 | Marquage comme lu à l'ouverture selon la préférence de l'utilisateur ; sans marquage, le message est consulté sans modifier ses drapeaux | Mis en œuvre |
| 74 | Les redirections après une action n'acceptent que la liste ou la page d'un message (pas de redirection ouverte) | Mis en œuvre |
| 75 | Les en-têtes qui ne sont pas de l'UTF-8 valide (serveurs ou expéditeurs non conformes) sont relus en ISO-8859-1 | Mis en œuvre |
| 76 | Outil de démonstration : `make mail-seed` dépose 44 messages variés (HTML avec images distantes et script, pièce jointe PDF, sujet piégé) dans GreenMail | Mis en œuvre |
| 77 | Filtre d'affichage « Tous / Non lus » (critère IMAP `UNSEEN`), conservé par la pagination, le tri, la recherche et le changement de dossier | Mis en œuvre (demande de l'utilisateur) |
| 78 | Sélecteur « Messages par page » dans la liste : 50 (défaut), 100, 200, 500 ; le choix est enregistré dans les préférences (les réglages acceptent de 10 à 500) | Mis en œuvre (demande de l'utilisateur) |
| 79 | Disposition **3 volets** par défaut (dossiers, liste, lecture), comme RainLoop ; option « liste seule » dans les réglages (le message s'ouvre alors sur une page) | Mis en œuvre (demande de l'utilisateur) |
| 80 | Volet de lecture sans JavaScript : un clic recharge la page avec `uid` dans l'adresse ; la ligne reste visible (`#mN`), le message est marqué lu avant le calcul des compteurs, la liste, le tri, la recherche et le filtre sont conservés | Mis en œuvre |
| 81 | Sélection multiple : « Tout sélectionner » (cases de la page affichée), Ctrl+A / Cmd+A, Maj+clic pour une plage, Échap pour effacer, compteur « N sélectionné(s) », boutons d'action grisés sans sélection ; sans JavaScript les cases restent utilisables | Mis en œuvre (demande de l'utilisateur) |
| 82 | Petits écrans (900 px et moins) : les volets s'empilent ; un message ouvert occupe l'écran avec un lien de retour à la liste | Mis en œuvre |
| 83 | Le panneau des dossiers reste visible sur la page d'un message (retour de l'utilisateur) | Mis en œuvre |
| 84 | Rechargement partiel du volet de lecture : un clic sur un message charge `/mail/pane` (fragment HTML) et remplace seulement le volet ; la liste, son défilement, la recherche et le tri ne bougent pas ; la ligne devient « lue » et les compteurs sont ajustés côté navigateur ; l'historique du navigateur est mis à jour ; sans JavaScript, ou en cas d'échec, navigation classique | Mis en œuvre (demande de l'utilisateur) |
| 85 | Les liens de lignes n'ont plus de fragment `#mN` (il faisait défiler la liste pour amener la ligne en haut) ; quand la page se recharge quand même (action, pagination), la position de défilement de la liste est conservée (stockage de session du navigateur) | Mis en œuvre (retour de l'utilisateur) |
| 86 | Volets redimensionnables : séparateurs à glisser (souris, tactile) ou au clavier (flèches, Maj pour 64 px), double-clic pour revenir au défaut ; bornes 160–480 px (dossiers) et 240–1000 px (liste), 320 px au moins gardés pour la lecture | Mis en œuvre (demande de l'utilisateur) |
| 87 | Largeurs des volets enregistrées dans la configuration chiffrée du compte (`folders_width`, `list_width`) par `POST /settings/panes`, et servies à chaque page par `GET /user.css` (variables CSS) : retrouvées sur tout navigateur et après reconnexion ; réinitialisables depuis les réglages | Mis en œuvre (demande de l'utilisateur) |
| 88 | Taille de base du texte réduite de 16 à 14 px (toute l'interface est en rem) et texte des messages de 14 à 13 px | Mis en œuvre (demande de l'utilisateur) |
| 89 | Ctrl+A / Cmd+A bascule : tout sélectionner, ou tout désélectionner si tout l'est déjà ; une sélection partielle devient une sélection complète | Mis en œuvre (demande de l'utilisateur) |
| 90 | Apparence calquée sur les captures de RainLoop de l'utilisateur : fond d'écran dégradé, barre d'outils (bouton vert « Nouveau », boutons clairs actualiser / déplacer / archiver / indésirable / supprimer / menu, adresse du compte et menu utilisateur), dossiers en blanc sur le fond avec pastilles de non lus, lignes compactes sur deux lignes (expéditeur, date, étoile, sujet, icône de pièce jointe), non lus sur fond jaune avec barre, ligne ouverte en bleu, pied de liste avec nombre de messages et pagination « 1 2 3 4 5 … N » | Mis en œuvre (demande de l'utilisateur) |
| 91 | Thème « rainloop » (défaut, `default_theme`), en plus de « light » et « dark » : tous les thèmes définissent le même jeu de variables CSS (fond, boutons, dossiers, liste, lecture, menus) | Mis en œuvre |
| 92 | Police de l'interface : Helvetica Neue / Helvetica / Arial, 11 px par défaut ; taille réglable par utilisateur de 10 à 16 px (préférence `font_size`, servie par `/user.css`), le corps des messages suit | Mis en œuvre (demande de l'utilisateur) |
| 93 | Dossiers : la boîte de réception et les dossiers à usage spécial (envoyés, brouillons, indésirables, corbeille, archives) passent en tête, en gras, séparés des dossiers de l'utilisateur ; ordre : réception, envoyés, brouillons, indésirables, corbeille, archives | Mis en œuvre |
| 94 | Barre d'outils : archiver, indésirable, supprimer, déplacer vers (menu des dossiers) et marquer lu / non lu / favori agissent sur les messages cochés, ou sur le message ouvert quand rien n'est coché (champ `current_uid`) ; dossier d'archives et d'indésirables : valeur du compte, sinon attribut IMAP, sinon `Archive` / `Junk`, créé si absent | Mis en œuvre |
| 95 | Liste : étoile cliquable sur chaque ligne, icône « répondu », sujet vide affiché en italique ; options de recherche (portée, filtre non lus) dans un menu sous le champ ; tri, filtre et taille de page dans le menu ≡ | Mis en œuvre |
| 96 | Lecture : en-tête (étoile, sujet, expéditeur cliquable, date, destinataires), bouton « i » pour les en-têtes complets, bouton de fermeture ; bandeau gris « Afficher les images » ; **pièces jointes au-dessus du corps** sous forme de cartes (miniature 36 px pour les images PNG, JPEG, GIF et WebP ; extension pour les autres fichiers ; nom tronqué ; taille), menu d'engrenage « Tout télécharger (.zip) » | Mis en œuvre (demande de l'utilisateur, 2e capture) |
| 97 | Miniatures calculées par le serveur (recadrage carré, 72 px) et incorporées en URI `data:` : aucune requête de plus ; au plus 12 par message, images de 8 Mo et 40 millions de pixels au plus ; SVG exclu | Mis en œuvre |
| 98 | Archive zip des pièces jointes : noms assainis (séparateurs remplacés, point de tête retiré) et dédoublonnés (« nom (2).ext ») | Mis en œuvre |
| 99 | Outil de captures d'écran (`tools/shot`, Chromium sans tête et puppeteer en conteneur) pour comparer l'interface aux captures de RainLoop | Mis en œuvre |
| 100 | Sources de RainLoop 1.17.0 de l'utilisateur montées en lecture seule dans `rainloop/` (sshfs) et lues comme **référence de dimensions et de structure** ; aucun fichier n'en est copié dans le projet (`rainloop/` est exclu de git et des images) | Mis en œuvre |
| 101 | Correction d'échelle : les captures de l'utilisateur sont réduites d'environ 0,58 (écran très large) ; mes proportions précédentes, calées sur l'image réduite, étaient trop petites. Valeurs réelles relevées dans le CSS de RainLoop : volet des dossiers 200 px, liste 500 px, ligne de message 52 px (texte 12 px, interligne 21 px), dossiers 34 px (texte 14 px), barre du haut 50 px, boutons 30 px, titre du message 18 px, vignette de pièce jointe 60 × 56 px | Mis en œuvre |
| 102 | Toutes les longueurs de l'interface courrier sont en rem (px de RainLoop ÷ 14) ; **taille de base par défaut 14 px**, comme RainLoop, réglable de 10 à 18 px dans les réglages (la préférence scale toute l'interface) | Mis en œuvre (remplace 92) |
| 103 | Thème « rainloop » = couleurs du thème **DarkShine** (fichier `styles.less` de RainLoop) : dossiers blancs, sélection `rgba(255,255,255,.3)`, ligne non lue `#ffffd9` avec barre orange, ligne ouverte `#dfefff` avec barre `#398cf2`, bouton vert `#84ab04`, menus `#48525c`, barres de liste en dégradé `#f4f4f4` vers `#dfdfdf` | Mis en œuvre |
| 104 | Fond d'écran : dégradé CSS qui approche DarkShine ; chaque utilisateur d'une instance peut poser sa propre image dans `<data_dir>/themes/rainloop/background.jpg` (ou `.png`, `.webp`), servie par `/themes/<nom>/background.*` ; `make theme-background FILE=image.jpg` la copie dans le volume | Mis en œuvre |
| 105 | Comptes mail dans le menu utilisateur en haut à droite (coche sur le compte actif), comme RainLoop, au lieu d'un sélecteur dans le volet des dossiers | Mis en œuvre |
| 106 | GreenMail de démonstration avec deux boîtes (`test@localhost`, `second@localhost`) pour essayer plusieurs comptes mail ; `make mail-seed` remplit les deux | Mis en œuvre |
| 107 | À refaire d'après RainLoop avant la composition : pages **Comptes**, **Filtres** et **Dossiers** des réglages ; réflexion sur la **gestion des contacts** à lancer ensuite (voir section 9) | À faire |
| 108 | Vérification du certificat des serveurs IMAP et SMTP : case propre à chaque serveur dans le formulaire du compte mail (cochée par défaut), avec un texte d'aide ; l'échec TLS du bouton Tester propose de la décocher ; test de bout en bout avec un certificat auto-signé | Mis en œuvre (demande de l'utilisateur) |
| 109 | « Vider la corbeille » et « Vider les indésirables » : lien en tête de liste (avec le nombre de messages) quand le dossier affiché est la corbeille ou le dossier des indésirables du compte, confirmation avant suppression définitive (`POST /mail/clear`) ; refus pour tout autre dossier et jamais pour la boîte de réception, même désignée comme corbeille ; un seul accès IMAP (`\\Deleted` sur `1:*` puis `EXPUNGE`) | Mis en œuvre (demande de l'utilisateur) |
| 110 | Option par serveur IMAP/SMTP « Autoriser les anciens protocoles et chiffrements TLS » (`legacy_tls`, désactivée par défaut, indépendante de la vérification du certificat) : TLS 1.0 minimum et suites de chiffrement sans confidentialité persistante (échange de clé RSA, CBC) que Go désactive par défaut. Cause constatée avec `mail.sdv.fr:993` : le serveur n'offre que des suites `TLS_RSA_*` (équivalent du `@SECLEVEL=1` d'OpenSSL). Lors du Test, si la négociation échoue puis réussit en mode ancien, l'erreur « le serveur n'accepte que d'anciens protocoles ou chiffrements » (catégorie `legacytls`) indique de cocher l'option ; une erreur de certificat n'est jamais masquée par ce diagnostic | Mis en œuvre (demande de l'utilisateur) |
| 111 | Mire de connexion, d'inscription et de récupération refaites comme celle de RainLoop : boîte unique centrée (verticalement et horizontalement, légèrement au-dessus du milieu), formulaire de 303 px + marges 40/40/20 px, champs de 40 px en police 18 px avec texte indicatif (les libellés restent lisibles par les lecteurs d'écran), bouton pleine largeur, liens sous la boîte ; couleurs DarkShine (fond `rgba(0,0,0,.5)`, ombre, rayon 7 px) portées par des variables de thème `--login-*` | Mis en œuvre (demande de l'utilisateur) |
| 112 | Journal des opérations IMAP : connexion (hôte:port, sécurité, durée), perte de connexion, et chaque opération du `Conn` (`select`, `dossiers`, `liste`, `message`, `drapeaux`, `deplacer`, `supprimer`, `vider`, `creer-dossier`) avec dossier, nombre de messages et durée. Les opérations qui modifient la boîte sont tracées au début (« imap début ») et à la fin, pour qu'une opération bloquée laisse une trace ; les lectures le sont en niveau `debug` (`info` si plus d'une seconde), les échecs en `warn`. Une requête HTTP encore en cours après 5 s est signalée (« requête lente en cours »). Jamais de mot de passe ni de contenu de message dans les traces. Les opérations SMTP seront tracées de la même façon avec la composition | Mis en œuvre (demande de l'utilisateur) |
| 113 | Déplacement de plus de 128 messages sur un serveur sans MOVE : le repli de la bibliothèque go-imap (COPY + STORE + EXPUNGE envoyés d'un coup) bloquait la lecture réseau (canal de 128 réponses EXPUNGE jamais vidé), et le délai de 30 s ne pouvait plus fermer la connexion. `Conn.Move` enchaîne maintenant COPY, STORE, EXPUNGE en attendant chaque réponse ; test de non-régression avec un serveur de test sans MOVE (400 messages). **Règle de travail : aucun test, aucune manipulation sur les vraies boîtes de l'utilisateur ; uniquement GreenMail** | Corrigé |

---

## 7. Points de vigilance (sécurité)

- **SSRF** : le bouton « Test » connecte le serveur à un hôte et un port choisis par l'utilisateur. Avec l'inscription ouverte, c'est un scanner de ports interne. Mesures : `block_private_networks` actif par défaut, plus `allowed_private_hosts` : liste blanche d'hôtes ou de réseaux privés autorisés explicitement (par exemple le serveur Dovecot de test de l'utilisateur sur le réseau local).
- **Inscription ouverte par défaut** : argon2 consomme CPU et mémoire, risque de déni de service et de remplissage du disque si l'instance est exposée. Mesures : limitation de débit par IP, `max_users`, désactivation après création des comptes d'une instance publique.
- **Force brute** : limitation de débit sur le login et la récupération, temps de réponse constant, même message d'erreur pour login inconnu, mauvais mot de passe, mauvaise clé ou fichier corrompu.
- **Mots de passe IMAP en clair en mémoire** pendant la session : inévitable, accepté.
- **HTML des messages** : assainissement, iframe en sandbox, CSP, images distantes bloquées par défaut (à confirmer).
- **OAuth2** : hors v1, mais prévoir l'extension du format.
- **Perte de `server.key`** : comptes inaccessibles, sauvegarde séparée à documenter.
- **Suppression d'un utilisateur par le CLI** : sessions actives non purgées.

---

## 8. Questions ouvertes

Résolues (décisions 19 à 37) : index des utilisateurs, emplacement de `server.key`, périmètre v1, contenu distant, arborescence hashée, corbeille, proxy d'images (plus tard), frontend, langues, journalisation, déploiement, serveur de test, politique de mot de passe, limitation de débit, durées de session, thèmes.

Résolues aussi (décisions 37 à 42) : nom du projet (voir 48), création automatique de la corbeille, limitation de débit, durées de session, look, tests.

Restent ouvertes :

1. **Nom « wmail-go »** (décision 48) : à vérifier avant publication. Le nom « WMail » existe déjà comme client Gmail de bureau (`Thomas101/wmail`, environ 2 000 étoiles, et son fork `openWMail`), sans rapport avec un webmail serveur. Le suffixe `-go` lève l'ambiguïté technique, mais un nom proche subsiste. Domaines `wmail-go.org`, `.dev`, `.com`, `.fr` et compte GitHub `wmail-go` sans trace visible (à réserver ; absence de DNS ne prouve pas la disponibilité). Registres de marques non consultés (INPI, EUIPO, USPTO).
2. **Look RainLoop** : détails du thème clair initial (couleurs, icônes, typographie), à préciser plus tard, par exemple avec des captures d'écran de l'instance RainLoop existante.
3. **Proximité avec « RainLoop »** : le nom évoque RainLoop (choix voulu) ; à garder en tête pour la communication du projet (ne pas laisser croire à un lien officiel).

---

## 9. Reste à faire (consolidé le 2026-10-23, fin de session)

> **Mise à jour de la nuit du 23 au 24 octobre** : tout ce que l'analyse ligne à ligne de RainLoop a fait ressortir est consigné au chapitre 14 et **réalisé** (contacts et CardDAV, double authentification, modèles, conversations, recherche avancée, sondage et notifications, glisser-déposer, pastilles d'adresses, visionneuse de pièces jointes, images distantes, fond d'écran, etc.). Les rubriques ci-dessous datent d'avant ; ce qui y figure encore et n'est pas au chapitre 14 reste à faire.

Cette section remplace la liste « étapes suivantes » des versions précédentes. Elle regroupe tout ce qui n'est pas fait, par thème, avec un ordre de priorité proposé. Rien ici n'est décidé définitivement : chaque ligne est à confirmer avant d'être lancée.

### 9.0 Réglages (demandes de l'utilisateur avant la composition)

**Fait** (décisions 123 à 125) : cadre des réglages, page Dossiers, identités, signature, dossiers envoyés/brouillons.
**Reste à faire** :
- **Filtres** : fait (décisions 130 à 134). Reste : essai sur un vrai serveur Pigeonhole (conteneur de test, puis boîte de l'utilisateur avec son accord), bouton « Test » du compte pour ManageSieve, consignes de filtre « répondre automatiquement » avec plage de dates.
- **Comptes** : réordonner les boîtes, boîte « principale », limites (déjà : maximum de boîtes) ; vérifier le comportement avec une deuxième boîte réelle (à faire avec l'utilisateur).
- **Contacts** : réflexion à mener (11.5) avant tout code.
- **Réglages généraux** manquants par rapport à RainLoop : animation de l'interface, notifications (son, navigateur), éditeur par défaut (avec la livraison B), regroupement en conversations.
- Identités : gestion dans les réglages faite ; l'adresse d'une identité n'est pas vérifiée auprès du serveur SMTP (un serveur peut refuser un expéditeur qu'il ne connaît pas : l'erreur de l'envoi est affichée).

### 9.1 Priorité 1 – composition et envoi

**Livraisons A et B faites** (décisions 120 à 129) : nouveau message, répondre, répondre à tous, transférer, transférer en pièce jointe, modifier comme nouveau, reprise de brouillon, envoi SMTP avec copie dans Envoyés, drapeaux, brouillons manuels et automatiques, pièces jointes avec glisser-déposer, signature, identité choisie automatiquement.
**Reste à faire** :
- Éditeur : coller des images (presse-papiers, glisser-déposer dans le texte), tableaux et caractères spéciaux (présents dans la barre de RainLoop), correcteur orthographique de l'interface (le navigateur fournit le sien), gestion du collage depuis Word (nettoyage).
- Livraison C : carnet d'adresses et autocomplétion des destinataires (voir 11.5 et 13), « ajouter automatiquement les destinataires », bouton Contacts de la composition 
- Transmettre plusieurs messages en pièces jointes depuis la liste ; « filtrer les messages similaires » ; conversations ; envoi différé ou annulable (option, à décider).
- Tests JS de la page de composition (jsdom) ; tests d'accessibilité (focus, libellés).

### 9.2 Priorité 2 – confort de lecture et de navigation (retours de l'utilisateur)

- **Actions du volet de lecture sans rechargement** : favori, supprimer, déplacer, archiver rechargent encore la page entière (la position de la liste est conservée).
- **Raccourcis clavier** : message suivant ou précédent, supprimer, archiver, marquer lu ou non lu, étoile, nouveau message, recherche. Prévoir une page d'aide.
- **Quota** : pourcentage d'occupation en pied de liste (IMAP QUOTA), comme dans RainLoop.
- **Sous-dossiers** : flèche pour déplier et replier, mémorisation de l'état.
- **Gestion des dossiers** : créer, renommer, supprimer, abonnements (vider la corbeille et les indésirables est fait, voir la décision 109 ; vider un dossier quelconque reste à décider).
- **Photo de l'expéditeur** : aujourd'hui une silhouette. À décider : Gravatar (fuite d'adresse vers un tiers), carnet d'adresses, ou rien.
- **Menu de réponse** (répondre, répondre à tous, transférer) à côté de la flèche.
- **Aperçu du corps** d'un message dans la liste (option).
- **Fils de discussion** (conversations) : SORT=REFERENCES ou THREAD côté IMAP.
- **Recherche multi-dossiers** et recherche avancée (dates, taille, pièces jointes, drapeaux).
- **Nouveaux messages en direct** : IMAP IDLE avec mise à jour de la liste et des compteurs, notification de bureau (option).
- **Vue 3 volets** : encore à comparer en détail avec les captures de RainLoop (couleurs exactes, espacements, tailles de police) ; une passe de mise au point est prévue après essai par l'utilisateur.
- **Liste seule** : vérifier que toutes les nouveautés (barre d'outils, étoile, pagination) s'y comportent bien.
- Fuseau horaire par utilisateur (les dates s'affichent en UTC dans le conteneur).

### 9.3 Priorité 3 – fonctions du webmail complet

- **Contacts** : carnet d'adresses (SQLite chiffré avec le reste du compte), autocomplétion à la saisie, import et export vCard. Le bouton existe, désactivé.
- **Filtres Sieve** (ManageSieve), réponse automatique d'absence.
- **PGP** et S/MIME (lecture, signature, chiffrement).
- **OAuth2** pour Gmail et Microsoft 365 (le champ d'authentification extensible est prévu dans le format).
- **Proxy d'images** par compte mail (l'option `image_proxy` existe dans le format, non implémentée) : masquer l'IP de l'utilisateur quand il autorise le contenu distant.
- Double authentification de l'accès au webmail (TOTP).
- Impression d'un message, enregistrement en `.eml`, affichage des en-têtes bruts.
- Marqueurs ou étiquettes (mots-clés IMAP).

### 9.4 Sécurité et exploitation

- **Sessions et CLI** : `user delete` et `user passwd` n'invalident pas les sessions déjà ouvertes dans le serveur (autre processus). Prévoir un canal de signal (fichier ou socket) ou une vérification de version du fichier à chaque requête.
- **Limitation de débit** : compteurs en mémoire, remis à zéro au redémarrage ; pas de délai croissant (blocage avec `Retry-After` seulement).
- **Adresse IP du client** derrière Podman rootless sans proxy : la passerelle du réseau est vue à la place du client. Documenter le montage avec proxy et `trusted_proxy`.
- **Hôtes publics** : la garde SSRF n'interdit pas les ports non standard vers des adresses publiques. Liste de ports autorisés à étudier.
- **Paramètres argon2id** (64 Mo, 3 passes) : à durcir selon la machine cible, avec une commande de mesure.
- **Migration de format** : numéro de version du fichier utilisateur et du schéma JSON, outil de migration, procédure de sauvegarde et de restauration (`server.key`, `users.idx`, `users/`).
- **Rotation de `server.key`** : non prévue (changer la clé rend les comptes inaccessibles) ; à documenter ou à concevoir.
- Journalisation : niveaux, rotation, champs à ne jamais écrire (déjà respecté pour les mots de passe), format JSON pour l'agrégation.
- Image de production : publication éventuelle, signature, analyse des vulnérabilités, `HEALTHCHECK` déjà présent.
- Documentation d'exploitation : `README` (installation, configuration complète, proxy nginx et HAProxy avec les en-têtes `Host`, `X-Forwarded-For`, `X-Forwarded-Proto`, sauvegardes, mise à jour).
- En-tête de licence AGPLv3 dans chaque fichier source et fichier `NOTICE` des dépendances.
- Dépôt de sources : le dossier n'est pas encore un dépôt git (à initialiser, avec `.gitignore` déjà présent).

### 9.5 Qualité, tests et dette technique

- **Essai en navigateur réel** : les comportements JavaScript (glisser des séparateurs, Ctrl+A, volet de lecture partiel) sont vérifiés dans jsdom et par captures d'écran, jamais à la main dans un navigateur ni sur d'autres navigateurs (Firefox, Safari). À faire par l'utilisateur, puis automatiser avec un test de bout en bout (Playwright).
- **Accessibilité** : navigation au clavier de toute l'interface, rôles ARIA des menus, contrastes des trois thèmes, lecteur d'écran.
- **Internationalisation** : relecture des textes anglais, pluriels (« 1 message(s) »), ajout d'autres langues.
- **Petits écrans** : la mise en page empilée existe mais n'a pas été éprouvée sur un vrai téléphone.
- Mise à jour du **cache des messages** : la page d'un message, son corps et chaque pièce jointe retéléchargent et analysent le message complet (30 Mo au plus) ; un cache court par session éviterait des allers-retours.
- Suppression définitive sans UIDPLUS : purge tous les messages marqués `\Deleted` du dossier.
- Tri sans SORT : limité à 5 000 messages.
- Feuilles de style `<style>` des courriels retirées (mise en page dégradée pour certaines newsletters) : étudier une réécriture sûre des sélecteurs.
- `make clean` peut échouer avec Podman rootless (processus réseau) ; le rendre robuste.
- Refactorisation : `internal/web/mail.go` est devenu long (liste, lecture, actions, pièces jointes, zip) ; le découper. Le script `list.js` regroupe quatre blocs : le séparer en modules.
- Tests de balisage fragiles : ils dépendent de fragments HTML précis ; en passer une partie sur des attributs `data-` stables.
- Intégration continue (construction, tests, `make test-js`, analyse statique).

### 9.6 Projet et décisions en attente

- **Nom du projet** : « wmail-go » est provisoire. Réserver le dépôt, le domaine et le compte, consulter les registres de marques (INPI, EUIPO, USPTO), vérifier la proximité avec « WMail » (client Gmail de bureau) et « RainLoop ».
- Page d'administration : écartée au départ (journaux et CLI) ; à reconsidérer si le nombre d'utilisateurs grandit.
- Valeurs par défaut à valider : limites de débit, bornes de session, 20 comptes mail par utilisateur, 12 miniatures par message, tailles de page proposées.
- Thème : valider la palette « rainloop » ; thèmes externes (`data_dir/themes/`) documentés mais sans guide pour en écrire.
- Choix de l'éditeur de texte riche pour la composition (Squire, TipTap, Quill) et de l'approche (rendu serveur avec htmx et Alpine.js, décidée mais non utilisée : tout est en JavaScript sans bibliothèque pour l'instant).

### 9.7 Ordre de travail proposé

1. Essai par l'utilisateur de l'interface actuelle et retours (apparence, volets, sélection), avec une seconde boîte mail ajoutée à son compte.
2. Pages Comptes, Filtres et Dossiers des réglages, puis réflexion sur les contacts (9.0).
3. Composition et envoi (9.1), avec brouillons et pièces jointes.
4. Raccourcis clavier, actions sans rechargement, quota, sous-dossiers (9.2).
5. Contacts et autocomplétion (9.3).
6. Durcissement et exploitation : sessions et CLI, migration de format, documentation, dépôt git, intégration continue (9.4, 9.5).
7. Nom définitif du projet et publication.

---

## 10. État d'avancement – socle (2026-10-23)

### 10.1 Réalisé et vérifié

Tout est exécuté dans des conteneurs (Podman, compatible Docker). Résultats :

- `make test-race` : tous les paquets passent avec le détecteur de concurrence ; `make vet` et `gofmt` sans remarque.
- Essai de bout en bout dans les vrais conteneurs : inscription web (HTTP 303 vers l'écran de clé), clé de récupération affichée, compte créé par la CLI dans un second conteneur partageant le volume, connexion web de ce compte, journaux sur la sortie standard avec login réduit (`Al***`, `bo***`), aucun login en clair dans les fichiers du volume, droits `0600` / `0700`.

| Élément du cahier des charges | Où |
| --- | --- |
| Configuration globale, validation, bornes de session | `internal/config` |
| Fichier `.cfg` : DEK, slots mot de passe et récupération, argon2id, XChaCha20-Poly1305, clé de récupération en base32 avec somme de contrôle | `internal/vault` |
| Arborescence hashée (HMAC-SHA256 avec `server.key`), `users.idx` chiffré, verrou `flock`, écriture atomique | `internal/store` |
| Contenu JSON utilisateur (préférences, comptes mail : structure prête) | `internal/userdata` |
| Sessions en mémoire (durée d'inactivité et maximale, effacement de la clé à la fermeture) | `internal/session` |
| Limitation de débit par IP et par login (fenêtre glissante) | `internal/ratelimit` |
| Français et anglais (fichiers de langue, vérification que les deux ont les mêmes clés) | `internal/i18n` |
| Pages : connexion, inscription, récupération, clé de récupération, accueil, réglages (mot de passe, clé, préférences) ; thèmes clair et sombre en variables CSS, thèmes externes dans `data_dir/themes/` | `internal/web` |
| CLI et serveur | `cmd/wmail-go` |

**Étape 4 (comptes mail, même jour)** :

| Élément | Où |
| --- | --- |
| Garde réseau anti-SSRF, connexions IMAP et SMTP (TLS, STARTTLS, sans chiffrement), tests, liste des dossiers, classement des erreurs | `internal/mailconn` |
| Serveurs IMAP et SMTP en mémoire pour les tests (chiffrement auto-signé) | `internal/mailconn/mailtest` |
| Pages : liste des comptes mail, formulaire (Tester / Enregistrer), suppression avec confirmation, sélecteur de compte, accueil avec dossiers | `internal/web` (`accounts.go`, modèles `accounts*.html`, `home.html`, `static/accounts.js`) |

Vérifié de bout en bout dans les conteneurs avec GreenMail : test IMAP et SMTP réussis, `127.0.0.1` et un nom de service interne refusés par la garde, premier compte enregistré, liste des dossiers affichée, aucun mot de passe dans les journaux.

**Étape 5 (liste, lecture, actions, même jour)** :

| Élément | Où |
| --- | --- |
| Pool IMAP, liste paginée avec tri et recherche, lecture, drapeaux, déplacement, suppression, création de dossier | `internal/mailconn` (`pool.go`, `messages.go`) |
| Analyse MIME (texte, HTML, pièces jointes, images incorporées, jeux de caractères), assainissement du HTML, texte vers HTML | `internal/mailview` |
| Pages : liste (barre de recherche, en-têtes triables, actions en lot, pagination), lecture (iframe sandbox, bandeau de contenu distant, pièces jointes, barre d'outils) | `internal/web/mail.go`, `home.html`, `mail_message.html` |

Vérifié de bout en bout avec GreenMail : 44 messages listés, recherche, tri, lecture du message HTML piégé (script et pixel espion retirés, style conservé, CSP du corps sans image distante), « Afficher pour ce message » (CSP élargie pour ce seul message), téléchargement du PDF, sujet piégé échappé, suppression vers une corbeille créée à la volée.

Batterie d'attaques de l'assainisseur (`internal/mailview`) : 30 charges XSS ou de pistage (script, gestionnaires d'événements, `javascript:` et variantes encodées, `data:` HTML ou SVG, `style` avec `url()`, `@import`, `expression()` et échappements, iframe, formulaires, meta refresh, base, link, object, embed, math, svg, noscript, template, vidéo, `srcset`, `background`, images à protocole relatif) : aucune ne survit, avec ou sans blocage du contenu distant.

**Étape 6 (retours de l'utilisateur après essai)** : panneau des dossiers conservé sur la page d'un message ; filtre Tous / Non lus ; sélecteur de taille de page ; vue 3 volets avec option « liste seule » ; « Tout sélectionner » et Ctrl+A.

Vérification de la sélection au clavier : le script `static/list.js` a été exécuté dans un DOM (jsdom, Node 22) sur la page réellement produite par le serveur : Ctrl+A et Cmd+A cochent toute la page et sont ignorés dans un champ de saisie ou une liste déroulante, Ctrl+Maj+A n'est pas intercepté, « Tout sélectionner » passe en état indéterminé quand la sélection est partielle, Maj+clic sélectionne une plage, Échap efface, le compteur et les boutons suivent. 22 contrôles passent. Ce n'est pas un navigateur réel : à confirmer à l'usage.

Les tests de lignes de la liste (`rowRe`) ne trouvaient aucune ligne après le changement de balisage : plusieurs contrôles réussissaient à vide. L'expression a été corrigée, et les tests qui en dépendent contrôlent maintenant réellement le gras des non lus et le marquage à l'ouverture.

**Étape 7 (volet de lecture partiel, largeurs des volets)** : `internal/web/mail.go` (`mailPane`), `internal/web/handlers.go` (`settingsPanes`, `userCSS`), `internal/web/static/list.js`.

Contrôles JavaScript dans le dépôt (`make test-js`, `tools/jstest/`) : la liste 3 volets et le fragment du volet sont produits par le vrai serveur, puis le script est exécuté dans jsdom (Node 22 en conteneur). 83 contrôles passent, dont : la liste ne bouge pas à un clic (défilement inchangé), clic avec Ctrl, Cmd, Maj, Alt ou bouton du milieu non intercepté, deux clics rapprochés (la réponse la plus récente gagne), échec réseau et session expirée (repli sur la navigation), lignes et compteurs mis à jour, restauration du défilement selon la liste (le paramètre `uid` n'en change pas la signature), glissement des séparateurs avec bornes, clavier, enregistrement différé, double-clic de réinitialisation.

Un contrôle a trouvé un défaut dans mon script : au chargement, les séparateurs figeaient les largeurs par défaut en pixels (perdant l'adaptation au redimensionnement de la fenêtre) et le double-clic ne rétablissait pas le défaut. Corrigé : les variables CSS ne sont posées qu'à l'action de l'utilisateur.

**Étape 8 (apparence RainLoop)** : refonte de l'interface d'après deux captures de l'utilisateur (liste et message avec pièce jointe). Fichiers : `internal/web/templates/base.html` (icônes, barre d'outils, dossiers, volet de lecture), `home.html`, `mail_message.html`, `internal/web/static/mail.css`, `themes/rainloop/theme.css`, `menus.js`, `list.js`, `internal/mailview/thumb.go`.

Méthode : des captures du vrai serveur (Chromium sans tête dans un conteneur, `tools/shot`) comparées aux captures de l'utilisateur. Elles ont montré deux défauts qu'aucun test ne voyait : les champs de type e-mail n'étaient pas stylés, et les miniatures des pièces jointes étaient remplacées par `#ZgotmplZ` (html/template refuse le schéma `data:` dans un attribut `src` ; corrigé avec `template.URL`, et un test vérifie maintenant que la miniature est bien décodable).

**Correction après lecture des sources de RainLoop** (décisions 100 à 106) : l'apparence a été reprise une seconde fois à partir du CSS réel de RainLoop 1.17 et du thème DarkShine. Une capture de mon interface à la même échelle que celles de l'utilisateur (fenêtre de 3 440 px réduite à 0,58) est maintenant comparable : colonnes, hauteurs de ligne, tailles de texte et barres d'outils coïncident. Défauts trouvés au passage : le fond d'écran disparaissait (un mot-clé `fixed` placé après une variable CSS invalidait toute la déclaration) et les cases à cocher de la liste étaient mal placées (une marge globale des étiquettes).

Dans les captures de l'utilisateur et pas encore reproduit : composition (« Nouveau » et « répondre » sont présents mais désactivés), contacts (bouton désactivé), quota (« 36 % » en pied de liste), flèche pour déplier les sous-dossiers, photo d'expéditeur (silhouette à la place), menu de réponse.

Pas encore fait : composition et envoi, brouillons, fils de discussion, aperçu du corps dans la liste, recherche multi-dossiers, gestion des dossiers, contacts, raccourcis clavier de navigation (suivant, précédent, supprimer), actions du volet de lecture sans rechargement de la page (favori, suppression, déplacement rechargent encore la page, avec défilement de la liste conservé).

### 10.2 Limites connues

- **Sessions et CLI** : `user delete` et `user passwd` n'invalident pas les sessions déjà ouvertes dans le serveur (processus différent). Elles expirent normalement.
- **Adresse IP derrière Podman rootless** : sans proxy, le serveur voit l'adresse de la passerelle du réseau du conteneur (observé : `10.89.2.14`) et non celle du vrai client. La limitation par IP s'appliquerait alors à tous les clients ensemble. Avec un proxy (nginx, HAProxy) et `trusted_proxy: true`, l'IP vient de `X-Forwarded-For`. À garder en tête pour les essais directs.
- **Derrière un proxy** : il doit transmettre `Host` (ou `X-Forwarded-Host`) et `X-Forwarded-Proto`, sinon le contrôle d'origine peut refuser des POST légitimes quand le navigateur n'envoie pas `Sec-Fetch-Site`, et le cookie ne sera pas `Secure`.
- **Paramètres argon2id** : 64 Mo, 3 passes, 4 voies. Mesure : inscription en environ 70 ms dans le conteneur. À durcir (par exemple 128 Mo) selon la machine cible.
- **Interface** : les pages n'ont pas été vérifiées visuellement dans un navigateur, seulement par tests HTTP.
- **Écritures concurrentes** : deux sessions du même utilisateur qui enregistrent leurs préférences en même temps s'écrasent (dernier enregistrement gagnant).
- **Limitation de débit** : état en mémoire, remis à zéro au redémarrage.
- **Messages retéléchargés** : la page d'un message, son corps (iframe) et chaque pièce jointe téléchargent et analysent le message complet (30 Mo au plus). Un cache court par session éviterait cela pour les gros messages.
- **Volet de lecture** : le chargement partiel suppose JavaScript. Les actions du volet (favori, supprimer, déplacer) et les liens de pagination rechargent encore la page entière, avec la position de la liste conservée. Les largeurs des volets ne s'appliquent pas sous 900 px (volets empilés).
- **Contrôles JavaScript** : vérifiés dans jsdom, pas dans un navigateur réel ; la mise en page 3 volets (CSS) n'a pas été vue à l'écran.
- **Fuseau horaire** : les dates s'affichent dans le fuseau du serveur (UTC dans le conteneur). Un réglage de fuseau par utilisateur reste à ajouter.
- **Suppression définitive sans UIDPLUS** : sur un serveur qui n'a pas UIDPLUS, la suppression définitive purge tous les messages du dossier portant `\Deleted`, y compris ceux marqués par un autre client.
- **Tri sans SORT** : sur un serveur sans SORT, le tri par expéditeur, sujet, date ou taille charge les en-têtes de tout le dossier (5 000 messages au plus, sinon ordre d'arrivée avec un avertissement).
- **Contenu HTML** : les feuilles de style `<style>` des messages sont retirées (seul l'attribut `style` filtré est conservé), ce qui dégrade la mise en page de certains courriels marketing. Les images distantes se chargent directement depuis le navigateur de l'utilisateur quand il les autorise (pas de proxy).
- **Mots de passe du formulaire** : lors d'un « Tester » ou d'une erreur de validation, les mots de passe saisis sont renvoyés dans le formulaire (réponse `no-store`, session de l'utilisateur) pour éviter de les retaper. À la modification d'un compte existant, ils ne sont jamais renvoyés.
- **Hôtes publics** : la garde n'interdit pas les ports non standard vers des adresses publiques. Un utilisateur peut donc sonder des ports d'hôtes publics depuis le serveur ; la limitation de débit du bouton Tester et le plafond de 20 comptes limitent l'usage. Une liste de ports autorisés reste possible.
- **Authentification sans chiffrement** : le choix « Aucune » envoie le mot de passe en clair ; il est signalé dans le formulaire mais autorisé (serveurs internes, tests).
- **Messages d'erreur** : le détail de l'erreur du serveur distant (200 caractères au plus) est affiché tel quel, échappé.

### 10.3 Commandes

```
make up                         # serveur sur http://127.0.0.1:8080 (docker restart après un changement de config/config.json)
make test | test-race | vet     # dans le conteneur dev ; les tests d'intégration Dovecot sont ignorés si make sieve-up n'a pas été lancé
make test-js                    # contrôles JavaScript (jsdom) sur les pages réelles : sélection, volets, raccourcis clavier
make mail-up | mail-seed        # GreenMail (IMAP 3143, SMTP 3025, test@localhost / testpass) et messages de démonstration
make sieve-up | sieve-down      # Dovecot de test (IMAP dovecot:31143, ManageSieve dovecot:34190, mot de passe testpass)
make test-sieve                 # scripts de filtres compilés et exécutés par Pigeonhole (image Dovecot)
make test-e2e                   # parcours composition et filtres dans Chromium (exige up, mail-up, mail-seed, sieve-up)
make user-add LOGIN=alice       # aussi : user-list, user-passwd, user-recover, user-delete
make logs
```

---

## 11. Inventaire fonctionnel de RainLoop 1.17 (analyse des sources, 2026-10-23)

Source : les fichiers de l'instance de l'utilisateur montés en lecture seule (`app.js` non minifié, `Actions.php`, gabarits, libellés `fr_FR.yml`). Ils servent uniquement à comprendre les fonctions et leur comportement ; aucun code ni texte n'est repris (les libellés de wmail-go sont écrits ici). Le dossier `data/` de RainLoop n'a jamais été lu.

### 11.1 Capacités déclarées par RainLoop (énumération `Capa`)
`TWO_FACTOR`, `TWO_FACTOR_FORCE`, `OPEN_PGP`, `PREFETCH`, `GRAVATAR`, `THEMES`, `USER_BACKGROUND`, `SIEVE`, `FILTERS`, `ATTACHMENT_THUMBNAILS`, `ADDITIONAL_ACCOUNTS`, `IDENTITIES`, `FOLDERS`, `COMPOSER`, `CONTACTS`, `RELOAD`, `SEARCH`, `SEARCH_ADV`, `SETTINGS`, `QUOTA`, `HELP`, `TEMPLATES`, `MESSAGE_ACTIONS`, `MESSAGELIST_ACTIONS`, `ATTACHMENTS_ACTIONS`, `DANGEROUS_ACTIONS`, `AUTOLOGOUT`.
État dans wmail-go : lecture, recherche (simple et par portée), rechargement, miniatures de pièces jointes, comptes multiples, thèmes et fond d'écran, déconnexion automatique : faits. À faire : composition, identités, dossiers, contacts, filtres, modèles, double authentification, OpenPGP, quota, aide, préchargement, Gravatar (à décider).

### 11.2 Écrans de réglages de RainLoop (menu de gauche)
Général, Contacts, Comptes (et identités), Filtres, Sécurité (double authentification), Réseaux sociaux (connexion Google/Facebook/Twitter : **hors périmètre**), Mot de passe, Modèles, Dossiers, Thèmes, OpenPGP.
- **Général** : langue ; identité par défaut ; disposition (sans séparation / séparation verticale / horizontale) ; éditeur par défaut (HTML, texte brut, HTML forcé, texte brut forcé) ; animation de l'interface (pleine, normale, aucune) ; options d'affichage : panneau de prévisualisation, cases à cocher dans la liste, regroupement en conversations, « placer les réponses dans le dossier du message », « toujours afficher les images », animations ; messages par page ; notifications (son, notification du navigateur pour les nouveaux messages) ; sauvegarde automatique des brouillons.
- **Contacts** : ajout automatique des destinataires au carnet ; synchronisation CardDAV distante (serveur, URL du carnet, utilisateur, mot de passe).
- **Comptes** : comptes supplémentaires (ajout, suppression) et identités supplémentaires (e-mail, nom, adresse de réponse, Cci automatique, signature HTML ou texte, « insérer la signature avant la citation »).
- **Dossiers** : liste avec création, renommage, suppression, abonnement/désabonnement, cases « afficher/masquer » et « vérifier les nouveaux messages » par dossier, dossiers système (Envoyés, Brouillons, Indésirables, Corbeille, Archive : choix d'un dossier ou « ne pas utiliser »), limite d'affichage si trop de dossiers.
- **Filtres** (Sieve / ManageSieve) : liste de filtres ; par filtre : nom, correspondance « toutes les règles » ou « l'une des règles », conditions (champ : De, Destinataires (À ou Cc), Sujet, En-tête libre, Taille ; type : contient, ne contient pas, correspond (jokers), ne correspond pas, expression régulière, n'est pas une expression régulière, égal à, différent de, au-dessus de, au-dessous de), actions (aucune, déplacer vers un dossier, faire suivre à une adresse, rejeter avec message, message d'absence (objet, message, destinataires, délai en jours), exclure) avec « marquer comme lu », « garder une copie », « ne pas arrêter le traitement » ; édition du script brut ; vérification des capacités du serveur.
- **Sécurité** : double authentification (TOTP) avec codes de secours, test, « ne plus demander pendant 2 semaines ». À décider : le chiffrement du fichier utilisateur dérive du mot de passe, un second facteur ne peut qu'ajouter une barrière de connexion.
- **Modèles** : textes pré-enregistrés à insérer à la composition.
- **Thèmes** : thèmes prédéfinis, thème personnel (clair/sombre) avec image de fond envoyée (JPG/PNG).
- **OpenPGP** : import/génération de clés, signature, chiffrement, déchiffrement (en texte brut uniquement).

### 11.3 Composition : comportement observé (fenêtre « Écrire »)
Fenêtre modale de 850 px (1000 px sur grand écran) au-dessus de la liste, qui reste en place ; barre d'outils : **Envoyer** (vert, bascule en rouge en cas d'erreur, orange si « envoyé mais non copié dans Envoyés »), **Enregistrer** (brouillon), fermer, **réduire** (la composition continue en arrière-plan : une seule composition à la fois), **supprimer** le brouillon, texte « Enregistré à hh:mm ».
- Champs : **De** (menu des identités si plusieurs ; choix automatique selon les destinataires du message d'origine, puis l'en-tête `Delivered-To`, sinon la première), liens « Cc », « Cci », « Répondre à » qui affichent les champs (affichés d'office s'ils sont remplis), **À**, **Objet**, bouton bascule texte/pièces jointes avec compteur, bouton « + joindre » ; menu « plus » : demander une confirmation de lecture (`Disposition-Notification-To`), demander un accusé de réception DSN, marquer comme important (`Importance`/`X-Priority` haute), OpenPGP, bouton contacts.
- Saisie des adresses : étiquettes (une adresse devient une pastille), séparateurs virgule et point-virgule, **autocomplétion** (carnet d'adresses et destinataires fréquents), adresses glissables entre champs.
- Éditeur : HTML (CKEditor) ou texte brut, bascule avec confirmation (« le formatage et les images seront perdus ») ; l'envoi HTML produit aussi une partie texte alternative convertie ; images collées ou intégrées (`data:`) transformées en images en ligne (`cid:`).
- **Signature** : texte ou HTML (préfixe `:HTML:`), insérée sous le texte ou avant la citation, remplacée si l'on change d'identité ; variables `{{FROM}}`, `{{FROM-FULL}}` (expéditeur du message d'origine), `{{DATE}}`, `{{TIME}}`, `{{MOMENT:format}}`.
- **Réponse** : destinataire = `Reply-To` du message, à défaut `From` (en excluant ses propres adresses ; si la liste est vide, on la recalcule sans exclusion) ; **répondre à tous** : même destinataire principal plus les autres `To` en À et les `Cc` en Cc, sans ses propres adresses ni doublons ; objet préfixé « Re: » **sans empiler** les préfixes (les « Re: », « Fwd: », « Re[2]: » existants sont ramenés à un seul) ; citation : ligne « date adresse a écrit : » puis le message cité en bloc de citation ; `In-Reply-To` = `Message-ID` d'origine, `References` = identifiant d'origine suivi des références d'origine ; seules les images en ligne du message d'origine sont reprises.
- **Transférer** : objet « Fwd: », en-tête « Message transféré » (De, À, Cc, date, objet), toutes les pièces jointes reprises ; **transférer en pièce jointe** (le message devient un fichier `.eml` joint) ; **modifier comme nouveau** (reprend les champs et pièces jointes d'un message reçu) ; **reprise d'un brouillon** (ouvre la composition avec les champs, pièces jointes, `In-Reply-To` et `References` ; l'enregistrement remplace le brouillon précédent).
- Type de réponse mémorisé dans l'en-tête du brouillon (`X-Draft-Info` : réponse/transfert, UID, dossier) pour marquer le message d'origine après l'envoi.
- Pièces jointes : envoi par glisser-déposer ou sélecteur, 2 envois simultanés, taille maximale (25 Mo par défaut, réglable), progression, annulation, erreur par pièce (trop grosse, type refusé, échec) ; l'envoi est bloqué tant qu'une pièce est en cours ou en erreur ; stockage temporaire côté serveur, supprimé après l'envoi.
- Contrôles à l'envoi : au moins un destinataire (À, Cc ou Cci), pièces jointes terminées ; dossier « Envoyés » non défini → invite à le choisir (ou « ne pas utiliser ») ; dossier « Brouillons » non défini → idem.
- Après l'envoi : copie dans « Envoyés » (le message sans Cci est copié tel qu'envoyé ; avec Cci, la copie contient l'en-tête Cci) avec le drapeau `\Seen` ; message d'origine marqué `\Answered` (réponse) ou `$Forwarded` (transfert) ; brouillon supprimé ; pièces temporaires supprimées ; destinataires ajoutés au carnet d'adresses (option) ; si la copie échoue, l'envoi reste réussi mais l'utilisateur est prévenu.
- Brouillon : enregistrement manuel (Ctrl+S) et **automatique** toutes les minutes (option), message ajouté au dossier Brouillons avec `\Seen`, ancien brouillon supprimé ; en-tête `X-Mailer` désactivable.
- Fermeture : confirmation « abandonner les données non enregistrées » si le formulaire n'est pas vide.
- Raccourcis : Ctrl+Entrée envoyer (option), Ctrl+S enregistrer, Échap ou Ctrl+↓ réduire, Maj+Échap fermer, « ` » ouvrir la liste des identités.
- Confirmation de lecture reçue : à l'ouverture d'un message demandant `Disposition-Notification-To`, bouton « l'expéditeur demande à être prévenu » ; si l'utilisateur accepte, envoi d'un accusé (message texte court) et drapeau IMAP optionnel.
- Envoi SMTP : expéditeur d'enveloppe = adresse de l'identité ; un destinataire invalide fait échouer l'envoi (« destinataires invalides ») ; délai de 60 s ; erreurs distinguées (connexion, authentification, rejet) ; option d'envoi par `mail()` PHP (**non retenue**).
- Réglages liés (config RainLoop) : `mail_reply_same_folder` (copie de la réponse dans le dossier du message d'origine), `allow_draft_autosave`, `contacts_autosave`, `allow_ctrl_enter_on_compose`, `attachment_size_limit`, `imap_forwarded_flag` (`$Forwarded`), `hide_x_mailer_header`, `use_imap_move` (RainLoop utilise COPY + suppression par défaut, d'où la correction de la décision 113).

### 11.4 Liste des messages et lecture : fonctions complémentaires relevées
- Barre d'actions : recharger, déplacer vers, supprimer, archiver, indésirable / acceptable (selon le dossier), vider le dossier, **transmettre plusieurs messages** (en pièces jointes), supprimer définitivement, menu « plus » : marquer lu / non lu / **tous lus**, signaler / retirer le signal, **sélection** : tous, aucun, inverser, non lus, lus, signalés, non signalés.
- Recherche avancée : De, À, Objet, Texte, avec pièce jointe, signalé, non lu, période (tous, 3 jours, 1 semaine, 1 mois, 3 mois, 6 mois, 1 an) ; message « Résultats de recherche pour … ».
- Conversations : regroupement par fil (`THREAD`), navigation précédent/suivant/plus dans le fil.
- Lecture : modifier (brouillon), répondre / répondre à tous / transférer / transférer en pièce jointe / modifier comme nouveau, « se désinscrire de cette liste » (`List-Unsubscribe`), afficher les images, voir les en-têtes, voir la source, télécharger en `.eml`, **filtrer les messages similaires** (crée un filtre), imprimer, voir dans une nouvelle fenêtre, citations repliables, pièces jointes avec aperçu d'images (galerie), téléchargement en zip, enregistrement sur le cloud (hors périmètre), messages signés/chiffrés OpenPGP.
- Dossiers : compteurs non lus, quota (« vous utilisez X sur Y »), glisser-déposer de messages sur un dossier, déplacement par menu, dossiers non abonnés masqués, état « vider le dossier » avec confirmation.
- Notifications : nombre de nouveaux messages, son, notification du navigateur ; interrogation périodique des dossiers cochés.
- Raccourcis : liste des messages (Ctrl+A tout sélectionner, Suppr, Maj+Suppr suppression définitive, `m` menu déplacer, Entrée ouvrir, Échap annuler), message (Ctrl+←/→ précédent/suivant, Échap/Retour fermer, répondre `r`, tous `a`, transférer `f`, plein écran), dossiers (↑/↓, Entrée), aide des raccourcis (`?`).

### 11.5 Contacts (carnet d'adresses de RainLoop)
Liste avec recherche ; fiche : nom affiché, prénom, nom, pseudonyme, adresses e-mail multiples (type), téléphones (type), adresses postales, URL, anniversaire, notes, étiquettes ; import **vCard, CSV** ; export **vCard, CSV** ; synchronisation **CardDAV** ; partage (personne / tout le monde) ; ajout automatique des destinataires ; fréquence d'utilisation pour l'autocomplétion ; accès depuis la composition (le contact choisi est inséré dans le champ À, Cc ou Cci actif). À concevoir dans wmail-go : stockage dans le fichier chiffré de l'utilisateur (volumétrie à borner), vCard, groupes, autocomplétion locale sans requête réseau.

### 11.6 Décisions de conception pour la composition de wmail-go (à valider avec l'utilisateur)
Ces choix guident la première version ; l'utilisateur les révisera à son retour.

| N° | Décision |
|----|----------|
| 114 | Composition sur une **page dédiée** `/compose` (pas de fenêtre modale : le serveur rend des pages et la liste se recharge à la fermeture) qui reprend l'aspect de la fenêtre RainLoop (boîte de 850 px sur le fond d'écran, barre Envoyer / Enregistrer / Fermer / Supprimer, grille De / À / Cc / Cci / Répondre à / Objet, bascule texte / pièces jointes). Une seule composition à la fois par session n'est pas imposée : chaque page est indépendante (brouillon identifié par son UID) |
| 115 | **Première livraison** (A) : texte brut ; réponse, répondre à tous, transférer, transférer en pièce jointe (`.eml`), modifier comme nouveau, reprise d'un brouillon ; identités (une par boîte au départ, structure prévue pour plusieurs) ; Cc / Cci / Répondre à ; pièces jointes (téléversement avec progression, glisser-déposer, limite de taille réglable, suppression) ; brouillons (manuel et automatique, option) ; envoi SMTP avec copie dans Envoyés ; drapeaux `\Answered` / `$Forwarded` ; confirmation de lecture demandée, importance, accusé DSN. **Deuxième livraison** (B) : éditeur HTML (`contenteditable` avec barre d'outils, HTML nettoyé par la même liste blanche que la lecture, partie texte alternative, images en ligne), signatures HTML et variables. **Troisième livraison** (C) : carnet d'adresses et autocomplétion, accusé de lecture envoyé à la demande |
| 116 | Les messages sont **construits et envoyés par wmail-go** (Go : `go-message` pour la construction MIME, `go-smtp` pour l'envoi) : en-têtes `Message-ID` (domaine de l'identité), `Date`, `MIME-Version`, `In-Reply-To`, `References`, `X-Mailer` désactivable, noms et objets encodés (RFC 2047), pièces jointes encodées en base64, texte en `quoted-printable` UTF-8. Les adresses sont validées (`net/mail`) ; tout caractère de contrôle (CR, LF) dans un champ est refusé (**injection d'en-têtes**) |
| 117 | Pièces jointes en attente d'envoi **gardées en mémoire du serveur** (jamais écrites en clair sur le disque, cohérent avec le chiffrement des données), par session puis par composition ; supprimées à l'envoi, à l'abandon, à la fermeture ou l'expiration de la session ; limites réglables dans `config.json` (`compose` : 25 Mo par fichier, 30 Mo par message, 30 pièces, 100 Mo par session). Conséquence : un redémarrage du serveur les perd (les brouillons enregistrés sur IMAP, eux, les conservent) |
| 118 | Envoi : on **n'ajoute pas** les destinataires à un carnet tant que les contacts n'existent pas ; la copie dans Envoyés est faite après l'envoi réussi ; son échec n'annule pas l'envoi mais est signalé (orange) ; le brouillon est supprimé après envoi réussi. Journal : une ligne par envoi (identité, nombre de destinataires, taille, durée), jamais le contenu ni les adresses |
| 119 | Anti-abus : l'envoi est soumis à la limitation de débit (messages par minute et par heure par session et par compte, réglable) et à une taille maximale de message ; l'adresse d'expéditeur doit être celle d'une identité de l'utilisateur (pas d'adresse libre) |
| 120 | **Composition, livraison A (faite le 2026-10-23)** : page `/compose` (modes `new`, `reply`, `replyall`, `forward`, `forwardatt`, `draft`, `editasnew`) ; `POST /compose/send`, `/compose/save` (brouillon, JSON si appel `fetch`), `/compose/attach` (un fichier par requête, JSON), `/compose/detach`, `/compose/discard`. Texte brut ; destinataires de réponse et objet « Re: / Fwd: » selon le comportement de RainLoop (voir 11.3) ; citation avec `>` ; signature précédée de `-- ` (avant ou après la citation selon l'option), variables `{{FROM}}`, `{{FROM-FULL}}`, `{{DATE}}`, `{{TIME}}` ; choix automatique de l'identité d'après les destinataires du message d'origine ; Cc, Cci, Répondre à affichés à la demande ; confirmation de lecture, DSN, importance ; copie dans « Envoyés » (créé si absent, avec `\Seen`, Cci conservé dans la copie seulement), `\Answered` / `$Forwarded` sur le message d'origine, brouillon supprimé ; enregistrement automatique toutes les minutes (option) ; Ctrl+Entrée, Ctrl+S ; avertissement à la fermeture ; « Modifier » sur un brouillon ; menu répondre/répondre à tous/transférer/transférer en pièce jointe/modifier comme nouveau dans la lecture ; bouton « Nouveau » actif |
| 121 | Sécurité de la composition : l'expéditeur est toujours une identité de l'utilisateur (identifiant contrôlé côté serveur) ; tout caractère de contrôle dans une adresse, un objet ou un nom de pièce jointe est refusé ou neutralisé (aucune injection d'en-tête) ; un destinataire refusé par le serveur SMTP annule tout l'envoi (pas d'envoi partiel) ; limitation de débit des envois (`rate_limit.send`, 60/h par IP et 30/h par compte) ; taille du corps des requêtes bornée par route ; les pièces d'une session ne sont jamais accessibles depuis une autre |
| 122 | Réglages ajoutés : par boîte, signature (+ « avant la citation »), dossier des envoyés et des brouillons (`-` = aucun) ; par utilisateur, enregistrement automatique des brouillons et « copie d'une réponse dans le dossier du message d'origine » ; message d'information à usage unique après un envoi (`Session.Flash`) |
| 123 | **Cadre des réglages** (comme RainLoop) : menu à gauche sur le fond d'écran (Général, Comptes, Dossiers, Filtres et Contacts grisés « bientôt », Sécurité), volet blanc à droite avec le bouton Retour, le nom de l'utilisateur et Déconnexion. Les anciennes pages sont réparties : `/settings` (préférences), `/settings/security` (mot de passe, clé de récupération), `/accounts` (+ identités), `/settings/folders` |
| 124 | **Page Dossiers** : dossiers système (envoyés, brouillons, indésirables, corbeille, archive : automatique, un dossier existant, ou « ne pas utiliser » pour envoyés et brouillons), création (avec dossier parent), renommage, suppression avec confirmation (les messages sont perdus ; INBOX protégée, y compris sous une autre casse), abonnement et désabonnement, liste avec nombre de messages et de non lus. Un dossier désigné qui est renommé ou supprimé est oublié des rôles. Les dossiers non abonnés sont masqués du panneau de gauche si le serveur sait lister les abonnements (LIST-EXTENDED) et qu'au moins un dossier est abonné ; INBOX et les dossiers à usage spécial restent visibles |
| 125 | **Identités supplémentaires** (`/accounts/{id}/identities`) : ajout, modification, suppression (20 au plus par boîte), adresse de réponse et Cci automatiques, signature propre ; unicité des adresses dans une boîte ; choix automatique à la composition ; conservées quand on modifie le compte |
| 126 | **Éditeur HTML (livraison B, faite)** : zone `contenteditable` et barre d'outils (police, taille, gras, italique, souligné, effacer la mise en forme, couleur du texte et du fond, listes, retraits, citation, alignements, lien http/https/mailto, ligne horizontale) ; bascule HTML ⇄ texte brut avec confirmation (la mise en forme et les images sont perdues) ; réglage « éditeur par défaut » : HTML, texte brut, HTML forcé, texte brut forcé (le mode forcé supprime la bascule) ; comme RainLoop, la réponse à un message en texte brut s'ouvre en texte brut sauf « HTML forcé » ; le texte brut du champ reste rempli (secours sans JavaScript) |
| 127 | Envoi HTML : le serveur **assainit** le HTML reçu (même liste blanche que la lecture : ni script, ni formulaire, ni cadre, ni gestionnaire d'événement, ni `url()`), retire les images distantes (pas de pisteur, pas de chargement distant à l'insu de l'auteur), transforme les images `data:` en parties incorporées `cid:` (4 Mo par image, 12 Mo et 40 images au plus), produit la partie texte alternative, et assemble `multipart/alternative` (texte, puis HTML ou `multipart/related` avec les images), dans un `multipart/mixed` s'il y a des pièces jointes. Citation HTML des réponses : contenu du message d'origine assaini, images en ligne reprises, images distantes retirées |
| 128 | Signatures HTML : une signature commençant par `:HTML:` est assainie ; sinon le texte est échappé ; elle est placée dans un bloc repéré (`data-wmail-sig`) que le changement d'identité remplace. Politique de sécurité du contenu : la page de composition seule autorise les styles en ligne (`style-src 'unsafe-inline'`) parce que l'éditeur écrit des attributs `style` ; scripts toujours interdits, images distantes toujours interdites (`img-src 'self' data:`) |
| 129 | Test de bout en bout dans un vrai navigateur : `make test-e2e` (Chromium sans tête, `tools/e2e/compose.js`) exécute création de boîte, composition HTML, Cc, pièce jointe, brouillon, bascule texte, envoi, copie dans Envoyés, réception et réponse, et échoue à la moindre erreur de console |
| 130 | **Filtres (Sieve)** : client ManageSieve (RFC 5804) écrit pour le projet (`internal/sieve`) : STARTTLS ou TLS implicite, authentification PLAIN (jamais avant TLS si STARTTLS est choisi), `LISTSCRIPTS`, `GETSCRIPT`, `PUTSCRIPT`, `CHECKSCRIPT`, `SETACTIVE`, `DELETESCRIPT`, littéraux bornés (1 Mo) ; connexions faites par le garde réseau (même protection contre les adresses privées que l'IMAP et le SMTP, aide « anciens chiffrements »). Activation **par boîte** dans le formulaire du compte (désactivée par défaut) : serveur, port (4190), sécurité (STARTTLS), identifiants (vides : ceux de l'IMAP), vérification du certificat |
| 131 | **Compatibilité avec RainLoop** : mêmes noms de scripts (`rainloop.user`, `rainloop.user.raw`), même en-tête (`# RAINLOOP:SIEVE`), mêmes blocs `BEGIN:FILTER` avec le filtre en JSON encodé en base64 (clés `ID`, `Enabled`, `Name`, `Conditions`, `ConditionsType`, `ActionType`, `ActionValue`…`Fourth`, `Keep`, `Stop`, `MarkAsRead`, booléens PHP acceptés), dossiers en UTF-7 modifié dans l'en-tête et en UTF-8 dans le script (comportement par défaut de RainLoop) : les filtres créés dans l'un des deux webmails sont relus et modifiés par l'autre. Un script du même nom qui n'a pas ce marqueur n'est jamais écrasé sans confirmation explicite |
| 132 | **Page Filtres** : liste (nom, résumé, active/inactive), activation/désactivation (le filtre reste dans le script, enveloppé en commentaire), ordre (monter/descendre), suppression avec confirmation (le script est supprimé du serveur quand il ne reste aucun filtre), ajout et modification (conditions De, Destinataires, Objet, En-tête libre, Taille avec contient, ne contient pas, égal, différent, expression régulière, plus grand, plus petit ; « l'une » ou « toutes » les conditions ; actions : rien, déplacer, faire suivre (avec copie), rejeter, message d'absence (objet, durée, autres adresses, arrêt), supprimer ; marquer comme lu), options grisées si le serveur n'annonce pas l'extension (`fileinto`, `reject`, `vacation`, `imap4flags`, `regex`), script brut avec vérification par le serveur avant enregistrement, et lien « Filtrer les messages similaires » dans la lecture (prérempli avec l'expéditeur) |
| 133 | **Sécurité des filtres** : toute valeur est placée entre guillemets avec échappement de `\` et `"` ; les suites `*/` et `/*` sont neutralisées (un filtre désactivé est dans un commentaire `/* … */` : une valeur ne doit pas le refermer) ; plus aucun commentaire de bloc dans les corps générés ; retours à la ligne des messages en CRLF ; nom d'en-tête limité aux caractères permis ; taille numérique avec K, M ou G ; identifiants de filtre contraints ; validation côté serveur avant tout envoi ; limitation de débit des connexions (règle `rate_limit.sieve`, 200 par IP et 100 par compte par 10 minutes) ; le script produit est vérifié en test par un interpréteur Sieve indépendant (`go-sieve`, test seulement) |
| 134 | Limites connues des filtres : pas d'essai sur un serveur ManageSieve réel (les échanges sont testés avec un serveur de test écrit pour le projet ; le vrai serveur de l'utilisateur ne sera sollicité qu'avec son accord) ; écrire des noms de dossier en UTF-8 suppose l'option `sieve_utf8` du serveur (défaut de RainLoop) ; pas de test d'accès Sieve dans le bouton « Test » du compte (la page des filtres affiche l'erreur de connexion) |
| 135 | **Vérification des scripts par le vrai interpréteur** : `make test-sieve` (`tools/sievecheck`) fait produire 22 scénarios de filtres par le générateur, puis les fait **compiler (`sievec`) et exécuter (`sieve-test`) par Pigeonhole 2.4 de Dovecot** (image officielle, aucun serveur ni boîte touchés) sur des messages d'essai : déplacer (dossiers accentués), plusieurs adresses, objet exclu, toutes/une des conditions, en-tête libre, taille, destinataires, expression régulière, supprimer, rejeter, faire suivre avec et sans copie, absence, marquer comme lu, filtre désactivé, ordre et arrêt, tous les messages, et deux scénarios de **valeurs hostiles** (guillemets, `*/`, `; discard;`) qui restent des données. 36 essais, tous réussis |
| 136 | **Serveur Dovecot de test** (`make sieve-up`, service `dovecot` du profil `test`, image officielle 2.4, sans TLS, jamais en production, configuration dans `tools/dovecot/test.conf`) : ManageSieve sur `dovecot:34190`, mot de passe `testpass`, utilisateur quelconque (chacun a son dossier). Servent à (1) `internal/sieve/integration_test.go` : le client ManageSieve et le script généré (déplacer, absence, expression régulière, drapeaux, rejet, filtre désactivé avec `*/`) contre le **vrai** serveur, y compris le refus d'un script invalide et un script volumineux ; (2) `tools/e2e/filters.js` : le parcours complet dans Chromium (activation de ManageSieve dans le compte, création, ajout de condition en JavaScript, ordre, désactivation, script brut refusé par le serveur, suppression, filtre prérempli). `make test-e2e` lance les deux scénarios (composition et filtres) ; `rate_limit.sieve` limite les pages de filtres (200 par IP et 100 par compte par 10 minutes) ; la configuration de développement relève la limite d'inscription à 200 par heure pour que les essais navigateur puissent créer des comptes |
| 137 | **Raccourcis clavier** (`static/keys.js`, ceux de RainLoop) : `C` ou `W` nouveau message, `R` répondre, `A` répondre à tous, `F` transférer, `Maj+F` transférer en pièce jointe, `Z` archiver, `Suppr` ou `#` supprimer, `M` menu « déplacer vers », `Q` lu, `U` non lu, `I` favori, `/` recherche, `Échap` ferme le message, `?` aide, `` ` `` menu utilisateur, `Ctrl+A` tout sélectionner (déjà fait), `Ctrl+Entrée` et `Ctrl+S` dans la composition ; ils sont inactifs dans un champ de saisie, avec Ctrl, Alt ou Cmd, et agissent par les boutons de la barre d'outils (le serveur contrôle chaque action) ; aide des raccourcis dans le menu de l'utilisateur ; tests jsdom (`make test-js`) |
| 138 | **Liens mailto:** : la page de composition accepte `?mailto=mailto:…` (adresses, `cc`, `bcc`, `subject`, `body`, RFC 6068) ; retours à la ligne retirés des champs d'adresse et d'objet, tailles bornées, tout autre schéma ignoré ; bouton « Utiliser wmail-go pour les liens mailto: » dans les réglages (`registerProtocolHandler`, connexion HTTPS requise) ; l'adresse de l'expéditeur dans la lecture ouvre la composition |
| 139 | **Accusé de lecture** (confirmation demandée par `Disposition-Notification-To`) : jamais envoyé automatiquement ; la lecture propose « Envoyer l'accusé de lecture » ou « Ne pas répondre » avec l'adresse de destination ; l'accusé est un `multipart/report` conforme à la RFC 8098 (explication lisible et `message/disposition-notification`), envoyé avec une enveloppe vide (`MAIL FROM:<>`) depuis l'identité qui correspond aux destinataires du message ; le mot-clé IMAP `$MDNSent` (RFC 3503) est posé dans les deux cas pour ne plus le proposer ; non proposé pour ses propres messages ni les brouillons ; soumis à la limitation de débit des envois |
| 140 | **Tests d'intégration IMAP contre le vrai Dovecot** (`internal/mailconn/dovecot_test.go`, `DOVECOT_IMAP`, `make sieve-up` puis `make test`) : ajout de 40 messages avec UID, pages triées par date, expéditeur et objet (SORT), recherches par objet accentué, corps, expéditeur, drapeaux et non lus, compteurs, lecture, déplacement (MOVE), suppression définitive, vidage, création, renommage, abonnement, désabonnement et suppression de dossiers, **déplacement puis suppression de 500 messages en moins de 15 s**. **Défaut trouvé et corrigé** : la bibliothèque go-imap écrit les noms de dossiers en UTF-8 dès que le serveur annonce `IMAP4rev2`, or Dovecot 2.4 l'annonce sans que la connexion soit en mode IMAP4rev2 tant que le client ne l'a pas demandé ; un dossier au nom accentué (« Éléments supprimés », « Été ») était refusé (`Mailbox name is not valid mUTF-7`). `enableUTF8` envoie maintenant `ENABLE UTF8=ACCEPT` après la connexion quand le serveur le propose ; test de non-régression avec un dossier « Éléments supprimés/Été 日本 » |
| 141 | **Quota** : quand le serveur annonce `QUOTA` et fixe une limite de stockage, le panneau des dossiers affiche « X (Y %) sur Z » avec une barre (rouge à partir de 90 %) ; aucune erreur si le serveur ne sait pas (information facultative) ; non testé contre un serveur avec quota (le Dovecot de test n'a pas de plugin de quota) |
| 142 | **Panneau des dossiers comme RainLoop** : dossiers système d'abord, dans l'ordre RainLoop (boîte de réception, envoyés, brouillons, indésirables, corbeille, archives), repérés par le choix de la boîte, l'attribut IMAP ou le nom usuel (« Deleted Items », « Sent Items », « Junk E-mail »… reconnus), avec leur nom traduit ; puis tous les autres en **arbre trié**, dont chaque niveau se **replie et se déplie** (flèche, état mémorisé dans le compte par boîte mail, `POST /settings/folder-open`) ; replié au départ, sauf les ancêtres du dossier affiché ; le préfixe d'espace de noms (« INBOX. » de Dovecot) est retiré de l'affichage ; les parents absents sont ajoutés en dossiers non sélectionnables ; sans JavaScript l'arbre reste entièrement visible (`noscript.css`) |
| 143 | **Boutons du bas du panneau** : le bouton aux flèches en diagonale **masque ou affiche le panneau des dossiers** (il se réduit à une colonne de boutons ; état mémorisé dans le compte, `POST /settings/panes` avec `hidden`) au lieu du plein écran (supprimé) ; le bouton « dossier » ouvre un **formulaire de création** (nom, dossier parent choisi parmi les dossiers existants, séparateur du serveur) sans quitter la page, et retombe sur la page « Dossiers » sans JavaScript |
| 144 | **Réglages explicitement par boîte mail** : les pages Dossiers (dossiers système, création, renommage, abonnement) et Filtres portent un paramètre `box` (formulaires et liens), nomment la boîte réglée et proposent les autres boîtes ; une boîte inconnue ou d'un autre utilisateur donne 404 ; les dossiers système des formulaires de compte sont des **sélecteurs de dossiers IMAP existants** (valeur introuvable conservée et signalée) |
| 145 | **Filtres sans activation préalable** : la page des filtres tente directement le serveur de l'IMAP (ManageSieve, port 4190, STARTTLS, mêmes identifiants et même vérification du certificat) ; le réglage ManageSieve du compte devient facultatif (autre serveur ou port) et une case permet de ne pas proposer les filtres ; si le serveur ne répond pas, la page l'explique et renvoie vers ce réglage |
| 146 | **Outils sur un message** : imprimer (`/mail/print`, page sans script externe, CSP `script-src 'self'`), voir la source / les en-têtes / télécharger en `.eml` (`/mail/source`), lien « se désinscrire » (premier lien `https` de `List-Unsubscribe`, sinon message pré-rempli vers l'adresse `mailto:`), copier vers un dossier, « marquer tout comme lu », sélection par critère (tous, aucun, inverser, non lus, lus, étoilés, non étoilés) |
| 147 | **Carnet d'adresses** (`internal/contacts`, `/contacts`) : un fichier chiffré par compte (`<id>.ctc`, sous-clé HKDF de la clé de données, liaison à l'identifiant du compte, supprimé avec le compte) ; fiche = nom en 5 parties, surnom, adresses, téléphones et sites typés, adresses postales, organisation, fonction, anniversaire, notes, étiquettes ; 5 000 contacts maximum ; recherche sans accent ni casse, tri par nom, pages de 50 ; import vCard 2.1/3.0/4.0 et CSV (virgule, point-virgule ou tabulation, UTF-8 ou Windows-1252, en-têtes RainLoop, Outlook, Thunderbird, Google ; les contacts dont une adresse existe déjà sont ignorés) ; export vCard 3.0 et CSV (neutralisation des formules de tableur) ; `/contacts/suggest` (autocomplétion par fréquence, 8 réponses) et sélecteur de contacts dans la composition ; fréquence augmentée pour les destinataires « À » ; **ajout automatique des destinataires inconnus après envoi** (comme RainLoop, désactivable dans les préférences) ; lien « Ajouter aux contacts » / « Voir le contact » dans le message |
| 148 | **Recherche à la manière de RainLoop** : dans la portée « partout », les mots-clés `from: to: cc: email: subject: text: has:attachment is:unseen|seen|flagged|unflagged date:AAAA.MM.JJ[/AAAA.MM.JJ] size: larger: smaller:` (guillemets pour les valeurs avec espace) sont traduits en critères IMAP ; une valeur invalide ou un mot-clé inconnu reste du texte libre. Le formulaire de recherche avancée (De, À, Objet, Texte, période, non lu, suivi, pièce jointe) compose cette requête dans le champ de recherche, qui reste modifiable à la main |
| 149 | **Authentification en deux étapes** (`internal/totp`, `/settings/2fa`) : TOTP RFC 6238 (SHA-1, 6 chiffres, 30 s, tolérance d'un pas ; vecteurs de la RFC testés), code QR produit localement (aucun service tiers, contrairement à RainLoop), 9 codes de secours à usage unique (seules leurs empreintes sont enregistrées dans le fichier chiffré), un code TOTP ne sert qu'une fois. À la connexion, un mot de passe correct n'ouvre pas de session : le compte attend le code en mémoire 5 minutes, 5 essais, limitation de débit commune avec la connexion. Case « ne plus demander pendant 2 semaines » : cookie signé par une clé propre au compte, invalidée à chaque activation ou désactivation. Désactiver, régénérer les codes exigent un code valide. **Limite assumée** : la récupération par clé de récupération (page « récupérer ») donne accès sans second facteur, comme elle remplace déjà le mot de passe |
| 150 | **Modèles de message** (`/settings/templates`) : 50 au plus, nom et texte (20 000 caractères), insérés au curseur depuis la composition (éditeur HTML ou texte), variables `{{DATE}}`, `{{TIME}}`, `{{MOMENT:format}}` ; stockés dans le fichier chiffré du compte |
| 151 | **Sondage et notifications** : `/mail/poll` (états IMAP `STATUS`, nouveaux messages non lus depuis un UID) interrogé toutes les 2 minutes par la page ; titre de l'onglet « (N) … », compteur du panneau mis à jour, notification de bureau (autorisation demandée quand on coche l'option) et son synthétisé (aucun fichier audio) selon les préférences |
| 152 | **Lecture** : état DKIM tiré d'`Authentication-Results` (« pass » seulement si le domaine signataire correspond à celui de l'expéditeur), en-tête complet (taille, Cci, Répondre à), citations longues dans un `details` replié, pixels espions retirés même avec le contenu distant autorisé, visionneuse d'images et aperçu du texte (`?inline=1`, types reconnus d'après les octets, jamais SVG ni HTML) ; image de fond personnelle (JPEG/PNG de 2 Mo et 6 000 × 6 000 au plus, chiffrée avec le compte, servie seulement à son propriétaire) ; touches fléchées, glisser-déposer vers les dossiers |
| 153 | **Synchronisation CardDAV** (`internal/contacts/carddav.go`, bibliothèque go-webdav) : adresse du carnet (ou du serveur : le premier carnet de l'utilisateur est alors découvert), identifiant, mot de passe (dans le fichier chiffré), intervalle de 5 à 320 minutes. Chaque passage reprend les contacts nouveaux ou modifiés de l'autre côté, envoie ceux de ce côté, supprime des deux côtés ce qui l'a été ; en cas de modification des deux côtés, la plus récente l'emporte ; les suppressions faites ici sont gardées comme « pierres tombales » jusqu'à leur envoi. Le réseau est utilisé hors verrou sur une copie du carnet ; si le carnet change pendant le passage, le résultat est écarté (les liens se retrouvent par UID au passage suivant). Passage automatique déclenché par le sondage de la page, au plus un toutes les 2 minutes, plus un bouton « synchroniser maintenant ». HTTPS exigé, sauf hôtes de `allowed_private_hosts` ; connexions faites par le garde anti-SSRF (aucun proxy d'environnement) ; testé de bout en bout contre un serveur CardDAV en mémoire (`internal/contacts/davtest`). **Limite** : les propriétés vCard que le carnet ne connaît pas (photo…) sont perdues quand un contact modifié ici est renvoyé |
| 154 | **Conversations** : option « regrouper les messages en conversations » (extension IMAP `THREAD`, algorithme REFERENCES sinon ORDEREDSUBJECT) ; une ligne par conversation, représentée par son message le plus haut dans l'ordre de tri, avec une pastille de compteur ; non lu et suivi si un membre l'est ; la pastille ouvre la conversation (`?thread=UID`, ses seuls messages) ; supprimer, déplacer, copier, archiver, signaler une conversation cochée agit sur tous ses messages, les drapeaux (lu, suivi) sur le seul message représentant ; sans `THREAD` sur le serveur, la liste reste message par message avec un avertissement ; raccourci `T` |
| 155 | **Dossiers surveillés** : jusqu'à 5 dossiers en plus de la boîte de réception, choisis par boîte mail (`/settings/folders`), dont le nombre de non lus est relevé par le sondage de 2 minutes ; **dépôt d'un `.eml`** sur la liste (`/mail/append`, message RFC 822 exigé, marqué lu) ; **confirmation** avant suppression définitive ; **champs d'adresses en pastilles** et images collées dans l'éditeur ; téléchargements avec requêtes partielles ; bouton principal de réponse mémorisé |
| 156 | **Images distantes** : en plus du blocage par boîte, « toujours afficher les images de cet expéditeur » (liste de 300 adresses, retrait dans les préférences) et, par boîte, **chargement par le serveur** (`/mail/image`, adresse signée par une clé de la session, connexion par le garde anti-SSRF, 5 Mo, formats raster reconnus d'après les octets, 400 requêtes par minute et par session) ; la page du message n'autorise alors que `'self'` comme source d'images |
| 157 | **Préférences d'affichage** : cases à cocher de la liste, animations, regroupement en conversations, notifications ; **choix de la langue** avant la connexion (cookie `wm_lang`, prioritaire sur `Accept-Language`) ; ordre des boîtes mail et des identités supplémentaires |

---

## 12. État d'avancement – composition (2026-10-23, soir)

Livraison A de la composition terminée et vérifiée : `internal/compose` (adresses, objet, destinataires de réponse, citation, assemblage MIME, en-tête privé `X-Wmail-Draft-Info`), `mailconn.Conn.Append`, `mailconn.Guard.SendSMTP` (erreurs `SendError`, DSN), `mailconn.ResolveSent/ResolveDrafts`, correction de `Conn.Move` (décision 113), pièces jointes en mémoire (`web/attachstore.go`), page, JavaScript (`static/compose.js`) et styles. Tests : unitaires du paquet `compose` (adresses, objet, destinataires, MIME, injection d'en-têtes, aller-retour avec l'analyse de lecture), tests `mailconn` (ajout IMAP, envoi SMTP avec serveur de test qui garde les messages, destinataire refusé, authentification), tests web de bout en bout (réponse, transfert avec pièces, envoi, copie, drapeaux, brouillon enregistré/remplacé/repris/supprimé, abandon, pièces jointes et limites, isolement entre sessions, nettoyage à la déconnexion, limitation de débit, validation, signature, absence de dossier) et un scénario complet dans Chromium (réponse, Cc, pièce jointe, brouillon, envoi, copie dans Envoyés).
Limites connues : pas d'autocomplétion ; un seul fichier à la fois par requête d'envoi (plusieurs en parallèle) ; les pièces jointes se perdent si le serveur redémarre ; la copie dans « Envoyés » n'est pas réessayée en cas d'échec (l'utilisateur est prévenu).

---

## 13. Réflexion : gestion des contacts (à valider avec l'utilisateur avant tout code)

### 13.1 Ce que fait RainLoop (voir 11.5)
Carnet par utilisateur, fiche riche (nom affiché, prénom, nom, pseudonyme, plusieurs e-mails et téléphones typés, adresses, URL, anniversaire, notes, étiquettes), recherche, import vCard et CSV, export vCard et CSV, synchronisation CardDAV, partage, ajout automatique des destinataires, fréquence d'utilisation pour l'autocomplétion, accès depuis la composition.

### 13.2 Propositions
| Sujet | Proposition | Alternatives |
|-------|-------------|--------------|
| Stockage | **Fichier chiffré séparé par utilisateur** (`contacts.cfg` dans la même arborescence hashée), chiffré avec une sous-clé dérivée (HKDF « contacts ») de la clé de données de l'utilisateur : le fichier de réglages reste petit et n'est pas réécrit à chaque contact modifié ; chargé à la demande | Tout dans le fichier de réglages (simple, mais réécrit en entier à chaque modification, et gonfle le fichier) ; base SQLite chiffrée (dépendance de plus) |
| Modèle | `Contact{ID, Nom affiché, Prénom, Nom, Pseudo, Emails[{type, valeur}], Téléphones[{type, valeur}], Adresses[], URL[], Anniversaire, Notes, Étiquettes[], Fréquence, Dernier envoi, Modifié le}` ; bornes : 5 000 contacts, 64 Kio par fiche | Groupes en plus des étiquettes (listes de diffusion) |
| Formats | vCard 3.0 et 4.0 en import/export (bibliothèque `emersion/go-vcard`), CSV en import/export (colonnes de Thunderbird et de Google reconnues, jeu de caractères détecté) ; import avec aperçu et détection de doublons (même adresse) | Export seul au début |
| Autocomplétion | Point d'accès `GET /contacts/suggest?q=` (réponse JSON, limitée à 10 résultats, débit limité) interrogé par la composition avec une temporisation ; tri : fréquence puis nom ; recherche sans accent ni casse sur nom, e-mail, pseudo, étiquettes ; **aucune requête vers un tiers** | Liste complète intégrée à la page (rapide, mais lourde au-delà de quelques milliers) |
| Ajout automatique | À l'envoi, les destinataires inconnus peuvent être ajoutés (option, **désactivée par défaut** ici, contrairement à RainLoop qui l'active) ; la fréquence des connus est incrémentée | Jamais d'ajout automatique ; destinataires récents lus dans « Envoyés » (sans stockage) |
| Interface | Page `/contacts` (liste à gauche, fiche à droite, recherche, import/export, étiquettes) ; dans la composition, bouton Contacts qui ouvre un sélecteur (`<dialog>`) et insère dans le champ actif (À, Cc, Cci) | Fenêtre modale sur la liste, comme RainLoop |
| CardDAV | **Phase ultérieure** : client (bibliothèque `emersion/go-webdav`), synchronisation à la demande puis périodique, résolution des conflits par date de modification, mot de passe stocké comme ceux des boîtes mail ; option par utilisateur | Jamais (import/export suffit) |
| Photos | Silhouette par défaut ; photo de la fiche vCard si présente (limitée en taille, jamais de Gravatar ni de requête externe sauf choix explicite de l'utilisateur) | Gravatar optionnel (fuite de l'adresse vers un tiers) |
| Partage | Non retenu (un utilisateur n'a accès qu'à ses données) | |
| Sécurité | Contenu des fiches assaini à l'affichage ; import borné (taille, nombre) ; export réservé à l'utilisateur connecté, avec jeton CSRF ; journal sans contenu | |

### 13.3 Questions à poser à l'utilisateur
1. Le stockage séparé chiffré convient-il ? 2. CardDAV est-il nécessaire, et avec quel serveur (Nextcloud, Dovecot, autre) ? 3. L'ajout automatique des destinataires doit-il être actif par défaut ? 4. Quels fichiers d'import avez-vous (export de RainLoop : vCard ou CSV) ? 5. Faut-il des groupes (listes de diffusion) ou les étiquettes suffisent-elles ?

### 13.4 Ordre de réalisation proposé
(1) Stockage chiffré et modèle ; (2) import et export vCard/CSV ; (3) page `/contacts` ; (4) autocomplétion et sélecteur dans la composition ; (5) ajout automatique et fréquence ; (6) CardDAV.

Mise à jour du soir : les raccourcis clavier, les liens mailto:, l'accusé de lecture, le quota et la vérification contre un vrai Dovecot (défaut des noms de dossiers accentués corrigé) sont faits, ainsi que les filtres Sieve (ManageSieve, compatibles RainLoop) sont faits, l'éditeur HTML (livraison B) aussi, ainsi que le cadre des réglages, la page Dossiers, les identités, la réflexion sur les contacts (section 13) et le test de bout en bout `make test-e2e`. Tous les tests passent (`make test-race`, `make test-e2e`). Rien n'a été fait sur les vraies boîtes de l'utilisateur.


## 14. Analyse ligne à ligne de RainLoop 1.17 : écarts à combler (2026-10-23, nuit)

Cinq relectures indépendantes du code de RainLoop (liste et lecture : 213 fonctions ; réglages et comptes : 213 ; composition et interface : 170 ; contacts : 104 ; serveur, configuration et sécurité : 240) ont été comparées à wmail-go. Les rapports sont conservés hors du dépôt ; ce chapitre garde la liste des écarts. Légende : [x] fait, [ ] à faire, [-] volontairement non repris (raison donnée).

### 14.1 Contacts
- [x] Stockage chiffré, modèle, recherche, pages, import/export vCard et CSV, autocomplétion, sélecteur, fréquence, ajout automatique, lien dans le message (décision 147)
- [x] Synchronisation CardDAV (adresse/identifiant/mot de passe/intervalle, la modification la plus récente l'emporte, appels passant par le garde anti-SSRF, https exigé hors hôtes locaux autorisés), décision 153
- [-] Photos Gravatar : envoie l'adresse du correspondant à un tiers (choix de confidentialité)

### 14.2 Liste des messages
- [x] Icônes répondu et transféré, destinataires (au lieu de l'expéditeur) dans Envoyés et Brouillons, « (Pas d'objet) »
- [x] Date relative pendant 4 heures (« il y a 12 min »), « hier 09:30 », « ! » pour les messages de priorité haute (en-têtes Importance, X-Priority), préfixe Re:/Fwd: en gris
- [x] Icône de pièce jointe selon le genre (image, PDF, archive, tableur…) ; l'étoile d'une conversation est allumée si l'un de ses messages est suivi
- [x] Recherche avancée (De, À, Objet, Texte, période, non lu, suivi, pièce jointe) et syntaxe `from: to: cc: email: subject: text: has:attachment is: date: size: larger: smaller:` — vérifiées contre le vrai Dovecot (décision 148)
- [x] Navigation au clavier dans la liste (flèches, début/fin, pages de 10, espace/inser, Maj+flèche, entrée) et passage à la page suivante (`listnav.js`)
- [x] Glisser-déposer de messages vers un dossier (Ctrl = copie), survol d'un dossier replié (`dnd.js`)
- [x] Conversations (THREAD IMAP) : option « regrouper », pastille de compteur, ouverture d'une conversation, suppression/déplacement/copie/archivage de toute la conversation — vérifiées contre le vrai Dovecot (décision 154)
- [x] Sondage de la boîte de réception toutes les 2 minutes (`/mail/poll`), compteur dans le titre de l'onglet et dans le panneau, notification de bureau et son (réglages)
- [x] Dossiers surveillés (5 en plus de la boîte de réception) : compteurs mis à jour par le sondage
- [x] Dossier parent en gras quand un sous-dossier a des messages non lus
- [x] Étoile de la boîte de réception (filtre des messages suivis)
- [x] Dépôt d'un fichier .eml sur la liste (ajout IMAP APPEND, marqué lu)
- [x] Confirmation avant suppression définitive (corbeille, indésirables)
- [x] Dossier système non configuré : la corbeille, les indésirables, l'archive sont retrouvés ou créés automatiquement (réglable dans Dossiers)

### 14.3 Lecture
- [x] En-tête complet : taille, Cci, Répondre à (menu « i »)
- [x] Statut DKIM lu dans `Authentication-Results` (coche verte ou avertissement)
- [x] Citations longues repliées (élément `details`, sans script)
- [x] Lecture en plein écran (bouton de l'en-tête, Échap pour sortir)
- [x] Détection d'images pisteurs (pixel de 0 ou 1 pixel, adresses de suivi connues) : jamais chargées
- [x] « Toujours afficher les images de cet expéditeur » (liste gérable dans les préférences) ; chargement des images distantes par le serveur (option par boîte : l'expéditeur ne voit pas l'IP), décision 156
- [x] Pièces jointes : visionneuse d'images (précédent/suivant), aperçu du texte, vignettes ; seuls les types sûrs, reconnus d'après les octets, s'affichent en ligne
- [x] Aperçu PDF et lecture audio (dans un nouvel onglet, par le navigateur)
- [x] Dernière action de réponse mémorisée sur le bouton principal

### 14.4 Réglages et comptes
- [x] Modèles de message (création, liste, insertion dans la composition ; variables de date)
- [x] Authentification en deux étapes (TOTP, 9 codes de secours, « ne plus demander 14 jours »), décision 149
- [-] Compteur de non lus de chaque boîte dans le menu : désactivé dans RainLoop 1.17 lui-même (code commenté)
- [x] Ordre des boîtes et des identités supplémentaires (monter/descendre)
- [x] Image de fond personnelle (JPEG/PNG, chiffrée avec le compte)
- [-] Personnalisation des couleurs du thème : RainLoop 1.17 ne propose que l'image de fond (faite) et le choix du thème (rainloop, clair, sombre)
- [x] Options d'affichage : cases à cocher de la liste, animations, disposition (déjà là)
- [x] Choix de la langue sur les pages de connexion, d'inscription et de récupération
- [-] Page d'accueil facultative (fenêtre « bienvenue » de l'administrateur) : sans objet, il n'y a pas d'administrateur
- [-] OpenPGP dans le navigateur, connexion sociale, Dropbox/ownCloud, plug-ins : hors périmètre (sécurité et maintenance)

### 14.5 Composition
- [x] Avertissements à l'envoi : objet vide, pièce jointe annoncée mais absente, plus de 20 destinataires
- [x] Variables de signature (`{{FROM}}`, `{{FROM-FULL}}`, `{{DATE}}`, `{{TIME}}`, `{{MOMENT:format}}`)
- [x] Champs d'adresses en pastilles (validation à la volée, Retour arrière, double-clic pour modifier, glisser d'un champ à l'autre, adresse invalide signalée)
- [x] Images collées ou déposées dans l'éditeur HTML
- [x] Le bouton principal du message répète la dernière action (répondre, répondre à tous, transférer)
- [x] Téléchargement des pièces avec requêtes partielles (reprise)
- [x] Modèles insérables (bouton « Modèle ») ; confirmation lors du changement d'éditeur


## 15. Seconde analyse (fonction par fonction, PHP + JavaScript + gabarits + CSS) et plug-ins de l'utilisateur (2026-10-24)

**Pourquoi.** La première relecture (chapitre 14) comparait des *fonctionnalités* par thème et a laissé passer des écarts
évidents (aperçu d'image en surimpression, signatures HTML listées comme pièces jointes, recherche différente de celle de
RainLoop, invitations iCalendar). La seconde relecture compare **chaque fonction PHP** (`RainLoop/` et `MailSo/`, environ
1 850), **chaque unité JavaScript** de `app.js`, **chaque gabarit HTML** de l'utilisateur, **chaque groupe de règles de
`app.css`** et les fichiers de langue. Les rapports (29 fichiers) sont dans `docs/audit-rainloop/` ; la section « À AJOUTER »
de chacun donne la proposition précise et la priorité.

**Plug-ins de l'utilisateur** (montage `rainloop_plugins`, lecture seule) :
- `ics-preview` : carte de l'invitation `.ics` au-dessus des pièces jointes (titre, date, organisateur, invités, boutons
  « Google » et « Outlook » pour l'ajouter à un agenda). Repris et amélioré : fuseau horaire, fin, lieu, annulation,
  récurrence signalée, description, plusieurs événements (`internal/ical`, décision 158).
- `add-x-originating-ip-header` : en-tête `X-Originating-IP` sur les messages envoyés. Repris (option `compose.x_originating_ip`
  de `config.json`, désactivée par défaut : l'adresse du poste de l'utilisateur est révélée aux destinataires).

**Décisions.**

| N° | Décision |
|---|---|
| 158 | **Invitations iCalendar** : toute pièce `text/calendar` ou `.ics` est lue (`internal/ical`, RFC 5545 : repliement de lignes, échappements, `TZID` (fuseaux embarqués), UTC, date flottante, journée entière, `DURATION`, organisateur et participants avec leur état) ; carte au-dessus des pièces jointes ; liens d'ajout Google et Outlook (ouverts avec `noopener noreferrer`) ; le fichier reste téléchargeable. Limites : 256 Ko par fichier, 3 événements affichés, pas de réponse (accepter/refuser) pour l'instant |
| 159 | **Texte des messages** : jeux de caractères inconnus ou fantaisistes (`asci`, `us-asci`, `utf-7`, inconnu) lus par repli (Windows-1252) au lieu de perdre la partie ; `format=flowed` / `delsp` défaits (RFC 3676) ; texte brut en police à chasse fixe, espaces conservés, citations imbriquées colorées, liens pour `www.`, adresses web, e-mail ; signatures S/MIME et PGP détachées non listées comme pièces jointes ; deuxième partie texte ou HTML « en ligne » (signature, avertissement légal) ajoutée à la suite du corps |
| 160 | **Recherche comme RainLoop** : le texte libre est cherché dans De, À, Cc et Objet (jamais dans le corps : `TEXT` obligeait le serveur à tout lire), le corps par `text:` ou la portée « corps » ; les messages `\Deleted` ne sont pas listés |
| 161 | **Liste** : actualisée sans recharger la page ni fermer le message quand `UIDNEXT` change (sondage) ou au clic sur « actualiser » (cases cochées, sélection, défilement conservés ; pas d'actualisation d'office en page 2 ou en recherche) ; messages marqués lus avant la corbeille et les indésirables ; bouton « pas un indésirable » dans Indésirables, « spam » masqué dans Indésirables, Envoyés, Brouillons, « archiver » masqué dans l'archive et les brouillons ; menus « déplacer vers » et « copier vers » présentés comme le panneau (système d'abord, séparateur, arbre indenté, dossier courant grisé) |

**État.** Faits : décisions 158 à 161, œil sur la vignette, visionneuse plein écran. À faire, par rapport (voir `docs/audit-rainloop/`) :
fidélité visuelle DarkShine (barres de défilement fines, états des boutons, focus des champs, messages d'état, pastilles,
barre de composition, en-tête de composition gris, bandeau de recherche, liste vide centrée, aperçu « N sélectionnés »,
conversations non lues), contacts en deux volets et « écrire à la sélection », boutons des réglages, éditeur riche pour les
signatures et modèles, Reply-To/Cci de l'identité principale, disposition « aperçu en bas », lecture par partie IMAP pour les
gros messages, IDN/punycode, assainissement HTML plus fidèle, `HTMLToText` plus fidèle, liens `mailto:` du corps vers la
composition, dates de liste « à la RainLoop », réponse (accepter/refuser) aux invitations, et les P3 listés dans les rapports.

## 16. Déploiement HTTPS et délais (2026-10-24, soir)

**Décision 162 — terminaison TLS par un proxy.** wmail-go ne sert que du HTTP ; le chiffrement est assuré par un
proxy devant lui. Un déploiement complet est fourni dans `deploy/` : `deploy/docker-compose.yaml` (wmail-go sans
port publié + nginx non privilégié en lecture seule, fichiers temporaires en tmpfs), `deploy/nginx.conf`
(TLS 1.2/1.3, HTTP/2, `client_max_body_size 64m`, `proxy_buffering off`, délais de 300 s) et
`config/config.https.json` (`trusted_proxy: true`). Cibles : `make tls-cert` (certificat auto-signé de 825 jours,
`TLS_HOST=` pour le nom), `make tls-up`, `make tls-down`, `make tls-logs`, `make tls-ps`. Le volume de données du
déploiement est distinct de celui du développement. nginx n'ajoute aucun en-tête de sécurité : CSP, HSTS,
`X-Frame-Options` et `Referrer-Policy` viennent de l'application, qui n'envoie HSTS que si elle voit
`X-Forwarded-Proto: https` **et** `trusted_proxy`.

**Décision 163 — délais réglables.** `config.json` reçoit une section `timeouts` : `mail_seconds` (30 par défaut,
toute opération IMAP : liste, lecture, recherche, déplacement, suppression), `send_seconds` (90, envoi SMTP) et
`dial_seconds` (20, établissement TCP et TLS), bornées à la validation (5–600, 10–900, 2–120). Les délais du serveur
HTTP passent de 30 s à 5 minutes en lecture et en écriture : ils coupaient le téléversement d'une grosse pièce
jointe et l'envoi SMTP, alors que les délais métier ci-dessus sont la vraie borne. Restent fixes : 20 s pour le
bouton « Test » d'une boîte, 3 s par commande ManageSieve, 5 min d'inactivité avant fermeture d'une connexion IMAP
du pool, 2 min entre deux sondages.

**Décision 165 — thèmes crépusculaires.** Deux thèmes s'ajoutent à « rainloop » : « ciel-crepuscule » (nouveau
défaut de `default_theme`, donc fond de la page de connexion) et « mer-de-nuages-crepuscule ». Ce sont des copies
de DarkShine : le jeu de variables CSS est identique au caractère près, seul le fond d'écran change. Les fonds sont
des SVG vectoriels livrés avec l'application, sans script ni appel extérieur, et chacun reste remplaçable par une
image personnelle déposée dans `<data_dir>/themes/<nom>/background.jpg`. Un test compare la liste des variables des
trois thèmes : une couleur ajoutée à l'un doit l'être aux trois.

**Décision 164 — vérification visuelle obligatoire.** Toute correction d'interface est contrôlée par une capture
d'écran (Chromium sans tête) avant d'être annoncée : c'est ainsi qu'ont été trouvés la barre de quota grise
(collision de la classe `.bar`), le nom de dossier tronqué (`flex: 1` sur l'étoile) et la bannière d'images
distantes sortie de sa barre (`<form>` dans un `<p>`).
