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
    else if ((k === " " || k === "Insert") && cur) { e.preventDefault(); check(cur); return; }
    else if (k === "Enter" && cur) { e.preventDefault(); open(cur, true); return; }
    else return;
    e.preventDefault();
    if (e.shiftKey && (k === "ArrowDown" || k === "ArrowUp")) {
      if (cur) check(cur, true);
      check(all[to], true);
      setFocus(all[to]);
      return;
    }
    setFocus(all[to]);
    open(all[to]);
  });
})();
