// Dépôt d'un fichier .eml sur la liste : le message est ajouté au dossier affiché (POST /mail/append), puis la liste
// est rechargée. Les autres fichiers sont ignorés.
(function () {
  "use strict";
  var form = document.querySelector("form.msglist-form[data-eml-folder]");
  if (!form || !window.fetch || !window.FormData) return;
  var zone = document.createElement("div");
  zone.className = "eml-drop"; zone.hidden = true;
  zone.textContent = form.getAttribute("data-eml-label");
  form.parentNode.insertBefore(zone, form);

  function hasFiles(e) { return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], "Files") >= 0; }
  function isEml(f) { return /\.eml$/i.test(f.name) || f.type === "message/rfc822"; }

  document.addEventListener("dragenter", function (e) { if (hasFiles(e)) zone.hidden = false; });
  document.addEventListener("dragleave", function (e) { if (!e.relatedTarget) zone.hidden = true; });
  document.addEventListener("dragover", function (e) { if (hasFiles(e) && zone.contains(e.target)) { e.preventDefault(); zone.classList.add("over"); } });
  document.addEventListener("drop", function (e) {
    if (!hasFiles(e) || !zone.contains(e.target)) { zone.hidden = true; return; }
    e.preventDefault();
    zone.hidden = true; zone.classList.remove("over");
    var files = Array.prototype.filter.call(e.dataTransfer.files, isEml);
    if (!files.length) return;
    var pending = files.length, failed = false;
    files.forEach(function (f) {
      var data = new FormData();
      data.append("csrf", form.getAttribute("data-eml-csrf")); data.append("folder", form.getAttribute("data-eml-folder")); data.append("file", f, f.name);
      fetch("/mail/append", { method: "POST", body: data, credentials: "same-origin", headers: { "X-Requested-With": "fetch" } })
        .then(function (r) { if (!r.ok) failed = true; })
        .catch(function () { failed = true; })
        .then(function () {
          if (--pending > 0) return;
          if (failed) window.alert(form.getAttribute("data-eml-error"));
          location.reload();
        });
    });
  });
})();
