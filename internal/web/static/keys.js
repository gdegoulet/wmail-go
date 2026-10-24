// Raccourcis clavier de la boîte aux lettres (ceux de RainLoop : c, r, a, f, z, m, q, u, i, /, ?, Suppr…).
// Ils agissent sur les boutons de la barre d'outils : rien n'est décidé ici, le serveur contrôle chaque action.
(function () {
  "use strict";
  var help = document.getElementById("keyhelp");

  function typing(el) {
    return !!(el && el.closest && el.closest("input:not([type=checkbox]):not([type=radio]), textarea, select, [contenteditable]"));
  }
  function click(sel) {
    var b = document.querySelector(sel);
    if (!b || b.disabled) return false;
    b.click();
    return true;
  }
  function go(el) { if (!el) return false; el.click(); return true; }
  function composeLink(mode) {
    var re = new RegExp("[?&]mode=" + mode + "(&|$)");
    var links = document.querySelectorAll('a[href*="/compose?"]');
    for (var i = 0; i < links.length; i++) if (re.test(links[i].getAttribute("href"))) return links[i];
    return null;
  }
  // toggleFlag suit ou ne suit plus les messages visés : si tous sont déjà suivis, l'étoile est retirée.
  function toggleFlag() {
    var rows = Array.prototype.slice.call(document.querySelectorAll(".msgrow"));
    var targets = rows.filter(function (r) { var c = r.querySelector("input[name=uid]"); return c && c.checked; });
    if (!targets.length) targets = rows.filter(function (r) { return r.classList.contains("selected") || r.classList.contains("focused"); });
    var open = document.querySelector(".msgview .star");
    var allOn = targets.length
      ? targets.every(function (r) { return !!r.querySelector(".star.on"); })
      : !!(open && open.classList.contains("on"));
    return click('button[form=bulk][name=action][value=' + (allOn ? "unflag" : "flag") + ']');
  }

  function toggleHelp(show) {
    if (!help) return;
    help.hidden = !(show === undefined ? help.hidden : show);
  }

  document.querySelectorAll("[data-keyhelp]").forEach(function (b) {
    b.hidden = false;
    b.addEventListener("click", function () { toggleHelp(true); });
  });
  document.querySelectorAll("[data-keyhelp-close]").forEach(function (b) { b.addEventListener("click", function () { toggleHelp(false); }); });

  document.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey || typing(e.target)) return;
    var k = e.key, handled = true;
    if (k === "Escape") {
      if (help && !help.hidden) { toggleHelp(false); e.preventDefault(); return; }
      if (document.querySelector("input[name=uid]:checked")) return; // Échap efface d'abord la sélection (list.js)
      // puis annule la recherche, puis ferme la conversation ouverte, puis ferme le message
      handled = go(document.querySelector(".search-clear")) || go(document.querySelector(".threadbar a")) || go(document.querySelector("a.mh-close"));
    } else if (k === "?") {
      toggleHelp();
    } else if (k === "/") {
      var q = document.getElementById("q");
      handled = !!q;
      if (q) q.focus();
    } else if (k === "Delete" || k === "#") {
      handled = click('button[form=bulk][name=action][value=delete]');
    } else if (e.shiftKey && (k === "F" || k === "f")) {
      handled = go(composeLink("forwardatt"));
    } else if (e.shiftKey) {
      return;
    } else {
      switch (k.toLowerCase()) {
        case "c": case "w": handled = go(document.querySelector("a.btn-new")); break;
        case "r": handled = go(composeLink("reply")); break;
        case "a": handled = go(composeLink("replyall")); break;
        case "f": handled = go(composeLink("forward")); break;
        case "z": handled = click('button[form=bulk][name=action][value=archive]'); break;
        case "q": handled = click('button[form=bulk][name=action][value=read]'); break;
        case "u": handled = click('button[form=bulk][name=action][value=unread]'); break;
        case "i": handled = toggleFlag(); break;
        case "t": handled = go(document.querySelector(".msgrow.focused .threads, .msgrow.selected .threads")); break;
        case "m": handled = click(".mb-mid details.dropdown summary"); break;
        case "`": handled = click(".mb-right details.dropdown summary"); break;
        default: return;
      }
    }
    if (handled) e.preventDefault();
  });
})();
