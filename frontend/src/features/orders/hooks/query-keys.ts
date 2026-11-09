import type { ServerListParams } from "@/components/entity";

export const ordersQueryKeys = {
  all: ["platform-orders"] as const,
  lists: () => [...ordersQueryKeys.all, "list"] as const,
  list: (params: ServerListParams & { status?: string }) =>
    [...ordersQueryKeys.lists(), params] as const,
  myOrders: (params?: { limit?: number; offset?: number; q?: string }) =>
    ["my-orders", params] as const,
  kpis: () => [...ordersQueryKeys.all, "kpis"] as const,
  details: () => [...ordersQueryKeys.all, "detail"] as const,
  detail: (uuid: string) => [...ordersQueryKeys.details(), uuid] as const,
  refunds: (uuid: string) => [...ordersQueryKeys.detail(uuid), "refunds"] as const,
  meta: () => [...ordersQueryKeys.all, "meta"] as const,
};
