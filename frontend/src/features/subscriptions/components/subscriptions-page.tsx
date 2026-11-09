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
import type { SubscriptionRowActionHandlers } from "@/features/subscriptions/components/subscription-row-actions";
import { useSubscriptionsColumns } from "@/features/subscriptions/components/subscriptions-columns";
import { subscriptionsKeys } from "@/features/subscriptions/hooks/query-keys";
import {
  subscriptionsService,
  type SubscriptionSummary,
} from "@/features/subscriptions/services/subscriptions.service";
import { useLocale } from "@/providers/locale-provider";

function firstString(value: string | string[] | undefined) {
  if (Array.isArray(value)) return value[0];
  return value;
}

export function SubscriptionsPage() {
  const { t } = useLocale();
  const router = useRouter();

  const listState = useServerListState({ initialPageSize: 20 });

  const listParams = useMemo(() => {
    const status = firstString(
      listState.columnFilters.find((filter) => filter.id === "status")
        ?.value as string | string[] | undefined,
    );
    return {
      ...listState.params,
      q: listState.params.q,
      status,
    };
  }, [listState.columnFilters, listState.params]);

  const listQuery = useQuery({
    queryKey: subscriptionsKeys.list(listParams),
    queryFn: () => subscriptionsService.list(listParams),
  });

  const metaQuery = useQuery({
    queryKey: subscriptionsKeys.meta(),
    queryFn: () => subscriptionsService.meta(),
    staleTime: 5 * 60_000,
  });

  const openDetail = useCallback(
    (subscription: SubscriptionSummary) =>
      router.push(routes.platform.subscriptions.detail(subscription.uuid)),
    [router],
  );

  const rowHandlers = useMemo<SubscriptionRowActionHandlers>(
    () => ({ onView: openDetail }),
    [openDetail],
  );

  const baseColumns = useSubscriptionsColumns(rowHandlers);
  const columns = useMemo(
    () => [createSelectColumnDef<SubscriptionSummary>(), ...baseColumns],
    [baseColumns],
  );

  const bulkQuery = useMemo(
    () => ({
      q: listParams.q,
      status: listParams.status,
      sort: listParams.sort,
    }),
    [listParams.q, listParams.status, listParams.sort],
  );

  const bulkSelection = useBulkSelection({
    listQueryKey: listParams,
    bulkQuery,
    total: listQuery.data?.total ?? 0,
  });

  const meta = metaQuery.data as ResourceMeta | undefined;
  const bulkActions = useMemo(
    () =>
      resolveBulkActionsWithIcons(
        "platform.subscriptions",
        meta?.bulk_actions ?? [],
      ),
    [meta?.bulk_actions],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("subscriptions.title")}
      description={t("subscriptions.subtitle")}
      permission={permissions.subscriptions.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("subscriptions.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("subscriptions.title") },
      ]}
      actions={
        <EntityCreateButton
          onClick={() => router.push(routes.platform.subscriptions.create)}
          label={t("subscriptions.actions.create")}
          permission={permissions.subscriptions.write}
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
        emptyTitle={t("subscriptions.empty_title")}
        emptyDescription={t("subscriptions.empty_description")}
        pageCount={pageCount}
        state={{
          ...listState.tableState,
          rowSelection: bulkSelection.rowSelection,
          onRowSelectionChange: bulkSelection.onRowSelectionChange,
        }}
        features={{
          persistKey: "platform-subscriptions-v1",
          rowSelection: true,
        }}
        toolbarExtra={
          <>
            <BulkActionMenu
              resource="platform.subscriptions"
              actions={bulkActions}
              scope={bulkSelection.scope}
              selectedCount={bulkSelection.selectedCount}
              onComplete={() => void listQuery.refetch()}
            />
            <ResourceIOToolbar
              resource="platform.subscriptions"
              query={{
                q: listParams.q,
                status: listParams.status,
                sort: listParams.sort,
              }}
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
