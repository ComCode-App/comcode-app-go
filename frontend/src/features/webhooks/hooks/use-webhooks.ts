import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import {
  type WebhookDeliveryStatus,
  type WebhookEndpointInput,
  webhooksService,
} from "@/features/webhooks/services/webhooks.service";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

export const webhooksKeys = {
  all: ["app", "webhooks"] as const,
  list: () => [...webhooksKeys.all, "list"] as const,
  events: () => [...webhooksKeys.all, "events"] as const,
  deliveries: (uuid: string, page: number, status?: WebhookDeliveryStatus) =>
    [...webhooksKeys.all, "deliveries", uuid, page, status ?? "all"] as const,
};

export function useWebhookEvents() {
  return useQuery({
    queryKey: webhooksKeys.events(),
    queryFn: () => webhooksService.events(),
    staleTime: 5 * 60_000,
  });
}

export function useWebhooks() {
  return useQuery({
    queryKey: webhooksKeys.list(),
    queryFn: () => webhooksService.list(),
  });
}

export function useWebhookDeliveries(
  uuid: string | null,
  page: number,
  pageSize: number,
  status?: WebhookDeliveryStatus,
) {
  return useQuery({
    queryKey: webhooksKeys.deliveries(uuid ?? "", page, status),
    queryFn: () =>
      webhooksService.deliveries(uuid ?? "", {
        limit: pageSize,
        offset: page * pageSize,
        status,
      }),
    enabled: Boolean(uuid),
    placeholderData: keepPreviousData,
  });
}

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error && err.message ? err.message : fallback;
}

export function useWebhookMutations() {
  const qc = useQueryClient();
  const { t } = useLocale();
  const invalidate = () => qc.invalidateQueries({ queryKey: webhooksKeys.all });
  const onError = (err: unknown) =>
    appToast.error(errorMessage(err, t("common.error_generic")));

  const create = useMutation({
    mutationFn: (body: WebhookEndpointInput) => webhooksService.create(body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("webhooks.toast.created"));
    },
    onError,
  });
  const update = useMutation({
    mutationFn: ({
      uuid,
      body,
    }: {
      uuid: string;
      body: WebhookEndpointInput;
    }) => webhooksService.update(uuid, body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("webhooks.toast.updated"));
    },
    onError,
  });
  const remove = useMutation({
    mutationFn: (uuid: string) => webhooksService.remove(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("webhooks.toast.deleted"));
    },
    onError,
  });
  const rotate = useMutation({
    mutationFn: (uuid: string) => webhooksService.rotateSecret(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("webhooks.toast.rotated"));
    },
    onError,
  });
  const sendTest = useMutation({
    mutationFn: (uuid: string) => webhooksService.sendTest(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("webhooks.toast.test_sent"));
    },
    onError,
  });
  const resend = useMutation({
    mutationFn: ({
      uuid,
      deliveryUuid,
    }: {
      uuid: string;
      deliveryUuid: string;
    }) => webhooksService.resend(uuid, deliveryUuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("webhooks.toast.resent"));
    },
    onError,
  });
  return { create, update, remove, rotate, sendTest, resend };
}
