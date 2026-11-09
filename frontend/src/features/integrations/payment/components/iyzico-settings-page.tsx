"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { iyzicoNavIcon } from "@/components/icons/iyzico-icon";
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

const QUERY_KEY = ["platform", "payment", "iyzico"] as const;
const IyzicoNavIcon = iyzicoNavIcon;

const schema = z.object({
  enabled: z.boolean(),
  base_url: z.string(),
  api_key: z.string().optional(),
  secret_key: z.string().optional(),
});

export function IyzicoSettingsPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const queryClient = useQueryClient();
  const canWrite = can(permissions.payment.write);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => paymentIntegrationService.getIyzico(),
  });

  const saveMutation = useMutation({
    mutationFn: (values: z.infer<typeof schema>) =>
      paymentIntegrationService.patchIyzico({
        enabled: values.enabled,
        base_url: values.base_url,
        ...(values.api_key?.trim() ? { api_key: values.api_key.trim() } : {}),
        ...(values.secret_key?.trim()
          ? { secret_key: values.secret_key.trim() }
          : {}),
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      appToast.success(t("integrations.payment.toast.saved"));
    },
    onError: (error) => {
      appToast.error(isApiError(error) ? error.message : t("integrations.payment.toast.failed"));
    },
  });

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<IyzicoNavIcon className="size-7" />}
        title={t("integrations.payment.iyzico.title")}
        description={t("integrations.payment.iyzico.description")}
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
            base_url: data.base_url,
            api_key: "",
            secret_key: "",
          }}
          onSubmit={(values) => saveMutation.mutateAsync(values)}
        >
          {() => (
            <FormLayout>
              <FormSection title={t("integrations.payment.iyzico.section")} columns={2}>
                <PaymentCallbackUrlField provider="iyzico" />
                <AppSwitch
                  name="enabled"
                  label={t("integrations.payment.iyzico.enabled")}
                  description={t("integrations.payment.iyzico.enabled_hint")}
                  disabled={!canWrite}
                  className="sm:col-span-2 items-center rounded-lg border p-4"
                />
                <AppInput name="base_url" label={t("integrations.payment.iyzico.base_url")} disabled={!canWrite} />
                <AppInput
                  name="api_key"
                  label={t("integrations.payment.iyzico.api_key")}
                  type="password"
                  placeholder={data.api_key_configured ? t("integrations.payment.secret_configured") : ""}
                  disabled={!canWrite}
                />
                <AppInput
                  name="secret_key"
                  label={t("integrations.payment.iyzico.secret_key")}
                  type="password"
                  placeholder={data.secret_key_configured ? t("integrations.payment.secret_configured") : ""}
                  disabled={!canWrite}
                />
              </FormSection>
              <FormActions>
                <Button type="submit" disabled={!canWrite || saveMutation.isPending}>
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
