"use client";

import { useRouter } from "next/navigation";

import { EntityForm, EntityPage } from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { PlanForm } from "@/features/plans/components/plan-form";
import { useCreatePlan } from "@/features/plans/hooks/use-plan-mutations";
import { mapPlanLimits } from "@/features/plans/schemas/plan-form";
import { priceMinorFromDisplay } from "@/features/plans/lib/format-price";
import { useLocale } from "@/providers/locale-provider";

export function PlanCreatePage() {
  const { t } = useLocale();
  const router = useRouter();
  const createPlan = useCreatePlan();

  return (
    <EntityPage
      title={t("plans.create_title")}
      description={t("plans.create_description")}
      permission={permissions.plans.write}
      forbiddenFallback={null}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("plans.title"), href: routes.platform.plans.root },
        { label: t("plans.create_title") },
      ]}
    >
      <EntityForm>
        <PlanForm
          mode="create"
          isSubmitting={createPlan.isPending}
          onCancel={() => router.push(routes.platform.plans.root)}
          onSubmit={async (values) => {
            const plan = await createPlan.mutateAsync({
              name: values.name,
              slug: values.slug,
              description: values.description,
              billing_interval: values.billing_interval,
              price_minor: priceMinorFromDisplay(values.price_display),
              currency: values.currency,
              trial_days: values.trial_days,
              is_active: values.is_active,
              sort_order: values.sort_order,
              limits: mapPlanLimits(values),
            });
            router.push(routes.platform.plans.detail(plan.uuid));
          }}
        />
      </EntityForm>
    </EntityPage>
  );
}
