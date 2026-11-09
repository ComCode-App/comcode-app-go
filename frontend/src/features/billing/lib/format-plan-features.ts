import type { BillingPlan } from "@/features/billing/services/billing.service";

function humanizeFeatureKey(key: string) {
  return key
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function formatPlanLimitFeature(
  limit: BillingPlan["limits"][number],
  labels: {
    enabled: string;
    disabled: string;
  },
) {
  const label = humanizeFeatureKey(limit.feature_key);

  if (limit.value_type === "bool") {
    return limit.bool_value
      ? `${label}: ${labels.enabled}`
      : `${label}: ${labels.disabled}`;
  }

  if (limit.int_value != null) {
    return `${label}: ${limit.int_value.toLocaleString()}`;
  }

  return label;
}

export function planFeatureList(
  plan: BillingPlan,
  labels: {
    enabled: string;
    disabled: string;
  },
) {
  if (plan.limits.length > 0) {
    return plan.limits.map((limit) => formatPlanLimitFeature(limit, labels));
  }

  if (plan.description?.trim()) {
    return [plan.description.trim()];
  }

  return [];
}
