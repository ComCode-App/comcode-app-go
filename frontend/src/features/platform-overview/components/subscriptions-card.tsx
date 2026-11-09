"use client";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { OverviewSection } from "@/features/platform-overview/components/overview-section";
import {
  formatMoneyMinor,
  numberFormatter,
} from "@/features/platform-overview/lib/format";
import type { PlatformOverview } from "@/features/platform-overview/services/overview.service";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

/** Subscription status counts and MRR per currency. */
export function SubscriptionsCard({
  data,
  loading,
}: {
  data?: PlatformOverview;
  loading?: boolean;
}) {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const fmt = numberFormatter(locale);
  const subs = data?.subscriptions;

  const counts = subs
    ? [
        {
          key: "active",
          labelKey: "dashboard.subscriptions.active",
          value: subs.active,
        },
        {
          key: "trialing",
          labelKey: "dashboard.subscriptions.trialing",
          value: subs.trialing,
        },
        {
          key: "past_due",
          labelKey: "dashboard.subscriptions.past_due",
          value: subs.past_due,
        },
        {
          key: "canceled_30d",
          labelKey: "dashboard.subscriptions.canceled_30d",
          value: subs.canceled_last_30d,
        },
      ]
    : [];

  return (
    <OverviewSection
      title={t("dashboard.subscriptions.title")}
      description={t("dashboard.subscriptions.description")}
      action={
        can(permissions.subscriptions.read)
          ? {
              href: routes.platform.settings.subscriptions,
              label: t("dashboard.subscriptions.open"),
            }
          : undefined
      }
      loading={loading}
    >
      {subs ? (
        <div className="space-y-5">
          <dl className="grid grid-cols-2 gap-3">
            {counts.map((c) => (
              <div key={c.key} className="bg-muted/40 rounded-md px-3 py-2">
                <dt className="text-muted-foreground text-xs">
                  {t(c.labelKey)}
                </dt>
                <dd className="text-lg font-semibold tabular-nums">
                  {fmt.format(c.value)}
                </dd>
              </div>
            ))}
          </dl>
          <div>
            <p className="text-muted-foreground text-xs">
              {t("dashboard.subscriptions.mrr")}
            </p>
            {subs.mrr.length === 0 ? (
              <p className="text-muted-foreground mt-1 text-sm">
                {t("dashboard.subscriptions.no_mrr")}
              </p>
            ) : (
              <ul className="mt-1 space-y-1">
                {subs.mrr.map((m) => (
                  <li key={m.currency}>
                    <p className="text-2xl font-semibold tracking-tight tabular-nums">
                      {formatMoneyMinor(m.amount_minor, m.currency, locale)}
                    </p>
                    {m.manual_amount_minor > 0 ? (
                      <p className="text-muted-foreground text-xs">
                        {t("dashboard.subscriptions.mrr_manual", {
                          amount: formatMoneyMinor(
                            m.manual_amount_minor,
                            m.currency,
                            locale,
                          ),
                        })}
                      </p>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
            {subs.at_risk_mrr.length > 0 ? (
              <p className="mt-2 text-xs text-amber-600 dark:text-amber-400">
                {t("dashboard.subscriptions.at_risk", {
                  amount: subs.at_risk_mrr
                    .map((m) =>
                      formatMoneyMinor(m.amount_minor, m.currency, locale),
                    )
                    .join(" · "),
                })}
              </p>
            ) : null}
            <p className="text-muted-foreground mt-2 text-xs">
              {t("dashboard.subscriptions.mrr_note")}
            </p>
          </div>
        </div>
      ) : null}
    </OverviewSection>
  );
}
