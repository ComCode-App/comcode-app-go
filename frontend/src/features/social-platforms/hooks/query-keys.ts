export const socialPlatformsKeys = {
  all: ["social-platforms"] as const,
  lists: () => [...socialPlatformsKeys.all, "list"] as const,
  list: (params: Record<string, unknown>) =>
    [...socialPlatformsKeys.lists(), params] as const,
  active: () => [...socialPlatformsKeys.all, "active"] as const,
  meta: () => [...socialPlatformsKeys.all, "meta"] as const,
  details: () => [...socialPlatformsKeys.all, "detail"] as const,
  detail: (uuid: string) => [...socialPlatformsKeys.details(), uuid] as const,
};
