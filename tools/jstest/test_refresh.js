// Rafraîchissement de la liste (listrefresh.js) : nouveaux messages et bouton actualiser.
const { JSDOM } = require("jsdom");
const fs = require("fs");
const html = fs.readFileSync("home3.html", "utf8").replace(/<script src="[^"]*"[^>]*><\/script>/g, "");
let fails = 0;
const ok = (c, m) => { console.log((c ? "OK   " : "ECHEC") + " " + m); if (!c) fails++; };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
(async () => {
  const dom = new JSDOM(html, { url: "http://127.0.0.1:8080/", runScripts: "outside-only", pretendToBeVisual: true });
  const w = dom.window, d = w.document;
  // la « nouvelle » page : la même, avec une ligne en plus en tête et une ligne retirée
  const fresh = html.replace('<ul class="msgrows">', '<ul class="msgrows"><li id="m9999" class="msgrow unread"><span class="sidebar"></span><div class="wrapper"><span class="checkedParent"><label class="chk"><input type="checkbox" name="uid" value="9999"></label></span><a class="rowlink" href="/?uid=9999"><span class="subj">Tout nouveau</span></a></div></li>');
  let calls = [];
  w.fetch = (url, o) => { calls.push(url); return Promise.resolve({ ok: true, redirected: false, text: () => Promise.resolve(fresh) }); };
  w.eval(fs.readFileSync("listrefresh.js", "utf8"));
  const ul = d.querySelector("ul.msgrows");
  const before = ul.querySelectorAll(".msgrow").length;
  const second = ul.querySelectorAll(".msgrow")[1].querySelector("input[name=uid]");
  second.checked = true;
  const secondValue = second.value;
  d.dispatchEvent(new w.Event("wmail:newmail"));
  await sleep(50);
  const rows = d.querySelectorAll("ul.msgrows .msgrow");
  ok(rows.length === before + 1 && rows[0].id === "m9999", "la nouvelle ligne apparaît en tête (" + rows.length + " lignes)");
  ok(d.querySelector('input[name=uid][value="' + secondValue + '"]').checked, "la case cochée est conservée");
  ok(calls.length === 1 && !/uid=/.test(calls[0]), "un seul chargement, sans le message ouvert : " + calls[0]);
  // pas de rafraîchissement automatique en page 2 ou en recherche
  w.history.pushState(null, "", "/?page=2");
  d.dispatchEvent(new w.Event("wmail:newmail"));
  w.history.pushState(null, "", "/?q=abc");
  d.dispatchEvent(new w.Event("wmail:newmail"));
  await sleep(50);
  ok(calls.length === 1, "page 2 et recherche ne sont pas rechargées d'office");
  // le bouton actualiser recharge sans naviguer
  w.history.pushState(null, "", "/");
  const btn = d.createElement("a"); btn.setAttribute("data-refresh", ""); btn.href = "/"; d.body.appendChild(btn);
  const ev = new w.MouseEvent("click", { bubbles: true, cancelable: true }); btn.dispatchEvent(ev);
  ok(ev.defaultPrevented, "le clic sur actualiser ne navigue pas");
  await sleep(50);
  ok(calls.length === 2, "le bouton recharge la liste");
  console.log(fails ? "ÉCHECS : " + fails : "TOUS LES CONTRÔLES PASSENT");
  process.exit(fails ? 1 : 0);
})();
