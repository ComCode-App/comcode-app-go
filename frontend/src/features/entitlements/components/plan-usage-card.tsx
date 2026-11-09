"use client";

import Link from "next/link";

import { EntitySectionCard } from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { routes } from "@/config/routes";
import { useEntitlements } from "@/features/entitlements/hooks/use-entitlements";
import type { EntitlementFeature } from "@/features/entitlements/services/entitlements.service";
import { useLocale } from "@/providers/locale-provider";

/** Plan of the active account with each feature's value and quota usage. */
export function PlanUsageCard() {
  const { t } = useLocale();
  const { data } = useEntitlements();
  if (!data) return null;

  const anyReached = data.features.some((f) => f.reached);

  return (
    <EntitySectionCard title={t("entitlements.title")}>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="text-muted-foreground text-sm">
            {t("entitlements.plan")}
          </span>
          <span className="font-medium">{data.plan_name}</span>
          <Badge variant="outline">
            {data.source === "free"
              ? t("entitlements.source_free")
              : t("entitlements.source_subscription")}
          </Badge>
        </div>
        {data.source === "free" || anyReached ? (
          <Button asChild size="sm">
            <Link href={routes.app.billing.root}>
              {t("entitlements.upgrade")}
            </Link>
          </Button>
        ) : null}
      </div>
      <ul className="divide-y rounded-md border">
        {data.features.map((feature) => (
          <li
            key={feature.key}
            className="flex flex-col gap-2 p-3 sm:flex-row sm:items-center"
          >
            <span className="flex-1 text-sm font-medium">
              {t(`entitlements.features.${feature.key}`)}
            </span>
            <FeatureValue feature={feature} />
          </li>
        ))}
      </ul>
      <p className="text-muted-foreground mt-2 text-xs">
        {t("entitlements.subtitle")}
      </p>
    </EntitySectionCard>
  );
}

function FeatureValue({ feature }: { feature: EntitlementFeature }) {
  const { t, locale } = useLocale();

  if (
    feature.monthly &&
    feature.included &&
    !feature.unlimited &&
    (feature.int_value ?? 0) <= 0
  ) {
    return (
      <span className="text-muted-foreground text-sm">
        {t("entitlements.off")}
      </span>
    );
  }
  if (!feature.included) {
    return (
      <span className="text-muted-foreground text-sm">
        {t("entitlements.not_included")}
      </span>
    );
  }
  if (feature.value_type === "bool") {
    return (
      <span className="text-sm">
        {feature.bool_value
          ? t("entitlements.included")
          : t("entitlements.not_included")}
      </span>
    );
  }
  if (feature.used !== undefined) {
    if (feature.unlimited) {
      return (
        <span className="text-sm">
          {feature.used} · {t("entitlements.unlimited")}
        </span>
      );
    }
    const limit = feature.int_value ?? 0;
    const percent =
      limit > 0 ? Math.min(100, Math.round((feature.used / limit) * 100)) : 100;
    return (
      <div className="flex w-full items-center gap-3 sm:w-64">
        <Progress
          value={percent}
          className="h-2"
          aria-label={t(`entitlements.features.${feature.key}`)}
        />
        <span className="shrink-0 text-sm tabular-nums">
          {t("entitlements.usage", { used: feature.used, limit })}
        </span>
        {feature.reached ? (
          <Badge variant="secondary" className="shrink-0 whitespace-nowrap">
            {t("entitlements.reached")}
          </Badge>
        ) : null}
        {feature.monthly && feature.resets_at ? (
          <span className="text-muted-foreground hidden shrink-0 text-xs sm:inline">
            {t("entitlements.per_month", {
              date: new Date(feature.resets_at).toLocaleDateString(
                locale === "tr" ? "tr-TR" : "en-GB",
              ),
            })}
          </span>
        ) : null}
      </div>
    );
  }
  return (
    <span className="text-sm">
      {feature.unlimited
        ? t("entitlements.unlimited")
        : t("entitlements.days", { count: feature.int_value ?? 0 })}
    </span>
  );
}
