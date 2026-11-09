import { z } from "zod";

export const productFormSchema = z.object({
  name: z.string().min(1, "Ürün adı zorunludur").max(200),
  slug: z.string().max(150).optional(),
  category_uuid: z.string().nullable().optional(),
  short_description: z.string().max(300).optional(),
  description: z.string().optional(),
  status: z.enum(["active", "draft", "archived"]),
  product_type: z.string(),
  base_price: z.number().min(0, "Fiyat 0 veya daha büyük olmalıdır"),
  currency: z.string(),
  tax_rate: z.number().min(0).max(100),
  stock_quantity: z.number().int().min(0),
  sku: z.string().max(64).optional(),
  allow_custom_print: z.boolean(),
  model_3d_file_url: z.string().optional(),
  model_3d_usdz_url: z.string().optional(),
  images: z.array(z.string()),
  meta_title: z.string().max(255).optional(),
  meta_description: z.string().optional(),
  meta_keywords: z.string().max(255).optional(),
  canonical_url: z.string().optional(),
  og_image_url: z.string().optional(),
  display_order: z.number().int(),
});

export type ProductFormValues = z.infer<typeof productFormSchema>;

export const variantFormSchema = z.object({
  name: z.string().min(1, "Varyant adı zorunludur").max(150),
  sku: z.string().max(64).optional(),
  price_delta: z.number(),
  stock_quantity: z.number().int().min(0),
  material: z.string(),
  color_code: z.string(),
  image_url: z.string().optional(),
  is_active: z.boolean(),
  display_order: z.number().int(),
});

export type VariantFormValues = z.infer<typeof variantFormSchema>;
