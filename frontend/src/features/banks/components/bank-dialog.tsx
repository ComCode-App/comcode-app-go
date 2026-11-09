"use client";

import { useMemo, useState } from "react";
import { useForm, Controller, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Landmark, Loader2, UploadCloud } from "lucide-react";

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
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";

import { useLocale } from "@/providers/locale-provider";
import { type BankSummary, type BankDetail } from "../services/banks.service";
import { useBank, useBankMutations } from "../hooks/use-banks";
import { bankFormSchema, type BankFormValues } from "../schemas/bank-form";

type BankDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bank?: BankSummary | BankDetail | null;
};

export function BankDialog({ open, onOpenChange, bank }: BankDialogProps) {
  const { t } = useLocale();
  const isEdit = Boolean(bank);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [localPreview, setLocalPreview] = useState<string>("");

  const { data: bankData, isLoading: isLoadingBank } = useBank(
    bank?.uuid ?? null,
  );
  const { createMutation, updateMutation, uploadIconMutation } =
    useBankMutations();

  const activeBank = bankData || bank;

  const formValues = useMemo<BankFormValues>(() => {
    if (!open) {
      return {
        name: "",
        slug: "",
        code: "",
        country_code: "TR",
        bic_swift: "",
        icon_url: "",
        brand_color: "#1e3a8a",
        iban_pattern: "",
        description: "",
        display_order: 0,
        is_active: true,
      };
    }
    return {
      name: activeBank?.name || "",
      slug: activeBank?.slug || "",
      code: activeBank?.code || "",
      country_code: activeBank?.country_code || "TR",
      bic_swift: activeBank?.bic_swift || "",
      icon_url: activeBank?.icon_url || "",
      brand_color: activeBank?.brand_color || "#1e3a8a",
      iban_pattern: activeBank?.iban_pattern || "",
      description: activeBank?.description || "",
      display_order: activeBank?.display_order ?? 0,
      is_active: activeBank?.is_active ?? true,
    };
  }, [activeBank, open]);

  const {
    register,
    handleSubmit,
    control,
    setValue,
    formState: { errors },
  } = useForm<BankFormValues>({
    resolver: zodResolver(bankFormSchema),
    values: formValues,
  });

  const brandColor = useWatch({ control, name: "brand_color" }) || "#1e3a8a";
  const iconUrl = useWatch({ control, name: "icon_url" });
  const nameValue = useWatch({ control, name: "name" });
  const codeValue = useWatch({ control, name: "code" });
  const bicSwiftValue = useWatch({ control, name: "bic_swift" });
  const isActiveValue = useWatch({ control, name: "is_active" });

  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) {
      setSelectedFile(null);
      setLocalPreview("");
    }
    onOpenChange(isOpen);
  };

  const handleIconChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setSelectedFile(file);
    const localUrl = URL.createObjectURL(file);
    setLocalPreview(localUrl);

    if (isEdit && bank?.uuid) {
      try {
        const res = await uploadIconMutation.mutateAsync({
          uuid: bank.uuid,
          file,
        });
        if (res?.icon_url) {
          setValue("icon_url", res.icon_url);
          setLocalPreview(res.icon_url);
        }
      } catch {
        // Handled by toast
      }
    }
  };

  const onSubmit = async (values: BankFormValues) => {
    let savedUUID = bank?.uuid;
    if (isEdit && bank?.uuid) {
      await updateMutation.mutateAsync({ uuid: bank.uuid, body: values });
    } else {
      const created = await createMutation.mutateAsync(values);
      savedUUID = created.uuid;
    }

    if (selectedFile && savedUUID && !isEdit) {
      try {
        await uploadIconMutation.mutateAsync({
          uuid: savedUUID,
          file: selectedFile,
        });
      } catch {
        // Handled by toast
      }
    }

    handleOpenChange(false);
  };

  const isSaving =
    createMutation.isPending ||
    updateMutation.isPending ||
    uploadIconMutation.isPending;
  const displayIcon = localPreview || iconUrl;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t("banks.dialog.edit_title")
              : t("banks.dialog.create_title")}
          </DialogTitle>
          <DialogDescription>{t("banks.dialog.description")}</DialogDescription>
        </DialogHeader>

        {isLoadingBank && isEdit ? (
          <div className="text-muted-foreground flex items-center justify-center gap-2 p-8 text-sm">
            <Loader2 className="h-4 w-4 animate-spin" />
            {t("common.loading")}
          </div>
        ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            {/* Live Preview Bar */}
            <div
              className="flex items-center gap-3 rounded-xl border p-3 transition-colors"
              style={{
                backgroundColor: `${brandColor || "#1e3a8a"}10`,
                borderColor: `${brandColor || "#1e3a8a"}30`,
              }}
            >
              <div
                className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-xl text-xs font-bold text-white shadow-xs"
                style={{ backgroundColor: brandColor || "#1e3a8a" }}
              >
                {displayIcon ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={displayIcon}
                    alt="Preview"
                    className="h-full w-full object-contain p-1"
                  />
                ) : (
                  <Landmark className="h-5 w-5 text-white" />
                )}
              </div>
              <div className="flex-1">
                <div className="text-sm font-semibold">
                  {nameValue || t("banks.fields.name")}
                </div>
                <div className="text-muted-foreground font-mono text-xs">
                  {codeValue ? `EFT: ${codeValue}` : "EFT: —"}{" "}
                  {bicSwiftValue ? `| BIC: ${bicSwiftValue}` : ""}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="col-span-2 space-y-1.5">
                <Label htmlFor="bank-name">{t("banks.fields.name")}</Label>
                <Input
                  id="bank-name"
                  placeholder="Türkiye İş Bankası"
                  {...register("name")}
                />
                {errors.name && (
                  <p className="text-destructive text-xs">
                    {errors.name.message}
                  </p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="bank-code">{t("banks.fields.code")}</Label>
                <Input
                  id="bank-code"
                  placeholder="00064"
                  {...register("code")}
                />
                {errors.code && (
                  <p className="text-destructive text-xs">
                    {errors.code.message}
                  </p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="country-code">
                  {t("banks.fields.country_code")}
                </Label>
                <Input
                  id="country-code"
                  placeholder="TR"
                  {...register("country_code")}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="bic-swift">{t("banks.fields.bic_swift")}</Label>
                <Input
                  id="bic-swift"
                  placeholder="ISBKTRIS"
                  {...register("bic_swift")}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="brand-color">
                  {t("banks.fields.brand_color")}
                </Label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={brandColor || "#1e3a8a"}
                    onChange={(e) => setValue("brand_color", e.target.value)}
                    className="h-9 w-9 shrink-0 cursor-pointer rounded-md border p-1"
                  />
                  <Input
                    id="brand-color"
                    placeholder="#1e3a8a"
                    {...register("brand_color")}
                  />
                </div>
              </div>

              <div className="col-span-2 space-y-1.5">
                <Label htmlFor="icon-url">{t("banks.fields.icon_url")}</Label>
                <div className="flex items-center gap-2">
                  <Input
                    id="icon-url"
                    placeholder="https://... / bank-logo.svg"
                    {...register("icon_url")}
                  />
                  <label className="border-input bg-muted/40 hover:bg-muted inline-flex h-9 shrink-0 cursor-pointer items-center gap-1.5 rounded-md border px-3 py-2 text-xs font-medium transition-colors">
                    <UploadCloud className="text-muted-foreground h-4 w-4" />
                    <span>{t("banks.actions.upload_icon")}</span>
                    <input
                      type="file"
                      accept="image/svg+xml,image/png,image/jpeg,image/webp,image/x-icon"
                      onChange={handleIconChange}
                      className="hidden"
                    />
                  </label>
                </div>
              </div>

              <div className="col-span-2 space-y-1.5">
                <Label htmlFor="bank-desc">
                  {t("banks.fields.description")}
                </Label>
                <Textarea
                  id="bank-desc"
                  placeholder=""
                  {...register("description")}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="disp-order">
                  {t("banks.fields.display_order")}
                </Label>
                <Input
                  id="disp-order"
                  type="number"
                  {...register("display_order", { valueAsNumber: true })}
                />
              </div>

              <div className="space-y-1.5">
                <Label>{t("banks.fields.is_active")}</Label>
                <div className="flex h-9 items-center justify-between rounded-lg border p-2.5 shadow-2xs">
                  <span className="text-muted-foreground text-xs">
                    {isActiveValue ? t("common.active") : t("common.inactive")}
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
