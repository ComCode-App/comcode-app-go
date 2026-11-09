import type { ServerListParams } from "@/components/entity";

export const categoriesQueryKeys = {
  all: ["platform-categories"] as const,
  lists: () => [...categoriesQueryKeys.all, "list"] as const,
  list: (params: ServerListParams) =>
    [...categoriesQueryKeys.lists(), params] as const,
  active: () => [...categoriesQueryKeys.all, "active"] as const,
  details: () => [...categoriesQueryKeys.all, "detail"] as const,
  detail: (uuid: string) => [...categoriesQueryKeys.details(), uuid] as const,
  meta: () => [...categoriesQueryKeys.all, "meta"] as const,
};
