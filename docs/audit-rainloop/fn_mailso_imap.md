# MailSo/Imap et MailSo/Net (RainLoop 1.17) : analyse fonction par fonction

Périmètre : `MailSo/Imap/*` (ImapClient, BodyStructure, FetchResponse, Folder, FolderInformation, NamespaceResult, Response, Enumerations, Exceptions) et `MailSo/Net/*` (NetClient, Enumerations, Exceptions).
Côté Go : `internal/mailconn` (client.go, messages.go, poll.go, pool.go, guard.go, errors.go, search.go) s'appuie sur go-imap v2, qui fournit tout l'analyseur de protocole (tags, littéraux, listes parenthésées, FETCH/BODYSTRUCTURE/ENVELOPE, SORT/THREAD/ESEARCH...). Ces fonctions de plomberie sont donc marquées SANS OBJET. Lecture/analyse du message : `internal/mailview/parse.go` (go-message), le message complet est téléchargé (`FetchMessage`, messages.go:584) puis analysé en Go, au lieu d'un FETCH par partie.
Légende : OK / PARTIEL / MANQUANT / SANS OBJET. Priorités P1 (quotidien) / P2 / P3.

## 1. Imap/ImapClient.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (93) | Init état (tag, capacités, dossier courant, callbacks) | Plomberie | go-imap `imapclient.Client` | SANS OBJET | Géré par la bibliothèque |
| NewInstance (121) | Fabrique | Plomberie | — | SANS OBJET | — |
| GetLogginedUser (129) | Retourne le login IMAP utilisé | Afficher/utiliser l'identifiant de connexion | `Endpoint.Login` (client.go:~42), clé de pool | OK | — |
| Connect (148) | Connexion TCP/SSL, lecture du greeting, STARTTLS si annoncé, mode AUTO_DETECT par port (993/995/465 => SSL), vérif. SSL, certificat client | Choix de sécurité du compte ; STARTTLS obligatoire ou implicite | `dialIMAP` client.go:97-136 : `SecurityTLS`/`SecuritySTARTTLS`/`SecurityNone`, `WaitGreeting`, TLS 1.2 mini, option `VerifyCert`, `LegacyTLS` (client.go:47-66) ; refus de repli en clair (client.go:23) | PARTIEL | Différences : pas de mode « auto-détection » selon le port (RainLoop déduit SSL si port 993) ; pas de certificat client (`local_cert`). Auto-détection : défaut de sécurité d'après le port dans le formulaire de compte (accounts.go, valeurs par défaut) : P3. Certificat client mTLS : inutile pour un seul utilisateur (P3, probablement à ne pas faire) |
| _xor (177) | Utilitaire XOR (resté de CRAM) | Plomberie | — | SANS OBJET | Mort/interne |
| Login (200) | Authentification : CRAM-MD5 si dispo, sinon PLAIN (AUTHENTICATE), sinon LOGIN ; PROXYAUTH ; mots de passe masqués dans le journal ; erreur ⇒ LoginBadCredentials | Se connecter à des serveurs n'acceptant que PLAIN/CRAM ; ne jamais journaliser le mot de passe | `c.Login()` client.go:142-155 (commande LOGIN uniquement) ; masquage : les traces `trace()` messages.go:40 ne consignent que op/dossier, jamais d'identifiants ; erreurs classées errors.go:48 (`Classify`) | PARTIEL | Go n'utilise que LOGIN. Un serveur annonçant `LOGINDISABLED` ou n'offrant que AUTH=PLAIN/SCRAM échouerait (cas rare avec STARTTLS/TLS obligatoire). Proposition : dans `dialIMAP`/`LoginIMAP`, si `caps.Has(imap.CapLoginDisabled)` ou `AUTH=PLAIN` seul, utiliser `c.Authenticate(sasl.NewPlainClient(...))` (la lib sasl est déjà importée pour SMTP, client.go:262). PROXYAUTH : SANS OBJET (administration). Priorité P3 |
| LoginWithXOauth2 (334) | Auth XOAUTH2 (jeton Gmail/Outlook) | Connexion Gmail via OAuth | aucun (grep `XOAUTH`/`oauth` dans internal : rien) | MANQUANT | Hors périmètre du cahier des charges pour un serveur auto-hébergé (mot de passe applicatif suffit) ; si besoin, `imapclient.Authenticate` + `sasl.NewOAuthBearerClient`. P3 (probablement à écarter) |
| Logout (390) | LOGOUT propre | Fermeture propre de session | `closeIMAP` client.go:183-186 (`Logout` puis `Close`) ; pool.go:114-132 ferme les connexions | OK | — |
| ForceCloseConnection (404) | Coupe la socket | Annuler une opération bloquée | `Pool.Close`/`closedClient` pool.go:38,114 ; contexte qui ferme la connexion `context.AfterFunc` client.go:80 | OK | — |
| IsLoggined (414) | État connecté+authentifié | Plomberie | `closedClient` pool.go:38 (détecte une connexion morte avant réutilisation) | OK | — |
| IsSelected (422) | Un dossier est-il sélectionné | Plomberie | go-imap suit l'état (`c.Mailbox()`) ; `Select` est rappelé à chaque opération messages.go:121 | OK | — |
| Capability (433) | Commande CAPABILITY, mémorisation | Adapter les fonctions aux extensions du serveur | `c.Caps()` (go-imap, capacités du greeting/login) utilisé partout : messages.go:186,498,830 ; client.go:177 | OK | — |
| IsSupported (446) | Test d'une extension | idem | `Caps().Has(...)` messages.go:186 (SORT), 498 (THREAD), 830 (QUOTA), 624s (MOVE), 659s (UIDPLUS), client.go:399 (LIST-EXTENDED) | OK | — |
| GetNamespace (464) | NAMESPACE : préfixe et séparateur du namespace personnel | Créer sous-dossiers avec le bon préfixe (ex. `INBOX.`) ; masquer le préfixe | aucun appel NAMESPACE (grep `Namespace` : rien). Équivalent partiel : séparateur lu dans LIST (`Folder.Delim`, client.go:~315), `FolderDelim()` accounts.go:65, retrait du préfixe INBOX foldertree.go:30 | PARTIEL | Création d'un dossier à la racine : sur un serveur dont le namespace personnel est `INBOX.` (Courier, Cyrus avec `altnamespace: no`), un `CREATE Travail` est refusé ; RainLoop ajoute le préfixe. Proposition : dans `Conn`, appeler `c.Client.Namespace()` (si cap NAMESPACE) et préfixer les noms de dossiers racine à la création (folders.go, création/renommage). P3 (Dovecot, serveur cible, n'en a pas besoin) |
| Noop (504) | NOOP | Maintien de connexion / rafraîchir | Sondage par STATUS poll.go:19 ; expiration des connexions du pool pool.go:135 | OK | Pas d'IDLE/NOOP nécessaire car sondage navigateur |
| FolderCreate (518) | CREATE | Créer dossier | `Conn.CreateFolder` messages.go:766 (+ abonnement auto, validation du nom messages.go:694) | OK | — |
| FolderDelete (533) | DELETE | Supprimer dossier | `Conn.DeleteFolder` messages.go:788 (INBOX protégée) | OK | — |
| FolderSubscribe (548) | SUBSCRIBE | Afficher/masquer dossiers | `Conn.SetSubscribed` messages.go:798 | OK | — |
| FolderUnSubscribe (563) | UNSUBSCRIBE | idem | messages.go:798-805 | OK | — |
| FolderRename (579) | RENAME | Renommer dossier | `Conn.RenameFolder` messages.go:779 | OK | — |
| getStatusFolderInformation (591) | Analyse réponse STATUS | Plomberie | go-imap `StatusData` | SANS OBJET | — |
| FolderStatus (634) | STATUS (MESSAGES, UNSEEN, UIDNEXT...) | Compteurs de dossier / non lus | `Poll` poll.go:19 (STATUS messages/unseen/uidnext) ; `listFolders` client.go:362 | OK | — |
| getArrayNameToStringName (654) | Aplatit les noms de dossiers retournés en listes | Plomberie parseur | go-imap | SANS OBJET | — |
| getFoldersFromResult (686) | Construit les dossiers depuis LIST/LSUB ; ajoute INBOX si absent ; associe STATUS (LIST-STATUS) | Arbre des dossiers + compteurs | `listFolders` client.go:362-396 ; INBOX synthétique absent (non nécessaire, tout serveur la liste) | OK | — |
| specificFolderList (808) | LIST/LSUB, option `RETURN (STATUS (MESSAGES UNSEEN UIDNEXT))` si LIST-STATUS | Charger arbre + compteurs en 1 requête | `listFolders` client.go:362 : LIST puis un STATUS par dossier en pipeline (client.go:373-380) ; pas de LIST-STATUS | PARTIEL | Fonctionnel mais N commandes. Utiliser `ListOptions{ReturnStatus: &imap.StatusOptions{...}}` quand `CapListStatus` pour une seule requête. P3 (perf sur gros arbres) |
| FolderList (859) | LIST | Liste des dossiers | `c.List("", "*")` client.go:363 | OK | — |
| FolderSubscribeList (873) | LSUB (dossiers abonnés) | Gestion des abonnements | `markUnsubscribed` client.go:398 et `Conn.Subscribed` messages.go:811 (LIST-EXTENDED SUBSCRIBED) | PARTIEL | Si le serveur n'a pas LIST-EXTENDED, Go considère tout abonné (messages.go:812) alors que RainLoop utilise LSUB. Repli : `c.List` avec LSUB n'est pas exposé par go-imap v2 : P3 (Dovecot a LIST-EXTENDED) |
| FolderStatusList (887) | LIST + STATUS | Arbre avec compteurs | `Conn.Folders(withStatus)` messages.go:127 | OK | — |
| initCurrentFolderInformation (899) | Analyse SELECT (FLAGS, PERMANENTFLAGS, UIDVALIDITY, UIDNEXT, HIGHESTMODSEQ, EXISTS, RECENT) | Connaître l'état du dossier | `imap.SelectData` retourné par `Conn.Select` messages.go:121 ; utilisé `sel.NumMessages` messages.go:675. UIDVALIDITY/HIGHESTMODSEQ non exploités | PARTIEL | UIDVALIDITY non vérifiée : une URL de message mémorisée (UID) peut viser un autre message si le dossier est recréé. Proposition : comparer `SelectData.UIDValidity` à celle embarquée dans les liens de lecture (mail.go) ; sinon 404. P3 |
| selectOrExamineFolder (979) | SELECT ou EXAMINE, évite le re-SELECT du même dossier | Lecture sans effet de bord (EXAMINE) | `Conn.Select(folder, readOnly)` messages.go:121 (readOnly => EXAMINE), `FetchMessage(...readOnly)` messages.go:584 | OK | Pas de cache « déjà sélectionné » : go-imap renvoie un nouveau SELECT à chaque appel (coût mineur) |
| FolderSelect (1017) | SELECT écriture | idem | messages.go:121 | OK | — |
| FolderExamine (1032) | EXAMINE (ou SELECT forcé) | idem | messages.go:121 | OK | — |
| FolderUnSelect (1043) | UNSELECT | Libérer le dossier sans EXPUNGE | aucun (Go ne ferme pas le dossier ; `Close()` non appelé, pas de risque d'EXPUNGE implicite) | SANS OBJET | Aucun intérêt fonctionnel |
| Fetch (1065) | FETCH/UID FETCH avec éléments et callbacks de flux sur littéraux | Lire entêtes, structure, corps, pièces | `c.Client.Fetch(...)` : liste messages.go:416-497 (Envelope, BodyStructure, Flags, BodySection entêtes priorité), `FetchMessage` messages.go:584 (RFC822 complet, taille bornée par `maxSize`), `Poll` poll.go | PARTIEL | Go télécharge le message entier (borné par la limite du dossier de config, web/mail.go:445) au lieu de FETCH par partie/PREVIEW ; une pièce de 20 Mo est donc relue en entier pour chaque ouverture/téléchargement. Optimisation : FETCH `BODY.PEEK[n]` pour une pièce (index issu de BODYSTRUCTURE). P3 |
| Quota (1123) | GETQUOTAROOT INBOX : STORAGE et MESSAGE | Jauge d'occupation de la boîte | `Conn.Quota` messages.go:830-846, affiché web/mail.go:525 (`quotaView`) | PARTIEL | Seul STORAGE est lu ; le quota en nombre de messages (MESSAGE) est ignoré. P3 |
| MessageSimpleSort (1180) | UID SORT avec critères | Tri côté serveur | `sortedUIDs` messages.go:185 (`UIDSort`, clés date/arrivée/from/subject/taille) | OK | — |
| simpleESearchOrESortHelper (1253) | ESEARCH/ESORT (RETURN ALL/MIN/MAX/COUNT) | Pagination rapide sans télécharger toutes les listes | aucun (go-imap `UIDSearch` utilise ESEARCH selon le serveur) | SANS OBJET | Optimisation transparente de la bibliothèque |
| MessageSimpleESearch (1365) | Enveloppe ESEARCH | idem | idem | SANS OBJET | — |
| MessageSimpleESort (1383) | Enveloppe ESORT | idem | idem | SANS OBJET | — |
| findLastResponse (1392) | Dernière réponse (continuation) | Plomberie | — | SANS OBJET | — |
| MessageSimpleSearch (1418) | UID SEARCH avec CHARSET UTF-8 et littéraux | Recherche de messages | `sortedUIDs` messages.go:218 (`UIDSearch`) ; critères `searchCriteria` messages.go:140 avec portée (tout/sujet/expéditeur/corps) et opérateurs `applyOperators` search.go:72 | OK | Dépasse RainLoop (opérateurs taille/date) |
| validateThreadItem (1497) | Normalise un arbre THREAD | Plomberie | go-imap `ThreadData` | SANS OBJET | — |
| MessageSimpleThread (1546) | THREAD REFS/REFERENCES/ORDEREDSUBJECT | Vue par conversations | `Conn.groupThreads` messages.go:512 (REFERENCES sinon ORDEREDSUBJECT), `hasThreads` messages.go:498, indicateur `ThreadsUnsupported` | PARTIEL | `THREAD=REFS` (Dovecot/Cyrus, plus rapide) n'est pas testé : `hasThreads` ne reconnaît que REFERENCES/ORDEREDSUBJECT. Ajouter `imap.Cap("THREAD=REFS")` (go-imap : `imap.ThreadAlgorithm("REFS")`). P3 |
| MessageCopy (1626) | UID COPY | Copier vers un dossier | `Conn.Copy` messages.go:867 | OK | — |
| MessageMove (1651) | UID MOVE (erreur si non supporté) | Déplacer, corbeille, archive | `Conn.Move` messages.go:624 (MOVE, repli COPY+\Deleted+EXPUNGE ciblé) | OK | Plus robuste que RainLoop (repli) |
| MessageExpunge (1682) | EXPUNGE ou UID EXPUNGE (UIDPLUS) | Suppression définitive sans toucher aux messages marqués par d'autres clients | `deleteAndExpunge` messages.go:647, `Expunge` 659, `ClearFolder` 672 | OK | — |
| MessageStoreFlag (1710) | UID STORE avec action +/-/= FLAGS | Marquer lu/important/etc. | `Conn.StoreFlags` messages.go:612 ; `MarkAllRead` 849 | OK | — |
| MessageAppendStream (1737) | APPEND (flags, date interne, flux), récupère APPENDUID | Enregistrer brouillon / copie envoyés ; importer un .eml | `Conn.Append` messages.go:726 (flags, repli par Message-ID si pas d'UIDPLUS) ; appelé compose.go:780, mailappend.go | PARTIEL | Pas de date interne : `AppendOptions.Time` jamais renseigné (messages.go:728). Pour un import .eml ou copie « Envoyés », la date de réception devient la date de l'append au lieu de la date du message : tri chronologique faussé. Proposition : paramètre `time.Time` optionnel (Date: du message) dans `Append` et `AppendOptions{Time: t}`. P2 pour l'import (mailappend.go), P3 sinon |
| FolderCurrentInformation (1783) | Renvoie l'info du dossier sélectionné | Plomberie | `SelectData` (messages.go:121) | OK | — |
| SendRequest (1798) | Envoie commande taguée, masque le mot de passe du log | Plomberie + journalisation sûre | go-imap ; aucun journal du trafic IMAP : `trace` messages.go:40 sans paramètres sensibles | OK | Mieux protégé (aucun secret dans les traces) |
| secureRequestParams (1847) | Masque le mot de passe LOGIN dans les logs | Sécurité des journaux | traces sans identifiants messages.go:40-62 | OK | — |
| SendRequestWithCheck (1875) | Commande + vérification du OK | Plomberie | `.Wait()` partout | SANS OBJET | — |
| GetLastResponse (1886) | Dernière réponse | Plomberie | — | SANS OBJET | — |
| validateResponse (1900) | Lève une exception si NO/BAD | Gestion d'erreurs | `Classify` errors.go:48, messages d'erreur web/mail.go:445 | OK | — |
| parseResponse (1935) | Boucle de lecture jusqu'au tag | Plomberie | go-imap | SANS OBJET | — |
| parseResponseWithValidation (1996) | idem + validation | Plomberie | go-imap | SANS OBJET | — |
| initCapabilityImapResponse (2006) | Capture [CAPABILITY] des réponses | Plomberie | go-imap | SANS OBJET | — |
| skipBracketParse (2036) | Cas particuliers du parseur LIST/STATUS | Plomberie | go-imap | SANS OBJET | — |
| partialParseResponseBranch (2052) | Analyseur récursif des réponses IMAP (atomes, listes, littéraux, FETCH) | Plomberie | go-imap | SANS OBJET | Couvert par la bibliothèque |
| partialResponseLiteralCallbackCallable (2540) | Flux des littéraux vers un callback (téléchargement de pièce en streaming sans tout charger) | Télécharger pièce volumineuse sans saturer la mémoire | `FetchMessage` charge le message entier en mémoire (messages.go:584-609, plafond `maxSize`) | PARTIEL | Même point que `Fetch` : pas de streaming, plafond de taille. Pièces > plafond impossibles à télécharger. Proposition : FETCH `BODY.PEEK[part]` et copie en flux vers `ResponseWriter` pour la route de téléchargement. P3 |
| prepearParamLine (2644) | Sérialise les paramètres | Plomberie | go-imap | SANS OBJET | — |
| getNewTag (2667) / getCurrentTag (2676) | Tags | Plomberie | go-imap | SANS OBJET | — |
| EscapeString (2686) | Cite les chaînes (injection de commandes) | Sécurité : pas d'injection IMAP via noms de dossiers/recherches | go-imap encode les arguments ; `ValidFolderName` messages.go:694 refuse `*`, `%`, contrôles | OK | — |
| getLogName (2694) | Nom du journal | Plomberie | — | SANS OBJET | — |
| SetLogger (2706) | Branche le journal | Journal de debug IMAP pour l'admin | `Conn.log` slog, trace par opération messages.go:30-62 | PARTIEL | Pas de journal protocolaire brut (RainLoop « Mode Debug » IMAP) ; compensé par des traces d'opérations (durée, erreur). Acceptable : P3 |
| TestSetValues (2719) / TestParseResponseWithValidationProxy (2733) | Aides de test unitaire | Tests | `mailtest` internal/mailconn/mailtest | SANS OBJET | — |

## 2. Imap/BodyStructure.php

L'analyse de BODYSTRUCTURE est faite par go-imap (`imap.BodyStructure`) ; l'affichage du message par `mailview` à partir du message complet.

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (114) | Stocke les champs MIME | Plomberie | `imap.BodyStructureSinglePart` | SANS OBJET | — |
| MailEncodingName (138) | Encodage de transfert | Décoder le corps | go-message décode automatiquement (mailview/parse.go:17,118) | OK | — |
| PartID (146) | Index MIME de la partie | Adresser une pièce pour téléchargement | `Attachment.Index` mailview/parse.go:233 (index de séquence dans le message analysé) | OK | — |
| FileName (154) | Nom de fichier (RFC 2231) | Nom de la pièce | `h.Filename()` mailview/parse.go:172 ; `sp.Filename()` messages.go:325 | OK | — |
| ContentType (162) | Type MIME | Icône/ouverture | `Attachment.ContentType` parse.go:233 | OK | — |
| Size (170) | Taille encodée | Taille de pièce | `Attachment.Size` parse.go:54,233 (taille décodée réelle) | OK | — |
| EstimatedSize (178) | Taille estimée décodée (base64 x0,75, QP x0,44) | Afficher taille des pièces dans la liste sans les télécharger | Taille exacte après décodage (parse.go:233) ; liste : taille totale `RFC822Size` seulement (messages.go:383) | OK | La taille exacte est connue à l'ouverture ; la liste ne montre pas la taille par pièce (RainLoop non plus dans la liste) |
| Charset (197) | Charset de la partie | Décoder correctement | `go-message/charset` parse.go:17 ; repli inconnu parse.go:118,156 | OK | — |
| ContentID (206) | Content-ID | Images inline cid: | `NormalizeCID` parse.go:95, sanitize.go:185 | OK | — |
| ContentLocation (214) | Content-Location | Images référencées par URL de localisation (HTML d'Outlook) | grep `Content-Location` mailview : aucun | MANQUANT | Certains expéditeurs référencent l'image inline par `Content-Location` et non par `cid:` ; l'image s'affiche alors comme pièce jointe/cassée. Indexer `Content-Location` dans `parse.go` à côté de CID et résoudre dans `sanitize.go` pour les `src` correspondants. P3 |
| IsInline (222) | Disposition inline ou présence de Content-ID | Ne pas lister les images incorporées comme pièces | parse.go:179-185 | OK | — |
| IsImage (231) | Détection d'image (type ou extension) | Vignettes / aperçu des images | `attachmentKind` messages.go:340 ; `Thumbnail` mailview/thumb.go:23 | OK | — |
| IsArchive (239) | Détection d'archive | Icône archive | `attachmentKind` messages.go:358 (« archive ») | OK | — |
| IsPdf (247) | Détection PDF | Icône/aperçu PDF | messages.go:351 (« pdf ») | OK | — |
| IsDoc (255) | Détection document bureautique | Icône document | messages.go:360-364 (texte/tableur/présentation) | OK | — |
| IsPgpSignature (263) | Pièce de signature PGP/S-MIME (application/pgp-signature, pkcs7-signature) | Ne pas afficher la signature comme pièce jointe | grep `pgp|pkcs7|smime` dans internal : aucun | MANQUANT | Un message signé (`multipart/signed`) montre une pièce « smime.p7s »/« signature.asc » inutile et la compteur de pièces jointes (trombone) est faussé (`hasAttachment` messages.go:306). Filtrer ces deux types dans `attachmentInfo` (messages.go:312) et `mailview/parse.go` (liste) tout en laissant le téléchargement possible. P2 |
| IsAttachBodyPart (272) | Une partie est-elle une pièce jointe (disposition attachment, ou ni texte/html/multipart) | Compteur et liste des pièces jointes | `attachmentInfo` messages.go:312-336 (attachment, ou filename sans inline) ; parse.go:166-233 | PARTIEL | Règle différente : RainLoop compte toute partie non text/html/plain, même sans nom ni disposition (ex. `image/png` anonyme, `message/rfc822`). Go exige une disposition attachment ou un nom : une pièce anonyme n'est pas signalée par le trombone. Aligner la condition sur RainLoop (hors `multipart/*`, `text/plain`, `text/html`, signatures) dans messages.go:322. P2 |
| IsFlowedFormat (291) | `format=flowed` (RFC 3676) hors base64/QP | Réaffichage correct du texte « flowed » (lignes recomposées) | grep `flowed` : aucun ; `TextToHTML` text.go:15 | MANQUANT | Les mails texte « flowed » (Thunderbird, Apple Mail) s'affichent avec des coupures de ligne en dur à 72 colonnes. Proposition : dans `mailview/parse.go`, lire le paramètre `format=flowed` (et `delsp`) du Content-Type de la partie text/plain et recomposer les lignes finissant par espace avant `TextToHTML`. P2 |
| SearchPlainParts (307) | Parties text/plain non jointes | Corps texte | parse.go (sélection du corps) | OK | — |
| SearchHtmlParts (324) | Parties text/html non jointes | Corps HTML | parse.go ; sanitize.go:124 | OK | — |
| SearchInlineEncryptedPart (343) | Corps d'un `multipart/encrypted` inline (PGP) | Afficher/déchiffrer PGP | aucun | SANS OBJET | OpenPGP : décision hors cahier des charges (à confirmer ; RainLoop le fait côté client avec OpenPGP.js) ; voir MANQUANT « PGP » dans la synthèse P3 |
| SearchHtmlOrPlainParts (363) | HTML sinon texte sinon chiffré | Choix du corps à afficher | parse.go (préférence HTML sinon texte) | OK | — |
| SearchCharset (386) | Charset dominant du message | Décoder le message d'après le texte | go-message par partie | OK | — |
| SearchByCallback (437) | Parcours de l'arbre | Plomberie | `bs.Walk` messages.go:318 | OK | — |
| SearchAttachmentsParts (459) | Liste des pièces jointes | Liste de pièces | parse.go:233 | OK | — |
| SearchByContentType (471) | Cherche par type | Plomberie | — | SANS OBJET | — |
| GetPartByMimeIndex (484) | Retrouve une partie par index | Télécharger pièce n | `m.Attachments[Index]` | OK | — |
| decodeAttrParamenter (517) | Décode paramètres RFC 2231/2047 (noms de fichiers accentués) | Noms de pièces lisibles | go-message `Filename()` parse.go:172 | OK | — |
| NewInstance (589) | Construit l'arbre depuis BODYSTRUCTURE (multipart, message/rfc822, disposition...) | Plomberie | go-imap | SANS OBJET | — |
| NewInstanceFromRfc822SubPart (901) | Structure d'un message attaché (message/rfc822) | Ouvrir un mail joint | parse.go analyse récursivement le message complet (pièce `message/rfc822` listée comme pièce) | PARTIEL | Un `.eml` joint est proposé en téléchargement ; pas d'ouverture dans la visionneuse (RainLoop l'affiche comme message). P3 |
| findPartByIndexInArray (919) | Plomberie | — | — | SANS OBJET | — |
| getKeyValueListFromArrayList (946) | Paramètres en liste clé/valeur | Plomberie | go-imap | SANS OBJET | — |

## 3. Imap/FetchResponse.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (35) / NewInstance (45) | Enveloppe de réponse FETCH | Plomberie | `imapclient.FetchMessageBuffer` | SANS OBJET | — |
| GetEnvelope (55) | ENVELOPE | Expéditeur/sujet/date | `m.Envelope` summaryOf messages.go:382 | OK | — |
| GetFetchEnvelopeValue (70) | Valeur d'index dans l'enveloppe | Plomberie | champs typés | SANS OBJET | — |
| GetFetchEnvelopeEmailCollection (81) | Adresses From/To/Cc avec décodage d'entêtes MIME et charset | Noms d'expéditeur lisibles | `Address` messages.go:66-75 ; décodage par go-imap + `fixUTF8` messages.go:371 (rattrape ISO-8859-1) | PARTIEL | Seuls From et To sont lus dans la liste (messages.go:391-398) ; Cc/Reply-To affichés à l'ouverture via mailview (non listés). Acceptable. P3 |
| GetFetchBodyStructure (119) | BODYSTRUCTURE → objet | Pièces jointes dans la liste | `m.BodyStructure` messages.go:385 | OK | — |
| GetFetchValue (144) | Valeur d'un élément FETCH | Plomberie | — | SANS OBJET | — |
| GetHeaderFieldsValue (178) | Entêtes demandés (HEADER.FIELDS) | Priorité, List-Unsubscribe, etc. | `BodySection` entêtes priorité messages.go:392 ; `highPriority` messages.go:880 | OK | — |
| findFetchUidAndSize (208) / IsValidFetchImapResponse (235) / IsNotEmptyFetchImapResponse (251) / findEnvelopeIndex (268) | Validation/filtrage des réponses FETCH incomplètes | Plomberie | go-imap + `uint32(uid) != uid` vérifié messages.go:600 | SANS OBJET | — |

## 4. Imap/Folder.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (59) | Normalise le nom (INBOX, délimiteur `.` par défaut, refuse noms vides) | Arbre de dossiers fiable | `Folder` client.go:305-320 ; préfixe INBOX normalisé foldertree.go:30 ; nœuds synthétiques foldertree.go:86-109 | OK | — |
| NewInstance (115) | Fabrique | Plomberie | — | SANS OBJET | — |
| NameRaw (123) | Dernier segment du nom | Affichage du nom court | `Folder.Leaf` client.go:337 | OK | — |
| FullNameRaw (131) | Nom complet | Identifiant de dossier | `Folder.Name` | OK | — |
| Delimiter (139) | Séparateur | Hiérarchie | `Folder.Delim` / `Depth` client.go:329 | OK | — |
| Flags (147) / FlagsLowerCase (155) | Attributs LIST | Dossiers spéciaux (\Sent, \Trash...) | `Folder.Attrs`, `HasAttr` client.go:321 ; `ResolveFolder` client.go:486-535 | OK | — |
| IsSelectable (163) | Absence de `\Noselect` | Ne pas ouvrir les dossiers conteneurs | `Folder.Selectable` client.go:324 | OK | — |
| IsInbox (171) | INBOX ou `\Inbox` | Tri/placement de la boîte de réception | `folderRank`/`SortFolders` client.go:429-470 | OK | — |
| SetExtended (180) / GetExtended (189) | Données STATUS attachées | Compteurs | `Folder.Total/Unseen/HasCount` client.go:312 | OK | — |

## 5. Imap/FolderInformation.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (76) / NewInstance (96) | Structure d'état de dossier | Plomberie | `imap.SelectData` | SANS OBJET | — |
| IsFlagSupported (106) | Le dossier accepte-t-il le mot-clé (PERMANENTFLAGS ou `\*`) ? | Tags/mots-clés personnalisés ($Forwarded, $MDNSent) uniquement si possible | aucun test : `$Forwarded` posé compose.go:695 et `$MDNSent` compose.go:1067 sans vérifier PERMANENTFLAGS | PARTIEL | Sur un serveur refusant les mots-clés libres, le STORE échoue (erreur remontée, message déjà envoyé). Consulter `SelectData.PermanentFlags` avant et ignorer silencieusement l'échec. P3 |

## 6. Imap/NamespaceResult.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (53) / NewInstance (66) | Structure | Plomberie | — | SANS OBJET | — |
| InitByImapResponse (76) | Lit les 3 espaces de noms (personnel, autres utilisateurs, partagés) | Dossiers partagés | aucun | SANS OBJET | RainLoop n'expose que l'espace personnel ; partagés non gérés ici non plus |
| GetPersonalNamespace (120) / GetPersonalNamespaceDelimiter (128) | Préfixe et séparateur personnels | Voir GetNamespace | cf. `GetNamespace` : séparateur depuis LIST uniquement | PARTIEL | Voir proposition NAMESPACE (P3) |

## 7. Imap/Response.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (58) / NewInstance (72) | Réponse analysée | Plomberie | go-imap | SANS OBJET | — |
| recToLine (82) / ToLine (100) | Sérialise pour le journal | Journal de debug | traces slog | SANS OBJET | — |

## 8. Imap/Enumerations (FetchType, FolderStatus, FolderType, FolderResponseStatus, MessageFlag, ResponseStatus, ResponseType, StoreAction)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| FetchType::addHelper (48) | Ajoute un élément FETCH | Plomberie | `imap.FetchOptions` | SANS OBJET | — |
| FetchType::BuildBodyCustomHeaderRequest (67) | Construit `BODY.PEEK[HEADER.FIELDS (...)]` | Récupérer quelques entêtes (priorité, Message-ID, List-Unsubscribe) | `FetchItemBodySection{Peek, HeaderFields}` messages.go:452-454 | OK | — |
| FetchType::ChangeFetchItemsBefourRequest (87) | Développe ALL/FAST/FULL, ajoute UID et RFC822.SIZE | Plomberie | `FetchOptions{UID, RFC822Size}` messages.go:452-454 | OK | — |
| Constantes FolderType, MessageFlag, StoreAction, ResponseStatus... | Types de dossier, drapeaux (`\Seen`, `\Flagged`, `\Answered`, `$Forwarded`, `$Junk`/`$NonJunk`), actions STORE | Marquage lu/important/répondu/transféré | `imap.FlagSeen/Flagged/Answered`, `$Forwarded` messages.go:403-408 ; `StoreFlagsOp` | PARTIEL | Les mots-clés `$Junk`/`$NotJunk` (spam) ne sont posés nulle part (grep : aucun) : le bouton spam déplace seulement vers Junk (Dovecot/antispam apprend par dossier, ok). P3 |

## 9. Imap/Exceptions

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ResponseException::__construct (32), GetResponses (45), GetLastResponse (53) | Porte les réponses du serveur dans l'erreur | Messages d'erreur précis pour l'utilisateur | `Classify` errors.go:48, `TestError.Detail` errors.go:36 | OK | — |
| NegativeResponseException::getAlertFromStatus (24) | Extrait le texte `[ALERT]` du serveur (ex. « compte verrouillé », « quota dépassé ») pour l'afficher | Voir l'alerte du serveur IMAP à l'utilisateur | grep `ALERT` : aucun (hors alerte TLS errors.go:62) | MANQUANT | Les alertes serveur (response code `[ALERT]` dans OK/NO/BAD, ou réponse non taguée) ne sont pas affichées. go-imap v2 les expose via `imapclient.Options.UnilateralDataHandler`/erreur `*imap.Error` (`Code`, `Text`). Proposition : dans `Classify` (errors.go:48), si `*imap.Error` a `Code == imap.ResponseCodeAlert`, afficher `Text` tel quel. P3 |
| Autres classes (LoginException, LoginBadCredentialsException, LoginBadMethodException, InvalidResponseException, ResponseNotFoundException, RuntimeException, Exception) | Types d'erreurs | Messages « identifiants invalides » | `KindAuth` etc. dans errors.go:17-26, `authStage` client.go:136 | OK | — |

## 10. Net/NetClient.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| __construct (88) | Init | Plomberie | — | SANS OBJET | — |
| __destruct (114) | Ferme la socket à la destruction | Pas de fuite de connexions | pool: `Close`/`Sweep` pool.go:114,135 ; `defer closeIMAP` client.go:~205 | OK | — |
| Clear (133) | Remet à zéro l'état | Plomberie | — | SANS OBJET | — |
| GetConnectedHost (147) / GetConnectedPort (155) | Hôte/port connectés | Journal | `Conn.server` messages.go:33 | OK | — |
| SetTimeOuts (166) | Délais connexion/lecture (10 s par défaut, configurable) | Ne pas bloquer sur serveur lent | `dialTimeout=20s` pool.go:13 ; délai par requête `WithTimeout` client.go:538, `testTimeout` accounts.go:296 ; fermeture socket à l'expiration du contexte client.go:80 | OK | — |
| ConnectionResource (175) | Accès socket | Plomberie | — | SANS OBJET | — |
| capturePhpErrorWithException (188) | Convertit les warnings PHP en exception | Interne PHP | — | SANS OBJET | — |
| Connect (207) | Ouvre la socket (ssl:// ou tcp://), contexte SSL (verify_peer, allow_self_signed, client cert), hook | Connexion sécurisée, option « vérifier certificat » | `Guard.connect` client.go:71-90 ; `tlsConfig` client.go:47 (`VerifyCert`, `LegacyTLS`) ; garde anti-SSRF `Guard.DialContext` guard.go:134 (réseaux privés bloqués) | OK | Mieux protégé que RainLoop (liste d'hôtes autorisés). Certificat client : voir ImapClient::Connect |
| EnableCrypto (316) | Bascule STARTTLS (TLS_CLIENT / ANY) | STARTTLS | `imapclient.NewStartTLS` client.go:114, `smtp.NewClientStartTLS` client.go:249 (TLS ≥ 1.2, repli ancien explicite) | OK | — |
| Disconnect (346) | Ferme la socket | Fermeture | `closeIMAP` client.go:183 | OK | — |
| LogoutAndDisconnect (372) | LOGOUT puis fermeture si aucun tampon non lu | Fermeture propre | client.go:183, pool.go:47 (`drop`) | OK | — |
| IsConnected (387) / IsConnectedWithException (405) | État connexion | Détecter connexion morte | `closedClient` pool.go:38 | OK | — |
| StreamContextParams (413) | Options TLS de la socket (infos du certificat) | Afficher le certificat à l'admin | aucun ; erreurs de certificat classées `isCertError` errors.go:90 | SANS OBJET | Utilitaire interne de test de connexion admin |
| sendRaw (429) | Écriture avec masquage dans le journal | Plomberie + journal | go-imap/net.Conn | SANS OBJET | — |
| getNextBuffer (481) | Lecture ligne/octets, détection délai dépassé, journal tronqué à 5 Ko | Plomberie, délai de lecture | go-imap ; délai via contexte (client.go:80) | SANS OBJET | — |
| getLogName (557) / writeLog (568) / writeLogWithCrlf (582) / writeLogException (594) / SetLogger (620) / Logger (633) | Journalisation réseau (admin « log IMAP/NET ») | Diagnostic admin | `Conn.trace` messages.go:40 + slog | PARTIEL | Pas de trace brute du protocole (RainLoop mode « debug » avec mots de passe masqués). P3 |

## 11. Net/Enumerations et Net/Exceptions

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| ConnectionSecurityType::UseSSL (32) | AUTO_DETECT : SSL implicite si port 993/995/465 | Défaut de sécurité d'après le port | formulaire compte : valeurs par défaut fixes accounts.go:118 (SMTP 587 STARTTLS) ; aucune déduction selon le port saisi | PARTIEL | Si l'utilisateur saisit 993 avec « STARTTLS », échec explicite (message classé). Proposer, dans le formulaire (templates + accounts.go), de préselectionner TLS lorsque le port vaut 993/465 et STARTTLS pour 143/587. P3 |
| ConnectionSecurityType::UseStartTLS (63) | STARTTLS si annoncé (et préféré en auto) | Chiffrement opportuniste | STARTTLS obligatoire sans repli en clair (client.go:23) | OK | Choix plus strict, voulu |
| SocketCanNotConnectToHostException::__construct (38) / getSocketMessage (49) / getSocketCode (57) | Message et code système de la panne de connexion | Diagnostic « hôte injoignable / refusé » | `Classify` errors.go:48 (catégories réseau/DNS/TLS/auth) | OK | — |
| Autres exceptions Net (SocketAlreadyConnected, SocketCanNotConnectToHost, SocketConnectionDoesNotAvailable, SocketReadTimeout, SocketRead, SocketWrite, SocketUnreadBuffer, SocketUnsuppoterdSecureConnection, InvalidArgument, Connection, Exception) | Types d'erreurs | Messages d'erreur distincts | errors.go:17-26 (Kind*) | OK | — |

## À AJOUTER (MANQUANT / PARTIEL, par priorité)

### P2
1. Signature S/MIME ou PGP (`application/pkcs7-signature`, `application/pgp-signature`) : ne plus compter ni lister comme pièce jointe (BodyStructure::IsPgpSignature) ; corriger le trombone (messages.go:306-336) et la liste de pièces (mailview/parse.go:166-233).
2. Règle des pièces jointes alignée sur RainLoop (IsAttachBodyPart) : toute partie non `multipart/*`, `text/plain`, `text/html` compte, même sans nom ni disposition (messages.go:322).
3. Texte `format=flowed` (RFC 3676, delsp) : recomposer les lignes avant `TextToHTML` (mailview/parse.go, text.go:15).
4. APPEND avec date interne (`AppendOptions.Time`) pour l'import .eml (mailappend.go) afin de conserver la date du message (messages.go:728).

### P3
5. `Content-Location` pour images inline sans `cid:` (mailview/parse.go, sanitize.go).
6. Authentification SASL PLAIN quand `LOGINDISABLED` (client.go:142-175) ; XOAUTH2 à écarter.
7. NAMESPACE : préfixe des dossiers racine (serveurs Courier/Cyrus) à la création de dossier (folders.go).
8. LIST-STATUS en une requête au lieu d'un STATUS par dossier (client.go:362-396).
9. `THREAD=REFS` reconnu (messages.go:498, 512).
10. Téléchargement de pièce par FETCH de partie, en flux, sans charger le message entier (messages.go:584, plafond de taille).
11. Quota : lire aussi la limite en nombre de messages (messages.go:830).
12. Affichage de l'alerte serveur `[ALERT]` (errors.go:48).
13. Vérification de `UIDVALIDITY` dans les liens de lecture (messages.go:121, web/mail.go).
14. Test `PERMANENTFLAGS` avant `$Forwarded`/`$MDNSent` (compose.go:695,1067).
15. Défaut de sécurité d'après le port saisi (993/465 => TLS) dans le formulaire de compte.
16. Ouverture d'un message `message/rfc822` joint dans la visionneuse (mailview/parse.go).
17. Abonnements sans LIST-EXTENDED (repli LSUB), `$Junk`/`$NotJunk`, journal protocolaire brut, certificat client mTLS : à ne faire que sur besoin avéré.
