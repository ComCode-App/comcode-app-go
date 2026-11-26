"use client";

import { Check, ChevronsUpDown } from "lucide-react";
import { useState } from "react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/** Up to this many profiles are shown as cards; more switch to a searchable list. */
export const PROFILE_CARD_LIMIT = 6;

type Props = {
  profiles: ProfileSummary[];
  value: string;
  onChange: (uuid: string) => void;
  /** Id of the visible label, for the radio group / list name. */
  labelId: string;
  placeholder: string;
  disabled?: boolean;
};

const fullName = (p: ProfileSummary) => `${p.first_name} ${p.last_name}`.trim();

const subline = (p: ProfileSummary) =>
  [p.title, p.company_name].filter(Boolean).join(" · ");

function ProfileAvatar({
  profile,
  className,
}: {
  profile: ProfileSummary;
  className?: string;
}) {
  const initials = [profile.first_name, profile.last_name]
    .map((part) => part?.trim()[0]?.toUpperCase())
    .filter(Boolean)
    .join("");
  return (
    <Avatar className={cn("size-10 shrink-0", className)}>
      {profile.avatar_url ? (
        <AvatarImage src={profile.avatar_url} alt="" />
      ) : null}
      <AvatarFallback className="text-xs">{initials || "?"}</AvatarFallback>
    </Avatar>
  );
}

/** Name, title/company and public path of one profile. */
function ProfileLines({ profile }: { profile: ProfileSummary }) {
  const { t } = useLocale();
  const sub = subline(profile);
  return (
    <span className="min-w-0 flex-1 text-left">
      <span className="flex items-center gap-1.5">
        <span className="truncate text-sm font-medium">
          {fullName(profile) || profile.username || "—"}
        </span>
        {profile.is_primary ? (
          <Badge variant="secondary" className="shrink-0 px-1.5 text-[10px]">
            {t("profiles.primary")}
          </Badge>
        ) : null}
      </span>
      {sub ? (
        <span className="text-muted-foreground block truncate text-xs">
          {sub}
        </span>
      ) : null}
      {profile.username ? (
        <span className="text-muted-foreground block truncate font-mono text-[11px]">
          /{profile.username}
        </span>
      ) : null}
    </span>
  );
}

/**
 * Profile choice with a preview of each profile (TEC-1313): avatar, name, title and public
 * path, so profiles with the same name stay apart. A few profiles are cards to click; more
 * than `PROFILE_CARD_LIMIT` become a searchable list that previews the chosen one.
 */
export function ProfilePicker({
  profiles,
  value,
  onChange,
  labelId,
  placeholder,
  disabled,
}: Props) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  const selected = profiles.find((p) => p.uuid === value);

  if (profiles.length <= PROFILE_CARD_LIMIT) {
    return (
      <div
        role="radiogroup"
        aria-labelledby={labelId}
        className="grid gap-2 sm:grid-cols-2"
      >
        {profiles.map((p) => {
          const isSelected = p.uuid === value;
          return (
            <button
              key={p.uuid}
              type="button"
              role="radio"
              aria-checked={isSelected}
              disabled={disabled}
              onClick={() => onChange(p.uuid)}
              className={cn(
                "flex items-center gap-3 rounded-lg border p-3 transition-colors",
                "focus-visible:ring-ring/50 outline-none focus-visible:ring-[3px] disabled:opacity-60",
                isSelected
                  ? "border-primary bg-primary/5 ring-primary/30 ring-2"
                  : "hover:border-foreground/30",
              )}
            >
              <ProfileAvatar profile={p} />
              <ProfileLines profile={p} />
              {isSelected ? (
                <Check className="text-primary size-4 shrink-0" />
              ) : null}
            </button>
          );
        })}
      </div>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-labelledby={labelId}
          disabled={disabled}
          className={cn(
            "border-input bg-background flex w-full items-center gap-3 rounded-lg border p-3 transition-colors",
            "focus-visible:ring-ring/50 outline-none focus-visible:ring-[3px] disabled:opacity-60",
            "hover:border-foreground/30",
          )}
        >
          {selected ? (
            <>
              <ProfileAvatar profile={selected} />
              <ProfileLines profile={selected} />
            </>
          ) : (
            <span className="text-muted-foreground flex-1 text-left text-sm">
              {placeholder}
            </span>
          )}
          <ChevronsUpDown className="text-muted-foreground size-4 shrink-0" />
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-(--radix-popover-trigger-width) p-0"
      >
        <Command>
          <CommandInput placeholder={t("profiles.picker.search")} />
          <CommandList className="max-h-72">
            <CommandEmpty>{t("profiles.picker.empty")}</CommandEmpty>
            {profiles.map((p) => (
              <CommandItem
                key={p.uuid}
                value={`${fullName(p)} ${p.username ?? ""} ${subline(p)} ${p.uuid}`}
                onSelect={() => {
                  onChange(p.uuid);
                  setOpen(false);
                }}
                className="gap-3"
              >
                <ProfileAvatar profile={p} className="size-8" />
                <ProfileLines profile={p} />
                {p.uuid === value ? (
                  <Check className="text-primary size-4 shrink-0" />
                ) : null}
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
