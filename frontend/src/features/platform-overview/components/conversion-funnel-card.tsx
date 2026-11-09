"use client";

import { useState } from "react";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { AppLocale } from "@/config/i18n";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { OverviewSection } from "@/features/platform-overview/components/overview-section";
import {
  DEFAULT_FUNNEL_PERIOD,
  FUNNEL_PERIODS,
} from "@/features/platform-overview/constants";
import { useConversionFunnel } from "@/features/platform-overview/hooks/use-overview-query";
import {
  formatPercent,
  numberFormatter,
} from "@/features/platform-overview/lib/format";
import type { FunnelPeriod } from "@/features/platform-overview/services/overview.service";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

/** Hours under two days stay in hours; longer spans read better in days. */
function formatDuration(
  hours: number,
  t: (key: string, params?: Record<string, string | number>) => string,
  locale: AppLocale,
) {
  const fmt = new Intl.NumberFormat(locale === "tr" ? "tr-TR" : "en-US", {
    maximumFractionDigits: 1,
  });
  return hours < 48
    ? t("dashboard.conversion.hours", { value: fmt.format(hours) })
    : t("dashboard.conversion.days", { value: fmt.format(hours / 24) });
}

/** TEC-944: stage-to-stage conversion of the orders opened in a period, from order_status_history. */
export function ConversionFunnelCard() {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const [days, setDays] = useState<FunnelPeriod>(DEFAULT_FUNNEL_PERIOD);
  const query = useConversionFunnel(days);
  const { data, error } = query;
  const fmt = numberFormatter(locale);
  const created = data?.stages[0]?.orders ?? 0;

  return (
    <OverviewSection
      title={t("dashboard.conversion.title")}
      description={t("dashboard.conversion.description")}
      toolbar={
        <Select
          value={String(days)}
          onValueChange={(v) => setDays(Number(v) as FunnelPeriod)}
        >
          <SelectTrigger
            className="h-8 w-36 text-xs"
            aria-label={t("dashboard.conversion.period_label")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FUNNEL_PERIODS.map((d) => (
              <SelectItem key={d} value={String(d)}>
                {t("dashboard.conversion.period", { days: d })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      }
      action={
        can(permissions.orders.read)
          ? { href: routes.platform.orders, label: t("dashboard.funnel.open") }
          : undefined
      }
      loading={query.isLoading}
    >
      {error ? (
        <p className="text-destructive text-sm">
          {isApiError(error) ? error.message : t("dashboard.conversion.error")}
        </p>
      ) : !data || created === 0 ? (
        <p className="text-muted-foreground text-sm">
          {t("dashboard.conversion.empty")}
        </p>
      ) : (
        <div className="space-y-4">
          <ol className="space-y-3">
            {data.stages.map((stage, i) => (
              <li key={stage.stage} className="space-y-1">
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="font-medium">
                    {t(`dashboard.conversion.stage_${stage.stage}`)}
                  </span>
                  <span className="tabular-nums">
                    {fmt.format(stage.orders)}
                    <span className="text-muted-foreground ml-2 text-xs">
                      {formatPercent(stage.conversion_from_created, locale)}
                    </span>
                  </span>
                </div>
                <div className="bg-muted h-2 overflow-hidden rounded-full">
                  <div
                    className="bg-primary h-full rounded-full"
                    style={{
                      width: `${Math.round(stage.conversion_from_created * 100)}%`,
                    }}
                  />
                </div>
                {i > 0 ? (
                  <p className="text-muted-foreground text-xs">
                    {t("dashboard.conversion.from_previous", {
                      rate: formatPercent(
                        stage.conversion_from_previous,
                        locale,
                      ),
                    })}
                    {" · "}
                    {stage.avg_hours_from_created == null
                      ? t("dashboard.conversion.no_timing")
                      : t("dashboard.conversion.avg_time", {
                          time: formatDuration(
                            stage.avg_hours_from_created,
                            t,
                            locale,
                          ),
                        })}
                  </p>
                ) : null}
              </li>
            ))}
          </ol>
          {data.cancelled > 0 ? (
            <p className="text-muted-foreground text-xs">
              {t("dashboard.conversion.cancelled", {
                count: fmt.format(data.cancelled),
              })}
            </p>
          ) : null}
          <p className="text-muted-foreground text-xs">
            {t("dashboard.conversion.note")}
          </p>
        </div>
      )}
    </OverviewSection>
  );
}
