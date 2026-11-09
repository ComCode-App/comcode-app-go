"use client";

import { useMemo } from "react";
import type { UseFormReturn } from "react-hook-form";

import {
  AppForm,
  AppSelect,
  FormActions,
  FormLayout,
  FormSection,
  createFormSections,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import { UserAsyncPicker } from "@/features/users/components/user-async-picker";
import {
  addCompanyMemberSchema,
  type AddCompanyMemberFormValues,
} from "@/features/companies/schemas/company-form";
import { useLocale } from "@/providers/locale-provider";

type CompanyAddMemberFormProps = {
  isSubmitting?: boolean;
  onSubmit: (values: AddCompanyMemberFormValues) => Promise<void> | void;
};

function CompanyAddMemberFormFields({
  form,
  isSubmitting,
}: {
  form: UseFormReturn<AddCompanyMemberFormValues>;
  isSubmitting?: boolean;
}) {
  const { t } = useLocale();

  const roleOptions = useMemo(
    () => [
      { value: "member", label: t("companies.roles.member") },
      { value: "admin", label: t("companies.roles.admin") },
      { value: "owner", label: t("companies.roles.owner") },
    ],
    [t],
  );

  const sections = useMemo(
    () =>
      createFormSections([
        {
          key: "member",
          label: t("companies.add_member"),
          description: t("companies.add_member_desc"),
        },
      ]),
    [t],
  );

  return (
    <FormLayout navItems={sections.navItems}>
      <FormSection
        id={sections.id("member")}
        title={t("companies.add_member")}
        description={t("companies.add_member_desc")}
        columns={2}
      >
        <UserAsyncPicker
          name="user_uuid"
          label={t("companies.fields.user")}
          className="sm:col-span-2"
        />
        <AppSelect
          name="role"
          label={t("companies.fields.role")}
          options={roleOptions}
          className="sm:col-span-2"
        />
      </FormSection>

      <FormActions>
        <Button type="submit" disabled={isSubmitting || form.formState.isSubmitting}>
          {t("companies.add_member")}
        </Button>
      </FormActions>
    </FormLayout>
  );
}

export function CompanyAddMemberForm({
  isSubmitting,
  onSubmit,
}: CompanyAddMemberFormProps) {
  const { t } = useLocale();
  const schema = addCompanyMemberSchema(t);

  const defaultValues: AddCompanyMemberFormValues = {
    user_uuid: "",
    role: "member",
  };

  return (
    <AppForm schema={schema} defaultValues={defaultValues} onSubmit={onSubmit}>
      {(form) => (
        <CompanyAddMemberFormFields
          form={form}
          isSubmitting={isSubmitting}
        />
      )}
    </AppForm>
  );
}
