#!/usr/bin/env python3
"""Choisit les mises à jour de dépendances assez anciennes pour être adoptées.

Lit la sortie de `go list -m -u -json all` sur l'entrée standard et écrit sur la sortie standard
la liste des `module@version` à installer, un par ligne.

Une version publiée depuis moins de N jours est mise en quarantaine : c'est la fenêtre pendant laquelle
un paquet compromis est habituellement repéré puis retiré. Les versions écartées sont décrites sur la
sortie d'erreur, pour figurer dans le récapitulatif.

Usage :
    go list -m -u -json all | quarantaine.py [jours] [--directes-seulement]

Sans date de publication connue (modules remplacés, dépôt privé), la mise à jour est écartée : on ne
peut pas vérifier son âge, donc on ne la prend pas automatiquement.
"""

import datetime as dt
import json
import sys


def objets(flux):
    """Découpe la suite d'objets JSON que « go list -json » écrit sans séparateur.

    La sortie peut contenir du bruit : podman-compose écrit un avertissement avant, et le code de
    retour du conteneur après. On repart donc de la prochaine accolade ouvrante à chaque problème,
    et on ignore tout ce qui n'est pas un objet.
    """
    decodeur = json.JSONDecoder()
    texte = flux.read()
    i = 0
    while True:
        debut = texte.find("{", i)
        if debut < 0:
            return
        try:
            obj, i = decodeur.raw_decode(texte, debut)
        except ValueError:
            i = debut + 1   # accolade isolée (texte libre) : on avance d'un caractère
            continue
        if isinstance(obj, dict):
            yield obj


def date(valeur):
    if not valeur:
        return None
    try:
        return dt.datetime.fromisoformat(valeur.replace("Z", "+00:00"))
    except ValueError:
        return None


def main():
    jours = 5
    directes_seulement = False
    for arg in sys.argv[1:]:
        if arg == "--directes-seulement":
            directes_seulement = True
        else:
            jours = int(arg)
    limite = dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=jours)

    retenues, quarantaine, sans_date = [], [], []
    for m in objets(sys.stdin):
        maj = m.get("Update")
        if not maj or m.get("Main"):
            continue
        if directes_seulement and m.get("Indirect"):
            continue
        publiee = date(maj.get("Time"))
        cible = f"{m['Path']}@{maj['Version']}"
        if publiee is None:
            sans_date.append(f"{cible} (date de publication inconnue)")
        elif publiee > limite:
            age = (dt.datetime.now(dt.timezone.utc) - publiee).days
            quarantaine.append(f"{cible} publiée il y a {age} j (< {jours} j)")
        else:
            retenues.append(cible)

    for ligne in retenues:
        print(ligne)
    if quarantaine:
        print(f"en quarantaine ({len(quarantaine)}) :", file=sys.stderr)
        for ligne in quarantaine:
            print("  " + ligne, file=sys.stderr)
    if sans_date:
        print(f"écartées faute de date ({len(sans_date)}) :", file=sys.stderr)
        for ligne in sans_date:
            print("  " + ligne, file=sys.stderr)
    if not retenues:
        print("aucune mise à jour éligible", file=sys.stderr)


if __name__ == "__main__":
    main()
