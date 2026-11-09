"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CreditCard } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { PageHeader } from "@/components/layout/page-header";
import {
  AppForm,
  AppSelect,
  AppSwitch,
  FormActions,
  FormLayout,
  FormSection,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { paymentIntegrationService } from "@/features/integrations/payment/services/payment-integration.service";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";
import { z } from "zod";

const QUERY_KEY = ["platform", "payment", "settings"] as const;

const schema = z.object({
  active_provider: z.enum(["none", "paytr", "iyzico"]),
  test_mode: z.boolean(),
});

export function PaymentSettingsPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const queryClient = useQueryClient();
  const canWrite = can(permissions.payment.write);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => paymentIntegrationService.getSettings(),
  });

  const saveMutation = useMutation({
    mutationFn: (values: z.infer<typeof schema>) =>
      paymentIntegrationService.patchSettings(values),
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
        icon={<CreditCard className="size-7" />}
        title={t("integrations.payment.settings.title")}
        description={t("integrations.payment.settings.description")}
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
            active_provider: data.active_provider,
            test_mode: data.test_mode,
          }}
          onSubmit={(values) => saveMutation.mutateAsync(values)}
        >
          {() => (
            <FormLayout>
              <FormSection title={t("integrations.payment.settings.section")} columns={2}>
                <AppSelect
                  name="active_provider"
                  label={t("integrations.payment.settings.active_provider")}
                  options={[
                    { value: "none", label: t("integrations.payment.providers.none") },
                    { value: "paytr", label: "PayTR" },
                    { value: "iyzico", label: "Iyzico" },
                  ]}
                  disabled={!canWrite}
                />
                <AppSwitch
                  name="test_mode"
                  label={t("integrations.payment.settings.test_mode")}
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
