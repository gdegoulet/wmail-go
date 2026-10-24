// Autocomplétion des destinataires (carnet d'adresses) et sélecteur de contacts de la page de composition.
(function () {
  "use strict";
  // Avec les pastilles (chips.js), la saisie se fait dans un champ visible distinct du champ d'origine.
  var fields = ["cp-to", "cp-cc", "cp-bcc", "cp-replyto"].map(function (id) { var el = document.getElementById(id); return el && (el.chipsInput || el); }).filter(Boolean);
  if (!fields.length) return;

  function tokenStart(v, pos) {
    var inq = false, start = 0;
    for (var i = 0; i < pos; i++) {
      var c = v.charAt(i);
      if (c === "\\" && inq) { i++; continue; }
      if (c === '"') inq = !inq;
      else if (!inq && (c === "," || c === ";")) start = i + 1;
    }
    return start;
  }

  function attach(input) {
    var box = document.createElement("ul");
    box.className = "ac-list";
    box.setAttribute("role", "listbox");
    box.hidden = true;
    input.parentNode.appendChild(box);
    var items = [], sel = -1, timer = null, seq = 0;

    function close() { box.hidden = true; sel = -1; items = []; }
    function mark() {
      Array.prototype.forEach.call(box.children, function (li, i) { li.classList.toggle("sel", i === sel); });
    }
    function pick(i) {
      var it = items[i];
      if (!it) return;
      var v = input.value, pos = input.selectionStart == null ? v.length : input.selectionStart;
      var from = tokenStart(v, pos), rest = v.slice(pos);
      var head = v.slice(0, from).replace(/\s+$/, "");
      head = head ? head + " " : "";
      input.value = head + it.label + ", " + rest.replace(/^\s*[,;]?\s*/, "");
      var np = (head + it.label + ", ").length;
      input.setSelectionRange(np, np);
      input.dispatchEvent(new Event("input", { bubbles: true }));
      close();
    }
    function show(list) {
      items = list; sel = -1;
      box.textContent = "";
      list.forEach(function (it, i) {
        var li = document.createElement("li");
        li.setAttribute("role", "option");
        var b = document.createElement("b"); b.textContent = it.name || it.email;
        li.appendChild(b);
        if (it.name) { var s = document.createElement("span"); s.textContent = " <" + it.email + ">"; li.appendChild(s); }
        li.addEventListener("mousedown", function (e) { e.preventDefault(); pick(i); });
        box.appendChild(li);
      });
      box.hidden = !list.length;
    }
    function query() {
      var v = input.value, pos = input.selectionStart == null ? v.length : input.selectionStart;
      var q = v.slice(tokenStart(v, pos), pos).trim();
      if (!q) { close(); return; }
      var my = ++seq;
      fetch("/contacts/suggest?q=" + encodeURIComponent(q), { credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
        .then(function (r) { return r.ok ? r.json() : []; })
        .then(function (l) { if (my === seq) show(l); })
        .catch(function () { close(); });
    }
    input.addEventListener("input", function () { clearTimeout(timer); timer = setTimeout(query, 250); });
    input.addEventListener("blur", function () { setTimeout(close, 100); });
    input.addEventListener("keydown", function (e) {
      if (box.hidden) return;
      if (e.key === "ArrowDown") { e.preventDefault(); sel = (sel + 1) % items.length; mark(); }
      else if (e.key === "ArrowUp") { e.preventDefault(); sel = (sel <= 0 ? items.length : sel) - 1; mark(); }
      else if ((e.key === "Enter" || e.key === "Tab") && sel >= 0) { e.preventDefault(); pick(sel); }
      else if (e.key === "Enter" && items.length === 1) { e.preventDefault(); pick(0); }
      else if (e.key === "Escape") { e.stopPropagation(); close(); }
    });
  }
  fields.forEach(attach);

  // --- sélecteur de contacts ---
  var btn = document.getElementById("cp-pick"), to = fields[0] && fields[0].id.indexOf("cp-to") === 0 ? fields[0] : document.getElementById("cp-to");
  if (!btn || !to || typeof HTMLDialogElement === "undefined") return;
  var dlg = document.createElement("dialog");
  dlg.className = "picker";
  var h = document.createElement("h2"); h.textContent = btn.getAttribute("data-title");
  var q = document.createElement("input"); q.type = "search"; q.placeholder = btn.getAttribute("data-search"); q.maxLength = 100;
  var ul = document.createElement("ul"); ul.className = "picker-list";
  var close = document.createElement("button"); close.type = "button"; close.textContent = btn.getAttribute("data-close");
  dlg.appendChild(h); dlg.appendChild(q); dlg.appendChild(ul); dlg.appendChild(close);
  document.body.appendChild(dlg);
  close.addEventListener("click", function () { dlg.close(); });
  var t = null;
  function load() {
    fetch("/contacts/suggest?all=1&q=" + encodeURIComponent(q.value), { credentials: "same-origin" })
      .then(function (r) { return r.ok ? r.json() : []; })
      .then(function (list) {
        ul.textContent = "";
        list.forEach(function (it) {
          var li = document.createElement("li"), a = document.createElement("button");
          a.type = "button"; a.className = "link";
          a.textContent = (it.name ? it.name + " " : "") + "<" + it.email + ">";
          a.addEventListener("click", function () {
            var v = to.value.trim();
            to.value = (v && !/[,;]\s*$/.test(v) ? v + ", " : v ? v + " " : "") + it.label + ", ";
            to.dispatchEvent(new Event("input", { bubbles: true }));
            a.disabled = true;
          });
          li.appendChild(a); ul.appendChild(li);
        });
      });
  }
  q.addEventListener("input", function () { clearTimeout(t); t = setTimeout(load, 250); });
  btn.hidden = false;
  btn.addEventListener("click", function () { q.value = ""; load(); dlg.showModal(); q.focus(); });
})();
