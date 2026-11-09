"use client";

import { Copy } from "lucide-react";
import { useCallback, useSyncExternalStore } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

type PaymentProvider = "paytr" | "iyzico";

function callbackPath(provider: PaymentProvider) {
  return `/api/v1/public/payment/${provider}/callback`;
}

const emptySubscribe = () => () => {};

export function PaymentCallbackUrlField({
  provider,
}: {
  provider: PaymentProvider;
}) {
  const { t } = useLocale();
  const origin = useSyncExternalStore(
    emptySubscribe,
    () => window.location.origin,
    () => "",
  );
  const callbackUrl = `${origin}${callbackPath(provider)}`;

  const hintKey =
    provider === "paytr"
      ? "integrations.payment.paytr.callback_hint"
      : "integrations.payment.iyzico.callback_hint";

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(callbackUrl);
      appToast.success(t("integrations.payment.callback_copied"));
    } catch {
      appToast.error(t("integrations.payment.callback_copy_failed"));
    }
  }, [callbackUrl, t]);

  return (
    <div className="space-y-2 sm:col-span-2">
      <div className="space-y-1">
        <Label htmlFor={`${provider}-callback-url`}>
          {t("integrations.payment.callback_url")}
        </Label>
        <p className="text-muted-foreground text-sm">{t(hintKey)}</p>
      </div>
      <div className="flex gap-2">
        <Input
          id={`${provider}-callback-url`}
          readOnly
          value={callbackUrl}
          className="font-mono text-xs"
        />
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="shrink-0"
          onClick={() => void copy()}
          aria-label={t("integrations.payment.copy_callback")}
        >
          <Copy className="size-4" />
        </Button>
      </div>
    </div>
  );
}
