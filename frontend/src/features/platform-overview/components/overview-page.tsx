"use client";

import { EmptyState } from "@/components/common/empty-state";
import { ErrorState } from "@/components/common/error-state";
import { PageHeader } from "@/components/layout";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { DashboardKpiGrid } from "@/features/platform-overview/components/dashboard-kpi-grid";
import { EventVolumeCard } from "@/features/platform-overview/components/event-volume-card";
import { ConversionFunnelCard } from "@/features/platform-overview/components/conversion-funnel-card";
import { OrderFunnelCard } from "@/features/platform-overview/components/order-funnel-card";
import { OverviewQuickLinks } from "@/features/platform-overview/components/overview-quick-links";
import { StaffHome } from "@/features/platform-overview/components/staff-home";
import { SubscriptionsCard } from "@/features/platform-overview/components/subscriptions-card";
import { TopProfilesCard } from "@/features/platform-overview/components/top-profiles-card";
import { useOverview } from "@/features/platform-overview/hooks/use-overview-query";
import { isApiError } from "@/lib/api";
import { isPlatformUser } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

/**
 * /platform home (TEC-594): business KPIs from GET /v1/platform/overview, then quick links. Staff
 * without platform.overview.read get pointers to their own work instead (TEC-944).
 */
export function OverviewPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const { can } = usePermission();

  const isPlatform = isPlatformUser(user);
  const canRead = isPlatform && can(permissions.overview.read);
  const query = useOverview(canRead);
  const { data, error } = query;
  const loading = query.isLoading;

  const forbidden = !isPlatform || (isApiError(error) && error.isForbidden);
  const updatedAt = data
    ? new Date(data.generated_at).toLocaleTimeString(locale, {
        hour: "2-digit",
        minute: "2-digit",
      })
    : null;

  return (
    <>
      <PageHeader
        title={t("layout.home")}
        description={t("dashboard.description")}
        breadcrumbs={[
          { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        ]}
      />

      {forbidden ? (
        <EmptyState
          title={t("common.error_forbidden")}
          description={t("dashboard.forbidden_description")}
        />
      ) : (
        <div className="space-y-8">
          {!canRead ? (
            <StaffHome />
          ) : error ? (
            <ErrorState
              title={t("common.error_generic")}
              description={
                isApiError(error)
                  ? error.message
                  : t("dashboard.error_description")
              }
              onRetry={() => void query.refetch()}
              retryLabel={t("common.retry")}
            />
          ) : (
            <div className="space-y-6">
              {updatedAt ? (
                <p className="text-muted-foreground text-xs">
                  {t("dashboard.updated_at", { time: updatedAt })}
                </p>
              ) : null}
              <DashboardKpiGrid data={data} loading={loading} />
              <div className="grid gap-6 lg:grid-cols-2">
                <ConversionFunnelCard />
                <SubscriptionsCard data={data} loading={loading} />
              </div>
              <OrderFunnelCard data={data} loading={loading} />
              <EventVolumeCard data={data} loading={loading} />
              <TopProfilesCard data={data} loading={loading} />
            </div>
          )}
          <OverviewQuickLinks />
        </div>
      )}
    </>
  );
}
