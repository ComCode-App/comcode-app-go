"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { permissions } from "@/config/permissions";
import { SettingsForm } from "@/features/io/components/settings-form";
import { ioKeys } from "@/features/io/hooks/query-keys";
import type { PatchSettingsRequest } from "@/features/io/services/settings.service";
import { settingsService } from "@/features/io/services/settings.service";
import { uploadErrorKey } from "@/lib/api/upload-errors";
import { useAppMutation } from "@/lib/query/mutation";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

export function GeneralSettingsPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const canWrite = can(permissions.settings.write);
  const queryClient = useQueryClient();

  const settingsQuery = useQuery({
    queryKey: ioKeys.settings.all,
    queryFn: () => settingsService.get(),
  });

  const save = useAppMutation({
    mutationFn: (body: PatchSettingsRequest) => settingsService.patch(body),
    onSuccess: (data) => {
      void queryClient.setQueryData(ioKeys.settings.all, data);
      appToast.success(t("settings.toast.saved"));
    },
    onError: () => {
      appToast.error(t("settings.toast.save_failed"));
    },
  });

  const uploadLogo = useAppMutation({
    mutationFn: (file: File) => settingsService.uploadLogo(file),
    onSuccess: (data) => {
      void queryClient.setQueryData(ioKeys.settings.all, data);
      appToast.success(t("settings.toast.logo_saved"));
    },
    onError: (err) => {
      // Unsafe SVG (422 SVG_UNSAFE_CONTENT) or oversized file: clear localized warning.
      const key = uploadErrorKey(err);
      appToast.error(key ? t(key) : err instanceof Error && err.message ? err.message : t("common.error_generic"));
    },
  });

  const removeLogo = useAppMutation({
    mutationFn: () => settingsService.deleteLogo(),
    onSuccess: (data) => {
      void queryClient.setQueryData(ioKeys.settings.all, data);
      appToast.success(t("settings.toast.logo_removed"));
    },
  });

  if (!can(permissions.settings.read)) {
    return (
      <ErrorState
        title={t("common.error_forbidden")}
        description={t("settings.forbidden")}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight text-foreground md:text-2xl">
          {t("settings.subnav.general_branding")}
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          {t("settings.description")}
        </p>
      </div>

      {settingsQuery.isLoading ? (
        <Loading label={t("common.loading")} />
      ) : settingsQuery.isError ? (
        <ErrorState
          title={t("common.error_generic")}
          description={t("settings.forbidden")}
          onRetry={() => void settingsQuery.refetch()}
          retryLabel={t("common.retry")}
        />
      ) : settingsQuery.data ? (
        <SettingsForm
          key={JSON.stringify(settingsQuery.data)}
          settings={settingsQuery.data}
          canWrite={canWrite}
          isSaving={save.isPending}
          isLogoUploading={uploadLogo.isPending}
          isLogoRemoving={removeLogo.isPending}
          onSubmit={async (payload) => {
            await save.mutateAsync(payload);
          }}
          onUploadLogo={(file) => uploadLogo.mutate(file)}
          onRemoveLogo={() => removeLogo.mutate()}
        />
      ) : null}
    </div>
  );
}
