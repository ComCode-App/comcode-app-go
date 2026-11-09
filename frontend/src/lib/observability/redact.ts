/**
 * Removes personal data and credentials from free-form text before it is written to the
 * Next.js server log (TEC-1113). Mirrors the API's `logredact` package. Best effort: never
 * log request bodies, passwords, cookies or tokens on purpose.
 */

const REDACTED = "[redacted]";

// PEM private keys (Apple .p8, GitHub App keys), also pasted as one line with
// literal \n; an unterminated block is cut to the end of the text.
const PRIVATE_KEY =
  /-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)/g;
const EMAIL = /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g;
const JWT = /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*/g;
const BEARER = /\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]+/gi;
const SECRET_PAIR =
  /\b((?:access_|refresh_|id_)?token|password|passwd|secret|api[_-]?key|authorization|cookie|code|otp|totp_code|ticket|dsn)(\s*=\s*|"\s*:\s*"?)([^\s"'&,;]+)/gi;
const PHONE_INTL = /\+\d{1,3}[\s.-]?\(?\d{2,4}\)?(?:[\s.-]?\d{2,4}){2,4}/g;
const PHONE_TR = /\b0?5\d{2}[\s.-]?\d{3}[\s.-]?\d{2}[\s.-]?\d{2}\b/g;

export function redactString(input: string): string {
  if (!input) return input;
  return input
    .replace(PRIVATE_KEY, "[private-key]")
    .replace(JWT, "[token]")
    .replace(BEARER, "$1 [token]")
    .replace(SECRET_PAIR, `$1$2${REDACTED}`)
    .replace(EMAIL, "[email]")
    .replace(PHONE_INTL, "[phone]")
    .replace(PHONE_TR, "[phone]");
}

/** A short, redacted description of an unknown thrown value (name + message + cause). */
export function describeError(err: unknown): string {
  if (err instanceof Error) {
    let text = `${err.name}: ${err.message}`;
    const cause = (err as { cause?: unknown }).cause;
    if (cause instanceof Error)
      text += ` | cause: ${cause.name}: ${cause.message}`;
    return redactString(text);
  }
  return redactString(String(err));
}
