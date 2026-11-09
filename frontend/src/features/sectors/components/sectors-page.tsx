"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Pencil, Trash2 } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityCreateButton,
  EntityPage,
  EntityRowActions,
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

import { useSectorMutations, useSectors } from "../hooks/use-sectors";
import type { PlatformSector } from "../services/sectors.service";
import { SectorDialog } from "./sector-dialog";

/** Platform-managed bilingual sector catalog companies pick from. */
export function SectorsPage() {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const { confirmDelete } = useDialogs();
  const canWrite = can(permissions.sectors.write);
  const { remove } = useSectorMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<PlatformSector | null>(null);

  // The API sorts by slug only.
  const listState = useServerListState({
    initialPageSize: 20,
    initialSort: "",
  });
  const listParams = useMemo(
    () => ({
      limit: listState.params.limit,
      offset: listState.params.offset,
      q: listState.params.q,
    }),
    [listState.params.limit, listState.params.offset, listState.params.q],
  );
  const listQuery = useSectors(listParams);

  const openDialog = useCallback((sector: PlatformSector | null) => {
    setEditing(sector);
    setDialogOpen(true);
  }, []);

  const onDelete = useCallback(
    async (sector: PlatformSector) => {
      const confirmed = await confirmDelete({
        title: t("sectors.delete_title"),
        description: t("sectors.delete_description", {
          name: locale === "tr" ? sector.name_tr : sector.name_en,
        }),
      });
      if (confirmed) remove.mutate(sector.uuid);
    },
    [confirmDelete, locale, remove, t],
  );

  const columns = useMemo<ColumnDef<PlatformSector, unknown>[]>(() => {
    const cols: ColumnDef<PlatformSector, unknown>[] = [
      createColumn<PlatformSector>({
        accessorKey: "slug",
        labelKey: "sectors.fields.slug",
        enableSorting: false,
        gridPrimary: true,
        cell: ({ row }) => (
          <span className="bg-muted rounded px-2 py-0.5 font-mono text-xs">
            {row.original.slug}
          </span>
        ),
      }),
      createColumn<PlatformSector>({
        accessorKey: "name_tr",
        labelKey: "sectors.fields.name_tr",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="font-medium">{row.original.name_tr}</span>
        ),
      }),
      createColumn<PlatformSector>({
        accessorKey: "name_en",
        labelKey: "sectors.fields.name_en",
        enableSorting: false,
      }),
    ];
    if (canWrite) {
      cols.push(
        createColumn<PlatformSector>({
          id: "actions",
          labelKey: "table.actions",
          enableSorting: false,
          enableHiding: false,
          enableResizing: false,
          enableColumnFilter: false,
          cell: ({ row }) => (
            <EntityRowActions
              actions={[
                {
                  id: "edit",
                  label: t("common.edit"),
                  icon: Pencil,
                  onSelect: () => openDialog(row.original),
                },
                {
                  id: "delete",
                  label: t("common.delete"),
                  icon: Trash2,
                  variant: "destructive",
                  disabled: remove.isPending,
                  onSelect: () => void onDelete(row.original),
                },
              ]}
            />
          ),
        }),
      );
    }
    return cols;
  }, [canWrite, onDelete, openDialog, remove.isPending, t]);

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("sectors.title")}
      description={t("sectors.subtitle")}
      permission={permissions.sectors.read}
      forbiddenFallback={<ErrorState title={t("common.error_forbidden")} />}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("sectors.title") },
      ]}
      actions={
        <EntityCreateButton
          onClick={() => openDialog(null)}
          label={t("sectors.actions.add")}
          permission={permissions.sectors.write}
        />
      }
    >
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        onRowClick={canWrite ? openDialog : undefined}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("sectors.empty_title")}
        emptyDescription={t("sectors.empty_description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{ persistKey: "platform-sectors-v1", rowSelection: false }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />
      {canWrite ? (
        <SectorDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          sector={editing}
        />
      ) : null}
    </EntityPage>
  );
}
