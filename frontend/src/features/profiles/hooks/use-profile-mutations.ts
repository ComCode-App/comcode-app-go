import { useMutation, useQueryClient } from "@tanstack/react-query";

import { entitlementKeys } from "@/features/entitlements/hooks/use-entitlements";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  profilesService,
  type UpsertProfilePayload,
} from "@/features/profiles/services/profiles.service";
import { isPlanLimitError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

export function useCreateProfile() {
  const qc = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: (body: UpsertProfilePayload) => profilesService.create(body),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: profilesKeys.all });
      void qc.invalidateQueries({ queryKey: entitlementKeys.overview });
      appToast.success(t("profiles.toasts.create_success"));
    },
    onError: (err: Error) => {
      if (isPlanLimitError(err)) return;
      appToast.error(err.message || t("common.error_generic"));
    },
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: ({
      uuid,
      body,
    }: {
      uuid: string;
      body: UpsertProfilePayload;
    }) => profilesService.update(uuid, body),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: profilesKeys.all });
      void qc.invalidateQueries({ queryKey: profilesKeys.detail(vars.uuid) });
      appToast.success(t("profiles.toasts.update_success"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });
}

export function useDeleteProfile() {
  const qc = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: (uuid: string) => profilesService.remove(uuid),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: profilesKeys.all });
      void qc.invalidateQueries({ queryKey: entitlementKeys.overview });
      appToast.success(t("profiles.toasts.delete_success"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });
}

export function useSetPrimaryProfile() {
  const qc = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: (uuid: string) => profilesService.setPrimary(uuid),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: profilesKeys.all });
      appToast.success(t("profiles.toasts.set_primary_success"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });
}

export function useUploadAvatar() {
  const qc = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: ({ uuid, file }: { uuid: string; file: File }) =>
      profilesService.uploadAvatar(uuid, file),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: profilesKeys.detail(vars.uuid) });
      appToast.success(t("profiles.toasts.update_success"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });
}

export function useUploadCover() {
  const qc = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: ({ uuid, file }: { uuid: string; file: File }) =>
      profilesService.uploadCover(uuid, file),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: profilesKeys.detail(vars.uuid) });
      appToast.success(t("profiles.toasts.update_success"));
    },
    onError: (err: Error) => {
      appToast.error(err.message || t("common.error_generic"));
    },
  });
}
