import type { ServerListParams } from "@/components/entity";
import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type PlatformSector = components["schemas"]["Sector"];
export type SectorInput = components["schemas"]["SectorInput"];
export type SectorListResult = components["schemas"]["SectorPage"];

/** Platform sector catalog (/v1/platform/sectors, `platform.sectors.*`). */
export const sectorsService = {
  list(params: ServerListParams) {
    return platformRequest<SectorListResult>("GET", "/v1/platform/sectors", {
      query: { limit: params.limit, offset: params.offset, q: params.q },
    });
  },
  create(body: SectorInput) {
    return platformRequest<PlatformSector>("POST", "/v1/platform/sectors", {
      body,
      silent: true,
    });
  },
  update(uuid: string, body: SectorInput) {
    return platformRequest<PlatformSector>(
      "PUT",
      `/v1/platform/sectors/${uuid}`,
      { body, silent: true },
    );
  },
  remove(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/platform/sectors/${uuid}`, {
      silent: true,
    });
  },
};
