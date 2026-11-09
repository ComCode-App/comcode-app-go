"use client";

import { useMemo, useState } from "react";
import { useForm, Controller, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { ChevronDown, ChevronUp, Landmark, Loader2 } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { useLocale } from "@/providers/locale-provider";
import { useActiveBanks } from "@/features/banks";
import {
  useBankAccount,
  useBankAccountMutations,
} from "../hooks/use-bank-accounts";
import {
  bankAccountFormSchema,
  type BankAccountFormValues,
} from "../schemas/bank-account-form";
import type { UserBankAccountSummary } from "../services/bank-accounts.service";

type BankAccountDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  account?: UserBankAccountSummary | null;
};

const CURRENCIES = [
  { code: "TRY", label: "TRY - Türk Lirası (₺)" },
  { code: "USD", label: "USD - ABD Doları ($)" },
  { code: "EUR", label: "EUR - Euro (€)" },
  { code: "GBP", label: "GBP - İngiliz Sterlini (£)" },
];

export function BankAccountDialog({
  open,
  onOpenChange,
  account,
}: BankAccountDialogProps) {
  const { t } = useLocale();
  const isEdit = Boolean(account);

  const { data: activeBanks = [] } = useActiveBanks();
  const { data: accountData, isLoading: isLoadingAccount } = useBankAccount(
    account?.uuid ?? null,
  );
  const { createMutation, updateMutation } = useBankAccountMutations();

  const activeAccount = accountData || account;

  const formValues = useMemo<BankAccountFormValues>(() => {
    if (!open) {
      return {
        iban: "",
        account_holder: "",
        title: "",
        currency: "TRY",
        bank_uuid: "",
        account_number: "",
        branch_code: "",
        is_primary: false,
        is_active: true,
      };
    }
    return {
      iban: activeAccount?.formatted_iban || activeAccount?.iban || "",
      account_holder: activeAccount?.account_holder || "",
      title: activeAccount?.title || "",
      currency: activeAccount?.currency || "TRY",
      bank_uuid: activeAccount?.bank_uuid || "",
      account_number: activeAccount?.account_number || "",
      branch_code: activeAccount?.branch_code || "",
      is_primary: activeAccount?.is_primary ?? false,
      is_active: activeAccount?.is_active ?? true,
    };
  }, [activeAccount, open]);

  const hasExtraFields = Boolean(
    activeAccount?.title ||
    activeAccount?.account_number ||
    activeAccount?.branch_code,
  );
  const [userExpanded, setUserExpanded] = useState<boolean | null>(null);
  const showAdvanced = userExpanded !== null ? userExpanded : hasExtraFields;

  const {
    register,
    handleSubmit,
    control,
    setValue,
    formState: { errors },
  } = useForm<BankAccountFormValues>({
    resolver: zodResolver(bankAccountFormSchema),
    values: formValues,
  });

  const rawIban = useWatch({ control, name: "iban" }) || "";
  const isPrimary = useWatch({ control, name: "is_primary" });
  const isActive = useWatch({ control, name: "is_active" });

  // Real-time Bank Detection
  const detectedBank = useMemo(() => {
    if (!rawIban) return null;
    const clean = rawIban.replace(/[^a-zA-Z0-9]/g, "").toUpperCase();
    if (clean.length < 9) return null;

    const country = clean.slice(0, 2);
    if (country === "TR") {
      const bankCode = clean.slice(4, 9);
      return activeBanks.find((b) => b.code === bankCode) || null;
    }

    const bbanPrefix = clean.slice(4, 8);
    return (
      activeBanks.find(
        (b) =>
          b.country_code === country ||
          b.code === bbanPrefix ||
          clean.includes(b.slug.toUpperCase()),
      ) || null
    );
  }, [rawIban, activeBanks]);

  // Format IBAN as user types
  const handleIbanChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = e.target.value.replace(/[^a-zA-Z0-9]/g, "").toUpperCase();
    let formatted = "";
    for (let i = 0; i < raw.length && i < 34; i++) {
      if (i > 0 && i % 4 === 0) {
        formatted += " ";
      }
      formatted += raw[i];
    }
    setValue("iban", formatted, { shouldValidate: true });
  };

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) {
      setUserExpanded(null);
    }
    onOpenChange(isOpen);
  };

  const onSubmit = async (values: BankAccountFormValues) => {
    const payload = {
      ...values,
      bank_uuid:
        values.bank_uuid || (detectedBank ? detectedBank.uuid : undefined),
    };

    if (isEdit && account?.uuid) {
      await updateMutation.mutateAsync({
        uuid: account.uuid,
        body: payload,
      });
    } else {
      await createMutation.mutateAsync(payload);
    }
    handleOpenChange(false);
  };

  const isSaving = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t("bank_accounts.dialog.edit_title")
              : t("bank_accounts.dialog.create_title")}
          </DialogTitle>
          <DialogDescription>
            {t("bank_accounts.dialog.description")}
          </DialogDescription>
        </DialogHeader>

        {isLoadingAccount && isEdit ? (
          <div className="text-muted-foreground flex items-center justify-center gap-2 p-8 text-sm">
            <Loader2 className="h-4 w-4 animate-spin" />
            {t("common.loading")}
          </div>
        ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            {/* Detected Bank Banner */}
            {detectedBank && (
              <div
                className="animate-in fade-in slide-in-from-top-1 flex items-center justify-between rounded-xl border p-3 shadow-2xs transition-all"
                style={{
                  backgroundColor: `${detectedBank.brand_color || "#3b82f6"}12`,
                  borderColor: `${detectedBank.brand_color || "#3b82f6"}30`,
                }}
              >
                <div className="flex items-center gap-3">
                  <div
                    className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-xl text-xs font-bold text-white shadow-xs"
                    style={{
                      backgroundColor: detectedBank.brand_color || "#3b82f6",
                    }}
                  >
                    {detectedBank.icon_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={detectedBank.icon_url}
                        alt={detectedBank.name}
                        className="h-full w-full object-contain p-1"
                      />
                    ) : (
                      <Landmark className="h-5 w-5 text-white" />
                    )}
                  </div>
                  <div>
                    <div className="text-foreground text-sm font-semibold">
                      {detectedBank.name}
                    </div>
                    <div className="text-muted-foreground font-mono text-xs">
                      {detectedBank.bic_swift
                        ? `BIC: ${detectedBank.bic_swift}`
                        : `EFT: ${detectedBank.code}`}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Primary Form Fields */}
            <div className="space-y-1.5">
              <Label htmlFor="account-iban">
                {t("bank_accounts.fields.iban")}
              </Label>
              <Input
                id="account-iban"
                placeholder="TR00 0000 0000 0000 0000 0000 00"
                value={rawIban}
                onChange={handleIbanChange}
                className="font-mono text-sm font-medium tracking-wider uppercase"
              />
              {errors.iban && (
                <p className="text-destructive text-xs">
                  {errors.iban.message}
                </p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="account-holder">
                {t("bank_accounts.fields.account_holder")}
              </Label>
              <Input
                id="account-holder"
                placeholder={t("bank_accounts.placeholders.account_holder")}
                {...register("account_holder")}
              />
              {errors.account_holder && (
                <p className="text-destructive text-xs">
                  {errors.account_holder.message}
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label>{t("bank_accounts.fields.currency")}</Label>
                <Controller
                  control={control}
                  name="currency"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <SelectTrigger>
                        <SelectValue placeholder="TRY" />
                      </SelectTrigger>
                      <SelectContent>
                        {CURRENCIES.map((curr) => (
                          <SelectItem key={curr.code} value={curr.code}>
                            {curr.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>

              <div className="space-y-1.5">
                <Label>{t("bank_accounts.fields.is_primary")}</Label>
                <div className="flex h-9 items-center justify-between rounded-lg border p-2.5 shadow-2xs">
                  <span className="text-muted-foreground text-xs">
                    {isPrimary ? t("common.yes") : t("common.no")}
                  </span>
                  <Controller
                    control={control}
                    name="is_primary"
                    render={({ field }) => (
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>
              </div>
            </div>

            {/* Toggle Advanced / Extra Options */}
            <div className="pt-1">
              <button
                type="button"
                onClick={() => setUserExpanded(!showAdvanced)}
                className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium transition-colors"
              >
                {showAdvanced ? (
                  <>
                    <ChevronUp className="h-3.5 w-3.5" />
                    {t("bank_accounts.hide_details")}
                  </>
                ) : (
                  <>
                    <ChevronDown className="h-3.5 w-3.5" />
                    {t("bank_accounts.show_details")}
                  </>
                )}
              </button>
            </div>

            {showAdvanced && (
              <div className="animate-in fade-in space-y-4 border-t pt-2">
                <div className="space-y-1.5">
                  <Label htmlFor="account-title">
                    {t("bank_accounts.fields.title")}
                  </Label>
                  <Input
                    id="account-title"
                    placeholder={t("bank_accounts.placeholders.title")}
                    {...register("title")}
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="branch-code">
                      {t("bank_accounts.fields.branch_code")}
                    </Label>
                    <Input
                      id="branch-code"
                      placeholder="1234"
                      {...register("branch_code")}
                    />
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="account-number">
                      {t("bank_accounts.fields.account_number")}
                    </Label>
                    <Input
                      id="account-number"
                      placeholder="5678901"
                      {...register("account_number")}
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label>{t("bank_accounts.fields.bank_override")}</Label>
                  <Controller
                    control={control}
                    name="bank_uuid"
                    render={({ field }) => (
                      <Select
                        value={
                          field.value || (detectedBank ? detectedBank.uuid : "")
                        }
                        onValueChange={field.onChange}
                      >
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t("bank_accounts.auto_detect")}
                          />
                        </SelectTrigger>
                        <SelectContent>
                          {activeBanks.map((b) => (
                            <SelectItem key={b.uuid} value={b.uuid}>
                              {b.name} ({b.code})
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label>{t("bank_accounts.fields.is_active")}</Label>
                  <div className="flex h-9 items-center justify-between rounded-lg border p-2.5 shadow-2xs">
                    <span className="text-muted-foreground text-xs">
                      {isActive ? t("common.active") : t("common.inactive")}
                    </span>
                    <Controller
                      control={control}
                      name="is_active"
                      render={({ field }) => (
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      )}
                    />
                  </div>
                </div>
              </div>
            )}

            <DialogFooter className="pt-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {t("common.save")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
