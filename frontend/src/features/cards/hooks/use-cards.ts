"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { entitlementKeys } from "@/features/entitlements/hooks/use-entitlements";
import { isPlanLimitError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import {
  cardsService,
  type UpsertCardPayload,
  type ActivateCardPayload,
} from "../services/cards.service";
import { cardsKeys } from "./query-keys";

export function useCardMutations() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: cardsKeys.all });
    queryClient.invalidateQueries({ queryKey: entitlementKeys.overview });
  };

  const createMutation = useMutation({
    mutationFn: (payload: UpsertCardPayload) => cardsService.create(payload),
    onSuccess: () => {
      toast.success(t("cards.created_success"));
      invalidate();
    },
    onError: (err: Error) => {
      if (isPlanLimitError(err)) return;
      toast.error(err?.message || t("cards.error_creating"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({
      uuid,
      payload,
    }: {
      uuid: string;
      payload: UpsertCardPayload;
    }) => cardsService.update(uuid, payload),
    onSuccess: () => {
      toast.success(t("cards.updated_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("cards.error_updating"));
    },
  });

  const toggleLockMutation = useMutation({
    mutationFn: (uuid: string) => cardsService.toggleLock(uuid),
    onSuccess: (res) => {
      if (res.is_locked) {
        toast.warning(t("cards.card_locked_success"));
      } else {
        toast.success(t("cards.card_unlocked_success"));
      }
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("cards.error_locking"));
    },
  });

  const activateMutation = useMutation({
    mutationFn: (payload: ActivateCardPayload) =>
      cardsService.activate(payload),
    onSuccess: () => {
      toast.success(t("cards.activated_success"));
      invalidate();
    },
    onError: (err: Error) => {
      if (isPlanLimitError(err)) return;
      toast.error(err?.message || t("cards.error_activating"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => cardsService.delete(uuid),
    onSuccess: () => {
      toast.success(t("cards.deleted_success"));
      invalidate();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("cards.error_deleting"));
    },
  });

  return {
    createMutation,
    updateMutation,
    toggleLockMutation,
    activateMutation,
    deleteMutation,
  };
}
