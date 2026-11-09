"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Globe } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { PageHeader } from "@/components/layout/page-header";
import { permissions } from "@/config/permissions";
import { GeoIPLookupTester } from "@/features/geoip/components/geoip-lookup-tester";
import { GeoIPSettingsForm } from "@/features/geoip/components/geoip-settings-form";
import { GeoIPStatusCard } from "@/features/geoip/components/geoip-status-card";
import type { GeoIPFormValues } from "@/features/geoip/schemas/geoip-form";
import { geoipService } from "@/features/geoip/services/geoip.service";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

const STATUS_QUERY_KEY = ["platform", "geoip", "status"] as const;
const CONFIG_QUERY_KEY = ["platform", "geoip", "config"] as const;

export function GeoIPPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const queryClient = useQueryClient();
  const canWrite = can(permissions.geoip.manage);

  const {
    data: statusData,
    isLoading: isStatusLoading,
    isError: isStatusError,
    refetch: refetchStatus,
  } = useQuery({
    queryKey: STATUS_QUERY_KEY,
    queryFn: () => geoipService.getStatus(),
  });

  const {
    data: configData,
    isLoading: isConfigLoading,
    isError: isConfigError,
    refetch: refetchConfig,
  } = useQuery({
    queryKey: CONFIG_QUERY_KEY,
    queryFn: () => geoipService.getConfig(),
  });

  const saveMutation = useMutation({
    mutationFn: (values: GeoIPFormValues) =>
      geoipService.patchConfig({
        enabled: values.enabled,
        provider: values.provider,
        database_type: values.database_type,
        database_path: values.database_path,
        auto_update_enabled: values.auto_update_enabled,
        update_interval_hours: values.update_interval_hours,
        account_id: values.account_id || undefined,
        license_key: values.license_key || undefined,
        download_url_template: values.download_url_template || undefined,
        cache_enabled: values.cache_enabled,
        cache_ttl: values.cache_ttl,
        cache_max_entries: values.cache_max_entries,
      }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: CONFIG_QUERY_KEY }),
        queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY }),
      ]);
      appToast.success(t("geoip.toast.saved"));
    },
    onError: (error) => {
      if (isApiError(error)) {
        appToast.error(error.message);
        return;
      }
      appToast.error(t("geoip.toast.save_failed"));
    },
  });

  const isLoading = isStatusLoading || isConfigLoading;
  const isError = isStatusError || isConfigError;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Globe className="size-7" />}
        title={t("geoip.title")}
        description={t("geoip.description")}
      />

      {isLoading ? <Loading label={t("common.loading")} /> : null}

      {isError ? (
        <ErrorState
          title={t("geoip.error.title")}
          description={t("geoip.error.description")}
          retryLabel={t("common.retry")}
          onRetry={() => {
            refetchStatus();
            refetchConfig();
          }}
        />
      ) : null}

      {statusData ? (
        <GeoIPStatusCard
          status={statusData}
          queryKey={STATUS_QUERY_KEY}
        />
      ) : null}

      <GeoIPLookupTester />

      {configData ? (
        <GeoIPSettingsForm
          settings={configData}
          canWrite={canWrite}
          isSaving={saveMutation.isPending}
          onSubmit={async (values) => {
            await saveMutation.mutateAsync(values);
          }}
        />
      ) : null}
    </div>
  );
}
