"use client";

import {
  Download,
  Globe,
  Landmark,
  Languages,
  Share2,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { useState, type ReactNode } from "react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { VerifiedBadge } from "@/features/profiles/components/verified-badge";
import type { PublicProfile } from "@/features/public-profile/lib/load";
import {
  contactEvent,
  contactHref,
  type PublicContact,
} from "@/features/public-profile/lib/view";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/** A bio this short reads as a tagline under the name; longer bios go to the profile tab. */
export const TAGLINE_MAX = 90;

export function isTagline(bio: string | undefined): boolean {
  const text = bio?.trim() ?? "";
  return text.length > 0 && text.length <= TAGLINE_MAX && !text.includes("\n");
}

const PANEL_ID = "pp-tabpanel";

type Tab = { id: string; label: string; icon: LucideIcon; content: ReactNode };

type Props = {
  profile: PublicProfile;
  name: string;
  initials: string;
  /** Short bio shown as a tagline (see `isTagline`); "" for none. */
  tagline: string;
  /** "Title · Company" line. */
  subline: string;
  department?: string;
  activeLocale: string;
  locales: string[];
  vcardHref: string;
  /** Smart link primary action button (TEC-556), or null. */
  primaryAction: ReactNode;
  quick: PublicContact[];
  contactIcons: Record<string, LucideIcon>;
  /** Glass surface classes of the template (blur is dropped on constrained devices). */
  surface: string;
  /** Profile tab body: contacts, addresses, forms, long bio. */
  profileTab: ReactNode;
  /** Bank accounts tab body, or null when the page publishes none. */
  bankTab: ReactNode;
  track: (type: string, props?: Record<string, string>) => void;
  onShare: () => void;
};

/**
 * "Comcode V1" layout (`legacy_comcode`, TEC-1309): one glass card with the avatar, name,
 * tagline, company logo, an icon row and the save-contact button, a footer line and a
 * bottom tab bar. The backdrop comes from the template or a Background Studio design; text
 * always sits on a themed surface.
 */
export function LegacyComcodeLayout({
  profile,
  name,
  initials,
  tagline,
  subline,
  department,
  activeLocale,
  locales,
  vcardHref,
  primaryAction,
  quick,
  contactIcons,
  surface,
  profileTab,
  bankTab,
  track,
  onShare,
}: Props) {
  const { t } = useLocale();
  const tabs: Tab[] = [
    {
      id: "profile",
      label: t("public_profile.tab.profile"),
      icon: UserRound,
      content: profileTab,
    },
  ];
  if (bankTab) {
    tabs.push({
      id: "bank_accounts",
      label: t("public_profile.tab.bank_accounts"),
      icon: Landmark,
      content: bankTab,
    });
  }
  const [active, setActive] = useState(tabs[0].id);
  const current = tabs.find((tab) => tab.id === active) ?? tabs[0];
  const nextLocale =
    locales.length > 1
      ? locales[(locales.indexOf(activeLocale) + 1) % locales.length]
      : null;
  const iconLink =
    "text-foreground hover:text-primary flex size-10 items-center justify-center rounded-xl transition-colors";

  return (
    <main
      className={cn(
        "relative mx-auto w-full max-w-md px-4 pt-10",
        tabs.length > 1 ? "pb-28" : "pb-10",
      )}
    >
      <article
        className={cn(
          surface,
          "relative flex flex-col items-center px-5 pt-8 pb-6 text-center",
        )}
      >
        <Avatar className="border-border size-32 border-2 shadow-lg">
          {profile.avatar_url ? (
            <AvatarImage src={profile.avatar_url} alt={name} />
          ) : null}
          <AvatarFallback className="text-3xl">
            {initials || "?"}
          </AvatarFallback>
        </Avatar>

        <h1
          className="mt-5 text-3xl tracking-tight"
          style={{
            fontFamily: "var(--pp-font-heading)",
            fontWeight: "var(--pp-heading-weight)",
          }}
        >
          {name}
          {profile.verified ? (
            <VerifiedBadge className="ml-2 -translate-y-0.5 text-[0.8em]" />
          ) : null}
        </h1>
        {tagline ? (
          <p className="text-muted-foreground mt-2 text-sm">{tagline}</p>
        ) : null}
        {subline ? (
          <p className="mt-3 text-sm font-medium">• {subline} •</p>
        ) : null}
        {department ? (
          <p className="text-muted-foreground mt-1 text-xs">{department}</p>
        ) : null}

        {profile.company_logo_url ? (
          <div className="bg-muted mt-6 flex w-full items-center justify-center rounded-3xl border px-6 py-7">
            {/* eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, any host */}
            <img
              src={profile.company_logo_url}
              alt=""
              className="max-h-24 max-w-full object-contain"
            />
          </div>
        ) : null}

        <ul className="mt-6 flex flex-wrap items-center justify-center gap-0.5">
          {profile.social_links.map((link, i) => (
            <li key={`${link.platform}-${i}`}>
              <a
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={link.label || link.platform}
                onClick={() =>
                  track("social.link.click", { platform: link.platform })
                }
                className={iconLink}
              >
                {link.icon_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- platform icons are API-owned
                  <img
                    src={link.icon_url}
                    alt=""
                    className="size-8 object-contain"
                    loading="lazy"
                  />
                ) : (
                  <Globe className="size-7" />
                )}
              </a>
            </li>
          ))}
          {quick.map((c) => {
            const Icon = contactIcons[c.category] ?? Globe;
            const href = contactHref(c);
            const event = contactEvent(c);
            if (!href) return null;
            return (
              <li key={`${c.category}-${c.value}`}>
                <a
                  href={href}
                  target={c.category === "whatsapp" ? "_blank" : undefined}
                  rel="noopener noreferrer"
                  aria-label={t(`public_profile.category.${c.category}`)}
                  onClick={() => event && track(event)}
                  className={iconLink}
                >
                  <Icon className="size-7" />
                </a>
              </li>
            );
          })}
          <li>
            <button
              type="button"
              aria-label={t("public_profile.share")}
              onClick={onShare}
              className={iconLink}
            >
              <Share2 className="size-7" />
            </button>
          </li>
        </ul>

        {primaryAction ? (
          <div className="mt-5 w-full">{primaryAction}</div>
        ) : null}

        <div
          className={cn(
            "border-primary/60 flex w-full items-stretch overflow-hidden rounded-2xl border-2",
            primaryAction ? "mt-2" : "mt-5",
          )}
        >
          {nextLocale ? (
            <Link
              href={`?lang=${nextLocale}`}
              replace
              scroll={false}
              aria-label={t("public_profile.next_language", {
                locale: nextLocale.toUpperCase(),
              })}
              className="hover:bg-accent border-primary/60 flex w-14 shrink-0 items-center justify-center border-r-2"
            >
              <Languages className="size-5" />
            </Link>
          ) : null}
          <a
            href={vcardHref}
            onClick={() => track("vcard.download", { locale: activeLocale })}
            className="hover:bg-accent flex flex-1 items-center justify-center gap-2 py-3 text-base font-semibold"
          >
            {nextLocale ? null : <Download className="size-4" />}
            {t("public_profile.save_contact")}
          </a>
        </div>
      </article>

      <div
        id={PANEL_ID}
        role="tabpanel"
        aria-label={current.label}
        className="scroll-mt-4"
      >
        {current.content}
      </div>

      <footer className="mt-8 text-center">
        {/* On a pill so it stays readable over any backdrop. No prefetch: "/" is the panel (TEC-858). */}
        <Link
          href="/"
          prefetch={false}
          className="bg-card text-muted-foreground hover:text-foreground inline-block rounded-full px-3 py-1 text-xs"
        >
          {t("public_profile.copyright", {
            year: String(new Date().getFullYear()),
          })}
        </Link>
      </footer>

      {tabs.length > 1 ? (
        <nav
          role="tablist"
          aria-label={t("public_profile.tabs")}
          className={cn(
            surface,
            "fixed inset-x-0 bottom-0 z-20 mx-auto flex max-w-md justify-around rounded-b-none px-2 pt-2 pb-[max(0.5rem,env(safe-area-inset-bottom))]",
          )}
        >
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const selected = tab.id === current.id;
            return (
              <button
                key={tab.id}
                type="button"
                role="tab"
                aria-selected={selected}
                onClick={() => {
                  setActive(tab.id);
                  document
                    .getElementById(PANEL_ID)
                    ?.scrollIntoView({ behavior: "smooth", block: "start" });
                }}
                className={cn(
                  "flex flex-1 flex-col items-center gap-1 rounded-xl px-2 py-1.5 text-xs font-medium transition-colors",
                  selected
                    ? "text-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                <Icon className="size-6" />
                {tab.label}
              </button>
            );
          })}
        </nav>
      ) : null}
    </main>
  );
}
