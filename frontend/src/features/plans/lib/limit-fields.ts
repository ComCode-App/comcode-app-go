// Plan limit form helpers (TEC-861). Kept free of imports so `node --test` can load it.
//
// Feature keys contain dots (`profiles.max`), which react-hook-form reads as nested paths.
// The form therefore stores limits under an encoded key (`profiles:max`) and decodes it when
// building the API body. API errors use `limits.<feature_key>`; `limitErrorField` maps them
// onto the form field.

/** Matches an allowed raw limit value: empty (not set), -1, a whole number, or a bool. */
export const LIMIT_VALUE_PATTERN = /^(|-1|\d+|true|false)$/;

export function encodeLimitKey(featureKey: string): string {
  return featureKey.replaceAll(".", ":");
}

export function decodeLimitKey(formKey: string): string {
  return formKey.replaceAll(":", ".");
}

/** Form field name of a feature's limit input. */
export function limitFieldName(featureKey: string): string {
  return `limits.${encodeLimitKey(featureKey)}`;
}

/** Maps an API error field to the form field; other fields are returned unchanged. */
export function limitErrorField(apiField: string): string {
  if (!apiField.startsWith("limits.")) return apiField;
  return limitFieldName(apiField.slice("limits.".length));
}

export type PlanLimitBody = {
  feature_key: string;
  int_value?: number;
  bool_value?: boolean;
};

/**
 * Builds the full `limits` array for create/PATCH (the API replaces the whole set).
 * Empty values are left out; the API then answers a `required` error on that feature's field.
 */
export function limitsToBody(
  limits: Record<string, string>,
  valueTypes: Record<string, string> = {},
): PlanLimitBody[] {
  return Object.entries(limits)
    .map(([formKey, raw]) => [decodeLimitKey(formKey), raw.trim()] as const)
    .filter(([, raw]) => raw !== "")
    .map(([feature_key, raw]) => {
      const type =
        valueTypes[feature_key] ??
        (raw === "true" || raw === "false" ? "bool" : "int");
      if (type === "bool") return { feature_key, bool_value: raw === "true" };
      return { feature_key, int_value: Number(raw) };
    });
}

/** Form record of a plan's limits, keyed by encoded feature key. */
export function limitsToForm(
  limits: Array<{
    feature_key: string;
    int_value?: number | null;
    bool_value?: boolean | null;
  }>,
): Record<string, string> {
  const record: Record<string, string> = {};
  for (const lim of limits) {
    const key = encodeLimitKey(lim.feature_key);
    if (lim.int_value != null) record[key] = String(lim.int_value);
    if (lim.bool_value != null) record[key] = lim.bool_value ? "true" : "false";
  }
  return record;
}
