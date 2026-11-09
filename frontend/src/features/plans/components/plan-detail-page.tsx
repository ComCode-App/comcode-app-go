"use client";

import Link from "next/link";
import { Pencil } from "lucide-react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { PermissionGuard } from "@/components/common/permission-guard";
import {
  EntityDetail,
  EntityHeader,
  EntityPage,
  EntitySectionCard,
} from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { plansKeys } from "@/features/plans/hooks/query-keys";
import { useDeletePlan } from "@/features/plans/hooks/use-plan-mutations";
import { plansService } from "@/features/plans/services/plans.service";
import { formatPlanPrice } from "@/features/plans/lib/format-price";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

export function PlanDetailPage({ uuid }: { uuid: string }) {
  const { t } = useLocale();
  const router = useRouter();
  const { confirmDelete } = useDialogs();
  const deletePlan = useDeletePlan();

  const detailQuery = useQuery({
    queryKey: plansKeys.detail(uuid),
    queryFn: () => plansService.get(uuid),
  });

  const data = detailQuery.data;
  const title = data?.name ?? t("plans.detail_title");
  const isProtected = data?.slug === "free";

  return (
    <EntityPage
      title={title}
      description={t("plans.detail_description")}
      permission={permissions.plans.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("plans.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("plans.title"), href: routes.platform.plans.root },
        { label: title },
      ]}
      actions={
        data ? (
          <div className="flex gap-2">
            <PermissionGuard permission={permissions.plans.write}>
              <Button asChild size="sm">
                <Link href={routes.platform.plans.edit(uuid)}>
                  <Pencil className="size-4" />
                  {t("plans.actions.edit")}
                </Link>
              </Button>
            </PermissionGuard>
            {!isProtected ? (
              <PermissionGuard permission={permissions.plans.write}>
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => {
                    void confirmDelete({
                      title: t("plans.delete_title"),
                      description: t("plans.delete_description", {
                        name: data.name,
                      }),
                    }).then(async (confirmed) => {
                      if (!confirmed) return;
                      await deletePlan.mutateAsync(uuid);
                      router.push(routes.platform.plans.root);
                    });
                  }}
                >
                  {t("plans.actions.delete")}
                </Button>
              </PermissionGuard>
            ) : null}
          </div>
        ) : null
      }
    >
      {detailQuery.isLoading ? <Loading label={t("common.loading")} /> : null}
      {detailQuery.isError ? (
        <ErrorState
          title={t("common.error_generic")}
          description={t("plans.detail_not_found")}
          onRetry={() => void detailQuery.refetch()}
          retryLabel={t("common.retry")}
        />
      ) : null}

      {data ? (
        <div className="space-y-6">
          <EntityHeader
            title={data.name}
            description={data.slug}
            badges={
              <>
                {isProtected ? (
                  <Badge variant="secondary">{t("plans.labels.default")}</Badge>
                ) : null}
                <Badge variant={data.is_active ? "outline" : "secondary"}>
                  {data.is_active
                    ? t("plans.labels.active")
                    : t("plans.labels.inactive")}
                </Badge>
              </>
            }
          />

          <EntitySectionCard title={t("plans.detail.profile")}>
            <EntityDetail
              sections={[
                {
                  id: "profile",
                  fields: [
                    {
                      key: "name",
                      label: t("plans.fields.name"),
                      value: data.name,
                    },
                    {
                      key: "slug",
                      label: t("plans.fields.slug"),
                      value: <code className="text-xs">{data.slug}</code>,
                    },
                    {
                      key: "billing_interval",
                      label: t("plans.fields.billing_interval"),
                      value: t(`plans.billing.${data.billing_interval}`),
                    },
                    {
                      key: "price",
                      label: t("plans.fields.price"),
                      value:
                        data.billing_interval === "free"
                          ? t("plans.billing.free")
                          : formatPlanPrice(data.price_minor, data.currency),
                    },
                    {
                      key: "trial_days",
                      label: t("plans.fields.trial_days"),
                      value: String(data.trial_days ?? 0),
                    },
                    {
                      key: "description",
                      label: t("plans.fields.description"),
                      value: data.description ?? "—",
                    },
                    {
                      key: "sort_order",
                      label: t("plans.fields.sort_order"),
                      value: String(data.sort_order),
                    },
                    {
                      key: "uuid",
                      label: t("plans.fields.uuid"),
                      value: <code className="text-xs">{data.uuid}</code>,
                    },
                  ],
                },
              ]}
            />
          </EntitySectionCard>

          <EntitySectionCard title={t("plans.form.section_limits")}>
            {data.limits.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {t("plans.limits_empty_description")}
              </p>
            ) : (
              <EntityDetail
                sections={[
                  {
                    id: "limits",
                    fields: data.limits.map((lim) => ({
                      key: lim.feature_key,
                      label: lim.label,
                      value:
                        lim.int_value != null
                          ? String(lim.int_value)
                          : lim.bool_value
                            ? t("plans.labels.enabled")
                            : t("plans.labels.disabled"),
                    })),
                  },
                ]}
              />
            )}
          </EntitySectionCard>
        </div>
      ) : null}
    </EntityPage>
  );
}
