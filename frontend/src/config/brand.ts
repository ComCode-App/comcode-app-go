/**
 * ComCode identity tokens.
 * Hex aliases for JS (theme-color, icons). CSS tokens live in theme.css.
 * Brand SVG/PNG assets live under `public/brand/`.
 */
export const brand = {
  name: "ComCode",
  productName: "ComCode",
  tagline: "Dijital Kartvizit",
  assets: {
    /** Icon glyph — sidebar tile, compact marks */
    mark: "/brand/mark.svg",
    /** Horizontal lockup — login, headers */
    logo: "/brand/logo-horizontal.svg",
    /** Vertical lockup — narrow panels */
    logoVertical: "/brand/logo-vertical.svg",
    /** Browser tab / PWA source */
    icon: "/brand/icon.svg",
  },
  colors: {
    /** Brand magenta */
    primary: "#D81B8C",
    primaryForeground: "#FFFFFF",
    /** Deep purple */
    purple: "#6A0DAD",
    /** True black canvas */
    ink: "#000000",
    mist: "#FAF5F9",
    white: "#FFFFFF",
    accent: "#D81B8C",
    accentPurple: "#6A0DAD",
    accentSoft: "#FCE7F3",
    /** Magenta → purple (Instagram profile) */
    gradient: "linear-gradient(135deg, #E0209A 0%, #7B1FA2 100%)",
    navy: "#000000",
    lightGray: "#FAF5F9",
  },
} as const;

export type BrandTone = "auto" | "light" | "dark";
export type BrandVariant = "logo" | "wordmark" | "mark" | "icon";
