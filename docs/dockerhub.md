# Description à copier sur la page Docker Hub

Le script de publication ne modifie pas la description du dépôt Docker Hub : elle se colle à la main dans
**Repository → General → Description**. Voici le texte proposé.

---

# wmail-go

Webmail en Go (IMAP, SMTP, ManageSieve) qui reprend l'ergonomie et les fonctions de **RainLoop 1.17**, dont son
thème DarkShine. Écrit parce que RainLoop n'est plus maintenu et qu'aucun successeur ne proposait la même
interface.

- **Un binaire statique d'environ 18 Mo**, sans base de données : les messages restent sur votre serveur IMAP.
- **Aucun mot de passe stocké** : un fichier chiffré par utilisateur (argon2id + XChaCha20-Poly1305), ouvert par
  le mot de passe ou une clé de récupération.
- Image *distroless*, utilisateur non root, système de fichiers en lecture seule, aucune capacité Linux.
- Interface sans cadriciel JavaScript, utilisable sans JavaScript ; français et anglais.

Code source, documentation et signalement de bogues : <https://github.com/gdegoulet/wmail-go>
Licence **AGPL-3.0-or-later**.

## Étiquettes

| Étiquette | Ce qu'elle désigne |
|---|---|
| `AAAAMMJJ-commit` (`20261025-af4debf`) | une construction précise, reproductible — **à utiliser en production** |
| `latest` | la dernière construction publiée |

## Démarrer

```sh
docker run -d --name wmail-go -p 127.0.0.1:8080:8080 \
  -v wmail-go-data:/data \
  -v "$PWD/config.json:/config/config.json:ro" \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  gdegoulet/wmail-go:latest
```

Créer un compte (l'inscription devrait rester fermée en service) :

```sh
docker run --rm -it -v wmail-go-data:/data \
  -v "$PWD/config.json:/config/config.json:ro" \
  gdegoulet/wmail-go:latest user add vous
```

Un exemple de `config.json`, un déploiement complet avec nginx et terminaison TLS, et la marche à suivre de
sauvegarde se trouvent dans le dépôt (`config/`, `deploy/`, `README.md`).

## À savoir

- **wmail-go ne sert que du HTTP** : la terminaison TLS revient à un proxy. Derrière un proxy, posez
  `trusted_proxy: true` dans la configuration, sinon les cookies ne seront pas marqués `Secure`.
- Le volume `/data` contient les comptes chiffrés et les carnets d'adresses. Le perdre coûte les comptes, jamais
  le courrier, qui reste sur le serveur IMAP. Sauvegardez-le.
- Les sessions vivent en mémoire : une mise à jour de l'image déconnecte les utilisateurs, sans perte de données.
- `wmail-go` **n'est pas** RainLoop et n'a aucun lien avec ses auteurs. Aucun code de RainLoop n'a été copié :
  seules ses fonctions et son ergonomie ont été étudiées pour être réimplémentées en Go.
