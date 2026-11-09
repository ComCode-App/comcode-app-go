import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformRequest } from "@/lib/api/platform-request";

export type PlanLimit = {
  feature_key: string;
  value_type: string;
  label: string;
  int_value?: number;
  bool_value?: boolean;
  description?: string | null;
};

export type PlanSummary = {
  uuid: string;
  slug: string;
  name: string;
  description?: string | null;
  billing_interval: string;
  price_minor: number;
  currency: string;
  trial_days: number;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
};

export type PlanDetail = PlanSummary & {
  limits: PlanLimit[];
};

export type FeatureDefinition = {
  key: string;
  value_type: string;
  label: string;
  description?: string | null;
  sort_order: number;
};

export type PlanListResult = {
  items: PlanSummary[];
  total: number;
  limit: number;
  offset: number;
};

export const plansService = {
  async list(params: ServerListParams) {
    return platformRequest<PlanListResult>("GET", "/v1/platform/plans", {
      query: { limit: params.limit, offset: params.offset, q: params.q },
    });
  },

  async meta() {
    return platformRequest<ResourceMeta>("GET", "/v1/platform/plans/meta");
  },

  async get(uuid: string) {
    return platformRequest<PlanDetail>("GET", `/v1/platform/plans/${uuid}`);
  },

  async listFeatures() {
    return platformRequest<{ items: FeatureDefinition[] }>(
      "GET",
      "/v1/platform/feature-definitions",
    );
  },

  async create(body: {
    slug: string;
    name: string;
    description?: string;
    billing_interval: string;
    price_minor: number;
    currency: string;
    trial_days: number;
    is_active: boolean;
    sort_order: number;
    limits: Array<{
      feature_key: string;
      int_value?: number;
      bool_value?: boolean;
    }>;
  }) {
    return platformRequest<PlanDetail>("POST", "/v1/platform/plans", { body });
  },

  async update(
    uuid: string,
    body: {
      name?: string;
      description?: string;
      billing_interval?: string;
      price_minor?: number;
      currency?: string;
      trial_days?: number;
      is_active?: boolean;
      sort_order?: number;
      limits?: Array<{
        feature_key: string;
        int_value?: number;
        bool_value?: boolean;
      }>;
    },
  ) {
    return platformRequest<PlanDetail>("PATCH", `/v1/platform/plans/${uuid}`, {
      body,
    });
  },

  async remove(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/platform/plans/${uuid}`);
  },
};
