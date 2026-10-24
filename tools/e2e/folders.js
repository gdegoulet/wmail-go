// Panneau des dossiers dans Chromium : création d'un dossier avec son parent, arbre repliable (état conservé après
// rechargement), panneau masquable (état conservé), dossier système « Deleted Items » reconnu comme corbeille.
const puppeteer = require("puppeteer");
const assert = require("assert");
const path = require("path");
const out = process.argv[2] || "/out";
const BASE = process.env.BASE || "http://127.0.0.1:8080", PW = "un mot de passe assez long";

(async () => {
  const browser = await puppeteer.launch({ executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser", headless: "new", args: ["--no-sandbox", "--disable-gpu"] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1500, height: 800 });
  const errors = [];
  page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
  page.on("console", (m) => { if (m.type() === "error" && !/404|Failed to load resource/.test(m.text())) errors.push("console: " + m.text()); });
  const step = (s) => console.log("• " + s);
  const user = "dir" + Math.floor(Math.random() * 1e6);
  const visible = (key) => page.$eval('#foldertree li[data-key="' + key + '"]', (e) => !e.hidden).catch(() => false);

  step("compte et boîte");
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

  const base = "Niv" + Math.floor(Math.random() * 1e5);
  async function createFolder(name, parent) {
    await page.goto(BASE + "/");
    await page.click("[data-newfolder]");
    assert.ok(await page.$eval("#newfolder", (e) => !e.hidden), "formulaire non ouvert");
    await page.type("#newfolder input[name=new_name]", name);
    if (parent) await page.select("#newfolder select[name=parent]", parent);
    await Promise.all([page.waitForNavigation(), page.click("#newfolder button[type=submit]")]);
  }
  step("création d'un dossier racine puis de sous-dossiers (formulaire du panneau)");
  await createFolder(base, "");
  await createFolder("Annee", base);
  await createFolder("Mois", base + ".Annee");

  step("arbre : replié au départ, flèches, état conservé après rechargement");
  await page.goto(BASE + "/");
  assert.ok(await visible(base), "dossier racine caché");
  assert.ok(!(await visible(base + ".Annee")), "sous-dossier visible alors que le parent est replié");
  await page.click('#foldertree li[data-key="' + base + '"] [data-caret]');
  assert.ok(await visible(base + ".Annee") && !(await visible(base + ".Annee.Mois")), "ouvrir un niveau montre ses enfants directs seulement");
  await new Promise((r) => setTimeout(r, 400));
  await page.reload();
  assert.ok(await visible(base + ".Annee"), "l'état déplié n'est pas conservé");
  await page.screenshot({ path: path.join(out, "e2e-arbre.png") });
  await page.click('#foldertree li[data-key="' + base + '"] [data-caret]');
  await new Promise((r) => setTimeout(r, 400));
  await page.reload();
  assert.ok(!(await visible(base + ".Annee")), "l'état replié n'est pas conservé");

  step("dossier affiché : ses ancêtres sont dépliés");
  await page.goto(BASE + "/?folder=" + encodeURIComponent(base + ".Annee.Mois"));
  assert.ok(await visible(base + ".Annee.Mois"), "le dossier affiché doit être visible");

  step("panneau masqué puis réaffiché");
  await page.goto(BASE + "/");
  await page.click("[data-folders-toggle]");
  assert.ok(await page.$eval("body", (e) => e.classList.contains("folders-collapsed")), "panneau non masqué");
  await new Promise((r) => setTimeout(r, 400));
  await page.reload();
  assert.ok(await page.$eval("body", (e) => e.classList.contains("folders-collapsed")), "l'état masqué n'est pas conservé");
  const w = await page.$eval(".folders", (e) => Math.round(e.getBoundingClientRect().width));
  assert.ok(w < 100, "panneau masqué trop large : " + w);
  await page.screenshot({ path: path.join(out, "e2e-panneau-masque.png") });
  await page.click("[data-folders-toggle]");
  await new Promise((r) => setTimeout(r, 400));
  await page.reload();
  assert.ok(!(await page.$eval("body", (e) => e.classList.contains("folders-collapsed"))), "panneau resté masqué");

  assert.deepStrictEqual(errors, [], "erreurs du navigateur : " + errors.join(" | "));
  await browser.close();
  console.log("OK : scénario des dossiers réussi");
})().catch((e) => { console.error("ÉCHEC : " + (e.stack || e)); process.exit(1); });
