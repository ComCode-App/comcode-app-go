"use client";

import { useRouter } from "next/navigation";

import { EntityForm, EntityPage } from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { SubscriptionForm } from "@/features/subscriptions/components/subscription-form";
import { useCreateSubscription } from "@/features/subscriptions/hooks/use-subscription-mutations";
import { useLocale } from "@/providers/locale-provider";

export function SubscriptionCreatePage() {
  const { t } = useLocale();
  const router = useRouter();
  const createSubscription = useCreateSubscription();

  return (
    <EntityPage
      title={t("subscriptions.create_title")}
      description={t("subscriptions.create_description")}
      permission={permissions.subscriptions.write}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        {
          label: t("subscriptions.title"),
          href: routes.platform.subscriptions.root,
        },
        { label: t("subscriptions.create_title") },
      ]}
    >
      <EntityForm>
        <SubscriptionForm
          isSubmitting={createSubscription.isPending}
          onCancel={() => router.push(routes.platform.subscriptions.root)}
          onSubmit={async (values) => {
            const subscription = await createSubscription.mutateAsync({
              plan_uuid: values.plan_uuid,
              status: values.status,
              user_uuid:
                values.subject_type === "user"
                  ? values.user_uuid
                  : undefined,
              company_uuid:
                values.subject_type === "company"
                  ? values.company_uuid
                  : undefined,
            });
            router.push(
              routes.platform.subscriptions.detail(subscription.uuid),
            );
          }}
        />
      </EntityForm>
    </EntityPage>
  );
}
