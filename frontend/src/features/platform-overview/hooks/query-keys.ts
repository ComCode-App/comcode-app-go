export const overviewKeys = {
  all: ["platform-overview"] as const,
  data: () => [...overviewKeys.all, "data"] as const,
  funnel: (days: number) => [...overviewKeys.all, "funnel", days] as const,
};
