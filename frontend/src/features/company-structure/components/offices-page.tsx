"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Building, Pencil, Trash2 } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import {
  EntityCreateButton,
  EntityPage,
  EntityRowActions,
  EntityTable,
  EntityToolbar,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";

import {
  useCompanyGroups,
  useCompanyOffices,
  useOfficeMutations,
} from "../hooks/use-company-structure";
import { useCompanyStructureAccess } from "../lib/access";
import type { CompanyOffice } from "../services/company-structure.service";
import { OfficeDialog } from "./office-dialog";
import { CompanyStructureGate } from "./structure-gate";

/** Company offices (owner/admin manage; everyone with read access sees the list). */
export function OfficesPage() {
  const { t } = useLocale();
  const access = useCompanyStructureAccess();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<CompanyOffice | null>(null);
  const openDialog = useCallback((office: CompanyOffice | null) => {
    setEditing(office);
    setDialogOpen(true);
  }, []);

  return (
    <EntityPage
      title={t("company_structure.offices.title")}
      description={t("company_structure.offices.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("company_structure.section") },
        { label: t("company_structure.offices.title") },
      ]}
      actions={
        access.canManageOffices ? (
          <EntityCreateButton
            onClick={() => openDialog(null)}
            label={t("company_structure.offices.add")}
          />
        ) : null
      }
    >
      <CompanyStructureGate>
        <OfficesContent
          canManage={access.canManageOffices}
          onEdit={openDialog}
        />
        {access.canManageOffices ? (
          <OfficeDialog
            open={dialogOpen}
            onOpenChange={setDialogOpen}
            office={editing}
          />
        ) : null}
      </CompanyStructureGate>
    </EntityPage>
  );
}

function OfficesContent({
  canManage,
  onEdit: openEdit,
}: {
  canManage: boolean;
  onEdit: (office: CompanyOffice) => void;
}) {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const officesQuery = useCompanyOffices();
  const groupsQuery = useCompanyGroups();
  const { remove } = useOfficeMutations();

  const groupCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const group of groupsQuery.data ?? []) {
      if (group.office_uuid)
        counts.set(group.office_uuid, (counts.get(group.office_uuid) ?? 0) + 1);
    }
    return counts;
  }, [groupsQuery.data]);

  const onDelete = useCallback(
    async (office: CompanyOffice) => {
      const confirmed = await confirmDelete({
        title: t("company_structure.offices.delete_title"),
        description: t("company_structure.offices.delete_description", {
          name: office.name,
        }),
      });
      if (confirmed) remove.mutate(office.uuid);
    },
    [confirmDelete, remove, t],
  );

  const columns = useMemo<ColumnDef<CompanyOffice, unknown>[]>(() => {
    const cols: ColumnDef<CompanyOffice, unknown>[] = [
      createColumn<CompanyOffice>({
        accessorKey: "name",
        labelKey: "company_structure.offices.fields.name",
        gridPrimary: true,
        cell: ({ row }) => (
          <div className="flex items-center gap-3">
            <span className="bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md">
              <Building className="size-4" />
            </span>
            <div>
              <div className="font-medium">{row.original.name}</div>
              {row.original.city ? (
                <div className="text-muted-foreground text-xs">
                  {row.original.city}
                </div>
              ) : null}
            </div>
          </div>
        ),
      }),
      createColumn<CompanyOffice>({
        accessorKey: "address",
        labelKey: "company_structure.offices.fields.address",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="text-muted-foreground line-clamp-2 text-sm">
            {row.original.address || "—"}
          </span>
        ),
      }),
      createColumn<CompanyOffice>({
        accessorKey: "phone",
        labelKey: "company_structure.offices.fields.phone",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="text-sm">{row.original.phone || "—"}</span>
        ),
      }),
      createColumn<CompanyOffice>({
        id: "groups",
        labelKey: "company_structure.offices.fields.groups",
        enableSorting: false,
        cell: ({ row }) => (
          <Badge variant="secondary">
            {groupCounts.get(row.original.uuid) ?? 0}
          </Badge>
        ),
      }),
      createColumn<CompanyOffice>({
        accessorKey: "sort_order",
        labelKey: "company_structure.offices.fields.sort_order",
        defaultHidden: true,
      }),
    ];
    if (canManage) {
      cols.push(
        createColumn<CompanyOffice>({
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
                  onSelect: () => openEdit(row.original),
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
  }, [canManage, groupCounts, onDelete, openEdit, remove.isPending, t]);

  return (
    <div className="space-y-4">
      {canManage ? null : (
        <p className="text-muted-foreground text-sm">
          {t("company_structure.read_only_hint")}
        </p>
      )}
      <EntityTable
        columns={columns}
        data={officesQuery.data ?? []}
        getRowId={(row) => row.uuid}
        onRowClick={canManage ? openEdit : undefined}
        isLoading={officesQuery.isLoading}
        isError={officesQuery.isError}
        onRetry={() => void officesQuery.refetch()}
        emptyTitle={t("company_structure.offices.empty_title")}
        emptyDescription={t("company_structure.offices.empty_description")}
        manual={{ sorting: false, filtering: false, pagination: false }}
        features={{ persistKey: "app-company-offices-v1", rowSelection: false }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void officesQuery.refetch()}
            refreshDisabled={officesQuery.isFetching}
          />
        }
      />
    </div>
  );
}
