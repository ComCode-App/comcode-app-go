import type { components } from "@/generated/api";
import { apiClient, unwrap } from "@/lib/api";

export type SupportCard = components["schemas"]["SupportCard"];
export type SupportRoute = components["schemas"]["SupportRoute"];
export type SupportRouteHistoryEntry = components["schemas"]["SupportRouteHistoryEntry"];
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
      await apiClient.POST("/v1/platform/cards/{uuid}/retire", { params: { path: { uuid } } }),
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
  async searchRoutes(q: string) {
    return unwrap<{ items: SupportRoute[] }>(
      await apiClient.GET("/v1/platform/routes", { params: { query: { q } } }),
    );
  },
  async releasePath(path: string) {
    return unwrap<QuarantineRelease>(
      await apiClient.POST("/v1/platform/routes/quarantine/release", { body: { path } }),
    );
  },
};

export const platformSupportKeys = {
  all: ["platform-support"] as const,
  cards: (q: string) => [...platformSupportKeys.all, "cards", q] as const,
  routes: (q: string) => [...platformSupportKeys.all, "routes", q] as const,
};
