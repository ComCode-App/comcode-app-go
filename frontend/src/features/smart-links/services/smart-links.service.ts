import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type SmartLink = components["schemas"]["RouteSummary"];
export type CreateSmartLinkPayload = components["schemas"]["CreateSmartLinkInput"];
export type UpdateSmartLinkPayload = components["schemas"]["UpdateRouteInput"];
export type SmartLinkDetail = components["schemas"]["RouteDetail"];
export type SmartLinkPresentation = components["schemas"]["SmartLinkPresentationConfig"];
export type SmartLinkPrimaryAction = components["schemas"]["SmartLinkPrimaryAction"];

/** Target types a smart link can point at. */
export type SmartLinkTarget = "profile" | "direct_url" | "vcard_download" | "social_hub";

/** Targets that open the profile page, where the link's presentation and lead form apply. */
export const PROFILE_PAGE_TARGETS: SmartLinkTarget[] = ["profile", "social_hub"];

/** Customer smart links (/s/{slug}) are routes of type custom_smart_link (/v1/routes). */
export const smartLinksService = {
  async list() {
    const res = await platformRequest<{ items: SmartLink[] }>("GET", "/v1/routes", {
      query: { route_type: "custom_smart_link", limit: 100 },
    });
    return res.items ?? [];
  },
  async get(uuid: string) {
    return platformRequest<SmartLinkDetail>("GET", `/v1/routes/${uuid}`);
  },
  async create(body: CreateSmartLinkPayload) {
    return platformRequest<SmartLink>("POST", "/v1/routes/smart-links", { body });
  },
  async update(uuid: string, body: UpdateSmartLinkPayload) {
    return platformRequest<SmartLink>("PUT", `/v1/routes/${uuid}`, { body });
  },
  async toggleLock(uuid: string) {
    return platformRequest<SmartLink>("PATCH", `/v1/routes/${uuid}/toggle-lock`);
  },
  async remove(uuid: string) {
    return platformRequest<{ status: string }>("DELETE", `/v1/routes/${uuid}`);
  },
};

export const smartLinkKeys = {
  list: ["smart-links", "list"] as const,
  detail: (uuid: string) => ["smart-links", "detail", uuid] as const,
};

/** Public URL of a smart link path ("s/promo" -> https://host/s/promo). */
export function smartLinkUrl(path: string): string {
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  return `${origin}/${path}`;
}
