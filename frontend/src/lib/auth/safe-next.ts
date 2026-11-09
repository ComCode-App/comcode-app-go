/**
 * Redirect-after-auth guard (TEC-1103). Every `?next=` / `callbackUrl` value
 * that comes from the URL goes through `safeNextPath` before it reaches
 * `router.replace`, `signIn({ callbackUrl })` or a link.
 *
 * Only same-origin relative paths are accepted. Browsers treat a backslash
 * like "/" and drop tabs/newlines while parsing, so "/\evil.com" or
 * "/<TAB>/evil.com" would turn into protocol-relative redirects. The value is
 * also checked once more after one round of percent-decoding, so "/%2f%2f"
 * and "/%5c" variants are rejected too.
 *
 * This module has no imports on purpose: `safe-next.test.mjs` transpiles it
 * standalone under `node --test`.
 */

/** Paths that would bounce straight back into the sign-in flow or hit the API. */
const BLOCKED_PATHS = [
  "/login",
  "/register",
  "/platform/login",
  "/platform/register",
];
const BLOCKED_PREFIXES = ["/api/"];

const CONTROL_OR_BACKSLASH = /[\\\u0000-\u001f\u007f]/;
const PROBE_ORIGIN = "http://next-path.invalid";

/** One round of %XX decoding that never throws (invalid escapes stay as-is). */
function percentDecodeOnce(value: string) {
  return value.replace(/%([0-9a-f]{2})/gi, (_, hex: string) =>
    String.fromCharCode(parseInt(hex, 16)),
  );
}

function isRelativePath(value: string) {
  return (
    value.startsWith("/") &&
    !value.startsWith("//") &&
    !CONTROL_OR_BACKSLASH.test(value)
  );
}

function isBlockedTarget(value: string) {
  let path = value.split(/[?#]/, 1)[0].toLowerCase();
  if (path.length > 1 && path.endsWith("/")) path = path.replace(/\/+$/, "");
  if (BLOCKED_PATHS.includes(path)) return true;
  return BLOCKED_PREFIXES.some(
    (prefix) => path === prefix.slice(0, -1) || path.startsWith(prefix),
  );
}

function staysSameOrigin(value: string) {
  try {
    return new URL(value, PROBE_ORIGIN).origin === PROBE_ORIGIN;
  } catch {
    return false;
  }
}

/**
 * Returns `raw` when it is a safe same-origin path (query and hash kept),
 * otherwise `fallback`.
 */
export function safeNextPath<F extends string | null>(
  raw: string | null | undefined,
  fallback: F,
): string | F {
  if (typeof raw !== "string" || raw.length === 0 || raw.length > 2048) {
    return fallback;
  }
  if (!isRelativePath(raw)) return fallback;

  const decoded = percentDecodeOnce(raw);
  if (!isRelativePath(decoded)) return fallback;

  if (isBlockedTarget(raw) || isBlockedTarget(decoded)) return fallback;
  if (!staysSameOrigin(raw)) return fallback;

  return raw;
}

/** `/login?next=<path>` for a deep link; plain login path when the link is unsafe. */
export function loginWithNext(
  loginPath: string,
  next: string | null | undefined,
) {
  const safe = safeNextPath(next, null);
  return safe ? `${loginPath}?next=${encodeURIComponent(safe)}` : loginPath;
}
