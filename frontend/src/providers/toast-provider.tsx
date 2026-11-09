"use client";

import { useTheme } from "next-themes";
import { useEffect, type ReactNode } from "react";
import { Toaster, toast } from "sonner";

import { routes } from "@/config/routes";
import { ApiError, setGlobalApiErrorHandler } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";

export function ToastProvider({ children }: { children: ReactNode }) {
  const { theme } = useTheme();
  const { t } = useLocale();

  useEffect(() => {
    setGlobalApiErrorHandler((error: ApiError) => {
      if (error.isPlanLimit) {
        toast.warning(t("entitlements.limit_reached_title"), {
          id: "api-error:PLAN_LIMIT_EXCEEDED",
          description: t("entitlements.limit_reached_toast"),
          action: {
            label: t("entitlements.upgrade"),
            onClick: () => window.location.assign(routes.app.billing.root),
          },
        });
        return;
      }
      // `error.message` is already localized by code (lib/api/error-messages.ts). The id folds
      // repeats of the same error (e.g. query retries) into one toast.
      const message = error.message || t("common.error_generic");
      toast.error(
        error.isServer && error.requestId
          ? `${message} (${error.requestId})`
          : message,
        { id: `api-error:${error.code}:${message}` },
      );
    });

    return () => setGlobalApiErrorHandler(null);
  }, [t]);

  return (
    <>
      {children}
      <Toaster
        richColors
        position="top-right"
        theme={theme === "dark" ? "dark" : "light"}
      />
    </>
  );
}

export const appToast = {
  success: (message: string) => toast.success(message),
  error: (message: string) => toast.error(message),
  warning: (message: string) => toast.warning(message),
  info: (message: string) => toast.info(message),
};
