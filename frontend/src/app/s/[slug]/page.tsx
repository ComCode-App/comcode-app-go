import type { Metadata } from "next";
import { cookies } from "next/headers";
import { notFound, redirect } from "next/navigation";

import { SmartLinkPasscodeScreen } from "./passcode-screen";
import {
  SMART_LINK_PASSCODE_COOKIE,
  decodeResolution,
  resolveSmartLink,
  unlockUpstream,
} from "./resolve";
import { brand } from "@/config/brand";
import { withVia } from "@/features/leads/lib/via";
import {
  publicProfileMetadata,
  renderPublicProfile,
} from "@/features/public-profile/lib/page";

type PageProps = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ src?: string; lang?: string }>;
};

/** Targets that open the profile page; those render in place at /s/{slug} (TEC-1307). */
const PROFILE_TARGETS = new Set(["profile", "social_hub"]);

/**
 * Smart link entry point (comcode.../s/{slug}). A link to a profile shows that profile here,
 * with the link's theme, sections, main button and lead attribution, so the address stays
 * /s/{slug} (TEC-1307). Other targets (web address, vCard) redirect on the server.
 */
export default async function SmartLinkPage({
  params,
  searchParams,
}: PageProps) {
  const [{ slug }, { src, lang }] = await Promise.all([params, searchParams]);
  let data = await resolveSmartLink(slug, src === "qr");
  // Locked or retargeted-away links answer 404 rather than leaking where they used to point.
  if (!data || !data.is_active || data.is_locked) notFound();
  // Passcode links resolve without a target until the visitor unlocks them (TEC-556). A
  // passcode entered on this link is kept in a cookie scoped to /s/{slug} and checked again.
  if (data.requires_passcode) {
    const passcode = (await cookies()).get(SMART_LINK_PASSCODE_COOKIE)?.value;
    const unlocked = passcode ? await unlockUpstream(slug, passcode) : null;
    data =
      unlocked?.status === 200 ? decodeResolution(unlocked.body) : undefined;
    if (!data) return <SmartLinkPasscodeScreen slug={slug} />;
  }
  const via = `s/${slug}`;
  if (
    data.profile_username &&
    data.target_type &&
    PROFILE_TARGETS.has(data.target_type)
  ) {
    return renderPublicProfile({
      username: data.profile_username,
      lang,
      via,
      qr: src === "qr",
      followRename: "inline",
    });
  }
  if (!data.target_url) notFound();
  // Always attribute, also when the link's own lead form is off (TEC-940).
  redirect(withVia(data.target_url, via));
}

export async function generateMetadata({
  params,
  searchParams,
}: PageProps): Promise<Metadata> {
  const [{ slug }, { lang, src }] = await Promise.all([params, searchParams]);
  // Same call as the page (React cache). Passcode links keep their target private: no
  // profile metadata.
  const data = await resolveSmartLink(slug, src === "qr");
  if (
    !data ||
    data.requires_passcode ||
    !data.profile_username ||
    !data.target_type ||
    !PROFILE_TARGETS.has(data.target_type)
  )
    return { title: brand.productName, robots: { index: false } };
  return {
    ...(await publicProfileMetadata({
      username: data.profile_username,
      lang,
      via: `s/${slug}`,
      qr: src === "qr",
    })),
    robots: { index: false },
  };
}
