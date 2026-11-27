import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type BusinessPage = components["schemas"]["BusinessPage"];
export type BusinessPageBlock = components["schemas"]["BusinessPageBlock"];
export type BusinessBlockType = components["schemas"]["BusinessBlockType"];
export type BusinessButtonTarget =
  components["schemas"]["BusinessButtonTarget"];
export type BusinessPageText = components["schemas"]["BusinessPageText"];
export type BusinessBlockText = components["schemas"]["BusinessBlockText"];
export type SaveBusinessPageRequest =
  components["schemas"]["SaveBusinessPageRequest"];
export type PublicBusinessPage = components["schemas"]["PublicBusinessPage"];
export type PublicBusinessBlock = components["schemas"]["PublicBusinessBlock"];

export const businessPageKeys = {
  all: ["business-page"] as const,
  page: ["business-page", "page"] as const,
};

/** Business page of the active company account (TEC-1327). */
export const businessPageService = {
  get() {
    return platformRequest<BusinessPage>("GET", "/v1/business-page");
  },
  save(body: SaveBusinessPageRequest) {
    return platformRequest<BusinessPage>("PUT", "/v1/business-page", { body });
  },
  publish() {
    return platformRequest<BusinessPage>("POST", "/v1/business-page/publish");
  },
  unpublish() {
    return platformRequest<BusinessPage>("POST", "/v1/business-page/unpublish");
  },
};
