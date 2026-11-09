import type { AppLocale } from "@/config/i18n";

export function numberFormatter(locale: AppLocale) {
  return new Intl.NumberFormat(locale === "tr" ? "tr-TR" : "en-US");
}

/** Amounts arrive in minor units (kuruş); every supported currency has 2 decimals. */
export function formatMoneyMinor(amountMinor: number, currency: string, locale: AppLocale) {
  try {
    return new Intl.NumberFormat(locale === "tr" ? "tr-TR" : "en-US", {
      style: "currency",
      currency,
      maximumFractionDigits: 2,
    }).format(amountMinor / 100);
  } catch {
    return `${(amountMinor / 100).toFixed(2)} ${currency}`;
  }
}

export function formatPercent(ratio: number, locale: AppLocale) {
  return new Intl.NumberFormat(locale === "tr" ? "tr-TR" : "en-US", {
    style: "percent",
    maximumFractionDigits: 1,
  }).format(ratio);
}

/** Translated label for a key, or the fallback when the catalog has no entry. */
export function labelOr(t: (key: string) => string, key: string, fallback: string) {
  const label = t(key);
  return label === key ? fallback : label;
}
