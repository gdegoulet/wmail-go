// Navigation au clavier dans la liste des messages (comme RainLoop) : ↑ ↓ déplacent le curseur et ouvrent le message,
// Maj+↑/↓ cochent en passant, Début/Fin, Page précédente/suivante (10 lignes), Espace ou Inser coche la ligne,
// Entrée ouvre la ligne ; au bout de la page, ↓ (↑) passe à la page suivante (précédente).
(function () {
  "use strict";
  var list = document.querySelector("ul.msgrows");
  if (!list) return;

  function typing(el) {
    return !!(el && el.closest && el.closest("input:not([type=checkbox]):not([type=radio]), textarea, select, [contenteditable], dialog[open]"));
  }
  function rows() { return Array.prototype.slice.call(list.querySelectorAll(".msgrow")); }
  function focused() { return list.querySelector(".msgrow.focused") || list.querySelector(".msgrow.selected"); }
  function setFocus(row) {
    list.querySelectorAll(".msgrow.focused").forEach(function (r) { r.classList.remove("focused"); });
    row.classList.add("focused");
    if (row.scrollIntoView) row.scrollIntoView({ block: "nearest" });
  }
  var timer = null;
  function open(row, now) {
    var a = row.querySelector("a.rowlink");
    if (!a) return;
    clearTimeout(timer);
    if (now) { a.click(); return; }
    timer = setTimeout(function () { a.click(); }, 200); // une touche maintenue n'ouvre que le dernier message
  }
  // Sélection par Maj+flèches : ancre fixe, la plage suit le curseur. Changer de sens réduit la plage
  // (les lignes quittées sont décochées), sauf celles déjà cochées avant la série.
  var anchor = -1, before = null;
  function uidOf(row) { var c = row.querySelector("input[name=uid]"); return c ? c.value : ""; }
  function applyRange(all, from, to) {
    var lo = Math.min(from, to), hi = Math.max(from, to), changed = false;
    all.forEach(function (row, idx) {
      var c = row.querySelector("input[name=uid]");
      if (!c) return;
      var want = (idx >= lo && idx <= hi) || before[c.value] === true;
      if (c.checked !== want) { c.checked = want; changed = true; }
    });
    if (changed) document.dispatchEvent(new Event("wmail:pane")); // recompte la sélection et les boutons
  }

  function check(row, on) {
    var c = row.querySelector("input[name=uid]");
    if (c) { c.checked = on === undefined ? !c.checked : on; c.dispatchEvent(new Event("click", { bubbles: true })); }
  }
  function pageLink(rel) {
    var cur = document.querySelector(".pager .cur");
    var n = cur && (rel === "next" ? cur.nextElementSibling : cur.previousElementSibling);
    while (n && n.tagName !== "A") n = rel === "next" ? n.nextElementSibling : n.previousElementSibling;
    return n || null;
  }

  // Un clic dans la liste repart d'une nouvelle ancre pour Maj+flèches, et amène le curseur sur la ligne
  // cliquée : sans cela le curseur restait là où le clavier l'avait laissé, et les actions suivantes
  // (Suppr, archiver) visaient une autre ligne que celle que l'on venait d'ouvrir.
  list.addEventListener("click", function (e) {
    anchor = -1;
    before = null;
    var row = e.target && e.target.closest && e.target.closest(".msgrow");
    if (row && !e.target.closest("label.chk")) setFocus(row); // cocher ne déplace pas le curseur
  });

  // --- curseur conservé au travers des actions de la barre d'outils ---
  // Supprimer, archiver ou déplacer recharge la page : la classe « focused » disparaît et le curseur
  // revenait au début de la liste. On note donc, avant l'envoi, la ligne du curseur et la première ligne
  // qui lui survivra ; au chargement suivant, le curseur se repose sur l'une des deux, comme dans RainLoop.
  var curseurCle = "wmail.listCursor";

  function signature() {
    var p = new URLSearchParams(location.search);
    ["uid", "view", "remote"].forEach(function (k) { p.delete(k); });
    p.sort();
    return location.pathname + "?" + p.toString();
  }
  function ligneDe(uid) { return uid ? document.getElementById("m" + uid) : null; }

  function memoriser() {
    var all = rows(), cur = focused();
    if (!cur) return;
    var i = all.indexOf(cur);
    // Lignes que l'action va traiter : celles qui sont cochées, ou à défaut celle du curseur.
    var vises = {};
    var coches = all.filter(function (r) { var c = r.querySelector("input[name=uid]"); return c && c.checked; });
    (coches.length ? coches : [cur]).forEach(function (r) { vises[uidOf(r)] = true; });
    var survivant = "", j;
    for (j = i + 1; j < all.length && !survivant; j++) if (!vises[uidOf(all[j])]) survivant = uidOf(all[j]);
    for (j = i - 1; j >= 0 && !survivant; j--) if (!vises[uidOf(all[j])]) survivant = uidOf(all[j]); // fin de liste
    try {
      sessionStorage.setItem(curseurCle, JSON.stringify({ sig: signature(), uid: uidOf(cur), suivant: survivant, idx: i }));
    } catch (e) { /* stockage indisponible : le curseur repartira du début */ }
  }

  function restaurer() {
    var brut = null;
    try {
      brut = sessionStorage.getItem(curseurCle);
      sessionStorage.removeItem(curseurCle); // valable pour le seul chargement qui suit l'action
    } catch (e) { return; }
    if (!brut) return;
    var etat = null;
    try { etat = JSON.parse(brut); } catch (e) { return; }
    if (!etat || etat.sig !== signature()) return; // autre dossier, autre recherche : on ne force rien
    var all = rows();
    if (!all.length) return;
    // Le serveur a rouvert un message dans le volet de lecture (open_next) : c'est lui qui fait foi, pour
    // que le curseur et le message affiché ne divergent jamais. Sinon : la ligne d'origine si elle existe
    // encore (marquer lu, suivre…), puis celle qui la suivait, et en dernier recours sa place.
    var cible = list.querySelector(".msgrow.selected") || ligneDe(etat.uid) || ligneDe(etat.suivant);
    if (!cible || all.indexOf(cible) < 0) cible = all[Math.min(Math.max(etat.idx | 0, 0), all.length - 1)];
    setFocus(cible);
  }

  // En phase de capture : le gestionnaire doit lire la liste avant que la page ne parte.
  document.addEventListener("submit", function (e) {
    if (e.target && e.target.id === "bulk") memoriser();
  }, true);
  restaurer();

  document.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || typing(e.target)) return;
    // Le volet de lecture a le focus (flèche droite) : les flèches y défilent au lieu de parcourir la liste.
    var act = document.activeElement;
    if (act && act.closest && act.closest(".readpane")) return;
    var all = rows();
    if (!all.length) return;
    var cur = focused(), i = cur ? all.indexOf(cur) : -1, to = -1, k = e.key;
    if (k === "ArrowDown") {
      if (i === all.length - 1) { var nx = pageLink("next"); if (nx) { e.preventDefault(); nx.click(); } return; }
      to = i + 1;
    } else if (k === "ArrowUp") {
      if (i <= 0 && i !== -1) { var pv = pageLink("prev"); if (pv) { e.preventDefault(); pv.click(); } return; }
      to = Math.max(i - 1, 0);
    } else if (k === "Home") to = 0;
    else if (k === "End") to = all.length - 1;
    else if (k === "PageDown") to = Math.min((i < 0 ? 0 : i) + 10, all.length - 1);
    else if (k === "PageUp") to = Math.max((i < 0 ? 0 : i) - 10, 0);
    else if ((k === " " || k === "Insert") && cur) { e.preventDefault(); anchor = -1; check(cur); return; }
    else if (k === "Enter" && cur) { e.preventDefault(); open(cur, true); return; }
    else return;
    e.preventDefault();
    if (e.shiftKey && (k === "ArrowDown" || k === "ArrowUp")) {
      if (anchor < 0 || anchor >= all.length) {
        anchor = i < 0 ? to : i;
        before = {};
        all.forEach(function (row) {
          var c = row.querySelector("input[name=uid]");
          if (c && c.checked) before[c.value] = true;
        });
        delete before[uidOf(all[anchor])]; // l'ancre appartient à la plage, pas aux cases d'avant
      }
      applyRange(all, anchor, to);
      setFocus(all[to]);
      return;
    }
    anchor = -1; // toute autre navigation repart d'une nouvelle ancre
    before = null;
    setFocus(all[to]);
    open(all[to]);
  });
})();

// Flèches « message précédent / suivant » de l'en-tête du volet de lecture.
// /mail/pane ne connaît pas la liste (il ne la recharge pas) : le serveur ne peut pas remplir les deux adresses et les
// flèches restaient grisées dès que le volet était injecté. On les renseigne ici d'après la liste affichée.
(function () {
  "use strict";
  var list = document.querySelector("ul.msgrows");
  var readpane = document.querySelector(".readpane");
  if (!list || !readpane) return;

  function neighbour(row, dir) {
    var n = dir === "prev" ? row.previousElementSibling : row.nextElementSibling;
    while (n && !n.classList.contains("msgrow")) n = dir === "prev" ? n.previousElementSibling : n.nextElementSibling;
    return n;
  }
  function refresh() {
    var view = readpane.querySelector(".msgview");
    var uid = view ? view.getAttribute("data-uid") : "";
    var row = uid ? document.getElementById("m" + uid) : null;

    // Le volet se charge sans recharger la page : sans cette mise à jour, « current_uid » et « open_next »
    // gardaient le message du dernier rendu complet. Supprimer visait alors un autre message que celui
    // affiché, et rouvrait le voisin de celui-là.
    var champUID = document.getElementById("current-uid");
    if (champUID) champUID.value = uid;
    var champNext = document.getElementById("open-next");
    if (champNext && (row || !uid)) { // message hors de la liste affichée : on garde la valeur du serveur
      var apres = row && (neighbour(row, "next") || neighbour(row, "prev"));
      var lien = apres ? apres.querySelector("a.rowlink") : null;
      champNext.value = lien ? lien.getAttribute("href") : "";
    }

    if (!row) return; // message absent de la liste affichée (autre page, liste filtrée) : on garde l'état du serveur
    readpane.querySelectorAll("a.mh-nav[data-msgnav]").forEach(function (a) {
      var n = neighbour(row, a.getAttribute("data-msgnav"));
      var link = n ? n.querySelector("a.rowlink") : null;
      if (link) { a.setAttribute("href", link.getAttribute("href")); a.classList.remove("off"); }
      else { a.removeAttribute("href"); a.classList.add("off"); } // en tête ou en fin de liste
    });
  }
  document.addEventListener("wmail:pane", refresh);
  refresh();
})();
