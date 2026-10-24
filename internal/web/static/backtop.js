// Bouton « haut de page » du volet de lecture : apparaît après un défilement, ramène en haut.
// Le volet peut être remplacé sans rechargement (preview.js) : on cherche le bouton à chaque fois.
(function () {
  "use strict";
  var pane = document.querySelector(".readpane");
  if (!pane) return;
  function btn() { return document.querySelector("[data-backtop]"); }
  function update() {
    var b = btn();
    if (b) b.hidden = pane.scrollTop < 200;
  }
  pane.addEventListener("scroll", update);
  document.addEventListener("click", function (e) {
    var b = e.target && e.target.closest ? e.target.closest("[data-backtop]") : null;
    if (!b) return;
    if (pane.scrollTo) pane.scrollTo({ top: 0, behavior: "smooth" }); else pane.scrollTop = 0;
    b.hidden = true;
  });
  document.addEventListener("wmail:pane", update);
  update();
})();
