import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";
import { bankAccountsQueryKeys } from "./query-keys";
import {
  bankAccountsService,
  type CreateBankAccountPayload,
  type UpdateBankAccountPayload,
} from "../services/bank-accounts.service";

export function useBankAccounts() {
  return useQuery({
    queryKey: bankAccountsQueryKeys.lists(),
    queryFn: () => bankAccountsService.list(),
  });
}

export function useBankAccount(uuid: string | null) {
  return useQuery({
    queryKey: bankAccountsQueryKeys.detail(uuid ?? ""),
    queryFn: () => (uuid ? bankAccountsService.get(uuid) : null),
    enabled: Boolean(uuid),
  });
}

export function useBankAccountMutations() {
  const qc = useQueryClient();
  const { t } = useLocale();

  const createMutation = useMutation({
    mutationFn: (body: CreateBankAccountPayload) =>
      bankAccountsService.create(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bankAccountsQueryKeys.all });
      appToast.success(t("bank_accounts.toast.created"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({
      uuid,
      body,
    }: {
      uuid: string;
      body: UpdateBankAccountPayload;
    }) => bankAccountsService.update(uuid, body),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: bankAccountsQueryKeys.all });
      qc.invalidateQueries({
        queryKey: bankAccountsQueryKeys.detail(vars.uuid),
      });
      appToast.success(t("bank_accounts.toast.updated"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: (uuid: string) => bankAccountsService.toggleActive(uuid),
    onSuccess: (_, uuid) => {
      qc.invalidateQueries({ queryKey: bankAccountsQueryKeys.all });
      qc.invalidateQueries({ queryKey: bankAccountsQueryKeys.detail(uuid) });
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => bankAccountsService.remove(uuid),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bankAccountsQueryKeys.all });
      appToast.success(t("bank_accounts.toast.deleted"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  const reorderMutation = useMutation({
    mutationFn: (uuids: string[]) => bankAccountsService.reorder(uuids),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bankAccountsQueryKeys.all });
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });

  return {
    createMutation,
    updateMutation,
    toggleActiveMutation,
    deleteMutation,
    reorderMutation,
  };
}
