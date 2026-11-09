import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import { platformFormRequest } from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type BankSummary = {
  id?: number;
  uuid: string;
  name: string;
  slug: string;
  code: string;
  country_code: string;
  bic_swift: string;
  icon_url: string;
  brand_color: string;
  iban_pattern: string;
  description: string;
  display_order: number;
  is_active: boolean;
};

export type BankDetail = BankSummary & {
  created_at: string;
  updated_at: string;
};

export type BankListResult = {
  items: BankSummary[];
  total: number;
  limit: number;
  offset: number;
};

export type UpsertBankPayload = {
  name: string;
  slug?: string;
  code: string;
  country_code?: string;
  bic_swift?: string;
  icon_url?: string;
  brand_color?: string;
  iban_pattern?: string;
  description?: string;
  display_order: number;
  is_active: boolean;
};

export const banksService = {
  async list(params: ServerListParams) {
    return platformRequest<BankListResult>("GET", "/v1/platform/banks", {
      query: {
        limit: params.limit,
        offset: params.offset,
        q: params.q,
      },
    });
  },

  async listActive() {
    const res = await platformRequest<{ data: BankSummary[] }>(
      "GET",
      "/v1/banks",
    );
    return res.data ?? [];
  },

  async meta() {
    return platformRequest<ResourceMeta>("GET", "/v1/platform/banks/meta");
  },

  async get(uuid: string) {
    return platformRequest<BankDetail>("GET", `/v1/platform/banks/${uuid}`);
  },

  async create(body: UpsertBankPayload) {
    return platformRequest<BankDetail>("POST", "/v1/platform/banks", { body });
  },

  async update(uuid: string, body: UpsertBankPayload) {
    return platformRequest<BankDetail>("PUT", `/v1/platform/banks/${uuid}`, {
      body,
    });
  },

  async uploadIcon(uuid: string, file: File) {
    const formData = new FormData();
    formData.append("icon", file);
    return platformFormRequest<BankDetail>(
      "POST",
      `/v1/platform/banks/${uuid}/icon`,
      formData,
    );
  },

  async remove(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/platform/banks/${uuid}`);
  },
};
