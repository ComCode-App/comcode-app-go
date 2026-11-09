import type { ServerListParams } from "@/components/entity";

export const productsQueryKeys = {
  all: ["platform-products"] as const,
  lists: () => [...productsQueryKeys.all, "list"] as const,
  list: (
    params: ServerListParams & { status?: string; category_id?: number },
  ) => [...productsQueryKeys.lists(), params] as const,
  details: () => [...productsQueryKeys.all, "detail"] as const,
  detail: (uuid: string) => [...productsQueryKeys.details(), uuid] as const,
  variants: (productUuid: string) =>
    [...productsQueryKeys.all, "variants", productUuid] as const,
  meta: () => [...productsQueryKeys.all, "meta"] as const,
};
