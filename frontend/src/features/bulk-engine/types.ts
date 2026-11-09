import type { LucideIcon } from "lucide-react";

/** Bulk action contract from resource meta API (no icon). */
export type BulkActionMeta = {
  id: string;
  label_key: string;
  permission: string;
  destructive?: boolean;
  reversible?: boolean;
  confirm_key?: string;
};

/** Bulk action for UI — icon is required. */
export type BulkActionDef = BulkActionMeta & {
  icon: LucideIcon;
};

export type BulkTarget = {
  scope: "ids" | "query";
  ids?: string[];
  query?: Record<string, string>;
};

export type BulkSummary = {
  total: number;
  succeeded: number;
  failed: number;
  /** First failed items (at most 50). */
  errors?: { entity_uuid: string; error: string }[];
};

export type BulkJob = {
  uuid: string;
  resource: string;
  action: string;
  status: string;
  target?: BulkTarget;
  params?: Record<string, unknown>;
  summary?: BulkSummary;
  error?: string | null;
  rollback_until?: string | null;
  applied_at?: string | null;
  created_at: string;
};

export type BulkExecuteSyncResult = {
  sync: true;
  summary: BulkSummary;
};

export type BulkResource =
  | "platform.users"
  | "platform.roles"
  | "platform.plans"
  | "platform.subscriptions"
  | "platform.companies"
  | "app.company_profiles";

export const BULK_PATHS: Record<BulkResource, string> = {
  "platform.users": "/v1/platform/users/bulk",
  "platform.roles": "/v1/platform/roles/bulk",
  "platform.plans": "/v1/platform/plans/bulk",
  "platform.subscriptions": "/v1/platform/subscriptions/bulk",
  "platform.companies": "/v1/platform/companies/bulk",
  "app.company_profiles": "/v1/company/profiles/bulk",
};

export type SelectionScope =
  | { mode: "none" }
  | { mode: "page"; ids: string[] }
  | { mode: "all"; query: Record<string, string>; total: number };

export type ListJobsResult = {
  items: BulkJob[];
  total: number;
  limit: number;
  offset: number;
};
