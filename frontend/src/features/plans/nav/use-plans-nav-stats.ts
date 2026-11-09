"use client";

import { useQuery } from "@tanstack/react-query";

import { plansKeys } from "@/features/plans/hooks/query-keys";
import { plansService } from "@/features/plans/services/plans.service";

const NAV_STATS_STALE_MS = 60_000;

export type PlansNavStats = {
  total: number;
};

export function usePlansNavStats(enabled = true) {
  return useQuery({
    queryKey: plansKeys.navStats(),
    queryFn: async (): Promise<PlansNavStats> => {
      const result = await plansService.list({ limit: 1, offset: 0 });
      return { total: result.total };
    },
    enabled,
    staleTime: NAV_STATS_STALE_MS,
  });
}
