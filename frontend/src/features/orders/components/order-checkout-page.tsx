"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, CreditCard, XCircle } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import {
  useCanPayOrders,
  useStartOrderPayment,
} from "../hooks/use-order-payment";
import {
  isOrderPayable,
  ordersService,
  type OrderCheckoutSession,
} from "../services/orders.service";

/**
 * Pays an order through the active provider (PayTR iFrame or iyzico hosted page) and shows the
 * result (TEC-943). The provider callback decides the outcome; this page only reads it.
 */
export function OrderCheckoutPage({ sessionUuid }: { sessionUuid: string }) {
  const { t } = useLocale();
  const result = useSearchParams().get("result");

  // PayTR returns to this page inside its iFrame; `PaymentFrameExit` in the app
  // layout moves it to the top window before the route guard runs (TEC-1125).

  const sessionQuery = useQuery({
    queryKey: ["orders", "checkout", sessionUuid],
    queryFn: () => ordersService.getCheckout(sessionUuid),
    refetchInterval: (query) =>
      query.state.data?.status === "pending" ? 3000 : false,
  });

  const checkoutURL =
    sessionQuery.data?.status === "pending"
      ? sessionQuery.data.checkout_url
      : undefined;
  useEffect(() => {
    if (checkoutURL && !result) {
      window.location.href = checkoutURL;
    }
  }, [checkoutURL, result]);

  if (sessionQuery.isLoading) return <Loading label={t("common.loading")} />;
  if (sessionQuery.isError || !sessionQuery.data) {
    return (
      <ErrorState
        title={t("orders.checkout.error_title")}
        description={t("orders.checkout.error_description")}
        retryLabel={t("common.retry")}
        onRetry={() => void sessionQuery.refetch()}
      />
    );
  }

  const session = sessionQuery.data;
  if (session.status === "completed")
    return <PaymentSucceeded session={session} />;
  if (
    session.status === "failed" ||
    session.status === "expired" ||
    result === "fail"
  ) {
    return <PaymentFailed session={session} />;
  }
  if (result === "success") {
    // The browser came back before the provider notification: wait for it.
    return <Loading className="py-16" label={t("orders.checkout.verifying")} />;
  }
  if (session.checkout_url)
    return (
      <Loading className="py-16" label={t("orders.checkout.redirecting")} />
    );
  if (session.iframe_token) {
    return (
      <div className="mx-auto w-full max-w-3xl space-y-4">
        <CheckoutSummary session={session} />
        <iframe
          title="PayTR"
          src={`https://www.paytr.com/odeme/guvenli/${session.iframe_token}`}
          className="min-h-[640px] w-full rounded-lg border"
        />
      </div>
    );
  }
  return <Loading className="py-16" label={t("orders.checkout.preparing")} />;
}

function formatAmount(session: OrderCheckoutSession, locale: string) {
  return (session.amount_minor / 100).toLocaleString(
    locale === "tr" ? "tr-TR" : "en-US",
    {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    },
  );
}

function CheckoutSummary({ session }: { session: OrderCheckoutSession }) {
  const { t, locale } = useLocale();
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="flex items-center gap-2">
        <CreditCard className="text-primary h-5 w-5" />
        <h1 className="text-lg font-semibold">{t("orders.checkout.title")}</h1>
        <span className="text-muted-foreground font-mono text-sm">
          #{session.order_number}
        </span>
      </div>
      <span className="text-primary font-bold">
        {formatAmount(session, locale)} {session.currency}
      </span>
    </div>
  );
}

function PaymentSucceeded({ session }: { session: OrderCheckoutSession }) {
  const { t, locale } = useLocale();
  return (
    <ResultCard
      icon={<CheckCircle2 className="h-8 w-8 text-emerald-600" />}
      title={t("orders.checkout.success_title")}
      description={t("orders.checkout.success_description")}
      session={session}
      amount={formatAmount(session, locale)}
    >
      <Button asChild>
        <Link href={routes.app.orders.root}>
          {t("orders.checkout.back_to_orders")}
        </Link>
      </Button>
    </ResultCard>
  );
}

function PaymentFailed({ session }: { session: OrderCheckoutSession }) {
  const { t, locale } = useLocale();
  const canPay = useCanPayOrders();
  const startPayment = useStartOrderPayment();
  const payable = isOrderPayable({
    status: session.order_status,
    payment_status: session.order_payment_status,
  });
  return (
    <ResultCard
      icon={<XCircle className="text-destructive h-8 w-8" />}
      title={t(
        session.status === "expired"
          ? "orders.checkout.expired_title"
          : "orders.checkout.failed_title",
      )}
      description={t(
        payable
          ? "orders.checkout.failed_description"
          : "orders.checkout.not_payable_description",
      )}
      session={session}
      amount={formatAmount(session, locale)}
    >
      {payable && canPay ? (
        <Button
          disabled={startPayment.isPending}
          onClick={() => startPayment.mutate(session.order_uuid)}
        >
          {t("orders.checkout.retry")}
        </Button>
      ) : null}
      <Button variant="outline" asChild>
        <Link href={routes.app.orders.root}>
          {t("orders.checkout.back_to_orders")}
        </Link>
      </Button>
    </ResultCard>
  );
}

function ResultCard({
  icon,
  title,
  description,
  session,
  amount,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  session: OrderCheckoutSession;
  amount: string;
  children: React.ReactNode;
}) {
  const { t } = useLocale();
  return (
    <Card className="mx-auto w-full max-w-lg">
      <CardContent className="flex flex-col items-center gap-3 p-8 text-center">
        {icon}
        <h1 className="text-xl font-semibold">{title}</h1>
        <p className="text-muted-foreground text-sm">{description}</p>
        <dl className="bg-muted/40 grid w-full grid-cols-2 gap-2 rounded-lg p-4 text-left text-sm">
          <dt className="text-muted-foreground">
            {t("orders.fields.order_number")}
          </dt>
          <dd className="text-right font-mono">#{session.order_number}</dd>
          <dt className="text-muted-foreground">
            {t("orders.fields.grand_total")}
          </dt>
          <dd className="text-right font-semibold">
            {amount} {session.currency}
          </dd>
        </dl>
        <div className="mt-2 flex flex-wrap justify-center gap-2">
          {children}
        </div>
      </CardContent>
    </Card>
  );
}
