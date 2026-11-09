"use client";

import { Building2 } from "lucide-react";
import { useMemo } from "react";
import type { UseFormReturn } from "react-hook-form";

import {
  AppForm,
  AppInput,
  FormActions,
  FormLayout,
  FormSection,
  createFormSections,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import { UserAsyncPicker } from "@/features/users/components/user-async-picker";
import {
  createCompanyFormSchema,
  type CreateCompanyFormValues,
} from "@/features/companies/schemas/company-form";
import { useLocale } from "@/providers/locale-provider";

type CompanyFormProps = {
  isSubmitting?: boolean;
  onSubmit: (values: CreateCompanyFormValues) => Promise<void> | void;
  onCancel: () => void;
};

function CompanyFormFields({
  form,
  isSubmitting,
  onCancel,
}: {
  form: UseFormReturn<CreateCompanyFormValues>;
  isSubmitting?: boolean;
  onCancel: () => void;
}) {
  const { t } = useLocale();

  const sections = useMemo(
    () =>
      createFormSections([
        {
          key: "details",
          label: t("companies.form.section_details"),
          description: t("companies.form.section_details_desc"),
          icon: Building2,
        },
      ]),
    [t],
  );

  return (
    <FormLayout navItems={sections.navItems}>
      <FormSection
        id={sections.id("details")}
        title={t("companies.form.section_details")}
        description={t("companies.form.section_details_desc")}
        columns={2}
      >
        <AppInput name="name" label={t("companies.fields.name")} />
        <AppInput
          name="slug"
          label={t("companies.fields.slug")}
          description={t("companies.fields.slug_hint")}
        />
        <UserAsyncPicker
          name="owner_user_uuid"
          label={t("companies.fields.owner")}
          description={t("companies.fields.owner_hint")}
          className="sm:col-span-2"
        />
      </FormSection>

      <FormActions>
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("form.cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting || form.formState.isSubmitting}>
          {t("companies.actions.create")}
        </Button>
      </FormActions>
    </FormLayout>
  );
}

export function CompanyForm({
  isSubmitting,
  onSubmit,
  onCancel,
}: CompanyFormProps) {
  const { t } = useLocale();
  const schema = createCompanyFormSchema(t);

  const defaultValues: CreateCompanyFormValues = {
    name: "",
    slug: "",
    owner_user_uuid: "",
  };

  return (
    <AppForm schema={schema} defaultValues={defaultValues} onSubmit={onSubmit}>
      {(form) => (
        <CompanyFormFields
          form={form}
          isSubmitting={isSubmitting}
          onCancel={onCancel}
        />
      )}
    </AppForm>
  );
}
