import { z } from "zod";

export const bankFormSchema = z.object({
  name: z.string().min(1, "Name is required"),
  slug: z.string().optional(),
  code: z.string().min(1, "Bank code is required"),
  country_code: z.string().optional(),
  bic_swift: z.string().optional(),
  icon_url: z.string().optional(),
  brand_color: z.string().optional(),
  iban_pattern: z.string().optional(),
  description: z.string().optional(),
  display_order: z.number().int().min(0),
  is_active: z.boolean(),
});

export type BankFormValues = z.infer<typeof bankFormSchema>;
