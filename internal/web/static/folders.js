// Panneau des dossiers : niveaux de l'arbre repliables, panneau masquable, création d'un dossier sans quitter la page.
(function () {
  "use strict";
  var tree = document.getElementById("foldertree");
  var box = document.querySelector(".mailbox");
  var csrf = box ? box.getAttribute("data-csrf") : "";

  function post(url, data) {
    var body = new URLSearchParams(data); body.set("csrf", csrf);
    return fetch(url, { method: "POST", body: body, credentials: "same-origin", headers: { "X-Requested-With": "fetch" } }).catch(function () { /* hors ligne : l'état reste appliqué jusqu'au rechargement */ });
  }

  // --- arbre ---
  document.querySelectorAll("[data-js-only]").forEach(function (el) { if (el.closest(".folders")) el.hidden = false; });
  function refresh() {
    var openAt = {};
    Array.prototype.forEach.call(tree.children, function (li) {
      var d = parseInt(li.getAttribute("data-depth") || "0", 10), visible = true;
      for (var k = 0; k < d; k++) if (!openAt[k]) { visible = false; break; }
      li.hidden = !visible;
      var open = li.getAttribute("data-open");
      openAt[d] = open === "1";
      var caret = li.querySelector("[data-caret]");
      if (caret) {
        caret.setAttribute("aria-expanded", open === "1" ? "true" : "false");
        caret.firstChild.textContent = open === "1" ? "▾" : "▸";
      }
    });
  }
  if (tree) {
    refresh();
    tree.addEventListener("click", function (e) {
      var caret = e.target.closest && e.target.closest("[data-caret]");
      if (!caret) return;
      e.preventDefault();
      var li = caret.closest("li"), open = li.getAttribute("data-open") !== "1";
      li.setAttribute("data-open", open ? "1" : "0");
      refresh();
      post("/settings/folder-open", { folder: li.getAttribute("data-key"), open: open ? "1" : "0" });
    });
  }

  // --- panneau masqué / affiché ---
  document.querySelectorAll("[data-folders-toggle]").forEach(function (b) {
    b.hidden = false;
    b.setAttribute("aria-pressed", document.body.classList.contains("folders-collapsed") ? "true" : "false");
    b.addEventListener("click", function () {
      var hidden = document.body.classList.toggle("folders-collapsed");
      b.setAttribute("aria-pressed", hidden ? "true" : "false");
      post("/settings/panes", { hidden: hidden ? "1" : "0" });
    });
  });

  // --- création d'un dossier ---
  var dlg = document.getElementById("newfolder");
  function show(on) {
    if (!dlg) return;
    dlg.hidden = !on;
    if (on) { var i = dlg.querySelector("input[name=new_name]"); if (i) i.focus(); }
  }
  document.querySelectorAll("[data-newfolder]").forEach(function (a) {
    a.addEventListener("click", function (e) { if (dlg) { e.preventDefault(); show(true); } });
  });
  document.querySelectorAll("[data-newfolder-close]").forEach(function (b) { b.addEventListener("click", function () { show(false); }); });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && dlg && !dlg.hidden) { e.preventDefault(); show(false); }
  });
})();
