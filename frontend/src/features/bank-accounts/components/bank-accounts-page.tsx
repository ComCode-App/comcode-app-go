"use client";

import { useState, useCallback, useMemo } from "react";
import { ErrorState } from "@/components/common/error-state";
import {
  EntityCreateButton,
  EntityPage,
  EntityTable,
  useServerListState,
} from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

import {
  useBankAccounts,
  useBankAccountMutations,
} from "../hooks/use-bank-accounts";
import { getBankAccountsColumns } from "./bank-accounts-columns";
import { BankAccountDialog } from "./bank-account-dialog";
import type { UserBankAccountSummary } from "../services/bank-accounts.service";

export function BankAccountsPage() {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingAccount, setEditingAccount] =
    useState<UserBankAccountSummary | null>(null);

  const listState = useServerListState({ initialPageSize: 20 });
  const {
    data: accounts = [],
    isLoading,
    isError,
    refetch,
  } = useBankAccounts();
  const { deleteMutation, toggleActiveMutation, updateMutation } =
    useBankAccountMutations();

  const handleCreate = useCallback(() => {
    setEditingAccount(null);
    setDialogOpen(true);
  }, []);

  const handleEdit = useCallback((account: UserBankAccountSummary) => {
    setEditingAccount(account);
    setDialogOpen(true);
  }, []);

  const handleDelete = useCallback(
    async (uuid: string, accountName: string) => {
      const confirmed = await confirmDelete({
        title: t("bank_accounts.confirm_delete"),
        description: `${accountName} - ${t("bank_accounts.confirm_delete")}`,
      });
      if (confirmed) {
        deleteMutation.mutate(uuid);
      }
    },
    [confirmDelete, deleteMutation, t],
  );

  const handleToggleActive = useCallback(
    (account: UserBankAccountSummary) => {
      toggleActiveMutation.mutate(account.uuid);
    },
    [toggleActiveMutation],
  );

  const handleSetPrimary = useCallback(
    (account: UserBankAccountSummary) => {
      updateMutation.mutate({
        uuid: account.uuid,
        body: {
          iban: account.iban,
          account_holder: account.account_holder,
          currency: account.currency,
          title: account.title,
          bank_uuid: account.bank_uuid,
          branch_code: account.branch_code,
          account_number: account.account_number,
          is_primary: true,
          is_active: account.is_active,
        },
      });
    },
    [updateMutation],
  );

  const filteredAccounts = useMemo(() => {
    if (!listState.params.q) return accounts;
    const q = listState.params.q.toLowerCase();
    return accounts.filter(
      (acc) =>
        acc.iban.toLowerCase().includes(q) ||
        acc.account_holder.toLowerCase().includes(q) ||
        (acc.bank_name && acc.bank_name.toLowerCase().includes(q)) ||
        (acc.title && acc.title.toLowerCase().includes(q)),
    );
  }, [accounts, listState.params.q]);

  const columns = useMemo(
    () =>
      getBankAccountsColumns({
        t,
        onEdit: handleEdit,
        onDelete: handleDelete,
        onToggleActive: handleToggleActive,
        onSetPrimary: handleSetPrimary,
      }),
    [handleDelete, handleEdit, handleSetPrimary, handleToggleActive, t],
  );

  const pageCount = useMemo(() => {
    const total = filteredAccounts.length;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [filteredAccounts.length, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("bank_accounts.title")}
      description={t("bank_accounts.subtitle")}
      permission={permissions.bankAccounts.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("bank_accounts.title") },
      ]}
      actions={
        <EntityCreateButton
          onClick={handleCreate}
          label={t("bank_accounts.actions.add")}
          permission={permissions.bankAccounts.write}
        />
      }
    >
      <EntityTable
        columns={columns}
        data={filteredAccounts}
        getRowId={(row) => row.uuid}
        onRowClick={handleEdit}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => void refetch()}
        emptyTitle={t("bank_accounts.empty.title")}
        emptyDescription={t("bank_accounts.empty.description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "app-bank-accounts-v1",
          rowSelection: false,
        }}
      />

      <BankAccountDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        account={editingAccount}
      />
    </EntityPage>
  );
}
