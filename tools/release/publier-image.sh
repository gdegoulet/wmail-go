#!/bin/sh
# Construit l'image de wmail-go à partir des sources du dépôt et la publie sur un registre
# (par défaut docker.io/gdegoulet/wmail-go).
#
#   ./tools/release/publier-image.sh              construit, contrôle, demande confirmation, publie
#   ./tools/release/publier-image.sh --essai      construit et contrôle, ne publie rien
#   ./tools/release/publier-image.sh --oui        publie sans demander confirmation (cron)
#
# Réglages : variables d'environnement, ou fichier passé par --env (voir publier.env.exemple).
#   IMAGE            nom complet de l'image            (docker.io/gdegoulet/wmail-go)
#   REGISTRE_USER    identifiant du registre           (obligatoire pour publier)
#   REGISTRE_TOKEN   jeton d'accès du registre         (jamais affiché ; sinon « docker login » préalable)
#   VERSION          étiquette de version              (par défaut AAAAMMJJ-commit, ex. 20261025-556be83)
#   ETIQUETTES_SUP   étiquettes supplémentaires        (« stable edge »)
#   PLATEFORMES      construction multi-architecture   (« linux/amd64,linux/arm64 », exige docker buildx)
#   SANS_TESTS=1     saute les tests Go avant publication
#   SANS_LATEST=1    ne publie pas l'étiquette « latest »
#   TIRER=1          « git pull --ff-only » avant la construction
#   ARBRE_SALE=1     autorise la publication avec des modifications non versionnées
set -eu

RACINE=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$RACINE"

ESSAI=0
CONFIRME=0
for arg in "$@"; do
	case "$arg" in
		--essai|-n) ESSAI=1 ;;
		--oui|-y) CONFIRME=1 ;;
		--env=*) FICHIER_ENV=${arg#--env=} ;;
		-h|--help) sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
		*) echo "option inconnue : $arg" >&2; exit 2 ;;
	esac
done

if [ -n "${FICHIER_ENV:-}" ]; then
	[ -r "$FICHIER_ENV" ] || { echo "fichier de réglages illisible : $FICHIER_ENV" >&2; exit 2; }
	sh -n "$FICHIER_ENV" || { echo "fichier de réglages mal formé : $FICHIER_ENV" >&2; exit 2; }
	# shellcheck disable=SC1090
	. "$FICHIER_ENV"
fi

IMAGE=${IMAGE:-docker.io/gdegoulet/wmail-go}
MOTEUR=${MOTEUR:-docker}
COMPOSE=${COMPOSE:-docker compose}

command -v "$MOTEUR" >/dev/null 2>&1 || { echo "moteur de conteneurs introuvable : $MOTEUR" >&2; exit 1; }

# --- sources ---------------------------------------------------------------
if [ "${TIRER:-0}" = "1" ]; then
	echo "== mise à jour des sources"
	git pull --ff-only
fi

REVISION=$(git rev-parse --short HEAD 2>/dev/null || echo inconnue)
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
	if [ "${ARBRE_SALE:-0}" != "1" ]; then
		echo "arbre de travail modifié : versionnez ou mettez de côté vos changements, ou posez ARBRE_SALE=1." >&2
		git status --short >&2
		exit 1
	fi
	echo "!! arbre de travail modifié : l'image ne correspondra à aucun commit publié"
	REVISION="$REVISION-sale"
fi

# Étiquette : date du jour (UTC) et numéro court du commit, par exemple 20261025-556be83.
VERSION=${VERSION:-"$(date -u +%Y%m%d)-$REVISION"}
DATE_BUILD=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "== image    : $IMAGE"
echo "== version  : $VERSION"
echo "== révision : $REVISION"

# --- contrôles -------------------------------------------------------------
if [ "${SANS_TESTS:-0}" != "1" ]; then
	echo "== mise en forme, analyse statique et tests (conteneur dev)"
	$COMPOSE --profile dev run --rm -T dev sh -c '
		reste=$(gofmt -l ./cmd ./internal)
		if [ -n "$reste" ]; then echo "fichiers non formatés :"; echo "$reste"; exit 1; fi
		go vet ./... && go test ./...' || { echo "contrôles en échec : rien n'est publié." >&2; exit 1; }
fi

# --- construction ----------------------------------------------------------
ARGS="--target runtime --build-arg VERSION=$VERSION --build-arg REVISION=$REVISION --build-arg BUILD_DATE=$DATE_BUILD"
# Podman produit du format OCI par défaut, qui perd le HEALTHCHECK : le registre doit recevoir du format Docker.
if $MOTEUR --version 2>&1 | grep -qi podman; then
	ARGS="--format docker $ARGS"
fi
TAGS="-t $IMAGE:$VERSION"
[ "${SANS_LATEST:-0}" = "1" ] || TAGS="$TAGS -t $IMAGE:latest"
for e in ${ETIQUETTES_SUP:-}; do TAGS="$TAGS -t $IMAGE:$e"; done

# --- connexion au registre -------------------------------------------------
# Faite avant la construction multi-architecture, qui publie dans la foulée.
connexion() {
	if [ -n "${REGISTRE_TOKEN:-}" ]; then
		[ -n "${REGISTRE_USER:-}" ] || { echo "REGISTRE_USER manquant." >&2; exit 2; }
		echo "== connexion à ${IMAGE%%/*} en tant que $REGISTRE_USER"
		printf '%s' "$REGISTRE_TOKEN" | $MOTEUR login "${IMAGE%%/*}" -u "$REGISTRE_USER" --password-stdin >/dev/null
	else
		echo "== pas de REGISTRE_TOKEN : on compte sur une connexion déjà établie (docker login)"
	fi
}

if [ -n "${PLATEFORMES:-}" ]; then
	# Multi-architecture : buildx construit et pousse en une fois, l'image n'existe pas en local.
	$MOTEUR buildx version >/dev/null 2>&1 || { echo "PLATEFORMES exige « docker buildx » (absent avec Podman)." >&2; exit 1; }
	if [ "$ESSAI" = "1" ]; then
		echo "== essai : construction multi-architecture sans publication"
		# shellcheck disable=SC2086
		$MOTEUR buildx build --platform "$PLATEFORMES" $ARGS $TAGS .
		echo "== essai terminé, rien n'a été publié."
		exit 0
	fi
	if [ "$CONFIRME" != "1" ]; then
		printf 'Publier %s:%s (et latest) pour %s ? [o/N] ' "$IMAGE" "$VERSION" "$PLATEFORMES"
		read -r rep
		case "$rep" in o|O|y|Y) ;; *) echo "abandon."; exit 0 ;; esac
	fi
	connexion
	# shellcheck disable=SC2086
	$MOTEUR buildx build --platform "$PLATEFORMES" --push $ARGS $TAGS .
	echo "== publié : $IMAGE:$VERSION ($PLATEFORMES)"
	exit 0
fi

echo "== construction de l'image"
# shellcheck disable=SC2086
$MOTEUR build $ARGS $TAGS .

echo "== contrôle de l'image construite"
SORTIE=$($MOTEUR run --rm "$IMAGE:$VERSION" version)
[ "$SORTIE" = "$VERSION" ] || { echo "la version du binaire ($SORTIE) ne correspond pas à l'étiquette ($VERSION)." >&2; exit 1; }
$MOTEUR run --rm "$IMAGE:$VERSION" --help >/dev/null 2>&1 || true
echo "   version du binaire : $SORTIE"
echo "   taille : $($MOTEUR image inspect "$IMAGE:$VERSION" --format '{{.Size}}' | awk '{printf "%.1f Mio", $1/1048576}')"

if [ "$ESSAI" = "1" ]; then
	echo "== essai terminé : image construite en local, rien n'a été publié."
	exit 0
fi

if [ "$CONFIRME" != "1" ]; then
	printf 'Publier %s:%s%s ? [o/N] ' "$IMAGE" "$VERSION" "$([ "${SANS_LATEST:-0}" = "1" ] || echo ' et :latest')"
	read -r rep
	case "$rep" in o|O|y|Y) ;; *) echo "abandon : rien n'a été publié."; exit 0 ;; esac
fi

connexion
echo "== publication"
$MOTEUR push "$IMAGE:$VERSION"
[ "${SANS_LATEST:-0}" = "1" ] || $MOTEUR push "$IMAGE:latest"
for e in ${ETIQUETTES_SUP:-}; do $MOTEUR push "$IMAGE:$e"; done

echo "== publié"
echo "   $IMAGE:$VERSION"
[ "${SANS_LATEST:-0}" = "1" ] || echo "   $IMAGE:latest"
echo "   docker run --rm -p 8080:8080 -v wmail-go-data:/data -v \$PWD/config/config.json:/config/config.json:ro $IMAGE:$VERSION"
