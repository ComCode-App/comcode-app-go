import type { components } from "@/generated/api";
import { exportDownloadFilename } from "@/features/io/lib/display";
import type { ExportFormat, ExportJob } from "@/features/io/types";
import {
  platformDownloadFile,
  triggerBrowserDownload,
} from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type Lead = components["schemas"]["Lead"];
export type LeadSource = components["schemas"]["LeadSource"];
export type LeadKind = components["schemas"]["LeadKind"];
export type LeadFormConfig = components["schemas"]["LeadFormConfig"];
export type SubmitLeadPayload = components["schemas"]["SubmitLeadRequest"];
export type ContactExchangePayload = components["schemas"]["ContactExchangeRequest"];
export type ShareProfileExchangePayload = components["schemas"]["ShareProfileExchangeRequest"];
export type DeleteExchangePayload = components["schemas"]["DeleteContactExchangeRequest"];

/** Outcome of a visitor post: `fields` lists API validation details as `field:code`. */
export type VisitorPostResult = { ok: true } | { ok: false; status: number; fields: string[] };

export type LeadListParams = {
  limit: number;
  offset: number;
  q?: string;
  unread?: boolean;
  source_type?: LeadSource;
  kind?: LeadKind;
};

export type LeadPage = {
  items: Lead[];
  total: number;
  limit: number;
  offset: number;
};

export const leadsService = {
  list(params: LeadListParams) {
    return platformRequest<LeadPage>("GET", "/v1/leads", {
      query: {
        limit: params.limit,
        offset: params.offset,
        q: params.q?.trim() || undefined,
        unread: params.unread ? "true" : undefined,
        source_type: params.source_type,
        kind: params.kind,
      },
    });
  },

  get(uuid: string) {
    return platformRequest<Lead>("GET", `/v1/leads/${uuid}`);
  },

  markRead(uuid: string) {
    return platformRequest<Lead>("POST", `/v1/leads/${uuid}/read`);
  },

  remove(uuid: string) {
    return platformRequest<{ deleted: boolean }>("DELETE", `/v1/leads/${uuid}`);
  },

  /** Queues an export of the active account's leads (account comes from the session). */
  requestExport(body: { format: ExportFormat; locale: string; query: Record<string, string> }) {
    return platformRequest<ExportJob>("POST", "/v1/leads/export", { body });
  },

  getExport(uuid: string) {
    return platformRequest<ExportJob>("GET", `/v1/exports/${uuid}`);
  },

  async downloadExport(job: ExportJob) {
    const { blob, filename } = await platformDownloadFile(`/v1/exports/${job.uuid}/download`);
    triggerBrowserDownload(
      blob,
      filename ?? exportDownloadFilename(job.resource, job.format, new Date(job.created_at)),
    );
  },
};

/**
 * Visitor calls: same-origin BFF, never the Go host. The lead form and the contact exchange form
 * need no session; sharing a profile uses the visitor's own session (TEC-567).
 */
export const publicLeadsService = {
  submit(body: SubmitLeadPayload) {
    return postVisitor("/api/v1/public/leads", body);
  },

  exchange(body: ContactExchangePayload) {
    return postVisitor("/api/v1/public/exchanges", body);
  },

  shareProfile(body: ShareProfileExchangePayload) {
    return postVisitor("/api/v1/exchanges", body);
  },

  /** KVKK delete link of the exchange visitor copy email (TEC-940); single use. */
  deleteExchange(token: string) {
    const body: DeleteExchangePayload = { token };
    return postVisitor("/api/v1/public/exchanges/delete", body);
  },
};

async function postVisitor(url: string, body: unknown): Promise<VisitorPostResult> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify(body),
  });
  if (res.ok) return { ok: true };
  const payload = (await res.json().catch(() => null)) as {
    error?: { details?: { field?: string; code?: string }[] };
  } | null;
  const fields = (payload?.error?.details ?? []).map((d) => `${d.field ?? ""}:${d.code ?? ""}`);
  return { ok: false, status: res.status, fields };
}
