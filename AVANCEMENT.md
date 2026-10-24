# Point d'avancement de wmail-go (2026-10-24, fin d'après-midi)

Les pourcentages sont un **jugement pondéré**, pas une mesure. Détail des écarts : `docs/audit-rainloop/` ;
décisions : `CAHIER_DES_CHARGES.md` ; historique : `git log`.

## 1. État par domaine

| Domaine | Code | Tests automatiques | Validé par vous | Reste à faire |
|---|---|---|---|---|
| Socle : comptes chiffrés, sessions, plusieurs boîtes, fr/en, thèmes, conteneurs, dépôt Git | ~100 % | oui | en partie | sauvegarde du volume |
| Liste : tri, recherche, conversations, dossiers, glisser-déposer, clavier, actualisation, sélection souris et clavier | ~98 % | oui (+ Dovecot) | en partie | liste d'une ligne en disposition « liste seule » |
| Lecture : HTML assaini, texte brut, charsets, pièces jointes, ICS, images distantes, photo, flèches préc./suiv. | ~95 % | oui | **à valider** | espacement du corps (en attente du `.eml`), aperçu PDF intégré, bouton « haut de page » |
| Composition : texte et HTML, pastilles, brouillons, modèles, signatures, accusés, transfert groupé | ~97 % | oui + navigateur | non | glisser une adresse d'un champ à l'autre |
| Contacts : deux volets, vCard/CSV, photo, autocomplétion, CardDAV, recherche par numéro | ~97 % | oui (serveur CardDAV simulé) | non | ordre des boutons de barre comme RainLoop |
| Filtres Sieve, dossiers système, garde-fous, quota, vider un dossier | ~97 % | oui (Dovecot) | en partie | quota réel à vérifier |
| Sécurité : 2FA, clé de récupération, limitation de débit, anti-SSRF, mandataire d'images, HSTS, cache | ~100 % | oui | non | essai avec votre application TOTP |
| Vos plug-ins : ICS, X-Originating-IP | 100 % | oui | à valider | réponse accepter/refuser à une invitation |
| **Fidélité visuelle DarkShine** | **~85 %** | captures | **à revalider** | icônes pleines restantes, animations, largeurs par taille d'écran |
| Analyse de RainLoop | 100 % lue | — | — | P1 : 2/2 ; P2 : ~95 % ; P3 : ~50 % |

**Avancement global : environ 88 %** (fonctions 40 %, fidélité visuelle 25 %, validation sur vos boîtes 20 %, tests 15 %).
Détail : fonctions ~95 %, couverture de RainLoop ~93 %, tests automatiques ~90 %, **validation par vous ~30 %**
(c'est le point faible ; la matrice `MATRICE_DE_TEST.md`, 135 cas, sert à la faire monter).

## 2. Fait depuis le passage à Opus 5

Dépôt Git créé (11 commits). P1 terminés. P2 : liens `mailto:` vers la composition, liens automatiques, conversion
HTML→texte fidèle, garde-fous de suppression de dossier, pièces jointes téléchargées partie par partie, saisie
d'adresses tolérante, punycode, en-têtes d'adresses tolérants, touche `i` en bascule, Échap enchaîné, menu unique
« Déplacer vers » (Ctrl = copier), photo du correspondant (carnet puis Gravatar en option), « Répondre à » et Cci de
l'identité principale, éditeur riche des signatures et modèles, « Écrire à la sélection », finitions visuelles
(barres de défilement, boutons, champs, liste vide).

## 2 bis. Fait dans la session du 24 octobre (après-midi)

Contacts : propriétés vCard inconnues conservées, types normalisés, en-têtes CSV typés, recherche par numéro
(chiffres seuls, zéro de tête ignoré). Web : favicon, ETag et cache des fichiers statiques, HSTS en HTTPS, retour à
la page demandée après reconnexion (2FA comprise). IMAP : tri naturel des dossiers, PERMANENTFLAGS respecté,
création automatique d'un dossier système absent (MOVE/COPY/APPEND). SMTP : EHLO avec le vrai nom d'hôte.
Lecture : X-Confirm-Reading-To et Return-Receipt-To, brouillons RainLoop (X-Draft-Info). Liste : suppression
définitive (Maj+Suppr), « tout marquer non lu », 20 suggestions d'adresses.

Reste des P2/P3 : fidélité visuelle DarkShine (icônes pleines, boutons des réglages, aperçu « N sélectionnés »,
fond des conversations non lues, icônes de pièces jointes par type), vue contacts en deux volets, disposition
« aperçu en bas », drapeaux rafraîchis au sondage, avertissements d'envoi côté navigateur, navigation clavier entre
volets, **matrice de test à produire**.

## 3. À tester et valider par vous

1. Thème « DarkShine » et fuseau horaire dans Réglages > Général.
2. Le message d'invitation Google : **fournir son `.eml` dans `scratch/`** (écarts d'espacement et de liens).
3. Photo du correspondant : option Gravatar, photo dans une fiche de contact.
4. Recherche sur la grosse boîte (vitesse et résultats comparés à RainLoop).
5. Actualisation automatique, notifications, glisser-déposer, conversations.
6. Signatures et modèles mis en forme, envoi avec « Répondre à » et Cci.
7. 2FA avec votre application, CardDAV avec votre serveur, filtres Sieve (avec votre accord).

## 4. Reprise du travail

Lire : `CAHIER_DES_CHARGES.md` (chapitres 11, 14, 15), ce fichier, `docs/audit-rainloop/` (sections « À AJOUTER »).
Règles : ne rien faire sur les vraies boîtes de l'utilisateur, ne jamais lire `data/` de RainLoop, tests uniquement en
conteneurs (`make test-race`, `make test-js`, `make test-e2e`, `make sieve-up` puis `make test`).
Montages en lecture seule : `rainloop/1.17.0`, `rainloop_plugins/`.
