import type { SystemInfo } from "@/features/settings/types";
import { platformRequest } from "@/lib/api/platform-request";

export const systemService = {
  async getInfo(): Promise<SystemInfo> {
    return platformRequest<SystemInfo>("GET", "/v1/platform/system/info");
  },

  async ping(): Promise<SystemInfo> {
    return platformRequest<SystemInfo>("POST", "/v1/platform/system/ping");
  },
};
