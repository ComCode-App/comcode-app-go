import { redirect } from "next/navigation";

import { CardResolverClient } from "./card-resolver-client";
import type { SmartLinkResolution } from "@/features/cards";
import { withVia } from "@/features/leads/lib/via";
import { fetchUpstream } from "@/lib/server/upstream";

type PageProps = {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ src?: string }>;
};

/**
 * NFC / QR entry point (comcode.../c/{code}). Open cards redirect on the server so a tap lands on
 * the public profile without a loading screen; anything else falls back to the client page.
 */
export default async function CardLinkPage({
  params,
  searchParams,
}: PageProps) {
  const [{ code }, { src }] = await Promise.all([params, searchParams]);
  const target = await resolveTarget(code, src === "qr");
  if (target) redirect(target);
  return <CardResolverClient code={code} />;
}

async function resolveTarget(
  code: string,
  qr: boolean,
): Promise<string | null> {
  try {
    const result = await fetchUpstream(
      `c/${encodeURIComponent(code)}${qr ? "?src=qr" : ""}`,
      {
        method: "GET",
        headers: { Accept: "application/json" },
      },
    );
    if (result.status !== 200) return null;
    const { data } = JSON.parse(new TextDecoder().decode(result.body)) as {
      data?: SmartLinkResolution;
    };
    if (!data || !data.is_active || data.is_locked || data.requires_passcode)
      return null;
    if (!data.target_url) return null;
    // Always attribute: leads, exchanges and visits through this card are the card's (TEC-940).
    return withVia(data.target_url, `c/${code}`);
  } catch {
    return null;
  }
}
