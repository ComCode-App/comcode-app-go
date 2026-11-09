"use client";

import React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import {
  CreditCard,
  Sparkles,
  KeyRound,
  CheckCircle2,
  User,
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
import {
  cardActivationSchema,
  type CardActivationFormValues,
} from "../schemas/card-form";

interface CardActivationDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
}

export function CardActivationDialog({
  open,
  onOpenChange,
  onSuccess,
}: CardActivationDialogProps) {
  const { t } = useLocale();
  const { activateMutation } = useCardMutations();

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
    formState: { errors, isSubmitting },
  } = useForm<CardActivationFormValues>({
    resolver: zodResolver(cardActivationSchema),
    defaultValues: {
      activation_code: "",
      name: "",
      profile_uuid: null,
    },
  });

  const onSubmit = async (values: CardActivationFormValues) => {
    try {
      await activateMutation.mutateAsync({
        activation_code: values.activation_code,
        name: values.name || undefined,
        profile_uuid: values.profile_uuid || undefined,
      });
      reset();
      onOpenChange(false);
      onSuccess?.();
    } catch {
      // Handled by mutation toast
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <div className="bg-primary/10 text-primary mb-2 flex h-10 w-10 items-center justify-center rounded-full">
            <Sparkles className="h-5 w-5" />
          </div>
          <DialogTitle>{t("cards.activation.title")}</DialogTitle>
          <DialogDescription>
            {t("cards.activation.description")}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 py-2">
          {/* Activation Code */}
          <div className="space-y-2">
            <Label
              htmlFor="activation_code"
              className="flex items-center gap-1.5"
            >
              <KeyRound className="text-primary h-4 w-4" />
              {t("cards.activation.code_label")}
            </Label>
            <Input
              id="activation_code"
              placeholder={t("cards.activation.code_placeholder")}
              className="h-12 text-center font-mono text-lg tracking-widest uppercase"
              {...register("activation_code")}
            />
            {errors.activation_code && (
              <p className="text-destructive text-xs">
                {errors.activation_code.message}
              </p>
            )}
            <p className="text-muted-foreground text-[11px]">
              {t("cards.activation.code_hint")}
            </p>
          </div>

          {/* Card Label / Name */}
          <div className="space-y-2">
            <Label htmlFor="name" className="flex items-center gap-1.5">
              <CreditCard className="text-muted-foreground h-4 w-4" />
              {t("cards.activation.name_label")}
            </Label>
            <Input
              id="name"
              placeholder={t("cards.activation.name_placeholder")}
              {...register("name")}
            />
          </div>

          {/* Connect to Profile */}
          <div className="space-y-2">
            <Label className="flex items-center gap-1.5">
              <User className="text-muted-foreground h-4 w-4" />
              {t("cards.activation.profile_label")}
            </Label>
            <Controller
              name="profile_uuid"
              control={control}
              render={({ field }) => (
                <Select
                  value={field.value || "none"}
                  onValueChange={(val) =>
                    field.onChange(val === "none" ? null : val)
                  }
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={t(
                        "cards.activation.select_profile_placeholder",
                      )}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">
                      {t("cards.activation.no_profile_later")}
                    </SelectItem>
                    {(profilesData?.items || []).map((prof) => (
                      <SelectItem key={prof.uuid} value={prof.uuid}>
                        {prof.title ||
                          `${prof.first_name} ${prof.last_name}`.trim() ||
                          prof.uuid}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
          </div>

          <DialogFooter className="pt-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || activateMutation.isPending}
              className="gap-2"
            >
              <CheckCircle2 className="h-4 w-4" />
              {isSubmitting || activateMutation.isPending
                ? t("cards.activation.activating")
                : t("cards.activation.submit_btn")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
