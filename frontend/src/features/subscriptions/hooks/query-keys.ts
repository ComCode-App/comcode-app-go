import type { ServerListParams } from "@/components/entity";

export const subscriptionsKeys = {
  all: ["platform", "subscriptions"] as const,
  lists: () => [...subscriptionsKeys.all, "list"] as const,
  list: (params: ServerListParams & { status?: string }) =>
    [...subscriptionsKeys.lists(), params] as const,
  meta: () => [...subscriptionsKeys.all, "meta"] as const,
  navStats: () => [...subscriptionsKeys.all, "nav-stats"] as const,
  details: () => [...subscriptionsKeys.all, "detail"] as const,
  detail: (uuid: string) => [...subscriptionsKeys.details(), uuid] as const,
};
