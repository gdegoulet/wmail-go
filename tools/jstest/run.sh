#!/bin/sh
# Contrôles JavaScript (jsdom, Node 22 dans un conteneur) sur les pages réellement produites par le serveur.
# Prérequis : conteneurs construits (make up). Remet GreenMail à zéro (messages non lus) avant de commencer.
set -eu
cd "$(dirname "$0")/../.."
COMPOSE="${COMPOSE:-docker compose}"
RUN="${CONTAINER_ENGINE:-docker}"
W="$(mktemp -d)"
# « npm install » tourne en root dans le conteneur : avec un moteur non rootless (intégration continue),
# node_modules appartient à root et l'hôte ne peut pas l'effacer. On le supprime donc depuis un conteneur,
# puis on rend la main avec le code de sortie du script, jamais celui du ménage.
nettoyer() {
	code=$?
	if [ -d "$W/node_modules" ]; then
		$RUN run --rm -v "$W":/w:Z -w /w docker.io/library/node:22-alpine rm -rf /w/node_modules >/dev/null 2>&1 || true
	fi
	rm -rf "$W" 2>/dev/null || true
	exit "$code"
}
trap nettoyer EXIT

$COMPOSE up -d --build webmail >/dev/null
$COMPOSE --profile test up -d greenmail >/dev/null
$COMPOSE --profile test restart greenmail >/dev/null

# Attendre que les deux services répondent : sur une machine froide (intégration continue), l'image de
# GreenMail se télécharge et la JVM démarre bien au-delà des quelques secondes d'une machine de travail.
attendre() {
	quoi=$1; hote=$2; port=$3; essais=${4:-60}
	i=0
	while [ "$i" -lt "$essais" ]; do
		if python3 -I -c "import socket,sys; s=socket.create_connection(('$hote',$port),2); s.close()" 2>/dev/null; then
			return 0
		fi
		i=$((i + 1))
		sleep 2
	done
	echo "$quoi ne répond pas sur $hote:$port après $((essais * 2)) s" >&2
	$COMPOSE logs --tail=40 >&2 || true
	return 1
}
attendre "le webmail" 127.0.0.1 8080
attendre "GreenMail" 127.0.0.1 3025
$COMPOSE --profile dev run --rm dev go run ./tools/seedmail -addr greenmail:3025 -to test@localhost -n 40 >/dev/null
$COMPOSE --profile dev run --rm dev go run ./tools/seedmail -addr greenmail:3025 -to second@localhost -n 6 >/dev/null

python3 -I tools/jstest/gen_fixtures.py "$W"
cp internal/web/static/list.js internal/web/static/chips.js internal/web/static/preview.js internal/web/static/msgfull.js internal/web/static/listrefresh.js internal/web/static/keys.js internal/web/static/folders.js internal/web/static/listnav.js internal/web/static/dnd.js tools/jstest/test_*.js "$W"/
$RUN run --rm -v "$W":/w:Z -w /w docker.io/library/node:22-alpine sh -c \
  'npm install --silent --no-audit --no-fund jsdom@26 >/dev/null 2>&1 && node test_select.js && node test_panes.js && node test_pane_reveal.js && node test_keys.js && node test_folders.js && node test_nav.js && node test_chips.js && node test_refresh.js'
cp internal/web/static/settings-auto.js internal/web/templates/settings.html "$W"/ && $RUN run --rm -v "$W":/w:Z -w /w docker.io/library/node:22-alpine node test_settings_auto.js
