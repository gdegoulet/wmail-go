# wmail-go – image de développement, de compilation et d'exécution.
# Fonctionne avec Docker et Podman (noms d'images complets).

ARG GO_VERSION=1.26

# --- dev : outillage Go, pour compiler et tester (aucun Go sur l'hôte) ---
FROM docker.io/library/golang:${GO_VERSION}-alpine AS dev
RUN apk add --no-cache git make gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
CMD ["sleep", "infinity"]

# --- build : binaire statique ---
FROM dev AS build
# VERSION est reportée dans le binaire (commande « version », réglages, journal de démarrage).
ARG VERSION=dev
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/wmail-go ./cmd/wmail-go
RUN mkdir -p /out/data

# --- runtime : image minimale, utilisateur non root, sans shell ---
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
ARG VERSION=dev
ARG REVISION=inconnue
ARG BUILD_DATE
LABEL org.opencontainers.image.title="wmail-go" \
      org.opencontainers.image.description="Webmail en Go (IMAP, SMTP, ManageSieve) reprenant l'ergonomie de RainLoop 1.17." \
      org.opencontainers.image.source="https://github.com/gdegoulet/wmail-go" \
      org.opencontainers.image.documentation="https://github.com/gdegoulet/wmail-go/blob/main/README.md" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${BUILD_DATE}"
COPY --from=build /out/wmail-go /wmail-go
COPY --from=build --chown=65532:65532 /out/data /data
ENV WMAIL_CONFIG=/config/config.json
VOLUME ["/data"]
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/wmail-go", "healthcheck"]
ENTRYPOINT ["/wmail-go"]
CMD ["serve"]
