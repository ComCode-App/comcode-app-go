/**
 * Sentry browser SDK setup. Only loaded through `import("./sdk")` from `./catcher`, so it is a
 * separate chunk that visitor pages download only after an error (TEC-952).
 */
import {
  captureException,
  captureMessage,
  close,
  dedupeIntegration,
  flush,
  globalHandlersIntegration,
  httpContextIntegration,
  init,
  linkedErrorsIntegration,
  type Breadcrumb,
  type BreadcrumbHint,
  type ErrorEvent,
  type EventHint,
} from "@sentry/browser";

import type { ErrorTrackingConfig, ErrorTrackingMode } from "./catcher";

let current: { config: ErrorTrackingConfig; mode: ErrorTrackingMode } | null =
  null;

function requestIdFromHint(hint?: BreadcrumbHint): string | undefined {
  const response = hint?.response as Response | undefined;
  if (response && typeof response.headers?.get === "function") {
    return response.headers.get("x-request-id") ?? undefined;
  }
  const xhr = hint?.xhr as XMLHttpRequest | undefined;
  if (xhr && typeof xhr.getResponseHeader === "function") {
    return xhr.getResponseHeader("x-request-id") ?? undefined;
  }
  return undefined;
}

/** Puts the API request id (`X-Request-ID`) on fetch/xhr breadcrumbs. */
function beforeBreadcrumb(
  breadcrumb: Breadcrumb,
  hint?: BreadcrumbHint,
): Breadcrumb {
  if (breadcrumb.category === "fetch" || breadcrumb.category === "xhr") {
    const requestId = requestIdFromHint(hint);
    if (requestId)
      breadcrumb.data = { ...breadcrumb.data, request_id: requestId };
  }
  return breadcrumb;
}

/** Tags an event raised from an API error with that request's id (`ApiError.requestId`). */
function beforeSend(event: ErrorEvent, hint: EventHint): ErrorEvent {
  const original = hint.originalException as
    { requestId?: unknown } | null | undefined;
  if (
    original &&
    typeof original === "object" &&
    typeof original.requestId === "string"
  ) {
    event.tags = { ...event.tags, request_id: original.requestId };
  }
  return event;
}

export function start(config: ErrorTrackingConfig, mode: ErrorTrackingMode) {
  current = { config, mode };
  const full = mode === "full";
  init({
    dsn: config.dsn,
    environment: config.environment,
    release: config.release,
    // No user info, cookies, headers, bodies or query strings (same stance as the API side).
    dataCollection: {
      userInfo: false,
      cookies: false,
      httpHeaders: false,
      httpBodies: [],
      urlQueryParams: false,
    },
    // Full: default browser integrations (breadcrumbs, global handlers, ...).
    // Lazy (visitor pages): only what is needed to report errors.
    ...(full
      ? {}
      : {
          defaultIntegrations: false,
          integrations: [
            globalHandlersIntegration(),
            linkedErrorsIntegration(),
            dedupeIntegration(),
            httpContextIntegration(),
          ],
        }),
    beforeBreadcrumb,
    beforeSend,
    initialScope: { tags: { surface: full ? "panel" : "public" } },
  });
}

/** Switches a lazily started SDK to the full setup (an error hit before the panel mounted). */
export function upgrade(mode: ErrorTrackingMode) {
  if (current && current.mode !== mode) start(current.config, mode);
}

export function capture(err: unknown) {
  captureException(err);
}

export async function captureTest(message: string): Promise<string> {
  const id = captureMessage(message, {
    level: "error",
    tags: { source: "platform_test" },
  });
  await flush(3000);
  return id;
}

export async function stop() {
  current = null;
  await close(2000);
}
