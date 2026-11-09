"use client";

import { useQuery } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import {
  EntityDetail,
  EntityHeader,
  EntityPage,
  EntitySectionCard,
} from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { subscriptionsKeys } from "@/features/subscriptions/hooks/query-keys";
import { subscriptionsService } from "@/features/subscriptions/services/subscriptions.service";
import { useLocale } from "@/providers/locale-provider";

export function SubscriptionDetailPage({ uuid }: { uuid: string }) {
  const { t } = useLocale();

  const detailQuery = useQuery({
    queryKey: subscriptionsKeys.detail(uuid),
    queryFn: () => subscriptionsService.get(uuid),
  });

  const data = detailQuery.data;
  const title = data?.plan_name ?? t("subscriptions.detail_title");

  return (
    <EntityPage
      title={title}
      description={t("subscriptions.detail_description")}
      permission={permissions.subscriptions.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("subscriptions.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        {
          label: t("subscriptions.title"),
          href: routes.platform.subscriptions.root,
        },
        { label: title },
      ]}
    >
      {detailQuery.isLoading ? <Loading label={t("common.loading")} /> : null}
      {detailQuery.isError ? (
        <ErrorState
          title={t("common.error_generic")}
          description={t("subscriptions.detail_not_found")}
          onRetry={() => void detailQuery.refetch()}
          retryLabel={t("common.retry")}
        />
      ) : null}

      {data ? (
        <div className="space-y-6">
          <EntityHeader
            title={data.plan_name}
            description={data.plan_slug}
            badges={
              <Badge variant="outline">
                {t(`subscriptions.status.${data.status}`)}
              </Badge>
            }
          />

          <EntitySectionCard title={t("subscriptions.detail.profile")}>
            <EntityDetail
              sections={[
                {
                  id: "profile",
                  fields: [
                    {
                      key: "plan",
                      label: t("subscriptions.fields.plan_name"),
                      value: data.plan_name,
                    },
                    {
                      key: "status",
                      label: t("subscriptions.fields.status"),
                      value: t(`subscriptions.status.${data.status}`),
                    },
                    {
                      key: "subject",
                      label: t("subscriptions.fields.subject"),
                      value: `${t(`subscriptions.subject_type.${data.subject.type}`)}: ${data.subject.name}`,
                    },
                    {
                      key: "starts_at",
                      label: t("subscriptions.fields.starts_at"),
                      value: data.starts_at,
                    },
                    {
                      key: "ends_at",
                      label: t("subscriptions.fields.ends_at"),
                      value: data.ends_at ?? "—",
                    },
                    {
                      key: "uuid",
                      label: t("subscriptions.fields.uuid"),
                      value: <code className="text-xs">{data.uuid}</code>,
                    },
                  ],
                },
              ]}
            />
          </EntitySectionCard>
        </div>
      ) : null}
    </EntityPage>
  );
}
