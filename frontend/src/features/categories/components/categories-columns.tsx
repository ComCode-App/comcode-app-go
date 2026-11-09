"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Tag, Pencil, Trash2 } from "lucide-react";

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

import type { CategorySummary } from "../services/categories.service";

interface ColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  onEdit: (cat: CategorySummary) => void;
  onDelete: (uuid: string) => void;
  onToggleActive: (cat: CategorySummary) => void;
}

export function getCategoriesColumns({
  t,
  onEdit,
  onDelete,
  onToggleActive,
}: ColumnOptions): ColumnDef<CategorySummary, unknown>[] {
  return [
    createColumn<CategorySummary>({
      accessorKey: "name",
      labelKey: "categories.columns.name",
      enableSorting: true,
      gridPrimary: true,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center gap-3">
            <div className="bg-muted/50 text-muted-foreground flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border">
              <Tag className="h-4 w-4" />
            </div>
            <div className="flex flex-col">
              <span className="text-foreground font-medium">{item.name}</span>
              <span className="text-muted-foreground font-mono text-xs">
                /{item.slug}
              </span>
            </div>
          </div>
        );
      },
    }),
    createColumn<CategorySummary>({
      accessorKey: "description",
      labelKey: "categories.columns.description",
      enableSorting: false,
      cell: ({ row }) => (
        <span className="text-muted-foreground line-clamp-1 max-w-[280px] text-sm">
          {row.original.description || "—"}
        </span>
      ),
    }),
    createColumn<CategorySummary>({
      accessorKey: "icon",
      labelKey: "categories.columns.icon",
      enableSorting: false,
      cell: ({ row }) =>
        row.original.icon ? (
          <Badge variant="outline" className="font-mono text-xs">
            {row.original.icon}
          </Badge>
        ) : (
          <span className="text-muted-foreground">—</span>
        ),
    }),
    createColumn<CategorySummary>({
      accessorKey: "display_order",
      labelKey: "categories.columns.order",
      enableSorting: true,
      cell: ({ row }) => (
        <span className="font-mono text-xs">{row.original.display_order}</span>
      ),
    }),
    createColumn<CategorySummary>({
      accessorKey: "is_active",
      labelKey: "categories.columns.status",
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
    createColumn<CategorySummary>({
      id: "actions",
      labelKey: "common.actions",
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex items-center justify-end gap-1">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground hover:text-foreground h-8 w-8"
                    onClick={() => onEdit(item)}
                  >
                    <Pencil className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("common.edit")}</TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-destructive/80 hover:text-destructive hover:bg-destructive/10 h-8 w-8"
                    onClick={() => onDelete(item.uuid)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("common.delete")}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
        );
      },
    }),
  ];
}
