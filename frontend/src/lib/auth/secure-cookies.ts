/**
 * Whether Auth.js cookies carry the `__Secure-` prefix and the Secure flag.
 *
 * Auth.js decides this from the protocol of AUTH_URL (or the request), not from NODE_ENV.
 * The BFF reads the session cookie itself, so both sides use this one answer: a production
 * build served over plain http (local prod compose, E2E) would otherwise write
 * `authjs.session-token` and look for `__Secure-authjs.session-token`.
 */
export function secureAuthCookies(): boolean {
  const authURL = process.env.AUTH_URL?.trim();
  if (authURL) return authURL.startsWith("https://");
  return process.env.NODE_ENV === "production";
}
