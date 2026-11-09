"use client";

import { useMemo } from "react";

import { AppChart, BarList } from "@/components/charts";
import { eventCountLabel } from "@/features/analytics/lib/event-labels";
import { OverviewSection } from "@/features/platform-overview/components/overview-section";
import { numberFormatter } from "@/features/platform-overview/lib/format";
import type { PlatformOverview } from "@/features/platform-overview/services/overview.service";
import { useLocale } from "@/providers/locale-provider";

/** Hourly event volume of the last 24 hours and the busiest event types. */
export function EventVolumeCard({
  data,
  loading,
}: {
  data?: PlatformOverview;
  loading?: boolean;
}) {
  const { t, locale } = useLocale();
  const fmt = numberFormatter(locale);
  const events = data?.events_last_24h;

  const chartData = useMemo(
    () =>
      (events?.hourly ?? []).map((h) => ({
        label: new Date(h.bucket).toLocaleTimeString(locale, {
          hour: "2-digit",
          minute: "2-digit",
        }),
        events: h.count,
      })),
    [events, locale],
  );

  return (
    <OverviewSection
      title={t("dashboard.events.title")}
      description={t("dashboard.events.description")}
      loading={loading}
    >
      {!events || events.total === 0 ? (
        <p className="text-muted-foreground text-sm">
          {t("dashboard.events.empty")}
        </p>
      ) : (
        <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
          <AppChart
            type="bar"
            data={chartData}
            config={{
              events: {
                label: t("dashboard.events.series"),
                color: "var(--chart-1)",
              },
            }}
            categoryKey="label"
            showLegend={false}
            height={220}
            valueFormatter={(v) => fmt.format(v)}
          />
          <div>
            <h3 className="mb-3 text-xs font-medium">
              {t("dashboard.events.by_type")}
            </h3>
            <BarList
              items={events.by_type.map((e) => ({
                key: e.event_type,
                label: eventCountLabel(t, e.event_type),
                count: e.count,
              }))}
              format={(v) => fmt.format(v)}
            />
          </div>
        </div>
      )}
    </OverviewSection>
  );
}
