import type { AppLocale } from "@/config/i18n";

import enAuth from "@/locales/en/auth.json";
import enCommon from "@/locales/en/common.json";
import enForm from "@/locales/en/form.json";
import enEntitlements from "@/locales/en/entitlements.json";
import enPublicProfile from "@/locales/en/public_profile.json";
import enLeads from "@/locales/en/leads.json";
import enCards from "@/locales/en/cards.json";
import enErrors from "@/locales/en/errors.json";
import trAuth from "@/locales/tr/auth.json";
import trCommon from "@/locales/tr/common.json";
import trForm from "@/locales/tr/form.json";
import trEntitlements from "@/locales/tr/entitlements.json";
import trPublicProfile from "@/locales/tr/public_profile.json";
import trLeads from "@/locales/tr/leads.json";
import trCards from "@/locales/tr/cards.json";
import trErrors from "@/locales/tr/errors.json";

export type MessageDictionary = Record<string, string>;
export type MessageCatalogs = Record<
  AppLocale,
  Record<string, MessageDictionary>
>;

/**
 * Namespaces every route needs, including visitor pages (`/{username}`, `/c/*`, `/s/*`):
 * public profile, leads, card resolver and the global toasts. Panel namespaces live in
 * `app-catalogs.ts` and are registered by the (shell) providers, so visitor pages do not download
 * them (TEC-858). New namespaces normally go there; add one here only if a visitor page needs it.
 */
const catalogs: MessageCatalogs = {
  tr: {
    common: trCommon,
    form: trForm,
    auth: trAuth,
    cards: trCards,
    entitlements: trEntitlements,
    public_profile: trPublicProfile,
    leads: trLeads,
    errors: trErrors,
  },
  en: {
    common: enCommon,
    form: enForm,
    auth: enAuth,
    cards: enCards,
    entitlements: enEntitlements,
    public_profile: enPublicProfile,
    leads: enLeads,
    errors: enErrors,
  },
};

/** Merges more namespaces into the catalog (see `app-catalogs.ts`). */
export function registerCatalogs(extra: MessageCatalogs) {
  for (const locale of Object.keys(extra) as AppLocale[]) {
    catalogs[locale] = { ...catalogs[locale], ...extra[locale] };
  }
}

export function loadMessages(locale: AppLocale) {
  return catalogs[locale];
}

export function translate(
  locale: AppLocale,
  key: string,
  params?: Record<string, string | number>,
  fallbackLocale: AppLocale = "en",
): string {
  const [ns, ...rest] = key.split(".");
  const path = rest.join(".");
  const primary = catalogs[locale]?.[ns]?.[path];
  const fallback = catalogs[fallbackLocale]?.[ns]?.[path];
  let text = primary ?? fallback ?? key;

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replace(new RegExp(`{{\\s*${k}\\s*}}`, "g"), String(v));
    }
  }

  return text;
}

/** Simple plural helper: key_one / key_other */
export function translatePlural(
  locale: AppLocale,
  key: string,
  count: number,
  params?: Record<string, string | number>,
) {
  const suffix = count === 1 ? "one" : "other";
  return translate(locale, `${key}_${suffix}`, { count, ...params });
}
