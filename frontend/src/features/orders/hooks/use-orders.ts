"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useLocale } from "@/providers/locale-provider";
import {
  ordersService,
  type CreateOrderPayload,
  type OrderStatus,
  type UpdateShippingPayload,
  type CreateAssetRequestPayload,
  type ReviewAssetRequestPayload,
} from "../services/orders.service";
import { ordersQueryKeys } from "./query-keys";

export function useOrderMutations() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ordersQueryKeys.all });
  };

  const createMutation = useMutation({
    mutationFn: (payload: CreateOrderPayload) => ordersService.create(payload),
    onSuccess: () => {
      toast.success(t("orders.created_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_creating"));
    },
  });

  const updateStatusMutation = useMutation({
    mutationFn: ({
      uuid,
      status,
      note,
    }: {
      uuid: string;
      status: OrderStatus;
      note?: string;
    }) => ordersService.updateStatus(uuid, status, note),
    onSuccess: () => {
      toast.success(t("orders.status_updated"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_updating"));
    },
  });

  const updateShippingMutation = useMutation({
    mutationFn: ({
      uuid,
      payload,
    }: {
      uuid: string;
      payload: UpdateShippingPayload;
    }) => ordersService.updateShipping(uuid, payload),
    onSuccess: () => {
      toast.success(t("orders.shipping_updated"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_updating"));
    },
  });

  const updateNotesMutation = useMutation({
    mutationFn: ({ uuid, adminNote }: { uuid: string; adminNote: string }) =>
      ordersService.updateNotes(uuid, adminNote),
    onSuccess: () => {
      toast.success(t("orders.notes_updated"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_updating"));
    },
  });

  const createAssetRequestMutation = useMutation({
    mutationFn: ({
      orderUuid,
      payload,
    }: {
      orderUuid: string;
      payload: CreateAssetRequestPayload;
    }) => ordersService.createAssetRequest(orderUuid, payload),
    onSuccess: (_, variables) => {
      toast.success(t("orders.asset_requested"));
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(variables.orderUuid),
      });
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_asset_request"));
    },
  });

  const reviewAssetRequestMutation = useMutation({
    mutationFn: ({
      orderUuid,
      assetUuid,
      payload,
    }: {
      orderUuid: string;
      assetUuid: string;
      payload: ReviewAssetRequestPayload;
    }) => ordersService.reviewAssetRequest(orderUuid, assetUuid, payload),
    onSuccess: (_, variables) => {
      toast.success(t("orders.asset_reviewed"));
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(variables.orderUuid),
      });
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_reviewing_asset"));
    },
  });

  const provisionOrderMutation = useMutation({
    mutationFn: (uuid: string) => ordersService.provisionOrder(uuid),
    onSuccess: (_, uuid) => {
      toast.success(t("orders.provision_success"));
      queryClient.invalidateQueries({ queryKey: ordersQueryKeys.detail(uuid) });
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.provision_error"));
    },
  });

  const provisionItemMutation = useMutation({
    mutationFn: ({
      orderUuid,
      itemUuid,
    }: {
      orderUuid: string;
      itemUuid: string;
    }) => ordersService.provisionItem(orderUuid, itemUuid),
    onSuccess: (_, vars) => {
      toast.success(t("orders.item_provision_success"));
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(vars.orderUuid),
      });
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.item_provision_error"));
    },
  });

  const bulkUpdateItemsMutation = useMutation({
    mutationFn: ({
      orderUuid,
      itemUuids,
      fulfillmentStatus,
    }: {
      orderUuid: string;
      itemUuids: string[];
      fulfillmentStatus: string;
    }) =>
      ordersService.bulkUpdateItems(orderUuid, itemUuids, fulfillmentStatus),
    onSuccess: (_, vars) => {
      toast.success(t("orders.bulk_update_items_success"));
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(vars.orderUuid),
      });
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_updating"));
    },
  });

  const updatePaymentStatusMutation = useMutation({
    mutationFn: ({
      uuid,
      paymentStatus,
      paymentReference,
    }: {
      uuid: string;
      paymentStatus: string;
      paymentReference?: string;
    }) =>
      ordersService.updatePaymentStatus(uuid, paymentStatus, paymentReference),
    onSuccess: (_, vars) => {
      toast.success(t("orders.payment_status_updated"));
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(vars.uuid),
      });
      invalidate();
    },
    onError: (err: Error, vars) => {
      toast.error(err?.message || t("orders.error_updating"));
      // A failed provider refund (TEC-935) is listed on the order; show it.
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(vars.uuid),
      });
    },
  });

  const updateFulfillmentStatusMutation = useMutation({
    mutationFn: ({
      uuid,
      fulfillmentStatus,
    }: {
      uuid: string;
      fulfillmentStatus: string;
    }) => ordersService.updateFulfillmentStatus(uuid, fulfillmentStatus),
    onSuccess: (_, vars) => {
      toast.success(t("orders.fulfillment_status_updated"));
      queryClient.invalidateQueries({
        queryKey: ordersQueryKeys.detail(vars.uuid),
      });
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_updating"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => ordersService.delete(uuid),
    onSuccess: () => {
      toast.success(t("orders.deleted_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_deleting"));
    },
  });

  return {
    createMutation,
    updateStatusMutation,
    updateShippingMutation,
    updateNotesMutation,
    createAssetRequestMutation,
    reviewAssetRequestMutation,
    provisionOrderMutation,
    provisionItemMutation,
    bulkUpdateItemsMutation,
    updatePaymentStatusMutation,
    updateFulfillmentStatusMutation,
    deleteMutation,
  };
}
