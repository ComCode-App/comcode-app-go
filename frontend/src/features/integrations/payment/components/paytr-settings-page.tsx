"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { paytrNavIcon } from "@/components/icons/paytr-icon";
import { PageHeader } from "@/components/layout/page-header";
import {
  AppForm,
  AppInput,
  AppSwitch,
  FormActions,
  FormLayout,
  FormSection,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { paymentIntegrationService } from "@/features/integrations/payment/services/payment-integration.service";
import { PaymentCallbackUrlField } from "@/features/integrations/payment/components/payment-callback-url-field";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";
import { z } from "zod";

const QUERY_KEY = ["platform", "payment", "paytr"] as const;
const PayTRNavIcon = paytrNavIcon;

const schema = z.object({
  enabled: z.boolean(),
  merchant_id: z.string(),
  merchant_key: z.string().optional(),
  merchant_salt: z.string().optional(),
});

export function PayTRSettingsPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const queryClient = useQueryClient();
  const canWrite = can(permissions.payment.write);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => paymentIntegrationService.getPayTR(),
  });

  const saveMutation = useMutation({
    mutationFn: (values: z.infer<typeof schema>) =>
      paymentIntegrationService.patchPayTR({
        enabled: values.enabled,
        merchant_id: values.merchant_id,
        ...(values.merchant_key?.trim()
          ? { merchant_key: values.merchant_key.trim() }
          : {}),
        ...(values.merchant_salt?.trim()
          ? { merchant_salt: values.merchant_salt.trim() }
          : {}),
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      appToast.success(t("integrations.payment.toast.saved"));
    },
    onError: (error) => {
      appToast.error(
        isApiError(error)
          ? error.message
          : t("integrations.payment.toast.failed"),
      );
    },
  });

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<PayTRNavIcon className="size-7" />}
        title={t("integrations.payment.paytr.title")}
        description={t("integrations.payment.paytr.description")}
      />
      {isLoading ? <Loading label={t("common.loading")} /> : null}
      {isError ? (
        <ErrorState
          title={t("integrations.payment.error.title")}
          description={t("integrations.payment.error.description")}
          retryLabel={t("common.retry")}
          onRetry={() => refetch()}
        />
      ) : null}
      {data ? (
        <AppForm
          schema={schema}
          defaultValues={{
            enabled: data.enabled,
            merchant_id: data.merchant_id,
            merchant_key: "",
            merchant_salt: "",
          }}
          onSubmit={(values) => saveMutation.mutateAsync(values)}
        >
          {() => (
            <FormLayout>
              <FormSection
                title={t("integrations.payment.paytr.section")}
                columns={2}
              >
                <PaymentCallbackUrlField provider="paytr" />
                <AppSwitch
                  name="enabled"
                  label={t("integrations.payment.paytr.enabled")}
                  description={t("integrations.payment.paytr.enabled_hint")}
                  disabled={!canWrite}
                  className="items-center rounded-lg border p-4 sm:col-span-2"
                />
                <AppInput
                  name="merchant_id"
                  label={t("integrations.payment.paytr.merchant_id")}
                  disabled={!canWrite}
                />
                <AppInput
                  name="merchant_key"
                  label={t("integrations.payment.paytr.merchant_key")}
                  type="password"
                  placeholder={
                    data.merchant_key_configured
                      ? t("integrations.payment.secret_configured")
                      : ""
                  }
                  disabled={!canWrite}
                />
                <AppInput
                  name="merchant_salt"
                  label={t("integrations.payment.paytr.merchant_salt")}
                  type="password"
                  placeholder={
                    data.merchant_salt_configured
                      ? t("integrations.payment.secret_configured")
                      : ""
                  }
                  disabled={!canWrite}
                />
              </FormSection>
              <FormActions>
                <Button
                  type="submit"
                  disabled={!canWrite || saveMutation.isPending}
                >
                  {t("form.save")}
                </Button>
              </FormActions>
            </FormLayout>
          )}
        </AppForm>
      ) : null}
    </div>
  );
}
