// Rafraîchit la liste sans quitter la page ni fermer le message ouvert : à l'arrivée de nouveaux messages (sondage) et au
// clic sur le bouton « actualiser ». Les cases cochées, la sélection et le défilement sont conservés.
// Seules les listes de première page, sans recherche, sont mises à jour d'office (c'est là qu'arrivent les nouveaux).
(function () {
  "use strict";
  var form = document.querySelector("form.msglist-form");
  if (!form || !window.fetch || !window.DOMParser) return;
  var busy = false;

  function listURL() {
    var u = new URL(location.href);
    u.searchParams.delete("uid"); u.searchParams.delete("remote"); u.searchParams.delete("view");
    return u;
  }
  function autoOK() {
    var p = new URLSearchParams(location.search);
    return !p.get("q") && (!p.get("page") || p.get("page") === "1") && !p.get("thread");
  }

  function refresh(spin) {
    if (busy) return Promise.resolve();
    busy = true;
    var btn = document.querySelector("[data-refresh]");
    if (spin && btn) btn.classList.add("spinning");
    var u = listURL();
    return fetch(u.pathname + u.search, { credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
      .then(function (r) { if (r.redirected || !r.ok) throw new Error("session"); return r.text(); })
      .then(function (html) {
        var doc = new DOMParser().parseFromString(html, "text/html");
        var fresh = doc.querySelector("ul.msgrows"), old = document.querySelector("ul.msgrows");
        if (!fresh || !old) return;
        var checked = {};
        old.querySelectorAll("input[name=uid]:checked").forEach(function (c) { checked[c.value] = true; });
        var cur = document.getElementById("current-uid"), open = cur ? cur.value : "";
        var focused = old.querySelector(".msgrow.focused"), focusID = focused ? focused.id : "";
        var top = form.scrollTop;
        old.innerHTML = fresh.innerHTML;
        old.querySelectorAll("input[name=uid]").forEach(function (c) { if (checked[c.value]) c.checked = true; });
        if (open) { var row = document.getElementById("m" + open); if (row) { row.classList.add("selected"); var a = row.querySelector("a.rowlink"); if (a) a.setAttribute("aria-current", "true"); } }
        if (focusID) { var f = document.getElementById(focusID); if (f) f.classList.add("focused"); }
        form.scrollTop = top;
        ["footer.lp-foot .count", ".listClear"].forEach(function (sel) {
          var n = doc.querySelector(sel), o = document.querySelector(sel);
          if (n && o) o.innerHTML = n.innerHTML;
        });
        document.dispatchEvent(new Event("wmail:pane")); // recompte les cases cochées et les boutons
      })
      .catch(function () { /* hors ligne ou session expirée : la prochaine navigation le dira */ })
      .then(function () { busy = false; if (btn) btn.classList.remove("spinning"); });
  }

  document.addEventListener("wmail:newmail", function () { if (autoOK()) refresh(false); });
  document.addEventListener("click", function (e) {
    var a = e.target && e.target.closest ? e.target.closest("a[data-refresh]") : null;
    if (!a || e.ctrlKey || e.metaKey || e.shiftKey) return;
    e.preventDefault();
    refresh(true);
  });
  window.wmailRefreshList = refresh;
})();
