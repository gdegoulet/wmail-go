// Panneau des dossiers (folders.js) : niveaux repliables, panneau masquable, création d'un dossier.
const { JSDOM } = require("jsdom");
const fs = require("fs");
const html = fs.readFileSync("home3.html", "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
const script = fs.readFileSync("folders.js", "utf8");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };

function boot() {
  const dom = new JSDOM(html, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document, calls = [];
  w.fetch = (url, o) => { calls.push({ url, body: String(o && o.body) }); return Promise.resolve({}); };
  w.eval(script);
  return { w, d, calls };
}
const row = (d, key) => d.querySelector('#foldertree li[data-key="' + key + '"]');

{
  const { d, calls } = boot();
  const tree = d.getElementById("foldertree");
  ok(!!tree && tree.children.length >= 3, tree.children.length + " lignes dans l'arbre");
  const p = row(d, "Projets"), a = row(d, "Projets.Annee"), m = row(d, "Projets.Annee.Mois");
  ok(p && a && m, "trois niveaux présents");
  ok(!p.hidden && a.hidden && m.hidden, "tout est replié au départ, seul le niveau 0 est visible");
  const caret = p.querySelector("[data-caret]");
  ok(caret && !caret.hidden, "la flèche s'affiche avec JavaScript");
  ok(!a.querySelector("[data-caret]").hidden && !p.querySelector(".caret.none"), "chaque dossier ayant des enfants a une flèche");
  caret.click();
  ok(p.getAttribute("data-open") === "1" && !a.hidden && m.hidden, "ouvrir Projets montre Annee, pas Mois");
  ok(caret.getAttribute("aria-expanded") === "true" && caret.textContent === "▾", "flèche vers le bas et aria-expanded");
  ok(calls.length === 1 && calls[0].url === "/settings/folder-open" && /folder=Projets/.test(calls[0].body) && /open=1/.test(calls[0].body) && /csrf=/.test(calls[0].body), "enregistrement : " + calls[0].body);
  a.querySelector("[data-caret]").click();
  ok(!m.hidden, "ouvrir Annee montre Mois");
  caret.click();
  ok(p.getAttribute("data-open") === "0" && a.hidden && m.hidden, "replier Projets cache aussi les petits-enfants");
  ok(calls[calls.length - 1].body.includes("open=0"), "le repliage est enregistré");
  caret.click();
  ok(!a.hidden && !m.hidden, "rouvrir Projets retrouve l'état de Annee (ouvert)");
}

{
  const { w, d, calls } = boot();
  const btn = d.querySelector("[data-folders-toggle]");
  ok(btn && !btn.hidden, "le bouton du panneau est visible avec JavaScript");
  btn.click();
  ok(d.body.classList.contains("folders-collapsed") && btn.getAttribute("aria-pressed") === "true", "le panneau est masqué");
  ok(calls.length === 1 && calls[0].url === "/settings/panes" && calls[0].body.includes("hidden=1"), "enregistré : " + calls[0].body);
  btn.click();
  ok(!d.body.classList.contains("folders-collapsed") && calls[1].body.includes("hidden=0"), "le panneau est réaffiché et enregistré");
  ok(!d.querySelector("[data-fullscreen]"), "plus de bouton plein écran");
}

{
  const { w, d } = boot();
  const dlg = d.getElementById("newfolder"), link = d.querySelector("[data-newfolder]");
  ok(dlg.hidden, "le formulaire de création est caché au départ");
  const ev = new w.MouseEvent("click", { bubbles: true, cancelable: true });
  link.dispatchEvent(ev);
  ok(ev.defaultPrevented && !dlg.hidden && d.activeElement === dlg.querySelector("input[name=new_name]"), "le bouton ouvre le formulaire, curseur sur le nom");
  const sel = dlg.querySelector("select[name=parent]");
  ok(sel.options.length > 3 && sel.options[0].value === "" && [...sel.options].some(o => o.value === "Projets.Annee"), sel.options.length + " dossiers proposés comme parent");
  ok(dlg.querySelector("input[name=delim]").value === "." && dlg.querySelector("input[name=return_to]"), "séparateur du serveur et retour à la page");
  const esc = new w.KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  d.body.dispatchEvent(esc);
  ok(dlg.hidden && esc.defaultPrevented, "Échap ferme le formulaire");
  link.dispatchEvent(new w.MouseEvent("click", { bubbles: true, cancelable: true }));
  dlg.querySelector("[data-newfolder-close]").click();
  ok(dlg.hidden, "Annuler ferme le formulaire");
}
if (fails) { console.log(fails + " échec(s)"); process.exit(1); }
