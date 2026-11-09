import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type ProfileVerificationReview = components["schemas"]["ProfileVerificationReview"];
export type ProfileVerificationRequestStatus =
  components["schemas"]["ProfileVerificationRequestStatus"];

export type ProfileVerificationListParams = {
  limit: number;
  offset: number;
  q?: string;
  status?: ProfileVerificationRequestStatus;
};

export type ProfileVerificationPage = {
  items: ProfileVerificationReview[];
  total: number;
  limit: number;
  offset: number;
};

const base = "/v1/platform/profile-verifications";

/** Platform review queue of verified-badge requests (TEC-549). */
export const profileVerificationsService = {
  list(params: ProfileVerificationListParams) {
    return platformRequest<ProfileVerificationPage>("GET", base, {
      query: {
        limit: params.limit,
        offset: params.offset,
        q: params.q?.trim() || undefined,
        status: params.status,
      },
    });
  },
  approve(uuid: string) {
    return platformRequest<ProfileVerificationReview>("POST", `${base}/${uuid}/approve`);
  },
  reject(uuid: string, reason: string) {
    return platformRequest<ProfileVerificationReview>("POST", `${base}/${uuid}/reject`, {
      body: { reason },
    });
  },
  revoke(uuid: string, reason: string) {
    return platformRequest<ProfileVerificationReview>("POST", `${base}/${uuid}/revoke`, {
      body: { reason },
    });
  },
};

export const profileVerificationsKeys = {
  all: ["profile-verifications"] as const,
  list: (params: ProfileVerificationListParams) =>
    [...profileVerificationsKeys.all, "list", params] as const,
};
