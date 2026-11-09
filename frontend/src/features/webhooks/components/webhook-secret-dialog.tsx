"use client";

import { useState } from "react";
import { Check, Copy, KeyRound } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import type { WebhookEndpointWithSecret } from "@/features/webhooks/services/webhooks.service";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

type Props = {
  endpoint: WebhookEndpointWithSecret | null;
  onClose: () => void;
};

/** Shows a freshly created or rotated signing secret once (TEC-603). */
export function WebhookSecretDialog({ endpoint, onClose }: Props) {
  const { t } = useLocale();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    if (!endpoint) return;
    try {
      await navigator.clipboard.writeText(endpoint.secret);
      setCopied(true);
      appToast.success(t("webhooks.secret.copied"));
    } catch {
      // toast-guard: clipboard only, no API call.
      appToast.error(t("common.error_generic"));
    }
  };

  return (
    <Dialog
      open={Boolean(endpoint)}
      onOpenChange={(open) => (!open ? onClose() : undefined)}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <KeyRound className="size-5" />
            {t("webhooks.secret.title")}
          </DialogTitle>
          <DialogDescription>
            {t("webhooks.secret.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="flex gap-2">
          <Input
            readOnly
            value={endpoint?.secret ?? ""}
            className="font-mono text-xs"
            onFocus={(e) => e.currentTarget.select()}
          />
          <Button type="button" variant="outline" onClick={() => void copy()}>
            {copied ? (
              <Check className="size-4" />
            ) : (
              <Copy className="size-4" />
            )}
            {t("webhooks.secret.copy")}
          </Button>
        </div>
        <Alert>
          <AlertTitle>
            {t("webhooks.secret.hint", { hint: endpoint?.secret_hint ?? "" })}
          </AlertTitle>
          <AlertDescription className="font-mono text-xs">
            {t("webhooks.secret.how")}
          </AlertDescription>
        </Alert>
        <DialogFooter>
          <Button type="button" onClick={onClose}>
            {t("webhooks.secret.done")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
