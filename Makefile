# wmail-go – toutes les commandes passent par des conteneurs (jamais de `go` sur l'hôte).
COMPOSE ?= docker compose
DEV     := $(COMPOSE) --profile dev run --rm dev

.PHONY: sieve-up sieve-down test-sieve test-e2e theme-background help build up down logs ps test test-race vet fmt shell mail-up mail-down mail-seed user-add user-list user-passwd user-recover user-delete clean

help:
	@echo "make build     construit l'image wmail-go"
	@echo "make up        démarre wmail-go (http://127.0.0.1:8080)"
	@echo "make down      arrête les conteneurs"
	@echo "make logs      suit les journaux de wmail-go"
	@echo "make test      go test ./... dans le conteneur dev"
	@echo "make test-race go test -race ./... dans le conteneur dev"
	@echo "make test-js    contrôles JavaScript (jsdom) sur les pages réelles"
	@echo "make vet       go vet ./... dans le conteneur dev"
	@echo "make fmt       gofmt -l -w dans le conteneur dev"
	@echo "make shell     shell dans le conteneur dev"
	@echo "make mail-up   démarre GreenMail (IMAP 3143, SMTP 3025)"
	@echo "make test-sieve  vérifie les scripts de filtres avec Pigeonhole (image Dovecot)"
	@echo "make test-e2e   scénario de composition dans Chromium (serveur et GreenMail démarrés)"
	@echo "make mail-seed  dépose des messages de démonstration dans GreenMail (test@localhost et second@localhost)"
	@echo "make sieve-up   démarre un Dovecot de test (ManageSieve) pour les tests d'intégration des filtres ; make sieve-down l'arrête"
	@echo "make mail-down arrête GreenMail"
	@echo "make user-add|user-list|user-passwd|user-recover|user-delete LOGIN=nom   gestion des comptes"
	@echo "make theme-background FILE=image.jpg   pose votre fond d'écran pour le thème rainloop"
	@echo "make clean     supprime conteneurs et volumes (efface les données !)"

build:
	$(COMPOSE) build webmail

up:
	$(COMPOSE) up -d --build webmail

down:
	$(COMPOSE) --profile dev --profile test --profile cli down

logs:
	$(COMPOSE) logs -f webmail

ps:
	$(COMPOSE) ps

test:
	$(DEV) go test ./...

test-race:
	$(DEV) env CGO_ENABLED=1 go test -race -count=1 ./...

test-js:
	./tools/jstest/run.sh

vet:
	$(DEV) go vet ./...

fmt:
	$(DEV) gofmt -l -w .

shell:
	$(DEV) sh

mail-up:
	$(COMPOSE) --profile test up -d greenmail

# Compile et exécute les scripts Sieve générés avec le vrai interpréteur Pigeonhole (image Dovecot, aucune boîte touchée).
test-sieve:
	tools/sievecheck/run.sh

# Scénario de composition dans Chromium (voir tools/e2e/compose.js) : exige make up, make mail-up, make mail-seed et make sieve-up (filtres).
test-e2e:
	docker run --rm --user 0 --network host -e NODE_PATH=/usr/src/app/node_modules -v $(CURDIR)/tools/e2e:/e2e:Z zenika/alpine-chrome:with-puppeteer node /e2e/compose.js /tmp
	docker run --rm --user 0 --network host -e NODE_PATH=/usr/src/app/node_modules -v $(CURDIR)/tools/e2e:/e2e:Z zenika/alpine-chrome:with-puppeteer node /e2e/filters.js /tmp
	docker run --rm --user 0 --network host -e NODE_PATH=/usr/src/app/node_modules -v $(CURDIR)/tools/e2e:/e2e:Z zenika/alpine-chrome:with-puppeteer node /e2e/folders.js /tmp
	docker run --rm --user 0 --network host -e NODE_PATH=/usr/src/app/node_modules -v $(CURDIR)/tools/e2e:/e2e:Z zenika/alpine-chrome:with-puppeteer node /e2e/contacts.js /tmp

mail-seed:
	$(DEV) go run ./tools/seedmail -addr greenmail:3025 -to test@localhost -n 40
	$(DEV) go run ./tools/seedmail -addr greenmail:3025 -to second@localhost -n 6

sieve-up:
	docker compose --profile test up -d dovecot

sieve-down:
	docker compose --profile test rm -sf dovecot

mail-down:
	$(COMPOSE) --profile test stop greenmail

# Gestion des comptes (mêmes données que le serveur). Exemple : make user-add LOGIN=alice
CLI := $(COMPOSE) --profile cli run --rm cli

user-add:
	$(CLI) user add $(LOGIN)

user-list:
	$(CLI) user list

user-passwd:
	$(CLI) user passwd $(LOGIN)

user-recover:
	$(CLI) user recover $(LOGIN)

user-delete:
	$(CLI) user delete $(LOGIN)

# Retire l'image de fond personnelle : le thème revient à l'image livrée (background.svg).
theme-background-reset:
	podman run --rm -v wmail-go_wmail-go-data:/data --user 0 docker.io/library/alpine:3 sh -c 'rm -f /data/themes/rainloop/background.jpg /data/themes/rainloop/background.png'

# --- sauvegarde et restauration du volume de données ---
BACKUP_DIR ?= .
VOLUME ?= wmail-go_wmail-go-data

# Archive datée du volume (comptes chiffrés, carnets, thèmes). Serveur arrêté ou non : les fichiers sont
# écrits de façon atomique, mais une sauvegarde à l'arrêt reste la plus sûre.
backup:
	docker run --rm -v $(VOLUME):/data:ro -v "$(abspath $(BACKUP_DIR))":/sauvegarde --user 0 docker.io/library/alpine:3 		sh -c 'tar czf /sauvegarde/wmail-go-$$(date +%Y%m%d-%H%M%S).tar.gz -C /data .'
	@ls -1t $(BACKUP_DIR)/wmail-go-*.tar.gz | head -1

# Restauration : make restore FILE=wmail-go-20261024-220000.tar.gz (le serveur doit être arrêté).
restore:
	@test -f "$(FILE)" || { echo "usage : make restore FILE=archive.tar.gz"; exit 1; }
	docker run --rm -v $(VOLUME):/data -v "$(abspath $(FILE))":/archive.tar.gz:ro --user 0 docker.io/library/alpine:3 		sh -c 'rm -rf /data/* && tar xzf /archive.tar.gz -C /data && chown -R 65532:65532 /data'
	@echo "volume $(VOLUME) restauré depuis $(FILE)"

# --- déploiement derrière nginx (terminaison TLS, certificat auto-signé) ---
TLS := docker compose -f deploy/docker-compose.yaml
TLS_DEV := docker compose -f deploy/docker-compose.yaml -f deploy/docker-compose.partage.yaml
TLS_HOST ?= localhost

# Certificat auto-signé (825 jours) : deploy/certs/wmail-go.crt et .key. CN et SAN = $(TLS_HOST).
tls-cert:
	mkdir -p deploy/certs
	docker run --rm -v "$(CURDIR)/deploy/certs":/certs:Z docker.io/library/alpine:3 sh -c '		apk add --no-cache openssl >/dev/null && 		openssl req -x509 -newkey rsa:2048 -nodes -days 825 -sha256 			-keyout /certs/wmail-go.key -out /certs/wmail-go.crt 			-subj "/CN=$(TLS_HOST)" -addext "subjectAltName=DNS:$(TLS_HOST),DNS:localhost,IP:127.0.0.1" && 		chmod 644 /certs/wmail-go.crt && chmod 640 /certs/wmail-go.key && \
		chown 101:101 /certs/wmail-go.key /certs/wmail-go.crt'
	@echo "certificat engendré pour $(TLS_HOST) : deploy/certs/wmail-go.crt"

tls-up: deploy/certs/wmail-go.crt
	$(TLS) up -d --build

# Même pile, mais sur les données de « make up » (poste de développement seulement).
tls-up-dev: deploy/certs/wmail-go.crt
	$(TLS_DEV) up -d --build

# Crée un compte dans le volume du déploiement : make tls-user-add LOGIN=alice
tls-user-add:
	$(TLS) run --rm --no-deps webmail user add $(LOGIN)

tls-user-list:
	$(TLS) run --rm --no-deps webmail user list

tls-down:
	$(TLS_DEV) down

tls-logs:
	$(TLS) logs -f --tail=100

tls-ps:
	$(TLS) ps

deploy/certs/wmail-go.crt:
	$(MAKE) tls-cert

# Pose une image de fond pour le thème « rainloop » dans le volume de données (dossier externe des thèmes).
theme-background:
	@test -f "$(FILE)" || { echo "usage : make theme-background FILE=chemin/image.jpg"; exit 1; }
	podman run --rm -v wmail-go_wmail-go-data:/data -v "$(abspath $(FILE))":/in:ro,Z --user 0 docker.io/library/alpine:3 sh -c 'mkdir -p /data/themes/rainloop && cp /in /data/themes/rainloop/background.jpg && chown -R 65532:65532 /data/themes && chmod -R u=rwX,go=rX /data/themes'

clean:
	$(COMPOSE) --profile dev --profile test --profile cli down -v
