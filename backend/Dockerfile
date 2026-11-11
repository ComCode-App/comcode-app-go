# syntax=docker/dockerfile:1

FROM golang:1.26.11-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Release provenance (TEC-1094), set by CI; local builds keep the defaults -> "dev".
# APP_RELEASE_JSON_B64 is base64 so multi-line release notes survive as one build arg; it
# replaces the `{}` placeholder that internal/platform/release embeds via go:embed.
# Declared after `go mod download` so the module cache layer stays warm.
ARG APP_VERSION=
ARG APP_COMMIT=
ARG APP_BUILT_AT=
ARG APP_RELEASE_JSON_B64=

RUN if [ -n "${APP_RELEASE_JSON_B64}" ]; then \
      printf '%s' "${APP_RELEASE_JSON_B64}" | base64 -d > internal/platform/release/release.json; \
    fi && \
    REL=github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/release && \
    LDFLAGS="-s -w -X ${REL}.Version=${APP_VERSION} -X ${REL}.Commit=${APP_COMMIT} -X ${REL}.BuiltAt=${APP_BUILT_AT}" && \
    for cmd in server worker seed create-super-admin search-reindex migrate-legacy profile-media-backfill; do \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags="${LDFLAGS}" -o "/out/${cmd}" "./cmd/${cmd}" || exit 1; \
    done

FROM migrate/migrate:v4.19.1 AS migrate

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 appuser

WORKDIR /app

COPY --from=migrate /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=builder /out/server /app/server
COPY --from=builder /out/worker /app/worker
COPY --from=builder /out/seed /app/seed
COPY --from=builder /out/create-super-admin /app/create-super-admin
COPY --from=builder /out/search-reindex /app/search-reindex
COPY --from=builder /out/migrate-legacy /app/migrate-legacy
COPY --from=builder /out/profile-media-backfill /app/profile-media-backfill
COPY migrations /app/migrations
COPY deploy/docker/entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh \
    && chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

# Same provenance as the binaries, for `docker inspect` / shells in the container. The
# server itself reads the values baked in via ldflags, not these.
ARG APP_VERSION=
ARG APP_COMMIT=
ENV APP_VERSION=${APP_VERSION}
ENV APP_COMMIT=${APP_COMMIT}

ENV APP_HTTP_ADDR=:8080
ENV APP_ROLE=server
ENV AUTO_MIGRATE=true

ENTRYPOINT ["/app/entrypoint.sh"]
