# Matrice de test de wmail-go (à compléter)

Remplissez la colonne **Résultat** avec `OK`, `KO` ou `NA`, et notez dans **Remarques** tout écart avec RainLoop
(capture d'écran bienvenue : chemin du fichier suffit).

- Version testée : `git rev-parse --short HEAD` = ................ Date : ................
- Navigateur : ................ Taille de fenêtre : ................ Thème : `DarkShine (comme RainLoop)`
- **Règle de sécurité** : aucun test destructif sur vos vraies boîtes. Pour les cas marqués ⚠, utilisez un compte de
  test ou un dossier de test, ou laissez `NA`.

Légende des colonnes : **Cas** = identifiant, **Comment tester** = marche à suivre, **Attendu** = comportement visé
(celui de RainLoop quand il existe).

---

## 1. Connexion, compte webmail, session

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| A-01 | Ouvrir `/login` | Carte centrée verticalement, fond DarkShine, icônes enveloppe et clé dans les champs | | |
| A-02 | Se connecter avec un mauvais mot de passe 6 fois | Message d'erreur, puis blocage temporaire (limitation de débit) | | |
| A-03 | Se connecter normalement | Arrivée sur la boîte de réception | | |
| A-04 | Activer la double authentification (Réglages > Sécurité), se déconnecter, se reconnecter | Demande du code TOTP, code de votre application accepté | | |
| A-05 | Cocher « appareil de confiance » au second facteur, se reconnecter | Plus de code demandé sur ce navigateur | | |
| A-06 | Utiliser un code de secours | Accepté une seule fois | | |
| A-07 | Afficher la clé de récupération puis la télécharger | Fichier `wmail-go-recovery-key.txt` correct | | |
| A-08 | Ouvrir `/settings/folders`, attendre l'expiration de session, recharger | Retour à la connexion **puis** retour automatique sur `/settings/folders` après connexion | | |
| A-09 | Changer le mot de passe du compte webmail | Les autres sessions sont fermées | | |
| A-10 | Changer la langue (fr/en) | Toute l'interface suit, y compris les menus | | |

## 2. Boîtes mail (IMAP/SMTP)

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| B-01 | Ajouter une boîte avec l'adresse seule | Serveurs proposés automatiquement | | |
| B-02 | Bouton « Test » IMAP et SMTP | Diagnostic clair (certificat, authentification, TLS hérité) | | |
| B-03 | Ajouter une deuxième boîte et basculer de l'une à l'autre | Liste, dossiers et compteurs changent ; aucun mélange | | |
| B-04 | Réglages de la boîte : dossiers système | Sélecteurs de dossiers existants, pas de saisie libre | | |
| B-05 | Identités : ajouter une identité secondaire avec signature | Choix de l'expéditeur à la composition, signature liée | | |
| B-06 | Options par boîte : « Répondre à », Cci systématique | Appliqués aux messages envoyés (vérifier les en-têtes reçus) | | |
| B-07 | Quota (si le serveur l'annonce) | Occupation affichée en bas du panneau des dossiers | | |

## 3. Liste des messages

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| C-01 | Comparer la liste à celle de RainLoop (même dossier) | Mêmes colonnes, mêmes couleurs, même densité | | |
| C-02 | Message non lu | Fond jaune pâle, barre orange, texte gras | | |
| C-03 | Conversation dont seul un ancien message est non lu | Fond atténué, texte **non** gras (`.subunread`) | | |
| C-04 | Trier par date, expéditeur, objet, taille | Ordre correct, flèche de tri visible | | |
| C-05 | Changer la taille de page (10 à 500) | Pagination correcte, choix mémorisé | | |
| C-06 | Recherche `from:`, `to:`, `is:unseen`, `date:`, `size:` | Mêmes résultats que RainLoop, **vitesse comparable** | | |
| C-07 | Recherche texte libre sur une grosse boîte | Résultats et temps comparables à RainLoop | | |
| C-08 | Bandeau « Résultats de recherche » et croix de remise à zéro | Présents et fonctionnels | | |
| C-09 | Filtre « non lus » | Liste filtrée, compteur cohérent | | |
| C-10 | Mode conversations | Regroupement identique à RainLoop, pastille du nombre | | |
| C-11 | Clic simple sur une ligne | Ouvre le message dans le volet de lecture | | |
| C-12 | **Ctrl+clic** sur une ligne | Coche la ligne sans ouvrir le message | | |
| C-13 | **Maj+clic** sur une autre ligne | Étend la sélection de la dernière ligne cochée jusqu'à celle-ci | | |
| C-14 | **Maj+↓** plusieurs fois puis **Maj+↑** | La plage se réduit : les lignes quittées sont décochées | | |
| C-15 | Alt+clic ou clic du milieu | Ouvre le message dans un nouvel onglet | | |
| C-16 | Flèches ↑ ↓, Début, Fin, Page préc./suiv. | Curseur et ouverture conformes ; passage de page aux extrémités | | |
| C-17 | Flèches ← → | Passage du focus entre la liste et le volet de lecture | | |
| C-18 | Touche `?` | Aide des raccourcis à jour (Maj+Suppr, ← →) | | |
| C-19 | Glisser-déposer d'un message vers un dossier | Déplacement, surbrillance du dossier cible | | |
| C-20 | Menu « Déplacer vers » ; Ctrl enfoncé | Déplacement ; avec Ctrl, copie (libellé qui change) | | |
| C-21 | Actions de masse : lu, non lu, favori, archiver, spam, corbeille | Appliquées à toute la sélection, retour à la liste | | |
| C-22 | **Maj+Suppr** sur une sélection ⚠ | Confirmation puis suppression définitive (sans passer par la corbeille) | | |
| C-23 | « Marquer tout le dossier comme lu » / « comme non lu » | Compteurs à jour | | |
| C-24 | « Transférer en pièces jointes » sur 3 messages cochés | Nouveau message avec 3 `.eml`, objet « Fwd : 3 messages » | | |
| C-25 | Sélection par critère (tous, aucun, lus, non lus, suivis, inverser) | Conforme | | |
| C-26 | Arrivée d'un nouveau message (s'envoyer un message) | La liste se met à jour seule, sans recharger la page | | |
| C-27 | Lire un message depuis un autre client | Au sondage suivant, la ligne passe en « lu » sans rechargement | | |
| C-28 | Vider la corbeille / les indésirables ⚠ | Confirmation explicite, compteur remis à zéro | | |
| C-29 | Supprimer 500 messages d'un coup ⚠ | L'interface rend la main, journal lisible dans `make logs` | | |

## 4. Lecture d'un message

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| D-01 | Ouvrir un message HTML complexe (newsletter) | Rendu identique à RainLoop, pas de débordement | | |
| D-02 | Même message dans RainLoop et wmail-go, comparer | Espacement, polices, couleurs, liens | | |
| D-03 | Images distantes bloquées | Bandeau « afficher les images », option « toujours pour cet expéditeur » | | |
| D-04 | Message texte brut avec citations | Police à chasse fixe, citations colorées et repliées | | |
| D-05 | Message `format=flowed` | Pas de coupures de ligne parasites | | |
| D-06 | Message avec charset exotique (ISO-8859-1, Windows-1252, UTF-7) | Texte lisible, pas de perte | | |
| D-07 | Message signé S/MIME ou PGP | La signature n'apparaît **pas** comme pièce jointe | | |
| D-08 | Signature HTML de l'expéditeur | Affichée dans le corps, pas en pièce jointe | | |
| D-09 | Pièce jointe image | Vignette avec œil en surimpression, aperçu plein écran sans téléchargement | | |
| D-10 | Pièces jointes PDF, ZIP, XLSX, MP3 | Icône différente selon le type | | |
| D-11 | Noms des pièces jointes | Identiques à RainLoop (y compris `calendar1.3.ics`) | | |
| D-12 | « Télécharger toutes les pièces jointes » | ZIP correct | | |
| D-13 | Invitation `.ics` (plug-in) | Carte d'événement : titre, date, De, Invités, boutons Google et Outlook | | |
| D-14 | Photo du correspondant | Carnet d'adresses d'abord, Gravatar si l'option est active | | |
| D-15 | Flèches ↑ ↓ en haut à droite du message | Message précédent et suivant de la liste ; grisées aux extrémités | | |
| D-16 | Boutons répondre / répondre à tous / transférer | Trois icônes RainLoop visibles dans l'en-tête | | |
| D-17 | Menu « … » : voir la source, les en-têtes, imprimer, filtrer, ajouter aux contacts | Tout fonctionne | | |
| D-18 | Message demandant un accusé de lecture | Bandeau, envoi de l'accusé seulement sur action | | |
| D-19 | Message marqué important | Icône ou couleur distincte dans la liste et en lecture | | |
| D-20 | Plein écran du message (bouton et Échap) | Bascule correcte | | |

## 5. Composition

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| E-01 | Nouveau message, barre de composition | Barre sombre 40 px, en-tête gris, comme RainLoop | | |
| E-02 | Saisir des adresses | Pastilles grises carrées, autocomplétion (20 suggestions) | | |
| E-03 | Sélecteur de contacts | Les adresses vont dans le **dernier champ utilisé** (À, Cc ou Cci) | | |
| E-04 | Éditeur de texte mis en forme | Gras, listes, couleurs, liens ; bascule texte/HTML | | |
| E-05 | Signature de l'identité | Insérée au bon endroit (avant/après la citation) | | |
| E-06 | Répondre / Répondre à tous | Destinataires et citation corrects, curseur au-dessus | | |
| E-07 | Transférer | Pièces jointes d'origine conservées | | |
| E-08 | Joindre un fichier (glisser-déposer et bouton) | Progression, taille limite respectée | | |
| E-09 | Envoyer sans objet | Avertissement avant envoi | | |
| E-10 | Écrire « ci-joint » sans pièce jointe | Avertissement avant envoi | | |
| E-11 | Brouillon automatique | Enregistré toutes les minutes, repris à l'ouverture | | |
| E-12 | Brouillon écrit dans RainLoop puis ouvert dans wmail-go | Réponse/transfert reconnus (`X-Draft-Info`) | | |
| E-13 | Modèles de message | Création, insertion, mise en forme conservée | | |
| E-14 | Demander un accusé de lecture | En-tête `Disposition-Notification-To` présent | | |
| E-15 | Envoyer à une adresse à domaine accentué (é, ü) | Envoi accepté (punycode) | | |
| E-16 | Envoi vers une adresse inexistante | Message d'erreur lisible, brouillon conservé | | |
| E-17 | Copie dans « Envoyés » | Présente, Cci visible seulement dans la copie | | |
| E-18 | Ctrl+Entrée / Ctrl+S | Envoi / enregistrement | | |

## 6. Contacts

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| F-01 | Ouvrir `/contacts` | Deux volets : liste à gauche, fiche à droite | | |
| F-02 | Créer, modifier, supprimer un contact ⚠ | Conforme | | |
| F-03 | Photo de contact | Envoi, affichage, suppression | | |
| F-04 | Recherche par nom, par étiquette | Conforme | | |
| F-05 | Recherche par numéro (« 0612 » pour `+33 6 12 34 56 78`) | Contact trouvé | | |
| F-06 | Import vCard et CSV (export d'Outlook ou de Google) | Champs typés repris (Home/Business/Other, Fax, Mobile) | | |
| F-07 | Export vCard puis réimport | Aucune perte, propriétés inconnues conservées | | |
| F-08 | Synchronisation CardDAV avec votre serveur | Ajouts et modifications dans les deux sens | | |
| F-09 | « Écrire à la sélection » | Nouveau message avec toutes les adresses cochées | | |
| F-10 | Ajouter l'expéditeur d'un message aux contacts | Fiche créée avec nom et adresse | | |

## 7. Dossiers

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| G-01 | Arborescence | Même ordre que RainLoop, INBOX en tête, dossiers système ensuite | | |
| G-02 | Dossiers nommés « Projet 2 » et « Projet 10 » | Tri naturel : 2 avant 10 | | |
| G-03 | Replier / déplier un niveau | État mémorisé | | |
| G-04 | Créer un dossier (bouton en bas à gauche) | Nom + dossier parent, création immédiate | | |
| G-05 | Renommer un dossier ⚠ | Sous-dossiers suivis | | |
| G-06 | Supprimer un dossier non vide ou avec sous-dossiers ⚠ | Refus explicite | | |
| G-07 | Masquer le panneau des dossiers (bouton flèches) | Le panneau se replie, la liste s'élargit | | |
| G-08 | Pastilles de compteurs | Brouillons = total ; corbeille, archive, envoyés = rien ; autres = non lus | | |
| G-09 | Dossiers surveillés (Réglages > Dossiers) | Compteurs rafraîchis toutes les 2 minutes | | |
| G-10 | Abonnement IMAP | Dossiers non abonnés masqués | | |

## 8. Filtres (Sieve) ⚠

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| H-01 | Ouvrir `/settings/filters` | Liste des règles, script actif | | |
| H-02 | Créer une règle « de X vers dossier Y » | Script valide, appliqué par le serveur | | |
| H-03 | Modifier l'ordre des règles | Conforme | | |
| H-04 | Édition du script brut | Vérification de syntaxe avant enregistrement | | |
| H-05 | Serveur sans ManageSieve | Message clair, pas de plantage | | |

## 9. Réglages et apparence

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| I-01 | Thème « DarkShine (comme RainLoop) » | Image de fond visible sur la barre du haut et le panneau des dossiers | | |
| I-02 | Thèmes clair et sombre | Lisibles, pas de texte invisible | | |
| I-03 | Fuseau horaire par utilisateur | Dates affichées dans ce fuseau | | |
| I-04 | Disposition « 3 volets » | Conforme à RainLoop | | |
| I-05 | Disposition « aperçu en bas » | Liste en haut, lecture dessous | | |
| I-06 | Disposition « liste seule » | Le message s'ouvre sur une page entière | | |
| I-07 | Taille de police | Prise en compte partout | | |
| I-08 | Notifications bureau et son | Autorisation demandée, notification à l'arrivée d'un message | | |
| I-09 | Barre « Retour » des réglages | Au-dessus de la carte blanche, sur le fond d'écran | | |
| I-10 | Image de fond personnalisée | `make theme-background FILE=...` prise en compte | | |

## 10. Sécurité et robustesse

| Cas | Comment tester | Attendu | Résultat | Remarques |
|---|---|---|---|---|
| J-01 | Message contenant `<script>` | Aucun script exécuté (CSP), texte visible tel quel | | |
| J-02 | Pixel espion | Signalé ou bloqué avec les images distantes | | |
| J-03 | Formulaire soumis depuis un autre site | Refus CSRF | | |
| J-04 | Serveur IMAP sur une adresse privée (192.168.x) | Refus, sauf si `allowed_private_hosts` l'autorise | | |
| J-05 | Derrière votre proxy HTTPS | En-tête `Strict-Transport-Security` présent | | |
| J-06 | Rechargement d'une page | Fichiers statiques servis en 304 (cache) | | |
| J-07 | Arrêt du serveur IMAP pendant l'usage | Message d'erreur clair, reconnexion automatique ensuite | | |
| J-08 | `make logs` pendant une action longue | Journal IMAP/SMTP lisible, durées indiquées | | |
| J-09 | Redémarrage du conteneur | Comptes et données chiffrées conservés | | |

## 11. Comparaison directe avec RainLoop

Pour chaque écran, mettre les deux côte à côte et noter l'écart le plus visible.

| Cas | Écran | Écart constaté | Résultat | Remarques |
|---|---|---|---|---|
| K-01 | Connexion | | | |
| K-02 | Liste + volet de lecture | | | |
| K-03 | Message avec pièces jointes | | | |
| K-04 | Composition | | | |
| K-05 | Contacts | | | |
| K-06 | Réglages | | | |
| K-07 | Filtres | | | |

---

## Suivi

| Bloc | Cas | OK | KO | NA |
|---|---|---|---|---|
| 1. Connexion | 10 | | | |
| 2. Boîtes mail | 7 | | | |
| 3. Liste | 29 | | | |
| 4. Lecture | 20 | | | |
| 5. Composition | 18 | | | |
| 6. Contacts | 10 | | | |
| 7. Dossiers | 10 | | | |
| 8. Filtres | 5 | | | |
| 9. Réglages | 10 | | | |
| 10. Sécurité | 9 | | | |
| 11. Comparaison | 7 | | | |
| **Total** | **135** | | | |
