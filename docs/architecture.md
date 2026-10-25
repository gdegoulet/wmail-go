# Architecture de wmail-go

Comment les paquets s'articulent, ce que devient une requête, et surtout les **invariants** : les règles que
rien ne signale à la compilation et dont la violation casse l'application en production ou en silence.

Pour la surface HTTP, voir `docs/routes.md`. Pour le modèle de menace et la cryptographie, `SECURITY.md`.
Pour le motif de chaque choix, le journal des décisions du `CAHIER_DES_CHARGES.md` (§ 6).

---

## 1. Vue d'ensemble

wmail-go est **un seul binaire statique** et **un seul répertoire de données**. Pas de base de données, pas de
service annexe, pas d'état partagé entre instances. Le serveur ne fait pas de TLS : un proxy s'en charge.

```
                 navigateur
                     │ HTTP
                     ▼
        ┌────────────────────────┐
        │  internal/web          │  routes, gabarits, CSS, JavaScript
        │   logging → secure     │  journal, en-têtes, origine des POST
        │   → mux → auth         │  session, CSRF
        └───┬────────────┬───────┘
            │            │
    ┌───────▼─────┐  ┌───▼──────────┐
    │  store      │  │  mailconn    │ ──► IMAP / SMTP / ManageSieve
    │  session    │  │  (pool,      │     (à travers le garde anti-SSRF)
    │  vault      │  │   garde)     │
    └───────┬─────┘  └──────────────┘
            │
      ┌─────▼────────────────┐
      │  <data_dir>/         │  server.key, users.idx, users/**
      └──────────────────────┘
```

### Les paquets

| Paquet | Rôle | Dépend de |
|---|---|---|
| `cmd/wmail-go` | point d'entrée : `serve`, `user`, `healthcheck`, `version` | config, store, web |
| `internal/config` | lecture et validation de `config.json` | — |
| `internal/vault` | format du fichier chiffré `.cfg`, des blobs et du journal | — |
| `internal/store` | comptes, index, chemins, lecture et écriture des fichiers chiffrés | vault |
| `internal/userdata` | **structures de données du compte**, sérialisées en JSON dans le `.cfg` | — |
| `internal/session` | sessions en mémoire, expiration, nettoyage | store |
| `internal/mailconn` | IMAP et SMTP : pool, recherche, tri, conversations, garde anti-SSRF | — |
| `internal/mailview` | analyse MIME, assainissement HTML, conversion texte, vignettes | — |
| `internal/compose` | construction des messages sortants, adresses, MDN | mailview |
| `internal/contacts` | carnet : vCard, CSV, CardDAV | — |
| `internal/sieve` | ManageSieve, règles de filtrage, génération du script | — |
| `internal/ical` | invitations iCalendar | — |
| `internal/totp` | second facteur et codes de secours | — |
| `internal/ratelimit` | compteurs par IP et par utilisateur | — |
| `internal/i18n` | traductions `fr` et `en` | — |
| `internal/web` | serveur HTTP, gabarits, ressources statiques | tous les précédents |

`internal/userdata` ne dépend de rien et ne doit jamais dépendre de quoi que ce soit : c'est le vocabulaire
commun entre le stockage et le web. Y ajouter une dépendance créerait un cycle.

---

## 2. Le chemin d'une requête

1. **`logging`** — chronomètre, puis écrit une ligne sur la sortie standard.
2. **`secure`** — pose les en-têtes de sécurité ; pour un `POST`, borne le corps par `postLimit(path)` et
   vérifie l'origine (`Origin` ou `Referer`). Origine différente → **403**.
3. **`http.ServeMux`** — motifs Go 1.22 : méthode, chemin, variables `{nom}`.
4. **`auth`** (routes authentifiées) — relit la session depuis le cookie, détourne vers `/recovery-key` si une
   clé est en attente, compare le jeton CSRF en temps constant pour les `POST`.
5. **Le gestionnaire** — valide ses paramètres, agit, puis rend une page ou redirige.
6. **`render` → `prepare` → `execute`** — `prepare` remplit les champs communs de `Page` (langue, thème,
   jeton CSRF, utilisateur, boîtes, rappel de connexion), `execute` exécute le gabarit `base` qui inclut le
   gabarit nommé.

Les écritures suivent le motif **POST / Redirect / GET** : un `303` après chaque succès, pour qu'un
rafraîchissement ne rejoue pas l'action.

### Gabarits

Chaque page est un `*template.Template` **séparé**, construit au démarrage dans `New()` :
`base.html` plus le fichier de la page, analysés ensemble, rangés dans `s.pages[nom]`.

Les ressources sont embarquées dans le binaire (`embed.FS`) : gabarits, CSS, JavaScript, thèmes, traductions.
Le binaire n'a besoin que de `config.json` et de `data_dir`.

---

## 3. Invariants

Ce sont les règles que ni le compilateur ni les tests unitaires ne rappellent.

### 3.1 Un nouveau gabarit doit être déclaré dans la liste

`internal/web/server.go`, dans `New()`, une **liste littérale** énumère les gabarits à analyser :

```go
for _, name := range []string{"login", "register", …, "templates", "activity"} {
```

Ajouter `templates/xyz.html` sans ajouter `"xyz"` à cette liste compile, passe `go vet`, et **panique à
l'exécution** à la première visite :

```
html/template: ExecuteTemplate ... lookupAndEscapeTemplate(0x0, "xyz")
```

Aucun test ne couvre une page qu'on vient d'écrire : le symptôme n'apparaît que dans le navigateur.

### 3.2 `session.Mu` n'est pas réentrant

`sync.Mutex` ne l'est jamais en Go. Plusieurs fonctions de `internal/web` prennent `sess.Mu` sans que leur nom
le dise — **`s.userLoc(sess)` en fait partie**. Le motif suivant fige la requête pour toujours :

```go
sess.Mu.Lock()
defer sess.Mu.Unlock()
… s.userLoc(sess) …        // reprend sess.Mu : interblocage
```

Règle : **sous `sess.Mu`, lisez les champs dont vous avez besoin, relâchez, puis travaillez.**

```go
sess.Mu.Lock()
a, b := sess.X, sess.Y
sess.Mu.Unlock()
… s.userLoc(sess) …        // sûr
```

Le symptôme n'est pas un plantage mais une page qui ne répond plus, et un conteneur qui reste `Up` : on perd
facilement une demi-heure à chercher ailleurs.

Verrous distincts et indépendants : `sess.Mu` (compte, second facteur, message éphémère, rappel de connexion)
et `sess.ContactsMu` (carnet en cache). Ne les imbriquez pas.

### 3.3 Toute cible de redirection passe par `safeNext` ou `safeBack`

Un paramètre `next`, `back`, `stay`, `return_to` ou `open_next` vient du client. Rendu tel quel dans un
`http.Redirect`, c'est une redirection ouverte. `safeNext` et `safeBack` n'acceptent qu'un chemin local
commençant par `/`, sans `//` ni schéma. **Aucune exception.**

### 3.4 Écriture d'un champ du compte : copier, écrire, restaurer en cas d'échec

Le compte en mémoire et le fichier sur disque doivent rester cohérents. Le motif employé partout :

```go
sess.Mu.Lock()
prev := *set                    // ou slices.Clone pour une tranche
*set = nouvelleValeur
err := s.store.SaveData(sess.Account)
if err != nil {
    *set = prev                 // le disque n'a pas changé : la mémoire non plus
}
sess.Mu.Unlock()
```

Sans la restauration, un disque plein laisse la session afficher un réglage que le fichier ne porte pas.

### 3.5 Un contenu de message n'est jamais rendu sans assainissement

Le HTML d'un message passe par `mailview.Sanitize` **et** est servi avec une CSP propre à la réponse
(`/mail/body`, `/mail/print`). La source brute est servie en `text/plain` avec
`Content-Security-Policy: sandbox` et `X-Content-Type-Options: nosniff`.

Le type d'une pièce jointe est décidé **d'après ses octets**, jamais d'après l'étiquette du message :
`previewKind` reconnaît les types sûrs ; tout le reste devient `application/octet-stream` en téléchargement.

### 3.6 `fr.json` et `en.json` portent les mêmes clés

`TestLocalesHaveSameKeys` le vérifie. Deux conventions **non testées** s'y ajoutent :

- les placeholders doivent correspondre à l'appel `fmt.Sprintf` du code (`%d`, `%s`, dans le même ordre) ;
- **insérez une clé en place**, à côté de ses voisines. Retrier le fichier produit un diff de plus de mille
  lignes qui noie le vrai changement.

### 3.7 Une connexion IMAP appartient à une session **et** à une boîte

La clé du pool est `sess.ID + "/" + acc.ID` (`poolKey`). À la fermeture d'une session, `OnClose` ferme toutes
les connexions dont la clé commence par `sess.ID + "/"` et vide les pièces jointes en cours. Une connexion
n'est donc jamais partagée entre deux utilisateurs.

### 3.8 Le journal d'activité ne doit jamais faire échouer une connexion

`ensureEventKeys` et `noteLogin` sont appelés pendant l'ouverture de session. Leurs erreurs sont **ignorées** :
au pire le journal est incomplet, jamais au prix de la connexion. Gardez cette propriété si vous y touchez.

---

## 4. Le chiffrement, de bout en bout

```
server.key  (32 octets, tirés au hasard au premier démarrage, dans data_dir)
   │
   ├── HMAC-SHA256(server.key, login normalisé) ──► id (64 hex)
   │        └── donne le nom du fichier ET le chemin : users/<id[0:2]>/<id[2:4]>/<id>.cfg
   │
   └── chiffre users.idx (l'index des logins, pour « user list »)

mot de passe ──argon2id(salt, paramètres du fichier)──► KEK
                                                        └── déchiffre le slot « mot de passe »
clé de récupération ──HKDF──► KEK de secours
                                └── déchiffre le slot « récupération »
                                            │
                                            ▼
                                          DEK  (clé de données, tirée au hasard par compte)
                                            │
         ┌──────────────────────────────────┼─────────────────────────────┐
         ▼                                  ▼                             ▼
   contenu du .cfg               subkey("contacts") → .ctc        subkey("activity") → .act
   (XChaCha20-Poly1305)          subkey("background") → .bgd
```

- **Deux slots indépendants** portent la même DEK, l'un ouvert par le mot de passe, l'autre par la clé de
  récupération. Changer de mot de passe ne rechiffre que le slot : **le contenu n'est pas retouché**.
- Le contenu est authentifié avec le magic, la version **et l'`id`** (`contentAAD`). Un fichier déplacé vers
  un autre utilisateur ne s'ouvre pas, même avec le bon mot de passe.
- Les sous-clés viennent de `HKDF(DEK, "wmail-go blob/v1/" + usage)` : compromettre un blob ne donne pas les
  autres.
- **Aucun hachage de mot de passe n'est stocké.** L'authentification *est* le déchiffrement réussi. Un login
  inconnu déclenche `DummyDerive`, un argon2id inutile, pour que le temps de réponse ne distingue pas les cas.

### Le journal d'activité, à part

Les échecs de connexion doivent être consignés **compte fermé** : la DEK n'est pas disponible. D'où une paire
de clés X25519 par compte :

- `<id>.pub` — clé publique, en clair ; publier une clé publique ne coûte rien ;
- `<id>.evt` — file d'attente en ajout seul, une « sealed box » par événement, écrite sans rien ouvrir ;
- `<id>.act` — historique compacté, chiffré par `subkey("activity")`.

La clé privée vit dans le `.cfg`. À la connexion suivante, la file est versée dans l'historique et vidée.
Ce journal détecte l'abus de compte à distance, **pas** la compromission du serveur : qui écrit dans `/data`
détient la clé publique et peut forger des entrées, ou les supprimer. Voir `SECURITY.md` § 9 bis.

---

## 5. Le répertoire de données

Tout l'état de l'application. `data_dir` vaut `/data` dans le conteneur.

| Chemin | Chiffré | Contenu |
|---|---|---|
| `server.key` | — | 32 octets tirés au hasard au premier démarrage. **Sans elle, aucun compte n'est retrouvable** : elle donne le nom des fichiers. |
| `users.idx` | par `server.key` | index des logins, pour `user list`. Magic `WMIX`. Reconstructible en le supprimant, au prix de la liste. |
| `.lock` | — | verrou des écritures de l'index |
| `users/<xx>/<yy>/<id>.cfg` | argon2id + XChaCha20 | le compte : réglages, boîtes, identités, modèles, carnet de synchronisation, TOTP, clé privée du journal. Magic `WMCF`. |
| `users/<xx>/<yy>/<id>.ctc` | `subkey("contacts")` | carnet d'adresses. Magic `WMBL`. |
| `users/<xx>/<yy>/<id>.bgd` | `subkey("background")` | image de fond, précédée d'un octet de format (`j` ou `p`) |
| `users/<xx>/<yy>/<id>.act` | `subkey("activity")` | historique d'activité compacté |
| `users/<xx>/<yy>/<id>.evt` | sealed box X25519 | file d'attente des événements. Magic `WMEL`, entrées `WMEV`. |
| `users/<xx>/<yy>/<id>.pub` | — | clé publique X25519 du journal |
| `themes/<nom>/…` | — | ressources de thème posées par l'administrateur, servies par `/themes/{name}/{file}` |

`<xx>` et `<yy>` sont les deux premiers et les deux suivants des caractères de l'`id` : l'arborescence évite
un répertoire à cent mille entrées.

**Une sauvegarde doit emporter `server.key` et `users/` ensemble.** L'un sans l'autre ne sert à rien. C'est
aussi pourquoi l'archive de `make backup` est l'exemplaire le plus sensible du dispositif.

**Droits** : le conteneur tourne en UID 65532. Un `/data` appartenant à `root:root` donne
`open /data/server.key: permission denied` au démarrage.

---

## 6. Sessions

En mémoire, jamais sur disque : **un redémarrage déconnecte tout le monde**, et c'est voulu — la DEK du compte
vit dans la session, elle ne doit pas être écrite.

- Deux expirations : par inactivité (`idle`) et absolue (`abs`), réglables par l'utilisateur dans les bornes
  de `config.json`.
- `Manager.Run` balaie les sessions expirées toutes les minutes.
- `OnClose` ferme les connexions IMAP de la session et jette ses pièces jointes en cours.
- Un changement de mot de passe ferme **toutes les autres** sessions du compte ; une récupération les ferme
  toutes.
- `Account.Wipe()` efface les clés en mémoire. À appeler sur tout chemin d'erreur qui abandonne un compte
  ouvert, sans quoi la DEK traîne jusqu'au ramasse-miettes.

---

## 7. Connexions sortantes

Trois seulement, et toutes passent par le **garde anti-SSRF** (`internal/mailconn`, `s.guard`) :

1. IMAP, SMTP et ManageSieve vers les serveurs que l'utilisateur a configurés ;
2. le mandataire d'images `/mail/image`, dont l'URL est **signée par le serveur** (`proxyMAC`) ;
3. la synchronisation CardDAV, dont l'adresse est validée à l'enregistrement.

Le garde refuse par défaut les réseaux privés (`block_private_networks`), avec une liste blanche explicite
(`allowed_private_hosts`) pour le Dovecot de test sur le réseau local. Sans lui, le bouton « Test » du
formulaire de compte serait un scanner de ports interne offert à quiconque peut s'inscrire.

**N'ajoutez jamais un `http.Client` ordinaire** pour une adresse venue de l'utilisateur ou d'un message :
passez par `s.guard.HTTPClient(timeout)`.

---

## 8. Tests

| Niveau | Commande | Ce qu'il couvre |
|---|---|---|
| unitaires | `make test` | les 17 paquets ; `internal/web` monte un serveur réel et parle HTTP |
| concurrence | `make test-race` | détecteur de courses — avant toute proposition |
| JavaScript | `make test-js` | jsdom sur les pages **réellement produites** par le serveur |
| bout en bout | `make test-e2e` | Chromium : composition, filtres, dossiers, contacts |
| intégration | `make sieve-up` puis `make test` | Dovecot réel : IMAP et ManageSieve |

Les tests de `internal/web` partent de `newEnv(t, nil)`, qui crée un `data_dir` temporaire et un serveur
complet : un test peut donc s'inscrire, se connecter, et vérifier qu'une donnée **ne se lit pas en clair sur
le disque** (`parcourirFichiers`). C'est le bon niveau pour prouver une propriété de confidentialité.

---

## 9. Points de couture connus

À savoir avant de refondre, signalés par l'audit de qualité :

- `internal/web/mail.go:home` et `buildMessageView` sont longues : elles enchaînent plusieurs accès IMAP dont
  l'**ordre compte** (le message du volet est lu avant les compteurs, parce que l'ouverture le marque comme
  lu). Découper sans conserver cet ordre introduit un compteur faux.
- `internal/web/compose.go` porte à la fois la préparation de la page et la construction du message ; les deux
  partagent la structure de formulaire.
- `internal/sieve` doit relire des scripts écrits par RainLoop (valeurs `From`, `Contains`, `MoveTo`…) :
  les constantes sont celles de RainLoop, pas des noms choisis. Ne les renommez pas.
