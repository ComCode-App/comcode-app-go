"use client";

import { Loader2 } from "lucide-react";
import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useLocale } from "@/providers/locale-provider";

import { useAssignmentMutations } from "../hooks/use-company-structure";
import type { CompanyStructureAccess } from "../lib/access";
import type {
  CompanyAssignment,
  CompanyGroup,
  CompanyOffice,
} from "../services/company-structure.service";

const NONE = "__none__";

type AssignmentDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  member: { user_uuid: string; name: string } | null;
  assignment: CompanyAssignment | undefined;
  offices: CompanyOffice[];
  groups: CompanyGroup[];
  access: CompanyStructureAccess;
};

export function AssignmentDialog(props: AssignmentDialogProps) {
  // Remount per member so the local selection starts from the stored assignment.
  return props.open && props.member ? (
    <AssignmentDialogBody key={props.member.user_uuid} {...props} />
  ) : null;
}

function AssignmentDialogBody({
  open,
  onOpenChange,
  member,
  assignment,
  offices,
  groups,
  access,
}: AssignmentDialogProps) {
  const { t } = useLocale();
  const { setAssignment } = useAssignmentMutations();
  const [office, setOffice] = useState<string>(assignment?.office_uuid ?? NONE);
  const [group, setGroup] = useState<string>(assignment?.group_uuid ?? NONE);

  // Office managers move members only between their own offices and never clear the office.
  const officeOptions = useMemo(
    () =>
      access.isAdmin
        ? offices
        : offices.filter((o) => access.managesOffice(o.uuid)),
    [access, offices],
  );
  const groupOptions = useMemo(
    () =>
      groups.filter((g) => {
        if (g.office_uuid) return office !== NONE && g.office_uuid === office;
        // Company-wide groups fit any office but are owner/admin-only to assign.
        return access.isAdmin;
      }),
    [access.isAdmin, groups, office],
  );

  const onOfficeChange = (value: string) => {
    setOffice(value);
    const current = groups.find((g) => g.uuid === group);
    if (current?.office_uuid && current.office_uuid !== value) setGroup(NONE);
  };

  const onSave = async () => {
    if (!member) return;
    try {
      await setAssignment.mutateAsync({
        userUuid: member.user_uuid,
        body: {
          office_uuid: office === NONE ? null : office,
          group_uuid: group === NONE ? null : group,
        },
      });
      onOpenChange(false);
    } catch {
      // Toast handled by the mutation.
    }
  };

  const officeRequired = !access.isAdmin && office === NONE;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {t("company_structure.members.assignment_title")}
          </DialogTitle>
          <DialogDescription>
            {t("company_structure.members.assignment_description", {
              name: member?.name ?? "",
            })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label>{t("company_structure.members.fields.office")}</Label>
            <Select value={office} onValueChange={onOfficeChange}>
              <SelectTrigger
                aria-label={t("company_structure.members.fields.office")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {access.isAdmin ? (
                  <SelectItem value={NONE}>
                    {t("company_structure.members.no_office")}
                  </SelectItem>
                ) : null}
                {officeOptions.map((o) => (
                  <SelectItem key={o.uuid} value={o.uuid}>
                    {o.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>{t("company_structure.members.fields.group")}</Label>
            <Select value={group} onValueChange={setGroup}>
              <SelectTrigger
                aria-label={t("company_structure.members.fields.group")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NONE}>
                  {t("company_structure.members.no_group")}
                </SelectItem>
                {groupOptions.map((g) => (
                  <SelectItem key={g.uuid} value={g.uuid}>
                    {g.name}
                    {g.office_uuid
                      ? ""
                      : ` · ${t("company_structure.groups.company_wide")}`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-muted-foreground text-xs">
              {t("company_structure.members.group_hint")}
            </p>
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
          <Button
            type="button"
            onClick={() => void onSave()}
            disabled={setAssignment.isPending || officeRequired}
          >
            {setAssignment.isPending ? (
              <Loader2 className="mr-2 size-4 animate-spin" />
            ) : null}
            {t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
