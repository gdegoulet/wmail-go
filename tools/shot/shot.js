// Captures d'écran de l'interface avec Chromium sans tête (puppeteer) : crée un compte, ajoute la boîte GreenMail
// de démonstration, puis photographie la liste et la lecture. Usage : node shot.js <dossier de sortie> [largeur] [hauteur]
const puppeteer = require("puppeteer");
const out = process.argv[2] || "/out";
const width = parseInt(process.argv[3] || "1800", 10), height = parseInt(process.argv[4] || "800", 10);
const BASE = process.env.BASE || "http://127.0.0.1:8080";
const PW = "un mot de passe assez long";

(async () => {
  const browser = await puppeteer.launch({ executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser", headless: "new", args: ["--no-sandbox", "--disable-gpu"] });
  const page = await browser.newPage();
  await page.setViewport({ width, height, deviceScaleFactor: parseFloat(process.env.DSF || "1") });
  const user = "shot" + Math.floor(Math.random() * 100000);
  await page.goto(BASE + "/register");
  await page.type("input[name=login]", user);
  await page.type("input[name=password]", PW);
  await page.type("input[name=password2]", PW);
  await Promise.all([page.waitForNavigation(), page.click("button[type=submit]")]);
  await Promise.all([page.waitForNavigation(), page.click("button[type=submit]")]); // clé de récupération
  await page.goto(BASE + "/accounts/new");
  await page.type("input[name=email]", "test@localhost");
  await page.keyboard.press("Tab"); // déclenche la suggestion automatique des serveurs
  await page.evaluate(() => { document.querySelector("input[name=imap_host]").value = "greenmail"; document.querySelector("input[name=smtp_host]").value = "greenmail"; });
  await page.select("select[name=imap_security]", "none"); await page.select("select[name=smtp_security]", "none");
  await page.evaluate(() => { document.querySelector("input[name=imap_port]").value = "3143"; document.querySelector("input[name=smtp_port]").value = "3025"; });
  await page.evaluate(() => { const l = document.querySelector("input[name=imap_login]"); l.value = "test"; });
  await page.type("input[name=imap_password]", "testpass");
  await Promise.all([page.waitForNavigation(), page.click("button[name=action][value=save]")]);
  await page.screenshot({ path: out + "/01-liste.png" });

  const rows = await page.$$(".msgrow a.rowlink");
  const target = process.env.ROW ? parseInt(process.env.ROW, 10) : 1;
  if (rows[target]) { await rows[target].click(); await new Promise(r => setTimeout(r, 1200)); }
  await page.screenshot({ path: out + "/02-lecture.png" });
  await page.setViewport({ width: 1100, height: 760, deviceScaleFactor: 1 });
  await page.screenshot({ path: out + "/03-etroit.png" });
  await page.setViewport({ width: 420, height: 800, deviceScaleFactor: 1 });
  await page.screenshot({ path: out + "/04-mobile.png" });
  await browser.close();
  console.log("captures dans " + out);
})().catch(e => { console.error(e); process.exit(1); });
