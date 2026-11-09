"use client";

import { useMemo } from "react";

import { permissions } from "@/config/permissions";
import { useAuthStore } from "@/lib/auth/session-store";
import { usePermission } from "@/providers/permission-provider";

/**
 * UI hints for company bulk operations (TEC-573). Owners/admins act on every employee profile,
 * office managers on profiles of members in their granted offices (the API checks every item);
 * import and profile links are owner/admin-only.
 */
export function useCompanyBulkAccess() {
  const { can } = usePermission();
  const account = useAuthStore((s) => s.user?.account);
  const isCompany = account?.type === "company";
  const role = account?.role;
  const isAdmin = role === "owner" || role === "admin";
  const isOfficeManager = role === "office_manager";

  return useMemo(
    () => ({
      isCompany,
      isAdmin,
      isOfficeManager,
      canBulk:
        isCompany &&
        (isAdmin || isOfficeManager) &&
        can(permissions.companyProfiles.bulk),
      canImport:
        isCompany && isAdmin && can(permissions.companyProfiles.import),
      canLink: isCompany && isAdmin && can(permissions.profiles.write),
    }),
    [can, isAdmin, isCompany, isOfficeManager],
  );
}

export type CompanyBulkAccess = ReturnType<typeof useCompanyBulkAccess>;
