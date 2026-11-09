/** The API caches the overview for 60s; refetch on the same cadence. */
export const OVERVIEW_STALE_MS = 60_000;

/** Selectable periods of the conversion funnel, in days (the API accepts only these). */
export const FUNNEL_PERIODS = [7, 30, 90, 365] as const;
export const DEFAULT_FUNNEL_PERIOD = 30;

/** Order statuses that no longer need work (the rest count as open). */
export const CLOSED_ORDER_STATUSES = ["delivered", "cancelled", "refunded"];
