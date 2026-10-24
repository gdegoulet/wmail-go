"""Génère les pages réelles du serveur en marche (liste 3 volets, fragment du volet de lecture)
pour les contrôles JavaScript. Usage : python3 -I gen_fixtures.py <dossier de sortie>"""
import html, http.cookiejar, random, re, sys, urllib.error, urllib.parse, urllib.request

out = sys.argv[1]
B, PW = "http://127.0.0.1:8080", "un mot de passe assez long"
jar = http.cookiejar.CookieJar()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *a, **k):
        return None


opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar), NoRedirect)


def req(path, data=None, headers=None):
    body = urllib.parse.urlencode(data, doseq=True).encode() if data is not None else None
    try:
        r = opener.open(urllib.request.Request(B + path, body, headers or {}), timeout=30)
        return r.status, r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "replace")


user = "js%d" % random.randint(1000, 9999)
req("/register", {"login": user, "password": PW, "password2": PW})
_, page = req("/recovery-key")
token = re.search(r'name="csrf" value="([^"]+)"', page).group(1)
req("/recovery-key", {"csrf": token})
req("/accounts/save", {"csrf": token, "action": "save", "email": "test@localhost", "imap_host": "greenmail", "imap_port": "3143",
                       "imap_security": "none", "imap_login": "test", "imap_password": "testpass", "smtp_host": "greenmail",
                       "smtp_port": "3025", "smtp_security": "none", "block_remote": "on"})
# arbre de dossiers pour les contrôles du panneau (séparateur de GreenMail : « . »)
req("/settings/folders/action", {"csrf": token, "action": "create", "new_name": "Projets", "delim": "."})
req("/settings/folders/action", {"csrf": token, "action": "create", "new_name": "Annee", "parent": "Projets", "delim": "."})
req("/settings/folders/action", {"csrf": token, "action": "create", "new_name": "Mois", "parent": "Projets.Annee", "delim": "."})
_, home = req("/")
open(out + "/home3.html", "w", encoding="utf-8").write(home)
rows = re.findall(r'<a class="rowlink" href="([^"]+)"[^>]*>[\s\S]*?<span class="subj">([^<]*)</span>', home)
link = [html.unescape(l) for l, t in rows if t.startswith("Offre")][0]  # message HTML non lu : liens du volet, bannière
_, frag = req("/mail/pane" + link[1:], headers={"X-Requested-With": "fetch"})
open(out + "/frag.html", "w", encoding="utf-8").write(frag)
_, opened = req(link)  # volet de lecture ouvert (liste + message) : raccourcis de réponse
open(out + "/open.html", "w", encoding="utf-8").write(opened)
print("pages générées : liste %d octets, fragment %d octets" % (len(home), len(frag)))
