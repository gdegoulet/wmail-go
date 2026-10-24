#!/usr/bin/env python3
"""Engendre l'image de fond vectorielle du thème « rainloop » (bokeh : halos flous sur un dégradé chaud→froid).

Usage : python3 tools/theme/gen_background.py internal/web/static/themes/rainloop/background.svg

Le tirage est déterministe (graine fixe) : relancer le script redonne exactement la même image.
"""

import random
import sys

W, H = 1600, 900
SEED = 20261024

# Teintes de gauche (chaud) à droite (froid), comme la photo d'origine.
STOPS = [
    (0.00, (255, 186, 70)),
    (0.18, (255, 136, 48)),
    (0.38, (242, 60, 92)),
    (0.60, (214, 48, 140)),
    (0.80, (142, 56, 158)),
    (1.00, (60, 42, 120)),
]


def hue(t):
    """Couleur de la palette à la position t (0 = gauche, 1 = droite)."""
    t = min(max(t, 0.0), 1.0)
    for (t0, c0), (t1, c1) in zip(STOPS, STOPS[1:]):
        if t <= t1:
            k = 0 if t1 == t0 else (t - t0) / (t1 - t0)
            return tuple(round(a + (b - a) * k) for a, b in zip(c0, c1))
    return STOPS[-1][1]


def mix(c, other, k):
    return tuple(round(a + (b - a) * k) for a, b in zip(c, other))


def rgb(c):
    return "#%02x%02x%02x" % c


def main(path):
    rnd = random.Random(SEED)
    out = []
    add = out.append
    add(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" '
        f'width="{W}" height="{H}" preserveAspectRatio="xMidYMid slice">')
    add("<defs>")
    # Dégradé de fond : chaud en haut à gauche, froid en bas à droite.
    add('<linearGradient id="g" x1="0" y1="0" x2="1" y2="1">')
    for t, c in STOPS:
        add(f'<stop offset="{t:.2f}" stop-color="{rgb(mix(c, (18, 8, 24), 0.74))}"/>')
    add("</linearGradient>")
    # Deux lueurs : le foyer orange en haut à gauche, un halo magenta au centre.
    add('<radialGradient id="glow1" cx="0.3" cy="0.12" r="0.42">'
        '<stop offset="0" stop-color="#ffc766" stop-opacity=".8"/>'
        '<stop offset="1" stop-color="#ffc766" stop-opacity="0"/></radialGradient>')
    add('<radialGradient id="glow2" cx="0.52" cy="0.62" r="0.5">'
        '<stop offset="0" stop-color="#ff2f7b" stop-opacity=".45"/>'
        '<stop offset="1" stop-color="#ff2f7b" stop-opacity="0"/></radialGradient>')
    add('<radialGradient id="glow3" cx="0.9" cy="0.85" r="0.5">'
        '<stop offset="0" stop-color="#2a2f8f" stop-opacity=".5"/>'
        '<stop offset="1" stop-color="#2a2f8f" stop-opacity="0"/></radialGradient>')
    # Un halo = un disque flouté ; trois forces de flou suffisent à donner la profondeur.
    for i, dev in enumerate((3, 9, 22)):
        add(f'<filter id="b{i}" x="-60%" y="-60%" width="220%" height="220%">'
            f'<feGaussianBlur stdDeviation="{dev}"/></filter>')
    add("</defs>")
    add(f'<rect width="{W}" height="{H}" fill="url(#g)"/>')
    add(f'<rect width="{W}" height="{H}" fill="url(#glow1)"/>')
    add(f'<rect width="{W}" height="{H}" fill="url(#glow2)"/>')
    add(f'<rect width="{W}" height="{H}" fill="url(#glow3)"/>')

    # Halos : les gros flous d'abord (arrière-plan), les petits nets ensuite.
    halos = []
    for _ in range(110):
        x = rnd.uniform(-40, W + 40)
        y = rnd.uniform(-20, H + 20)
        r = rnd.choice([rnd.uniform(14, 34), rnd.uniform(30, 64), rnd.uniform(55, 110)])
        halos.append((x, y, r))
    halos.sort(key=lambda h: -h[2])
    for x, y, r in halos:
        t = x / W
        # Les halos du haut restent plus clairs, ceux du bas se fondent dans le fond.
        c = mix(hue(t), (255, 255, 255), max(0.05, 0.38 - y / H * 0.3))
        blur = 2 if r > 70 else (1 if r > 34 else 0)
        op = round(rnd.uniform(0.3, 0.8) * (1.15 - y / H * 0.45), 3)
        if t < 0.22:  # sous le panneau des dossiers : halos atténués, le texte blanc doit rester lisible
            op = round(op * (0.35 + t * 2.0), 3)
        if rnd.random() < 0.22:  # anneau : le bord du cercle de confusion
            add(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{r:.0f}" fill="none" stroke="{rgb(c)}" '
                f'stroke-width="{max(2, r * 0.09):.1f}" opacity="{op:.2f}" filter="url(#b{blur})"/>')
        else:
            add(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{r:.0f}" fill="{rgb(c)}" '
                f'opacity="{op:.2f}" filter="url(#b{blur})"/>')

    # Étincelles : petits points nets, un peu de blanc chaud.
    for _ in range(220):
        x = rnd.uniform(0, W)
        y = rnd.uniform(0, H)
        r = rnd.uniform(0.8, 2.6)
        c = mix(hue(x / W), (255, 250, 230), 0.65)
        add(f'<circle cx="{x:.0f}" cy="{y:.0f}" r="{r:.1f}" fill="{rgb(c)}" '
            f'opacity="{rnd.uniform(0.25, 0.85):.2f}"/>')

    # Assombrissement du bord gauche : le panneau des dossiers (texte blanc) se pose dessus.
    add('<linearGradient id="left" x1="0" y1="0" x2="1" y2="0">'
        '<stop offset="0" stop-color="#0d0616" stop-opacity=".72"/>'
        '<stop offset="0.14" stop-color="#0d0616" stop-opacity=".5"/>'
        '<stop offset="0.3" stop-color="#0d0616" stop-opacity="0"/></linearGradient>')
    add(f'<rect width="{W}" height="{H}" fill="url(#left)"/>')

    # Assombrissement du bas : le texte des panneaux reste lisible.
    add('<linearGradient id="vign" x1="0" y1="0" x2="0" y2="1">'
        '<stop offset="0.45" stop-color="#000" stop-opacity="0"/>'
        '<stop offset="1" stop-color="#000" stop-opacity=".45"/></linearGradient>')
    add(f'<rect width="{W}" height="{H}" fill="url(#vign)"/>')
    add("</svg>")
    svg = "\n".join(out) + "\n"
    with open(path, "w", encoding="utf-8") as f:
        f.write(svg)
    print(f"{path} : {len(svg)} octets")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "internal/web/static/themes/rainloop/background.svg")
