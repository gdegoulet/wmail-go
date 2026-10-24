// Aperçu des pièces jointes : images et PDF s'ouvrent dans une visionneuse (flèches ou boutons pour passer de l'une
// à l'autre, Échap pour fermer) ; les textes s'ouvrent dans un nouvel onglet (comportement du lien).
(function () {
  "use strict";
  if (typeof HTMLDialogElement === "undefined") return;
  var dlg = null, img, frame, wait, caption, list = [], pos = 0, waitTimer = null;

  function build() {
    dlg = document.createElement("dialog");
    dlg.className = "viewer";
    img = document.createElement("img");
    img.alt = "";
    frame = document.createElement("iframe");
    frame.className = "v-pdf";
    frame.setAttribute("title", "PDF");
    frame.hidden = true;
    wait = document.createElement("div"); // sablier pendant le chargement d'une image
    wait.className = "v-wait";
    wait.hidden = true;
    wait.setAttribute("role", "progressbar");
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
    dlg.addEventListener("close", function () { stopWait(); img.removeAttribute("src"); frame.removeAttribute("src"); });
    img.addEventListener("load", ready);
    img.addEventListener("error", ready); // image illisible : on arrête d'attendre, le navigateur montre l'alternative
    [prev, img, frame, wait, next, caption, close].forEach(function (el) { dlg.appendChild(el); });
    document.body.appendChild(dlg);
  }
  function stopWait() { clearTimeout(waitTimer); waitTimer = null; if (wait) wait.hidden = true; }
  function ready() { stopWait(); img.hidden = false; }

  function show() {
    var a = list[pos], href = a.getAttribute("href"), pdf = a.getAttribute("data-kind") === "pdf";
    stopWait();
    frame.hidden = !pdf;
    caption.textContent = a.getAttribute("data-name") + " (" + (pos + 1) + "/" + list.length + ")";
    if (pdf) { img.hidden = true; img.removeAttribute("src"); frame.src = href; return; }
    frame.removeAttribute("src");
    // L'image précédente disparaît tout de suite : sans cela elle restait affichée pendant le chargement de la suivante.
    img.hidden = true;
    img.removeAttribute("src"); // force l'événement « load » même si l'adresse ne change pas
    img.src = href;
    if (img.complete && img.naturalWidth) { ready(); return; } // déjà en cache : aucun clignotement
    // Le sablier n'apparaît qu'au bout de 150 ms : une petite image arrive avant et ne fait pas clignoter la visionneuse.
    waitTimer = setTimeout(function () { waitTimer = null; wait.hidden = false; }, 150);
  }
  function go(d) { pos = (pos + d + list.length) % list.length; show(); }

  // Sélection de pièces jointes : les cases cochées alimentent le lien « Télécharger la sélection (ZIP) ».
  function syncZip() {
    var link = document.querySelector("[data-zip-selected]");
    if (!link) return;
    var picked = [].slice.call(document.querySelectorAll(".att-check:checked")).map(function (c) { return c.value; });
    var base = link.getAttribute("data-base") || link.getAttribute("href");
    link.setAttribute("data-base", base);
    link.hidden = picked.length === 0;
    link.setAttribute("href", base + picked.map(function (i) { return "&index=" + encodeURIComponent(i); }).join(""));
  }
  document.addEventListener("change", function (e) {
    if (!e.target || !e.target.classList || !e.target.classList.contains("att-check")) return;
    syncZip();
  });
  // Le volet de lecture est réécrit sans recharger la page (list.js) : les cases arrivées avec le nouveau volet sont
  // encore « hidden ». On les révèle à chaque fois, pas seulement au premier affichage.
  function revealPicks() {
    document.querySelectorAll(".att-pick[hidden]").forEach(function (el) { el.hidden = false; });
    syncZip();
  }
  document.addEventListener("wmail:pane", revealPicks);
  revealPicks();

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
