"use client";

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { requestErrorText } from "../lib/errors";
import {
  companyBulkService,
  type MemberProfileQuery,
} from "../services/company-bulk.service";
import { companyBulkKeys } from "./query-keys";

const ACTIVE = new Set(["queued", "processing"]);

export function useMemberProfiles(query: MemberProfileQuery, enabled = true) {
  return useQuery({
    queryKey: companyBulkKeys.members(query),
    queryFn: () => companyBulkService.memberProfiles(query),
    placeholderData: keepPreviousData,
    enabled,
  });
}

export function useCompanyBulkMeta(enabled = true) {
  return useQuery({
    queryKey: companyBulkKeys.meta(),
    queryFn: companyBulkService.meta,
    staleTime: 5 * 60_000,
    enabled,
  });
}

export function useCompanyBulkJobs(enabled = true) {
  return useQuery({
    queryKey: companyBulkKeys.jobs(),
    queryFn: () => companyBulkService.listJobs({ limit: 20, offset: 0 }),
    enabled,
    refetchInterval: (query) =>
      query.state.data?.items.some((j) => ACTIVE.has(j.status)) ? 2000 : false,
  });
}

/** Polls a queued/processing job until it settles. */
export function useCompanyBulkJob(uuid: string | null) {
  return useQuery({
    queryKey: companyBulkKeys.job(uuid ?? ""),
    queryFn: () => companyBulkService.getJob(uuid ?? ""),
    enabled: Boolean(uuid),
    refetchInterval: (query) =>
      query.state.data && ACTIVE.has(query.state.data.status) ? 2000 : false,
  });
}

export function useCompanyBulkRollback() {
  const { t } = useLocale();
  const qc = useQueryClient();
  return useMutation({
    meta: { silentErrors: true },
    mutationFn: (uuid: string) => companyBulkService.rollback(uuid),
    onSuccess: (job) => {
      appToast.success(
        job.status === "rolled_back_partial"
          ? t("company_bulk.jobs.rolled_back_partial")
          : t("company_bulk.jobs.rolled_back"),
      );
      void qc.invalidateQueries({ queryKey: companyBulkKeys.all });
    },
    onError: (err) => appToast.error(requestErrorText(t, err)),
  });
}

export function useLinkProfile() {
  const { t } = useLocale();
  const qc = useQueryClient();
  return useMutation({
    meta: { silentErrors: true },
    mutationFn: ({
      userUuid,
      profileUuid,
    }: {
      userUuid: string;
      profileUuid: string | null;
    }) => companyBulkService.linkProfile(userUuid, profileUuid),
    onSuccess: (_, vars) => {
      appToast.success(
        vars.profileUuid
          ? t("company_bulk.link.linked")
          : t("company_bulk.link.unlinked"),
      );
      void qc.invalidateQueries({ queryKey: companyBulkKeys.all });
    },
    onError: (err) => appToast.error(requestErrorText(t, err)),
  });
}
