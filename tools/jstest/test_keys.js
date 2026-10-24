// Raccourcis clavier (keys.js) sur les pages produites par le serveur : liste seule, puis volet de lecture ouvert.
const { JSDOM } = require("jsdom");
const fs = require("fs");
const script = fs.readFileSync("keys.js", "utf8");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };

function boot(file) {
  const html = fs.readFileSync(file, "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
  const dom = new JSDOM(html, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document;
  const clicked = [];
  // On observe les clics (liens, boutons) sans laisser jsdom naviguer ni envoyer de formulaire.
  d.addEventListener("click", (e) => {
    const t = e.target.closest("a, button, summary");
    if (t) { clicked.push(t.getAttribute("href") || (t.tagName === "SUMMARY" ? "summary" : t.getAttribute("name") + "=" + t.getAttribute("value"))); e.preventDefault(); }
  }, true);
  w.eval(script);
  const press = (key, opts = {}, target = d.body) => { const e = new w.KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...opts }); target.dispatchEvent(e); return e; };
  return { w, d, clicked, press };
}

// --- liste seule, rien de sélectionné ---
{
  const { d, clicked, press } = boot("home3.html");
  d.querySelectorAll("[data-needs-selection]").forEach(b => { b.disabled = true; }); // état que list.js donne sans sélection
  let e = press("c");
  ok(e.defaultPrevented && clicked.pop().startsWith("/compose"), "C ouvre un nouveau message");
  e = press("z");
  ok(!e.defaultPrevented && clicked.length === 0, "Z ne fait rien sans sélection (bouton grisé)");
  e = press("r");
  ok(!e.defaultPrevented && clicked.length === 0, "R ne fait rien sans message ouvert : " + clicked.join(" "));
  e = press("/");
  ok(e.defaultPrevented && d.activeElement === d.getElementById("q"), "/ met le curseur dans la recherche");
  clicked.length = 0;
  press("c", {}, d.getElementById("q"));
  ok(clicked.length === 0, "les lettres tapées dans la recherche ne déclenchent rien");
  e = press("c", { ctrlKey: true });
  ok(!e.defaultPrevented && clicked.length === 0, "Ctrl+C n'est pas intercepté");
  // avec une ligne cochée, les actions de la barre d'outils deviennent possibles (list.js active les boutons)
  const box = d.querySelector("input[name=uid]");
  box.checked = true;
  d.querySelectorAll("[data-needs-selection]").forEach(b => { b.disabled = false; });
  press("z"); press("q"); press("u"); press("i"); press("Delete"); press("#");
  ok(clicked.join(" ") === "action=archive action=read action=unread action=flag action=delete action=delete",
    "Z, Q, U, I, Suppr et # actionnent archiver, lu, non lu, favori, supprimer : " + clicked.join(" "));
  clicked.length = 0;
  press("m");
  ok(clicked.join(" ") === "summary", "M ouvre le menu « déplacer vers » : " + clicked.join(" "));
}

// --- volet de lecture ouvert ---
{
  const { d, clicked, press } = boot("open.html");
  const got = (key, opts) => { clicked.length = 0; const e = press(key, opts); return [e.defaultPrevented, clicked[0] || ""]; };
  let [prevented, href] = got("r");
  ok(prevented && /mode=reply(&|$)/.test(href) && !/replyall/.test(href), "R : répondre — " + href);
  [prevented, href] = got("a");
  ok(prevented && /mode=replyall/.test(href), "A : répondre à tous");
  [prevented, href] = got("f");
  ok(prevented && /mode=forward(&|$)/.test(href), "F : transférer");
  [prevented, href] = got("F", { shiftKey: true });
  ok(prevented && /mode=forwardatt/.test(href), "Maj+F : transférer en pièce jointe");
  [prevented, href] = got("Escape");
  ok(prevented && href !== "", "Échap ferme le message : " + href);
  // l'aide
  const help = d.getElementById("keyhelp");
  ok(help.hidden, "l'aide est cachée au départ");
  press("?", { shiftKey: true });
  ok(!help.hidden, "? ouvre l'aide");
  press("?", { shiftKey: true });
  ok(help.hidden, "? la referme");
  press("?", { shiftKey: true }); clicked.length = 0;
  const e = press("Escape");
  ok(e.defaultPrevented && help.hidden && clicked.length === 0, "Échap ferme l'aide sans fermer le message");
  d.querySelector("[data-keyhelp]").click();
  ok(!help.hidden, "l'entrée du menu utilisateur ouvre aussi l'aide");
  d.querySelector("[data-keyhelp-close]").click();
  ok(help.hidden, "le bouton Fermer referme l'aide");
}
if (fails) { console.log(fails + " échec(s)"); process.exit(1); }
