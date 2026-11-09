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
    return platformRequest<RouteQR>("PUT", `/v1/routes/${routeUuid}/qr`, { body: style });
  },
  /** Server-rendered file of the *saved* style (svg|png|pdf). */
  async render(routeUuid: string, format: "svg" | "png" | "pdf", size: number, signal?: AbortSignal) {
    const base = apiConfig.baseUrl.replace(/\/$/, "");
    const qs = new URLSearchParams({ format, size: String(size) });
    const response = await fetch(`${base}/v1/routes/${routeUuid}/qr/render?${qs}`, {
      credentials: "include",
      signal,
    });
    if (!response.ok) {
      // Errors are JSON envelopes (400 = style/size rejected, 404 = not this account's route).
      throw parseApiError(response.status, await response.json().catch(() => undefined));
    }
    return {
      blob: await response.blob(),
      filename: parseContentDispositionFilename(response.headers.get("Content-Disposition")),
    };
  },
  /**
   * Server render of an *unsaved* style (TEC-954): same renderer as downloads and the print pack,
   * nothing is stored. Rate limited per user, so callers debounce.
   */
  async preview(routeUuid: string, style: RouteQRStyle, format: "svg" | "png", size: number, signal?: AbortSignal) {
    const base = apiConfig.baseUrl.replace(/\/$/, "");
    const qs = new URLSearchParams({ format, size: String(size) });
    const response = await fetch(`${base}/v1/routes/${routeUuid}/qr/preview?${qs}`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(style),
      signal,
    });
    if (!response.ok) {
      // 400 = style/logo rejected, 404 = not this account's route, 429 = too many previews.
      throw parseApiError(response.status, await response.json().catch(() => undefined));
    }
    return response.blob();
  },
  /** The canonical /{username} route of a profile (the QR subject of its public page). */
  async canonicalRoute(profileUuid: string, username?: string) {
    const res = await platformRequest<{ items: RouteSummary[] }>("GET", "/v1/routes", {
      query: { route_type: "canonical_profile", q: username, limit: 100 },
    });
    return (res.items ?? []).find((r) => r.profile_uuid === profileUuid) ?? null;
  },
  async canonicalRoutes() {
    const res = await platformRequest<{ items: RouteSummary[] }>("GET", "/v1/routes", {
      query: { route_type: "canonical_profile", limit: 100 },
    });
    return res.items ?? [];
  },
};

export const routeQrKeys = {
  all: ["route-qr"] as const,
  detail: (routeUuid: string) => ["route-qr", routeUuid] as const,
  preview: (routeUuid: string, signature: string) => ["route-qr", routeUuid, "preview", signature] as const,
  canonical: (profileUuid: string, username?: string) => ["route-qr", "canonical", profileUuid, username ?? ""] as const,
  canonicalAll: ["route-qr", "canonical", "all"] as const,
};
