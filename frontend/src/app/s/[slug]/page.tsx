import { notFound, redirect } from "next/navigation";

import { SmartLinkPasscodeScreen } from "./passcode-screen";
import type { SmartLinkResolution } from "@/features/cards/services/cards.service";
import { withVia } from "@/features/leads/lib/via";
import { fetchUpstream } from "@/lib/server/upstream";

type PageProps = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ src?: string }>;
};

/** Smart link entry point (comcode.../s/{slug}): redirect on the server to the link's target. */
export default async function SmartLinkPage({
  params,
  searchParams,
}: PageProps) {
  const [{ slug }, { src }] = await Promise.all([params, searchParams]);
  const result = await fetchUpstream(
    `s/${encodeURIComponent(slug)}${src === "qr" ? "?src=qr" : ""}`,
    {
      method: "GET",
      headers: { Accept: "application/json" },
    },
  );
  if (result.status !== 200) notFound();
  const { data } = JSON.parse(new TextDecoder().decode(result.body)) as {
    data?: SmartLinkResolution;
  };
  // Locked or retargeted-away links answer 404 rather than leaking where they used to point.
  if (!data || !data.is_active || data.is_locked) notFound();
  // Passcode links resolve without a target until the visitor unlocks them (TEC-556).
  if (data.requires_passcode) return <SmartLinkPasscodeScreen slug={slug} />;
  if (!data.target_url) notFound();
  // Always attribute, also when the link's own lead form is off (TEC-940).
  redirect(withVia(data.target_url, `s/${slug}`));
}
