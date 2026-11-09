import { z } from "zod";

export const categoryFormSchema = z.object({
  name: z.string().min(1, "Kategori adı zorunludur").max(150),
  slug: z.string().max(120).optional(),
  description: z.string().max(500).optional(),
  icon: z.string().max(64).optional(),
  image_url: z.string().optional(),
  display_order: z.number().int(),
  is_active: z.boolean(),
});

export type CategoryFormValues = z.infer<typeof categoryFormSchema>;
