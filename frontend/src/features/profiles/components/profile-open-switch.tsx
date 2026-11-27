"use client";

import { useQuery } from "@tanstack/react-query";
import { Lock } from "lucide-react";
import { useId } from "react";

import { Switch } from "@/components/ui/switch";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { useSetProfileOpen } from "@/features/profiles/hooks/use-profile-mutations";
import {
  profilesService,
  type ProfileOpenState,
} from "@/features/profiles/services/profiles.service";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

type Props = {
  profileUuid: string;
  /** State from the profile list; without it the switch loads GET /v1/profiles/{uuid}/open. */
  state?: ProfileOpenState;
  /** Show the explanation line under the switch. */
  withHint?: boolean;
  className?: string;
};

/**
 * Open / closed switch of a profile (TEC-1326). A closed profile is not shown to visitors:
 * its links, cards and QR show the person's first open profile, and open profiles can be
 * swiped through. One click flips it, so it can be changed on the way to a meeting.
 */
export function ProfileOpenSwitch({
  profileUuid,
  state,
  withHint,
  className,
}: Props) {
  const { t } = useLocale();
  const id = useId();
  const setOpen = useSetProfileOpen();
  const query = useQuery({
    queryKey: profilesKeys.open(profileUuid),
    queryFn: () => profilesService.getOpen(profileUuid),
    enabled: !state,
  });
  // The latest answer wins: the mutation's, then the list's, then the switch's own load.
  const current = setOpen.data ?? state ?? query.data;
  const isOpen = setOpen.isPending
    ? Boolean(setOpen.variables?.isOpen)
    : Boolean(current?.is_open);
  const locked = Boolean(current?.open_locked);
  const disabled =
    !current || !current.can_toggle_open || locked || setOpen.isPending;

  return (
    <div
      className={cn("flex flex-col gap-1", className)}
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <div className="flex items-center gap-2">
        <Switch
          id={id}
          checked={isOpen}
          disabled={disabled}
          onCheckedChange={(checked) =>
            setOpen.mutate({ uuid: profileUuid, isOpen: checked })
          }
          aria-describedby={withHint || locked ? `${id}-hint` : undefined}
        />
        <label
          htmlFor={id}
          className={cn(
            "text-sm font-medium",
            disabled && "text-muted-foreground",
          )}
        >
          {isOpen
            ? t("profiles.open_switch.open")
            : t("profiles.open_switch.off")}
        </label>
        {locked ? (
          <Lock className="text-muted-foreground size-3.5" aria-hidden />
        ) : null}
      </div>
      {locked ? (
        <p id={`${id}-hint`} className="text-muted-foreground text-xs">
          {t("profiles.open_switch.locked_hint")}
        </p>
      ) : withHint ? (
        <p id={`${id}-hint`} className="text-muted-foreground text-xs">
          {t("profiles.open_switch.hint")}
        </p>
      ) : null}
    </div>
  );
}
