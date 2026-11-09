"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowRight, ListTree, Settings2, TableProperties } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";

import { AppChart, BarList } from "@/components/charts";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/common/card";
import { ErrorState } from "@/components/common/error-state";
import { StatusChip } from "@/components/common/status-chip";
import { EntityPage } from "@/components/entity";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  eventTypeLabel,
  subjectTypeLabel,
} from "@/features/analytics/lib/event-labels";
import { datetime } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

import { platformAnalyticsKeys } from "../hooks/query-keys";
import {
  useAnalyticsConfig,
  useEventDefinitions,
} from "../hooks/use-platform-analytics";
import { dayStartIso, daysAgo } from "../lib/time-range";
import { platformAnalyticsService } from "../services/platform-analytics.service";

const CHART_DAYS = 7;
const RECENT_LIMIT = 8;
/** The overview reads the rollups (cheap), so a minute of staleness is plenty. */
const OVERVIEW_STALE_MS = 60_000;

/** The viewer's IANA zone, so the API buckets events into the same local days as the UI. */
function viewerTimeZone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || undefined;
  } catch {
    return undefined;
  }
}

const DELIVERY_MODES = new Set(["async", "sync", "hybrid"]);

/** `async` → "Async (kuyruk)"; unknown modes are shown as-is. */
function deliveryModeLabel(t: (key: string) => string, mode: string) {
  return DELIVERY_MODES.has(mode)
    ? t(`platform_analytics.config.mode_${mode}`)
    : mode;
}

function eventsHref(params: Record<string, string>) {
  const qs = new URLSearchParams(params).toString();
  return qs
    ? `${routes.platform.analytics.events}?${qs}`
    : routes.platform.analytics.events;
}

function Kpi({
  label,
  value,
  hint,
  href,
  loading,
}: {
  label: string;
  value: string;
  hint?: string;
  href?: string;
  loading?: boolean;
}) {
  const body = (
    <>
      <CardHeader className="pb-1">
        <CardTitle className="text-muted-foreground text-sm font-medium">
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <Skeleton className="h-8 w-20" />
        ) : (
          <p className="text-2xl font-semibold tracking-tight tabular-nums">
            {value}
          </p>
        )}
        {hint ? (
          <p className="text-muted-foreground mt-1 text-xs">{hint}</p>
        ) : null}
      </CardContent>
    </>
  );
  if (!href) return <Card className="shadow-none">{body}</Card>;
  return (
    <Card className="hover:border-primary/40 hover:bg-muted/30 shadow-none transition-colors">
      <Link href={href} className="block focus-visible:outline-none">
        {body}
      </Link>
    </Card>
  );
}

/**
 * Platform-wide analytics overview (TEC-565). KPIs and the daily chart come from
 * `GET /v1/platform/analytics/overview`, which sums the hourly rollups per local day (TEC-859);
 * only the "latest events" card reads raw rows (8 of them).
 */
export function PlatformAnalyticsOverviewPage() {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const canRead = can(permissions.analytics.read);
  const canManageEvents = can(permissions.events.manage);
  const fmt = useMemo(() => new Intl.NumberFormat(locale), [locale]);

  const tz = useMemo(() => viewerTimeZone() ?? "", []);
  const monthFrom = useMemo(() => daysAgo(29), []);
  const weekFrom = useMemo(() => daysAgo(CHART_DAYS - 1), []);
  const today = useMemo(() => daysAgo(0), []);

  const overview = useQuery({
    queryKey: platformAnalyticsKeys.overview(tz),
    queryFn: () => platformAnalyticsService.getOverview(tz || undefined),
    enabled: canRead,
    staleTime: OVERVIEW_STALE_MS,
    refetchOnWindowFocus: false,
  });

  const recentParams = useMemo(
    () => ({ from: dayStartIso(monthFrom), limit: RECENT_LIMIT, offset: 0 }),
    [monthFrom],
  );
  const recent = useQuery({
    queryKey: platformAnalyticsKeys.eventList(recentParams),
    queryFn: () => platformAnalyticsService.listEvents(recentParams),
    enabled: canRead,
    staleTime: OVERVIEW_STALE_MS,
    refetchOnWindowFocus: false,
  });

  const config = useAnalyticsConfig(canRead);
  const definitions = useEventDefinitions({}, canManageEvents);

  const ov = overview.data;
  const dailyLoading = overview.isLoading;
  const todayCount = ov?.today ?? 0;
  const weekCount = ov?.last_7_days ?? 0;

  const chartData = useMemo(
    () =>
      (ov?.daily ?? []).slice(-CHART_DAYS).map((d) => ({
        label: new Date(`${d.date}T00:00:00`).toLocaleDateString(locale, {
          day: "2-digit",
          month: "short",
        }),
        events: d.count,
      })),
    [ov, locale],
  );

  const defs = useMemo(() => definitions.data?.items ?? [], [definitions.data]);
  const defNames = useMemo(
    () => new Map(defs.map((d) => [d.key, d.name])),
    [defs],
  );
  const enabledDefs = defs.filter((d) => d.enabled).length;
  const publicDefs = defs.filter((d) => d.is_public).length;
  const cfg = config.data;

  return (
    <EntityPage
      title={t("platform_analytics.overview.title")}
      description={t("platform_analytics.overview.subtitle")}
      permission={permissions.analytics.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("platform_analytics.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("layout.nav_platform_analytics") },
      ]}
      actions={
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" asChild className="gap-2">
            <Link href={routes.platform.analytics.events}>
              <TableProperties className="size-4" />
              {t("platform_analytics.overview.open_events")}
            </Link>
          </Button>
          {canManageEvents ? (
            <Button variant="outline" asChild className="gap-2">
              <Link href={routes.platform.analytics.definitions}>
                <ListTree className="size-4" />
                {t("platform_analytics.overview.open_definitions")}
              </Link>
            </Button>
          ) : null}
        </div>
      }
    >
      {cfg ? (
        <div className="bg-card flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border px-4 py-3 text-sm">
          <span className="font-medium">
            {t("platform_analytics.overview.pipeline")}
          </span>
          <StatusChip
            label={
              cfg.enabled
                ? t("platform_analytics.overview.pipeline_on")
                : t("platform_analytics.overview.pipeline_off")
            }
            tone={cfg.enabled ? "success" : "danger"}
          />
          <span className="text-muted-foreground">
            {t("platform_analytics.config.event_delivery_mode")}:{" "}
            <span className="text-foreground">
              {cfg.event_delivery_mode
                ? deliveryModeLabel(t, cfg.event_delivery_mode)
                : "—"}
            </span>
          </span>
          <span className="text-muted-foreground">
            {t("platform_analytics.config.realtime_enabled")}:{" "}
            <span className="text-foreground">
              {cfg.realtime_enabled ? t("common.yes") : t("common.no")}
            </span>
          </span>
          <span className="text-muted-foreground">
            {t("platform_analytics.overview.raw_retention")}:{" "}
            <span className="text-foreground tabular-nums">
              {t("platform_analytics.definitions.days", {
                count: cfg.raw_event_retention_days ?? 0,
              })}
            </span>
          </span>
          <Link
            href={routes.platform.analytics.config}
            className="text-primary ml-auto inline-flex items-center gap-1 text-xs font-medium hover:underline"
          >
            <Settings2 className="size-3.5" />
            {t("platform_analytics.overview.open_config")}
          </Link>
        </div>
      ) : null}

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <Kpi
          label={t("platform_analytics.overview.kpi_today")}
          value={fmt.format(todayCount)}
          loading={dailyLoading}
          href={eventsHref({ from: today })}
        />
        <Kpi
          label={t("platform_analytics.overview.kpi_week")}
          value={fmt.format(weekCount)}
          loading={dailyLoading}
          href={eventsHref({ from: weekFrom })}
        />
        <Kpi
          label={t("platform_analytics.overview.kpi_month")}
          value={fmt.format(ov?.last_30_days ?? 0)}
          loading={dailyLoading}
          href={eventsHref({ from: monthFrom })}
        />
        <Kpi
          label={t("platform_analytics.overview.kpi_visitors")}
          value={fmt.format(ov?.unique_visitors?.last_30_days ?? 0)}
          hint={t("platform_analytics.overview.kpi_visitors_hint", {
            today: fmt.format(ov?.unique_visitors?.today ?? 0),
            week: fmt.format(ov?.unique_visitors?.last_7_days ?? 0),
          })}
          loading={dailyLoading}
        />
        {canManageEvents ? (
          <Kpi
            label={t("platform_analytics.overview.kpi_definitions")}
            value={`${fmt.format(enabledDefs)} / ${fmt.format(defs.length)}`}
            hint={t("platform_analytics.overview.kpi_definitions_hint", {
              public: publicDefs,
            })}
            loading={definitions.isLoading}
            href={routes.platform.analytics.definitions}
          />
        ) : (
          <Kpi
            label={t("platform_analytics.overview.kpi_avg")}
            value={fmt.format(Math.round(weekCount / CHART_DAYS))}
            loading={dailyLoading}
          />
        )}
      </div>

      {overview.isError || recent.isError ? (
        <ErrorState
          title={t("platform_analytics.load_error")}
          retryLabel={t("common.retry")}
          onRetry={() => {
            void overview.refetch();
            void recent.refetch();
          }}
        />
      ) : null}

      <div className="grid gap-6 lg:grid-cols-5">
        <div className="lg:col-span-3">
          <AppChart
            type="bar"
            title={t("platform_analytics.overview.chart_title")}
            description={t("platform_analytics.overview.chart_hint")}
            data={chartData}
            config={{
              events: {
                label: t("platform_analytics.overview.series_events"),
                color: "var(--chart-1)",
              },
            }}
            categoryKey="label"
            showLegend={false}
            height={260}
            loading={dailyLoading}
            emptyTitle={t("platform_analytics.overview.no_data")}
            valueFormatter={(v) => fmt.format(v)}
          />
        </div>

        <Card className="shadow-none lg:col-span-2">
          <CardHeader className="flex flex-row items-center justify-between gap-2 pb-2">
            <CardTitle className="text-sm font-semibold">
              {t("platform_analytics.overview.recent")}
            </CardTitle>
            <Link
              href={routes.platform.analytics.events}
              className="text-primary inline-flex items-center gap-1 text-xs font-medium hover:underline"
            >
              {t("platform_analytics.overview.see_all")}
              <ArrowRight className="size-3.5" />
            </Link>
          </CardHeader>
          <CardContent className="pt-0">
            {recent.isLoading ? (
              <div className="space-y-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-8 w-full" />
                ))}
              </div>
            ) : (recent.data?.items ?? []).length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {t("platform_analytics.overview.no_data")}
              </p>
            ) : (
              <ul className="divide-y">
                {(recent.data?.items ?? []).map((e) => (
                  <li
                    key={e.uuid}
                    className="flex items-center justify-between gap-3 py-2"
                  >
                    <div className="min-w-0">
                      <Link
                        href={eventsHref({ event_type: e.type })}
                        title={e.type}
                        className="block truncate text-sm font-medium hover:underline"
                      >
                        {eventTypeLabel(t, e.type, defNames.get(e.type))}
                      </Link>
                      <p className="text-muted-foreground truncate text-xs">
                        <span className="font-mono">{e.type}</span> ·{" "}
                        {subjectTypeLabel(t, e.subject.type)} ·{" "}
                        {[e.context?.city, e.context?.country_code]
                          .filter(Boolean)
                          .join(", ") || "—"}
                      </p>
                    </div>
                    <time
                      className="text-muted-foreground shrink-0 text-xs tabular-nums"
                      dateTime={e.occurred_at}
                    >
                      {datetime(e.occurred_at, "dd.MM HH:mm", locale)}
                    </time>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Top 10 event types of the 30-day window, from the hourly rollups (TEC-953). */}
      <Card className="shadow-none">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold">
            {t("platform_analytics.overview.top_event_types")}
          </CardTitle>
          <p className="text-muted-foreground text-xs">
            {t("platform_analytics.overview.top_event_types_hint")}
          </p>
        </CardHeader>
        <CardContent className="pt-0">
          {dailyLoading ? (
            <Skeleton className="h-40 w-full" />
          ) : (ov?.top_event_types ?? []).length === 0 ? (
            <p className="text-muted-foreground text-sm">
              {t("platform_analytics.overview.no_data")}
            </p>
          ) : (
            <BarList
              items={(ov?.top_event_types ?? []).map((e) => ({
                key: e.event_type,
                label: `${eventTypeLabel(t, e.event_type, defNames.get(e.event_type))} · ${e.event_type}`,
                count: e.count,
              }))}
              format={(n) => fmt.format(n)}
            />
          )}
        </CardContent>
      </Card>
    </EntityPage>
  );
}
