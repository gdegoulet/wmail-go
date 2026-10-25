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

# adresses « nues » : « Nom <a@b> , "c" <c@d> » devient « a@b c@d ».
# Un serveur refuse l'adresse avec un 504 si elle garde ses guillemets, ses chevrons ou son nom d'affichage.
adresses() {
	printf '%s\n' "$1" | tr ',;' '\n\n' | while IFS= read -r part; do   # \n final : sinon « read » perd la dernière adresse
		case "$part" in
			*"<"*">"*) printf '%s\n' "$part" | sed -n 's/.*<\([^>]*\)>.*/\1/p' ;;
			*) printf '%s\n' "$part" | tr -d '"'"'"'<> \t' ;;
		esac
	done | grep '@' || true
}

FROM=$(adresses "${SMTP_FROM:-$(entete From)}" | head -1)
TO=$(adresses "${SMTP_TO:-$(entete To)}" | tr '\n' ' ')
[ -n "$FROM" ] || { echo "envoi-smtp : expéditeur inconnu" >&2; exit 1; }
[ -n "$TO" ] || { echo "envoi-smtp : destinataire inconnu" >&2; exit 1; }

{
	printf 'url = "%s"\n' "$SMTP_URL"
	printf 'mail-from = "%s"\n' "$FROM"
	for d in $TO; do printf 'mail-rcpt = "%s"\n' "$d"; done
	printf 'upload-file = "%s"\n' "$MESSAGE"
	printf 'silent\nshow-error\n'
	case "$SMTP_URL" in
		smtp://*) [ "${SMTP_STARTTLS:-1}" = "1" ] && printf 'ssl-reqd\n' ;;
	esac
	[ -n "${SMTP_USER:-}" ] && printf 'user = "%s:%s"\n' "$SMTP_USER" "${SMTP_PASS:-}"
} > "$CONF"

if ! curl --config "$CONF"; then
	echo "envoi-smtp : échec pour « $TO » depuis « $FROM » via $SMTP_URL" >&2
	exit 1
fi
