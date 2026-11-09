import type { CSSProperties } from "react";

import type { components } from "@/generated/api";

import catalog from "./themes.json";

/**
 * Public profile theme catalog (TEC-601).
 *
 * A theme is a layout mode plus color and typography tokens plus a default backdrop. The tokens
 * live in `themes.json` so `themes.test.mjs` can check every theme's text contrast (≥ 4.5:1)
 * without a TS toolchain; the ids mirror the backend allow-list (`model.PublicThemes`).
 *
 * Tokens are applied as CSS variables on the public page root, overriding the app's shadcn
 * tokens (`--card`, `--primary`, ...), so every existing component picks them up. A Background
 * Studio design chosen for the page replaces the theme's backdrop; the surfaces stay themed.
 */

export type ThemeId = components["schemas"]["PublicTheme"];
export type LayoutMode = components["schemas"]["PublicLayoutMode"];

export type ThemeColors = {
  /** Surface for outline buttons and inputs (not the page backdrop). */
  background: string;
  foreground: string;
  card: string;
  muted: string;
  mutedForeground: string;
  primary: string;
  /** Second stop of the primary button gradient. */
  primaryEnd: string;
  primaryForeground: string;
  accent: string;
  accentForeground: string;
  border: string;
};

export type FontRole = "sans" | "serif";

export type ProfileTheme = {
  id: ThemeId;
  layout: LayoutMode;
  scheme: "light" | "dark";
  fonts: { heading: FontRole; body: FontRole };
  headingWeight: number;
  colors: ThemeColors;
  /** Cover band when the profile has no cover image. */
  cover: string;
  /** Default page backdrop (CSS background); a Background Studio choice overrides it. */
  backdrop: string;
  /** Opaque color under the backdrop (contrast checks of translucent surfaces). */
  backdropBase: string;
  /** Translucent cards with a backdrop blur (blur is skipped on constrained devices). */
  glass: boolean;
  /**
   * Needs the `design.premium` entitlement (TEC-947). Without it the public page renders
   * `fallback`, a basic theme; mirrors `model.PremiumThemes` in the backend.
   */
  premium?: boolean;
  fallback?: ThemeId;
};

export const THEME_IDS = Object.keys(catalog) as ThemeId[];
export const LAYOUT_MODES: LayoutMode[] = ["standard", "compact", "minimal"];

const THEMES = Object.fromEntries(
  Object.entries(catalog).map(([id, def]) => [
    id,
    { ...def, id } as ProfileTheme,
  ]),
) as Record<ThemeId, ProfileTheme>;

export function isThemeId(value: unknown): value is ThemeId {
  return typeof value === "string" && Object.hasOwn(THEMES, value);
}

export function isLayoutMode(value: unknown): value is LayoutMode {
  return (
    typeof value === "string" && (LAYOUT_MODES as string[]).includes(value)
  );
}

/** True for themes that need the `design.premium` entitlement (TEC-947). */
export function isPremiumTheme(id: string | null | undefined): boolean {
  return getTheme(id)?.premium === true;
}

/** The basic theme a public page shows instead of a premium one when the plan lacks it. */
export function basicThemeFor(id: ThemeId): ThemeId {
  return THEMES[id]?.fallback ?? id;
}

/** The theme for an id, or null for "no theme" (the app's own look) and unknown ids. */
export function getTheme(id: string | null | undefined): ProfileTheme | null {
  return isThemeId(id) ? THEMES[id] : null;
}

/** Layout override first, then the theme's layout, then the classic layout. */
export function resolveLayout(
  theme: ProfileTheme | null,
  override: string | null | undefined,
): LayoutMode {
  if (isLayoutMode(override)) return override;
  return theme?.layout ?? "standard";
}

const FONT_STACKS: Record<FontRole, string> = {
  sans: "var(--font-comcode-sans), ui-sans-serif, system-ui, sans-serif",
  // --font-pp-serif comes from theme-fonts.ts (loaded on demand, not preloaded).
  serif: "var(--font-pp-serif), Georgia, 'Times New Roman', serif",
};

/** CSS variables for a theme root. Returns an empty object for "no theme". */
export function themeStyle(theme: ProfileTheme | null): CSSProperties {
  if (!theme) return {};
  const c = theme.colors;
  const vars: Record<string, string> = {
    "--background": c.background,
    "--foreground": c.foreground,
    "--card": c.card,
    "--card-foreground": c.foreground,
    "--popover": c.card,
    "--popover-foreground": c.foreground,
    "--muted": c.muted,
    "--muted-foreground": c.mutedForeground,
    "--secondary": c.muted,
    "--secondary-foreground": c.foreground,
    "--primary": c.primary,
    "--primary-foreground": c.primaryForeground,
    "--accent": c.accent,
    "--accent-foreground": c.accentForeground,
    "--border": c.border,
    "--input": c.border,
    "--ring": c.primary,
    "--comcode-gradient": `linear-gradient(135deg, ${c.primary} 0%, ${c.primaryEnd} 100%)`,
    "--pp-cover": theme.cover,
    "--pp-backdrop": theme.backdrop,
    "--pp-font-heading": FONT_STACKS[theme.fonts.heading],
    "--pp-font-body": FONT_STACKS[theme.fonts.body],
    "--pp-heading-weight": String(theme.headingWeight),
  };
  return {
    ...(vars as CSSProperties),
    colorScheme: theme.scheme,
    color: c.foreground,
    fontFamily: "var(--pp-font-body)",
  };
}
