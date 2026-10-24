// Scénario complet dans un vrai navigateur (Chromium sans tête) contre le serveur lancé (make up, make mail-up,
// make mail-seed) : composition HTML, bascule en texte brut, Cc, pièce jointe, brouillon, envoi, copie dans Envoyés,
// réponse et suppression du brouillon. Sort avec un code non nul au premier écart.
// Usage : node compose.js <dossier de captures> [dossier contenant pj.txt]
const puppeteer = require("puppeteer");
const assert = require("assert");
const fs = require("fs"), path = require("path");
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
  const user = "e2e" + Math.floor(Math.random() * 1e6);

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

  step("nouveau message : éditeur HTML actif, Cc à la demande");
  await page.goto(BASE + "/compose");
  assert.strictEqual(await page.$eval("#cp-pane-text", (e) => e.dataset.mode), "html");
  assert.ok(await page.$eval("#cp-editor", (e) => !e.hidden), "éditeur masqué");
  assert.ok(await page.$eval("[data-row=cc]", (e) => e.hidden), "Cc visible d'emblée");
  await page.click("[data-show=cc]");
  assert.ok(!(await page.$eval("[data-row=cc]", (e) => e.hidden)), "Cc non affiché");

  step("mise en forme");
  await page.type("#cp-to-in", "test@localhost,");
  assert.strictEqual(await page.$eval("#cp-to", (e) => e.value), "test@localhost", "adresse non transformée en pastille");
  await page.type("#cp-subject", "E2E " + user);
  await page.click("#cp-editor");
  await page.keyboard.type("Bonjour ");
  await page.click("[data-cmd=bold]"); await page.keyboard.type("gras"); await page.click("[data-cmd=bold]");
  await page.keyboard.type(" fin");
  const html = await page.$eval("#cp-editor", (e) => e.innerHTML);
  assert.ok(/font-weight: ?bold|<b>/.test(html), "gras absent : " + html);

  step("image collée dans l'éditeur");
  await page.evaluate(async () => {
    const png = Uint8Array.from(atob("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="), (c) => c.charCodeAt(0));
    const dt = new DataTransfer();
    dt.items.add(new File([png], "pixel.png", { type: "image/png" }));
    const ed = document.getElementById("cp-editor");
    ed.focus();
    ed.dispatchEvent(new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true }));
  });
  await page.waitForFunction(() => /<img[^>]+src="data:image\/png/.test(document.getElementById("cp-editor").innerHTML), { timeout: 5000 });

  step("pièce jointe");
  await page.click("[data-tab=files]");
  const pj = path.join(__dirname, "pj.txt");
  if (!fs.existsSync(pj)) fs.writeFileSync(pj, "contenu de la pièce jointe\n");
  await (await page.$("#cp-file")).uploadFile(pj);
  await page.waitForSelector("#cp-atts li[data-id]", { timeout: 10000 });
  await page.click("[data-tab=text]");

  step("brouillon enregistré");
  await page.click("#cp-save");
  await page.waitForFunction(() => document.getElementById("cp-saved").textContent.trim() !== "", { timeout: 10000 });
  assert.ok(await page.$eval("[name=draft_uid]", (e) => e.value), "UID du brouillon absent");

  step("bascule texte brut puis HTML");
  await page.click("#cp-mode");
  assert.ok(await page.$eval("#cp-bodytext", (e) => !e.hidden && e.value.includes("Bonjour gras fin")), "texte brut incorrect");
  await page.click("#cp-mode");
  assert.ok(await page.$eval("#cp-editor", (e) => !e.hidden && e.textContent.includes("Bonjour gras fin")));
  await page.click("#cp-editor"); await page.keyboard.down("Control"); await page.keyboard.press("End"); await page.keyboard.up("Control");
  await page.click("[data-cmd=bold]"); await page.keyboard.type(" gras final");

  step("envoi");
  await Promise.all([page.waitForNavigation(), page.click("#cp-send")]);
  assert.ok(/Message (sent|envoy)/i.test(await page.$eval(".flashbar", (e) => e.textContent)), "confirmation absente");

  step("copie dans Envoyés et brouillon supprimé");
  await page.goto(BASE + "/?folder=Sent");
  const subjects = await page.$$eval(".msgrow .subj", (l) => l.map((x) => x.textContent));
  assert.ok(subjects.includes("E2E " + user), "message absent des envoyés : " + subjects);
  await page.goto(BASE + "/?folder=Drafts");
  assert.ok(!(await page.$$eval(".msgrow .subj", (l) => l.map((x) => x.textContent))).includes("E2E " + user), "brouillon encore présent");

  step("réception et réponse");
  await page.goto(BASE + "/");
  const link = await page.evaluate((s) => { for (const a of document.querySelectorAll(".msgrow a.rowlink")) if (a.textContent.includes(s)) return a.getAttribute("href"); return null; }, "E2E " + user);
  assert.ok(link, "message non reçu");
  const uid = /uid=(\d+)/.exec(link)[1];
  await page.goto(BASE + "/compose?mode=reply&folder=INBOX&uid=" + uid);
  assert.strictEqual(await page.$eval("#cp-subject", (e) => e.value), "Re: E2E " + user);
  assert.ok(await page.$eval("#cp-editor", (e) => e.querySelector("blockquote") !== null), "citation absente");
  await page.screenshot({ path: path.join(out, "e2e-reponse.png") });

  assert.deepStrictEqual(errors, [], "erreurs du navigateur : " + errors.join(" | "));
  await browser.close();
  console.log("OK : scénario de composition réussi");
})().catch((e) => { console.error("ÉCHEC : " + (e.stack || e)); process.exit(1); });
