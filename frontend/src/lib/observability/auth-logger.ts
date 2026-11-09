import { describeError, redactString } from "@/lib/observability/redact";

/**
 * NextAuth error types caused by the user or the browser (wrong password, denied consent,
 * stale tab). Logged as warnings, not errors.
 */
const EXPECTED_AUTH_ERRORS = new Set([
  "CredentialsSignin",
  "AccessDenied",
  "Verification",
  "OAuthAccountNotLinked",
  "MissingCSRF",
  "UnknownAction",
]);

type AuthLikeError = Error & { type?: string; cause?: unknown };

function causeOf(cause: unknown): unknown {
  // NextAuth wraps the original error as `cause.err`.
  if (cause && typeof cause === "object" && "err" in cause) {
    return (cause as { err?: unknown }).err;
  }
  return cause;
}

/**
 * NextAuth `logger.error` hook (TEC-1113). Replaces the default `[auth][error]` output, which
 * can print adapter responses and provider payloads, with one redacted line. Sign-in
 * callback, adapter and session decode failures are logged at error level.
 */
export function logAuthError(error: Error) {
  const err = error as AuthLikeError;
  const type = err.type || err.name || "AuthError";
  const cause = causeOf(err.cause);
  const line =
    `[auth][error] ${type}: ${redactString(err.message ?? "")}` +
    (cause ? ` | cause: ${describeError(cause)}` : "");
  if (EXPECTED_AUTH_ERRORS.has(type)) {
    console.warn(line);
    return;
  }
  console.error(line);
}

/**
 * Logs a rejected credentials sign-in. Never includes the email, password or authenticator
 * code: only the API error code and HTTP status.
 */
export function logCredentialsFailure(error: unknown) {
  const e = error as { code?: unknown; status?: unknown } | null | undefined;
  const code = typeof e?.code === "string" ? e.code : undefined;
  const status = typeof e?.status === "number" ? e.status : undefined;
  const detail: Record<string, unknown> = { code: code ?? "UNKNOWN" };
  if (status !== undefined) detail.status = status;
  if (code === undefined) detail.error = describeError(error);
  console.warn("[auth][credentials] authorize failed", detail);
}
