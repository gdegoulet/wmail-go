// Sondage périodique de la boîte de réception : compteur de non lus dans le titre de l'onglet, notification de bureau
// et son pour les nouveaux messages.
(function () {
  "use strict";
  var el = document.getElementById("poll");
  if (!el || !window.fetch) return;
  var base = el.getAttribute("data-title") || document.title;
  var desktop = el.hasAttribute("data-desktop"), sound = el.hasAttribute("data-sound");
  var since = 0, unseen = parseInt(el.getAttribute("data-unseen"), 10) || 0;
  var busy = false, failures = 0;

  function title() { document.title = (unseen > 0 ? "(" + unseen + ") " : "") + base; }
  title();

  function beep() {
    try {
      var Ctx = window.AudioContext || window.webkitAudioContext;
      if (!Ctx) return;
      var ctx = new Ctx(), o = ctx.createOscillator(), g = ctx.createGain();
      o.type = "sine"; o.frequency.value = 880; g.gain.value = 0.08;
      o.connect(g); g.connect(ctx.destination);
      o.start(); o.stop(ctx.currentTime + 0.18);
      o.onended = function () { ctx.close(); };
    } catch (e) { /* son indisponible */ }
  }

  function notify(list) {
    if (sound) beep();
    if (!desktop || !("Notification" in window) || Notification.permission !== "granted") return;
    if (list.length > 3) {
      new Notification(el.getAttribute("data-many").replace(/\b0\b/, String(list.length)));
      return;
    }
    list.forEach(function (m) {
      var n = new Notification(m.from || "", { body: m.subject || "", tag: "wmail-" + m.uid });
      n.onclick = function () { window.focus(); location.href = "/mail/message?folder=INBOX&uid=" + m.uid; n.close(); };
      setTimeout(function () { n.close(); }, 7000);
    });
  }

  function poll() {
    if (busy || document.hidden && failures > 3) return;
    busy = true;
    fetch("/mail/poll?since=" + since, { credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
      .then(function (r) { if (!r.ok) throw new Error(r.status); return r.json(); })
      .then(function (d) {
        failures = 0;
        var first = since === 0;
        unseen = d.unseen; title();
        if (!first && d.new && d.new.length) notify(d.new);
        if (!first && d.uidnext > since) document.dispatchEvent(new Event("wmail:newmail")); // UIDNEXT a changé : nouveaux messages, lus ou non
        since = d.uidnext || since;
        Object.keys(d.folders || {}).forEach(function (name) {
          var n = d.folders[name], row = null;
          document.querySelectorAll("[data-drop]").forEach(function (x) { if (x.getAttribute("data-drop") === name) row = x; });
          if (!row || row.hasAttribute("data-nobadge")) return; // corbeille, archive, envoyés : pas de pastille
          var bd = row.querySelector("[data-unread-badge]");
          if (!bd && n > 0) { bd = document.createElement("span"); bd.className = "badge"; bd.setAttribute("data-unread-badge", ""); row.appendChild(bd); }
          if (bd) { if (n > 0) bd.textContent = String(n); else bd.remove(); }
          row.classList.toggle("has-unread", n > 0);
        });
        var li = document.querySelector(".folder-list.sys .role-inbox");
        if (li) {
          var badge = li.querySelector("[data-unread-badge]");
          if (!badge && unseen > 0) {
            badge = document.createElement("span"); badge.className = "badge"; badge.setAttribute("data-unread-badge", "");
            li.appendChild(badge);
          }
          if (badge) { if (unseen > 0) badge.textContent = String(unseen); else badge.remove(); }
          li.classList.toggle("has-unread", unseen > 0);
        }
      })
      .catch(function () { failures++; })
      .then(function () { busy = false; });
  }
  poll();
  setInterval(poll, 120000);
  document.addEventListener("visibilitychange", function () { if (!document.hidden) poll(); });
})();
