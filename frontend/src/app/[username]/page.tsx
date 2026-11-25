import type { Metadata } from "next";

import { VIA_RE } from "@/features/leads/lib/via";
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
  // Cards still redirect here with ?via=c/{code} (TEC-552, TEC-940); the view drops it from the
  // address bar once read (TEC-1307).
  return renderPublicProfile({
    username,
    lang,
    via: via && VIA_RE.test(via) ? via : undefined,
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
  return publicProfileMetadata({
    username,
    lang,
    via: via && VIA_RE.test(via) ? via : undefined,
    qr: src === "qr",
  });
}
