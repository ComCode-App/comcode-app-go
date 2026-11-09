import { z } from "zod";

export const socialPlatformFormSchema = z.object({
  name: z.string().min(1, "Name is required"),
  slug: z.string().optional(),
  category: z.string().min(1, "Category is required"),
  url_pattern: z.string().min(1, "URL pattern is required"),
  placeholder: z.string().optional(),
  description: z.string().optional(),
  display_order: z.number().int().min(0),
  is_active: z.boolean(),
});

export type SocialPlatformFormValues = z.infer<typeof socialPlatformFormSchema>;
