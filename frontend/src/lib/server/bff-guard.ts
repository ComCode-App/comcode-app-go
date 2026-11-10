/**
 * Request checks for the same-origin `/api/v1/*` BFF (TEC-1102) and the
 * cookie-free mobile gateway `/api/mobile/v1/*` (TEC-1178). Pure functions
 * (no Next imports) so they are unit-tested in `bff-guard.test.mjs`.
 */

/** JSON error in the Go API's envelope shape (`backend/API_CONVENTIONS.md`). */
export function bffErrorResponse(
  status: number,
  code: string,
  message: string,
  requestId?: string,
): Response {
  const body: Record<string, unknown> = {
    success: false,
    error: { code, message },
  };
  if (requestId) body.meta = { request_id: requestId };
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
      ...(requestId ? { "X-Request-ID": requestId } : {}),
    },
  });
}

/**
 * Path allowlist. The BFF forwards only to the Go `/v1` base, so a segment that
 * could change the path once the upstream URL is normalised (`..`, `.`, encoded
 * slashes / dots, backslashes, query or fragment markers, control characters)
 * is refused, as are the server-to-server `/v1/internal/*` adapter routes the
 * browser must never reach (Auth.js adapter calls them with `fetchUpstream`).
 *
 * Next decodes each catch-all segment once, so `%2F` arrives as `/` and `%2e%2e`
 * as `..`; a remaining `%` means a double-encoded value and is refused too.
 */
export function isForbiddenProxyPath(segments: readonly string[]): boolean {
  if (segments.length === 0) return true;
  for (const seg of segments) {
    if (seg === "" || seg === "." || seg === "..") return true;
    if (/[/\\%?#\u0000-\u001f\u007f]/.test(seg)) return true;
  }
  return segments[0]!.toLowerCase() === "internal";
}

/**
 * Mobile gateway (`/api/mobile/v1/*`, TEC-1178) path check: everything the
 * cookie BFF refuses (`isForbiddenProxyPath`: traversal, encoded separators,
 * `/v1/internal/*`), plus Go routes only a server or a payment provider calls:
 *
 * - `public/payment/**`: iyzico / PayTR callbacks. The provider posts them to
 *   the web origin (`/api/v1/public/payment/...`); the app never does.
 * - `auth/qr/exchange`: the QR sign-in exchange is called by the web server
 *   that showed the QR code (design: docs/design/mobile.md §3), never by the
 *   phone that approves it.
 *
 * Compared case-insensitively so a differently cased path cannot slip past
 * this list (Go routes are case-sensitive, so it would 404 there anyway).
 */
export function isMobileGatewayRefusedPath(
  segments: readonly string[],
): boolean {
  if (isForbiddenProxyPath(segments)) return true;
  const path = segments.join("/").toLowerCase();
  if (path === "public/payment" || path.startsWith("public/payment/")) {
    return true;
  }
  return MOBILE_REFUSED_PATHS.has(path);
}

const MOBILE_REFUSED_PATHS = new Set(["auth/qr/exchange"]);

/**
 * Mutating paths that are POSTed cross-site on purpose, so they skip the CSRF
 * origin check. Keep this list minimal and comment every entry.
 *
 * - `public/payment/iyzico/callback`: after 3-D Secure / checkout, iyzico's page
 *   auto-submits a form to our callback (top-level cross-site POST, Origin is
 *   iyzico's). The Go handler verifies the payment with iyzico server-to-server
 *   using the posted token; it does not trust the browser.
 * - `public/payment/paytr/callback`: PayTR's server-to-server notification
 *   (normally no Origin / Sec-Fetch-Site, listed so a proxy that adds one cannot
 *   break payments). The Go handler checks PayTR's hash.
 *
 * Both are also routed through dedicated handlers in
 * `app/api/v1/public/payment/{iyzico,paytr}/callback/route.ts`. Neither uses the
 * session cookie: `isAuthPublicTokenPath` sends them without a Bearer token.
 * ComCode has no other inbound webhooks (`/v1/webhooks` is the authenticated
 * management API for outgoing webhooks, used same-origin from the panel).
 */
const CSRF_EXEMPT_PATHS = new Set([
  "public/payment/iyzico/callback",
  "public/payment/paytr/callback",
]);

export function isCsrfExemptPath(path: string): boolean {
  return CSRF_EXEMPT_PATHS.has(path);
}

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

function originOf(value: string | null | undefined): string | null {
  if (!value) return null;
  try {
    const url = new URL(value.trim());
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    return url.origin;
  } catch {
    return null;
  }
}

function firstHeaderValue(value: string | null): string | null {
  const first = value?.split(",")[0]?.trim();
  return first ? first : null;
}

/**
 * Origins the app is served from: the configured public URLs (`AUTH_URL`,
 * `PUBLIC_FRONTEND_URL`) plus the host the request arrived on. Behind Traefik
 * the host is in `X-Forwarded-Host` / `X-Forwarded-Proto`; locally it is `Host`.
 */
export function allowedAppOrigins(
  headers: Headers,
  env: Record<string, string | undefined> = process.env,
): Set<string> {
  const out = new Set<string>();
  for (const raw of [env.AUTH_URL, env.PUBLIC_FRONTEND_URL]) {
    const origin = originOf(raw);
    if (origin) out.add(origin);
  }
  const host =
    firstHeaderValue(headers.get("x-forwarded-host")) ??
    firstHeaderValue(headers.get("host"));
  if (host) {
    const proto = firstHeaderValue(headers.get("x-forwarded-proto"));
    const protos = proto ? [proto] : ["https", "http"];
    for (const p of protos) {
      const origin = originOf(`${p}://${host}`);
      if (origin) out.add(origin);
    }
  }
  return out;
}

/**
 * CSRF defence for cookie-authenticated BFF mutations (the Auth.js session
 * cookie is SameSite=Lax, which still lets top-level cross-site form POSTs
 * through in some browsers and does not cover same-site sibling hosts).
 *
 * A non-safe method is refused when the browser says `Sec-Fetch-Site:
 * cross-site`, or when an `Origin` header is present and is not one of the
 * app's own origins (`null` included). Requests with neither header are
 * non-browser clients and pass; they carry no ambient browser cookie risk.
 */
export function isCrossSiteMutation(
  request: Request,
  allowedOrigins: Set<string>,
): boolean {
  if (SAFE_METHODS.has(request.method.toUpperCase())) return false;
  const site = request.headers.get("sec-fetch-site")?.trim().toLowerCase();
  if (site === "cross-site") return true;
  const origin = request.headers.get("origin");
  if (origin === null) return false;
  const normalized = originOf(origin);
  return normalized === null || !allowedOrigins.has(normalized);
}

/**
 * Decode a JWT's `exp` (seconds) without verifying it. Only used to decide
 * whether to refresh before a request whose body cannot be replayed; the Go
 * API still verifies every token.
 */
export function jwtExpiresAt(token: string | null | undefined): number | null {
  const payload = token?.split(".")[1];
  if (!payload) return null;
  try {
    const json = JSON.parse(
      Buffer.from(
        payload.replace(/-/g, "+").replace(/_/g, "/"),
        "base64",
      ).toString("utf8"),
    ) as { exp?: unknown };
    return typeof json.exp === "number" ? json.exp : null;
  } catch {
    return null;
  }
}
