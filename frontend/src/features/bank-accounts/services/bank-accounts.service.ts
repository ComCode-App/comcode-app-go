import { platformRequest } from "@/lib/api/platform-request";

export type UserBankAccountSummary = {
  uuid: string;
  bank_uuid?: string;
  bank_name?: string;
  bank_slug?: string;
  bank_code?: string;
  bank_icon_url?: string;
  bank_brand_color?: string;
  bank_bic_swift?: string;
  account_holder: string;
  iban: string;
  formatted_iban: string;
  account_number: string;
  branch_code: string;
  currency: string;
  title: string;
  is_primary: boolean;
  is_active: boolean;
  display_order: number;
  created_at: string;
  updated_at: string;
};

export type CreateBankAccountPayload = {
  account_holder: string;
  iban: string;
  bank_uuid?: string;
  account_number?: string;
  branch_code?: string;
  currency?: string;
  title?: string;
  is_primary?: boolean;
  is_active?: boolean;
};

export type UpdateBankAccountPayload = {
  account_holder?: string;
  iban?: string;
  bank_uuid?: string;
  account_number?: string;
  branch_code?: string;
  currency?: string;
  title?: string;
  is_primary?: boolean;
  is_active?: boolean;
  display_order?: number;
};

export const bankAccountsService = {
  async list() {
    const res = await platformRequest<{ data: UserBankAccountSummary[] }>(
      "GET",
      "/v1/bank-accounts",
    );
    return res.data ?? [];
  },

  async get(uuid: string) {
    return platformRequest<UserBankAccountSummary>(
      "GET",
      `/v1/bank-accounts/${uuid}`,
    );
  },

  async create(body: CreateBankAccountPayload) {
    return platformRequest<UserBankAccountSummary>(
      "POST",
      "/v1/bank-accounts",
      { body },
    );
  },

  async update(uuid: string, body: UpdateBankAccountPayload) {
    return platformRequest<UserBankAccountSummary>(
      "PUT",
      `/v1/bank-accounts/${uuid}`,
      { body },
    );
  },

  async toggleActive(uuid: string) {
    return platformRequest<UserBankAccountSummary>(
      "PATCH",
      `/v1/bank-accounts/${uuid}/toggle-active`,
    );
  },

  async reorder(uuids: string[]) {
    return platformRequest<void>("POST", "/v1/bank-accounts/reorder", {
      body: { uuids },
    });
  },

  async remove(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/bank-accounts/${uuid}`);
  },
};
