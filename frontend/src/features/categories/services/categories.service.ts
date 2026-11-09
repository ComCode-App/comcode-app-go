import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformRequest } from "@/lib/api/platform-request";

export type CategorySummary = {
  id?: number;
  uuid: string;
  name: string;
  slug: string;
  description: string;
  icon: string;
  image_url: string;
  display_order: number;
  is_active: boolean;
};

export type CategoryDetail = CategorySummary & {
  created_at: string;
  updated_at: string;
};

export type CategoryListResult = {
  items: CategorySummary[];
  total: number;
  limit: number;
  offset: number;
};

export type UpsertCategoryPayload = {
  name: string;
  slug?: string;
  description?: string;
  icon?: string;
  image_url?: string;
  display_order: number;
  is_active: boolean;
};

export const categoriesService = {
  async list(params: ServerListParams) {
    return platformRequest<CategoryListResult>(
      "GET",
      "/v1/platform/categories",
      {
        query: {
          limit: params.limit,
          offset: params.offset,
          q: params.q,
        },
      },
    );
  },

  async listActive(): Promise<CategorySummary[]> {
    const res = await platformRequest<{ data?: CategorySummary[]; items?: CategorySummary[] } | CategorySummary[]>(
      "GET",
      "/v1/platform/categories/active",
    );
    if (Array.isArray(res)) return res;
    if (Array.isArray(res?.data)) return res.data;
    if (Array.isArray(res?.items)) return res.items;
    return [];
  },

  async get(uuid: string) {
    return platformRequest<CategoryDetail>(
      "GET",
      `/v1/platform/categories/${uuid}`,
    );
  },

  async create(payload: UpsertCategoryPayload) {
    return platformRequest<CategoryDetail>(
      "POST",
      "/v1/platform/categories",
      { body: payload },
    );
  },

  async update(uuid: string, payload: UpsertCategoryPayload) {
    return platformRequest<CategoryDetail>(
      "PUT",
      `/v1/platform/categories/${uuid}`,
      { body: payload },
    );
  },

  async delete(uuid: string) {
    return platformRequest<{ success: boolean }>(
      "DELETE",
      `/v1/platform/categories/${uuid}`,
    );
  },

  async meta() {
    return platformRequest<ResourceMeta>(
      "GET",
      "/v1/platform/categories/meta",
    );
  },
};
