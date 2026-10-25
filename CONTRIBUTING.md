# Contribuer à wmail-go

Merci de l'intérêt porté au projet. Ce document dit comment construire, tester et proposer une modification.

## En deux minutes

```bash
git clone https://github.com/gdegoulet/wmail-go.git && cd wmail-go
make up                    # construit l'image et démarre  →  http://127.0.0.1:8080
make mail-up mail-seed     # serveur IMAP/SMTP de test (GreenMail) et messages de démonstration
make user-add LOGIN=alice  # crée un compte webmail
```

Aucun Go, aucun Node, aucun navigateur n'est nécessaire sur votre poste : **tout tourne en conteneur** (Docker ou
Podman avec le greffon compose). C'est une règle du projet, pas une préférence : elle garantit que le résultat des
tests est le même pour tout le monde.

## Construire et tester

| Commande | Ce qu'elle fait |
|---|---|
| `make build` | construit l'image de production (binaire statique, ~18 Mo) |
| `make test` | `go test ./...` dans le conteneur de développement |
| `make test-race` | les mêmes tests avec le détecteur de concurrence (à lancer avant toute proposition) |
| `make test-js` | contrôles jsdom sur les pages **réellement produites** par le serveur |
| `make test-e2e` | parcours dans Chromium (composition, filtres, dossiers, contacts) |
| `make sieve-up` puis `make test` | ajoute les tests d'intégration contre un vrai Dovecot (IMAP + ManageSieve) |
| `make vet`, `make fmt` | analyse statique et mise en forme |
| `make vuln` | failles connues (govulncheck) des dépendances et de Go |
| `make deps-outdated`, `make deps-update` | dépendances en retard, mise à jour groupée |
| `make logs`, `make shell` | journaux, shell dans le conteneur de développement |

L'intégration continue rejoue `gofmt`, `go vet`, `go test -race`, les contrôles JavaScript et la construction de
l'image. Une proposition qui ne passe pas ces étapes ne sera pas fusionnée.

## Structure du dépôt

```
cmd/wmail-go/        point d'entrée (serve, user, healthcheck)
internal/config/     lecture et validation de config.json
internal/vault/      fichier utilisateur chiffré (argon2id + XChaCha20-Poly1305)
internal/store/      comptes, index, lecture/écriture des fichiers chiffrés
internal/session/    sessions en mémoire
internal/mailconn/   IMAP et SMTP (pool, recherche, garde anti-SSRF)
internal/mailview/   analyse MIME, assainissement HTML, texte, vignettes
internal/compose/    construction des messages sortants, adresses, MDN
internal/contacts/   carnet d'adresses (vCard, CSV, CardDAV)
internal/sieve/      ManageSieve et règles de filtrage
internal/web/        serveur HTTP, gabarits, CSS et JavaScript de l'interface
internal/i18n/       traductions (fr, en)
deploy/              déploiement derrière nginx (TLS)
tools/               outils de test (jsdom, Chromium, Dovecot, GreenMail, captures)
docs/audit-rainloop/ analyse fonction par fonction de RainLoop 1.17
```

## Règles de code

- **Langue** : tout est en français — commentaires, messages d'erreur, interface, messages de commit. Les termes
  techniques, les noms d'API et les chaînes de protocole restent en anglais.
- **Commentaires** : expliquer le *pourquoi* et les invariants, jamais paraphraser la ligne suivante. Un choix non
  évident (format binaire, ordre imposé, contournement d'une bibliothèque, raison de sécurité) mérite une phrase.
- **Dépendances** : le moins possible, et seulement des bibliothèques maintenues. L'interface n'utilise **aucun**
  cadriciel JavaScript : du JavaScript natif, sans dépendance, qui doit rester utilisable sans JavaScript.
- **Sécurité** : lisez `SECURITY.md` avant de toucher à l'authentification, à l'assainissement HTML, aux en-têtes
  ou aux connexions sortantes. Toute nouvelle entrée utilisateur est validée côté serveur.
- **Tests** : une correction de bogue s'accompagne d'un test qui échoue avant elle. Une modification d'interface
  s'accompagne d'une capture d'écran dans la proposition.
- **Jamais de vraie boîte mail** dans les tests : GreenMail et le Dovecot de test suffisent.

## Proposer une modification

1. Ouvrez un ticket d'abord si le changement est important (le projet suit une ligne précise : reproduire
   l'ergonomie de RainLoop 1.17).
2. Branche dédiée, commits en français, un commit par idée. Le message explique le *pourquoi* ; la première ligne
   fait moins de 72 caractères.
3. Lancez `make test-race`, `make test-js`, `make vet`, `make fmt`.
4. Ouvrez la proposition en remplissant le modèle (il liste les vérifications attendues).

## Signaler une faille de sécurité

Pas dans un ticket public : suivez `SECURITY.md`.
