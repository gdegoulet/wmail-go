// Page de composition : champs Cc/Cci/Répondre à à la demande, onglets texte/pièces jointes, envoi des pièces jointes
// avec progression, enregistrement automatique du brouillon, signature liée à l'identité, raccourcis clavier.
(function () {
  "use strict";
  var form = document.getElementById("compose");
  if (!form) return;
  document.querySelectorAll("[data-js-only]").forEach(function (el) { el.hidden = false; });

  var $ = function (id) { return document.getElementById(id); };
  var csrf = form.elements.csrf.value, compId = form.elements.compose_id.value;
  var status = $("cp-status"), savedEl = $("cp-saved"), body = $("cp-bodytext");
  var ident = $("cp-identity");
  var busy = 0;          // envois de pièces jointes en cours
  var submitting = false;
  var dirty = false;

  function showError(msg) {
    status.textContent = msg || "";
    status.hidden = !msg;
  }

  // Réponse : le curseur va au début du texte (au-dessus de la citation).
  if ($("cp-to").value !== "" && !body.hidden && $("cp-pane-text").getAttribute("data-mode") !== "html") { body.focus(); body.setSelectionRange(0, 0); body.scrollTop = 0; }

  // Sur mobile, le clavier des champs d'adresse montre « @ » : chips.js recrée le champ de saisie, on lui
  // repasse l'attribut inputmode posé dans le gabarit.
  ["cp-to", "cp-cc", "cp-bcc", "cp-replyto"].forEach(function (id) {
    var el = $(id);
    if (el && el.chipsInput) el.chipsInput.setAttribute("inputmode", el.getAttribute("inputmode") || "email");
  });

  // --- champs facultatifs ---
  function rowOf(name) { return form.querySelector('[data-row="' + name + '"]'); }
  ["cc", "bcc", "reply_to"].forEach(function (n) {
    var row = rowOf(n), input = row && row.querySelector("input");
    if (row && input && input.value === "") row.hidden = true;
  });
  form.addEventListener("click", function (e) {
    var a = e.target.closest && e.target.closest("[data-show]");
    if (!a) return;
    e.preventDefault();
    var row = rowOf(a.getAttribute("data-show"));
    if (row) { row.hidden = false; var i = row.querySelector("input"); if (i) i.focus(); }
    a.hidden = true;
  });

  // --- onglets ---
  var paneText = $("cp-pane-text"), paneFiles = $("cp-pane-files");
  var tabs = form.querySelectorAll("[data-tab]");
  function showTab(name) {
    tabs.forEach(function (t) { t.classList.toggle("active", t.getAttribute("data-tab") === name); });
    paneText.hidden = name !== "text";
    paneFiles.hidden = name !== "files";
  }
  tabs.forEach(function (t) { t.addEventListener("click", function () { showTab(t.getAttribute("data-tab")); }); });
  paneFiles.hidden = true;
  if (!tabs.length) paneFiles.hidden = false; // sans les onglets, tout reste visible

  // --- signature selon l'identité ---
  var lastSig = sigOf(ident);
  function sigOf(sel) {
    var o = sel && sel.options[sel.selectedIndex];
    return o ? (o.getAttribute("data-signature") || "") : "";
  }
  ident.addEventListener("change", function () {
    var next = sigOf(ident), text = body.value;
    var oldBlock = lastSig ? "-- \n" + lastSig : "";
    if (oldBlock && text.indexOf(oldBlock) >= 0) {
      text = text.replace(oldBlock, next ? "-- \n" + next : "");
    } else if (next && !oldBlock) {
      text = text.replace(/\s*$/, "") + "\n\n-- \n" + next + "\n";
    }
    body.value = text;
    lastSig = next;
    dirty = true;
  });

  // --- pièces jointes ---
  var list = $("cp-atts"), noAtt = $("cp-noatt"), countEl = $("cp-count");
  function refreshCount() {
    var n = list.querySelectorAll("li").length;
    countEl.textContent = n ? n + " " : "";
    noAtt.hidden = n > 0;
  }
  refreshCount();

  function addItem(file) {
    var li = document.createElement("li");
    li.className = "uploading";
    var name = document.createElement("span"); name.className = "name"; name.textContent = file.name;
    var info = document.createElement("span"); info.className = "size"; info.textContent = "0 %";
    var rm = document.createElement("button"); rm.type = "button"; rm.className = "link"; rm.textContent = "×";
    li.appendChild(name); li.appendChild(document.createTextNode(" ")); li.appendChild(info); li.appendChild(document.createTextNode(" ")); li.appendChild(rm);
    list.appendChild(li);
    refreshCount();
    return { li: li, info: info, rm: rm };
  }

  function upload(file) {
    showTab("files");
    var item = addItem(file), xhr = new XMLHttpRequest(), data = new FormData();
    data.append("csrf", csrf); data.append("compose_id", compId); data.append("file", file, file.name);
    busy++;
    item.rm.addEventListener("click", function () { if (busy && !item.li.getAttribute("data-id")) { xhr.abort(); } });
    xhr.open("POST", "/compose/attach");
    xhr.setRequestHeader("X-Requested-With", "fetch");
    xhr.upload.onprogress = function (e) {
      if (e.lengthComputable) item.info.textContent = Math.floor(e.loaded / e.total * 100) + " %";
    };
    function fail(msg) {
      item.li.classList.remove("uploading"); item.li.classList.add("failed");
      item.info.textContent = msg || form.getAttribute("data-attach-error");
      item.rm.onclick = function () { item.li.remove(); refreshCount(); };
    }
    xhr.onload = function () {
      busy--;
      var res = {};
      try { res = JSON.parse(xhr.responseText); } catch (e) { /* réponse inattendue */ }
      if (xhr.status === 200 && res.id) {
        item.li.classList.remove("uploading");
        item.li.setAttribute("data-id", res.id);
        item.info.textContent = res.size;
        item.rm.onclick = function () { detach(res.id, item.li); };
        dirty = true;
      } else {
        fail(res.error);
      }
    };
    xhr.onerror = function () { busy--; fail(); };
    xhr.onabort = function () { busy--; item.li.remove(); refreshCount(); };
    xhr.send(data);
  }

  function detach(id, li) {
    var data = new FormData();
    data.append("csrf", csrf); data.append("compose_id", compId); data.append("id", id);
    fetch("/compose/detach", { method: "POST", body: data, credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
      .then(function () { li.remove(); refreshCount(); dirty = true; });
  }
  list.querySelectorAll("[data-remove]").forEach(function (b) {
    b.addEventListener("click", function () { detach(b.getAttribute("data-remove"), b.closest("li")); });
  });

  function addFiles(files) { Array.prototype.forEach.call(files, upload); }
  $("cp-file").addEventListener("change", function (e) { addFiles(e.target.files); e.target.value = ""; });
  var drop = $("cp-drop");
  ["dragenter", "dragover"].forEach(function (ev) {
    document.addEventListener(ev, function (e) {
      if (e.target && e.target.closest && e.target.closest("#cp-editor:not([hidden])")) return; // images : insérées dans le texte (editor.js)
      if (e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], "Files") >= 0) {
        e.preventDefault(); showTab("files"); drop.classList.add("over");
      }
    });
  });
  document.addEventListener("dragleave", function (e) { if (!e.relatedTarget) drop.classList.remove("over"); });
  document.addEventListener("drop", function (e) {
    if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) {
      e.preventDefault(); drop.classList.remove("over"); addFiles(e.dataTransfer.files);
    }
  });

  // --- envoi, brouillon, abandon ---
  var sendBtn = $("cp-send"), saveBtn = $("cp-save");
  form.addEventListener("submit", function (e) {
    var act = e.submitter && e.submitter.getAttribute("formaction");
    if (act === "/compose/save") { e.preventDefault(); saveDraft(); return; }
    if (act === "/compose/discard") { submitting = true; return; }
    if (busy > 0) { e.preventDefault(); showError(form.getAttribute("data-uploading")); return; }
    var warn = sendWarning();
    if (warn && !window.confirm(warn)) { e.preventDefault(); return; }
    submitting = true;
    sendBtn.disabled = true;
  });
  form.addEventListener("input", function () { dirty = true; });

  // Avertissements avant l'envoi : objet vide, pièce jointe annoncée mais absente, très nombreux destinataires.
  var ATTACH_WORDS = /pi[èe]ces?\s+jointes?|ci[- ]joint|en\s+pi[èe]ce\s+jointe|attach(ed|ment)|enclosed/i;
  function sendWarning() {
    if (form.elements.subject.value.trim() === "") return form.getAttribute("data-warn-subject");
    var text = (form.elements.body ? form.elements.body.value : "").split("\n").filter(function (l) { return l.charAt(0) !== ">"; }).join("\n");
    if (ATTACH_WORDS.test(text) && !document.querySelectorAll("#cp-atts li").length) return form.getAttribute("data-warn-attach");
    var n = 0;
    ["to", "cc", "bcc"].forEach(function (f) { var v = form.elements[f]; if (v && v.value.trim()) n += v.value.split(/[,;]/).filter(function (x) { return x.trim(); }).length; });
    if (n > 20) return form.getAttribute("data-warn-many").replace("%d", String(n));
    return "";
  }

  var saving = false;
  function saveDraft(quiet) {
    if (saving || !saveBtn) return;
    saving = true;
    var data = new FormData(form);
    fetch("/compose/save", { method: "POST", body: data, credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        saving = false;
        if (res.error) { if (!quiet) showError(res.error); return; }
        showError("");
        form.elements.draft_folder.value = res.folder || "";
        form.elements.draft_uid.value = res.uid || "";
        savedEl.textContent = form.getAttribute("data-saved-label") + " " + res.saved;
        dirty = false;
      })
      .catch(function () { saving = false; });
  }
  if (form.getAttribute("data-autosave")) {
    setInterval(function () { if (dirty && busy === 0 && !submitting) saveDraft(true); }, 60000);
  }

  // --- raccourcis ---
  document.addEventListener("keydown", function (e) {
    var mod = e.ctrlKey || e.metaKey;
    if (mod && e.key === "Enter") { e.preventDefault(); if (form.requestSubmit) form.requestSubmit(sendBtn); else sendBtn.click(); }
    else if (mod && String(e.key).toLowerCase() === "s" && saveBtn) { e.preventDefault(); saveDraft(); }
  });

  // --- fermeture : prévenir si des données ne sont pas enregistrées ---
  window.addEventListener("beforeunload", function (e) {
    if (dirty && !submitting) { e.preventDefault(); e.returnValue = ""; }
  });
  $("cp-close").addEventListener("click", function (e) {
    if (dirty && !submitting && !window.confirm(form.getAttribute("data-leave"))) e.preventDefault();
    else submitting = true;
  });
})();

// --- modèles de message : insertion à l'emplacement du curseur ---
(function () {
  "use strict";
  var form = document.getElementById("compose");
  if (!form) return;
  form.querySelectorAll("[data-template]").forEach(function (b) {
    b.addEventListener("click", function () {
      var d = b.closest("details"); if (d) d.open = false;
      fetch("/settings/templates/body?id=" + encodeURIComponent(b.getAttribute("data-template")), { credentials: "same-origin" })
        .then(function (r) { return r.ok ? r.json() : null; })
        .then(function (tpl) {
          if (!tpl) return;
          var text = tpl.text || "";
          var editor = document.getElementById("cp-editor");
          if (editor && !editor.hidden) {
            editor.focus();
            if (tpl.html) {
              document.execCommand("insertHTML", false, tpl.html);
            } else {
              text.split("\n").forEach(function (line, i) {
                if (i > 0) document.execCommand("insertLineBreak");
                if (line) document.execCommand("insertText", false, line);
              });
            }
          } else {
            var area = document.getElementById("cp-bodytext");
            if (!area) return;
            area.focus();
            area.setRangeText(text, area.selectionStart, area.selectionEnd, "end");
          }
          form.dispatchEvent(new Event("input", { bubbles: true }));
        });
    });
  });
})();
