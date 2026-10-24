// Carnet d'adresses dans un vrai navigateur : création d'un contact, autocomplétion dans la composition (clavier et
// souris), sélecteur de contacts, ajout automatique après envoi. Usage : node contacts.js <dossier de captures>
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

  step("création d'un contact");
  await page.goto(BASE + "/contacts?new=1");
  await page.type("input[name=first]", "Zoë"); await page.type("input[name=last]", "Dupont");
  await page.type("input[name=email]", "zoe@example.org");
  await Promise.all([page.waitForNavigation(), page.click(".contact-form button[type=submit]")]);
  assert.ok(page.url().includes("info=saved"), "enregistrement : " + page.url());

  step("autocomplétion au clavier");
  await page.goto(BASE + "/compose");
  await page.type("#cp-to-in", "zo");
  await page.waitForSelector(".ac-list li", { visible: true });
  await page.keyboard.press("ArrowDown"); await page.keyboard.press("Enter");
  assert.strictEqual(await page.$eval("#cp-to", (e) => e.value), "Zoë Dupont <zoe@example.org>");
  assert.strictEqual(await page.$$eval("#cp-to-in", (l) => l[0].parentNode.querySelectorAll(".chip").length), 1, "pastille absente");
  assert.strictEqual(await page.$eval("#cp-to-in", (e) => e.value), "");

  step("sélecteur de contacts");
  await page.click(".chip-x");
  assert.strictEqual(await page.$eval("#cp-to", (e) => e.value), "", "pastille non retirée");
  await page.click("#cp-pick");
  await page.waitForSelector("dialog.picker[open] .picker-list button");
  await page.click("dialog.picker[open] .picker-list button");
  assert.ok((await page.$eval("#cp-to", (e) => e.value)).includes("zoe@example.org"));
  await page.click("dialog.picker[open] > button");

  step("envoi puis ajout automatique du destinataire inconnu");
  await page.click(".chip-x");
  await page.type("#cp-to-in", "Nouveau <nouveau@example.org>");
  await page.type("input[name=subject]", "Test contacts");
  await page.screenshot({ path: path.join(out, "e2e-contacts.png") });
  await Promise.all([page.waitForNavigation(), page.click("#cp-send")]);
  await page.goto(BASE + "/contacts?q=nouveau");
  assert.ok((await page.content()).includes("nouveau@example.org"), "contact non ajouté");

  step("export vCard");
  const vcf = await page.evaluate(async () => (await fetch("/contacts/export?format=vcf")).text());
  assert.ok(vcf.includes("BEGIN:VCARD") && vcf.includes("zoe@example.org"));

  await browser.close();
  if (errors.length) { console.error(errors.join("\n")); process.exit(1); }
  console.log("OK : scénario des contacts réussi");
})().catch((e) => { console.error(e); process.exit(1); });
