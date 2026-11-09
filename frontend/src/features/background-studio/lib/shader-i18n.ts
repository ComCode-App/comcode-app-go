import type { AppLocale } from "@/config/i18n";

import en from "../data/shaders-en.json";
import tr from "../data/shaders-tr.json";

type ShaderDict = typeof en;

const dictionaries: Record<"en" | "tr", ShaderDict> = { en, tr };

function dictFor(locale: AppLocale): ShaderDict {
  return locale === "tr" ? dictionaries.tr : dictionaries.en;
}

export function getShaderLabel(shaderKey: string, locale: AppLocale): string {
  const d = dictFor(locale);
  return (
    (d.shaders as Record<string, { name?: string }>)[shaderKey]?.name ??
    shaderKey
  );
}

export function getShaderDescription(
  shaderKey: string,
  locale: AppLocale,
): string {
  const d = dictFor(locale);
  return (
    (d.shaders as Record<string, { description?: string }>)[shaderKey]
      ?.description ?? ""
  );
}

export function getParamLabel(
  shaderKey: string,
  paramKey: string,
  locale: AppLocale,
): string {
  const d = dictFor(locale);
  const shaderParams = (
    d.shaders as Record<string, { params?: Record<string, string> }>
  )[shaderKey]?.params;
  return (
    shaderParams?.[paramKey] ??
    (d.commonParams as Record<string, string>)[paramKey] ??
    paramKey
  );
}

export function getCategoryLabel(
  categoryKey: string,
  locale: AppLocale,
): string {
  const d = dictFor(locale);
  return (d.categories as Record<string, string>)[categoryKey] ?? categoryKey;
}
