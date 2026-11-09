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
import { Textarea } from "@/components/ui/textarea";
import { useLocale } from "@/providers/locale-provider";

import { useOfficeMutations } from "../hooks/use-company-structure";
import {
  officeFormSchema,
  type OfficeFormValues,
} from "../schemas/office-form";
import type { CompanyOffice } from "../services/company-structure.service";

type OfficeDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  office: CompanyOffice | null;
};

const EMPTY: OfficeFormValues = {
  name: "",
  city: "",
  address: "",
  phone: "",
  sort_order: 0,
};

export function OfficeDialog({
  open,
  onOpenChange,
  office,
}: OfficeDialogProps) {
  const { t } = useLocale();
  const { create, update } = useOfficeMutations();
  const isEdit = Boolean(office);

  const values = useMemo<OfficeFormValues>(
    () =>
      office && open
        ? {
            name: office.name,
            city: office.city ?? "",
            address: office.address ?? "",
            phone: office.phone ?? "",
            sort_order: office.sort_order ?? 0,
          }
        : EMPTY,
    [office, open],
  );

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<OfficeFormValues>({
    resolver: zodResolver(officeFormSchema),
    values,
  });

  const onSubmit = async (body: OfficeFormValues) => {
    try {
      if (office) await update.mutateAsync({ uuid: office.uuid, body });
      else await create.mutateAsync(body);
      onOpenChange(false);
    } catch {
      // Toast handled by the mutation.
    }
  };

  const saving = create.isPending || update.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t("company_structure.offices.dialog.edit_title")
              : t("company_structure.offices.dialog.create_title")}
          </DialogTitle>
          <DialogDescription>
            {t("company_structure.offices.dialog.description")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="office-name">
              {t("company_structure.offices.fields.name")}
            </Label>
            <Input id="office-name" autoComplete="off" {...register("name")} />
            {errors.name ? (
              <p className="text-destructive text-xs">
                {t("company_structure.validation.name_required")}
              </p>
            ) : null}
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="office-city">
                {t("company_structure.offices.fields.city")}
              </Label>
              <Input
                id="office-city"
                autoComplete="off"
                {...register("city")}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="office-phone">
                {t("company_structure.offices.fields.phone")}
              </Label>
              <Input
                id="office-phone"
                type="tel"
                autoComplete="off"
                {...register("phone")}
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="office-address">
              {t("company_structure.offices.fields.address")}
            </Label>
            <Textarea id="office-address" rows={3} {...register("address")} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="office-sort">
              {t("company_structure.offices.fields.sort_order")}
            </Label>
            <Input
              id="office-sort"
              type="number"
              min={0}
              className="w-32"
              {...register("sort_order", { valueAsNumber: true })}
            />
            <p className="text-muted-foreground text-xs">
              {t("company_structure.offices.fields.sort_order_hint")}
            </p>
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
