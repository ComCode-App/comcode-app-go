/**
 * Human-readable labels for analytics event types, subject types and categories (TEC-856).
 *
 * Event keys are free-form (`[a-z0-9._-]+`, platform admins can register new ones), so a
 * catalog lookup alone can leak raw keys like `analytics.event_type.foo.bar` into the UI.
 * Every helper here resolves catalog → definition name → prettified key, never the raw slug.
 */

type Translate = (
  key: string,
  params?: Record<string, string | number>,
) => string;

/** Catalog lookup; `undefined` when the key is missing (translate() echoes the key back). */
function lookup(t: Translate, key: string): string | undefined {
  const label = t(key);
  return label === key ? undefined : label;
}

/** `social.instagram.click` → `Social instagram click`. Last-resort fallback for unknown keys. */
export function prettifyEventKey(key: string): string {
  const words = key
    .split(/[._-]+/)
    .filter(Boolean)
    .join(" ");
  if (!words) return key;
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/**
 * Singular label for one event type (raw event rows, definitions, filters).
 * `definitionName` is the event definition's display name, used for custom types.
 */
export function eventTypeLabel(
  t: Translate,
  type: string,
  definitionName?: string | null,
): string {
  const name = definitionName?.trim();
  return (
    lookup(t, `analytics.event_type.${type}`) ??
    (name && name !== type ? name : undefined) ??
    prettifyEventKey(type)
  );
}

/** Plural "count of" label for breakdown lists (e.g. "Phone clicks"); falls back to the singular label. */
export function eventCountLabel(
  t: Translate,
  type: string,
  definitionName?: string | null,
): string {
  return (
    lookup(t, `analytics.event.${type}`) ??
    eventTypeLabel(t, type, definitionName)
  );
}

/** Label for an event subject type (`profile`, `route`, `card`, …). */
export function subjectTypeLabel(t: Translate, type: string): string {
  return lookup(t, `analytics.subject_type.${type}`) ?? prettifyEventKey(type);
}

/** Label for an event definition category (`card`, `contact`, `billing`, …). */
export function eventCategoryLabel(t: Translate, category: string): string {
  return (
    lookup(t, `analytics.event_category.${category}`) ??
    prettifyEventKey(category)
  );
}
