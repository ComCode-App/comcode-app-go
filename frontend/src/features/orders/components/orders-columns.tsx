"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { ShoppingBag, Eye } from "lucide-react";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { OrderSummary } from "../services/orders.service";

interface ColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  onView: (order: OrderSummary) => void;
}

export function getOrdersColumns({
  t,
  onView,
}: ColumnOptions): ColumnDef<OrderSummary, unknown>[] {
  return [
    createColumn<OrderSummary>({
      accessorKey: "order_number",
      labelKey: "orders.columns.order_number",
      enableSorting: true,
      gridPrimary: true,
      cell: ({ row }) => {
        const item = row.original;
        const isBulk =
          item.order_type === "bulk" ||
          item.order_type === "corporate" ||
          item.items_count > 1;
        return (
          <div className="flex items-center gap-2.5">
            <div className="bg-muted/50 text-foreground flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border">
              <ShoppingBag className="h-4 w-4" />
            </div>
            <div className="flex flex-col">
              <div className="flex items-center gap-1.5">
                <span
                  className="text-foreground cursor-pointer font-mono font-bold hover:underline"
                  onClick={() => onView(item)}
                >
                  {item.order_number}
                </span>
                {isBulk && (
                  <Badge
                    variant="secondary"
                    className="bg-primary/10 text-primary border-primary/20 px-1.5 py-0 text-[10px] font-semibold"
                  >
                    Bulk ({item.items_count})
                  </Badge>
                )}
              </div>
              <span className="text-muted-foreground font-mono text-[11px]">
                {new Date(item.created_at).toLocaleString()}
              </span>
            </div>
          </div>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "user_name",
      labelKey: "orders.columns.customer",
      enableSorting: true,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex flex-col">
            <span className="text-foreground font-medium">
              {item.company_name || item.user_name || item.user_email}
            </span>
            {item.company_name ? (
              <span className="text-muted-foreground text-xs">
                {item.user_name || item.user_email}
              </span>
            ) : (
              <span className="text-muted-foreground font-mono text-xs">
                {item.user_email}
              </span>
            )}
          </div>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "grand_total",
      labelKey: "orders.columns.total",
      enableSorting: true,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <div className="flex flex-col">
            <span className="text-foreground font-mono font-bold">
              {item.grand_total.toLocaleString(undefined, {
                minimumFractionDigits: 2,
              })}{" "}
              {item.currency}
            </span>
            <span className="text-muted-foreground text-[11px]">
              {item.items_count} {t("orders.detail_items_count")}
            </span>
          </div>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "status",
      labelKey: "orders.columns.status",
      enableSorting: true,
      cell: ({ row }) => {
        const status = row.original.status;
        const map: Record<
          string,
          {
            labelKey: string;
            variant: "default" | "secondary" | "danger" | "outline";
          }
        > = {
          pending: { labelKey: "orders.status_pending", variant: "outline" },
          paid: { labelKey: "orders.status_paid", variant: "secondary" },
          design_pending: {
            labelKey: "orders.status_design_pending",
            variant: "secondary",
          },
          design_approved: {
            labelKey: "orders.status_design_approved",
            variant: "default",
          },
          in_production: {
            labelKey: "orders.status_in_production",
            variant: "default",
          },
          ready_to_ship: {
            labelKey: "orders.status_ready_to_ship",
            variant: "default",
          },
          shipped: { labelKey: "orders.status_shipped", variant: "default" },
          delivered: {
            labelKey: "orders.status_delivered",
            variant: "default",
          },
          cancelled: { labelKey: "orders.status_cancelled", variant: "danger" },
        };
        const conf = map[status] || { labelKey: status, variant: "outline" };
        return (
          <Badge variant={conf.variant} className="text-xs">
            {conf.labelKey.startsWith("orders.")
              ? t(conf.labelKey)
              : conf.labelKey}
          </Badge>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "payment_status",
      labelKey: "orders.columns.payment",
      enableSorting: true,
      cell: ({ row }) => {
        const paid = row.original.payment_status === "paid";
        return (
          <Badge
            variant={paid ? "default" : "outline"}
            className={
              paid ? "bg-emerald-600 text-white hover:bg-emerald-700" : ""
            }
          >
            {paid
              ? t("orders.payment_paid")
              : row.original.payment_status || t("orders.payment_unpaid")}
          </Badge>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "fulfillment_status",
      labelKey: "orders.columns.fulfillment",
      enableSorting: true,
      cell: ({ row }) => {
        const fStatus = row.original.fulfillment_status || "unfulfilled";
        const fMap: Record<
          string,
          {
            labelKey: string;
            variant: "default" | "secondary" | "danger" | "outline";
          }
        > = {
          unfulfilled: {
            labelKey: "orders.detail_fulfillment_unfulfilled",
            variant: "outline",
          },
          pending: {
            labelKey: "orders.detail_fulfillment_pending",
            variant: "outline",
          },
          processing: {
            labelKey: "orders.detail_fulfillment_processing",
            variant: "secondary",
          },
          in_production: {
            labelKey: "orders.detail_fulfillment_in_production",
            variant: "secondary",
          },
          ready_to_ship: {
            labelKey: "orders.detail_fulfillment_ready_to_ship",
            variant: "default",
          },
          shipped: {
            labelKey: "orders.detail_fulfillment_shipped",
            variant: "default",
          },
          delivered: {
            labelKey: "orders.detail_fulfillment_delivered",
            variant: "default",
          },
          cancelled: {
            labelKey: "orders.detail_fulfillment_cancelled",
            variant: "danger",
          },
        };
        const fConf = fMap[fStatus] || {
          labelKey: fStatus,
          variant: "outline",
        };
        return (
          <Badge variant={fConf.variant} className="text-xs">
            {fConf.labelKey.startsWith("orders.")
              ? t(fConf.labelKey)
              : fConf.labelKey}
          </Badge>
        );
      },
    }),
    createColumn<OrderSummary>({
      accessorKey: "shipping_carrier",
      labelKey: "orders.columns.shipping",
      enableSorting: false,
      cell: ({ row }) => {
        const item = row.original;
        if (!item.tracking_number) {
          return <span className="text-muted-foreground text-xs">—</span>;
        }
        return (
          <div className="flex flex-col font-mono text-xs">
            <span className="font-semibold">
              {item.shipping_carrier || t("orders.columns.shipping")}
            </span>
            <span className="text-muted-foreground">
              {item.tracking_number}
            </span>
          </div>
        );
      },
    }),
    createColumn<OrderSummary>({
      id: "actions",
      labelKey: "common.actions",
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const item = row.original;
        return (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onView(item)}
            className="h-8 text-xs"
          >
            <Eye className="mr-1 h-3.5 w-3.5" />
            {t("orders.inspect")}
          </Button>
        );
      },
    }),
  ];
}
