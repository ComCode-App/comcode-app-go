import { ApiError } from "@/lib/api/errors";

import type { ProductFormValues } from "../schemas/product-form";

/** A product field error from the API (TEC-1299), shaped for react-hook-form's setError. */
export type ProductServerFieldError = {
  field: keyof ProductFormValues;
  /** RHF error type; NumberFieldError picks its message from it. */
  type: string;
  /** i18n key for fields without a NumberFieldError. */
  message: string;
  tab: "general" | "pricing" | "seo";
};

const FIELD_TABS: Partial<
  Record<keyof ProductFormValues, ProductServerFieldError["tab"]>
> = {
  name: "general",
  slug: "general",
  status: "general",
  short_description: "general",
  description: "general",
  category_uuid: "general",
  base_price: "pricing",
  tax_rate: "pricing",
  stock_quantity: "pricing",
  currency: "pricing",
  product_type: "pricing",
  sku: "pricing",
  meta_title: "seo",
  meta_description: "seo",
  meta_keywords: "seo",
};

/** RHF types matching the zod issues the form raises for the same rule. */
function errorType(field: string, code: string, values: ProductFormValues) {
  switch (code) {
    case "negative":
      return "too_small";
    case "out_of_range":
      return field === "tax_rate" && values.tax_rate > 100
        ? "too_big"
        : "too_small";
    case "too_many_decimals":
      return "custom";
    default:
      return "server";
  }
}

/**
 * The field errors of a 400 VALIDATION_ERROR from POST/PUT /v1/platform/products, or [] when
 * the error is something else. Unknown fields are skipped.
 */
export function productServerFieldErrors(
  error: unknown,
  values: ProductFormValues,
): ProductServerFieldError[] {
  if (!(error instanceof ApiError) || error.code !== "VALIDATION_ERROR") {
    return [];
  }
  const out: ProductServerFieldError[] = [];
  for (const detail of error.details) {
    const field = detail.field as keyof ProductFormValues | undefined;
    if (!field || !(field in FIELD_TABS)) continue;
    const code = detail.code ?? "invalid";
    out.push({
      field,
      type: errorType(field, code, values),
      message: `products.errors.${code}`,
      tab: FIELD_TABS[field] ?? "general",
    });
  }
  return out;
}
