import { z } from "zod";

export const cardFormSchema = z.object({
  name: z.string().min(1, "Kart adı zorunludur").max(150),
  card_uid: z.string().optional(),
  target_type: z.enum(["profile", "direct_url", "vcard_download", "social_hub"]),
  profile_uuid: z.string().nullable().optional(),
  direct_url: z.string().optional(),
  card_type: z.enum(["physical_nfc", "metal_card", "wood_card", "pvc_card", "digital_only"]),
  material: z.string(),
  color_hex: z.string(),
  is_active: z.boolean(),
  is_locked: z.boolean(),
  lead_capture_enabled: z.boolean(),
  passcode: z.string().optional(),
  custom_data: z.record(z.string(), z.unknown()).optional(),
});

export type CardFormValues = z.infer<typeof cardFormSchema>;

export const cardActivationSchema = z.object({
  activation_code: z.string().min(3, "Aktivasyon kodu gereklidir"),
  name: z.string().optional(),
  profile_uuid: z.string().nullable().optional(),
});

export type CardActivationFormValues = z.infer<typeof cardActivationSchema>;
