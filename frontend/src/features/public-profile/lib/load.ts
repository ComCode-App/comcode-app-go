import { cache } from "react";

import type { components } from "@/generated/api";
import { fetchUpstream } from "@/lib/server/upstream";

export type PublicProfile = components["schemas"]["PublicProfile"];

export type PublicProfileResult =
  | { kind: "ok"; profile: PublicProfile }
  | { kind: "redirect"; username: string }
  | { kind: "legacy_redirect"; target: string }
  | { kind: "not_found" };

/** Same rule as backend platform/publicpath: 3-40 chars, a-z 0-9 and single . _ - between. */
const USERNAME_RE = /^[a-z0-9](?:[a-z0-9]|[._-][a-z0-9])*$/;

export function isUsernameShape(value: string): boolean {
  return value.length >= 3 && value.length <= 40 && USERNAME_RE.test(value);
}

/**
 * Loads /{username} once per request (page + generateMetadata share it via React cache).
 * Unknown, disabled or malformed usernames are not_found; renamed ones redirect. `via` is the
 * smart link that opened the page: the API applies its theme, sections and primary action (TEC-556).
 */
export const loadPublicProfile = cache(
  async (
    rawUsername: string,
    qr = false,
    via?: string,
  ): Promise<PublicProfileResult> => {
    const username = decodeURIComponent(rawUsername).toLowerCase();
    // Legacy hashes can be shorter/longer than a username; the same backend call
    // resolves them. Static app pages never enter this loader.
    if (!/^[a-z0-9][a-z0-9_.-]{0,118}$/.test(username))
      return { kind: "not_found" };

    const query = new URLSearchParams();
    if (qr) query.set("src", "qr");
    if (via?.startsWith("s/")) query.set("via", via);
    const qs = query.toString();
    const result = await fetchUpstream(
      `public/p/${encodeURIComponent(username)}${qs ? `?${qs}` : ""}`,
      {
        method: "GET",
        headers: { Accept: "application/json" },
      },
    );
    if (result.status === 307) {
      const location = result.headers.get("Location");
      if (!location) throw new Error("legacy redirect has no destination");
      const target = new URL(location);
      if (
        !["https:", "http:"].includes(target.protocol) ||
        target.username ||
        target.password
      )
        throw new Error("invalid legacy redirect destination");
      return { kind: "legacy_redirect", target: target.href };
    }
    if (result.status === 404) return { kind: "not_found" };
    if (result.status >= 400) {
      throw new Error(`public profile lookup failed (${result.status})`);
    }
    const envelope = JSON.parse(new TextDecoder().decode(result.body)) as {
      data?: PublicProfile;
    };
    const profile = envelope.data;
    if (!profile) return { kind: "not_found" };
    if (profile.redirect_to)
      return { kind: "redirect", username: profile.redirect_to };
    return { kind: "ok", profile };
  },
);
