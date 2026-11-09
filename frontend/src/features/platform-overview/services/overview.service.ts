import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type PlatformOverview = components["schemas"]["PlatformOverview"];
export type ConversionFunnel =
  components["schemas"]["PlatformConversionFunnel"];
export type FunnelPeriod = ConversionFunnel["period_days"];

/** Platform-wide business KPIs (GET /v1/platform/overview, `platform.overview.read`). */
export const overviewService = {
  get() {
    return platformRequest<PlatformOverview>("GET", "/v1/platform/overview");
  },
  /** Order conversion funnel of the last `days` days (GET /v1/platform/overview/funnel). */
  funnel(days: FunnelPeriod) {
    return platformRequest<ConversionFunnel>(
      "GET",
      `/v1/platform/overview/funnel?days=${days}`,
    );
  },
};
