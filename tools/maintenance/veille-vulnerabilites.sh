#!/bin/sh
# Veille quotidienne : govulncheck sur le dépôt tel qu'il est, sans rien modifier.
# Une faille peut être publiée des semaines après la dernière mise à jour des dépendances :
# ce contrôle-là doit tourner même les jours où rien ne bouge.
#
# Courriel envoyé selon NOTIFIER : failures (défaut), always, never.
# Mêmes réglages que maj-dependances.sh (DEPOT, DESTINATAIRE, EXPEDITEUR, MAILER, NOTIFIER, JOURNAL).

set -eu

ICI=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
DEPOT=${DEPOT:-$(CDPATH= cd -- "$ICI/../.." && pwd)}
if [ -n "${WMAIL_MAJ_CONF:-}" ] && [ -f "$WMAIL_MAJ_CONF" ]; then
	# Lu par le shell : une valeur à espaces non protégée par des guillemets serait prise pour une commande.
	sh -n "$WMAIL_MAJ_CONF" || { echo "réglages invalides : $WMAIL_MAJ_CONF (guillemets manquants ?)" >&2; exit 2; }
	. "$WMAIL_MAJ_CONF"
fi
# Projet compose distinct : l'automate ne doit pas toucher aux conteneurs ni aux volumes du webmail en service.
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-wmail-go-ci}
DESTINATAIRE=${DESTINATAIRE:-wmail-go@degoulet.net}
EXPEDITEUR=${EXPEDITEUR:-wmail-go@$(hostname -f 2>/dev/null || hostname)}
MAILER=${MAILER:-"sendmail -t"}
NOTIFIER=${NOTIFIER:-failures}
JOURNAL=${JOURNAL:-"$DEPOT/.maj-dependances.log"}

cd "$DEPOT"
RAPPORT=$(mktemp)
trap 'rm -f "$RAPPORT"' EXIT

{
	printf '%s veille govulncheck (%s)\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$(git rev-parse --short HEAD)"
} | tee -a "$JOURNAL" >"$RAPPORT"

if make vuln >>"$RAPPORT" 2>&1; then
	echo "aucune vulnérabilité atteignable" >>"$RAPPORT"
	etat=0
	sujet="[wmail-go] veille : aucune vulnérabilité atteignable"
else
	etat=1
	sujet="[wmail-go] VEILLE : vulnérabilité atteignable détectée"
fi
cat "$RAPPORT" >>"$JOURNAL"

envoyer=0
case "$NOTIFIER" in
	always) envoyer=1 ;;
	failures) [ "$etat" -eq 1 ] && envoyer=1 ;;
	never) envoyer=0 ;;
esac

if [ "$envoyer" -eq 1 ] && command -v "${MAILER%% *}" >/dev/null 2>&1; then
	{
		printf 'From: %s\nTo: %s\nSubject: %s\nContent-Type: text/plain; charset=utf-8\n\n' \
			"$EXPEDITEUR" "$DESTINATAIRE" "$sujet"
		cat "$RAPPORT"
	} | $MAILER || echo "envoi du courriel impossible" >>"$JOURNAL"
fi

exit "$etat"
