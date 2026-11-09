"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Loader2 } from "lucide-react";
import { useMemo } from "react";
import { Controller, useForm } from "react-hook-form";

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useLocale } from "@/providers/locale-provider";

import { useGroupMutations } from "../hooks/use-company-structure";
import type { CompanyStructureAccess } from "../lib/access";
import {
  COMPANY_WIDE,
  groupFormSchema,
  type GroupFormValues,
} from "../schemas/group-form";
import type {
  CompanyGroup,
  CompanyOffice,
} from "../services/company-structure.service";

type GroupDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  group: CompanyGroup | null;
  offices: CompanyOffice[];
  access: CompanyStructureAccess;
  /** Preselected office when creating from a filtered list. */
  defaultOffice?: string;
};

export function GroupDialog({
  open,
  onOpenChange,
  group,
  offices,
  access,
  defaultOffice,
}: GroupDialogProps) {
  const { t } = useLocale();
  const { create, update } = useGroupMutations();

  // Office managers only see their granted offices and cannot pick company-wide.
  const officeOptions = useMemo(
    () =>
      access.isAdmin
        ? offices
        : offices.filter((o) => access.managesOffice(o.uuid)),
    [access, offices],
  );
  const allowCompanyWide = access.isAdmin;

  const values = useMemo<GroupFormValues>(() => {
    if (group && open) {
      return {
        name: group.name,
        kind: group.kind,
        office: group.office_uuid ?? COMPANY_WIDE,
      };
    }
    const fallback = allowCompanyWide
      ? COMPANY_WIDE
      : (officeOptions[0]?.uuid ?? "");
    const preset =
      defaultOffice &&
      (defaultOffice === COMPANY_WIDE
        ? allowCompanyWide
        : officeOptions.some((o) => o.uuid === defaultOffice))
        ? defaultOffice
        : fallback;
    return { name: "", kind: "department", office: preset };
  }, [allowCompanyWide, defaultOffice, group, officeOptions, open]);

  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<GroupFormValues>({
    resolver: zodResolver(groupFormSchema),
    values,
  });

  const onSubmit = async (form: GroupFormValues) => {
    const body = {
      name: form.name,
      kind: form.kind,
      office_uuid: form.office === COMPANY_WIDE ? null : form.office,
    };
    try {
      if (group) await update.mutateAsync({ uuid: group.uuid, body });
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
            {group
              ? t("company_structure.groups.dialog.edit_title")
              : t("company_structure.groups.dialog.create_title")}
          </DialogTitle>
          <DialogDescription>
            {t("company_structure.groups.dialog.description")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="group-name">
              {t("company_structure.groups.fields.name")}
            </Label>
            <Input id="group-name" autoComplete="off" {...register("name")} />
            {errors.name ? (
              <p className="text-destructive text-xs">
                {t("company_structure.validation.name_required")}
              </p>
            ) : null}
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>{t("company_structure.groups.fields.kind")}</Label>
              <Controller
                control={control}
                name="kind"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger
                      aria-label={t("company_structure.groups.fields.kind")}
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="department">
                        {t("company_structure.groups.kinds.department")}
                      </SelectItem>
                      <SelectItem value="team">
                        {t("company_structure.groups.kinds.team")}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t("company_structure.groups.fields.office")}</Label>
              <Controller
                control={control}
                name="office"
                render={({ field }) => (
                  <Select
                    value={field.value || undefined}
                    onValueChange={field.onChange}
                  >
                    <SelectTrigger
                      aria-label={t("company_structure.groups.fields.office")}
                    >
                      <SelectValue
                        placeholder={t(
                          "company_structure.groups.select_office",
                        )}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {allowCompanyWide ? (
                        <SelectItem value={COMPANY_WIDE}>
                          {t("company_structure.groups.company_wide")}
                        </SelectItem>
                      ) : null}
                      {officeOptions.map((office) => (
                        <SelectItem key={office.uuid} value={office.uuid}>
                          {office.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {errors.office ? (
                <p className="text-destructive text-xs">
                  {t("company_structure.validation.office_required")}
                </p>
              ) : null}
            </div>
          </div>
          <p className="text-muted-foreground text-xs">
            {allowCompanyWide
              ? t("company_structure.groups.office_hint")
              : t("company_structure.groups.office_hint_scoped")}
          </p>
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
