"use client";

import { permissions } from "@/config/permissions";
import { useAuthStore } from "@/lib/auth/session-store";
import { usePermission } from "@/providers/permission-provider";

/**
 * UI hints for the business page (TEC-1327): a company account and `app.business_page.read` to
 * see it; changes also need the write permission and an owner/admin role. The API enforces the
 * same rules.
 */
export function useBusinessPageAccess() {
  const { can } = usePermission();
  const account = useAuthStore((s) => s.user?.account);
  const isCompany = account?.type === "company";
  const canRead = isCompany && can(permissions.businessPage.read);
  const isAdmin = account?.role === "owner" || account?.role === "admin";
  return {
    isCompany,
    canRead,
    canWrite: canRead && isAdmin && can(permissions.businessPage.write),
    accountName: account?.name ?? "",
    accountSlug: account?.slug ?? "",
  };
}
