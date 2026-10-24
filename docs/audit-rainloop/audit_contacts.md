# Audit fonctionnel : contacts, carnet d'adresses et autocomplétion (RainLoop 1.17.0)

Périmètre lu (lecture seule, aucun dossier `data/`) : `static/js/app.js`, `static/js/libs.js` (widget inputosaurus), `app/templates/Views/User/{PopupsContacts,SettingsContacts,PopupsCompose,MailMessageView,MailFolderList}.html`, `app/templates/Views/Admin/AdminSettingsContacts.html`, `app/libraries/RainLoop/Actions.php`, `Providers/AddressBook.php`, `Providers/AddressBook/{PdoAddressBook.php,Classes/*,Enumerations/PropertyType.php}`, `Providers/Suggestions*`, `Utils.php`, `Config/Application.php`, `localization/{webmail,admin}/fr_FR.yml`.
Abréviations de chemins : `AB` = `app/libraries/RainLoop/Providers/AddressBook`, `ACT` = `app/libraries/RainLoop/Actions.php`, `JS` = `static/js/app.js`, `TPL` = `app/templates/Views`.
Numéros de ligne : ceux du fichier source non minifié tel que lu (les lignes de `app.js` sont celles du bundle).

---------------------------------------------------------------------
## 1. Modèle de données

- Deux tables par utilisateur (clé `id_user`, entier résolu par `getUserId($email)`, email du compte parent) : `rainloop_ab_contacts` (id_contact auto-incrément, id_contact_str UUID/hex 128 car., id_user, display 255, changed timestamp Unix, deleted 0/1, etag 128) et `rainloop_ab_properties` (id_prop, id_contact, id_user, prop_type, prop_type_str 255, prop_value, prop_value_custom, prop_value_lower, prop_frec). Pas de table de tags, de groupes ni de partage (AB/PdoAddressBook.php:1752-1874, 1879-1924).
- Moteurs : SQLite (par défaut, fichier `AddressBook.sqlite` dans le dossier de données privées), MySQL, PostgreSQL ; toute autre valeur retombe sur sqlite (ACT:336-353, 9120-9123). Schéma versionné (v1 tables, v2 colonne `prop_value_lower`, v3 passage en TEXT) via `dataBaseUpgrade` (PdoAddressBook:1891-1921).
- Un contact est un sac de propriétés typées (Property: Type int, TypeStr "Home,Work,Mobile,Fax,Pref,Other", Value, ValueLower, ValueCustom, Frec). Aucun champ structuré adresse postale, société, titre, photo, catégories.
- Classes `Tag` (IdContactTag, Name, ReadOnly) existent et sont sérialisées pour le client, mais aucune table, requête ni UI ne les utilise (AB/Classes/Tag.php:1-33 ; ACT:9935-9943).
- Enum PHP `PropertyType` : UID 9, FULLNAME 10, FIRST_NAME 15, LAST_NAME 16, MIDDLE_NAME 17, NICK_NAME 18, NAME_PREFIX 20, NAME_SUFFIX 21, EMAIl 30, PHONE 31, WEB_PAGE 32, BIRTHDAY 40, FACEBOOK 90, SKYPE 91, GITHUB 92, NOTE 110, CUSTOM 250 (AB/Enumerations/PropertyType.php:7-34).
- Enum JS `ContactPropertyType` : identique, mais `MiddleName` est déclaré à 16 (valeur de LastName) au lieu de 17 (JS:520-538). Anomalie.

---------------------------------------------------------------------
## 2. Fiche contact et édition

- Pop-up « Contacts » (`View/Popup/Contacts`, gabarit `PopupsContacts`) ouverte depuis le bouton de la liste de dossiers (si contacts autorisés) ou depuis la composition (JS:34274, 12781).
- Panneau de détail : prénom + nom (toujours présents), pseudo, e-mails multiples, téléphones multiples, URL multiples (TPL/User/PopupsContacts.html:176-261).
- Menu « Ajouter » (e-mail, téléphone, URL, pseudo) ; les actions `addNewNotes` et `addNewBirthday` existent en JS mais n'ont aucune entrée de menu (JS:31010-31030 ; PopupsContacts.html:117-159). L'entrée « Adresse » est commentée.
- Types par défaut à l'ajout : e-mail = `Home`, téléphone = `Mobile`, URL sans type. Le type (TypeStr) n'est jamais modifiable dans l'UI (pas de sélecteur) (JS:31000-31013).
- Notes (type 110) rendues en `textarea` ; pseudo, notes, anniversaire regroupés dans `viewPropertiesOther` (JS:23878-23880 ; PopupsContacts.html:191-206). Anniversaire est lu/enregistré en base mais n'est pas dans le menu, n'apparaît pas dans la vue (filtre « Other » = Nick seulement, JS:30735) et n'est pas exporté.
- Types non affichés mais conservés lors d'une édition (second prénom, préfixe, suffixe, UID, anniversaire, notes) : ils sont reinjectés dans la liste de propriétés et renvoyés à la sauvegarde (JS:31140-31172, 30925-30930).
- Champ « nom complet » (FULLNAME) jamais envoyé par le client (exclu à la sauvegarde) ; il est recalculé côté serveur (JS:30927 ; ACT:7037).
- Validation du bouton « Créer/Modifier » : au moins un prénom/nom non vide OU un e-mail non vide, pas en cours d'enregistrement, contact non en lecture seule (JS:30669-30672, 30734-30747).
- Aucune validation de format d'e-mail, de téléphone ou d'URL, côté client ou serveur ; seules les valeurs vides sont écartées.
- Propriétés vides retirées à l'envoi ; lignes vides non focalisées supprimées automatiquement de la vue (JS:30757-30770, 30927).
- Indicateur « modifié » (`watchDirty`) basé sur le hash des valeurs ; bouton Enregistrer a la classe `dirty` (JS:30846-30860 ; PopupsContacts.html:162).
- Mode lecture seule : champs masqués/vides, bouton enregistrer désactivé, classe `read-only` (JS:30669 ; PopupsContacts.html:116,170). En pratique `ReadOnly` ne devient vrai que si `id_user` du contact diffère de l'utilisateur, ce qui ne peut pas arriver dans le code actuel.
- Pas de photo, d'avatar de contact dans la fiche ni dans la liste (`srcAttr()` renvoie l'image vide, jamais affichée dans le gabarit) (JS:34385-34393).
- Pas de détection de modification non enregistrée à la fermeture ni de confirmation de suppression (JS:30887-30890, 31100-31127).

---------------------------------------------------------------------
## 3. Liste, recherche, sélection

- Liste serveur triée par `display ASC`, pagination `LIMIT/OFFSET`, 50 contacts par page (constante `CONTACTS_PER_PAGE`) (PdoAddressBook:1158-1160 ; JS:7744).
- Pagination : composant Pagenator, nombre de pages = ceil(count/50), minimum 1 (JS:30710-30715 ; PopupsContacts.html:111-115).
- Côté serveur `Limit` <0 → 20 ; `Offset` <0 → 0 (ACT:6940-6943).
- Recherche (champ « Recherche ») : déclenche un rechargement à chaque modification (pas de debounce) (JS:30790-30793). Bouton « Effacer la recherche ».
- Recherche serveur : filtre sur propriétés de types e-mail, prénom, nom, pseudo, téléphone, URL ; `LIKE %terme%` avec échappement `=` de `=`, `_`, `%` ; comparaison aussi via `prop_value_lower` (mb_strtolower UTF-8) pour e-mail/noms/URL ; téléphone comparé à `prop_value_custom` (chiffres seuls, sans `+` de tête) avec le terme réduit à ses chiffres (PdoAddressBook:1069-1121, 1967-2000 ; AB/Classes/Property.php:220-234).
- Les notes, anniversaire, nom complet, second prénom ne sont pas indexés par la recherche (liste de types fixe) (PdoAddressBook:1074-1077).
- Le décompte total sans recherche = nombre distinct de contacts ayant au moins une propriété ; comme les propriétés sont effacées à la suppression, les contacts supprimés logiquement ne sont pas comptés (PdoAddressBook:1124-1139).
- `IdPropertyFromSearch` renvoyé pour chaque résultat de recherche mais inutilisé côté client (ACT:9931 ; PdoAddressBook:1185).
- Contacts supprimés logiquement (`deleted=1`) exclus des listes et lectures (PdoAddressBook:1147, 1258).
- Sélection : clic, cases à cocher (affichées selon l'option « cases à cocher » des réglages), Maj+clic, flèches haut/bas, Maj+flèches, Début/Fin, Page préc./suiv., Insert, Espace, Entrée (classe Selector générique) (JS:24630-24720, 30822-30834 ; PopupsContacts.html:74).
- Raccourcis du contexte `contact-list` : `Suppr` supprime la sélection, `c` ou `w` écrit un message aux contacts sélectionnés (JS:31236-31245).
- Suppression : ids cochés + contact sélectionné ; retrait visuel immédiat (classe `deleted`, retrait 500 ms), rechargement de liste, retour à la page 1 si la page est vidée (JS:31050-31127).
- Message vide / chargement / aucune correspondance : trois états distincts (PopupsContacts.html:77-88).
- Quand la fenêtre se ferme : recherche, liste et sélection réinitialisées (JS:31262-31275).
- Bouton « nouveau message » (enveloppe) : bascule sur la composition avec les contacts cochés/sélectionnés ; l'adresse utilisée est la première propriété e-mail rencontrée, nom = « Prénom Nom » ; les contacts sans e-mail sont ignorés (JS:34340-34358, 30864-30910).

---------------------------------------------------------------------
## 4. Import / export

- Un seul point d'entrée « Importer (csv, vcf, vCard) » : upload d'un fichier à la fois (Jua, 1 fichier, pas de glisser-déposer), action serveur `UploadContacts` (JS:31041-31072 ; ACT:7843-7906).
- Détection du format : lecture des 20 premiers octets ; si contient `BEGIN:VCARD` → VCF ; sinon s'il contient `,` ou `;` → CSV ; sinon rien n'est importé (résultat faux) (ACT:7865-7879).
- Import VCF : fichier lu en mémoire (>5 octets), BOM UTF-8 retiré, découpé par `Splitter\VCard` de Sabre (multi-contacts), UID généré si absent, chaque carte convertie par `PopulateByVCard` puis `ContactSave` ; renvoie le nombre de cartes importées (AB.php:320-376 ; ACT:7974-7997).
- Lecture vCard : FN → nom complet ; NICKNAME ; NOTE ; N (nom, prénom, second prénom, préfixe, suffixe) ; EMAIL, URL, TEL multiples avec TYPE ; UID ; les propriétés TYPE=PREF sont placées en tête (`array_unshift`) ; vCard v2.1/2.0/1.0 sans CHARSET : conversion `utf8_encode` ; nettoyage UTF-8 (AB/Classes/Contact.php:476-521, 523-661).
- Champs vCard ignorés à l'import : ADR, ORG, TITLE, BDAY, PHOTO, CATEGORIES, X-*, IMPP, ROLE, REV, etc. (aucun mapping) ; l'anniversaire n'est donc jamais importé.
- Import CSV : séparateur déduit (virgule si la première virgule précède le premier point-virgule, sinon `;`) ; `fgetcsv` longueur max de ligne 5000, guillemets `"`, locale `en_US.UTF-8` ; ligne d'en-tête obligatoire avec plus de 3 colonnes sinon 0 contact importé ; en-têtes nettoyés UTF-8 (ACT:7915-7966).
- Correspondance des colonnes CSV (nom normalisé : minuscules, espaces et tirets supprimés, ASCII uniquement) (AB.php:179-257) : Title/Name/FullName/DisplayName/GivenName → nom complet ; First/FirstName → prénom ; Middle/MiddleName → second prénom ; Last/LastName → nom ; Suffix/NameSuffix, Prefix/NamePrefix ; ShortName/NickName → pseudo ; BusinessFax(2,3)/BusinessPhone(2,3)/CompanyPhone/CompanyMainPhone/HomeFax/HomePhone/Mobile/MobilePhone/BusinessMobile/OtherFax/OtherPhone/PrimaryPhone → téléphone avec type (Work, Home, Mobile, Fax, Other, Pref) ; Email(2,3)/HomeEmail/PrimaryEmail/EmailAddress/Email2Address/PersonalEmail → e-mail `Home` ; OtherEmail → `Other` ; BusinessEmail(2,3) → `Work` ; Notes → note ; Web/WebPage/WebSite/BusinessWeb*/PersonalWebSite → URL (Work pour Business).
- Compatibilité de fait (déduite de la table de mapping, non testée) : Outlook (First Name, Middle Name, Last Name, Suffix, E-mail Address, E-mail 2/3 Address, Business/Home/Mobile Phone, Notes, Web Page) reconnu ; le champ Outlook « Title » est mappé sur le nom complet (anomalie). Thunderbird : First Name, Last Name, Display Name, Nickname, Primary Email, Home Phone reconnus ; Secondary Email, Work Phone, Fax Number, Mobile Number, Pager Number non reconnus. Gmail/Google : « Name » et « Given Name » reconnus (tous deux → nom complet) ; « Family Name », « E-mail 1 - Value », « Phone 1 - Value » non reconnus (normalisés en `email1value`, `phone1value`), donc aucun e-mail ni téléphone importé depuis un export Google CSV. Colonnes non reconnues ignorées silencieusement ; en-têtes non ASCII (localisés) jamais reconnus.
- Colonnes adresse postale, société, titre, anniversaire jamais importées en CSV.
- Valeurs de cellule vides ignorées ; un contact CSV sans propriété reconnue n'est pas créé (AB.php:275-303).
- Aucune détection de doublons à l'import (ni VCF ni CSV) : chaque ligne/carte crée un contact (nouveau id_contact_str), même si un contact identique existe. Un VCF réimporté avec un UID déjà présent crée quand même un nouveau contact.
- Réponse d'import : nombre entier ; le client alerte « Erreur d'importation (format de fichier invalide) » si le résultat est faux ou 0 (JS:31062-31070 ; ACT:7905 ; fr_FR CONTACTS/ERROR_IMPORT_FILE).
- Erreurs d'upload (taille, partiel, etc.) renvoyées via `getUploadErrorMessageByCode` (ACT:7894-7903). Limites réelles = `upload_max_filesize`/`post_max_size` PHP.
- Export VCF : un fichier `contacts.vcf` concaténant des vCard 3.0 ; export CSV : `contacts.csv` avec une ligne d'en-tête (34 colonnes) puis une ligne par contact (AB.php, ACT:8902-8933 ; PdoAddressBook:824-862). Téléchargement via `/Raw/.../ContactsVcf/` et `/ContactsCsv/` ; menu d'export masqué sur appareils mobiles (`allowExport = !bMobileDevice`) (JS:6398-6407, 30700).
- vCard exportée : VERSION 3.0, PRODID `-//RainLoop//<version>//EN`, FN, NICKNAME, NOTE, N (nom;prénom;second;préfixe;suffixe), EMAIL/URL/TEL avec paramètres TYPE, UID, REV UTC ; anniversaire, réseaux sociaux, propriétés custom non exportés (Contact.php:201-310).
- Export CSV : colonnes fixes Title, First Name, Middle Name, Last Name, Nick Name, Display Name, Company, Department, Job Title, Office Location, E-mail Address, Notes, Web Page, Birthday, Other Email, Other Phone, Other Mobile, Mobile Phone, Home Email, Home Phone, Home Fax, Home Street/City/State/Postal Code/Country, Business Email/Phone/Fax/Street/City/State/Postal Code/Country. Seules sont renseignées : prénom, second prénom, nom, pseudo, nom affiché, e-mail principal, notes, web, e-mails (Other/Work/Home), téléphones (Other/Work/Mobile/Home). Champs société, adresses, anniversaire, fax toujours vides. Une seule valeur par colonne (la dernière rencontrée écrase) ; l'e-mail principal prend Home, sinon Work, sinon Other (Contact.php:315-469).
- Échappement CSV : champ entre guillemets si contient CR/LF, virgule ou guillemet ; guillemets doublés ; fin de ligne CRLF ; pas de BOM ; séparateur virgule fixe (Contact.php:457-466).
- Les contacts supprimés ne sont jamais exportés (PdoAddressBook:842).

---------------------------------------------------------------------
## 5. Synchronisation CardDAV

- Fonction optionnelle : activée par l'administrateur (`contacts.allow_sync`, défaut faux) ET contacts actifs ET classe Sabre disponible ; configurée par l'utilisateur dans Paramètres > Contacts (ACT:1595-1613 ; Config/Application.php:203).
- Paramètres utilisateur : activer, URL du carnet, utilisateur, mot de passe. Stockés par compte (`StorageType::CONFIG`, clé `contacts_sync`) sous forme encodée (EncodeKeyValues) ; le mot de passe est renvoyé au client sous forme du marqueur `APP_DUMMY` et conservé si le marqueur est renvoyé (ACT:6577-6636).
- Enregistrement automatique des réglages de synchro à chaque modification (throttle 500 ms) (JS:36950-36973).
- Déclenchement : 10 s après le chargement de l'interface puis toutes les `sync_interval` minutes (+5 s), valeur serveur par défaut 20, bornée côté client entre 5 et 320 ; bouton manuel « Synchroniser » dans le menu de la fenêtre contacts (visible si synchro active et autorisée) ; verrou `syncing`/`importing` (JS:29586-29600, 28604-28618 ; PopupsContacts.html:57-64 ; Config:204).
- Délai max de la requête de synchro 200 s (JS:7749).
- Erreur : `ContactsSyncError` (140) → « Erreur de synchronisation des contacts » affiché en `alert`, puis rechargement de la liste (ACT:6658-6661 ; JS:30962-30970).
- Normalisation de l'URL : sans schéma, préfixes `fruux.com`→`dav.fruux.com`, `icloud.com`→`contacts.icloud.com`, `gmail.com`→`google.com`, https pour google/fruux/icloud, sinon http ; accepte « URL|Nom du carnet » pour choisir un carnet parmi plusieurs ; port par défaut 80 (même pour https sans port, d'où baseUri sans port) ; vérification du certificat TLS désactivée (`setVerifyPeer(false)`) ; proxy possible (PdoAddressBook:504-554, 565-656).
- Découverte : PROPFIND sur le chemin donné ; si vide ou non carnet, `/.well-known/carddav`, `current-user-principal`, `addressbook-home-set`, parcours des collections `carddav:addressbook` ; sélection du carnet : par nom fourni, sinon nom parmi contacts/default/addressbook/address book, sinon le premier (PdoAddressBook:262-446, 597-643).
- Authentification : identifiants Basic/Digest gérés par le client Sabre (utilisateur, mot de passe saisis) ; le mot de passe est ajouté aux secrets du journal (PdoAddressBook:539).
- Algorithme (PdoAddressBook:667-816) : (1) supprime côté distant les contacts supprimés localement ; (2) supprime localement ceux qui ont un etag mais n'existent plus à distance ; (3) purge physique des contacts `deleted=1` ; (4) envoie (PUT) les contacts nouveaux (sans etag, absents à distance) ou plus récents localement (etag différent et date distante < date locale) en fusionnant avec la vCard distante existante (GET) afin de préserver les champs non gérés ; (5) récupère (GET) les contacts nouveaux ou plus récents à distance (etag différent et date locale < date distante) et les enregistre.
- Gestion des conflits : « dernier modifié gagne » par comparaison d'horodatages (`changed` local vs `getlastmodified`/date de l'etag distant) ; pas de fusion champ à champ ; si les etags diffèrent et que les dates sont égales, rien n'est transféré.
- Nom de fichier distant `<id_contact_str>.vcf` ; ETag mémorisé après PUT (en-tête `etag`), date = en-tête `Date` ; Content-Type `text/vcard; charset=utf-8` (Contact.php:193-196 ; PdoAddressBook:184-221, 754-767).
- Après une modification locale d'un contact déjà synchronisé, l'etag local est recalculé `md5(vCard)` pour forcer la comparaison (ACT:7032-7035).
- Pas de suivi par sync-token / ctag, pas d'ETag conditionnel (If-Match), pas de gestion de plusieurs carnets distants, pas de synchro partielle ; tous les contacts distants sont listés à chaque exécution.

---------------------------------------------------------------------
## 6. Partage

- Fonction de partage non active : `IsSharingAllowed()` vaut toujours faux (`&& false; // TODO`) (PdoAddressBook:53-56) ; la méthode d'interface est exposée dans AddressBook.php:59-63 mais jamais appelée par l'UI.
- Vestiges : icône « partage » dans chaque ligne de la liste (`shareParent`, visuel seulement, la même zone sert de poignée de sélection) (PopupsContacts.html:98-100) ; attribut `ReadOnly`/`IdPropertyFromSearch` ; mode CSS `read-only`. Aucun contact en lecture seule ne peut être créé aujourd'hui.
- Comptes additionnels : le carnet est rattaché à l'e-mail du compte parent (`ParentEmailHelper`), donc partagé entre un compte principal et ses comptes additionnels (ACT:6952, 6978 ; Model/Account.php:158-161).
- Suppression d'un compte : `DeleteAllContacts` purge toutes les tables de l'utilisateur (Api.php:244-248 ; PdoAddressBook:1029-1044).

---------------------------------------------------------------------
## 7. Fréquence et autocomplétion

- Composant d'adresse = widget jQuery « inputosaurus » (libs.js) lié par le binding Knockout `emailsTags` + `autoCompleteSource` (JS:1430-1493 ; PopupsCompose.html:117,125,133,141). Champs : À, Cc, Cci, Répondre à.
- Délimiteurs de saisie `,` `;` et saut de ligne ; analyse à la perte de focus ; glisser-déposer des étiquettes entre champs ; suppression par Retour arrière/Suppr ; édition par `e`/Entrée ; navigation flèches ; doublons empêchés ; l'analyse utilise `emailjs-addressparser` (formes `"Nom" <a@b>` acceptées) (JS:1445-1470, 10555-10600 ; libs.js).
- Autocomplétion jQuery UI : `minLength: 1`, délai par défaut 300 ms, `autoFocus: true`, menu aligné sur le conteneur ; la sélection insère `"Nom" <email>` ; pas de mise en évidence ni d'avatar dans les suggestions (libs.js inputosaurus `_instAutocomplete`).
- Source : `emailsSource` → `getAutocomplete(term)` → requête AJAX `Suggestions` (paramètres `Query`, `Page` non utilisée) ; les résultats `[email, nom]` sont convertis en `EmailModel` puis `toLine(false)` ; les requêtes abandonnées ne vident pas la liste (JS:12831-12836, 29236-29247, 7460-7465).
- Serveur `DoSuggestions` : limite = `contacts.suggestions_limit` (défaut 30 dans Application.php, 20 dans le code), minimum forcé à 5 ; hooks de plugins `ajax.suggestions-input-parameters`, `-pre`, `-post` ; ordre : plugins → carnet d'adresses → fournisseurs (ownCloud) ; déduplication `email~~nom` ; tronqué à la limite (ACT:7052-7125 ; Utils.php:604-623 ; Config:209).
- Carnet : `GetSuggestions` cherche dans e-mail, prénom, nom, pseudo (LIKE + version minuscule), triés par fréquence décroissante `prop_frec` (tri sur la propriété trouvée), `LIMIT` ; construit des paires `[email, nom]` : nom = « Prénom Nom » si la recherche a touché un nom, pseudo si seulement le pseudo, vide si seulement l'e-mail ; si la recherche a touché seulement des e-mails, tous les e-mails de la fiche ne sont pas listés, seuls ceux dont la propriété correspond (PdoAddressBook:1358-1551). Bug mineur : `$aNicks` non initialisé si aucun pseudo (accès tableau non défini).
- L'ordre « Prénom Nom » est figé (pas de « Nom, Prénom »). Les téléphones et URL ne sont pas recherchés pour l'autocomplétion.
- Fournisseurs de suggestions externes : interface `ISuggestions::Process`, implémentation `OwnCloudSuggestions` (recherche FN, NICKNAME, TITLE, EMAIL via le gestionnaire de contacts ownCloud, actif seulement si connecté à ownCloud et `labs.owncloud_suggestions`), `TestSuggestions` (exemple factice, non branché) (Providers/Suggestions/*.php ; ACT:354-366).
- Fréquence (`prop_frec`) : incrémentée de 1 pour chaque e-mail destinataire (À) à l'envoi réussi ; conservée lors de la mise à jour d'un contact (copie par valeur d'e-mail via `getContactFreq`) (PdoAddressBook:1700-1718, 1931-1959, 945-952).
- Seuls les destinataires « À » sont pris en compte pour la fréquence ; Cc et Cci ignorés (ACT:6449-6458).
- Aucune décroissance temporelle, pas de plafond, pas de remise à zéro, pas de tri de la liste de contacts par fréquence.
- Clé SQL de mise à jour fréquence : `prop_value = <e-mail>` (sensible à la casse sur certains moteurs ; l'e-mail est en casse d'origine tandis que la base stocke le domaine en minuscule) (PdoAddressBook:1700-1713).
- Chaîne « SUGGESTIONS/SEARCHING_DESC » (« Recherche... ») présente dans les traductions mais non référencée dans le JS (fr_FR.yml:163-164).

---------------------------------------------------------------------
## 8. Ajout automatique

- Réglage utilisateur « Ajouter automatiquement les destinataires à votre carnet d'adresses » (`ContactsAutosave`), valeur par défaut serveur `defaults.contacts_autosave = true` ; modifiable dans Paramètres > Contacts ; enregistré via `saveSettings` (JS:36966-36969 ; Config:300 ; ACT:1847, 1883, 4999).
- Déclenché dans `DoSendMessage` après envoi réussi, uniquement si le carnet est actif ; exceptions journalisées sans faire échouer l'envoi (ACT:6445-6474).
- Traitement `IncFrec($email, $destinataires_À, $autosave)` : analyse chaque destinataire `Nom <email>` avec `Mime\Email::Parse` (échecs ignorés) ; exception « Empty Emails argument » si aucun valide (PdoAddressBook:1560-1580).
- Création : pour chaque destinataire dont l'e-mail (minuscule, ASCII seulement) n'existe pas déjà dans les propriétés e-mail de l'utilisateur, crée un contact avec propriété e-mail ; si un nom d'affichage existe, le coupe au premier espace : prénom = avant, nom = reste ; sinon prénom = tout (PdoAddressBook:1589-1698).
- Pas de création pour Cc/Cci. Pas de confirmation, pas de lien « ajouter au carnet » dans la lecture d'un message (MailMessageView.html n'a aucun lien contact).
- Bug probable : la comparaison `in_array($sEmail, $aExists)` compare l'e-mail en casse d'origine à des valeurs mises en minuscule : un e-mail avec majuscules dans la partie locale est recréé à chaque envoi (PdoAddressBook:1606, 1612-1624).
- Si l'ajout automatique est désactivé, seule la fréquence des contacts existants est incrémentée (PdoAddressBook:1626-1639).
- Le contact créé reçoit un nom complet dérivé (prénom nom, sinon e-mail) par `PopulateDisplayAndFullNameValue` (voir F-CT-12).

---------------------------------------------------------------------
## 9. Intégration composition / lecture

- Bouton « Contacts » dans la barre de composition (si contacts autorisés) : sauvegarde le brouillon (mémorise), ouvre la pop-up avec `backToCompose = true` et le dernier champ ayant eu le focus (À, Cc ou Cci) ; à la fermeture, rouvre la composition (JS:12781-12790, 12432-12446, 31262-31275 ; PopupsCompose.html:88).
- Depuis la liste des contacts, « nouveau message » ouvre la composition avec les destinataires dans le champ mémorisé (À par défaut) ; si une composition est déjà en cours, les adresses sont ajoutées à la valeur existante, séparées par `, ` (JS:13107-13160, 30880-30910).
- Bouton « Contacts » dans la liste de dossiers : visible si `ContactsIsAllowed` (MailFolderList.html:9 ; JS:29225-29229).
- Capacité `CONTACTS` imbriquée sous `COMPOSER` : si la composition est désactivée, les contacts le sont aussi (ACT:8221-8227, 971).
- Pastille / photo de l'expéditeur dans l'affichage d'un message : image `fromPic` ; l'URL est `/Raw/0/Avatar/<email>/` seulement si la capacité Gravatar est active ; sinon point gris par défaut (JS:5819-5840, 35800-35812 ; MailMessageView.html:239 ; ACT:8239-8242).
- Serveur `RawAvatar` : valide `x@y`, passe l'e-mail en ASCII (IDN), requête `http://gravatar.com/avatar/<md5(email en minuscule)>.jpg?s=80&d=404` (HTTP, délai 5 s, proxy curl configurable) ; accepte jpeg/jpg/png avec code 200 ; sinon icône du domaine (38 images `resources/images/services/<domaine>.png`, ex. google.com, github.com, paypal) ; sinon `empty-contact.png` ; cache HTTP (ACT:8844-8897). Ne tient pas compte de la photo d'un contact du carnet.
- Aucun affichage du nom du contact à la place de l'adresse dans la liste de messages/affichage ; aucune pastille dans la liste de messages ; aucun lien « ajouter aux contacts » ni « modifier le contact » dans la lecture.
- Recherche avancée de messages : champs De/À/Sujet sans autocomplétion du carnet (PopupsAdvancedSearch.html:19-46).
- Identités et réglages : champs d'adresse (identité, Répondre à) sans autocomplétion.
- Étiquettes d'adresse sérialisées en `"Nom" <email>` ; `EmailModel.clearDuplicateName` retire le nom s'il est identique à l'e-mail ; `validate()` accepte nom OU e-mail non vide (JS:10401-10540).

---------------------------------------------------------------------
## 10. Réglages (utilisateur et administrateur)

- Utilisateur (Paramètres > Contacts, visible seulement si les contacts sont autorisés) : case « ajout automatique » ; bloc CardDAV (activer, URL, utilisateur, mot de passe) visible seulement si l'administrateur autorise la synchro (JS:36622 ; SettingsContacts.html:275-337).
- Administrateur (onglet Contacts) : activer les contacts (`contacts.enable`, défaut faux), autoriser la synchro (`allow_sync`), type de stockage sqlite/mysql/pgsql, DSN PDO, utilisateur, mot de passe (masqué par marqueur), bouton « Test » (`DoAdminContactsTest`, renvoie le message d'erreur PDO) ; avertissement « ne pas utiliser sqlite avec de nombreux utilisateurs » ; message si l'extension PDO manque (AdminSettingsContacts.html:1-111 ; ACT:3817-3824, 3935-3955 ; PdoAddressBook:1724-1750).
- Options de configuration (Application.php:201-210, 245, 300, 405, 458) : `contacts.enable`, `allow_sync`, `sync_interval` (20), `type` (sqlite), `pdo_dsn`, `pdo_user`, `pdo_password`, `suggestions_limit` (30) ; `capa.contacts` (true) ; `defaults.contacts_autosave` (true) ; `labs.allow_gravatar` (true) ; `labs.owncloud_suggestions` (true).
- Administrateur (Général) : case « Autoriser Gravatar » (ACT:3724-3725, 3833 ; AdminSettingsGeneral.html:115).
- Mode statique : `ContactsIsAllowed` (fournisseur actif) conditionne l'UI ; `ContactsSyncIsAllowed`, `ContactsSyncInterval`, `EnableContactsSync`, `ContactsSyncUrl`, `ContactsSyncUser`, `ContactsSyncPassword` fournis dans la config de l'application (ACT:1594-1612).

---------------------------------------------------------------------
## 11. Sécurité et limites

- Toutes les actions utilisateur vérifient le jeton (`getAccountFromToken`) ; les requêtes SQL utilisent des paramètres liés ; seul le tri/id est interpolé après conversion en entiers (`array_map('intval')`) (ACT:6970-6973 ; PdoAddressBook:999-1010, 1199).
- Suppression logique puis purge à la synchro seulement ; si la synchro est désactivée les lignes `deleted=1` restent indéfiniment dans `rainloop_ab_contacts` (PdoAddressBook:58-63, 723, 1017).
- `DeleteContacts` supprime d'abord les propriétés (perte irréversible, pas de corbeille).
- Aucune limite du nombre de propriétés par contact, de la taille d'une valeur (colonne TEXT en MySQL, varchar(255) pour `display`), du nombre de contacts, ni de la taille d'un import hors limites PHP et 5000 caractères par ligne CSV.
- Valeur d'un seul `Etag` md5 ; collisions improbables.
- Mot de passe CardDAV stocké réversiblement (encodage de l'application), vérification TLS désactivée, URL fournie par l'utilisateur (risque SSRF vers le réseau interne : le serveur effectue des PROPFIND/GET/PUT sur l'hôte saisi) (PdoAddressBook:541-549, ACT:6616-6633).
- Gravatar : requête en clair `http://` vers un service tiers contenant le hash de l'adresse de chaque expéditeur consulté ; contrôlé par `allow_gravatar`.
- Sérialisation JSON : valeurs nettoyées UTF-8 ; `Property` de type Web : correction `\://` → `://` (ACT:9946-9950).
- Rendu : valeurs affichées avec `text:`/`value:` Knockout (échappement HTML) ; infobulles et noms d'étiquettes d'adresse échappés via `encodeHtml` (PopupsContacts.html, JS:10475-10520).
- La saisie de recherche est échappée pour LIKE (`=` comme caractère d'échappement) mais sans normalisation d'accents (comparaison sensible aux accents hors casse).
- Pas de journal d'audit, pas de quotas, pas de limitation de débit sur les suggestions (une requête AJAX par frappe après 300 ms).

---------------------------------------------------------------------
# Liste numérotée des fonctionnalités

F-CT-01 : Activation globale du carnet d'adresses — le fournisseur n'est créé que si la capacité `CONTACTS` est présente (liée à `capa.contacts` ET `capa.composer`) et que `contacts.enable` est vrai ; sinon toutes les actions renvoient faux/liste vide — ACT:966-983, 8221-8227 ; Config/Application.php:202, 245
F-CT-02 : Pilotes de stockage PDO — SQLite par défaut (fichier privé `AddressBook.sqlite`), MySQL, PostgreSQL ; valeur inconnue → sqlite — ACT:336-353, 9120-9123 ; PdoAddressBook.php:44-48
F-CT-03 : Schéma versionné des tables — `rainloop_ab_contacts` et `rainloop_ab_properties`, migrations v1→v3 automatiques — PdoAddressBook.php:1752-1924
F-CT-04 : Test de configuration de la base (admin) — bouton Test, renvoie succès ou message d'erreur PDO, pause 1 s — ACT:3935-3955 ; AddressBook.php:31-36 ; PdoAddressBook.php:1724-1750
F-CT-05 : Message « PDO non supporté » (admin) — bloc d'avis si l'extension PDO/driver est absente — AdminSettingsContacts.html:2-8 ; PdoAddressBook.php:44-48
F-CT-06 : Contact = liste de propriétés typées — propriétés (type, type libellé, valeur, valeur minuscule, valeur personnalisée, fréquence) rattachées à un id_contact et un id_user — PdoAddressBook.php:1774-1790 ; Classes/Property.php:77-134
F-CT-07 : Identifiant double — id numérique base + UUID (`id_contact_str`, UUID Sabre ou md5 aléatoire) servant de nom de fichier vCard — Contact.php:57-67, 167-171, 193-196
F-CT-08 : Types de propriétés pris en charge en base — UID, nom complet, prénom, nom, second prénom, pseudo, préfixe, suffixe, e-mail, téléphone, URL, anniversaire, Facebook, Skype, GitHub, note, personnalisé — Enumerations/PropertyType.php:7-34
F-CT-09 : Types réellement éditables dans l'UI — prénom, nom, pseudo, e-mail, téléphone, URL ; notes et anniversaire sans entrée de menu ; second prénom, préfixe, suffixe, réseaux sociaux, custom non éditables — PopupsContacts.html:117-159 ; app.js:31010-31030
F-CT-10 : Anomalie d'énumération JS — `MiddleName` vaut 16 (comme `LastName`) côté JS — app.js:520-538
F-CT-11 : Type de propriété (TypeStr) — liste séparée par virgules (Home, Work, Mobile, Fax, Other, Pref), espaces supprimés, majuscules pour comparaison ; défauts à l'ajout : e-mail Home, téléphone Mobile — Property.php:177-198 ; app.js:31001-31013
F-CT-12 : Règle du nom affiché (Display) — priorité : nom complet existant (sauf mode forcé) → « Prénom Nom » → nom complet → premier e-mail → premier téléphone ; ajoute/maj une propriété FULLNAME avec ce résultat ; `DoContactSave` force le recalcul (ignore le nom complet fourni) — Contact.php:69-152 ; ACT:7037
F-CT-13 : Normalisation d'une propriété — valeurs rognées ; domaine d'e-mail passé en minuscule (ASCII) ; noms : espaces multiples réduits ; valeur minuscule (UTF-8) pour e-mail/nom/URL ; téléphone : chiffres seuls sans `+` de tête dans `prop_value_custom` — Property.php:200-236
F-CT-14 : Création/édition d'un contact — `ContactSave` avec RequestUid (anti-course), Uid optionnel ; remplace toutes les propriétés (suppression + réinsertion) ; renvoie l'id ; met `Changed` à l'heure courante — ACT:6987-7047 ; PdoAddressBook.php:871-982
F-CT-15 : Propriétés acceptées à la sauvegarde — tableau `[type numérique, valeur, typeStr]` ; entrées non numériques ou sans valeur écartées — ACT:7015-7030
F-CT-16 : Conservation de la fréquence à l'édition — la fréquence de chaque e-mail est reportée sur les nouvelles propriétés — PdoAddressBook.php:889-892, 945-952, 1931-1959
F-CT-17 : Validation de la fiche — bouton actif seulement si prénom, nom ou e-mail non vide ; pas de validation de format — app.js:30669-30672, 30734-30747
F-CT-18 : Retour visuel d'enregistrement — états animé / réussi / échec, retour au repos après 1 s — app.js:30921-30958
F-CT-19 : Nouveau contact — vide la fiche, sélection supprimée, prénom focalisé — app.js:31140-31172 ; 31186-31190 (newCommand)
F-CT-20 : Ajout dynamique de lignes — e-mail, téléphone, URL multiples ; pseudo/notes/anniversaire uniques (focus si présent) ; lignes vides non focalisées retirées automatiquement — app.js:30735-30790, 30996-31030
F-CT-21 : Champ note multi-lignes — type 110 rendu dans un textarea — ContactPropertyModel (app.js:23878-23880) ; PopupsContacts.html:199-204
F-CT-22 : Anniversaire — stocké comme propriété 40 si présent, jamais affiché, ni exporté/importé — PropertyType.php:26 ; Contact.php:253-300
F-CT-23 : Suivi des modifications non enregistrées — hash des valeurs → classe `dirty` sur le bouton — app.js:30846-30860 ; PopupsContacts.html:162
F-CT-24 : Mode lecture seule (vestige) — propriété `ReadOnly` (id_user différent) désactive le bouton et masque les liens d'ajout — Classes/Contact.php:40 ; PdoAddressBook.php:1183 ; app.js:30669, 30850-30852
F-CT-25 : Liste triée par nom affiché — `ORDER BY display ASC` (casse/accents selon la collation du moteur) — PdoAddressBook.php:1158
F-CT-26 : Pagination 50 par page — Offset/Limit, pager client ; Limit invalide → 20 — app.js:7744, 30710-30717, 31182-31220 ; ACT:6940-6943
F-CT-27 : Recherche texte — LIKE `%terme%` échappé sur e-mail, prénom, nom, pseudo, téléphone, URL ; insensible à la casse via `prop_value_lower` — PdoAddressBook.php:1069-1121, 1967-1989
F-CT-28 : Recherche par téléphone — termes réduits aux chiffres, comparés à `prop_value_custom` — PdoAddressBook.php:1083, 1996-2000
F-CT-29 : Champs non recherchables — notes, anniversaire, nom complet, second prénom — PdoAddressBook.php:1074-1077
F-CT-30 : Recherche réactive — chaque frappe recharge la liste ; effacement par lien ; états vide/chargement/aucun résultat — app.js:30790-30793 ; PopupsContacts.html:77-88
F-CT-31 : Comptage total — nombre de contacts (ou de résultats de recherche) pour calculer les pages — PdoAddressBook.php:1118-1142
F-CT-32 : Sélection simple et multiple — clic, cases, Maj+clic, clavier (flèches, Début, Fin, Page, Espace, Insert, Entrée) ; cases masquables selon réglage « cases à cocher » — app.js:24630-24720 ; PopupsContacts.html:74
F-CT-33 : Raccourcis dans la liste — `Suppr` = supprimer ; `c` ou `w` = nouveau message vers la sélection — app.js:31236-31245
F-CT-34 : Suppression multiple — ids transmis par virgules, convertis en entiers ; propriétés effacées, contact marqué `deleted=1` — ACT:6968-6982 ; PdoAddressBook.php:991-1021
F-CT-35 : Retour d'interface après suppression — animation, recharge, retour page 1 si page vide — app.js:31072-31127
F-CT-36 : Suppression de tous les contacts d'un utilisateur — à la suppression du compte — Api.php:244-248 ; PdoAddressBook.php:1029-1044
F-CT-37 : Nouveau message depuis la sélection — première adresse e-mail de chaque contact, nom « Prénom Nom » ; ouvre la composition dans le champ À/Cc/Cci mémorisé — app.js:34340-34358, 30864-30910
F-CT-38 : Aucune sélection de contact sans e-mail — contacts sans e-mail ignorés lors de l'envoi groupé — app.js:34340-34358
F-CT-39 : Import par fichier unique — bouton « Importer (csv, vcf, vCard) », un fichier à la fois, indicateur d'activité — app.js:31041-31072 ; PopupsContacts.html:35-41
F-CT-40 : Détection de format à l'import — `BEGIN:VCARD` → VCF ; virgule/point-virgule → CSV ; autre → rejet — ACT:7865-7879
F-CT-41 : Import vCard multi-contacts — découpage Sabre, BOM retiré, UID généré si absent — AddressBook.php:320-376
F-CT-42 : Lecture vCard — FN, NICKNAME, NOTE, N (5 composants), EMAIL, URL, TEL avec TYPE, UID ; TYPE=PREF en tête ; versions 1.0/2.0/2.1 converties en UTF-8 — Contact.php:476-661
F-CT-43 : vCard : champs perdus à l'import — ADR, ORG, TITLE, BDAY, PHOTO, CATEGORIES, etc. non conservés — Contact.php:573-655
F-CT-44 : Import CSV — détection du séparateur, en-tête obligatoire (>3 colonnes), lecture `fgetcsv` — ACT:7915-7966
F-CT-45 : Table de correspondance CSV — en-têtes normalisés (minuscule, sans espaces ni tirets) vers type et TypeStr ; couvre Outlook, Thunderbird, Gmail partiellement — AddressBook.php:179-257
F-CT-46 : CSV : colonnes inconnues ignorées — adresses, société, anniversaire, e-mail/téléphone Google (`E-mail 1 - Value`), Thunderbird (Work Phone, Secondary Email), en-têtes non ASCII ; « Title » mappé sur nom complet — AddressBook.php:255-256, 285
F-CT-47 : CSV : un contact par ligne — ligne sans propriété reconnue ignorée ; nombre importé renvoyé — AddressBook.php:265-312
F-CT-48 : Pas de détection de doublons à l'import — chaque entrée crée un contact, même UID/e-mail déjà présent — AddressBook.php:299, 363 ; PdoAddressBook.php:871-940
F-CT-49 : Gestion d'erreur d'import — retour faux/0 → alerte « Erreur d'importation (format de fichier invalide) » ; erreurs d'upload traduites — app.js:31062-31070 ; ACT:7894-7905
F-CT-50 : Export vCard — `contacts.vcf` (vCard 3.0 concaténées) — ACT:8902-8915 ; Contact.php:201-310 ; PdoAddressBook.php:824-862
F-CT-51 : Contenu des vCard exportées — FN, N, NICKNAME, NOTE, EMAIL/URL/TEL avec TYPE, UID, REV, PRODID — Contact.php:242-307
F-CT-52 : Export CSV — `contacts.csv`, en-tête de 34 colonnes style Outlook, une ligne par contact — ACT:8920-8933 ; Contact.php:315-469
F-CT-53 : Règles d'export CSV — mapping type+TypeStr vers colonnes (Home/Work/Other email, Other/Work/Mobile/Home phone), e-mail principal de repli, échappement RFC4180 simplifié, CRLF — Contact.php:371-466
F-CT-54 : Export masqué sur mobile — `allowExport = !bMobileDevice` — app.js:30700 ; PopupsContacts.html:42-56
F-CT-55 : Synchro CardDAV optionnelle — double autorisation (admin + utilisateur), réglages par compte — ACT:1595-1613, 6577-6664
F-CT-56 : Réglages CardDAV utilisateur — activer, URL, utilisateur, mot de passe ; sauvegarde automatique ; mot de passe masqué — SettingsContacts.html:293-336 ; app.js:36950-36973 ; ACT:6606-6636
F-CT-57 : Normalisation de l'URL CardDAV — raccourcis fruux/iCloud/Gmail, schéma automatique, suffixe `|nom du carnet` — PdoAddressBook.php:504-554, 572-583
F-CT-58 : Découverte automatique du carnet — `.well-known/carddav`, principal, home-set, collections addressbook ; choix par nom, par noms usuels ou premier — PdoAddressBook.php:262-446, 597-643
F-CT-59 : Algorithme de synchro bidirectionnelle — suppressions, créations, mises à jour dans les deux sens par comparaison d'etag et de date — PdoAddressBook.php:667-816
F-CT-60 : Résolution des conflits — « le plus récent gagne » (dates `changed`), pas de fusion de champs ; fusion avec la vCard distante préservant les champs inconnus lors du PUT — PdoAddressBook.php:726-773, 776-813
F-CT-61 : Planification de la synchro — 10 s après le chargement puis toutes les 5-320 min (défaut 20) ; bouton manuel ; verrous importing/syncing — app.js:29586-29600, 28604-28618, 30962-30970
F-CT-62 : Erreur de synchro — notification 140 « Erreur de synchronisation des contacts » — Notifications.php:27 ; ACT:6658-6661
F-CT-63 : Sécurité TLS réduite — vérification de certificat désactivée pour CardDAV — PdoAddressBook.php:547
F-CT-64 : Suppression logique pour la synchro — `deleted=1`, propagation distante, puis purge physique — PdoAddressBook.php:58-63, 696-723
F-CT-65 : Mise à jour d'etag après PUT — etag et date de la réponse stockés — PdoAddressBook.php:65-75, 758-767
F-CT-66 : Recalcul d'etag à l'édition d'un contact synchronisé — `md5(vCard)` — ACT:7032-7035
F-CT-67 : Partage inactif — `IsSharingAllowed()` renvoie toujours faux ; icône de partage décorative — PdoAddressBook.php:53-56 ; PopupsContacts.html:98-100
F-CT-68 : Carnet mutualisé avec les comptes additionnels — clé d'utilisateur = email du compte parent — ACT:6952, 6978
F-CT-69 : Tags/groupes — classe Tag sérialisée mais aucune persistance ni UI — Classes/Tag.php ; ACT:9935-9943
F-CT-70 : Suggestions depuis le carnet — recherche e-mail/prénom/nom/pseudo triée par fréquence décroissante, limitée — PdoAddressBook.php:1358-1551
F-CT-71 : Règle du nom proposé — « Prénom Nom » si nom présent dans la fiche (premier/dernier), sinon pseudo, sinon vide — PdoAddressBook.php:1490-1534
F-CT-72 : Plusieurs e-mails par fiche — chaque e-mail correspondant donne une suggestion — PdoAddressBook.php:1477-1534
F-CT-73 : Action AJAX Suggestions — Query requise ; limite configurable (min. 5) ; fusion plugins/carnet/fournisseurs ; déduplication ; troncature — ACT:7052-7125
F-CT-74 : Hooks de plugins sur les suggestions — `ajax.suggestions-input-parameters`, `-pre`, `-post` — ACT:7061, 7069, 7116
F-CT-75 : Fournisseur ownCloud/Nextcloud — recherche FN/NICKNAME/TITLE/EMAIL dans les contacts du serveur, dédoublonnage par `"nom" <email>` — Suggestions/OwnCloudSuggestions.php ; ACT:361-365
F-CT-76 : Fournisseur de test — exemple fixe, non utilisé — Suggestions/TestSuggestions.php
F-CT-77 : Widget d'adresses (étiquettes) — À, Cc, Cci, Répondre à ; délimiteurs `, ; \n`, parse à la perte de focus, doublons évités, glisser-déposer, édition — app.js:1430-1493 ; PopupsCompose.html:117-141 ; libs.js (inputosaurus)
F-CT-78 : Paramètres de l'autocomplétion — `minLength` 1, délai 300 ms (défaut jQuery UI), `autoFocus` — libs.js (inputosaurus `_instAutocomplete`)
F-CT-79 : Sélection d'une suggestion — insère `"Nom" <email>` converti en étiquette — app.js:12831-12836 ; libs.js
F-CT-80 : Conversion Email — `toLine(false)` = `"nom" <adresse>` ou adresse seule ; nom identique à l'adresse supprimé — app.js:10401-10540
F-CT-81 : Séparation d'une ligne d'adresses — addressparser tolérant (`Nom <a@b>`, `a@b`) — app.js:10543-10575
F-CT-82 : Fréquence d'usage — +1 à chaque envoi pour chaque destinataire « À » présent dans le carnet — PdoAddressBook.php:1700-1718 ; ACT:6449-6468
F-CT-83 : Cc/Cci exclus de la fréquence et de l'auto-création — ACT:6450-6458
F-CT-84 : Ajout automatique des destinataires — réglage utilisateur (défaut vrai), création de contacts pour e-mails inconnus avec prénom/nom déduits — PdoAddressBook.php:1589-1698 ; Config/Application.php:300
F-CT-85 : Déduction prénom/nom — premier espace du nom d'affichage ; un seul mot = prénom — PdoAddressBook.php:1657-1688
F-CT-86 : Anomalie de casse dans la détection d'existant — comparaison avec e-mails en minuscule : doublons possibles — PdoAddressBook.php:1606, 1619
F-CT-87 : Échec silencieux de l'auto-ajout — exceptions journalisées sans impact sur l'envoi — ACT:6471-6474
F-CT-88 : Lien « Contacts » de la liste de dossiers — visible si carnet actif — MailFolderList.html:9 ; app.js:29225-29229
F-CT-89 : Bouton Contacts en composition — sauvegarde brouillon, ouvre la pop-up, mémorise le champ actif, revient à la composition — app.js:12781-12790, 12432-12446, 31262-31275
F-CT-90 : Insertion dans la composition en cours — fusion avec `to/cc/bcc` existants (séparateur `, `, uniques) — app.js:13107-13160
F-CT-91 : Pastille expéditeur (gravatar) — image affichée dans la vue du message si `GRAVATAR` actif — MailMessageView.html:239 ; app.js:5837-5840, 35800-35812
F-CT-92 : Service d'avatar serveur — Gravatar (http, 5 s, 80 px, 404 si absent) → icône de domaine connue → image vide ; cache HTTP — ACT:8844-8897
F-CT-93 : Option admin Gravatar — case « Autoriser Gravatar » (`labs.allow_gravatar`) — ACT:3724-3725, 3833, 8239-8242 ; AdminSettingsGeneral.html:115
F-CT-94 : Pas de lien contact dans la lecture — aucun « ajouter aux contacts », aucun nom issu du carnet — MailMessageView.html (aucune occurrence)
F-CT-95 : Sérialisation JSON des contacts — IdContact, Display, ReadOnly, IdPropertyFromSearch, Properties[IdProperty, Type, TypeStr, Value] — ACT:9924-9959
F-CT-96 : Correction de schéma d'URL — `\://` → `://` dans la valeur d'URL — ACT:9946-9950
F-CT-97 : Hash d'identifiant côté client — `generateUid()` = id en chaîne pour le Selector — app.js:34396-34402
F-CT-98 : Réglage « cases à cocher dans les listes » appliqué à la liste de contacts — `useCheckboxesInList` — app.js:30780-30781 ; PopupsContacts.html:74
F-CT-99 : Paramètres admin du stockage — type, DSN, utilisateur, mot de passe masqué par marqueur — ACT:3680-3683, 3817-3824 ; AdminSettingsContacts.html:24-102
F-CT-100 : Avertissement admin SQLite — texte de mise en garde pour beaucoup d'utilisateurs — AdminSettingsContacts.html:83-92 ; fr_FR admin TAB_CONTACTS
F-CT-101 : Libellés français — sections CONTACTS, SETTINGS_CONTACTS, SUGGESTIONS, NOTIFICATIONS (CONTACTS_SYNC_ERROR), TAB_CONTACTS — fr_FR.yml webmail:163-181, 499-507, 667 ; admin:72-87, 258
F-CT-102 : Limite de requêtes de suggestions — une requête par frappe (délai 300 ms) sans mise en cache côté client — libs.js ; app.js:7460-7465
F-CT-103 : Contacts sans champ de société/adresse/photo — aucun champ ORG, ADR, PHOTO dans le modèle — PropertyType.php ; Contact.php
F-CT-104 : Limites de taille de valeur — colonne TEXT sauf `display` varchar(255) ; aucune limite applicative — PdoAddressBook.php:1759-1783, 1897-1899

---------------------------------------------------------------------
# Résumé (10 lignes)

1. Le carnet est un sac de propriétés typées par utilisateur (SQLite/MySQL/PostgreSQL), sans tags, groupes, partage ni champs société/adresse/photo.
2. Édition UI limitée : prénom, nom, pseudo, e-mails, téléphones, URL ; notes et anniversaire existent en base sans entrée de menu ; types (Home/Work...) non éditables.
3. Aucun contrôle de format sur e-mails/téléphones/URL ; nom affiché calculé côté serveur (nom complet > prénom+nom > e-mail > téléphone).
4. Liste triée par nom affiché, 50 par page, recherche LIKE (e-mails, noms, pseudo, téléphone en chiffres, URL), sélection clavier/cases, raccourcis Suppr et c/w.
5. Import VCF multi-cartes (Sabre) et CSV (en-têtes mappés, Outlook/Thunderbird/Gmail partiels) sans aucune détection de doublons ; export VCF 3.0 et CSV 34 colonnes Outlook.
6. Synchro CardDAV optionnelle bidirectionnelle (découverte, etag, le plus récent gagne), toutes les 5-320 min, TLS non vérifié, mot de passe stocké réversiblement.
7. Partage non implémenté (`IsSharingAllowed` toujours faux, icône décorative, `ReadOnly` inatteignable).
8. Autocomplétion : inputosaurus + jQuery UI (minLength 1, 300 ms), suggestions triées par fréquence, plugins/ownCloud en complément, limite configurable (défaut 30).
9. Fréquence +1 par envoi sur les destinataires À seulement ; ajout automatique (défaut activé) avec bug de casse créant des doublons ; aucun lien « ajouter au contact » en lecture ; pastille expéditeur = Gravatar/icône de domaine, jamais la photo du contact.
10. Anomalies relevées : MiddleName JS=16, `$aNicks` non initialisé, sérialisation de l'anniversaire absente, chaîne SUGGESTIONS inutilisée, suppression logique jamais purgée sans synchro.

Fichier : /tmp/claude-1000/-home-gdegoulet-Work-webmail-project/be75865a-1a74-4b90-b555-b7c7c12ae39b/scratchpad/audit_contacts.md
