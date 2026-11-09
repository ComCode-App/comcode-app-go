"use client";

import { useQuery } from "@tanstack/react-query";

import { subscriptionsKeys } from "@/features/subscriptions/hooks/query-keys";
import { subscriptionsService } from "@/features/subscriptions/services/subscriptions.service";

const NAV_STATS_STALE_MS = 60_000;

export type SubscriptionsNavStats = {
  total: number;
};

export function useSubscriptionsNavStats(enabled = true) {
  return useQuery({
    queryKey: subscriptionsKeys.navStats(),
    queryFn: async (): Promise<SubscriptionsNavStats> => {
      const result = await subscriptionsService.list({ limit: 1, offset: 0 });
      return { total: result.total };
    },
    enabled,
    staleTime: NAV_STATS_STALE_MS,
  });
}
