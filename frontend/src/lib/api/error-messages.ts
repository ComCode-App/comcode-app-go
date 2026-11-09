/**
 * The one place that turns an API error code into a localized message (TEC-1110).
 *
 * Every backend error code (`backend/pkg/response/response.go` and the handler `Code*` constants)
 * maps to a locale key here; `error-messages.test.mjs` fails when a backend code has no entry or a
 * key is missing in en/tr. Kept dependency-free so the node test can load it.
 */

/** Error code → locale key. Existing keys are reused so their texts stay the same. */
export const API_ERROR_MESSAGE_KEYS: Readonly<Record<string, string>> = {
  // Generic codes (pkg/response).
  VALIDATION_ERROR: "common.error_validation",
  UNAUTHENTICATED: "auth.session_expired",
  FORBIDDEN: "common.error_forbidden",
  NOT_FOUND: "common.error_not_found",
  CONFLICT: "errors.codes.conflict",
  INTERNAL_ERROR: "common.error_server",
  PAYLOAD_TOO_LARGE: "errors.upload_too_large",
  SVG_UNSAFE_CONTENT: "errors.upload_svg_unsafe",
  RATE_LIMITED: "errors.codes.rate_limited",
  PLAN_LIMIT_EXCEEDED: "entitlements.limit_reached_toast",
  // Auth.
  INVALID_CREDENTIALS: "errors.codes.invalid_credentials",
  INVALID_RESET_CODE: "errors.codes.invalid_reset_code",
  INVALID_VERIFICATION_CODE: "errors.codes.invalid_verification_code",
  PASSWORD_RESET_FAILED: "errors.codes.password_reset_failed",
  NO_TENANT_MEMBERSHIP: "errors.codes.no_tenant_membership",
  STEP_UP_REQUIRED: "errors.codes.step_up_required",
  MFA_REQUIRED: "errors.codes.mfa_required",
  MFA_NOT_ENROLLED: "errors.codes.mfa_not_enrolled",
  INVALID_MFA_CODE: "errors.codes.invalid_mfa_code",
  LAST_SIGN_IN_METHOD: "auth.oauth.last_sign_in_method",
  INVALID_EMAIL_CODE: "errors.codes.invalid_email_code",
  REFRESH_RACE: "errors.codes.refresh_race",
  REALTIME_DISABLED: "errors.codes.realtime_disabled",
  // Routes / public profile.
  INVALID_PASSCODE: "errors.codes.invalid_passcode",
  // Company invites (accounts module).
  INVITE_EXPIRED: "accounts.invite_page.expired",
  INVITE_CLOSED: "accounts.invite_page.closed",
  INVITE_EMAIL_MISMATCH: "accounts.invite_page.email_mismatch",
  INVITE_IMPERSONATION: "accounts.invite_page.impersonating",
  // Leads.
  SELF_EXCHANGE: "errors.codes.self_exchange",
  INVALID_DELETE_TOKEN: "errors.codes.invalid_delete_token",
  // Billing refunds.
  REFUND_FAILED: "errors.codes.refund_failed",
  REFUND_IN_PROGRESS: "errors.codes.refund_in_progress",
  NOTHING_TO_REFUND: "errors.codes.nothing_to_refund",
  // Webhooks.
  WEBHOOK_LIMIT_REACHED: "errors.codes.webhook_limit_reached",
  DELIVERY_PENDING: "errors.codes.delivery_pending",
  // Platform support (cards / routes).
  CARD_RETIRED: "errors.codes.card_retired",
  CARD_HAS_NO_ROUTE: "errors.codes.card_has_no_route",
  CARD_NOT_ACTIVATED: "errors.codes.card_not_activated",
  PATH_NOT_QUARANTINED: "errors.codes.path_not_quarantined",
};

/**
 * Codes whose server message is usually more specific than the code text ("username is taken"
 * rather than "conflict"). For these the server message wins when it is already in the active
 * language; the code text is used otherwise.
 */
export const GENERIC_ERROR_CODES: ReadonlySet<string> = new Set([
  "VALIDATION_ERROR",
  "CONFLICT",
  "REFUND_FAILED",
]);

/** Locale key for an HTTP status when neither the code nor the server gave a usable text. */
export function httpFallbackKey(status: number): string {
  if (status === 401) return "auth.session_expired";
  if (status === 403) return "common.error_forbidden";
  if (status === 404) return "common.error_not_found";
  if (status === 400 || status === 422) return "common.error_validation";
  if (status >= 500) return "common.error_server";
  return "common.error_generic";
}

/** Looks up a key; returns undefined when the active catalogs do not have it. */
export type ErrorTranslator = (key: string) => string | undefined;

const TURKISH_CHARS = /[çğıöşüÇĞİÖŞÜ]/;

/** Rough language check for server messages (the API answers in English). */
export function isInLocale(message: string, locale: string): boolean {
  return (locale === "tr") === TURKISH_CHARS.test(message);
}

function codeOf(error: unknown): string | undefined {
  if (!error || typeof error !== "object") return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : undefined;
}

/**
 * Locale key for an API error (anything with a `code`), or null. `overrides` lets a screen use
 * its own wording for some codes (e.g. the invite page's RATE_LIMITED text).
 */
export function apiErrorMessageKey(
  error: unknown,
  overrides?: Readonly<Record<string, string>>,
): string | null {
  const code = codeOf(error);
  if (!code) return null;
  return overrides?.[code] ?? API_ERROR_MESSAGE_KEYS[code] ?? null;
}

/**
 * Message shown for an API error, in this order:
 * 1. a generic code (see GENERIC_ERROR_CODES) keeps a server message already in the active language;
 * 2. the localized text for the code;
 * 3. the server message;
 * 4. the localized HTTP fallback (`httpFallbackKey`).
 */
export function resolveApiErrorMessage(input: {
  code: string;
  status: number;
  serverMessage?: string;
  locale: string;
  translate: ErrorTranslator;
}): string {
  const server = input.serverMessage?.trim() || undefined;
  if (
    server &&
    GENERIC_ERROR_CODES.has(input.code) &&
    isInLocale(server, input.locale)
  ) {
    return server;
  }
  const key = API_ERROR_MESSAGE_KEYS[input.code];
  const byCode = key ? input.translate(key) : undefined;
  if (byCode) return byCode;
  if (server) return server;
  return input.translate(httpFallbackKey(input.status)) ?? "Request failed";
}
