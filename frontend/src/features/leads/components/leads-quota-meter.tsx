"use client";

import { Lock } from "lucide-react";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { routes } from "@/config/routes";
import { type EntitlementFeature, useQuota } from "@/features/entitlements";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/** Formats a quota reset date for the active locale. */
export function formatResetDate(
  iso: string | undefined,
  locale: string,
): string {
  return iso
    ? new Date(iso).toLocaleDateString(locale === "tr" ? "tr-TR" : "en-GB")
    : "";
}

/**
 * This month's lead and contact exchange quotas of the active account (TEC-941): "Bu ay 18/20
 * lead". Past the quota new records are kept locked; the hint explains it and offers an upgrade.
 */
export function LeadsQuotaMeter() {
  const { t, locale } = useLocale();
  const leads = useQuota("leads.monthly_max");
  const exchanges = useQuota("exchange.monthly_max");
  if (!leads || !exchanges) return null;

  const reached = [leads, exchanges].some((f) => f.reached && !isOff(f));
  const resets = formatResetDate(
    leads.resets_at ?? exchanges.resets_at,
    locale,
  );

  return (
    <div className="bg-card space-y-3 rounded-xl border p-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <Meter feature={leads} kind="leads" />
        <Meter feature={exchanges} kind="exchanges" />
      </div>
      {reached ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-muted-foreground flex items-start gap-2 text-xs">
            <Lock className="mt-0.5 size-3.5 shrink-0" />
            <span>
              {t("leads.quota.reached")}{" "}
              {resets ? t("leads.quota.resets", { date: resets }) : null}
            </span>
          </p>
          <Button asChild size="sm" className="shrink-0">
            <Link href={routes.app.billing.root}>
              {t("leads.quota.upgrade")}
            </Link>
          </Button>
        </div>
      ) : resets ? (
        <p className="text-muted-foreground text-xs">
          {t("leads.quota.resets", { date: resets })}
        </p>
      ) : null}
    </div>
  );
}

function isOff(f: EntitlementFeature): boolean {
  return !f.unlimited && (!f.included || (f.int_value ?? 0) <= 0);
}

function Meter({
  feature,
  kind,
}: {
  feature: EntitlementFeature;
  kind: "leads" | "exchanges";
}) {
  const { t } = useLocale();
  const used = feature.used ?? 0;
  if (isOff(feature)) {
    return (
      <p className="text-muted-foreground text-sm">
        {t(`leads.quota.${kind}_off`)}
      </p>
    );
  }
  if (feature.unlimited) {
    return (
      <p className="text-sm">{t(`leads.quota.${kind}_unlimited`, { used })}</p>
    );
  }
  const limit = feature.int_value ?? 0;
  const percent = Math.min(100, Math.round((used / limit) * 100));
  const label = t(`leads.quota.${kind}`, { used, limit });
  return (
    <div className="space-y-1.5">
      <p className="text-sm tabular-nums">{label}</p>
      <Progress
        value={percent}
        aria-label={label}
        className={cn(
          "h-2",
          percent >= 100
            ? "[&>[data-slot=progress-indicator]]:bg-destructive"
            : percent >= 80 &&
                "[&>[data-slot=progress-indicator]]:bg-amber-500",
        )}
      />
    </div>
  );
}
