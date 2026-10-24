// Lecture en plein écran : le message occupe toute la page (le bouton « plein écran » de l'en-tête du volet, ou la
// touche Entrée quand le volet de lecture a le focus) ; Échap revient à la disposition normale.
(function () {
  "use strict";
  var root = document.body;
  function active() { return root.classList.contains("msg-fullscreen"); }
  function set(on) { root.classList.toggle("msg-fullscreen", on); }

  // Le volet de lecture est réécrit sans recharger la page (list.js) : le bouton arrive « hidden » à chaque message.
  // Sans cette révélation répétée, il n'apparaissait qu'au tout premier affichage — donc presque jamais.
  function reveal() {
    document.querySelectorAll("[data-fullscreen-msg][hidden]").forEach(function (b) { b.hidden = false; });
  }

  document.addEventListener("click", function (e) {
    var b = e.target && e.target.closest ? e.target.closest("[data-fullscreen-msg]") : null;
    if (b) { e.preventDefault(); set(!active()); }
  });
  // Phase de capture : quitter le plein écran passe avant la fermeture du message (keys.js).
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && active()) { set(false); e.preventDefault(); e.stopImmediatePropagation(); }
  }, true);
  // Entrée dans le volet de lecture (on y arrive avec →) : bascule le plein écran.
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Enter" || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
    var el = document.activeElement;
    if (!el || !el.closest || !el.closest(".readpane")) return;
    if (el.closest("a[href], button, input, textarea, select, [contenteditable]")) return; // Entrée appartient au lien ou au bouton
    if (!document.querySelector(".msgview")) return;
    e.preventDefault();
    set(!active());
  });
  // Une autre page de message (clic dans la liste) ne garde pas le plein écran, et son bouton doit être visible.
  document.addEventListener("wmail:pane", function () { set(false); reveal(); });
  reveal();
})();
