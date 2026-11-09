import type { BulkJob, BulkSummary, BulkTarget } from "@/features/bulk-engine";
import type { ResourceMeta } from "@/features/io/types";
import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type CompanyMemberProfile = components["schemas"]["CompanyMemberProfile"];
export type CompanyBulkAction = components["schemas"]["CompanyBulkRequest"]["action"];
export type ContactParams = components["schemas"]["CompanyBulkContactParams"];
export type LogoParams = components["schemas"]["CompanyBulkLogoParams"];
export type ThemeParams = components["schemas"]["CompanyBulkThemeParams"];
export type LinksParams = components["schemas"]["CompanyBulkLinksParams"];
export type CompanyBulkParams =
  | ContactParams
  | LogoParams
  | ThemeParams
  | LinksParams;

export type MemberProfileQuery = {
  limit: number;
  offset: number;
  q?: string;
  office_uuid?: string;
  group_uuid?: string;
  has_profile?: boolean;
};

type Page<T> = { items: T[]; total: number; limit: number; offset: number };

/** Inline runs answer with the finished job; larger runs return the queued job (202). */
export type CompanyBulkResult =
  | { sync: true; summary: BulkSummary; job: BulkJob }
  | BulkJob;

export function resultJob(result: CompanyBulkResult): BulkJob {
  return "sync" in result ? result.job : result;
}

/** Employee profiles of the active company (TEC-573, docs/COMPANIES.md "Bulk operations"). */
export const companyBulkService = {
  memberProfiles(query: MemberProfileQuery) {
    return platformRequest<Page<CompanyMemberProfile>>(
      "GET",
      "/v1/company/member-profiles",
      {
        query: {
          ...query,
          has_profile:
            query.has_profile === undefined
              ? undefined
              : String(query.has_profile),
        },
      },
    );
  },
  meta() {
    return platformRequest<ResourceMeta>(
      "GET",
      "/v1/company/member-profiles/meta",
    );
  },
  linkProfile(userUuid: string, profileUuid: string | null) {
    return platformRequest<{ user_uuid: string; profile_uuid: string | null }>(
      "PUT",
      `/v1/company/members/${userUuid}/profile`,
      { body: { profile_uuid: profileUuid }, silent: true },
    );
  },
  execute(body: {
    action: CompanyBulkAction;
    target: BulkTarget;
    params: CompanyBulkParams;
    locale: string;
  }) {
    return platformRequest<CompanyBulkResult>(
      "POST",
      "/v1/company/profiles/bulk",
      { body, silent: true },
    );
  },
  listJobs(params: { limit?: number; offset?: number }) {
    return platformRequest<Page<BulkJob>>("GET", "/v1/company/bulk", {
      query: params,
    });
  },
  getJob(uuid: string) {
    return platformRequest<BulkJob>("GET", `/v1/company/bulk/${uuid}`);
  },
  rollback(uuid: string) {
    return platformRequest<BulkJob>(
      "POST",
      `/v1/company/bulk/${uuid}/rollback`,
      { silent: true },
    );
  },
};
