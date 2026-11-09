"use client";

import { useQueryClient } from "@tanstack/react-query";

import { subscriptionsKeys } from "@/features/subscriptions/hooks/query-keys";
import {
  subscriptionsService,
  type SubscriptionSummary,
} from "@/features/subscriptions/services/subscriptions.service";
import { useAppMutation } from "@/lib/query/mutation";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";

export function useCreateSubscription() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: subscriptionsService.create,
    onSuccess: (subscription) => {
      queryClient.setQueryData(
        subscriptionsKeys.detail(subscription.uuid),
        subscription,
      );
      void queryClient.invalidateQueries({
        queryKey: subscriptionsKeys.lists(),
      });
      void queryClient.invalidateQueries({
        queryKey: subscriptionsKeys.navStats(),
      });
      appToast.success(t("subscriptions.toast.created"));
    },
  });
}

export function useUpdateSubscription(uuid: string) {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: (body: Parameters<typeof subscriptionsService.update>[1]) =>
      subscriptionsService.update(uuid, body),
    onSuccess: (subscription) => {
      queryClient.setQueryData<SubscriptionSummary>(
        subscriptionsKeys.detail(subscription.uuid),
        subscription,
      );
      void queryClient.invalidateQueries({
        queryKey: subscriptionsKeys.lists(),
      });
      appToast.success(t("subscriptions.toast.updated"));
    },
  });
}
