"use client";

import type { ColumnDef } from "@tanstack/react-table";
import Link from "next/link";
import {
  CreditCard,
  ExternalLink,
  Eye,
  FileUp,
  ShoppingBag,
  Truck,
} from "lucide-react";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";

import { isOrderPayable, type OrderSummary } from "../services/orders.service";
import { OrderStatusBadge } from "./order-status-badge";

interface MyOrdersColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  locale: string;
  canPay: boolean;
  payPending: boolean;
  onPay: (order: OrderSummary) => void;
}

// Columns for the subscriber "Siparişlerim" table (TEC-1319). Sorting stays off: GET /v1/orders
// always returns newest first.
export function getMyOrdersColumns({
  t,
  locale,
  canPay,
  payPending,
  onPay,
}: MyOrdersColumnOptions): ColumnDef<OrderSummary, unknown>[] {
  const intlLocale = locale === "tr" ? "tr-TR" : "en-US";

  return [
    createColumn<OrderSummary>({
      accessorKey: "order_number",
      labelKey: "orders.columns.order_number",
      enableSorting: false,
      enableHiding: false,
      gridPrimary: true,
      cell: ({ row }) => {
        const item = row.original;
        const needsAsset = item.status === "design_pending";
        return (
          <div className="flex items-center gap-2.5">
            <div
              className={cn(
                "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border",
                needsAsset
                  ? "border-amber-500/40 bg-amber-500/10 text-amber-600"
                  : "bg-muted/50 text-foreground",
              )}
            >
              {needsAsset ? (
                <FileUp className="h-4 w-4" />
              ) : (
                <ShoppingBag className="h-4 w-4" />
              )}
            </div>
            <span className="text-foreground font-mono font-bold tracking-wide">
              #{item.order_number}
            </span>
          </div>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "created_at",
      labelKey: "orders.subscriber.col_date",
      enableSorting: false,
      gridSecondary: true,
      cell: ({ row }) => (
        <span className="text-muted-foreground text-sm whitespace-nowrap">
          {new Date(row.original.created_at).toLocaleDateString(intlLocale, {
            dateStyle: "medium",
          })}
        </span>
      ),
    }),
    createColumn<OrderSummary>({
      accessorKey: "status",
      labelKey: "orders.columns.status",
      enableSorting: false,
      cell: ({ row }) => {
        const item = row.original;
        const needsAsset = item.status === "design_pending";
        return (
          <div className="flex flex-wrap items-center gap-1.5">
            <OrderStatusBadge status={item.status} />
            {isOrderPayable(item) && (
              <Badge variant="warning" className="gap-1 text-[11px]">
                <CreditCard className="h-3 w-3" />
                {t("orders.checkout.awaiting_payment")}
              </Badge>
            )}
            {needsAsset && (
              <Badge variant="warning" className="gap-1 text-[11px]">
                <FileUp className="h-3 w-3" />
                {t("orders.subscriber.action_required_badge")}
              </Badge>
            )}
          </div>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "items_count",
      labelKey: "orders.subscriber.col_items",
      enableSorting: false,
      cell: ({ row }) => {
        const count = row.original.items_count;
        return (
          <span className="text-sm whitespace-nowrap">
            {count === 1
              ? t("orders.subscriber.items_count_one", { count })
              : t("orders.subscriber.items_count_other", { count })}
          </span>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "tracking_number",
      labelKey: "orders.columns.shipping",
      enableSorting: false,
      cell: ({ row }) => {
        const item = row.original;
        if (!item.tracking_number) {
          return <span className="text-muted-foreground text-xs">—</span>;
        }
        const label = (
          <span className="flex items-center gap-1 font-mono text-xs">
            <Truck className="text-primary h-3.5 w-3.5 shrink-0" />
            {item.tracking_number}
            {item.tracking_url && <ExternalLink className="h-3 w-3" />}
          </span>
        );
        return item.tracking_url ? (
          <a
            href={item.tracking_url}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-primary hover:underline"
            aria-label={t("orders.subscriber.tracking_label")}
            onClick={(e) => e.stopPropagation()}
          >
            {label}
          </a>
        ) : (
          label
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "grand_total",
      labelKey: "orders.subscriber.total_amount",
      enableSorting: false,
      cell: ({ row }) => (
        <span className="text-primary font-mono font-bold whitespace-nowrap">
          {row.original.grand_total.toLocaleString(intlLocale, {
            minimumFractionDigits: 2,
          })}{" "}
          {row.original.currency}
        </span>
      ),
    }),
    createColumn<OrderSummary>({
      id: "actions",
      labelKey: "common.actions",
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const item = row.original;
        const needsAsset = item.status === "design_pending";
        const payable = isOrderPayable(item);
        return (
          <div
            className="flex items-center justify-end gap-2"
            onClick={(e) => e.stopPropagation()}
          >
            {payable && canPay && (
              <Button
                size="sm"
                className="h-8 gap-1.5 text-xs"
                disabled={payPending}
                onClick={() => onPay(item)}
              >
                <CreditCard className="h-3.5 w-3.5" />
                {t("orders.checkout.pay_now")}
              </Button>
            )}
            <Button
              size="sm"
              variant={needsAsset ? "default" : "outline"}
              className="h-8 gap-1.5 text-xs"
              asChild
            >
              <Link href={routes.app.orders.detail(item.uuid)}>
                {needsAsset ? (
                  <>
                    <FileUp className="h-3.5 w-3.5" />
                    {t("orders.subscriber.upload_file_btn")}
                  </>
                ) : (
                  <>
                    <Eye className="h-3.5 w-3.5" />
                    {t("orders.subscriber.view_order_btn")}
                  </>
                )}
              </Link>
            </Button>
          </div>
        );
      },
    }),
  ];
}
