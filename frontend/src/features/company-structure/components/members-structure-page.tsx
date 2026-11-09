"use client";

import { useQuery } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { KeyRound, MapPin } from "lucide-react";
import Link from "next/link";
import { useCallback, useMemo, useState } from "react";

import {
  EntityPage,
  type EntityRowAction,
  EntityRowActions,
  EntityTable,
  EntityToolbar,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import {
  type AccountMember,
  accountService,
} from "@/features/accounts/services/account.service";
import { useAuthStore } from "@/lib/auth/session-store";
import { useLocale } from "@/providers/locale-provider";

import {
  useCompanyAssignments,
  useCompanyGroups,
  useCompanyOffices,
  useMemberScopes,
} from "../hooks/use-company-structure";
import {
  type CompanyStructureAccess,
  useCompanyStructureAccess,
} from "../lib/access";
import type { CompanyAssignment } from "../services/company-structure.service";
import { AssignmentDialog } from "./assignment-dialog";
import { ScopesDialog } from "./scopes-dialog";
import { CompanyStructureGate } from "./structure-gate";

type MemberRow = AccountMember & {
  fullName: string;
  assignment?: CompanyAssignment;
  assignmentLoading: boolean;
  scopes?: string[];
};

/** Members placed into offices and groups; office manager scope grants. */
export function MembersStructurePage() {
  const { t } = useLocale();
  const access = useCompanyStructureAccess();

  return (
    <EntityPage
      title={t("company_structure.members.title")}
      description={t("company_structure.members.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("company_structure.section") },
        { label: t("company_structure.members.title") },
      ]}
    >
      <CompanyStructureGate>
        <MembersContent access={access} />
      </CompanyStructureGate>
    </EntityPage>
  );
}

function MembersContent({ access }: { access: CompanyStructureAccess }) {
  const { t } = useLocale();
  const selfUuid = useAuthStore((s) => s.user?.uuid);
  const membersQuery = useQuery({
    queryKey: ["account", "members"],
    queryFn: accountService.members,
  });
  const officesQuery = useCompanyOffices();
  const groupsQuery = useCompanyGroups();

  const members = useMemo(
    () => membersQuery.data?.items ?? [],
    [membersQuery.data],
  );
  // Only owners/admins may read other managers' grants (TEC-948).
  const managerUuids = useMemo(
    () =>
      members
        .filter(
          (m) =>
            m.role === "office_manager" &&
            (access.isAdmin || m.user_uuid === selfUuid),
        )
        .map((m) => m.user_uuid),
    [access.isAdmin, members, selfUuid],
  );
  const assignmentsQuery = useCompanyAssignments();
  const scopeQueries = useMemberScopes(managerUuids);

  const officeNames = useMemo(
    () => new Map((officesQuery.data ?? []).map((o) => [o.uuid, o.name])),
    [officesQuery.data],
  );
  const groupNames = useMemo(
    () => new Map((groupsQuery.data ?? []).map((g) => [g.uuid, g.name])),
    [groupsQuery.data],
  );

  const rows = useMemo<MemberRow[]>(() => {
    const scopesByUser = new Map<string, string[]>();
    managerUuids.forEach((uuid, i) => {
      const data = scopeQueries[i]?.data;
      if (data) scopesByUser.set(uuid, data.office_uuids);
    });
    const assignmentByUser = new Map(
      (assignmentsQuery.data ?? []).map((a) => [a.user_uuid, a]),
    );
    // Non-admins see only the members the assignments API returns for them.
    const visible =
      access.isAdmin || !assignmentsQuery.data
        ? members
        : members.filter((m) => assignmentByUser.has(m.user_uuid));
    return visible.map((member) => ({
      ...member,
      fullName: `${member.name} ${member.surname}`.trim() || member.email,
      assignment: assignmentByUser.get(member.user_uuid),
      assignmentLoading: assignmentsQuery.isLoading,
      scopes: scopesByUser.get(member.user_uuid),
    }));
  }, [
    access.isAdmin,
    assignmentsQuery.data,
    assignmentsQuery.isLoading,
    managerUuids,
    members,
    scopeQueries,
  ]);

  const [assignTarget, setAssignTarget] = useState<MemberRow | null>(null);
  const [scopeTarget, setScopeTarget] = useState<MemberRow | null>(null);

  const editAssignment = useCallback(
    (row: MemberRow) => setAssignTarget(row),
    [],
  );
  const editScopes = useCallback((row: MemberRow) => setScopeTarget(row), []);

  const columns = useMemo<ColumnDef<MemberRow, unknown>[]>(
    () => [
      createColumn<MemberRow>({
        accessorKey: "fullName",
        labelKey: "company_structure.members.fields.member",
        gridPrimary: true,
        cell: ({ row }) => (
          <div>
            <div className="font-medium">
              {row.original.fullName}
              {row.original.user_uuid === selfUuid ? (
                <span className="text-muted-foreground ml-1 text-xs">
                  ({t("accounts.you")})
                </span>
              ) : null}
            </div>
            <div className="text-muted-foreground text-xs">
              {row.original.email}
            </div>
          </div>
        ),
      }),
      createColumn<MemberRow>({
        accessorKey: "role",
        labelKey: "company_structure.members.fields.role",
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.role === "office_manager" ? "default" : "secondary"
            }
          >
            {t(`accounts.roles.${row.original.role}`)}
          </Badge>
        ),
      }),
      createColumn<MemberRow>({
        id: "office",
        accessorFn: (row) =>
          row.assignment?.office_uuid
            ? (officeNames.get(row.assignment.office_uuid) ?? "")
            : "",
        labelKey: "company_structure.members.fields.office",
        cell: ({ row }) => {
          const uuid = row.original.assignment?.office_uuid;
          if (row.original.assignmentLoading)
            return <span className="text-muted-foreground">…</span>;
          return uuid ? (
            <span className="text-sm">{officeNames.get(uuid) ?? "—"}</span>
          ) : (
            <span className="text-muted-foreground text-sm">
              {t("company_structure.members.unassigned")}
            </span>
          );
        },
      }),
      createColumn<MemberRow>({
        id: "group",
        accessorFn: (row) =>
          row.assignment?.group_uuid
            ? (groupNames.get(row.assignment.group_uuid) ?? "")
            : "",
        labelKey: "company_structure.members.fields.group",
        cell: ({ row }) => {
          const uuid = row.original.assignment?.group_uuid;
          return uuid ? (
            <span className="text-sm">{groupNames.get(uuid) ?? "—"}</span>
          ) : (
            <span className="text-muted-foreground text-sm">—</span>
          );
        },
      }),
      createColumn<MemberRow>({
        id: "scopes",
        labelKey: "company_structure.members.fields.managed_offices",
        enableSorting: false,
        cell: ({ row }) => {
          if (row.original.role !== "office_manager") {
            return <span className="text-muted-foreground text-sm">—</span>;
          }
          const scopes = row.original.scopes ?? [];
          if (!scopes.length) {
            return (
              <Badge
                variant="outline"
                className="text-amber-600 dark:text-amber-400"
              >
                {t("company_structure.members.no_scopes")}
              </Badge>
            );
          }
          return (
            <div className="flex flex-wrap gap-1">
              {scopes.map((uuid) => (
                <Badge key={uuid} variant="outline">
                  {officeNames.get(uuid) ?? "—"}
                </Badge>
              ))}
            </div>
          );
        },
      }),
      createColumn<MemberRow>({
        id: "actions",
        labelKey: "table.actions",
        enableSorting: false,
        enableHiding: false,
        enableResizing: false,
        enableColumnFilter: false,
        cell: ({ row }) => {
          const actions: EntityRowAction[] = [];
          if (
            !row.original.assignmentLoading &&
            access.canEditAssignment(row.original.assignment, row.original.role)
          ) {
            actions.push({
              id: "assign",
              label: t("company_structure.members.edit_assignment"),
              icon: MapPin,
              onSelect: () => editAssignment(row.original),
            });
          }
          if (
            access.canManageScopes &&
            row.original.role === "office_manager"
          ) {
            actions.push({
              id: "scopes",
              label: t("company_structure.members.edit_scopes"),
              icon: KeyRound,
              onSelect: () => editScopes(row.original),
            });
          }
          return actions.length ? <EntityRowActions actions={actions} /> : null;
        },
      }),
    ],
    [access, editAssignment, editScopes, groupNames, officeNames, selfUuid, t],
  );

  return (
    <div className="space-y-4">
      <p className="text-muted-foreground text-sm">
        {access.isAdmin
          ? t("company_structure.members.admin_hint")
          : access.isOfficeManager
            ? t("company_structure.members.office_manager_hint")
            : t("company_structure.read_only_hint")}{" "}
        {access.isAdmin ? (
          <Link
            href={routes.app.account.root}
            className="text-primary underline-offset-4 hover:underline"
          >
            {t("company_structure.members.roles_link")}
          </Link>
        ) : null}
      </p>
      <EntityTable
        columns={columns}
        data={rows}
        getRowId={(row) => row.user_uuid}
        isLoading={membersQuery.isLoading}
        isError={membersQuery.isError}
        onRetry={() => void membersQuery.refetch()}
        emptyTitle={t("company_structure.members.empty_title")}
        emptyDescription={t("company_structure.members.empty_description")}
        manual={{ sorting: false, filtering: false, pagination: false }}
        features={{ persistKey: "app-company-members-v1", rowSelection: false }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => {
              void membersQuery.refetch();
              void assignmentsQuery.refetch();
            }}
            refreshDisabled={membersQuery.isFetching}
          />
        }
      />
      <AssignmentDialog
        open={Boolean(assignTarget)}
        onOpenChange={(open) => (open ? undefined : setAssignTarget(null))}
        member={
          assignTarget
            ? { user_uuid: assignTarget.user_uuid, name: assignTarget.fullName }
            : null
        }
        assignment={assignTarget?.assignment}
        offices={officesQuery.data ?? []}
        groups={groupsQuery.data ?? []}
        access={access}
      />
      <ScopesDialog
        open={Boolean(scopeTarget)}
        onOpenChange={(open) => (open ? undefined : setScopeTarget(null))}
        member={
          scopeTarget
            ? { user_uuid: scopeTarget.user_uuid, name: scopeTarget.fullName }
            : null
        }
        officeUuids={scopeTarget?.scopes ?? []}
        offices={officesQuery.data ?? []}
      />
    </div>
  );
}
