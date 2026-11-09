"use client";

import { Lock } from "lucide-react";
import Link from "next/link";

import { routes } from "@/config/routes";
import { useEntitlements } from "@/features/entitlements/hooks/use-entitlements";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/**
 * Whether the active account may use premium themes and animated backgrounds (`design.premium`,
 * TEC-947). True while loading, so pickers do not flash locks; the API enforces it regardless
 * (403 PLAN_LIMIT_EXCEEDED).
 */
export function useDesignPremium(): boolean {
  const { data } = useEntitlements();
  const feature = data?.features.find((f) => f.key === "design.premium");
  return data ? feature?.bool_value === true : true;
}

/** Lock icon for a premium item the plan does not include. */
export function PremiumLockIcon({ className }: { className?: string }) {
  const { t } = useLocale();
  return (
    <Lock
      aria-label={t("entitlements.premium")}
      className={cn("text-muted-foreground size-3.5 shrink-0", className)}
    />
  );
}

/** "Yükselt" link to the billing page, shown next to locked premium items. */
export function UpgradeLink({ className }: { className?: string }) {
  const { t } = useLocale();
  return (
    <Link
      href={routes.app.billing.root}
      className={cn(
        "text-primary text-xs font-medium hover:underline",
        className,
      )}
    >
      {t("entitlements.upgrade")}
    </Link>
  );
}

/** One-line notice: premium visuals need a paid plan, with the upgrade link. */
export function PremiumNotice({ className }: { className?: string }) {
  const { t } = useLocale();
  return (
    <p
      className={cn(
        "text-muted-foreground flex flex-wrap items-center gap-1.5 text-xs",
        className,
      )}
    >
      <PremiumLockIcon />
      <span>{t("entitlements.premium_locked")}</span>
      <UpgradeLink />
    </p>
  );
}
