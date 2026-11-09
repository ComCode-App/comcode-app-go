"use client";

import { ArrowLeftRight, CheckCircle2, Download } from "lucide-react";
import dynamic from "next/dynamic";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import type { ExchangeForm } from "@/features/leads/lib/form-config";
import { useVisitorUser } from "@/features/leads/hooks/use-visitor-user";
import {
  profilesService,
  type ProfileSummary,
} from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

// Shown only after the visitor opens the exchange, so not needed for first paint (TEC-858).
const ShareProfileForm = dynamic(
  () => import("./contact-exchange-forms").then((m) => m.ShareProfileForm),
  { ssr: false },
);
const ExchangeDetailsForm = dynamic(
  () => import("./contact-exchange-forms").then((m) => m.ExchangeDetailsForm),
  { ssr: false },
);

type Props = {
  exchange: ExchangeForm;
  ownerName: string;
  locale: string;
  /** The owner's vCard, offered once the visitor has shared their details. */
  vcardHref: string;
  onSaveContact?: () => void;
};

type Mode = "profile" | "form";

/**
 * Contact exchange on a public profile (TEC-567): the visitor shares their own details back with
 * the owner. A signed-in visitor can share one of their ComCode profiles; anyone can fill in the
 * form. Afterwards the visitor is offered the owner's vCard.
 */
export function ContactExchange({
  exchange,
  ownerName,
  locale,
  vcardHref,
  onSaveContact,
}: Props) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  const { user, ready: bootstrapped } = useVisitorUser(open);
  const [profiles, setProfiles] = useState<ProfileSummary[]>([]);
  const [mode, setMode] = useState<Mode>("form");
  const [sent, setSent] = useState(false);

  const canShareProfile = Boolean(
    user?.permissions.includes("app.profiles.read"),
  );
  useEffect(() => {
    if (!open || !bootstrapped || !canShareProfile) return;
    let cancelled = false;
    profilesService
      .list({ limit: 50, offset: 0 })
      .then((res) => {
        if (cancelled || !res.items.length) return;
        setProfiles(res.items);
        setMode("profile");
      })
      .catch(() => undefined); // the form still works without a session
    return () => {
      cancelled = true;
    };
  }, [open, bootstrapped, canShareProfile]);

  if (sent) {
    return (
      <div className="flex flex-col items-center gap-3 py-2 text-center">
        <CheckCircle2 className="size-8 text-emerald-600" />
        <div>
          <p className="font-medium">{t("leads.exchange.success_title")}</p>
          <p className="text-muted-foreground text-sm">
            {t("leads.exchange.success_body", { name: ownerName })}
          </p>
        </div>
        <Button asChild variant="outline" className="w-full">
          <a href={vcardHref} onClick={onSaveContact}>
            <Download className="size-4" />
            {t("leads.exchange.save_owner", { name: ownerName })}
          </a>
        </Button>
      </div>
    );
  }

  if (!open) {
    return (
      <div className="space-y-3">
        <p className="text-muted-foreground text-sm">
          {t("leads.exchange.subtitle", { name: ownerName })}
        </p>
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={() => setOpen(true)}
        >
          <ArrowLeftRight className="size-4" />
          {t("leads.exchange.open")}
        </Button>
      </div>
    );
  }

  return mode === "profile" && profiles.length > 0 ? (
    <ShareProfileForm
      exchange={exchange}
      profiles={profiles}
      locale={locale}
      onSent={() => setSent(true)}
      onUseForm={() => setMode("form")}
    />
  ) : (
    <ExchangeDetailsForm
      exchange={exchange}
      locale={locale}
      onSent={() => setSent(true)}
      onUseProfile={profiles.length > 0 ? () => setMode("profile") : undefined}
    />
  );
}
