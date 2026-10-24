// Aperçu des pièces jointes : les images s'ouvrent dans une visionneuse (flèches ou boutons pour passer de l'une à
// l'autre, Échap pour fermer) ; les textes s'ouvrent dans un nouvel onglet (comportement du lien).
(function () {
  "use strict";
  if (typeof HTMLDialogElement === "undefined") return;
  var dlg = null, img, caption, list = [], pos = 0;

  function build() {
    dlg = document.createElement("dialog");
    dlg.className = "viewer";
    img = document.createElement("img");
    img.alt = "";
    caption = document.createElement("p");
    var prev = document.createElement("button"), next = document.createElement("button"), close = document.createElement("button");
    prev.type = next.type = close.type = "button";
    prev.textContent = "‹"; next.textContent = "›"; close.textContent = "×";
    prev.className = "v-prev"; next.className = "v-next"; close.className = "v-close";
    prev.setAttribute("aria-label", "←"); next.setAttribute("aria-label", "→"); close.setAttribute("aria-label", "×");
    prev.addEventListener("click", function () { go(-1); });
    next.addEventListener("click", function () { go(1); });
    close.addEventListener("click", function () { dlg.close(); });
    dlg.addEventListener("click", function (e) { if (e.target === dlg) dlg.close(); });
    dlg.addEventListener("keydown", function (e) {
      if (e.key === "ArrowLeft") { go(-1); e.preventDefault(); } else if (e.key === "ArrowRight") { go(1); e.preventDefault(); }
    });
    [prev, img, next, caption, close].forEach(function (el) { dlg.appendChild(el); });
    document.body.appendChild(dlg);
  }
  function show() {
    var a = list[pos];
    img.src = a.getAttribute("href");
    caption.textContent = a.getAttribute("data-name") + " (" + (pos + 1) + "/" + list.length + ")";
  }
  function go(d) { pos = (pos + d + list.length) % list.length; show(); }

  document.addEventListener("click", function (e) {
    var a = e.target && e.target.closest ? e.target.closest("a.preview[data-kind=image]") : null;
    if (!a || e.ctrlKey || e.metaKey || e.shiftKey) return;
    e.preventDefault();
    if (!dlg) build();
    var all = a.closest(".atts") ? a.closest(".atts").querySelectorAll("a.preview[data-kind=image]") : [a];
    list = Array.prototype.slice.call(all);
    pos = list.indexOf(a);
    show();
    dlg.showModal();
  });
})();
