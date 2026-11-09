export function formatPlanPrice(
  priceMinor: number,
  currency: string,
  locale = "tr-TR",
) {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: currency || "TRY",
  }).format(priceMinor / 100);
}

export function priceMinorFromDisplay(display: number) {
  return Math.round(display * 100);
}

export function priceDisplayFromMinor(minor: number) {
  return minor / 100;
}
