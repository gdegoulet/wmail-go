// Photo d'un contact : le fichier choisi est réduit à 128 pixels et enregistré avec la fiche (aucun envoi ailleurs).
(function () {
  "use strict";
  var field = document.getElementById("c-photo"), file = document.getElementById("c-photo-file");
  if (!field || !file || !window.FileReader) return;
  document.querySelectorAll("[data-js-only]").forEach(function (el) { el.hidden = false; });
  function brancherRetrait(bouton) {
    bouton.addEventListener("click", function () {
      field.value = "";
      var p = bouton.closest(".c-photo");
      if (p) p.remove();
    });
  }
  var clear = document.getElementById("c-photo-clear");
  if (clear) brancherRetrait(clear);

  // Crée la zone d'aperçu quand le contact n'avait pas encore de photo : le gabarit ne la rend que s'il y
  // en a une, et sans elle le choix d'un fichier ne se voyait nulle part.
  function apercu() {
    var img = document.querySelector(".c-photo img");
    if (img) return img;
    var p = document.createElement("p");
    p.className = "c-photo";
    img = document.createElement("img");
    img.alt = "";
    img.width = img.height = 64;
    var bouton = document.createElement("button");
    bouton.type = "button";
    bouton.className = "link";
    bouton.textContent = file.closest("label").dataset.removeLabel || "";
    p.appendChild(img);
    p.appendChild(document.createTextNode(" "));
    if (bouton.textContent) {
      p.appendChild(bouton);
      brancherRetrait(bouton);
    }
    file.closest("label").insertAdjacentElement("afterend", p);
    return img;
  }
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
        apercu().src = field.value;
      };
      img.src = r.result;
    };
    r.readAsDataURL(f);
  });
})();
