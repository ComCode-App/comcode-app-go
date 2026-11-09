import type { ServerListParams } from "@/components/entity";

export const companiesKeys = {
  all: ["platform", "companies"] as const,
  lists: () => [...companiesKeys.all, "list"] as const,
  list: (params: ServerListParams & { status?: string }) =>
    [...companiesKeys.lists(), params] as const,
  meta: () => [...companiesKeys.all, "meta"] as const,
  navStats: () => [...companiesKeys.all, "nav-stats"] as const,
  details: () => [...companiesKeys.all, "detail"] as const,
  detail: (uuid: string) => [...companiesKeys.details(), uuid] as const,
};
