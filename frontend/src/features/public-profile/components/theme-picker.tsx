"use client";

import { Check } from "lucide-react";
import type { CSSProperties } from "react";

import { Label } from "@/components/ui/label";
import {
  PremiumLockIcon,
  PremiumNotice,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { fontProfileSerif } from "@/features/public-profile/lib/theme-fonts";
import {
  THEME_IDS,
  basicThemeFor,
  getTheme,
  isPremiumTheme,
  resolveLayout,
  themeStyle,
  type LayoutMode,
  type ProfileTheme,
  type ThemeId,
} from "@/features/public-profile/lib/themes";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

type Props = {
  /** The profile's own template; "" = inherit / default look. */
  theme: ThemeId | "";
  /** Company template used when `theme` is "" (TEC-601); "" = none. */
  inheritedTheme: ThemeId | "";
  disabled?: boolean;
  /** Label and hint of the "" option when it means something else (smart links: the profile's own look). */
  noneLabel?: string;
  hint?: string;
  /**
   * Static image of the chosen Background Studio design (its thumbnail). The preview tiles
   * draw the template over it, since the background replaces the template's backdrop.
   */
  backdropImage?: string | null;
  /**
   * Whether premium templates may be picked (`design.premium`, TEC-947). Defaults to the active
   * account's plan; pass it to preview another account's plan.
   */
  premiumAllowed?: boolean;
  onThemeChange: (theme: ThemeId | "") => void;
};

/**
 * Owner-side template gallery for the public page (TEC-601, TEC-1309). A template is a theme
 * with its own layout, so there is no separate density switch; the background is picked
 * separately (Background Studio) and replaces only the template's backdrop.
 * Premium templates the plan lacks show a lock and cannot be picked; a premium template
 * already stored stays selected and the hint says which basic one visitors see instead
 * (TEC-947).
 */
export function ProfileThemePicker({
  theme,
  inheritedTheme,
  disabled,
  noneLabel,
  hint,
  backdropImage,
  premiumAllowed,
  onThemeChange,
}: Props) {
  const { t } = useLocale();
  const planPremium = useDesignPremium();
  const allowPremium = premiumAllowed ?? planPremium;
  const effectiveId = theme || inheritedTheme;
  const shownInstead =
    effectiveId && !allowPremium && isPremiumTheme(effectiveId)
      ? basicThemeFor(effectiveId)
      : null;
  const options: {
    value: ThemeId | "";
    label: string;
    preview: ProfileTheme | null;
  }[] = [
    {
      value: "",
      label:
        noneLabel ??
        (inheritedTheme
          ? t("profiles.public_page.theme_inherit")
          : t("profiles.public_page.theme_none")),
      preview: getTheme(inheritedTheme),
    },
    ...THEME_IDS.map((id) => ({
      value: id,
      label: t(`profiles.public_page.theme_name.${id}`),
      preview: getTheme(id),
    })),
  ];

  return (
    <div className={cn("grid gap-1.5", fontProfileSerif.variable)}>
      <Label id="public-page-theme-label">
        {t("profiles.public_page.theme")}
      </Label>
      <div
        role="radiogroup"
        aria-labelledby="public-page-theme-label"
        className="grid grid-cols-2 gap-3 sm:grid-cols-4"
      >
        {options.map((opt) => {
          const selected = opt.value === theme;
          const locked = !allowPremium && isPremiumTheme(opt.value);
          const layout = resolveLayout(opt.preview);
          return (
            <button
              key={opt.value || "none"}
              type="button"
              role="radio"
              aria-checked={selected}
              aria-disabled={locked && !selected ? true : undefined}
              disabled={disabled}
              onClick={() => {
                if (!locked || selected) onThemeChange(opt.value);
              }}
              className={cn(
                "group relative flex flex-col gap-1.5 rounded-xl border p-1.5 text-left transition-colors",
                "focus-visible:ring-ring/50 outline-none focus-visible:ring-[3px] disabled:opacity-60",
                selected
                  ? "border-primary ring-primary/30 ring-2"
                  : locked
                    ? "cursor-not-allowed opacity-70"
                    : "hover:border-foreground/30",
              )}
            >
              <ThemePreview
                theme={opt.preview}
                layout={layout}
                backdropImage={backdropImage}
              />
              <span className="flex items-center justify-between gap-1 px-1 pb-0.5 text-xs font-medium">
                <span className="min-w-0">
                  <span className="block truncate">{opt.label}</span>
                  <span className="text-muted-foreground block truncate text-[11px] font-normal">
                    {t(`profiles.public_page.layout_name.${layout}`)}
                  </span>
                </span>
                {selected ? (
                  <Check className="text-primary size-3.5 shrink-0" />
                ) : locked ? (
                  <PremiumLockIcon />
                ) : null}
              </span>
            </button>
          );
        })}
      </div>
      <p className="text-muted-foreground text-xs">
        {hint ??
          (!theme && inheritedTheme
            ? t("profiles.public_page.theme_inherit_hint", {
                theme: t(`profiles.public_page.theme_name.${inheritedTheme}`),
              })
            : t("profiles.public_page.theme_hint"))}
      </p>
      {shownInstead ? (
        <p className="text-xs text-amber-700 dark:text-amber-400">
          {t("profiles.public_page.theme_premium_fallback", {
            theme: t(`profiles.public_page.theme_name.${shownInstead}`),
          })}
        </p>
      ) : null}
      {!allowPremium ? <PremiumNotice /> : null}
    </div>
  );
}

/** A miniature public page drawn with the template's own tokens and layout. */
function ThemePreview({
  theme,
  layout,
  backdropImage,
}: {
  theme: ProfileTheme | null;
  layout: LayoutMode;
  backdropImage?: string | null;
}) {
  const style: CSSProperties | undefined = theme
    ? { ...themeStyle(theme), background: theme.backdrop }
    : undefined;
  const backdrop = backdropImage ? (
    // eslint-disable-next-line @next/next/no-img-element -- API-owned thumbnail URL
    <img
      src={backdropImage}
      alt=""
      className="absolute inset-0 size-full object-cover"
      loading="lazy"
    />
  ) : null;
  const heading = theme
    ? {
        fontFamily: "var(--pp-font-heading)",
        fontWeight: "var(--pp-heading-weight)",
      }
    : undefined;

  if (layout === "legacy_comcode") {
    return (
      <div
        aria-hidden
        style={style}
        className={cn(
          "relative flex h-28 flex-col overflow-hidden rounded-lg",
          !theme && "bg-muted",
        )}
      >
        {backdrop}
        <div className="bg-card relative mx-auto mt-2 flex w-[72%] flex-col items-center gap-1 rounded-lg border p-1.5 backdrop-blur-sm">
          <span className="bg-muted size-6 rounded-full border" />
          <span
            className="text-foreground text-[9px] leading-none"
            style={heading}
          >
            Aa Bb
          </span>
          <span className="bg-muted h-2.5 w-full rounded border" />
          <span className="flex gap-0.5">
            {[0, 1, 2, 3].map((i) => (
              <span key={i} className="bg-foreground/70 size-1.5 rounded-sm" />
            ))}
          </span>
          <span className="border-primary/60 h-2 w-full rounded border" />
        </div>
        <div className="bg-card relative mt-auto flex justify-around border-t px-2 py-1">
          {[0, 1, 2].map((i) => (
            <span
              key={i}
              className={cn(
                "size-1.5 rounded-full",
                i === 0 ? "bg-foreground" : "bg-muted-foreground/50",
              )}
            />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div
      aria-hidden
      style={style}
      className={cn(
        "relative flex h-28 flex-col justify-center overflow-hidden rounded-lg p-2",
        !theme && "bg-muted",
      )}
    >
      {backdrop}
      <div
        className={cn(
          "bg-card relative overflow-hidden",
          layout === "compact" ? "rounded-md" : "rounded-lg",
          layout !== "minimal" && "border",
        )}
      >
        {layout === "standard" ? (
          <div
            className="h-6"
            style={{
              backgroundImage: theme
                ? "var(--pp-cover)"
                : "linear-gradient(135deg, var(--primary), #7e22ce)",
            }}
          />
        ) : null}
        <div
          className={cn(
            "flex gap-1.5 p-1.5",
            layout === "compact"
              ? "flex-row items-center"
              : "flex-col items-center",
            layout === "standard" && "-mt-3.5",
          )}
        >
          <span className="bg-muted border-card size-6 shrink-0 rounded-full border-2" />
          <span
            className={cn(
              "flex flex-col gap-1",
              layout === "compact" ? "items-start" : "items-center",
            )}
          >
            <span
              className="text-foreground text-[9px] leading-none"
              style={heading}
            >
              Aa Bb
            </span>
            <span className="bg-muted-foreground/60 h-1 w-8 rounded-full" />
          </span>
        </div>
        <div className="px-1.5 pb-1.5">
          <span className="comcode-brand-gradient block h-2.5 rounded" />
        </div>
      </div>
    </div>
  );
}
