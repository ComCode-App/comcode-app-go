export const banksQueryKeys = {
  all: ["platform", "banks"] as const,
  lists: () => [...banksQueryKeys.all, "list"] as const,
  list: (params: Record<string, unknown>) =>
    [...banksQueryKeys.lists(), params] as const,
  active: () => [...banksQueryKeys.all, "active"] as const,
  details: () => [...banksQueryKeys.all, "detail"] as const,
  detail: (uuid: string) => [...banksQueryKeys.details(), uuid] as const,
  meta: () => [...banksQueryKeys.all, "meta"] as const,
};
