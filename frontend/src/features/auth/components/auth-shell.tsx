"use client";

import { useSyncExternalStore, type ReactNode } from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
import { useTheme } from "next-themes";

import { AppWordmark } from "@/components/brand";
import {
  canRenderShaderBackground,
  useIdleValue,
  usePrefersReducedMotion,
} from "@/features/background-studio/lib/render-budget";
import { LocaleSwitch } from "@/components/layout/locale-switch";
import { ThemeSwitch } from "@/components/layout/theme-switch";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";

// ComCode brand palette: Magenta (#D81B8C) & Purple (#6A0DAD)
const DARK_COLORS = ["#09010f", "#26063b", "#6a0dad", "#d81b8c", "#a21caf"];
const LIGHT_COLORS = ["#ffffff", "#fdf2f8", "#f5d0fe", "#f472b6", "#c084fc"];

const emptySubscribe = () => () => {};

// Same split point as the public pages (TEC-937): the WebGL shader is not in the guest bundle and
// never renders on the server; the solid `bg-background` stays until the chunk arrives.
const MeshGradient = dynamic(
  () => import("@/features/background-studio/data/shader-chunks/mesh-gradient"),
  {
    ssr: false,
  },
);

export function AuthShell({ children }: { children: ReactNode }) {
  const { t } = useLocale();
  const { resolvedTheme } = useTheme();
  const mounted = useSyncExternalStore(
    emptySubscribe,
    () => true,
    () => false,
  );

  const isDark = mounted ? resolvedTheme === "dark" : true;
  // Same gates as the public profile backgrounds (TEC-600): weak devices, data saver and software
  // WebGL (no GPU, e.g. CI runners) keep the plain background; reduced motion freezes the shader.
  const capable = useIdleValue(canRenderShaderBackground);
  const reduceMotion = usePrefersReducedMotion();
  const colors = isDark ? DARK_COLORS : LIGHT_COLORS;

  return (
    <div className="bg-background relative flex min-h-svh flex-col items-center justify-center overflow-hidden p-6 md:p-10">
      {/* Background Mesh Gradient Shader */}
      <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden opacity-90 dark:opacity-85">
        {capable ? (
          <MeshGradient
            width="100%"
            height="100%"
            colors={colors}
            distortion={0.65}
            swirl={0.15}
            grainMixer={0.08}
            grainOverlay={0.04}
            speed={reduceMotion ? 0 : 0.4}
            fit="cover"
          />
        ) : null}
      </div>

      {/* Subtle overlay grid for tech texture */}
      <div
        aria-hidden
        className="comcode-auth-grid pointer-events-none absolute inset-0 z-[1] opacity-35 dark:opacity-25"
      />

      <div className="absolute top-4 right-4 z-10 flex items-center gap-1 rounded-full border border-white/50 bg-white/40 p-1 shadow-sm backdrop-blur-md dark:border-white/10 dark:bg-white/[0.06]">
        <LocaleSwitch />
        <ThemeSwitch />
      </div>

      <div className="relative z-10 flex w-full max-w-sm flex-col gap-6">
        <Link
          href={routes.guest.login}
          className="self-center drop-shadow-sm transition-transform hover:scale-[1.02]"
          aria-label={t("common.app_product")}
        >
          <AppWordmark />
        </Link>
        {children}
      </div>
    </div>
  );
}
