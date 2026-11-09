"use client";

import { permissions } from "@/config/permissions";
import { useAuthStore } from "@/lib/auth/session-store";
import { usePermission } from "@/providers/permission-provider";

/**
 * UI hint for company analytics (TEC-574): owners/admins see the whole company, office managers
 * their granted offices (the API applies the scope); plain members and personal accounts cannot.
 */
export function useCompanyAnalyticsAccess() {
  const { can } = usePermission();
  const account = useAuthStore((s) => s.user?.account);
  const isCompany = account?.type === "company";
  const role = account?.role;
  const isAdmin = role === "owner" || role === "admin";
  const isOfficeManager = role === "office_manager";
  return {
    isCompany,
    isAdmin,
    isOfficeManager,
    canRead: isCompany && (isAdmin || isOfficeManager) && can(permissions.companyAnalytics.read),
  };
}
