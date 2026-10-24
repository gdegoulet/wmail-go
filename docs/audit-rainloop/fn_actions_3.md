# Analyse RainLoop 1.17 Actions.php, lignes 5200 à 7700, face à wmail-go

Périmètre : 52 fonctions (de `recFoldersTypes` ligne 5198 à `Upload` ligne 7666 ; `DoClearUserBackground` ligne 7712 est hors plage). Tout le périmètre a été lu. Les chemins Go sont relatifs à /home/gdegoulet/Work/webmail_project.

Légende : OK, PARTIEL, MANQUANT, SANS OBJET. Les priorités P1/P2/P3 ne concernent que PARTIEL et MANQUANT.

## 1. Dossiers (Actions.php 5198 à 5714)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| recFoldersTypes (5198) | Parcourt récursivement l'arbre des dossiers pour trouver Envoyés, Brouillons, Spam, Corbeille, Archive : d'abord par attribut IMAP (SPECIAL-USE), puis par table de noms usuels | Détection automatique des dossiers système sans configuration | internal/mailconn/client.go:486-536 (`ResolveFolder` : valeur explicite, puis attribut SPECIAL-USE, puis noms usuels, puis défaut) ; liste plate donc pas de récursion nécessaire | OK | Les noms usuels couvrent Sent Items, Envoyés, Corbeille, Spam, etc. |
| DoFolders (5269) | Liste des dossiers (LIST, abonnés, limite 200), calcul des types système, **création automatique** des dossiers système manquants si non réglés, hash de la liste | Arbre de dossiers à gauche, dossiers système toujours présents | internal/mailconn/client.go:362 `listFolders`, :443 `SortFolders`, internal/web/folders.go:55 ; création **paresseuse** uniquement : `EnsureFolder` (internal/mailconn/messages.go:707) appelé à l'envoi (internal/web/compose.go:686), à l'enregistrement de brouillon (compose.go:~776) et au déplacement archive/spam/corbeille (internal/web/mail.go, `moveTo`) | PARTIEL | Différence : RainLoop crée Sent/Drafts/Spam/Trash/Archive dès le chargement de la liste ; Go ne les crée qu'au premier usage, donc ils peuvent être absents de l'arbre au début. Proposition : au premier chargement de l'accueil (une fois par session et par boîte), pour chaque rôle non réglé à `NoFolder` et sans dossier résolu existant, appeler `EnsureFolder(Resolve*)`. P3 |
| DoFolderCreate (5415) | Crée un dossier (parent + nom), l'abonne | Bouton « créer un dossier » | internal/web/folders.go:169 (`case "create"`), internal/mailconn/messages.go:766 `CreateFolder` (crée et abonne) | OK | Validation de nom et du délimiteur en plus (folders.go:~150). |
| DoFolderSubscribe (5443) | Abonne ou désabonne | Cacher ou montrer un dossier | folders.go:193 (`case "subscribe"`), messages.go:798 `SetSubscribed`, lecture messages.go:811 | OK | |
| DoFolderCheckable (5477) | Mémorise la liste de dossiers dont le compteur de non lus est surveillé | « Vérifier ce dossier » en arrière-plan | internal/web/folders.go:300 `foldersCheck` (`acc.CheckFolders`, plafond `userdata.MaxCheckFolders`), utilisée par internal/web/poll.go:56 | OK | |
| DoFolderRename (5529) | Renomme un dossier | Renommer | folders.go:176 (`case "rename"`), messages.go:779 `RenameFolder` ; oublie les rôles qui pointaient dessus (folders.go:~215 `forgetRoleFolder`) | OK | |
| DoFolderDelete (5559) | Supprime un dossier ; **refuse s'il contient des messages** (erreur CantDeleteNonEmptyFolder) | Garde-fou contre la perte de mails | folders.go:187 (`case "delete"`), messages.go:788 `DeleteFolder` : supprime le dossier ET ses messages, mais template folders_settings.html:60-64 impose une confirmation explicite (`folders.delete_confirm` : « …ET tous les messages… définitive ») | PARTIEL | Différence : pas de refus automatique d'un dossier non vide. Proposition (optionnelle) : dans `DeleteFolder`, faire un STATUS MESSAGES et refuser si > 0 sauf case « je veux aussi supprimer les messages » cochée. P3 |
| DoFolderClear (5592) | Vide un dossier (messages supprimés + expunge) | « Vider la corbeille / le spam » | internal/web/mail.go:1213 `mailClear` (limité à corbeille et indésirables, INBOX jamais), messages.go:672 `ClearFolder` | OK | |
| DoFolderInformation (5615) | Sonde un dossier : non lus, UIDNEXT, nouveaux messages, **et drapeaux (lu, étoile, répondu, supprimé, transféré, MDN) des UID affichés** pour les rafraîchir | Compteurs en direct, notification de nouveaux mails, liste synchronisée avec les actions faites ailleurs (téléphone, autre client) | internal/web/poll.go:36 `mailPoll`, internal/mailconn/poll.go:19 `Poll` (UNSEEN, UIDNEXT, nouveaux non lus avec expéditeur et objet), internal/web/static/poll.js | PARTIEL | Compteur et nouveaux : OK. Absent : rafraîchissement des drapeaux des messages visibles (un message lu ou marqué depuis un autre client reste « non lu » dans la liste tant qu'on ne recharge pas). Proposition : `GET /mail/flags?folder=&uids=` renvoyant `FETCH FLAGS` pour les UID de la page, et poll.js met à jour les classes `unread`, `answered`, étoile. P3 |
| DoFolderInformationMultiply (5674) | Sonde plusieurs dossiers (sauf INBOX) pour leurs compteurs | Compteurs des dossiers « surveillés » | poll.go:54-60 (boucle sur `acc.CheckFolders`, champ `Folders`) | OK | |

## 2. Liste, construction de message, brouillons, envoi (5715 à 6555)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| DoMessageList (5715) | Liste paginée d'un dossier : offset, limite, recherche, UIDNEXT, fils de discussion, tri serveur optionnel, cache HTTP | Liste des messages, recherche, conversations | internal/web/mail.go:472 `home`, :209 `parseListParams`, :248 `advancedTerms` (recherche avancée), internal/mailconn/messages.go:416 `ListMessages`, :185 `sortedUIDs`, :512 `groupThreads`, internal/mailconn/search.go | OK | Le cache par clé (`RawKey`) est propre à l'AJAX de RainLoop : sans objet ici (rendu serveur). |
| buildMessage (5804) | Construit le MIME sortant : From d'une identité, Reply-To, Cc/Bcc, accusé de lecture, priorité haute, X-Mailer (masquable), DraftInfo, In-Reply-To, References, corps texte+HTML (avec images `data:` converties en CID), pièces jointes, Content-Location | Rédaction : envoi et brouillons | internal/web/compose.go:534 `buildOutgoing` ; internal/compose/build.go:100 `Build` (Importance/X-Priority, Disposition-Notification-To, X-Mailer conditionné par `cfg.Compose.HideXMailer`), internal/compose/html.go:43 `PrepareHTML` (data: vers cid:, texte brut dérivé), internal/compose/build.go:74 `NewMessageID` | PARTIEL | Fonctionnellement équivalent. Écarts : (a) le lien reply/forward d'un brouillon est écrit dans `X-Wmail-Draft-Info` (build.go:242, mailview/parse.go:129) alors que RainLoop utilise `X-Draft-Info` (valeur `type=reply; uid=N; folder=base64`) : les brouillons déjà créés par RainLoop perdent leur lien « répondu » à l'envoi. (b) Content-Location non géré. (c) Les images distantes du HTML sont supprimées volontairement (html.go:34-36), RainLoop les conserve. Proposition (a) : dans mailview/parse.go lire aussi `X-Draft-Info` et décoder type/uid/folder(base64) vers `compose.DraftInfo` en repli. P3 |
| deleteMessageAttachmnets (5981) | Supprime les fichiers temporaires des pièces jointes après envoi | Hygiène du stockage temporaire | internal/web/compose.go:~633 `s.attach.Clear(sess.ID, f.ID)` après envoi ; internal/web/attachstore.go:112 `Clear` | OK | |
| buildReadReceiptMessage (6002) | Construit le message d'accusé de lecture (From identité, Reply-To identité, texte) | Réponse à une demande de confirmation de lecture | internal/compose/mdn.go:26 `MDN.Build` (multipart/report conforme), usage internal/web/compose.go:1040-1048 | OK | Plus strict que RainLoop (RFC 8098). |
| DoSaveMessage (6057) | Enregistre un brouillon par APPEND (drapeau Seen), supprime l'ancienne version, renvoie le nouvel UID | « Enregistrer » et enregistrement auto du brouillon | internal/web/compose.go:712 `composeSave` (APPEND avec Seen+Draft, Bcc conservé, ancienne version expungée, réponse JSON folder/uid/saved), enregistrement auto : internal/web/static/compose.js:199 (`data-autosave`) | OK | |
| smtpSendMessage (6138) | Envoi SMTP : MAIL FROM, RCPT, DSN, rcpt cachés (plugin), repli `mail()` PHP, mapping des erreurs (connexion, authentification, destinataires invalides) | Envoi fiable, notification de livraison | internal/mailconn/client.go:561 `SendSMTP` (paramètre `dsn`), erreurs mappées par `composeErrKey` (compose.go:498), destinataires vides refusés (build `ErrNoRecipient`, compose.go:~563) | OK | `mail()` PHP, destinataires cachés par plugin : sans objet. |
| DoSendMessage (6292) | Construit, envoie, pose Answered ou Forwarded sur le message d'origine, copie dans Envoyés (avec Bcc), supprime le brouillon, incrémente la fréquence des contacts (To) et ajout auto au carnet | Envoi complet et ses effets de bord | internal/web/compose.go:593 `composeSend`, :666 `afterSend` (copie Envoyés ou dossier du message répondu si option `ReplySameFolder`, Answered/`$Forwarded` en :693-698, suppression du brouillon :704), compose.go internal/web/contactsweb.go:402 `contactsAfterSend` (Bump + ajout auto selon `NoContactsAutosave`) ; limitation de débit compose.go:603 | OK | Léger écart voulu : Go ajoute aussi les Cc au carnet. L'échec de la copie dans Envoyés ne bloque pas l'envoi et affiche `compose.sent_not_saved`. |
| DoSendReadReceiptMessage (6482) | Envoie l'accusé et marque le message (drapeau réglable) | « Envoyer l'accusé de lecture » | internal/web/compose.go:1000 `mailMDN` (envoyer ou ignorer, drapeau `$MDNSent`, enveloppe `<>`), affichage conditionnel mail.go:888-890 | OK | |
| DoQuota (6556) | Retourne usage et limite du quota IMAP | Jauge de quota | internal/mailconn/messages.go:830 `Quota`, internal/web/mail.go:525 et :1392-1401 `QuotaView` (alerte à 90 %) | OK | |

## 3. Synchronisation des contacts (6577 à 6665)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| getContactsSyncData (6577) | Lit la config CardDAV chiffrée (Enable, Url, User, Password) | Réglage de synchro | internal/userdata (`ContactsSync`), internal/web/contactssync.go:45 `syncView` | OK | |
| DoSaveContactsSyncData (6606) | Enregistre la config ; mot de passe factice = conserver l'ancien | Réglage de synchro CardDAV | internal/web/contactssync.go:146 `contactsSyncSave` (mot de passe vide = on garde, validation d'URL avec garde SSRF, intervalle borné) | OK | |
| DoContactsSync (6641) | Lance la synchronisation maintenant | Bouton « synchroniser » | contactssync.go:185 `contactsSyncRun`, :82 `runContactsSync`, synchro périodique :63 `maybeSyncContacts` (appelée par le sondage poll.go:50) ; internal/contacts/carddav.go | OK | |

## 4. Double authentification (6666 à 6934)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| getTwoFactorInfo (6666) | Lit l'état 2FA (activé, secret, codes de secours) pour le compte | Page de réglage 2FA | internal/web/twofactor.go:230 `twoFactorPage` (Enabled, Secret de configuration, BackupLeft) ; stockage internal/userdata `TwoFactor` | OK | |
| removeBackupCodeFromTwoFactorInfo (6734) | Consomme un code de secours utilisé | Code de secours à usage unique | twofactor.go:171 `totp.UseBackup` + enregistrement (codes stockés hachés) | OK | |
| DoGetTwoFactorInfo (6771) | Retourne l'état sans secret | Afficher « activé ou non » | twofactor.go:239 `twoFactorGet` | OK | |
| DoCreateTwoFactorSecret (6788) | Génère secret + 9 codes de secours | Démarrer l'activation | twofactor.go:243 `twoFactorStart`, QR : :252 `twoFactorQR` | OK | Go ne génère les codes qu'à l'activation validée (plus sûr). |
| DoShowTwoFactorSecret (6828) | Ré-affiche le secret (sans codes) après activation | Réenregistrer l'application d'authentification sur un nouvel appareil | twofactor.go:235 : le secret n'est affiché que pendant la configuration (`Setup: setup != "" && !tf.Enabled`) | PARTIEL | Différence : une fois activée, on ne peut plus revoir le secret/QR. Proposition : bouton « afficher le secret » protégé par la saisie d'un code valide (réutiliser `secondFactorOK`, twofactor.go:318). P3 |
| DoEnableTwoFactor (6850) | Active ou désactive le drapeau Enable | Valider l'activation | twofactor.go:285 `twoFactorEnable` (exige un code TOTP valide avant d'activer) | OK | |
| DoTestTwoFactorInfo (6884) | Teste un code TOTP | Bouton « tester » | twofactor.go:285 (le test fait partie de l'activation), :318 `secondFactorOK` | OK | |
| DoClearTwoFactorInfo (6913) | Supprime la config 2FA | Désactiver | twofactor.go:349 `twoFactorDisable` (code exigé, invalide les cookies « appareil de confiance ») | OK | |

## 5. Contacts et suggestions (6935 à 7129)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| DoContacts (6935) | Liste paginée + recherche | Écran contacts | internal/web/contactsweb.go:110 `contactsPage` (page, recherche, étiquette), internal/contacts/contacts.go:345 `Search` | OK | |
| DoContactsDelete (6968) | Supprime des contacts | Suppression | contactsweb.go:255 `contactsDelete`, contacts.go:270 `Delete` | OK | |
| DoContactSave (6987) | Crée ou modifie un contact (propriétés typées) | Éditeur de contact | contactsweb.go:238 `contactsSave`, :211 `contactFromForm`, contacts.go:236 `Save`, :114 `Normalize` | OK | |
| DoSuggestions (7052) | Autocomplétion d'adresses du carnet, limite configurable (défaut 20, min 5), dédoublonnage | Saisie rapide des destinataires | contactsweb.go:355 `contactsSuggest`, contacts.go:402 `Suggest` (nom, surnom, adresse ; tri par fréquence puis nom), front internal/web/static/contacts-ac.js | PARTIEL | Écart : la limite est figée à 8 (contactsweb.go:~375 `b.Suggest(q, 8)`) contre 20 chez RainLoop, non réglable. Proposition : constante 20 (ou champ `contacts.suggestions_limit` dans internal/config). P3 |

## 6. Actions sur les messages (7130 à 7399)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| messageSetFlag (7130) | Aide commune : pose ou retire un drapeau sur des UID | Lu/non lu, étoile | internal/web/mail.go:1251 `mailAction` (read, unread, flag, unflag), internal/mailconn/messages.go:612 `StoreFlags` | OK | |
| DoMessageSetSeen (7155) | Marque lu ou non lu | Idem | mail.go:1315-1322 | OK | |
| DoMessageSetSeenToAll (7163) | Marque **tous** les messages du dossier (ou d'un fil) lus ou **non lus** | « Tout marquer comme lu » (et non lu) | mail.go:1412 `markAllRead`, messages.go:849 `MarkAllRead` (action `readall`, mail.go:1264) | PARTIEL | Absent : « tout marquer comme non lu » (RainLoop le propose, app.js:35021) et la variante limitée aux messages d'un fil. Proposition : action `unreadall` symétrique (STORE -FLAGS \Seen sur 1:*), et, sur un fil, utiliser la liste des membres déjà transmise (`th_<uid>`, mail.go:1294). P3 |
| DoMessageSetFlagged (7187) | Étoile | Messages suivis | mail.go:1272-1281 (`star_on`/`star_off`) et :1318 | OK | |
| DoMessage (7197) | Charge un message complet (corps limité par `imap_body_text_limit`), cache HTTP | Lecture d'un message | mail.go:950 `mailMessage`, :724 `mailPane`, :776 `fetchMessage` (limite 30 Mio `maxMessageBytes`), :1008 `mailBody`, marquage lu à l'ouverture selon réglage `MarkReadOnOpen` (mail.go:739) | OK | |
| DoMessageDelete (7250) | Supprime (flag Deleted + expunge, UID ciblé) puis renvoie le hash du dossier | Suppression définitive | mail.go:1349-1356 (`delete` : déplacement en corbeille, ou EXPUNGE si on est déjà dans la corbeille), messages.go:647-670 `deleteAndExpunge`/`Expunge` | OK | Nuance : Go déplace en corbeille ; RainLoop JS fait pareil sauf dans corbeille/spam. |
| DoMessageMove (7300) | Déplace des messages ; **option MarkAsRead** (le client l'active quand la destination est Spam ou Corbeille, app.js:28632) ; apprentissage antispam | Déplacer, archiver, jeter | mail.go:1330-1347 (`move`, `archive`, `spam`, `moveTo`), messages.go:624 `Move` (MOVE ou COPY+EXPUNGE) | PARTIEL | Absent : marquage « lu » avant déplacement vers Spam/Corbeille. Conséquence : un mail jeté non lu fait monter le compteur de non lus de la corbeille ou du spam. Proposition : dans `mailAction`, avant `moveTo(trash)` (action `delete` hors corbeille) et `moveTo(junk)` (action `spam`), appeler `StoreFlags(... Add, FlagSeen)` sur les UID. Apprentissage antispam : sans objet. P2 |
| DoMessageCopy (7366) | Copie vers un dossier | « Copier vers » | mail.go:1340-1345 (`copy`), messages.go:867 `Copy` | OK | |

## 7. Pièces jointes, import, social (7400 à 7700)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| MainClearFileName (7400) | Nettoie un nom de fichier (caractères, longueur, nom par défaut) | Noms de pièces jointes sûrs | internal/compose/build.go:89 `safeFilename` (retire chemins et contrôles, nom par défaut « piece-jointe »), internal/web/mail.go:1200 `safeFileName` (téléchargements) | OK | Pas de troncature de longueur à 250 : P3 négligeable. |
| DoMessageUploadAttachments (7417) | Copie des pièces jointes d'un message existant dans la composition (transfert, édition) | Transférer avec les pièces jointes, rouvrir un brouillon | internal/web/compose.go:308 `copyAttachments` (modes forward, draft, editnew), transfert en pièce jointe .eml : compose.go:350-361 (`modeForwardAt`, `message/rfc822`) | OK | |
| DoComposeUploadExternals (7475) | Télécharge côté serveur des URL externes comme pièces jointes ; appelé uniquement par l'import Dropbox (app.js:13843) | Pièces jointes Dropbox | aucun | SANS OBJET | Dropbox/lien externe : hors périmètre. (Un téléchargement d'URL côté serveur serait aussi un risque SSRF.) |
| DoComposeUploadDrive (7515) | Idem depuis Google Drive avec jeton OAuth | Pièces jointes Google Drive | aucun | SANS OBJET | Service tiers. |
| DoSocialUsers (7561) | Lit les comptes sociaux liés | Réglages sociaux | aucun | SANS OBJET | Social. |
| DoSocialGoogleDisconnect (7586) | Délie le compte Google | idem | aucun | SANS OBJET | Social. |
| DoSocialTwitterDisconnect (7595) | idem Twitter | idem | aucun | SANS OBJET | Social. |
| DoSocialFacebookDisconnect (7604) | idem Facebook | idem | aucun | SANS OBJET | Social. |
| getUploadErrorMessageByCode (7616) | Traduit les codes d'erreur PHP d'envoi de fichier en messages client | Messages d'erreur d'envoi de fichier | internal/web/compose.go:812-850 (trop gros, trop nombreux, limite de message, limite de session ; messages i18n `compose.err.attach_too_big` etc.) | OK | Les erreurs `tmp_dir` PHP n'ont pas d'équivalent Go. |
| Upload (7666) | Réception d'un fichier téléversé : stockage temporaire, renvoie nom, taille, type, ou erreur | Joindre un fichier (clic ou glisser-déposer) | internal/web/compose.go:805 `composeAttach` (limites de taille, nombre, message, session), internal/web/attachstore.go:57 `Add`, :90 `Remove` (= compose.go:856 `composeDetach`), JS internal/web/static/compose.js, dnd.js | OK | |

## Comptage

52 fonctions : OK = 38, PARTIEL = 8 (DoFolders, DoFolderDelete, DoFolderInformation, buildMessage, DoShowTwoFactorSecret, DoSuggestions, DoMessageSetSeenToAll, DoMessageMove), MANQUANT = 0, SANS OBJET = 6 (DoComposeUploadExternals, DoComposeUploadDrive, DoSocialUsers, DoSocialGoogleDisconnect, DoSocialTwitterDisconnect, DoSocialFacebookDisconnect).

## À AJOUTER (MANQUANT / PARTIEL, par priorité)

1. **P2, Marquer « lu » avant de jeter en Corbeille ou Spam** (DoMessageMove). Dans `mailAction` (internal/web/mail.go, actions `delete` hors corbeille et `spam`), `StoreFlags(... FlagSeen)` avant `moveTo`. Évite des compteurs de non lus faux dans Corbeille et Spam.
2. **P3, Rafraîchir les drapeaux de la liste affichée** (DoFolderInformation) : route `/mail/flags` + mise à jour par poll.js des états lu, étoile, répondu des UID visibles quand un autre client les modifie.
3. **P3, Lire l'en-tête `X-Draft-Info` de RainLoop** (buildMessage) en repli de `X-Wmail-Draft-Info` (internal/mailview/parse.go:129) pour que les brouillons créés par RainLoop marquent bien « répondu » ou « transféré » à l'envoi. Format : `type=reply|forward; uid=N; folder=<base64>`.
4. **P3, Création automatique des dossiers système** au premier chargement (DoFolders) plutôt qu'au premier usage.
5. **P3, « Tout marquer comme non lu »** et variante limitée au fil (DoMessageSetSeenToAll).
6. **P3, Revoir le secret 2FA une fois activé**, derrière un code valide (DoShowTwoFactorSecret).
7. **P3, Limite des suggestions d'adresses à 20 et réglable** (DoSuggestions, internal/web/contactsweb.go).
8. **P3, Suppression d'un dossier non vide** : refus par défaut ou case d'accord explicite (DoFolderDelete) ; la confirmation actuelle (`folders.delete_confirm`) reste en place.
9. **P3, Content-Location et images distantes** dans le HTML sortant (buildMessage) : écart volontaire de sécurité, à ne traiter que si un besoin réel apparaît.
