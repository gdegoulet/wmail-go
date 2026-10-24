# Analyse fonction par fonction : RainLoop 1.17 `MailSo/Mime/` contre wmail-go

Périmètre : 12 fichiers de `/Work/webmail_project/rainloop/1.17.0/app/libraries/MailSo/Mime/` (Message, Part, PartCollection, Header, HeaderCollection, Email, EmailDep, EmailCollection, Attachment, AttachmentCollection, Parameter, ParameterCollection). Les 5 fermetures anonymes (`FilterList`/`ForeachList`) sont rattachées à leur fonction. Les `Enumerations/` n'ont pas été analysés (hors liste de fichiers fournie, ce sont des constantes).

Références Go principales lues : `internal/compose/build.go`, `address.go`, `multipart.go`, `mdn.go` ; `internal/mailview/parse.go`, `text.go` ; `internal/mailconn/messages.go` (`attachmentKind` l.340, `highPriority` l.879, `fixUTF8` l.372+) ; `internal/web/compose.go`, `mail.go`.

Constat général : go-message / net/mail / go-imap font déjà la mécanique MIME (parsing récursif, boundary, encodages QP/base64, RFC 2047/2231, repliement d'en-têtes). Ces fonctions sont classées SANS OBJET. Les écarts réels portent sur : (1) tolérance du parsing d'adresses, (2) priorité Basse/Normale et en-têtes X-Confirm-Reading-To / Sensitivity à l'envoi, (3) `format=flowed`, (4) DKIM `X-DKIM-Authentication-Results` et par destinataire, (5) IDN, (6) en-têtes personnalisés.

## Message.php (construction du message sortant)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (48), NewInstance (60) | Instancie le message sortant | Plomberie | `compose.Message` struct, build.go:~30 | SANS OBJET | Constructeur PHP. |
| DoesNotCreateEmptyTextPart (68) | Ne crée pas de partie texte vide (pour pièce jointe seule) | Message sans corps | build.go : partie text/plain toujours créée (même vide) | SANS OBJET | Pas de cas d'usage UI (corps vide = partie vide inoffensive). |
| DoesNotAddDefaultXMailer (78) | Désactive le X-Mailer par défaut | Option masquer X-Mailer | `cfg.Compose.HideXMailer` config.go:88, web/compose.go:544 | OK | — |
| MessageId (88), SetMessageId (103) | Lit/force le Message-ID | Identifiant unique, brouillon réutilisant l'ID | `Message.MessageID` build.go:~120 (« généré si vide ») | OK | — |
| RegenerateMessageId (113) / generateNewMessageId (528) | Génère `<md5@hostname>` | Message-ID unique | `NewMessageID` build.go:74 (domaine de l'expéditeur, aléa 96 bits) | OK | Meilleur que l'original (domaine expéditeur). |
| Attachments (121) | Collection des PJ à joindre | Ajout de PJ à l'envoi | `Message.Attachments` build.go:~45 | OK | — |
| GetSubject (129), GetFrom (138), GetTo (154) (unique), GetBcc (170), GetRcpt (186) | Accesseurs et destinataires d'enveloppe To+Cc+Bcc dédoublonnés | Envoi SMTP vers tous les destinataires sans doublon | `Message.Recipients()` build.go:58 (insensible à la casse) | OK | Dédoublonnage insensible à la casse, supérieur. |
| SetCustomHeader (217) | En-tête arbitraire (utilisé par plugins/identités) | Extensions | aucun | SANS OBJET | Réservé aux plugins ; aucun besoin mono-utilisateur. |
| SetSubject (233) | Objet | Objet | build.go:~115 `h.SetSubject(cleanLine(...))` | OK | Anti-injection CR/LF présent. |
| SetInReplyTo (245), SetReferences (257) | Chaînage des réponses | Conversations correctement chaînées | build.go:129 et 139 | OK | — |
| SetReadReceipt (270), SetReadConfirmation (283) | Demande d'accusé de lecture : pose `Disposition-Notification-To` ET `X-Confirm-Reading-To` | Demande de confirmation de lecture reconnue par Outlook/Thunderbird anciens | `Disposition-Notification-To` seul, build.go:151 | PARTIEL | Ajouter `h.SetAddressList("X-Confirm-Reading-To", ...)` à côté, 1 ligne. P3. |
| SetPriority (293) | X-Priority 1 (Highest) / 3 (Normal) / 5 (Lowest) | Choix de l'importance à l'envoi (Haute/Normale/Basse) | Booléen `Important` seul → Importance: High + X-Priority: 1, build.go:146-148, web/compose.go:71 | PARTIEL | RainLoop propose 3 niveaux (MessagePriority dans app.js). Remplacer la case « Important » par un select (basse/normale/haute) : `Priority int` dans `compose.Message`, `X-Priority: 5 (Lowest)` + `Importance: Low`. Affichage liste déjà géré pour Haute (`highPriority`). P3. |
| SetSensitivity (322) | En-tête Sensitivity (Personal/Private/Company-Confidential) | Marquer un message confidentiel | aucun (grep « sensitivity » : 0 résultat) | MANQUANT | Aucune interface RainLoop 1.17 n'utilise ce réglage (aucune occurrence dans app.js) : fonction morte côté UI. Non prioritaire. P3 (ignorer). |
| SetXMailer (351) | X-Mailer | Identification client | build.go:143 | OK | — |
| SetFrom (363), SetTo (375), SetDate (387), SetReplyTo (399), SetCc (411), SetBcc (423) | En-têtes d'adressage et Date | Composition standard | build.go:~108-122 (From, To, Cc, Bcc, Reply-To, Date) | OK | Bcc uniquement dans la copie « Envoyés » (build.go:116). |
| SetSender (435) | En-tête `Sender:` (envoi au nom d'une identité) | Envoyer « de la part de » | aucun | MANQUANT | Utilisé par RainLoop quand identité différente du compte ? À vérifier ; aucune zone UI. Ajouter `Sender` seulement si l'identité From n'appartient pas au compte SMTP (évite rejets DMARC/SPF alignement). P3. |
| SetDraftInfo (449) | Lien brouillon → message d'origine (reply/forward) pour poser $Answered/$Forwarded à l'envoi | Indicateurs « répondu/transféré » mis à jour | `X-Wmail-Draft-Info` : `EncodeDraftInfo/ParseDraftInfo` build.go:242/250 ; usage web/compose.go:372,569 | OK | En-tête privé distinct (X-Draft-Info) : incompatible avec brouillons créés dans RainLoop, sans importance. |
| AddPlain (465), AddHtml (476), AddText (489), AddAlternative (502) | Parties alternatives texte/HTML | Mail HTML + alternative texte | `buildHTML` multipart.go:63 (alternative text/plain + html, QP) | OK | — |
| generateNewBoundary (517) | Boundary aléatoire | Plomberie | `boundary()` multipart.go:18 | SANS OBJET | — |
| createNewMessageAttachmentBody (555) | Partie PJ : Content-Type name, Disposition inline/attachment filename, Content-ID, Content-Location, base64 (sauf message/rfc822) | PJ avec bon nom, images en ligne | multipart.go:~90 (name/filename, base64) ; images `Content-ID` + `inline` multipart.go:~75 ; build.go:~212 | PARTIEL | Pas de `Content-Location`, et pas de PJ « inline » hors HTML. message/rfc822 est encodé en base64 (RainLoop laisse brut) : valide. Écart sans conséquence visible. P3. |
| createNewMessageAlternativePartBody (638) | Parties alt avec Content-Transfer-Encoding choisi | Plomberie | multipart.go `writeQP` | SANS OBJET | — |
| createNewMessageSimpleOrAlternativeBody (717) | Choisit simple / alternative | Structure MIME correcte | build.go:~170 vs buildHTML | OK | — |
| createNewMessageRelatedBody (787) | multipart/related si images liées (CID) | Images collées dans l'éditeur | multipart.go:~70 | OK | — |
| createNewMessageMixedBody (827) | multipart/mixed si PJ | PJ | multipart.go:~104 et build.go:~200 | OK | — |
| setDefaultHeaders (866) | Date/Message-ID/X-Mailer/MIME-Version par défaut ; Bcc retiré si demandé | Envoi sans fuite Bcc | build.go:~104-145 (`includeBcc`) | OK | — |
| ToPart (913), ToStream (928), ToString (938) | Sérialisation | Plomberie | `Build()` build.go:100 | SANS OBJET | — |

## Part.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (75), NewInstance (84), Reset (92) | Instanciation/RAZ | Plomberie | — | SANS OBJET | — |
| Boundary (109), SetBoundary (137), HeaderBoundary (168) | Boundary du multipart | Plomberie | go-message mail.Reader | SANS OBJET | — |
| ParentCharset (117), SetParentCharset (126), HeaderCharset (158), SetParseBuffer (148) | Charset hérité/tampon | Plomberie | go-message/charset importé parse.go:17 | SANS OBJET | — |
| ContentType (178), ContentTransferEncoding (188), ContentID (198), ContentLocation (207) | Accès aux en-têtes MIME | Rendu, images cid | parse.go:~160-180 (`ContentType()`, `Content-Id`) | OK (ContentLocation : aucun) | Les images référencées par `Content-Location` (au lieu de `cid:`) ne sont pas résolues : rare. P3. |
| IsFlowedFormat (216) | Détecte `format=flowed` (hors base64/QP) pour reconstituer les paragraphes | Courriels Thunderbird/Apple Mail au texte reflowed affichés sans coupures artificielles | aucun (grep flowed/delsp : 0 résultat) ; `TextToHTML` mailview/text.go:15 traite chaque ligne comme dure | MANQUANT | Dans `mailview.Parse` lire `params["format"]=="flowed"` et `delsp` sur la partie text/plain ; dépli : une ligne finissant par espace est jointe à la suivante (avec respect de la profondeur de citation `>`), supprimer l'espace final si delsp=yes. Appliquer avant `TextToHTML`. Visible quotidiennement avec expéditeurs Thunderbird. P2. |
| FileName (237) | Nom de fichier : filename de Content-Disposition, sinon name de Content-Type | Nom de PJ | parse.go:~170-205 (`Filename()` puis `params["name"]`, nom par défaut) | OK | Nom par défaut avec extension (parse.go:214) : mieux. |
| ParseFromFile (261), ParseFromString (281), ParseFromStream (304), ParseFromStreamRecursion (366) | Analyse MIME récursive (boundary, 8 en-têtes, sous-parties) | Lecture de tout message | `mailview.Parse` parse.go:116 (go-message) | SANS OBJET | Plomberie. Note : Parse ne descend que dans les parties retournées par go-message, pas de message/rfc822 imbriqué affiché en ligne (RainLoop non plus). |
| Rewind (615), ToStream (630) | Relecture / sérialisation | Plomberie | — | SANS OBJET | — |

## PartCollection.php

| Fonction (ligne) | But | Besoin | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| __construct (23), NewInstance (31) | Collection | Plomberie | — | SANS OBJET | — |
| ToStream (41) | Concatène les sous-parties avec boundary | Plomberie | multipart.go `buildHTML` | SANS OBJET | — |

## Header.php

| Fonction (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| __construct (58), initInputData (72), NewInstance (112) | Création d'en-tête, détection paramétré | Plomberie | go-message `Header` | SANS OBJET | — |
| NewInstanceFromEncodedString (123) | Décode un en-tête RFC 2047 brut | Objet/noms accentués lisibles | `mr.Header.Subject()` parse.go:~140 | OK | — |
| Name (147), NameWithDelimitrom (155), Value (163), FullValue (171), Parameters (199), IsParameterized (267), IsSubject (259) | Accesseurs | Plomberie | — | SANS OBJET | — |
| SetParentCharset (180) | Charset par défaut du parent | Plomberie | — | SANS OBJET | — |
| wordWrapHelper (208), EncodedValue (218) | Repliement/encodage RFC 2047 à l'écriture | Objets/noms non-ASCII à l'envoi | go-message (`SetSubject`, `SetAddressList`) build.go:~113 | SANS OBJET | Géré par la bibliothèque. |
| IsEmail (278) | Liste des en-têtes d'adresse (From, To, Cc, Bcc, Reply-To, Return-Path, Sender) | Décodage de noms | `addresses()` parse.go:83 pour From/To/Cc/Reply-To/Bcc | PARTIEL | Pas de lecture de `Sender`/`Return-Path` : non affichés, sans importance. P3 (ignorer). |
| ValueWithCharsetAutoDetect (294) | Redétection du charset d'un en-tête 8 bits mal encodé | Objet/expéditeur non UTF-8 lisible | `fixUTF8` (relit en ISO-8859-1) mailconn/messages.go:~372 ; go-message/charset parse.go:17 | PARTIEL | Pas de détection de charset réelle (uniquement repli Latin-1) : suffisant pour usage FR. Côté lecteur (parse.go:~139 `Subject()`) vérifier que fixUTF8 est aussi appliqué (il l'est dans la liste). P3. |
| IsReparsed (315) | Indique re-parse | Plomberie | — | SANS OBJET | — |

## HeaderCollection.php

| Fonction (ligne) | But | Besoin | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| __construct (36), NewInstance (55), Clear (255), SetAsArray (220), GetByIndex (91), SetParentCharset (231) | Collection | Plomberie | — | SANS OBJET | — |
| AddByName (69), SetByName (83), RemoveByName (138) | Édition d'en-têtes | Plomberie | go-message `Header.Set/Add/Del` | SANS OBJET | — |
| ValueByName (103), ValuesByName (115), GetByName (196), ParametersByName (169), ParameterValue (186) | Accès par nom/paramètre | Plomberie | `mr.Header.Get/Values` parse.go:~126 | SANS OBJET | — |
| GetAsEmailCollection (153) | En-tête → liste d'adresses, tolérant | Afficher les destinataires même si adresse mal formée | `addresses()` parse.go:83 via `AddressList` (strict) | PARTIEL | Si UNE adresse d'un `To:` est invalide (ex. `Nom <a@b>, <>` ou point final), `AddressList` renvoie une erreur et `addresses()` retourne `nil` : tous les destinataires disparaissent. Écrire un repli : sur erreur, découper par virgule hors guillemets/chevrons (réutiliser le découpeur de `compose.ParseList` address.go:24) et tenter chaque morceau, en gardant les valides ; extraire sinon par regex `[^@\s]+@\S+`. P2. |
| Parse (269) | Analyse du bloc d'en-têtes (repliement, charset) | Plomberie | go-message | SANS OBJET | — |
| DkimStatuses (362) | Lit `Authentication-Results` (dkim=, header.d/i/from) ; à défaut `X-DKIM-Authentication-Results` (signer/status) | Pastille DKIM expéditeur authentifié | `dkimStatus` parse.go:251 (Authentication-Results uniquement) ; affichage base.html:246 et 264 | PARTIEL | (1) Pas de repli sur `X-DKIM-Authentication-Results: signer="x" status="pass"` (OVH/Hostinger notamment). (2) Ne retient « pass » que si le domaine signataire correspond à l'expéditeur (plus strict que RainLoop, c'est bon). Ajouter le repli : regex `status\s*=\s*"(\w+)"` et `signer\s*=\s*"([^";]+)"` dans `dkimStatus`. P3. |
| PopulateEmailColectionByDkim (441) | Applique le statut DKIM à chaque adresse (From/To/Cc) | Pastille par destinataire | Statut unique pour l'expéditeur seulement (Message.DKIM parse.go:69) | PARTIEL | RainLoop n'affiche l'état DKIM que sur l'expéditeur dans l'UI de lecture 1.17 ; équivalent suffisant. P3 (ignorer). |
| ToEncodedString (469) | Sérialise les en-têtes | Affichage/téléchargement des en-têtes bruts | `view=headers` web/mailtools.go:16 | OK | — |

## Email.php (version sans remarque) et EmailDep.php (version avec remarque, héritée)

| Fonction (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| Email::__construct (48) / EmailDep::__construct (55), NewInstance (72 / 81) | Adresse (nom, email) | Plomberie | `mailview.Address` parse.go:29 | SANS OBJET | — |
| Email::Parse (83) / EmailDep::Parse (92) | Analyseur tolérant d'une adresse : noms entre guillemets simples/doubles, commentaires `(..)`, nom sans guillemets, adresse nue, point final retiré, repli regex | Saisie libre dans À/Cc/Cci, collage depuis d'autres sources | `compose.ParseList` address.go:24 → `net/mail.ParseAddress` strict | PARTIEL | Écarts : (a) échec total de la saisie avec un message d'erreur si une entrée est invalide (RainLoop ignore silencieusement l'entrée fautive ou la garde comme nom) ; (b) pas de support des noms entre apostrophes `'Dupont' <a@b>`, ni de point final `a@b.fr.` ; (c) séparateur espace seul (« a@b.fr c@d.fr ») non géré. Ajouter un prétraitement : retirer les `.` finaux, remplacer `'Nom' <` par `"Nom" <`, et, sur échec de `ParseAddress`, retenter la regex `[^@\s]+@\S+`. L'erreur actuelle (ErrInvalidAddress) reste utile pour une vraie faute de frappe : à garder pour le cas sans `@`. P2 (usage quotidien de collages). |
| GetEmail (215/224), GetDomain (257/274), GetAccountName (247/264) | Partie email/domaine/compte | Choix d'identité selon domaine | `compose.Domain` address.go:110 | OK | Compte (partie locale) : pas de fonction dédiée, non nécessaire. |
| GetDisplayName (223/232), GetRemark (EmailDep 240) | Nom affiché / remarque | Affichage expéditeur | `Address.Name`, `Short()` parse.go:29-45 | OK | Remarque (commentaire) : abandonnée, sans importance. |
| GetDkimStatus (231/248), GetDkimValue (239/256), SetDkimStatusAndValue (266/283) | Statut DKIM par adresse | Pastille DKIM | `Message.DKIM` (global) parse.go:69 | PARTIEL | Voir `PopulateEmailColectionByDkim`. P3. |
| ToArray (278/295) | Sérialisation JSON de l'adresse vers le JS (nom, mail, dkim) | UI | Gabarits Go | SANS OBJET | — |
| ToString (291/307) | « "Nom" <a@b> » ; option encodage RFC 2047 du nom ; option IDN | Construction d'en-têtes sûrs, domaine Unicode → punycode | `FormatList` address.go:81 ; nom encodé par go-message ; IDN : aucun (grep idn/punycode : 0) | PARTIEL | IDN : une adresse `x@café.fr` est envoyée en UTF-8 brut ; sans SMTPUTF8 le serveur la refuse. Ajouter `golang.org/x/net/idna.ToASCII` sur le domaine de chaque destinataire d'enveloppe (SMTP RCPT TO + en-têtes). P3 (peu de domaines accentués en pratique). |

## EmailCollection.php

| Fonction (ligne) | But | Besoin | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| __construct (25), NewInstance (41), Parse (51) | Analyse d'une liste | Saisie d'une liste d'adresses | `compose.ParseList` address.go:24 | PARTIEL | Voir `Email::Parse` : découpeur Go correct (guillemets, chevrons, `,` `;` retour ligne, address.go:34-62) mais pas de gestion des commentaires `( )` ni des apostrophes ; échec global. P2 (fusionné avec Email::Parse). |
| ToArray (59) | Vers JSON | UI | — | SANS OBJET | — |
| MergeWithOtherCollection (76) | Fusion | Réponse à tous, transfert | `append` + `Dedup` web/compose.go:562 | OK | — |
| Unique (90) | Dédoublonne (sensible à la casse) | Pas de doublons | `compose.Dedup` address.go:92 (insensible à la casse, avec exclusion de son adresse) | OK | Supérieur. |
| ToString (117) | Liste en chaîne | En-tête | `FormatList` address.go:81 | OK | — |
| parseEmailAddresses (134) | Découpeur tolérant (guillemets simples/doubles, parenthèses, `,` et `;`) | Voir Parse | address.go:24-62 | PARTIEL | Pas de `'` comme guillemet ni de `(` `)` ; `ParseList` ignore silencieusement les entrées vides, comme RainLoop. Inclus dans la proposition Email::Parse. P2. |

## Attachment.php / AttachmentCollection.php

| Fonction (ligne) | But | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| Attachment::__construct (63), NewInstance (88), Resource (97), ContentType (105), CustomContentTypeParams (113), CID (121), ContentLocation (129), FileName (137), FileSize (145) | Objet PJ sortante (flux, nom, taille, CID, location) | Envoi de PJ | `compose.Attachment` build.go:~22 (Name, ContentType, Data) ; images `compose.InlineImage` (CID) | OK | CustomContentTypeParams et ContentLocation : non nécessaires (voir createNewMessageAttachmentBody). |
| IsInline (153), IsLinked (193) | PJ intégrée / liée à un CID | Images dans le corps | `Message.Images` build.go / multipart.go:~75 | OK | — |
| IsImage (161), IsArchive (169), IsPdf (177), IsDoc (185) | Type de la PJ pour icône/aperçu | Icône/aperçu par genre, vignettes | `attachmentKind` mailconn/messages.go:340 (image, audio, video, pdf, archive, tableur, presentation, texte) ; `Attachment.IsImage` mailview/thumb.go:51 ; aperçu PDF web/mail.go:1065-1125 | OK | Plus fin que RainLoop. |
| AttachmentCollection::__construct (23), NewInstance (31) | Collection | Plomberie | — | SANS OBJET | — |
| LinkedAttachments (39), UnlinkedAttachments (49) | Filtre inline vs jointes | Structure related/mixed | `buildHTML` séparation Images / Attachments multipart.go:63 | OK | — |
| SizeOfAttachments (59) | Somme des tailles | Limite de taille d'envoi | `attachstore.go` (limites) | PARTIEL | Je n'ai pas relu attachstore.go ; contrôle de taille totale côté Go non vérifié ici. Vérifier `grep -n "max" internal/web/attachstore.go`. P3. |

## Parameter.php / ParameterCollection.php

| Fonction (ligne) | But | Besoin | Équivalent Go | Verdict | Proposition |
|---|---|---|---|---|---|
| Parameter::__construct (36), NewInstance (48), CreateFromParameterLine (59), Reset (68), Name (79), Value (87), Parse (98), ToString (118) | Paramètre d'en-tête `clé=valeur` (RFC 2231, guillemets, encodage) | noms de fichiers accentués, boundary, charset | `mime.ParseMediaType`/`FormatMediaType` (build.go:~245, multipart.go:~104) et go-message | SANS OBJET | Plomberie assurée par la bibliothèque standard. |
| ParameterCollection::__construct (25), NewInstance (40), GetByIndex (48), SetAsArray (62), ParameterValueByName (74), Parse (97), ToString (118), reParseParameters (137) | Collection de paramètres | Plomberie | idem | SANS OBJET | — |

---

## À AJOUTER (MANQUANT / PARTIEL), par priorité

**P2**
1. Parsing d'adresses tolérant à la saisie (`compose.ParseList`) : apostrophes comme guillemets de nom, point final retiré, commentaires `( )`, repli regex `[^@\s]+@\S+` avant d'échouer. (Email::Parse, EmailCollection::parseEmailAddresses)
2. Lecture d'en-têtes d'adresse tolérante (`mailview.addresses`) : sur erreur de `AddressList`, découper et conserver les adresses valides au lieu de tout perdre. (HeaderCollection::GetAsEmailCollection)
3. `format=flowed` / `delsp` : dépliage des paragraphes avant `TextToHTML` (mailview/text.go, parse.go). (Part::IsFlowedFormat)

**P3**
4. Priorité Basse/Normale/Haute à la rédaction (select à la place de la case « Important »), `X-Priority: 5 (Lowest)` + `Importance: Low`. (Message::SetPriority)
5. `X-Confirm-Reading-To` en plus de `Disposition-Notification-To`. (Message::SetReadReceipt)
6. DKIM : repli `X-DKIM-Authentication-Results` (signer/status). (HeaderCollection::DkimStatuses)
7. IDN : conversion punycode du domaine des destinataires à l'envoi (`x/net/idna`). (Email::ToString / GetEmail($bIdn))
8. En-tête `Sender:` quand l'identité d'expédition diffère du compte SMTP. (Message::SetSender)
9. Résolution des images via `Content-Location` en plus de `cid:`. (Part::ContentLocation / Attachment::ContentLocation)
10. Détection de charset des en-têtes 8 bits au-delà du repli ISO-8859-1. (Header::ValueWithCharsetAutoDetect)
11. Vérifier la limite de taille totale des PJ à l'envoi. (AttachmentCollection::SizeOfAttachments)
12. Sensitivity : sans interface dans RainLoop 1.17, à ignorer. (Message::SetSensitivity)

Non traité : fichiers `Mime/Enumerations/` (constantes). Aucun arrêt anticipé : toutes les fonctions des 12 fichiers sont couvertes.
