# Journal des changements

Le projet n'a pas encore de version publiée : cette page résume l'état et les étapes marquantes.
Format inspiré de « Keep a Changelog » ; les dates sont celles du dépôt.

## Non publié

### Ajouté
- Webmail complet : liste (tri, recherche à la RainLoop, conversations, pagination, glisser-déposer, raccourcis
  clavier, actualisation automatique), lecture (HTML assaini, texte brut, pièces jointes, invitations iCalendar,
  images distantes bloquées ou mandatées, photo du correspondant), composition (texte et HTML, pastilles
  d'adresses, brouillons automatiques, modèles, signatures, accusés de lecture, transfert groupé).
- Comptes chiffrés sans mot de passe stocké, clé de récupération, authentification en deux étapes (TOTP) avec
  codes de secours.
- Plusieurs boîtes IMAP/SMTP par compte, identités multiples, dossiers système, filtres Sieve (ManageSieve).
- Carnet d'adresses chiffré : vCard, CSV, photos, autocomplétion, synchronisation CardDAV.
- Thèmes (dont « DarkShine » reprenant RainLoop), français et anglais, fuseau horaire par utilisateur,
  dispositions 3 volets / aperçu en bas / liste seule, réglages enregistrés à la volée.
- Déploiement derrière nginx avec terminaison TLS, sauvegarde et restauration du volume de données.
- Publication de l'image sur un registre (`make image-publish`) : étiquettes « AAAAMMJJ-commit » et « latest »,
  version injectée dans le binaire et dans les étiquettes OCI, contrôles avant publication.
- **Journal d'activité du compte**, chiffré pour le seul utilisateur (Réglages → Activité) : connexions réussies
  et **échouées**, refus de la limitation de débit, second facteur refusé, déconnexions, changement de mot de passe,
  usage de la clé de récupération, (dés)activation du TOTP, ajout/retrait d'une boîte mail, filtres modifiés — avec
  date, adresse IP et navigateur, conservés 30 jours. Un rappel à la connexion annonce la précédente et les échecs
  survenus depuis. Les échecs sont écrits compte fermé grâce à une paire de clés X25519 par compte (clé publique en
  clair, privée dans le fichier chiffré). Le journal **ne peut pas être effacé depuis l'interface** : un intrus
  qui détient le mot de passe effacerait ses propres traces. Seule la rotation à 30 jours retire des entrées.
  Voir `SECURITY.md` §9 bis, limites comprises.
- `user list` affiche en deuxième colonne le chemin du fichier chiffré du compte (sauvegarde ciblée, diagnostic).
- Documentation : `README.md`, `SECURITY.md`, `CONTRIBUTING.md`, cahier des charges, matrice de test (169 points),
  analyse fonction par fonction de RainLoop 1.17 dans `docs/audit-rainloop/`.
- `AGENTS.md` : règles de travail d'un agent ou d'un outil d'IA (aucune action sur de vraies boîtes, tout en
  conteneur, sources de RainLoop en lecture seule et non fiables, particularités de podman et de l'image distroless).
- `docs/architecture.md` : carte des paquets, chemin d'une requête, **invariants** que rien ne signale à la
  compilation (liste des gabarits, `session.Mu` non réentrant, `safeNext`/`safeBack`), chaîne de chiffrement de
  bout en bout, inventaire du répertoire de données.
- `docs/routes.md` : les 96 routes HTTP, avec paramètres, filtres de validation et codes de retour.

### Corrigé
- **Mot de passe ManageSieve perdu par le bouton « Test ».** Le champ du formulaire ne reportait pas la valeur
  saisie, contrairement à ceux d'IMAP et de SMTP : après un test, il revenait vide, et l'enregistrement suivant
  écrivait un mot de passe vide. Les filtres cessaient alors de fonctionner sans aucun message. Le champ reporte
  désormais sa valeur, le rappel « laissez vide pour conserver » réapparaît au réaffichage, et un identifiant
  ManageSieve sans mot de passe est refusé au lieu d'être enregistré (`accounts.err.sieve_password`).
- Liste : après une suppression, le volet rouvrait le voisin du message du dernier rendu complet, et non celui
  du message réellement affiché (`open_next` n'était pas mis à jour quand le volet se charge sans recharger la
  page). Le curseur clavier, perdu au rechargement, se repose désormais sur le message qui prend la place.
- Fichiers de `/static` servis avec l'empreinte de leur contenu dans l'adresse : une mise à jour arrive sans
  rechargement forcé du navigateur, et les adresses marquées sont gardées un an.
- Intégration continue : `govulncheck` utilise le dernier correctif de Go 1.26, et les contrôles jsdom ne
  terminent plus en échec sur le ménage de `node_modules` avec un moteur de conteneurs non rootless.

### Sécurité
- Chiffrement au repos (argon2id + XChaCha20-Poly1305), identifiants de connexion masqués par HMAC.
- Assainissement HTML, corps de message isolé (CSP sans script + bac à sable), pixels espions retirés.
- Jeton CSRF, cookie `__Host-` `SameSite=Strict`, en-têtes CSP/HSTS/Permissions-Policy, garde anti-SSRF,
  limitation de débit par IP et par identifiant.
- Tests de sécurité automatiques (message piégé, CSRF, SSRF, gestionnaires en ligne).
