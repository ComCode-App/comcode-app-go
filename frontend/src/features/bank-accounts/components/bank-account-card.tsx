"use client";

import { useState } from "react";
import {
  Check,
  Copy,
  Edit2,
  Landmark,
  MoreVertical,
  Star,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";
import type { UserBankAccountSummary } from "../services/bank-accounts.service";
import { useBankAccountMutations } from "../hooks/use-bank-accounts";

type BankAccountCardProps = {
  account: UserBankAccountSummary;
  onEdit: (account: UserBankAccountSummary) => void;
  onDelete: (uuid: string) => void;
};

export function BankAccountCard({
  account,
  onEdit,
  onDelete,
}: BankAccountCardProps) {
  const { t } = useLocale();
  const [copied, setCopied] = useState(false);

  const { toggleActiveMutation, updateMutation } = useBankAccountMutations();

  const handleCopyIban = async () => {
    try {
      await navigator.clipboard.writeText(account.iban);
      setCopied(true);
      appToast.success(t("bank_accounts.toast.copied"));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // toast-guard: clipboard only, no API call.
      appToast.error(t("common.error_generic"));
    }
  };

  const handleToggleActive = () => {
    toggleActiveMutation.mutate(account.uuid);
  };

  const handleSetPrimary = () => {
    updateMutation.mutate({
      uuid: account.uuid,
      body: { is_primary: true },
    });
  };

  const brandColor = account.bank_brand_color || "#3b82f6";

  return (
    <div
      className={`bg-card relative rounded-2xl border p-5 shadow-xs transition-all hover:shadow-md ${
        !account.is_active ? "bg-muted/30 opacity-60" : ""
      }`}
    >
      {/* Top Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div
            className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl text-xs font-bold text-white shadow-xs"
            style={{ backgroundColor: brandColor }}
          >
            {account.bank_icon_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={account.bank_icon_url}
                alt={account.bank_name || "Bank"}
                className="h-full w-full object-contain p-1.5"
              />
            ) : (
              <Landmark className="h-5 w-5 text-white" />
            )}
          </div>

          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-foreground text-sm font-semibold">
                {account.bank_name || t("bank_accounts.unknown_bank")}
              </h3>
              {account.is_primary && (
                <Badge
                  variant="secondary"
                  className="gap-1 border-amber-500/30 bg-amber-500/15 text-[10px] font-semibold text-amber-600 dark:text-amber-400"
                >
                  <Star className="h-3 w-3 fill-amber-500 text-amber-500" />
                  {t("bank_accounts.primary_badge")}
                </Badge>
              )}
            </div>
            <p className="text-muted-foreground text-xs">
              {account.title || account.currency}
            </p>
          </div>
        </div>

        {/* Actions Dropdown */}
        <div className="flex items-center gap-1.5">
          <Switch
            checked={account.is_active}
            onCheckedChange={handleToggleActive}
            className="scale-85"
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8">
                <MoreVertical className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => onEdit(account)}>
                <Edit2 className="mr-2 h-4 w-4" />
                {t("common.edit")}
              </DropdownMenuItem>
              {!account.is_primary && (
                <DropdownMenuItem onClick={handleSetPrimary}>
                  <Star className="mr-2 h-4 w-4" />
                  {t("bank_accounts.actions.set_primary")}
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => onDelete(account.uuid)}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                {t("common.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Account Info Details */}
      <div className="mt-4 space-y-3">
        <div>
          <span className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
            {t("bank_accounts.fields.account_holder")}
          </span>
          <p className="text-foreground text-sm font-medium">
            {account.account_holder}
          </p>
        </div>

        {/* IBAN Display Box with One-click Copy */}
        <div className="bg-muted/60 border-muted/80 flex items-center justify-between gap-2 rounded-lg border p-2.5 font-mono text-xs">
          <div className="text-foreground truncate font-semibold tracking-wider">
            {account.formatted_iban || account.iban}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleCopyIban}
            className="hover:bg-background h-7 shrink-0 gap-1 px-2 text-xs"
          >
            {copied ? (
              <>
                <Check className="h-3.5 w-3.5 text-emerald-500" />
                <span className="font-sans text-emerald-500">
                  {t("common.copied")}
                </span>
              </>
            ) : (
              <>
                <Copy className="text-muted-foreground h-3.5 w-3.5" />
                <span className="font-sans">{t("common.copy")}</span>
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Footer Info (Currency, BIC/SWIFT, Branch Code) */}
      <div className="text-muted-foreground mt-4 flex items-center justify-between border-t pt-3 text-xs">
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="font-mono text-[11px]">
            {account.currency}
          </Badge>
          {account.branch_code && (
            <span>
              {t("bank_accounts.branch")}: {account.branch_code}
            </span>
          )}
        </div>
        {account.bank_bic_swift && (
          <span className="font-mono text-[11px]">
            BIC: {account.bank_bic_swift}
          </span>
        )}
      </div>
    </div>
  );
}
