# wmail-go — trame de test et de validation

Document à remplir par l'utilisateur, point par point. Il sert aussi de journal : j'y reporte les corrections.

## Mode d'emploi

Chaque point a un **identifiant stable** (`C-12`, `D-09`…). Répondez sur la ligne `réponse :`.

```
[X-00] Exemple (ce point n'existe pas : il montre seulement la forme attendue)
  tester  : Ctrl (ou Cmd) + clic sur une ligne de la liste
  attendu : la case se coche, le message ne s'ouvre pas
  réponse : OK
```

- `OK` : conforme, rien à faire.
- `KO : <description>` : décrivez ce que vous voyez (et, si possible, le chemin d'une capture d'écran).
- `NA` : non testé ou sans objet.
- Vous pouvez répondre en vrac dans le chat (« C-12 KO : la case ne se coche pas ») : je reporte dans ce fichier.

**Règle de sécurité** : les points marqués ⚠ détruisent ou déplacent des messages. Ne les passez que sur un compte
ou un dossier de test. Je ne touche jamais à vos vraies boîtes.

- Version testée : `git rev-parse --short HEAD` = ............ — date : ............
- Navigateur : ............ — taille de fenêtre : ............ — thème : `DarkShine (comme RainLoop)`
- Avant de tester une nouvelle version : `make up` (le conteneur redémarre, la session est perdue).

---

## A. Connexion, compte webmail, session

```
[A-01] Écran de connexion fidèle à RainLoop
  tester  : ouvrir /login
  attendu : carte centrée verticalement, fond DarkShine, icônes enveloppe et clé dans les champs
  réponse : OK

[A-02] Limitation des tentatives
  tester  : 6 connexions avec un mauvais mot de passe
  attendu : erreur à chaque fois, puis blocage temporaire annoncé
  réponse : OK

[A-03] Connexion normale
  tester  : se connecter avec le bon mot de passe
  attendu : arrivée sur la boîte de réception, dernier compte mail actif
  réponse : OK

[A-04] Double authentification (TOTP)
  tester  : Réglages > Sécurité, activer, se déconnecter, se reconnecter
  attendu : code demandé, code de votre application accepté
  réponse : OK

[A-05] Appareil de confiance
  tester  : cocher la case au second facteur, se reconnecter
  attendu : plus de code demandé sur ce navigateur
  réponse : OK

[A-06] Code de secours
  tester  : utiliser un code de secours au lieu du code TOTP
  attendu : accepté une seule fois, puis refusé
  réponse :

[A-12] Revoir le secret 2FA
  tester  : Réglages > Sécurité, « Revoir le secret », saisir un code valide
  attendu : secret et QR affichés une seule fois ; refus avec un code faux
  réponse : OK mais GET ￼http://localhost:8080/settings/2fa/qr.png?again=1 404 (Not Found)

[A-07] Clé de récupération
  tester  : afficher puis télécharger la clé
  attendu : fichier wmail-go-recovery-key.txt correct ; la clé permet de récupérer le compte
  réponse : OK

[A-08] Retour à la page demandée après reconnexion
  tester  : ouvrir /settings/folders, attendre l'expiration de session, recharger, se reconnecter
  attendu : retour automatique sur /settings/folders (et non sur l'accueil)
  réponse : 

[A-09] Changement de mot de passe
  tester  : Réglages > Sécurité, changer le mot de passe
  attendu : les autres sessions sont fermées, la nouvelle session reste valide
  réponse : OK

[A-11] Message « session expirée »
  tester  : laisser la session expirer, recharger une page
  attendu : bandeau « Votre session a expiré » sur l'écran de connexion, puis retour à la page demandée
  réponse : OK

[A-10] Langue de l'interface
  tester  : basculer fr / en
  attendu : toute l'interface suit, menus compris ; choix mémorisé
  réponse : OK
```

## B. Boîtes mail (IMAP / SMTP)

```
[B-01] Suggestion automatique des serveurs
  tester  : ajouter une boîte en saisissant seulement l'adresse
  attendu : serveurs, ports et sécurité proposés
  réponse : OK

[B-02] Bouton « Test »
  tester  : tester IMAP puis SMTP, y compris avec un mauvais mot de passe
  attendu : diagnostic clair (authentification, certificat, TLS hérité)
  réponse : OK

[B-03] Plusieurs boîtes
  tester  : ajouter une deuxième boîte, basculer de l'une à l'autre
  attendu : liste, dossiers, compteurs et signature changent ; aucun mélange
  réponse : OK

[B-04] Dossiers système d'une boîte
  tester  : Réglages > Dossiers, section « dossiers système »
  attendu : sélecteurs de dossiers existants (pas de saisie libre), choix « Automatique » par défaut
  réponse : OK

[B-05] Identités
  tester  : ajouter une identité secondaire avec sa signature
  attendu : choix de l'expéditeur à la composition, signature liée à l'identité
  réponse : OK

[B-06] Répondre à / Cci systématique
  tester  : renseigner ces champs dans les options de la boîte, envoyer un message
  attendu : en-têtes Reply-To et Bcc présents dans le message reçu
  réponse : OK

[B-11] Serveur refusant LOGIN (LOGINDISABLED)
  tester  : boîte dont le serveur annonce LOGINDISABLED
  attendu : connexion par AUTHENTICATE PLAIN, sans erreur
  réponse :

[B-10] Test ManageSieve inclus
  tester  : bouton « Test » d'une boîte dont les filtres sont actifs
  attendu : trois résultats — IMAP, SMTP, ManageSieve
  réponse :

[B-12] Réordonner par glisser-déposer
  tester  : glisser une ligne dans la liste des comptes, puis des identités, puis des modèles
  attendu : nouvel ordre conservé après rechargement ; sans JavaScript les pages restent utilisables
  réponse : OK

[B-08] Boîte en double refusée
  tester  : ajouter deux fois la même adresse sur le même serveur IMAP
  attendu : refus explicite au deuxième ajout
  réponse :

[B-09] Alias de dossiers système
  tester  : boîte dont les dossiers s'appellent « Sent Mail », « Bin », « All Mail », « Courrier indésirable »…
  attendu : reconnus automatiquement comme envoyés, corbeille, archive, indésirables
  réponse :

[B-07] Quota
  tester  : boîte dont le serveur annonce QUOTA
  attendu : occupation et pourcentage en bas du panneau des dossiers ; barre verte visible même à moins de 1 %
            (rouge au-delà de 90 %) ; le texte garde la valeur exacte
  réponse : OK
```

## C. Liste des messages

```
[C-01] Aspect général
  tester  : comparer le même dossier dans RainLoop et dans wmail-go
  attendu : mêmes colonnes, mêmes couleurs, même densité
  réponse : OK

[C-33] Objets et expéditeurs encodés (RFC 2047)
  tester  : messages dont l'en-tête contient « =?utf-8?q?… » ou « =?Windows-1252?Q?… », y compris entre guillemets
  attendu : texte décodé partout — liste, volet de lecture, colonne expéditeur
  réponse : OK

[C-02] Message non lu
  attendu : fond jaune pâle, barre orange à gauche, expéditeur et objet en gras
  réponse : OK

[C-03] Conversation partiellement non lue
  tester  : fil dont seul un message ancien est non lu
  attendu : fond atténué, texte non gras (différent d'un vrai non lu)
  réponse :

[C-04] Tri
  tester  : trier par date, expéditeur, objet, taille, dans les deux sens
  attendu : ordre correct, repère de tri visible, choix conservé en changeant de page
  réponse : OK

[C-05] Taille de page
  tester  : 10, 50, 100, 500 messages par page
  attendu : pagination correcte, choix mémorisé
  réponse : OK

[C-06] Recherche par opérateurs
  tester  : from:, to:, is:unseen, is:flagged, date:2026.10.05, size:2MB
  attendu : mêmes résultats que RainLoop
  réponse : ERROR from: pascalinejaouen@yahoo.fr ne retourne rien alors que recherche simple pascalinejaouen@yahoo.fr retourne des resultats

[C-07] Recherche texte libre sur une grosse boîte
  attendu : résultats et temps comparables à RainLoop (pas de recherche dans le corps)
  réponse : OK

[C-08] Bandeau et croix de recherche
  attendu : bandeau « Résultats de recherche pour … » et croix qui vide le champ
  réponse : WARNING : probleme d'affichage de la croix (on a l'impression qu'il a 2 croix mals alignées

[C-09] Filtre « non lus »
  attendu : liste filtrée, compteur cohérent, retour à « tous » possible
  réponse : OK

[C-10] Mode conversations
  attendu : regroupement identique à RainLoop, pastille du nombre de messages
  réponse :

[C-11] Clic simple sur une ligne
  attendu : ouvre le message dans le volet de lecture, sans recharger la page
  réponse : OK

[C-12] Ctrl+clic coche la ligne sans ouvrir le message
  tester  : Ctrl (ou Cmd) + clic sur une ligne
  attendu : la case se coche ou se décoche, le message ne s'ouvre pas
  réponse : OK

[C-13] Maj+clic étend la sélection
  tester  : cocher une ligne, puis Maj+clic sur une ligne plus bas
  attendu : toutes les lignes intermédiaires sont cochées
  réponse : OK

[C-14] Maj+flèches avec changement de sens
  tester  : Maj+↓ trois fois, puis Maj+↑
  attendu : la plage se réduit, la ligne quittée est décochée
  réponse : OK

[C-15] Ouverture dans un nouvel onglet
  tester  : clic du bouton du milieu (Alt+clic, dans Chrome, télécharge la cible : comportement du navigateur,
            pas de l'application)
  attendu : le message s'ouvre dans un nouvel onglet (pas de sélection)
  réponse : ERROR : ça download la page au lieu de l'afficher

[C-16] Navigation au clavier
  tester  : ↑ ↓ Début Fin PagePréc PageSuiv
  attendu : curseur et ouverture conformes ; au bout de la page, passage à la page suivante
  réponse : BUG: on ne voit pas de changement visuel du message selectionné

[C-17] Bascule entre volets
  tester  : → puis ←
  attendu : le focus passe au volet de lecture puis revient à la liste
  réponse : OK

[C-18] Aide des raccourcis
  tester  : touche ?
  attendu : fenêtre large, voile sombre derrière, liste à jour (Maj+Suppr, ← →)
  réponse : OK

[C-19] Glisser-déposer vers un dossier ⚠
  attendu : déplacement effectif, dossier cible en surbrillance pendant le glisser
  réponse : OK

[C-34] Repères pendant le glisser
  tester  : glisser deux messages cochés vers un dossier
  attendu : pastille indiquant « 2 » près du curseur, ligne inclinée et rayée, dossier cible mis en évidence
  réponse : OK

[C-20] Menu « Déplacer vers » ⚠
  tester  : menu seul, puis avec Ctrl enfoncé
  attendu : déplacement ; avec Ctrl, copie (le libellé du menu change)
  réponse : OK

[C-21] Actions de masse ⚠
  tester  : lu, non lu, favori, archiver, indésirable, corbeille sur une sélection
  attendu : appliquées à toute la sélection, retour à la liste au bon endroit
  réponse : OK

[C-22] Suppression définitive (Maj+Suppr) ⚠
  attendu : confirmation, puis suppression sans passer par la corbeille
  réponse : OK

[C-23] Tout marquer lu / non lu
  tester  : menu « … » de la barre d'outils
  attendu : tout le dossier change d'état, compteurs à jour
  réponse : OK

[C-24] Transférer en pièces jointes
  tester  : cocher 3 messages, menu « … » > « Transférer en pièces jointes »
  attendu : nouveau message avec 3 fichiers .eml nommés d'après leur objet, objet « Fwd : 3 messages »
  réponse : OK

[C-25] Sélection par critère
  tester  : tous, aucun, lus, non lus, suivis, non suivis, inverser
  attendu : conforme
  réponse : OK

[C-26] Actualisation automatique
  tester  : s'envoyer un message depuis un autre client
  attendu : la liste se met à jour seule, sans recharger, sans perdre la sélection ni le défilement
  réponse :

[C-27] Drapeaux changés ailleurs
  tester  : lire un message depuis un autre client
  attendu : au sondage suivant (2 min), la ligne passe en « lu » sans rechargement
  réponse :

[C-28] Vider la corbeille / les indésirables ⚠
  attendu : confirmation explicite, dossier vidé, compteur à zéro
  réponse : OK

[C-29] Action de masse sur 500 messages ⚠
  attendu : l'interface rend la main, journal lisible (make logs), pas de blocage
  réponse : OK

[C-31] Voisin ouvert après suppression du message ouvert ⚠
  tester  : ouvrir le 2e message d'une liste, le supprimer
  attendu : le message suivant s'ouvre (ou le précédent s'il n'y a pas de suivant), pas de retour à la liste vide
  réponse :

[C-32] Import d'un .eml par glisser-déposer
  tester  : déposer un fichier .eml sur la liste
  attendu : message ajouté au dossier avec sa date d'origine (pas la date du jour)
  réponse :

[C-30] Liste d'une ligne (disposition « liste seule »)
  tester  : Réglages > Général > disposition « Liste seule »
  attendu : lignes de 35 px, expéditeur à gauche, objet extensible, date à droite
  réponse : OK
```

## D. Lecture d'un message

```
[D-01] Message HTML complexe (newsletter)
  attendu : rendu identique à RainLoop, pas de débordement horizontal
  réponse : OK

[D-02] Comparaison directe d'un même message
  tester  : ouvrir le même message dans les deux webmails
  attendu : espacement, polices, couleurs et liens identiques
  réponse : OK

[D-27] Domaine international lisible
  tester  : message venant d'une adresse à domaine accentué (punycode xn--…)
  attendu : le domaine s'affiche en clair ; un domaine trompeur reste en xn--
  réponse :

[D-26] Images incorporées d'Outlook (Content-Location)
  tester  : message Outlook dont les images sont référencées par une URL et jointes au message
  attendu : images affichées sans autoriser les images distantes, pas listées en pièces jointes
  réponse :

[D-03] Images distantes
  attendu : bloquées, bandeau pour les afficher, option « toujours pour cet expéditeur »
  réponse : OK

[D-04] Texte brut et citations
  attendu : police à chasse fixe, citations colorées et repliables
  réponse : OK

[D-05] format=flowed
  attendu : pas de coupures de ligne parasites
  réponse :

[D-06] Charsets exotiques
  tester  : messages ISO-8859-1, Windows-1252, UTF-7, charset absent
  attendu : texte lisible, aucun message vide
  réponse :

[D-07] Signature S/MIME ou PGP
  attendu : la signature n'apparaît pas comme pièce jointe
  réponse :

[D-08] Signature HTML de l'expéditeur
  attendu : affichée dans le corps, pas en pièce jointe
  réponse : OK

[D-09] Aperçu d'une image jointe
  attendu : vignette avec œil en surimpression, aperçu plein écran sans téléchargement
  réponse : OK mais l'aperçu plein ecran affiche l'image precedente pendant le chargement de l'image courante

[D-25] Photo verticale en pièce jointe
  tester  : joindre une photo prise à la verticale (orientation EXIF 6 ou 8)
  attendu : la vignette est redressée, pas couchée
  réponse :

[D-10] Icônes par type de pièce jointe
  tester  : PDF, ZIP, XLSX, PPTX, MP3, MP4, TXT, ICS
  attendu : icône différente selon le type
  réponse : OK

[D-11] Noms des pièces jointes
  attendu : identiques à RainLoop, y compris les noms engendrés (calendar1.3.ics)
  réponse : OK

[D-24] Source d'une pièce jointe
  tester  : cliquer le petit « i » en bas à droite d'une vignette
  attendu : contenu décodé de la partie, en texte brut, dans un nouvel onglet
  réponse : OK

[D-23] ZIP des pièces jointes sélectionnées
  tester  : cocher la case en haut à gauche de deux vignettes, menu de l'engrenage > « Télécharger la sélection »
  attendu : archive contenant seulement ces deux fichiers
  réponse : ERROR : je ne vois pas les cases a cocher ?

[D-12] Télécharger toutes les pièces jointes
  attendu : archive ZIP correcte
  réponse : OK

[D-13] Invitation .ics (votre plug-in)
  attendu : carte d'événement — titre, date, De, Invités, boutons Google et Outlook — sans bloc superflu
  réponse : OK

[D-14] Photo du correspondant
  attendu : photo du carnet d'abord, Gravatar si l'option est active, icône générique sinon
  réponse : OK

[D-15] Flèches message précédent / suivant
  attendu : en haut à droite, suivent l'ordre de la liste, grisées aux extrémités
  réponse : ERROR : fleches inoperantes et toujours grisées

[D-16] Boutons de réponse
  attendu : trois icônes RainLoop (répondre, répondre à tous, transférer) dans l'en-tête
  réponse : OK

[D-17] Menu « … » du message
  tester  : source, en-têtes, imprimer, filtrer les messages semblables, ajouter aux contacts, voir en texte brut
  attendu : toutes les entrées fonctionnent
  réponse : OK

[D-18] Accusé de lecture demandé
  attendu : bandeau ; l'accusé n'est envoyé que si vous cliquez
  réponse : OK

[D-19] Message important
  attendu : repère visible dans la liste et en lecture
  réponse : OK

[D-28] Impression d'un message
  tester  : menu « … » > Imprimer, puis aperçu avant impression
  attendu : objet, De, À, Cc, Cci, Répondre à, Date, pièces jointes en liste ; corps lisible ; URL des liens entre
            parenthèses ; aucune barre d'outils ; pas d'aplat de couleur
  réponse : OK

[D-20] Plein écran du message
  tester  : bouton, puis Échap
  attendu : bascule correcte, la liste réapparaît
  réponse : ?? Quel bouton ? comment faire ?

[D-22] Aperçu PDF intégré
  tester  : cliquer sur l'œil d'une pièce jointe PDF
  attendu : le PDF s'affiche dans la visionneuse (pas de téléchargement, pas de nouvel onglet)
  réponse : ERROR : Refused to display '￼http://localhost:8080/' in a frame because it set 'X-Frame-Options' to 'deny'.

[D-21] Bouton « haut de page »
  tester  : ouvrir un long message et faire défiler
  attendu : bouton rond en bas à droite, retour en haut au clic
  réponse :
```

## E. Composition

```
[E-01] Barre de composition
  attendu : barre sombre de 40 px, en-tête gris, comme RainLoop
  réponse : OK

[E-02] Saisie des adresses
  attendu : pastilles grises carrées, autocomplétion (20 suggestions), adresses invalides signalées
  réponse : OK

[E-03] Sélecteur de contacts
  tester  : cliquer dans Cc, puis ouvrir le sélecteur et choisir un contact
  attendu : l'adresse va dans Cc (dernier champ utilisé), pas dans À
  réponse : OK

[E-04] Éditeur mis en forme
  tester  : gras, italique, listes, couleurs, alignement, lien ; bascule texte / mise en forme
  attendu : conforme, conversion sans perte
  réponse : OK

[E-05] Signature
  attendu : insérée au bon endroit selon l'option (avant ou après la citation)
  réponse :

[E-06] Répondre / Répondre à tous
  attendu : destinataires corrects, citation correcte, curseur au-dessus de la citation
  réponse : ERROR : erreur de parsing/encodage des emails =?utf-8?q?St=C3=...

[E-07] Transférer
  attendu : pièces jointes d'origine conservées, en-tête de transfert comme RainLoop
  réponse : OK

[E-08] Joindre un fichier
  tester  : bouton et glisser-déposer, fichier volumineux
  attendu : progression, limite de taille annoncée clairement
  réponse : OK

[E-09] Envoi sans objet
  attendu : avertissement avant envoi
  réponse : OK

[E-10] « ci-joint » sans pièce jointe
  attendu : avertissement avant envoi
  réponse :

[E-11] Brouillon automatique
  attendu : enregistré environ chaque minute, repris à l'ouverture, pas de doublon
  réponse :

[E-12] Brouillon écrit dans RainLoop
  tester  : créer un brouillon de réponse dans RainLoop, l'ouvrir dans wmail-go
  attendu : reconnu comme réponse (en-tête X-Draft-Info)
  réponse :

[E-13] Modèles de message
  attendu : création, insertion, mise en forme conservée
  réponse :

[E-14] Demander un accusé de lecture
  attendu : en-tête Disposition-Notification-To dans le message reçu
  réponse : OK

[E-15] Domaine accentué
  tester  : envoyer à une adresse dont le domaine contient é ou ü
  attendu : envoi accepté (punycode), adresse lisible dans « Envoyés »
  réponse :

[E-16] Destinataire inexistant
  attendu : erreur lisible, brouillon conservé
  réponse :

[E-17] Copie dans « Envoyés »
  attendu : message présent ; Cci visible seulement dans la copie, jamais chez les destinataires
  réponse : OK

[E-18] Raccourcis
  tester  : Ctrl+Entrée, Ctrl+S
  attendu : envoi, enregistrement
  réponse : OK
```

## F. Contacts

```
[F-01] Vue en deux volets
  attendu : liste étroite à gauche, fiche à droite, contact courant mis en évidence
  réponse :

[F-02] Créer, modifier, supprimer ⚠
  attendu : conforme, confirmation avant suppression
  réponse :

[F-03] Photo de contact
  attendu : envoi, affichage dans la fiche et dans les messages, suppression
  réponse :

[F-04] Recherche par nom et par étiquette
  attendu : conforme
  réponse :

[F-05] Recherche par numéro
  tester  : chercher « 0612 » pour un contact enregistré « +33 6 12 34 56 78 »
  attendu : contact trouvé
  réponse :

[F-06] Import vCard et CSV
  tester  : export d'Outlook, de Google, de RainLoop
  attendu : champs typés repris (Home / Business / Other, Fax, Mobile), pas de doublon
  réponse :

[F-07] Export puis réimport
  attendu : aucune perte, propriétés inconnues conservées
  réponse :

[F-08] Synchronisation CardDAV
  attendu : ajouts et modifications dans les deux sens, erreurs lisibles
  réponse :

[F-09] Écrire à la sélection
  attendu : nouveau message avec toutes les adresses cochées
  réponse :

[F-10] Ajouter l'expéditeur aux contacts
  attendu : fiche créée avec nom et adresse, ouverte pour complément
  réponse :
```

## G. Dossiers

```
[G-01] Arborescence
  attendu : même ordre que RainLoop — INBOX, dossiers système, puis les autres
  réponse : OK

[G-02] Tri naturel
  tester  : dossiers « Projet 2 » et « Projet 10 »
  attendu : 2 avant 10
  réponse : OK

[G-03] Replier / déplier
  attendu : état mémorisé entre deux visites
  réponse : OK

[G-04] Créer un dossier
  tester  : bouton en bas à gauche
  attendu : nom et dossier parent, création immédiate, dossier visible
  réponse :

[G-05] Renommer ⚠
  attendu : sous-dossiers suivis, compteurs corrects
  réponse :

[G-06] Garde-fous de suppression ⚠
  tester  : supprimer un dossier système, un dossier non vide, un dossier avec sous-dossiers
  attendu : refus explicite dans les trois cas
  réponse :

[G-07] Vider un dossier ⚠
  tester  : Réglages > Dossiers, lien « Vider le dossier »
  attendu : confirmation, puis dossier vide
  réponse : OK

[G-08] Masquer le panneau des dossiers
  tester  : bouton flèches en bas à gauche
  attendu : panneau replié, liste élargie, état mémorisé
  réponse : OK

[G-09] Pastilles de compteurs
  attendu : brouillons = total ; corbeille, archive, envoyés = rien ; autres = non lus
  réponse : OK

[G-10] Dossiers surveillés
  tester  : Réglages > Dossiers, cocher jusqu'à 5 dossiers
  attendu : compteurs rafraîchis toutes les 2 minutes
  réponse : OK mais pourquoi limiter a seulement 5 dossiers ???

[G-11] Abonnement IMAP
  attendu : dossiers non abonnés masqués, abonnement modifiable
  réponse : OK
```

## H. Filtres (Sieve) ⚠

```
[H-01] Page des filtres
  attendu : règles listées, script actif indiqué
  réponse :

[H-02] Créer une règle
  tester  : « si De contient X, déplacer vers Y »
  attendu : script valide, règle appliquée par le serveur
  réponse :

[H-03] Ordre des règles
  attendu : modifiable, ordre respecté
  réponse :

[H-04] Édition du script brut
  attendu : vérification de syntaxe avant enregistrement
  réponse :

[H-05] Serveur sans ManageSieve
  attendu : message clair, pas de plantage
  réponse :
```

## I. Réglages et apparence

```
[I-01] Thème DarkShine
  attendu : image de fond visible sur la barre du haut et le panneau des dossiers, ancrée en haut à gauche
  réponse : OK

[I-02] Thèmes clair et sombre
  attendu : lisibles, aucun texte invisible
  réponse : OK

[I-03] Fuseau horaire par utilisateur
  attendu : dates affichées dans le fuseau choisi, valeur par défaut depuis config.json
  réponse : OK

[I-04] Disposition 3 volets
  attendu : conforme à RainLoop, largeurs ajustables et mémorisées
  réponse : OK

[I-05] Disposition « aperçu en bas »
  attendu : liste en haut, lecture dessous
  réponse : OK

[I-06] Disposition « liste seule »
  attendu : le message s'ouvre sur une page entière
  réponse : OK

[I-07] Taille de police
  attendu : prise en compte partout
  réponse :

[I-08] Notifications bureau et son
  attendu : autorisation demandée, notification à l'arrivée d'un message, clic qui ouvre le message
  réponse :

[I-09] Barre « Retour » des réglages
  attendu : au-dessus de la carte blanche, sur le fond d'écran
  réponse :

[I-10] Image de fond personnalisée
  tester  : make theme-background FILE=chemin/image.jpg
  attendu : prise en compte après rechargement
  réponse :

[I-12] Compteur de non lus sur l'icône de l'onglet
  tester  : laisser des messages non lus, regarder l'onglet du navigateur ; décocher l'option dans Réglages > Général
  attendu : pastille rouge avec le nombre sur l'icône ; elle disparaît quand l'option est décochée
  réponse :

[I-13] Non lus des autres boîtes
  tester  : deux boîtes avec des non lus, ouvrir le menu du compte en haut à droite
  attendu : un compteur par boîte, rafraîchi au sondage (2 min)
  réponse : OK mais la zone d'affichage n'est pas asser large et la pastille peut se retrouver en dessous

[I-14] Réglages enregistrés à la volée
  tester  : changer la taille de police, la langue, une case à cocher dans Réglages > Général
  attendu : coche verte à côté du réglage, valeur conservée après rechargement, sans cliquer « Enregistrer »
  réponse :

[I-15] Vignettes de thèmes
  tester  : Réglages > Général, zone « Thème »
  attendu : une vignette par thème, celle du thème courant mise en évidence, changement immédiat
  réponse :

[I-11] Boutons des réglages
  attendu : relief léger, bord foncé, comme RainLoop
  réponse : OK
```

## I bis. Téléphone et mode sans JavaScript

```
[I-16] Courrier sur téléphone (≈420 px)
  tester  : réduire la fenêtre à 420 px, ouvrir un message, ouvrir le menu « Déplacer vers »
  attendu : volet de lecture pleine largeur, barre d'outils sur plusieurs lignes, menus dans l'écran
  réponse :

[I-17] Composition sur téléphone
  tester  : écrire un message à 420 px avec une identité au nom long, joindre un fichier
  attendu : aucun débordement horizontal, champs et boutons accessibles
  réponse :

[I-18] Navigateur sans JavaScript
  tester  : désactiver JavaScript, parcourir la liste, ouvrir un message, écrire et envoyer
  attendu : tout reste utilisable ; les commandes inopérantes (dépôt de fichiers) sont masquées
  réponse :
```

## J. Sécurité et robustesse

```
[J-01] Script dans un message
  attendu : aucun script exécuté, contenu affiché tel quel
  réponse : OK — couvert par TestSecurityMessageScriptsNeutralised (script, onerror, javascript:, iframe, object retirés ; corps en bac à sable)

[J-02] Pixel espion
  attendu : signalé, bloqué avec les images distantes
  réponse : OK — couvert par TestSecurityRemoteImagesAndTrackers (références distantes bloquées, pixel 1×1 retiré même images autorisées)

[J-03] CSRF
  tester  : soumettre un formulaire depuis un autre site
  attendu : refus explicite
  réponse : OK — couvert par TestSecurityCSRFRejected (jeton absent, jeton faux, Sec-Fetch-Site: cross-site → 403)

[J-04] Serveur sur une adresse privée
  attendu : refus, sauf si allowed_private_hosts l'autorise
  réponse : OK — couvert par TestSecurityPrivateNetworkBlocked (refus IMAP et SMTP, acceptation si allowed_private_hosts)

[J-05] HTTPS derrière votre proxy
  attendu : en-tête Strict-Transport-Security présent
  réponse :

[J-06] Cache des fichiers statiques
  attendu : 304 au rechargement (ETag)
  réponse :

[J-07] Coupure du serveur IMAP
  attendu : message d'erreur clair, reconnexion automatique ensuite
  réponse :

[J-08] Journal
  tester  : make logs pendant une action longue
  attendu : opérations IMAP et SMTP lisibles, avec durées
  réponse :

[J-09] Redémarrage du conteneur
  attendu : comptes et données chiffrées conservés
  réponse :
```

## K. Comparaison écran par écran avec RainLoop

Mettre les deux côte à côte et noter l'écart le plus visible.

```
[K-01] Écran de connexion
  réponse :

[K-02] Liste et volet de lecture
  réponse :

[K-03] Message avec pièces jointes
  réponse :

[K-04] Composition
  réponse :

[K-05] Contacts
  réponse :

[K-06] Réglages
  réponse :

[K-07] Filtres
  réponse :
```

---

## Journal des retours

Je remplis ce tableau à partir de vos réponses : un retour `KO` y reste jusqu'à correction et nouvelle validation.

| Identifiant | Date | Votre retour | Correction | État |
|---|---|---|---|---|
| A-12 | 24/10 | QR « again=1 » en 404 | le secret révélé reste affichable 2 min, le temps du chargement de l'image | **à revalider** |
| C-06 | 24/10 | `from: adresse` sans résultat | une espace après les deux-points faisait repartir toute la requête en texte libre ; le mot-clé consomme le jeton suivant (test Dovecot réel) | **à revalider** |
| C-08 | 24/10 | deux croix mal alignées | croix native du champ `type=search` masquée, la nôtre recentrée | **à revalider** |
| C-15 | 24/10 | « ça télécharge la page » | comportement de Chrome (Alt+clic télécharge) ; point reformulé, clic du milieu conservé | sans objet |
| C-16 | 24/10 | curseur clavier invisible | le fond d'état n'est plus écrasé ; liseré et barre latérale marqués | **à revalider** |
| D-09 | 24/10 | image précédente pendant le chargement | image masquée et `src` retiré, anneau d'attente après 150 ms | **à revalider** |
| D-15 | 24/10 | flèches inopérantes et grisées | `/mail/pane` rendait une liste vide ; les voisins sont désormais déduits de la liste affichée | **à revalider** |
| D-20 | 24/10 | bouton plein écran introuvable | icône distincte, infobulle explicite, raccourci Entrée, bouton rouge quand actif | **à revalider** |
| D-22 | 24/10 | PDF refusé dans la visionneuse | `X-Frame-Options: deny` global s'appliquait à l'aperçu ; `SAMEORIGIN` + `frame-ancestors 'self'` | **à revalider** |
| D-23 | 24/10 | cases des pièces jointes invisibles | révélation refaite à chaque injection du volet, pastille lisible sur la vignette | **à revalider** |
| E-06 | 24/10 | `=?utf-8?q?St…` dans les champs de réponse | `FormatList` réencodait les noms non ASCII ; mise en forme lisible | **à revalider** |
| G-10 | 24/10 | pourquoi 5 dossiers surveillés ? | relevé en une commande (LIST-STATUS), limite portée à 20 | **à revalider** |
| I-13 | 24/10 | pastille des comptes à la ligne | entrée de menu en flex, adresse tronquée par ellipsis | **à revalider** |
| J-01 à J-04 | 24/10 | « à toi de tester » | quatre tests automatiques ajoutés (message piégé, CSRF, SSRF, en-têtes) | validé |
| — | 25/10 | menu « plus d'actions » trop étroit | largeur suivant le contenu, bornée à l'écran | **à revalider** |
| — | 25/10 | panneau « détails » translucide illisible | sorti de la surcouche, placé dans le flux comme la bande des pièces jointes | **à revalider** |
| — | 25/10 | Suppr : le curseur ne passe pas au message suivant | la liste est rechargée, donc le curseur (classe `focused`, côté client) était perdu ; il est noté avant l'envoi et reposé au chargement suivant | validé |
| — | 25/10 | Suppr : ouvre le message d'après le suivant | `open_next` n'était pas mis à jour quand le volet se charge sans recharger la page ; le voisin rouvert était celui du dernier rendu complet | validé |
| — | 25/10 | correctifs invisibles sans rechargement forcé | `/static` était servi avec `max-age=600` ; les adresses portent désormais l'empreinte du contenu (`?v=…`), gardées un an, revalidées sans marque | validé |

## Points livrés depuis la dernière session de test

Ajoutés aux points ci-dessus, à valider en priorité : `C-12`, `C-13`, `C-14`, `C-15`, `C-30`, `C-31`, `D-10`, `D-15`, `D-16`,
`D-21`, `D-22`, `E-03`, `F-01`, `F-05`, `G-02`, `G-07`, `I-01`, `I-05`, `I-09`, `I-11`, `A-08`, `C-24`, `B-08`, `B-09`, `C-32`, `I-12`, `I-13`, `D-23`, `A-11`, `B-10`, `A-12`, `D-24`, `C-33`, `B-11`, `B-12`, `C-34`, `D-25`, `D-26`, `D-27`, `I-01`, `I-14`, `I-15`, `D-28`, `I-16`, `I-17`, `I-18`.
