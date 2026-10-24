// Enregistrement des réglages à la volée (comme RainLoop) : chaque champ modifié envoie tout le formulaire
// en arrière-plan et une petite coche verte apparaît à côté de lui. Sans JavaScript, le bouton « Enregistrer »
// fait le travail comme avant ; il reste utilisable même quand l'envoi en arrière-plan échoue.
(function () {
  "use strict";
  var form = document.querySelector("form[data-autosave]");
  if (!form || typeof window.fetch !== "function") return;

  var DUREE = 2000; // durée d'affichage de la coche, en millisecondes
  var TEXTE_OK = form.getAttribute("data-saved") || "Enregistré";
  var TEXTE_KO = form.getAttribute("data-save-error") || "Échec de l'enregistrement";

  // corps rassemble les champs du formulaire comme le ferait un envoi normal
  // (cases et boutons radio non cochés ignorés, boutons exclus).
  function corps() {
    var body = new URLSearchParams();
    Array.prototype.forEach.call(form.elements, function (el) {
      if (!el.name || el.disabled) return;
      var t = (el.type || "").toLowerCase();
      if (t === "submit" || t === "button" || t === "reset" || t === "file") return;
      if ((t === "checkbox" || t === "radio") && !el.checked) return;
      if (el.tagName === "SELECT" && el.multiple) {
        Array.prototype.forEach.call(el.options, function (o) { if (o.selected) body.append(el.name, o.value); });
        return;
      }
      body.append(el.name, el.value);
    });
    return body;
  }

  // hote renvoie l'élément auquel accrocher la coche : la légende pour la grille de thèmes,
  // l'étiquette du champ sinon.
  function hote(champ) {
    var grille = champ.closest ? champ.closest(".theme-field") : null;
    if (grille) return grille.querySelector(".theme-legend") || grille;
    return (champ.closest && champ.closest("label")) || champ.parentNode || champ;
  }

  // placer glisse la coche juste après l'intitulé du champ : au bout de l'étiquette quand le champ la
  // commence (cases à cocher), avant le champ sinon (l'étiquette occupant alors toute la largeur).
  function placer(cible, span) {
    var controle = null, premier = cible.firstElementChild;
    Array.prototype.forEach.call(cible.children, function (el) {
      if (!controle && /^(INPUT|SELECT|TEXTAREA)$/.test(el.tagName)) controle = el;
    });
    if (controle && controle !== premier) cible.insertBefore(span, controle);
    else cible.appendChild(span);
  }

  function marque(champ, ok) {
    var cible = hote(champ);
    var span = null;
    Array.prototype.forEach.call(cible.children, function (el) {
      if (el.classList && el.classList.contains("saved-tick")) span = el;
    });
    if (!span) {
      span = document.createElement("span");
      span.className = "saved-tick";
      placer(cible, span);
    }
    if (span.timer) { clearTimeout(span.timer); span.timer = 0; }
    span.textContent = ok ? "✓" : "✗";
    span.className = ok ? "saved-tick" : "saved-tick err";
    span.setAttribute("title", ok ? TEXTE_OK : TEXTE_KO);
    span.setAttribute("role", "status");
    if (ok) {
      span.timer = setTimeout(function () { span.remove(); }, DUREE);
    }
  }

  function envoyer(champ) {
    fetch(form.getAttribute("action") || location.pathname, {
      method: "POST",
      body: corps(),
      credentials: "same-origin",
      headers: { "X-Requested-With": "fetch", "Content-Type": "application/x-www-form-urlencoded" }
    }).then(function (r) {
      marque(champ, !!(r && r.ok));
    }, function () {
      marque(champ, false); // hors ligne : le bouton « Enregistrer » reste là pour réessayer
    });
  }

  // La vignette du thème choisi est mise en évidence tout de suite (sans attendre la réponse).
  function majThemes() {
    form.querySelectorAll(".theme-tile").forEach(function (tile) {
      var r = tile.querySelector("input[type=radio]");
      tile.classList.toggle("on", !!(r && r.checked));
    });
  }

  form.addEventListener("change", function (ev) {
    var champ = ev.target;
    if (!champ || !champ.name || champ.name === "csrf" || champ.disabled) return;
    if (champ.name === "theme") majThemes();
    // Un champ invalide (nombre hors bornes, champ vide) n'est pas envoyé : le navigateur le signalera
    // au moment d'appuyer sur « Enregistrer ».
    if (typeof champ.checkValidity === "function" && !champ.checkValidity()) { marque(champ, false); return; }
    envoyer(champ);
  });
})();
