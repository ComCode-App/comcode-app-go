"use client";

import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { permissions } from "@/config/permissions";
import { useAuthStore } from "@/lib/auth/session-store";
import { usePermission } from "@/providers/permission-provider";

import { companyStructureKeys } from "../hooks/query-keys";
import {
  type CompanyAssignment,
  type CompanyGroup,
  companyStructureService,
} from "../services/company-structure.service";

const EMPTY = new Set<string>();

/**
 * UI hints for company-structure writes, derived from the active account role, the RBAC
 * permission and (for office managers) the granted office scopes. Hints only hide or disable
 * controls; every write is still authorized by the API, so callers must handle 403.
 */
export function useCompanyStructureAccess() {
  const { can } = usePermission();
  const userUuid = useAuthStore((s) => s.user?.uuid);
  const account = useAuthStore((s) => s.user?.account);

  const isCompany = account?.type === "company";
  const canRead = isCompany && can(permissions.companyStructure.read);
  const hasWrite = canRead && can(permissions.companyStructure.write);
  const role = account?.role;
  const isAdmin = role === "owner" || role === "admin";
  const isOfficeManager = role === "office_manager";

  const scopesQuery = useQuery({
    queryKey: companyStructureKeys.scopes(userUuid ?? ""),
    queryFn: () => companyStructureService.getScopes(userUuid ?? ""),
    enabled: Boolean(userUuid) && hasWrite && isOfficeManager,
    staleTime: 30_000,
  });

  const managedOffices = useMemo(
    () =>
      isOfficeManager && scopesQuery.data
        ? new Set(scopesQuery.data.office_uuids)
        : EMPTY,
    [isOfficeManager, scopesQuery.data],
  );

  return useMemo(() => {
    const managesOffice = (officeUuid: string | null | undefined) =>
      Boolean(officeUuid) && managedOffices.has(officeUuid as string);
    const scoped = hasWrite && isOfficeManager && managedOffices.size > 0;

    return {
      isCompany,
      canRead,
      isAdmin,
      isOfficeManager,
      managedOffices,
      /** Offices, the sector and scope grants are owner/admin-only. */
      canManageOffices: hasWrite && isAdmin,
      canManageSector: hasWrite && isAdmin,
      canManageScopes: hasWrite && isAdmin,
      /** Office managers create groups only inside their granted offices. */
      canCreateGroups: (hasWrite && isAdmin) || scoped,
      /** Company-wide groups (null office) stay owner/admin-only. */
      canManageGroup: (group: Pick<CompanyGroup, "office_uuid">) =>
        hasWrite &&
        (isAdmin || (isOfficeManager && managesOffice(group.office_uuid))),
      /**
       * Office managers move members within their offices and claim unassigned plain members
       * into them (TEC-948); they never move members out of scope.
       */
      canEditAssignment: (
        assignment: Pick<CompanyAssignment, "office_uuid"> | undefined,
        memberRole: string,
      ) =>
        hasWrite &&
        (isAdmin ||
          (scoped &&
            (managesOffice(assignment?.office_uuid ?? null) ||
              (Boolean(assignment) &&
                !assignment?.office_uuid &&
                memberRole === "member")))),
      managesOffice,
    };
  }, [canRead, hasWrite, isAdmin, isCompany, isOfficeManager, managedOffices]);
}

export type CompanyStructureAccess = ReturnType<
  typeof useCompanyStructureAccess
>;
