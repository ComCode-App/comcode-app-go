import type { components } from "@/generated/api";
import { apiClient, unwrap } from "@/lib/api";

export type EventDefinition = components["schemas"]["EventDefinition"];
export type EventDefinitionInput = components["schemas"]["EventDefinitionInput"];
export type AnalyticsSettings = components["schemas"]["AnalyticsSettings"];
/** PATCH body: every writable setting (the salt schedule fields are read-only, TEC-936). */
export type AnalyticsSettingsInput = Required<
  Omit<AnalyticsSettings, "ip_hash_salt_rotation_previous_days" | "ip_hash_salt_rotation_changed_on">
>;

export type EventDefinitionFilters = {
  category?: string;
  enabled?: boolean;
  is_public?: boolean;
};

/** One raw event row from `GET /v1/analytics/events` (`domain.Event`). */
export type RawEvent = components["schemas"]["AnalyticsRawEvent"];

/** `total` counts at most 10,000 rows (`total_capped`); `has_more` drives paging past the cap. */
export type RawEventList = components["schemas"]["AnalyticsRawEventList"];

/** Platform-wide event volume from the hourly rollups (TEC-859). */
export type PlatformAnalyticsOverview = components["schemas"]["PlatformAnalyticsOverview"];

/** Raw event filters; `from` / `to` are RFC 3339 instants (the API ignores anything else). */
export type RawEventParams = {
  subject_type?: string;
  subject_id?: string;
  event_type?: string;
  country?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
};

function clean<T extends Record<string, unknown>>(params: T): T {
  return Object.fromEntries(
    Object.entries(params).filter(([, v]) => v !== undefined && v !== ""),
  ) as T;
}

/** Platform analytics admin endpoints (event registry, runtime config, raw event drilldown). */
export const platformAnalyticsService = {
  async listDefinitions(filters: EventDefinitionFilters = {}) {
    const data = await unwrap<{ items?: EventDefinition[]; total?: number }>(
      await apiClient.GET("/v1/platform/events/definitions", {
        params: { query: clean(filters) },
      }),
    );
    return { items: data.items ?? [], total: data.total ?? data.items?.length ?? 0 };
  },

  async createDefinition(body: EventDefinitionInput) {
    return unwrap<EventDefinition>(
      await apiClient.POST("/v1/platform/events/definitions", { body }),
    );
  },

  async updateDefinition(id: number, body: EventDefinitionInput) {
    return unwrap<EventDefinition>(
      await apiClient.PATCH("/v1/platform/events/definitions/{id}", {
        params: { path: { id } },
        body,
      }),
    );
  },

  async deleteDefinition(id: number) {
    await unwrap<unknown>(
      await apiClient.DELETE("/v1/platform/events/definitions/{id}", {
        params: { path: { id } },
      }),
    );
  },

  async getConfig() {
    return unwrap<AnalyticsSettings>(await apiClient.GET("/v1/platform/analytics/config"));
  },

  /** The API writes every field (booleans included), so always send the full settings object. */
  async updateConfig(body: AnalyticsSettingsInput) {
    return unwrap<AnalyticsSettings>(
      await apiClient.PATCH("/v1/platform/analytics/config", { body }),
    );
  },

  async listEvents(params: RawEventParams = {}): Promise<RawEventList> {
    const data = await unwrap<Partial<RawEventList>>(
      await apiClient.GET("/v1/analytics/events", { params: { query: clean(params) } }),
    );
    return {
      items: data.items ?? [],
      total: data.total ?? 0,
      total_capped: data.total_capped ?? false,
      has_more: data.has_more ?? false,
      limit: data.limit ?? params.limit ?? 20,
      offset: data.offset ?? params.offset ?? 0,
    };
  },

  /** KPIs and the daily series bucketed into the viewer's local days (`tz`, IANA name). */
  async getOverview(tz?: string) {
    return unwrap<PlatformAnalyticsOverview>(
      await apiClient.GET("/v1/platform/analytics/overview", { params: { query: clean({ tz }) } }),
    );
  },
};
