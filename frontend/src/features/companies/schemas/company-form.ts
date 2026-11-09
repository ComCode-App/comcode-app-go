import { z } from "zod";

type Translate = (key: string) => string;

export function createCompanyFormSchema(t: Translate) {
  return z.object({
    name: z.string().trim().min(1, t("companies.validation.name")),
    slug: z
      .string()
      .trim()
      .regex(/^[a-z0-9_-]*$/, t("companies.validation.slug_format")),
    owner_user_uuid: z.string().trim().min(1, t("companies.validation.owner")),
  });
}

export type CreateCompanyFormValues = z.infer<
  ReturnType<typeof createCompanyFormSchema>
>;

export function addCompanyMemberSchema(t: Translate) {
  return z.object({
    user_uuid: z.string().trim().min(1, t("companies.validation.user")),
    role: z.enum(["owner", "admin", "member"]).default("member"),
  });
}

export type AddCompanyMemberFormValues = z.infer<
  ReturnType<typeof addCompanyMemberSchema>
>;
