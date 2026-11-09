import { z } from "zod";

export const geoipFormSchema = z.object({
  enabled: z.boolean(),
  provider: z.string().min(1, "Provider is required"),
  database_type: z.string().min(1, "Database type is required"),
  database_path: z.string().min(1, "Database path is required"),
  auto_update_enabled: z.boolean(),
  update_interval_hours: z.coerce.number().min(1, "Interval must be at least 1 hour"),
  account_id: z.string().optional().default(""),
  license_key: z.string().optional().default(""),
  download_url_template: z.string().optional().default(""),
  cache_enabled: z.boolean(),
  cache_ttl: z.coerce.number().min(1, "Cache TTL must be at least 1 hour"),
  cache_max_entries: z.coerce.number().min(100, "Max entries must be at least 100"),
});

export type GeoIPFormValues = z.infer<typeof geoipFormSchema>;
