"use client";

import { useRouter } from "next/navigation";
import { useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityCreateButton,
  EntityPage,
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { createSelectColumnDef } from "@/components/tables";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  BulkActionMenu,
  SelectionBanner,
  resolveBulkActionsWithIcons,
  useBulkSelection,
} from "@/features/bulk-engine";
import { ResourceIOToolbar } from "@/features/io";
import type { ResourceMeta } from "@/features/io/types";
import { usePlansColumns } from "@/features/plans/components/plans-columns";
import type { PlanRowActionHandlers } from "@/features/plans/components/plan-row-actions";
import { plansKeys } from "@/features/plans/hooks/query-keys";
import { useDeletePlan } from "@/features/plans/hooks/use-plan-mutations";
import {
  plansService,
  type PlanSummary,
} from "@/features/plans/services/plans.service";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

export function PlansPage() {
  const { t } = useLocale();
  const router = useRouter();
  const { confirmDelete } = useDialogs();
  const deletePlan = useDeletePlan();

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({ ...listState.params, q: listState.params.q }),
    [listState.params],
  );

  const listQuery = useQuery({
    queryKey: plansKeys.list(listParams),
    queryFn: () => plansService.list(listParams),
  });

  const metaQuery = useQuery({
    queryKey: plansKeys.meta(),
    queryFn: () => plansService.meta(),
    staleTime: 5 * 60_000,
  });

  const openDetail = useCallback(
    (plan: PlanSummary) => router.push(routes.platform.plans.detail(plan.uuid)),
    [router],
  );

  const openEdit = useCallback(
    (plan: PlanSummary) => router.push(routes.platform.plans.edit(plan.uuid)),
    [router],
  );

  const handleDelete = useCallback(
    async (plan: PlanSummary) => {
      const confirmed = await confirmDelete({
        title: t("plans.delete_title"),
        description: t("plans.delete_description", { name: plan.name }),
      });
      if (!confirmed) return;
      await deletePlan.mutateAsync(plan.uuid);
    },
    [confirmDelete, deletePlan, t],
  );

  const rowHandlers = useMemo<PlanRowActionHandlers>(
    () => ({
      onView: openDetail,
      onEdit: openEdit,
      onDelete: (plan) => void handleDelete(plan),
    }),
    [handleDelete, openDetail, openEdit],
  );

  const baseColumns = usePlansColumns(rowHandlers);
  const columns = useMemo(
    () => [createSelectColumnDef<PlanSummary>(), ...baseColumns],
    [baseColumns],
  );

  const bulkQuery = useMemo(
    () => ({ q: listParams.q, sort: listParams.sort }),
    [listParams.q, listParams.sort],
  );

  const bulkSelection = useBulkSelection({
    listQueryKey: listParams,
    bulkQuery,
    total: listQuery.data?.total ?? 0,
  });

  const meta = metaQuery.data as ResourceMeta | undefined;
  const bulkActions = useMemo(
    () => resolveBulkActionsWithIcons("platform.plans", meta?.bulk_actions ?? []),
    [meta?.bulk_actions],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("plans.title")}
      description={t("plans.subtitle")}
      permission={permissions.plans.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("plans.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("plans.title") },
      ]}
      actions={
        <EntityCreateButton
          onClick={() => router.push(routes.platform.plans.create)}
          label={t("plans.actions.create")}
          permission={permissions.plans.write}
        />
      }
    >
      <SelectionBanner
        selectedCount={bulkSelection.selectedCount}
        total={listQuery.data?.total ?? 0}
        showSelectAll={bulkSelection.showSelectAllBanner}
        allMatchingSelected={bulkSelection.scope.mode === "all"}
        onSelectAllMatching={bulkSelection.selectAllMatching}
        onClearSelection={bulkSelection.clearSelection}
      />
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        onRowClick={openDetail}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("plans.empty_title")}
        emptyDescription={t("plans.empty_description")}
        pageCount={pageCount}
        state={{
          ...listState.tableState,
          rowSelection: bulkSelection.rowSelection,
          onRowSelectionChange: bulkSelection.onRowSelectionChange,
        }}
        features={{
          persistKey: "platform-plans-v1",
          rowSelection: true,
        }}
        toolbarExtra={
          <>
            <BulkActionMenu
              resource="platform.plans"
              actions={bulkActions}
              scope={bulkSelection.scope}
              selectedCount={bulkSelection.selectedCount}
              onComplete={() => void listQuery.refetch()}
            />
            <ResourceIOToolbar
              resource="platform.plans"
              query={{ q: listParams.q, sort: listParams.sort }}
              capabilities={meta?.capabilities}
              onImportComplete={() => void listQuery.refetch()}
            />
            <EntityToolbar
              onRefresh={() => void listQuery.refetch()}
              refreshDisabled={listQuery.isFetching}
            />
          </>
        }
      />
    </EntityPage>
  );
}
