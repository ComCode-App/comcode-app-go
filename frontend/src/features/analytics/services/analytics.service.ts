import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type AnalyticsSummary = components["schemas"]["AnalyticsSummary"];
export type AnalyticsTimeSeries = components["schemas"]["AnalyticsTimeSeriesResult"];
export type AnalyticsBreakdown = components["schemas"]["AnalyticsBreakdownResult"];
/** Views, QR scans and the QR / NFC / link / legacy entry split of a subject (TEC-953). */
export type AnalyticsSources = components["schemas"]["AnalyticsSources"];

/** What the owner is looking at: a profile page, a card (its c/ route) or a smart link (route). */
export type AnalyticsSubject = { type: "profile" | "route" | "card"; id: string };

export type AnalyticsRange = { from: string; to: string };

const base = (s: AnalyticsSubject, r: AnalyticsRange) => ({
  subject_type: s.type,
  subject_id: s.id,
  from: r.from,
  to: r.to,
});

/** Account-scoped analytics (/v1/analytics/*): only the active account's subjects, clamped to the plan's history. */
export const analyticsService = {
  summary(s: AnalyticsSubject, r: AnalyticsRange, eventType?: string) {
    return platformRequest<AnalyticsSummary>("GET", "/v1/analytics/summary", {
      query: { ...base(s, r), event_type: eventType },
    });
  },
  /** Includes the subjects that lead to it: a profile's cards, smart links and legacy hash routes. */
  sources(s: AnalyticsSubject, r: AnalyticsRange) {
    return platformRequest<AnalyticsSources>("GET", "/v1/analytics/sources", { query: base(s, r) });
  },
  timeseries(s: AnalyticsSubject, r: AnalyticsRange) {
    return platformRequest<AnalyticsTimeSeries>("GET", "/v1/analytics/timeseries", {
      query: { ...base(s, r), interval: "day" },
    });
  },
  breakdown(s: AnalyticsSubject, r: AnalyticsRange, dimension: string) {
    return platformRequest<AnalyticsBreakdown>("GET", "/v1/analytics/breakdown", {
      query: { ...base(s, r), dimension },
    });
  },
};
