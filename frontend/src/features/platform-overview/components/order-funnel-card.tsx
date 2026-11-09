"use client";

import { BarList } from "@/components/charts";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { OverviewSection } from "@/features/platform-overview/components/overview-section";
import { labelOr, numberFormatter } from "@/features/platform-overview/lib/format";
import type { PlatformOverview } from "@/features/platform-overview/services/overview.service";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

/** Orders of the last 30 days per current status, in lifecycle order. */
export function OrderFunnelCard({ data, loading }: { data?: PlatformOverview; loading?: boolean }) {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const fmt = numberFormatter(locale);
  const funnel = data?.orders.funnel ?? [];

  return (
    <OverviewSection
      title={t("dashboard.funnel.title")}
      description={t("dashboard.funnel.description")}
      action={can(permissions.orders.read) ? { href: routes.platform.orders, label: t("dashboard.funnel.open") } : undefined}
      loading={loading}
    >
      {!data || data.orders.total === 0 ? (
        <p className="text-muted-foreground text-sm">{t("dashboard.funnel.empty")}</p>
      ) : (
        <BarList
          limit={funnel.length}
          items={funnel.map((step) => ({
            key: step.status,
            label: labelOr(t, `orders.status_${step.status}`, step.status),
            count: step.count,
          }))}
          format={(v) => fmt.format(v)}
        />
      )}
    </OverviewSection>
  );
}
