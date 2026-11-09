import { z } from "zod";

/** Select sentinel for a company-wide group (office_uuid = null). */
export const COMPANY_WIDE = "__company_wide__";

export const groupFormSchema = z.object({
  name: z.string().trim().min(1, "required").max(150),
  kind: z.enum(["department", "team"]),
  office: z.string().min(1),
});

export type GroupFormValues = z.infer<typeof groupFormSchema>;
