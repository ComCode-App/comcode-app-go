"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Lock, Sparkles, AlertCircle, ArrowRight, Ban } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import {
  cardsService,
  type SmartLinkResolution,
} from "@/features/cards/services/cards.service";
import { withVia } from "@/features/leads/lib/via";
import { useLocale } from "@/providers/locale-provider";

/** Fallback UI for card links the server could not redirect (locked, passcode, inactive, errors). */
export function CardResolverClient({ code }: { code: string }) {
  const { t } = useLocale();
  const router = useRouter();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<SmartLinkResolution | null>(null);

  useEffect(() => {
    if (!code) return;

    cardsService
      .resolveSmartLink(code)
      .then((res) => {
        setData(res);
        setLoading(false);

        // Auto-redirect if not locked and target_url exists; ?via= attributes the visit (TEC-940).
        if (
          res.status !== "retired" &&
          !res.is_locked &&
          !res.requires_passcode &&
          res.target_url
        ) {
          const target = withVia(res.target_url, `c/${code}`);
          if (target.startsWith("http://") || target.startsWith("https://")) {
            window.location.href = target;
          } else {
            router.push(target);
          }
        }
      })
      .catch((err) => {
        setError(err?.message || t("cards.resolver_not_found_desc"));
        setLoading(false);
      });
  }, [code, router, t]);

  if (loading) {
    return (
      <div className="bg-background flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-primary/10 text-primary flex h-16 w-16 animate-bounce items-center justify-center rounded-3xl shadow-lg">
          <Sparkles className="h-8 w-8" />
        </div>
        <p className="text-muted-foreground mt-4 animate-pulse text-sm font-medium">
          {t("cards.resolver_redirecting")}
        </p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="bg-background flex min-h-screen items-center justify-center p-4">
        <Card className="w-full max-w-md space-y-4 p-6 text-center">
          <div className="bg-destructive/10 text-destructive mx-auto flex h-12 w-12 items-center justify-center rounded-full">
            <AlertCircle className="h-6 w-6" />
          </div>
          <CardHeader className="p-0">
            <CardTitle>{t("cards.resolver_not_found_title")}</CardTitle>
            <CardDescription>
              {error || t("cards.resolver_not_found_desc")}
            </CardDescription>
          </CardHeader>
          <Button
            variant="outline"
            className="w-full"
            onClick={() => router.push("/")}
          >
            {t("cards.resolver_return_home")}
          </Button>
        </Card>
      </div>
    );
  }

  // Cancelled/refunded cards and removed links (TEC-585).
  if (data.status === "retired") {
    return (
      <div className="bg-background flex min-h-screen items-center justify-center p-4">
        <Card className="w-full max-w-md space-y-4 p-6 text-center">
          <div className="bg-muted text-muted-foreground mx-auto flex h-12 w-12 items-center justify-center rounded-full">
            <Ban className="h-6 w-6" />
          </div>
          <CardHeader className="p-0">
            <CardTitle>{t("cards.resolver_retired_title")}</CardTitle>
            <CardDescription>
              {t("cards.resolver_retired_desc")}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  if (data.is_locked) {
    return (
      <div className="bg-background flex min-h-screen items-center justify-center p-4">
        <Card className="w-full max-w-md space-y-4 border-amber-500/30 p-6 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-amber-500/10 text-amber-500">
            <Lock className="h-6 w-6" />
          </div>
          <CardHeader className="p-0">
            <CardTitle>{t("cards.resolver_locked_title")}</CardTitle>
            <CardDescription>{t("cards.resolver_locked_desc")}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="bg-background flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-md space-y-5 p-6 text-center shadow-xl">
        <div className="bg-primary/10 text-primary mx-auto flex h-14 w-14 items-center justify-center rounded-3xl shadow-inner">
          <Sparkles className="h-7 w-7" />
        </div>

        <div className="space-y-1">
          <h2 className="text-foreground text-xl font-bold">
            {data.card_name}
          </h2>
          <p className="text-muted-foreground text-xs">
            {t("cards.resolver_tagline")}
          </p>
        </div>

        {data.target_url && (
          <Button
            className="h-11 w-full gap-2 text-sm shadow-md"
            onClick={() => {
              if (
                data.target_url?.startsWith("http://") ||
                data.target_url?.startsWith("https://")
              ) {
                window.location.href = data.target_url;
              } else if (data.target_url) {
                router.push(data.target_url);
              }
            }}
          >
            <span>{t("cards.resolver_view_profile")}</span>
            <ArrowRight className="h-4 w-4" />
          </Button>
        )}
      </Card>
    </div>
  );
}
