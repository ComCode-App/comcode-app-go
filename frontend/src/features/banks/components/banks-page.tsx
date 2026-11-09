"use client";

import { useState, useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityPage,
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

import { banksService, type BankSummary } from "../services/banks.service";
import { banksQueryKeys } from "../hooks/query-keys";
import { useBankMutations } from "../hooks/use-banks";
import { getBanksColumns } from "./banks-columns";
import { BankDialog } from "./bank-dialog";

export function BanksPage() {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const { deleteMutation, toggleActiveMutation } = useBankMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingBank, setEditingBank] = useState<BankSummary | null>(null);

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({ ...listState.params, q: listState.params.q }),
    [listState.params],
  );

  const listQuery = useQuery({
    queryKey: banksQueryKeys.list(listParams),
    queryFn: () => banksService.list(listParams),
  });

  const handleCreate = useCallback(() => {
    setEditingBank(null);
    setDialogOpen(true);
  }, []);

  const handleEdit = useCallback((bank: BankSummary) => {
    setEditingBank(bank);
    setDialogOpen(true);
  }, []);

  const handleDelete = useCallback(
    async (uuid: string) => {
      const confirmed = await confirmDelete({
        title: t("banks.delete_title"),
        description: t("banks.delete_description"),
      });
      if (confirmed) {
        deleteMutation.mutate(uuid);
      }
    },
    [confirmDelete, deleteMutation, t],
  );

  const handleToggleActive = useCallback(
    (bank: BankSummary) => {
      toggleActiveMutation.mutate(bank);
    },
    [toggleActiveMutation],
  );

  const columns = useMemo(
    () =>
      getBanksColumns({
        t,
        onEdit: handleEdit,
        onDelete: handleDelete,
        onToggleActive: handleToggleActive,
      }),
    [t, handleEdit, handleDelete, handleToggleActive],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("banks.title")}
      description={t("banks.subtitle")}
      permission={permissions.banks.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("banks.title") },
      ]}
      actions={
        <Button onClick={handleCreate} className="gap-2 shadow-xs">
          <Plus className="h-4 w-4" />
          <span>{t("banks.actions.add")}</span>
        </Button>
      }
    >
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("banks.empty_title")}
        emptyDescription={t("banks.empty_description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "platform-banks-v1",
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />

      <BankDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        bank={editingBank}
      />
    </EntityPage>
  );
}
