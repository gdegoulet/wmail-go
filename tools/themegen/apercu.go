package main

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// Apercu rend une page autonome qui montre le thème et permet de le retoucher.
//
// Autonome veut dire : aucun serveur, aucune dépendance, le fichier s'ouvre depuis le disque. Le fond
// d'écran y est donc référencé de façon relative (l'image est à côté), alors que le CSS exporté le réfère de
// façon absolue comme l'exige l'application. La page fait la conversion dans les deux sens.
//
// Ce n'est pas une copie de l'interface : c'est une maquette qui porte les mêmes classes et les mêmes
// variables, assez fidèle pour juger des couleurs et des contrastes, assez courte pour rester lisible.
func (t *Theme) Apercu(ext string) string {
	type varJS struct {
		Nom     string `json:"nom"`
		Valeur  string `json:"valeur"`
		Couleur bool   `json:"couleur"`
		Role    string `json:"role"`
		Groupe  string `json:"groupe"`
	}
	var vars []varJS
	for _, g := range t.Groupes {
		for _, v := range g.Variables {
			vars = append(vars, varJS{v.Nom, v.Valeur, v.Couleur != nil, v.Role, g.Titre})
		}
	}
	type ctrlJS struct {
		Quoi  string  `json:"quoi"`
		Texte string  `json:"texte"`
		Fond  string  `json:"fond"`
		Vise  float64 `json:"vise"`
	}
	var ctrls []ctrlJS
	for _, c := range t.Controles {
		ctrls = append(ctrls, ctrlJS{c.Quoi, c.Texte, c.Fond, c.Vise})
	}
	// Le fond d'écran composé sert de référence pour les deux contrôles posés sur l'image.
	fondEcran := Melange(moyenne(t.Palette), Couleur{0, 0, 0}, .45)

	jvars, _ := json.Marshal(vars)
	jctrls, _ := json.Marshal(ctrls)

	return `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Thème ` + html.EscapeString(t.Nom) + ` — aperçu</title>
<style>
` + apercuCSS + `
</style>
</head>
<body>
<header class="tg-head">
  <h1>Thème « ` + html.EscapeString(t.Nom) + ` »</h1>
  <p>Modifiez une couleur : l'aperçu suit. Les contrastes sont recalculés à chaque changement.
     Le bouton <b>CSS final</b> donne le fichier à copier dans <code>theme.css</code>.</p>
  <div class="tg-actions">
    <button type="button" id="tg-css">CSS final</button>
    <button type="button" id="tg-reset">Revenir au thème généré</button>
    <label class="tg-inline"><input type="checkbox" id="tg-focus"> montrer la ligne au clavier</label>
  </div>
</header>

<div class="tg-wrap">
<main class="tg-demo">
` + apercuMaquette + `
</main>

<aside class="tg-panel">
  <section id="tg-contrastes"><h2>Contrastes</h2><div id="tg-liste-contrastes"></div></section>
  <section id="tg-vars"><h2>Couleurs</h2></section>
</aside>
</div>

<dialog id="tg-dialog">
  <h2>theme.css</h2>
  <p class="tg-note">Chemins d'image rétablis en absolu, comme l'exige l'application.</p>
  <textarea id="tg-sortie" rows="22" spellcheck="false" readonly></textarea>
  <div class="tg-actions">
    <button type="button" id="tg-copier">Copier</button>
    <button type="button" id="tg-fermer">Fermer</button>
  </div>
</dialog>

<script>
"use strict";
var NOM = ` + jsonStr(t.Nom) + `;
var EXT = ` + jsonStr(ext) + `;
var SOMBRE = ` + fmt.Sprint(strings.Contains(t.CSS(true), "color-scheme: dark")) + `;
var VARS = ` + string(jvars) + `;
var CONTROLES = ` + string(jctrls) + `;
var FOND_ECRAN = ` + jsonStr(fondEcran.Hex()) + `;
` + apercuJS + `
</script>
</body>
</html>
`
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// apercuCSS : l'habillage de la page d'aperçu elle-même, volontairement neutre pour ne pas se confondre
// avec le thème présenté. Les couleurs du thème ne sortent pas de .tg-demo.
const apercuCSS = `
*, *::before, *::after { box-sizing: border-box; }
body { margin: 0; font: 14px/1.5 system-ui, sans-serif; color: #1a1a1a; background: #f4f5f7; }
.tg-head { padding: 1rem 1.25rem .75rem; background: #fff; border-bottom: 1px solid #dcdfe3; }
.tg-head h1 { margin: 0 0 .25rem; font-size: 1.15rem; }
.tg-head p { margin: 0 0 .6rem; color: #555; max-width: 70ch; }
.tg-actions { display: flex; gap: .5rem; align-items: center; flex-wrap: wrap; }
.tg-actions button { padding: .35rem .8rem; font: inherit; background: #fff; border: 1px solid #b9bec4; border-radius: .25rem; cursor: pointer; }
.tg-actions button:hover { background: #eef0f2; }
.tg-inline { display: inline-flex; gap: .35rem; align-items: center; color: #555; }
.tg-wrap { display: flex; align-items: flex-start; gap: 1rem; padding: 1rem; flex-wrap: wrap; }
.tg-demo { flex: 1 1 42rem; min-width: 22rem; }
.tg-panel { flex: 0 0 22rem; max-height: calc(100vh - 2rem); overflow: auto; background: #fff; border: 1px solid #dcdfe3; border-radius: .3rem; padding: .75rem; position: sticky; top: 1rem; }
.tg-panel h2 { margin: 0 0 .5rem; font-size: .95rem; text-transform: uppercase; letter-spacing: .04em; color: #666; }
.tg-panel section + section { margin-top: 1rem; }
.tg-grp { margin: .8rem 0 .3rem; font-size: .8rem; font-weight: 700; color: #888; text-transform: uppercase; }
.tg-row { display: grid; grid-template-columns: 2rem 1fr; gap: .5rem; align-items: center; padding: .2rem 0; }
.tg-row input[type=color] { width: 2rem; height: 1.7rem; padding: 0; border: 1px solid #b9bec4; border-radius: .2rem; background: none; cursor: pointer; }
.tg-row.libre { grid-template-columns: 1fr; }
.tg-row input[type=text] { width: 100%; font: 12px/1.4 ui-monospace, monospace; padding: .25rem; border: 1px solid #b9bec4; border-radius: .2rem; }
.tg-row label { font-size: .78rem; line-height: 1.25; }
.tg-row code { display: block; font-size: .72rem; color: #777; }
.tg-ctrl { display: flex; justify-content: space-between; gap: .5rem; padding: .15rem 0; font-size: .78rem; border-bottom: 1px solid #f0f1f3; }
.tg-ctrl b { font-variant-numeric: tabular-nums; }
.tg-ctrl.faible { color: #a3251d; font-weight: 600; }
.tg-ctrl.faible b::after { content: " ⚠"; }
#tg-dialog { width: min(52rem, 94vw); border: 1px solid #b9bec4; border-radius: .3rem; }
#tg-dialog h2 { margin: 0 0 .25rem; font-size: 1rem; }
.tg-note { margin: 0 0 .5rem; font-size: .8rem; color: #666; }
#tg-sortie { width: 100%; font: 12px/1.45 ui-monospace, monospace; }

/* --- la maquette : à partir d'ici, tout est peint par le thème --- */
.wm { font: 11px/1.45 "Helvetica Neue", Helvetica, Arial, sans-serif; border-radius: .3rem; overflow: hidden; border: 1px solid #c9ced3; }
.wm-wall { background: var(--wall); background-size: cover; padding: .5rem; }
.wm-bar { display: flex; align-items: center; gap: .5rem; padding: .55rem .6rem; background: var(--bar-bg); color: var(--bar-fg); border-radius: .2rem; }
.wm-bar .wm-user { margin-left: auto; }
.wm-btn { padding: .25rem .7rem; font: inherit; color: var(--btn-fg); background: var(--btn-bg); border: 1px solid var(--btn-bd); border-radius: .2rem; cursor: default; }
.wm-btn.new { color: var(--new-fg); background: var(--new-bg); border-color: var(--new-bd); font-weight: 700; }
.wm-cols { display: grid; grid-template-columns: 11rem 1fr 1fr; gap: .4rem; margin-top: .4rem; }
.wm-folders { color: var(--folders-fg); padding: .4rem .2rem; }
.wm-folders div { display: flex; align-items: center; padding: .25rem .4rem; border-radius: .2rem; }
.wm-folders .on { background: var(--folders-sel); font-weight: 700; }
.wm-folders .off { color: var(--folders-disabled); }
.wm-folders hr { margin: .5rem .3rem; border: 0; border-bottom: 1px solid var(--folders-sep); }
.wm-badge { margin-left: auto; padding: 0 .45rem; border-radius: .6rem; background: var(--badge-bg); color: var(--badge-fg); font-size: .85em; font-weight: 700; }
.wm-list { background: var(--list-bg); border-radius: .2rem; display: flex; flex-direction: column; min-height: 17rem; }
.wm-tb { padding: .45rem .5rem; background: var(--toolbar-bg); color: var(--fg); border-bottom: 1px solid var(--bar-line); }
.wm-rows { flex: 1; }
.wm-row { display: flex; gap: .4rem; padding: .35rem .5rem; background: var(--row-bg); border-bottom: 1px solid var(--row-bd); border-left: .3rem solid transparent; }
.wm-row .who { color: var(--sender); font-weight: 700; }
.wm-row .subj { color: var(--row-fg); }
.wm-row .when { margin-left: auto; color: var(--date); }
.wm-row .star { color: var(--star); }
.wm-row.unread { background: var(--unread-bg); border-left-color: var(--unread-bar); }
.wm-row.subunread { background: var(--subunread-bg); border-left-color: var(--subunread-bar); }
.wm-row.sel { background: var(--sel-bg); border-left-color: var(--sel-bar); }
.wm-row.star-on .star { color: var(--star-on); }
.wm-row:hover { background: var(--row-hover); }
body.tg-focus .wm-row.sel { box-shadow: inset 0 0 0 2px var(--focus-ring), inset 0 0 0 100vw var(--focus-tint); }
.wm-foot { padding: .45rem .5rem; background: var(--toolbar-bg); color: var(--muted); border-top: 1px solid var(--bar-line); }
.wm-read { background: var(--read-bg); color: var(--read-fg); border-radius: .2rem; display: flex; flex-direction: column; }
.wm-mh { padding: .5rem; border-bottom: 1px solid var(--bar-line); }
.wm-mh .subj { font-size: 1.15em; }
.wm-mh a { color: var(--link); }
.wm-body { flex: 1; padding: .6rem; background: #fff; color: #000; border-top: 1px solid var(--bar-line); }
.wm-body .iso { font-size: .9em; color: #666; }
.wm-banner { display: flex; gap: .5rem; padding: .45rem .6rem; }
.wm-banner.gray { background: var(--bar-gray); color: var(--fg); border-bottom: 1px solid var(--bar-line); }
.wm-banner.warn { background: var(--warn-bg); color: var(--warn-fg); }
.wm-banner.info { background: var(--info-bg); color: var(--info-fg); }
.wm-banner.danger { background: var(--danger-bg); color: var(--danger-fg); }
.wm-extra { display: grid; grid-template-columns: 1fr 1fr; gap: .4rem; margin-top: .4rem; }
.wm-menu { background: var(--menu-bg); color: var(--menu-fg); border: 1px solid var(--border); border-radius: .2rem; padding: .3rem 0; }
.wm-menu div { padding: .2rem .9rem; }
.wm-menu .on { background: var(--menu-hover-bg); color: var(--menu-hover-fg); }
.wm-card { background: var(--card-bg); color: var(--fg); border: 1px solid var(--border); border-radius: .2rem; padding: .6rem; }
.wm-card input { width: 100%; margin: .2rem 0; padding: .25rem; color: var(--fg); background: var(--input-bg); border: 1px solid var(--border); border-radius: .2rem; }
.wm-card .muted { color: var(--muted); }
.wm-login { margin: .4rem auto; max-width: 17rem; padding: 1rem; color: var(--login-fg); background: var(--login-bg); border: var(--login-border); border-radius: var(--login-radius); box-shadow: var(--login-shadow); }
.wm-login h3 { margin: 0 0 .5rem; font-weight: normal; color: var(--login-title); text-shadow: var(--login-title-shadow); }
.wm-login input { width: 100%; margin: .2rem 0; padding: .3rem; border: 1px solid var(--border); background: var(--input-bg); color: var(--fg); }
.wm-login a { color: var(--login-link); font-size: .9em; }
`

const apercuMaquette = `
<div class="wm">
 <div class="wm-wall">
  <div class="wm-bar">
   <button class="wm-btn new" type="button">Nouveau message</button>
   <button class="wm-btn" type="button">Actualiser</button>
   <button class="wm-btn" type="button">Déplacer</button>
   <span class="wm-user">vous@example.org</span>
  </div>
  <div class="wm-cols">
   <nav class="wm-folders">
    <div class="on">Boîte de réception <span class="wm-badge">12</span></div>
    <div>Envoyés</div>
    <div>Brouillons <span class="wm-badge">3</span></div>
    <div>Indésirables</div>
    <div class="off">Corbeille</div>
    <hr>
    <div>Projets</div>
    <div>Factures</div>
   </nav>
   <section class="wm-list">
    <div class="wm-tb">Rechercher dans ce dossier</div>
    <div class="wm-rows">
     <div class="wm-row unread star-on"><span class="who">Camille Roux</span><span class="subj">Réunion de lundi</span><span class="when">11:42</span><span class="star">★</span></div>
     <div class="wm-row subunread"><span class="who">Équipe</span><span class="subj">Re: compte rendu (4)</span><span class="when">10:15</span><span class="star">☆</span></div>
     <div class="wm-row sel"><span class="who">Dominique</span><span class="subj">Devis signé</span><span class="when">hier</span><span class="star">☆</span></div>
     <div class="wm-row"><span class="who">Lettre d'information</span><span class="subj">Nouveautés du mois</span><span class="when">2 nov.</span><span class="star">☆</span></div>
     <div class="wm-row"><span class="who">Facturation</span><span class="subj">Votre reçu</span><span class="when">28 oct.</span><span class="star">☆</span></div>
    </div>
    <div class="wm-foot">43 message(s) — 1 2 3 … 9</div>
   </section>
   <section class="wm-read">
    <div class="wm-mh"><div class="subj">Devis signé</div><div><a href="#">dominique@example.org</a> — hier 16:08</div></div>
    <div class="wm-banner gray">Images distantes bloquées. <a href="#">Afficher</a></div>
    <div class="wm-banner warn">Ce message présente des anomalies de fabrication.</div>
    <div class="wm-body">Bonjour,<br>ci-joint le devis signé.<br><br><span class="iso">Le corps d'un message garde son propre fond : il est servi dans un iframe isolé, et aucun thème ne le teinte.</span></div>
   </section>
  </div>
  <div class="wm-extra">
   <div class="wm-menu"><div>Répondre</div><div class="on">Répondre à tous</div><div>Transférer</div><div>Ajouter aux contacts</div></div>
   <div class="wm-card">Réglages <input value="valeur d'un champ"><div class="muted">Texte secondaire d'explication.</div>
    <div class="wm-banner info">Enregistré.</div><div class="wm-banner danger">Identifiants refusés.</div></div>
  </div>
  <div class="wm-login"><h3>Connexion</h3><input placeholder="identifiant"><input type="password" placeholder="mot de passe"><button class="wm-btn new" type="button">Entrer</button><br><a href="#">Mot de passe oublié</a></div>
 </div>
</div>
`

const apercuJS = `
var racine = document.documentElement;
var etat = {};
VARS.forEach(function (v) { etat[v.nom] = v.valeur; });

// Le fichier s'ouvre depuis le disque : l'image est à côté, pas sous /themes/<nom>/.
function versApercu(valeur) { return valeur.split("/themes/" + NOM + "/").join(""); }
function versCSS(valeur) {
  return valeur.split('url("background.').join('url("/themes/' + NOM + '/background.')
               .split("url(background.").join("url(/themes/" + NOM + "/background.");
}

function appliquer() {
  Object.keys(etat).forEach(function (nom) { racine.style.setProperty(nom, versApercu(etat[nom])); });
  calculerContrastes();
}

// --- contraste (mêmes formules que couleur.go) ---
function canal(v) { v /= 255; return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }
function lum(hex) {
  var m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return null;
  var n = parseInt(m[1], 16);
  return 0.2126 * canal((n >> 16) & 255) + 0.7152 * canal((n >> 8) & 255) + 0.0722 * canal(n & 255);
}
function contraste(a, b) {
  var la = lum(a), lb = lum(b);
  if (la === null || lb === null) return null;
  if (la < lb) { var t = la; la = lb; lb = t; }
  return (la + 0.05) / (lb + 0.05);
}

function calculerContrastes() {
  var hote = document.getElementById("tg-liste-contrastes");
  hote.textContent = "";
  CONTROLES.forEach(function (c) {
    var fond = c.fond === "(fond d'écran)" ? FOND_ECRAN : etat[c.fond];
    var r = contraste(etat[c.texte], fond);
    var el = document.createElement("div");
    el.className = "tg-ctrl" + (r !== null && r < c.vise ? " faible" : "");
    var quoi = document.createElement("span");
    quoi.textContent = c.quoi;
    var val = document.createElement("b");
    val.textContent = r === null ? "—" : r.toFixed(2) + ":1";
    el.appendChild(quoi); el.appendChild(val);
    hote.appendChild(el);
  });
}

// --- panneau de réglage ---
(function construirePanneau() {
  var hote = document.getElementById("tg-vars"), groupe = null;
  VARS.forEach(function (v) {
    if (v.groupe !== groupe) {
      groupe = v.groupe;
      var h = document.createElement("div");
      h.className = "tg-grp";
      h.textContent = groupe;
      hote.appendChild(h);
    }
    var ligne = document.createElement("div");
    ligne.className = "tg-row";
    var champ = document.createElement("input");
    if (v.couleur) {
      champ.type = "color";
      champ.value = v.valeur;
    } else {
      champ.type = "text";
      champ.value = v.valeur;
    }
    champ.addEventListener("input", function () {
      etat[v.nom] = champ.value;
      appliquer();
    });
    var lab = document.createElement("label");
    lab.textContent = v.role;
    var code = document.createElement("code");
    code.textContent = v.nom;
    lab.appendChild(code);
    if (v.couleur) { ligne.appendChild(champ); ligne.appendChild(lab); }
    else { ligne.className = "tg-row libre"; ligne.appendChild(lab); ligne.appendChild(champ); }
    hote.appendChild(ligne);
  });
})();

document.getElementById("tg-focus").addEventListener("change", function (e) {
  document.body.classList.toggle("tg-focus", e.target.checked);
});

document.getElementById("tg-reset").addEventListener("click", function () {
  VARS.forEach(function (v) { etat[v.nom] = v.valeur; });
  document.getElementById("tg-vars").textContent = "";
  location.reload();
});

document.getElementById("tg-css").addEventListener("click", function () {
  var l = ['/* Thème « ' + NOM + ' ». Les 65 variables et ce qu\'elles peignent : docs/themes.md. */',
           ":root {", "  color-scheme: " + (SOMBRE ? "dark" : "light") + ";"];
  var groupe = null;
  VARS.forEach(function (v) {
    if (v.groupe !== groupe) { groupe = v.groupe; l.push("", "  /* " + groupe + " */"); }
    l.push("  " + v.nom + ": " + versCSS(etat[v.nom]) + ";");
  });
  l.push("}", "");
  document.getElementById("tg-sortie").value = l.join("\n");
  document.getElementById("tg-dialog").showModal();
});

document.getElementById("tg-copier").addEventListener("click", function () {
  var zone = document.getElementById("tg-sortie");
  zone.select();
  if (navigator.clipboard) { navigator.clipboard.writeText(zone.value); } else { document.execCommand("copy"); }
  this.textContent = "Copié";
  var self = this;
  setTimeout(function () { self.textContent = "Copier"; }, 1500);
});
document.getElementById("tg-fermer").addEventListener("click", function () {
  document.getElementById("tg-dialog").close();
});

appliquer();
`
