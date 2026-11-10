import { z } from "zod";

export const PAPER_SIZES = ["A4", "A3", "Letter", "Legal"] as const;

export type PaperSize = (typeof PAPER_SIZES)[number];

type Translate = (key: string) => string;

/** Same rule as the API (`appversion.Valid`): "" or major[.minor[.patch]]. */
export const MOBILE_VERSION_PATTERN = /^(\d{1,6}(\.\d{1,6}){0,2})?$/;

function compareVersions(a: string, b: string): number {
  const pa = a.split(".").map(Number);
  const pb = b.split(".").map(Number);
  for (let i = 0; i < 3; i += 1) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (diff !== 0) return diff;
  }
  return 0;
}

function isStoreUrl(value: string): boolean {
  if (value === "") return true;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && url.host !== "" && !url.username;
  } catch {
    return false;
  }
}

export function createSettingsFormSchema(t: Translate) {
  const version = z
    .string()
    .trim()
    .regex(MOBILE_VERSION_PATTERN, t("settings.mobile.validation.version"));
  const storeUrl = z
    .string()
    .trim()
    .max(512)
    .refine(isStoreUrl, t("settings.mobile.validation.store_url"));
  return z
    .object({
      company_name: z.string().trim().min(1),
      tagline: z.string().trim(),
      primary_color: z
        .string()
        .trim()
        .regex(/^#[0-9A-Fa-f]{6}$/, "Invalid color"),
      paper_size: z.enum(PAPER_SIZES),
      address: z.string().trim(),
      phone: z.string().trim(),
      email: z.string().trim(),
      website: z.string().trim(),
      footer_text: z.string().trim(),
      mobile_min_version: version,
      mobile_latest_version: version,
      mobile_ios_store_url: storeUrl,
      mobile_android_store_url: storeUrl,
    })
    .refine(
      (v) =>
        !v.mobile_min_version ||
        !v.mobile_latest_version ||
        compareVersions(v.mobile_latest_version, v.mobile_min_version) >= 0,
      {
        path: ["mobile_latest_version"],
        message: t("settings.mobile.validation.latest_below_min"),
      },
    );
}

export type SettingsFormValues = z.infer<
  ReturnType<typeof createSettingsFormSchema>
>;

export function normalizeHexColor(value: string, fallback: string): string {
  const trimmed = value.trim();
  if (/^#[0-9A-Fa-f]{6}$/.test(trimmed)) return trimmed;
  if (/^[0-9A-Fa-f]{6}$/.test(trimmed)) return `#${trimmed}`;
  return fallback;
}

export function toSettingsFormValues(
  data: {
    company_name: string;
    tagline: string;
    primary_color: string;
    paper_size: string;
    address: string;
    phone: string;
    email: string;
    website: string;
    footer_text: string;
    mobile_min_version?: string;
    mobile_latest_version?: string;
    mobile_ios_store_url?: string;
    mobile_android_store_url?: string;
  },
  fallbackColor: string,
): SettingsFormValues {
  const paperSize = PAPER_SIZES.includes(data.paper_size as PaperSize)
    ? (data.paper_size as PaperSize)
    : "A4";

  return {
    company_name: data.company_name,
    tagline: data.tagline,
    primary_color: normalizeHexColor(data.primary_color, fallbackColor),
    paper_size: paperSize,
    address: data.address,
    phone: data.phone,
    email: data.email,
    website: data.website,
    footer_text: data.footer_text,
    mobile_min_version: data.mobile_min_version ?? "",
    mobile_latest_version: data.mobile_latest_version ?? "",
    mobile_ios_store_url: data.mobile_ios_store_url ?? "",
    mobile_android_store_url: data.mobile_android_store_url ?? "",
  };
}

export function toPatchPayload(values: SettingsFormValues) {
  return {
    company_name: values.company_name,
    tagline: values.tagline,
    primary_color: values.primary_color,
    paper_size: values.paper_size,
    address: values.address,
    phone: values.phone,
    email: values.email,
    website: values.website,
    footer_text: values.footer_text,
    mobile_min_version: values.mobile_min_version,
    mobile_latest_version: values.mobile_latest_version,
    mobile_ios_store_url: values.mobile_ios_store_url,
    mobile_android_store_url: values.mobile_android_store_url,
  };
}
