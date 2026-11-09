"use client";

import { useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Check, Copy, Landmark, Pencil, Star, Trash2 } from "lucide-react";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { appToast } from "@/providers/toast-provider";
import type { UserBankAccountSummary } from "../services/bank-accounts.service";

interface ColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  onEdit: (account: UserBankAccountSummary) => void;
  onDelete: (uuid: string, accountName: string) => void;
  onToggleActive: (account: UserBankAccountSummary) => void;
  onSetPrimary: (account: UserBankAccountSummary) => void;
}

function BankLogoCell({ item, t }: { item: UserBankAccountSummary; t: (k: string) => string }) {
  const [error, setError] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <div
        className="w-9 h-9 rounded-xl flex items-center justify-center text-white text-xs font-bold shrink-0 overflow-hidden shadow-xs"
        style={{ backgroundColor: item.bank_brand_color || "#1e3a8a" }}
      >
        {item.bank_icon_url && !error ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.bank_icon_url}
            alt={item.bank_name || "Bank"}
            onError={() => setError(true)}
            className="w-full h-full object-contain p-1"
          />
        ) : (
          <Landmark className="w-4 h-4 text-white" />
        )}
      </div>
      <div>
        <div className="font-semibold text-sm text-foreground flex items-center gap-1.5">
          <span>{item.bank_name || t("bank_accounts.unknown_bank")}</span>
          {item.is_primary && (
            <Badge
              variant="secondary"
              className="gap-1 px-1.5 py-0 text-[10px] font-normal"
            >
              <Star className="h-3 w-3 fill-amber-500 text-amber-500" />
              {t("bank_accounts.primary_badge")}
            </Badge>
          )}
        </div>
        {item.title ? (
          <div className="text-xs text-muted-foreground font-medium">
            {item.title}
          </div>
        ) : item.bank_code ? (
          <div className="text-xs text-muted-foreground font-mono">
            EFT: {item.bank_code}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function IbanCell({ iban, t }: { iban: string; t: (k: string) => string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(iban);
    setCopied(true);
    appToast.success(t("bank_accounts.toast.copied"));
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="flex items-center gap-1.5">
      <span className="font-mono text-xs font-medium tracking-wide bg-muted/60 px-2 py-1 rounded">
        {iban}
      </span>
      <TooltipProvider delayDuration={200}>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-foreground shrink-0"
              onClick={handleCopy}
            >
              {copied ? (
                <Check className="h-3.5 w-3.5 text-emerald-600" />
              ) : (
                <Copy className="h-3.5 w-3.5" />
              )}
              <span className="sr-only">{t("common.copy")}</span>
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">
            <p className="text-xs">{copied ? t("common.copied") : t("common.copy")}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  );
}

export function getBankAccountsColumns({
  t,
  onEdit,
  onDelete,
  onToggleActive,
  onSetPrimary,
}: ColumnOptions): ColumnDef<UserBankAccountSummary, unknown>[] {
  return [
    createColumn<UserBankAccountSummary>({
      accessorKey: "bank_name",
      labelKey: "bank_accounts.fields.bank_name",
      enableSorting: true,
      gridPrimary: true,
      cell: ({ row }) => <BankLogoCell item={row.original} t={t} />,
    }),
    createColumn<UserBankAccountSummary>({
      accessorKey: "account_holder",
      labelKey: "bank_accounts.fields.account_holder",
      enableSorting: true,
      cell: ({ row }) => (
        <div className="flex flex-col">
          <span className="text-sm font-medium text-foreground">
            {row.original.account_holder}
          </span>
          {row.original.branch_code && (
            <span className="text-xs text-muted-foreground">
              {t("bank_accounts.branch")}: {row.original.branch_code}
            </span>
          )}
        </div>
      ),
    }),
    createColumn<UserBankAccountSummary>({
      accessorKey: "iban",
      labelKey: "bank_accounts.fields.iban",
      enableSorting: true,
      cell: ({ row }) => <IbanCell iban={row.original.iban} t={t} />,
    }),
    createColumn<UserBankAccountSummary>({
      accessorKey: "currency",
      labelKey: "bank_accounts.fields.currency",
      enableSorting: true,
      cell: ({ row }) => (
        <Badge variant="outline" className="font-mono text-xs font-semibold">
          {row.original.currency}
        </Badge>
      ),
    }),
    createColumn<UserBankAccountSummary>({
      accessorKey: "is_active",
      labelKey: "bank_accounts.fields.is_active",
      enableSorting: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center gap-2">
            <Switch
              checked={item.is_active}
              onCheckedChange={() => onToggleActive(item)}
              className="scale-85"
            />
            <span className="text-xs text-muted-foreground">
              {item.is_active ? t("common.active") : t("common.inactive")}
            </span>
          </div>
        );
      },
    }),
    createColumn<UserBankAccountSummary>({
      id: "actions",
      labelKey: "table.actions",
      enableSorting: false,
      enableHiding: false,
      enableResizing: false,
      enableColumnFilter: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center justify-end gap-1">
            <TooltipProvider delayDuration={200}>
              {!item.is_primary && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-amber-500 hover:bg-amber-500/10"
                      onClick={() => onSetPrimary(item)}
                    >
                      <Star className="h-4 w-4" />
                      <span className="sr-only">{t("bank_accounts.actions.set_primary")}</span>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="top">
                    <p className="text-xs">{t("bank_accounts.actions.set_primary")}</p>
                  </TooltipContent>
                </Tooltip>
              )}

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-foreground"
                    onClick={() => onEdit(item)}
                  >
                    <Pencil className="h-4 w-4" />
                    <span className="sr-only">{t("common.edit")}</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">{t("common.edit")}</p>
                </TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                    onClick={() =>
                      onDelete(
                        item.uuid,
                        `${item.bank_name || "Bank"} (${item.iban.slice(-4)})`,
                      )
                    }
                  >
                    <Trash2 className="h-4 w-4" />
                    <span className="sr-only">{t("common.delete")}</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">{t("common.delete")}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        );
      },
    }),
  ];
}
