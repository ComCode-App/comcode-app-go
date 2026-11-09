/**
 * Sign in with Apple on the web (TEC-1123). Pure functions (no Next imports) so
 * they are unit-tested in `apple-form-post.test.mjs`.
 *
 * Apple returns with `response_mode=form_post`: appleid.apple.com auto-submits a
 * form, i.e. a top-level cross-site POST to `/api/auth/callback/apple`. Browsers
 * leave SameSite=Lax cookies off such a request, so the Auth.js state / PKCE /
 * nonce check cookies were missing and the callback failed with `InvalidCheck`.
 *
 * Fix (ported from the boilerplate, narrowed): only for the two requests of the
 * Apple flow (`signin/apple` sets the cookies, `callback/apple` reads them) and
 * only over https, those check cookies (plus a dedicated callback-url cookie so
 * the `next` deep link survives) are SameSite=None; Secure. The session and
 * CSRF cookies keep Auth.js defaults, and every other provider is unchanged.
 * The check cookies are random, httpOnly and single-use: sending them cross-site
 * lets Apple's POST complete the PKCE/state/nonce check, it does not let another
 * site forge one (it cannot read or set them).
 *
 * CSRF: the BFF origin check (`lib/server/bff-guard.ts`) covers `/api/v1/*` only.
 * For `/api/auth/*`, `isBlockedCrossSiteAuthPost` refuses every cross-site POST
 * except exactly `callback/apple`.
 */

/** Auth.js base path (`app/api/auth/[...nextauth]`). */
export const AUTH_BASE_PATH = "/api/auth";

const APPLE_SIGNIN_PATH = `${AUTH_BASE_PATH}/signin/apple`;
export const APPLE_CALLBACK_PATH = `${AUTH_BASE_PATH}/callback/apple`;

/** The two Auth.js requests that set and read the Apple check cookies. */
export function isAppleFlowPath(pathname: string): boolean {
  const p = pathname.endsWith("/") ? pathname.slice(0, -1) : pathname;
  return p === APPLE_SIGNIN_PATH || p === APPLE_CALLBACK_PATH;
}

type CookieOption = {
  name: string;
  options: {
    httpOnly: true;
    sameSite: "none";
    path: "/";
    secure: true;
    maxAge?: number;
  };
};

function crossSiteCookie(name: string, maxAge?: number): CookieOption {
  return {
    name,
    options: {
      httpOnly: true,
      sameSite: "none",
      path: "/",
      secure: true,
      ...(maxAge ? { maxAge } : {}),
    },
  };
}

/**
 * Auth.js `cookies` override for one request, or undefined for Auth.js defaults.
 * Names match Auth.js' secure defaults (`__Secure-` prefix) so the Apple flow and
 * other providers share them; the callback-url cookie gets its own name so a
 * stale Lax copy written by another flow can never shadow it.
 */
export function appleFormPostCookies(
  pathname: string,
  secure: boolean,
): Record<string, CookieOption> | undefined {
  // SameSite=None requires Secure; Apple only redirects to https anyway.
  if (!secure || !isAppleFlowPath(pathname)) return undefined;
  return {
    pkceCodeVerifier: crossSiteCookie(
      "__Secure-authjs.pkce.code_verifier",
      60 * 15,
    ),
    state: crossSiteCookie("__Secure-authjs.state", 60 * 15),
    nonce: crossSiteCookie("__Secure-authjs.nonce", 60 * 15),
    callbackUrl: crossSiteCookie("__Secure-authjs.apple.callback-url", 60 * 15),
  };
}

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

/**
 * True when a POST (or other non-safe method) to an Auth.js route must be
 * refused: it comes cross-site (`isCrossSite`, from the BFF's
 * `isCrossSiteMutation`) and is not the Apple form_post callback.
 */
export function isBlockedCrossSiteAuthPost(
  method: string,
  pathname: string,
  isCrossSite: boolean,
): boolean {
  if (SAFE_METHODS.has(method.toUpperCase())) return false;
  if (!isCrossSite) return false;
  const p = pathname.endsWith("/") ? pathname.slice(0, -1) : pathname;
  return p !== APPLE_CALLBACK_PATH;
}
