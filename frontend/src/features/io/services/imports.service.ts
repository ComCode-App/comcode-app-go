import type { ImportFormat, ImportJob } from "@/features/io/types";
import {
  platformDownloadRequest,
  platformFormRequest,
} from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type ListJobsResult = {
  items: ImportJob[];
  total: number;
  limit: number;
  offset: number;
};

const PLATFORM_JOBS = "/v1/platform/imports";

/** Job routes take `base`: platform jobs by default, `/v1/company/imports` for company imports. */
export const importsService = {
  async upload(
    path: string,
    file: File,
    format: ImportFormat,
    locale: string,
  ) {
    const form = new FormData();
    form.append("file", file);
    form.append("format", format);
    form.append("locale", locale);
    return platformFormRequest<ImportJob>("POST", path, form);
  },

  async downloadSample(path: string, format: ImportFormat, locale: string) {
    return platformDownloadRequest(path, { format, locale });
  },

  async updateMapping(
    uuid: string,
    body: { mapping: Record<string, string>; defaults?: Record<string, string> },
    base = PLATFORM_JOBS,
  ) {
    return platformRequest<ImportJob>("PATCH", `${base}/${uuid}/mapping`, {
      body,
    });
  },

  async preview(uuid: string, base = PLATFORM_JOBS) {
    return platformRequest<ImportJob>("POST", `${base}/${uuid}/preview`);
  },

  async confirm(uuid: string, base = PLATFORM_JOBS) {
    return platformRequest<ImportJob>("POST", `${base}/${uuid}/confirm`);
  },

  async rollback(uuid: string, base = PLATFORM_JOBS) {
    return platformRequest<ImportJob>("POST", `${base}/${uuid}/rollback`);
  },

  async list(
    params: { limit?: number; offset?: number },
    base = PLATFORM_JOBS,
  ) {
    return platformRequest<ListJobsResult>("GET", base, {
      query: params,
    });
  },

  async get(uuid: string, base = PLATFORM_JOBS) {
    return platformRequest<ImportJob>("GET", `${base}/${uuid}`);
  },
};
