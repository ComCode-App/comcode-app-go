"use client";

import { useEffect } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
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

import { categoryFormSchema, type CategoryFormValues } from "../schemas/category-form";
import type { CategorySummary } from "../services/categories.service";
import { useCategoryMutations } from "../hooks/use-categories";

interface CategoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  category: CategorySummary | null;
}

export function CategoryDialog({
  open,
  onOpenChange,
  category,
}: CategoryDialogProps) {
  const { t } = useLocale();
  const { createMutation, updateMutation } = useCategoryMutations();
  const isEditing = Boolean(category);

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors },
  } = useForm<CategoryFormValues>({
    resolver: zodResolver(categoryFormSchema),
    defaultValues: {
      name: "",
      slug: "",
      description: "",
      icon: "",
      image_url: "",
      display_order: 0,
      is_active: true,
    },
  });

  useEffect(() => {
    if (category) {
      reset({
        name: category.name,
        slug: category.slug,
        description: category.description || "",
        icon: category.icon || "",
        image_url: category.image_url || "",
        display_order: category.display_order ?? 0,
        is_active: category.is_active ?? true,
      });
    } else {
      reset({
        name: "",
        slug: "",
        description: "",
        icon: "",
        image_url: "",
        display_order: 0,
        is_active: true,
      });
    }
  }, [category, reset]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  const onSubmit = async (values: CategoryFormValues) => {
    const payload = {
      name: values.name,
      slug: values.slug || undefined,
      description: values.description || "",
      icon: values.icon || "",
      image_url: values.image_url || "",
      display_order: values.display_order,
      is_active: values.is_active,
    };

    if (isEditing && category) {
      await updateMutation.mutateAsync({ uuid: category.uuid, payload });
    } else {
      await createMutation.mutateAsync(payload);
    }
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[540px]">
        <DialogHeader>
          <DialogTitle>
            {isEditing
              ? t("categories.edit_title")
              : t("categories.create_title")}
          </DialogTitle>
          <DialogDescription>
            {isEditing
              ? t("categories.edit_description")
              : t("categories.create_description")}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label>{t("categories.fields.name")}</Label>
            <Input placeholder="Örn. Metal Kartlar" {...register("name")} />
            {errors.name && (
              <p className="text-xs text-destructive">{errors.name.message}</p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{t("categories.fields.slug")}</Label>
              <Input placeholder="metal-cards" {...register("slug")} />
              <p className="text-[11px] text-muted-foreground">
                {t("categories.fields.slug_hint")}
              </p>
            </div>

            <div className="space-y-2">
              <Label>{t("categories.fields.icon")}</Label>
              <Input placeholder="CreditCard, Shield, QrCode..." {...register("icon")} />
            </div>
          </div>

          <div className="space-y-2">
            <Label>{t("categories.fields.image_url")}</Label>
            <Input placeholder="https://..." {...register("image_url")} />
            {errors.image_url && (
              <p className="text-xs text-destructive">{errors.image_url.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label>{t("categories.fields.description")}</Label>
            <Textarea
              placeholder="Kategori hakkında kısa açıklama..."
              rows={2}
              {...register("description")}
            />
          </div>

          <div className="grid grid-cols-2 gap-4 items-center">
            <div className="space-y-2">
              <Label>{t("categories.fields.display_order")}</Label>
              <Input type="number" {...register("display_order")} />
            </div>

            <div className="flex flex-row items-center justify-between rounded-lg border p-3 mt-4">
              <Label className="text-sm cursor-pointer">
                {t("categories.fields.is_active")}
              </Label>
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
          </div>

          <DialogFooter className="pt-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={isPending}>
              {isEditing ? t("common.save") : t("common.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
