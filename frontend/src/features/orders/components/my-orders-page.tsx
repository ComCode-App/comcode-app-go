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
import { useCanPayOrders, useStartOrderPayment } from "../hooks/use-order-payment";
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
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string | null>(null);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ordersQueryKeys.myOrders(),
    queryFn: () => ordersService.listMyOrders(),
  });

  const canPay = useCanPayOrders();
  const startPayment = useStartOrderPayment();

  const orders = data?.items || [];
  const total = data?.total || 0;

  const getStatusConfig = (status: string): { label: string; variant: "default" | "secondary" | "outline" | "success" | "warning" | "danger"; icon: LucideIcon } => {
    switch (status) {
      case "paid":
        return { label: t("orders.status_paid"), variant: "success", icon: CheckCircle2 };
      case "design_pending":
        return { label: t("orders.status_design_pending"), variant: "warning", icon: AlertCircle };
      case "design_approved":
        return { label: t("orders.status_design_approved"), variant: "success", icon: CheckCircle2 };
      case "in_production":
        return { label: t("orders.status_in_production"), variant: "default", icon: Package };
      case "ready_to_ship":
        return { label: t("orders.status_ready_to_ship"), variant: "secondary", icon: Package };
      case "shipped":
        return { label: t("orders.status_shipped"), variant: "default", icon: Truck };
      case "delivered":
        return { label: t("orders.status_delivered"), variant: "success", icon: CheckCircle2 };
      case "cancelled":
        return { label: t("orders.status_cancelled"), variant: "danger", icon: AlertCircle };
      case "refunded":
        return { label: t("orders.status_refunded"), variant: "danger", icon: AlertCircle };
      case "pending":
      default:
        return { label: t("orders.status_pending"), variant: "secondary", icon: Clock };
    }
  };

  return (
    <div className="space-y-6 pb-12">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
              <ShoppingBag className="w-6 h-6 text-primary" />
              {t("orders.subscriber.page_title")}
            </h1>
            <Badge variant="secondary" className="font-mono text-xs">
              {total} {t("orders.subscriber.orders_count_badge")}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            {t("orders.subscriber.page_description")}
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <Button
            variant="outline"
            className="gap-2 shadow-sm"
            onClick={() => setBulkDialogOpen(true)}
          >
            <Users className="w-4 h-4 text-primary" />
            {t("orders.subscriber.bulk_order_btn")}
          </Button>
          <Button
            variant="default"
            className="gap-2 shadow-sm"
            onClick={() => setOrderDialogOpen(true)}
          >
            <Plus className="w-4 h-4" />
            {t("orders.subscriber.order_new_card_btn")}
          </Button>
        </div>
      </div>

      {/* Main List */}
      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-32 rounded-2xl bg-muted/40 animate-pulse border" />
          ))}
        </div>
      ) : orders.length === 0 ? (
        /* Empty State */
        <div className="flex flex-col items-center justify-center py-16 text-center border rounded-2xl bg-card/40 p-8 space-y-4">
          <div className="w-16 h-16 rounded-3xl bg-primary/10 text-primary flex items-center justify-center shadow-inner">
            <ShoppingBag className="w-8 h-8" />
          </div>
          <div className="space-y-1 max-w-md">
            <h3 className="text-lg font-bold text-foreground">
              {t("orders.subscriber.empty_title")}
            </h3>
            <p className="text-xs text-muted-foreground">
              {t("orders.subscriber.empty_desc")}
            </p>
          </div>
          <Button
            variant="default"
            className="gap-2 mt-2"
            onClick={() => setOrderDialogOpen(true)}
          >
            <Plus className="w-4 h-4" />
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
                  "overflow-hidden border transition-all hover:shadow-md cursor-pointer",
                  needsAsset ? "border-amber-500/40 bg-amber-500/5 hover:border-amber-500/70" : "bg-card"
                )}
                onClick={() => setSelectedOrderUuid(order.uuid)}
              >
                <CardContent className="p-5 flex flex-col md:flex-row md:items-center justify-between gap-4">
                  {/* Left info */}
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2.5">
                      <span className="font-mono font-bold text-base text-foreground tracking-wide">
                        #{order.order_number}
                      </span>
                      <Badge variant={statusConfig.variant} className="gap-1 text-xs">
                        <StatusIcon className="w-3 h-3" />
                        {statusConfig.label}
                      </Badge>
                      {payable && (
                        <Badge variant="warning" className="gap-1 text-[11px]">
                          <CreditCard className="w-3 h-3" />
                          {t("orders.checkout.awaiting_payment")}
                        </Badge>
                      )}
                      {needsAsset && (
                        <Badge variant="warning" className="gap-1 text-[11px] animate-pulse">
                          <FileUp className="w-3 h-3" />
                          {t("orders.subscriber.action_required_badge")}
                        </Badge>
                      )}
                    </div>

                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <span>
                        {t("orders.subscriber.order_date")}: {new Date(order.created_at).toLocaleDateString(locale === "tr" ? "tr-TR" : "en-US", { dateStyle: "medium" })}
                      </span>
                      <span>
                        {order.items_count} {t("orders.detail_items_count")}
                      </span>
                      {order.tracking_number && (
                        <span className="flex items-center gap-1 font-medium text-foreground">
                          <Truck className="w-3.5 h-3.5 text-primary" />
                          {t("orders.fields.tracking_number")}: {order.tracking_number}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Right summary and actions */}
                  <div className="flex flex-wrap items-center justify-between md:justify-end gap-4 pt-3 md:pt-0 border-t md:border-t-0">
                    <div className="text-left md:text-right">
                      <p className="text-xs text-muted-foreground">{t("orders.subscriber.total_amount")}</p>
                      <p className="font-bold text-base text-primary">
                        {order.grand_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}
                      </p>
                    </div>

                    <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                      {order.tracking_url && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="gap-1 text-xs h-8"
                          onClick={() => window.open(order.tracking_url, "_blank")}
                        >
                          <Truck className="w-3.5 h-3.5 text-primary" />
                          <span>{t("orders.subscriber.tracking_label")}</span>
                          <ExternalLink className="w-3 h-3" />
                        </Button>
                      )}

                      {payable && canPay && (
                        <Button
                          size="sm"
                          className="gap-1.5 text-xs h-8 shadow-xs"
                          disabled={startPayment.isPending}
                          onClick={() => startPayment.mutate(order.uuid)}
                        >
                          <CreditCard className="w-3.5 h-3.5" />
                          <span>{t("orders.checkout.pay_now")}</span>
                        </Button>
                      )}

                      <Button
                        size="sm"
                        variant={needsAsset ? "default" : "secondary"}
                        className="gap-1.5 text-xs h-8 shadow-xs"
                        onClick={() => setSelectedOrderUuid(order.uuid)}
                      >
                        {needsAsset ? (
                          <>
                            <FileUp className="w-3.5 h-3.5" />
                            <span>{t("orders.subscriber.upload_file_btn")}</span>
                          </>
                        ) : (
                          <>
                            <Eye className="w-3.5 h-3.5" />
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
