export const sectorsQueryKeys = {
  all: ["platform", "sectors"] as const,
  lists: () => [...sectorsQueryKeys.all, "list"] as const,
  list: (params: Record<string, unknown>) =>
    [...sectorsQueryKeys.lists(), params] as const,
};
