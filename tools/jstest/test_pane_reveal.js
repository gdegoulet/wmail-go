// Contrôles jsdom sur le volet de lecture réécrit sans rechargement de la page (/mail/pane) :
//  - D-23 : les cases de sélection des pièces jointes restent visibles d'un message à l'autre ;
//  - D-20 : le bouton « plein écran » est révélé à chaque message, et Échap en sort ;
//  - D-15 : les flèches « message précédent / suivant » sont renseignées d'après la liste affichée.
const { JSDOM, VirtualConsole } = require("jsdom");
const fs = require("fs");
const home = fs.readFileSync("home3.html", "utf8");
const fragatt = fs.readFileSync("fragatt.html", "utf8");
const scripts = ["list.js", "listnav.js", "preview.js", "msgfull.js"].map(f => fs.readFileSync(f, "utf8"));
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };
const tick = (ms = 0) => new Promise(r => setTimeout(r, ms));

function boot() {
  const vc = new VirtualConsole();
  vc.on("jsdomError", () => {}); // les navigations simulées ne sont pas des échecs
  const dom = new JSDOM(home, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true, virtualConsole: vc });
  const w = dom.window, d = w.document;
  w.HTMLFormElement.prototype.submit = function () {};
  if (!w.HTMLDialogElement) w.HTMLDialogElement = function () {};
  // Le serveur renvoie toujours le même message à pièces jointes ; on lui donne l'uid demandé pour que les
  // flèches se calculent sur la bonne ligne.
  w.fetch = (url) => {
    const uid = new w.URL(url, "http://127.0.0.1:8080/").searchParams.get("uid");
    return Promise.resolve({ redirected: false, text: () => Promise.resolve(fragatt.replace(/data-uid="[^"]*"/, 'data-uid="' + uid + '"')) });
  };
  const lp = d.querySelector(".msglist-form");
  Object.defineProperty(lp, "scrollTop", { value: 0, writable: true, configurable: true });
  scripts.forEach(s => w.eval(s));
  return { w, d };
}
const click = (w, el, init = {}) => { const e = new w.MouseEvent("click", { bubbles: true, cancelable: true, button: 0, ...init }); el.dispatchEvent(e); return e; };
const key = (w, k, opts = {}) => { const e = new w.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true, ...opts }); (opts.on || w.document).dispatchEvent(e); return e; };
const rows = d => [...d.querySelectorAll(".msgrow")];
const open = async (w, d, row) => { click(w, row.querySelector("a.rowlink")); await tick(20); };

(async () => {
  console.log("== D-23 cases à cocher des pièces jointes après injection du volet");
  {
    const { w, d } = boot();
    ok(d.querySelectorAll(".att-pick").length === 0, "au départ le volet est vide (aucune pièce jointe)");
    await open(w, d, rows(d)[1]);
    const picks = [...d.querySelectorAll(".att-pick")];
    ok(picks.length >= 2, picks.length + " cases de sélection dans le volet injecté");
    ok(picks.every(p => !p.hidden), "toutes les cases sont visibles (hidden retiré)");
    ok(picks.every(p => p.querySelector("input.att-check") !== null), "chaque case contient bien sa boîte à cocher");
    const zip = d.querySelector("[data-zip-selected]");
    ok(zip !== null && zip.hidden, "le lien « télécharger la sélection » reste caché tant que rien n'est coché");

    // navigation d'un message à l'autre : le volet est remplacé, les cases doivent revenir
    await open(w, d, rows(d)[3]);
    const again = [...d.querySelectorAll(".att-pick")];
    ok(again.length >= 2 && again.every(p => !p.hidden), "après un second message, les cases sont de nouveau visibles");

    // cocher une case alimente le lien ZIP
    const box = again[0].querySelector("input.att-check");
    box.checked = true;
    box.dispatchEvent(new w.Event("change", { bubbles: true }));
    ok(!d.querySelector("[data-zip-selected]").hidden, "une case cochée fait apparaître le lien « télécharger la sélection »");
    ok(d.querySelector("[data-zip-selected]").getAttribute("href").includes("index=" + box.value), "le lien ZIP porte l'index coché");
  }

  console.log("== D-20 bouton plein écran");
  {
    const { w, d } = boot();
    await open(w, d, rows(d)[1]);
    const btn = d.querySelector("[data-fullscreen-msg]");
    ok(btn !== null && !btn.hidden, "le bouton plein écran du volet injecté est visible");
    ok((btn.getAttribute("title") || "").length > (btn.getAttribute("aria-label") || "").length, "son infobulle rappelle les touches : " + btn.getAttribute("title"));
    click(w, btn);
    ok(d.body.classList.contains("msg-fullscreen"), "le clic passe en plein écran");
    key(w, "Escape");
    ok(!d.body.classList.contains("msg-fullscreen"), "Échap sort du plein écran");
    click(w, btn);
    ok(d.body.classList.contains("msg-fullscreen"), "retour en plein écran");
    await open(w, d, rows(d)[3]);
    ok(!d.body.classList.contains("msg-fullscreen"), "ouvrir un autre message quitte le plein écran");
    ok(!d.querySelector("[data-fullscreen-msg]").hidden, "et son bouton est visible à son tour");

    // Entrée quand le volet de lecture a le focus (on y arrive avec →)
    const pane = d.querySelector(".readpane");
    pane.setAttribute("tabindex", "-1");
    pane.focus();
    key(w, "Enter", { on: pane });
    ok(d.body.classList.contains("msg-fullscreen"), "Entrée dans le volet de lecture passe en plein écran");
    key(w, "Escape");
    ok(!d.body.classList.contains("msg-fullscreen"), "Échap en sort");
  }

  console.log("== D-15 flèches message précédent / suivant");
  {
    const { w, d } = boot();
    const all = rows(d);
    const mid = all[Math.floor(all.length / 2)];
    await open(w, d, mid);
    const nav = () => [...d.querySelectorAll(".readpane a.mh-nav[data-msgnav]")];
    const prev = () => nav().find(a => a.getAttribute("data-msgnav") === "prev");
    const next = () => nav().find(a => a.getAttribute("data-msgnav") === "next");
    ok(nav().length === 2, "les deux flèches sont des liens");
    ok(!prev().classList.contains("off") && prev().getAttribute("href"), "au milieu de la liste la flèche « précédent » est active : " + prev().getAttribute("href"));
    ok(!next().classList.contains("off") && next().getAttribute("href"), "et la flèche « suivant » aussi : " + next().getAttribute("href"));
    const prevUid = all[all.indexOf(mid) - 1].id.slice(1), nextUid = all[all.indexOf(mid) + 1].id.slice(1);
    ok(prev().getAttribute("href").includes("uid=" + prevUid), "« précédent » vise la ligne au-dessus (uid " + prevUid + ")");
    ok(next().getAttribute("href").includes("uid=" + nextUid), "« suivant » vise la ligne en dessous (uid " + nextUid + ")");

    // la flèche recharge bien le volet sans recharger la page
    const e = click(w, next());
    ok(e.defaultPrevented, "un clic sur la flèche est intercepté (pas de rechargement)");
    await tick(20);
    ok(d.querySelector(".msgview").getAttribute("data-uid") === nextUid, "le volet affiche le message suivant");

    await open(w, d, all[0]);
    ok(prev().classList.contains("off") && !prev().getAttribute("href"), "en tête de liste « précédent » est grisée");
    ok(!next().classList.contains("off"), "mais « suivant » reste active");
    await open(w, d, all[all.length - 1]);
    ok(next().classList.contains("off") && !next().getAttribute("href"), "en fin de liste « suivant » est grisée");
    ok(!prev().classList.contains("off"), "mais « précédent » reste active");
    // une flèche grisée ne déclenche aucun chargement
    const before = d.querySelector(".msgview").getAttribute("data-uid");
    click(w, next());
    await tick(20);
    ok(d.querySelector(".msgview").getAttribute("data-uid") === before, "cliquer la flèche grisée ne change rien");
  }

  console.log(fails === 0 ? "tous les contrôles passent" : fails + " contrôle(s) en échec");
  process.exit(fails === 0 ? 0 : 1);
})();
