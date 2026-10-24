// Éditeur de texte riche léger pour les champs qui portent « data-rich » (signatures, modèles de message).
// Le champ d'origine reste la source : il garde « :HTML:<html> » en mode mis en forme, le texte tel quel sinon.
// Sans JavaScript, la zone de texte ordinaire reste utilisable.
(function () {
  "use strict";
  var PREFIX = ":HTML:";

  function escapeHTML(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }

  function button(label, cmd, title) {
    var b = document.createElement("button");
    b.type = "button";
    b.innerHTML = label;
    b.title = title || "";
    b.setAttribute("aria-label", b.title);
    b.setAttribute("data-cmd", cmd);
    return b;
  }

  document.querySelectorAll("textarea[data-rich]").forEach(function (area) {
    var wrap = document.createElement("div");
    wrap.className = "rt";
    var bar = document.createElement("div");
    bar.className = "rt-bar";
    var editor = document.createElement("div");
    editor.className = "rt-editor";
    editor.contentEditable = "true";
    editor.setAttribute("role", "textbox");
    editor.setAttribute("aria-multiline", "true");
    if (area.getAttribute("aria-label")) editor.setAttribute("aria-label", area.getAttribute("aria-label"));

    var labels = (area.getAttribute("data-rich-labels") || "").split("|");
    var L = function (i, d) { return labels[i] || d; };
    [["<b>B</b>", "bold", L(0, "Gras")], ["<i>I</i>", "italic", L(1, "Italique")], ["<u>U</u>", "underline", L(2, "Souligné")],
     ["•", "insertUnorderedList", L(3, "Liste")], ["1.", "insertOrderedList", L(4, "Liste numérotée")],
     ["🔗", "link", L(5, "Lien")], ["T<sub>x</sub>", "removeFormat", L(6, "Effacer la mise en forme")]].forEach(function (b) {
      bar.appendChild(button(b[0], b[1], b[2]));
    });
    var toggle = button(L(7, "texte brut"), "toggle", L(8, "Basculer entre texte brut et mise en forme"));
    toggle.className = "rt-toggle";
    bar.appendChild(toggle);

    area.parentNode.insertBefore(wrap, area);
    wrap.appendChild(bar);
    wrap.appendChild(editor);
    wrap.appendChild(area);

    // Mise en forme par défaut pour un champ vide (nouveau modèle, nouvelle signature) ; un contenu déjà en texte brut le reste.
    var rich = area.value.indexOf(PREFIX) === 0 || area.value.trim() === "";
    function show() {
      editor.hidden = !rich;
      bar.classList.toggle("plain", !rich);
      area.hidden = rich;
      toggle.textContent = rich ? L(7, "texte brut") : L(9, "mise en forme");
    }
    if (rich) editor.innerHTML = area.value.slice(PREFIX.length);
    show();

    bar.addEventListener("click", function (e) {
      var b = e.target.closest ? e.target.closest("button[data-cmd]") : null;
      if (!b) return;
      e.preventDefault();
      var cmd = b.getAttribute("data-cmd");
      if (cmd === "toggle") {
        if (rich) {
          area.value = editor.innerText.replace(/ /g, " ").replace(/\n{3,}/g, "\n\n").replace(/\s+$/, "");
          rich = false;
        } else {
          editor.innerHTML = escapeHTML(area.value).replace(/\r?\n/g, "<br>");
          rich = true;
        }
        show();
        if (rich) editor.focus(); else area.focus();
        return;
      }
      if (!rich) { rich = true; editor.innerHTML = escapeHTML(area.value).replace(/\r?\n/g, "<br>"); show(); }
      editor.focus();
      if (cmd === "link") {
        var url = window.prompt(area.getAttribute("data-link-prompt") || "https://");
        if (!url) return;
        if (!/^(https?:|mailto:)/i.test(url)) { window.alert(area.getAttribute("data-link-bad") || "https:// ?"); return; }
        document.execCommand("createLink", false, url);
        return;
      }
      document.execCommand(cmd, false, null);
    });

    var form = area.form;
    if (!form) return;
    function sync() { if (rich) area.value = PREFIX + editor.innerHTML; }
    form.addEventListener("submit", sync, true);
    form.addEventListener("formdata", sync);
  });
})();
