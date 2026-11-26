"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

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
import { ProfilePicker } from "@/features/profiles/components/profile-picker";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

import { useLinkProfile } from "../hooks/use-company-bulk";

type Props = {
  member: { user_uuid: string; name: string; profileUuid?: string } | null;
  onOpenChange: (open: boolean) => void;
};

/** Owners/admins pick which company profile is a member's employee profile. */
export function LinkProfileDialog({ member, onOpenChange }: Props) {
  return member ? (
    <LinkProfileDialogBody
      key={member.user_uuid}
      member={member}
      onOpenChange={onOpenChange}
    />
  ) : null;
}

function LinkProfileDialogBody({
  member,
  onOpenChange,
}: {
  member: NonNullable<Props["member"]>;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useLocale();
  const [profile, setProfile] = useState(member.profileUuid ?? "");
  const link = useLinkProfile();
  const profilesQuery = useQuery({
    queryKey: ["company-bulk", "link-candidates"],
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
    staleTime: 30_000,
  });
  const save = (profileUuid: string | null) =>
    link.mutate(
      { userUuid: member.user_uuid, profileUuid },
      { onSuccess: () => onOpenChange(false) },
    );

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("company_bulk.link.title")}</DialogTitle>
          <DialogDescription>
            {t("company_bulk.link.description", { name: member.name })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <Label id="link-profile-label">
            {t("company_bulk.link.profile")}
          </Label>
          <ProfilePicker
            profiles={profilesQuery.data?.items ?? []}
            value={profile}
            onChange={setProfile}
            labelId="link-profile-label"
            placeholder={t("company_bulk.link.pick")}
          />
          <p className="text-muted-foreground text-xs">
            {t("company_bulk.link.hint")}
          </p>
        </div>
        <DialogFooter className="gap-2 sm:gap-0">
          {member.profileUuid ? (
            <Button
              type="button"
              variant="outline"
              className="me-auto"
              disabled={link.isPending}
              onClick={() => save(null)}
            >
              {t("company_bulk.link.unlink")}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            onClick={() => onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            disabled={!profile || link.isPending}
            onClick={() => save(profile)}
          >
            {t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
