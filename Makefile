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

# Pose une image de fond pour le thème « rainloop » dans le volume de données (dossier externe des thèmes).
theme-background:
	@test -f "$(FILE)" || { echo "usage : make theme-background FILE=chemin/image.jpg"; exit 1; }
	podman run --rm -v wmail-go_wmail-go-data:/data -v "$(abspath $(FILE))":/in:ro,Z --user 0 docker.io/library/alpine:3 sh -c 'mkdir -p /data/themes/rainloop && cp /in /data/themes/rainloop/background.jpg && chown -R 65532:65532 /data/themes && chmod -R u=rwX,go=rX /data/themes'

clean:
	$(COMPOSE) --profile dev --profile test --profile cli down -v
