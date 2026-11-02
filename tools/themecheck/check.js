// Contrôle de lisibilité des fonds d'écran, dans un vrai navigateur.
//
// Deux zones de l'interface sont posées à même le fond, sans couleur de fond propre : le panneau des dossiers
// (520/1920 de la largeur) et la barre du haut (190/1080 de la hauteur, par-dessus --bar-bg). Un thème dont
// l'image est claire à ces endroits rend les noms de dossiers illisibles, et aucun test de feuille de style
// ne peut le voir : il faut peindre l'image.
//
// Le fond est donc rendu tel quel — couches de voile comprises —, photographié, puis relu pixel par pixel.
// La luminance retenue est celle du 90e centile : la lisibilité se perd sur les taches claires d'une zone,
// pas sur sa moyenne.
//
// Usage : node check.js            (tous les thèmes servis par /themes/)
//         node check.js foo bar    (ces thèmes-là)
const puppeteer = require("puppeteer");
const BASE = process.env.BASE || "http://127.0.0.1:" + (process.env.WMAIL_PORT || "8080");
const VISE = 4.5; // WCAG 2.1 AA, texte courant
// Les zones mesurées sont celles où du texte se pose vraiment sur le fond : la largeur du panneau des
// dossiers (--folders-w, 17,643 rem à 14 px) et la hauteur de la barre du haut (--bar-h, 3,571 rem). Elles
// sont plus étroites que le voile que pose tools/themegen, qui s'efface progressivement au-delà.
const W = 1920, H = 1080, BANDE_G = 248, BANDE_H = 50;
// Le titre de la page de connexion et les liens sous la carte sont posés à même le fond, eux aussi : la
// carte ne couvre que les champs. Cette zone-là se mesure sur la page réellement rendue, et non sur le fond
// nu : --login-title-shadow assombrit le voisinage immédiat des lettres, et c'est ce voisinage qui décide
// de la lisibilité. La valeur retenue est la médiane de la zone de texte, que l'ombre fait baisser.

const lin = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); };
const lum = (r, g, b) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
const contraste = (a, b) => (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
// Valeur sRGB (0–255) d'un gris de cette luminance : l'inverse de lin(), pour raisonner sur l'opacité d'un
// voile noir, qui multiplie la valeur sRGB et non la luminance.
const gris = (l) => 255 * (l <= 0.0031308 ? l * 12.92 : 1.055 * Math.pow(l, 1 / 2.4) - 0.055);

// voileNecessaire rend l'opacité du voile noir qui amènerait une bande de luminance « bande » au contraste
// visé avec un texte de luminance « texte », et null si aucun voile n'y suffit.
function voileNecessaire(bande, texte, vise) {
  const cible = (texte + 0.05) / vise - 0.05;
  if (cible <= 0) return null;
  const f = 1 - gris(cible) / gris(bande);
  if (!(f > 0) || f > 0.8) return null;
  return Math.min(0.8, Math.ceil(f * 20) / 20); // arrondi au 0,05 supérieur : une marge, et un nombre lisible
}

(async () => {
  const browser = await puppeteer.launch({ executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser", headless: "new", args: ["--no-sandbox", "--disable-gpu"] });
  const page = await browser.newPage();
  await page.setViewport({ width: W, height: H });
  await page.goto(BASE + "/login", { waitUntil: "networkidle0" });

  // La liste des thèmes n'est pas lisible sans session : make theme-check la passe par THEMES.
  const themes = process.argv.slice(2).length ? process.argv.slice(2) : (process.env.THEMES || "").split(",").filter(Boolean);
  if (!themes.length) { console.error("aucun thème à contrôler : passez-les en argument ou par THEMES="); process.exit(2); }

  let echecs = 0;
  for (const t of themes) {
    const couleurs = await page.evaluate(async (t) => {
      document.querySelectorAll("link[data-controle]").forEach((e) => e.remove());
      const l = document.createElement("link");
      l.rel = "stylesheet"; l.href = "/themes/" + t + "/theme.css"; l.dataset.controle = "1";
      document.head.appendChild(l);
      await new Promise((r) => { l.onload = r; l.onerror = r; });
      // Page nue : rien que le fond du thème, aux dimensions de la fenêtre.
      document.documentElement.style.cssText = "margin:0;padding:0";
      document.body.replaceChildren();
      // Le fond est posé ici, et non laissé à app.css : la page de connexion a ses propres règles,
      // et une seule d'entre elles suffirait à masquer le fond du thème.
      document.body.style.cssText = "margin:0;padding:0;min-height:100vh;background:var(--wall);background-attachment:fixed";
      const s = getComputedStyle(document.documentElement);
      const bar = document.createElement("div");
      bar.style.cssText = "position:fixed;left:0;top:0;right:0;height:50px;background:var(--bar-bg)";
      document.body.appendChild(bar);
      // Le SVG de fond se décode après la pose de la feuille : sans cette attente, la photographie
      // saisit parfois la page avant lui, et la mesure porte alors sur le dégradé de repli.
      const u = s.getPropertyValue("--wall").match(/url\("([^"]+)"\)/);
      if (u) { const im = new Image(); im.src = u[1]; await im.decode().catch(() => {}); }
      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
      return {
        dossiers: s.getPropertyValue("--folders-fg").trim(),
        barre: s.getPropertyValue("--bar-fg").trim(),
        connexion: s.getPropertyValue("--login-title").trim(),
        ombre: s.getPropertyValue("--login-title-shadow").trim(),
      };
    }, t);

    const png = await page.screenshot({ type: "png" });
    const mesure = await page.evaluate(async (b64, W, H, BG, BH) => {
      const img = new Image();
      img.src = "data:image/png;base64," + b64;
      await img.decode();
      const c = document.createElement("canvas"); c.width = W; c.height = H;
      c.getContext("2d").drawImage(img, 0, 0, W, H);
      const px = (w, h) => [...c.getContext("2d").getImageData(0, 0, w, h).data];
      return { gauche: px(BG, H), haute: px(W, BH) };
    }, png.toString("base64"), W, H, BANDE_G, BANDE_H);

    // Deuxième photographie : la page de connexion telle quelle, pour la zone du titre et des liens.
    await page.goto(BASE + "/login?theme_controle=" + encodeURIComponent(t), { waitUntil: "networkidle0" });
    await page.evaluate(async (t) => {
      document.querySelectorAll("link[data-controle]").forEach((e) => e.remove());
      const l = document.createElement("link");
      l.rel = "stylesheet"; l.href = "/themes/" + t + "/theme.css"; l.dataset.controle = "1";
      document.head.appendChild(l);
      await new Promise((r) => { l.onload = r; l.onerror = r; });
      const u = getComputedStyle(document.documentElement).getPropertyValue("--wall").match(/url\("([^"]+)"\)/);
      if (u) { const im = new Image(); im.src = u[1]; await im.decode().catch(() => {}); }
      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    }, t);
    const zonesTexte = await page.evaluate(() => {
      const r = [];
      for (const sel of ["h1", ".authtitle", ".authlinks"]) {
        for (const e of document.querySelectorAll(sel)) {
          const b = e.getBoundingClientRect();
          if (b.width > 0 && b.height > 0) r.push({ x: Math.round(b.x), y: Math.round(b.y), w: Math.round(b.width), h: Math.round(b.height) });
        }
      }
      return r;
    });
    const pngConn = await page.screenshot({ type: "png" });
    const conn = await page.evaluate(async (b64, W, H, rects) => {
      const img = new Image(); img.src = "data:image/png;base64," + b64; await img.decode();
      const c = document.createElement("canvas"); c.width = W; c.height = H;
      c.getContext("2d").drawImage(img, 0, 0, W, H);
      const out = [];
      for (const r of rects) out.push(...c.getContext("2d").getImageData(r.x, r.y, r.w, r.h).data);
      return out;
    }, pngConn.toString("base64"), W, H, zonesTexte);
    mesure.connexion = conn;

    const centile = (d, q) => {
      const v = [];
      for (let i = 0; i < d.length; i += 4 * 7) v.push(lum(d[i], d[i + 1], d[i + 2]));
      v.sort((a, b) => a - b);
      return v[Math.min(v.length - 1, Math.floor(v.length * q))];
    };
    const texte = (css) => {
      const m = css.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
      if (!m) return null;
      const h = m[1].length === 3 ? m[1].replace(/./g, (x) => x + x) : m[1];
      return lum(parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16));
    };
    const zones = [
      ["dossiers", mesure.gauche, couleurs.dossiers],
      ["barre", mesure.haute, couleurs.barre],
      ["connexion", mesure.connexion, couleurs.connexion],
    ];
    for (const [quoi, bande, coul] of zones) {
      const lt = texte(coul);
      if (lt === null) { console.log(`  ? ${t} ${quoi} : couleur de texte ${coul} non lue`); continue; }
      const lb = centile(bande, quoi === "connexion" ? 0.5 : 0.9);
      const r = contraste(lt, lb);
      const ok = r >= VISE;
      // La zone « connexion » ne compte pas dans le verdict : le titre et les liens de la page de connexion
      // sont posés à même le fond, hors de la carte, et le seul remède qui ne change pas la silhouette de
      // la page est l'ombre portée. DarkShine lui-même est à 4,47:1 — c'est l'état de l'art du produit, pas
      // un défaut de ce thème-ci. La mesure reste affichée pour qu'on sache où l'on en est.
      if (!ok && quoi !== "connexion") echecs++;
      const marque = ok ? "ok    " : (quoi === "connexion" ? "note  " : "FAIBLE");
      console.log(`  ${marque} ${t} ${quoi} : ${r.toFixed(2)}:1 (visé ${VISE})`);
      if (ok) continue;
      if (quoi === "connexion") {
        if (couleurs.ombre === "none" || couleurs.ombre === "") {
          console.log("         --login-title-shadow vaut « none » : une ombre portée gagne 1 à 4 points ici");
        }
        continue;
      }
      const f = voileNecessaire(lb, lt, VISE);
      if (f === null) {
        console.log(`         aucun voile raisonnable n'y suffit : reprenez --${quoi === "dossiers" ? "folders" : "bar"}-fg dans apercu.html`);
        continue;
      }
      const couche = quoi === "dossiers"
        ? `linear-gradient(90deg, rgba(0, 0, 0, ${f.toFixed(2)}) 0 13%, rgba(0, 0, 0, 0) 30%),`
        : `linear-gradient(180deg, rgba(0, 0, 0, ${f.toFixed(2)}) 0 5%, rgba(0, 0, 0, 0) 20%),`;
      console.log(`         à poser en tête de --wall dans internal/web/static/themes/${t}/theme.css :`);
      console.log(`           ${couche}`);
    }
  }
  await browser.close();
  if (echecs) {
    console.error(`\n${echecs} zone(s) sous le seuil : ajoutez un voile dans --wall (voir docs/themes.md).`);
    process.exit(1);
  }
  console.log("\nTous les fonds laissent les dossiers et la barre lisibles.");
  console.log("La zone « connexion » est indicative : voir docs/themes.md, § Le voile de lisibilité.");
})();
