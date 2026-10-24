# Journal des changements

Le projet n'a pas encore de version publiée : cette page résume l'état et les étapes marquantes.
Format inspiré de « Keep a Changelog » ; les dates sont celles du dépôt.

## Non publié

### Ajouté
- Webmail complet : liste (tri, recherche à la RainLoop, conversations, pagination, glisser-déposer, raccourcis
  clavier, actualisation automatique), lecture (HTML assaini, texte brut, pièces jointes, invitations iCalendar,
  images distantes bloquées ou mandatées, photo du correspondant), composition (texte et HTML, pastilles
  d'adresses, brouillons automatiques, modèles, signatures, accusés de lecture, transfert groupé).
- Comptes chiffrés sans mot de passe stocké, clé de récupération, authentification en deux étapes (TOTP) avec
  codes de secours.
- Plusieurs boîtes IMAP/SMTP par compte, identités multiples, dossiers système, filtres Sieve (ManageSieve).
- Carnet d'adresses chiffré : vCard, CSV, photos, autocomplétion, synchronisation CardDAV.
- Thèmes (dont « DarkShine » reprenant RainLoop), français et anglais, fuseau horaire par utilisateur,
  dispositions 3 volets / aperçu en bas / liste seule, réglages enregistrés à la volée.
- Déploiement derrière nginx avec terminaison TLS, sauvegarde et restauration du volume de données.
- Documentation : `README.md`, `SECURITY.md`, `CONTRIBUTING.md`, cahier des charges, matrice de test (165 points),
  analyse fonction par fonction de RainLoop 1.17 dans `docs/audit-rainloop/`.

### Sécurité
- Chiffrement au repos (argon2id + XChaCha20-Poly1305), identifiants de connexion masqués par HMAC.
- Assainissement HTML, corps de message isolé (CSP sans script + bac à sable), pixels espions retirés.
- Jeton CSRF, cookie `__Host-` `SameSite=Strict`, en-têtes CSP/HSTS/Permissions-Policy, garde anti-SSRF,
  limitation de débit par IP et par identifiant.
- Tests de sécurité automatiques (message piégé, CSRF, SSRF, gestionnaires en ligne).
