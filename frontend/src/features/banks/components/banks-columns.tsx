"use client";

import { useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Landmark, Pencil, Trash2 } from "lucide-react";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import type { BankSummary } from "../services/banks.service";

interface ColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  onEdit: (bank: BankSummary) => void;
  onDelete: (uuid: string) => void;
  onToggleActive: (bank: BankSummary) => void;
}

function BankLogoCell({ item }: { item: BankSummary }) {
  const [error, setError] = useState(false);

  return (
    <div className="flex items-center gap-3">
      <div
        className="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-xl text-xs font-bold text-white shadow-xs"
        style={{ backgroundColor: item.brand_color || "#1e3a8a" }}
      >
        {item.icon_url && !error ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.icon_url}
            alt={item.name}
            onError={() => setError(true)}
            className="h-full w-full object-contain p-1"
          />
        ) : (
          <Landmark className="h-4 w-4 text-white" />
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

export function getBanksColumns({
  t,
  onEdit,
  onDelete,
  onToggleActive,
}: ColumnOptions): ColumnDef<BankSummary, unknown>[] {
  return [
    createColumn<BankSummary>({
      accessorKey: "name",
      labelKey: "banks.fields.name",
      enableSorting: true,
      gridPrimary: true,
      cell: ({ row }) => <BankLogoCell item={row.original} />,
    }),
    createColumn<BankSummary>({
      accessorKey: "code",
      labelKey: "banks.fields.code",
      enableSorting: true,
      cell: ({ row }) => (
        <span className="bg-muted rounded px-2 py-0.5 font-mono text-xs font-semibold">
          {row.original.code}
        </span>
      ),
    }),
    createColumn<BankSummary>({
      accessorKey: "country_code",
      labelKey: "banks.fields.country_code",
      enableSorting: true,
      cell: ({ row }) => (
        <Badge variant="outline" className="font-mono text-xs">
          {row.original.country_code}
        </Badge>
      ),
    }),
    createColumn<BankSummary>({
      accessorKey: "bic_swift",
      labelKey: "banks.fields.bic_swift",
      enableSorting: false,
      cell: ({ row }) => (
        <span className="text-muted-foreground font-mono text-xs">
          {row.original.bic_swift || "—"}
        </span>
      ),
    }),
    createColumn<BankSummary>({
      accessorKey: "display_order",
      labelKey: "banks.fields.display_order",
      enableSorting: true,
      cell: ({ row }) => <span>{row.original.display_order}</span>,
    }),
    createColumn<BankSummary>({
      accessorKey: "is_active",
      labelKey: "banks.fields.is_active",
      enableSorting: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center gap-2">
            <Switch
              checked={item.is_active}
              onCheckedChange={() => onToggleActive(item)}
              className="scale-85"
            />
            <span className="text-muted-foreground text-xs">
              {item.is_active ? t("common.active") : t("common.inactive")}
            </span>
          </div>
        );
      },
    }),
    createColumn<BankSummary>({
      id: "actions",
      labelKey: "table.actions",
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
