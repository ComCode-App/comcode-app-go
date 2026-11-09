"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Layers, Link2, Upload } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import {
  EntityPage,
  type EntityRowAction,
  EntityRowActions,
  EntityTable,
  EntityToolbar,
  EntityUnavailable,
  useServerListState,
} from "@/components/entity";
import { createColumn, createSelectColumnDef } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { routes } from "@/config/routes";
import {
  resolveBulkActionsWithIcons,
  SelectionBanner,
  useBulkSelection,
} from "@/features/bulk-engine";
import { buildBulkTarget } from "@/features/bulk-engine";
import {
  useCompanyGroups,
  useCompanyOffices,
} from "@/features/company-structure/hooks/use-company-structure";
import { ImportWizard } from "@/features/io/components/import-wizard";
import { useLocale } from "@/providers/locale-provider";

import {
  useCompanyBulkMeta,
  useMemberProfiles,
} from "../hooks/use-company-bulk";
import { type CompanyBulkAccess, useCompanyBulkAccess } from "../lib/access";
import type { CompanyMemberProfile } from "../services/company-bulk.service";
import { BulkActionDialog } from "./bulk-action-dialog";
import { BulkJobsCard } from "./bulk-jobs-card";
import { LinkProfileDialog } from "./link-profile-dialog";

const ALL = "__all__";
type View = "with_profile" | "without_profile";

/** Company → Bulk operations: employee profile bulk actions, import and job history (TEC-573). */
export function CompanyBulkPage() {
  const { t } = useLocale();
  const access = useCompanyBulkAccess();

  let body = <CompanyBulkContent access={access} />;
  if (!access.isCompany) {
    body = (
      <EntityUnavailable
        title={t("company_structure.unavailable.personal_title")}
        description={t("company_bulk.unavailable.personal")}
      />
    );
  } else if (!access.canBulk) {
    body = (
      <EntityUnavailable
        title={t("common.error_forbidden")}
        description={t("company_bulk.unavailable.forbidden")}
      />
    );
  }

  return (
    <EntityPage
      title={t("company_bulk.title")}
      description={t("company_bulk.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("company_structure.section") },
        { label: t("company_bulk.title") },
      ]}
    >
      {body}
    </EntityPage>
  );
}

function memberName(m: CompanyMemberProfile) {
  return `${m.name} ${m.surname}`.trim() || m.email;
}

function CompanyBulkContent({ access }: { access: CompanyBulkAccess }) {
  const { t } = useLocale();
  const [view, setView] = useState<View>("with_profile");
  const [office, setOffice] = useState(ALL);
  const [group, setGroup] = useState(ALL);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [linkTarget, setLinkTarget] = useState<CompanyMemberProfile | null>(
    null,
  );

  const listState = useServerListState({ initialPageSize: 20, initialSort: "" });
  const officesQuery = useCompanyOffices();
  const groupsQuery = useCompanyGroups();
  const metaQuery = useCompanyBulkMeta();

  const filters = useMemo(
    () => ({
      q: listState.params.q,
      office_uuid: office === ALL ? undefined : office,
      group_uuid: group === ALL ? undefined : group,
    }),
    [group, listState.params.q, office],
  );
  const query = useMemo(
    () => ({
      ...filters,
      limit: listState.params.limit,
      offset: listState.params.offset,
      has_profile: view === "with_profile",
    }),
    [filters, listState.params.limit, listState.params.offset, view],
  );
  const membersQuery = useMemberProfiles(query);
  const rows = useMemo(
    () => membersQuery.data?.items ?? [],
    [membersQuery.data],
  );
  const total = membersQuery.data?.total ?? 0;

  const officeNames = useMemo(
    () => new Map((officesQuery.data ?? []).map((o) => [o.uuid, o.name])),
    [officesQuery.data],
  );
  const groupNames = useMemo(
    () => new Map((groupsQuery.data ?? []).map((g) => [g.uuid, g.name])),
    [groupsQuery.data],
  );
  const groupOptions = useMemo(
    () =>
      (groupsQuery.data ?? []).filter(
        (g) => office === ALL || !g.office_uuid || g.office_uuid === office,
      ),
    [groupsQuery.data, office],
  );

  // The bulk "query" scope uses the same filters; the server keeps linked profiles only and,
  // for office managers, members of their offices.
  const bulkQuery = useMemo(
    () => ({
      q: filters.q,
      office_uuid: filters.office_uuid,
      group_uuid: filters.group_uuid,
    }),
    [filters],
  );
  const selection = useBulkSelection({
    listQueryKey: query,
    bulkQuery,
    total,
  });

  const actions = useMemo(
    () =>
      resolveBulkActionsWithIcons(
        "app.company_profiles",
        metaQuery.data?.bulk_actions ?? [],
      ),
    [metaQuery.data?.bulk_actions],
  );

  const names = useMemo(() => {
    const out = new Map<string, string>();
    for (const m of rows) if (m.profile) out.set(m.profile.uuid, memberName(m));
    return out;
  }, [rows]);

  const target = useMemo(
    () =>
      selection.scope.mode === "none" ? null : buildBulkTarget(selection.scope),
    [selection.scope],
  );

  const openLink = useCallback(
    (row: CompanyMemberProfile) => setLinkTarget(row),
    [],
  );

  const columns = useMemo<ColumnDef<CompanyMemberProfile, unknown>[]>(() => {
    const base: ColumnDef<CompanyMemberProfile, unknown>[] = [
      createColumn<CompanyMemberProfile>({
        id: "member",
        accessorFn: memberName,
        labelKey: "company_bulk.fields.member",
        gridPrimary: true,
        enableSorting: false,
        cell: ({ row }) => (
          <div>
            <div className="font-medium">{memberName(row.original)}</div>
            <div className="text-muted-foreground text-xs">
              {row.original.email}
            </div>
          </div>
        ),
      }),
      createColumn<CompanyMemberProfile>({
        id: "profile",
        labelKey: "company_bulk.fields.profile",
        enableSorting: false,
        cell: ({ row }) => {
          const p = row.original.profile;
          if (!p)
            return (
              <span className="text-muted-foreground text-sm">
                {t("company_bulk.no_profile")}
              </span>
            );
          return (
            <div>
              <div className="text-sm">
                {`${p.first_name} ${p.last_name}`.trim() || "—"}
              </div>
              {p.title ? (
                <div className="text-muted-foreground text-xs">{p.title}</div>
              ) : null}
            </div>
          );
        },
      }),
      createColumn<CompanyMemberProfile>({
        id: "office",
        labelKey: "company_structure.members.fields.office",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="text-sm">
            {row.original.office_uuid
              ? (officeNames.get(row.original.office_uuid) ?? "—")
              : t("company_structure.members.unassigned")}
          </span>
        ),
      }),
      createColumn<CompanyMemberProfile>({
        id: "group",
        labelKey: "company_structure.members.fields.group",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="text-sm">
            {row.original.group_uuid
              ? (groupNames.get(row.original.group_uuid) ?? "—")
              : "—"}
          </span>
        ),
      }),
      createColumn<CompanyMemberProfile>({
        id: "role",
        labelKey: "company_structure.members.fields.role",
        enableSorting: false,
        cell: ({ row }) => (
          <Badge variant="secondary">
            {t(`accounts.roles.${row.original.role}`)}
          </Badge>
        ),
      }),
    ];
    if (access.canLink) {
      base.push(
        createColumn<CompanyMemberProfile>({
          id: "actions",
          labelKey: "table.actions",
          enableSorting: false,
          enableHiding: false,
          enableResizing: false,
          enableColumnFilter: false,
          cell: ({ row }) => {
            const items: EntityRowAction[] = [
              {
                id: "link",
                label: t(
                  row.original.profile
                    ? "company_bulk.link.change"
                    : "company_bulk.link.title",
                ),
                icon: Link2,
                onSelect: () => openLink(row.original),
              },
            ];
            return <EntityRowActions actions={items} />;
          },
        }),
      );
    }
    return view === "with_profile"
      ? [createSelectColumnDef<CompanyMemberProfile>(), ...base]
      : base;
  }, [access.canLink, groupNames, officeNames, openLink, t, view]);

  const pageCount = Math.max(
    1,
    Math.ceil(total / (listState.pagination.pageSize || 20)),
  );

  return (
    <div className="space-y-4">
      <p className="text-muted-foreground text-sm">
        {access.isAdmin
          ? t("company_bulk.admin_hint")
          : t("company_bulk.office_manager_hint")}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={view}
          onValueChange={(v) => {
            if (v) setView(v as View);
          }}
        >
          <ToggleGroupItem value="with_profile">
            {t("company_bulk.view.with_profile")}
          </ToggleGroupItem>
          <ToggleGroupItem value="without_profile">
            {t("company_bulk.view.without_profile")}
          </ToggleGroupItem>
        </ToggleGroup>
        <Select
          value={office}
          onValueChange={(v) => {
            setOffice(v);
            setGroup(ALL);
          }}
        >
          <SelectTrigger
            className="h-8 w-44"
            aria-label={t("company_bulk.filters.office")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>
              {t("company_bulk.filters.all_offices")}
            </SelectItem>
            {(officesQuery.data ?? []).map((o) => (
              <SelectItem key={o.uuid} value={o.uuid}>
                {o.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={group} onValueChange={setGroup}>
          <SelectTrigger
            className="h-8 w-44"
            aria-label={t("company_bulk.filters.group")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>
              {t("company_bulk.filters.all_groups")}
            </SelectItem>
            {groupOptions.map((g) => (
              <SelectItem key={g.uuid} value={g.uuid}>
                {g.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="ms-auto flex flex-wrap gap-2">
          {access.canImport ? (
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setImportOpen(true)}
            >
              <Upload className="me-1.5 size-3.5" />
              {t("company_bulk.import.button")}
            </Button>
          ) : null}
          {view === "with_profile" ? (
            <Button
              type="button"
              size="sm"
              disabled={selection.selectedCount === 0 || actions.length === 0}
              onClick={() => setDialogOpen(true)}
            >
              <Layers className="me-1.5 size-3.5" />
              {t("company_bulk.apply", { count: selection.selectedCount })}
            </Button>
          ) : null}
        </div>
      </div>
      <SelectionBanner
        selectedCount={selection.selectedCount}
        total={total}
        showSelectAll={selection.showSelectAllBanner}
        allMatchingSelected={selection.scope.mode === "all"}
        onSelectAllMatching={selection.selectAllMatching}
        onClearSelection={selection.clearSelection}
      />
      <EntityTable
        columns={columns}
        data={rows}
        getRowId={(row) => row.profile?.uuid ?? `member-${row.user_uuid}`}
        isLoading={membersQuery.isLoading}
        isError={membersQuery.isError}
        onRetry={() => void membersQuery.refetch()}
        emptyTitle={t(`company_bulk.empty.${view}`)}
        emptyDescription={t("company_bulk.empty.description")}
        pageCount={pageCount}
        state={{
          ...listState.tableState,
          rowSelection: selection.rowSelection,
          onRowSelectionChange: selection.onRowSelectionChange,
        }}
        manual={{ sorting: false }}
        features={{
          persistKey: "app-company-bulk-v1",
          rowSelection: view === "with_profile",
          sorting: false,
          facetedFilters: false,
          columnFilters: false,
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void membersQuery.refetch()}
            refreshDisabled={membersQuery.isFetching}
          />
        }
      />
      <BulkJobsCard />
      <BulkActionDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        actions={actions}
        target={target}
        targetCount={selection.selectedCount}
        names={names}
      />
      <LinkProfileDialog
        member={
          linkTarget
            ? {
                user_uuid: linkTarget.user_uuid,
                name: memberName(linkTarget),
                profileUuid: linkTarget.profile?.uuid,
              }
            : null
        }
        onOpenChange={(open) => (open ? undefined : setLinkTarget(null))}
      />
      {access.canImport ? (
        <ImportWizard
          resource="app.company_employees"
          open={importOpen}
          onOpenChange={setImportOpen}
          onComplete={() => void membersQuery.refetch()}
        />
      ) : null}
    </div>
  );
}
