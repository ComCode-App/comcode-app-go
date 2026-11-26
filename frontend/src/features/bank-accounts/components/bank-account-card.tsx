"use client";

import { useState } from "react";
import { Check, Copy, Landmark, Pencil, Star, Trash2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { TooltipHint } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import type { UserBankAccountSummary } from "../services/bank-accounts.service";

type BankAccountCardProps = {
  account: UserBankAccountSummary;
  onEdit: (account: UserBankAccountSummary) => void;
  onDelete: (account: UserBankAccountSummary) => void;
  onToggleActive: (account: UserBankAccountSummary) => void;
  onSetPrimary: (account: UserBankAccountSummary) => void;
};

const FALLBACK_BRAND = "#1e3a8a";

/** "TR12 0006 2000 ..." groups of four, like a printed IBAN. */
function groupIban(iban: string) {
  return iban
    .replace(/\s+/g, "")
    .replace(/(.{4})/g, "$1 ")
    .trim();
}

/**
 * Grid card for /bank-accounts (TEC-1323): the account drawn as a bank card in the bank's
 * brand color (logo, grouped IBAN, holder, currency), with copy, active switch and actions.
 */
export function BankAccountCard({
  account,
  onEdit,
  onDelete,
  onToggleActive,
  onSetPrimary,
}: BankAccountCardProps) {
  const { t } = useLocale();
  const [copied, setCopied] = useState(false);
  const brand = account.bank_brand_color || FALLBACK_BRAND;
  const bankName = account.bank_name || t("bank_accounts.unknown_bank");

  const handleCopyIban = async () => {
    try {
      await navigator.clipboard.writeText(account.iban.replace(/\s+/g, ""));
      setCopied(true);
      appToast.success(t("bank_accounts.toast.copied"));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // toast-guard: clipboard only, no API call.
      appToast.error(t("common.error_generic"));
    }
  };

  return (
    <div
      className={cn(
        "-m-4 flex h-full flex-col",
        !account.is_active && "opacity-70",
      )}
    >
      {/* Bank card face */}
      <div
        className="relative overflow-hidden p-5 text-white"
        style={{
          background: `linear-gradient(135deg, ${brand} 0%, color-mix(in oklab, ${brand} 55%, #000) 100%)`,
        }}
      >
        {/* Decorative rings */}
        <div className="pointer-events-none absolute -end-10 -top-12 h-40 w-40 rounded-full bg-white/10" />
        <div className="pointer-events-none absolute -end-2 top-10 h-28 w-28 rounded-full bg-white/5" />

        <div className="relative flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm">
              {account.bank_icon_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={account.bank_icon_url}
                  alt={bankName}
                  className="h-full w-full object-contain p-1.5"
                />
              ) : (
                <Landmark className="h-5 w-5" style={{ color: brand }} />
              )}
            </div>
            <div className="min-w-0">
              <p className="truncate text-sm leading-tight font-semibold">
                {bankName}
              </p>
              {account.title ? (
                <p className="truncate text-xs text-white/75">
                  {account.title}
                </p>
              ) : null}
            </div>
          </div>
          <Badge className="shrink-0 border-white/30 bg-white/15 font-mono text-[11px] text-white hover:bg-white/20">
            {account.currency}
          </Badge>
        </div>

        <div className="relative mt-6 flex items-center gap-2">
          <p className="min-w-0 flex-1 font-mono text-[13px] font-semibold tracking-wider break-all sm:text-sm">
            {account.formatted_iban || groupIban(account.iban)}
          </p>
          <TooltipHint label={copied ? t("common.copied") : t("common.copy")}>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="h-8 w-8 shrink-0 text-white hover:bg-white/15 hover:text-white"
              onClick={() => void handleCopyIban()}
              aria-label={t("common.copy")}
            >
              {copied ? (
                <Check className="h-4 w-4" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </TooltipHint>
        </div>

        <div className="relative mt-4 flex items-end justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[10px] tracking-wider text-white/65 uppercase">
              {t("bank_accounts.fields.account_holder")}
            </p>
            <p className="truncate text-sm font-medium">
              {account.account_holder}
            </p>
          </div>
          {account.is_primary ? (
            <Badge className="shrink-0 gap-1 border-amber-300/40 bg-amber-400/20 text-[10px] text-amber-50 hover:bg-amber-400/25">
              <Star className="h-3 w-3 fill-amber-300 text-amber-300" />
              {t("bank_accounts.primary_badge")}
            </Badge>
          ) : null}
        </div>
      </div>

      {/* Details and actions */}
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs">
          {account.branch_code ? (
            <span>
              {t("bank_accounts.branch")}:{" "}
              <span className="text-foreground font-mono">
                {account.branch_code}
              </span>
            </span>
          ) : null}
          {account.bank_code ? (
            <span>
              EFT:{" "}
              <span className="text-foreground font-mono">
                {account.bank_code}
              </span>
            </span>
          ) : null}
          {account.bank_bic_swift ? (
            <span>
              BIC:{" "}
              <span className="text-foreground font-mono">
                {account.bank_bic_swift}
              </span>
            </span>
          ) : null}
        </div>

        <div className="border-border/60 mt-auto flex items-center justify-between gap-2 border-t pt-3">
          <label className="flex items-center gap-2 text-xs">
            <Switch
              checked={account.is_active}
              onCheckedChange={() => onToggleActive(account)}
              aria-label={t("bank_accounts.fields.is_active")}
            />
            <span
              className={cn(
                account.is_active ? "text-foreground" : "text-muted-foreground",
              )}
            >
              {account.is_active ? t("common.active") : t("common.inactive")}
            </span>
          </label>
          <div className="flex items-center gap-1">
            {!account.is_primary && (
              <TooltipHint label={t("bank_accounts.actions.set_primary")}>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground h-8 w-8 hover:text-amber-500"
                  onClick={() => onSetPrimary(account)}
                  aria-label={t("bank_accounts.actions.set_primary")}
                >
                  <Star className="h-4 w-4" />
                </Button>
              </TooltipHint>
            )}
            <TooltipHint label={t("common.edit")}>
              <Button
                variant="ghost"
                size="icon"
                className="text-muted-foreground hover:text-foreground h-8 w-8"
                onClick={() => onEdit(account)}
                aria-label={t("common.edit")}
              >
                <Pencil className="h-4 w-4" />
              </Button>
            </TooltipHint>
            <TooltipHint label={t("common.delete")}>
              <Button
                variant="ghost"
                size="icon"
                className="text-muted-foreground hover:text-destructive h-8 w-8"
                onClick={() => onDelete(account)}
                aria-label={t("common.delete")}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </TooltipHint>
          </div>
        </div>
      </div>
    </div>
  );
}
