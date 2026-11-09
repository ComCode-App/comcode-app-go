"use client";

import { Building2 } from "lucide-react";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  createNavAdornment,
  defineNavItem,
  type NavAdornment,
} from "@/features/nav-engine";
import { useCompaniesNavStats } from "@/features/companies/nav/use-companies-nav-stats";

function useCompaniesNavAdornment(): NavAdornment {
  const { data } = useCompaniesNavStats();
  const total = data?.total ?? 0;

  return {
    badges: [{ kind: "count", value: total, variant: "secondary" }],
  };
}

export const CompaniesNavAdornment = createNavAdornment(useCompaniesNavAdornment);

export const companiesNavItem = defineNavItem({
  id: "companies",
  titleKey: "layout.nav_companies",
  href: routes.platform.companies.root,
  icon: Building2,
  permission: permissions.companies.read,
  Adornment: CompaniesNavAdornment,
});
