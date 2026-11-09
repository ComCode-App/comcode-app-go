"use client";

import { Loader2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useLocale } from "@/providers/locale-provider";

import { useAssignmentMutations } from "../hooks/use-company-structure";
import type { CompanyOffice } from "../services/company-structure.service";

type ScopesDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  member: { user_uuid: string; name: string } | null;
  officeUuids: string[];
  offices: CompanyOffice[];
};

/** Owner/admin grants or revokes the offices an office manager controls. */
export function ScopesDialog(props: ScopesDialogProps) {
  return props.open && props.member ? (
    <ScopesDialogBody key={props.member.user_uuid} {...props} />
  ) : null;
}

function ScopesDialogBody({
  open,
  onOpenChange,
  member,
  officeUuids,
  offices,
}: ScopesDialogProps) {
  const { t } = useLocale();
  const { setScopes } = useAssignmentMutations();
  const [selected, setSelected] = useState<Set<string>>(
    () => new Set(officeUuids),
  );

  const toggle = (uuid: string, checked: boolean) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(uuid);
      else next.delete(uuid);
      return next;
    });
  };

  const onSave = async () => {
    if (!member) return;
    try {
      await setScopes.mutateAsync({
        userUuid: member.user_uuid,
        officeUuids: [...selected],
      });
      onOpenChange(false);
    } catch {
      // Toast handled by the mutation.
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {t("company_structure.members.scopes_title")}
          </DialogTitle>
          <DialogDescription>
            {t("company_structure.members.scopes_description", {
              name: member?.name ?? "",
            })}
          </DialogDescription>
        </DialogHeader>
        {offices.length ? (
          <div className="max-h-72 space-y-1 overflow-y-auto rounded-md border p-2">
            {offices.map((office) => {
              const id = `scope-${office.uuid}`;
              return (
                <label
                  key={office.uuid}
                  htmlFor={id}
                  className="hover:bg-muted/60 flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm"
                >
                  <Checkbox
                    id={id}
                    checked={selected.has(office.uuid)}
                    onCheckedChange={(checked) =>
                      toggle(office.uuid, checked === true)
                    }
                  />
                  <span className="font-medium">{office.name}</span>
                  {office.city ? (
                    <span className="text-muted-foreground text-xs">
                      {office.city}
                    </span>
                  ) : null}
                </label>
              );
            })}
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">
            {t("company_structure.members.no_offices")}
          </p>
        )}
        <p className="text-muted-foreground text-xs">
          {t("company_structure.members.scopes_hint")}
        </p>
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
            disabled={setScopes.isPending}
          >
            {setScopes.isPending ? (
              <Loader2 className="mr-2 size-4 animate-spin" />
            ) : null}
            {t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
