/**
 * Sign-in error handling shared by the password and email-code flows (TEC-1121).
 * Dependency-free so the node test can load it.
 */

/** Go API codes the NextAuth credentials providers pass through as `result.code`. */
export const SIGN_IN_ERROR_CODES = [
  "MFA_REQUIRED",
  "INVALID_MFA_CODE",
  "MFA_NOT_ENROLLED",
  "INVALID_EMAIL_CODE",
  "RATE_LIMITED",
] as const;

export type SignInErrorCode = (typeof SIGN_IN_ERROR_CODES)[number];

export function isSignInErrorCode(code: unknown): code is SignInErrorCode {
  return (
    typeof code === "string" &&
    (SIGN_IN_ERROR_CODES as readonly string[]).includes(code)
  );
}

/**
 * Locale key for a failed sign-in. `signal` is NextAuth's `result.code` (or `result.error`);
 * `fallbackKey` is the method's generic failure text.
 */
export function signInErrorKey(
  signal: string | null | undefined,
  fallbackKey: string,
): string {
  const s = signal ?? "";
  if (s.includes("RATE_LIMITED")) return "auth.login.rate_limited";
  if (s.includes("NO_TENANT_MEMBERSHIP"))
    return "auth.login.no_tenant_membership";
  if (s.includes("INVALID_EMAIL_CODE")) return "auth.email_code.invalid";
  if (s.includes("INVALID_MFA_CODE")) return "auth.totp.login_invalid";
  if (s.includes("MFA_NOT_ENROLLED")) return "auth.totp.login_not_enrolled";
  if (s.includes("MFA_REQUIRED")) return "auth.totp.login_required";
  return fallbackKey;
}

/** The 6-digit email code with spaces / dashes from copy-paste removed, or null when malformed. */
export function normalizeEmailCode(raw: string): string | null {
  const code = raw.replace(/[\s-]/g, "");
  return /^\d{6}$/.test(code) ? code : null;
}
