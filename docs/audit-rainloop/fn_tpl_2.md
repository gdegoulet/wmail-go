# Analyse élément par élément : templates RainLoop 1.17 (Settings*, Popups Account/Identity/Template/Filter/Contacts/TwoFactor*, Common, Components) contre wmail-go

Source lue : `/Work/webmail_project/rainloop/1.17.0/app/templates/Views/{User,Common,Components}/` (fichiers de la tranche lus en entier). Pour les listes d'options (mise en page, messages par page, déconnexion auto, types de condition de filtre) j'ai aussi lu les définitions dans `static/js/app.js` ; pour les libellés, `app/localization/webmail/en_US.yml`.
wmail-go lu : `internal/web/templates/{settings,accounts,account_form,identities,templates,contacts,filters,filter_edit,filters_raw,folders_settings,twofactor,base}.html`, `internal/web/{contactsweb,filters,handlers,identities,templates_settings}.go`, `internal/web/static/{keys,listnav,list,filters,accounts,notify-setting,folders,msgfull}.js`, `app.css`, `mail.css`, `themes/rainloop/theme.css`, `internal/sieve/filters.go`, `internal/i18n/locales/fr.json`.
Non lu / limites : les CSS de base de RainLoop (`static/css/app.css`, less Bootstrap 2) ne font pas partie de la tranche : les remarques de rendu viennent de la structure des templates (classes `btn`, `control-group`, `icon-*`, `form-horizontal`) et de `out.css` (variables de thème seulement). Les contrôleurs JS de RainLoop (tranche voisine) ne sont lus que pour les listes d'options. Aucun fichier projet modifié.

Légende : OK / PARTIEL / MANQUANT / SANS OBJET. Priorités P1 (quotidien, très visible) / P2 / P3.

Constat d'ensemble sur le rendu : dans RainLoop les réglages utilisent `form-horizontal` (libellé à gauche, contrôle à droite), des boutons Bootstrap `.btn` (dégradé blanc, icône + libellé), des cases à cocher en police d'icônes (`Checkbox.html`), un indicateur de sauvegarde automatique par champ (`SaveTrigger`) et des suppressions « corbeille puis bouton rouge Êtes-vous sûr ? ». wmail-go a un menu à gauche + volet blanc avec la même structure (mail.css:306-320) mais des formulaires empilés, un seul bouton « Enregistrer », des cases natives et des `<details>` pour les suppressions.

---

## 1. SettingsGeneral.html

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Général » (3-5) | Titre de section | Titre | settings.html:2 `<h1 class="legend">`, mail.css:317 | OK | |
| Identité (6-16) : nom de l'identité principale cliquable (pointillé), ouvre la popup identité | Voir/modifier d'un clic l'identité par défaut | Accès rapide à nom, reply-to, bcc, signature de l'identité principale | Aucun sur la page Général. Identité principale modifiable seulement via Comptes > Modifier (identities.html:5-6 renvoie vers `/accounts/{id}/edit`) | PARTIEL | Ajouter en tête de Général une ligne « Identité : Nom <adresse> » liée à `/accounts/<actif>/edit`. P3 |
| Langue (17-34) : drapeau + nom, clic ouvre PopupsLanguages, déclencheur de sauvegarde | Choix de langue avec drapeaux | Changer la langue | settings.html:9-10 `<select name="language">` (fr, en), sans drapeaux, enregistré avec le bouton global | PARTIEL | Rendu différent (pas de drapeau/popup). 2 langues seulement : un select suffit. P3 |
| Mise en page (35-51), masquée mobile : Select `layoutTypes` = Sans division / Division verticale / Division horizontale | Choix du volet de lecture | Layout aucun / à droite / en bas | settings.html:19-23 : `three` (3 volets) et `list` (liste seule) ; handlers.go:404 refuse toute autre valeur ; mail.go:488 `threePane` | PARTIEL | « Division horizontale » (aperçu en bas) MANQUANT. Ajouter `bottom` (grille liste au-dessus, lecture dessous, séparateur horizontal mémorisé comme les largeurs). P2 |
| Éditeur par défaut (52-68) : Html / Plain / Html (forced) / Plain (forced) | Éditeur de rédaction | Défaut de l'éditeur | settings.html:27-33, 4 valeurs identiques, libellés fr.json:341-344 | OK | |
| Messages par page (69-85) : Select [10,20,30,50,100], largeur 80 | Taille de page | Page size | settings.html:24-25 `<input type=number min=10 max=500>` (défaut dans userdata) | OK | Contrôle différent (nombre libre au lieu d'une liste), fonction équivalente. P3 si on veut la même liste |
| Case « Toujours afficher les images externes » (88-94) `showImages` | Ne jamais bloquer les images distantes | Réglage global d'affichage des images | Pas sur Général : option par compte `block_remote` (account_form.html:100) et expéditeurs de confiance (settings.html:51-52) | PARTIEL | Fonction présente mais ailleurs et inversée. Ajouter à Général un rappel/lien ou déplacer l'option. P2 |
| Case « Afficher des cases dans la liste » (95-101, hide-on-mobile) | Cases de sélection | `useCheckboxesInList` | settings.html:35 `show_checkboxes` ; mail.css:`body.hide-checks .checkedParent` | OK | |
| Saut de ligne puis case « Utiliser les fils » (102-109, si threadsAllowed) | Conversations | Threads | settings.html:37 `threads`, voir threads_test.go | OK | |
| Case « Placer les réponses dans le dossier du message d'origine » (110-116) | Réponses dans le même dossier | replySameFolder | settings.html:41 | OK | |
| Légende « Notifications » (120-123) visible si notifications/son supportés | Section séparée | Regrouper les notifications | settings.html:39-40 : deux cases dans la liste générale, sans légende ni masquage | PARTIEL | Ajouter une légende « Notifications » et masquer si `Notification` indisponible (notify-setting.js:6 se contente de désactiver). P3 |
| Case notification bureau + texte « (Bloqué par le navigateur) » si refusée (126-139) | Informer du blocage navigateur | Retour utilisateur si permission refusée | notify-setting.js:9-13 : décoche silencieusement, aucun message | PARTIEL | Afficher le texte `(Bloqué par le navigateur)` à côté de la case si `Notification.permission==="denied"`. P3 |
| Case son + bouton vert ▶ « tester le son » (141-154) | Écouter le son d'alerte | `testSoundNotification` | Case présente (settings.html:40), aucun bouton de test (grep `playSound|Audio` dans static/*.js : aucun) | MANQUANT | Ajouter un bouton ▶ (JS : `new Audio(<son embarqué>).play()`). P3 |
| Indicateurs de sauvegarde (SaveTrigger) à côté de chaque champ | Retour visuel immédiat, enregistrement à chaque changement | Auto-enregistrement | Un seul bouton « Enregistrer » (settings.html:42) | PARTIEL | Option : enregistrement au `change` via fetch + coche verte. P3 |
| Ordre des champs : identité, langue, mise en page, éditeur, par page, cases | Ordonnancement | Ressemblance | Notre ordre : langue, thème, inactivité, durée max, taille texte, mise en page, page, lu à l'ouverture, éditeur, cases (settings.html:9-41) | PARTIEL | Réordonner et grouper (Général / Notifications). P3 |
| Champs propres à wmail-go (thème, inactivité, durée de session, taille du texte, marquer lu, brouillons auto, mailto, expéditeurs de confiance, largeur volets) | — | Extras | settings.html:11-18, 26-34, 51-60 | SANS OBJET | Pas d'équivalent RainLoop à vérifier (thème = page Thèmes ci-dessous) |

## 2. SettingsThemes.html

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Thèmes » + SaveTrigger (3-10) | Titre | Titre | Pas de page : le thème est un select dans Général (settings.html:11-12) | PARTIEL | Pas d'entrée de menu « Thèmes » (RainLoop : entrée distincte, voir §21). P3 |
| Grille `b-themes-list` (12-17) : vignette par thème avec nom, clic = applique, `selected` | Choisir un thème en voyant son aperçu | Sélection visuelle du thème | `<select>` de noms bruts (`{{.}}` : rainloop, dark, light) settings.html:12 | PARTIEL | Un seul utilisateur, 3 thèmes : remplacer par 3 vignettes générées (couleurs du thème) ou conserver. P3 |
| « Thème personnalisé » + alertes d'erreur (20-27) | Fond perso | Image de fond | settings.html:44-50 ; erreurs fr.json:627 | OK | |
| Bouton × + nom du fichier quand un fond est actif (28-34) ; bouton « Téléverser une image (JPG, PNG) » avec icône image/spinner (35-41) | Gestion du fond | Ajouter / retirer | settings.html:44-50 : champ fichier + « Utiliser cette image » + « Retirer mon image de fond » | OK | Rendu différent (pas d'upload instantané ni de nom affiché). P3 |

## 3. SettingsAccounts.html (comptes + identités)

| Élément (ligne) | But / rendu | Besoin → fonctionnalité | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Comptes » + spinner (4-8) | Titre, chargement | Titre | accounts.html:2 | OK | |
| Bouton « Ajouter un compte » icône user-add (10-14) | Ouvre PopupsAccount (email + mot de passe) | Ajouter un compte | accounts.html:26 `<a class="button" href="/accounts/new">` (formulaire IMAP/SMTP complet account_form.html) | PARTIEL | Fonction OK mais pas de saisie « e-mail + mot de passe » seule (RainLoop déduit les serveurs via ses domaines, SANS OBJET ici). Déjà aidé par accounts.js:15-22. Bouton sans icône. P3 |
| Liste triable par glisser-déposer (18-48), poignée `icon-braille`, icône user, nom e-mail, e-action (clic = éditer, si `canBeEdit`) | Réordonner et éditer | Ordre des comptes, accès édition | accounts.html:6-21 : ▲▼ (route `/accounts/{id}/move`), nom + e-mail + hôte IMAP, lien « Modifier » | PARTIEL | Ordre OK (flèches au lieu du glisser). Ajouter poignée + dnd (static/dnd.js existe pour les messages). Ligne cliquable. P3 |
| Suppression : corbeille, puis bouton rouge « Êtes-vous sûr ? » (34-44) ; compte principal non supprimable | Suppression en deux temps | Confirmer la suppression | accounts.html:12-19 `<details class="danger">` + « Confirmer la suppression » | PARTIEL | Fonction OK, rendu différent (icône corbeille → bouton rouge en ligne). P3 |
| Légende « Identités » + bouton « Ajouter une identité » (50-61) | Identités additionnelles sur la même page | Gestion d'identités | Page séparée par compte : accounts.html:11 lien « Identités » puis identities.html | PARTIEL | Plusieurs comptes = identités rattachées à chaque boîte, voulu. Ajouter les identités dans la même page sous chaque compte serait plus proche. P3 |
| Liste d'identités triable, « (par défaut) » sur la première (65-99) | Ordre et identité par défaut | Ordre / défaut | identities.html:5 identité principale « (identité principale…) » en tête ; ▲▼ identities.html:10 | OK | Défaut = identité principale, différent de RainLoop où l'ordre décide ; pas de dnd. P3 |
| Suppression d'une identité deux temps (85-95) | idem | idem | identities.html:12-17 | PARTIEL | idem comptes. P3 |

## 4. PopupsAccount.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Modale « Ajouter un compte ? / Mettre à jour » (2-10) | Titre selon `isNew` | Titre | account_form.html:2 titre unique « Compte mail » | PARTIEL | Voir §3. P3 |
| Alerte d'erreur fermable + détail supplémentaire (13-20) | Erreur de connexion | Diagnostic | account_form.html:4-7 messages IMAP/SMTP détaillés par `Kind` (fr.json:111-118) | OK | Plus détaillé que RainLoop |
| E-mail (22-29) éditable si nouveau, lecture seule sinon | Identifiant du compte | Email | account_form.html:15-16 éditable toujours | OK | |
| Mot de passe (30-36), Entrée valide | Authentification | Mot de passe | account_form.html:39-41 (+ « laissez vide pour conserver ») | OK | |
| Bouton Ajouter/Mettre à jour avec spinner (40-46) | Valider | Test + enregistrement | account_form.html:118-119 boutons « Tester » et « Enregistrer » | OK | |

## 5. PopupsIdentity.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Titre Ajouter/Modifier (7-8) | | | identities.html:22 | OK | |
| Alerte d'erreur (13-16) | Erreur de validation | | identities.html via `messages`, fr.json:336-338 | OK | |
| E-mail (18-31) : saisie si non propriétaire, texte si identité principale | | | identities.html:26-27 (toujours saisissable pour les extras), main = lecture seule via compte | OK | |
| Nom (32-39) | Nom d'expéditeur | | identities.html:28-29 | OK | |
| Reply-To masqué derrière un lien (40-47, 56-62) | Champ révélé au clic | Reply-To | identities.html:30-31 toujours visible | OK | Toujours visible : acceptable |
| Bcc masqué derrière un lien (48-55, 63-67) | | Bcc auto | identities.html:32-33 | OK | |
| Reply-To et Bcc de l'identité PRINCIPALE | Mêmes champs pour l'identité par défaut (popup ouverte sur l'identité principale depuis Général) | Reply-To/Bcc de l'identité par défaut | account_form.html:13-21 : seulement adresse, nom affiché, nom du compte ; aucun reply_to/bcc ; userdata.go:127-135 `Identity` les a, mais le formulaire du compte ne les propose pas (grep `reply_to` dans web/accounts.go : aucun) | MANQUANT | Ajouter Reply-To et Cci automatique au fieldset « Identité » du compte, stockés sur l'identité principale. P2 |
| Case « Insérer la signature avant le texte cité » (72-82) | Position de signature | signature avant citation | identities.html:37 `signature_before` ; account_form.html:114 | OK | |
| Éditeur de signature riche HTML (83-85) `editor: signature` | Signature mise en forme (gras, liens, images) | Signature HTML | identities.html:35 `<textarea>` texte ; le backend sait lire une signature `:HTML:` (compose.go:178-211, 892-905) mais aucun éditeur HTML dans les réglages | PARTIEL | Réutiliser editor.js (barre d'outils de composition) pour la signature, enregistrée avec le préfixe `:HTML:`. P2 |
| Bouton Ajouter/Mettre à jour (89-95) | | | identities.html:38 | OK | |

## 6. SettingsTemplates.html + PopupsTemplate.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Modèles » + spinner (SettingsTemplates 3-7) | | | templates.html:3 | OK | |
| Bouton « Ajouter un modèle » (9-13) | Ouvre la popup | Ajouter | templates.html:20 formulaire en bas de page (visible si `Editing` ou liste non pleine) | OK | Formulaire inline au lieu d'une popup. P3 |
| Liste triable (17-45) : nom, poignée, suppression 2 temps | Réordonner ses modèles | Ordre manuel, suppression | templates.html:7-16 : nom + aperçu 100 caractères (templates_settings.go:43-47), Modifier, `<details>` supprimer ; pas de réordonnancement | PARTIEL | Ajouter ▲▼ (comme comptes) pour fixer l'ordre du menu « Modèle » de la composition. P3 |
| Popup : titre Ajouter/Modifier (Popups 7-8) | | | templates.html:20 | OK | |
| Nom (18-24) | | | templates.html:23 | OK | |
| Corps via éditeur riche (26-31) | Modèle HTML | Modèle HTML | templates.html:24 `<textarea>` brut ; aide variables `{{DATE}}…` fr.json:620 | PARTIEL | Éditeur HTML (editor.js) pour le corps. P2 |
| Alerte d'erreur (13-16) ; bouton Ajouter/Mettre à jour | | | templates.html:4, 26 ; fr.json:621-622 | OK | |
| Utilisation dans la composition | Menu « Modèle » | Insérer | compose.html:67-68 `data-template` | OK | |

## 7. SettingsChangePassword.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende (3-5) | | | settings.html:64 (sous Sécurité) | OK | Entrée de menu séparée « Mot de passe » dans RainLoop (§21) |
| Mot de passe actuel / nouveau / confirmation, rangées en erreur si différent (8-34) | | Changer le mot de passe | settings.html:67-73 (+ minlength, aide fr.json register.password_hint) | OK | |
| Bouton « Définir le nouveau mot de passe » : icône clé, vert/rouge selon résultat (35-43) | Retour visuel | | settings.html:74 bouton standard ; retour par message | PARTIEL | Pas d'icône/couleur. P3 |
| Encart d'erreur à droite (45) | | | settings.html:3 `messages` ; fr.json:56 | OK | |
| Clé de récupération (propre à wmail-go) | — | | settings.html:84-93 | SANS OBJET | |

## 8. SettingsContacts.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Contacts » (4) | | | contacts.html:3 | OK | |
| Case « Ajouter automatiquement les destinataires au carnet » (6-16) | Auto-enregistrement | contactsAutosave | settings.html:38 `contacts_autosave`, contactsweb.go:402 `contactsAfterSend` | OK | Réglage placé sur Général et non sur la page Contacts. P3 |
| Section « Synchronisation distante (CardDAV) » (19-62) : case activer, URL (placeholder https://), utilisateur, mot de passe | Sync CardDAV | Synchroniser | contacts.html:81-99 (URL, user, mot de passe, effacer le mot de passe, intervalle 5-320, bouton « Synchroniser maintenant », dernier état/erreur) | OK | Plus complet |

## 9. SettingsSecurity.html + PopupsTwoFactorConfiguration.html + PopupsTwoFactorTest.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Sécurité » (Sec 4) | | | settings.html:2 | OK | |
| Déconnexion automatique : Select Jamais/5/10/30/60/120/300/600 min (Sec 6-23) avec SaveTrigger | Durée d'inactivité | autoLogout | settings.html:13-14 `idle_minutes` nombre (1..IdleMax, défaut 60) + durée absolue 15-16 | OK | Contrôle différent, fonction plus riche. Mais ces champs sont dans Général, pas Sécurité. P3 : les déplacer sous Sécurité |
| Lien « Configurer la 2-étapes » icône cadenas (25-32) | Ouvre la popup | Accès 2FA | settings.html:78-82 bouton « Configurer… » → `/settings/2fa` | OK | |
| Popup 2FA : titre (Cfg 7) | | | twofactor.html:3 | OK | |
| Case « Activer la 2-étapes » (désactivée tant que non testé) + lien « test » (12-28) | Activation différée après test | Contrôler qu'on peut se connecter | twofactor.html:30-40 : on saisit un code pour confirmer l'activation (équivalent au test) | OK | |
| Utilisateur affiché (30-42) | Identité du compte | | Non affiché sur la page 2FA (le nom est dans la barre `.who`, base.html setopen) | OK | |
| « Afficher le secret » / masquer (43-49) | Revoir le secret après activation | Ré-afficher secret et QR | twofactor.html:30-40 secret + QR seulement pendant l'installation ; actif : pas de secret | MANQUANT | Volontaire (sécurité) : on peut garder tel quel ; sinon ajouter « Afficher le secret » protégé par mot de passe. P3 |
| Secret + description + QR code (50-67) | Configuration d'une appli TOTP | Enrôlement | twofactor.html:33-34 QR 256x256 + clé | OK | |
| Codes de secours (68-79) | Affichés à tout moment | Codes de secours | twofactor.html:6-13 affichés une seule fois, régénération avec un code (14-21) | PARTIEL | Plus strict que RainLoop : acceptable. P3 |
| Boutons Déconnexion (mode obligatoire), Effacer (rouge), Activer, Terminé (83-102) | | Désactiver / activer | twofactor.html:22-28 « Désactiver » avec code, 44 « Activer » | OK | Mode « 2FA exigée » : SANS OBJET (réglage admin) |
| Popup de test : titre, champ code, bouton Tester vert/rouge (TwoFactorTest 1-33) | Vérifier un code sans désactiver | Test d'un code | Aucun test séparé | MANQUANT | Ajouter « Tester un code » sur la page 2FA active. P3 |

## 10. SettingsFolders.html + SettingsFolderItem.html

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Liste des dossiers » + spinner (Fold 4-8) | | | folders_settings.html:2, 45 | OK | |
| Bouton « Créer un dossier » (10-14) ouvre popup | Créer, avec parent | | folders_settings.html:32-43 formulaire (nom + « Dans ») | OK | Inline au lieu de popup |
| Bouton « Dossiers système » (16-20) | Définir envoyés/brouillons/corbeille/archives | | folders_settings.html:9-22 (rôles, « Automatique » ou « Ne pas utiliser ») | OK | |
| Alerte « trop de dossiers » (21-25), erreur fermable (26-29) | Garde-fou performance | | `messages` (folders_settings.html:4-7) | SANS OBJET (trop de dossiers) / OK (erreurs) | |
| Tableau : indentation par profondeur, nom, nom système (FolderItem 1-9) | Arbre | | folders_settings.html:46-67 classes `indent0..5` (mail.css:327), rôle en `<small>` (50) | OK | Ajout wmail-go : totaux/non lus |
| Renommer en ligne : clic sur le nom → champ, Entrée/Échap (FolderItem 4-9) | Renommage direct | | folders_settings.html:56-59 menu déroulant « Renommer » avec champ | PARTIEL | Fonction OK ; rendu différent. P3 |
| Suppression 2 temps (11-19) ; interdite aux dossiers système | | | folders_settings.html:60-65 (confirmation textuelle nominative) ; `.Protected` (55) | OK | |
| Œil abonner/désabonner (20-27) | Abonnement IMAP | | folders_settings.html:54 bouton texte (si `SubKnown`) | OK | |
| Pastille « vérifier les nouveaux messages » (28-35) | Choix des dossiers rafraîchis | | folders_settings.html:23-31 liste à cocher (5 max, fr.json:654) | OK | |
| Panneau d'aide à droite avec l'infobulle des icônes (40-44) `folderListHelp` | Aide au survol | | aucun | MANQUANT | P3 : ajouter title="" sur boutons (déjà texte explicite) → peu utile |

## 11. SettingsFilters.html (liste)

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Légende « Filtres » + spinner (3-7) | | | filters.html:2 | OK | |
| Bouton « Ajouter un filtre » icône + (11-15) | | | filters.html:33 | OK | |
| Bouton « script brut » icône fichier-code, bascule (17-20) | Éditer le sieve à la main | | filters.html:33 lien « Script brut » → filters_raw.html (zone de texte 22 lignes, case « utiliser ce script ») | OK | Page séparée |
| Bouton « Enregistrer » avec erreur (22-27) | Sauvegarde groupée | | Chaque action est enregistrée immédiatement (filters.html:22-29) | OK | Sémantique différente mais plus sûre |
| Avertissement « modifications à enregistrer » (30-39) | | | sans objet (enregistrement immédiat) | SANS OBJET | |
| Erreur serveur (40-48) | | | filters.html:7-10, `messages` | OK | |
| Zone « Capacité : … » + textarea monospace 300px (53-62) | Voir les extensions Sieve du serveur | Contrôle des capacités | filters_raw.html:8 textarea ; capacités non affichées (grep `Capab` dans filters_raw.html : aucun) | PARTIEL | Afficher la liste des extensions (déjà lue : `Modules.*` filter_edit.html:35-54) au-dessus du script. P3 |
| Tableau triable (63-102) : case activer, poignée, nom + sous-nom (« Déplacer vers "X" »), supprimer 2 temps | Liste des règles | | filters.html:15-31 : colonnes Nom, Résumé (« N conditions → action », web/filters.go:166-190), ▲▼, Activer/Désactiver (texte), Modifier, Supprimer `<details>`. Ligne grisée si désactivée (mail.css:`tr.off`) | PARTIEL | Fonctions OK. Rendu : case à cocher d'activation en icône, poignée dnd, sous-nom = action seule. Option : cocher d'activation dans la 1re colonne. P3 |
| Ordre d'exécution = ordre de la liste | | | ▲▼ filters.html:23-24 | OK | |

## 12. PopupsFilter.html + SettingsFiltersCondition*/Action*

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Titre Créer/Modifier (7-8) | | | filter_edit.html:3 | OK | |
| Nom du filtre (placeholder) (15-23) | | | filter_edit.html:11-12 | OK | |
| Case « actif » (RainLoop : dans la liste) | | | filter_edit.html:13 | OK | |
| Légende Conditions + select « Correspondant à l'UNE / TOUTES » visible si > 1 condition (25-34) | Any / All | | filter_edit.html:17-18 toujours visible | OK | |
| « Tous les messages entrants » si aucune condition (38-40) | Message d'état vide | | filter_edit.html:19 note permanente `filters.no_condition` | OK | |
| Bouton « Ajouter une condition » (42-46) | | | filter_edit.html:43 + filters.js:30-38 | OK | |
| Condition par défaut : champ + type + valeur + corbeille (CondDefault 1-12) | | | filter_edit.html:21-41 ; filters.js:10-21 | OK | |
| Condition « En-tête » : champ + nom d'en-tête + type + valeur (CondMore) | | | filter_edit.html:29 `cond_second` affiché seulement si champ = Header (filters.js:11) | OK | |
| Condition « Taille » : types Plus grand / Plus petit seulement (CondSize 4) | | | filters.js:12-20 ne laisse que Over/Under | OK | |
| Champs : De, Destinataires, Objet, Taille, En-tête (JS) | Ordre RainLoop | | filter_edit.html:23-27 : De, Destinataires, Objet, En-tête, Taille | OK | Ordre Taille/En-tête inversé. P3 |
| Types : Contient, Ne contient pas, Égal, Différent, Regex (si module), Plus grand, Plus petit | Types Sieve | | filter_edit.html:31-37 identiques ; sieve/filters.go:29-35 (les types « Correspond à * ? » n'existent pas dans le JS 1.17) | OK | |
| Action : select (50) | Choix de l'action | | filter_edit.html:48-56 : Rien, Déplacer, Faire suivre, Rejeter, Absence, Supprimer ; actions grisées si module Sieve absent | OK | |
| MoveTo : liste de dossiers + « Marquer comme lu » (ActionMoveToFolder) | | | filter_edit.html:57-60, 75 | OK | |
| Forward : e-mail, case « Conserver », « Marquer lu » (ActionForward) | | | filter_edit.html:61-64, 75 | OK | |
| Reject : textarea (ActionReject) | | | filter_edit.html:65-67 | OK | |
| Vacation (ActionVacation) : intervalle Select [1,2,3,7,30] jours, destinataires + bouton « + » (remplit avec toutes les adresses du compte), objet facultatif, message, case « Ne pas arrêter » | Réponse automatique | | filter_edit.html:68-74 : objet, message, délai en nombre 1-365, adresses, case stop. Ordre différent ; pas de bouton « + » | PARTIEL | Ajouter un bouton qui insère les adresses du compte et des identités (équivalent `setRecipients`). Libellé stop : RainLoop « Ne pas arrêter le traitement des règles » (case cochée = poursuivre) ; notre case `stop` (fr.json:434) a le sens inverse, fonction identique. P3 |
| Discard / None : aucun champ | | | filter_edit.html:50, 55 | OK | |
| Bouton « Terminé » (56-60) | | | filter_edit.html:77 « Enregistrer » | OK | |
| Fenêtre modale | Édition sans quitter la liste | | page dédiée `/settings/filters/edit` | PARTIEL | Rendu différent (page). P3 |

## 13. Contacts : PopupsContacts.html (comparaison avec `/contacts`)

| Élément (ligne) | But / rendu | Besoin | Équivalent wmail-go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Modale plein écran sur la boîte (2) `b-contacts-content`, deux volets | Carnet en fenêtre au-dessus du courrier | Accès rapide sans quitter la page | `/contacts` : page de réglages (contacts.html:1-3), liste en haut et formulaire en dessous (32-69) ; bouton carnet dans la barre de la boîte (base.html:98) | PARTIEL | Pas de vue 2 volets (liste à gauche, fiche à droite). Option : mise en page en deux colonnes dans `.setcontent` (liste cliquable ouvrant `?edit=` à droite). P2 |
| Barre : « Ajouter un contact » + (10-15) | Nouveau contact | | contacts.html:10 « Nouveau contact » | OK | |
| Bouton vert « Nouveau message » (18-20) : écrit aux contacts cochés | Écrire à une sélection | Compose multi-destinataires | Lien par contact vers `/compose?to=` (contacts.html:21) et bouton « Écrire » dans la fiche (66) ; pas de multi-sélection → compose | MANQUANT | Ajouter un bouton « Écrire à la sélection » qui compose avec toutes les adresses principales des lignes cochées. P2 |
| Bouton corbeille (24-26) | Supprimer la sélection | | contacts.html:26-28 `<details>` « Supprimer la sélection » + confirmation | OK | |
| Menu « plus » (29-66) : Importer (csv, vcf), séparateur, Exporter CSV, Exporter vCard, Synchroniser | Actions carnet | | contacts.html:71-80 : section Importer/Exporter (accept `.vcf,.vcard,.csv`), 2 liens d'export, section Sync (81-99) | OK | Rendu section plutôt que menu |
| Icône spinner pendant import/sync (31-33) | Retour | | `contacts.sync.running` (95) ; import = rechargement | PARTIEL | P3 |
| Recherche (71-73) `valueUpdate` temps réel | Filtre en direct | | contacts.html:6-11 : GET avec bouton « Rechercher » + filtre par étiquette | PARTIEL | Pas de filtre en direct (JS léger sur la liste). Étiquettes = extra. P3 |
| Lien « Effacer la recherche » (77-79) | | | aucun (aucun `clear` dans contacts.html) | MANQUANT | Ajouter un lien visible si `Q` ou `Tag` non vide. P3 |
| États vides : « Aucun contact » / « Chargement… » / « Aucun contact trouvé » (80-88) | | | contacts.html:29 un seul texte `contacts.none` | PARTIEL | Distinguer vide et recherche sans résultat. P3 |
| Ligne : case, icône partage, nom d'affichage (89-107) ; cases masquées si `useCheckboxesInList` faux (74) | Liste compacte | | contacts.html:15-23 tableau Nom / E-mail / Téléphone / Étiquettes, case toujours visible | PARTIEL | Plus d'info que RainLoop ; partage = SANS OBJET. Respecter `HideChecks` pour les cases. P3 |
| Pagination numérotée (111-114 + Common/Pagenator) | Pages | | contacts.html:24-25 « Précédent / x / y / Suivant », 50 par page (contacts.go:26) | PARTIEL | Ajouter les numéros de page comme la liste des messages (home.html:104 `.pager`). P3 |
| Menu « Ajouter » (117-159) : E-mail, Téléphone, URL, Pseudo ; Adresse commenté | Ajouter une propriété | | Champs toujours présents (contacts.html:38-64) : 2 lignes e-mail, 2 téléphone, 1 URL, adresse, pseudo, société, fonction, date, étiquettes, notes | OK | Plus complet ; pas de bouton « + » pour ajouter une 3e ligne e-mail/téléphone (padTyped fixe 2 lignes, contactsweb.go:~160). Ajouter un bouton « Ajouter une ligne ». P3 |
| Bouton Créer/Mettre à jour, surlignage « dirty » (161-167) | | | contacts.html:65 « Enregistrer » | OK | |
| Texte d'aide « Sélectionnez un contact… » (173-175) | Volet vide | | sans objet (pas de volet) | SANS OBJET | |
| Zone haute : icône utilisateur, nom (préfixe, prénom, nom…) en texte statique devenant champ au clic (177-207) | Édition « sur place » | | contacts.html:38-46 champs de saisie permanents | PARTIEL | Rendu différent (champs ouverts). P3 |
| Icônes @, téléphone, web avec infobulle (209-254) ; mode lecture seule (contacts distants) | | | libellés texte dans des `fieldset` (48-60) ; pas de mode lecture seule | PARTIEL | Mode lecture seule : SANS OBJET (contacts en lecture seule viennent de plugins/CardDAV). Rendu icônes : P3 |
| Lien « Ajouter une adresse e-mail » (222) | | | absent (voir ci-dessus) | MANQUANT | P3 |
| Aperçu de contact depuis un message ; ajout depuis l'expéditeur | | | fr.json:559-560 `contacts.add_sender`, `edit_contact` ; contactsAdd (contactsweb.go:383) | OK | |
| Sélecteur de contacts dans la composition | | | contacts-ac.js + `dialog.picker` (mail.css:`.picker-list`) | OK | |
| Raccourcis dans la modale (Échap ferme) | | | page : Échap sans effet (menus.js seulement) | SANS OBJET | |

## 14. Common/ : templates

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Cmd.html (1-10) : console `#` avec historique | Console de commandes cachée | Outil de diagnostic | aucun | SANS OBJET | Fonction cachée/développeur |
| Pagenator.html (1-4) : liens de page numérotés (`current`, `custom`) | Pagination | | home.html:104-110 `.pager` (mail.css:114-117 : page courante soulignée, gap) | OK | |
| PopupsAsk.html (1-26) : fenêtre « Oui / Non » avec icônes ok/remove, focus sur le bouton | Confirmation | | `data-confirm` géré en JS (base.html:114, compose.html:66) et `.menu.confirm` (mail.css:230-231) ; réglages : `<details>` | PARTIEL | Style/rendu différents selon écran. P3 |
| PopupsKeyboardShortcutsHelp.html (1-90) : 4 onglets Boîte aux lettres / Liste / Message / Rédaction | Aide des raccourcis | | base.html:163-195 une seule fenêtre à sections Liste / Message / Rédaction / Général (aide `?`) | PARTIEL | Contenu comparé ci-dessous |
| — Boîte : « ` » menu utilisateur, R, A, F, Maj+F, ?  | | | keys.js:57-62, 65 ; aide base.html:173-190 | OK | |
| — Liste : Ctrl+A, Z, Suppr / Maj+Suppr / #, T, M, Q, U, I, /, Entrée (ouvrir/plein écran), → / Tab (vers le message), ← / Maj+Tab (vers les dossiers) | | | Ctrl+A list.js:98-105 ; Z, Q, U, I, M, T, / keys.js ; Entrée listnav.js:55 (ouvre) ; flèches haut/bas, Début/Fin, PgUp/PgDn, Espace/Inser (plus riche) listnav.js | PARTIEL | MANQUANT : Entrée = plein écran dans la liste (msgfull.js ne gère que Échap, 14-15), → / Tab / ← de bascule de volet. P3 |
| — Message : Entrée plein écran, B bloc citations, Ctrl+P, Échap sortir du plein écran / fermer, Tab retour liste | | | Échap msgfull.js:13-15 + keys.js:38-40 ; Ctrl+P : impression native du navigateur (print.js) ; aucune touche B (grep `blockquote` dans static/*.js : seulement editor.js) | PARTIEL | MANQUANT : touche B (afficher/masquer les citations ; suppose le repli des citations dans le message, absent) et Entrée = plein écran. P3 |
| — Rédaction : W/C nouvelle, ` identités, Ctrl+S, Échap réduire, Maj+Échap fermer | | | keys.js:55 (c, w) ; Ctrl+S compose.js:207 ; Ctrl+Entrée envoyer compose.js:206 (extra) ; pas de fenêtre réduite | PARTIEL | Réduire la composition : SANS OBJET (composition en page). « ` » ouvre le menu utilisateur (keys.js:62) et pas les identités. P3 |
| PopupsLanguages.html (1-22) : liste de langues avec drapeaux, coche verte | Choix de la langue | | settings.html:9-10 select ; login.html:17 liens texte | PARTIEL | Voir §1. P3 |
| PopupsWelcomePage.html (1-21) : iframe d'accueil réglée par l'administrateur | Page d'accueil | | aucun | SANS OBJET | Réglage admin |

## 15. Components/ (briques de formulaire)

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Checkbox.html (1-10) : case en police d'icône (`icon-checkbox-checked/unchecked`), Espace, tabindex, libellé animé, désactivable | Case à cocher du thème | Rendu identique à DarkShine | `<input type=checkbox>` native avec `label.check` (app.css:`label.check`) ; aucune règle `appearance` (grep mail.css/app.css : aucune) | PARTIEL | Habiller les cases des réglages (appearance:none + coche) pour ressembler à l'icône RainLoop. P3 |
| CheckboxClassic.html (1-6) | Variante natives | | natives | OK | |
| CheckboxMaterialDesign.html (1-10) | Variante Material (choix du thème) | | aucun | SANS OBJET | Non utilisée par DarkShine (Checkbox standard) |
| Date.html (1-13) : champ + Pikaday | Sélecteur de date | | contacts.html:46 `<input type=date>` natif | OK | |
| Input.html (1-13) : champ + libellé + SaveTrigger | | | inputs natifs | OK | |
| Radio.html (1-6) : boutons radio en icônes | | | aucun radio dans nos réglages | SANS OBJET | |
| SaveTrigger.html (1-2) : spinner / croix / coche verte à côté du champ | Retour d'enregistrement | | aucun | PARTIEL | Voir §1 (auto-enregistrement). P3 |
| Select.html (1-20) : select + libellés + SaveTrigger | | | select natifs | OK | |
| TextArea.html (1-9) | | | textarea natifs (mail.css:479) | OK | |

## 16. Structure des réglages : SettingsMenu / SettingsPane / SettingsCustom

| Élément (ligne) | But / rendu | Besoin | Équivalent | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| Menu gauche `b-settings-menu` : entrées sélectionnables / désactivées ; ordre Général, Contacts, Comptes, Filtres, Sécurité, Mot de passe, Modèles, Dossiers, Thèmes (app.js:36620-36655) | Navigation | Mêmes entrées | base.html:75-84 : Général, Comptes, Dossiers, Filtres, Modèles, Contacts, Sécurité (mot de passe + 2FA + clé de récupération dans Sécurité) | PARTIEL | Ordre et découpage différents (pas d'entrée Mot de passe/Thèmes séparées). Réordonner : Général, Contacts, Comptes, Filtres, Sécurité, Modèles, Dossiers. P3 |
| Navigation clavier du menu (↑ ↓) (app.js:38334) | | | aucune | MANQUANT | P3 |
| Bouton de repli du panneau gauche (resize in/out) (Menu 17-27) | Agrandir la zone | | aucun | MANQUANT | P3 |
| Menu gauche : texte blanc sur fond d'écran, sélectionné = fond semi-transparent | Rendu DarkShine | | mail.css:307-310 (`--folders-fg`, `--folders-sel`) | OK | |
| Barre d'outils du volet droit : bouton « Retour » icône flèche (Pane 14-18) ; boutons mobiles gauche/droite (4-13) | Revenir à la boîte | | base.html setopen:87 « Retour » avec icône ; mail.css:313-314 ; menu en haut en mobile (mail.css:332) | OK | Extra : utilisateur + déconnexion (88-89) |
| Zone de contenu défilante avec ombres (nano scroller) (21-26) | | | `.setcontent` overflow:auto (mail.css:316) | OK | |
| SettingsCustom.html (2) : espace réservé aux plugins | | | aucun | SANS OBJET | Plugins |

## 17. Éléments marqués SANS OBJET (lus)

| Élément | Raison |
|---|---|
| SettingsOpenPGP.html (99 lignes : importer/générer des clés, case brouillon auto OpenPGP, listes de clés) | OpenPGP exclu |
| SettingsSocial.html (100 lignes : Google, Facebook, Twitter connectés) | Connexion sociale exclue |
| PopupsTwoFactorConfiguration « mode verrouillé » / bouton Déconnexion | Réglage administrateur de 2FA obligatoire |
| Pastille « trop de dossiers » | Détection de volumétrie propre à RainLoop |
| Cmd.html, PopupsWelcomePage.html, CheckboxMaterialDesign.html, Radio.html, SettingsCustom.html | Outil caché, page d'accueil admin, variante non utilisée par DarkShine, plugin |

---

# À AJOUTER (MANQUANT / PARTIEL), par priorité

Aucun point P1 : les réglages ne sont pas l'écran du quotidien et nos fonctions de fond (comptes, identités, filtres, modèles, contacts, 2FA, dossiers, préférences) sont présentes. Les écarts sont d'abord visuels.

## P2

1. **Mise en page « aperçu en bas » (division horizontale)** : ajouter la valeur `bottom` à `settings.layout` (handlers.go:404 + userdata.go:51 + select settings.html:19-23 + libellé fr/en « 3 volets : aperçu en bas »). Côté boîte (mail.go:488, grille CSS `.mailbox.three`) : classe `.mailbox.bottom` avec liste au-dessus du volet de lecture et un séparateur horizontal glissable dont la hauteur est mémorisée comme les largeurs (list.js splitters, handlers.go settingsPanes).
2. **Reply-To et Cci automatique de l'identité principale** : dans account_form.html (fieldset « Identité », après la ligne 20) ajouter `reply_to` et `bcc` ; les lire et les valider dans web/accounts.go comme identities.go:67-80 ; les appliquer à la composition pour l'identité principale (compose.go identityLabel/`Identity.ReplyTo/Bcc`).
3. **Éditeur riche pour la signature et pour le corps des modèles** : réutiliser editor.js (barre `.cp-fmt`, `.cp-editor`) dans identities.html:35, account_form.html:112 et templates.html:24 ; enregistrer la signature avec le préfixe `:HTML:` déjà compris par compose.go:178-211/892-905 ; stocker le corps d'un modèle en HTML assaini et l'insérer selon le mode actif de la composition.
4. **Contacts : « Écrire à la sélection »** : bouton dans la barre de contacts.html (près de « Nouveau contact ») soumettant les cases `id` cochées vers `/compose?to=a,b,c` (première adresse de chaque contact, comme `ContactRow.Email`).
5. **Contacts : vue deux volets** : dans `.setcontent` afficher liste (tableau actuel réduit : Nom) à gauche et fiche (`section#edit`) à droite quand `Editing`, avec `?edit=` sur chaque ligne ; boutons de barre dans l'ordre RainLoop : Ajouter, Écrire, Supprimer, menu Plus (Importer, Exporter CSV, Exporter vCard, Synchroniser).
6. **Option « Toujours afficher les images externes » sur Général** : case globale qui désactive `BlockRemoteContent` pour tous les comptes (ou lien explicatif vers l'option par compte et la liste de confiance, settings.html:51-52).
7. **Style des boutons des réglages** : sous `.setcontent`, `button:not(.link)` et `a.button` reprennent `--btn-bg`, `--btn-bd`, `--btn-fg` (dégradé blanc, texte sombre, bordure sombre transparente) ; `.danger` en `#bd362f` ; icône + libellé devant les actions principales (ajouter, enregistrer, supprimer). Remplace le bleu `--accent` plein actuel (app.css:`button`).

## P3

8. **Formulaires horizontaux** (libellé à gauche 160 px, contrôle à droite) dans `.setcontent`, et groupement Général / Notifications avec légendes.
9. **Auto-enregistrement avec indicateur** (spinner / croix / coche verte) sur Général : `change` → fetch POST `/settings/preferences`, composant `.saved-trigger`.
10. **Notifications** : bouton ▶ « tester le son » (JS Audio), texte « (Bloqué par le navigateur) » si permission refusée (notify-setting.js:9-13), masquage si l'API Notification est absente.
11. **Identité principale en tête de Général** : ligne « Identité : Nom <adresse> » liée à l'édition du compte.
12. **Suppression en deux temps** (icône corbeille → bouton rouge « Êtes-vous sûr ? ») pour comptes, identités, modèles, filtres, dossiers au lieu de `<details>` ; réordonnancement par glisser-déposer (comptes, identités, modèles, filtres) en plus des flèches.
13. **Vacation** : bouton « + » qui insère toutes les adresses du compte et des identités dans `vac_addrs` ; intervalle en liste [1, 2, 3, 7, 30] ; ordre RainLoop (intervalle, destinataires, objet, message, case). Ordre des champs de condition Taille avant En-tête.
14. **Filtres** : afficher « Capacité : <extensions> » dans filters_raw.html ; colonne d'état avec case d'activation en icône ; sous-nom = action seule (« Déplacer vers "X" »).
15. **Raccourcis** : Entrée en vue message = plein écran (msgfull.js), touche B pour replier/déplier les citations (suppose ce repli dans mailview), → / ← / Tab de bascule entre volets ; regrouper l'aide en 4 onglets Boîte / Liste / Message / Rédaction comme RainLoop (base.html:163-195).
16. **Contacts, détails** : lien « Effacer la recherche », états vides distincts (aucun contact / aucun résultat), filtre en direct, pagination numérotée, cases masquées si « cases à cocher » désactivé, bouton « Ajouter une ligne » pour e-mails/téléphones au-delà de 2 (contactsweb.go padTyped).
17. **Navigation des réglages** : réordonner le menu (Général, Contacts, Comptes, Filtres, Sécurité, Modèles, Dossiers), navigation ↑↓ au clavier, bouton de repli du panneau gauche, déplacer idle/durée de session sous Sécurité.
18. **Rendu des cases à cocher** en icônes comme DarkShine (`appearance:none` + coche), vignettes de thème au lieu du select, drapeaux de langue, test d'un code 2FA et « afficher le secret » protégé par mot de passe, renommage de dossier en ligne.

---

## Bilan chiffré

Voir la dernière ligne du message final (décomptes calculés par comptage des verdicts dans ce fichier).
