import { z } from "zod";

import {
  LIMIT_VALUE_PATTERN,
  limitsToBody,
  limitsToForm,
} from "@/features/plans/lib/limit-fields";

type Translate = (key: string) => string;

const billingIntervals = ["free", "month", "year", "lifetime"] as const;

// Keys are encoded feature keys (see lib/limit-fields); values are raw input strings.
function limitsSchema(t: Translate) {
  return z
    .record(
      z.string(),
      z
        .string()
        .trim()
        .regex(LIMIT_VALUE_PATTERN, t("plans.validation.limit_value")),
    )
    .default({});
}

export function createPlanFormSchema(t: Translate) {
  return z.object({
    name: z.string().trim().min(1, t("plans.validation.name")),
    slug: z
      .string()
      .trim()
      .min(1, t("plans.validation.slug"))
      .regex(/^[a-z0-9_-]+$/, t("plans.validation.slug_format")),
    description: z.string().optional(),
    billing_interval: z.enum(billingIntervals),
    price_display: z.coerce.number().min(0),
    currency: z.enum(["TRY"]).default("TRY"),
    trial_days: z.coerce.number().int().min(0),
    is_active: z.boolean(),
    sort_order: z.coerce.number().int(),
    limits: limitsSchema(t),
  });
}

export type CreatePlanFormValues = z.infer<
  ReturnType<typeof createPlanFormSchema>
>;

export function updatePlanFormSchema(t: Translate) {
  return z.object({
    name: z.string().trim().min(1, t("plans.validation.name")),
    slug: z.string().trim().min(1, t("plans.validation.slug")),
    description: z.string().optional(),
    billing_interval: z.enum(billingIntervals),
    price_display: z.coerce.number().min(0),
    currency: z.enum(["TRY"]).default("TRY"),
    trial_days: z.coerce.number().int().min(0),
    is_active: z.boolean(),
    sort_order: z.coerce.number().int(),
    limits: limitsSchema(t),
  });
}

export type UpdatePlanFormValues = z.infer<
  ReturnType<typeof updatePlanFormSchema>
>;

export type PlanFormValues = CreatePlanFormValues | UpdatePlanFormValues;

/** Full limit set for the API (PATCH replaces the whole set). */
export function mapPlanLimits(values: PlanFormValues) {
  return limitsToBody(values.limits);
}

export const limitsFromPlan = limitsToForm;
