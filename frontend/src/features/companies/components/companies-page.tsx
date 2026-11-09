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
import type { CompanyRowActionHandlers } from "@/features/companies/components/company-row-actions";
import { useCompaniesColumns } from "@/features/companies/components/companies-columns";
import { companiesKeys } from "@/features/companies/hooks/query-keys";
import {
  companiesService,
  type CompanySummary,
} from "@/features/companies/services/companies.service";
import { useLocale } from "@/providers/locale-provider";

function firstString(value: string | string[] | undefined) {
  if (Array.isArray(value)) return value[0];
  return value;
}

export function CompaniesPage() {
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
    queryKey: companiesKeys.list(listParams),
    queryFn: () => companiesService.list(listParams),
  });

  const metaQuery = useQuery({
    queryKey: companiesKeys.meta(),
    queryFn: () => companiesService.meta(),
    staleTime: 5 * 60_000,
  });

  const openDetail = useCallback(
    (company: CompanySummary) =>
      router.push(routes.platform.companies.detail(company.uuid)),
    [router],
  );

  const rowHandlers = useMemo<CompanyRowActionHandlers>(
    () => ({
      onView: openDetail,
    }),
    [openDetail],
  );

  const baseColumns = useCompaniesColumns(rowHandlers);
  const columns = useMemo(
    () => [createSelectColumnDef<CompanySummary>(), ...baseColumns],
    [baseColumns],
  );

  const bulkQuery = useMemo(
    () => ({
      q: listParams.q,
      sort: listParams.sort,
      status: listParams.status,
    }),
    [listParams.q, listParams.sort, listParams.status],
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
        "platform.companies",
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
      title={t("companies.title")}
      description={t("companies.subtitle")}
      permission={permissions.companies.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("companies.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("companies.title") },
      ]}
      actions={
        <EntityCreateButton
          onClick={() => router.push(routes.platform.companies.create)}
          label={t("companies.actions.create")}
          permission={permissions.companies.write}
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
        emptyTitle={t("companies.empty_title")}
        emptyDescription={t("companies.empty_description")}
        pageCount={pageCount}
        state={{
          ...listState.tableState,
          rowSelection: bulkSelection.rowSelection,
          onRowSelectionChange: bulkSelection.onRowSelectionChange,
        }}
        features={{
          persistKey: "platform-companies-v1",
          rowSelection: true,
        }}
        toolbarExtra={
          <>
            <BulkActionMenu
              resource="platform.companies"
              actions={bulkActions}
              scope={bulkSelection.scope}
              selectedCount={bulkSelection.selectedCount}
              onComplete={() => void listQuery.refetch()}
            />
            <ResourceIOToolbar
              resource="platform.companies"
              query={{
                q: listParams.q,
                sort: listParams.sort,
                status: listParams.status,
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
