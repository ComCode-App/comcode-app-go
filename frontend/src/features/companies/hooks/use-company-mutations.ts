"use client";

import { useQueryClient } from "@tanstack/react-query";

import { companiesKeys } from "@/features/companies/hooks/query-keys";
import { companiesService } from "@/features/companies/services/companies.service";
import { useAppMutation } from "@/lib/query/mutation";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";

export function useCreateCompany() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: companiesService.create,
    onSuccess: (company) => {
      queryClient.setQueryData(companiesKeys.detail(company.uuid), company);
      void queryClient.invalidateQueries({ queryKey: companiesKeys.lists() });
      void queryClient.invalidateQueries({
        queryKey: companiesKeys.navStats(),
      });
      appToast.success(t("companies.toast.created"));
    },
  });
}

export function useAddCompanyMember(companyUuid: string) {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: (body: { user_uuid: string; role: string }) =>
      companiesService.addMember(companyUuid, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: companiesKeys.detail(companyUuid),
      });
      appToast.success(t("companies.toast.member_added"));
    },
  });
}

export function useUpdateCompany(companyUuid: string) {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: (body: { name?: string; status?: string }) =>
      companiesService.update(companyUuid, body),
    onSuccess: (company) => {
      queryClient.setQueryData(companiesKeys.detail(companyUuid), company);
      void queryClient.invalidateQueries({ queryKey: companiesKeys.lists() });
      appToast.success(t("companies.toast.updated"));
    },
  });
}

export function useChangeCompanyMemberRole(companyUuid: string) {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: ({ userUuid, role }: { userUuid: string; role: string }) =>
      companiesService.changeMemberRole(companyUuid, userUuid, role),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: companiesKeys.detail(companyUuid),
      });
      appToast.success(t("companies.toast.member_updated"));
    },
  });
}

export function useRemoveCompanyMember(companyUuid: string) {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useAppMutation({
    mutationFn: (userUuid: string) =>
      companiesService.removeMember(companyUuid, userUuid),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: companiesKeys.detail(companyUuid),
      });
      void queryClient.invalidateQueries({ queryKey: companiesKeys.lists() });
      appToast.success(t("companies.toast.member_removed"));
    },
  });
}
