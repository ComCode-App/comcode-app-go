import { brand } from "@/config/brand";

export type ThemeMode = "light" | "dark";

export const themeConfig = {
  defaultMode: "light" as ThemeMode,
  storageKey: "comcode.theme",
  brand: {
    primary: brand.colors.primary,
    primaryForeground: brand.colors.primaryForeground,
    navy: brand.colors.navy,
    muted: brand.colors.mist,
    accent: brand.colors.accentSoft,
    radius: "1rem",
  },
} as const;
