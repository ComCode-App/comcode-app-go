"use client";

import { CreditCard } from "lucide-react";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  createNavAdornment,
  defineNavItem,
  type NavAdornment,
} from "@/features/nav-engine";
import { useSubscriptionsNavStats } from "@/features/subscriptions/nav/use-subscriptions-nav-stats";

function useSubscriptionsNavAdornment(): NavAdornment {
  const { data } = useSubscriptionsNavStats();
  const total = data?.total ?? 0;

  return {
    badges: [{ kind: "count", value: total, variant: "secondary" }],
  };
}

export const SubscriptionsNavAdornment = createNavAdornment(
  useSubscriptionsNavAdornment,
);

export const subscriptionsNavItem = defineNavItem({
  id: "subscriptions",
  titleKey: "layout.nav_subscriptions",
  href: routes.platform.subscriptions.root,
  icon: CreditCard,
  permission: permissions.subscriptions.read,
  Adornment: SubscriptionsNavAdornment,
});
