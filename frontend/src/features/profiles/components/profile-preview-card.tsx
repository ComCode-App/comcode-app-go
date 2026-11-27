"use client";

import type { CSSProperties } from "react";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, Star, User } from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  getTheme,
  type ProfileTheme,
} from "@/features/public-profile/lib/themes";
import { ProfileCompletionBadge } from "@/features/profiles/components/profile-completion-badge";
import { ProfileOpenSwitch } from "@/features/profiles/components/profile-open-switch";
import {
  ProfileRowActionsMenu,
  type ProfileRowActionHandlers,
} from "@/features/profiles/components/profile-row-actions";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  profilesService,
  type ProfileSummary,
} from "@/features/profiles/services/profiles.service";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

// The app's own look when the profile has no theme (no profile or account theme).
const DEFAULT_COVER =
  "linear-gradient(135deg, var(--primary) 0%, color-mix(in oklab, var(--primary) 55%, #000) 100%)";

/**
 * Grid card for /profiles (TEC-1321): a small mock of the profile's public page, drawn with
 * the page's own theme (profile theme, else the inherited account theme) and publish state.
 * Public pages cannot be framed (TEC-1119) and a frame would count visits, so the preview
 * is rendered from data instead of an iframe.
 */
export function ProfilePreviewCard({
  profile,
  handlers,
}: {
  profile: ProfileSummary;
  handlers: ProfileRowActionHandlers;
}) {
  const { t } = useLocale();

  // Same key as the public page settings card, so the detail page reuses this fetch.
  const pageQuery = useQuery({
    queryKey: profilesKeys.publicPage(profile.uuid),
    queryFn: () => profilesService.getPublicPage(profile.uuid),
    staleTime: 5 * 60_000,
  });
  const page = pageQuery.data;
  const theme: ProfileTheme | null = getTheme(
    page?.theme || page?.inherited_theme || null,
  );

  const fullName =
    `${profile.first_name} ${profile.last_name}`.trim() ||
    t("profiles.preview_card.untitled");
  const initials = `${profile.first_name?.[0] || ""}${
    profile.last_name?.[0] || ""
  }`.toUpperCase();

  const surface: CSSProperties = theme
    ? {
        background: theme.colors.card,
        color: theme.colors.foreground,
        borderColor: theme.colors.border,
      }
    : {};
  const mutedText: CSSProperties = theme
    ? { color: theme.colors.mutedForeground }
    : {};
  const pill: CSSProperties = theme
    ? {
        background: `linear-gradient(135deg, ${theme.colors.primary}, ${theme.colors.primaryEnd})`,
        color: theme.colors.primaryForeground,
      }
    : {};

  return (
    <div className="-m-4 flex h-full flex-col">
      {/* Mini public page */}
      <div
        className="relative overflow-hidden border-b p-3 pb-4"
        style={{ background: theme?.backdrop ?? undefined }}
      >
        <div className="absolute end-2.5 top-2.5 z-10 flex items-center gap-1.5">
          {profile.is_primary && (
            <Badge
              variant="secondary"
              className="gap-1 px-1.5 py-0 text-[10px]"
            >
              <Star className="h-3 w-3 fill-amber-500 text-amber-500" />
              {t("profiles.preview_card.primary")}
            </Badge>
          )}
          {page && (
            <Badge
              variant={page.enabled ? "success" : "outline"}
              className="bg-background/80 px-1.5 py-0 text-[10px] backdrop-blur"
            >
              {page.enabled
                ? t("profiles.preview_card.published")
                : t("profiles.preview_card.unpublished")}
            </Badge>
          )}
        </div>

        <div
          className={cn(
            "mx-auto w-full max-w-[260px] overflow-hidden rounded-2xl border shadow-sm",
            !theme && "bg-card",
            pageQuery.isLoading && "animate-pulse",
          )}
          style={surface}
        >
          <div
            className="h-14 w-full"
            style={{ background: theme?.cover ?? DEFAULT_COVER }}
          />
          <div className="flex flex-col items-center px-3 pb-3 text-center">
            <Avatar className="-mt-7 h-14 w-14 rounded-full border-2 border-white shadow-md">
              <AvatarImage src={profile.avatar_url} alt={fullName} />
              <AvatarFallback className="rounded-full text-sm font-semibold">
                {initials || <User className="h-5 w-5" />}
              </AvatarFallback>
            </Avatar>
            <p
              className="mt-2 line-clamp-1 text-sm font-semibold"
              style={theme ? { fontWeight: theme.headingWeight } : undefined}
            >
              {fullName}
            </p>
            <p
              className="text-muted-foreground line-clamp-1 min-h-4 text-[11px]"
              style={mutedText}
            >
              {[profile.title, profile.company_name]
                .filter(Boolean)
                .join(" · ") || " "}
            </p>
            {/* Contact button placeholders, like the public page's action row */}
            <div className="mt-2.5 flex w-full gap-1.5" aria-hidden="true">
              <span
                className="bg-primary text-primary-foreground flex h-6 flex-1 items-center justify-center rounded-full text-[10px] font-medium"
                style={pill}
              >
                {t("profiles.preview_card.save_contact")}
              </span>
              <span
                className="bg-muted h-6 w-6 rounded-full"
                style={theme ? { background: theme.colors.muted } : undefined}
              />
              <span
                className="bg-muted h-6 w-6 rounded-full"
                style={theme ? { background: theme.colors.muted } : undefined}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Owner info */}
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="min-w-0">
          <p className="text-foreground truncate font-semibold">{fullName}</p>
          <div className="text-muted-foreground flex items-center gap-1.5 text-xs">
            <span className="bg-muted rounded px-1 font-mono text-[10px] uppercase">
              {profile.primary_locale}
            </span>
            {profile.username ? (
              <a
                href={`/${profile.username}`}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
                className="text-primary inline-flex min-w-0 items-center gap-1 hover:underline"
              >
                <span className="truncate">/{profile.username}</span>
                <ExternalLink className="h-3 w-3 shrink-0" />
              </a>
            ) : (
              <span>{t("profiles.preview_card.no_address")}</span>
            )}
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <ProfileCompletionBadge rate={profile.completion_rate} />
          {/* Open switch (TEC-1326): one click before a meeting. */}
          <ProfileOpenSwitch
            profileUuid={profile.uuid}
            state={{
              is_open: profile.is_open ?? true,
              open_locked: profile.open_locked ?? false,
              can_toggle_open: profile.can_toggle_open ?? false,
            }}
          />
        </div>
        <div className="border-border/60 mt-auto border-t pt-2">
          <ProfileRowActionsMenu profile={profile} handlers={handlers} />
        </div>
      </div>
    </div>
  );
}
