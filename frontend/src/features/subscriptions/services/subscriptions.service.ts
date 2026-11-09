import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformRequest } from "@/lib/api/platform-request";

export type SubscriptionSummary = {
  uuid: string;
  status: string;
  plan_uuid: string;
  plan_slug: string;
  plan_name: string;
  subject: { type: string; uuid: string; name: string };
  starts_at: string;
  ends_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type SubscriptionListResult = {
  items: SubscriptionSummary[];
  total: number;
  limit: number;
  offset: number;
};

export const subscriptionsService = {
  async list(params: ServerListParams & { status?: string }) {
    return platformRequest<SubscriptionListResult>(
      "GET",
      "/v1/platform/subscriptions",
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

  async meta() {
    return platformRequest<ResourceMeta>(
      "GET",
      "/v1/platform/subscriptions/meta",
    );
  },

  async get(uuid: string) {
    return platformRequest<SubscriptionSummary>(
      "GET",
      `/v1/platform/subscriptions/${uuid}`,
    );
  },

  async create(body: {
    plan_uuid: string;
    status?: string;
    user_uuid?: string;
    company_uuid?: string;
    starts_at?: string;
    ends_at?: string;
  }) {
    return platformRequest<SubscriptionSummary>(
      "POST",
      "/v1/platform/subscriptions",
      { body },
    );
  },

  async update(
    uuid: string,
    body: {
      plan_uuid?: string;
      status?: string;
      starts_at?: string;
      ends_at?: string | null;
    },
  ) {
    return platformRequest<SubscriptionSummary>(
      "PATCH",
      `/v1/platform/subscriptions/${uuid}`,
      { body },
    );
  },
};
