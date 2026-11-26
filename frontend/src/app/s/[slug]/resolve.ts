import "server-only";

import { cache } from "react";

import type { SmartLinkResolution } from "@/features/cards/services/cards.service";
import { fetchUpstream } from "@/lib/server/upstream";

/** Cookie holding a visitor's correct passcode, scoped to its /s/{slug} path (TEC-1307). */
export const SMART_LINK_PASSCODE_COOKIE = "cc_sl_passcode";

export function decodeResolution(
  body: ArrayBuffer,
): SmartLinkResolution | undefined {
  try {
    return (
      JSON.parse(new TextDecoder().decode(body)) as {
        data?: SmartLinkResolution;
      }
    ).data;
  } catch {
    return undefined;
  }
}

/**
 * GET /v1/s/{slug} once per request: the API counts a visit on every resolve, so the page and
 * generateMetadata share this call (React cache).
 */
export const resolveSmartLink = cache(async (slug: string, qr: boolean) => {
  const result = await fetchUpstream(
    `s/${encodeURIComponent(slug)}${qr ? "?src=qr" : ""}`,
    { method: "GET", headers: { Accept: "application/json" } },
  );
  return result.status === 200 ? decodeResolution(result.body) : undefined;
});

/** POST /v1/s/{slug}/unlock; the passcode travels in the body only. */
export async function unlockUpstream(slug: string, passcode: string) {
  return fetchUpstream(`s/${encodeURIComponent(slug)}/unlock`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ passcode }),
  });
}
