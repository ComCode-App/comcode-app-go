import type { components } from "@/generated/api";
import { apiClient, unwrap } from "@/lib/api";

export type GeoIPStatus = components["schemas"]["GeoIPStatus"];
export type GeoIPSettings = components["schemas"]["GeoIPSettings"];
export type GeoIPLookupResult = components["schemas"]["GeoIPLookupResult"];

export type GeoIPUpdateResult = {
  success?: boolean;
  version?: string;
  downloaded_at?: string;
  bytes?: number;
};

export const geoipService = {
  async getStatus() {
    return unwrap<GeoIPStatus>(await apiClient.GET("/v1/platform/geoip/status"));
  },

  async getConfig() {
    return unwrap<GeoIPSettings>(await apiClient.GET("/v1/platform/geoip/config"));
  },

  async patchConfig(body: GeoIPSettings) {
    return unwrap<GeoIPSettings>(
      await apiClient.PATCH("/v1/platform/geoip/config", { body }),
    );
  },

  async updateNow() {
    return unwrap<GeoIPUpdateResult>(
      await apiClient.POST("/v1/platform/geoip/update"),
    );
  },

  async clearCache() {
    return unwrap<{ status: string }>(
      await apiClient.POST("/v1/platform/geoip/cache/clear"),
    );
  },

  async lookup(ip?: string) {
    return unwrap<GeoIPLookupResult>(
      await apiClient.GET("/v1/platform/geoip/lookup", {
        params: {
          query: {
            ip: ip || undefined,
          },
        },
      }),
    );
  },
};
