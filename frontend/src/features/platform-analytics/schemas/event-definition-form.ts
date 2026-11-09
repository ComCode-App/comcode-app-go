import { z } from "zod";

import type {
  EventDefinition,
  EventDefinitionInput,
} from "../services/platform-analytics.service";

type T = (key: string, params?: Record<string, string | number>) => string;

/** Same shape the ingestion registry accepts: dot-separated lowercase keys. */
export const EVENT_KEY_PATTERN = /^[a-z0-9._-]+$/;

/** Categories already used by the seeded registry; free text is still allowed. */
export const KNOWN_EVENT_CATEGORIES = ["card", "contact", "social", "auth", "billing", "general"];

export function createEventDefinitionSchema(t: T) {
  return z.object({
    key: z
      .string()
      .trim()
      .min(1, t("platform_analytics.definitions.validation.key_required"))
      .max(100, t("platform_analytics.definitions.validation.key_max"))
      .regex(EVENT_KEY_PATTERN, t("platform_analytics.definitions.validation.key_pattern")),
    name: z
      .string()
      .trim()
      .min(1, t("platform_analytics.definitions.validation.name_required"))
      .max(200, t("platform_analytics.definitions.validation.name_max")),
    description: z.string(),
    category: z
      .string()
      .trim()
      .min(1, t("platform_analytics.definitions.validation.category_required"))
      .max(64, t("platform_analytics.definitions.validation.category_max")),
    schema_version: z.coerce
      .number()
      .int()
      .min(1, t("platform_analytics.definitions.validation.schema_version_min")),
    retention_days: z.coerce
      .number()
      .int()
      .min(1, t("platform_analytics.definitions.validation.retention_min")),
    required_properties: z.string(),
    enabled: z.boolean(),
    is_public: z.boolean(),
    aggregate_enabled: z.boolean(),
  });
}

export type EventDefinitionFormValues = z.infer<ReturnType<typeof createEventDefinitionSchema>>;

export const emptyEventDefinitionValues: EventDefinitionFormValues = {
  key: "",
  name: "",
  description: "",
  category: "general",
  schema_version: 1,
  retention_days: 365,
  required_properties: "",
  enabled: true,
  is_public: true,
  aggregate_enabled: true,
};

export function definitionToFormValues(def: EventDefinition): EventDefinitionFormValues {
  return {
    key: def.key,
    name: def.name,
    description: def.description ?? "",
    category: def.category,
    schema_version: def.schema_version ?? 1,
    retention_days: def.retention_days ?? 365,
    required_properties: (def.required_properties ?? []).join(", "),
    enabled: def.enabled,
    is_public: def.is_public,
    aggregate_enabled: def.aggregate_enabled,
  };
}

/** Comma / whitespace separated property keys → unique, trimmed list. */
export function parseRequiredProperties(raw: string): string[] {
  return [...new Set(raw.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean))];
}

export function formValuesToInput(values: EventDefinitionFormValues): EventDefinitionInput {
  return {
    key: values.key.trim(),
    name: values.name.trim(),
    description: values.description.trim(),
    category: values.category.trim(),
    schema_version: values.schema_version,
    retention_days: values.retention_days,
    // Always an array: an omitted list would be stored as NULL on create.
    required_properties: parseRequiredProperties(values.required_properties),
    enabled: values.enabled,
    is_public: values.is_public,
    aggregate_enabled: values.aggregate_enabled,
  };
}
