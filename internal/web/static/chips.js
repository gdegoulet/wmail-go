// Champs d'adresses en pastilles (À, Cc, Cci, Répondre à) : une adresse validée devient une pastille ; Entrée, Tab, virgule,
// point-virgule ou la sortie du champ valident la saisie ; Retour arrière retire la dernière pastille ; double-clic
// sur une pastille la remet en saisie ; on peut glisser une pastille d'un champ à l'autre. Le champ d'origine reste
// la source de vérité (il garde la liste « Nom <adresse>, … » envoyée au serveur) : sans JavaScript, il reste utilisable.
(function () {
  "use strict";
  var ids = ["cp-to", "cp-cc", "cp-bcc", "cp-replyto"];
  var EMAIL = /^[^\s@<>,;"]+@[^\s@<>,;"]+\.[^\s@<>,;"]+$/;
  var boxes = [];
  var dragged = null;

  // Découpe une saisie en adresses (séparateurs , ; et fin de ligne, hors guillemets et chevrons).
  function split(text) {
    var out = [], cur = "", q = false, a = false;
    for (var i = 0; i < text.length; i++) {
      var c = text.charAt(i);
      if (c === "\\" && q) { cur += c + (text.charAt(++i) || ""); continue; }
      if (c === '"') q = !q;
      else if (!q && c === "<") a = true;
      else if (!q && c === ">") a = false;
      if (!q && !a && (c === "," || c === ";" || c === "\n")) { out.push(cur); cur = ""; continue; }
      cur += c;
    }
    out.push(cur);
    return out;
  }
  function parse(raw) {
    raw = raw.trim();
    if (!raw) return null;
    var m = /^(?:"((?:[^"\\]|\\.)*)"|([^<]*?))\s*<([^<>]+)>$/.exec(raw);
    if (m) {
      var name = (m[1] !== undefined ? m[1].replace(/\\(.)/g, "$1") : m[2]).trim(), mail = m[3].trim();
      return { raw: raw, name: name, mail: mail, ok: EMAIL.test(mail) };
    }
    return { raw: raw, name: "", mail: raw, ok: EMAIL.test(raw) };
  }

  function enhance(orig) {
    var box = document.createElement("div");
    box.className = "chips-box";
    var input = document.createElement("input");
    input.type = "text"; input.className = "chips-in";
    input.setAttribute("autocomplete", "off"); input.setAttribute("autocapitalize", "off"); input.spellcheck = false;
    if (orig.autofocus) input.autofocus = true;
    var label = document.querySelector('label[for="' + orig.id + '"]');
    if (label) label.setAttribute("for", orig.id + "-in");
    input.id = orig.id + "-in";
    box.appendChild(input);
    orig.parentNode.insertBefore(box, orig);
    orig.hidden = true; orig.tabIndex = -1;
    orig.chipsInput = input;
    var api = { box: box, input: input, orig: orig };
    boxes.push(api);

    function sync() {
      var raws = [];
      box.querySelectorAll(".chip").forEach(function (c) { raws.push(c.getAttribute("data-raw")); });
      orig.value = raws.join(", ");
      orig.dispatchEvent(new Event("input", { bubbles: true }));
    }
    function addChip(raw, before) {
      var p = parse(raw);
      if (!p) return;
      var chip = document.createElement("span");
      chip.className = "chip" + (p.ok ? "" : " bad");
      chip.setAttribute("data-raw", p.raw);
      chip.title = p.name ? p.name + " <" + p.mail + ">" : p.mail;
      chip.draggable = true;
      var t = document.createElement("span"); t.className = "chip-t"; t.textContent = p.name || p.mail;
      var x = document.createElement("button"); x.type = "button"; x.className = "chip-x"; x.textContent = "×"; x.tabIndex = -1;
      x.setAttribute("aria-label", "×");
      x.addEventListener("click", function () { chip.remove(); sync(); input.focus(); });
      chip.addEventListener("dblclick", function () { input.value = p.raw; chip.remove(); sync(); input.focus(); });
      chip.addEventListener("dragstart", function (e) {
        dragged = { chip: chip, raw: p.raw };
        if (e.dataTransfer) { e.dataTransfer.effectAllowed = "move"; try { e.dataTransfer.setData("text/plain", p.raw); } catch (err) { /* ignoré */ } }
      });
      chip.addEventListener("dragend", function () { dragged = null; });
      chip.appendChild(t); chip.appendChild(x);
      box.insertBefore(chip, before || input);
    }
    api.add = function (raw) { addChip(raw); sync(); };
    api.load = function (text) {
      box.querySelectorAll(".chip").forEach(function (c) { c.remove(); });
      split(text).forEach(function (p) { addChip(p); });
    };
    function commit() { // pendant la frappe : les parties terminées par un séparateur deviennent des pastilles
      var parts = split(input.value);
      var rest = parts.pop();
      parts.forEach(function (p) { addChip(p); });
      input.value = rest.trim() === "" ? "" : rest;
      sync();
    }
    function commitAll() {
      var parts = split(input.value);
      parts.forEach(function (p) { addChip(p); });
      input.value = "";
      sync();
    }

    input.addEventListener("input", function () {
      if (/[,;\n]/.test(input.value) && split(input.value).length > 1) commit();
    });
    input.addEventListener("keydown", function (e) {
      // Une suggestion choisie (flèches) ou seule proposée est validée par contacts-ac.js, pas ici.
      var list = box.querySelector(".ac-list");
      if ((e.key === "Enter" || e.key === "Tab") && list && !list.hidden && (list.querySelector("li.sel") || list.children.length === 1)) return;
      if (e.key === "Enter" || (e.key === "Tab" && !e.shiftKey && input.value.trim() !== "")) {
        if (input.value.trim() !== "") { e.preventDefault(); commitAll(); }
        else if (e.key === "Enter") e.preventDefault(); // Entrée ne doit pas envoyer le formulaire depuis ce champ
      } else if (e.key === "Backspace" && input.value === "") {
        var chips = box.querySelectorAll(".chip");
        if (chips.length) { chips[chips.length - 1].remove(); sync(); }
      }
    });
    input.addEventListener("blur", function () { if (input.value.trim() !== "") commitAll(); });
    box.addEventListener("click", function (e) { if (e.target === box) input.focus(); });
    box.addEventListener("dragover", function (e) { if (dragged) { e.preventDefault(); box.classList.add("over"); } });
    box.addEventListener("dragleave", function () { box.classList.remove("over"); });
    box.addEventListener("drop", function (e) {
      box.classList.remove("over");
      if (!dragged) return;
      e.preventDefault();
      var src = dragged; dragged = null;
      if (src.chip.parentNode === box) return;
      var from = boxes.filter(function (b) { return b.box === src.chip.parentNode; })[0];
      src.chip.remove();
      if (from) from.sync();
      api.add(src.raw);
    });
    api.sync = sync;
    api.load(orig.value);
    // Le formulaire envoyé par Entrée ailleurs doit contenir la saisie en cours.
    orig.form && orig.form.addEventListener("submit", function () { if (input.value.trim() !== "") commitAll(); }, true);
    orig.form && orig.form.addEventListener("formdata", function () { if (input.value.trim() !== "") commitAll(); });
  }

  ids.forEach(function (id) { var el = document.getElementById(id); if (el) enhance(el); });
})();
