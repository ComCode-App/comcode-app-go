"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Box, Sparkles, CheckCircle2, FileText, Pencil, Trash2 } from "lucide-react";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import type { ProductSummary } from "../services/products.service";

interface ColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  onEdit: (product: ProductSummary) => void;
  onDelete: (uuid: string) => void;
  onOpenStudio: (product: ProductSummary) => void;
}

export function getProductsColumns({
  t,
  onEdit,
  onDelete,
  onOpenStudio,
}: ColumnOptions): ColumnDef<ProductSummary, unknown>[] {
  return [
    createColumn<ProductSummary>({
      accessorKey: "name",
      labelKey: "products.columns.name",
      enableSorting: true,
      gridPrimary: true,
      cell: ({ row }) => {
        const item = row.original;
        const has3D = Boolean(item.model_3d_file_url || item.model_3d_usdz_url);
        return (
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-gradient-to-br from-zinc-800 to-zinc-950 text-white shadow-xs">
              <Box className="h-5 w-5 text-amber-400" />
            </div>
            <div className="flex flex-col">
              <div className="flex items-center gap-1.5">
                <span className="font-semibold text-foreground">{item.name}</span>
                {has3D && (
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0 border-amber-500/30 text-amber-500 bg-amber-500/5">
                    <Sparkles className="h-2.5 w-2.5 mr-0.5 inline" /> 3D
                  </Badge>
                )}
                {item.model_3d_usdz_url && (
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0 border-emerald-500/30 text-emerald-500 bg-emerald-500/5">
                    USDZ
                  </Badge>
                )}
              </div>
              <span className="text-xs text-muted-foreground font-mono">{item.sku || `/${item.slug}`}</span>
            </div>
          </div>
        );
      },
    }),
    createColumn<ProductSummary>({
      accessorKey: "category_name",
      labelKey: "products.columns.category",
      enableSorting: true,
      cell: ({ row }) => {
        const cat = row.original.category_name;
        return cat ? (
          <Badge variant="secondary" className="font-normal text-xs">
            {cat}
          </Badge>
        ) : (
          <span className="text-muted-foreground text-xs">—</span>
        );
      },
    }),
    createColumn<ProductSummary>({
      accessorKey: "base_price",
      labelKey: "products.columns.price",
      enableSorting: true,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex flex-col">
            <span className="font-mono font-medium text-foreground">
              {item.base_price.toLocaleString("tr-TR", { minimumFractionDigits: 2 })} {item.currency}
            </span>
            <span className="text-[11px] text-muted-foreground">+{item.tax_rate}% KDV</span>
          </div>
        );
      },
    }),
    createColumn<ProductSummary>({
      accessorKey: "stock_quantity",
      labelKey: "products.columns.stock",
      enableSorting: true,
      cell: ({ row }) => {
        const qty = row.original.stock_quantity;
        return (
          <Badge
            variant={qty > 10 ? "outline" : qty > 0 ? "secondary" : "danger"}
            className="font-mono text-xs"
          >
            {qty} adet
          </Badge>
        );
      },
    }),
    createColumn<ProductSummary>({
      accessorKey: "status",
      labelKey: "products.columns.status",
      enableSorting: true,
      cell: ({ row }) => {
        const status = row.original.status;
        if (status === "active") {
          return (
            <Badge variant="default" className="bg-emerald-600 hover:bg-emerald-700 text-xs">
              <CheckCircle2 className="h-3 w-3 mr-1" /> {t("common.active")}
            </Badge>
          );
        }
        if (status === "draft") {
          return (
            <Badge variant="outline" className="text-xs">
              <FileText className="h-3 w-3 mr-1" /> {t("common.draft")}
            </Badge>
          );
        }
        return (
          <Badge variant="secondary" className="text-xs">
            {t("common.archived")}
          </Badge>
        );
      },
    }),
    createColumn<ProductSummary>({
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
                    className="h-8 w-8 text-amber-500 hover:text-amber-400 hover:bg-amber-500/10"
                    onClick={() => onOpenStudio(item)}
                  >
                    <Sparkles className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>3D Viewbox Studio</TooltipContent>
              </Tooltip>
            </TooltipProvider>

            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-foreground"
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
                    className="h-8 w-8 text-destructive/80 hover:text-destructive hover:bg-destructive/10"
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
