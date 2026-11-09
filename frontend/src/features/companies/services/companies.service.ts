import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformRequest } from "@/lib/api/platform-request";

export type CompanyMember = {
  user_uuid: string;
  email: string;
  name: string;
  surname: string;
  role: string;
  created_at: string;
};

export type CompanySummary = {
  uuid: string;
  type?: string;
  name: string;
  slug: string;
  status: string;
  member_count?: number;
  created_at: string;
  updated_at: string;
};

export type CompanyDetail = CompanySummary & {
  members: CompanyMember[];
};

export type CompanyListResult = {
  items: CompanySummary[];
  total: number;
  limit: number;
  offset: number;
};

/** Company accounts on the platform panel, backed by /v1/platform/accounts (type=company). */
export const companiesService = {
  async list(params: ServerListParams & { status?: string }) {
    return platformRequest<CompanyListResult>("GET", "/v1/platform/accounts", {
      query: {
        limit: params.limit,
        offset: params.offset,
        q: params.q,
        type: "company",
        status: params.status,
      },
    });
  },

  async meta() {
    return platformRequest<ResourceMeta>("GET", "/v1/platform/accounts/meta");
  },

  async get(uuid: string) {
    return platformRequest<CompanyDetail>(
      "GET",
      `/v1/platform/accounts/${uuid}`,
    );
  },

  async create(body: { name: string; slug?: string; owner_user_uuid: string }) {
    return platformRequest<CompanyDetail>("POST", "/v1/platform/accounts", {
      body,
    });
  },

  async update(uuid: string, body: { name?: string; status?: string }) {
    return platformRequest<CompanyDetail>(
      "PATCH",
      `/v1/platform/accounts/${uuid}`,
      {
        body,
      },
    );
  },

  async addMember(
    companyUUID: string,
    body: { user_uuid: string; role: string },
  ) {
    return platformRequest<CompanyMember>(
      "POST",
      `/v1/platform/accounts/${companyUUID}/members`,
      { body },
    );
  },

  async changeMemberRole(companyUUID: string, userUUID: string, role: string) {
    return platformRequest<CompanyMember>(
      "PATCH",
      `/v1/platform/accounts/${companyUUID}/members/${userUUID}`,
      { body: { role } },
    );
  },

  async removeMember(companyUUID: string, userUUID: string) {
    return platformRequest<void>(
      "DELETE",
      `/v1/platform/accounts/${companyUUID}/members/${userUUID}`,
    );
  },
};
