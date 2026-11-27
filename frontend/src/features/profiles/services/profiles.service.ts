import type { components } from "@/generated/api";
import type { ServerListParams } from "@/components/entity";
import type { ResourceMeta } from "@/features/io/types";
import {
  platformDownloadFile,
  platformFormRequest,
  triggerBrowserDownload,
} from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type AddressDTO = {
  uuid?: string;
  type?: string;
  label?: string;
  street?: string;
  extended?: string;
  city?: string;
  region?: string;
  postal_code?: string;
  country?: string;
  is_primary?: boolean;
  display_order?: number;
};

export type ContactDetailDTO = {
  uuid?: string;
  category: "phone" | "email" | "url" | "whatsapp" | "other";
  type?: string;
  label?: string;
  value: string;
  is_primary?: boolean;
  display_order?: number;
};

export type SocialLinkDTO = {
  uuid?: string;
  platform_id?: number | null;
  platform?: string;
  label?: string;
  url: string;
  icon_url?: string;
  display_order?: number;
  is_active?: boolean;
};

export type ProfileTranslationDTO = {
  id?: number;
  locale: "tr" | "en";
  prefix?: string;
  first_name: string;
  last_name?: string;
  middle_name?: string;
  suffix?: string;
  nickname?: string;
  company_name?: string;
  department?: string;
  title?: string;
  role?: string;
  bio?: string;
  note?: string;
  is_default?: boolean;
};

export type ProfileSummary = {
  uuid: string;
  first_name: string;
  last_name: string;
  primary_locale: "tr" | "en";
  title?: string;
  company_name?: string;
  avatar_url?: string;
  is_primary: boolean;
  completion_rate: number;
  /** Public path: comcode.../{username} */
  username?: string;
  /** Shared on the public page and in the profile swipe (TEC-1326); effective value. */
  is_open?: boolean;
  /** The company keeps employee profiles open; the switch cannot close it. */
  open_locked?: boolean;
  /** The caller may change the switch (owner/admin, or their own employee profile). */
  can_toggle_open?: boolean;
  created_at: string;
  updated_at: string;
};

export type ProfileOpenState = components["schemas"]["ProfileOpenState"];

export type ProfileDetail = {
  id?: number;
  uuid: string;
  user_uuid?: string;
  primary_locale: "tr" | "en";
  birthday?: string | null;
  gender?: string;
  avatar_url?: string;
  cover_url?: string;
  is_primary: boolean;
  completion_rate: number;
  /** What is missing for 100%, highest gain first (server-computed, TEC-549). */
  completion_suggestions?: ProfileCompletionSuggestion[];
  verification_status?: ProfileVerificationStatus;
  translations: ProfileTranslationDTO[];
  addresses: AddressDTO[];
  contact_details: ContactDetailDTO[];
  social_links: SocialLinkDTO[];
  username?: string;
  created_at: string;
  updated_at: string;
};

export type PublicPageSettings = components["schemas"]["PublicPageSettings"];
export type ProfileCompletionSuggestion =
  components["schemas"]["ProfileCompletionSuggestion"];
export type ProfileVerificationStatus =
  components["schemas"]["ProfileVerificationStatus"];
export type ProfileVerification = components["schemas"]["ProfileVerification"];
export type PublicModule = components["schemas"]["PublicModule"];
export type UpdatePublicPagePayload =
  components["schemas"]["UpdatePublicPageRequest"];

export type UpsertProfilePayload = {
  primary_locale: "tr" | "en";
  birthday?: string | null;
  gender?: string;
  avatar_url?: string;
  cover_url?: string;
  is_primary?: boolean;
  translations: ProfileTranslationDTO[];
  addresses?: AddressDTO[];
  contact_details?: ContactDetailDTO[];
  social_links?: SocialLinkDTO[];
};

export type ProfileListResult = {
  items: ProfileSummary[];
  total: number;
  limit: number;
  offset: number;
};

export type LinkMetadata = {
  title: string;
  icon_url: string;
  description: string;
};

export type ParsedVCardProfile = {
  display_name?: string;
  prefix?: string;
  first_name?: string;
  last_name?: string;
  middle_name?: string;
  suffix?: string;
  nickname?: string;
  company_name?: string;
  department?: string;
  title?: string;
  role?: string;
  bio?: string;
  note?: string;
  birthday?: string | null;
  gender?: string;
  avatar_data_url?: string;
  avatar_url?: string;
  contact_details?: ContactDetailDTO[];
  addresses?: AddressDTO[];
  social_links?: SocialLinkDTO[];
};

export const profilesService = {
  async list(params: ServerListParams) {
    return platformRequest<ProfileListResult>("GET", "/v1/profiles", {
      query: {
        limit: params.limit,
        offset: params.offset,
        q: params.q,
      },
    });
  },

  async meta() {
    return platformRequest<ResourceMeta>("GET", "/v1/profiles/meta");
  },

  async get(uuid: string) {
    return platformRequest<ProfileDetail>("GET", `/v1/profiles/${uuid}`);
  },

  /** Username, visibility, visible modules and published bank accounts. */
  async getPublicPage(uuid: string) {
    return platformRequest<PublicPageSettings>(
      "GET",
      `/v1/profiles/${uuid}/public-page`,
    );
  },

  async updatePublicPage(uuid: string, body: UpdatePublicPagePayload) {
    return platformRequest<PublicPageSettings>(
      "PUT",
      `/v1/profiles/${uuid}/public-page`,
      { body },
    );
  },

  /** Verified badge status and the latest request (TEC-549). */
  async getVerification(uuid: string) {
    return platformRequest<ProfileVerification>(
      "GET",
      `/v1/profiles/${uuid}/verification`,
    );
  },

  async requestVerification(uuid: string, note: string) {
    return platformRequest<ProfileVerification>(
      "POST",
      `/v1/profiles/${uuid}/verification`,
      {
        body: { note },
      },
    );
  },

  async cancelVerification(uuid: string) {
    return platformRequest<ProfileVerification>(
      "DELETE",
      `/v1/profiles/${uuid}/verification`,
    );
  },

  async create(body: UpsertProfilePayload) {
    return platformRequest<ProfileDetail>("POST", "/v1/profiles", { body });
  },

  async update(uuid: string, body: UpsertProfilePayload) {
    return platformRequest<ProfileDetail>("PUT", `/v1/profiles/${uuid}`, {
      body,
    });
  },

  async remove(uuid: string) {
    return platformRequest<{ deleted: boolean }>(
      "DELETE",
      `/v1/profiles/${uuid}`,
    );
  },

  /** Open switch of a profile (TEC-1326). */
  async getOpen(uuid: string) {
    return platformRequest<ProfileOpenState>(
      "GET",
      `/v1/profiles/${uuid}/open`,
    );
  },

  async setOpen(uuid: string, isOpen: boolean) {
    return platformRequest<ProfileOpenState>(
      "PUT",
      `/v1/profiles/${uuid}/open`,
      { body: { is_open: isOpen } },
    );
  },

  async setPrimary(uuid: string) {
    return platformRequest<{ primary: boolean }>(
      "POST",
      `/v1/profiles/${uuid}/primary`,
    );
  },

  async uploadAvatar(uuid: string, file: File) {
    const formData = new FormData();
    formData.append("avatar", file);
    return platformFormRequest<{ avatar_url: string }>(
      "POST",
      `/v1/profiles/${uuid}/avatar`,
      formData,
    );
  },

  async uploadCover(uuid: string, file: File) {
    const formData = new FormData();
    formData.append("cover", file);
    return platformFormRequest<{ cover_url: string }>(
      "POST",
      `/v1/profiles/${uuid}/cover`,
      formData,
    );
  },

  async fetchLinkMetadata(url: string) {
    return platformRequest<LinkMetadata>("POST", "/v1/utils/link-metadata", {
      body: { url },
    });
  },

  async downloadVCard(
    uuid: string,
    fallbackName = "contact",
    locale?: string,
    version: "4.0" = "4.0",
  ) {
    const { blob, filename } = await platformDownloadFile(
      `/v1/profiles/${uuid}/vcard`,
      { version, locale: locale || "" },
    );
    triggerBrowserDownload(blob, filename ?? `${fallbackName}.vcf`);
  },

  async parseVCard(file: File) {
    const formData = new FormData();
    formData.append("file", file);
    return platformFormRequest<ParsedVCardProfile>(
      "POST",
      "/v1/profiles/vcard/parse",
      formData,
    );
  },

  async parseVCardText(vcardText: string) {
    return platformRequest<ParsedVCardProfile>(
      "POST",
      "/v1/profiles/vcard/parse",
      {
        body: { vcard: vcardText },
      },
    );
  },
};

/** A company the user belongs to and whether a personal profile is shared with it (TEC-1315). */
export type ProfileCompanyShareTarget = {
  company_uuid: string;
  company_name: string;
  role: string;
  shared: boolean;
};

/** A member's personal profile shared with the active company; read-only (TEC-1315). */
export type CompanySharedProfile = {
  uuid: string;
  first_name: string;
  last_name: string;
  title: string;
  company_name: string;
  avatar_url: string;
  completion_rate: number;
  username: string;
  shared_at: string;
  member: { user_uuid: string; email: string; name: string; surname: string };
};

export const profileSharesService = {
  async targets(profileUuid: string) {
    return platformRequest<{ items: ProfileCompanyShareTarget[] }>(
      "GET",
      `/v1/profiles/${profileUuid}/company-shares`,
    );
  },

  async setShared(profileUuid: string, companyUuid: string, shared: boolean) {
    return platformRequest<{ shared: boolean }>(
      shared ? "PUT" : "DELETE",
      `/v1/profiles/${profileUuid}/company-shares/${companyUuid}`,
    );
  },

  async companyProfiles(params: { limit: number; offset: number }) {
    return platformRequest<{
      items: CompanySharedProfile[];
      total: number;
      limit: number;
      offset: number;
    }>("GET", "/v1/company/shared-profiles", { query: params });
  },
};
