import type { Metadata } from "next";
import { notFound, permanentRedirect, redirect } from "next/navigation";

import { brand } from "@/config/brand";
import {
  loadExchangeForm,
  loadLeadForm,
} from "@/features/leads/lib/form-config";
import { ProfileNotShared } from "@/features/public-profile/components/profile-not-shared";
import { ProfileSwipe } from "@/features/public-profile/components/profile-swipe";
import { PublicProfileView } from "@/features/public-profile/components/public-profile-view";
import { loadPublicProfile } from "@/features/public-profile/lib/load";
import { getTheme, resolveLayout } from "@/features/public-profile/lib/themes";
import {
  displayName,
  headline,
  pickTranslation,
} from "@/features/public-profile/lib/view";

type RenderOptions = {
  username: string;
  lang?: string;
  /** The card or smart link that opened the page (`c/{code}` | `s/{slug}`), already validated. */
  via?: string;
  qr?: boolean;
  /**
   * Where a renamed username goes: the profile URL (default) or, for a smart link rendering the
   * profile in place (TEC-1307), the new username is loaded under the link's own URL.
   */
  followRename?: "redirect" | "inline";
};

/**
 * Server-renders a public profile (TEC-548): /{username} itself, and /s/{slug} smart links that
 * show their target profile in place instead of redirecting with ?via (TEC-1307).
 */
export async function renderPublicProfile({
  username,
  lang,
  via,
  qr = false,
  followRename = "redirect",
}: RenderOptions) {
  let result = await loadPublicProfile(username, qr, via);
  if (result.kind === "legacy_redirect") redirect(result.target);
  if (result.kind === "redirect" && followRename === "inline") {
    result = await loadPublicProfile(result.username, qr, via);
  }
  if (result.kind === "not_found" || result.kind === "legacy_redirect")
    notFound();
  if (result.kind === "redirect") {
    const query = new URLSearchParams();
    if (lang) query.set("lang", lang);
    if (via) query.set("via", via);
    const qs = query.toString();
    permanentRedirect(`/${result.username}${qs ? `?${qs}` : ""}`);
  }
  if (result.kind === "not_shared") return <ProfileNotShared />;
  // Cards and smart links attribute leads and exchanges to themselves (TEC-552, TEC-940).
  const [leadForm, exchange] = await Promise.all([
    loadLeadForm(result.profile.username, result.profile.profile_uuid, via),
    // Contact exchange (TEC-567): the profile's own switch; `via` only attributes it.
    loadExchangeForm(result.profile.username, result.profile.profile_uuid, via),
  ]);
  const view = (
    <PublicProfileView
      profile={result.profile}
      lang={lang}
      leadForm={leadForm}
      exchange={exchange}
      via={via}
    />
  );
  // The person's other open profiles (TEC-1326); one open profile renders as before.
  const siblings = result.profile.siblings ?? [];
  if (siblings.length < 2) return view;
  return (
    <ProfileSwipe
      siblings={siblings}
      current={result.profile.username}
      raised={
        resolveLayout(getTheme(result.profile.theme)) === "legacy_comcode"
      }
    >
      {view}
    </ProfileSwipe>
  );
}

/** Page metadata of a public profile; the canonical URL is always /{username}. */
export async function publicProfileMetadata({
  username,
  lang,
  via,
  qr = false,
}: Omit<RenderOptions, "followRename">): Promise<Metadata> {
  // Same arguments as the page, so React cache shares one upstream call.
  const result = await loadPublicProfile(username, qr, via);
  if (result.kind !== "ok")
    return { title: brand.productName, robots: { index: false } };

  const profile = result.profile;
  const tr = pickTranslation(profile, lang);
  const name = displayName(tr) || profile.username;
  const sub = headline(tr);
  const description = (tr?.bio || sub || brand.tagline).slice(0, 180);
  const base = process.env.AUTH_URL || process.env.PUBLIC_FRONTEND_URL;

  return {
    metadataBase: base ? new URL(base) : undefined,
    title: sub ? `${name} — ${sub}` : name,
    description,
    alternates: { canonical: `/${profile.username}` },
    openGraph: {
      type: "profile",
      title: name,
      description,
      url: `/${profile.username}`,
      siteName: brand.productName,
      images: profile.avatar_url
        ? [{ url: profile.avatar_url, alt: name }]
        : undefined,
    },
    twitter: {
      card: "summary",
      title: name,
      description,
      images: profile.avatar_url ? [profile.avatar_url] : undefined,
    },
  };
}
