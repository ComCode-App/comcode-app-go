import { z } from "zod";

/** Mirrors the backend: lowercase kebab slug ≤ 80, names ≤ 150. */
export const sectorFormSchema = z.object({
  slug: z
    .string()
    .trim()
    .min(1)
    .max(80)
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/),
  name_tr: z.string().trim().min(1).max(150),
  name_en: z.string().trim().min(1).max(150),
});

export type SectorFormValues = z.infer<typeof sectorFormSchema>;
