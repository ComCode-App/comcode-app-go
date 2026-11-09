"use client";

import Link from "next/link";

import { EmptyState } from "@/components/common/empty-state";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

/**
 * Platform home for staff without `platform.overview.read` (TEC-944): the KPIs stay super admin
 * only, so instead of a permission error this points to the work the user can do.
 */
export function StaffHome() {
  const { t } = useLocale();
  const { can } = usePermission();

  const links = [
    {
      key: "orders_pipeline",
      href: routes.platform.ordersPipeline,
      visible: can(permissions.orders.read),
    },
    {
      key: "orders",
      href: routes.platform.orders,
      visible: can(permissions.orders.read),
    },
    {
      key: "profile_verifications",
      href: routes.platform.profileVerifications,
      visible: can(permissions.profileVerifications.manage),
    },
    {
      key: "products",
      href: routes.platform.products,
      visible: can(permissions.products.read),
    },
    {
      key: "subscriptions",
      href: routes.platform.settings.subscriptions,
      visible: can(permissions.subscriptions.read),
    },
    {
      key: "analytics",
      href: routes.platform.analytics.root,
      visible: can(permissions.analytics.read),
    },
  ].filter((link) => link.visible);

  return (
    <EmptyState
      title={t("dashboard.staff.title")}
      description={
        links.length
          ? t("dashboard.staff.description")
          : t("dashboard.staff.none")
      }
      action={
        links.length ? (
          <div className="flex flex-wrap justify-center gap-2">
            {links.map((link, i) => (
              <Button
                key={link.key}
                asChild
                size="sm"
                variant={i === 0 ? "default" : "outline"}
              >
                <Link href={link.href}>{t(`dashboard.staff.${link.key}`)}</Link>
              </Button>
            ))}
          </div>
        ) : undefined
      }
    />
  );
}
