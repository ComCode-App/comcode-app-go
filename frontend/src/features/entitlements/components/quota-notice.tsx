"use client";

import { Sparkles } from "lucide-react";
import Link from "next/link";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useEntitlements, useQuota } from "@/features/entitlements/hooks/use-entitlements";
import type { QuotaFeatureKey } from "@/features/entitlements/services/entitlements.service";
import { useLocale } from "@/providers/locale-provider";

/** True when the active account cannot create more of `key` (button disabling). */
export function useQuotaReached(key: QuotaFeatureKey): boolean {
  return useQuota(key)?.reached ?? false;
}

/**
 * Upgrade hint shown on list pages when the plan quota for `feature` is used up.
 * The API enforces the limit regardless (403 PLAN_LIMIT_EXCEEDED); this only explains it.
 */
export function QuotaNotice({ feature }: { feature: QuotaFeatureKey }) {
  const { t } = useLocale();
  const { data } = useEntitlements();
  const quota = data?.features.find((f) => f.key === feature);
  if (!data || !quota?.reached) return null;

  const featureLabel = t(`entitlements.features.${feature}`);
  const message = quota.included
    ? t("entitlements.banner_reached", {
        used: quota.used ?? 0,
        limit: quota.int_value ?? 0,
        feature: featureLabel.toLocaleLowerCase(),
        plan: data.plan_name,
      })
    : t("entitlements.banner_not_included", { feature: featureLabel, plan: data.plan_name });

  return (
    <Alert className="mb-4">
      <Sparkles className="size-4" />
      <AlertTitle>{t("entitlements.limit_reached_title")}</AlertTitle>
      <AlertDescription className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span>{message}</span>
        <Button asChild size="sm" className="shrink-0">
          <Link href={routes.app.billing.root}>{t("entitlements.upgrade")}</Link>
        </Button>
      </AlertDescription>
    </Alert>
  );
}
