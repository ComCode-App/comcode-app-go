"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type { ServerListParams } from "@/components/entity";
import { ApiError } from "@/lib/api/errors";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { type SectorInput, sectorsService } from "../services/sectors.service";
import { sectorsQueryKeys } from "./query-keys";

export function useSectors(params: ServerListParams) {
  return useQuery({
    queryKey: sectorsQueryKeys.list(
      params as unknown as Record<string, unknown>,
    ),
    queryFn: () => sectorsService.list(params),
  });
}

export function useSectorMutations() {
  const qc = useQueryClient();
  const { t } = useLocale();
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: sectorsQueryKeys.all });
  const message = (err: unknown, inUseKey: string) => {
    if (err instanceof ApiError && err.status === 409) return t(inUseKey);
    return err instanceof Error && err.message
      ? err.message
      : t("common.error_generic");
  };

  const create = useMutation({
    meta: { silentErrors: true },
    mutationFn: (body: SectorInput) => sectorsService.create(body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("sectors.toast.created"));
    },
    onError: (err) => appToast.error(message(err, "sectors.errors.duplicate")),
  });
  const update = useMutation({
    meta: { silentErrors: true },
    mutationFn: ({ uuid, body }: { uuid: string; body: SectorInput }) =>
      sectorsService.update(uuid, body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("sectors.toast.updated"));
    },
    onError: (err) => appToast.error(message(err, "sectors.errors.duplicate")),
  });
  const remove = useMutation({
    meta: { silentErrors: true },
    mutationFn: (uuid: string) => sectorsService.remove(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("sectors.toast.deleted"));
    },
    onError: (err) => appToast.error(message(err, "sectors.errors.in_use")),
  });

  return { create, update, remove };
}
