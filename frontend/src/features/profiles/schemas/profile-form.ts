import { z } from "zod";

export const profileTranslationSchema = z.object({
  locale: z.enum(["tr", "en"]),
  prefix: z.string().optional(),
  first_name: z.string().min(1, "First name is required"),
  last_name: z.string().optional(),
  middle_name: z.string().optional(),
  suffix: z.string().optional(),
  nickname: z.string().optional(),
  company_name: z.string().optional(),
  department: z.string().optional(),
  title: z.string().optional(),
  role: z.string().optional(),
  bio: z.string().optional(),
  note: z.string().optional(),
  is_default: z.boolean().optional(),
});

export const contactDetailFormSchema = z.object({
  uuid: z.string().optional(),
  category: z.enum(["phone", "email", "url", "whatsapp", "other"]),
  type: z.string().optional(),
  label: z.string().optional(),
  value: z.string().min(1, "Value is required"),
  is_primary: z.boolean().optional(),
  display_order: z.number().int().optional(),
});

export const addressFormSchema = z.object({
  uuid: z.string().optional(),
  type: z.string().optional(),
  label: z.string().optional(),
  street: z.string().optional(),
  extended: z.string().optional(),
  city: z.string().optional(),
  region: z.string().optional(),
  postal_code: z.string().optional(),
  country: z.string().optional(),
  is_primary: z.boolean().optional(),
  display_order: z.number().int().optional(),
});

export const socialLinkFormSchema = z.object({
  uuid: z.string().optional(),
  platform_id: z.number().nullable().optional(),
  platform: z.string().optional(),
  label: z.string().optional(),
  url: z.string().min(1, "URL is required"),
  icon_url: z.string().optional(),
  display_order: z.number().int().optional(),
  is_active: z.boolean().optional(),
});

export const profileFormSchema = z.object({
  primary_locale: z.enum(["tr", "en"]),
  birthday: z.string().nullable().optional(),
  gender: z.string().optional(),
  avatar_url: z.string().optional(),
  cover_url: z.string().optional(),
  is_primary: z.boolean().optional(),
  translations: z
    .array(profileTranslationSchema)
    .min(1, "At least one language translation is required"),
  contact_details: z.array(contactDetailFormSchema).optional(),
  addresses: z.array(addressFormSchema).optional(),
  social_links: z.array(socialLinkFormSchema).optional(),
});

export type ProfileFormValues = z.infer<typeof profileFormSchema>;
export type ProfileTranslationFormValues = z.infer<
  typeof profileTranslationSchema
>;
export type ContactDetailFormValues = z.infer<typeof contactDetailFormSchema>;
export type AddressFormValues = z.infer<typeof addressFormSchema>;
export type SocialLinkFormValues = z.infer<typeof socialLinkFormSchema>;

export interface ProfilePreviewData {
  primary_locale?: "tr" | "en" | string;
  birthday?: string | null;
  gender?: string;
  avatar_url?: string | null;
  cover_url?: string | null;
  is_primary?: boolean;
  status?: string;
  translations?: Partial<ProfileTranslationFormValues>[];
  contact_details?: Partial<ContactDetailFormValues>[];
  addresses?: Partial<AddressFormValues>[];
  social_links?: Partial<SocialLinkFormValues>[];
}

export function calculateLiveCompletion(
  values?: ProfilePreviewData | null,
): number {
  if (!values) return 0;
  let score = 0;

  const translations = values.translations || [];
  const primaryLocale = values.primary_locale || "tr";
  const primaryTrans =
    translations.find((t) => t?.locale === primaryLocale) || translations[0];

  // Identity (30%)
  if (primaryTrans?.first_name?.trim()) score += 15;
  if (primaryTrans?.last_name?.trim()) score += 10;
  if (values.avatar_url?.trim()) score += 10;

  // Professional (20%)
  if (primaryTrans?.company_name?.trim()) score += 10;
  if (primaryTrans?.title?.trim()) score += 10;

  // Contact (25%)
  let hasPhone = false;
  let hasEmail = false;
  if (values.contact_details) {
    for (const cd of values.contact_details) {
      if (cd?.category === "phone" && cd?.value?.trim()) hasPhone = true;
      if (cd?.category === "email" && cd?.value?.trim()) hasEmail = true;
    }
  }
  if (hasPhone) score += 15;
  if (hasEmail) score += 10;

  // Bio / Notes (10%)
  if (primaryTrans?.bio?.trim()) score += 10;

  // Address & Social (15%)
  if (
    values.addresses &&
    values.addresses.some((a) => a?.city?.trim() || a?.street?.trim())
  ) {
    score += 5;
  }
  if (
    values.social_links &&
    values.social_links.some((s) => s?.url?.trim())
  ) {
    score += 10;
  }

  return Math.min(100, score);
}
