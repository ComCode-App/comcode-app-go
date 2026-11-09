import { platformRequest } from "@/lib/api/platform-request";

export type WebhookEventName =
  "lead.created" | "card.scanned" | "order.created" | "order.status_changed";

export type WebhookEventDefinition = {
  name: WebhookEventName;
  description_tr: string;
  description_en: string;
};

export type WebhookEndpoint = {
  uuid: string;
  url: string;
  description: string;
  events: WebhookEventName[];
  is_active: boolean;
  secret_hint: string;
  consecutive_failures: number;
  last_delivery_at?: string;
  last_success_at?: string;
  created_at: string;
  updated_at: string;
};

export type WebhookEndpointWithSecret = WebhookEndpoint & { secret: string };

export type WebhookEndpointInput = {
  url: string;
  description?: string;
  events: WebhookEventName[];
  is_active?: boolean;
};

export type WebhookDeliveryStatus =
  "pending" | "succeeded" | "failed" | "exhausted";

export type WebhookDelivery = {
  uuid: string;
  event_name: string;
  event_id: string;
  payload: Record<string, unknown>;
  status: WebhookDeliveryStatus;
  attempts: number;
  max_attempts: number;
  next_attempt_at?: string;
  last_status_code?: number;
  last_error?: string;
  last_attempt_at?: string;
  delivered_at?: string;
  created_at: string;
};

export type WebhookDeliveryPage = {
  items: WebhookDelivery[];
  total: number;
  limit: number;
  offset: number;
};

export const webhooksService = {
  events() {
    return platformRequest<{ items: WebhookEventDefinition[] }>(
      "GET",
      "/v1/webhooks/events",
    );
  },
  list() {
    return platformRequest<{ items: WebhookEndpoint[] }>("GET", "/v1/webhooks");
  },
  create(body: WebhookEndpointInput) {
    return platformRequest<WebhookEndpointWithSecret>("POST", "/v1/webhooks", {
      body,
    });
  },
  update(uuid: string, body: WebhookEndpointInput) {
    return platformRequest<WebhookEndpoint>("PUT", `/v1/webhooks/${uuid}`, {
      body,
    });
  },
  remove(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/webhooks/${uuid}`);
  },
  rotateSecret(uuid: string) {
    return platformRequest<WebhookEndpointWithSecret>(
      "POST",
      `/v1/webhooks/${uuid}/rotate-secret`,
    );
  },
  sendTest(uuid: string) {
    return platformRequest<WebhookDelivery>(
      "POST",
      `/v1/webhooks/${uuid}/test`,
    );
  },
  deliveries(
    uuid: string,
    params: { limit: number; offset: number; status?: WebhookDeliveryStatus },
  ) {
    return platformRequest<WebhookDeliveryPage>(
      "GET",
      `/v1/webhooks/${uuid}/deliveries`,
      {
        query: {
          limit: String(params.limit),
          offset: String(params.offset),
          status: params.status,
        },
      },
    );
  },
  resend(uuid: string, deliveryUuid: string) {
    return platformRequest<WebhookDelivery>(
      "POST",
      `/v1/webhooks/${uuid}/deliveries/${deliveryUuid}/resend`,
    );
  },
};
