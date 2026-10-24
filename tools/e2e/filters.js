// Scénario des filtres dans Chromium contre un vrai Dovecot (make sieve-up) : activation de ManageSieve sur la boîte,
// création, modification, désactivation, réordonnancement et suppression de filtres, script brut, filtre « similaire ».
const puppeteer = require("puppeteer");
const assert = require("assert");
const BASE = process.env.BASE || "http://127.0.0.1:8080", PW = "un mot de passe assez long";

(async () => {
  const browser = await puppeteer.launch({ executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser", headless: "new", args: ["--no-sandbox", "--disable-gpu"] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1400, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
  page.on("console", (m) => { if (m.type() === "error" && !/404|Failed to load resource/.test(m.text())) errors.push("console: " + m.text()); });
  page.on("dialog", (d) => d.accept());
  const step = (s) => console.log("• " + s);
  const user = "flt" + Math.floor(Math.random() * 1e6);
  const rows = () => page.$$eval(".filters-table tbody tr", (r) => r.map((x) => x.querySelector("td").textContent.trim().replace(/\s+/g, " ")));

  step("compte avec ManageSieve activé");
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
  await page.type("input[name=sieve_host]", "dovecot"); await page.type("input[name=sieve_port]", "34190");
  await page.select("select[name=sieve_security]", "none");
  await page.type("input[name=sieve_login]", user); await page.type("input[name=sieve_password]", "testpass");
  await Promise.all([page.waitForNavigation(), page.click("button[name=action][value=save]")]);

  step("liste vide puis premier filtre");
  await page.goto(BASE + "/settings/filters");
  const start = await page.$eval(".setcontent", (e) => e.textContent.replace(/\s+/g, " "));
  assert.ok(/Aucun filtre|No filters/.test(start), "liste non vide au départ : " + start);
  await page.goto(BASE + "/settings/filters/edit");
  await page.type("input[name=name]", "Factures");
  await page.type("input[name=cond_value]", "compta@example.org");
  await page.select("#action", "MoveTo");
  await page.select("select[name=move_to]", "INBOX");
  await Promise.all([page.waitForNavigation(), page.click("#filter-form button[type=submit]")]);
  assert.deepStrictEqual(await rows(), ["Factures"]);

  step("second filtre avec condition ajoutée (JavaScript)");
  await page.goto(BASE + "/settings/filters/edit");
  await page.type("input[name=name]", "Absence");
  await page.click("#add-cond");
  const values = await page.$$("input[name=cond_value]");
  await values[0].type("boss@example.org");
  await page.select("#action", "Vacation");
  await page.type("textarea[name=vac_msg]", "Je suis absent jusqu'à lundi.");
  await page.type("input[name=vac_subject]", "Absent");
  await Promise.all([page.waitForNavigation(), page.click("#filter-form button[type=submit]")]);
  assert.deepStrictEqual(await rows(), ["Factures", "Absence"]);

  step("ordre, désactivation");
  await Promise.all([page.waitForNavigation(), page.click("tbody tr:nth-child(2) button[value=up]")]);
  assert.deepStrictEqual(await rows(), ["Absence", "Factures"]);
  await Promise.all([page.waitForNavigation(), page.click("tbody tr:nth-child(1) button[value=toggle]")]);
  assert.ok((await rows())[0].includes("désactivé") || (await rows())[0].includes("disabled"), "filtre non désactivé");

  step("script brut (vérifié par le vrai serveur)");
  await page.goto(BASE + "/settings/filters/raw");
  const raw = await page.$eval("textarea[name=raw]", (e) => e.value);
  assert.ok(raw.includes("RAINLOOP:SIEVE") && raw.includes("vacation"), "script brut inattendu");
  await page.$eval("textarea[name=raw]", (e) => { e.value = "require [\"fileinto\"];\nif header :contains \"Subject\" { fileinto \"x\"; }\n"; });
  await page.click("input[name=use_raw]");
  await Promise.all([page.waitForNavigation(), page.click("form[action$=\"/raw\"] button[type=submit]")]);
  assert.ok((await page.content()).includes("Dovecot") === false && /refus|refused|NO/i.test(await page.$eval(".msg.error", (e) => e.textContent)), "script invalide non refusé");

  step("suppression");
  await page.goto(BASE + "/settings/filters");
  for (let i = 0; i < 2; i++) {
    await page.click("tbody tr:first-child details.danger summary");
    await Promise.all([page.waitForNavigation(), page.click("tbody tr:first-child details.danger button.danger")]);
  }
  const after = await page.$eval(".setcontent", (e) => e.textContent.replace(/\s+/g, " "));
  assert.ok(/Aucun filtre|No filters/.test(after), "liste non vide après suppression : " + after);

  step("filtre prérempli depuis un message");
  await page.goto(BASE + "/settings/filters/edit?from=alice%40example.org");
  assert.strictEqual(await page.$eval("input[name=cond_value]", (e) => e.value), "alice@example.org");

  assert.deepStrictEqual(errors, [], "erreurs du navigateur : " + errors.join(" | "));
  await browser.close();
  console.log("OK : scénario des filtres réussi");
})().catch((e) => { console.error("ÉCHEC : " + (e.stack || e)); process.exit(1); });
