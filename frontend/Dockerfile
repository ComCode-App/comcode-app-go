FROM node:22.22.0-alpine3.23 AS base

ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
# Pin pnpm: corepack + pnpm@latest is flaky on Alpine (broken cache path).
ENV PNPM_VERSION=10.15.1

RUN npm install -g "pnpm@${PNPM_VERSION}" \
  && pnpm --version

WORKDIR /app

FROM base AS deps

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS build

ENV NEXT_TELEMETRY_DISABLED=1

# Release provenance (TEC-1094), set by CI as build args; local builds report "dev".
# NEXT_PUBLIC_* values are inlined by `pnpm build` wherever the code reads them, so the
# version can be shown in the UI later without a runtime lookup.
ARG APP_VERSION=dev
ARG APP_COMMIT=
ARG APP_BUILT_AT=
ENV NEXT_PUBLIC_APP_VERSION=${APP_VERSION}
ENV NEXT_PUBLIC_APP_COMMIT=${APP_COMMIT}

COPY --from=deps /app/node_modules ./node_modules
COPY . .

RUN pnpm build

FROM node:22.22.0-alpine3.23 AS runtime

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

# Same provenance for the Node server (process.env) and `docker inspect`.
ARG APP_VERSION=dev
ARG APP_COMMIT=
ARG APP_BUILT_AT=
ENV APP_VERSION=${APP_VERSION}
ENV APP_COMMIT=${APP_COMMIT}
ENV APP_BUILT_AT=${APP_BUILT_AT}

RUN addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app

COPY --from=build --chown=app:app /app/public ./public
COPY --from=build --chown=app:app /app/.next/standalone ./
COPY --from=build --chown=app:app /app/.next/static ./.next/static

USER app

EXPOSE 3000

HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=10 \
  CMD ["node", "-e", "fetch('http://127.0.0.1:3000/health').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"]

CMD ["node", "server.js"]
