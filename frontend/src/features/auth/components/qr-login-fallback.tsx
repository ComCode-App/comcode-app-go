"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Smartphone } from "lucide-react";

import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { fetchAppPublicConfig } from "@/services/app-config.service";

import { AuthCard } from "./auth-card";
import { AuthShell } from "./auth-shell";

/**
 * `/login/qr/{id}` opened in a browser instead of the ComCode app (TEC-1185):
 * a desktop that scanned its own screen, or a phone without the app / with
 * universal links off. The QR only works when the signed-in app opens it, so
 * this page explains that and links the stores. The session id is never used
 * here (it is a short-lived secret).
 */
export function QRLoginFallback() {
  const { t, locale } = useLocale();
  const { data: appConfig } = useQuery({
    queryKey: ["app", "config", locale],
    queryFn: () => fetchAppPublicConfig(locale),
  });
  const stores = appConfig?.mobile_store_urls;

  return (
    <AuthShell>
      <AuthCard
        title={t("auth.qr.fallback_title")}
        description={t("auth.qr.fallback_description")}
      >
        <div className="flex flex-col items-center gap-4 text-center">
          <Smartphone className="text-muted-foreground size-10" aria-hidden />
          <ol className="text-muted-foreground list-decimal space-y-1 pl-5 text-left text-sm">
            <li>{t("auth.qr.step_open")}</li>
            <li>{t("auth.qr.step_scan")}</li>
            <li>{t("auth.qr.step_approve")}</li>
          </ol>
          {stores?.ios ? (
            <Button asChild variant="outline" className="w-full">
              <a href={stores.ios} rel="noopener noreferrer">
                {t("auth.qr.app_store")}
              </a>
            </Button>
          ) : null}
          {stores?.android ? (
            <Button asChild variant="outline" className="w-full">
              <a href={stores.android} rel="noopener noreferrer">
                {t("auth.qr.play_store")}
              </a>
            </Button>
          ) : null}
          <Button asChild className="w-full">
            <Link href={routes.guest.login}>{t("auth.back_to_login")}</Link>
          </Button>
        </div>
      </AuthCard>
    </AuthShell>
  );
}
