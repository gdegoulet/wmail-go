// Éditeur de texte riche de la composition : zone modifiable (contenteditable) et barre d'outils. Le HTML produit est
// assaini par le serveur à l'envoi ; ici on ne fait que lui transmettre le contenu et le choix de mode.
(function () {
  "use strict";
  var form = document.getElementById("compose");
  var pane = document.getElementById("cp-pane-text");
  var editor = document.getElementById("cp-editor");
  var area = document.getElementById("cp-bodytext");
  var bar = document.getElementById("cp-fmt");
  if (!form || !pane || !editor || !area || !bar) return;
  var toggle = document.getElementById("cp-mode");
  var html = pane.getAttribute("data-mode") === "html";

  function show() {
    editor.hidden = !html;
    bar.hidden = !html;
    area.hidden = html;
    pane.classList.toggle("rich", html);
  }
  show();

  // Contenu transmis : en mode HTML, le HTML et l'indicateur ; le texte brut reste dans le champ « body ».
  form.addEventListener("formdata", function (e) {
    e.formData.set("is_html", html ? "1" : "0");
    if (html) e.formData.set("html", editor.innerHTML);
  });

  // --- images collées ou déposées : insérées en ligne (le serveur les incorpore au message) ---
  var IMG_TYPES = /^image\/(png|jpeg|gif|webp)$/, MAX_IMG = 3 * 1024 * 1024;
  function insertImages(files) {
    var any = false;
    Array.prototype.forEach.call(files || [], function (f) {
      if (!IMG_TYPES.test(f.type) || f.size > MAX_IMG || !window.FileReader) return;
      any = true;
      var r = new FileReader();
      r.onload = function () {
        editor.focus();
        document.execCommand("insertImage", false, r.result);
        form.dispatchEvent(new Event("input", { bubbles: true }));
      };
      r.readAsDataURL(f);
    });
    return any;
  }
  editor.addEventListener("paste", function (e) {
    var items = e.clipboardData && e.clipboardData.files;
    if (items && items.length && insertImages(items)) e.preventDefault();
  });
  editor.addEventListener("drop", function (e) {
    var files = e.dataTransfer && e.dataTransfer.files;
    if (files && files.length && Array.prototype.some.call(files, function (f) { return IMG_TYPES.test(f.type); })) {
      e.preventDefault(); e.stopPropagation();
      insertImages(files);
    }
  });

  // --- bascule HTML ⇄ texte brut ---
  function editorText() {
    // innerText respecte les blocs et les <br> : on obtient des lignes comme à l'écran.
    return editor.innerText.replace(/\u00a0/g, " ").replace(/\r/g, "").replace(/\n{3,}/g, "\n\n").replace(/\s+$/, "") + "\n";
  }
  function escapeHTML(s) { return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }
  if (toggle) toggle.addEventListener("click", function () {
    if (html) {
      if (editor.textContent.trim() !== "" && !window.confirm(toggle.getAttribute("data-confirm"))) return;
      area.value = editorText();
      html = false;
    } else {
      editor.innerHTML = escapeHTML(area.value).replace(/\r?\n/g, "<br>");
      html = true;
    }
    show();
    (html ? editor : area).focus();
    form.dispatchEvent(new Event("input", { bubbles: true }));
  });

  // --- barre d'outils ---
  document.execCommand("styleWithCSS", false, true);
  bar.addEventListener("mousedown", function (e) {
    // garder la sélection de l'éditeur quand on clique un bouton (pas sur les listes déroulantes ni les couleurs)
    if (e.target.closest("button")) e.preventDefault();
  });
  function run(cmd, value) { editor.focus(); document.execCommand(cmd, false, value); }

  function safeURL(u) {
    u = (u || "").trim();
    if (/^mailto:[^\s]+$/i.test(u)) return u;
    if (/^https?:\/\/[^\s]+$/i.test(u)) return u;
    if (/^[^\s:\/]+\.[^\s:\/]+/.test(u) && !/^[a-z][a-z0-9+.-]*:/i.test(u)) return "https://" + u;
    return "";
  }

  bar.addEventListener("click", function (e) {
    var b = e.target.closest("button[data-cmd]");
    if (!b) return;
    var cmd = b.getAttribute("data-cmd");
    if (cmd === "quote") {
      run("formatBlock", "blockquote");
    } else if (cmd === "link") {
      var sel = window.getSelection();
      var saved = sel.rangeCount ? sel.getRangeAt(0).cloneRange() : null;
      var u = window.prompt(b.getAttribute("data-prompt"), "https://");
      if (u === null) return;
      var url = safeURL(u);
      if (!url) { window.alert(b.getAttribute("data-bad")); return; }
      editor.focus();
      if (saved) { sel.removeAllRanges(); sel.addRange(saved); }
      if (sel.isCollapsed) document.execCommand("insertHTML", false, '<a href="' + escapeHTML(url).replace(/"/g, "&quot;") + '">' + escapeHTML(url) + "</a>");
      else document.execCommand("createLink", false, url);
    } else {
      run(cmd);
    }
    form.dispatchEvent(new Event("input", { bubbles: true }));
  });

  bar.querySelectorAll("input[data-color]").forEach(function (inp) {
    inp.addEventListener("input", function () { run(inp.getAttribute("data-color"), inp.value); form.dispatchEvent(new Event("input", { bubbles: true })); });
  });
  var font = bar.querySelector("[data-font]");
  font.addEventListener("change", function () { run("fontName", font.value); });
  var size = bar.querySelector("[data-size]");
  size.addEventListener("change", function () {
    // execCommand ne connaît que 7 tailles : on applique la plus grande puis on la remplace par la taille voulue en px.
    run("fontSize", "7");
    editor.querySelectorAll('font[size="7"], span[style*="xxx-large"]').forEach(function (n) {
      var span = document.createElement("span");
      span.style.fontSize = size.value + "px";
      while (n.firstChild) span.appendChild(n.firstChild);
      n.parentNode.replaceChild(span, n);
    });
    form.dispatchEvent(new Event("input", { bubbles: true }));
  });

  // --- signature liée à l'identité (mode HTML) ---
  var ident = document.getElementById("cp-identity");
  var lastSig = sigHTML();
  function sigHTML() {
    var o = ident && ident.options[ident.selectedIndex];
    return o ? (o.getAttribute("data-signature-html") || "") : "";
  }
  if (ident) ident.addEventListener("change", function () {
    var next = sigHTML();
    if (html) {
      var block = editor.querySelector("[data-wmail-sig]");
      if (block) {
        if (next) block.innerHTML = "-- <br>" + next; else block.remove();
      } else if (next) {
        var d = document.createElement("div");
        d.setAttribute("data-wmail-sig", "1");
        d.innerHTML = "-- <br>" + next;
        editor.appendChild(document.createElement("br"));
        editor.appendChild(d);
      }
    }
    lastSig = next;
  });

  // Réponse : le curseur va au début du texte (au-dessus de la citation).
  if (html && document.getElementById("cp-to").value !== "") {
    editor.focus();
    var r = document.createRange();
    r.setStart(editor, 0); r.collapse(true);
    var s = window.getSelection(); s.removeAllRanges(); s.addRange(r);
  }
})();
