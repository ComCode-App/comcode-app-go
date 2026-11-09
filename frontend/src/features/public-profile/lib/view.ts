import type { PublicProfile } from "@/features/public-profile/lib/load";

export type PublicTranslation = PublicProfile["translations"][number];
export type PublicContact = PublicProfile["contacts"][number];
export type PublicAddress = PublicProfile["addresses"][number];

/** Translation for `lang` (?lang=), else the primary locale, else the default one. */
export function pickTranslation(
  profile: PublicProfile,
  lang?: string | null,
): PublicTranslation | undefined {
  const list = profile.translations;
  return (
    list.find((t) => t.locale === lang) ??
    list.find((t) => t.locale === profile.primary_locale) ??
    list.find((t) => t.is_default) ??
    list[0]
  );
}

export function displayName(t?: PublicTranslation): string {
  if (!t) return "";
  return [t.prefix, t.first_name, t.middle_name, t.last_name, t.suffix]
    .filter(Boolean)
    .join(" ")
    .trim();
}

/** "Title · Company" line under the name. */
export function headline(t?: PublicTranslation): string {
  if (!t) return "";
  const role = t.title || t.role;
  return [role, t.company_name].filter(Boolean).join(" · ");
}

const digits = (v: string) => v.replace(/[^\d+]/g, "");

/** Link for a contact entry, or null for plain text entries. */
export function contactHref(c: PublicContact): string | null {
  const value = c.value.trim();
  if (!value) return null;
  switch (c.category) {
    case "phone":
      return `tel:${digits(value)}`;
    case "email":
      return `mailto:${value}`;
    case "whatsapp":
      return `https://wa.me/${digits(value).replace(/^\+/, "")}`;
    case "url":
      return /^https?:\/\//i.test(value) ? value : `https://${value}`;
    default:
      return null;
  }
}

/** Analytics event type for a contact click (POST /v1/events allow-list). */
export function contactEvent(c: PublicContact): string | null {
  switch (c.category) {
    case "phone":
      return "contact.phone.click";
    case "email":
      return "contact.email.click";
    case "whatsapp":
      return "contact.whatsapp.click";
    case "url":
      return "contact.website.click";
    default:
      return null;
  }
}

export function addressLines(a: PublicAddress): string[] {
  const cityLine = [a.postal_code, a.city, a.region].filter(Boolean).join(" ");
  return [a.street, a.extended, cityLine, a.country].filter((l): l is string =>
    Boolean(l && l.trim()),
  );
}

export function mapsHref(a: PublicAddress): string {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(addressLines(a).join(", "))}`;
}

/** "TR33 0006 1005 ..." grouping for readability; copy uses the raw value. */
export function formatIban(iban: string): string {
  return iban
    .replace(/\s+/g, "")
    .replace(/(.{4})/g, "$1 ")
    .trim();
}
