"use client";

import { CreditCard, RefreshCw, Trash2 } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Permission } from "@/config/permissions";
import { usePermission } from "@/providers/permission-provider";
import { useAuthStore } from "@/lib/auth/session-store";
import { routes } from "@/config/routes";
import { BillingPaymentsCard } from "@/features/billing/components/billing-payments-card";
import { BillingPricing } from "@/features/billing/components/billing-pricing";
import { billingService } from "@/features/billing/services/billing.service";
import { formatPlanPrice } from "@/features/plans/lib/format-price";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const billingKeys = {
  all: ["billing"] as const,
  subscription: () => [...billingKeys.all, "subscription"] as const,
  plans: () => [...billingKeys.all, "plans"] as const,
  cards: () => [...billingKeys.all, "cards"] as const,
};

export function BillingPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const account = useAuthStore((s) => s.user?.account);
  const canWrite =
    can(Permission.AppBillingWrite) &&
    (account?.role === "owner" || account?.role === "admin");
  const router = useRouter();
  const queryClient = useQueryClient();

  const subscriptionQuery = useQuery({
    queryKey: [...billingKeys.subscription(), account?.uuid],
    queryFn: () => billingService.getSubscription(),
  });
  const plansQuery = useQuery({
    queryKey: [...billingKeys.plans(), account?.uuid],
    queryFn: () => billingService.listPlans(),
  });
  const cardsQuery = useQuery({
    queryKey: [...billingKeys.cards(), account?.uuid],
    queryFn: () => billingService.listCards(),
    enabled: canWrite,
  });

  const cancelMutation = useMutation({
    mutationFn: () => billingService.cancelSubscription(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: billingKeys.all });
      appToast.success(t("billing.toast.subscription_canceled"));
    },
    onError: (error) => {
      appToast.error(
        isApiError(error) ? error.message : t("billing.toast.failed"),
      );
    },
  });

  const resumeMutation = useMutation({
    mutationFn: () => billingService.resumeSubscription(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: billingKeys.all });
      appToast.success(t("billing.toast.saved"));
    },
    onError: (error) =>
      appToast.error(
        isApiError(error) ? error.message : t("billing.toast.failed"),
      ),
  });
  const changePlanMutation = useMutation({
    mutationFn: (planUuid: string) => billingService.changePlan(planUuid),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: billingKeys.all });
      appToast.success(t("billing.toast.saved"));
    },
    onError: (error) =>
      appToast.error(
        isApiError(error) ? error.message : t("billing.toast.failed"),
      ),
  });

  const addCardMutation = useMutation({
    mutationFn: () => billingService.startAddCard(),
    onSuccess: (data) => {
      router.push(routes.app.billing.addCard(data.session.uuid));
    },
    onError: (error) => {
      appToast.error(
        isApiError(error) ? error.message : t("billing.toast.failed"),
      );
    },
  });

  const checkoutMutation = useMutation({
    mutationFn: (planUuid: string) => billingService.startCheckout(planUuid),
    onSuccess: (data) => {
      router.push(routes.app.billing.checkout(data.session.uuid));
    },
    onError: (error) => {
      appToast.error(
        isApiError(error) ? error.message : t("billing.toast.failed"),
      );
    },
  });

  const subscribeMutation = useMutation({
    mutationFn: ({
      planUuid,
      cardUuid,
    }: {
      planUuid: string;
      cardUuid: string;
    }) => billingService.subscribe(planUuid, cardUuid),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: billingKeys.all });
      appToast.success(t("billing.toast.subscribed"));
    },
    onError: (error) => {
      appToast.error(
        isApiError(error) ? error.message : t("billing.toast.failed"),
      );
    },
  });

  const deleteCardMutation = useMutation({
    mutationFn: (uuid: string) => billingService.deleteCard(uuid),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: billingKeys.cards() });
      appToast.success(t("billing.toast.card_deleted"));
    },
  });

  const defaultCardMutation = useMutation({
    mutationFn: (uuid: string) => billingService.setDefaultCard(uuid),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: billingKeys.cards() });
      appToast.success(t("billing.toast.default_card_set"));
    },
  });

  const loading =
    subscriptionQuery.isLoading || plansQuery.isLoading || cardsQuery.isLoading;

  if (loading) return <Loading label={t("common.loading")} />;
  if (subscriptionQuery.isError || plansQuery.isError || cardsQuery.isError) {
    return (
      <ErrorState
        title={t("billing.error.title")}
        description={t("billing.error.description")}
        retryLabel={t("common.retry")}
        onRetry={() => {
          void subscriptionQuery.refetch();
          void plansQuery.refetch();
          void cardsQuery.refetch();
        }}
      />
    );
  }

  const subscription = subscriptionQuery.data?.subscription ?? null;
  const plans = plansQuery.data?.items ?? [];
  const cards = cardsQuery.data?.items ?? [];
  const defaultCard = cards.find((c) => c.is_default) ?? cards[0];
  const planActionPending =
    !canWrite ||
    changePlanMutation.isPending ||
    subscribeMutation.isPending ||
    checkoutMutation.isPending;

  const handleSelectPlan = (planUuid: string) => {
    if (subscription) {
      changePlanMutation.mutate(planUuid);
      return;
    }
    if (defaultCard) {
      subscribeMutation.mutate({
        planUuid,
        cardUuid: defaultCard.uuid,
      });
      return;
    }
    checkoutMutation.mutate(planUuid);
  };

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-8">
      <PageHeader
        icon={<CreditCard className="size-7" />}
        title={t("billing.title")}
        description={t("billing.description")}
      />

      {subscription && canWrite ? (
        <p className="text-muted-foreground text-sm">
          {t("billing.subscription.change_rule")}
        </p>
      ) : null}

      <BillingPricing
        plans={plans}
        subscription={subscription}
        defaultCard={defaultCard}
        isSubmitting={planActionPending}
        onSelectPlan={handleSelectPlan}
      />

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t("billing.subscription.title")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {subscription ? (
              <>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{subscription.plan_name}</span>
                  <Badge variant="outline">{subscription.status}</Badge>
                </div>
                <p className="text-muted-foreground">
                  {formatPlanPrice(
                    subscription.price_minor,
                    subscription.currency,
                  )}{" "}
                  / {t(`plans.billing.${subscription.billing_interval}`)}
                </p>
                {subscription.next_billing_at ? (
                  <p className="text-muted-foreground">
                    {t("billing.subscription.next_billing")}:{" "}
                    {new Date(
                      subscription.next_billing_at,
                    ).toLocaleDateString()}
                  </p>
                ) : null}
                {subscription.status === "past_due" ? (
                  <p role="alert" className="text-destructive">
                    {t("billing.subscription.past_due")}{" "}
                    {subscription.grace_until
                      ? new Date(subscription.grace_until).toLocaleDateString()
                      : ""}
                  </p>
                ) : null}
                {subscription.cancel_at_period_end ? (
                  <p role="status">
                    {t("billing.subscription.cancel_scheduled")}{" "}
                    {subscription.period_end
                      ? new Date(subscription.period_end).toLocaleDateString()
                      : ""}
                  </p>
                ) : null}
                {subscription.pending_plan_uuid ? (
                  <p role="status">
                    {t("billing.subscription.plan_scheduled")}
                  </p>
                ) : null}
                {canWrite ? (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={
                      cancelMutation.isPending || resumeMutation.isPending
                    }
                    onClick={() =>
                      subscription.cancel_at_period_end
                        ? resumeMutation.mutate()
                        : cancelMutation.mutate()
                    }
                  >
                    {t(
                      subscription.cancel_at_period_end
                        ? "billing.subscription.resume"
                        : "billing.subscription.cancel",
                    )}
                  </Button>
                ) : null}
              </>
            ) : (
              <p className="text-muted-foreground">
                {t("billing.subscription.none")}
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>{t("billing.cards.title")}</CardTitle>
            <Button
              size="sm"
              onClick={() => addCardMutation.mutate()}
              disabled={!canWrite || addCardMutation.isPending}
            >
              {t("billing.cards.add")}
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            {cards.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {t("billing.cards.empty")}
              </p>
            ) : (
              cards.map((card) => (
                <div
                  key={card.uuid}
                  className="flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3"
                >
                  <div>
                    <p className="font-medium">
                      {card.brand || t("billing.cards.card")} •••• {card.last4}
                    </p>
                    {card.is_default ? (
                      <Badge variant="secondary" className="mt-1 text-[10px]">
                        {t("billing.cards.default")}
                      </Badge>
                    ) : null}
                  </div>
                  <div className="flex gap-2">
                    {!card.is_default ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => defaultCardMutation.mutate(card.uuid)}
                      >
                        <RefreshCw className="size-4" />
                        {t("billing.cards.make_default")}
                      </Button>
                    ) : null}
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => deleteCardMutation.mutate(card.uuid)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>

      <BillingPaymentsCard accountUuid={account?.uuid} />
    </div>
  );
}
