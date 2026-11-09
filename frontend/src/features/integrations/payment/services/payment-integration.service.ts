import { platformRequest } from "@/lib/api/platform-request";

export type PaymentSettings = {
  active_provider: "none" | "paytr" | "iyzico";
  test_mode: boolean;
};

export type PayTRSettings = {
  enabled: boolean;
  merchant_id: string;
  merchant_key_configured: boolean;
  merchant_salt_configured: boolean;
};

export type IyzicoSettings = {
  enabled: boolean;
  base_url: string;
  api_key_configured: boolean;
  secret_key_configured: boolean;
};

export const paymentIntegrationService = {
  getSettings() {
    return platformRequest<PaymentSettings>(
      "GET",
      "/v1/platform/payment/settings",
    );
  },
  patchSettings(body: Partial<PaymentSettings>) {
    return platformRequest<PaymentSettings>(
      "PATCH",
      "/v1/platform/payment/settings",
      {
        body,
      },
    );
  },
  getPayTR() {
    return platformRequest<PayTRSettings>(
      "GET",
      "/v1/platform/payment/providers/paytr",
    );
  },
  patchPayTR(body: {
    enabled?: boolean;
    merchant_id?: string;
    merchant_key?: string;
    merchant_salt?: string;
  }) {
    return platformRequest<PayTRSettings>(
      "PATCH",
      "/v1/platform/payment/providers/paytr",
      { body },
    );
  },
  getIyzico() {
    return platformRequest<IyzicoSettings>(
      "GET",
      "/v1/platform/payment/providers/iyzico",
    );
  },
  patchIyzico(body: {
    enabled?: boolean;
    base_url?: string;
    api_key?: string;
    secret_key?: string;
  }) {
    return platformRequest<IyzicoSettings>(
      "PATCH",
      "/v1/platform/payment/providers/iyzico",
      { body },
    );
  },
};
