// Photo d'un contact : le fichier choisi est réduit à 128 pixels et enregistré avec la fiche (aucun envoi ailleurs).
(function () {
  "use strict";
  var field = document.getElementById("c-photo"), file = document.getElementById("c-photo-file");
  if (!field || !file || !window.FileReader) return;
  document.querySelectorAll("[data-js-only]").forEach(function (el) { el.hidden = false; });
  var clear = document.getElementById("c-photo-clear");
  if (clear) clear.addEventListener("click", function () {
    field.value = "";
    var p = clear.closest(".c-photo");
    if (p) p.remove();
  });
  file.addEventListener("change", function () {
    var f = file.files && file.files[0];
    if (!f) return;
    var r = new FileReader();
    r.onload = function () {
      var img = new Image();
      img.onload = function () {
        var size = 128, c = document.createElement("canvas");
        c.width = c.height = size;
        var g = c.getContext("2d");
        var side = Math.min(img.width, img.height);
        g.drawImage(img, (img.width - side) / 2, (img.height - side) / 2, side, side, 0, 0, size, size);
        field.value = c.toDataURL("image/png");
        var prev = document.querySelector(".c-photo img");
        if (prev) prev.src = field.value;
      };
      img.src = r.result;
    };
    r.readAsDataURL(f);
  });
})();
