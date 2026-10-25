#!/bin/sh
# Envoi d'un courriel par SMTP avec curl, pour les machines sans sendmail ni msmtp.
# Le message complet (en-têtes puis corps) est lu sur l'entrée standard, comme « sendmail -t ».
#
# À utiliser comme programme d'envoi :
#   MAILER=/data/Work/wmail-go/tools/maintenance/envoi-smtp.sh
#
# Réglages (dans le même fichier que les autres, par exemple .maj.env) :
#   SMTP_URL     smtp://serveur:587 ou smtps://serveur:465        (obligatoire)
#   SMTP_USER    identifiant                                      (facultatif)
#   SMTP_PASS    mot de passe                                     (facultatif)
#   SMTP_FROM    adresse d'enveloppe                              (défaut : en-tête From:)
#   SMTP_TO      destinataires séparés par des espaces            (défaut : en-tête To:)
#   SMTP_STARTTLS 1 pour exiger STARTTLS sur un port 587          (défaut : 1 si smtp://)
#
# Le mot de passe passe par un fichier de configuration curl temporaire, jamais par la ligne de
# commande : il n'apparaît donc pas dans la liste des processus.

set -eu

[ -n "${WMAIL_MAJ_CONF:-}" ] && [ -f "$WMAIL_MAJ_CONF" ] && . "$WMAIL_MAJ_CONF"
: "${SMTP_URL:?SMTP_URL manquant (exemple : smtp://mail.example.fr:587)}"

MESSAGE=$(mktemp)
CONF=$(mktemp)
trap 'rm -f "$MESSAGE" "$CONF"' EXIT
chmod 600 "$MESSAGE" "$CONF"
cat > "$MESSAGE"

entete() { sed -n "s/^$1: *//Ip" "$MESSAGE" | head -1 | tr -d '\r'; }
FROM=${SMTP_FROM:-$(entete From)}
TO=${SMTP_TO:-$(entete To)}
[ -n "$FROM" ] || { echo "envoi-smtp : expéditeur inconnu" >&2; exit 1; }
[ -n "$TO" ] || { echo "envoi-smtp : destinataire inconnu" >&2; exit 1; }

{
	printf 'url = "%s"\n' "$SMTP_URL"
	printf 'mail-from = "%s"\n' "$FROM"
	for d in $TO; do printf 'mail-rcpt = "%s"\n' "$(printf '%s' "$d" | tr -d ',')"; done
	printf 'upload-file = "%s"\n' "$MESSAGE"
	printf 'silent\nshow-error\n'
	case "$SMTP_URL" in
		smtp://*) [ "${SMTP_STARTTLS:-1}" = "1" ] && printf 'ssl-reqd\n' ;;
	esac
	[ -n "${SMTP_USER:-}" ] && printf 'user = "%s:%s"\n' "$SMTP_USER" "${SMTP_PASS:-}"
} > "$CONF"

curl --config "$CONF"
