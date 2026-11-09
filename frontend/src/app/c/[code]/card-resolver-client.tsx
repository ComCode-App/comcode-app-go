"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Lock, Sparkles, AlertCircle, ArrowRight, Ban } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { cardsService, type SmartLinkResolution } from "@/features/cards/services/cards.service";
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
        if (res.status !== "retired" && !res.is_locked && !res.requires_passcode && res.target_url) {
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
      <div className="min-h-screen flex flex-col items-center justify-center p-4 bg-background">
        <div className="w-16 h-16 rounded-3xl bg-primary/10 text-primary flex items-center justify-center animate-bounce shadow-lg">
          <Sparkles className="w-8 h-8" />
        </div>
        <p className="mt-4 text-sm font-medium text-muted-foreground animate-pulse">
          {t("cards.resolver_redirecting")}
        </p>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-background">
        <Card className="w-full max-w-md text-center p-6 space-y-4">
          <div className="w-12 h-12 rounded-full bg-destructive/10 text-destructive flex items-center justify-center mx-auto">
            <AlertCircle className="w-6 h-6" />
          </div>
          <CardHeader className="p-0">
            <CardTitle>{t("cards.resolver_not_found_title")}</CardTitle>
            <CardDescription>{error || t("cards.resolver_not_found_desc")}</CardDescription>
          </CardHeader>
          <Button variant="outline" className="w-full" onClick={() => router.push("/")}>
            {t("cards.resolver_return_home")}
          </Button>
        </Card>
      </div>
    );
  }

  // Cancelled/refunded cards and removed links (TEC-585).
  if (data.status === "retired") {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-background">
        <Card className="w-full max-w-md text-center p-6 space-y-4">
          <div className="w-12 h-12 rounded-full bg-muted text-muted-foreground flex items-center justify-center mx-auto">
            <Ban className="w-6 h-6" />
          </div>
          <CardHeader className="p-0">
            <CardTitle>{t("cards.resolver_retired_title")}</CardTitle>
            <CardDescription>{t("cards.resolver_retired_desc")}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  if (data.is_locked) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-background">
        <Card className="w-full max-w-md text-center p-6 space-y-4 border-amber-500/30">
          <div className="w-12 h-12 rounded-full bg-amber-500/10 text-amber-500 flex items-center justify-center mx-auto">
            <Lock className="w-6 h-6" />
          </div>
          <CardHeader className="p-0">
            <CardTitle>{t("cards.resolver_locked_title")}</CardTitle>
            <CardDescription>
              {t("cards.resolver_locked_desc")}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-background">
      <Card className="w-full max-w-md text-center p-6 space-y-5 shadow-xl">
        <div className="w-14 h-14 rounded-3xl bg-primary/10 text-primary flex items-center justify-center mx-auto shadow-inner">
          <Sparkles className="w-7 h-7" />
        </div>

        <div className="space-y-1">
          <h2 className="text-xl font-bold text-foreground">{data.card_name}</h2>
          <p className="text-xs text-muted-foreground">{t("cards.resolver_tagline")}</p>
        </div>

        {data.target_url && (
          <Button
            className="w-full gap-2 text-sm h-11 shadow-md"
            onClick={() => {
              if (data.target_url?.startsWith("http://") || data.target_url?.startsWith("https://")) {
                window.location.href = data.target_url;
              } else if (data.target_url) {
                router.push(data.target_url);
              }
            }}
          >
            <span>{t("cards.resolver_view_profile")}</span>
            <ArrowRight className="w-4 h-4" />
          </Button>
        )}
      </Card>
    </div>
  );
}
