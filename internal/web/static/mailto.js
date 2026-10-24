// Déclare wmail-go comme gestionnaire des liens mailto: du navigateur (registerProtocolHandler, connexion sécurisée requise).
(function () {
  "use strict";
  var btn = document.getElementById("mailto-register");
  if (!btn || !navigator.registerProtocolHandler) return;
  btn.closest("p").hidden = false;
  btn.addEventListener("click", function () {
    var note = document.getElementById("mailto-note");
    try {
      navigator.registerProtocolHandler("mailto", window.location.origin + btn.getAttribute("data-uri"));
    } catch (e) {
      if (note) note.textContent = String(e && e.message ? e.message : e);
    }
  });
})();
