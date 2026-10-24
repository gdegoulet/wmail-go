# wmail-go

Webmail en Go (IMAP/SMTP, Sieve) qui reprend l'apparence et les fonctions de RainLoop. Licence AGPLv3.

- **Cahier des charges, décisions et état d'avancement** : [`CAHIER_DES_CHARGES.md`](CAHIER_DES_CHARGES.md) (sections 6 et 11 à 13 pour le détail des fonctions).
- **Développement et tests uniquement dans des conteneurs** (Podman ou Docker, jamais `go` sur le poste) : `make help`.

```
make up                  # http://127.0.0.1:8080
make mail-up mail-seed   # GreenMail (IMAP/SMTP de test) et messages de démonstration
make test-race           # tests Go
make sieve-up            # Dovecot de test : active les tests d'intégration IMAP et ManageSieve
make test-sieve test-js  # Pigeonhole réel, jsdom
make test-e2e            # parcours dans Chromium
```

Les données d'un utilisateur sont chiffrées (un fichier par utilisateur, sans mot de passe stocké) ; la configuration se trouve dans
`config/config.json`. Aucun test n'accède à une vraie boîte : uniquement GreenMail et le Dovecot de test.

## Fonctions récentes (voir CAHIER_DES_CHARGES.md, chapitre 14)

Carnet d'adresses chiffré (vCard, CSV, CardDAV), authentification en deux étapes (TOTP), modèles de message, conversations,
recherche avancée, notifications, glisser-déposer, champs d'adresses en pastilles, visionneuse de pièces jointes, images
distantes bloquées ou mandatées, fond d'écran personnel. Tests : `make test-race`, `make test-js`, `make test-e2e`
(composition, filtres, dossiers, contacts), `make sieve-up` puis `make test` pour les tests contre un vrai Dovecot.
