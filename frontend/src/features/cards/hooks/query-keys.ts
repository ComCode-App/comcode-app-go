export const cardsKeys = {
  all: ["cards"] as const,
  lists: () => [...cardsKeys.all, "list"] as const,
  list: (params?: Record<string, unknown>) => [...cardsKeys.lists(), params] as const,
  details: () => [...cardsKeys.all, "detail"] as const,
  detail: (uuid: string) => [...cardsKeys.details(), uuid] as const,
  meta: () => [...cardsKeys.all, "meta"] as const,
  smartLink: (code: string) => [...cardsKeys.all, "smart-link", code] as const,
};
