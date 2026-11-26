"use client";

import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { AppChart, BarList } from "@/components/charts";
import { Loading } from "@/components/common/loading";
import { EntityPage } from "@/components/entity";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { routes } from "@/config/routes";
import {
  type AnalyticsRange,
  type AnalyticsSubject,
  analyticsService,
} from "@/features/analytics/services/analytics.service";
import { cardsService } from "@/features/cards";
import { eventCountLabel } from "@/features/analytics/lib/event-labels";
import { useEntitlements } from "@/features/entitlements";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import {
  smartLinkKeys,
  smartLinksService,
} from "@/features/smart-links/services/smart-links.service";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { TooltipHint } from "@/components/ui/tooltip";

const RANGES = [7, 30, 90, 365] as const;
/** "All time" (TEC-1322): back to the subject's creation, capped by the plan's history. */
const ALL_TIME = "all";
type RangeChoice = (typeof RANGES)[number] | typeof ALL_TIME;
const DAY_MS = 86_400_000;
/** Above this many days the activity chart shows one bar per month instead of per day. */
const MONTHLY_CHART_AFTER_DAYS = 400;
const DIMENSIONS = ["country", "city", "device", "browser", "referer"] as const;
const CLICK_PREFIXES = ["contact.", "social.", "bank."];

function isoDay(d: Date) {
  return d.toISOString().slice(0, 10);
}

type SubjectOption = {
  value: string;
  label: string;
  /** Second line that tells same-named profiles apart: title · company (TEC-1322). */
  detail?: string;
  createdAt?: string;
  group: "profiles" | "cards" | "links";
};

/** Owner analytics (TEC-564): KPIs, daily activity and breakdowns for one profile, card or link. */
export function AnalyticsPage() {
  const { t, locale } = useLocale();
  const fmt = useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const pct = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: "percent",
        maximumFractionDigits: 0,
      }),
    [locale],
  );

  const profiles = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
  });
  const cards = useQuery({
    queryKey: ["cards", "analytics-picker"],
    queryFn: () => cardsService.list({ page: 1, pageSize: 100 }),
  });
  const links = useQuery({
    queryKey: smartLinkKeys.list,
    queryFn: smartLinksService.list,
  });

  const options = useMemo<SubjectOption[]>(() => {
    const out: SubjectOption[] = [];
    for (const p of profiles.data?.items ?? []) {
      out.push({
        value: `profile:${p.uuid}`,
        label: `${p.first_name} ${p.last_name}`.trim() || p.uuid,
        detail:
          [p.title, p.company_name].filter(Boolean).join(" · ") ||
          (p.username ? `/${p.username}` : undefined),
        createdAt: p.created_at,
        group: "profiles",
      });
    }
    for (const c of cards.data?.items ?? []) {
      if (c.route_uuid) {
        out.push({
          value: `route:${c.route_uuid}`,
          label: `${c.name} · ${c.smart_code}`,
          createdAt: c.created_at,
          group: "cards",
        });
      }
    }
    for (const l of links.data ?? []) {
      out.push({
        value: `route:${l.uuid}`,
        label: `/${l.path}`,
        createdAt: l.created_at,
        group: "links",
      });
    }
    return out;
  }, [profiles.data, cards.data, links.data]);

  const [picked, setPicked] = useState<string | null>(null);
  const subjectValue = picked ?? options[0]?.value ?? null;
  const subject: AnalyticsSubject | null = subjectValue
    ? {
        type: subjectValue.split(":")[0] as AnalyticsSubject["type"],
        id: subjectValue.split(":")[1],
      }
    : null;

  // The plan caps how far back analytics can go; longer ranges are shown but disabled.
  const history = useEntitlements().data?.features.find(
    (f) => f.key === "analytics.history_days",
  );
  const maxDays = history?.unlimited ? Infinity : (history?.int_value ?? 30);
  const [choice, setChoice] = useState<RangeChoice>(30);
  const rangeValue =
    choice === ALL_TIME ? ALL_TIME : String(Math.min(choice, maxDays));
  // All time starts on the day the subject was created (no events before it); the plan's
  // history window still caps it, as the API does.
  const subjectCreatedAt = options.find(
    (o) => o.value === subjectValue,
  )?.createdAt;
  const range: AnalyticsRange = useMemo(() => {
    const to = new Date();
    let span: number;
    if (choice === ALL_TIME) {
      const created = subjectCreatedAt ? Date.parse(subjectCreatedAt) : NaN;
      const sinceCreated = Number.isNaN(created)
        ? 365
        : Math.floor(
            (Date.parse(isoDay(to)) - Date.parse(isoDay(new Date(created)))) /
              DAY_MS,
          ) + 1;
      span = Math.max(1, Math.min(sinceCreated, maxDays));
    } else {
      span = Math.min(choice, maxDays);
    }
    const from = new Date(to.getTime() - (span - 1) * DAY_MS);
    return { from: isoDay(from), to: isoDay(to) };
  }, [choice, maxDays, subjectCreatedAt]);

  const [dimension, setDimension] =
    useState<(typeof DIMENSIONS)[number]>("country");
  const enabled = Boolean(subject);
  const key = [subject?.type, subject?.id, range.from, range.to];
  const summary = useQuery({
    queryKey: ["analytics", "summary", ...key],
    queryFn: () => analyticsService.summary(subject!, range),
    enabled,
  });
  const series = useQuery({
    queryKey: ["analytics", "timeseries", ...key],
    queryFn: () => analyticsService.timeseries(subject!, range),
    enabled,
  });
  // Views, QR scans and the entry sources also cover the routes and cards that lead to the
  // subject (a profile's cards, smart links and legacy hash routes), from the rollups (TEC-953).
  const sources = useQuery({
    queryKey: ["analytics", "sources", ...key],
    queryFn: () => analyticsService.sources(subject!, range),
    enabled,
  });
  const breakdown = useQuery({
    queryKey: ["analytics", "breakdown", dimension, ...key],
    queryFn: () => analyticsService.breakdown(subject!, range, dimension),
    enabled,
  });

  const counts = new Map(
    (summary.data?.top_events ?? []).map((e) => [
      e.event_type ?? "",
      e.count ?? 0,
    ]),
  );
  const sum = (pred: (type: string) => boolean) =>
    [...counts.entries()]
      .filter(([type]) => pred(type))
      .reduce((n, [, c]) => n + c, 0);
  // "Views" are page openings only; QR scans are their own KPI and never added to it (TEC-953).
  const kpis = [
    {
      key: "views",
      value: sources.data?.views ?? 0,
      loading: sources.isLoading,
    },
    {
      key: "qr_scans",
      value: sources.data?.qr_scans ?? 0,
      loading: sources.isLoading,
    },
    { key: "visitors", value: summary.data?.unique_sessions ?? 0 },
    { key: "saves", value: counts.get("vcard.download") ?? 0 },
    {
      key: "clicks",
      value: sum((type) => CLICK_PREFIXES.some((p) => type.startsWith(p))),
    },
    { key: "today", value: summary.data?.today_live_count ?? 0 },
  ];

  // One point per day of the range (per month for long all-time ranges): empty days are 0.
  const chartData = useMemo(() => {
    const byDay = new Map(
      (series.data?.points ?? []).map((p) => [
        p.bucket.slice(0, 10),
        p.total_count,
      ]),
    );
    const out: { label: string; events: number }[] = [];
    const spanDays =
      (Date.parse(range.to) - Date.parse(range.from)) / DAY_MS + 1;
    if (spanDays > MONTHLY_CHART_AFTER_DAYS) {
      const byMonth = new Map<string, number>();
      for (const [day, count] of byDay) {
        byMonth.set(
          day.slice(0, 7),
          (byMonth.get(day.slice(0, 7)) ?? 0) + count,
        );
      }
      for (
        let d = new Date(`${range.from.slice(0, 7)}-01T00:00:00Z`);
        isoDay(d).slice(0, 7) <= range.to.slice(0, 7);
        d = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 1))
      ) {
        out.push({
          label: d.toLocaleDateString(locale, {
            month: "short",
            year: "numeric",
            timeZone: "UTC",
          }),
          events: byMonth.get(isoDay(d).slice(0, 7)) ?? 0,
        });
      }
      return out;
    }
    for (
      let d = new Date(`${range.from}T00:00:00Z`);
      isoDay(d) <= range.to;
      d = new Date(d.getTime() + DAY_MS)
    ) {
      out.push({
        label: d.toLocaleDateString(locale, {
          day: "2-digit",
          month: "short",
          timeZone: "UTC",
        }),
        events: byDay.get(isoDay(d)) ?? 0,
      });
    }
    return out;
  }, [series.data, range, locale]);
  const loadingSubjects =
    profiles.isLoading || cards.isLoading || links.isLoading;

  return (
    <EntityPage
      title={t("analytics.title")}
      description={t("analytics.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("analytics.title") },
      ]}
    >
      {loadingSubjects ? <Loading label={t("common.loading")} /> : null}
      {!loadingSubjects && options.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          {t("analytics.no_subjects")}
        </p>
      ) : null}

      {subject ? (
        <div className="space-y-6">
          {/* Filters: one row above the charts */}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label className="grid min-w-0 gap-1.5 text-sm sm:w-96">
              <span className="font-medium">{t("analytics.subject")}</span>
              <Select
                value={subjectValue ?? undefined}
                onValueChange={setPicked}
              >
                <SelectTrigger className="[&>span]:truncate">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(["profiles", "cards", "links"] as const).map((group) => {
                    const items = options.filter((o) => o.group === group);
                    if (items.length === 0) return null;
                    return (
                      <SelectGroup key={group}>
                        <SelectLabel>
                          {t(`analytics.group.${group}`)}
                        </SelectLabel>
                        {items.map((o) => (
                          <SelectItem key={o.value} value={o.value}>
                            {o.label}
                            {o.detail ? (
                              <span className="text-muted-foreground ms-1.5 text-xs">
                                {o.detail}
                              </span>
                            ) : null}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    );
                  })}
                </SelectContent>
              </Select>
            </label>
            <label className="grid gap-1.5 text-sm sm:w-56">
              <span className="font-medium">{t("analytics.range")}</span>
              <Select
                value={rangeValue}
                onValueChange={(v) =>
                  setChoice(
                    v === ALL_TIME ? ALL_TIME : (Number(v) as RangeChoice),
                  )
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {RANGES.map((d) => (
                    <SelectItem
                      key={d}
                      value={String(d)}
                      disabled={d > maxDays}
                    >
                      {t(`analytics.range.${d}`)}
                      {d > maxDays ? ` · ${t("analytics.range_locked")}` : ""}
                    </SelectItem>
                  ))}
                  <SelectItem value={ALL_TIME}>
                    {t("analytics.range.all")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </label>
            {Number.isFinite(maxDays) ? (
              <p className="text-muted-foreground pb-2 text-xs">
                {t("analytics.history_notice", { days: maxDays })}
              </p>
            ) : null}
          </div>

          {/* KPI tiles */}
          <ul className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
            {kpis.map((k) => (
              <TooltipHint
                key={k.key}
                label={
                  k.key === "qr_scans" || k.key === "views"
                    ? t(`analytics.kpi.${k.key}_hint`)
                    : undefined
                }
              >
                <li className="bg-card rounded-xl border p-4">
                  <p className="text-muted-foreground text-xs">
                    {t(`analytics.kpi.${k.key}`)}
                  </p>
                  <p className="mt-1 text-2xl font-semibold tabular-nums">
                    {summary.isLoading || ("loading" in k && k.loading)
                      ? "–"
                      : fmt.format(k.value)}
                  </p>
                </li>
              </TooltipHint>
            ))}
          </ul>

          <AppChart
            type="area"
            title={t("analytics.timeline")}
            data={chartData}
            config={{
              events: {
                label: t("analytics.series.events"),
                color: "var(--chart-1)",
              },
            }}
            categoryKey="label"
            curved
            showLegend={false}
            height={260}
            loading={series.isLoading}
            emptyTitle={t("analytics.no_data")}
            valueFormatter={(v) => fmt.format(v)}
          />

          <div className="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
            <section className="bg-card rounded-xl border p-4">
              <h2 className="text-sm font-semibold">
                {t("analytics.sources")}
              </h2>
              <p className="text-muted-foreground mb-3 text-xs">
                {t("analytics.sources_hint")}
              </p>
              {sources.isLoading ? (
                <Loading label={t("common.loading")} />
              ) : (sources.data?.total_entries ?? 0) === 0 ? (
                <p className="text-muted-foreground text-sm">
                  {t("analytics.no_data")}
                </p>
              ) : (
                <BarList
                  items={(sources.data?.sources ?? []).map((s) => ({
                    key: s.source,
                    label: `${t(`analytics.source.${s.source}`)} · ${pct.format(s.percentage / 100)}`,
                    count: s.count,
                  }))}
                  format={(n) => fmt.format(n)}
                />
              )}
            </section>

            <section className="bg-card rounded-xl border p-4">
              <h2 className="mb-3 text-sm font-semibold">
                {t("analytics.interactions")}
              </h2>
              {counts.size === 0 ? (
                <p className="text-muted-foreground text-sm">
                  {t("analytics.no_data")}
                </p>
              ) : (
                <BarList
                  items={[...counts.entries()]
                    .sort((a, b) => b[1] - a[1])
                    .map(([type, count]) => ({
                      key: type,
                      label: eventCountLabel(t, type),
                      count,
                    }))}
                  format={(n) => fmt.format(n)}
                />
              )}
            </section>

            <section className="bg-card rounded-xl border p-4">
              <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-sm font-semibold">
                  {t("analytics.breakdown")}
                </h2>
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
                        d === dimension
                          ? "bg-primary text-primary-foreground"
                          : "text-muted-foreground hover:bg-muted",
                      )}
                    >
                      {t(`analytics.dimension.${d}`)}
                    </button>
                  ))}
                </div>
              </div>
              {(breakdown.data?.items ?? []).length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  {t("analytics.no_data")}
                </p>
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
        </div>
      ) : null}
    </EntityPage>
  );
}
