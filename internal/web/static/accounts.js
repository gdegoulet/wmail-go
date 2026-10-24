// Aide à la saisie du formulaire de compte mail (sans elle, le formulaire fonctionne quand même).
(function () {
  "use strict";
  var form = document.getElementById("account-form");
  if (!form) return;
  var f = form.elements;
  var ports = {
    imap: { tls: "993", starttls: "143", none: "143" },
    smtp: { tls: "465", starttls: "587", none: "25" }
  };

  // Suggestion de serveurs à partir de l'adresse : imap.domaine, smtp.domaine, login = adresse.
  f.email.addEventListener("change", function () {
    var m = /^[^@\s]+@([^@\s]+)$/.exec(f.email.value.trim());
    if (!m) return;
    var domain = m[1].toLowerCase();
    if (!f.imap_host.value) f.imap_host.value = "imap." + domain;
    if (!f.smtp_host.value) f.smtp_host.value = "smtp." + domain;
    if (!f.imap_login.value) f.imap_login.value = f.email.value.trim();
  });

  // Quand la sécurité change, le port suit s'il avait encore la valeur par défaut de l'ancienne sécurité.
  ["imap", "smtp"].forEach(function (kind) {
    var sec = f[kind + "_security"], port = f[kind + "_port"];
    var last = sec.value;
    sec.addEventListener("change", function () {
      if (port.value === ports[kind][last] || port.value === "") port.value = ports[kind][sec.value];
      last = sec.value;
    });
  });
})();
