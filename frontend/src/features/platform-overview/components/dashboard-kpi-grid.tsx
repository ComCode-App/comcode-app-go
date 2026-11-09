"use client";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { CLOSED_ORDER_STATUSES } from "@/features/platform-overview/constants";
import { DashboardStatCard } from "@/features/platform-overview/components/dashboard-stat-card";
import {
  formatMoneyMinor,
  formatPercent,
  numberFormatter,
} from "@/features/platform-overview/lib/format";
import type { PlatformOverview } from "@/features/platform-overview/services/overview.service";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

type DashboardKpiGridProps = {
  data?: PlatformOverview;
  loading?: boolean;
};

/** Headline tiles: users, profiles, card activation, MRR, at-risk MRR, 30-day orders, 24-hour events. */
export function DashboardKpiGrid({ data, loading }: DashboardKpiGridProps) {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const fmt = numberFormatter(locale);
  const n = (v: number | undefined) => (v === undefined ? "—" : fmt.format(v));

  const mrr = data?.subscriptions.mrr ?? [];
  const mrrValue = mrr.length
    ? mrr
        .map((m) => formatMoneyMinor(m.amount_minor, m.currency, locale))
        .join(" · ")
    : formatMoneyMinor(0, "TRY", locale);
  const paying = mrr.reduce((sum, m) => sum + m.subscriptions, 0);
  const atRisk = data?.subscriptions.at_risk_mrr ?? [];
  const atRiskValue = atRisk.length
    ? atRisk
        .map((m) => formatMoneyMinor(m.amount_minor, m.currency, locale))
        .join(" · ")
    : formatMoneyMinor(0, "TRY", locale);
  const openOrders = (data?.orders.funnel ?? [])
    .filter((s) => !CLOSED_ORDER_STATUSES.includes(s.status))
    .reduce((sum, s) => sum + s.count, 0);

  const cards = [
    {
      key: "users",
      label: t("dashboard.kpi.users"),
      value: n(data?.users.total),
      hint: data
        ? t("dashboard.kpi.users_hint", {
            active: fmt.format(data.users.active_last_30d),
            new: fmt.format(data.users.new_last_30d),
          })
        : undefined,
      href: can(permissions.users.read)
        ? routes.platform.users.root
        : undefined,
    },
    {
      key: "profiles",
      label: t("dashboard.kpi.profiles"),
      value: n(data?.profiles.total),
      hint: data
        ? t("dashboard.kpi.profiles_hint", {
            published: fmt.format(data.profiles.published),
          })
        : undefined,
    },
    {
      key: "activation",
      label: t("dashboard.kpi.activation"),
      value: data ? formatPercent(data.cards.activation_rate, locale) : "—",
      hint: data
        ? t("dashboard.kpi.activation_hint", {
            activated: fmt.format(data.cards.activated),
            total: fmt.format(data.cards.total),
          })
        : undefined,
    },
    {
      key: "mrr",
      label: t("dashboard.kpi.mrr"),
      value: data ? mrrValue : "—",
      hint: data
        ? t("dashboard.kpi.mrr_hint", { count: fmt.format(paying) })
        : undefined,
      href: can(permissions.subscriptions.read)
        ? routes.platform.settings.subscriptions
        : undefined,
    },
    {
      // TEC-944: past_due subscriptions are kept out of MRR and shown on their own.
      key: "at_risk_mrr",
      label: t("dashboard.kpi.at_risk_mrr"),
      value: data ? atRiskValue : "—",
      hint: data
        ? t("dashboard.kpi.at_risk_mrr_hint", {
            count: fmt.format(data.subscriptions.past_due),
          })
        : undefined,
      href: can(permissions.subscriptions.read)
        ? routes.platform.settings.subscriptions
        : undefined,
    },
    {
      key: "orders",
      label: t("dashboard.kpi.orders"),
      value: n(data?.orders.total),
      hint: data
        ? t("dashboard.kpi.orders_hint", { active: fmt.format(openOrders) })
        : undefined,
      href: can(permissions.orders.read) ? routes.platform.orders : undefined,
    },
    {
      key: "events",
      label: t("dashboard.kpi.events"),
      value: n(data?.events_last_24h.total),
      hint: t("dashboard.kpi.events_hint"),
    },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-7">
      {cards.map((card) => (
        <DashboardStatCard
          key={card.key}
          label={card.label}
          value={card.value}
          hint={card.hint}
          href={card.href}
          loading={loading}
        />
      ))}
    </div>
  );
}
