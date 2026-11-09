import type { Metadata } from "next";
import { notFound, permanentRedirect, redirect } from "next/navigation";

import { brand } from "@/config/brand";
import {
  loadExchangeForm,
  loadLeadForm,
} from "@/features/leads/lib/form-config";
import { VIA_RE } from "@/features/leads/lib/via";
import { PublicProfileView } from "@/features/public-profile";
import { loadPublicProfile } from "@/features/public-profile/lib/load";
import {
  displayName,
  headline,
  pickTranslation,
} from "@/features/public-profile/lib/view";

type PageProps = {
  params: Promise<{ username: string }>;
  searchParams: Promise<{ lang?: string; via?: string; src?: string }>;
};

/**
 * Public profile at comcode.../{username} (TEC-548). Static app routes (/login, /cards, ...)
 * win over this dynamic segment; the backend refuses usernames that collide with them.
 */
export default async function PublicProfilePage({
  params,
  searchParams,
}: PageProps) {
  const [{ username }, { lang, via, src }] = await Promise.all([
    params,
    searchParams,
  ]);
  const validVia = via && VIA_RE.test(via) ? via : undefined;
  const result = await loadPublicProfile(username, src === "qr", validVia);
  if (result.kind === "legacy_redirect") redirect(result.target);
  if (result.kind === "not_found") notFound();
  if (result.kind === "redirect") {
    const query = new URLSearchParams();
    if (lang) query.set("lang", lang);
    if (via) query.set("via", via);
    const qs = query.toString();
    permanentRedirect(`/${result.username}${qs ? `?${qs}` : ""}`);
  }
  // Cards and smart links always append ?via=c/{code} | s/{slug} (TEC-552, TEC-940).
  const [leadForm, exchange] = await Promise.all([
    loadLeadForm(result.profile.username, result.profile.profile_uuid, via),
    // Contact exchange (TEC-567): the profile's own switch; `via` only attributes it.
    loadExchangeForm(result.profile.username, result.profile.profile_uuid, via),
  ]);
  return (
    <PublicProfileView
      profile={result.profile}
      lang={lang}
      leadForm={leadForm}
      exchange={exchange}
      via={validVia}
    />
  );
}

export async function generateMetadata({
  params,
  searchParams,
}: PageProps): Promise<Metadata> {
  const [{ username }, { lang, src, via }] = await Promise.all([
    params,
    searchParams,
  ]);
  // Same arguments as the page, so React cache shares one upstream call.
  const result = await loadPublicProfile(
    username,
    src === "qr",
    via && VIA_RE.test(via) ? via : undefined,
  );
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
