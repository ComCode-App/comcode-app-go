"use client";

import type { ReactNode } from "react";

import { LocaleProvider } from "@/providers/locale-provider";
import { ThemeProvider } from "@/providers/theme-provider";
import { ToastProvider } from "@/providers/toast-provider";

/**
 * Providers every route gets, visitor pages included: locale, theme and toasts. Keep this lean;
 * panel-only providers belong in `AppProviders` (TEC-858).
 */
export function PublicProviders({ children }: { children: ReactNode }) {
  return (
    <LocaleProvider>
      <ThemeProvider>
        <ToastProvider>{children}</ToastProvider>
      </ThemeProvider>
    </LocaleProvider>
  );
}
