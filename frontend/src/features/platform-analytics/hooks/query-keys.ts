import type {
  EventDefinitionFilters,
  RawEventParams,
} from "../services/platform-analytics.service";

export const platformAnalyticsKeys = {
  all: ["platform", "analytics"] as const,
  definitions: () => [...platformAnalyticsKeys.all, "definitions"] as const,
  definitionList: (filters: EventDefinitionFilters) =>
    [...platformAnalyticsKeys.definitions(), filters] as const,
  config: () => [...platformAnalyticsKeys.all, "config"] as const,
  events: () => [...platformAnalyticsKeys.all, "events"] as const,
  eventList: (params: RawEventParams) => [...platformAnalyticsKeys.events(), "list", params] as const,
  overview: (tz: string) => [...platformAnalyticsKeys.all, "overview", tz] as const,
};
