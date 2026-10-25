// Scénario de la liste dans un vrai navigateur (Chromium sans tête) contre le serveur lancé (make up,
// make mail-up, make mail-seed) : ouverture d'un message, passage à un autre sans rechargement de page,
// puis suppression au clavier. On vérifie que l'action vise bien le message affiché et que le message
// suivant prend sa place, curseur compris. Sort avec un code non nul au premier écart.
// Usage : node liste.js <dossier de captures>
const puppeteer = require("puppeteer");
const assert = require("assert");
const out = process.argv[2] || "/out";
const BASE = process.env.BASE || "http://127.0.0.1:8080", PW = "un mot de passe assez long";

(async () => {
  const browser = await puppeteer.launch({ executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser", headless: "new", args: ["--no-sandbox", "--disable-gpu"] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1600, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
  page.on("console", (m) => { if (m.type() === "error" && !/404|Failed to load resource/.test(m.text())) errors.push("console: " + m.text()); });
  page.on("dialog", (d) => d.accept());
  const step = (s) => console.log("• " + s);
  const user = "e2l" + Math.floor(Math.random() * 1e6);

  step("création du compte et de la boîte");
  await page.goto(BASE + "/register");
  await page.type("input[name=login]", user); await page.type("input[name=password]", PW); await page.type("input[name=password2]", PW);
  await Promise.all([page.waitForNavigation(), page.click("button[type=submit]")]);
  await Promise.all([page.waitForNavigation(), page.click("button[type=submit]")]);
  await page.goto(BASE + "/accounts/new");
  await page.type("input[name=email]", "test@localhost"); await page.keyboard.press("Tab");
  await page.evaluate(() => { document.querySelector("input[name=imap_host]").value = "greenmail"; document.querySelector("input[name=smtp_host]").value = "greenmail"; });
  await page.select("select[name=imap_security]", "none"); await page.select("select[name=smtp_security]", "none");
  await page.evaluate(() => { document.querySelector("input[name=imap_port]").value = "3143"; document.querySelector("input[name=smtp_port]").value = "3025"; document.querySelector("input[name=imap_login]").value = "test"; });
  await page.type("input[name=imap_password]", "testpass");
  await Promise.all([page.waitForNavigation(), page.click("button[name=action][value=save]")]);

  const ordre = () => page.$$eval("li.msgrow", (rs) => rs.map((r) => r.id.slice(1)));
  const affiche = () => page.$eval(".msgview", (v) => v.getAttribute("data-uid")).catch(() => "");
  const champ = (id) => page.$eval("#" + id, (e) => e.value);

  step("ouverture du premier message");
  await page.goto(BASE + "/");
  assert.ok(await page.$(".readpane"), "pas de volet de lecture dans cette disposition");
  const uids = await ordre();
  assert.ok(uids.length >= 6, "liste trop courte : " + uids.length);
  await page.click("#m" + uids[2] + " a.rowlink");
  await page.waitForFunction((u) => { const v = document.querySelector(".msgview"); return v && v.getAttribute("data-uid") === u; }, {}, uids[2]);

  step("passage à un autre message sans rechargement de page");
  await page.click("#m" + uids[3] + " a.rowlink");
  await page.waitForFunction((u) => { const v = document.querySelector(".msgview"); return v && v.getAttribute("data-uid") === u; }, {}, uids[3]);
  assert.strictEqual(await champ("current-uid"), uids[3], "current_uid ne suit pas le message affiché");
  assert.ok((await champ("open-next")).includes("uid=" + uids[4]), "open_next ne désigne pas le voisin du message affiché");

  step("suppression au clavier");
  await Promise.all([page.waitForNavigation(), page.keyboard.press("Delete")]);
  const apres = await ordre();
  assert.ok(!apres.includes(uids[3]), "le message affiché n'a pas été supprimé");
  assert.ok(apres.includes(uids[2]), "un autre message a été supprimé : " + uids[2] + " a disparu");
  assert.strictEqual(await affiche(), uids[4], "le message suivant n'est pas ouvert après la suppression");
  const curseur = await page.$eval("li.msgrow.focused", (r) => r.id.slice(1)).catch(() => "");
  assert.strictEqual(curseur, uids[4], "le curseur ne suit pas le message suivant");

  step("suppression enchaînée : le curseur avance d'une ligne à la fois");
  await Promise.all([page.waitForNavigation(), page.keyboard.press("Delete")]);
  assert.strictEqual(await affiche(), uids[5], "la seconde suppression saute un message");
  await page.screenshot({ path: out + "/liste.png" });

  assert.strictEqual(errors.length, 0, "erreurs de page : " + errors.join(" | "));
  console.log("liste : tous les contrôles passent");
  await browser.close();
})().catch((e) => { console.error("ÉCHEC : " + e.message); process.exit(1); });
