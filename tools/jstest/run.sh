#!/bin/sh
# Contrôles JavaScript (jsdom, Node 22 dans un conteneur) sur les pages réellement produites par le serveur.
# Prérequis : conteneurs construits (make up). Remet GreenMail à zéro (messages non lus) avant de commencer.
set -eu
cd "$(dirname "$0")/../.."
COMPOSE="${COMPOSE:-docker compose}"
RUN="${CONTAINER_ENGINE:-docker}"
W="$(mktemp -d)"
trap 'rm -rf "$W"' EXIT

$COMPOSE up -d --build webmail >/dev/null
$COMPOSE --profile test up -d greenmail >/dev/null
$COMPOSE --profile test restart greenmail >/dev/null
sleep 6
$COMPOSE --profile dev run --rm dev go run ./tools/seedmail -addr greenmail:3025 -to test@localhost -n 40 >/dev/null
$COMPOSE --profile dev run --rm dev go run ./tools/seedmail -addr greenmail:3025 -to second@localhost -n 6 >/dev/null

python3 -I tools/jstest/gen_fixtures.py "$W"
cp internal/web/static/list.js internal/web/static/chips.js internal/web/static/preview.js internal/web/static/msgfull.js internal/web/static/listrefresh.js internal/web/static/keys.js internal/web/static/folders.js internal/web/static/listnav.js internal/web/static/dnd.js tools/jstest/test_*.js "$W"/
$RUN run --rm -v "$W":/w:Z -w /w docker.io/library/node:22-alpine sh -c \
  'npm install --silent --no-audit --no-fund jsdom@26 >/dev/null 2>&1 && node test_select.js && node test_panes.js && node test_keys.js && node test_folders.js && node test_nav.js && node test_chips.js && node test_refresh.js'
cp internal/web/static/settings-auto.js internal/web/templates/settings.html "$W"/ && $RUN run --rm -v "$W":/w:Z -w /w docker.io/library/node:22-alpine node test_settings_auto.js
