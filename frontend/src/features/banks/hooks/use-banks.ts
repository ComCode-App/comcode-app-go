import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ServerListParams } from "@/components/entity";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";
import { banksQueryKeys } from "./query-keys";
import {
  banksService,
  type UpsertBankPayload,
  type BankSummary,
} from "../services/banks.service";

export function useBanks(params: ServerListParams) {
  return useQuery({
    queryKey: banksQueryKeys.list(params as unknown as Record<string, unknown>),
    queryFn: () => banksService.list(params),
  });
}

export function useActiveBanks() {
  return useQuery({
    queryKey: banksQueryKeys.active(),
    queryFn: () => banksService.listActive(),
  });
}

export function useBankMeta() {
  return useQuery({
    queryKey: banksQueryKeys.meta(),
    queryFn: () => banksService.meta(),
  });
}

export function useBank(uuid: string | null) {
  return useQuery({
    queryKey: banksQueryKeys.detail(uuid ?? ""),
    queryFn: () => (uuid ? banksService.get(uuid) : null),
    enabled: Boolean(uuid),
  });
}

export function useBankMutations() {
  const qc = useQueryClient();
  const { t } = useLocale();

  const createMutation = useMutation({
    mutationFn: (body: UpsertBankPayload) => banksService.create(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: banksQueryKeys.all });
      appToast.success(t("banks.toast.created"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, body }: { uuid: string; body: UpsertBankPayload }) =>
      banksService.update(uuid, body),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: banksQueryKeys.all });
      qc.invalidateQueries({ queryKey: banksQueryKeys.detail(vars.uuid) });
      appToast.success(t("banks.toast.updated"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => banksService.remove(uuid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: banksQueryKeys.all });
      appToast.success(t("banks.toast.deleted"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const uploadIconMutation = useMutation({
    mutationFn: ({ uuid, file }: { uuid: string; file: File }) =>
      banksService.uploadIcon(uuid, file),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: banksQueryKeys.all });
      qc.invalidateQueries({ queryKey: banksQueryKeys.detail(vars.uuid) });
      appToast.success(t("banks.toast.icon_uploaded"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: (bank: BankSummary) =>
      banksService.update(bank.uuid, {
        name: bank.name,
        code: bank.code,
        country_code: bank.country_code,
        bic_swift: bank.bic_swift,
        icon_url: bank.icon_url,
        brand_color: bank.brand_color,
        iban_pattern: bank.iban_pattern,
        description: bank.description,
        display_order: bank.display_order,
        is_active: !bank.is_active,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: banksQueryKeys.all });
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  return {
    createMutation,
    updateMutation,
    deleteMutation,
    uploadIconMutation,
    toggleActiveMutation,
  };
}
