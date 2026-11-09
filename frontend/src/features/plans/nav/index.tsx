"use client";

import { CreditCard } from "lucide-react";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  createNavAdornment,
  defineNavItem,
  type NavAdornment,
} from "@/features/nav-engine";
import { usePlansNavStats } from "@/features/plans/nav/use-plans-nav-stats";

function usePlansNavAdornment(): NavAdornment {
  const { data } = usePlansNavStats();
  const total = data?.total ?? 0;

  return {
    badges: [{ kind: "count", value: total, variant: "secondary" }],
  };
}

export const PlansNavAdornment = createNavAdornment(usePlansNavAdornment);

export const plansNavItem = defineNavItem({
  id: "plans",
  titleKey: "layout.nav_plans",
  href: routes.platform.plans.root,
  icon: CreditCard,
  permission: permissions.plans.read,
  Adornment: PlansNavAdornment,
});
