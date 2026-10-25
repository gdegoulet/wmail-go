# Publication de l'image

`publier-image.sh` construit l'image de wmail-go à partir des sources du dépôt et la publie sur un registre,
par défaut [`docker.io/gdegoulet/wmail-go`](https://hub.docker.com/r/gdegoulet/wmail-go).

| Étiquette | Exemple | Ce qu'elle désigne |
|---|---|---|
| `AAAAMMJJ-commit` | `20261025-af4debf` | une construction précise, reproductible : date UTC et commit court |
| `latest` | | la dernière construction publiée |

L'étiquette datée est celle à écrire dans un fichier de déploiement : elle ne bouge jamais. `latest` sert à
essayer la dernière version, pas à faire tourner un service.

## Mise en route (une fois)

```sh
cp tools/release/publier.env.exemple tools/release/publier.env
chmod 600 tools/release/publier.env
```

Dans ce fichier, renseignez `REGISTRE_USER` et `REGISTRE_TOKEN`. Sur Docker Hub, le jeton se crée dans
**Account settings → Personal access tokens**, avec la permission **Read & Write** ; n'utilisez pas le mot de
passe du compte, qui donnerait accès à tout. Le fichier est ignoré par `.gitignore` et ne doit jamais être
versionné. Sans jeton, le script compte sur un `docker login` déjà fait.

## Usage

```sh
make image-publish-essai     # construit et contrôle, ne publie rien
make image-publish           # publie après confirmation
```

ou directement :

```sh
./tools/release/publier-image.sh --env=tools/release/publier.env
./tools/release/publier-image.sh --essai      # tout sauf la publication
./tools/release/publier-image.sh --oui        # sans confirmation (tâche programmée)
```

## Ce que fait le script

1. **Refuse un arbre de travail modifié.** Une image publiée doit correspondre à un commit, sinon personne ne
   peut reconstruire ce qui tourne. `ARBRE_SALE=1` passe outre, et l'étiquette de révision porte alors `-sale`.
2. **Met à jour les sources** si `TIRER=1` (`git pull --ff-only`).
3. **Passe les barrières** dans le conteneur de développement : `gofmt -l`, `go vet ./...`, `go test ./...`.
   En échec, rien n'est construit ni publié. `SANS_TESTS=1` les saute (à réserver aux essais).
4. **Construit** l'étage `runtime` du `Dockerfile`, avec la version injectée dans le binaire
   (`-ldflags "-X main.version=…"`) et reportée dans les étiquettes OCI de l'image : source, documentation,
   licence, version, révision, date. Avec Podman, la construction force le format Docker — en format OCI, le
   `HEALTHCHECK` serait perdu.
5. **Contrôle l'image construite** : `wmail-go version` doit répondre exactement l'étiquette attendue. C'est ce
   qui garantit qu'on ne publie pas une image compilée à partir d'autres sources que celles annoncées.
6. **Demande confirmation**, se connecte au registre par l'entrée standard (le jeton n'apparaît ni dans la ligne
   de commande, ni dans les journaux, ni dans l'historique du shell), puis pousse les deux étiquettes.

## Réglages

| Variable | Défaut | Effet |
|---|---|---|
| `IMAGE` | `docker.io/gdegoulet/wmail-go` | nom complet de l'image |
| `REGISTRE_USER` | — | identifiant du registre |
| `REGISTRE_TOKEN` | — | jeton d'accès ; vide, le script compte sur un `docker login` |
| `VERSION` | `AAAAMMJJ-commit` | étiquette de version imposée |
| `ETIQUETTES_SUP` | — | étiquettes supplémentaires, séparées par des espaces (`"stable edge"`) |
| `SANS_LATEST` | `0` | `1` : ne publie pas `latest` |
| `SANS_TESTS` | `0` | `1` : saute les contrôles Go |
| `TIRER` | `0` | `1` : `git pull --ff-only` avant de construire |
| `ARBRE_SALE` | `0` | `1` : autorise un arbre de travail modifié |
| `PLATEFORMES` | — | construction multi-architecture (`linux/amd64,linux/arm64`) |
| `MOTEUR` | `docker` | moteur de conteneurs |

### Multi-architecture

`PLATEFORMES` exige `docker buildx`, absent de Podman : la construction et la publication se font alors en une
seule étape, et l'image n'existe pas en local. Sans buildx, l'image est construite pour l'architecture de la
machine — amd64 dans la plupart des cas.

## Utiliser l'image publiée

```sh
docker run -d --name wmail-go -p 127.0.0.1:8080:8080 \
  -v wmail-go-data:/data \
  -v "$PWD/config/config.json:/config/config.json:ro" \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  docker.io/gdegoulet/wmail-go:20261025-af4debf
```

wmail-go ne sert que du HTTP : placez-le derrière un proxy TLS (voir `deploy/` et la section correspondante du
`README.md`). Créez un compte avec la même image :

```sh
docker run --rm -it -v wmail-go-data:/data \
  -v "$PWD/config/config.json:/config/config.json:ro" \
  docker.io/gdegoulet/wmail-go:20261025-af4debf user add vous
```

## Ce que le script ne fait pas

- Il ne pose **aucune étiquette git** et ne crée aucune version publiée : l'étiquette de l'image est datée, rien
  de plus. Une vraie version viendra avec un `CHANGELOG.md` arrêté et un `git tag`.
- Il ne met pas à jour la description affichée sur Docker Hub : elle se modifie depuis l'interface du site, à
  partir de `docs/dockerhub.md`.
- Il ne signe pas l'image (cosign) et n'engendre pas de nomenclature logicielle (SBOM). À faire le jour où
  d'autres que vous feront tourner cette image.
