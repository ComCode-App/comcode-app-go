/**
 * Browser error tracking entry (TEC-952). This file is tiny on purpose: it ships with every
 * page, visitor pages included, while the Sentry SDK (`./sdk`) is a separate chunk.
 *
 * - Visitor pages (`/{username}`, `/c/*`, `/s/*`): `installErrorCatcher()` only adds two window
 *   listeners. Nothing is fetched until the first uncaught error; then the runtime config is read
 *   and, when tracking is on, the SDK chunk loads with a minimal setup and sends the error.
 * - Panel / admin: `enableFullErrorTracking()` loads the SDK on mount with breadcrumbs and the
 *   request id of each API call.
 *
 * The DSN comes from `GET /v1/public/error-tracking` at runtime (platform "Hata izleme" setting),
 * never from the build.
 */
import { apiConfig } from "@/config/api";

export type ErrorTrackingMode = "lazy" | "full";

export type ErrorTrackingConfig = {
  dsn: string;
  environment?: string;
  release?: string;
};

type Sdk = typeof import("./sdk");

const CONFIG_URL = `${apiConfig.baseUrl}/v1/public/error-tracking`;
/** Errors kept while the SDK chunk loads. */
const MAX_PENDING = 20;

let installed = false;
let listening = false;
let wanted: ErrorTrackingMode = "lazy";
let pending: unknown[] = [];
let loading: Promise<Sdk | null> | null = null;

function onError(event: ErrorEvent) {
  capture(event.error ?? event.message);
}

function onRejection(event: PromiseRejectionEvent) {
  capture(event.reason);
}

function listen(on: boolean) {
  if (typeof window === "undefined" || listening === on) return;
  if (on) {
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
  } else {
    window.removeEventListener("error", onError);
    window.removeEventListener("unhandledrejection", onRejection);
  }
  listening = on;
}

async function fetchConfig(
  fresh: boolean,
): Promise<ErrorTrackingConfig | null> {
  try {
    const res = await fetch(CONFIG_URL, {
      headers: { Accept: "application/json" },
      credentials: "omit",
      cache: fresh ? "no-store" : "default",
    });
    if (!res.ok) return null;
    const body = (await res.json()) as {
      data?: { enabled?: boolean } & Partial<ErrorTrackingConfig>;
    };
    const data = body.data;
    if (!data?.enabled || !data.dsn) return null;
    return {
      dsn: data.dsn,
      environment: data.environment,
      release: data.release,
    };
  } catch {
    return null;
  }
}

function load(fresh = false): Promise<Sdk | null> {
  loading ??= (async () => {
    const config = await fetchConfig(fresh);
    if (!config) {
      // Tracking is off: stop listening so later errors cost nothing.
      listen(false);
      pending = [];
      return null;
    }
    const sdk = await import("./sdk");
    sdk.start(config, wanted);
    // The SDK's own global handlers take over from here.
    listen(false);
    const queued = pending;
    pending = [];
    for (const err of queued) sdk.capture(err);
    return sdk;
  })().catch(() => null);
  return loading;
}

function capture(err: unknown) {
  if (pending.length < MAX_PENDING) pending.push(err);
  void load();
}

/** Adds the lazy window listeners. Safe to call more than once. */
export function installErrorCatcher() {
  if (installed || typeof window === "undefined") return;
  installed = true;
  listen(true);
}

/** Panel and admin pages: load the full SDK now (no-op when tracking is off). */
export async function enableFullErrorTracking() {
  installErrorCatcher();
  wanted = "full";
  const sdk = await load();
  sdk?.upgrade("full");
}

/** Reports an error caught by an error boundary (it never reaches the window listeners). */
export function reportError(err: unknown) {
  capture(err);
}

/**
 * Re-reads the config (after the admin changed it) and restarts the SDK with it.
 * Returns whether tracking is on.
 */
export async function reloadErrorTracking(): Promise<boolean> {
  const previous = await loading;
  await previous?.stop();
  loading = null;
  listen(true);
  return (await load(true)) !== null;
}

/** Sends a test error from this browser. Returns the event id, or null when tracking is off. */
export async function sendTestError(message: string): Promise<string | null> {
  const sdk = await load();
  return sdk ? sdk.captureTest(message) : null;
}
