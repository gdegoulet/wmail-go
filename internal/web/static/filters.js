// Formulaire d'un filtre : lignes de condition à volonté, champs de l'action selon le choix.
(function () {
  "use strict";
  var form = document.getElementById("filter-form");
  if (!form) return;
  document.querySelectorAll("[data-js-only]").forEach(function (el) { el.hidden = false; });
  var conds = document.getElementById("conds");

  function syncRow(row) {
    var field = row.querySelector("[name=cond_field]").value;
    row.querySelector(".second").hidden = field !== "Header";
    var type = row.querySelector("[name=cond_type]");
    // Over/Under pour la taille seulement ; les autres types pour les autres champs
    Array.prototype.forEach.call(type.options, function (o) {
      var sizeType = o.value === "Over" || o.value === "Under";
      o.hidden = o.disabled = (field === "Size") !== sizeType;
    });
    if (type.options[type.selectedIndex].disabled) {
      for (var i = 0; i < type.options.length; i++) if (!type.options[i].disabled) { type.selectedIndex = i; break; }
    }
  }
  function wire(row) {
    row.querySelector("[name=cond_field]").addEventListener("change", function () { syncRow(row); });
    var rm = row.querySelector(".rm");
    if (rm) rm.addEventListener("click", function () { if (conds.children.length > 1) row.remove(); else row.querySelector("[name=cond_value]").value = ""; });
    syncRow(row);
  }
  Array.prototype.forEach.call(conds.children, wire);
  document.getElementById("add-cond").addEventListener("click", function () {
    var last = conds.lastElementChild, row = last.cloneNode(true);
    row.querySelector("[name=cond_value]").value = "";
    row.querySelector("[name=cond_second]").value = "";
    conds.appendChild(row);
    row.querySelector(".rm").hidden = false;
    wire(row);
    row.querySelector("[name=cond_value]").focus();
  });

  var action = document.getElementById("action");
  function syncAction() {
    var v = action.value;
    form.querySelectorAll(".act").forEach(function (d) { d.hidden = d.getAttribute("data-act") !== v; });
    var mark = form.querySelector(".mark");
    mark.hidden = !(v === "None" || v === "MoveTo" || v === "Forward");
  }
  action.addEventListener("change", syncAction);
  syncAction();
})();
