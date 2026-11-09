import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformFormRequest } from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type SocialPlatformSummary = {
  id?: number;
  uuid: string;
  name: string;
  slug: string;
  category: string;
  url_pattern: string;
  icon_url: string;
  placeholder: string;
  description: string;
  display_order: number;
  is_active: boolean;
};

export type SocialPlatformDetail = SocialPlatformSummary & {
  created_at: string;
  updated_at: string;
};

export type SocialPlatformListResult = {
  items: SocialPlatformSummary[];
  total: number;
  limit: number;
  offset: number;
};

export type UpsertSocialPlatformPayload = {
  name: string;
  slug?: string;
  category: string;
  url_pattern: string;
  icon_url?: string;
  placeholder: string;
  description: string;
  display_order: number;
  is_active: boolean;
};

export const socialPlatformsService = {
  async list(params: ServerListParams) {
    return platformRequest<SocialPlatformListResult>(
      "GET",
      "/v1/platform/social-platforms",
      {
        query: {
          limit: params.limit,
          offset: params.offset,
          q: params.q,
        },
      },
    );
  },

  async listActive() {
    const res = await platformRequest<{ data: SocialPlatformSummary[] }>(
      "GET",
      "/v1/social-platforms",
    );
    return res.data ?? [];
  },

  async meta() {
    return platformRequest<ResourceMeta>(
      "GET",
      "/v1/platform/social-platforms/meta",
    );
  },

  async get(uuid: string) {
    return platformRequest<SocialPlatformDetail>(
      "GET",
      `/v1/platform/social-platforms/${uuid}`,
    );
  },

  async create(body: UpsertSocialPlatformPayload) {
    return platformRequest<SocialPlatformDetail>(
      "POST",
      "/v1/platform/social-platforms",
      { body },
    );
  },

  async update(uuid: string, body: UpsertSocialPlatformPayload) {
    return platformRequest<SocialPlatformDetail>(
      "PUT",
      `/v1/platform/social-platforms/${uuid}`,
      { body },
    );
  },

  async uploadIcon(uuid: string, file: File) {
    const formData = new FormData();
    formData.append("icon", file);
    return platformFormRequest<SocialPlatformDetail>(
      "POST",
      `/v1/platform/social-platforms/${uuid}/icon`,
      formData,
    );
  },

  async fetchIcon(uuid: string) {
    return platformRequest<SocialPlatformDetail>(
      "POST",
      `/v1/platform/social-platforms/${uuid}/fetch-icon`,
    );
  },

  async bulkFetchIcons(uuids?: string[]) {
    return platformRequest<{ updated_count: number; success: boolean }>(
      "POST",
      "/v1/platform/social-platforms/bulk-fetch-icons",
      { body: { uuids: uuids || [] } },
    );
  },

  async remove(uuid: string) {
    return platformRequest<void>(
      "DELETE",
      `/v1/platform/social-platforms/${uuid}`,
    );
  },
};
