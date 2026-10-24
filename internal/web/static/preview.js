// Aperçu des pièces jointes : images et PDF s'ouvrent dans une visionneuse (flèches ou boutons pour passer de l'une
// à l'autre, Échap pour fermer) ; les textes s'ouvrent dans un nouvel onglet (comportement du lien).
(function () {
  "use strict";
  if (typeof HTMLDialogElement === "undefined") return;
  var dlg = null, img, frame, caption, list = [], pos = 0;

  function build() {
    dlg = document.createElement("dialog");
    dlg.className = "viewer";
    img = document.createElement("img");
    img.alt = "";
    frame = document.createElement("iframe");
    frame.className = "v-pdf";
    frame.setAttribute("title", "PDF");
    frame.hidden = true;
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
    [prev, img, frame, next, caption, close].forEach(function (el) { dlg.appendChild(el); });
    document.body.appendChild(dlg);
  }
  function show() {
    var a = list[pos], pdf = a.getAttribute("data-kind") === "pdf";
    img.hidden = pdf;
    frame.hidden = !pdf;
    if (pdf) { frame.src = a.getAttribute("href"); img.removeAttribute("src"); }
    else { img.src = a.getAttribute("href"); frame.removeAttribute("src"); }
    caption.textContent = a.getAttribute("data-name") + " (" + (pos + 1) + "/" + list.length + ")";
  }
  function go(d) { pos = (pos + d + list.length) % list.length; show(); }

  // Sélection de pièces jointes : les cases cochées alimentent le lien « Télécharger la sélection (ZIP) ».
  document.addEventListener("change", function (e) {
    if (!e.target || !e.target.classList || !e.target.classList.contains("att-check")) return;
    var link = document.querySelector("[data-zip-selected]");
    if (!link) return;
    var picked = [].slice.call(document.querySelectorAll(".att-check:checked")).map(function (c) { return c.value; });
    var base = link.getAttribute("data-base") || link.getAttribute("href");
    link.setAttribute("data-base", base);
    link.hidden = picked.length === 0;
    link.setAttribute("href", base + picked.map(function (i) { return "&index=" + encodeURIComponent(i); }).join(""));
  });
  document.querySelectorAll(".att-pick").forEach(function (el) { el.hidden = false; });

  document.addEventListener("click", function (e) {
    var a = e.target && e.target.closest ? e.target.closest("a.preview[data-kind=image], a.preview[data-kind=pdf]") : null;
    if (!a || e.ctrlKey || e.metaKey || e.shiftKey) return;
    e.preventDefault();
    if (!dlg) build();
    var sel = "a.preview[data-kind=image], a.preview[data-kind=pdf]";
    var all = a.closest(".atts") ? a.closest(".atts").querySelectorAll(sel) : [a];
    list = Array.prototype.slice.call(all);
    pos = list.indexOf(a);
    show();
    dlg.showModal();
  });
})();
