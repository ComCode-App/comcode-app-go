"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useLocale } from "@/providers/locale-provider";
import { productsService, type UpsertProductPayload, type UpsertVariantPayload } from "../services/products.service";
import { productsQueryKeys } from "./query-keys";

export function useProductMutations() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: productsQueryKeys.all });
  };

  const createMutation = useMutation({
    mutationFn: (payload: UpsertProductPayload) => productsService.create(payload),
    onSuccess: () => {
      toast.success(t("products.created_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("products.error_creating"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, payload }: { uuid: string; payload: UpsertProductPayload }) =>
      productsService.update(uuid, payload),
    onSuccess: () => {
      toast.success(t("products.updated_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("products.error_updating"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => productsService.delete(uuid),
    onSuccess: () => {
      toast.success(t("products.deleted_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("products.error_deleting"));
    },
  });

  const createVariantMutation = useMutation({
    mutationFn: ({ productUuid, payload }: { productUuid: string; payload: UpsertVariantPayload }) =>
      productsService.createVariant(productUuid, payload),
    onSuccess: (_, variables) => {
      toast.success(t("products.variant_created"));
      queryClient.invalidateQueries({ queryKey: productsQueryKeys.variants(variables.productUuid) });
      queryClient.invalidateQueries({ queryKey: productsQueryKeys.detail(variables.productUuid) });
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("products.error_variant_creating"));
    },
  });

  const deleteVariantMutation = useMutation({
    mutationFn: ({ productUuid, variantUuid }: { productUuid: string; variantUuid: string }) =>
      productsService.deleteVariant(productUuid, variantUuid),
    onSuccess: (_, variables) => {
      toast.success(t("products.variant_deleted"));
      queryClient.invalidateQueries({ queryKey: productsQueryKeys.variants(variables.productUuid) });
      queryClient.invalidateQueries({ queryKey: productsQueryKeys.detail(variables.productUuid) });
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("products.error_variant_deleting"));
    },
  });

  return {
    createMutation,
    updateMutation,
    deleteMutation,
    createVariantMutation,
    deleteVariantMutation,
  };
}
