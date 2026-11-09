"use client";

import { useMemo, useState } from "react";

import { PricingCard } from "@/features/billing/components/pricing-card";
import { PricingHeader } from "@/features/billing/components/pricing-header";
import { planFeatureList } from "@/features/billing/lib/format-plan-features";
import type {
  BillingPlan,
  SavedCard,
  UserSubscription,
} from "@/features/billing/services/billing.service";
import { formatPlanPrice } from "@/features/plans/lib/format-price";
import { useLocale } from "@/providers/locale-provider";

const BILLING_FREQUENCIES = ["month", "year"] as const;

type BillingPricingProps = {
  plans: BillingPlan[];
  subscription: UserSubscription | null;
  defaultCard?: SavedCard;
  isSubmitting?: boolean;
  onSelectPlan: (planUuid: string) => void;
};

function resolvePlanBadges(plans: BillingPlan[]) {
  const sorted = [...plans].sort((a, b) => a.price_minor - b.price_minor);
  const highlightedUuid = sorted.at(-1)?.uuid;
  const popularUuid =
    plans.find((plan) => plan.trial_days > 0)?.uuid ??
    sorted[Math.floor((sorted.length - 1) / 2)]?.uuid;

  return { highlightedUuid, popularUuid };
}

export function BillingPricing({
  plans,
  subscription,
  defaultCard,
  isSubmitting = false,
  onSelectPlan,
}: BillingPricingProps) {
  const { t } = useLocale();

  const availableFrequencies = useMemo(
    () =>
      BILLING_FREQUENCIES.filter((interval) =>
        plans.some((plan) => plan.billing_interval === interval),
      ),
    [plans],
  );

  const [selectedFrequency, setSelectedFrequency] = useState(() =>
    subscription?.billing_interval &&
    availableFrequencies.includes(
      subscription.billing_interval as (typeof BILLING_FREQUENCIES)[number],
    )
      ? subscription.billing_interval
      : (availableFrequencies[0] ?? "month"),
  );

  const visiblePlans = useMemo(() => {
    if (availableFrequencies.length <= 1) {
      return [...plans].sort((a, b) => a.price_minor - b.price_minor);
    }

    return plans
      .filter(
        (plan) =>
          plan.billing_interval === selectedFrequency ||
          plan.billing_interval === "lifetime",
      )
      .sort((a, b) => a.price_minor - b.price_minor);
  }, [availableFrequencies.length, plans, selectedFrequency]);

  const { highlightedUuid, popularUuid } = useMemo(
    () => resolvePlanBadges(visiblePlans),
    [visiblePlans],
  );

  const featureLabels = useMemo(
    () => ({
      enabled: t("plans.labels.enabled"),
      disabled: t("plans.labels.disabled"),
    }),
    [t],
  );

  if (visiblePlans.length === 0) {
    return null;
  }

  return (
    <section className="flex flex-col items-center gap-10">
      <PricingHeader
        title={t("billing.pricing.title")}
        subtitle={t("billing.pricing.subtitle")}
        frequencies={availableFrequencies.map((interval) => ({
          id: interval,
          label: t(`plans.billing.${interval}`),
          saveBadge:
            interval === "year" ? t("billing.pricing.save_badge") : undefined,
        }))}
        selectedFrequency={selectedFrequency}
        onFrequencyChange={setSelectedFrequency}
      />

      <div className="grid w-full gap-6 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {visiblePlans.map((plan) => {
          const isCurrent = subscription?.plan_uuid === plan.uuid;
          const isPopular = plan.uuid === popularUuid;
          const isHighlighted = plan.uuid === highlightedUuid;
          const features = planFeatureList(plan, featureLabels);
          const trialSuffix =
            plan.trial_days > 0
              ? ` · ${t("billing.plans.trial_days", { days: plan.trial_days })}`
              : "";

          return (
            <PricingCard
              key={plan.uuid}
              name={plan.name}
              priceLabel={formatPlanPrice(plan.price_minor, plan.currency)}
              intervalLabel={`${t(`plans.billing.${plan.billing_interval}`)}${trialSuffix}`}
              description={plan.description}
              features={features}
              isCurrent={isCurrent}
              isPopular={isPopular}
              isHighlighted={isHighlighted}
              isLoading={isSubmitting}
              disabled={isSubmitting}
              currentBadgeLabel={t("billing.pricing.current_plan")}
              popularBadgeLabel={t("billing.pricing.most_popular")}
              ctaLabel={
                isCurrent
                  ? t("billing.pricing.current_plan")
                  : defaultCard
                    ? t("billing.plans.subscribe_with_card")
                    : t("billing.plans.checkout")
              }
              onSelect={() => onSelectPlan(plan.uuid)}
            />
          );
        })}
      </div>
    </section>
  );
}
