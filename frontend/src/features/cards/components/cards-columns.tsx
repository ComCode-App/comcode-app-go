"use client";

import type { ColumnDef } from "@tanstack/react-table";
import {
  Lock,
  QrCode,
  Settings,
  User,
  ExternalLink,
  Share2,
  Sparkles,
} from "lucide-react";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { CardSummary } from "../services/cards.service";

interface ColumnOptions {
  t: (key: string) => string;
  onSettingsClick: (card: CardSummary) => void;
  onQRClick: (card: CardSummary) => void;
  onToggleLock: (card: CardSummary) => void;
}

export function createCardsColumns({
  t,
  onSettingsClick,
  onQRClick,
  onToggleLock,
}: ColumnOptions): ColumnDef<CardSummary, unknown>[] {
  return [
    createColumn<CardSummary>({
      accessorKey: "name",
      labelKey: "cards.columns.name",
      enableSorting: true,
      gridPrimary: true,
      cell: ({ row }) => {
        const card = row.original;
        return (
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-2">
              <span className="text-foreground font-semibold">{card.name}</span>
              {card.is_locked && (
                <Badge variant="danger" className="h-4 px-1.5 py-0 text-[10px]">
                  <Lock className="mr-0.5 h-2.5 w-2.5" />
                  {t("cards.fields.is_locked")}
                </Badge>
              )}
            </div>
            <span className="text-muted-foreground font-mono text-xs">
              {card.smart_code}
            </span>
          </div>
        );
      },
    }),

    createColumn<CardSummary>({
      accessorKey: "target_type",
      labelKey: "cards.columns.target_type",
      cell: ({ row }) => {
        const card = row.original;
        return (
          <div className="text-foreground flex items-center gap-1.5 text-xs">
            {card.target_type === "profile" && (
              <User className="text-primary h-3.5 w-3.5" />
            )}
            {card.target_type === "direct_url" && (
              <ExternalLink className="text-primary h-3.5 w-3.5" />
            )}
            {card.target_type === "vcard_download" && (
              <Share2 className="text-primary h-3.5 w-3.5" />
            )}
            {card.target_type === "social_hub" && (
              <Sparkles className="text-primary h-3.5 w-3.5" />
            )}
            <span className="capitalize">
              {card.target_type === "profile" &&
                t("cards.settings.target_profile")}
              {card.target_type === "direct_url" &&
                (card.direct_url || t("cards.settings.target_direct_url"))}
              {card.target_type === "vcard_download" &&
                t("cards.settings.target_vcard")}
              {card.target_type === "social_hub" &&
                t("cards.settings.target_social")}
            </span>
          </div>
        );
      },
    }),

    createColumn<CardSummary>({
      accessorKey: "material",
      labelKey: "cards.columns.material",
      cell: ({ getValue }) => {
        const val = String(getValue() || "");
        const key = `cards.materials.${val}`;
        const translated = t(key);
        return (
          <span className="text-xs">
            {translated.startsWith("cards.") ? val : translated}
          </span>
        );
      },
    }),

    createColumn<CardSummary>({
      accessorKey: "is_active",
      labelKey: "cards.columns.is_active",
      cell: ({ getValue }) => (
        <Badge
          variant={getValue() ? "success" : "secondary"}
          className="text-xs"
        >
          {getValue() ? t("common.active") : t("common.inactive")}
        </Badge>
      ),
    }),

    createColumn<CardSummary>({
      id: "actions",
      labelKey: "common.actions",
      cell: ({ row }) => {
        const card = row.original;
        return (
          <div className="flex items-center justify-end gap-1">
            <Button
              size="icon"
              variant="ghost"
              className="text-muted-foreground hover:text-foreground h-8 w-8"
              onClick={() => onQRClick(card)}
              title="QR Code"
            >
              <QrCode className="h-4 w-4" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="text-muted-foreground hover:text-foreground h-8 w-8"
              onClick={() => onToggleLock(card)}
              title={card.is_locked ? t("cards.unlock") : t("cards.lock")}
            >
              <Lock className="h-4 w-4" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="text-muted-foreground hover:text-foreground h-8 w-8"
              onClick={() => onSettingsClick(card)}
              title={t("common.edit")}
            >
              <Settings className="h-4 w-4" />
            </Button>
          </div>
        );
      },
    }),
  ];
}
