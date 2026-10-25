#!/bin/sh
# Mise à jour automatique des dépendances, à lancer par cron ou par un minuteur systemd.
#
#   1. récupère la dernière version du dépôt (git pull --ff-only)
#   2. relève les vulnérabilités connues AVANT (govulncheck)
#   3. met à jour les dépendances (go get -u ./... && go mod tidy)
#   4. passe toutes les barrières : construction, go vet, tests avec détecteur de concurrence,
#      contrôles JavaScript, puis govulncheck APRÈS
#   5. si et seulement si tout passe : complète le journal des changements, commite, pousse sur
#      chaque dépôt distant configuré
#   6. envoie un courriel récapitulatif
#
# En cas d'échec à n'importe quelle étape : go.mod et go.sum sont remis dans leur état d'origine,
# rien n'est commité ni poussé, et le courriel contient la raison et les dernières lignes du journal.
#
# Réglages (variables d'environnement, ou fichier indiqué par WMAIL_MAJ_CONF) :
#   DEPOT            chemin du dépôt                       (défaut : racine déduite du script)
#   BRANCHE          branche à mettre à jour               (défaut : main)
#   REMOTES          dépôts distants où pousser            (défaut : « origin gitea »)
#   DESTINATAIRE     adresse du récapitulatif              (défaut : wmail-go@degoulet.net)
#   EXPEDITEUR       adresse d'expédition                  (défaut : wmail-go@<nom de machine>)
#   MAILER           commande d'envoi lisant l'en-tête     (défaut : sendmail -t)
#   NOTIFIER         always | changes | failures           (défaut : changes)
#   JOURNAL          fichier journal                       (défaut : <dépôt>/.maj-dependances.log)
#   SANS_PUSH        1 pour tout faire sauf pousser        (défaut : vide)
#   SANS_JS          1 pour sauter make test-js (lourd)    (défaut : vide)
#   CLE_SSH          clé privée à utiliser pour git         (défaut : vide = configuration ssh habituelle)
#
# Accès SSH : les dépôts distants sont en SSH (git@github.com, git.degoulet.net:2223). Un cron n'hérite
# d'aucun agent ssh : fournissez une clé **sans phrase de passe** réservée à cet usage, via CLE_SSH, ou
# déclarez-la dans ~/.ssh/config pour l'utilisateur du cron. La clé n'a besoin que du droit d'écriture
# sur ce dépôt (clé de déploiement GitHub avec « Allow write access », clé dédiée côté Gitea).
#
# Prérequis : git, make, et Docker ou Podman utilisables par l'utilisateur du cron.
# Avec Podman sans privilèges, le cron doit disposer de XDG_RUNTIME_DIR et DBUS_SESSION_BUS_ADDRESS
# (voir tools/maintenance/wmail-go-maj.timer, qui règle ce point).

set -eu

# --- réglages ---------------------------------------------------------------
ICI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEPOT=${DEPOT:-$(CDPATH= cd -- "$ICI/../.." && pwd)}
[ -n "${WMAIL_MAJ_CONF:-}" ] && [ -f "$WMAIL_MAJ_CONF" ] && . "$WMAIL_MAJ_CONF"
# Projet compose distinct : l'automate ne doit pas toucher aux conteneurs ni aux volumes du webmail en service.
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-wmail-go-ci}
BRANCHE=${BRANCHE:-main}
REMOTES=${REMOTES:-"origin gitea"}
DESTINATAIRE=${DESTINATAIRE:-wmail-go@degoulet.net}
EXPEDITEUR=${EXPEDITEUR:-wmail-go@$(hostname -f 2>/dev/null || hostname)}
MAILER=${MAILER:-"sendmail -t"}
NOTIFIER=${NOTIFIER:-changes}
JOURNAL=${JOURNAL:-"$DEPOT/.maj-dependances.log"}

cd "$DEPOT"

# Clé dédiée pour git : un cron n'a ni agent ssh ni terminal pour une phrase de passe.
if [ -n "${CLE_SSH:-}" ]; then
	GIT_SSH_COMMAND="ssh -i $CLE_SSH -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new"
	export GIT_SSH_COMMAND
else
	GIT_SSH_COMMAND="${GIT_SSH_COMMAND:-ssh -o BatchMode=yes}"
	export GIT_SSH_COMMAND
fi

# --- journalisation ---------------------------------------------------------
SORTIE=$(mktemp)                      # tout ce que le courriel reprendra
trap 'rm -f "$SORTIE" "$SORTIE.cmd" 2>/dev/null || true' EXIT

dire() { printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$SORTIE" "$JOURNAL" >/dev/null; }

# etape « titre » commande… : exécute, journalise, et s'arrête à la première erreur.
etape() {
	titre=$1
	shift
	dire "── $titre"
	if "$@" >"$SORTIE.cmd" 2>&1; then
		tail -n 20 "$SORTIE.cmd" >>"$SORTIE"
		tail -n 20 "$SORTIE.cmd" >>"$JOURNAL"
		return 0
	fi
	dire "ÉCHEC : $titre"
	tail -n 60 "$SORTIE.cmd" >>"$SORTIE"
	tail -n 60 "$SORTIE.cmd" >>"$JOURNAL"
	return 1
}

# --- courriel ---------------------------------------------------------------
# envoyer « sujet » : construit l'en-tête et passe le tout au programme d'envoi.
envoyer() {
	sujet=$1
	[ "$NOTIFIER" = "never" ] && return 0
	if ! command -v "${MAILER%% *}" >/dev/null 2>&1; then
		dire "pas de programme d'envoi ($MAILER) : récapitulatif seulement dans $JOURNAL"
		return 0
	fi
	{
		printf 'From: %s\n' "$EXPEDITEUR"
		printf 'To: %s\n' "$DESTINATAIRE"
		printf 'Subject: %s\n' "$sujet"
		printf 'Content-Type: text/plain; charset=utf-8\n'
		printf 'X-Mailer: wmail-go maj-dependances\n\n'
		cat "$SORTIE"
	} | $MAILER || dire "envoi du courriel impossible"
}

# arreter « raison » : remet go.mod et go.sum en état, prévient, et sort en erreur.
arreter() {
	dire "ARRÊT : $1"
	git checkout -- go.mod go.sum 2>/dev/null || true
	envoyer "[wmail-go] mise à jour des dépendances ÉCHOUÉE : $1"
	exit 1
}

# --- un seul exemplaire à la fois ------------------------------------------
VERROU="$DEPOT/.maj-dependances.lock"
if command -v flock >/dev/null 2>&1; then
	exec 9>"$VERROU"
	flock -n 9 || { echo "une autre exécution est en cours ($VERROU)"; exit 0; }
fi

dire "=== mise à jour des dépendances de wmail-go ($(git rev-parse --short HEAD))"

# --- 1. état du dépôt -------------------------------------------------------
if [ -n "$(git status --porcelain)" ]; then
	dire "l'arbre de travail n'est pas propre : rien n'est touché"
	git status --short >>"$SORTIE"
	envoyer "[wmail-go] mise à jour ignorée : arbre de travail modifié"
	exit 0
fi
git rev-parse --verify "$BRANCHE" >/dev/null 2>&1 || arreter "branche $BRANCHE inconnue"
[ "$(git rev-parse --abbrev-ref HEAD)" = "$BRANCHE" ] || etape "bascule sur $BRANCHE" git checkout "$BRANCHE" || arreter "bascule impossible"
# Accès SSH vérifié avant toute chose : sans lui, inutile d'aller plus loin (et le message est clair).
for r in $REMOTES; do
	git remote get-url "$r" >/dev/null 2>&1 || continue
	etape "accès à $r ($(git remote get-url "$r"))" git ls-remote --heads "$r" "$BRANCHE" ||
		arreter "accès SSH refusé pour $r : fournissez une clé sans phrase de passe (CLE_SSH) ou configurez ~/.ssh/config"
done
etape "récupération ($BRANCHE)" git pull --ff-only || arreter "git pull impossible (divergence ?)"

AVANT=$(git rev-parse --short HEAD)
MODULES_AVANT=$(mktemp)
go_liste() { make -s deps-outdated 2>/dev/null || true; }
go_liste >"$MODULES_AVANT"

# --- 2. état de sécurité avant ---------------------------------------------
etape "vulnérabilités connues (avant)" make vuln || dire "govulncheck signale déjà quelque chose : la mise à jour peut le corriger"

# --- 3. mise à jour ---------------------------------------------------------
etape "mise à jour des modules" make deps-update || arreter "go get -u / go mod tidy / tests en échec"

if git diff --quiet -- go.mod go.sum; then
	dire "aucune dépendance en retard : rien à faire"
	[ "$NOTIFIER" = "always" ] && envoyer "[wmail-go] dépendances déjà à jour"
	exit 0
fi

DIFF_MOD=$(git diff --unified=0 -- go.mod | grep -E '^[+-]\s+[a-z0-9].*/' | grep -v '^[+-][+-]' || true)
dire "modules modifiés :"
printf '%s\n' "$DIFF_MOD" >>"$SORTIE"

# --- 4. barrières -----------------------------------------------------------
etape "mise en forme et analyse statique" make fmt vet || arreter "gofmt ou go vet"
etape "tests Go (détecteur de concurrence)" make test-race || arreter "tests Go"
if [ -z "${SANS_JS:-}" ]; then
	etape "contrôles JavaScript" make test-js || arreter "contrôles JavaScript"
else
	dire "contrôles JavaScript sautés (SANS_JS=1)"
fi
etape "construction de l'image" make build || arreter "construction de l'image"
etape "vulnérabilités connues (après)" make vuln || arreter "govulncheck signale une faille atteignable après mise à jour"

# --- 5. journal des changements, commit, poussée ----------------------------
DATE=$(date '+%Y-%m-%d')
python3 - "$DATE" <<'PY' || arreter "mise à jour du journal des changements"
import subprocess, sys, re
date = sys.argv[1]
diff = subprocess.run(["git", "diff", "--unified=0", "--", "go.mod"], capture_output=True, text=True).stdout
# « + 	module v1.2.3 » / « - 	module v1.2.2 » : on garde la version d'arrivée de chaque module.
versions = {}
for ligne in diff.splitlines():
    m = re.match(r'^([+-])\t([^\s]+) (v[^\s/]+)', ligne)
    if m:
        signe, module, version = m.groups()
        versions.setdefault(module, {})[signe] = version
lignes = []
for module in sorted(versions):
    v = versions[module]
    if "+" in v and "-" in v and v["+"] != v["-"]:
        lignes.append(f"  - `{module}` {v['-']} → {v['+']}")
    elif "+" in v and "-" not in v:
        lignes.append(f"  - `{module}` {v['+']} (ajouté)")
if not lignes:
    sys.exit(0)
entree = f"- Dépendances mises à jour ({date}) :\n" + "\n".join(lignes) + "\n"
chemin = "CHANGELOG.md"
texte = open(chemin, encoding="utf-8").read()
repere = "### Sécurité"
if repere in texte:   # entrée insérée juste avant la section Sécurité de « Non publié »
    texte = texte.replace(repere, entree + "\n" + repere, 1)
else:
    texte = texte.rstrip("\n") + "\n\n" + entree
open(chemin, "w", encoding="utf-8").write(texte)
PY

git add go.mod go.sum CHANGELOG.md
etape "commit" git commit -q -m "dépendances : mise à jour automatique du $DATE

Toutes les barrières sont passées avant commit : gofmt, go vet, go test -race, contrôles JavaScript,
construction de l'image et govulncheck.

$(printf '%s' "$DIFF_MOD" | sed 's/^/    /')" || arreter "commit impossible"

APRES=$(git rev-parse --short HEAD)
if [ -n "${SANS_PUSH:-}" ]; then
	dire "poussée sautée (SANS_PUSH=1) : commit $APRES prêt en local"
else
	for r in $REMOTES; do
		if git remote get-url "$r" >/dev/null 2>&1; then
			etape "poussée vers $r" git push "$r" "$BRANCHE" || arreter "poussée vers $r impossible (commit $APRES conservé en local)"
		else
			dire "dépôt distant « $r » inconnu : ignoré"
		fi
	done
fi

dire "=== terminé : $AVANT → $APRES"
[ "$NOTIFIER" = "failures" ] || envoyer "[wmail-go] dépendances mises à jour ($APRES)"
exit 0
