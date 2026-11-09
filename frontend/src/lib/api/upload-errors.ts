import { API_ERROR_MESSAGE_KEYS } from "./error-messages";

/**
 * Upload error codes the API returns for rejected files (TEC-1097). Their locale keys live in the
 * central code → key map (`error-messages.ts`); this lets upload UIs tell an upload rejection apart
 * from other errors.
 */
export const UPLOAD_ERROR_KEYS: Readonly<Record<string, string>> = {
  SVG_UNSAFE_CONTENT: API_ERROR_MESSAGE_KEYS.SVG_UNSAFE_CONTENT,
  PAYLOAD_TOO_LARGE: API_ERROR_MESSAGE_KEYS.PAYLOAD_TOO_LARGE,
};

/** Locale key for an upload error (anything with an API error `code`), or null. */
export function uploadErrorKey(err: unknown): string | null {
  if (!err || typeof err !== "object") return null;
  const code = (err as { code?: unknown }).code;
  if (typeof code !== "string") return null;
  return UPLOAD_ERROR_KEYS[code] ?? null;
}
