// Lecture en plein écran : le message occupe toute la page (le bouton de l'en-tête ou la touche Entrée dans le volet de
// lecture) ; Échap revient à la disposition normale.
(function () {
  "use strict";
  var root = document.body;
  function active() { return root.classList.contains("msg-fullscreen"); }
  function set(on) { root.classList.toggle("msg-fullscreen", on); }

  document.addEventListener("click", function (e) {
    var b = e.target && e.target.closest ? e.target.closest("[data-fullscreen-msg]") : null;
    if (b) { e.preventDefault(); set(!active()); }
  });
  // Phase de capture : quitter le plein écran passe avant la fermeture du message (keys.js).
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && active()) { set(false); e.preventDefault(); e.stopImmediatePropagation(); }
  }, true);
  // Une autre page de message (clic dans la liste) ne garde pas le plein écran.
  document.addEventListener("wmail:pane", function () { set(false); });
})();
