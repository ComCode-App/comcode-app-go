import type { components } from "@/generated/api";
import type { ResourceMeta } from "@/features/io/types";
import { platformRequest } from "@/lib/api/platform-request";

export type CardSummary = {
  /** The card's c/{code} route: card scans are analytics events of this route. */
  route_uuid?: string;
  uuid: string;
  name: string;
  card_uid?: string;
  smart_code: string;
  target_type: "profile" | "direct_url" | "vcard_download" | "social_hub" | string;
  profile_uuid?: string;
  profile_avatar_url?: string;
  profile_locale?: string;
  direct_url?: string;
  custom_data?: Record<string, unknown>;
  card_type: "physical_nfc" | "metal_card" | "wood_card" | "pvc_card" | "digital_only" | string;
  material: string;
  color_hex: string;
  is_active: boolean;
  is_locked: boolean;
  lead_capture_enabled: boolean;
  has_passcode?: boolean;
  created_at: string;
  updated_at: string;
};

export type CardDetail = {
  uuid: string;
  name: string;
  card_uid?: string;
  activation_code?: string;
  smart_code: string;
  target_type: "profile" | "direct_url" | "vcard_download" | "social_hub" | string;
  profile_uuid?: string;
  profile_avatar_url?: string;
  profile_locale?: string;
  direct_url?: string;
  custom_data?: Record<string, unknown>;
  card_type: "physical_nfc" | "metal_card" | "wood_card" | "pvc_card" | "digital_only" | string;
  material: string;
  color_hex: string;
  is_active: boolean;
  is_locked: boolean;
  lead_capture_enabled: boolean;
  has_passcode?: boolean;
  created_at: string;
  updated_at: string;
};

export type UpsertCardPayload = {
  name: string;
  card_uid?: string;
  target_type: string;
  profile_uuid?: string | null;
  direct_url?: string;
  custom_data?: Record<string, unknown>;
  card_type?: string;
  material?: string;
  color_hex?: string;
  is_active?: boolean;
  is_locked?: boolean;
  lead_capture_enabled?: boolean;
  passcode?: string;
};

export type ActivateCardPayload = {
  activation_code: string;
  name?: string;
  profile_uuid?: string;
};

/** GET /v1/c/{code} — the route resolver's answer (see RouteResolution in OpenAPI). */
export type SmartLinkResolution = components["schemas"]["RouteResolution"];

export type CardListResult = {
  items: CardSummary[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
};

export type ListCardsParams = {
  page?: number;
  pageSize?: number;
  search?: string;
  is_active?: boolean;
};

export const cardsService = {
  async list(params?: ListCardsParams): Promise<CardListResult> {
    const query = new URLSearchParams();
    if (params?.page) query.set("offset", String((params.page - 1) * (params.pageSize || 20)));
    if (params?.pageSize) query.set("limit", String(params.pageSize));
    if (params?.search) query.set("q", params.search);
    if (params?.is_active !== undefined) query.set("is_active", String(params.is_active));

    return platformRequest<CardListResult>(
      "GET",
      `/v1/cards?${query.toString()}`,
    );
  },

  async get(uuid: string): Promise<CardDetail> {
    const res = await platformRequest<{ data: CardDetail }>("GET", `/v1/cards/${uuid}`);
    return res.data;
  },

  async create(body: UpsertCardPayload): Promise<CardDetail> {
    const res = await platformRequest<{ data: CardDetail }>("POST", "/v1/cards", { body });
    return res.data;
  },

  async update(uuid: string, body: UpsertCardPayload): Promise<CardDetail> {
    const res = await platformRequest<{ data: CardDetail }>("PUT", `/v1/cards/${uuid}`, { body });
    return res.data;
  },

  async toggleLock(uuid: string): Promise<{ is_locked: boolean }> {
    const res = await platformRequest<{ data: { is_locked: boolean } }>(
      "PATCH",
      `/v1/cards/${uuid}/toggle-lock`,
    );
    return res.data;
  },

  async activate(body: ActivateCardPayload): Promise<CardDetail> {
    const res = await platformRequest<{ data: CardDetail }>("POST", "/v1/cards/activate", { body });
    return res.data;
  },

  async delete(uuid: string): Promise<{ success: boolean }> {
    return platformRequest<{ success: boolean }>("DELETE", `/v1/cards/${uuid}`);
  },

  async meta(): Promise<ResourceMeta> {
    return platformRequest<ResourceMeta>("GET", "/v1/cards/meta");
  },

  async resolveSmartLink(code: string): Promise<SmartLinkResolution> {
    // platformRequest already unwraps the {success, data} envelope.
    return platformRequest<SmartLinkResolution>("GET", `/v1/c/${encodeURIComponent(code)}`);
  },
};
