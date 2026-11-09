"use client";

import { permissions } from "@/config/permissions";
import { useAuthStore } from "@/lib/auth/session-store";
import { usePermission } from "@/providers/permission-provider";

/**
 * UI hints for company branding (TEC-946): reading needs a company account and
 * `app.company_branding.read`; changes also need the write permission and an owner/admin
 * role. The API enforces the same rules, so callers still handle 403.
 */
export function useCompanyBrandingAccess() {
  const { can } = usePermission();
  const account = useAuthStore((s) => s.user?.account);
  const isCompany = account?.type === "company";
  const canRead = isCompany && can(permissions.companyBranding.read);
  const isAdmin = account?.role === "owner" || account?.role === "admin";
  return {
    isCompany,
    canRead,
    canWrite: canRead && isAdmin && can(permissions.companyBranding.write),
  };
}
