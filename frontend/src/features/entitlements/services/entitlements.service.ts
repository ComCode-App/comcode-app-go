import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type EntitlementOverview = components["schemas"]["EntitlementOverview"];
export type EntitlementFeature = components["schemas"]["EntitlementFeature"];

/** Quota feature keys the API enforces on create (403 PLAN_LIMIT_EXCEEDED). */
export type QuotaFeatureKey = "profiles.max" | "smart_links.max" | "cards.max";

/**
 * Monthly quotas (TEC-941), counted per calendar month in Europe/Istanbul. Leads and exchanges
 * past the quota are stored locked; exports past it are refused (403 PLAN_LIMIT_EXCEEDED).
 */
export type MonthlyQuotaKey =
  "leads.monthly_max" | "exchange.monthly_max" | "leads.export.monthly_max";

/** Plan, features and quota usage of the active account (/v1/account/entitlements). */
export const entitlementsService = {
  async overview() {
    return platformRequest<EntitlementOverview>(
      "GET",
      "/v1/account/entitlements",
    );
  },
};
