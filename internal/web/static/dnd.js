// Glisser-déposer : un message (ou la sélection) glissé sur un dossier du panneau de gauche est déplacé ; avec Ctrl
// (Option sur Mac) il est copié. Un dossier replié se déplie quand on le survole un instant.
// Le déplacement passe par le formulaire des actions groupées : le serveur contrôle tout, comme pour les boutons.
(function () {
  "use strict";
  var form = document.querySelector("form.msglist-form");
  if (!form) return;
  var targets = document.querySelectorAll("[data-drop]");
  if (!targets.length) return;

  var dragging = false, badge = null, dragged = null;

  function rows() { return form.querySelectorAll(".msgrow"); }
  rows().forEach(function (r) { r.setAttribute("draggable", "true"); });

  // Pastille du nombre de messages glissés, collée au curseur (RainLoop affiche un calque rayé avec le compte).
  function showBadge(n, x, y) {
    if (!badge) {
      badge = document.createElement("div");
      badge.className = "dnd-count";
      badge.setAttribute("aria-hidden", "true");
      document.body.appendChild(badge);
    }
    badge.textContent = String(n);
    moveBadge(x, y);
  }
  function moveBadge(x, y) {
    if (!badge) return;
    badge.style.left = (x || 0) + "px";
    badge.style.top = (y || 0) + "px";
  }
  function hideBadge() {
    if (badge && badge.parentNode) badge.parentNode.removeChild(badge);
    badge = null;
  }
  document.addEventListener("dragover", function (e) { if (dragging) moveBadge(e.clientX, e.clientY); });

  function stop() {
    dragging = false;
    document.body.classList.remove("dnd");
    if (dragged) { dragged.classList.remove("dragged"); dragged = null; }
    hideBadge();
    targets.forEach(function (t) { t.classList.remove("drop-over"); });
  }

  form.addEventListener("dragstart", function (e) {
    var row = e.target && e.target.closest ? e.target.closest(".msgrow") : null;
    if (!row) return;
    var box = row.querySelector("input[name=uid]");
    if (box && !box.checked) { // glisser un message hors sélection : seul celui-là part
      form.querySelectorAll("input[name=uid]:checked").forEach(function (c) { c.checked = false; });
      box.checked = true;
      form.dispatchEvent(new Event("click", { bubbles: true }));
      document.dispatchEvent(new Event("wmail:pane"));
    }
    var n = form.querySelectorAll("input[name=uid]:checked").length;
    dragging = true;
    document.body.classList.add("dnd");
    dragged = row;
    row.classList.add("dragged");
    showBadge(n, e.clientX, e.clientY);
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "copyMove";
      try { e.dataTransfer.setData("text/plain", String(n)); } catch (err) { /* ignoré */ }
    }
  });
  form.addEventListener("dragend", stop);

  targets.forEach(function (t) {
    var timer = null;
    t.addEventListener("dragover", function (e) {
      if (!dragging) return;
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = e.ctrlKey || e.altKey ? "copy" : "move";
      t.classList.add("drop-over");
    });
    t.addEventListener("dragenter", function () {
      if (!dragging) return;
      t.classList.add("drop-over");
      var caret = t.querySelector("[data-caret]");
      if (caret && t.getAttribute("data-open") === "0") {
        clearTimeout(timer);
        timer = setTimeout(function () { caret.click(); }, 500);
      }
    });
    t.addEventListener("dragleave", function () { t.classList.remove("drop-over"); clearTimeout(timer); });
    t.addEventListener("drop", function (e) {
      if (!dragging) return;
      e.preventDefault();
      clearTimeout(timer);
      t.classList.remove("drop-over");
      var copy = e.ctrlKey || e.altKey;
      var folder = t.getAttribute("data-drop");
      if (!form.querySelector("input[name=uid]:checked")) return;
      if (t.classList.contains("active") && !copy) return; // déjà dans ce dossier
      var h = document.createElement("input");
      h.type = "hidden"; h.name = copy ? "copy_to" : "move_to"; h.value = folder;
      form.appendChild(h);
      stop();
      t.classList.add("drop-pulse"); // courte pulsation du dossier, visible le temps du rechargement
      form.submit();
    });
  });
})();
