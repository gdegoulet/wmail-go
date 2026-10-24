# Point d'avancement de wmail-go (2026-10-24, soir)

Les pourcentages sont un **jugement pondéré**, pas une mesure. Détail des écarts : `docs/audit-rainloop/` ;
décisions : `CAHIER_DES_CHARGES.md` ; historique : `git log`.

## 1. État par domaine

| Domaine | Code | Tests automatiques | Validé par l'utilisateur | Reste à faire |
|---|---|---|---|---|
| Socle : comptes chiffrés, sessions, plusieurs boîtes, fr/en, thèmes, conteneurs, dépôt Git | ~100 % | oui | en partie | HTTPS derrière votre proxy, sauvegarde du volume |
| Liste : tri, recherche RainLoop, conversations, dossiers, glisser-déposer, clavier, actualisation automatique, pastilles | ~95 % | oui (+ Dovecot) | en partie | disposition « aperçu en bas », drapeaux rafraîchis au sondage, transfert groupé |
| Lecture : HTML assaini, texte brut, charsets, pièces jointes par partie IMAP, ICS, images distantes, photo du correspondant | ~92 % | oui | **à valider** | espacement du corps (en attente du `.eml`), aperçu PDF intégré, bouton « haut de page » |
| Composition : texte et HTML, pastilles, brouillons, modèles, signatures riches, Répondre à et Cci, accusés | ~95 % | oui + navigateur | non | avertissements à l'envoi côté navigateur, glisser une adresse entre champs |
| Contacts : carnet chiffré, vCard/CSV, photo, autocomplétion, CardDAV, écrire à la sélection | ~93 % | oui (serveur CardDAV simulé) | non | vue deux volets, propriétés vCard inconnues conservées, en-têtes CSV supplémentaires |
| Filtres Sieve, dossiers système, garde-fous, quota | ~96 % | oui (Dovecot) | en partie | quota réel à vérifier |
| Sécurité : 2FA, clé de récupération, limitation de débit, anti-SSRF, mandataire d'images | ~100 % | oui | non | essai avec votre application TOTP |
| Vos plug-ins : ICS, X-Originating-IP | 100 % | oui | à valider | réponse accepter/refuser à une invitation |
| **Fidélité visuelle DarkShine** | **~72 %** | captures | **partiellement conforme** | barre de composition sombre, icônes pleines, boutons des réglages, en-tête de composition gris, aperçu « N sélectionnés » |
| Analyse de RainLoop | 100 % lue | — | — | P1 : 2/2 faits ; P2 : ~85 % ; P3 : ~35 % |

**Avancement global : environ 82 %** (fonctions 40 %, fidélité visuelle 25 %, validation sur vos boîtes 20 %, tests 15 %).
Le point faible reste la validation sur vos vraies boîtes (~30 %).

## 2. Fait depuis le passage à Opus 5

Dépôt Git créé (11 commits). P1 terminés. P2 : liens `mailto:` vers la composition, liens automatiques, conversion
HTML→texte fidèle, garde-fous de suppression de dossier, pièces jointes téléchargées partie par partie, saisie
d'adresses tolérante, punycode, en-têtes d'adresses tolérants, touche `i` en bascule, Échap enchaîné, menu unique
« Déplacer vers » (Ctrl = copier), photo du correspondant (carnet puis Gravatar en option), « Répondre à » et Cci de
l'identité principale, éditeur riche des signatures et modèles, « Écrire à la sélection », finitions visuelles
(barres de défilement, boutons, champs, liste vide).

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
