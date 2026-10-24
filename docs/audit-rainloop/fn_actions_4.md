# Analyse RainLoop 1.17 Actions.php (lignes 7700 à fin, 10125) face à wmail-go

Périmètre : 59 fonctions/méthodes, toutes traitées jusqu'à la fin du fichier. Chemins Go relatifs à `/home/gdegoulet/Work/webmail_project/internal/`.
Cette fin d'Actions.php est surtout de la plomberie de l'API JSON (réponses, paramètres, cache, capacités). Ce qui compte pour l'utilisateur : image de fond, import/export de contacts, pièces jointes (aperçu, miniatures, téléchargement, Range), ajout d'un .eml, thèmes/langues, et `responseObject` (transformation d'un message en réponse).

## Actions.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| DoClearUserBackground (7715) | Supprime l'image de fond perso (blob + réglages) | Revenir au thème sans fond | `web/background.go:67` backgroundDelete, route `web/server.go:170` | OK | - |
| UploadBackground (7743) | Reçoit un PNG/JPEG, le stocke, génère un hash de cache | Personnaliser le fond d'écran | `web/background.go:26` backgroundSave (JPEG/PNG vérifiés sur les octets, 2 Mo, 6000 px, chiffré) ; compteur `Background++` comme version de cache (`web/handlers.go:526` `?v=`) | OK | Plus strict que l'original (type vérifié sur le contenu) |
| UploadContacts (7843) | Reçoit un fichier, détecte VCF ou CSV, importe | Importer son carnet d'un autre outil | `web/contactsweb.go:266` contactsImport (VCF/CSV, 8 Mo, dédoublonnage par e-mail, compteurs ajoutés/ignorés `:305-311`) | OK | Détection un peu différente (extension .csv, ou absence de BEGIN:VCARD) : équivalent |
| importContactsFromCsvFile (7915) | Lit un CSV (séparateur `,` ou `;`, en-têtes), importe | Importer un export Google/Outlook/Thunderbird | `contacts/csv.go:102` detectSep, `:113` ParseCSV, `:28` headerKind, `:91` decodeText | OK | - |
| importContactsFromVcfFile (7974) | Importe un .vcf | Importer des vCards | `contacts/vcard.go:16` ParseVCards | OK | - |
| RawViewAsPlain (8004) | Envoie une partie MIME précise en text/plain | « Voir la source » d'une partie (rare) | `web/mailtools.go:17` mailSource : source/en-têtes/eml du message entier seulement | PARTIEL | Ajouter `view=part&index=N` à mailSource (text/plain, nosniff, CSP sandbox). P3 |
| RawFramedView (8028) | Affiche DOC/XLS/PPT dans le visualiseur Google Docs | Aperçu des documents Office | aucun | SANS OBJET | Envoie la pièce à un tiers (Google) : contraire à la confidentialité visée |
| Append (8075) | Ajoute un fichier .eml dans un dossier IMAP | Glisser-déposer d'un .eml sur la liste | `web/mailappend.go:19` mailAppend (route `web/server.go:147`), JS `web/static/emldrop.js`, contrôle RFC 822 `:51`, CRLF `:68`, taille bornée `:30` | OK | - |
| Capa (8117) | Liste des capacités selon la config admin et le mobile | Activer/désactiver des blocs d'interface | Aucun mécanisme dynamique ; tout est actif : comptes `web/server.go:131`, identités `:184`, thèmes `:116`, fond `:169`, filtres `:178`, 2FA `:172`, contacts `:188`, miniatures `web/mail.go:934` | SANS OBJET | Réglages d'administration multi-utilisateurs ; un seul utilisateur |
| GetCapa (8270) | Test d'appartenance à Capa | Garde d'accès aux actions | aucun | SANS OBJET | Idem Capa |
| etag (8280) | ETag à partir de la clé et de l'index de cache | Cache HTTP | aucun | SANS OBJET | Infra PHP |
| cacheByKey (8291) | En-têtes de cache (durée config) ou no-cache | Rapidité d'affichage des pièces, images, fond | Fond : `private, max-age=86400` + `?v=` (`web/background.go:104`, `web/handlers.go:526`). Pièces : `private, no-store` (`web/mail.go:1139`) | PARTIEL | Choix volontaire pour les pièces (sécurité). P3 : `max-age` court pour les aperçus d'images (`inline=1`), l'UID étant immuable |
| verifyCacheByKey (8318) | Répond 304 si If-None-Match concorde | Évite de renvoyer le même contenu | aucun (`http.ServeContent` avec `time.Time{}` : `web/mail.go:1141`) | SANS OBJET | Même raison (no-store voulu) |
| getMimeFileByHash (8338) | Copie une partie MIME dans un temporaire pour la joindre ailleurs | Garder les pièces au transfert | `web/compose.go:309-310` (pièces du message d'origine reprises au transfert), `web/attachstore.go` | OK | - |
| rotateImageByOrientation (8389) | Retourne/pivote selon l'EXIF | Photos à l'endroit | aucun (grep `exif`/`orientation` dans les .go : aucun résultat) | MANQUANT | Voir correctImageOrientation. P3 |
| correctImageOrientation (8435) | Lit l'orientation EXIF, corrige, miniature 60x60 recadrée | Miniatures correctes | Miniature recadrée `mailview/thumb.go:23` ; orientation EXIF : aucun | PARTIEL | Dans `mailview/thumb.go` : lire l'orientation JPEG (APP1) et pivoter avant recadrage. L'aperçu plein format est déjà corrigé par les navigateurs. P3 |
| rawSmart (8468) | Cœur des téléchargements/affichages : Range, Content-Type, nom nettoyé, inline/attachment, miniature, EXIF, cache | Télécharger, voir, miniaturiser une pièce ou le .eml | `web/mail.go:1080` mailAttachment : type forcé en octet-stream si dangereux (`:1110`), nom sûr (`:1113`), aperçu inline contrôlé sur le contenu (`:1116`), Range via ServeContent (`:1141`), PDF (`:1127`), CSP sandbox (`:1138`) ; .eml `web/mailtools.go:45` ; zip `web/server.go:143` | OK | Plus sûr que l'original ; seul écart : EXIF |
| RawDownload (8695) | Téléchargement en pièce jointe | Bouton « Télécharger » | `web/mail.go:1115,1137` (disposition attachment par défaut) | OK | - |
| RawView (8703) | Affichage en ligne | Aperçu image/PDF | `web/mail.go:1116-1122`, `web/static/preview.js` | OK | - |
| RawViewThumbnail (8711) | Miniature d'une pièce image | Vignettes des pièces | `web/mail.go:934-936` + `mailview/thumb.go:23` (data:, max 12 `web/mail.go:34`) ; affichage `web/templates/base.html:301` | OK | - |
| RawUserBackground (8719) | Sert le fond de l'utilisateur | Fond perso | `web/background.go:92` backgroundGet, route `web/server.go:171` | OK | - |
| RawPublic (8755) | Sert une image publique stockée hors session | Logos/avatars de domaines | aucun | SANS OBJET | Fonction d'administration multi-domaines |
| isFileHasFramedPreview (8796) | Extensions Office pour la visionneuse Google | Idem RawFramedView | aucun | SANS OBJET | Voir RawFramedView |
| isFileHasThumbnail (8807) | Vérifie que GD sait traiter png/gif/jpg | Décider d'afficher une vignette | `mailview/thumb.go:51` IsImage, `web/mail.go:934` | OK | - |
| RawAvatar (8844) | Image de l'expéditeur : Gravatar, sinon icône par domaine, sinon vide | Photo de l'expéditeur | `web/templates/base.html:262` silhouette fixe ; décision laissée ouverte `CAHIER_DES_CHARGES.md:595` | MANQUANT | Photo issue du carnet (PHOTO de la vCard) si le contact existe ; Gravatar seulement en option par compte via le garde SSRF (`mailconn/guard.go`), car il fuit l'adresse. P3 |
| RawContactsVcf (8902) | Export vCard | Sauvegarde / migration | `web/contactsweb.go:317` contactsExport (vcf par défaut, `contacts.vcf` `:346`) | OK | - |
| RawContactsCsv (8920) | Export CSV | Idem | `web/contactsweb.go:340-343` + `contacts/csv.go:251` EncodeCSV | OK | - |
| initMailClientConnection (8938) | Ouvre la connexion IMAP, gère les erreurs connexion/auth | Accès au courrier | `mailconn/pool.go` (`s.pool.Do` ex. `web/mailappend.go:57`), `mailconn/errors.go`, `web/mail.go:440` mailErr | OK | - |
| getDecodedRawKeyValue (8970) | Décode la clé opaque d'un lien (dossier, uid, index, type, nom) | Liens de pièces | Paramètres explicites folder/uid/index : `web/mail.go:1086-1091` | OK | Plus simple |
| getDecodedClientRawKeyValue (8990) | Décode une clé client séparée par NUL | Usage interne JS | aucun | SANS OBJET | Protocole JS de RainLoop |
| StaticCache (9010) | Hash version+plugins pour les assets | Cache des assets | `web/server.go:115` fichiers embarqués | SANS OBJET | Plugins/PHP |
| ThemeLink (9025) | URL de la feuille de style du thème | Charger le thème | `web/server.go:116` `GET /themes/{name}/{file}`, `:422` themeFile | OK | - |
| ValidateTheme (9035) | Thème valide sinon défaut ; « Mobile » pour mobile | Ne jamais casser l'affichage | `web/server.go:382` themeExists, `web/handlers.go:591-598` theme() (repli), `config/config.go:122` default_theme | OK | Pas de thème Mobile dédié (mise en page responsive) |
| normalizeLanguage (9052) | Normalise les codes de langue | Choix de langue | `i18n/i18n.go:75` Match | OK | Seulement fr/en |
| ValidateLanguage (9085) | Valide la langue, repli sur défaut | Idem | `i18n/i18n.go:58` Has, `web/server.go:578` setLang, `userdata/userdata.go:45` Language | OK | - |
| ValidateContactPdoType (9120) | Valide mysql/pgsql/sqlite du carnet | Stockage des contacts | aucun | SANS OBJET | Contacts dans le coffre chiffré (`internal/contacts`, `internal/store`) |
| GetThemes (9130) | Liste des thèmes (intégrés + personnalisés) | Sélecteur de thème | `web/server.go:394` themeNames (embarqués + `data_dir/themes`) ; `static/themes/{dark,light,rainloop}` | PARTIEL | « DarkShine » n'existe pas comme tel (grep `darkshine` : aucun hors RainLoop). À comparer visuellement avec `dark`/`rainloop`. P2 |
| GetLanguages (9219) | Liste des langues disponibles | Sélecteur de langue | `i18n/i18n.go:48` Languages (en, fr) | PARTIEL | Deux langues : suffisant pour l'utilisateur. P3, aucune action requise |
| ProcessTemplate (9266) | Nettoie les gabarits HTML (plugins, scripts) | Sécurité des gabarits | `html/template` (échappement automatique) | SANS OBJET | Moteur de gabarits PHP |
| mainDefaultResponse (9284) | Enveloppe `Action`/`Result` des réponses JSON | API AJAX | Pages rendues côté serveur + petits JSON (`web/mailappend.go:21`, `web/poll.go:45`) | SANS OBJET | Architecture différente |
| DefaultResponse (9311) | Réponse standard + crochets plugins | Idem | idem | SANS OBJET | Plugins |
| TrueResponse (9325) | Réponse « vrai » | Idem | idem | SANS OBJET | - |
| FalseResponse (9342) | Réponse d'erreur avec code/messages | Afficher l'erreur | `web/server.go:664` message(), clés `mail.err.*` (`web/mail.go:1105`) | OK | - |
| ExceptionResponse (9370) | Exception → code d'erreur utilisateur + journal | Messages d'erreur clairs | `web/mail.go:440` mailErr, journal `web/mailappend.go:62`, `mailconn/errors.go` | OK | - |
| SetActionParams (9417) | Mémorise les paramètres de l'action | Plomberie | `r.URL.Query()` / `r.FormValue` | SANS OBJET | - |
| GetActionParam (9432) | Lit un paramètre | Plomberie | idem | SANS OBJET | - |
| GetActionParams (9441) | Lit tous les paramètres | Plomberie | idem | SANS OBJET | - |
| HasActionParam (9451) | Présence d'un paramètre | Plomberie | idem | SANS OBJET | - |
| HasOneOfActionParams (9461) | Présence d'au moins un paramètre | Plomberie | idem | SANS OBJET | - |
| Location (9477) | Redirection HTTP + journal | Redirections après action | `http.Redirect(... StatusSeeOther)` (`web/background.go:67`) | OK | - |
| objectData (9490) | Ajoute `@Object`/`@Count`/`@Collection` aux objets sérialisés | Protocole JSON | aucun | SANS OBJET | Protocole interne |
| hashFolderFullName (9520) | MD5 du nom de dossier sauf dossiers système (ids HTML/JS) | Identifiants de dossiers | Noms passés en paramètres d'URL (`web/server.go:139`) | SANS OBJET | - |
| GetLanguageAndTheme (9533) | Langue (réglage utilisateur) et thème (réglage local) | Appliquer ses préférences | `web/handlers.go:591` theme(sess), `userdata/userdata.go:45`, cookie langue `web/server.go:581` | OK | - |
| StaticI18N (9574) | Traduit une clé côté serveur | Textes côté serveur | `i18n/i18n.go:61` T | OK | - |
| StaticPath (9606) | URL d'un fichier statique avec paramètre anti-cache | Chargement des assets | `web/server.go:115` `/static/` | OK | - |
| cacherForUids (9616) | Cache serveur des UID de recherche (option) | Rapidité des grandes listes | aucun (recherche IMAP à chaque requête : `mailconn/search.go`) | SANS OBJET | Optimisation facultative, désactivée par défaut |
| cacherForThreads (9626) | Cache des fils de discussion (option) | Rapidité des conversations | `mailconn/messages.go:435-436` THREAD IMAP direct ; `web/templates/home.html:93` | SANS OBJET | Idem |
| explodeSubject (9637) | Sépare les préfixes « Re:/Fwd: » du sujet (grisés) | Lecture de la liste | `web/mail.go:1430` splitSubject, `:605`, champs `:47`, rendu `web/templates/home.html:93` | OK | - |
| responseObject (9687) | Transforme les objets en JSON : message, e-mail, contact, dossier, collections | Tout l'affichage d'un message et des listes | Détail ci-dessous | PARTIEL | Voir sous-points |

### Détail de responseObject (9687)

| Sous-fonction | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|
| Priorité haute (en-tête) | Liste : `mailconn/messages.go:398,454,880`, marqueur `web/templates/home.html:93`. Absent de la vue détaillée | PARTIEL | Afficher la priorité dans l'en-tête détaillé (`web/templates/base.html:246`). P3 |
| Sensibilité (Private/Confidential) | aucun (grep Sensitivity : aucun) | MANQUANT | Ligne optionnelle dans la vue détaillée. P3 |
| Liens de désabonnement | `web/templates/base.html:288` | OK | - |
| Drapeaux lu/étoilé/répondu/transféré | `mailconn/messages.go:84-87,406-409`, `web/mail.go:603` | OK | - |
| `$Forwarded` posé à l'envoi | `web/compose.go:32` | OK | - |
| Confirmation de lecture (demande, `$MDNSent`) | `web/compose.go:33,999-1027`, `web/mail.go:888-890`, `web/templates/base.html:289`, route `web/server.go:150` | OK | - |
| Assainissement HTML (ClearHtml), citations repliées | `mailview/sanitize.go:70` (liste blanche), `:240` CollapseQuotes | OK | - |
| Images externes bloquées, expéditeur de confiance | `web/remoteimages.go:27` blockRemote, `web/server.go:149` trustSender | OK | - |
| Détection des pixels espions 1x1 | `mailview/sanitize.go:225-232` | OK | - |
| Proxy local pour images externes | `web/server.go:148`, `web/remoteimages.go:66` mailImage (HMAC, 5 Mo `:21`, anti-SSRF) | OK | - |
| DKIM de l'expéditeur | `mailview/parse.go:69,138`, `web/templates/base.html:246,264` | OK | - |
| Pièces jointes : nom nettoyé, aperçu, miniature | `web/mail.go:931-940`, `:1113` | OK | - |
| Brouillon (DraftInfo), In-Reply-To | `web/compose.go:568,694` | OK | - |
| Pliage `<pre>`, images cid: internes | non vérifié en détail (`mailview/parse.go`, `compose/html.go:39`) | OK présumé | Contrôle visuel |
| TextPartIsTrimmed | aucun | SANS OBJET | Go lit le message entier |
| PGP signé/chiffré | aucun | SANS OBJET | OpenPGP hors périmètre |
| Contacts/étiquettes/propriétés | `contacts/contacts.go`, `web/contactsweb.go:211` | OK | - |
| Dossiers : abonné/masqué | `mailconn/client.go:315-317,396`, `web/foldertree.go:51` | OK | - |
| Dossier `Checkable` (dossiers surveillés) | aucun : `web/poll.go:44-58` n'interroge que INBOX (une map `watched` existe `:57`, à vérifier avant d'étendre) | PARTIEL | Choix de dossiers surveillés dans `/settings/folders`, liste dans `userdata` Settings, étendre le sondage. P2 |
| Collections de messages/dossiers (compteurs, fils, namespace) | `mailconn/messages.go:103-117`, `web/foldertree.go` | OK | - |
| Limite de 100 destinataires par collection | aucun | SANS OBJET | Protection de la sortie JSON |

## À AJOUTER (MANQUANT / PARTIEL), par priorité

1. **P2 - Dossiers surveillés (« Checkable »)** : plusieurs dossiers dont les non lus sont relevés et notifiés (étendre `web/poll.go`, réglage dans `/settings/folders`).
2. **P2 - Thème DarkShine** : confirmer que `static/themes/dark` et `rainloop` reproduisent le thème de l'utilisateur (aucune référence « DarkShine » dans le code).
3. **P3 - Orientation EXIF des miniatures** (`mailview/thumb.go:23`).
4. **P3 - Avatar de l'expéditeur** (`web/templates/base.html:262`) : photo du carnet ; Gravatar en option seulement.
5. **P3 - Vue source d'une partie MIME** (`web/mailtools.go:17`, `view=part&index=N`).
6. **P3 - Cache navigateur court des aperçus d'images** (`web/mail.go:1139`).
7. **P3 - Priorité et sensibilité dans l'en-tête détaillé** (`web/templates/base.html:246`).
8. **P3 - Langues** : seulement fr/en, aucune action requise.

Non reproduits volontairement : visionneuse Google Docs (RawFramedView, isFileHasFramedPreview), capacités d'administration (Capa/GetCapa), plugins, protocole JSON interne, bases PDO, PGP.
