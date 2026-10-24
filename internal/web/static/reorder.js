// Réordonner par glisser-déposer les listes de comptes mail, d'identités et de modèles de message.
// Le nouvel ordre complet part en POST (jeton CSRF compris) ; le serveur reste seul juge de ce qu'il accepte.
// Sans JavaScript rien ne change : les flèches ▲ ▼ de chaque ligne restent le moyen de réordonner.
(function () {
  "use strict";
  var lists = document.querySelectorAll("[data-reorder]");
  if (!lists.length) return;

  // Chaque liste déclare son genre ; l'adresse d'envoi n'est jamais prise dans la page.
  var endpoints = {
    accounts: "/accounts/reorder",
    identities: "/settings/identities/reorder",
    templates: "/settings/templates/reorder"
  };

  // Les textes qui n'ont de sens qu'avec le script actif.
  document.querySelectorAll("[data-reorder-hint]").forEach(function (el) { el.hidden = false; });

  // rows : les lignes réordonnables de la liste (celles qui portent un identifiant ; l'identité
  // principale, par exemple, n'en a pas et ne bouge donc jamais).
  function rows(list) {
    return Array.prototype.filter.call(list.children, function (el) {
      return el.nodeType === 1 && el.hasAttribute("data-id");
    });
  }

  function order(list) {
    return rows(list).map(function (el) { return el.getAttribute("data-id"); }).join(",");
  }

  function setup(list) {
    var url = endpoints[list.getAttribute("data-reorder")];
    if (!url) return;
    var box = list.getAttribute("data-box") || ""; // boîte visée (identités)
    var errBox = document.querySelector("[data-reorder-error]");
    var dragged = null;
    var saved = order(list); // dernier ordre accepté par le serveur

    rows(list).forEach(function (r) { r.setAttribute("draggable", "true"); });

    function clear() {
      rows(list).forEach(function (r) { r.classList.remove("drop-over", "dragging"); });
    }

    function rowAt(e) {
      var el = e.target && e.target.closest ? e.target.closest("[data-id]") : null;
      return el && el.parentNode === list ? el : null;
    }

    list.addEventListener("dragstart", function (e) {
      var row = rowAt(e);
      if (!row) return;
      // Un lien, un bouton ou un champ garde son comportement propre : on ne déplace pas la ligne depuis lui.
      if (e.target.closest && e.target.closest("a, button, input, textarea, select, summary")) {
        e.preventDefault();
        return;
      }
      dragged = row;
      row.classList.add("dragging");
      if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = "move";
        try { e.dataTransfer.setData("text/plain", row.getAttribute("data-id")); } catch (err) { /* ignoré */ }
      }
    });

    list.addEventListener("dragover", function (e) {
      if (!dragged) return;
      var row = rowAt(e);
      if (!row || row === dragged) return;
      e.preventDefault(); // sans cela le navigateur refuse le dépôt
      if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
      rows(list).forEach(function (r) { if (r !== row) r.classList.remove("drop-over"); });
      row.classList.add("drop-over");
    });

    list.addEventListener("dragleave", function (e) {
      var row = rowAt(e);
      if (row) row.classList.remove("drop-over");
    });

    list.addEventListener("drop", function (e) {
      if (!dragged) return;
      e.preventDefault();
      var row = rowAt(e), moved = dragged;
      dragged = null;
      clear();
      if (!row || row === moved) return;
      // Déposé sur la moitié basse d'une ligne : la ligne glissée passe après elle.
      var r = row.getBoundingClientRect ? row.getBoundingClientRect() : { top: 0, height: 0 };
      if (e.clientY > r.top + r.height / 2) list.insertBefore(moved, row.nextSibling);
      else list.insertBefore(moved, row);
      save();
    });

    list.addEventListener("dragend", function () { dragged = null; clear(); });

    // restore remet les lignes dans l'ordre donné (enregistrement refusé ou réseau coupé).
    function restore(snapshot) {
      var byID = {};
      rows(list).forEach(function (r) { byID[r.getAttribute("data-id")] = r; });
      snapshot.split(",").forEach(function (id) {
        if (byID[id]) list.appendChild(byID[id]);
      });
    }

    function save() {
      var now = order(list);
      if (now === saved) return;
      var snapshot = saved;
      if (errBox) errBox.hidden = true;
      var body = "csrf=" + encodeURIComponent(list.getAttribute("data-csrf") || "") +
        "&order=" + encodeURIComponent(now);
      if (box) body += "&box=" + encodeURIComponent(box);
      fetch(url, {
        method: "POST",
        body: body,
        credentials: "same-origin",
        headers: { "X-Requested-With": "fetch", "Content-Type": "application/x-www-form-urlencoded" }
      }).then(function (resp) {
        if (!resp.ok) throw new Error("refusé : " + resp.status);
        saved = now;
      }).catch(function () {
        restore(snapshot);
        if (errBox) errBox.hidden = false;
      });
    }
  }

  Array.prototype.forEach.call(lists, setup);
})();
