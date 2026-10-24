// Sondage périodique de la boîte de réception : compteur de non lus dans le titre de l'onglet, notification de bureau
// et son pour les nouveaux messages.
(function () {
  "use strict";
  var el = document.getElementById("poll");
  if (!el || !window.fetch) return;
  var base = el.getAttribute("data-title") || document.title;
  var desktop = el.hasAttribute("data-desktop"), sound = el.hasAttribute("data-sound");
  var faviconOn = el.hasAttribute("data-favicon");
  var since = 0, unseen = parseInt(el.getAttribute("data-unseen"), 10) || 0;
  var busy = false, failures = 0;

  function title() { document.title = (unseen > 0 ? "(" + unseen + ") " : "") + base; favicon(); }

  // Icône de l'onglet : pastille rouge avec le nombre de non lus (dessinée en SVG, comme RainLoop).
  var link = null, plain = "/static/favicon.svg";
  function favicon() {
    if (!faviconOn) return;
    if (!link) {
      link = document.querySelector('link[rel="icon"]');
      if (!link) { link = document.createElement("link"); link.rel = "icon"; document.head.appendChild(link); }
      plain = link.getAttribute("href") || plain;
    }
    if (unseen <= 0) { link.setAttribute("href", plain); link.setAttribute("type", "image/svg+xml"); return; }
    var n = unseen > 99 ? "99+" : String(unseen);
    var w = n.length > 2 ? 34 : 26;
    var svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">' +
      '<rect width="64" height="64" rx="12" fill="#3a4a5a"/>' +
      '<path d="M12 22h40v20a4 4 0 0 1-4 4H16a4 4 0 0 1-4-4z" fill="#f2f2f2"/>' +
      '<path d="M12 22 32 37 52 22" fill="none" stroke="#3a4a5a" stroke-width="4" stroke-linejoin="round"/>' +
      '<rect x="' + (62 - w) + '" y="2" width="' + w + '" height="26" rx="13" fill="#bd362f"/>' +
      '<text x="' + (62 - w / 2) + '" y="21" font-family="Arial,Helvetica,sans-serif" font-size="18" font-weight="bold" fill="#fff" text-anchor="middle">' + n + "</text></svg>";
    link.setAttribute("type", "image/svg+xml");
    link.setAttribute("href", "data:image/svg+xml," + encodeURIComponent(svg));
  }
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
        var first = since === 0, before = unseen;
        unseen = d.unseen; title();
        if (!first && d.new && d.new.length) notify(d.new);
        // UIDNEXT a changé (nouveaux messages) ou le nombre de non lus a bougé (un autre client a lu ou marqué
        // des messages) : la liste est rechargée pour que les drapeaux affichés soient à jour.
        if (!first && (d.uidnext > since || d.unseen !== before)) document.dispatchEvent(new Event("wmail:newmail"));
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
        Object.keys(d.accounts || {}).forEach(function (id) { // non lus des autres boîtes
          var b = document.querySelector('[data-acc-badge="' + id + '"]');
          if (!b) return;
          b.textContent = String(d.accounts[id]);
          b.hidden = d.accounts[id] === 0;
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
