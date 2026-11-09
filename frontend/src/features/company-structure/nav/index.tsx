import {
  BarChart3,
  Building,
  Layers,
  MapPin,
  Network,
  Palette,
  UsersRound,
} from "lucide-react";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { defineNavGroup } from "@/features/nav-engine";

/** App-shell section shown only while the active account is a company (TEC-611). */
export const companyStructureNavGroup = defineNavGroup({
  id: "company-structure",
  labelKey: "layout.section_company",
  icon: Network,
  defaultOpen: true,
  accountType: "company",
  permission: permissions.companyStructure.read,
  items: [
    {
      id: "company-offices",
      titleKey: "layout.nav_company_offices",
      href: routes.app.company.offices,
      icon: Building,
    },
    {
      id: "company-groups",
      titleKey: "layout.nav_company_groups",
      href: routes.app.company.groups,
      icon: UsersRound,
    },
    {
      id: "company-members",
      titleKey: "layout.nav_company_members",
      href: routes.app.company.members,
      icon: MapPin,
    },
    {
      id: "company-bulk",
      titleKey: "layout.nav_company_bulk",
      href: routes.app.company.bulk,
      icon: Layers,
      permission: permissions.companyProfiles.bulk,
      // TEC-948: bulk actions are an owner/admin menu even though office managers hold the permission.
      accountRoles: ["owner", "admin"],
    },
    {
      id: "company-analytics",
      titleKey: "layout.nav_company_analytics",
      href: routes.app.company.analytics,
      icon: BarChart3,
      permission: permissions.companyAnalytics.read,
    },
    {
      id: "company-branding",
      titleKey: "layout.nav_company_branding",
      href: routes.app.company.branding,
      icon: Palette,
      permission: permissions.companyBranding.read,
    },
  ],
});
