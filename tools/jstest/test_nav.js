// Navigation au clavier (listnav.js) et glisser-déposer (dnd.js) sur la vraie page de la liste.
const { JSDOM } = require("jsdom");
const fs = require("fs");
const html = fs.readFileSync("home3.html", "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };

function boot(scripts) {
  const dom = new JSDOM(html, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document;
  w.Element.prototype.scrollIntoView = function () {};
  const opened = [];
  d.querySelectorAll("a.rowlink").forEach((a) => a.addEventListener("click", (e) => { e.preventDefault(); opened.push(a.getAttribute("href")); }));
  scripts.forEach((s) => w.eval(fs.readFileSync(s, "utf8")));
  return { w, d, opened };
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const key = (w, k, o) => { const e = new w.KeyboardEvent("keydown", Object.assign({ key: k, bubbles: true, cancelable: true }, o || {})); w.document.body.dispatchEvent(e); return e; };

(async () => {
{
  const { w, d, opened } = boot(["listnav.js"]);
  const rows = d.querySelectorAll(".msgrow");
  ok(rows.length >= 10, rows.length + " lignes");
  key(w, "ArrowDown"); await sleep(260);
  ok(rows[0].classList.contains("focused") && opened.length === 1, "↓ : première ligne, message ouvert");
  key(w, "ArrowDown"); key(w, "ArrowDown"); await sleep(260);
  ok(rows[2].classList.contains("focused") && opened.length === 2, "↓↓ rapides : un seul message ouvert (le dernier)");
  key(w, "ArrowUp"); await sleep(260);
  ok(rows[1].classList.contains("focused") && !rows[2].classList.contains("focused"), "↑ : ligne précédente");
  key(w, "End");
  ok(rows[rows.length - 1].classList.contains("focused"), "Fin : dernière ligne");
  key(w, "Home");
  ok(rows[0].classList.contains("focused"), "Début : première ligne");
  key(w, "PageDown");
  ok(rows[Math.min(10, rows.length - 1)].classList.contains("focused"), "Page suivante : dix lignes plus bas");
  key(w, "Home");
  const n0 = opened.length;
  key(w, " ");
  ok(rows[0].querySelector("input[name=uid]").checked && opened.length === n0, "Espace coche sans ouvrir");
  key(w, " ");
  ok(!rows[0].querySelector("input[name=uid]").checked, "Espace décoche");
  key(w, "ArrowDown", { shiftKey: true });
  ok(rows[0].querySelector("input[name=uid]").checked && rows[1].querySelector("input[name=uid]").checked && opened.length === n0, "Maj+↓ coche en passant, sans ouvrir");
  key(w, "Enter");
  ok(opened.length === n0 + 1, "Entrée ouvre la ligne");
  const input = d.getElementById("q");
  const e = new w.KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, cancelable: true });
  input.dispatchEvent(e);
  ok(!e.defaultPrevented, "pas de navigation pendant la saisie");
}

{
  const { w, d } = boot(["dnd.js"]);
  const form = d.querySelector("form.msglist-form");
  let submitted = null;
  form.submit = () => { submitted = Array.from(form.querySelectorAll("input[type=hidden]")).filter((i) => /_to$/.test(i.name)).map((i) => i.name + "=" + i.value); };
  const rows = d.querySelectorAll(".msgrow");
  ok(rows[0].getAttribute("draggable") === "true", "les lignes sont glissables");
  const targets = d.querySelectorAll("[data-drop]");
  ok(targets.length >= 2, targets.length + " dossiers cibles");
  const target = Array.from(targets).find((t) => !t.classList.contains("active"));
  const ev = (type, o) => { const e = new w.Event(type, Object.assign({ bubbles: true, cancelable: true }, o || {})); e.dataTransfer = { setData() {}, effectAllowed: "", dropEffect: "" }; Object.assign(e, o || {}); return e; };
  rows[2].dispatchEvent(ev("dragstart"));
  ok(rows[2].querySelector("input[name=uid]").checked && form.querySelectorAll("input[name=uid]:checked").length === 1, "le message glissé est la seule sélection");
  const over = ev("dragover"); target.dispatchEvent(over);
  ok(over.defaultPrevented && target.classList.contains("drop-over"), "survol : dépôt autorisé et dossier mis en évidence");
  target.dispatchEvent(ev("drop"));
  ok(submitted && submitted.length === 1 && submitted[0] === "move_to=" + target.getAttribute("data-drop"), "dépôt : déplacement vers " + target.getAttribute("data-drop") + " (" + submitted + ")");
  // Ctrl = copie
  submitted = null;
  form.querySelectorAll("input[type=hidden][name$=_to]").forEach((i) => i.remove());
  rows[3].dispatchEvent(ev("dragstart"));
  target.dispatchEvent(ev("dragover", { ctrlKey: true }));
  target.dispatchEvent(ev("drop", { ctrlKey: true }));
  ok(submitted && submitted[0] === "copy_to=" + target.getAttribute("data-drop"), "Ctrl + dépôt : copie (" + submitted + ")");
  // glisser une ligne cochée avec d'autres : toute la sélection part
  submitted = null;
  form.querySelectorAll("input[type=hidden][name$=_to]").forEach((i) => i.remove());
  form.querySelectorAll("input[name=uid]:checked").forEach((c) => { c.checked = false; });
  rows[4].querySelector("input[name=uid]").checked = true; rows[5].querySelector("input[name=uid]").checked = true;
  rows[4].dispatchEvent(ev("dragstart"));
  ok(form.querySelectorAll("input[name=uid]:checked").length === 2, "glisser un message coché emporte la sélection");
  // dépôt sur le dossier déjà affiché : ignoré
  const active = d.querySelector("[data-drop].active");
  if (active) { submitted = null; form.querySelectorAll("input[type=hidden][name$=_to]").forEach((i) => i.remove()); active.dispatchEvent(ev("dragover")); active.dispatchEvent(ev("drop")); ok(submitted === null, "dépôt sur le dossier courant ignoré"); }
}

})().then(() => {
{
  const html2 = fs.readFileSync("open.html", "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
  const dom = new JSDOM(html2, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document;
  w.eval(fs.readFileSync("msgfull.js", "utf8"));
  const btn = d.querySelector("[data-fullscreen-msg]");
  ok(!!btn, "bouton plein écran dans l'en-tête du message");
  btn.click();
  ok(d.body.classList.contains("msg-fullscreen"), "le clic passe en plein écran");
  let reached = false;
  d.addEventListener("keydown", () => { reached = true; });
  const e = new w.KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  d.body.dispatchEvent(e);
  ok(!d.body.classList.contains("msg-fullscreen") && !reached, "Échap quitte le plein écran sans fermer le message");
  btn.click(); d.dispatchEvent(new w.Event("wmail:pane"));
  ok(!d.body.classList.contains("msg-fullscreen"), "un autre message n'hérite pas du plein écran");
}
console.log(fails ? "ÉCHECS : " + fails : "TOUS LES CONTRÔLES PASSENT");
process.exit(fails ? 1 : 0);
});
