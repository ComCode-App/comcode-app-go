import type { components } from "@/generated/api";
import { apiConfig } from "@/config/api";
import { parseApiError } from "@/lib/api/errors";
import { parseContentDispositionFilename } from "@/lib/api/platform-form-request";
import { platformRequest } from "@/lib/api/platform-request";

export type RouteQRStyle = components["schemas"]["RouteQRStyle"];
export type RouteQR = components["schemas"]["RouteQR"];
export type RouteSummary = components["schemas"]["RouteSummary"];

/** Smart QR (TEC-569): per-route appearance; the payload is always the route identity + ?src=qr. */
export const routeQrService = {
  get(routeUuid: string) {
    return platformRequest<RouteQR>("GET", `/v1/routes/${routeUuid}/qr`);
  },
  put(routeUuid: string, style: RouteQRStyle) {
    return platformRequest<RouteQR>("PUT", `/v1/routes/${routeUuid}/qr`, {
      body: style,
    });
  },
  /** Server-rendered file of the *saved* style (svg|png|pdf). */
  async render(
    routeUuid: string,
    format: "svg" | "png" | "pdf",
    size: number,
    signal?: AbortSignal,
  ) {
    const base = apiConfig.baseUrl.replace(/\/$/, "");
    const qs = new URLSearchParams({ format, size: String(size) });
    const response = await fetch(
      `${base}/v1/routes/${routeUuid}/qr/render?${qs}`,
      {
        credentials: "include",
        signal,
      },
    );
    if (!response.ok) {
      // Errors are JSON envelopes (400 = style/size rejected, 404 = not this account's route).
      throw parseApiError(
        response.status,
        await response.json().catch(() => undefined),
      );
    }
    return {
      blob: await response.blob(),
      filename: parseContentDispositionFilename(
        response.headers.get("Content-Disposition"),
      ),
    };
  },
  /**
   * Same-origin URL of a QR logo (TEC-1271): a same-account profile avatar or cover, read through
   * the API so browser-drawn files can embed it without access to the storage origin.
   */
  logoUrl(routeUuid: string, profileUuid: string, field: "avatar" | "cover") {
    const base = apiConfig.baseUrl.replace(/\/$/, "");
    const qs = new URLSearchParams({ profile_uuid: profileUuid, field });
    return `${base}/v1/routes/${routeUuid}/qr/logo?${qs}`;
  },
  /** The canonical /{username} route of a profile (the QR subject of its public page). */
  async canonicalRoute(profileUuid: string, username?: string) {
    const res = await platformRequest<{ items: RouteSummary[] }>(
      "GET",
      "/v1/routes",
      {
        query: { route_type: "canonical_profile", q: username, limit: 100 },
      },
    );
    return (
      (res.items ?? []).find((r) => r.profile_uuid === profileUuid) ?? null
    );
  },
  async canonicalRoutes() {
    const res = await platformRequest<{ items: RouteSummary[] }>(
      "GET",
      "/v1/routes",
      {
        query: { route_type: "canonical_profile", limit: 100 },
      },
    );
    return res.items ?? [];
  },
};

export const routeQrKeys = {
  all: ["route-qr"] as const,
  detail: (routeUuid: string) => ["route-qr", routeUuid] as const,
  canonical: (profileUuid: string, username?: string) =>
    ["route-qr", "canonical", profileUuid, username ?? ""] as const,
  canonicalAll: ["route-qr", "canonical", "all"] as const,
};
