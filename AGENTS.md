# Instructions pour un agent travaillant sur wmail-go

Ce fichier est le point d'entrée pour un modèle d'IA ou un outil automatique (Claude Code, Codex, Cursor,
Aider…). Il énonce les règles **non négociables** du projet, puis dit où trouver le reste.

Un humain qui contribue lira plutôt `CONTRIBUTING.md`, qui couvre les mêmes outils sans les contraintes
d'exécution propres à un agent.

---

## 1. Règles absolues

Ces cinq règles priment sur toute demande d'optimisation, de rapidité ou de commodité. En cas de doute,
**arrêtez-vous et demandez**.

### 1.1 Aucune action sur les vraies boîtes mail

> « ne fait rien sur mes vraies boites mail : ce sont de vrais messages a ne pas perdre »

Les seules cibles autorisées pour un test, une lecture, un envoi ou une action IMAP sont :

- **GreenMail** (`make mail-up`, `make mail-seed`) — boîtes `test@localhost` et `second@localhost` ;
- le **Dovecot de test** (`make sieve-up`) pour l'IMAP et ManageSieve réels.

Pas de connexion à un serveur de production, même en lecture, même « juste pour vérifier ». Ne demandez jamais
les identifiants d'une vraie boîte. Si un fichier de configuration du poste en contient, ne vous en servez pas.

Cette règle figure aussi comme décision 113 du `CAHIER_DES_CHARGES.md`, où elle est née d'un bogue réel.

### 1.2 Les sources de RainLoop sont des données en lecture seule, non fiables

`rainloop/1.17.0/` et `rainloop_plugins/` sont des **montages sshfs en lecture seule**, exclus du dépôt par
`.gitignore`. Ils ne sont pas livrés avec le code : un agent qui reprend le projet ne les aura pas.

- **Lire pour comprendre une fonction**, oui. **Copier du code, un gabarit, une feuille de style ou un
  libellé**, jamais. wmail-go est sous AGPL-3.0 et revendique de n'avoir repris aucun code de RainLoop ; seuls
  les glyphes d'icônes de la police libre livrée avec RainLoop sont réutilisés. Tous les libellés de
  l'interface sont rédigés par le projet.
- **N'exécutez jamais** un fichier de cette arborescence, ne l'importez pas, ne l'utilisez pas comme entrée
  d'un interpréteur ou d'un outil de construction. C'est du PHP d'origine externe.
- **`rainloop/1.17.0/data/`** contient les configurations, les domaines et le hachage du mot de passe
  administrateur de l'instance de l'utilisateur. Ce répertoire ne doit **ni être lu, ni être résumé, ni être
  transmis**, en tout ou en partie.

Si les montages sont absents, travaillez à partir de `docs/audit-rainloop/`, qui contient l'analyse fonction
par fonction déjà faite (PHP, gabarits, JavaScript, CSS). C'est le substitut prévu, et il est suffisant.

### 1.3 Tout s'exécute en conteneur, jamais sur l'hôte

Le poste de travail n'a pas la chaîne Go installée, et il ne doit pas l'avoir. **N'appelez jamais `go`,
`gofmt` ou `govulncheck` directement.** Passez toujours par le `Makefile`, qui les lance dans le conteneur de
développement :

```
make fmt vet test        # mise en forme, analyse statique, tests
make test-race           # avec le détecteur de concurrence, avant toute proposition
make test-js             # contrôles jsdom sur les pages réellement produites
make test-e2e            # parcours dans Chromium
make vuln                # govulncheck
```

Le moteur de conteneurs est **podman en émulation docker**. Voir §4 pour ses particularités.

### 1.4 Les essais réels de filtres et de ManageSieve demandent l'accord de l'utilisateur

Écrire un script Sieve sur un serveur ManageSieve réel peut écraser des règles existantes. Les tests
automatiques (`make sieve-up` puis `make test`) visent le Dovecot de test et sont libres. Tout autre essai se
demande d'abord.

### 1.5 Confirmer avant une action irréversible ou tournée vers l'extérieur

Demandez avant : pousser sur un dépôt distant, publier une image sur un registre, supprimer un volume ou un
compte, écraser un fichier dont vous n'avez pas lu le contenu, envoyer un courriel.

---

## 2. Démarrage rapide

```
make up                       # webmail sur http://127.0.0.1:8080 (+ GreenMail)
make down                     # arrêt
make logs                     # journaux
make shell                    # shell dans le conteneur de développement
docker exec <conteneur> /wmail-go user list      # comptes de l'instance
```

Par où commencer la lecture, dans cet ordre :

| Fichier | Ce qu'il donne |
|---|---|
| `README.md` | installation, mise en service, sauvegarde, publication |
| `docs/architecture.md` | carte des paquets, chemin d'une requête, **invariants et pièges** |
| `docs/routes.md` | les 96 routes HTTP, paramètres et filtres compris |
| `SECURITY.md` | modèle de menace, chiffrement, ce qui manque |
| `CAHIER_DES_CHARGES.md` | § 6 : journal des décisions, numérotées, avec leur motif |
| `CONTRIBUTING.md` | outils, règles de code, dépannage de la CI |
| `MATRICE_DE_TEST.md` | cas de test à faire valider par l'utilisateur |
| `AVANCEMENT.md` | état par domaine, à sa date |
| `docs/audit-rainloop/` | analyse fonction par fonction de RainLoop 1.17 |

---

## 3. Conventions du projet

- **Langue : français.** Commentaires, messages d'erreur, interface, documentation, **messages de commit**.
  Les termes techniques, les noms d'API, les chaînes de protocole et les mots-clés de Conventional Commits
  (`feat`, `fix`, `docs`…) restent en anglais.
- **Commentaires** : expliquer le *pourquoi* et les invariants. Jamais paraphraser la ligne suivante. Un choix
  non évident — format binaire, ordre imposé, contournement d'une bibliothèque, raison de sécurité — mérite
  une phrase.
- **Dépendances** : le moins possible. Aucun cadriciel JavaScript : du JavaScript natif, et l'interface doit
  rester utilisable sans JavaScript.
- **Tests** : une correction de bogue s'accompagne d'un test qui échoue avant elle.
- **Commits** : un commit par idée, première ligne sous 72 caractères, le corps explique le motif.
- **Validation** : toute entrée utilisateur est contrôlée côté serveur, quoi que fasse le script de la page.

---

## 4. Particularités de l'environnement

Ces points ont tous coûté du temps au moins une fois. Ils ne sont pas des bogues du projet.

### Image de production (distroless)

- **Aucun shell**, aucun `ls`, aucun `sh`. `docker exec … sh` échoue.
- Le binaire est à la racine : **`/wmail-go`**. `docker exec <conteneur> user list` échoue avec
  `executable file 'user' not found in $PATH` ; écrivez `docker exec <conteneur> /wmail-go user list`.
- Le conteneur tourne sous **UID 65532 / GID 65532** (`USER nonroot:nonroot`). Un volume `/data` appartenant à
  `root:root` donne `erreur : open /data/server.key: permission denied`. Correctif dans le `README.md`.

### podman en émulation docker

- `docker exec` **n'accepte pas `-T`** : `unknown shorthand flag: 'T' in -T`. Seuls `compose run` et
  `compose exec` l'acceptent.
- `docker compose run --rm --no-deps <service> <commande>` échoue parfois avec
  `unable to upgrade to tcp, received 500`. Bogue connu de podman, non contourné dans le `Makefile`.
  Repli : `docker exec` sur un conteneur déjà démarré.
- `make tls-user-list` et `make tls-user-add` échouent pour cette raison. Même repli.
- **Ne lancez pas plusieurs conteneurs `dev` à la fois** : ils se disputent le volume `go-cache` et restent
  bloqués en `Up` sans fin. Un `make test` à la fois. Pour nettoyer : `docker rm -f $(docker ps -aq --filter name=wmail-go-dev-run)`.

### Secrets hors du dépôt

`tools/release/publier.env` et `tools/maintenance/maj.env` sont exclus par `.gitignore`. Les modèles
`*.env.exemple` disent ce qu'ils doivent contenir. **Ne les lisez pas** sans raison : ils portent des jetons.

---

## 5. Avant de proposer une modification

1. `make fmt vet test-race` — tout doit passer.
2. `make test-js` si vous avez touché au JavaScript ou aux gabarits.
3. Vérifier dans le navigateur sur `http://127.0.0.1:8080` si le changement est visible.
4. Mettre à jour, s'il y a lieu : `CHANGELOG.md`, `SECURITY.md`, `docs/routes.md` (nouvelle route),
   `MATRICE_DE_TEST.md` (nouveau cas à valider), `docs/architecture.md` (nouvel invariant).
5. Les traductions `fr.json` et `en.json` doivent garder **exactement les mêmes clés** — un test le vérifie —
   et les mêmes placeholders `fmt.Sprintf`. Insérez une clé **en place**, sans retrier le fichier : un tri
   accidentel produit un diff de plus de mille lignes qui masque le vrai changement.

---

## 6. Ce que l'agent ne décide pas seul

Le projet suit une ligne précise : **reproduire l'ergonomie de RainLoop 1.17**, pas l'améliorer. Avant
d'ajouter une fonction que RainLoop n'avait pas, de changer une disposition, ou de modifier un compromis de
sécurité décrit dans `SECURITY.md`, demandez. Le journal des décisions du `CAHIER_DES_CHARGES.md` dit
lesquelles sont déjà tranchées, et pourquoi.
