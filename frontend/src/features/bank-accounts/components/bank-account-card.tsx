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
import {
  useBankAccountMutations,
} from "../hooks/use-bank-accounts";

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
      className={`relative rounded-2xl border bg-card p-5 shadow-xs transition-all hover:shadow-md ${
        !account.is_active ? "opacity-60 bg-muted/30" : ""
      }`}
    >
      {/* Top Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div
            className="w-11 h-11 rounded-xl flex items-center justify-center text-white text-xs font-bold shrink-0 overflow-hidden shadow-xs"
            style={{ backgroundColor: brandColor }}
          >
            {account.bank_icon_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={account.bank_icon_url}
                alt={account.bank_name || "Bank"}
                className="w-full h-full object-contain p-1.5"
              />
            ) : (
              <Landmark className="w-5 h-5 text-white" />
            )}
          </div>

          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-foreground text-sm">
                {account.bank_name ||
                  t("bank_accounts.unknown_bank")}
              </h3>
              {account.is_primary && (
                <Badge
                  variant="secondary"
                  className="gap-1 text-[10px] font-semibold bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30"
                >
                  <Star className="w-3 h-3 fill-amber-500 text-amber-500" />
                  {t("bank_accounts.primary_badge")}
                </Badge>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
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
          <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
            {t("bank_accounts.fields.account_holder")}
          </span>
          <p className="text-sm font-medium text-foreground">
            {account.account_holder}
          </p>
        </div>

        {/* IBAN Display Box with One-click Copy */}
        <div className="flex items-center justify-between gap-2 p-2.5 rounded-lg bg-muted/60 border border-muted/80 font-mono text-xs">
          <div className="truncate font-semibold tracking-wider text-foreground">
            {account.formatted_iban || account.iban}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleCopyIban}
            className="h-7 px-2 shrink-0 gap-1 text-xs hover:bg-background"
          >
            {copied ? (
              <>
                <Check className="w-3.5 h-3.5 text-emerald-500" />
                <span className="text-emerald-500 font-sans">
                  {t("common.copied")}
                </span>
              </>
            ) : (
              <>
                <Copy className="w-3.5 h-3.5 text-muted-foreground" />
                <span className="font-sans">{t("common.copy")}</span>
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Footer Info (Currency, BIC/SWIFT, Branch Code) */}
      <div className="mt-4 pt-3 border-t flex items-center justify-between text-xs text-muted-foreground">
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
