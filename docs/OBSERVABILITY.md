# Observability

How to follow one request from the browser to a background job, where failures show up, and the optional error tracking and metrics. Error tracking and metrics are **off by default**; request correlation is always on.

## Request correlation (`X-Request-ID`)

One id follows a request end to end:

1. **Browser → BFF.** `frontend/src/lib/server/bff-proxy.ts` picks one id per browser request: it reuses a well-formed incoming `X-Request-ID` (1–128 chars of `[A-Za-z0-9._:-]`) or mints a UUID. Every Go call made for that request uses it, including the retry after a 401 token refresh. Other server-side upstream calls (`fetchUpstream` / `fetchUpstreamStream` in `src/lib/server/upstream.ts`) do the same.
2. **BFF → Go API.** `middleware.RequestID` (the outermost middleware) applies the same rule (`internal/platform/reqid`). It reuses a valid id and replaces a missing or malformed one. The id is put in the request context and in the `X-Request-ID` response header. The BFF passes that header back to the browser.
3. **Response envelope.** Every `/v1` envelope, success and error, carries `meta.request_id` (`pkg/response`). A support ticket only needs that value.
4. **Logs.** `logging.New` wraps the slog handler, so every record logged with a context (`log.InfoContext(ctx, ...)`, `ErrorContext`, ...) gets a `request_id` attribute. Persisted warn/error logs (platform log viewer, `/v1/platform/logs`) store it in their `request_id` column, which you can search.
5. **Queue.** Tasks enqueued while serving a request carry the id in an Asynq task header: `queue.WithRequestID(ctx, task)` at each `Enqueue` call site (notifications, exports, imports, bulk, search, analytics events). The worker middleware puts the id back into the handler context. Worker logs, and any task the handler enqueues in turn, keep the same id.

So `grep request_id=<id>` (or the log viewer search) shows the HTTP 5xx, the handler error, and the job failures that came from that one click.

When you add a new `Enqueue` call, wrap the task: `q.Enqueue(queue.WithRequestID(ctx, task), asynq.Queue(...))`. Pass queue and retry options to `Enqueue`, not to the task constructor, because `WithRequestID` rebuilds the task.

## API failures

Middleware order (`internal/httpserver/server.go`): `RequestID → ServerErrors → Recover → clientip → CaptureRoute(mux)`.

- `ServerErrors` logs every 5xx as `http_handler_failed` (when the handler used `response.InternalErr(w, r, err, msg)`, so the real error is attached) or `http_server_error`, with method, route, status, `duration_ms`, `error_code` and `request_id`.
- Logs carry the matched **route pattern** (`POST /v1/auth/login`, `GET /v1/invites/{token}`), recorded by `CaptureRoute`, never the raw path, query string or body. Unrouted requests log `route=unmatched`.
- Every 4xx, and every `/v1/auth/*` and `/v1/internal/auth/*` request (success too), writes one INFO `http_request` line with `error_code` (from the envelope) and `reason`: the cause a handler kept with `response.RecordFailure(w, err)` (the auth handlers do; a wrong `AUTH_ADAPTER_SECRET` shows as `auth adapter key missing or does not match`).
- A request the client cancelled (closed tab, navigation) is logged at INFO as `http_request_canceled` with status **499** and counted as 499 in metrics. It is never logged at ERROR or reported, even when the handler answered 5xx because its context was cancelled.
- Error text (`error`, `reason`, panic values) goes through `internal/platform/logredact`: emails, phone numbers, JWTs, bearer credentials and `password=` / `token=`-style pairs are replaced with placeholders. Do not log request bodies, passwords or tokens on purpose.
- `Recover` turns a handler panic into a normal `500 INTERNAL_ERROR` envelope (with `meta.request_id`) instead of a dropped connection. The panic value and stack are logged. `http.ErrAbortHandler` is re-panicked, as net/http expects.

Prefer `response.InternalErr` over `response.Internal` for unexpected errors so the cause is logged and reported.

## Worker failures

The Asynq `ErrorHandler` (`internal/queue/observability.go`) runs after every failed attempt, including handler panics, which Asynq turns into errors. It logs `queue_task_failed` at error level with `type`, `task_id`, `queue`, `retry`, `max_retry`, `final`, `error` and `request_id`. `final=true` means the retries are used up (or the handler returned `asynq.SkipRetry`), so the task is archived. Archived tasks can be inspected or re-run with the Asynq CLI/UI against the same Redis.

## Error tracking (optional, Sentry / GlitchTip)

DSNs are managed from the platform admin panel: **Platform → Ayarlar → Hata izleme** (`/platform/settings/error-tracking`, TEC-952). The page needs `platform.error_tracking.read` / `.write` and a super admin.

| Field | Meaning |
|-------|---------|
| Hata izleme açık | Master switch. Off = neither the API/worker nor the browser sends anything, `SENTRY_DSN` included. |
| Backend DSN | Used by the API and the worker. Empty = fall back to `SENTRY_DSN`. |
| Frontend DSN | Used by the browser SDK. No env fallback. |
| Test buttons | "Backend test hatası" sends one event through the API's active client (`POST /v1/platform/error-tracking/test`); "Frontend test hatası" sends one from the admin's browser. |

The DSNs are stored encrypted with `ENCRYPTION_KEY` in `error_tracking_settings` and never returned by the API (only a masked hint such as `https://***@host/42`). The page shows where the backend DSN comes from (`panel`, `env` or `off`).

| Env | Default | Meaning |
|-----|---------|---------|
| `SENTRY_DSN` | empty | Bootstrap/fallback only: applied at start-up, and used while the panel has no backend DSN. |
| `SENTRY_ENVIRONMENT` | `APP_ENV` | Environment tag (backend and frontend). |
| `SENTRY_RELEASE` | empty | Release tag (e.g. the deployed commit SHA), backend and frontend. |

### Changing a DSN without a restart

`internal/platform/errreport` keeps the current client behind an atomic pointer; `errreport.SetDSN` swaps it (and flushes the old one). `internal/modules/errortracking` keeps the decrypted row in memory in every API and worker process:

1. On start each process loads the row and applies the backend DSN (falling back to `SENTRY_DSN`).
2. `PATCH /v1/platform/error-tracking` saves, reloads the local copy at once, and publishes on the Redis channel `app:<APP_ENV>:errortracking:changed`.
3. Every process subscribes to that channel and reloads. A one-minute periodic reload covers a missed message.

### What is reported

- **API:** 5xx errors recorded through `response.InternalErr`, 500s without a recorded error (sent as a message), and panics (with the panicking stack). Tags: `request_id`, `http.method`, `http.route` (route pattern), `http.status`. 4xx responses and client cancels (499) are never reported.
- **Worker:** task failures only once they are **final**, so a transient error that a retry fixes does not create an issue. Tags: `request_id`, `task.type`, `task.id`, `task.queue`.
- `server_name` is `<APP_NAME>-api` or `<APP_NAME>-worker`. PII is not sent. Request bodies, headers and user data are not attached.

### Frontend

The browser reads its config at runtime from `GET /v1/public/error-tracking` (`{enabled, dsn, environment, release}`, served from memory, `Cache-Control: public, max-age=60`), so no DSN is baked into the build. Events go straight from the browser to the Sentry server. Code: `frontend/src/lib/error-tracking/`.

- **Panel and admin** (`app/(shell)`): `FullErrorTracking` loads `@sentry/browser` on mount with its default integrations: uncaught errors and rejections, console/navigation/click/fetch breadcrumbs. Each fetch breadcrumb carries the API `request_id` (`X-Request-ID`), and an event raised from an `ApiError` is tagged with its `request_id`, so a browser issue links to the API logs. Tag `surface=panel`.
- **Visitor pages** (`/{username}`, `/s/*`, `/c/*`): only `catcher.ts` (about 1 KB gzip) ships. It adds two window listeners and does nothing else. On the first uncaught error it fetches the config and, if tracking is on, loads the SDK chunk (about 30 KB gzip) with a minimal setup and sends the error. A page without errors makes no extra request and downloads no SDK, so the TEC-858 bundle gains are kept. Tag `surface=public`. `src/app/visitor-bundle.test.mjs` fails if a visitor page imports `@sentry/browser` statically.
- Route `error.tsx` boundaries call `reportError(error)`, since React does not rethrow errors a boundary caught.
- No user info, cookies, headers, bodies or query strings are collected (`dataCollection`).

### Next.js server (BFF and NextAuth)

The Next.js server has no Sentry SDK; its failures go to the container log, redacted with `frontend/src/lib/observability/redact.ts` (same rules as the API):

- BFF (`lib/server/bff-proxy.ts`): `[bff] upstream unavailable` (502) and `[bff] <path>` (500) at error level. A browser cancel is answered `499` and logged at info level as `[bff] client closed request`.
- NextAuth: a `logger.error` hook (`lib/observability/auth-logger.ts`) prints one `[auth][error] <type>: …` line. User-caused types (`CredentialsSignin`, `AccessDenied`, …) are warnings.
- Credentials sign-in failures log `[auth][credentials] authorize failed {code, status}`: never the email, password or authenticator code.

## Metrics (optional, Prometheus)

`METRICS_ADDR` (empty = disabled) starts a separate listener that serves only `GET /metrics` in the Prometheus text format. It runs in both the API and the worker process. It is never mounted on the public API router. In `compose.prod.yml` the port is not published and Traefik does not route it, so scrape it over the Docker network (e.g. `METRICS_ADDR=:9091`, target `<prefix>-api:9091` / `<prefix>-worker:9091`). Locally, bind to loopback, e.g. `127.0.0.1:64091`.

The metrics have no extra dependencies and low-cardinality labels:

| Metric | Labels |
|--------|--------|
| `http_requests_total` | `method`, `code` (`2xx`..`5xx`) |
| `http_request_duration_seconds` (histogram) | none |
| `queue_tasks_total` | `type`, `result` (`success` / `failure`) |
| `queue_task_duration_seconds` (histogram) | none |
| `go_goroutines`, `go_memstats_heap_alloc_bytes`, `go_memstats_sys_bytes` | none |

Per-route latency, DB pool stats and queue depth are not exported yet. Queue depth is available from Asynq's own inspector or asynqmon.
