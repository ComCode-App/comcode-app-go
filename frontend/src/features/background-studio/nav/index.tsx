"use client";

import { Sparkles } from "lucide-react";

import { routes } from "@/config/routes";
import {
  createNavAdornment,
  defineNavItem,
  type NavAdornment,
} from "@/features/nav-engine";
import { useLocale } from "@/providers/locale-provider";

function useBackgroundStudioNavAdornment(): NavAdornment {
  const { t } = useLocale();
  return {
    badges: [
      {
        kind: "label",
        text: t("common.beta"),
        variant: "warning",
      },
    ],
  };
}

export const BackgroundStudioNavAdornment = createNavAdornment(
  useBackgroundStudioNavAdornment,
);

export const backgroundStudioNavItem = defineNavItem({
  id: "background-studio",
  titleKey: "layout.nav_background_studio",
  href: routes.app.backgroundStudio.root,
  icon: Sparkles,
  Adornment: BackgroundStudioNavAdornment,
});
