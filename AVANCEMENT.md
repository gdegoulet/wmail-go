# Point d'avancement de wmail-go (2026-10-25, soir)

Les pourcentages sont un **jugement pondéré**, pas une mesure. Détail des écarts : `docs/audit-rainloop/` ;
décisions : `CAHIER_DES_CHARGES.md` § 6 ; historique exact : `git log` (122 commits).

Un agent qui reprend le projet lit d'abord `AGENTS.md`, puis `docs/architecture.md`.

## 1. État par domaine

| Domaine | Code | Tests automatiques | Validé par vous | Reste à faire |
|---|---|---|---|---|
| Socle : comptes chiffrés, sessions, plusieurs boîtes, fr/en, thèmes, conteneurs, dépôt Git, sauvegarde | ~100 % | oui | en partie | — |
| Liste : tri, recherche, conversations, dossiers, glisser-déposer, clavier, actualisation, sélection souris et clavier | ~98 % | oui (+ Dovecot) | en partie | liste d'une ligne en disposition « liste seule » |
| Lecture : HTML assaini, texte brut, charsets, pièces jointes, ICS, images distantes, photo, flèches préc./suiv. | ~96 % | oui | **à revalider** | espacement du corps (en attente du `.eml`), bouton « haut de page » |
| Composition : texte et HTML, pastilles, brouillons, modèles, signatures, accusés, transfert groupé | ~97 % | oui + navigateur | non | glisser une adresse d'un champ à l'autre |
| Contacts : deux volets, vCard/CSV, photo, autocomplétion, CardDAV, recherche par numéro | ~97 % | oui (serveur CardDAV simulé) | non | ordre des boutons de barre comme RainLoop |
| Filtres Sieve, dossiers système, garde-fous, quota, vider un dossier | ~97 % | oui (Dovecot) | en partie | quota réel à vérifier |
| Sécurité : 2FA, clé de récupération, limitation de débit, anti-SSRF, mandataire d'images, HSTS, cache, **journal d'activité** | ~100 % | oui | non | essai avec votre application TOTP |
| Vos plug-ins : ICS, X-Originating-IP | 100 % | oui | à valider | réponse accepter/refuser à une invitation |
| **Fidélité visuelle DarkShine** | **~90 %** | captures | **à revalider** | icônes pleines restantes, animations, largeurs par taille d'écran |
| Exploitation : image publiée, CI maison, documentation | ~95 % | CI au vert | non | **publication effective sur Docker Hub jamais faite** |
| Analyse de RainLoop | 100 % lue | — | — | P1 : 2/2 ; P2 : ~95 % ; P3 : ~50 % |

**Avancement global : environ 97 %** (fonctions 40 %, fidélité visuelle 25 %, validation sur vos boîtes 20 %,
tests 15 %). Détail : fonctions ~99 %, couverture de RainLoop ~97 %, fidélité visuelle ~95 %, tests
automatiques ~95 %, **validation par vous ~55 %** — toujours le point faible, mais il monte.

## 2. Chiffres

| Mesure | Valeur |
|---|---|
| Commits | 122 |
| Paquets Go | 17, tous avec des tests (46 fichiers `_test.go`) |
| Routes HTTP | 96 (6 publiques, 90 sous session) — voir `docs/routes.md` |
| Matrice de test | 169 points : **90 répondus**, **70 vierges**, 0 non corrigé |
| Retours ouverts | 14 « à revalider », corrigés mais pas encore confirmés par vous |

## 3. Fait depuis le point du 24 octobre

**Exploitation.** Script de publication de l'image sur un registre (`make image-publish`), étiquettes
« AAAAMMJJ-commit » et `latest`, contrôles avant publication ; mode d'emploi (`docs/PUBLICATION.md`) et page
Docker Hub (`docs/dockerhub.md`). Droits du volume `/data` documentés : le conteneur tourne en **UID 65532**,
un volume `root:root` donne `open /data/server.key: permission denied`.

**Ligne de commande.** `user list` affiche en deuxième colonne le chemin du fichier chiffré de chaque compte :
sauvegarde ciblée, vérification des droits, diagnostic.

**Journal d'activité du compte** (demande du 25/10). Chiffré pour son seul propriétaire, 30 jours : connexions
réussies et **échouées**, refus de la limitation de débit, second facteur refusé, déconnexions, changement de
mot de passe, usage et régénération de la clé de récupération, (dés)activation du TOTP, ajout et retrait d'une
boîte, filtres modifiés, export du carnet. Un rappel à la connexion annonce la précédente et les échecs
survenus depuis. Les échecs sont écrits **compte fermé** grâce à une paire de clés X25519 par compte. Le
journal **ne peut pas être effacé depuis l'interface** : un intrus qui détient le mot de passe effacerait ses
propres traces.

**Documentation.** `docs/routes.md` (les 96 routes, paramètres et filtres compris), `AGENTS.md` (règles de
travail pour un agent), `docs/architecture.md` (carte des paquets, chemin d'une requête, invariants, chaîne de
chiffrement, inventaire du répertoire de données).

**Corrections.** Curseur de la liste conservé après une suppression ; voisin rouvert correctement ; fichiers
statiques marqués d'une empreinte et gardés un an, pour que vos correctifs soient visibles sans rechargement
forcé.

## 4. Ce qui reste, par ordre d'utilité

1. **Valider les 70 points vierges de la matrice** et les 14 retours « à revalider ». C'est ce qui fait le
   plus avancer le pourcentage réel.
2. **Publier l'image sur Docker Hub** : `make image-publish` n'a jamais été lancé. `tools/release/publier.env`
   est en place. Il restera à coller la description de `docs/dockerhub.md` dans Repository → General.
3. **Point d'étape après une semaine d'usage réel** — demandé, pas encore fait.
4. Fidélité visuelle : icônes pleines restantes, animations, largeurs par taille d'écran.
5. P3 cosmétiques, et les points de refonte signalés par l'audit (`home`, `buildMessageView`, voir
   `docs/architecture.md` § 9).

## 5. Propositions discutées, non engagées

Vous avez dit « laisse comme cela » ou « on discute seulement » : rien n'a été mis en œuvre.

- Poivre `server.key` dans la dérivation de clé.
- Phrase de passe proposée à la création, refus des mots de passe fuités.
- Archives de sauvegarde chiffrées (`make backup` produit aujourd'hui un `.tar.gz` en clair, qui contient
  `server.key` **et** tous les comptes).
- argon2id à 256 Mio avec mise à niveau automatique à la connexion.
- Journaliser l'envoi de message et les actions IMAP dans le journal d'activité (décision du 25/10 : on garde
  les 14 événements actuels).

## 6. À tester et valider par vous

1. Thème « DarkShine » et fuseau horaire dans Réglages → Général.
2. Le message d'invitation Google : **fournir son `.eml` dans `scratch/`** (écarts d'espacement et de liens).
3. Photo du correspondant : option Gravatar, photo dans une fiche de contact.
4. Recherche sur la grosse boîte (vitesse et résultats comparés à RainLoop).
5. Actualisation automatique, notifications, glisser-déposer, conversations.
6. Signatures et modèles mis en forme, envoi avec « Répondre à » et Cci.
7. 2FA avec votre application, CardDAV avec votre serveur, filtres Sieve (avec votre accord).
8. **Journal d'activité** : points A-13 à A-16 de la matrice. La pile HTTPS doit être relancée
   (`make tls-up-dev`) : seul le port 8080 a été reconstruit.

`MATRICE_DE_TEST.md` : un point par fonctionnalité, identifiant stable (`C-12`…), ligne `réponse :` à remplir
(`OK`, `KO : description`, `NA`). **À tenir à jour à chaque fonctionnalité ajoutée** : nouveau point en fin de
bloc, report des retours dans le « Journal des retours ».

## 7. Reprise du travail

Lire, dans cet ordre : **`AGENTS.md`** (règles absolues), `docs/architecture.md` (invariants et pièges),
`CAHIER_DES_CHARGES.md` (chapitres 11, 14, 15 et le journal des décisions), ce fichier,
`docs/audit-rainloop/` (sections « À AJOUTER »).

Rappel des règles : rien sur les vraies boîtes de l'utilisateur, jamais lire `data/` de RainLoop, tout en
conteneur (`make test-race`, `make test-js`, `make test-e2e`, `make sieve-up` puis `make test`).
Montages en lecture seule, absents du dépôt : `rainloop/1.17.0`, `rainloop_plugins/`.
