import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type CompanyAnalyticsMetrics =
  components["schemas"]["CompanyAnalyticsMetrics"];
export type CompanyAnalyticsSummary =
  components["schemas"]["CompanyAnalyticsSummary"];
export type CompanyAnalyticsTimeSeries =
  components["schemas"]["CompanyAnalyticsTimeSeries"];
export type CompanyAnalyticsBreakdown =
  components["schemas"]["CompanyAnalyticsBreakdown"];
export type CompanyAnalyticsEmployee =
  components["schemas"]["CompanyAnalyticsEmployee"];

/** Range and office/group filter shared by every company analytics request. */
export type CompanyAnalyticsScope = {
  from: string;
  to: string;
  office_uuid?: string;
  group_uuid?: string;
};

export type CompanyAnalyticsEmployeeQuery = CompanyAnalyticsScope & {
  limit: number;
  offset: number;
  sort?: string;
  q?: string;
};

type Page<T> = { items: T[]; total: number; limit: number; offset: number };

/**
 * Company analytics (TEC-574, docs/ANALYTICS.md "Company analytics"): totals and per-employee
 * comparison of the active company account, summed from the existing daily aggregates.
 */
export const companyAnalyticsService = {
  summary(scope: CompanyAnalyticsScope) {
    return platformRequest<CompanyAnalyticsSummary>(
      "GET",
      "/v1/company/analytics/summary",
      { query: scope },
    );
  },
  timeseries(scope: CompanyAnalyticsScope) {
    return platformRequest<CompanyAnalyticsTimeSeries>(
      "GET",
      "/v1/company/analytics/timeseries",
      { query: scope },
    );
  },
  breakdown(scope: CompanyAnalyticsScope, dimension: string, limit = 10) {
    return platformRequest<CompanyAnalyticsBreakdown>(
      "GET",
      "/v1/company/analytics/breakdown",
      {
        query: { ...scope, dimension, limit },
      },
    );
  },
  employees(query: CompanyAnalyticsEmployeeQuery) {
    return platformRequest<Page<CompanyAnalyticsEmployee>>(
      "GET",
      "/v1/company/analytics/employees",
      { query },
    );
  },
};

export const companyAnalyticsKeys = {
  all: ["company-analytics"] as const,
  summary: (s: CompanyAnalyticsScope) =>
    [...companyAnalyticsKeys.all, "summary", s] as const,
  timeseries: (s: CompanyAnalyticsScope) =>
    [...companyAnalyticsKeys.all, "timeseries", s] as const,
  breakdown: (s: CompanyAnalyticsScope, dimension: string) =>
    [...companyAnalyticsKeys.all, "breakdown", dimension, s] as const,
  employees: (q: CompanyAnalyticsEmployeeQuery) =>
    [...companyAnalyticsKeys.all, "employees", q] as const,
};
