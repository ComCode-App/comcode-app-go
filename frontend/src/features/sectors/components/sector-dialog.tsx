"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Loader2 } from "lucide-react";
import { useMemo } from "react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useLocale } from "@/providers/locale-provider";

import { useSectorMutations } from "../hooks/use-sectors";
import {
  sectorFormSchema,
  type SectorFormValues,
} from "../schemas/sector-form";
import type { PlatformSector } from "../services/sectors.service";

type SectorDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sector: PlatformSector | null;
};

const EMPTY: SectorFormValues = { slug: "", name_tr: "", name_en: "" };

export function SectorDialog({
  open,
  onOpenChange,
  sector,
}: SectorDialogProps) {
  const { t } = useLocale();
  const { create, update } = useSectorMutations();

  const values = useMemo<SectorFormValues>(
    () =>
      sector && open
        ? {
            slug: sector.slug,
            name_tr: sector.name_tr,
            name_en: sector.name_en,
          }
        : EMPTY,
    [open, sector],
  );

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SectorFormValues>({
    resolver: zodResolver(sectorFormSchema),
    values,
  });

  const onSubmit = async (body: SectorFormValues) => {
    try {
      if (sector) await update.mutateAsync({ uuid: sector.uuid, body });
      else await create.mutateAsync(body);
      onOpenChange(false);
    } catch {
      // Toast handled by the mutation.
    }
  };

  const saving = create.isPending || update.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {sector
              ? t("sectors.dialog.edit_title")
              : t("sectors.dialog.create_title")}
          </DialogTitle>
          <DialogDescription>
            {t("sectors.dialog.description")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="sector-slug">{t("sectors.fields.slug")}</Label>
            <Input
              id="sector-slug"
              autoComplete="off"
              className="font-mono"
              placeholder="food-beverage"
              {...register("slug")}
            />
            <p
              className={
                errors.slug
                  ? "text-destructive text-xs"
                  : "text-muted-foreground text-xs"
              }
            >
              {t("sectors.validation.slug")}
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="sector-name-tr">
                {t("sectors.fields.name_tr")}
              </Label>
              <Input
                id="sector-name-tr"
                autoComplete="off"
                {...register("name_tr")}
              />
              {errors.name_tr ? (
                <p className="text-destructive text-xs">
                  {t("sectors.validation.name")}
                </p>
              ) : null}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="sector-name-en">
                {t("sectors.fields.name_en")}
              </Label>
              <Input
                id="sector-name-en"
                autoComplete="off"
                {...register("name_en")}
              />
              {errors.name_en ? (
                <p className="text-destructive text-xs">
                  {t("sectors.validation.name")}
                </p>
              ) : null}
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
            <Button type="submit" disabled={saving}>
              {saving ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}
              {t("common.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
