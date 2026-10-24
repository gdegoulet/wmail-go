// Réglages enregistrés à la volée (settings-auto.js) et vignettes de thèmes (settings.html).
const { JSDOM } = require("jsdom");
const fs = require("fs");
const script = fs.readFileSync("settings-auto.js", "utf8");
const gabarit = fs.readFileSync("settings.html", "utf8");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };

// Page de réglages reproduite d'après le gabarit (les contrôles de gabarit ci-dessous vérifient
// qu'ils restent en accord : le serveur n'est pas interrogé ici).
const page = `<!doctype html><html lang="fr"><body>
<form method="post" action="/settings/preferences" data-autosave data-saved="Enregistré" data-save-error="Échec">
<input type="hidden" name="csrf" value="jeton42">
<label id="l-lang">Langue
<select name="language"><option value="fr" selected>Français</option><option value="en">English</option></select></label>
<fieldset class="theme-field">
<legend class="theme-legend">Thème</legend>
<div class="theme-grid">
<label class="theme-tile on"><input type="radio" name="theme" value="rainloop" checked><span class="theme-prev" data-theme-prev="rainloop"></span><span class="theme-name">DarkShine</span></label>
<label class="theme-tile"><input type="radio" name="theme" value="light"><span class="theme-prev" data-theme-prev="light"></span><span class="theme-name">Clair</span></label>
<label class="theme-tile"><input type="radio" name="theme" value="dark"><span class="theme-prev" data-theme-prev="dark"></span><span class="theme-name">Sombre</span></label>
</div>
</fieldset>
<label id="l-page">Messages par page
<input name="page_size" type="number" min="10" max="500" value="50" required></label>
<label class="check" id="l-mark"><input name="mark_read" type="checkbox"> Marquer comme lu</label>
<label class="check" id="l-threads"><input name="threads" type="checkbox" checked> Conversations</label>
<button type="submit" id="save">Enregistrer</button>
</form></body></html>`;

function boot(reponse) {
  const dom = new JSDOM(page, { url: "http://127.0.0.1:8080/settings", runScripts: "outside-only" });
  const w = dom.window, d = w.document, appels = [], minuteries = [];
  w.fetch = (url, o) => {
    appels.push({ url, body: String(o && o.body), entetes: (o && o.headers) || {}, methode: o && o.method });
    return reponse === "rejet" ? Promise.reject(new Error("hors ligne")) : Promise.resolve({ ok: reponse !== "ko", status: reponse === "ko" ? 400 : 200 });
  };
  // Minuteries capturées : la disparition de la coche est déclenchée à la main.
  w.setTimeout = (fn, ms) => { minuteries.push({ fn, ms }); return minuteries.length; };
  w.clearTimeout = (id) => { if (minuteries[id - 1]) minuteries[id - 1].annulee = true; };
  w.eval(script);
  return { w, d, appels, minuteries };
}
const attendre = () => new Promise((r) => setImmediate(() => setImmediate(r)));
const coche = (d, sel) => d.querySelector(sel + " > .saved-tick");

(async function () {
  // --- gabarit : la grille de thèmes a remplacé la liste déroulante, sans changer le champ envoyé ---
  {
    ok(!/<select name="theme"/.test(gabarit), "plus de liste déroulante pour le thème");
    ok(/name="theme" value="\{\{\.\}\}"/.test(gabarit) && /type="radio"/.test(gabarit), "le thème est un bouton radio nommé « theme »");
    ok(/class="theme-grid"/.test(gabarit) && /class="theme-prev"/.test(gabarit), "grille et aperçus présents");
    ok(/data-autosave/.test(gabarit), "le formulaire des préférences est marqué data-autosave");
    ok(/settings-auto\.js/.test(gabarit) && /settings-auto\.css/.test(gabarit), "script et feuille de style chargés par la page");
    ok(/<button type="submit">\{\{\.T "settings\.prefs\.submit"\}\}<\/button>/.test(gabarit), "le bouton « Enregistrer » est conservé");
  }

  // --- enregistrement à la volée d'un champ ordinaire ---
  {
    const { d, appels, minuteries } = boot();
    d.querySelector("select[name=language]").value = "en";
    d.querySelector("select[name=language]").dispatchEvent(new d.defaultView.Event("change", { bubbles: true }));
    ok(appels.length === 1, appels.length + " envoi déclenché par le changement");
    const a = appels[0];
    ok(a.url === "/settings/preferences" && a.methode === "POST", "POST vers " + a.url);
    ok(a.entetes["X-Requested-With"] === "fetch", "en-tête X-Requested-With: fetch");
    const corps = new URLSearchParams(a.body);
    ok(corps.get("language") === "en" && corps.get("csrf") === "jeton42" && corps.get("page_size") === "50", "tout le formulaire est envoyé : " + a.body);
    ok(corps.get("theme") === "rainloop", "le bouton radio coché est joint");
    ok(corps.get("mark_read") === null && corps.get("threads") === "on", "cases décochées omises, cochées envoyées");
    ok(!/submit|Enregistrer/.test(a.body), "le bouton n'est pas envoyé comme champ");
    await attendre();
    const t = coche(d, "#l-lang");
    ok(!!t && t.textContent === "✓" && !t.classList.contains("err"), "coche verte à côté du champ");
    ok(t.getAttribute("title") === "Enregistré", "libellé traduit : " + (t && t.getAttribute("title")));
    const m = minuteries[minuteries.length - 1];
    ok(m && m.ms === 2000, "coche effacée après 2 s (" + (m && m.ms) + " ms)");
    m.fn();
    ok(!coche(d, "#l-lang"), "la coche disparaît");
    ok(!d.getElementById("save").disabled, "le bouton « Enregistrer » reste utilisable");
  }

  // --- case à cocher : un seul envoi, coche dans l'étiquette ---
  {
    const { d, appels } = boot();
    const c = d.querySelector("input[name=mark_read]");
    c.checked = true;
    c.dispatchEvent(new d.defaultView.Event("change", { bubbles: true }));
    await attendre();
    ok(new URLSearchParams(appels[0].body).get("mark_read") === "on", "la case cochée est envoyée");
    ok(!!coche(d, "#l-mark"), "coche affichée dans l'étiquette de la case");
  }

  // --- thème : la vignette choisie est mise en évidence et enregistrée ---
  {
    const { d, appels } = boot();
    const sombre = d.querySelector("input[name=theme][value=dark]");
    sombre.checked = true;
    sombre.dispatchEvent(new d.defaultView.Event("change", { bubbles: true }));
    await attendre();
    ok(new URLSearchParams(appels[0].body).get("theme") === "dark", "theme=dark envoyé");
    const tuiles = d.querySelectorAll(".theme-tile");
    ok(!tuiles[0].classList.contains("on") && tuiles[2].classList.contains("on"), "la vignette du thème courant est mise en évidence");
    ok(!!coche(d, ".theme-legend"), "coche à côté du titre « Thème »");
  }

  // --- réponse en erreur : croix rouge, elle reste affichée ---
  {
    const { d, minuteries } = boot("ko");
    d.querySelector("select[name=language]").dispatchEvent(new d.defaultView.Event("change", { bubbles: true }));
    await attendre();
    const t = coche(d, "#l-lang");
    ok(!!t && t.textContent === "✗" && t.classList.contains("err"), "croix rouge quand le serveur refuse");
    ok(t.getAttribute("title") === "Échec", "libellé d'échec traduit");
    ok(minuteries.length === 0, "la croix n'est pas effacée au bout de 2 s");
    ok(!d.getElementById("save").disabled, "le bouton « Enregistrer » reste utilisable après un échec");
  }

  // --- panne réseau : croix rouge aussi, sans exception ---
  {
    const { d } = boot("rejet");
    d.querySelector("select[name=language]").dispatchEvent(new d.defaultView.Event("change", { bubbles: true }));
    await attendre();
    const t = coche(d, "#l-lang");
    ok(!!t && t.classList.contains("err"), "croix rouge quand le réseau tombe");
  }

  // --- champ invalide : rien n'est envoyé ---
  {
    const { d, appels } = boot();
    const n = d.querySelector("input[name=page_size]");
    n.value = "9999";
    n.dispatchEvent(new d.defaultView.Event("change", { bubbles: true }));
    await attendre();
    ok(appels.length === 0, "une valeur hors bornes n'est pas envoyée");
    ok(!!coche(d, "#l-page") && coche(d, "#l-page").classList.contains("err"), "croix rouge sur le champ invalide");
  }

  console.log(fails === 0 ? "\nTOUS LES CONTRÔLES PASSENT" : "\n" + fails + " ÉCHEC(S)");
  if (fails) process.exit(1);
})();
