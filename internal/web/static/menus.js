// Menus déroulants (<details data-dropdown>) : un seul ouvert à la fois ; un clic ailleurs ou Échap les ferme.
(function () {
  "use strict";
  var menus = function () { return Array.prototype.slice.call(document.querySelectorAll("details[data-dropdown]")); };
  document.addEventListener("toggle", function (e) {
    var d = e.target;
    if (!d || !d.matches || !d.matches("details[data-dropdown]") || !d.open) return;
    menus().forEach(function (o) { if (o !== d) o.open = false; });
  }, true);
  document.addEventListener("click", function (e) {
    menus().forEach(function (d) { if (d.open && !d.contains(e.target)) d.open = false; });
  });
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    menus().forEach(function (d) { if (d.open) { d.open = false; var s = d.querySelector("summary"); if (s) s.focus(); } });
  });
  // Un bouton du menu (action) referme le menu avant l'envoi du formulaire.
  document.addEventListener("submit", function () { menus().forEach(function (d) { d.open = false; }); });
})();
