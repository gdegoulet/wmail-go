// Impression automatique de la version imprimable (le bouton reste disponible).
(function () {
  "use strict";
  var b = document.getElementById("print-btn");
  if (b) b.addEventListener("click", function () { window.print(); });
  window.addEventListener("load", function () { setTimeout(function () { window.print(); }, 300); });
})();
