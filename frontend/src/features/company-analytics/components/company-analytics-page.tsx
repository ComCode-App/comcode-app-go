"use client";

import { useQuery } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";

import { AppChart, BarList } from "@/components/charts";
import {
  EntityPage,
  EntityTable,
  EntityToolbar,
  EntityUnavailable,
  useServerListState,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { routes } from "@/config/routes";
import { eventCountLabel } from "@/features/analytics/lib/event-labels";
import {
  useCompanyGroups,
  useCompanyOffices,
} from "@/features/company-structure/hooks/use-company-structure";
import { useEntitlements } from "@/features/entitlements";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

import { useCompanyAnalyticsAccess } from "../lib/access";
import {
  type CompanyAnalyticsEmployee,
  type CompanyAnalyticsMetrics,
  type CompanyAnalyticsScope,
  companyAnalyticsKeys,
  companyAnalyticsService,
} from "../services/company-analytics.service";

const ALL = "__all__";
const RANGES = [7, 30, 90, 365] as const;
const DIMENSIONS = ["country", "city", "device", "browser", "referer"] as const;
/** Metric columns, in display order; ids equal the API's sort fields. */
const METRICS = [
  "profile_views",
  "card_taps",
  "qr_scans",
  "contact_saves",
  "link_clicks",
  "contact_clicks",
  "leads",
] as const satisfies readonly (keyof CompanyAnalyticsMetrics)[];
const SERIES = ["profile_views", "card_taps", "contact_saves", "link_clicks"] as const;

function isoDay(d: Date) {
  return d.toISOString().slice(0, 10);
}

/** Company → Analytics (TEC-574): company totals, daily activity and per-employee comparison. */
export function CompanyAnalyticsPage() {
  const { t } = useLocale();
  const access = useCompanyAnalyticsAccess();

  let body = <CompanyAnalyticsContent isAdmin={access.isAdmin} />;
  if (!access.isCompany) {
    body = (
      <EntityUnavailable
        title={t("company_structure.unavailable.personal_title")}
        description={t("analytics.company.unavailable_personal")}
      />
    );
  } else if (!access.canRead) {
    body = (
      <EntityUnavailable
        title={t("common.error_forbidden")}
        description={t("analytics.company.unavailable_forbidden")}
      />
    );
  }

  return (
    <EntityPage
      title={t("analytics.company.title")}
      description={t("analytics.company.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("company_structure.section") },
        { label: t("analytics.company.title") },
      ]}
    >
      {body}
    </EntityPage>
  );
}

function CompanyAnalyticsContent({ isAdmin }: { isAdmin: boolean }) {
  const { t, locale } = useLocale();
  const fmt = useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const pct = useMemo(
    () => new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }),
    [locale],
  );

  // The plan caps how far back analytics can go; longer ranges are shown but disabled.
  const history = useEntitlements().data?.features.find((f) => f.key === "analytics.history_days");
  const maxDays = history?.unlimited ? Infinity : (history?.int_value ?? 30);
  const [days, setDays] = useState<number>(30);
  const effectiveDays = Math.min(days, maxDays);
  const [office, setOffice] = useState(ALL);
  const [group, setGroup] = useState(ALL);
  const [dimension, setDimension] = useState<(typeof DIMENSIONS)[number]>("country");

  const offices = useCompanyOffices();
  const groups = useCompanyGroups();
  const groupOptions = useMemo(
    () => (groups.data ?? []).filter((g) => office === ALL || !g.office_uuid || g.office_uuid === office),
    [groups.data, office],
  );

  const scope: CompanyAnalyticsScope = useMemo(() => {
    const to = new Date();
    const from = new Date(to.getTime() - (effectiveDays - 1) * 86_400_000);
    return {
      from: isoDay(from),
      to: isoDay(to),
      office_uuid: office === ALL ? undefined : office,
      group_uuid: group === ALL ? undefined : group,
    };
  }, [effectiveDays, office, group]);

  const summary = useQuery({
    queryKey: companyAnalyticsKeys.summary(scope),
    queryFn: () => companyAnalyticsService.summary(scope),
  });
  const series = useQuery({
    queryKey: companyAnalyticsKeys.timeseries(scope),
    queryFn: () => companyAnalyticsService.timeseries(scope),
  });
  const breakdown = useQuery({
    queryKey: companyAnalyticsKeys.breakdown(scope, dimension),
    queryFn: () => companyAnalyticsService.breakdown(scope, dimension),
  });

  const listState = useServerListState({ initialPageSize: 20, initialSort: "-total_events" });
  const employeeQuery = useMemo(() => ({ ...scope, ...listState.params }), [scope, listState.params]);
  const employees = useQuery({
    queryKey: companyAnalyticsKeys.employees(employeeQuery),
    queryFn: () => companyAnalyticsService.employees(employeeQuery),
  });

  const totals = summary.data?.totals;
  const kpis = [
    ...METRICS.map((key) => ({ key, value: totals ? fmt.format(totals[key]) : "–" })),
    {
      key: "active_profiles",
      value: summary.data
        ? `${fmt.format(summary.data.active_profiles)} / ${fmt.format(summary.data.profiles)}`
        : "–",
    },
  ];

  const chartData = useMemo(
    () =>
      (series.data?.points ?? []).map((p) => ({
        label: new Date(`${p.date}T00:00:00Z`).toLocaleDateString(locale, {
          day: "2-digit",
          month: "short",
          timeZone: "UTC",
        }),
        ...Object.fromEntries(SERIES.map((k) => [k, p[k]])),
      })),
    [series.data, locale],
  );
  const chartConfig = useMemo(
    () =>
      Object.fromEntries(
        SERIES.map((k, i) => [k, { label: t(`analytics.company.metric.${k}`), color: `var(--chart-${i + 1})` }]),
      ),
    [t],
  );

  const columns = useMemo<ColumnDef<CompanyAnalyticsEmployee, unknown>[]>(
    () => [
      createColumn<CompanyAnalyticsEmployee>({
        id: "name",
        accessorFn: (r) => r.profile_name || r.member_name,
        labelKey: "analytics.company.fields.employee",
        gridPrimary: true,
        cell: ({ row }) => {
          const r = row.original;
          const unit = [r.office_name, r.group_name].filter(Boolean).join(" · ");
          return (
            <div>
              <div className="font-medium">{r.profile_name || r.member_name || "—"}</div>
              <div className="text-muted-foreground text-xs">
                {r.member_uuid ? r.member_name || r.member_email : t("analytics.company.unlinked")}
                {unit ? ` · ${unit}` : ""}
              </div>
            </div>
          );
        },
      }),
      createColumn<CompanyAnalyticsEmployee>({
        id: "total_events",
        accessorFn: (r) => r.metrics.total_events,
        labelKey: "analytics.company.metric.total_events",
        cell: ({ row }) => <span className="tabular-nums">{fmt.format(row.original.metrics.total_events)}</span>,
      }),
      ...METRICS.map((key) =>
        createColumn<CompanyAnalyticsEmployee>({
          id: key,
          accessorFn: (r) => r.metrics[key],
          labelKey: `analytics.company.metric.${key}`,
          cell: ({ row }) => <span className="tabular-nums">{fmt.format(row.original.metrics[key])}</span>,
        }),
      ),
      createColumn<CompanyAnalyticsEmployee>({
        id: "share",
        accessorFn: (r) => r.share_percent,
        labelKey: "analytics.company.fields.share",
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex min-w-24 items-center gap-2">
            <div className="bg-muted h-1.5 flex-1 overflow-hidden rounded-full">
              <div
                className="bg-primary h-full rounded-full"
                style={{ width: `${Math.min(100, row.original.share_percent)}%` }}
              />
            </div>
            <span className="text-muted-foreground w-12 text-end text-xs tabular-nums">
              {pct.format(row.original.share_percent)}%
            </span>
          </div>
        ),
      }),
    ],
    [fmt, pct, t],
  );
  const total = employees.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / (listState.pagination.pageSize || 20)));

  return (
    <div className="space-y-6">
      <p className="text-muted-foreground text-sm">
        {isAdmin ? t("analytics.company.admin_hint") : t("analytics.company.office_manager_hint")}
      </p>

      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end">
        <label className="grid gap-1.5 text-sm sm:w-56">
          <span className="font-medium">{t("analytics.range")}</span>
          <Select value={String(effectiveDays)} onValueChange={(v) => setDays(Number(v))}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {RANGES.map((d) => (
                <SelectItem key={d} value={String(d)} disabled={d > maxDays}>
                  {t(`analytics.range.${d}`)}
                  {d > maxDays ? ` · ${t("analytics.range_locked")}` : ""}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        <label className="grid gap-1.5 text-sm sm:w-48">
          <span className="font-medium">{t("analytics.company.filters.office")}</span>
          <Select
            value={office}
            onValueChange={(v) => {
              setOffice(v);
              setGroup(ALL);
            }}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{t("analytics.company.filters.all_offices")}</SelectItem>
              {(offices.data ?? []).map((o) => (
                <SelectItem key={o.uuid} value={o.uuid}>
                  {o.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        <label className="grid gap-1.5 text-sm sm:w-48">
          <span className="font-medium">{t("analytics.company.filters.group")}</span>
          <Select value={group} onValueChange={setGroup}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{t("analytics.company.filters.all_groups")}</SelectItem>
              {groupOptions.map((g) => (
                <SelectItem key={g.uuid} value={g.uuid}>
                  {g.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        {Number.isFinite(maxDays) ? (
          <p className="text-muted-foreground pb-2 text-xs">{t("analytics.history_notice", { days: maxDays })}</p>
        ) : null}
      </div>

      <ul className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {kpis.map((k) => (
          <li key={k.key} className="bg-card rounded-xl border p-4">
            <p className="text-muted-foreground text-xs">{t(`analytics.company.metric.${k.key}`)}</p>
            <p className="mt-1 text-2xl font-semibold tabular-nums">{summary.isLoading ? "–" : k.value}</p>
          </li>
        ))}
      </ul>

      <AppChart
        type="line"
        title={t("analytics.timeline")}
        data={chartData}
        config={chartConfig}
        categoryKey="label"
        curved
        height={260}
        loading={series.isLoading}
        emptyTitle={t("analytics.no_data")}
        valueFormatter={(v) => fmt.format(v)}
      />

      <div className="grid gap-6 lg:grid-cols-2">
        <section className="bg-card rounded-xl border p-4">
          <h2 className="mb-3 text-sm font-semibold">{t("analytics.interactions")}</h2>
          {(summary.data?.top_events ?? []).length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("analytics.no_data")}</p>
          ) : (
            <BarList
              items={(summary.data?.top_events ?? []).map((e) => ({
                key: e.event_type,
                label: eventCountLabel(t, e.event_type),
                count: e.count,
              }))}
              format={(n) => fmt.format(n)}
            />
          )}
        </section>
        <section className="bg-card rounded-xl border p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-sm font-semibold">{t("analytics.breakdown")}</h2>
            <div className="flex flex-wrap gap-1" role="tablist">
              {DIMENSIONS.map((d) => (
                <button
                  key={d}
                  type="button"
                  role="tab"
                  aria-selected={d === dimension}
                  onClick={() => setDimension(d)}
                  className={cn(
                    "rounded-full px-2.5 py-1 text-xs",
                    d === dimension ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted",
                  )}
                >
                  {t(`analytics.dimension.${d}`)}
                </button>
              ))}
            </div>
          </div>
          {(breakdown.data?.items ?? []).length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("analytics.no_data")}</p>
          ) : (
            <BarList
              items={(breakdown.data?.items ?? []).map((i) => ({
                key: i.key,
                label: i.key || t("analytics.unknown"),
                count: i.count,
              }))}
              format={(n) => fmt.format(n)}
            />
          )}
        </section>
      </div>

      <section className="space-y-2">
        <div>
          <h2 className="text-sm font-semibold">{t("analytics.company.employees_title")}</h2>
          <p className="text-muted-foreground text-xs">{t("analytics.company.employees_hint")}</p>
        </div>
        <EntityTable
          columns={columns}
          data={employees.data?.items ?? []}
          getRowId={(row) => row.profile_uuid}
          isLoading={employees.isLoading}
          isError={employees.isError}
          onRetry={() => void employees.refetch()}
          emptyTitle={t("analytics.company.empty_title")}
          emptyDescription={t("analytics.company.empty_description")}
          pageCount={pageCount}
          state={listState.tableState}
          features={{
            persistKey: "app-company-analytics-v1",
            rowSelection: false,
            facetedFilters: false,
            columnFilters: false,
          }}
          toolbarExtra={
            <EntityToolbar
              onRefresh={() => void employees.refetch()}
              refreshDisabled={employees.isFetching}
            />
          }
        />
      </section>
    </div>
  );
}
