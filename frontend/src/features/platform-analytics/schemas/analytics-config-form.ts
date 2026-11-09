import { z } from "zod";

import type { AnalyticsSettings } from "../services/platform-analytics.service";

type T = (key: string, params?: Record<string, string | number>) => string;

export const DELIVERY_MODES = ["async", "sync", "hybrid"] as const;

export function createAnalyticsConfigSchema(t: T) {
  const positiveInt = (min: number) =>
    z.coerce
      .number()
      .int()
      .min(min, t("platform_analytics.config.validation.min", { min }));
  return z.object({
    enabled: z.boolean(),
    event_delivery_mode: z.enum(DELIVERY_MODES),
    raw_event_retention_days: positiveInt(1),
    aggregate_retention_days: positiveInt(1),
    queue_name: z
      .string()
      .trim()
      .min(1, t("platform_analytics.config.validation.queue_required")),
    realtime_enabled: z.boolean(),
    batch_size: positiveInt(1),
    batch_flush_interval_ms: positiveInt(1),
    // TEC-936: 1-30 days; a change applies from the next UTC day.
    ip_hash_salt_rotation_days: positiveInt(1).max(
      30,
      t("platform_analytics.config.validation.max", { max: 30 }),
    ),
    anonymize_ip: z.boolean(),
  });
}

export type AnalyticsConfigFormValues = z.infer<
  ReturnType<typeof createAnalyticsConfigSchema>
>;

/** Fills gaps with the documented defaults (docs/ANALYTICS_CONFIGURATION.md). */
export function settingsToFormValues(
  s: AnalyticsSettings,
): AnalyticsConfigFormValues {
  const mode = DELIVERY_MODES.includes(
    s.event_delivery_mode as (typeof DELIVERY_MODES)[number],
  )
    ? (s.event_delivery_mode as (typeof DELIVERY_MODES)[number])
    : "async";
  return {
    enabled: s.enabled ?? true,
    event_delivery_mode: mode,
    raw_event_retention_days: s.raw_event_retention_days || 730,
    aggregate_retention_days: s.aggregate_retention_days || 1095,
    queue_name: s.queue_name || "analytics",
    realtime_enabled: s.realtime_enabled ?? true,
    batch_size: s.batch_size || 200,
    batch_flush_interval_ms: s.batch_flush_interval_ms || 1000,
    ip_hash_salt_rotation_days: s.ip_hash_salt_rotation_days || 7,
    anonymize_ip: s.anonymize_ip ?? true,
  };
}
