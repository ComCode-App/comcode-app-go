import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformRequest } from "@/lib/api/platform-request";
import type { Model3DViewboxConfig } from "./../components/Bounded3DStage";

export type ProductVariantSummary = {
  id?: number;
  uuid: string;
  name: string;
  sku: string;
  price_delta: number;
  stock_quantity: number;
  material: string;
  color_code: string;
  image_url: string;
  model_3d_file_url: string;
  model_3d_usdz_url: string;
  model_3d_viewbox?: Model3DViewboxConfig;
  is_active: boolean;
  display_order: number;
};

export type ProductSummary = {
  id?: number;
  uuid: string;
  category_uuid?: string;
  category_name?: string;
  category_slug?: string;
  name: string;
  slug: string;
  short_description: string;
  status: "active" | "draft" | "archived" | string;
  product_type: string;
  base_price: number;
  currency: string;
  tax_rate: number;
  stock_quantity: number;
  sku: string;
  allow_custom_print: boolean;
  model_3d_file_url: string;
  model_3d_usdz_url: string;
  images: string[];
  meta_title: string;
  meta_description: string;
  meta_keywords: string;
  canonical_url: string;
  og_image_url: string;
  display_order: number;
};

export type ProductDetail = ProductSummary & {
  description: string;
  model_3d_viewbox?: Model3DViewboxConfig;
  variants?: ProductVariantSummary[];
  created_at: string;
  updated_at: string;
};

export type ProductListResult = {
  items: ProductSummary[];
  total: number;
  limit: number;
  offset: number;
};

export type UpsertProductPayload = {
  category_uuid?: string;
  name: string;
  slug?: string;
  short_description?: string;
  description?: string;
  status: string;
  product_type?: string;
  base_price: number;
  currency?: string;
  tax_rate?: number;
  stock_quantity: number;
  sku?: string;
  allow_custom_print?: boolean;
  model_3d_file_url?: string;
  model_3d_usdz_url?: string;
  model_3d_viewbox?: Model3DViewboxConfig;
  images?: string[];
  meta_title?: string;
  meta_description?: string;
  meta_keywords?: string;
  canonical_url?: string;
  og_image_url?: string;
  display_order?: number;
};

export type UpsertVariantPayload = {
  name: string;
  sku?: string;
  price_delta?: number;
  stock_quantity?: number;
  material?: string;
  color_code?: string;
  image_url?: string;
  model_3d_file_url?: string;
  model_3d_usdz_url?: string;
  model_3d_viewbox?: Model3DViewboxConfig;
  is_active?: boolean;
  display_order?: number;
};

export const productsService = {
  async listPublic(params?: { limit?: number; offset?: number; q?: string; product_type?: string }) {
    return platformRequest<ProductListResult>(
      "GET",
      "/v1/products",
      {
        query: {
          limit: params?.limit ?? 50,
          offset: params?.offset ?? 0,
          q: params?.q,
          product_type: params?.product_type,
        },
      },
    );
  },

  async getPublic(uuid: string) {
    return platformRequest<ProductDetail>(
      "GET",
      `/v1/products/${uuid}`,
    );
  },

  async list(params: ServerListParams & { status?: string; category_id?: number }) {
    return platformRequest<ProductListResult>(
      "GET",
      "/v1/platform/products",
      {
        query: {
          limit: params.limit,
          offset: params.offset,
          q: params.q,
          status: params.status,
        },
      },
    );
  },

  async get(uuid: string) {
    return platformRequest<ProductDetail>(
      "GET",
      `/v1/platform/products/${uuid}`,
    );
  },

  async create(payload: UpsertProductPayload) {
    return platformRequest<ProductDetail>(
      "POST",
      "/v1/platform/products",
      { body: payload },
    );
  },

  async update(uuid: string, payload: UpsertProductPayload) {
    return platformRequest<ProductDetail>(
      "PUT",
      `/v1/platform/products/${uuid}`,
      { body: payload },
    );
  },

  async update3DViewbox(uuid: string, payload: { model_3d_viewbox: Model3DViewboxConfig; model_3d_file_url?: string; model_3d_usdz_url?: string }) {
    return platformRequest<ProductDetail>(
      "PUT",
      `/v1/platform/products/${uuid}/viewbox`,
      { body: payload },
    );
  },

  async delete(uuid: string) {
    return platformRequest<{ success: boolean }>(
      "DELETE",
      `/v1/platform/products/${uuid}`,
    );
  },

  async listVariants(productUuid: string) {
    return platformRequest<ProductVariantSummary[]>(
      "GET",
      `/v1/platform/products/${productUuid}/variants`,
    );
  },

  async createVariant(productUuid: string, payload: UpsertVariantPayload) {
    return platformRequest<ProductVariantSummary>(
      "POST",
      `/v1/platform/products/${productUuid}/variants`,
      { body: payload },
    );
  },

  async updateVariant(productUuid: string, variantUuid: string, payload: UpsertVariantPayload) {
    return platformRequest<ProductVariantSummary>(
      "PUT",
      `/v1/platform/products/${productUuid}/variants/${variantUuid}`,
      { body: payload },
    );
  },

  async deleteVariant(productUuid: string, variantUuid: string) {
    return platformRequest<{ success: boolean }>(
      "DELETE",
      `/v1/platform/products/${productUuid}/variants/${variantUuid}`,
    );
  },

  async meta() {
    return platformRequest<ResourceMeta>(
      "GET",
      "/v1/platform/products/meta",
    );
  },
};
