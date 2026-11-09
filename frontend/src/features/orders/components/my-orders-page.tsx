"use client";

import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ShoppingBag,
  Plus,
  Package,
  Truck,
  Clock,
  CheckCircle2,
  AlertCircle,
  ExternalLink,
  Eye,
  FileUp,
  CreditCard,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { useLocale } from "@/providers/locale-provider";
import { isOrderPayable, ordersService } from "../services/orders.service";
import {
  useCanPayOrders,
  useStartOrderPayment,
} from "../hooks/use-order-payment";
import { ordersQueryKeys } from "../hooks/query-keys";
import { SubscriberOrderDialog } from "./subscriber-order-dialog";
import { SubscriberOrderDetailDialog } from "./subscriber-order-detail-dialog";
import { BulkOrderDialog } from "./bulk-order-dialog";
import { Users } from "lucide-react";
import { cn } from "@/lib/utils";

export function MyOrdersPage() {
  const { t, locale } = useLocale();
  const [orderDialogOpen, setOrderDialogOpen] = useState(false);
  const [bulkDialogOpen, setBulkDialogOpen] = useState(false);
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string | null>(
    null,
  );

  const { data, isLoading, refetch } = useQuery({
    queryKey: ordersQueryKeys.myOrders(),
    queryFn: () => ordersService.listMyOrders(),
  });

  const canPay = useCanPayOrders();
  const startPayment = useStartOrderPayment();

  const orders = data?.items || [];
  const total = data?.total || 0;

  const getStatusConfig = (
    status: string,
  ): {
    label: string;
    variant:
      "default" | "secondary" | "outline" | "success" | "warning" | "danger";
    icon: LucideIcon;
  } => {
    switch (status) {
      case "paid":
        return {
          label: t("orders.status_paid"),
          variant: "success",
          icon: CheckCircle2,
        };
      case "design_pending":
        return {
          label: t("orders.status_design_pending"),
          variant: "warning",
          icon: AlertCircle,
        };
      case "design_approved":
        return {
          label: t("orders.status_design_approved"),
          variant: "success",
          icon: CheckCircle2,
        };
      case "in_production":
        return {
          label: t("orders.status_in_production"),
          variant: "default",
          icon: Package,
        };
      case "ready_to_ship":
        return {
          label: t("orders.status_ready_to_ship"),
          variant: "secondary",
          icon: Package,
        };
      case "shipped":
        return {
          label: t("orders.status_shipped"),
          variant: "default",
          icon: Truck,
        };
      case "delivered":
        return {
          label: t("orders.status_delivered"),
          variant: "success",
          icon: CheckCircle2,
        };
      case "cancelled":
        return {
          label: t("orders.status_cancelled"),
          variant: "danger",
          icon: AlertCircle,
        };
      case "refunded":
        return {
          label: t("orders.status_refunded"),
          variant: "danger",
          icon: AlertCircle,
        };
      case "pending":
      default:
        return {
          label: t("orders.status_pending"),
          variant: "secondary",
          icon: Clock,
        };
    }
  };

  return (
    <div className="space-y-6 pb-12">
      {/* Header */}
      <div className="flex flex-col justify-between gap-4 border-b pb-2 sm:flex-row sm:items-center">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-foreground flex items-center gap-2 text-2xl font-bold tracking-tight">
              <ShoppingBag className="text-primary h-6 w-6" />
              {t("orders.subscriber.page_title")}
            </h1>
            <Badge variant="secondary" className="font-mono text-xs">
              {total} {t("orders.subscriber.orders_count_badge")}
            </Badge>
          </div>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("orders.subscriber.page_description")}
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <Button
            variant="outline"
            className="gap-2 shadow-sm"
            onClick={() => setBulkDialogOpen(true)}
          >
            <Users className="text-primary h-4 w-4" />
            {t("orders.subscriber.bulk_order_btn")}
          </Button>
          <Button
            variant="default"
            className="gap-2 shadow-sm"
            onClick={() => setOrderDialogOpen(true)}
          >
            <Plus className="h-4 w-4" />
            {t("orders.subscriber.order_new_card_btn")}
          </Button>
        </div>
      </div>

      {/* Main List */}
      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="bg-muted/40 h-32 animate-pulse rounded-2xl border"
            />
          ))}
        </div>
      ) : orders.length === 0 ? (
        /* Empty State */
        <div className="bg-card/40 flex flex-col items-center justify-center space-y-4 rounded-2xl border p-8 py-16 text-center">
          <div className="bg-primary/10 text-primary flex h-16 w-16 items-center justify-center rounded-3xl shadow-inner">
            <ShoppingBag className="h-8 w-8" />
          </div>
          <div className="max-w-md space-y-1">
            <h3 className="text-foreground text-lg font-bold">
              {t("orders.subscriber.empty_title")}
            </h3>
            <p className="text-muted-foreground text-xs">
              {t("orders.subscriber.empty_desc")}
            </p>
          </div>
          <Button
            variant="default"
            className="mt-2 gap-2"
            onClick={() => setOrderDialogOpen(true)}
          >
            <Plus className="h-4 w-4" />
            {t("orders.subscriber.order_new_card_btn")}
          </Button>
        </div>
      ) : (
        /* Orders List */
        <div className="space-y-4">
          {orders.map((order) => {
            const statusConfig = getStatusConfig(order.status);
            const StatusIcon = statusConfig.icon;
            const needsAsset = order.status === "design_pending";
            const payable = isOrderPayable(order);

            return (
              <Card
                key={order.uuid}
                className={cn(
                  "cursor-pointer overflow-hidden border transition-all hover:shadow-md",
                  needsAsset
                    ? "border-amber-500/40 bg-amber-500/5 hover:border-amber-500/70"
                    : "bg-card",
                )}
                onClick={() => setSelectedOrderUuid(order.uuid)}
              >
                <CardContent className="flex flex-col justify-between gap-4 p-5 md:flex-row md:items-center">
                  {/* Left info */}
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2.5">
                      <span className="text-foreground font-mono text-base font-bold tracking-wide">
                        #{order.order_number}
                      </span>
                      <Badge
                        variant={statusConfig.variant}
                        className="gap-1 text-xs"
                      >
                        <StatusIcon className="h-3 w-3" />
                        {statusConfig.label}
                      </Badge>
                      {payable && (
                        <Badge variant="warning" className="gap-1 text-[11px]">
                          <CreditCard className="h-3 w-3" />
                          {t("orders.checkout.awaiting_payment")}
                        </Badge>
                      )}
                      {needsAsset && (
                        <Badge
                          variant="warning"
                          className="animate-pulse gap-1 text-[11px]"
                        >
                          <FileUp className="h-3 w-3" />
                          {t("orders.subscriber.action_required_badge")}
                        </Badge>
                      )}
                    </div>

                    <div className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
                      <span>
                        {t("orders.subscriber.order_date")}:{" "}
                        {new Date(order.created_at).toLocaleDateString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { dateStyle: "medium" },
                        )}
                      </span>
                      <span>
                        {order.items_count} {t("orders.detail_items_count")}
                      </span>
                      {order.tracking_number && (
                        <span className="text-foreground flex items-center gap-1 font-medium">
                          <Truck className="text-primary h-3.5 w-3.5" />
                          {t("orders.fields.tracking_number")}:{" "}
                          {order.tracking_number}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Right summary and actions */}
                  <div className="flex flex-wrap items-center justify-between gap-4 border-t pt-3 md:justify-end md:border-t-0 md:pt-0">
                    <div className="text-left md:text-right">
                      <p className="text-muted-foreground text-xs">
                        {t("orders.subscriber.total_amount")}
                      </p>
                      <p className="text-primary text-base font-bold">
                        {order.grand_total.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 2 },
                        )}{" "}
                        {order.currency}
                      </p>
                    </div>

                    <div
                      className="flex items-center gap-2"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {order.tracking_url && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-8 gap-1 text-xs"
                          onClick={() =>
                            window.open(order.tracking_url, "_blank")
                          }
                        >
                          <Truck className="text-primary h-3.5 w-3.5" />
                          <span>{t("orders.subscriber.tracking_label")}</span>
                          <ExternalLink className="h-3 w-3" />
                        </Button>
                      )}

                      {payable && canPay && (
                        <Button
                          size="sm"
                          className="h-8 gap-1.5 text-xs shadow-xs"
                          disabled={startPayment.isPending}
                          onClick={() => startPayment.mutate(order.uuid)}
                        >
                          <CreditCard className="h-3.5 w-3.5" />
                          <span>{t("orders.checkout.pay_now")}</span>
                        </Button>
                      )}

                      <Button
                        size="sm"
                        variant={needsAsset ? "default" : "secondary"}
                        className="h-8 gap-1.5 text-xs shadow-xs"
                        onClick={() => setSelectedOrderUuid(order.uuid)}
                      >
                        {needsAsset ? (
                          <>
                            <FileUp className="h-3.5 w-3.5" />
                            <span>
                              {t("orders.subscriber.upload_file_btn")}
                            </span>
                          </>
                        ) : (
                          <>
                            <Eye className="h-3.5 w-3.5" />
                            <span>{t("orders.subscriber.view_order_btn")}</span>
                          </>
                        )}
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      {/* Place Order Dialog */}
      <SubscriberOrderDialog
        open={orderDialogOpen}
        onOpenChange={setOrderDialogOpen}
        onSuccess={() => refetch()}
      />

      {/* Bulk Place Order Dialog */}
      <BulkOrderDialog
        open={bulkDialogOpen}
        onOpenChange={setBulkDialogOpen}
        isPlatformAdmin={false}
      />

      {/* View Details & Asset Upload Dialog */}
      <SubscriberOrderDetailDialog
        orderUuid={selectedOrderUuid}
        open={Boolean(selectedOrderUuid)}
        onOpenChange={(open) => {
          if (!open) setSelectedOrderUuid(null);
        }}
      />
    </div>
  );
}
