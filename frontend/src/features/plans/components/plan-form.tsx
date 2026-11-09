"use client";

import { CreditCard } from "lucide-react";
import { useEffect, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import type { UseFormReturn } from "react-hook-form";

import { Loading } from "@/components/common/loading";
import {
  AppForm,
  AppInput,
  AppSelect,
  AppSwitch,
  AppTextarea,
  FormActions,
  FormLayout,
  FormSection,
  createFormSections,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import { limitErrorField, limitFieldName } from "@/features/plans/lib/limit-fields";
import { plansKeys } from "@/features/plans/hooks/query-keys";
import {
  createPlanFormSchema,
  limitsFromPlan,
  updatePlanFormSchema,
  type PlanFormValues,
} from "@/features/plans/schemas/plan-form";
import {
  priceDisplayFromMinor,
} from "@/features/plans/lib/format-price";
import {
  plansService,
  type PlanDetail,
} from "@/features/plans/services/plans.service";
import { ApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";

const limitErrorCodes = new Set([
  "required",
  "unknown_feature",
  "duplicate",
  "type_mismatch",
  "out_of_range",
]);

/**
 * Re-targets API limit errors (`limits.<feature_key>`) onto the encoded form fields and
 * localizes them by `code`, so AppForm shows each one under its input.
 */
function withLimitFieldErrors(
  error: ApiError,
  t: (key: string) => string,
): ApiError {
  const details = error.details.map((detail) => {
    if (!detail.field?.startsWith("limits")) return detail;
    const message =
      detail.code && limitErrorCodes.has(detail.code)
        ? t(`plans.validation.limit_${detail.code}`)
        : detail.message;
    return { ...detail, field: limitErrorField(detail.field), message };
  });
  return new ApiError({
    status: error.status,
    code: error.code,
    message: error.message,
    details,
    requestId: error.requestId,
    body: error.body,
  });
}

type PlanFormProps = {
  mode?: "create" | "edit";
  initial?: PlanDetail;
  isSubmitting?: boolean;
  onSubmit: (values: PlanFormValues) => Promise<void> | void;
  onCancel: () => void;
};

function slugifyName(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function PlanFormFields({
  form,
  mode,
  isSubmitting,
  onCancel,
  features,
  featuresLoading,
}: {
  form: UseFormReturn<PlanFormValues>;
  mode: "create" | "edit";
  isSubmitting?: boolean;
  onCancel: () => void;
  features: Array<{ key: string; label: string; value_type: string }>;
  featuresLoading: boolean;
}) {
  const { t } = useLocale();
  const name = form.watch("name");
  const billingInterval = form.watch("billing_interval");
  const isFree = billingInterval === "free";

  useEffect(() => {
    if (isFree) {
      form.setValue("price_display", 0, { shouldDirty: true });
    }
  }, [form, isFree]);

  const billingOptions = useMemo(
    () =>
      (["free", "month", "year", "lifetime"] as const).map((value) => ({
        value,
        label: t(`plans.billing.${value}`),
      })),
    [t],
  );

  const boolOptions = useMemo(
    () => [
      { value: "true", label: t("plans.labels.enabled") },
      { value: "false", label: t("plans.labels.disabled") },
    ],
    [t],
  );

  const sections = useMemo(
    () =>
      createFormSections([
        {
          key: "details",
          label: t("plans.form.section_details"),
          description: t("plans.form.section_details_desc"),
          icon: CreditCard,
        },
        {
          key: "limits",
          label: t("plans.form.section_limits"),
          description: t("plans.form.section_limits_desc"),
          icon: CreditCard,
        },
      ]),
    [t],
  );

  useEffect(() => {
    if (mode !== "create") return;
    if (form.formState.dirtyFields.slug) return;
    const next = slugifyName(name);
    if (next && form.getValues("slug") !== next) {
      form.setValue("slug", next, { shouldDirty: false });
    }
  }, [form, mode, name]);

  return (
    <FormLayout navItems={sections.navItems}>
      <FormSection
        id={sections.id("details")}
        title={t("plans.form.section_details")}
        description={t("plans.form.section_details_desc")}
        columns={2}
      >
        <AppInput name="name" label={t("plans.fields.name")} />
        <AppInput
          name="slug"
          label={t("plans.fields.slug")}
          description={t("plans.fields.slug_hint")}
          disabled={mode === "edit"}
        />
        <AppTextarea
          name="description"
          label={t("plans.fields.description")}
          className="sm:col-span-2"
        />
        <AppSelect
          name="billing_interval"
          label={t("plans.fields.billing_interval")}
          options={billingOptions}
        />
        <AppInput
          name="price_display"
          label={t("plans.fields.price")}
          type="number"
          step="0.01"
          min={0}
          disabled={isFree}
          description={t("plans.fields.price_hint")}
        />
        <AppSelect
          name="currency"
          label={t("plans.fields.currency")}
          options={[{ value: "TRY", label: "TRY" }]}
          disabled={isFree}
        />
        <AppInput
          name="trial_days"
          label={t("plans.fields.trial_days")}
          type="number"
          min={0}
          disabled={isFree}
        />
        <AppInput
          name="sort_order"
          label={t("plans.fields.sort_order")}
          type="number"
        />
        <AppSwitch
          name="is_active"
          label={t("plans.fields.is_active")}
          className="sm:col-span-2"
        />
      </FormSection>

      <FormSection
        id={sections.id("limits")}
        title={t("plans.form.section_limits")}
        description={t("plans.form.section_limits_desc")}
        columns={2}
      >
        {featuresLoading ? (
          <Loading label={t("common.loading")} />
        ) : (
          features.map((feature) =>
            feature.value_type === "bool" ? (
              <AppSelect
                key={feature.key}
                name={limitFieldName(feature.key)}
                label={feature.label}
                options={boolOptions}
              />
            ) : (
              <AppInput
                key={feature.key}
                name={limitFieldName(feature.key)}
                label={feature.label}
                type="number"
                min={-1}
                step={1}
                description={t("plans.fields.limit_int_hint")}
              />
            ),
          )
        )}
      </FormSection>

      <FormActions>
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("form.cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting || form.formState.isSubmitting}>
          {mode === "create"
            ? t("plans.actions.create")
            : t("plans.actions.save")}
        </Button>
      </FormActions>
    </FormLayout>
  );
}

export function PlanForm({
  mode = "create",
  initial,
  isSubmitting,
  onSubmit,
  onCancel,
}: PlanFormProps) {
  const { t } = useLocale();
  const schema =
    mode === "create" ? createPlanFormSchema(t) : updatePlanFormSchema(t);

  const featuresQuery = useQuery({
    queryKey: plansKeys.features(),
    queryFn: () => plansService.listFeatures(),
  });

  const features = featuresQuery.data?.items ?? [];

  const resolvedDefaults: PlanFormValues = initial
    ? {
        name: initial.name,
        slug: initial.slug,
        description: initial.description ?? "",
        billing_interval: initial.billing_interval as PlanFormValues["billing_interval"],
        price_display: priceDisplayFromMinor(initial.price_minor ?? 0),
        currency: (initial.currency ?? "TRY") as "TRY",
        trial_days: initial.trial_days ?? 0,
        is_active: initial.is_active,
        sort_order: initial.sort_order,
        limits: limitsFromPlan(initial.limits),
      }
    : {
        name: "",
        slug: "",
        description: "",
        billing_interval: "free",
        price_display: 0,
        currency: "TRY",
        trial_days: 0,
        is_active: true,
        sort_order: 0,
        limits: {},
      };

  return (
    <AppForm
      schema={schema}
      defaultValues={resolvedDefaults}
      onSubmit={async (values) => {
        try {
          await onSubmit(values);
        } catch (error) {
          if (error instanceof ApiError && error.isValidation) {
            throw withLimitFieldErrors(error, t);
          }
          throw error;
        }
      }}
    >
      {(form) => (
        <PlanFormFields
          form={form}
          mode={mode}
          isSubmitting={isSubmitting}
          onCancel={onCancel}
          features={features}
          featuresLoading={featuresQuery.isLoading}
        />
      )}
    </AppForm>
  );
}
