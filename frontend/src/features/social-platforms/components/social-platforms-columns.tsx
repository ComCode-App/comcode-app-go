"use client";

import { useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Globe, Pencil, Sparkles, Trash2 } from "lucide-react";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { SocialPlatformSummary } from "../services/social-platforms.service";

interface ColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  onEdit: (platform: SocialPlatformSummary) => void;
  onFetchIcon: (platform: SocialPlatformSummary) => void;
  onDelete: (uuid: string) => void;
  onToggleActive: (platform: SocialPlatformSummary, isActive: boolean) => void;
}

function PlatformIconCell({ item }: { item: SocialPlatformSummary }) {
  const [error, setError] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <div className="border-border bg-muted/30 flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-xl border shadow-xs">
        {item.icon_url && !error ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.icon_url}
            alt={item.name}
            onError={() => setError(true)}
            className="h-5 w-5 object-contain"
          />
        ) : (
          <Globe className="text-muted-foreground h-4 w-4" />
        )}
      </div>
      <div>
        <div className="text-foreground text-sm font-semibold">{item.name}</div>
        <div className="text-muted-foreground font-mono text-xs">
          {item.slug}
        </div>
      </div>
    </div>
  );
}

export function getSocialPlatformsColumns({
  t,
  onEdit,
  onFetchIcon,
  onDelete,
  onToggleActive,
}: ColumnOptions): ColumnDef<SocialPlatformSummary, unknown>[] {
  return [
    createColumn<SocialPlatformSummary>({
      accessorKey: "name",
      labelKey: "social_platforms.name",
      gridPrimary: true,
      enableSorting: true,
      cell: ({ row }) => <PlatformIconCell item={row.original} />,
    }),
    createColumn<SocialPlatformSummary>({
      accessorKey: "category",
      labelKey: "social_platforms.category",
      enableSorting: true,
      cell: ({ row }) => {
        const cat = row.original.category;
        const catKey = `social_platforms.categories.${cat}`;
        return (
          <Badge variant="outline" className="text-xs font-normal capitalize">
            {t(catKey)}
          </Badge>
        );
      },
    }),
    createColumn<SocialPlatformSummary>({
      accessorKey: "url_pattern",
      labelKey: "social_platforms.url_pattern",
      enableSorting: false,
      cell: ({ row }) => {
        return (
          <span className="text-muted-foreground block max-w-xs truncate font-mono text-xs">
            {row.original.url_pattern}
          </span>
        );
      },
    }),
    createColumn<SocialPlatformSummary>({
      accessorKey: "display_order",
      labelKey: "social_platforms.display_order",
      enableSorting: true,
      cell: ({ row }) => (
        <span className="text-muted-foreground text-xs font-semibold">
          {row.original.display_order}
        </span>
      ),
    }),
    createColumn<SocialPlatformSummary>({
      accessorKey: "is_active",
      labelKey: "social_platforms.is_active",
      enableSorting: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center gap-2">
            <Switch
              checked={item.is_active}
              onCheckedChange={(checked) => onToggleActive(item, checked)}
            />
            <span className="text-muted-foreground text-xs">
              {item.is_active ? t("common.active") : t("common.inactive")}
            </span>
          </div>
        );
      },
    }),
    createColumn<SocialPlatformSummary>({
      id: "actions",
      labelKey: "common.actions",
      enableSorting: false,
      enableHiding: false,
      enableResizing: false,
      enableColumnFilter: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center justify-end gap-1">
            <TooltipProvider delayDuration={200}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground hover:text-primary hover:bg-primary/10 h-8 w-8"
                    onClick={() => onFetchIcon(item)}
                  >
                    <Sparkles className="h-4 w-4" />
                    <span className="sr-only">
                      {t("social_platforms.actions.fetch_icon_og")}
                    </span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">
                    {t("social_platforms.actions.fetch_icon_og")}
                  </p>
                </TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground hover:text-foreground h-8 w-8"
                    onClick={() => onEdit(item)}
                  >
                    <Pencil className="h-4 w-4" />
                    <span className="sr-only">{t("common.edit")}</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">{t("common.edit")}</p>
                </TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground hover:text-destructive hover:bg-destructive/10 h-8 w-8"
                    onClick={() => onDelete(item.uuid)}
                  >
                    <Trash2 className="h-4 w-4" />
                    <span className="sr-only">{t("common.delete")}</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  <p className="text-xs">{t("common.delete")}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        );
      },
    }),
  ];
}
