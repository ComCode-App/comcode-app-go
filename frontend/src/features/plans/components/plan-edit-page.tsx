"use client";

import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { EntityForm, EntityPage } from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { PlanForm } from "@/features/plans/components/plan-form";
import { plansKeys } from "@/features/plans/hooks/query-keys";
import { useUpdatePlan } from "@/features/plans/hooks/use-plan-mutations";
import { mapPlanLimits } from "@/features/plans/schemas/plan-form";
import { priceMinorFromDisplay } from "@/features/plans/lib/format-price";
import { plansService } from "@/features/plans/services/plans.service";
import { useLocale } from "@/providers/locale-provider";

export function PlanEditPage({ uuid }: { uuid: string }) {
  const { t } = useLocale();
  const router = useRouter();
  const updatePlan = useUpdatePlan(uuid);

  const detailQuery = useQuery({
    queryKey: plansKeys.detail(uuid),
    queryFn: () => plansService.get(uuid),
  });

  if (detailQuery.isLoading) {
    return <Loading label={t("common.loading")} />;
  }

  if (detailQuery.isError || !detailQuery.data) {
    return (
      <ErrorState
        title={t("common.error_not_found")}
        description={t("plans.detail_not_found")}
        onRetry={() => void detailQuery.refetch()}
        retryLabel={t("common.retry")}
      />
    );
  }

  return (
    <EntityPage
      title={t("plans.edit_title")}
      description={t("plans.edit_description")}
      permission={permissions.plans.write}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("plans.title"), href: routes.platform.plans.root },
        { label: detailQuery.data.name },
      ]}
    >
      <EntityForm>
        <PlanForm
          mode="edit"
          initial={detailQuery.data}
          isSubmitting={updatePlan.isPending}
          onCancel={() => router.push(routes.platform.plans.detail(uuid))}
          onSubmit={async (values) => {
            await updatePlan.mutateAsync({
              name: values.name,
              description: values.description,
              billing_interval: values.billing_interval,
              price_minor: priceMinorFromDisplay(values.price_display),
              currency: values.currency,
              trial_days: values.trial_days,
              is_active: values.is_active,
              sort_order: values.sort_order,
              limits: mapPlanLimits(values),
            });
            router.push(routes.platform.plans.detail(uuid));
          }}
        />
      </EntityForm>
    </EntityPage>
  );
}
