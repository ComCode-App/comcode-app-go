import { platformRequest } from "@/lib/api/platform-request";
import {
  platformDownloadFile,
  triggerBrowserDownload,
} from "@/lib/api/platform-form-request";

export type BillingPlan = {
  uuid: string;
  slug: string;
  name: string;
  description?: string | null;
  billing_interval: string;
  price_minor: number;
  currency: string;
  trial_days: number;
  limits: Array<{
    feature_key: string;
    value_type: string;
    int_value?: number;
    bool_value?: boolean;
  }>;
};

export type SavedCard = {
  uuid: string;
  provider: string;
  last4: string;
  brand: string;
  exp_month?: number;
  exp_year?: number;
  is_default: boolean;
  created_at: string;
};

export type UserSubscription = {
  uuid: string;
  status: string;
  plan_uuid: string;
  plan_slug: string;
  plan_name: string;
  billing_interval: string;
  price_minor: number;
  currency: string;
  provider?: string;
  saved_card_uuid?: string;
  starts_at: string;
  next_billing_at?: string;
  canceled_at?: string;
  cancel_at_period_end: boolean;
  period_end?: string;
  grace_until?: string;
  pending_plan_uuid?: string;
};

export type CheckoutSession = {
  uuid: string;
  status: string;
  provider: string;
  plan_uuid: string;
  plan_name: string;
  amount_minor: number;
  currency: string;
  iframe_token?: string;
  checkout_url?: string;
  provider_token?: string;
  expires_at: string;
  completed_at?: string;
};

export type CardSession = {
  uuid: string;
  status: string;
  provider: string;
  iframe_token?: string;
  checkout_url?: string;
  provider_token?: string;
  expires_at: string;
};

export type BillingPaymentKind =
  | "subscription_checkout"
  | "subscription_renewal"
  | "subscription_upgrade"
  | "order"
  | "order_manual";

export type BillingPayment = {
  uuid: string;
  kind: BillingPaymentKind;
  provider: string;
  provider_ref?: string;
  amount_minor: number;
  currency: string;
  paid_at: string;
  plan_name?: string;
  billing_interval?: string;
  order_number?: string;
  order_uuid?: string;
  receipt_number: string;
};

export type BillingPaymentPage = {
  items: BillingPayment[];
  total: number;
  limit: number;
  offset: number;
};

export const billingService = {
  /** Payment history of the active account (TEC-590). */
  listPayments(params: { limit: number; offset: number }) {
    return platformRequest<BillingPaymentPage>("GET", "/v1/billing/payments", {
      query: { limit: String(params.limit), offset: String(params.offset) },
    });
  },
  /** Downloads the PDF receipt of a payment and hands it to the browser. */
  async downloadReceipt(payment: BillingPayment, locale: string) {
    const { blob, filename } = await platformDownloadFile(
      `/v1/billing/payments/${payment.uuid}/receipt`,
      { locale },
    );
    triggerBrowserDownload(blob, filename ?? `${payment.receipt_number}.pdf`);
  },
  listPlans() {
    return platformRequest<{ items: BillingPlan[] }>(
      "GET",
      "/v1/billing/plans",
    );
  },

  getSubscription() {
    return platformRequest<{ subscription: UserSubscription | null }>(
      "GET",
      "/v1/billing/subscription",
    );
  },

  cancelSubscription() {
    return platformRequest<void>("DELETE", "/v1/billing/subscription");
  },

  resumeSubscription() {
    return platformRequest<void>("POST", "/v1/billing/subscription/resume");
  },

  changePlan(planUuid: string) {
    return platformRequest<void>("PATCH", "/v1/billing/subscription/plan", {
      body: { plan_uuid: planUuid },
    });
  },

  listCards() {
    return platformRequest<{ items: SavedCard[] }>("GET", "/v1/billing/cards");
  },

  startAddCard() {
    return platformRequest<{ session: CardSession }>(
      "POST",
      "/v1/billing/cards",
    );
  },

  getAddCardSession(uuid: string) {
    return platformRequest<CardSession>("GET", `/v1/billing/cards/add/${uuid}`);
  },

  deleteCard(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/billing/cards/${uuid}`);
  },

  setDefaultCard(uuid: string) {
    return platformRequest<SavedCard>(
      "PATCH",
      `/v1/billing/cards/${uuid}/default`,
    );
  },

  subscribe(planUuid: string, savedCardUuid: string) {
    return platformRequest<{ subscription: UserSubscription | null }>(
      "POST",
      "/v1/billing/subscribe",
      { body: { plan_uuid: planUuid, saved_card_uuid: savedCardUuid } },
    );
  },

  startCheckout(planUuid: string) {
    return platformRequest<{ session: CheckoutSession }>(
      "POST",
      "/v1/billing/checkout",
      { body: { plan_uuid: planUuid } },
    );
  },

  getCheckout(uuid: string) {
    return platformRequest<CheckoutSession>(
      "GET",
      `/v1/billing/checkout/${uuid}`,
    );
  },
};
