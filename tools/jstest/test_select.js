const { JSDOM } = require("jsdom");
const fs = require("fs");
const html = fs.readFileSync("home3.html", "utf8").replace(/<script src="[^"]*list.js"[^>]*><\/script>/, "");
const dom = new JSDOM(html, { runScripts: "outside-only", pretendToBeVisual: true });
const { window } = dom, { document } = window;
let submitted = 0;
window.HTMLFormElement.prototype.submit = function () { submitted++; };
window.eval(fs.readFileSync("list.js", "utf8"));

let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };
const boxes = () => [...document.querySelectorAll("form.msglist-form input[name=uid]")];
const checked = () => boxes().filter(b => b.checked).length;
const key = (target, opts) => { const e = new window.KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...opts }); target.dispatchEvent(e); return e; };
const selAll = document.getElementById("select-all");
const actions = [...document.querySelectorAll("[data-needs-selection]")];

const N = boxes().length; ok(N > 10, N + " cases à cocher dans la liste");
ok(selAll.closest("label").hidden === false, "« Tout sélectionner » devient visible avec JavaScript");
ok(actions.length === 9 && actions.every(b => b.disabled), actions.length + " boutons d'action (barre d'outils) grisés sans sélection ni message ouvert");

let e = key(document.body, { key: "a", ctrlKey: true });
ok(e.defaultPrevented, "Ctrl+A est intercepté (pas de sélection de texte de la page)");
ok(checked() === N && selAll.checked && !selAll.indeterminate, "Ctrl+A coche les N messages affichés");
ok(actions.every(b => !b.disabled), "boutons d'action activés");
ok(document.querySelectorAll(".msgrow.checked").length === N, "lignes mises en évidence");

key(document.body, { key: "Escape" });
ok(checked() === 0 && actions.every(b => b.disabled), "Échap efface la sélection");

// Ctrl+A bascule : tout sélectionner, puis (tout étant sélectionné) tout désélectionner.
e = key(document.body, { key: "a", ctrlKey: true });
ok(e.defaultPrevented && checked() === N, "Ctrl+A (rien de sélectionné) : tout sélectionner");
e = key(document.body, { key: "a", ctrlKey: true });
ok(e.defaultPrevented && checked() === 0 && actions.every(b => b.disabled) && !selAll.checked && !selAll.indeterminate,
  "Ctrl+A (tout est déjà sélectionné) : tout désélectionner");
e = key(document.body, { key: "a", ctrlKey: true });
ok(checked() === N, "Ctrl+A de nouveau : tout sélectionner");
boxes()[10].checked = false; boxes()[10].dispatchEvent(new window.Event("change", { bubbles: true })); boxes()[10].click(); boxes()[10].click();
ok(checked() === N - 1 + 0 || checked() === N, "case décochée à la main : " + checked() + " cochée(s)");
boxes()[10].checked = false; document.getElementById("select-all").dispatchEvent(new window.Event("change", { bubbles: true }));
key(document.body, { key: "Escape" });
boxes()[2].click();
e = key(document.body, { key: "a", ctrlKey: true });
ok(checked() === N, "Ctrl+A avec une sélection partielle : tout sélectionner (et non désélectionner)");
key(document.body, { key: "Escape" });

e = key(document.body, { key: "A", metaKey: true });
ok(e.defaultPrevented && checked() === N, "Cmd+A (Mac) fonctionne aussi");
e = key(document.body, { key: "A", metaKey: true });
ok(e.defaultPrevented && checked() === 0, "Cmd+A de nouveau : tout désélectionner");

const q = document.getElementById("q");
e = key(q, { key: "a", ctrlKey: true });
ok(!e.defaultPrevented && checked() === 0, "Ctrl+A dans le champ de recherche reste un Ctrl+A de texte");
const dest = document.querySelector("select[name=dest]") || document.querySelector("select[name=scope]");
e = key(dest, { key: "a", ctrlKey: true });
ok(!e.defaultPrevented, "Ctrl+A dans une liste déroulante n'est pas intercepté");
e = key(document.body, { key: "a", ctrlKey: true, shiftKey: true });
ok(!e.defaultPrevented && checked() === 0, "Ctrl+Maj+A n'est pas intercepté");

selAll.checked = true; selAll.dispatchEvent(new window.Event("change", { bubbles: true }));
ok(checked() === N, "case « Tout sélectionner » : tout coché");
selAll.checked = false; selAll.dispatchEvent(new window.Event("change", { bubbles: true }));
ok(checked() === 0, "case « Tout sélectionner » décochée : tout décoché");

const b = boxes();
b[3].click();
ok(checked() === 1, "clic sur une case : 1 sélectionné");
ok(selAll.indeterminate && !selAll.checked, "« Tout sélectionner » en état indéterminé");
b[8].dispatchEvent(new window.MouseEvent("click", { bubbles: true, cancelable: true, shiftKey: true }));
ok(checked() === 6 && b.slice(3, 9).every(x => x.checked), "Maj+clic sélectionne la plage 4 à 9 : " + checked());
b[5].click();
ok(checked() === 5 && !b[5].checked, "décocher une case au milieu");

// Un message ouvert dans le volet de lecture active les boutons même sans sélection.
boxes().forEach(b => { b.checked = false; });
document.getElementById("current-uid").value = "12";
document.dispatchEvent(new window.Event("wmail:pane"));
ok(checked() === 0 && actions.every(b => !b.disabled), "message ouvert : boutons d'action activés sans sélection");
document.getElementById("current-uid").value = "";
document.dispatchEvent(new window.Event("wmail:pane"));
ok(actions.every(b => b.disabled), "message refermé : boutons de nouveau grisés");

// --- sélection par critère ---
{
  const rowsAll = [...document.querySelectorAll(".msgrow")];
  const pick = (kind) => document.querySelector('[data-select="' + kind + '"]').click();
  const sel = () => boxes().filter(b => b.checked).length;
  const unread = rowsAll.filter(r => r.classList.contains("unread")).length;
  const flagged = rowsAll.filter(r => r.querySelector(".star.on")).length;
  ok(!!document.querySelector('[data-select="invert"]'), "menu de sélection par critère présent");
  pick("none"); ok(sel() === 0, "Aucun");
  pick("all"); ok(sel() === rowsAll.length, "Tous");
  pick("none"); pick("unseen"); ok(sel() === unread && unread > 0, "Non lus : " + sel() + "/" + unread);
  pick("seen"); ok(sel() === rowsAll.length - unread, "Lus : " + sel());
  pick("flagged"); ok(sel() === flagged, "Favoris : " + sel() + "/" + flagged);
  pick("unflagged"); ok(sel() === rowsAll.length - flagged, "Sans favori : " + sel());
  pick("none"); boxes()[0].checked = true; pick("invert");
  ok(sel() === rowsAll.length - 1 && !boxes()[0].checked, "Inverser : " + sel());
  ok(actions.every(b => !b.disabled), "les boutons d'action suivent la sélection");
}
console.log(fails === 0 ? "\nTOUS LES CONTRÔLES PASSENT" : "\n" + fails + " ÉCHEC(S)");
process.exit(fails ? 1 : 0);
