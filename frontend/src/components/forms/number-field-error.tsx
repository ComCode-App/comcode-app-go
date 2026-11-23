"use client";

import type { FieldError } from "react-hook-form";

import { useLocale } from "@/providers/locale-provider";

type NumberFieldErrorProps = {
  id?: string;
  error?: FieldError;
  min?: number;
  max?: number;
  /** Whole numbers only (zod `.int()`). */
  integer?: boolean;
  /** Maximum decimal places (zod refinement, e.g. prices). */
  decimals?: number;
};

/**
 * Localized message for a `type="number"` field validated by zod. Forms using it set
 * `noValidate`, so this replaces the browser's step/min/max bubble (TEC-1292).
 */
export function NumberFieldError({
  id,
  error,
  min,
  max,
  integer,
  decimals,
}: NumberFieldErrorProps) {
  const { t } = useLocale();
  if (!error) return null;
  let message: string;
  if (error.type === "too_small" && min !== undefined) {
    message = t("form.number_min", { min });
  } else if (error.type === "too_big" && max !== undefined) {
    message = t("form.number_max", { max });
  } else if (error.type === "custom" && decimals !== undefined) {
    message = t("form.number_decimals", { decimals });
  } else if (integer) {
    message = t("form.number_integer");
  } else {
    message = t("form.number_invalid");
  }
  return (
    <p id={id} role="alert" className="text-destructive text-xs">
      {message}
    </p>
  );
}
