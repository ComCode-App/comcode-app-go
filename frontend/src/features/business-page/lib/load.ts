import { cache } from "react";

import { fetchUpstream } from "@/lib/server/upstream";

import type { PublicBusinessPage } from "../services/business-page.service";

export type PublicBusinessResult =
  | { kind: "ok"; page: PublicBusinessPage }
  | { kind: "redirect"; username: string }
  | { kind: "not_found" };

/**
 * Loads a published business page (TEC-1327) once per request (page + generateMetadata share it).
 * `/{username}` tries the profile first; a username that is no profile lands here.
 */
export const loadPublicBusiness = cache(
  async (rawUsername: string): Promise<PublicBusinessResult> => {
    const username = decodeURIComponent(rawUsername).toLowerCase();
    if (!/^[a-z0-9][a-z0-9_.-]{0,39}$/.test(username))
      return { kind: "not_found" };
    const result = await fetchUpstream(
      `public/business/${encodeURIComponent(username)}`,
      { method: "GET", headers: { Accept: "application/json" } },
    );
    if (result.status === 404) return { kind: "not_found" };
    if (result.status >= 400) {
      throw new Error(`business page lookup failed (${result.status})`);
    }
    const envelope = JSON.parse(new TextDecoder().decode(result.body)) as {
      data?: PublicBusinessPage;
    };
    const page = envelope.data;
    if (!page) return { kind: "not_found" };
    if (page.redirect_to)
      return { kind: "redirect", username: page.redirect_to };
    return { kind: "ok", page };
  },
);
