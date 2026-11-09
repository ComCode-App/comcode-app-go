"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Lock, QrCode, Settings, User, ExternalLink, Share2, Sparkles } from "lucide-react";
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
              <span className="font-semibold text-foreground">{card.name}</span>
              {card.is_locked && (
                <Badge variant="danger" className="text-[10px] px-1.5 py-0 h-4">
                  <Lock className="w-2.5 h-2.5 mr-0.5" />
                  {t("cards.fields.is_locked")}
                </Badge>
              )}
            </div>
            <span className="font-mono text-xs text-muted-foreground">{card.smart_code}</span>
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
          <div className="flex items-center gap-1.5 text-xs text-foreground">
            {card.target_type === "profile" && <User className="w-3.5 h-3.5 text-primary" />}
            {card.target_type === "direct_url" && <ExternalLink className="w-3.5 h-3.5 text-primary" />}
            {card.target_type === "vcard_download" && <Share2 className="w-3.5 h-3.5 text-primary" />}
            {card.target_type === "social_hub" && <Sparkles className="w-3.5 h-3.5 text-primary" />}
            <span className="capitalize">
              {card.target_type === "profile" && t("cards.settings.target_profile")}
              {card.target_type === "direct_url" && (card.direct_url || t("cards.settings.target_direct_url"))}
              {card.target_type === "vcard_download" && t("cards.settings.target_vcard")}
              {card.target_type === "social_hub" && t("cards.settings.target_social")}
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
        return <span className="text-xs">{translated.startsWith("cards.") ? val : translated}</span>;
      },
    }),

    createColumn<CardSummary>({
      accessorKey: "is_active",
      labelKey: "cards.columns.is_active",
      cell: ({ getValue }) => (
        <Badge variant={getValue() ? "success" : "secondary"} className="text-xs">
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
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              onClick={() => onQRClick(card)}
              title="QR Code"
            >
              <QrCode className="w-4 h-4" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              onClick={() => onToggleLock(card)}
              title={card.is_locked ? t("cards.unlock") : t("cards.lock")}
            >
              <Lock className="w-4 h-4" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              onClick={() => onSettingsClick(card)}
              title={t("common.edit")}
            >
              <Settings className="w-4 h-4" />
            </Button>
          </div>
        );
      },
    }),
  ];
}
