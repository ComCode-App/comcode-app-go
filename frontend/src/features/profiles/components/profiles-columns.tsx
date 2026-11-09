"use client";

import Link from "next/link";
import { useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Star, User } from "lucide-react";

import { createColumn } from "@/components/tables";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import { ProfileCompletionBadge } from "@/features/profiles/components/profile-completion-badge";
import {
  ProfileRowActionsMenu,
  type ProfileRowActionHandlers,
} from "@/features/profiles/components/profile-row-actions";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

export function useProfilesColumns(handlers: ProfileRowActionHandlers) {
  const { t } = useLocale();

  return useMemo(
    () =>
      [
        createColumn<ProfileSummary>({
          accessorKey: "first_name",
          labelKey: "profiles.fields.first_name",
          enableSorting: true,
          filterVariant: "text",
          gridPrimary: true,
          cell: ({ row }) => {
            const item = row.original;
            const fullName =
              `${item.first_name} ${item.last_name}`.trim() ||
              "Untitled Profile";
            const initials =
              `${item.first_name?.[0] || ""}${item.last_name?.[0] || ""}`.toUpperCase() ||
              "ID";

            return (
              <div className="flex items-center gap-3">
                <Avatar className="h-9 w-9 rounded-xl border">
                  <AvatarImage src={item.avatar_url} alt={fullName} />
                  <AvatarFallback className="rounded-xl text-xs font-semibold">
                    {initials || <User className="h-4 w-4" />}
                  </AvatarFallback>
                </Avatar>
                <div className="flex flex-col">
                  <div className="flex items-center gap-1.5">
                    <Link
                      href={routes.app.profiles.detail(item.uuid)}
                      className="text-foreground font-medium hover:underline"
                    >
                      {fullName}
                    </Link>
                    {item.is_primary && (
                      <Badge
                        variant="secondary"
                        className="gap-1 px-1.5 py-0 text-[10px] font-normal"
                      >
                        <Star className="h-3 w-3 fill-amber-500 text-amber-500" />
                        {t("profiles.fields.is_primary")}
                      </Badge>
                    )}
                  </div>
                  <div className="text-muted-foreground flex items-center gap-1.5 text-xs">
                    <span className="bg-muted rounded px-1 font-mono text-[10px] uppercase">
                      {item.primary_locale}
                    </span>
                    {item.company_name && <span>• {item.company_name}</span>}
                  </div>
                </div>
              </div>
            );
          },
        }),
        createColumn<ProfileSummary>({
          accessorKey: "title",
          labelKey: "profiles.fields.title",
          enableSorting: false,
          cell: ({ row }) => (
            <div className="flex flex-col">
              <span className="text-sm">{row.original.title || "-"}</span>
              {row.original.company_name && (
                <span className="text-muted-foreground text-xs">
                  {row.original.company_name}
                </span>
              )}
            </div>
          ),
        }),
        createColumn<ProfileSummary>({
          accessorKey: "username",
          labelKey: "profiles.columns.public_url",
          enableSorting: false,
          cell: ({ row }) =>
            row.original.username ? (
              <a
                href={`/${row.original.username}`}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
                className="text-primary text-sm hover:underline"
              >
                /{row.original.username}
              </a>
            ) : (
              <span className="text-muted-foreground text-sm">-</span>
            ),
        }),
        createColumn<ProfileSummary>({
          accessorKey: "completion_rate",
          labelKey: "profiles.fields.completion_rate",
          enableSorting: false,
          cell: ({ row }) => (
            <ProfileCompletionBadge rate={row.original.completion_rate} />
          ),
        }),
        createColumn<ProfileSummary>({
          id: "actions",
          labelKey: "table.actions",
          enableSorting: false,
          enableColumnFilter: false,
          cell: ({ row }) => (
            <ProfileRowActionsMenu profile={row.original} handlers={handlers} />
          ),
        }),
      ] as ColumnDef<ProfileSummary, unknown>[],
    [handlers, t],
  );
}
