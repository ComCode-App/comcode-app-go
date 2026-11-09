/**
 * Upload error codes the API returns for rejected files (TEC-1097), mapped to
 * locale keys so upload UIs can show a clear, translated warning instead of
 * the raw server message. Kept dependency-free for the node test.
 */
export const UPLOAD_ERROR_KEYS: Record<string, string> = {
  SVG_UNSAFE_CONTENT: "errors.upload_svg_unsafe",
  PAYLOAD_TOO_LARGE: "errors.upload_too_large",
};

/** Locale key for an upload error (anything with an API error `code`), or null. */
export function uploadErrorKey(err: unknown): string | null {
  if (!err || typeof err !== "object") return null;
  const code = (err as { code?: unknown }).code;
  if (typeof code !== "string") return null;
  return UPLOAD_ERROR_KEYS[code] ?? null;
}
