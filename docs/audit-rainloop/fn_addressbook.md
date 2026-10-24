# Analyse fonction par fonction : Providers/AddressBook* de RainLoop 1.17 contre wmail-go

Périmètre : `app/libraries/RainLoop/Providers/AddressBook.php`, `AddressBook/AddressBookInterface.php`, `AddressBook/PdoAddressBook.php`, `AddressBook/Classes/{Contact,Property,Tag}.php`, `AddressBook/Enumerations/PropertyType.php` (72 fonctions + 1 énumération). Lecture seule, aucun `data/`. Le détail de l'audit fonctionnel précédent (`audit_contacts.md`) n'est pas répété : ce fichier le complète fonction par fonction et vérifie ce que wmail-go fait réellement.

Abréviations : `C` = `internal/contacts/contacts.go`, `V` = `internal/contacts/vcard.go`, `CSV` = `internal/contacts/csv.go`, `DAV` = `internal/contacts/carddav.go`, `W` = `internal/web/contactsweb.go`, `WS` = `internal/web/contactssync.go`, `AC` = `internal/web/static/contacts-ac.js`, `T` = `internal/web/templates/contacts.html`.

Constats nouveaux par rapport à l'audit précédent (tous vérifiés dans le code Go lu) :
- Type de téléphone « Mobile » (RainLoop) devient `mobile` à l'import vCard (`V:41-48`, minuscule du premier TYPE), alors que wmail-go utilise `cell` partout ailleurs (`T:64`, `CSV:70`, `CSV:265-286`) : les mobiles issus de RainLoop sortent dans « Other Phone » à l'export CSV.
- En-têtes CSV non reconnus alors qu'ils le sont dans RainLoop : Home/Business/Other/Primary Email, Secondary Email (Thunderbird), Fax, Business Mobile (`CSV:77` n'accepte que les préfixes « e mail »/« email »).
- Le type d'e-mail (Home/Work/Other) n'est jamais posé à l'import CSV (`CSV:186-189`), alors que RainLoop le déduit de la colonne.
- La synchronisation CardDAV perd les propriétés vCard inconnues (photo, IMPP, X-*) à chaque renvoi d'un contact modifié (`DAV:49-50`, `V:91-130`) ; RainLoop fusionne avec la vCard distante (PdoAddressBook.php:754-773).
- Recherche de téléphone : pas de comparaison sur les chiffres seuls (`C:329-342` indexe la valeur brute).
- Le sélecteur de contacts de la composition n'écrit que dans « À » (`AC:82`) ; RainLoop écrit dans le dernier champ actif (À/Cc/Cci).
- Aucun envoi groupé « nouveau message aux contacts cochés » depuis la liste (`T:19`, cases = suppression seulement).

## 1. AddressBook.php (façade du fournisseur)

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (19) | Enveloppe un pilote `AddressBookInterface` ou le laisse nul | Infra PHP | aucun (pas de pilote : carnet interne) | SANS OBJET | Pilote unique JSON chiffré (`C:182`, `store.go:566-569`). |
| `Test` (31) | Test de connexion du pilote avec pause 1 s (bouton admin « Test ») | Admin : vérifier la base SQL | aucun | SANS OBJET | Pas de base externe ; stockage fichier chiffré. |
| `IsActive` (41) | Carnet utilisable si pilote présent et supporté (conditionne toute l'UI contacts) | Activer/désactiver les contacts globalement | routes toujours enregistrées `server.go:188-196` | SANS OBJET | Un seul utilisateur, contacts toujours actifs. |
| `IsSupported` (50) | Idem, extension PDO présente | Message admin « PDO non supporté » | aucun | SANS OBJET | Pas de PDO. |
| `IsSharingAllowed` (59) | Partage de carnet (toujours faux dans le pilote PDO, TODO) | Aucun (fonction inactive) | aucun | SANS OBJET | Fonction jamais active dans RainLoop. |
| `Sync` (73) | Lance la synchro CardDAV du compte (url, user, mot de passe) | Garder le carnet identique à un serveur CardDAV (Nextcloud, iCloud…) | `DAV:51-203` `Book.SyncCardDAV`, `WS:82-133`, routes `server.go:195-196`, déclenchement `poll.go:52` | PARTIEL | Présent et bidirectionnel. Écarts : propriétés vCard inconnues perdues au renvoi ; voir `Sync` du pilote (§2). P2. |
| `Export` (84) | Exporte tout le carnet en VCF ou CSV (téléchargement) | Sauvegarde / migration | `W:317-352`, `V:133-141`, `CSV:250-297`, liens `T:79` | PARTIEL | VCF OK (3.0, plus complet : adresses, société, anniversaire, catégories). CSV : en-têtes maison, pas les 34 colonnes Outlook de RainLoop ; adaptation facultative, P3. |
| `ContactSave` (95) | Crée/modifie un contact, recalcule nom affiché | Éditer la fiche | `C:236-260`, `W:238-253`, formulaire `T:35-67` | OK | Normalisation et plafonds en plus (`C:114-179`). Pas de rejet d'e-mail invalide dans RainLoop ; ici les e-mails invalides sont écartés silencieusement (`C:144`), acceptable. |
| `DeleteContacts` (106) | Supprime une liste d'ids | Supprimer une sélection | `C:270-289`, `W:255-264`, formulaire `T:12-30` | OK | Confirmation intégrée (`T:26-28`) ; pierres tombales pour CardDAV (`C:280`). Pas de raccourci `Suppr` (voir `keys`, P3). |
| `DeleteAllContacts` (116) | Purge tout le carnet d'un compte (suppression de compte) | Hygiène à la suppression du compte | `store.go:573-586` (boucle sur `blobExtensions`, dont `.ctc`) | OK | Fichier chiffré supprimé avec le compte. |
| `GetContacts` (130) | Liste paginée (50) triée par nom affiché, avec recherche et total | Liste et recherche de contacts | `C:345-367`, `W:110-188`, vue `T:6-29` | PARTIEL | Recherche sans accents ni casse (mieux), filtre par étiquette (en plus), pages de 50. Écarts : (1) tri par « Nom Prénom » (`C:95-101`) au lieu du nom affiché RainLoop ; (2) recherche de téléphone sans normalisation en chiffres (voir `specialConvertSearchValueCustomPhone`). P3 / P2. |
| `GetContactByID` (143) | Lit un contact (par id numérique ou chaîne) | Ouvrir une fiche | `C:213-220` (`Get`), `C:223-233` (`ByEmail`), `W:140-144` | OK | — |
| `GetSuggestions` (157) | Autocomplétion destinataires, triée par fréquence | Saisie rapide des adresses | `C:402-440` `Suggest`, `W:355-380`, `AC:19-79` (250 ms, flèches, Tab/Entrée) | PARTIEL | OK : tri fréquence puis nom, sans accents, plusieurs e-mails par fiche, avortement des réponses périmées (`AC:62-65`). Écart : limite fixe de 8 (`W:375`) contre 30 par défaut (min. 5) ; pas de recherche sur société/étiquettes. Proposition : porter à ~20 avec défilement du menu (`.ac-list`). P3. |
| `IncFrec` (169) | Après envoi : +1 de fréquence pour les destinataires À, création automatique des inconnus | Faire monter les contacts fréquents ; carnet qui se remplit seul | `C:291-316` (`AddAddress`, `Bump`), `W:402-420`, appelé par `compose.go:648`, réglage `handlers.go:285,431` | PARTIEL | Présent et désactivable. Écarts : (1) wmail-go crée aussi les contacts des Cc (`W:411`) alors que RainLoop ne touche que À ; (2) Cci ignoré des deux côtés ; (3) la fréquence est portée par le contact, non par l'adresse (sans conséquence). Correction de casse (bug RainLoop) déjà évitée (`C:293`). Proposition : option ou limiter la création à À pour coller à RainLoop. P3. |
| `csvNameToTypeConvertor` (179) | Table colonne CSV → type de propriété et TypeStr | Importer des exports Outlook/Thunderbird/autres | `CSV:28-89` `headerKind` | PARTIEL | Couvert : prénom, nom, second prénom, pseudo, société, fonction, notes, web, adresse, téléphones mobile/domicile/travail, e-mail* (préfixe), colonnes Google (« E-mail 1 - Value »). Manquants (non reconnus par `headerKind`) : « Home Email », « Business Email », « Other Email », « Primary Email », « Secondary Email » (Thunderbird), « Personal Email » ; « Business/Home/Other Fax », « Fax Number », « Business Mobile », « Mobile Number », « Pager » ; « Other Phone »/« Primary Phone »/« Company Main Phone » reconnus mais sans type. Proposition : étendre `headerKind` (suffixe/infixe « email » et « e mail », « fax », « mobile », types home/work/other) et poser `Typed.Type` des e-mails. P2. |
| `ImportCsvArray` (265) | Un contact par ligne CSV, propriétés selon la table, sauvegarde | Import de carnet CSV | `CSV:113-217` `ParseCSV`, `W:266-315` | PARTIEL | Mieux que RainLoop : séparateur `, ; tab`, UTF-8 ou Windows-1252, BOM, déduplication par e-mail (`W:296-301`), compte des ignorés. Manque : le type d'e-mail (Home/Work/Other) et de téléphone « Other » ne sont pas posés (`CSV:186-190`) ; une colonne e-mail vide-ligne est ignorée correctement. P2 (avec `csvNameToTypeConvertor`). |
| `ImportVcfFile` (320) | Découpe un fichier multi-vCard, génère un UID manquant, sauvegarde | Import vCard | `V:16-39` `ParseVCards`, `W:284-288`, UID généré `C:246` | PARTIEL | Multi-cartes 2.1/3.0/4.0, doublons ignorés, compteur. Manques : BOM UTF-8 non retiré avant le décodeur (retiré seulement pour le CSV `CSV:92`) ; vCard 2.1 sans CHARSET (latin-1) non converti (RainLoop : `utf8_encode`). Proposition : `bytes.TrimPrefix` BOM et repli Windows-1252 si UTF-8 invalide dans `contactsImport`. P3. |

## 2. AddressBook/AddressBookInterface.php et PdoAddressBook.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `AddressBookInterface::IsSupported` (10) | Contrat du pilote | Infra | aucun | SANS OBJET | — |
| `__construct` (31) | Mémorise DSN, utilisateur, mot de passe, type SQL | Admin : choix du moteur | aucun | SANS OBJET | Pas de SQL. |
| `IsSupported` (44) | Vérifie PDO et le pilote SQL | Admin | aucun | SANS OBJET | — |
| `IsSharingAllowed` (53) | Toujours faux | — | aucun | SANS OBJET | — |
| `flushDeletedContacts` (58) | Purge physique des contacts `deleted=1` après synchro | Éviter l'accumulation de lignes supprimées | `DAV:189-199` (`b.Deleted` vidé), `C:270-289` (suppression immédiate) | OK | Pas de suppression logique : le contact est retiré tout de suite et sa pierre tombale est purgée après l'envoi. |
| `updateContactEtagAndTime` (65) | Enregistre etag et date après envoi distant | Détecter les changements au passage suivant | `DAV:176-185` (etag de la réponse, sinon GET) | OK | — |
| `prepearDatabaseSyncData` (77) | Carte locale (uuid → etag, date, id) pour la synchro | Comparaison local/distant | `DAV:80-95` (`Href`, `ETag`, `Changed`, `byUID`) | OK | Lien aussi par UID si l'URL est inconnue (`DAV:114-116`). |
| `prepearRemoteSyncData` (113) | PROPFIND des etags et dates distants | Savoir quoi télécharger | `DAV:62` `QueryAddressBook` avec `AllProp` | PARTIEL | Fonctionne mais télécharge toutes les vCard à chaque passage (RainLoop liste d'abord les etags). Pour 5000 contacts toutes les 20 min c'est lourd. Proposition : demander d'abord `getetag` (multiget ensuite) ; ou ignorer si ctag/sync-token inchangé. P3. |
| `davClientRequest` (184) | Requête DAV brute avec journalisation et proxy | Plomberie | bibliothèque `go-webdav` (`DAV:57`) ; HTTP via garde SSRF `WS:114` | SANS OBJET | Plomberie remplacée par la bibliothèque ; pas de proxy (voulu, `CDC 153`). |
| `detectionPropFind` (229) | Détermine si un chemin est un carnet (resourcetype) | Accepter une URL de carnet ou de serveur | `DAV:62-77` (si la requête échoue, découverte du carnet) | OK | Approche différente mais équivalente. |
| `getContactsPaths` (262) | Découverte : `.well-known/carddav`, principal, home-set, collections `addressbook`, choix du carnet par nom (`url|nom`, contacts/default/…) sinon premier | Saisir juste l'hôte | `DAV:65-76` (`FindAddressBookHomeSet`, `FindAddressBooks`, premier carnet) | PARTIEL | Manquent : `/.well-known/carddav`, choix d'un carnet par nom quand le serveur en propose plusieurs (on prend le premier : risque de mauvais carnet sur iCloud/Nextcloud), reprise si le home-set échoue. Proposition : champ facultatif « Nom du carnet » dans `SyncView`/`userdata.ContactsSync` + tentative `.well-known` ; sélection par nom sinon par noms usuels. P3. |
| `checkContactsPath` (454) | Vérifie qu'un chemin est bien un carnet CardDAV | Sécuriser la découverte | `DAV:62-77` | OK | — |
| `getDavClientFromUrl` (504) | Normalise l'URL (raccourcis fruux/iCloud/gmail, schéma, port) et crée le client Sabre ; TLS non vérifié | Saisie simplifiée d'URL | `contacts.ValidSyncURL` `DAV:31-46`, saisie `T:87`, `WS:157-162` | PARTIEL | https exigé (sauf hôtes autorisés), TLS vérifié, pas de SSRF (mieux). Pas de raccourcis `icloud.com`, `fruux.com`, `gmail.com`. Proposition : table de raccourcis simple (3 entrées) dans `WS` avant `ValidSyncURL`. P3. |
| `getDavClient` (565) | Résout chemin, découverte, authentification, retourne client et chemin | Connexion réelle | `DAV:57-77` (Basic) | PARTIEL | Basic seulement ; Sabre gérait aussi Digest. Rare en 2026. P3. |
| `Sync` (667) | Algorithme bidirectionnel (suppressions, créations, mises à jour, plus récent gagne), fusion avec la vCard distante pour garder les champs inconnus | Contacts identiques sur téléphone/ordinateur | `DAV:51-203` | PARTIEL | OK : suppression des deux côtés (`DAV:147-161,189-198`), plus récent gagne (`DAV:132-135`), tombstones, nouveau lien par UID. Manque : **fusion avec la vCard distante au PUT** (`DAV:175-176` réécrit la carte entière via `toCard`) : photo, IMPP, X-* perdus sur le serveur distant dès qu'un contact est modifié ici. Proposition : champ `Contact.Extra` (propriétés vCard non gérées, sérialisées dans le `.ctc`) rempli par `fromCard` et réémis par `toCard` ; ou fusion avec le dernier GET. P2. |
| `Export` (824) | Génère VCF ou CSV de tous les contacts non supprimés | Sauvegarde | `W:317-352` | OK | Filtre par recherche/étiquette en plus. |
| `ContactSave` (871) | Insert/update en base, conserve la fréquence, met `Changed` | Persistance | `C:236-260` (Frec, UID, Href, ETag conservés `C:254`) | OK | — |
| `DeleteContacts` (991) | Suppression logique + purge des propriétés | Suppression | `C:270-289` | OK | — |
| `DeleteAllContacts` (1029) | Purge totale | Suppression de compte | `store.go:586` | OK | — |
| `GetContacts` (1055) | SQL : recherche LIKE sur e-mail/prénom/nom/pseudo/téléphone/URL, tri par display, pages | Liste/recherche | `C:329-367` | PARTIEL | Voir façade : couvert (plus large : société, notes, étiquettes). Écarts : téléphone en chiffres, tri. P2/P3. |
| `GetContactByID` (1252) | Lecture d'un contact et ses propriétés | Fiche | `C:213` | OK | — |
| `GetSuggestions` (1358) | Requête e-mail/prénom/nom/pseudo triée par fréquence, un résultat par e-mail correspondant, nom affiché = « Prénom Nom » ou pseudo | Autocomplétion | `C:402-440` | PARTIEL | Libellé : wmail-go affiche `Display()` qui inclut préfixe/suffixe/second prénom (`C:78`) : « Dr Jean Paul Martin » dans l'adresse insérée, alors que RainLoop met « Prénom Nom ». Proposition : `SuggestionName()` = Prénom + Nom (sans préfixe/suffixe). P3. Limite 8, voir ci-dessus. |
| `IncFrec` (1560) | Fréquence +1 et création auto | Voir façade | `C:291-316`, `W:402-420` | PARTIEL | Voir façade (Cc créé en plus). |
| `Test` (1724) | Connexion à la base | Admin | aucun | SANS OBJET | — |
| `getInitialTablesArray` (1752) | DDL des tables selon moteur | Infra | aucun (JSON `C:182-189`) | SANS OBJET | — |
| `SyncDatabase` (1879) | Migrations de schéma v1→v3 | Infra | `json:"…,omitempty"` tolérant, `C:192-201` | SANS OBJET | Format JSON extensible sans migration. |
| `getContactFreq` (1931) | Reporte la fréquence par e-mail lors d'une réécriture du contact | Ne pas perdre le classement | `C:254` (`Frec` conservé) | OK | — |
| `specialConvertSearchValue` (1967) | Échappe `%`, `_`, `=` pour LIKE | Recherche sûre | `C:321-327` `fold` + `strings.Contains` (pas de SQL) | OK | Aucun risque d'injection ; accents ignorés en plus. |
| `specialConvertSearchValueLower` (1979) | Version minuscule UTF-8 de la recherche | Insensible à la casse | `C:321-327` | OK | — |
| `specialConvertSearchValueCustomPhone` (1996) | Terme réduit à ses chiffres pour comparer au téléphone normalisé | Retrouver « 06 12 34 56 78 » en tapant « 0612 » ou « +33… » | aucun (`C:334-336` indexe le texte brut) | MANQUANT | Dans `haystack()` ajouter les téléphones « chiffres seuls » (sans `+` de tête) et, si `q` contient surtout des chiffres, comparer aussi à cette forme. P2. |
| `getPdoAccessData` (2005) | Paramètres de connexion PDO | Infra | aucun | SANS OBJET | — |

## 3. AddressBook/Classes/Contact.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (52) | Appelle `Clear` | Init | struct `C:52-74` | OK | — |
| `Clear` (57) | Remet les champs à zéro (Changed = maintenant) | Réutiliser l'objet pendant les imports | aucun besoin (nouvelle struct par fiche `CSV:147`) | SANS OBJET | Spécificité PHP (objet réutilisé). |
| `PopulateDisplayAndFullNameValue` (69) | Nom affiché : nom complet fourni, sinon « Prénom Nom », sinon e-mail, sinon téléphone ; crée/maj la propriété FN | Libellé de la liste, de l'autocomplétion et du FN exporté | `C:77-92` `Display()` | PARTIEL | Wmail-go : préfixe + prénom + second + nom + suffixe, puis pseudo, société, e-mail, téléphone. Écarts visibles : (1) RainLoop n'affiche jamais préfixe/suffixe/second prénom ; (2) le FN d'une vCard importée est ignoré dès que N existe (`V:55-63`) : un FN « Johnny S. » devient « John Smith » ; (3) FN sans N est coupé au dernier espace (« ACME Corp » → prénom ACME, nom Corp). Proposition : champ `FullName` conservé (import FN, réexport FN) ou à défaut : si N vide et FN contenant société/une seule entité → ne pas couper. P3. |
| `UpdateDependentValues` (154) | Génère l'id chaîne et recalcule le nom | Cohérence avant sauvegarde | `C:114-179` `Normalize`, UID `C:246` | OK | — |
| `RegenerateContactStr` (167) | UUID (Sabre) ou md5 aléatoire = nom du fichier `.vcf` | Identifiant de ressource CardDAV | `C:206-210` `newID`, `DAV:173` | OK | UID hexadécimal 24 car. (pas un UUID) : accepté par les serveurs CardDAV. |
| `GetEmails` (176) | Liste dédoublonnée des e-mails | Envoi groupé, recherche | `C.Emails` dédoublonné `C:148` | OK | — |
| `CardDavNameUri` (193) | `<uid>.vcf` | Nom de ressource distante | `DAV:173` (`safeName(UID)+".vcf"`) | OK | — |
| `ToVCard` (201) | vCard 3.0 (FN, N, NICKNAME, NOTE, EMAIL/URL/TEL avec TYPE, UID, REV, PRODID), en fusionnant avec la vCard d'origine | Export et PUT CardDAV sans perte | `V:91-130` `toCard` | PARTIEL | Plus de champs (ADR, ORG, TITLE, BDAY, CATEGORIES). Manque : fusion avec la carte d'origine (champs inconnus perdus), pas de `REV`/`PRODID`, un seul TYPE par valeur. Voir `Sync` : champ `Extra`. P2. |
| `ToCsv` (315) | Ligne CSV Outlook 34 colonnes avec règle de repli de l'e-mail principal | Export compatible Outlook | `CSV:250-297` (24 colonnes maison) | PARTIEL | Contenu plus riche (adresse, société, anniversaire, étiquettes), neutralisation des formules tableur (`CSV:243-248`, mieux). Défaut : le téléphone `mobile` (RainLoop/vCard) tombe dans « Other Phone » (`CSV:279-283`) ; en-têtes différents d'Outlook. Proposition : accepter `mobile` comme `cell` dans `phone()`. P2. |
| `getPropertyValueHelper` (476) | Nettoie l'UTF-8 ; convertit en UTF-8 les vCard 2.0/2.1/1.0 sans CHARSET | Cartes anciennes lisibles | `V:16-39` (aucune conversion) ; CSV `CSV:91-100` (repli Windows-1252) | PARTIEL | Appliquer le repli Windows-1252 si le fichier vCard n'est pas de l'UTF-8 valide. P3. |
| `addArrayPropertyHelper` (501) | Ajoute e-mails/URL/téléphones ; les valeurs `PREF` ou sans type passent en tête | L'adresse principale est la première (liste, envoi groupé) | `V:65-73`, `typeOf` `V:41-48` | PARTIEL | `PREF` ignoré (pas de tri) donc `Emails[0]` n'est pas forcément l'adresse préférée (affichée en liste `W:130-132`) ; un seul TYPE gardé (« work,fax » devient « work ») ; « Mobile » reste `mobile` au lieu de `cell`. Proposition : dans `fromCard`, placer PREF en tête, composer le type (fax prioritaire), normaliser `mobile`→`cell`. P2. |
| `PopulateByVCard` (523) | vCard → propriétés (FN, NICKNAME, NOTE, N, EMAIL, URL, TEL, UID) | Import et réception CardDAV | `V:50-89` `fromCard` | PARTIEL | Plus complet (ADR, ORG, TITLE, BDAY, CATEGORIES). Écarts : voir `PREF`, FN, types. Photos ignorées (RainLoop aussi). P2/P3. |

## 4. AddressBook/Classes/Property.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `__construct` (44) | Type, valeur, type libellé | Modèle générique | `Typed` `C:36-39` | OK | — |
| `Clear` (54) | Remise à zéro | Réutilisation d'objet | aucun besoin | SANS OBJET | — |
| `IsName` (71) | Le type est une propriété de nom | Aiguillage interne | champs nommés `C:55-60` | SANS OBJET | Modèle structuré sans énumération. |
| `IsEmail` (80) | Type e-mail | idem | `Contact.Emails` | SANS OBJET | idem |
| `IsPhone` (88) | Type téléphone | idem | `Contact.Phones` | SANS OBJET | idem |
| `IsWeb` (96) | Type URL | idem | `Contact.URLs` | SANS OBJET | idem |
| `IsValueForLower` (104) | Doit-on stocker la version minuscule | Recherche sans casse | `fold` `C:321-327` | OK | Recherche sans casse ni accents, plus tolérante. |
| `TypesAsArray` (112) | Types multiples séparés par virgule (`Work,Fax`) | Distinguer fax/mobile/travail | `Typed.Type` unique `C:37` | PARTIEL | Un seul type par valeur ; les combinaisons sont écrasées. Proposition : autoriser `Type` = liste séparée par virgules (valeurs `work,fax`) ; l'UI garde un champ texte (`T:49-51`). P3. |
| `TypesUpperAsArray` (127) | Majuscules pour comparer | idem | `strings.ToLower` `C:138` | OK | — |
| `UpdateDependentValues` (132) | Trim, domaine e-mail en minuscule, espaces des noms réduits, minuscule de recherche, chiffres du téléphone | Normalisation | `C:114-179` | PARTIEL | Tout est couvert sauf le champ « téléphone en chiffres » (voir `specialConvertSearchValueCustomPhone`). P2. |

## 5. Classes/Tag.php et Enumerations/PropertyType.php

| Fonction (ligne) | But / périmètre | Besoin → fonctionnalité | Équivalent Go | Verdict | Proposition & priorité |
|---|---|---|---|---|---|
| `Tag::__construct` (22) / `Tag::Clear` (27) | Étiquette de contact (id, nom, lecture seule) ; jamais persistée ni affichée dans RainLoop | Aucun (vestige) | étiquettes réelles et utilisables : `C:69`, `C:379-392`, filtre `T:8` | SANS OBJET | wmail-go dépasse RainLoop ici. (2 fonctions) |
| `PropertyType` (constantes, 1-34) | UID, noms, e-mail, téléphone, URL, anniversaire, Facebook, Skype, GitHub, note, personnalisé | Types de propriétés | champs `C:52-74` | SANS OBJET | Facebook/Skype/GitHub/custom non éditables dans RainLoop (aucune entrée de menu). |

## À AJOUTER (MANQUANT ou PARTIEL, par priorité)

P2
1. Recherche de téléphone par chiffres seuls (sans espaces, points, `+`) : indexer la forme « chiffres » dans `haystack()` `C:329` et comparer si `q` est surtout numérique.
2. Import CSV : étendre `headerKind` `CSV:28-89` (Home/Business/Other/Primary/Secondary Email, Personal Email, Fax, Business Mobile, Mobile Number, Pager) et poser les types d'e-mail (home/work/other) et de téléphone « other ».
3. Normaliser les types de téléphone à l'import vCard : `mobile`/`cell` → `cell`, `PREF` en tête, types composés (`work,fax`) ; export CSV : traiter `mobile` comme `cell` (`V:41-48`, `CSV:265-286`).
4. CardDAV : conserver les propriétés vCard inconnues (photo, IMPP, X-*) dans `Contact.Extra` et les réémettre au PUT (`V:50-130`, `DAV:175`) pour ne pas appauvrir le carnet distant.
5. Envoi groupé depuis la liste des contacts : lien « Écrire aux contacts cochés » (composition avec tous les e-mails principaux des cases cochées, `T:19`, `W`).
6. Sélecteur de contacts de la composition : écrire dans le dernier champ actif (À/Cc/Cci) au lieu de toujours À (`AC:82-115`), plus pagination au-delà de 50 (`W:362-364`).

P3
7. Retirer le BOM UTF-8 et repli Windows-1252 pour les fichiers vCard anciens (`V:16`, `W:284`).
8. Nom proposé dans l'autocomplétion = Prénom + Nom, sans préfixe/suffixe/second prénom ; conserver le FN d'une vCard importée comme nom affiché (`C:77-92`, `V:55-63`).
9. Limite de suggestions portée de 8 à ~20 avec défilement (`W:375`).
10. Création automatique après envoi limitée aux destinataires « À » comme RainLoop (option) (`W:411`).
11. Tri de la liste par nom affiché (comme RainLoop) ou option de tri (`C:357`).
12. CardDAV : champ « nom du carnet » et tentative `.well-known/carddav`, raccourcis `icloud.com`/`fruux.com`, moins de téléchargements (etags d'abord) (`DAV:62-77`, `WS:157`).
13. Raccourcis clavier dans la liste des contacts (`Suppr`, `c`/`w`), absents de `keys.js`.
14. Export CSV avec les 34 colonnes Outlook de RainLoop (`CSV:17-20`) si la compatibilité Outlook importe.
15. Types composés (`work,fax`) dans `Typed.Type`.

Fin de la lecture : toutes les fonctions du périmètre ont été traitées (aucune coupure budgétaire). Photo de contact, partage, tags RainLoop, administration SQL : sans objet ou déjà dépassés par wmail-go.
