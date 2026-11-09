import type { components } from "@/generated/api";
import { apiClient, unwrap } from "@/lib/api";

export type ErrorTrackingSettings =
  components["schemas"]["ErrorTrackingSettings"];
export type ErrorTrackingSettingsPatch =
  components["schemas"]["PatchErrorTrackingSettingsRequest"];

export const errorTrackingService = {
  async get() {
    return unwrap<ErrorTrackingSettings>(
      await apiClient.GET("/v1/platform/error-tracking"),
    );
  },

  async patch(body: ErrorTrackingSettingsPatch) {
    return unwrap<ErrorTrackingSettings>(
      await apiClient.PATCH("/v1/platform/error-tracking", { body }),
    );
  },

  async sendBackendTest() {
    return unwrap<{ event_id: string }>(
      await apiClient.POST("/v1/platform/error-tracking/test"),
      { silent: true },
    );
  },
};
