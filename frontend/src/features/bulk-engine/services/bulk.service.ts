import type {
  BulkExecuteSyncResult,
  BulkJob,
  BulkResource,
  BulkTarget,
  ListJobsResult,
} from "@/features/bulk-engine/types";
import { BULK_JOB_PATHS, BULK_PATHS } from "@/features/bulk-engine/types";
import { platformRequest } from "@/lib/api/platform-request";

export const bulkService = {
  async execute(
    resource: BulkResource,
    body: { action: string; target: BulkTarget; locale: string },
  ) {
    return platformRequest<BulkJob | BulkExecuteSyncResult>(
      "POST",
      BULK_PATHS[resource],
      { body },
    );
  },

  async get(uuid: string) {
    return platformRequest<BulkJob>("GET", `/v1/platform/bulk/${uuid}`);
  },

  /** Rolls a job back; account-scoped resources use their own job path. */
  async rollback(uuid: string, resource?: BulkResource) {
    const base = (resource && BULK_JOB_PATHS[resource]) ?? "/v1/platform/bulk";
    return platformRequest<BulkJob>("POST", `${base}/${uuid}/rollback`);
  },

  async list(params: { limit?: number; offset?: number }) {
    return platformRequest<ListJobsResult>("GET", "/v1/platform/bulk", {
      query: params,
    });
  },
};
