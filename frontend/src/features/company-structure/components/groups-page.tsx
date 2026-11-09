"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Pencil, Trash2, UsersRound } from "lucide-react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { routes } from "@/config/routes";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";

import {
  useCompanyGroups,
  useCompanyOffices,
  useGroupMutations,
} from "../hooks/use-company-structure";
import {
  type CompanyStructureAccess,
  useCompanyStructureAccess,
} from "../lib/access";
import { COMPANY_WIDE } from "../schemas/group-form";
import type { CompanyGroup } from "../services/company-structure.service";
import { GroupDialog } from "./group-dialog";
import { CompanyStructureGate } from "./structure-gate";

const ALL = "__all__";

/** Departments and teams, company-wide or bound to an office. */
export function GroupsPage() {
  const { t } = useLocale();
  const access = useCompanyStructureAccess();
  const officesQuery = useCompanyOffices(access.canRead);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<CompanyGroup | null>(null);
  const [officeFilter, setOfficeFilter] = useState<string>(ALL);

  const openDialog = useCallback((group: CompanyGroup | null) => {
    setEditing(group);
    setDialogOpen(true);
  }, []);

  return (
    <EntityPage
      title={t("company_structure.groups.title")}
      description={t("company_structure.groups.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("company_structure.section") },
        { label: t("company_structure.groups.title") },
      ]}
      actions={
        access.canCreateGroups ? (
          <EntityCreateButton
            onClick={() => openDialog(null)}
            label={t("company_structure.groups.add")}
          />
        ) : null
      }
    >
      <CompanyStructureGate>
        <GroupsContent
          access={access}
          officeFilter={officeFilter}
          onOfficeFilterChange={setOfficeFilter}
          onEdit={openDialog}
        />
        <GroupDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          group={editing}
          offices={officesQuery.data ?? []}
          access={access}
          defaultOffice={officeFilter === ALL ? undefined : officeFilter}
        />
      </CompanyStructureGate>
    </EntityPage>
  );
}

function GroupsContent({
  access,
  officeFilter,
  onOfficeFilterChange,
  onEdit,
}: {
  access: CompanyStructureAccess;
  officeFilter: string;
  onOfficeFilterChange: (value: string) => void;
  onEdit: (group: CompanyGroup) => void;
}) {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  // The API filters by office (`office_id`); "none" selects company-wide groups.
  const groupsQuery = useCompanyGroups(
    true,
    officeFilter === ALL
      ? undefined
      : officeFilter === COMPANY_WIDE
        ? "none"
        : officeFilter,
  );
  const officesQuery = useCompanyOffices();
  const { remove } = useGroupMutations();

  const officeNames = useMemo(
    () => new Map((officesQuery.data ?? []).map((o) => [o.uuid, o.name])),
    [officesQuery.data],
  );

  const rows = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data]);

  const onDelete = useCallback(
    async (group: CompanyGroup) => {
      const confirmed = await confirmDelete({
        title: t("company_structure.groups.delete_title"),
        description: t("company_structure.groups.delete_description", {
          name: group.name,
        }),
      });
      if (confirmed) remove.mutate(group.uuid);
    },
    [confirmDelete, remove, t],
  );

  const columns = useMemo<ColumnDef<CompanyGroup, unknown>[]>(
    () => [
      createColumn<CompanyGroup>({
        accessorKey: "name",
        labelKey: "company_structure.groups.fields.name",
        gridPrimary: true,
        cell: ({ row }) => (
          <div className="flex items-center gap-3">
            <span className="bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md">
              <UsersRound className="size-4" />
            </span>
            <span className="font-medium">{row.original.name}</span>
          </div>
        ),
      }),
      createColumn<CompanyGroup>({
        accessorKey: "kind",
        labelKey: "company_structure.groups.fields.kind",
        cell: ({ row }) => (
          <Badge variant="outline">
            {t(`company_structure.groups.kinds.${row.original.kind}`)}
          </Badge>
        ),
      }),
      createColumn<CompanyGroup>({
        id: "office",
        accessorFn: (row) =>
          row.office_uuid ? (officeNames.get(row.office_uuid) ?? "") : "",
        labelKey: "company_structure.groups.fields.office",
        cell: ({ row }) =>
          row.original.office_uuid ? (
            <span className="text-sm">
              {officeNames.get(row.original.office_uuid) ?? "—"}
            </span>
          ) : (
            <Badge variant="secondary">
              {t("company_structure.groups.company_wide")}
            </Badge>
          ),
      }),
      createColumn<CompanyGroup>({
        id: "actions",
        labelKey: "table.actions",
        enableSorting: false,
        enableHiding: false,
        enableResizing: false,
        enableColumnFilter: false,
        cell: ({ row }) =>
          access.canManageGroup(row.original) ? (
            <EntityRowActions
              actions={[
                {
                  id: "edit",
                  label: t("common.edit"),
                  icon: Pencil,
                  onSelect: () => onEdit(row.original),
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
          ) : null,
      }),
    ],
    [access, officeNames, onDelete, onEdit, remove.isPending, t],
  );

  return (
    <div className="space-y-4">
      {access.isOfficeManager && access.canCreateGroups ? (
        <p className="text-muted-foreground text-sm">
          {t("company_structure.groups.scoped_hint")}
        </p>
      ) : null}
      {!access.canCreateGroups ? (
        <p className="text-muted-foreground text-sm">
          {t("company_structure.read_only_hint")}
        </p>
      ) : null}
      <EntityTable
        columns={columns}
        data={rows}
        getRowId={(row) => row.uuid}
        onRowClick={(row) =>
          access.canManageGroup(row) ? onEdit(row) : undefined
        }
        isLoading={groupsQuery.isLoading}
        isError={groupsQuery.isError}
        onRetry={() => void groupsQuery.refetch()}
        emptyTitle={t("company_structure.groups.empty_title")}
        emptyDescription={t("company_structure.groups.empty_description")}
        manual={{ sorting: false, filtering: false, pagination: false }}
        features={{ persistKey: "app-company-groups-v1", rowSelection: false }}
        toolbarExtra={
          <>
            <Select value={officeFilter} onValueChange={onOfficeFilterChange}>
              <SelectTrigger
                className="h-8 w-48"
                aria-label={t("company_structure.groups.filter_office")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>
                  {t("company_structure.groups.filter_all")}
                </SelectItem>
                <SelectItem value={COMPANY_WIDE}>
                  {t("company_structure.groups.company_wide")}
                </SelectItem>
                {(officesQuery.data ?? []).map((office) => (
                  <SelectItem key={office.uuid} value={office.uuid}>
                    {office.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <EntityToolbar
              onRefresh={() => void groupsQuery.refetch()}
              refreshDisabled={groupsQuery.isFetching}
            />
          </>
        }
      />
    </div>
  );
}
