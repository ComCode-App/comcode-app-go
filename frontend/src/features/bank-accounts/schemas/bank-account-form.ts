import { z } from "zod";

export const bankAccountFormSchema = z.object({
  iban: z.string().min(5, "IBAN is required"),
  account_holder: z.string().min(2, "Account holder name is required"),
  title: z.string().optional(),
  currency: z.string().default("TRY"),
  bank_uuid: z.string().optional(),
  account_number: z.string().optional(),
  branch_code: z.string().optional(),
  is_primary: z.boolean().default(false),
  is_active: z.boolean().default(true),
});

export type BankAccountFormValues = {
  iban: string;
  account_holder: string;
  title?: string;
  currency?: string;
  bank_uuid?: string;
  account_number?: string;
  branch_code?: string;
  is_primary?: boolean;
  is_active?: boolean;
};
