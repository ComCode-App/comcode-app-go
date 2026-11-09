"use client";

import React, { useEffect } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import {
  CreditCard,
  User,
  ExternalLink,
  Share2,
  Lock,
  Palette,
  ShieldCheck,
  Trash2,
  Radio,
  FileDown,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useLocale } from "@/providers/locale-provider";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { useCardMutations } from "../hooks/use-cards";
import { cardFormSchema, type CardFormValues } from "../schemas/card-form";
import type { CardSummary, CardDetail } from "../services/cards.service";
import { cn } from "@/lib/utils/index";

interface CardSettingsDialogProps {
  card: CardSummary | CardDetail | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
}

const MATERIAL_OPTIONS = [
  { id: "matte_black", name: "Mat Siyah", color: "#18181b", class: "bg-zinc-900 border-zinc-700" },
  { id: "metal_gold", name: "Altın Metal", color: "#eab308", class: "bg-amber-600 border-amber-400" },
  { id: "metal_silver", name: "Gümüş Çelik", color: "#94a3b8", class: "bg-slate-700 border-slate-500" },
  { id: "rose_gold", name: "Rose Gold", color: "#f43f5e", class: "bg-rose-800 border-rose-500" },
  { id: "midnight_blue", name: "Gece Mavisi", color: "#1e3a8a", class: "bg-blue-900 border-blue-600" },
  { id: "emerald", name: "Zümrüt Yeşili", color: "#065f46", class: "bg-emerald-900 border-emerald-600" },
];

export function CardSettingsDialog({
  card,
  open,
  onOpenChange,
  onSuccess,
}: CardSettingsDialogProps) {
  const { t } = useLocale();
  const { updateMutation, createMutation, deleteMutation } = useCardMutations();

  const isEdit = Boolean(card?.uuid);

  const { data: profilesData } = useQuery({
    queryKey: ["profiles", "list-simple"],
    queryFn: () => profilesService.list({ limit: 50, offset: 0 }),
    enabled: open,
  });

  const {
    register,
    handleSubmit,
    control,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<CardFormValues>({
    resolver: zodResolver(cardFormSchema),
    defaultValues: {
      name: "",
      card_uid: "",
      target_type: "profile",
      profile_uuid: null,
      direct_url: "",
      card_type: "physical_nfc",
      material: "matte_black",
      color_hex: "#18181b",
      is_active: true,
      is_locked: false,
      lead_capture_enabled: false,
      passcode: "",
    },
  });

  const selectedTargetType = watch("target_type");
  const selectedMaterial = watch("material");

  useEffect(() => {
    if (card && open) {
      reset({
        name: card.name || "",
        card_uid: card.card_uid || "",
        target_type: (card.target_type as CardFormValues["target_type"]) || "profile",
        profile_uuid: card.profile_uuid || null,
        direct_url: card.direct_url || "",
        card_type: (card.card_type as CardFormValues["card_type"]) || "physical_nfc",
        material: card.material || "matte_black",
        color_hex: card.color_hex || "#18181b",
        is_active: card.is_active ?? true,
        is_locked: card.is_locked ?? false,
        lead_capture_enabled: card.lead_capture_enabled ?? false,
        passcode: "",
      });
    } else if (!card && open) {
      reset({
        name: t("cards.settings.default_new_card_name"),
        card_uid: "",
        target_type: "profile",
        profile_uuid: null,
        direct_url: "",
        card_type: "physical_nfc",
        material: "matte_black",
        color_hex: "#18181b",
        is_active: true,
        is_locked: false,
        lead_capture_enabled: false,
        passcode: "",
      });
    }
  }, [card, open, reset, t]);

  const onSubmit = async (values: CardFormValues) => {
    try {
      if (isEdit && card?.uuid) {
        await updateMutation.mutateAsync({
          uuid: card.uuid,
          payload: values,
        });
      } else {
        await createMutation.mutateAsync(values);
      }
      onOpenChange(false);
      onSuccess?.();
    } catch {
      // Handled in mutation toast
    }
  };

  const handleDelete = async () => {
    if (!card?.uuid) return;
    if (confirm(t("cards.delete_confirm"))) {
      try {
        await deleteMutation.mutateAsync(card.uuid);
        onOpenChange(false);
        onSuccess?.();
      } catch {
        // Handled in mutation toast
      }
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[620px] max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <CreditCard className="w-5 h-5 text-primary" />
            {isEdit ? t("cards.settings.title_edit") : t("cards.settings.title_create")}
          </DialogTitle>
          <DialogDescription>
            {t("cards.settings.description")}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <Tabs defaultValue="target" className="w-full">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="target" className="gap-2">
                <Radio className="w-4 h-4" />
                {t("cards.settings.tab_smart_link")}
              </TabsTrigger>
              <TabsTrigger value="appearance" className="gap-2">
                <Palette className="w-4 h-4" />
                {t("cards.settings.tab_appearance")}
              </TabsTrigger>
            </TabsList>

            {/* TAB 1: SMART LINK TARGETING */}
            <TabsContent value="target" className="space-y-4 py-3">
              {/* Card Label Name */}
              <div className="space-y-1.5">
                <Label htmlFor="name">{t("cards.fields.name")}</Label>
                <Input
                  id="name"
                  placeholder={t("cards.settings.name_placeholder")}
                  {...register("name")}
                />
                {errors.name && (
                  <p className="text-xs text-destructive">{errors.name.message}</p>
                )}
              </div>

              {/* Target Type Selector Cards */}
              <div className="space-y-2">
                <Label>{t("cards.settings.target_type_label")}</Label>
                <div className="grid grid-cols-2 gap-2.5">
                  <div
                    onClick={() => setValue("target_type", "profile")}
                    className={cn(
                      "p-3 rounded-xl border cursor-pointer transition-all flex flex-col gap-1.5",
                      selectedTargetType === "profile"
                        ? "border-primary bg-primary/5 text-primary shadow-sm"
                        : "border-border hover:border-border/80 bg-card text-muted-foreground"
                    )}
                  >
                    <div className="flex items-center gap-2 font-semibold text-sm text-foreground">
                      <User className="w-4 h-4 text-primary" />
                      {t("cards.settings.target_profile")}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {t("cards.settings.target_profile_desc")}
                    </p>
                  </div>

                  <div
                    onClick={() => setValue("target_type", "direct_url")}
                    className={cn(
                      "p-3 rounded-xl border cursor-pointer transition-all flex flex-col gap-1.5",
                      selectedTargetType === "direct_url"
                        ? "border-primary bg-primary/5 text-primary shadow-sm"
                        : "border-border hover:border-border/80 bg-card text-muted-foreground"
                    )}
                  >
                    <div className="flex items-center gap-2 font-semibold text-sm text-foreground">
                      <ExternalLink className="w-4 h-4 text-primary" />
                      {t("cards.settings.target_direct_url")}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {t("cards.settings.target_direct_url_desc")}
                    </p>
                  </div>

                  <div
                    onClick={() => setValue("target_type", "vcard_download")}
                    className={cn(
                      "p-3 rounded-xl border cursor-pointer transition-all flex flex-col gap-1.5",
                      selectedTargetType === "vcard_download"
                        ? "border-primary bg-primary/5 text-primary shadow-sm"
                        : "border-border hover:border-border/80 bg-card text-muted-foreground"
                    )}
                  >
                    <div className="flex items-center gap-2 font-semibold text-sm text-foreground">
                      <FileDown className="w-4 h-4 text-primary" />
                      {t("cards.settings.target_vcard")}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {t("cards.settings.target_vcard_desc")}
                    </p>
                  </div>

                  <div
                    onClick={() => setValue("target_type", "social_hub")}
                    className={cn(
                      "p-3 rounded-xl border cursor-pointer transition-all flex flex-col gap-1.5",
                      selectedTargetType === "social_hub"
                        ? "border-primary bg-primary/5 text-primary shadow-sm"
                        : "border-border hover:border-border/80 bg-card text-muted-foreground"
                    )}
                  >
                    <div className="flex items-center gap-2 font-semibold text-sm text-foreground">
                      <Share2 className="w-4 h-4 text-primary" />
                      {t("cards.settings.target_social")}
                    </div>
                    <p className="text-xs text-muted-foreground">
                      {t("cards.settings.target_social_desc")}
                    </p>
                  </div>
                </div>
              </div>

              {/* Conditional Inputs Based on Target */}
              {selectedTargetType === "profile" && (
                <div className="space-y-1.5 p-3.5 rounded-xl bg-muted/40 border">
                  <Label className="flex items-center gap-1.5">
                    <User className="w-4 h-4 text-primary" />
                    {t("cards.settings.select_profile_label")}
                  </Label>
                  <Controller
                    name="profile_uuid"
                    control={control}
                    render={({ field }) => (
                      <Select
                        value={field.value || "none"}
                        onValueChange={(val) => field.onChange(val === "none" ? null : val)}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t("cards.settings.select_profile_placeholder")} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">{t("cards.settings.no_profile_selected")}</SelectItem>
                          {(profilesData?.items || []).map((prof) => (
                            <SelectItem key={prof.uuid} value={prof.uuid}>
                              {prof.title || `${prof.first_name} ${prof.last_name}`.trim() || prof.uuid}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  />
                </div>
              )}

              {selectedTargetType === "direct_url" && (
                <div className="space-y-1.5 p-3.5 rounded-xl bg-muted/40 border">
                  <Label htmlFor="direct_url" className="flex items-center gap-1.5">
                    <ExternalLink className="w-4 h-4 text-primary" />
                    {t("cards.settings.direct_url_label")}
                  </Label>
                  <Input
                    id="direct_url"
                    placeholder="https://sirketiniz.com veya https://instagram.com/hesabiniz"
                    {...register("direct_url")}
                  />
                </div>
              )}

              {selectedTargetType === "vcard_download" && (
                <div className="space-y-1.5 p-3.5 rounded-xl bg-muted/40 border">
                  <Label className="flex items-center gap-1.5">
                    <User className="w-4 h-4 text-primary" />
                    {t("cards.settings.vcard_profile_source")}
                  </Label>
                  <Controller
                    name="profile_uuid"
                    control={control}
                    render={({ field }) => (
                      <Select
                        value={field.value || "none"}
                        onValueChange={(val) => field.onChange(val === "none" ? null : val)}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t("cards.settings.select_profile_placeholder")} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">{t("cards.settings.no_profile_selected")}</SelectItem>
                          {(profilesData?.items || []).map((prof) => (
                            <SelectItem key={prof.uuid} value={prof.uuid}>
                              {prof.title || `${prof.first_name} ${prof.last_name}`.trim() || prof.uuid}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  />
                  <p className="text-[11px] text-muted-foreground">
                    {t("cards.settings.vcard_hint")}
                  </p>
                </div>
              )}

              {/* Advanced Security & Lead Features */}
              <div className="space-y-3 pt-2">
                <div className="flex items-center justify-between p-3 rounded-xl border bg-card">
                  <div className="space-y-0.5">
                    <Label className="text-sm font-semibold flex items-center gap-1.5">
                      <ShieldCheck className="w-4 h-4 text-emerald-500" />
                      {t("cards.settings.lead_capture_title")}
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      {t("cards.settings.lead_capture_desc")}
                    </p>
                  </div>
                  <Controller
                    name="lead_capture_enabled"
                    control={control}
                    render={({ field }) => (
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>

                <div className="flex items-center justify-between p-3 rounded-xl border bg-card">
                  <div className="space-y-0.5">
                    <Label className="text-sm font-semibold flex items-center gap-1.5">
                      <Lock className="w-4 h-4 text-amber-500" />
                      {t("cards.settings.lock_title")}
                    </Label>
                    <p className="text-xs text-muted-foreground">
                      {t("cards.settings.lock_desc")}
                    </p>
                  </div>
                  <Controller
                    name="is_locked"
                    control={control}
                    render={({ field }) => (
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>
              </div>
            </TabsContent>

            {/* TAB 2: APPEARANCE & MATERIAL */}
            <TabsContent value="appearance" className="space-y-4 py-3">
              {/* Material Selection */}
              <div className="space-y-2">
                <Label>{t("cards.settings.material_label")}</Label>
                <div className="grid grid-cols-3 gap-2.5">
                  {MATERIAL_OPTIONS.map((mat) => {
                    const matKey = `cards.materials.${mat.id}`;
                    const matLabel = t(matKey);
                    const displayName = matLabel.startsWith("cards.") ? mat.name : matLabel;
                    return (
                      <div
                        key={mat.id}
                        onClick={() => setValue("material", mat.id)}
                        className={cn(
                          "p-3 rounded-xl border cursor-pointer transition-all flex flex-col items-center justify-center gap-2 text-center",
                          selectedMaterial === mat.id
                            ? "ring-2 ring-primary border-transparent shadow-md"
                            : "border-border hover:border-border/80 bg-card"
                        )}
                      >
                        <div className={cn("w-10 h-6 rounded-md shadow-inner border", mat.class)} />
                        <span className="text-xs font-medium text-foreground">{displayName}</span>
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* Physical NFC UID Tag */}
              <div className="space-y-1.5">
                <Label htmlFor="card_uid">{t("cards.fields.card_uid")}</Label>
                <Input
                  id="card_uid"
                  placeholder="04:A2:88:1A:3F:80"
                  className="font-mono text-sm uppercase"
                  {...register("card_uid")}
                />
                <p className="text-[11px] text-muted-foreground">
                  {t("cards.settings.card_uid_hint")}
                </p>
              </div>

              {/* Card Active Status Switch */}
              <div className="flex items-center justify-between p-3 rounded-xl border bg-card">
                <div className="space-y-0.5">
                  <Label className="text-sm font-semibold">
                    {t("cards.fields.is_active")}
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    {t("cards.settings.active_switch_desc")}
                  </p>
                </div>
                <Controller
                  name="is_active"
                  control={control}
                  render={({ field }) => (
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  )}
                />
              </div>
            </TabsContent>
          </Tabs>

          <DialogFooter className="flex items-center justify-between sm:justify-between pt-4 border-t">
            {isEdit ? (
              <Button
                type="button"
                variant="destructive"
                size="sm"
                className="gap-1.5"
                onClick={handleDelete}
                disabled={deleteMutation.isPending}
              >
                <Trash2 className="w-4 h-4" />
                {t("common.delete")}
              </Button>
            ) : <div />}

            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button
                type="submit"
                disabled={isSubmitting || updateMutation.isPending || createMutation.isPending}
              >
                {isSubmitting || updateMutation.isPending || createMutation.isPending
                  ? t("common.saving")
                  : t("common.save")}
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
