# Point d'avancement de wmail-go (2026-10-24)

Chiffres de pourcentage = **jugement pondéré**, pas une mesure. Détail des écarts : `docs/audit-rainloop/` ; décisions : `CAHIER_DES_CHARGES.md`.

| Domaine | Code | Testé automatiquement | Validé par l'utilisateur sur ses vraies boîtes | Reste à faire |
|---|---|---|---|---|
| Socle : comptes chiffrés, sessions, multi-boîtes, i18n fr/en, journaux, conteneurs, HTTP | ~100 % | oui | partiel (login, ajout de boîtes) | HTTPS derrière votre proxy, sauvegarde du volume |
| Lecture : liste, tri, recherche, conversations, dossiers (arbre), actions, glisser-déposer, clavier | ~92 % | oui (+ vrai Dovecot) | partiel (suppression par 500 validée) | aperçu en bas, lecture par partie IMAP (gros messages), IDN, drapeaux rafraîchis au sondage |
| Affichage d'un message : HTML assaini, texte brut, pièces jointes, aperçu, ICS, images distantes | ~88 % | oui | **à valider** (signatures, charset, ICS, espacement du corps) | `.eml` de l'invitation Google à fournir ; assainissement HTML plus fidèle ; liens `mailto:` du corps |
| Composition : texte/HTML, pastilles, brouillons, modèles, signatures, pièces jointes, accusés | ~90 % | oui + navigateur | non | éditeur riche pour signatures/modèles, Reply-To et Cci de l'identité principale |
| Contacts : carnet chiffré, vCard/CSV, autocomplétion, CardDAV | ~90 % | oui (serveur CardDAV simulé) | non | deux volets, « écrire aux cochés », propriétés vCard inconnues, en-têtes CSV |
| Filtres Sieve / dossiers système / quotas | ~95 % | oui (vrai Dovecot) | partiel | quota réel à vérifier |
| Sécurité : 2FA TOTP, récupération, limitation de débit, anti-SSRF | ~100 % | oui | non | essai réel avec votre application TOTP |
| Plug-ins repris : ICS, X-Originating-IP | 100 % | oui | à valider | — |
| **Fidélité visuelle à RainLoop / DarkShine** | **~60 %** | captures | **non conforme sur plusieurs points** | icônes pleines, barre de composition, barres de défilement, états des boutons, focus, boutons des réglages, contacts en deux volets, en-tête de composition |
| Analyses de RainLoop | 100 % lues (1 850 fonctions PHP, `app.js`, 55 gabarits, CSS, langues, 2 plug-ins) | — | — | P1 : 1/1 fait ; P2 : environ 60 % ; P3 : environ 25 % |
| Tests | 312 tests Go, 6 contrôles jsdom, 4 scénarios Chromium, tests contre Dovecot | — | — | essais contre vos serveurs (mail.sdv.fr) |

**Avancement global : environ 72 %** (pondération : fonctions 40 %, fidélité visuelle 25 %, validation sur boîtes réelles 20 %, tests et robustesse 15 %).
Le chiffre le plus faible est la validation sur vos vraies boîtes (~25 %) ; la fidélité visuelle (~60 %) vient ensuite.

## À tester et valider par vous (dans cet ordre)
1. Choisir le thème « DarkShine (comme RainLoop) » dans Réglages > Général et régler le fuseau horaire.
2. Ouvrir le message d'invitation Google : espacement du corps, liens, carte ICS (fournir son `.eml` dans `scratch/`).
3. Signatures HTML, charsets inhabituels, pièces jointes (aperçu, noms `calendar1.3.ics`).
4. Recherche sur la grosse boîte (vitesse et résultats comparés à RainLoop).
5. Actualisation automatique de la liste, notifications, glisser-déposer, conversations.
6. 2FA avec votre application, CardDAV avec votre serveur, filtres Sieve (avec accord pour toucher au serveur).
7. Déplacer/supprimer sur une boîte de test (jamais vos messages réels sans copie).

## Reprise du travail (autre session ou autre modèle)
Lire dans l'ordre : `CAHIER_DES_CHARGES.md` (chapitres 11, 14, 15), ce fichier, `docs/audit-rainloop/` (sections « À AJOUTER »). Règles de l'utilisateur : ne rien faire sur ses vraies boîtes, ne jamais lire `data/` de RainLoop, tests uniquement en conteneurs (`make test-race`, `make test-js`, `make test-e2e`, `make sieve-up` puis `make test`). Montages : `rainloop/1.17.0` (sshfs, lecture seule) et `rainloop_plugins`.
