import { cache } from "react";

import type { components } from "@/generated/api";
import { VIA_RE } from "@/features/leads/lib/via";
import { fetchUpstream } from "@/lib/server/upstream";

type LeadFormConfig = components["schemas"]["LeadFormConfig"];

/** `via` is set when the profile's own form opened through a card or smart link (TEC-940). */
export type LeadForm = { path: string; consentVersion: string; via?: string };

/** Contact exchange on a profile page (TEC-567). `via` is the card / smart link the visitor came through. */
export type ExchangeForm = {
  path: string;
  consentVersion: string;
  via?: string;
};

const fetchConfig = cache(
  async (path: string): Promise<LeadFormConfig | null> => {
    try {
      const result = await fetchUpstream(
        `public/leads/form?path=${encodeURIComponent(path)}`,
        {
          method: "GET",
          headers: { Accept: "application/json" },
        },
      );
      if (result.status !== 200) return null;
      const { data } = JSON.parse(new TextDecoder().decode(result.body)) as {
        data?: LeadFormConfig;
      };
      return data ?? null;
    } catch {
      return null; // the form is optional; never break the profile page
    }
  },
);

/**
 * Server-side: which lead form a public profile page shows (TEC-552). A visitor who arrived
 * through a card or smart link (`via`) submits through that route when its form is on and it
 * opens this same profile; otherwise the profile's own route decides.
 */
export const loadLeadForm = cache(
  async (
    username: string,
    profileUuid: string,
    via?: string,
  ): Promise<LeadForm | null> => {
    if (via && VIA_RE.test(via)) {
      const cfg = await fetchConfig(via);
      if (cfg?.enabled && cfg.profile_uuid === profileUuid) {
        return { path: cfg.path, consentVersion: cfg.consent_version };
      }
    }
    const own = await fetchConfig(username);
    if (!own?.enabled) return null;
    // The card's own form is off: the profile's form still attributes the lead to the card.
    return {
      path: own.path,
      consentVersion: own.consent_version,
      via: via && VIA_RE.test(via) ? via : undefined,
    };
  },
);

/** Server-side: whether a public profile page offers contact exchange (TEC-567). */
export const loadExchangeForm = cache(
  async (
    username: string,
    profileUuid: string,
    via?: string,
  ): Promise<ExchangeForm | null> => {
    const cfg = await fetchConfig(username);
    if (!cfg?.exchange_enabled || cfg.profile_uuid !== profileUuid) return null;
    return {
      path: cfg.path,
      consentVersion: cfg.exchange_consent_version,
      via: via && VIA_RE.test(via) ? via : undefined,
    };
  },
);
