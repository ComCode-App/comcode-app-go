"use client";

import { useQuery } from "@tanstack/react-query";

import {
  type EntitlementFeature,
  entitlementsService,
  type MonthlyQuotaKey,
  type QuotaFeatureKey,
} from "@/features/entitlements/services/entitlements.service";

export const entitlementKeys = {
  overview: ["account", "entitlements"] as const,
};

/** Active account plan + usage. Invalidate `entitlementKeys.overview` after quota-capped creates. */
export function useEntitlements() {
  return useQuery({
    queryKey: entitlementKeys.overview,
    queryFn: entitlementsService.overview,
    staleTime: 30_000,
  });
}

/** One quota feature of the active account, or undefined while loading. */
export function useQuota(key: QuotaFeatureKey | MonthlyQuotaKey): EntitlementFeature | undefined {
  const { data } = useEntitlements();
  return data?.features.find((f) => f.key === key);
}
