"use client";

import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { billingService } from "@/features/billing/services/billing.service";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

export function BillingCheckoutPage({ sessionUuid }: { sessionUuid: string }) {
  const { t } = useLocale();
  const router = useRouter();
  const searchParams = useSearchParams();
  const result = searchParams.get("result");

  const sessionQuery = useQuery({
    queryKey: ["billing", "checkout", sessionUuid],
    queryFn: () => billingService.getCheckout(sessionUuid),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (
        status === "completed" ||
        status === "failed" ||
        status === "expired"
      ) {
        return false;
      }
      return 3000;
    },
  });

  useEffect(() => {
    if (sessionQuery.data?.checkout_url && typeof window !== "undefined") {
      window.location.href = sessionQuery.data.checkout_url;
    }
  }, [sessionQuery.data?.checkout_url]);

  useEffect(() => {
    if (sessionQuery.data?.status === "completed") {
      appToast.success(t("billing.toast.checkout_success"));
      router.replace(routes.app.billing.success);
    }
    if (result === "fail" || sessionQuery.data?.status === "failed") {
      appToast.error(t("billing.toast.checkout_failed"));
    }
  }, [result, router, sessionQuery.data?.status, t]);

  if (sessionQuery.isLoading) return <Loading label={t("common.loading")} />;
  if (sessionQuery.isError || !sessionQuery.data) {
    return (
      <ErrorState
        title={t("billing.checkout.error_title")}
        description={t("billing.checkout.error_description")}
        retryLabel={t("common.retry")}
        onRetry={() => void sessionQuery.refetch()}
      />
    );
  }

  const session = sessionQuery.data;

  const iframeToken = session.iframe_token ?? session.provider_token;

  if (session.checkout_url) {
    return <Loading label={t("billing.checkout.redirecting")} />;
  }

  if (iframeToken) {
    return (
      <div className="mx-auto w-full max-w-3xl">
        <iframe
          title="PayTR checkout"
          src={`https://www.paytr.com/odeme/guvenli/${iframeToken}`}
          className="min-h-[640px] w-full rounded-lg border"
        />
      </div>
    );
  }

  return <Loading label={t("billing.checkout.preparing")} />;
}
