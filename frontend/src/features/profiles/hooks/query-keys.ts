export const profilesKeys = {
  all: ["profiles"] as const,
  lists: () => [...profilesKeys.all, "list"] as const,
  list: (params: Record<string, unknown>) =>
    [...profilesKeys.lists(), params] as const,
  details: () => [...profilesKeys.all, "detail"] as const,
  detail: (uuid: string) => [...profilesKeys.details(), uuid] as const,
  meta: () => [...profilesKeys.all, "meta"] as const,
  publicPage: (uuid: string) => [...profilesKeys.all, "public-page", uuid] as const,
  verification: (uuid: string) => [...profilesKeys.all, "verification", uuid] as const,
};
