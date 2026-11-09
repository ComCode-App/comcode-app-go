/** Local-calendar day helpers: the date pickers hold `yyyy-MM-dd`, the raw events API wants RFC 3339. */

function pad(n: number) {
  return String(n).padStart(2, "0");
}

/** `yyyy-MM-dd` of a date in the viewer's local calendar. */
export function localDay(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** Start of a local day as an RFC 3339 instant; undefined for an empty or invalid day. */
export function dayStartIso(day?: string): string | undefined {
  if (!day) return undefined;
  const d = new Date(`${day}T00:00:00`);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

/** End of a local day (inclusive, last millisecond) as an RFC 3339 instant. */
export function dayEndIso(day?: string): string | undefined {
  if (!day) return undefined;
  const d = new Date(`${day}T23:59:59.999`);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

/** `yyyy-MM-dd` → `yyyy-MM-dd` of the day `days` before today (local). */
export function daysAgo(days: number, now = new Date()): string {
  return localDay(
    new Date(now.getFullYear(), now.getMonth(), now.getDate() - days),
  );
}
