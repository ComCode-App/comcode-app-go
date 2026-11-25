/**
 * Profile keywords (TEC-1304). Keywords live in the translation `note` field as one
 * comma-separated string, so the API and the vCard NOTE line stay unchanged.
 *
 * No runtime imports: node --test loads this file directly with type stripping.
 */

export const KEYWORD_MAX_LENGTH = 40;
export const KEYWORDS_MAX_COUNT = 30;

/** Splits a stored note (commas, semicolons or new lines) into unique trimmed keywords. */
export function parseKeywords(value: string | null | undefined): string[] {
  if (!value) return [];
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of value.split(/[,;\n]+/)) {
    const keyword = raw.trim().replace(/\s+/g, " ");
    if (!keyword) continue;
    const key = keyword.toLocaleLowerCase("tr");
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(keyword);
  }
  return out;
}

/** Joins keywords back into the stored note string. */
export function joinKeywords(keywords: readonly string[]): string {
  return keywords.join(", ");
}

/**
 * Adds the keywords typed in `input` (may hold several, comma-separated) to `current`.
 * Limits apply to the new input only, so an existing long keyword (an old free-text
 * note) is never cut.
 */
export function addKeywords(
  current: readonly string[],
  input: string,
): string[] {
  const out = [...current];
  const seen = new Set(current.map((k) => k.toLocaleLowerCase("tr")));
  for (const raw of parseKeywords(input)) {
    if (out.length >= KEYWORDS_MAX_COUNT) break;
    const keyword = raw.slice(0, KEYWORD_MAX_LENGTH).trim();
    const key = keyword.toLocaleLowerCase("tr");
    if (!keyword || seen.has(key)) continue;
    seen.add(key);
    out.push(keyword);
  }
  return out;
}
