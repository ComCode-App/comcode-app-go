import { z } from "zod";

export const officeFormSchema = z.object({
  name: z.string().trim().min(1, "required").max(150),
  city: z.string().trim().max(150),
  address: z.string().trim().max(10000),
  phone: z.string().trim().max(50),
  sort_order: z.number().int().min(0).max(100000),
});

export type OfficeFormValues = z.infer<typeof officeFormSchema>;
