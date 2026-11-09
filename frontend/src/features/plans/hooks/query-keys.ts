import type { ServerListParams } from "@/components/entity";

export const plansKeys = {
  all: ["platform", "plans"] as const,
  lists: () => [...plansKeys.all, "list"] as const,
  list: (params: ServerListParams) => [...plansKeys.lists(), params] as const,
  meta: () => [...plansKeys.all, "meta"] as const,
  navStats: () => [...plansKeys.all, "nav-stats"] as const,
  details: () => [...plansKeys.all, "detail"] as const,
  detail: (uuid: string) => [...plansKeys.details(), uuid] as const,
  features: () => [...plansKeys.all, "features"] as const,
};
