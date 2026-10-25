# Avant de publier le dépôt

Liste de vérification à passer une fois, avant le premier `git push` public (Gitea puis GitHub).

## 1. Chemin du module Go

Le module s'appelle aujourd'hui `wmail-go` : suffisant en local, insuffisant pour `go get`. Avant publication :

```bash
make module-path NEW=github.com/<compte>/wmail-go
git diff            # relire : go.mod + imports internes
git commit -am "module : chemin public"
```

Si le dépôt de référence est Gitea, utilisez son chemin (`gitea.example.fr/<compte>/wmail-go`) : c'est l'adresse
par laquelle les autres importeront le code.

## 2. Adresses dans la documentation

- `README.md` : badge d'intégration continue et liens (`github.com/gdegoulet/wmail-go` par défaut).
- `.github/ISSUE_TEMPLATE/config.yml` : lien vers `SECURITY.md`.
- `CONTRIBUTING.md` : commande `git clone`.

## 3. Contenu du dépôt

- [ ] `git ls-files | grep -iE 'key|secret|password|\.pem$|\.crt$'` ne renvoie rien de sensible.
- [ ] `data/`, `deploy/certs/`, `rainloop/`, `rainloop_plugins/`, `scratch/` sont ignorés (`git check-ignore -v`).
- [ ] `config/config.json` et `config/config.https.json` ne contiennent aucun identifiant réel.
- [ ] Les captures de `docs/images/` viennent du serveur de test (aucune adresse ni message personnel).
- [ ] `AVANCEMENT.md` et `MATRICE_DE_TEST.md` : relire, ce sont des documents de travail internes rendus publics.

## 4. Licence et filiation

- [ ] `LICENSE` (AGPL-3.0) présent et inchangé.
- [ ] En-tête de `README.md` : rappel que le projet n'est pas affilié à RainLoop et qu'aucun code n'en est copié.
- [ ] Les glyphes d'icônes proviennent de la police libre livrée avec RainLoop : la mention figure dans le README.

## 5. Dernier contrôle technique

```bash
make fmt vet
make test-race
make test-js
make test-e2e
make build          # l'image doit se construire depuis un dépôt propre
```

## 6. Réglages du dépôt distant

- Branche par défaut `main`, descriptions et sujets renseignés.
- Intégration continue active (`.github/workflows/ci.yml`) ; sur Gitea, Actions compatibles ou pipeline équivalent.
- Signalement des failles : la page de sécurité pointe sur `SECURITY.md`.
- Dependabot activé (`.github/dependabot.yml`) pour les modules Go, les actions et l'image de base.

## 7. Publier l'image sur Docker Hub

- [ ] `make image-publish-essai` passe (barrières Go, construction, version du binaire conforme à l'étiquette).
- [ ] `tools/release/publier.env` existe, en `chmod 600`, avec un *personal access token* en lecture-écriture —
      jamais le mot de passe du compte. Le fichier est ignoré par `.gitignore` : vérifiez-le avec
      `git check-ignore -v tools/release/publier.env`.
- [ ] `make image-publish` : deux étiquettes partent, `AAAAMMJJ-commit` et `latest`.
- [ ] La description de la page Docker Hub est à jour, à partir de [`dockerhub.md`](dockerhub.md)
      (Repository → General → Description) : le script ne la met pas à jour.
- [ ] Le `README.md` du dépôt renvoie bien vers l'image publiée.

Mode d'emploi complet : [`tools/release/README.md`](../tools/release/README.md).
