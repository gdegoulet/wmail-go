// Version imprimable : impression automatique (le bouton reste disponible) et adresse des liens sur le papier.
(function () {
  "use strict";

  // Sur le papier, l'adresse d'un lien est ajoutée entre parenthèses (print.css). Inutile quand le texte du lien
  // est déjà cette adresse : on marque ces liens pour que la feuille de style les laisse tranquilles.
  function markLinks() {
    var links = document.querySelectorAll('.body a[href]');
    Array.prototype.forEach.call(links, function (a) {
      var href = a.getAttribute("href") || "";
      var text = (a.textContent || "").replace(/\s+/g, " ").trim();
      if (text === "") return;
      var bare = href.replace(/^(https?:\/\/|mailto:)/i, "").replace(/\/$/, "");
      var shown = text.replace(/^(https?:\/\/|mailto:)/i, "").replace(/\/$/, "");
      if (shown.toLowerCase() === bare.toLowerCase() || text.indexOf(href) >= 0) {
        a.setAttribute("data-same", "");
      }
    });
  }
  markLinks();

  var b = document.getElementById("print-btn");
  if (b) b.addEventListener("click", function () { window.print(); });
  window.addEventListener("load", function () { setTimeout(function () { window.print(); }, 300); });
})();
