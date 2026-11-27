"use client";

import { Building2, Globe, MapPin, Navigation, Phone } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { PublicBankAccount } from "@/features/public-profile/components/public-bank-account";
import { brandStyle } from "@/features/public-profile/lib/brand";
import { fontProfileSerif } from "@/features/public-profile/lib/theme-fonts";
import { getTheme, themeStyle } from "@/features/public-profile/lib/themes";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

import { buttonHref, buttonIconElement, officeMapsHref } from "../lib/blocks";
import type {
  BusinessBlockText,
  PublicBusinessBlock,
  PublicBusinessPage,
} from "../services/business-page.service";

/** Visitor events of a business page (`page.view`, `button.click`, ...), sent by the caller. */
export type BusinessPageEvent = (
  type: string,
  properties?: Record<string, string>,
) => void;

type Props = {
  page: PublicBusinessPage;
  /** Locale of the texts; falls back to the page's primary locale. */
  locale: string;
  /** Locales the visitor can switch to (two or more show the switch). */
  locales?: string[];
  onLocaleChange?: (locale: string) => void;
  /** Opens the page's lead form (a `lead_form` button); the form itself is the caller's. */
  onLeadForm?: () => void;
  /** Rendered under the blocks, e.g. the lead form. */
  children?: ReactNode;
  onEvent?: BusinessPageEvent;
  /** Panel preview: the backdrop stays inside the frame and links open in a new tab. */
  preview?: boolean;
};

function text(
  translations: Record<string, BusinessBlockText> | undefined,
  locale: string,
  fallback: string,
): BusinessBlockText {
  return (
    translations?.[locale] ??
    translations?.[fallback] ??
    Object.values(translations ?? {})[0] ?? { title: "", subtitle: "" }
  );
}

/**
 * ComCode Business page (TEC-1327): the company's header, large action buttons, social icons,
 * offices, team and bank accounts, in the order the company set. Drawn with the page theme (or the
 * company branding theme) and the brand color, like employee profile pages. Every text sits on a
 * themed surface, so contrast does not depend on the backdrop.
 */
export function BusinessPageView({
  page,
  locale,
  locales = [],
  onLocaleChange,
  onLeadForm,
  children,
  onEvent,
  preview = false,
}: Props) {
  const { t } = useLocale();
  const theme = getTheme(page.theme);
  const primary = page.primary_locale;
  const pageText = page.translations[locale] ??
    page.translations[primary] ?? { name: "", tagline: "" };
  const name = pageText.name || page.company_name;
  const target = preview ? "_blank" : undefined;
  const surface = cn(
    "bg-card text-card-foreground rounded-3xl border shadow-sm",
    theme?.glass && "backdrop-blur-xl",
  );
  const headingStyle = theme
    ? {
        fontFamily: "var(--pp-font-heading)",
        fontWeight: "var(--pp-heading-weight)",
      }
    : undefined;
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase())
    .join("");

  const sectionTitle = (b: PublicBusinessBlock, key: string) =>
    text(b.translations, locale, primary).title || t(key);

  // Panel preview: an empty list still shows where the section goes.
  const emptySection = (b: PublicBusinessBlock, i: number, key: string) =>
    preview ? (
      <section key={i} className={cn(surface, "p-5")}>
        <h2 className="text-muted-foreground mb-2 text-xs font-semibold tracking-wide uppercase">
          {sectionTitle(b, key)}
        </h2>
        <p className="text-muted-foreground text-sm">
          {t("public_profile.business.preview_empty")}
        </p>
      </section>
    ) : null;

  const renderBlock = (b: PublicBusinessBlock, i: number) => {
    switch (b.type) {
      case "header":
        return (
          <header key={i} className={cn(surface, "relative overflow-hidden")}>
            <div
              className="h-28 w-full bg-cover bg-center"
              style={
                page.cover_url
                  ? { backgroundImage: `url("${page.cover_url}")` }
                  : { background: theme?.cover ?? "var(--comcode-gradient)" }
              }
            />
            <div className="flex flex-col items-center px-5 pb-5 text-center">
              <Avatar className="bg-card -mt-10 size-20 rounded-2xl border-4 border-[var(--card)] shadow-md">
                {page.logo_url ? (
                  <AvatarImage
                    src={page.logo_url}
                    alt=""
                    className="object-contain p-1"
                  />
                ) : null}
                <AvatarFallback className="rounded-2xl text-lg font-semibold">
                  {initials || <Building2 className="size-6" />}
                </AvatarFallback>
              </Avatar>
              <h1 className="mt-3 text-2xl font-bold" style={headingStyle}>
                {name}
              </h1>
              {pageText.tagline ? (
                <p className="text-muted-foreground mt-1 text-sm">
                  {pageText.tagline}
                </p>
              ) : null}
            </div>
          </header>
        );
      case "button": {
        if (!b.button) return null;
        const label = text(b.translations, locale, primary);
        const title =
          label.title || t(`public_profile.business.target.${b.button.target}`);
        const body = (
          <>
            <span className="bg-primary text-primary-foreground flex size-10 shrink-0 items-center justify-center rounded-full">
              {buttonIconElement(b.button.target, b.button.icon, "size-5")}
            </span>
            <span className="min-w-0 flex-1 text-left">
              <span className="block truncate font-semibold">{title}</span>
              {label.subtitle ? (
                <span className="text-muted-foreground block truncate text-xs">
                  {label.subtitle}
                </span>
              ) : null}
            </span>
          </>
        );
        const className = cn(
          surface,
          "hover:border-primary flex w-full items-center gap-3 rounded-2xl px-4 py-3 transition-colors",
        );
        const click = () =>
          onEvent?.("business.button.click", { target: b.button!.target });
        if (b.button.target === "lead_form") {
          // Without a live form (quota 0, not saved yet) the public page leaves the button out.
          if (!onLeadForm && !preview) return null;
          return (
            <button
              key={i}
              type="button"
              className={className}
              onClick={() => {
                click();
                onLeadForm?.();
              }}
            >
              {body}
            </button>
          );
        }
        const href = buttonHref(b.button.target, b.button.value);
        if (!href) return null;
        const external = /^https?:/.test(href);
        return (
          <a
            key={i}
            href={href}
            target={external || preview ? "_blank" : undefined}
            rel="noopener noreferrer"
            className={className}
            onClick={click}
          >
            {body}
          </a>
        );
      }
      case "social":
        if (!b.social?.length) return null;
        return (
          <div key={i} className="flex flex-wrap justify-center gap-2">
            {b.social.map((s, j) => (
              <a
                key={j}
                href={s.url}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={s.platform}
                className={cn(
                  surface,
                  "hover:border-primary flex size-11 items-center justify-center rounded-full",
                )}
                onClick={() =>
                  onEvent?.("social.link.click", { platform: s.platform })
                }
              >
                {s.icon_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- API-owned icon URL, any host
                  <img src={s.icon_url} alt="" className="size-5" />
                ) : (
                  <Globe className="size-5" />
                )}
              </a>
            ))}
          </div>
        );
      case "offices":
        if (!b.offices?.length)
          return emptySection(b, i, "public_profile.business.offices");
        return (
          <section key={i} className={cn(surface, "p-5")}>
            <h2 className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">
              {sectionTitle(b, "public_profile.business.offices")}
            </h2>
            <ul className="space-y-4">
              {b.offices.map((o, j) => {
                const area = [o.district, o.city].filter(Boolean).join(" / ");
                return (
                  <li key={j} className="flex gap-3">
                    <MapPin className="text-muted-foreground mt-0.5 size-4 shrink-0" />
                    <div className="min-w-0 flex-1 text-sm">
                      <p className="font-medium">{o.name}</p>
                      {o.address ? (
                        <p className="text-muted-foreground">{o.address}</p>
                      ) : null}
                      {area ? (
                        <p className="text-muted-foreground">{area}</p>
                      ) : null}
                      <div className="mt-1.5 flex flex-wrap gap-3 text-xs font-medium">
                        <a
                          href={officeMapsHref([o.address, o.district, o.city])}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-primary inline-flex items-center gap-1 hover:underline"
                          onClick={() => onEvent?.("contact.address.click")}
                        >
                          <Navigation className="size-3.5" />
                          {t("public_profile.business.directions")}
                        </a>
                        {o.phone ? (
                          <a
                            href={`tel:${o.phone.replace(/[^\d+]/g, "")}`}
                            className="text-primary inline-flex items-center gap-1 hover:underline"
                            onClick={() => onEvent?.("contact.phone.click")}
                          >
                            <Phone className="size-3.5" />
                            {o.phone}
                          </a>
                        ) : null}
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          </section>
        );
      case "team":
        if (!b.team?.length)
          return emptySection(b, i, "public_profile.business.team");
        return (
          <section key={i} className={cn(surface, "p-5")}>
            <h2 className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">
              {sectionTitle(b, "public_profile.business.team")}
            </h2>
            <ul className="grid grid-cols-2 gap-3">
              {b.team.map((m) => {
                const full = `${m.first_name} ${m.last_name}`.trim();
                return (
                  <li key={m.username}>
                    <Link
                      href={`/${m.username}`}
                      prefetch={false}
                      target={target}
                      className="hover:border-primary flex h-full flex-col items-center gap-2 rounded-2xl border p-3 text-center"
                    >
                      <Avatar className="size-12">
                        {m.avatar_url ? (
                          <AvatarImage src={m.avatar_url} alt="" />
                        ) : null}
                        <AvatarFallback>
                          {`${m.first_name[0] ?? ""}${m.last_name[0] ?? ""}`.toUpperCase()}
                        </AvatarFallback>
                      </Avatar>
                      <span className="text-sm font-medium">{full}</span>
                      {m.title ? (
                        <span className="text-muted-foreground -mt-1 text-xs">
                          {m.title}
                        </span>
                      ) : null}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </section>
        );
      case "bank_accounts":
        if (!b.bank_accounts?.length)
          return emptySection(b, i, "public_profile.bank_accounts");
        return (
          <section key={i} className={cn(surface, "p-5")}>
            <h2 className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">
              {sectionTitle(b, "public_profile.bank_accounts")}
            </h2>
            <div className="space-y-3">
              {b.bank_accounts.map((a, j) => (
                <PublicBankAccount
                  key={j}
                  account={a}
                  onIbanCopy={() => onEvent?.("bank.iban.copy")}
                />
              ))}
            </div>
          </section>
        );
      default:
        return null;
    }
  };

  return (
    <div
      className={cn(
        "relative",
        preview ? "min-h-full" : "min-h-dvh",
        theme ? fontProfileSerif.variable : "bg-background",
      )}
      style={{
        ...(theme
          ? { ...themeStyle(theme), backgroundColor: theme.backdropBase }
          : {}),
        ...brandStyle(page.primary_color),
      }}
      data-business-theme={theme?.id}
    >
      <div
        aria-hidden
        className={cn(
          "pointer-events-none inset-0",
          preview ? "absolute" : "fixed",
          !theme &&
            "from-primary/15 via-background to-background bg-gradient-to-b",
        )}
        style={theme ? { background: "var(--pp-backdrop)" } : undefined}
      />
      <main className="relative mx-auto flex w-full max-w-md flex-col gap-3 px-4 py-6">
        {locales.length > 1 ? (
          <nav
            aria-label={t("public_profile.language")}
            className="bg-card flex self-end rounded-full border p-0.5 text-xs font-medium"
          >
            {locales.map((loc) => (
              <button
                key={loc}
                type="button"
                onClick={() => onLocaleChange?.(loc)}
                aria-pressed={loc === locale}
                className={cn(
                  "rounded-full px-2.5 py-1 uppercase",
                  loc === locale
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground",
                )}
              >
                {loc}
              </button>
            ))}
          </nav>
        ) : null}
        {page.blocks.map(renderBlock)}
        {children}
        <footer className="mt-5 text-center">
          <Link
            href="/"
            prefetch={false}
            target={target}
            className="bg-card text-muted-foreground hover:text-foreground inline-block rounded-full px-3 py-1 text-xs"
          >
            {t("public_profile.made_with")}
          </Link>
        </footer>
      </main>
    </div>
  );
}
