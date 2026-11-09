"use client";

import { useQuery } from "@tanstack/react-query";

import { OVERVIEW_STALE_MS } from "@/features/platform-overview/constants";
import { overviewKeys } from "@/features/platform-overview/hooks/query-keys";
import { type FunnelPeriod, overviewService } from "@/features/platform-overview/services/overview.service";

export function useOverview(enabled = true) {
  return useQuery({
    queryKey: overviewKeys.data(),
    queryFn: () => overviewService.get(),
    enabled,
    staleTime: OVERVIEW_STALE_MS,
    refetchInterval: OVERVIEW_STALE_MS,
  });
}

export function useConversionFunnel(days: FunnelPeriod, enabled = true) {
  return useQuery({
    queryKey: overviewKeys.funnel(days),
    queryFn: () => overviewService.funnel(days),
    enabled,
    staleTime: OVERVIEW_STALE_MS,
  });
}
