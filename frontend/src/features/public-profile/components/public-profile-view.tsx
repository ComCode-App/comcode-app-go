"use client";

import {
  Building2,
  Copy,
  Download,
  Globe,
  Mail,
  MapPin,
  MessageCircle,
  Phone,
  Share2,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, type ReactNode } from "react";

import { useConstrainedDevice } from "@/features/background-studio/lib/render-budget";
import { PublicProfileBackground } from "@/features/public-profile/components/public-profile-background";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { ContactExchange } from "@/features/leads/components/contact-exchange";
import { LeadCaptureForm } from "@/features/leads/components/lead-capture-form";
import { VerifiedBadge } from "@/features/profiles/components/verified-badge";
import type { ExchangeForm, LeadForm } from "@/features/leads/lib/form-config";
import type { PublicProfile } from "@/features/public-profile/lib/load";
import { fontProfileSerif } from "@/features/public-profile/lib/theme-fonts";
import { brandStyle } from "@/features/public-profile/lib/brand";
import { getTheme, resolveLayout, themeStyle, type LayoutMode } from "@/features/public-profile/lib/themes";
import { trackProfileEvent } from "@/features/public-profile/lib/track";
import {
  addressLines,
  contactEvent,
  contactHref,
  displayName,
  formatIban,
  headline,
  mapsHref,
  pickTranslation,
  type PublicContact,
} from "@/features/public-profile/lib/view";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const CONTACT_ICONS: Record<string, LucideIcon> = {
  phone: Phone,
  email: Mail,
  url: Globe,
  whatsapp: MessageCircle,
};

/** Contact category behind each smart link primary action (save_contact needs none). */
const PRIMARY_ACTION_CATEGORY: Record<string, string> = {
  call: "phone",
  email: "email",
  whatsapp: "whatsapp",
  website: "url",
};

type Props = {
  profile: PublicProfile;
  lang?: string;
  /** Lead capture form (TEC-552); null when the page's form is off. */
  leadForm?: LeadForm | null;
  /** Contact exchange (TEC-567); null when the page does not offer it. */
  exchange?: ExchangeForm | null;
  /** Card or smart link that opened the page (`c/{code}` / `s/{slug}`); sent with visitor events (TEC-940). */
  via?: string;
};

/** Card chrome per layout mode (TEC-601). */
const SURFACE: Record<LayoutMode, string> = {
  standard: "rounded-3xl border shadow-sm",
  compact: "rounded-2xl border shadow-sm",
  minimal: "rounded-3xl",
};
const SECTION: Record<LayoutMode, string> = {
  standard: "mt-4 p-5",
  compact: "mt-3 p-4",
  minimal: "mt-3 p-6",
};

/**
 * Visitor page for comcode.../{username}. Hidden modules arrive empty from the API.
 * The theme (TEC-601) sets the tokens on the root; a Background Studio design replaces only the
 * theme's backdrop.
 */
export function PublicProfileView({ profile, lang, leadForm, exchange, via }: Props) {
  const { t } = useLocale();
  const theme = getTheme(profile.theme);
  const layout = resolveLayout(theme, profile.layout_mode);
  // Backdrop blur is costly on weak phones; glass cards are opaque enough without it.
  const constrained = useConstrainedDevice();
  const surface = cn("bg-card", SURFACE[layout], theme?.glass && !constrained && "backdrop-blur-xl");
  const section = cn(surface, SECTION[layout]);
  const headingStyle = theme
    ? { fontFamily: "var(--pp-font-heading)", fontWeight: "var(--pp-heading-weight)" }
    : undefined;
  const tr = pickTranslation(profile, lang);
  const name = displayName(tr);
  const subline = headline(tr);
  const activeLocale = tr?.locale ?? profile.primary_locale;
  const locales = Array.from(new Set(profile.translations.map((x) => x.locale)));
  const track = (type: string, props?: Record<string, string>) =>
    trackProfileEvent(type, profile.profile_uuid, via ? { ...props, via } : props);

  const viewed = useRef(false);
  useEffect(() => {
    if (viewed.current) return;
    viewed.current = true;
    trackProfileEvent("card.view", profile.profile_uuid, {
      username: profile.username,
      ...(via ? { via } : {}),
    });
  }, [profile.profile_uuid, profile.username, via]);

  const first = (category: string) => profile.contacts.find((c) => c.category === category);
  const quick = [first("phone"), first("email"), first("whatsapp")].filter(
    (c): c is PublicContact => Boolean(c),
  );
  const vcardHref = `/api/v1/public/p/${encodeURIComponent(profile.username)}/vcard?locale=${activeLocale}`;
  // The smart link that opened the page may lead with another action (TEC-556); without a
  // matching contact the main button stays "save contact".
  const primaryContact = profile.primary_action ? first(PRIMARY_ACTION_CATEGORY[profile.primary_action] ?? "") : undefined;
  const primaryHref = primaryContact ? contactHref(primaryContact) : null;
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");

  const share = async () => {
    const url = window.location.href.split("?")[0];
    try {
      if (navigator.share) {
        await navigator.share({ title: name, url });
        return;
      }
      await navigator.clipboard.writeText(url);
      appToast.success(t("public_profile.link_copied"));
    } catch {
      // the visitor closed the share sheet
    }
  };

  const languageNav =
    locales.length > 1 ? (
      <nav
        aria-label={t("public_profile.language")}
        className="bg-card absolute top-3 right-3 z-10 flex rounded-full border p-0.5 text-xs font-medium"
      >
        {locales.map((loc) => (
          <Link
            key={loc}
            href={`?lang=${loc}`}
            replace
            scroll={false}
            className={cn(
              "rounded-full px-2.5 py-1 uppercase",
              loc === activeLocale ? "bg-primary text-primary-foreground" : "text-muted-foreground",
            )}
          >
            {loc}
          </Link>
        ))}
      </nav>
    ) : null;

  const cover = (
    <div className="relative h-36 sm:h-44">
      {profile.cover_url ? (
        // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, any host
        <img src={profile.cover_url} alt="" className="h-full w-full object-cover" fetchPriority="high" />
      ) : theme ? (
        <div className="h-full w-full" style={{ backgroundImage: "var(--pp-cover)" }} />
      ) : (
        <div className="from-primary h-full w-full bg-gradient-to-br to-purple-700" />
      )}
      {profile.company_logo_url ? (
        // On a pill so the company logo (TEC-946) stays readable over any cover.
        <div className="bg-card absolute top-3 left-3 z-10 rounded-lg border p-1.5 shadow-sm">
          {/* eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, any host */}
          <img src={profile.company_logo_url} alt="" className="h-8 max-w-28 object-contain" />
        </div>
      ) : null}
      {languageNav}
    </div>
  );

  const avatar = (size: string, text: string, ring: boolean) => (
    <Avatar className={cn(size, ring && "border-card border-4 shadow-md")}>
      {profile.avatar_url ? <AvatarImage src={profile.avatar_url} alt={name} /> : null}
      <AvatarFallback className={text}>{initials || "?"}</AvatarFallback>
    </Avatar>
  );

  const nameBlock = (align: "center" | "left") => (
    <div className={cn("min-w-0", align === "center" && "text-center")}>
      <h1
        className={cn("font-semibold tracking-tight", layout === "compact" ? "text-xl" : "mt-3 text-2xl")}
        style={headingStyle}
      >
        {name}
        {profile.verified ? <VerifiedBadge className="ml-1.5 -translate-y-px text-[0.85em]" /> : null}
      </h1>
      {subline ? <p className="text-muted-foreground mt-1 text-sm">{subline}</p> : null}
      {tr?.department ? <p className="text-muted-foreground text-xs">{tr.department}</p> : null}
    </div>
  );

  return (
    <div
      className={cn("min-h-dvh", theme ? fontProfileSerif.variable : "bg-background")}
      style={{
        ...(theme ? { ...themeStyle(theme), backgroundColor: theme.backdropBase } : {}),
        // Company brand color (TEC-946) over the theme's primary.
        ...brandStyle(profile.primary_color),
      }}
      data-profile-theme={theme?.id}
      data-profile-layout={layout}
    >
      {profile.background ? (
        <PublicProfileBackground
          document={profile.background}
          thumbnailUrl={profile.background_thumbnail_url}
        />
      ) : theme ? (
        <div aria-hidden className="pointer-events-none fixed inset-0" style={{ background: "var(--pp-backdrop)" }} />
      ) : (
        <div
          aria-hidden
          className="from-primary/15 via-background to-background pointer-events-none fixed inset-0 bg-gradient-to-b"
        />
      )}
      <main className={cn("relative mx-auto w-full max-w-md px-4 pb-10", layout === "minimal" ? "pt-8" : "pt-4")}>
        <article className={cn(surface, "relative overflow-hidden")}>
          {layout === "standard" ? (
            <>
              {cover}
              <div className="-mt-12 flex flex-col items-center px-5">
                {avatar("size-24", "text-2xl", true)}
                {nameBlock("center")}
              </div>
            </>
          ) : layout === "compact" ? (
            <div className="flex items-center gap-4 px-4 pt-4 pr-20">
              {languageNav}
              {avatar("size-16 shrink-0", "text-lg", false)}
              {nameBlock("left")}
            </div>
          ) : (
            <div className="flex flex-col items-center px-6 pt-8">
              {languageNav}
              {avatar("size-20", "text-xl", false)}
              {nameBlock("center")}
            </div>
          )}

          <div className={cn("flex flex-col items-center text-center", layout === "compact" ? "px-4 pb-4" : "px-5 pb-6")}>
            <div className={cn("flex w-full flex-col gap-2", layout === "compact" ? "mt-4" : "mt-5")}>
              {primaryContact && primaryHref ? (
                <Button asChild size="lg" className="w-full">
                  <a
                    href={primaryHref}
                    target={primaryContact.category === "phone" || primaryContact.category === "email" ? undefined : "_blank"}
                    rel="noopener noreferrer"
                    onClick={() => {
                      const event = contactEvent(primaryContact);
                      if (event) track(event);
                    }}
                  >
                    {(() => {
                      const Icon = CONTACT_ICONS[primaryContact.category] ?? Globe;
                      return <Icon className="size-4" />;
                    })()}
                    {t(`public_profile.primary_action.${profile.primary_action}`)}
                  </a>
                </Button>
              ) : null}
              <Button asChild size="lg" variant={primaryHref ? "outline" : "default"} className="w-full">
                <a href={vcardHref} onClick={() => track("vcard.download", { locale: activeLocale })}>
                  <Download className="size-4" />
                  {t("public_profile.save_contact")}
                </a>
              </Button>
              <div className="flex justify-center gap-2">
                {quick.map((c) => {
                  const Icon = CONTACT_ICONS[c.category] ?? Globe;
                  const href = contactHref(c);
                  const event = contactEvent(c);
                  if (!href) return null;
                  return (
                    <Button key={`${c.category}-${c.value}`} asChild variant="outline" size="icon" className="size-11 rounded-full">
                      <a
                        href={href}
                        target={c.category === "whatsapp" ? "_blank" : undefined}
                        rel="noopener noreferrer"
                        aria-label={t(`public_profile.category.${c.category}`)}
                        onClick={() => event && track(event)}
                      >
                        <Icon className="size-5" />
                      </a>
                    </Button>
                  );
                })}
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  className="size-11 rounded-full"
                  aria-label={t("public_profile.share")}
                  onClick={() => void share()}
                >
                  <Share2 className="size-5" />
                </Button>
              </div>
            </div>
          </div>
        </article>

        {exchange ? (
          <Section className={section} title={t("leads.exchange.title")}>
            <ContactExchange
              exchange={exchange}
              ownerName={name || profile.username}
              locale={activeLocale}
              vcardHref={vcardHref}
              onSaveContact={() => track("vcard.download", { locale: activeLocale })}
            />
          </Section>
        ) : null}

        {leadForm ? (
          <Section className={section} title={t("leads.form.title")}>
            <LeadCaptureForm
              path={leadForm.path}
              consentVersion={leadForm.consentVersion}
              via={leadForm.via}
              ownerName={name || profile.username}
              locale={activeLocale}
            />
          </Section>
        ) : null}

        {tr?.bio ? (
          <Section className={section} title={t("public_profile.about")}>
            <p className="text-sm leading-relaxed whitespace-pre-line">{tr.bio}</p>
          </Section>
        ) : null}

        {profile.contacts.length > 0 ? (
          <Section className={section} title={t("public_profile.contact")}>
            <ul className="-my-1 divide-y">
              {profile.contacts.map((c, i) => {
                const Icon = CONTACT_ICONS[c.category] ?? Globe;
                const href = contactHref(c);
                const event = contactEvent(c);
                const body = (
                  <>
                    <span className="bg-muted text-foreground flex size-9 shrink-0 items-center justify-center rounded-full">
                      <Icon className="size-4" />
                    </span>
                    <span className="min-w-0 text-left">
                      <span className="text-muted-foreground block text-xs">
                        {c.label || t(`public_profile.category.${c.category}`)}
                      </span>
                      <span className="block truncate text-sm font-medium">{c.value}</span>
                    </span>
                  </>
                );
                return (
                  <li key={`${c.category}-${i}`}>
                    {href ? (
                      <a
                        href={href}
                        target={c.category === "url" || c.category === "whatsapp" ? "_blank" : undefined}
                        rel="noopener noreferrer"
                        onClick={() => event && track(event)}
                        className="hover:bg-muted/60 -mx-2 flex items-center gap-3 rounded-lg px-2 py-2"
                      >
                        {body}
                      </a>
                    ) : (
                      <div className="flex items-center gap-3 py-2">{body}</div>
                    )}
                  </li>
                );
              })}
            </ul>
          </Section>
        ) : null}

        {profile.social_links.length > 0 ? (
          <Section className={section} title={t("public_profile.social")}>
            <ul className="grid grid-cols-4 gap-3">
              {profile.social_links.map((link, i) => (
                <li key={`${link.platform}-${i}`}>
                  <a
                    href={link.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={() => track("social.link.click", { platform: link.platform })}
                    className="hover:bg-muted/60 flex flex-col items-center gap-1.5 rounded-xl p-2 text-center"
                  >
                    <span className="bg-muted flex size-11 items-center justify-center overflow-hidden rounded-full">
                      {link.icon_url ? (
                        // eslint-disable-next-line @next/next/no-img-element -- platform icons are API-owned
                        <img src={link.icon_url} alt="" className="size-6 object-contain" loading="lazy" />
                      ) : (
                        <Globe className="size-5" />
                      )}
                    </span>
                    <span className="text-muted-foreground w-full truncate text-[11px] capitalize">
                      {link.label || link.platform}
                    </span>
                  </a>
                </li>
              ))}
            </ul>
          </Section>
        ) : null}

        {profile.addresses.length > 0 ? (
          <Section className={section} title={t("public_profile.addresses")}>
            <ul className="space-y-3">
              {profile.addresses.map((a, i) => (
                <li key={i} className="flex gap-3">
                  <span className="bg-muted flex size-9 shrink-0 items-center justify-center rounded-full">
                    <MapPin className="size-4" />
                  </span>
                  <div className="min-w-0 text-sm">
                    {a.label ? <p className="text-muted-foreground text-xs">{a.label}</p> : null}
                    {addressLines(a).map((line) => (
                      <p key={line}>{line}</p>
                    ))}
                    <a
                      href={mapsHref(a)}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={() => track("contact.address.click")}
                      className="text-primary mt-1 inline-block text-xs font-medium"
                    >
                      {t("public_profile.open_in_maps")}
                    </a>
                  </div>
                </li>
              ))}
            </ul>
          </Section>
        ) : null}

        {profile.bank_accounts.length > 0 ? (
          <Section className={section} title={t("public_profile.bank_accounts")}>
            <ul className="space-y-3">
              {profile.bank_accounts.map((b, i) => (
                <li key={`${b.iban}-${i}`} className="rounded-xl border p-3">
                  <div className="flex items-center gap-2">
                    <span className="bg-muted flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-md">
                      {b.bank_icon_url ? (
                        // eslint-disable-next-line @next/next/no-img-element -- bank icons are API-owned
                        <img src={b.bank_icon_url} alt="" className="size-6 object-contain" loading="lazy" />
                      ) : (
                        <Building2 className="size-4" />
                      )}
                    </span>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{b.bank_name || b.title}</p>
                      <p className="text-muted-foreground truncate text-xs">{b.account_holder}</p>
                    </div>
                  </div>
                  <div className="mt-2 flex items-center gap-2">
                    <code className="bg-muted min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-xs">
                      {formatIban(b.iban)}
                    </code>
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      aria-label={t("public_profile.copy_iban")}
                      onClick={() => {
                        void navigator.clipboard
                          .writeText(b.iban.replace(/\s+/g, ""))
                          .then(() => appToast.success(t("public_profile.iban_copied")));
                        track("bank.iban.copy");
                      }}
                    >
                      <Copy className="size-4" />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          </Section>
        ) : null}

        <footer className="mt-8 text-center">
          {/* On a pill so it stays readable over any theme backdrop or Background Studio design. */}
          {/* No prefetch: "/" is the panel; visitors should not download its JS (TEC-858). */}
          <Link
            href="/"
            prefetch={false}
            className="bg-card text-muted-foreground hover:text-foreground inline-block rounded-full px-3 py-1 text-xs"
          >
            {t("public_profile.made_with")}
          </Link>
        </footer>
      </main>
    </div>
  );
}

function Section({ title, className, children }: { title: string; className: string; children: ReactNode }) {
  return (
    <section className={className}>
      <h2 className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">{title}</h2>
      {children}
    </section>
  );
}
