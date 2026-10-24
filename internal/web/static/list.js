// Confort de la boîte de réception (les pages fonctionnent aussi sans ce script) :
//  - sélecteurs « data-autosubmit » : envoi du formulaire dès que la valeur change ;
//  - « Tout sélectionner » et Ctrl+A (Cmd+A sur Mac) : cochent tous les messages affichés, ou les décochent s'ils le sont déjà ;
//    Maj+clic : plage ; Échap : efface la sélection ; boutons d'action grisés sans sélection ni message ouvert ;
//  - clic sur un message : seul le volet de lecture est rechargé (/mail/pane), la liste ne bouge pas ;
//    quand la page se recharge quand même, la position de défilement de la liste est restaurée ;
//  - séparateurs entre les volets : glisser ou clavier, largeurs enregistrées dans le compte (POST /settings/panes).
(function () {
  "use strict";

  document.querySelectorAll("[data-js-only]").forEach(function (el) { el.hidden = false; });

  document.querySelectorAll("select[data-autosubmit]").forEach(function (sel) {
    sel.addEventListener("change", function () { sel.form.submit(); });
    if (sel.form) sel.form.querySelectorAll(".js-hide").forEach(function (b) { b.hidden = true; });
  });

  // Menu « déplacer vers » : Ctrl (ou Cmd) pendant le clic copie au lieu de déplacer, comme RainLoop.
  (function () {
    var menus = document.querySelectorAll("details.movemenu");
    if (!menus.length) return;
    function setCopy(on) {
      menus.forEach(function (m) {
        m.classList.toggle("copying", on);
        var lbl = m.querySelector("[data-move-label]");
        if (lbl) {
          if (!lbl.dataset.moveText) lbl.dataset.moveText = lbl.textContent;
          lbl.textContent = on ? m.getAttribute("data-copy-label") : lbl.dataset.moveText;
        }
      });
    }
    document.addEventListener("keydown", function (e) { if (e.key === "Control" || e.key === "Meta") setCopy(true); });
    document.addEventListener("keyup", function (e) { if (e.key === "Control" || e.key === "Meta") setCopy(false); });
    window.addEventListener("blur", function () { setCopy(false); });
    document.addEventListener("click", function (e) {
      var b = e.target && e.target.closest ? e.target.closest("details.movemenu button[name]") : null;
      if (!b) return;
      b.name = e.ctrlKey || e.metaKey ? "copy_to" : "move_to";
    }, true);
  })();

  // Boutons à confirmer (suppression définitive dans la corbeille ou les indésirables).
  document.addEventListener("click", function (e) {
    var b = e.target && e.target.closest ? e.target.closest("button[data-confirm]") : null;
    if (b && !window.confirm(b.getAttribute("data-confirm"))) e.preventDefault();
  });

  // Liste des contacts : le bouton « Écrire » suit la sélection.
  (function () {
    var clist = document.getElementById("clist");
    if (!clist) return;
    var buttons = document.querySelectorAll("[data-needs-contacts]");
    function refreshContacts() {
      var n = clist.querySelectorAll("input[name=id]:checked").length;
      buttons.forEach(function (b) { b.disabled = n === 0; });
    }
    clist.addEventListener("change", refreshContacts);
    refreshContacts();
  })();

  var form = document.querySelector("form.msglist-form");
  if (!form) return;

  var selectAll = document.getElementById("select-all");
  var counter = document.getElementById("select-count");
  var actions = document.querySelectorAll("[data-needs-selection]");
  var current = document.getElementById("current-uid");
  var paneSel = document.getElementById("pane-sel");
  var lastIndex = -1;

  function boxes() {
    return Array.prototype.slice.call(form.querySelectorAll("input[type=checkbox][name=uid]"));
  }

  function refresh() {
    var all = boxes();
    var n = all.filter(function (c) { return c.checked; }).length;
    var open = !!(current && current.value);
    if (selectAll) {
      selectAll.checked = n > 0 && n === all.length;
      selectAll.indeterminate = n > 0 && n < all.length;
    }
    if (counter) counter.textContent = n > 0 ? (counter.getAttribute("data-template") || "%d").replace("%d", String(n)) : "";
    actions.forEach(function (b) { b.disabled = n === 0 && !open; });
    if (paneSel) { // volet de lecture : « N messages sélectionnés » tant qu'aucun message n'est ouvert
      paneSel.hidden = n === 0;
      if (n > 0) {
        paneSel.querySelector(".pane-num").textContent = String(n);
        paneSel.querySelector(".pane-txt").textContent = (paneSel.getAttribute("data-template") || "").replace("%d", "").trim();
      }
    }
    all.forEach(function (c) {
      var row = c.closest(".msgrow");
      if (row) row.classList.toggle("checked", c.checked);
    });
  }

  function setAll(checked) {
    boxes().forEach(function (c) { c.checked = checked; });
    refresh();
  }

  // Transfert groupé : les messages cochés partent en pièces jointes d'un nouveau message.
  var fwdAtt = document.querySelector("[data-forward-att]");
  if (fwdAtt) {
    fwdAtt.addEventListener("click", function () {
      var ids = boxes().filter(function (c) { return c.checked; }).map(function (c) { return c.value; });
      if (!ids.length) return;
      location.href = fwdAtt.getAttribute("data-url") + "&uid=" + encodeURIComponent(ids[0]) + "&uids=" + encodeURIComponent(ids.join(","));
    });
  }

  if (selectAll) selectAll.addEventListener("change", function () { setAll(selectAll.checked); });

  // Sélection par critère (menu à côté de la case « tout sélectionner »).
  var criteria = {
    all: function () { return true; },
    none: function () { return false; },
    unseen: function (row) { return row.classList.contains("unread"); },
    seen: function (row) { return !row.classList.contains("unread"); },
    flagged: function (row) { return !!row.querySelector(".star.on"); },
    unflagged: function (row) { return !row.querySelector(".star.on"); }
  };
  document.querySelectorAll("[data-select]").forEach(function (b) {
    b.addEventListener("click", function () {
      var kind = b.getAttribute("data-select");
      boxes().forEach(function (c) {
        var row = c.closest(".msgrow");
        c.checked = kind === "invert" ? !c.checked : !!(row && criteria[kind](row));
      });
      refresh();
      var d = b.closest("details"); if (d) d.open = false;
    });
  });

  form.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
    // Ctrl+clic (ou Cmd) sur une ligne : coche ou décoche sans ouvrir le message ; Maj+clic : étend la sélection.
    if (t.type !== "checkbox" && !t.closest("input, button, .star, .threads, a.preview") && (e.ctrlKey || e.metaKey || e.shiftKey)) {
      var row = t.closest(".msgrow");
      var box = row && row.querySelector("input[name=uid]");
      if (box) {
        e.preventDefault();
        e.stopPropagation();
        var list = boxes(), at = list.indexOf(box);
        if (e.shiftKey && lastIndex >= 0 && lastIndex !== at) {
          var a = Math.min(at, lastIndex), b = Math.max(at, lastIndex);
          for (var j = a; j <= b; j++) list[j].checked = true;
        } else {
          box.checked = !box.checked;
        }
        lastIndex = at;
        refresh();
        return;
      }
    }
    if (t.type !== "checkbox" || t.name !== "uid") return;
    var all = boxes();
    var i = all.indexOf(t);
    if (e.shiftKey && lastIndex >= 0 && lastIndex !== i) {
      var lo = Math.min(i, lastIndex), hi = Math.max(i, lastIndex);
      for (var k = lo; k <= hi; k++) all[k].checked = t.checked;
    }
    lastIndex = i;
    refresh();
  });

  function typing(el) {
    return !!(el && el.closest && el.closest("input:not([type=checkbox]):not([type=radio]), textarea, select, [contenteditable]"));
  }

  document.addEventListener("keydown", function (e) {
    if (typing(e.target)) return;
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && String(e.key).toLowerCase() === "a") {
      var all = boxes();
      if (all.length === 0) return;
      e.preventDefault();
      // Tout sélectionner ; si tout est déjà sélectionné, tout désélectionner.
      setAll(!all.every(function (c) { return c.checked; }));
    } else if (e.key === "Escape" && boxes().some(function (c) { return c.checked; })) {
      setAll(false);
    }
  });

  document.addEventListener("wmail:pane", refresh);
  refresh();
})();

// Volet de lecture sans rechargement de la liste, et position de défilement de la liste.
(function () {
  "use strict";
  var listpane = document.querySelector(".listpane");
  var scroller = document.querySelector(".msglist-form"); // la zone qui défile dans la liste
  var readpane = document.querySelector(".readpane");
  if (!listpane || !scroller) return;

  // --- position de défilement de la liste ---
  var key = "wmail.listScroll";
  function signature() {
    var p = new URLSearchParams(location.search);
    ["uid", "view", "remote"].forEach(function (k) { p.delete(k); });
    p.sort();
    return location.pathname + "?" + p.toString();
  }
  function store(value) { try { sessionStorage.setItem(key, value); } catch (e) { /* stockage indisponible */ } }
  function load() { try { return sessionStorage.getItem(key); } catch (e) { return null; } }

  var restored = false;
  try {
    var saved = JSON.parse(load() || "null");
    if (saved && saved.sig === signature() && typeof saved.top === "number") {
      scroller.scrollTop = saved.top;
      restored = true;
    }
  } catch (e) { /* valeur illisible : ignorée */ }
  if (!restored) {
    // Arrivée sur la page (lien direct, retour) : amener la ligne ouverte dans la zone visible, sans toucher à la page.
    var sel = scroller.querySelector(".msgrow.selected");
    if (sel && (sel.offsetTop < scroller.scrollTop || sel.offsetTop + sel.offsetHeight > scroller.scrollTop + scroller.clientHeight)) {
      scroller.scrollTop = Math.max(0, sel.offsetTop - Math.round(scroller.clientHeight / 3));
    }
  }
  var pending = false;
  scroller.addEventListener("scroll", function () {
    if (pending) return;
    pending = true;
    (window.requestAnimationFrame || function (f) { return setTimeout(f, 50); })(function () {
      pending = false;
      store(JSON.stringify({ sig: signature(), top: scroller.scrollTop }));
    });
  });

  if (!readpane || !window.fetch || !window.URL) return;

  // --- chargement du volet de lecture ---
  var mailbox = document.querySelector(".mailbox");
  var currentRequest = 0;

  function decrement(el) {
    if (!el) return;
    var n = parseInt(el.textContent, 10);
    if (isNaN(n)) return;
    if (n <= 1) el.remove(); else el.textContent = String(n - 1);
  }

  function markSelected(uid) {
    listpane.querySelectorAll(".msgrow.selected").forEach(function (r) {
      r.classList.remove("selected");
      var a = r.querySelector("a.rowlink");
      if (a) a.removeAttribute("aria-current");
    });
    var row = document.getElementById("m" + uid);
    if (!row) return null;
    row.classList.add("selected");
    var link = row.querySelector("a.rowlink");
    if (link) link.setAttribute("aria-current", "true");
    return row;
  }

  function openPane(href) {
    var url = new URL(href, location.href);
    var uid = url.searchParams.get("uid");
    var token = ++currentRequest;
    return fetch("/mail/pane" + url.search, { credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
      .then(function (r) {
        if (r.redirected) { location.href = href; return null; } // session expirée : page de connexion
        return r.text();
      })
      .then(function (html) {
        if (html === null || token !== currentRequest) return; // réponse périmée : un autre clic est passé entre-temps
        readpane.innerHTML = html;
        readpane.scrollTop = 0;
        if (mailbox) mailbox.classList.add("has-message");
        var row = markSelected(uid);
        history.pushState(null, "", url.pathname + url.search);
        var view = readpane.querySelector(".msgview");
        if (view && view.hasAttribute("data-just-read") && row && row.classList.contains("unread")) {
          row.classList.remove("unread");
          decrement(document.querySelector(".folder-list li.active .badge"));
        }
        var cur = document.getElementById("current-uid");
        if (cur) cur.value = view ? (view.getAttribute("data-uid") || "") : "";
        var stay = document.querySelector("form.msglist-form input[name=stay]");
        if (stay) stay.value = url.pathname + url.search;
        document.dispatchEvent(new Event("wmail:pane"));
      })
      .catch(function () { location.href = href; });
  }

  document.addEventListener("click", function (e) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    var a = e.target && e.target.closest ? e.target.closest("a") : null;
    if (!a) return;
    var inList = listpane.contains(a) && a.classList.contains("rowlink");
    var inPane = readpane.contains(a) && a.hasAttribute("data-pane");
    if (!inList && !inPane) return;
    if (!a.getAttribute("href")) return; // flèche désactivée (première ou dernière ligne) : rien à charger
    e.preventDefault();
    openPane(a.getAttribute("href"));
  });

  window.addEventListener("popstate", function () { location.reload(); });
})();

// Volets redimensionnables : glisser un séparateur (ou le clavier : ← →, Maj pour aller plus vite) change la largeur
// du volet à sa gauche ; la largeur est enregistrée dans la configuration du compte (POST /settings/panes).
// Double-clic sur un séparateur : retour à la largeur par défaut.
(function () {
  "use strict";
  var mailbox = document.querySelector(".mailbox");
  if (!mailbox || !window.PointerEvent || !window.fetch) return;
  var splitters = mailbox.querySelectorAll(".splitter");
  if (!splitters.length) return;
  mailbox.classList.add("resizable");

  var root = document.documentElement;
  var GAP = 12;          // les deux séparateurs
  var MIN_MAIN = 320;    // largeur minimale gardée pour la lecture
  var spec = {
    folders: { selector: ".folders", variable: "--folders-w", min: 160, max: 480 },
    list: { selector: ".listpane", variable: "--list-w", min: 240, max: 1000 }
  };

  function pane(name) { return mailbox.querySelector(spec[name].selector); }
  function width(name) { var el = pane(name); return el ? Math.round(el.getBoundingClientRect().width) : 0; }

  function clamp(name, px) {
    var s = spec[name];
    var others = name === "folders" ? (pane("list") ? width("list") : 0) : width("folders");
    var room = mailbox.clientWidth - others - GAP - MIN_MAIN;
    return Math.round(Math.max(s.min, Math.min(px, s.max, room > s.min ? room : s.min)));
  }

  // Valeurs ARIA du séparateur (sans toucher à la largeur : elle reste celle de la feuille de style).
  function describe(name, px) {
    var sp = mailbox.querySelector('.splitter[data-pane="' + name + '"]');
    if (sp) {
      sp.setAttribute("aria-valuenow", String(px));
      sp.setAttribute("aria-valuemin", String(spec[name].min));
      sp.setAttribute("aria-valuemax", String(spec[name].max));
    }
  }

  // Fixe la largeur d'un volet (variable CSS) ; tant que l'utilisateur n'a rien changé, les largeurs par défaut restent adaptatives.
  function apply(name, px) {
    root.style.setProperty(spec[name].variable, px + "px");
    describe(name, px);
  }

  function save(values) {
    var body = new URLSearchParams();
    body.set("csrf", mailbox.getAttribute("data-csrf") || "");
    Object.keys(values).forEach(function (k) { body.set(k, String(values[k])); });
    return fetch("/settings/panes", {
      method: "POST", credentials: "same-origin", body: body,
      headers: { "X-Requested-With": "fetch", "Content-Type": "application/x-www-form-urlencoded" }
    }).catch(function () { /* hors ligne : la largeur reste appliquée jusqu'au rechargement */ });
  }

  var timer = null;
  function saveSoon(name) {
    clearTimeout(timer);
    timer = setTimeout(function () { var v = {}; v[name] = width(name); save(v); }, 400);
  }

  var bottom = mailbox.classList.contains("bottom"); // aperçu en bas : la liste ne se redimensionne pas en largeur
  splitters.forEach(function (sp) {
    var name = sp.getAttribute("data-pane");
    if (!spec[name] || !pane(name) || (bottom && name === "list")) return;
    describe(name, width(name));
    var startX = 0, startW = 0, dragging = false;

    sp.addEventListener("pointerdown", function (e) {
      if (e.button !== 0) return;
      dragging = true; startX = e.clientX; startW = width(name);
      if (sp.setPointerCapture) { try { sp.setPointerCapture(e.pointerId); } catch (err) { /* ignoré */ } }
      document.body.classList.add("dragging");
      sp.classList.add("active");
      e.preventDefault();
    });
    sp.addEventListener("pointermove", function (e) {
      if (!dragging) return;
      apply(name, clamp(name, startW + (e.clientX - startX)));
    });
    function stop() {
      if (!dragging) return;
      dragging = false;
      document.body.classList.remove("dragging");
      sp.classList.remove("active");
      var v = {}; v[name] = width(name); save(v);
    }
    sp.addEventListener("pointerup", stop);
    sp.addEventListener("pointercancel", stop);

    sp.addEventListener("keydown", function (e) {
      var step = e.shiftKey ? 64 : 16;
      if (e.key === "ArrowLeft") apply(name, clamp(name, width(name) - step));
      else if (e.key === "ArrowRight") apply(name, clamp(name, width(name) + step));
      else return;
      e.preventDefault();
      saveSoon(name);
    });

    sp.addEventListener("dblclick", function () {
      root.style.removeProperty(spec[name].variable);
      describe(name, width(name));
      var v = {}; v[name] = 0; save(v);
    });
  });
})();
