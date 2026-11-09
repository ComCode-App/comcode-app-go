/**
 * Last sign-in method guard (TEC-1105). The API refuses to unlink the last
 * OAuth identity or delete the last passkey of a user without a password
 * (409 LAST_SIGN_IN_METHOD); these helpers let the UI explain it up front.
 */

export const LAST_SIGN_IN_METHOD = "LAST_SIGN_IN_METHOD";

/** i18n key of the localized LAST_SIGN_IN_METHOD message. */
export const LAST_SIGN_IN_METHOD_MESSAGE_KEY = "auth.oauth.last_sign_in_method";

export type SignInMethods = {
  hasPassword: boolean;
  hasPasskey: boolean;
  /** Linked OAuth identities, including providers that are now disabled. */
  identityCount: number;
};

/** True when unlinking one identity would leave the user unable to sign in. */
export function isLastSignInIdentity(methods: SignInMethods): boolean {
  return (
    !methods.hasPassword && !methods.hasPasskey && methods.identityCount <= 1
  );
}

/** True for the API error refusing to remove the last sign-in method. */
export function isLastSignInMethodError(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    (error as { code?: unknown }).code === LAST_SIGN_IN_METHOD
  );
}
