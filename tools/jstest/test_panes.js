const { JSDOM, VirtualConsole } = require("jsdom");
const fs = require("fs");
const home = fs.readFileSync("home3.html", "utf8").replace(/<script src="[^"]*list.js"[^>]*><\/script>/, "");
const frag = fs.readFileSync("frag.html", "utf8");
const script = fs.readFileSync("list.js", "utf8");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };
const tick = (ms = 0) => new Promise(r => setTimeout(r, ms));

function boot(opts = {}) {
  const nav = [];
  const vc = new VirtualConsole();
  vc.on("jsdomError", e => { if (/navigation/i.test(String(e.message))) nav.push(String(e.message)); });
  const dom = new JSDOM(home, { url: opts.url || "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true, virtualConsole: vc });
  const w = dom.window, d = w.document;
  w.HTMLFormElement.prototype.submit = function () {};
  const calls = [];
  w.fetch = opts.fetch || ((url, o) => { calls.push({ url, o }); return Promise.resolve({ redirected: false, text: () => Promise.resolve(frag) }); });
  if (opts.wrapFetch) { const f = w.fetch; w.fetch = (u, o) => { calls.push({ url: u, o }); return f(u, o); }; }
  const lp = d.querySelector(".msglist-form");
  Object.defineProperty(lp, "scrollTop", { value: opts.scrollTop || 0, writable: true, configurable: true });
  if (opts.storage) for (const [k, v] of Object.entries(opts.storage)) w.sessionStorage.setItem(k, v);
  if (!w.PointerEvent) w.PointerEvent = w.MouseEvent;
  if (opts.layout) opts.layout(w, d);
  w.eval(script);
  return { dom, w, d, calls, nav, lp };
}
const click = (w, el, init = {}) => { const e = new w.MouseEvent("click", { bubbles: true, cancelable: true, button: 0, ...init }); el.dispatchEvent(e); return e; };
const rows = d => [...d.querySelectorAll(".msgrow")];
const badgeNums = d => [...d.querySelectorAll(".folder-list li.active .badge, .filters a:last-child .badge")].map(b => parseInt(b.textContent, 10));

(async () => {
  console.log("== volet de lecture sans rechargement de la liste");
  {
    const { w, d, calls, lp } = boot({ scrollTop: 800 });
    const row = rows(d).find(r => r.textContent.includes("Offre spéciale"));
    const uid = row.id.slice(1);
    const before = badgeNums(d);
    const e = click(w, row.querySelector("a.rowlink"));
    ok(e.defaultPrevented, "le clic sur une ligne est intercepté (pas de rechargement)");
    await tick(20);
    ok(calls.length === 1 && calls[0].url.startsWith("/mail/pane?") && calls[0].url.includes("uid=" + uid), "un seul appel : " + (calls[0] && calls[0].url));
    ok(calls[0].o.credentials === "same-origin" && calls[0].o.headers["X-Requested-With"] === "fetch", "cookie de session envoyé, en-tête X-Requested-With");
    ok(d.querySelector(".readpane .msgview") !== null, "le volet de lecture contient le message");
    ok(lp.scrollTop === 800, "la liste n'a pas bougé (scrollTop = " + lp.scrollTop + ")");
    ok(row.classList.contains("selected") && row.querySelector("a.rowlink").getAttribute("aria-current") === "true", "ligne mise en évidence");
    ok(rows(d).filter(r => r.classList.contains("selected")).length === 1, "une seule ligne sélectionnée");
    ok(!row.classList.contains("unread"), "la ligne passe en « lu »");
    const after = badgeNums(d);
    ok(before.length > 0 && after.every((n, i) => n === before[i] - 1), "compteurs de non lus décrémentés : " + before + " → " + after);
    ok(w.location.search.includes("uid=" + uid), "adresse mise à jour (historique) : " + w.location.search);
    ok(d.querySelector(".mailbox").classList.contains("has-message"), "classe has-message ajoutée");
    ok(d.getElementById("current-uid").value === d.querySelector(".msgview").getAttribute("data-uid") && d.getElementById("current-uid").value !== "", "uid du message ouvert mémorisé pour les actions de la barre d'outils");
    ok(d.querySelector("form.msglist-form input[name=stay]").value.includes("uid=" + uid), "adresse de retour mise à jour (l'étoile d'une ligne garde le message ouvert)");
    ok([...d.querySelectorAll("[data-needs-selection]")].every(b => !b.disabled), "boutons d'action activés : un message est ouvert");
    ok(fs.existsSync("frag.html") && frag.includes('data-just-read="1"') === d.querySelector(".msgview").hasAttribute("data-just-read"), "le fragment annonce la lecture");
  }
  {
    const { w, d, calls } = boot();
    const a = d.querySelector(".msgrow a.rowlink");
    for (const init of [{ ctrlKey: true }, { metaKey: true }, { shiftKey: true }, { altKey: true }, { button: 1 }]) {
      const e = click(w, a, init);
      if (e.defaultPrevented) ok(false, "clic avec " + JSON.stringify(init) + " intercepté à tort");
    }
    ok(calls.length === 0, "clic avec modificateur ou bouton du milieu : navigation normale (nouvel onglet possible)");
  }
  {
    const pending = [];
    const { w, d } = boot({ fetch: (url) => new Promise(res => pending.push({ url, res })) });
    const rs = rows(d);
    click(w, rs[1].querySelector("a.rowlink")); click(w, rs[2].querySelector("a.rowlink"));
    pending[1].res({ redirected: false, text: () => Promise.resolve('<div class="msgview" data-uid="B">DEUX</div>') });
    await tick(10);
    pending[0].res({ redirected: false, text: () => Promise.resolve('<div class="msgview" data-uid="A">UN</div>') });
    await tick(10);
    ok(d.querySelector(".readpane").textContent.includes("DEUX") && !d.querySelector(".readpane").textContent.includes("UN"), "deux clics rapprochés : la réponse la plus récente gagne");
  }
  {
    const { w, d, nav } = boot({ fetch: () => Promise.reject(new Error("réseau")) });
    click(w, d.querySelector(".msgrow a.rowlink")); await tick(20);
    ok(nav.length === 1, "échec réseau : repli sur la navigation classique");
  }
  {
    const { w, d, nav } = boot({ fetch: () => Promise.resolve({ redirected: true, text: () => Promise.resolve("<html>connexion</html>") }) });
    click(w, d.querySelector(".msgrow a.rowlink")); await tick(20);
    ok(nav.length === 1 && !d.querySelector(".readpane").textContent.includes("connexion"), "session expirée (redirection) : navigation vers la page de connexion");
  }
  {
    const { w, d, calls } = boot();
    click(w, d.querySelector(".msgrow a.rowlink")); await tick(20);
    const sw = d.querySelector(".readpane a[data-pane]");
    if (!sw) { ok(false, "aucun lien data-pane dans le fragment testé"); }
    else {
      const e = click(w, sw); await tick(20);
      ok(e.defaultPrevented && calls.length === 2 && calls[1].url.startsWith("/mail/pane?"), "lien du volet (texte brut / contenu distant) chargé dans le volet : " + calls[1].url);
    }
  }

  console.log("== position de défilement de la liste");
  {
    const { w, d, lp } = boot({ scrollTop: 0 });
    lp.scrollTop = 250; lp.dispatchEvent(new w.Event("scroll")); await tick(100);
    const saved = JSON.parse(w.sessionStorage.getItem("wmail.listScroll"));
    ok(saved && saved.top === 250 && saved.sig === "/?", "défilement enregistré : " + JSON.stringify(saved));
  }
  {
    const { lp } = boot({ storage: { "wmail.listScroll": JSON.stringify({ sig: "/?", top: 321 }) } });
    ok(lp.scrollTop === 321, "défilement restauré après rechargement (même liste) : " + lp.scrollTop);
  }
  {
    const { lp } = boot({ url: "http://127.0.0.1:8080/?uid=7&view=text", storage: { "wmail.listScroll": JSON.stringify({ sig: "/?", top: 410 }) } });
    ok(lp.scrollTop === 410, "uid et view ne comptent pas dans la signature de la liste");
  }
  {
    const { lp } = boot({ url: "http://127.0.0.1:8080/?folder=Sent", storage: { "wmail.listScroll": JSON.stringify({ sig: "/?", top: 500 }) } });
    ok(lp.scrollTop === 0, "autre dossier : défilement non restauré");
  }
  {
    const { lp } = boot({ storage: { "wmail.listScroll": "pas du json" } });
    ok(lp.scrollTop === 0, "valeur enregistrée illisible : ignorée");
  }

  console.log("== volets redimensionnables");
  const layout = (w, d) => {
    const mb = d.querySelector(".mailbox");
    Object.defineProperty(mb, "clientWidth", { value: 1400, configurable: true });
    const def = { ".folders": ["--folders-w", 224], ".listpane": ["--list-w", 420] };
    for (const [sel, [v, dflt]] of Object.entries(def)) {
      const el = d.querySelector(sel);
      el.getBoundingClientRect = () => { const raw = w.document.documentElement.style.getPropertyValue(v); return { width: raw ? parseFloat(raw) : dflt }; };
    }
  };
  const ptr = (w, el, type, x) => el.dispatchEvent(new w.MouseEvent(type, { bubbles: true, cancelable: true, button: 0, clientX: x }));
  const root = d => d.documentElement.style;
  {
    const { w, d, calls } = boot({ layout });
    const mb = d.querySelector(".mailbox"), a = d.querySelector(".splitter.a"), b = d.querySelector(".splitter.b");
    ok(mb.classList.contains("resizable"), "classe resizable ajoutée");
    ok(a.getAttribute("aria-valuenow") === "224" && b.getAttribute("aria-valuenow") === "420", "valeurs ARIA initiales");
    ptr(w, a, "pointerdown", 100); ok(d.body.classList.contains("dragging"), "classe dragging pendant le glissement");
    ptr(w, a, "pointermove", 180);
    ok(root(d).getPropertyValue("--folders-w") === "304px", "glisser de +80 px : 304 px (" + root(d).getPropertyValue("--folders-w") + ")");
    ok(a.getAttribute("aria-valuenow") === "304", "aria-valuenow mis à jour");
    ptr(w, a, "pointerup", 180);
    ok(!d.body.classList.contains("dragging"), "fin du glissement");
    ok(calls.length === 1 && calls[0].url === "/settings/panes" && calls[0].o.method === "POST", "enregistrement : POST /settings/panes");
    const body = calls[0].o.body;
    ok(body.get("folders") === "304" && !body.has("list") && body.get("csrf") === mb.getAttribute("data-csrf") && body.get("csrf").length > 10, "corps : folders=304, csrf présent, list absent");
    ok(calls[0].o.headers["X-Requested-With"] === "fetch", "en-tête X-Requested-With: fetch");

    ptr(w, b, "pointerdown", 500); ptr(w, b, "pointermove", 2500);
    ok(root(d).getPropertyValue("--list-w") === "764px", "liste bornée pour garder 320 px à la lecture : " + root(d).getPropertyValue("--list-w"));
    ptr(w, b, "pointermove", -2500); ok(root(d).getPropertyValue("--list-w") === "240px", "largeur minimale de la liste : 240 px");
    ptr(w, b, "pointerup", -2500);
    ptr(w, a, "pointerdown", 0); ptr(w, a, "pointermove", 5000);
    ok(root(d).getPropertyValue("--folders-w") === "480px", "largeur maximale des dossiers : 480 px");
    ptr(w, a, "pointermove", -5000); ok(root(d).getPropertyValue("--folders-w") === "160px", "largeur minimale des dossiers : 160 px");
    ptr(w, a, "pointerup", -5000);
    const n = calls.length;
    ptr(w, a, "pointermove", 300);
    ok(calls.length === n && root(d).getPropertyValue("--folders-w") === "160px", "pas de déplacement sans bouton enfoncé");
    ptr(w, a, "pointerdown", 0); a.dispatchEvent(new w.MouseEvent("pointerdown", { bubbles: true, button: 2 }));
    ptr(w, a, "pointerup", 0);
  }
  {
    const { w, d, calls } = boot({ layout });
    const a = d.querySelector(".splitter.a");
    a.dispatchEvent(new w.KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true, cancelable: true }));
    ok(root(d).getPropertyValue("--folders-w") === "240px", "flèche droite : +16 px");
    const e = new w.KeyboardEvent("keydown", { key: "ArrowRight", shiftKey: true, bubbles: true, cancelable: true }); a.dispatchEvent(e);
    ok(root(d).getPropertyValue("--folders-w") === "304px" && e.defaultPrevented, "Maj + flèche droite : +64 px");
    a.dispatchEvent(new w.KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true, cancelable: true }));
    ok(root(d).getPropertyValue("--folders-w") === "288px", "flèche gauche : -16 px");
    const other = new w.KeyboardEvent("keydown", { key: "a", bubbles: true, cancelable: true }); a.dispatchEvent(other);
    ok(!other.defaultPrevented, "les autres touches ne sont pas interceptées");
    ok(calls.length === 0, "pas d'enregistrement à chaque frappe");
    await tick(500);
    ok(calls.length === 1 && calls[0].o.body.get("folders") === "288", "un seul enregistrement différé, valeur finale 288 : " + calls.length);
    a.dispatchEvent(new w.MouseEvent("dblclick", { bubbles: true }));
    ok(root(d).getPropertyValue("--folders-w") === "" && calls.length === 2 && calls[1].o.body.get("folders") === "0", "double-clic : valeur par défaut rétablie et enregistrée (folders=0)");
  }
  {
    const { d } = boot({ url: "http://127.0.0.1:8080/", fetch: undefined });
    ok(true, "chargement sans erreur");
  }

  console.log(fails === 0 ? "\nTOUS LES CONTRÔLES PASSENT" : "\n" + fails + " ÉCHEC(S)");
  process.exit(fails ? 1 : 0);
})();
