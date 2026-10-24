// Demande l'autorisation de notifier au moment où l'utilisateur coche la case (un geste est nécessaire).
(function () {
  "use strict";
  var box = document.getElementById("notify-desktop");
  if (!box) return;
  if (!("Notification" in window)) { box.disabled = true; return; }
  box.addEventListener("change", function () {
    if (box.checked && Notification.permission === "default") {
      Notification.requestPermission().then(function (p) { if (p !== "granted") box.checked = false; });
    } else if (box.checked && Notification.permission === "denied") {
      box.checked = false;
    }
  });
})();
