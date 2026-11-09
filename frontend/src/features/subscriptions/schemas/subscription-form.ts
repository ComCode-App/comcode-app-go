import { z } from "zod";

type Translate = (key: string) => string;

export function createSubscriptionFormSchema(t: Translate) {
  return z
    .object({
      plan_uuid: z.string().trim().min(1, t("subscriptions.validation.plan")),
      subject_type: z.enum(["user", "company"]),
      user_uuid: z.string().optional(),
      company_uuid: z.string().optional(),
      status: z.enum(["active", "cancelled", "expired", "trialing"]),
    })
    .superRefine((values, ctx) => {
      if (values.subject_type === "user" && !values.user_uuid?.trim()) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("subscriptions.validation.user"),
          path: ["user_uuid"],
        });
      }
      if (values.subject_type === "company" && !values.company_uuid?.trim()) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("subscriptions.validation.company"),
          path: ["company_uuid"],
        });
      }
    });
}

export type CreateSubscriptionFormValues = z.infer<
  ReturnType<typeof createSubscriptionFormSchema>
>;
