# Maintenance automatique (« CI maison »)

Deux tâches programmées, indépendantes :

| Tâche | Rythme conseillé | Script | Ce qu'elle fait |
|---|---|---|---|
| Veille de vulnérabilités | **tous les jours** | `veille-vulnerabilites.sh` | `govulncheck` sur le dépôt tel quel ; ne modifie rien ; courriel seulement si une faille **atteignable** apparaît |
| Mise à jour des dépendances | **le lundi** | `maj-dependances.sh` | met à jour, teste tout, commite, pousse, récapitulatif par courriel |

Une faille peut être publiée des semaines après la dernière mise à jour : la veille quotidienne est le filet
qui compte. La mise à jour, elle, n'a pas besoin d'être quotidienne — vos dépendances bougent peu, et chaque
exécution consomme plusieurs minutes de construction et de tests.


`maj-dependances.sh` met à jour les dépendances Go chaque nuit, **sans jamais publier quelque chose qui ne passe
pas toutes les barrières**.

## Ce que fait le script

1. Vérifie que l'arbre de travail est propre (sinon il ne touche à rien).
2. Vérifie l'accès SSH à chaque dépôt distant, puis `git pull --ff-only`.
3. `make vuln` (état de sécurité avant).
4. `make deps-update` : `go get -u ./...`, `go mod tidy`, construction, tests.
5. Barrières complètes : `make fmt vet`, `make test-race`, `make test-js`, `make build`, `make vuln`.
6. Complète `CHANGELOG.md` (liste des modules et des versions), commite, pousse sur chaque dépôt distant.
7. Envoie un récapitulatif par courriel.

À la moindre erreur : `go.mod` et `go.sum` sont remis en état, **rien n'est commité ni poussé**, et le courriel
contient la raison et les dernières lignes du journal.

## Installation (minuteur systemd, recommandé)

```bash
mkdir -p ~/.config/systemd/user
cp tools/maintenance/wmail-go-maj.{service,timer} ~/.config/systemd/user/
printf 'CLE_SSH=%s/.ssh/id_wmail_go_maj\nREMOTES=origin gitea\n' "$HOME" > ~/.config/wmail-go-maj.env
systemctl --user daemon-reload
systemctl --user enable --now wmail-go-maj.timer
loginctl enable-linger "$USER"      # le minuteur tourne même sans session ouverte
systemctl --user list-timers wmail-go-maj.timer
```

Le minuteur est préférable à cron avec Podman sans privilèges : la session utilisateur (et donc
`XDG_RUNTIME_DIR`) est correctement mise en place, ce qu'un cron classique ne garantit pas.

## Installation (cron)

`crontab -e` sous l'utilisateur qui possède le dépôt et les conteneurs :

```cron
# Environnement : cron ne charge ni votre profil ni votre session.
SHELL=/bin/sh
PATH=/usr/local/bin:/usr/bin:/bin
MAILTO=""
WMAIL=/srv/wmail-go
WMAIL_MAJ_CONF=/srv/wmail-go/.maj.env

# Veille de vulnérabilités, tous les jours à 6 h 20 (courriel seulement en cas de faille atteignable).
20 6 * * *   cd $WMAIL && XDG_RUNTIME_DIR=/run/user/$(id -u) ./tools/maintenance/veille-vulnerabilites.sh >> $WMAIL/.cron-veille.log 2>&1

# Mise à jour des dépendances, le lundi à 4 h 30.
30 4 * * 1   cd $WMAIL && XDG_RUNTIME_DIR=/run/user/$(id -u) ./tools/maintenance/maj-dependances.sh >> $WMAIL/.cron-maj.log 2>&1
```

`$(id -u)` n'est pas interprété par cron dans la ligne : remplacez-le par votre identifiant numérique
(`id -u` vous le donne), ou passez par le minuteur systemd ci-dessus qui s'en charge.

Variante minuteur systemd (recommandée avec Podman sans privilèges) : copiez aussi
`wmail-go-veille.{service,timer}` et activez-les de la même manière.

## Clé SSH dédiée

Un cron n'a pas d'agent ssh : il faut une clé **sans phrase de passe**, réservée à cet usage.

```bash
ssh-keygen -t ed25519 -N '' -C 'wmail-go maj auto' -f ~/.ssh/id_wmail_go_maj
```

- GitHub : dépôt → Settings → Deploy keys → ajouter la clé publique avec **Allow write access**.
- Gitea : dépôt → Paramètres → Clés de déploiement, avec droit d'écriture.

Une clé de déploiement ne donne accès qu'à **ce** dépôt : si elle fuit, le reste de vos comptes n'est pas exposé.

## Réglages

| Variable | Défaut | Rôle |
|---|---|---|
| `BRANCHE` | `main` | branche mise à jour |
| `REMOTES` | `origin gitea` | dépôts distants où pousser |
| `DESTINATAIRE` | `wmail-go@degoulet.net` | récapitulatif |
| `MAILER` | `sendmail -t` | programme d'envoi (par exemple `msmtp -t`) |
| `NOTIFIER` | `changes` | `always`, `changes`, `failures`, `never` |
| `CLE_SSH` | — | clé privée dédiée |
| `SANS_PUSH` | — | `1` : tout faire sauf pousser |
| `SANS_JS` | — | `1` : sauter `make test-js` (gagne plusieurs minutes) |
| `JOURNAL` | `.maj-dependances.log` | journal complet |

## Essai à blanc

```bash
SANS_PUSH=1 NOTIFIER=never ./tools/maintenance/maj-dependances.sh
```

Le dépôt est mis à jour et commité **en local** ; `git reset --hard HEAD~1` annule l'essai.
