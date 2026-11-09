"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useLocale } from "@/providers/locale-provider";
import { categoriesService, type UpsertCategoryPayload } from "../services/categories.service";
import { categoriesQueryKeys } from "./query-keys";

export function useCategoryMutations() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: categoriesQueryKeys.all });
  };

  const createMutation = useMutation({
    mutationFn: (payload: UpsertCategoryPayload) => categoriesService.create(payload),
    onSuccess: () => {
      toast.success(t("categories.created_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("categories.error_creating"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, payload }: { uuid: string; payload: UpsertCategoryPayload }) =>
      categoriesService.update(uuid, payload),
    onSuccess: () => {
      toast.success(t("categories.updated_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("categories.error_updating"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => categoriesService.delete(uuid),
    onSuccess: () => {
      toast.success(t("categories.deleted_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("categories.error_deleting"));
    },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: ({ uuid, payload, isActive }: { uuid: string; payload: UpsertCategoryPayload; isActive: boolean }) =>
      categoriesService.update(uuid, { ...payload, is_active: isActive }),
    onSuccess: () => {
      toast.success(t("categories.status_updated"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("categories.error_updating"));
    },
  });

  return {
    createMutation,
    updateMutation,
    deleteMutation,
    toggleActiveMutation,
  };
}
