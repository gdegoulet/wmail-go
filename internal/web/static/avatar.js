// Photo du correspondant : si le serveur n'en a pas, l'image est retirée pour laisser la silhouette.
// (Un attribut onerror= serait refusé par la politique de sécurité du contenu.)
(function () {
  "use strict";
  document.addEventListener("error", function (e) {
    var el = e.target;
    if (el && el.tagName === "IMG" && el.hasAttribute("data-avatar")) el.remove();
  }, true); // phase de capture : l'événement « error » d'une ressource ne remonte pas
})();
