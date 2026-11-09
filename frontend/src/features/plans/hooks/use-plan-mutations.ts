"use client";

import { useQueryClient } from "@tanstack/react-query";

import { plansKeys } from "@/features/plans/hooks/query-keys";
import {
  plansService,
  type PlanDetail,
  type PlanListResult,
} from "@/features/plans/services/plans.service";
import { useAppMutation } from "@/lib/query/mutation";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";

export function useCreatePlan() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: plansService.create,
    onSuccess: (plan) => {
      queryClient.setQueryData(plansKeys.detail(plan.uuid), plan);
      void queryClient.invalidateQueries({ queryKey: plansKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: plansKeys.navStats() });
      appToast.success(t("plans.toast.created"));
    },
  });
}

export function useUpdatePlan(uuid: string) {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: (body: Parameters<typeof plansService.update>[1]) =>
      plansService.update(uuid, body),
    onSuccess: (plan) => {
      queryClient.setQueryData<PlanDetail>(plansKeys.detail(plan.uuid), plan);
      void queryClient.invalidateQueries({ queryKey: plansKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: plansKeys.navStats() });
      appToast.success(t("plans.toast.updated"));
    },
  });
}

export function useDeletePlan() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: (planUuid: string) => plansService.remove(planUuid),
    onSuccess: (_data, planUuid) => {
      queryClient.removeQueries({ queryKey: plansKeys.detail(planUuid) });
      queryClient.setQueriesData<PlanListResult>(
        { queryKey: plansKeys.lists() },
        (current) => {
          if (!current) return current;
          return {
            ...current,
            items: current.items.filter((item) => item.uuid !== planUuid),
            total: Math.max(0, current.total - 1),
          };
        },
      );
      void queryClient.invalidateQueries({ queryKey: plansKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: plansKeys.navStats() });
      appToast.success(t("plans.toast.deleted"));
    },
  });
}
