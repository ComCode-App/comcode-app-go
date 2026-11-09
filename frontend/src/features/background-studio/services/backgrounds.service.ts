import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

import type { BackgroundDocument } from "../types";

export type SavedBackground = Omit<
  components["schemas"]["Background"],
  "document"
> & {
  document: BackgroundDocument;
};

/** Background Studio designs of the active account plus platform presets (/v1/backgrounds). */
export const backgroundsService = {
  async list() {
    const res = await platformRequest<{ items: SavedBackground[] }>(
      "GET",
      "/v1/backgrounds",
    );
    return res.items ?? [];
  },
  async create(name: string, document: BackgroundDocument) {
    return platformRequest<SavedBackground>("POST", "/v1/backgrounds", {
      body: { name, document },
    });
  },
  async update(uuid: string, name: string, document: BackgroundDocument) {
    return platformRequest<SavedBackground>("PUT", `/v1/backgrounds/${uuid}`, {
      body: { name, document },
    });
  },
  async remove(uuid: string) {
    return platformRequest<{ status: string }>(
      "DELETE",
      `/v1/backgrounds/${uuid}`,
    );
  },
};

export const backgroundKeys = {
  list: ["backgrounds", "list"] as const,
};
