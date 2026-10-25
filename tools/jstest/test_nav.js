// Navigation au clavier (listnav.js) et glisser-déposer (dnd.js) sur la vraie page de la liste.
const { JSDOM } = require("jsdom");
const fs = require("fs");
const html = fs.readFileSync("home3.html", "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };

function boot(scripts, prep) {
  const dom = new JSDOM(html, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document;
  w.Element.prototype.scrollIntoView = function () {};
  if (prep) prep(w, d);
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
  const chk = (i) => rows[i].querySelector("input[name=uid]").checked;
  const nChecked = () => [...d.querySelectorAll("input[name=uid]")].filter((c) => c.checked).length;
  key(w, "ArrowDown", { shiftKey: true });
  key(w, "ArrowDown", { shiftKey: true });
  ok(chk(0) && chk(1) && chk(2) && chk(3) && nChecked() === 4, "Maj+↓ encore : plage de 4 lignes");
  key(w, "ArrowUp", { shiftKey: true });
  ok(chk(0) && chk(1) && chk(2) && !chk(3) && nChecked() === 3, "Maj+↑ : changement de sens, la dernière ligne est décochée");
  key(w, "ArrowUp", { shiftKey: true });
  key(w, "ArrowUp", { shiftKey: true });
  ok(chk(0) && !chk(1) && !chk(2) && nChecked() === 1, "Maj+↑ jusqu'à l'ancre : seule l'ancre reste cochée");
  key(w, "ArrowUp", { shiftKey: true });
  ok(nChecked() === 1 && chk(0), "Maj+↑ en tête de liste : rien de plus");
  key(w, "Escape");
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

// Curseur conservé au travers d'une suppression : la page est rechargée, le curseur doit se poser
// sur la ligne qui suivait celle qui vient de disparaître.
{
  const { w, d } = boot(["listnav.js"]);
  const rows = d.querySelectorAll(".msgrow");
  const uid = (r) => r.querySelector("input[name=uid]").value;
  key(w, "Home"); key(w, "ArrowDown"); key(w, "ArrowDown"); // curseur sur la troisième ligne
  ok(rows[2].classList.contains("focused"), "curseur posé avant l'action");
  rows[2].querySelector("input[name=uid]").checked = true;
  d.getElementById("bulk").dispatchEvent(new w.Event("submit", { bubbles: true, cancelable: true }));
  const note = JSON.parse(w.sessionStorage.getItem("wmail.listCursor") || "null");
  ok(note && note.uid === uid(rows[2]) && note.suivant === uid(rows[3]), "l'envoi note la ligne visée et sa suivante");

  // Rechargement : la ligne supprimée a disparu de la liste, son UID n'existe plus.
  const suivant = uid(rows[3]);
  const rejoue = boot(["listnav.js"], (w2) => {
    w2.sessionStorage.setItem("wmail.listCursor", JSON.stringify({ sig: "/?", uid: "999999", suivant: suivant, idx: 2 }));
  });
  const apres = rejoue.d.getElementById("m" + suivant);
  ok(apres && apres.classList.contains("focused"), "après suppression, le curseur passe au message suivant");
  ok(!rejoue.w.sessionStorage.getItem("wmail.listCursor"), "la note ne sert qu'une fois");

  // Marquer lu ne retire pas la ligne : le curseur reste là où il était.
  const memeLigne = uid(rows[1]);
  const garde = boot(["listnav.js"], (w2) => {
    w2.sessionStorage.setItem("wmail.listCursor", JSON.stringify({ sig: "/?", uid: memeLigne, suivant: suivant, idx: 1 }));
  });
  ok(garde.d.getElementById("m" + memeLigne).classList.contains("focused"), "ligne toujours présente : le curseur ne bouge pas");
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
// Le volet de lecture se charge sans recharger la page : les champs de l'action groupée doivent suivre,
// sinon Suppr vise le message du dernier rendu complet et rouvre le voisin de celui-là.
{
  const html3 = fs.readFileSync("open.html", "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
  const dom = new JSDOM(html3, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document;
  w.Element.prototype.scrollIntoView = function () {};
  w.eval(fs.readFileSync("listnav.js", "utf8"));
  const uids = Array.from(d.querySelectorAll(".msgrow")).map((r) => r.id.slice(1));
  const view = d.querySelector(".msgview");
  const i = uids.indexOf(view.getAttribute("data-uid"));
  ok(i >= 0 && i + 2 < uids.length, "message ouvert repéré dans la liste (" + i + ")");
  view.setAttribute("data-uid", uids[i + 1]); // le volet vient d'afficher le message suivant
  d.dispatchEvent(new w.Event("wmail:pane"));
  ok(d.getElementById("current-uid").value === uids[i + 1], "current_uid suit le message affiché");
  const attendu = d.getElementById("m" + uids[i + 2]).querySelector("a.rowlink").getAttribute("href");
  ok(d.getElementById("open-next").value === attendu, "open_next désigne le voisin du message affiché");
  view.remove(); // message fermé : plus rien à supprimer ni à rouvrir
  d.dispatchEvent(new w.Event("wmail:pane"));
  ok(d.getElementById("current-uid").value === "" && d.getElementById("open-next").value === "", "volet fermé : champs vidés");
}
console.log(fails ? "ÉCHECS : " + fails : "TOUS LES CONTRÔLES PASSENT");
process.exit(fails ? 1 : 0);
});
