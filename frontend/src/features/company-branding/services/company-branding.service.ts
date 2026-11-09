import type { components } from "@/generated/api";
import { platformFormRequest } from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type CompanyMedia = components["schemas"]["CompanyMedia"];
export type CompanyMediaKind = components["schemas"]["CompanyMediaKind"];
export type CompanyMediaUsage = components["schemas"]["CompanyMediaUsage"];
export type CompanyBranding = components["schemas"]["CompanyBranding"];
export type UpdateCompanyBrandingRequest =
  components["schemas"]["UpdateCompanyBrandingRequest"];

type Page<T> = { items: T[]; total: number; limit: number; offset: number };

export const MEDIA_KINDS: CompanyMediaKind[] = [
  "logo",
  "cover",
  "image",
  "document",
];

export const companyBrandingKeys = {
  all: ["company-branding"] as const,
  media: (kind: string) => ["company-branding", "media", kind] as const,
  branding: ["company-branding", "branding"] as const,
};

/**
 * Company media library and branding (TEC-946) of the active company account. Media URLs come
 * from the API; the browser never builds them from object keys.
 */
export const companyBrandingService = {
  listMedia(kind?: CompanyMediaKind) {
    return platformRequest<Page<CompanyMedia>>("GET", "/v1/company/media", {
      query: { limit: 100, ...(kind ? { kind } : {}) },
    });
  },
  uploadMedia(input: { file: File; kind: CompanyMediaKind; name?: string }) {
    const formData = new FormData();
    formData.append("file", input.file);
    formData.append("kind", input.kind);
    if (input.name) formData.append("name", input.name);
    return platformFormRequest<CompanyMedia>(
      "POST",
      "/v1/company/media",
      formData,
    );
  },
  deleteMedia(uuid: string) {
    return platformRequest<{ status: string }>(
      "DELETE",
      `/v1/company/media/${uuid}`,
    );
  },
  getBranding() {
    return platformRequest<CompanyBranding>("GET", "/v1/company/branding");
  },
  updateBranding(body: UpdateCompanyBrandingRequest) {
    return platformRequest<CompanyBranding>("PUT", "/v1/company/branding", {
      body,
    });
  },
};
