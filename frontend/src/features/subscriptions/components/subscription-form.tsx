"use client";

import { CreditCard } from "lucide-react";
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
import { PlanAsyncPicker } from "@/features/plans/components/plan-async-picker";
import { CompanyAsyncPicker } from "@/features/companies/components/company-async-picker";
import {
  createSubscriptionFormSchema,
  type CreateSubscriptionFormValues,
} from "@/features/subscriptions/schemas/subscription-form";
import { UserAsyncPicker } from "@/features/users/components/user-async-picker";
import { useLocale } from "@/providers/locale-provider";

type SubscriptionFormProps = {
  isSubmitting?: boolean;
  onSubmit: (values: CreateSubscriptionFormValues) => Promise<void> | void;
  onCancel: () => void;
};

function SubscriptionFormFields({
  form,
  isSubmitting,
  onCancel,
}: {
  form: UseFormReturn<CreateSubscriptionFormValues>;
  isSubmitting?: boolean;
  onCancel: () => void;
}) {
  const { t } = useLocale();
  const subjectType = form.watch("subject_type");

  const subjectTypeOptions = useMemo(
    () => [
      { value: "user", label: t("subscriptions.subject_type.user") },
      {
        value: "company",
        label: t("subscriptions.subject_type.company"),
      },
    ],
    [t],
  );

  const statusOptions = useMemo(
    () =>
      (["active", "trialing", "cancelled", "expired"] as const).map(
        (value) => ({
          value,
          label: t(`subscriptions.status.${value}`),
        }),
      ),
    [t],
  );

  const sections = useMemo(
    () =>
      createFormSections([
        {
          key: "assignment",
          label: t("subscriptions.form.section_assignment"),
          description: t("subscriptions.form.section_assignment_desc"),
          icon: CreditCard,
        },
      ]),
    [t],
  );

  return (
    <FormLayout navItems={sections.navItems}>
      <FormSection
        id={sections.id("assignment")}
        title={t("subscriptions.form.section_assignment")}
        description={t("subscriptions.form.section_assignment_desc")}
        columns={2}
      >
        <PlanAsyncPicker
          name="plan_uuid"
          label={t("subscriptions.fields.plan")}
          className="sm:col-span-2"
        />
        <AppSelect
          name="subject_type"
          label={t("subscriptions.fields.subject_type")}
          options={subjectTypeOptions}
          className="sm:col-span-2"
        />
        {subjectType === "user" ? (
          <UserAsyncPicker
            name="user_uuid"
            label={t("subscriptions.fields.user")}
            className="sm:col-span-2"
          />
        ) : (
          <CompanyAsyncPicker
            name="company_uuid"
            label={t("subscriptions.fields.company")}
            className="sm:col-span-2"
          />
        )}
        <AppSelect
          name="status"
          label={t("subscriptions.fields.status")}
          options={statusOptions}
          className="sm:col-span-2"
        />
      </FormSection>

      <FormActions>
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("form.cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting || form.formState.isSubmitting}>
          {t("subscriptions.actions.create")}
        </Button>
      </FormActions>
    </FormLayout>
  );
}

export function SubscriptionForm({
  isSubmitting,
  onSubmit,
  onCancel,
}: SubscriptionFormProps) {
  const { t } = useLocale();
  const schema = createSubscriptionFormSchema(t);

  const defaultValues: CreateSubscriptionFormValues = {
    plan_uuid: "",
    subject_type: "user",
    user_uuid: "",
    company_uuid: "",
    status: "active",
  };

  return (
    <AppForm schema={schema} defaultValues={defaultValues} onSubmit={onSubmit}>
      {(form) => (
        <SubscriptionFormFields
          form={form}
          isSubmitting={isSubmitting}
          onCancel={onCancel}
        />
      )}
    </AppForm>
  );
}
