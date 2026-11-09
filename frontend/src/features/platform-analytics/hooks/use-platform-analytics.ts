import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import {
  platformAnalyticsService,
  type AnalyticsSettingsInput,
  type EventDefinitionFilters,
  type EventDefinitionInput,
} from "../services/platform-analytics.service";
import { platformAnalyticsKeys } from "./query-keys";

export function useEventDefinitions(
  filters: EventDefinitionFilters = {},
  enabled = true,
) {
  return useQuery({
    queryKey: platformAnalyticsKeys.definitionList(filters),
    queryFn: () => platformAnalyticsService.listDefinitions(filters),
    enabled,
  });
}

export function useAnalyticsConfig(enabled = true) {
  return useQuery({
    queryKey: platformAnalyticsKeys.config(),
    queryFn: () => platformAnalyticsService.getConfig(),
    enabled,
  });
}

function useErrorToast() {
  const { t } = useLocale();
  return (err: unknown) => {
    appToast.error(
      isApiError(err) && err.message ? err.message : t("common.error_generic"),
    );
  };
}

export function useEventDefinitionMutations() {
  const qc = useQueryClient();
  const { t } = useLocale();
  const onError = useErrorToast();
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: platformAnalyticsKeys.definitions() });

  const create = useMutation({
    mutationFn: (body: EventDefinitionInput) =>
      platformAnalyticsService.createDefinition(body),
    onSuccess: async () => {
      await invalidate();
      appToast.success(t("platform_analytics.definitions.toast.created"));
    },
    onError,
  });

  const update = useMutation({
    mutationFn: ({ id, body }: { id: number; body: EventDefinitionInput }) =>
      platformAnalyticsService.updateDefinition(id, body),
    onSuccess: async () => {
      await invalidate();
      appToast.success(t("platform_analytics.definitions.toast.updated"));
    },
    onError,
  });

  /** Quiet flag flip from the table (enabled / public / aggregate switches). */
  const toggle = useMutation({
    mutationFn: ({ id, body }: { id: number; body: EventDefinitionInput }) =>
      platformAnalyticsService.updateDefinition(id, body),
    onSuccess: invalidate,
    onError,
  });

  const remove = useMutation({
    mutationFn: (id: number) => platformAnalyticsService.deleteDefinition(id),
    onSuccess: async () => {
      await invalidate();
      appToast.success(t("platform_analytics.definitions.toast.deleted"));
    },
    onError,
  });

  return { create, update, toggle, remove };
}

export function useUpdateAnalyticsConfig() {
  const qc = useQueryClient();
  const { t } = useLocale();
  const onError = useErrorToast();
  return useMutation({
    mutationFn: (body: AnalyticsSettingsInput) =>
      platformAnalyticsService.updateConfig(body),
    onSuccess: (data) => {
      qc.setQueryData(platformAnalyticsKeys.config(), data);
      appToast.success(t("platform_analytics.config.toast.saved"));
    },
    onError,
  });
}
