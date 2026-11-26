import type { components } from "@/generated/api";
import { apiClient, unwrap } from "@/lib/api";
import { platformFormRequest } from "@/lib/api/platform-form-request";

export type SupportCard = components["schemas"]["SupportCard"];
export type SupportRoute = components["schemas"]["SupportRoute"];
export type SupportRouteHistoryEntry =
  components["schemas"]["SupportRouteHistoryEntry"];
export type QuarantineRelease = components["schemas"]["QuarantineRelease"];

/** Platform card and route support screens (TEC-950). Writes need a step-up grant. */
export const platformSupportService = {
  async searchCards(q: string) {
    return unwrap<{ items: SupportCard[] }>(
      await apiClient.GET("/v1/platform/cards", { params: { query: { q } } }),
    );
  },
  async retireCard(uuid: string) {
    return unwrap<SupportCard>(
      await apiClient.POST("/v1/platform/cards/{uuid}/retire", {
        params: { path: { uuid } },
      }),
    );
  },
  async reassignCard(uuid: string, profileUuid: string) {
    return unwrap<SupportCard>(
      await apiClient.POST("/v1/platform/cards/{uuid}/reassign", {
        params: { path: { uuid } },
        body: { profile_uuid: profileUuid },
      }),
    );
  },
  /** Upload a card face's design image (TEC-1320): PNG, JPEG or WebP up to 5 MB. */
  async uploadCardImage(uuid: string, side: "front" | "back", file: File) {
    const form = new FormData();
    form.append("image", file);
    return platformFormRequest<SupportCard>(
      "PUT",
      `/v1/platform/cards/${uuid}/images/${side}`,
      form,
    );
  },
  async removeCardImage(uuid: string, side: "front" | "back") {
    return unwrap<SupportCard>(
      await apiClient.DELETE("/v1/platform/cards/{uuid}/images/{side}", {
        params: { path: { uuid, side } },
      }),
    );
  },
  async searchRoutes(q: string) {
    return unwrap<{ items: SupportRoute[] }>(
      await apiClient.GET("/v1/platform/routes", { params: { query: { q } } }),
    );
  },
  async releasePath(path: string) {
    return unwrap<QuarantineRelease>(
      await apiClient.POST("/v1/platform/routes/quarantine/release", {
        body: { path },
      }),
    );
  },
};

export const platformSupportKeys = {
  all: ["platform-support"] as const,
  cards: (q: string) => [...platformSupportKeys.all, "cards", q] as const,
  routes: (q: string) => [...platformSupportKeys.all, "routes", q] as const,
};
