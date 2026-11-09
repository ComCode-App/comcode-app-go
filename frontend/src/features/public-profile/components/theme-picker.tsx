"use client";

import { Check } from "lucide-react";

import { Label } from "@/components/ui/label";
import {
  PremiumLockIcon,
  PremiumNotice,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { fontProfileSerif } from "@/features/public-profile/lib/theme-fonts";
import {
  LAYOUT_MODES,
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
  /** The profile's own theme; "" = inherit / default look. */
  theme: ThemeId | "";
  layoutMode: LayoutMode | "";
  /** Company theme used when `theme` is "" (TEC-601); "" = none. */
  inheritedTheme: ThemeId | "";
  disabled?: boolean;
  /** Label and hint of the "" option when it means something else (smart links: the profile's own look). */
  noneLabel?: string;
  hint?: string;
  /**
   * Whether premium themes may be picked (`design.premium`, TEC-947). Defaults to the active
   * account's plan; pass it to preview another account's plan.
   */
  premiumAllowed?: boolean;
  onThemeChange: (theme: ThemeId | "") => void;
  onLayoutModeChange: (layout: LayoutMode | "") => void;
};

/**
 * Owner-side theme + layout picker for the public page (TEC-601), with live preview tiles.
 * Premium themes the plan lacks show a lock and cannot be picked; a premium theme already
 * stored stays selected and the hint says which basic theme visitors see instead (TEC-947).
 */
export function ProfileThemePicker({
  theme,
  layoutMode,
  inheritedTheme,
  disabled,
  noneLabel,
  hint,
  premiumAllowed,
  onThemeChange,
  onLayoutModeChange,
}: Props) {
  const { t } = useLocale();
  const planPremium = useDesignPremium();
  const allowPremium = premiumAllowed ?? planPremium;
  const effectiveId = theme || inheritedTheme;
  const effective = getTheme(effectiveId);
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
    <div className={cn("space-y-4", fontProfileSerif.variable)}>
      <div className="grid gap-1.5">
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
                  layout={resolveLayout(opt.preview, layoutMode)}
                />
                <span className="flex items-center justify-between gap-1 px-1 pb-0.5 text-xs font-medium">
                  <span className="truncate">{opt.label}</span>
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

      <div className="grid gap-1.5">
        <Label id="public-page-layout-label">
          {t("profiles.public_page.layout")}
        </Label>
        <div
          role="radiogroup"
          aria-labelledby="public-page-layout-label"
          className="bg-muted/50 inline-flex w-fit flex-wrap gap-1 rounded-lg border p-1"
        >
          {(["", ...LAYOUT_MODES] as const).map((mode) => {
            const selected = mode === layoutMode;
            const label =
              mode === ""
                ? `${t("profiles.public_page.layout_theme")} · ${t(`profiles.public_page.layout_name.${resolveLayout(effective, "")}`)}`
                : t(`profiles.public_page.layout_name.${mode}`);
            return (
              <button
                key={mode || "theme"}
                type="button"
                role="radio"
                aria-checked={selected}
                disabled={disabled}
                onClick={() => onLayoutModeChange(mode)}
                className={cn(
                  "rounded-md px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-60",
                  selected
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

/** A miniature public page drawn with the theme's own tokens. */
function ThemePreview({
  theme,
  layout,
}: {
  theme: ProfileTheme | null;
  layout: LayoutMode;
}) {
  const style = theme
    ? { ...themeStyle(theme), background: theme.backdrop }
    : undefined;
  return (
    <div
      aria-hidden
      style={style}
      className={cn(
        "flex h-28 flex-col justify-center overflow-hidden rounded-lg p-2",
        !theme && "bg-muted",
      )}
    >
      <div
        className={cn(
          "bg-card overflow-hidden",
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
              style={
                theme
                  ? {
                      fontFamily: "var(--pp-font-heading)",
                      fontWeight: "var(--pp-heading-weight)",
                    }
                  : undefined
              }
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
