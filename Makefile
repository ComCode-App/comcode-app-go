SHELL := /bin/bash

ENV_FILE ?= $(firstword $(wildcard .env))
PROD_ENV_FILE ?= .env.server

define load_env
if [ -f "$(ENV_FILE)" ]; then \
	set -a; \
	while IFS= read -r line || [ -n "$$line" ]; do \
		case "$$line" in \#*|'') continue ;; esac; \
		if [[ "$$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$$ ]]; then \
			export "$${BASH_REMATCH[1]}=$${BASH_REMATCH[2]}"; \
		fi; \
	done < "$(ENV_FILE)"; \
	set +a; \
fi;
endef

FRONTEND_PORT ?= 3000

.PHONY: infra infra-down infra-logs infra-ps migrate-up backend-dev frontend-dev local-dev \
	free-dev-ports prod-env-init prod-env-upgrade prod-env-check env-check compose-env-check prod-config prod-up prod-build-local prod-up-local prod-down tools create-super-admin search-reindex seed check-i18n

infra:
	@if [ -n "$(ENV_FILE)" ]; then \
		docker compose --env-file "$(ENV_FILE)" -f compose.local.yml up -d; \
	else \
		docker compose -f compose.local.yml up -d; \
	fi

infra-down:
	@if [ -n "$(ENV_FILE)" ]; then \
		docker compose --env-file "$(ENV_FILE)" -f compose.local.yml down; \
	else \
		docker compose -f compose.local.yml down; \
	fi

infra-logs:
	docker compose -f compose.local.yml logs -f

infra-ps:
	docker compose -f compose.local.yml ps

migrate-up:
	@$(load_env) $(MAKE) -C backend migrate-up

backend-dev:
	@$(load_env) $(MAKE) -C backend dev

frontend-dev:
	@$(load_env) \
	cd frontend && \
	if [ ! -d node_modules ]; then pnpm install; fi && \
	pnpm dev

# Free host ports bound by dev servers and services (backend, frontend, redis, postgres, seaweedfs, meili, etc.)
free-dev-ports:
	@$(load_env) \
	free_listen_port() { \
		local port="$$1"; \
		local label="$$2"; \
		[ -z "$$port" ] && return 0; \
		if ! command -v lsof >/dev/null 2>&1; then \
			echo "lsof not found; skip :$$port ($$label)"; \
			return 0; \
		fi; \
		local attempt pids pid comm ppid pcomm sig non_docker_killed; \
		for attempt in 1 2; do \
			pids=$$(lsof -nP -iTCP:"$$port" -sTCP:LISTEN -t 2>/dev/null | sort -u); \
			if [ -z "$$pids" ]; then \
				if [ "$$attempt" -eq 1 ]; then echo ":$$port ($$label) is free"; fi; \
				break; \
			fi; \
			non_docker_killed=0; \
			for pid in $$pids; do \
				comm=$$(ps -o comm= -p "$$pid" 2>/dev/null | tr -d '[:space:]'); \
				case "$$comm" in \
					*docker*|*com.docker*|dockerd) \
						if [ "$$attempt" -eq 1 ]; then echo "docker active on :$$port ($$label, pid $$pid)"; fi; \
						continue ;; \
				esac; \
				non_docker_killed=1; \
				ppid=$$(ps -o ppid= -p "$$pid" 2>/dev/null | tr -d '[:space:]'); \
				pcomm=$$(ps -o comm= -p "$$ppid" 2>/dev/null | tr -d '[:space:]'); \
				sig="-TERM"; \
				if [ "$$attempt" -eq 2 ]; then sig="-KILL"; fi; \
				echo "freeing :$$port ($$label) pid $$pid ($$comm) $$sig"; \
				case "$$pcomm" in \
					*air*|*next*|*node*|*pnpm*|*npm*) kill "$$sig" "$$ppid" 2>/dev/null || true ;; \
				esac; \
				kill "$$sig" "$$pid" 2>/dev/null || true; \
			done; \
			if [ "$$non_docker_killed" -eq 0 ]; then break; fi; \
			sleep 0.4; \
		done; \
	}; \
	http_addr="$${APP_HTTP_ADDR:-:8080}"; \
	backend_port="$${http_addr##*:}"; \
	backend_port="$${backend_port:-8080}"; \
	frontend_port="$${PORT:-$${FRONTEND_PORT:-$(FRONTEND_PORT)}}"; \
	frontend_port="$${frontend_port:-3000}"; \
	postgres_port="$${POSTGRES_PORT:-$${DB_PORT:-5432}}"; \
	redis_addr="$${REDIS_ADDR:-:6379}"; \
	redis_port="$${REDIS_PORT:-$${redis_addr##*:}}"; \
	redis_port="$${redis_port:-6379}"; \
	s3_port="$${S3_API_PORT:-8333}"; \
	centrifugo_port="$${CENTRIFUGO_PORT:-8000}"; \
	meili_port="$${MEILI_PORT:-7700}"; \
	mailhog_smtp_port="$${MAILHOG_SMTP_PORT:-$${SMTP_PORT:-1025}}"; \
	mailhog_ui_port="$${MAILHOG_UI_PORT:-8025}"; \
	adminer_port="$${ADMINER_PORT:-8081}"; \
	echo "Checking dev service ports..."; \
	free_listen_port "$$backend_port" "backend api"; \
	free_listen_port "$$frontend_port" "frontend"; \
	free_listen_port "$$redis_port" "redis"; \
	free_listen_port "$$postgres_port" "postgres"; \
	free_listen_port "$$s3_port" "seaweedfs s3"; \
	free_listen_port "$$centrifugo_port" "centrifugo"; \
	free_listen_port "$$meili_port" "meilisearch"; \
	free_listen_port "$$mailhog_smtp_port" "mailhog smtp"; \
	free_listen_port "$$mailhog_ui_port" "mailhog ui"; \
	free_listen_port "$$adminer_port" "adminer"

# Infra in Docker; backend (air) + frontend (pnpm) on the host. No app image builds.
local-dev: infra
	@echo "Waiting for postgres..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
		docker compose -f compose.local.yml exec -T postgres pg_isready -U $${DB_USER:-app} -d $${DB_NAME:-app} >/dev/null 2>&1 && break; \
		sleep 1; \
	done
	@$(load_env) $(MAKE) -C backend migrate-up || true
	@$(MAKE) free-dev-ports
	@echo "Starting backend (air) + frontend (pnpm) — Ctrl+C stops both"
	@trap 'kill 0' INT TERM; \
	$(MAKE) backend-dev & \
	$(MAKE) frontend-dev & \
	wait

# Production env from deploy/env/schema (docs/design/env-core.md).
# Extra flags: make prod-env-init ARGS="--set APP_URL=https://app.example.com --yes"
prod-env-init:
	scripts/prod-env.sh init --file $(PROD_ENV_FILE) $(ARGS)

prod-env-upgrade:
	scripts/prod-env.sh upgrade --file $(PROD_ENV_FILE) $(ARGS)

prod-env-check:
	scripts/prod-env.sh check --file $(PROD_ENV_FILE) $(ARGS)

# CI check (the `env` job): .env.server.example matches the schema, .env.example
# has every schema key, every backend env read is in the schema, prod-env tests
# pass. Fix drift: scripts/prod-env.sh example > .env.server.example
env-check: compose-env-check
	scripts/check-env-sync.sh

# CI check: every env var the backend reads reaches the backend/worker
# containers in compose.prod.yml, or has a decision in deploy/env/compose-env.allow.
compose-env-check:
	scripts/check-compose-env.sh
	scripts/check-compose-env.test.sh

prod-config:
	docker compose --env-file $(PROD_ENV_FILE) -f compose.prod.yml config

# Prebuilt images: CI publishes them to GHCR and the server pulls
# <IMAGE_REPO>-<service>:<IMAGE_TAG> (pull_policy from IMAGE_PULL_POLICY, default
# always). Nothing is built on the host.
prod-up:
	docker compose --env-file $(PROD_ENV_FILE) -f compose.prod.yml up -d

# Local smoke test of the prod stack with images built on this machine
# (compose.build.yml tags them <NAME_PREFIX>-backend:local / -frontend:local).
prod-build-local:
	docker compose --env-file $(PROD_ENV_FILE) -f compose.prod.yml -f compose.build.yml build

prod-up-local:
	docker compose --env-file $(PROD_ENV_FILE) -f compose.prod.yml -f compose.build.yml up -d --build

prod-down:
	docker compose --env-file $(PROD_ENV_FILE) -f compose.prod.yml down

tools:
	@echo "docker:  $$(docker version --format '{{.Server.Version}}' 2>&1)"
	@echo "compose: $$(docker compose version 2>&1)"
	@echo "env:     $(ENV_FILE)"
	@$(MAKE) -C backend tools

# Bootstrap platform super_admin (loads root .env). Override: SA_EMAIL SA_PASSWORD SA_NAME SA_SURNAME
SA_EMAIL    ?= admin@example.com
SA_PASSWORD ?= Password1
SA_NAME     ?= Platform
SA_SURNAME  ?= Admin

create-super-admin:
	@$(load_env) $(MAKE) -C backend create-super-admin \
		SA_EMAIL="$(SA_EMAIL)" \
		SA_PASSWORD="$(SA_PASSWORD)" \
		SA_NAME="$(SA_NAME)" \
		SA_SURNAME="$(SA_SURNAME)"

search-reindex:
	@$(load_env) $(MAKE) -C backend search-reindex

STAGE ?= all

seed:
	@$(load_env) $(MAKE) -C backend seed STAGE="$(STAGE)"

check-i18n:
	node scripts/check-i18n.mjs
