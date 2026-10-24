// Pastilles d'adresses (chips.js) sur un squelette de la page de composition.
const { JSDOM } = require("jsdom");
const fs = require("fs");
const script = fs.readFileSync("chips.js", "utf8");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };

function boot(to = "", cc = "") {
  const dom = new JSDOM(`<form id="compose"><label for="cp-to">À</label><div class="cp-value"><input type="text" id="cp-to" name="to" value="${to.replace(/"/g, "&quot;")}"></div>
    <div class="cp-value"><input type="text" id="cp-cc" name="cc" value="${cc}"></div></form>`, { runScripts: "outside-only", pretendToBeVisual: true });
  dom.window.eval(script);
  return dom.window;
}
const chips = (w, id) => Array.from(w.document.getElementById(id).chipsInput.parentNode.querySelectorAll(".chip"));
const type = (w, id, text) => { const i = w.document.getElementById(id).chipsInput; i.value = text; i.dispatchEvent(new w.Event("input", { bubbles: true })); return i; };
const key = (i, k) => { const e = new i.ownerDocument.defaultView.KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }); i.dispatchEvent(e); return e; };

{
  const w = boot('Zoë Dupont <zoe@x.org>, "Martin, Paul" <paul@x.org>, nul@part');
  const c = chips(w, "cp-to");
  ok(c.length === 3, "trois pastilles à partir du champ rempli : " + c.length);
  ok(c[0].textContent.startsWith("Zoë Dupont") && c[1].textContent.startsWith("Martin, Paul"), "nom affiché, virgule entre guillemets respectée");
  ok(c[2].classList.contains("bad"), "une adresse invalide est signalée (rien n'est perdu en silence)");
  ok(w.document.getElementById("cp-to").hidden && w.document.getElementById("cp-to").value.includes('"Martin, Paul" <paul@x.org>'), "le champ d'origine garde la liste");
  ok(w.document.querySelector('label[for="cp-to-in"]'), "l'étiquette pointe vers la saisie visible");
}

{
  const w = boot();
  let i = type(w, "cp-to", "a@x.org, b@x.org; c@x.org,");
  ok(chips(w, "cp-to").length === 3 && i.value === "", "collage de plusieurs adresses : une pastille chacune");
  ok(w.document.getElementById("cp-to").value === "a@x.org, b@x.org, c@x.org", "valeur synchronisée : " + w.document.getElementById("cp-to").value);
  i = type(w, "cp-to", "d@x.org");
  ok(chips(w, "cp-to").length === 3, "tant qu'il n'y a pas de séparateur, c'est de la saisie");
  const e = key(i, "Enter");
  ok(e.defaultPrevented && chips(w, "cp-to").length === 4 && i.value === "", "Entrée valide la saisie sans envoyer le formulaire");
  i.dispatchEvent(new w.Event("blur"));
  const e2 = key(i, "Enter");
  ok(e2.defaultPrevented, "Entrée sur un champ vide n'envoie pas le formulaire");
  key(i, "Backspace");
  ok(chips(w, "cp-to").length === 3 && w.document.getElementById("cp-to").value === "a@x.org, b@x.org, c@x.org", "Retour arrière retire la dernière pastille");
  chips(w, "cp-to")[0].querySelector(".chip-x").click();
  ok(chips(w, "cp-to").length === 2 && w.document.getElementById("cp-to").value === "b@x.org, c@x.org", "la croix retire la pastille");
  chips(w, "cp-to")[0].dispatchEvent(new w.Event("dblclick", { bubbles: true }));
  ok(i.value === "b@x.org" && chips(w, "cp-to").length === 1, "double-clic : la pastille revient en saisie");
  i.value = "e@x.org"; i.dispatchEvent(new w.Event("blur"));
  ok(chips(w, "cp-to").length === 2, "la sortie du champ valide la saisie");
}

{
  const w = boot("a@x.org, b@x.org", "");
  const src = chips(w, "cp-to")[0];
  const dst = w.document.getElementById("cp-cc").chipsInput.parentNode;
  src.dispatchEvent(Object.assign(new w.Event("dragstart", { bubbles: true }), { dataTransfer: { setData() {}, effectAllowed: "" } }));
  const over = new w.Event("dragover", { bubbles: true, cancelable: true }); dst.dispatchEvent(over);
  ok(over.defaultPrevented, "le champ cible accepte la pastille");
  dst.dispatchEvent(new w.Event("drop", { bubbles: true, cancelable: true }));
  ok(w.document.getElementById("cp-to").value === "b@x.org" && w.document.getElementById("cp-cc").value === "a@x.org", "la pastille passe de À à Cc : " + w.document.getElementById("cp-to").value + " | " + w.document.getElementById("cp-cc").value);
}

console.log(fails ? "ÉCHECS : " + fails : "TOUS LES CONTRÔLES PASSENT");
process.exit(fails ? 1 : 0);
