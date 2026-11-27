import type { Metadata } from "next";
import { permanentRedirect } from "next/navigation";

import { brand } from "@/config/brand";
import { BusinessPagePublic } from "@/features/business-page/components/business-page-public";
import { loadPublicBusiness } from "@/features/business-page/lib/load";
import type { PublicBusinessPage } from "@/features/business-page/services/business-page.service";
import { loadLeadForm } from "@/features/leads/lib/form-config";
import { VIA_RE } from "@/features/leads/lib/via";
import { loadPublicProfile } from "@/features/public-profile/lib/load";
import {
  publicProfileMetadata,
  renderPublicProfile,
} from "@/features/public-profile/lib/page";

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
  const viaPath = via && VIA_RE.test(via) ? via : undefined;
  // A username that is no profile may be a company business page (TEC-1327); both share the
  // routes registry, so a name is one or the other. Same arguments as renderPublicProfile, so
  // React cache keeps it one upstream call (and one QR scan).
  const profile = await loadPublicProfile(username, src === "qr", viaPath);
  if (profile.kind === "not_found") {
    const business = await loadPublicBusiness(username);
    if (business.kind === "redirect")
      permanentRedirect(
        `/${business.username}${lang ? `?lang=${encodeURIComponent(lang)}` : ""}`,
      );
    if (business.kind === "ok") {
      const leadForm = await loadLeadForm(business.page.username, "");
      return (
        <BusinessPagePublic
          page={business.page}
          lang={lang}
          leadForm={leadForm}
        />
      );
    }
  }
  // Cards still redirect here with ?via=c/{code} (TEC-552, TEC-940); the view drops it from the
  // address bar once read (TEC-1307).
  return renderPublicProfile({
    username,
    lang,
    via: viaPath,
    qr: src === "qr",
  });
}

export async function generateMetadata({
  params,
  searchParams,
}: PageProps): Promise<Metadata> {
  const [{ username }, { lang, src, via }] = await Promise.all([
    params,
    searchParams,
  ]);
  const viaPath = via && VIA_RE.test(via) ? via : undefined;
  const profile = await loadPublicProfile(username, src === "qr", viaPath);
  if (profile.kind === "not_found") {
    const business = await loadPublicBusiness(username);
    if (business.kind === "ok") return businessMetadata(business.page, lang);
  }
  return publicProfileMetadata({
    username,
    lang,
    via: viaPath,
    qr: src === "qr",
  });
}

function businessMetadata(page: PublicBusinessPage, lang?: string): Metadata {
  const text =
    (lang ? page.translations[lang] : undefined) ??
    page.translations[page.primary_locale];
  const name = text?.name || page.company_name;
  const description = (text?.tagline || brand.tagline).slice(0, 180);
  const base = process.env.AUTH_URL || process.env.PUBLIC_FRONTEND_URL;
  return {
    metadataBase: base ? new URL(base) : undefined,
    title: name,
    description,
    alternates: { canonical: `/${page.username}` },
    openGraph: {
      type: "website",
      title: name,
      description,
      url: `/${page.username}`,
      siteName: brand.productName,
      images: page.logo_url ? [{ url: page.logo_url, alt: name }] : undefined,
    },
    twitter: { card: "summary", title: name, description },
  };
}
