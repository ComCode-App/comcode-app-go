import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import type { components } from "@/generated/api";
import { ApiError } from "@/lib/api/errors";
import { platformRequest } from "@/lib/api/platform-request";

/** Provider refunds of online order payments (TEC-935). */
export type OrderRefund = components["schemas"]["OrderRefund"];
export type OrderRefunds = components["schemas"]["OrderRefunds"];
export type OrderRefundablePayment = components["schemas"]["OrderRefundablePayment"];
export type RefundOrderPayload = components["schemas"]["RefundOrderInput"];

export type OrderStatus =
  | "pending"
  | "paid"
  | "design_pending"
  | "design_approved"
  | "in_production"
  | "ready_to_ship"
  | "shipped"
  | "delivered"
  | "cancelled"
  | "refunded";

export type OrderType = "standard" | "bulk" | "corporate";
export type PaymentStatus = "unpaid" | "pending" | "paid" | "failed" | "partially_refunded" | "refunded";
export type FulfillmentStatus = "unfulfilled" | "pending" | "processing" | "in_production" | "ready_to_ship" | "shipped" | "delivered" | "cancelled";
export type ProvisioningStatus = "unprovisioned" | "provisioning" | "provisioned" | "failed";

export type OrderItemSummary = {
  id?: number;
  uuid: string;
  product_id?: number;
  variant_id?: number;
  product_slug?: string;
  product_name: string;
  variant_name: string;
  sku: string;
  quantity: number;
  unit_price: number;
  total_price: number;
  custom_print_data?: Record<string, unknown>;
  product_images?: string[];
  recipient_name?: string;
  recipient_email?: string;
  recipient_phone?: string;
  recipient_title?: string;
  recipient_department?: string;
  profile_uuid?: string;
  profile_avatar_url?: string;
  card_uuid?: string;
  card_smart_code?: string;
  card_uid?: string;
  card_status?: string;
  card_is_active?: boolean;
  card_is_locked?: boolean;
  route_path?: string;
  route_status?: string;
  fulfillment_status?: string;
  provisioning_status?: ProvisioningStatus | string;
  provisioning_error?: string;
};

export type AssetRequestSummary = {
  id?: number;
  uuid: string;
  order_id: number;
  request_type: string;
  title: string;
  description: string;
  status: "requested" | "uploaded" | "approved" | "rejected";
  token: string;
  uploaded_file_url: string;
  rejection_reason: string;
  requested_by?: number;
  created_at: string;
  updated_at: string;
};

export type Address = {
  full_name?: string;
  company_name?: string;
  phone?: string;
  tax_id?: string;
  tax_office?: string;
  address_line1?: string;
  address_line2?: string;
  city?: string;
  state?: string;
  postal_code?: string;
  country?: string;
};

export type OrderSummary = {
  id?: number;
  uuid: string;
  order_number: string;
  order_type?: OrderType | string;
  user_id: number;
  user_name?: string;
  user_email?: string;
  company_id?: number;
  company_name?: string;
  status: OrderStatus;
  fulfillment_status?: FulfillmentStatus | string;
  subtotal: number;
  tax_total: number;
  shipping_total: number;
  discount_total: number;
  grand_total: number;
  currency: string;
  payment_status: PaymentStatus | string;
  payment_provider: string;
  shipping_carrier: string;
  tracking_number: string;
  tracking_url: string;
  items_count: number;
  created_at: string;
  updated_at: string;
};

export type OrderDetail = OrderSummary & {
  payment_reference: string;
  shipping_address?: Address;
  billing_address?: Address;
  customer_note: string;
  admin_note: string;
  metadata?: Record<string, unknown>;
  paid_at?: string;
  shipped_at?: string;
  delivered_at?: string;
  cancelled_at?: string;
  items: OrderItemSummary[];
  asset_requests: AssetRequestSummary[];
};

export type OrderListResult = {
  items: OrderSummary[];
  total: number;
  limit: number;
  offset: number;
};

export type OrderOverviewKPIs = {
  total_orders: number;
  today_orders: number;
  pending_orders?: number;
  design_pending_orders?: number;
  in_production_orders?: number;
  shipped_orders?: number;
  pending_design_count?: number;
  in_production_count?: number;
  ready_to_ship_count?: number;
  total_revenue: number;
};

export type CreateOrderItemPayload = {
  product_uuid?: string;
  variant_uuid?: string;
  product_name?: string;
  quantity: number;
  unit_price?: number;
  custom_print_data?: Record<string, unknown>;
  recipient_name?: string;
  recipient_email?: string;
  recipient_phone?: string;
  recipient_title?: string;
  recipient_department?: string;
  profile_uuid?: string;
  idempotency_key?: string;
};

export type CreateOrderPayload = {
  company_id?: number;
  company_uuid?: string;
  user_uuid?: string;
  order_type?: OrderType | string;
  items: CreateOrderItemPayload[];
  shipping_address: Address;
  billing_address?: Address;
  customer_note?: string;
  admin_note?: string;
  payment_provider?: string;
  currency?: string;
  metadata?: Record<string, unknown>;
};

export type UpdateShippingPayload = {
  shipping_carrier: string;
  tracking_number: string;
  tracking_url?: string;
  mark_shipped?: boolean;
};

export type CreateAssetRequestPayload = {
  request_type: string;
  title: string;
  description?: string;
};

export type ReviewAssetRequestPayload = {
  status: "approved" | "rejected";
  rejection_reason?: string;
};

/** Online payment attempt of an order (TEC-943). */
export type OrderCheckoutSession = {
  uuid: string;
  status: "pending" | "completed" | "failed" | "expired";
  provider: "paytr" | "iyzico";
  order_uuid: string;
  order_number: string;
  order_status: OrderStatus | string;
  order_payment_status: PaymentStatus | string;
  amount_minor: number;
  currency: string;
  iframe_token?: string;
  checkout_url?: string;
  expires_at: string;
  completed_at?: string;
};

/** Whether the customer can start an online payment for the order. */
export function isOrderPayable(order: { status: string; payment_status: string }) {
  return (
    order.status === "pending" &&
    (order.payment_status === "unpaid" || order.payment_status === "failed")
  );
}

export const ordersService = {
  async list(params: ServerListParams & { status?: string }) {
    return platformRequest<OrderListResult>(
      "GET",
      "/v1/platform/orders",
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

  async kpis() {
    return platformRequest<OrderOverviewKPIs>(
      "GET",
      "/v1/platform/orders/kpis",
    );
  },

  async get(uuid: string) {
    return platformRequest<OrderDetail>(
      "GET",
      `/v1/platform/orders/${uuid}`,
    );
  },

  async create(payload: CreateOrderPayload) {
    return platformRequest<OrderDetail>(
      "POST",
      "/v1/platform/orders",
      { body: payload },
    );
  },

  async createBulk(payload: CreateOrderPayload) {
    return platformRequest<OrderDetail>(
      "POST",
      "/v1/platform/orders/bulk",
      { body: payload },
    );
  },

  async provisionOrder(uuid: string) {
    return platformRequest<OrderDetail>(
      "POST",
      `/v1/platform/orders/${uuid}/provision`,
    );
  },

  async provisionItem(orderUuid: string, itemUuid: string) {
    return platformRequest<OrderDetail>(
      "POST",
      `/v1/platform/orders/${orderUuid}/items/${itemUuid}/provision`,
    );
  },

  async bulkUpdateItems(orderUuid: string, itemUuids: string[], fulfillmentStatus: string) {
    return platformRequest<OrderDetail>(
      "PUT",
      `/v1/platform/orders/${orderUuid}/items/bulk`,
      { body: { item_uuids: itemUuids, fulfillment_status: fulfillmentStatus } },
    );
  },

  async updatePaymentStatus(uuid: string, paymentStatus: string, paymentReference?: string) {
    return platformRequest<OrderDetail>(
      "PUT",
      `/v1/platform/orders/${uuid}/payment`,
      { body: { payment_status: paymentStatus, payment_reference: paymentReference } },
    );
  },

  async listRefunds(uuid: string) {
    return platformRequest<OrderRefunds>("GET", `/v1/platform/orders/${uuid}/refunds`);
  },

  // Refunds need a step-up grant; platformRequest asks for it and repeats the request with the
  // same idempotency key. Errors are shown by the refunds panel.
  async refund(uuid: string, payload: RefundOrderPayload) {
    return platformRequest<OrderRefund>("POST", `/v1/platform/orders/${uuid}/refunds`, {
      body: payload,
      silent: true,
    });
  },

  async retryRefund(uuid: string, refundUuid: string) {
    return platformRequest<OrderRefund>(
      "POST",
      `/v1/platform/orders/${uuid}/refunds/${refundUuid}/retry`,
      { silent: true },
    );
  },

  async updateFulfillmentStatus(uuid: string, fulfillmentStatus: string) {
    return platformRequest<OrderDetail>(
      "PUT",
      `/v1/platform/orders/${uuid}/fulfillment`,
      { body: { fulfillment_status: fulfillmentStatus } },
    );
  },

  async updateStatus(uuid: string, status: OrderStatus, note?: string) {
    return platformRequest<OrderDetail>(
      "PUT",
      `/v1/platform/orders/${uuid}/status`,
      { body: { status, note } },
    );
  },

  async updateShipping(uuid: string, payload: UpdateShippingPayload) {
    return platformRequest<OrderDetail>(
      "PUT",
      `/v1/platform/orders/${uuid}/shipping`,
      { body: payload },
    );
  },

  async updateNotes(uuid: string, adminNote: string) {
    return platformRequest<OrderDetail>(
      "PUT",
      `/v1/platform/orders/${uuid}/notes`,
      { body: { admin_note: adminNote } },
    );
  },

  async delete(uuid: string) {
    return platformRequest<{ success: boolean }>(
      "DELETE",
      `/v1/platform/orders/${uuid}`,
    );
  },

  async createAssetRequest(orderUuid: string, payload: CreateAssetRequestPayload) {
    return platformRequest<AssetRequestSummary>(
      "POST",
      `/v1/platform/orders/${orderUuid}/assets`,
      { body: payload },
    );
  },

  async reviewAssetRequest(orderUuid: string, assetUuid: string, payload: ReviewAssetRequestPayload) {
    return platformRequest<AssetRequestSummary>(
      "PUT",
      `/v1/platform/orders/${orderUuid}/assets/${assetUuid}/review`,
      { body: payload },
    );
  },

  async listMyOrders(params?: { limit?: number; offset?: number; q?: string }) {
    return platformRequest<OrderListResult>(
      "GET",
      "/v1/orders",
      {
        query: {
          limit: params?.limit ?? 20,
          offset: params?.offset ?? 0,
          q: params?.q,
        },
      },
    );
  },

  async getMyOrder(uuid: string) {
    return platformRequest<OrderDetail>(
      "GET",
      `/v1/orders/${uuid}`,
    );
  },

  async createMyOrder(payload: CreateOrderPayload) {
    return platformRequest<OrderDetail>(
      "POST",
      "/v1/orders",
      { body: payload },
    );
  },

  async startCheckout(orderUuid: string) {
    return platformRequest<OrderCheckoutSession>("POST", `/v1/orders/${orderUuid}/checkout`);
  },

  async getCheckout(sessionUuid: string) {
    return platformRequest<OrderCheckoutSession>("GET", `/v1/orders/checkout/${sessionUuid}`);
  },

  async uploadAsset(token: string, file: File) {
    const formData = new FormData();
    formData.append("token", token);
    formData.append("file", file);

    const res = await fetch("/api/v1/orders/assets/upload", {
      method: "POST",
      body: formData,
    });
    const json = await res.json();
    if (!res.ok || json.error) {
      // Keep the API error code (e.g. SVG_UNSAFE_CONTENT, PAYLOAD_TOO_LARGE) for the UI.
      throw new ApiError({
        status: res.status,
        code: json.error?.code ?? "",
        message: json.error?.message || "Failed to upload file",
        details: json.error?.details,
      });
    }
    return json.data as AssetRequestSummary;
  },

  async meta() {
    return platformRequest<ResourceMeta>(
      "GET",
      "/v1/platform/orders/meta",
    );
  },
};
