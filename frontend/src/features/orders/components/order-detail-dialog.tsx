"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  ShoppingBag,
  Truck,
  FileCheck,
  Check,
  X,
  ExternalLink,
  Plus,
  Building,
  User,
  CreditCard,
  Sparkles,
} from "lucide-react";
import { ordersService, type OrderStatus } from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";
import { useOrderMutations } from "../hooks/use-orders";
import { OrderManifestMenu } from "./order-manifest-menu";
import { OrderRequestAssetDialog } from "./order-request-asset-dialog";
import { OrderRefundsPanel } from "./order-refunds-panel";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

interface OrderDetailDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orderUuid: string | null;
}

export function OrderDetailDialog({
  open,
  onOpenChange,
  orderUuid,
}: OrderDetailDialogProps) {
  const { t, locale } = useLocale();
  const { confirm } = useDialogs();
  const {
    updateStatusMutation,
    updateShippingMutation,
    updateNotesMutation,
    reviewAssetRequestMutation,
    provisionOrderMutation,
    provisionItemMutation,
    updatePaymentStatusMutation,
    updateFulfillmentStatusMutation,
  } = useOrderMutations();

  const [assetModalOpen, setAssetModalOpen] = useState(false);
  const [carrier, setCarrier] = useState("");
  const [trackingNumber, setTrackingNumber] = useState("");
  const [trackingUrl, setTrackingUrl] = useState("");
  const [adminNote, setAdminNote] = useState("");
  const [rejectionReason, setRejectionReason] = useState("");
  const [selectedAssetUuid, setSelectedAssetUuid] = useState<string | null>(null);

  const { data: order, isLoading } = useQuery({
    queryKey: ordersQueryKeys.detail(orderUuid || ""),
    queryFn: () => ordersService.get(orderUuid!),
    enabled: Boolean(orderUuid) && open,
  });

  // Online payments with something left to refund (TEC-935): "refunded" refunds them at the provider.
  const { data: refunds } = useQuery({
    queryKey: ordersQueryKeys.refunds(orderUuid || ""),
    queryFn: () => ordersService.listRefunds(orderUuid!),
    enabled: Boolean(orderUuid) && open,
  });
  const refundsAtProvider = Boolean(
    refunds?.payments.some((payment) => payment.paid_order && payment.refundable_minor > 0),
  );

  if (!open || !orderUuid) return null;

  // Cancelling or refunding retires the order's cards for good (TEC-585), so ask first.
  const confirmRetire = () =>
    confirm({
      title: t("orders.retire_confirm_title"),
      description: t("orders.retire_confirm_desc"),
      confirmLabel: t("orders.retire_confirm_action"),
      variant: "destructive",
    });

  const handleStatusChange = async (newStatus: OrderStatus) => {
    if (newStatus === "cancelled" && !(await confirmRetire())) return;
    await updateStatusMutation.mutateAsync({
      uuid: orderUuid,
      status: newStatus,
    });
  };

  const handleSaveShipping = async (markShipped = false) => {
    await updateShippingMutation.mutateAsync({
      uuid: orderUuid,
      payload: {
        shipping_carrier: carrier || order?.shipping_carrier || "",
        tracking_number: trackingNumber || order?.tracking_number || "",
        tracking_url: trackingUrl || order?.tracking_url || "",
        mark_shipped: markShipped,
      },
    });
  };

  const handleSaveAdminNote = async () => {
    await updateNotesMutation.mutateAsync({
      uuid: orderUuid,
      adminNote: adminNote || order?.admin_note || "",
    });
  };

  const handleReviewAsset = async (assetUuid: string, status: "approved" | "rejected") => {
    await reviewAssetRequestMutation.mutateAsync({
      orderUuid,
      assetUuid,
      payload: {
        status,
        rejection_reason: status === "rejected" ? rejectionReason : "",
      },
    });
    setSelectedAssetUuid(null);
    setRejectionReason("");
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          {isLoading || !order ? (
            <div className="py-12 flex flex-col items-center justify-center gap-2">
              <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
              <p className="text-sm text-muted-foreground">{t("orders.detail_loading")}</p>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Dialog Header with Status & Order Number */}
              <DialogHeader className="border-b pb-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <DialogTitle className="flex items-center gap-2 text-xl font-bold font-mono">
                        <ShoppingBag className="h-5 w-5 text-primary" />
                        {order.order_number}
                      </DialogTitle>
                      {order.order_type && (
                        <Badge variant="secondary" className="font-mono text-xs uppercase">
                          {order.order_type}
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t("orders.detail_created_at")}: {new Date(order.created_at).toLocaleString(locale === "tr" ? "tr-TR" : "en-US")}
                    </p>
                  </div>

                  {/* 3 Separate State Machine Controls */}
                  <div className="flex flex-wrap items-center gap-2">
                    {/* 1. Overall Order Status */}
                    <div className="flex items-center gap-1.5">
                      <span className="text-[11px] text-muted-foreground font-medium">{t("orders.detail_label_order_status")}</span>
                      <Select
                        value={order.status}
                        onValueChange={(val) => handleStatusChange(val as OrderStatus)}
                        disabled={updateStatusMutation.isPending}
                      >
                        <SelectTrigger className="h-8 w-36 text-xs font-semibold">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="pending">⏳ {t("orders.status_pending")}</SelectItem>
                          <SelectItem value="paid">💳 {t("orders.status_paid")}</SelectItem>
                          <SelectItem value="design_pending">🎨 {t("orders.status_design_pending")}</SelectItem>
                          <SelectItem value="design_approved">✅ {t("orders.status_design_approved")}</SelectItem>
                          <SelectItem value="in_production">⚙️ {t("orders.status_in_production")}</SelectItem>
                          <SelectItem value="ready_to_ship">📦 {t("orders.status_ready_to_ship")}</SelectItem>
                          <SelectItem value="shipped">🚚 {t("orders.status_shipped")}</SelectItem>
                          <SelectItem value="delivered">🎉 {t("orders.status_delivered")}</SelectItem>
                          <SelectItem value="cancelled">❌ {t("orders.status_cancelled")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    {/* 2. Payment State Machine */}
                    <div className="flex items-center gap-1.5">
                      <span className="text-[11px] text-muted-foreground font-medium">{t("orders.detail_label_payment_status")}</span>
                      <Select
                        value={order.payment_status || "unpaid"}
                        onValueChange={async (val) => {
                          if (val === "refunded") {
                            const ok = refundsAtProvider
                              ? await confirm({
                                  title: t("orders.refund_status_confirm_title"),
                                  description: t("orders.refund_status_confirm_desc"),
                                  confirmLabel: t("orders.refund.action"),
                                  variant: "destructive",
                                })
                              : await confirmRetire();
                            if (!ok) return;
                          }
                          updatePaymentStatusMutation.mutate({ uuid: order.uuid, paymentStatus: val });
                        }}
                        disabled={updatePaymentStatusMutation.isPending}
                      >
                        <SelectTrigger className="h-8 w-32 text-xs font-semibold">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="unpaid">⏳ {t("orders.detail_payment_unpaid")}</SelectItem>
                          <SelectItem value="paid">💳 {t("orders.detail_payment_paid")}</SelectItem>
                          <SelectItem value="partially_refunded" disabled>↪️ {t("orders.detail_payment_partially_refunded")}</SelectItem>
                          <SelectItem value="refunded">↩️ {t("orders.detail_payment_refunded")}</SelectItem>
                          <SelectItem value="failed">❌ {t("orders.detail_payment_failed")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    {/* 3. Fulfillment State Machine */}
                    <div className="flex items-center gap-1.5">
                      <span className="text-[11px] text-muted-foreground font-medium">{t("orders.detail_label_fulfillment_status")}</span>
                      <Select
                        value={order.fulfillment_status || "unfulfilled"}
                        onValueChange={(val) =>
                          updateFulfillmentStatusMutation.mutate({ uuid: order.uuid, fulfillmentStatus: val })
                        }
                        disabled={updateFulfillmentStatusMutation.isPending}
                      >
                        <SelectTrigger className="h-8 w-32 text-xs font-semibold">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="unfulfilled">{t("orders.detail_fulfillment_unfulfilled")}</SelectItem>
                          <SelectItem value="in_production">{t("orders.detail_fulfillment_in_production")}</SelectItem>
                          <SelectItem value="ready_to_ship">{t("orders.detail_fulfillment_ready_to_ship")}</SelectItem>
                          <SelectItem value="shipped">{t("orders.detail_fulfillment_shipped")}</SelectItem>
                          <SelectItem value="delivered">{t("orders.detail_fulfillment_delivered")}</SelectItem>
                          <SelectItem value="cancelled">{t("orders.detail_fulfillment_cancelled")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              </DialogHeader>

              {/* Order Tabs */}
              <Tabs defaultValue="items" className="w-full">
                <TabsList className="grid grid-cols-4 bg-muted mb-4">
                  <TabsTrigger value="items" className="text-xs">
                    {t("orders.detail_tab_recipients")} ({order.items.length})
                  </TabsTrigger>
                  <TabsTrigger value="assets" className="text-xs flex items-center gap-1">
                    <FileCheck className="h-3.5 w-3.5" />
                    {t("orders.detail_design_requests")} ({order.asset_requests.length})
                  </TabsTrigger>
                  <TabsTrigger value="shipping" className="text-xs flex items-center gap-1">
                    <Truck className="h-3.5 w-3.5" /> {t("orders.columns.shipping")}
                  </TabsTrigger>
                  <TabsTrigger value="customer" className="text-xs flex items-center gap-1">
                    <User className="h-3.5 w-3.5" /> {t("orders.detail_customer_finance")}
                  </TabsTrigger>
                </TabsList>

                {/* TAB 1: RECIPIENTS, NFC CARDS & TEC-77 PROVISIONING */}
                <TabsContent value="items" className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <h4 className="text-sm font-semibold">{t("orders.detail_provision_heading")}</h4>
                      <p className="text-xs text-muted-foreground">
                        {t("orders.detail_provision_subtext")}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <OrderManifestMenu
                        orderUuid={order.uuid}
                        disabled={!order.items.some((item) => item.provisioning_status === "provisioned")}
                      />
                      <Button
                        size="sm"
                        onClick={() => provisionOrderMutation.mutate(order.uuid)}
                        disabled={provisionOrderMutation.isPending}
                        className="bg-primary hover:bg-primary/90 text-primary-foreground text-xs"
                      >
                        <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                        {provisionOrderMutation.isPending ? t("orders.detail_provisioning_in_progress") : t("orders.detail_provision_all_btn")}
                      </Button>
                    </div>
                  </div>

                  <div className="rounded-xl border overflow-x-auto">
                    <table className="w-full text-xs text-left">
                      <thead className="bg-muted/60 text-muted-foreground uppercase font-mono text-[10px]">
                        <tr>
                          <th className="p-3">{t("orders.detail_col_recipient")}</th>
                          <th className="p-3">{t("orders.detail_col_product_print")}</th>
                          <th className="p-3">{t("orders.detail_col_provision_status")}</th>
                          <th className="p-3">{t("orders.detail_col_nfc_card")}</th>
                          <th className="p-3">{t("orders.detail_col_tec77_route")}</th>
                          <th className="p-3">{t("orders.detail_col_fulfillment")}</th>
                          <th className="p-3 text-right">{t("orders.detail_col_price")}</th>
                          <th className="p-3 text-center">{t("orders.detail_col_action")}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {order.items.map((item) => {
                          const isProvisioned = item.provisioning_status === "provisioned";
                          return (
                            <tr key={item.uuid} className="hover:bg-muted/30">
                              {/* Recipient details */}
                              <td className="p-3">
                                <div className="font-semibold text-foreground">
                                  {item.recipient_name || order.user_name || "—"}
                                </div>
                                {item.recipient_title && (
                                  <div className="text-[11px] text-muted-foreground">
                                    {item.recipient_title} {item.recipient_department ? `• ${item.recipient_department}` : ""}
                                  </div>
                                )}
                                {item.recipient_email && (
                                  <div className="text-[10px] text-muted-foreground font-mono">
                                    {item.recipient_email}
                                  </div>
                                )}
                              </td>

                              {/* Product & Variant */}
                              <td className="p-3">
                                <div className="font-medium text-foreground">{item.product_name}</div>
                                {item.variant_name && (
                                  <span className="text-[11px] text-muted-foreground">{item.variant_name}</span>
                                )}
                                {item.custom_print_data && Object.keys(item.custom_print_data).length > 0 && (
                                  <div className="mt-1 p-1 rounded bg-muted/40 text-[10px] font-mono border">
                                    {Object.entries(item.custom_print_data).map(([k, v]) => (
                                      <span key={k} className="mr-2 text-muted-foreground">
                                        <span className="font-semibold">{k}:</span> {String(v)}
                                      </span>
                                    ))}
                                  </div>
                                )}
                              </td>

                              {/* Provisioning status */}
                              <td className="p-3">
                                <Badge
                                  variant={isProvisioned ? "default" : "outline"}
                                  className={`text-[10px] font-mono ${
                                    isProvisioned ? "bg-emerald-600 hover:bg-emerald-700 text-white" : ""
                                  }`}
                                >
                                  {isProvisioned ? "✅ Provisioned" : (item.provisioning_status || "⏳ Unprovisioned")}
                                </Badge>
                                {item.provisioning_error && (
                                  <p className="text-[10px] text-destructive mt-0.5">{item.provisioning_error}</p>
                                )}
                              </td>

                              {/* Card info */}
                              <td className="p-3 font-mono">
                                {item.card_smart_code ? (
                                  <div className="space-y-0.5">
                                    <Badge variant="secondary" className="font-bold text-[10px]">
                                      {item.card_smart_code}
                                    </Badge>
                                    {item.card_uid && (
                                      <div className="text-[10px] text-muted-foreground">
                                        UID: {item.card_uid}
                                      </div>
                                    )}
                                  </div>
                                ) : (
                                  <span className="text-muted-foreground text-[11px]">—</span>
                                )}
                              </td>

                              {/* TEC-77 Route */}
                              <td className="p-3 font-mono">
                                {item.route_path ? (
                                  <a
                                    href={item.route_path}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="text-primary hover:underline text-[11px] inline-flex items-center gap-1 font-semibold"
                                  >
                                    {item.route_path} <ExternalLink className="h-3 w-3" />
                                  </a>
                                ) : (
                                  <span className="text-muted-foreground text-[11px]">—</span>
                                )}
                              </td>

                              {/* Item fulfillment */}
                              <td className="p-3">
                                <Badge variant="outline" className="text-[10px] capitalize">
                                  {item.fulfillment_status || "pending"}
                                </Badge>
                              </td>

                              {/* Price */}
                              <td className="p-3 text-right font-mono font-bold text-foreground">
                                {item.total_price.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}
                              </td>

                              {/* Single item provision action */}
                              <td className="p-3 text-center">
                                {!isProvisioned ? (
                                  <Button
                                    size="sm"
                                    variant="outline"
                                    className="h-7 text-[11px]"
                                    onClick={() =>
                                      provisionItemMutation.mutate({
                                        orderUuid: order.uuid,
                                        itemUuid: item.uuid,
                                      })
                                    }
                                    disabled={provisionItemMutation.isPending}
                                  >
                                    {t("orders.detail_provision_item_btn")}
                                  </Button>
                                ) : (
                                  <span className="text-emerald-600 font-bold text-xs">✓ {t("orders.detail_provision_ready")}</span>
                                )}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>

                  {/* Financial Summary Box */}
                  <div className="flex justify-end pt-2">
                    <div className="w-64 space-y-1.5 text-xs">
                      <div className="flex justify-between text-muted-foreground">
                        <span>{t("orders.detail_total")}:</span>
                        <span className="font-mono">{order.subtotal.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}</span>
                      </div>
                      <div className="flex justify-between text-muted-foreground">
                        <span>{t("orders.subscriber.vat_label")}:</span>
                        <span className="font-mono">{order.tax_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}</span>
                      </div>
                      <div className="flex justify-between text-muted-foreground">
                        <span>{t("orders.columns.shipping")}:</span>
                        <span className="font-mono">{order.shipping_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}</span>
                      </div>
                      <Separator className="my-1" />
                      <div className="flex justify-between text-sm font-bold text-foreground">
                        <span>{t("orders.fields.grand_total")}:</span>
                        <span className="font-mono text-primary font-bold">{order.grand_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}</span>
                      </div>
                    </div>
                  </div>
                </TabsContent>

                {/* TAB 2: ASSET REQUESTS & CUSTOMER LOGO ENGINE */}
                <TabsContent value="assets" className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <h4 className="text-sm font-semibold">{t("orders.detail_design_requests")}</h4>
                      <p className="text-xs text-muted-foreground">{t("orders.detail_design_requests_sub")}</p>
                    </div>
                    <Button
                      size="sm"
                      onClick={() => setAssetModalOpen(true)}
                    >
                      <Plus className="mr-1.5 h-3.5 w-3.5" /> {t("orders.detail_request_file_btn")}
                    </Button>
                  </div>

                  <div className="space-y-3">
                    {order.asset_requests.length === 0 ? (
                      <div className="p-8 text-center border border-dashed rounded-xl text-xs text-muted-foreground space-y-2">
                        <FileCheck className="h-8 w-8 mx-auto text-muted-foreground/50" />
                        <p>{t("orders.detail_no_requests")}</p>
                      </div>
                    ) : (
                      order.asset_requests.map((asset) => (
                        <div
                          key={asset.uuid}
                          className="p-4 rounded-xl border bg-card/60 shadow-xs space-y-3"
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <Badge variant="outline" className="font-mono text-xs capitalize">
                                {asset.request_type.replace("_", " ")}
                              </Badge>
                              <span className="font-semibold text-sm text-foreground">{asset.title}</span>
                            </div>
                            <Badge
                              variant={
                                asset.status === "approved"
                                  ? "default"
                                  : asset.status === "uploaded"
                                  ? "secondary"
                                  : asset.status === "rejected"
                                  ? "danger"
                                  : "outline"
                              }
                              className="text-xs"
                            >
                              {asset.status === "requested" && `⏳ ${t("orders.detail_asset_status_requested")}`}
                              {asset.status === "uploaded" && `📥 ${t("orders.detail_asset_status_uploaded")}`}
                              {asset.status === "approved" && `✅ ${t("orders.detail_asset_status_approved")}`}
                              {asset.status === "rejected" && `❌ ${t("orders.detail_asset_status_rejected")}`}
                            </Badge>
                          </div>

                          <p className="text-xs text-muted-foreground">{asset.description}</p>

                          {/* Uploaded File Action & Preview */}
                          {asset.uploaded_file_url ? (
                            <div className="flex items-center justify-between p-2.5 rounded-lg border bg-muted/40 text-xs">
                              <div className="flex items-center gap-2">
                                <FileCheck className="h-4 w-4 text-emerald-500" />
                                <a
                                  href={asset.uploaded_file_url}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="text-blue-500 hover:underline font-mono inline-flex items-center gap-1"
                                >
                                  {t("orders.detail_download_file")} <ExternalLink className="h-3 w-3" />
                                </a>
                              </div>

                              {asset.status === "uploaded" && (
                                <div className="flex items-center gap-2">
                                  <Button
                                    size="sm"
                                    variant="outline"
                                    className="h-7 text-xs border-destructive text-destructive hover:bg-destructive/10"
                                    onClick={() => setSelectedAssetUuid(asset.uuid)}
                                  >
                                    <X className="mr-1 h-3.5 w-3.5" /> {t("orders.detail_request_revision")}
                                  </Button>
                                  <Button
                                    size="sm"
                                    className="h-7 text-xs bg-emerald-600 hover:bg-emerald-700 text-white"
                                    onClick={() => handleReviewAsset(asset.uuid, "approved")}
                                  >
                                    <Check className="mr-1 h-3.5 w-3.5" /> {t("orders.detail_approve_print")}
                                  </Button>
                                </div>
                              )}
                            </div>
                          ) : (
                            <div className="text-[11px] text-muted-foreground font-mono bg-muted/20 p-2 rounded border">
                              {t("orders.detail_token_label")}: <span className="text-foreground">{asset.token}</span>
                            </div>
                          )}

                          {/* Rejection Prompt Box */}
                          {selectedAssetUuid === asset.uuid && (
                            <div className="p-3 rounded-lg border border-destructive/30 bg-destructive/5 space-y-2">
                              <Label className="text-xs text-destructive">{t("orders.detail_revision_reason_label")}</Label>
                              <Input
                                placeholder={t("orders.detail_revision_placeholder")}
                                className="text-xs"
                                value={rejectionReason}
                                onChange={(e) => setRejectionReason(e.target.value)}
                              />
                              <div className="flex justify-end gap-2">
                                <Button size="sm" variant="ghost" onClick={() => setSelectedAssetUuid(null)}>
                                  {t("common.cancel")}
                                </Button>
                                <Button
                                  size="sm"
                                  variant="destructive"
                                  onClick={() => handleReviewAsset(asset.uuid, "rejected")}
                                >
                                  {t("orders.detail_send_revision")}
                                </Button>
                              </div>
                            </div>
                          )}
                        </div>
                      ))
                    )}
                  </div>
                </TabsContent>

                {/* TAB 3: SHIPPING & TRACKING */}
                <TabsContent value="shipping" className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-3">
                      <h4 className="text-sm font-semibold flex items-center gap-2">
                        <Truck className="h-4 w-4 text-primary" /> {t("orders.detail_shipping_info")}
                      </h4>

                      <div className="space-y-2">
                        <Label className="text-xs">{t("orders.detail_carrier_label")}</Label>
                        <Input
                          placeholder={t("orders.detail_carrier_placeholder")}
                          defaultValue={order.shipping_carrier}
                          onChange={(e) => setCarrier(e.target.value)}
                        />
                      </div>

                      <div className="space-y-2">
                        <Label className="text-xs">{t("orders.detail_tracking_no_label")}</Label>
                        <Input
                          placeholder="123456789"
                          defaultValue={order.tracking_number}
                          onChange={(e) => setTrackingNumber(e.target.value)}
                        />
                      </div>

                      <div className="space-y-2">
                        <Label className="text-xs">{t("orders.detail_tracking_url_label")}</Label>
                        <Input
                          placeholder="https://..."
                          defaultValue={order.tracking_url}
                          onChange={(e) => setTrackingUrl(e.target.value)}
                        />
                      </div>

                      <div className="flex items-center gap-2 pt-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleSaveShipping(false)}
                          disabled={updateShippingMutation.isPending}
                        >
                          {t("orders.detail_save_shipping")}
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => handleSaveShipping(true)}
                          disabled={updateShippingMutation.isPending}
                          className="bg-emerald-600 hover:bg-emerald-700"
                        >
                          <Truck className="mr-1.5 h-3.5 w-3.5" /> {t("orders.detail_mark_shipped")}
                        </Button>
                      </div>
                    </div>

                    {/* Delivery Address Card */}
                    <div className="space-y-3">
                      <h4 className="text-sm font-semibold">{t("orders.detail_shipping_address_heading")}</h4>
                      <div className="p-4 rounded-xl border bg-muted/30 text-xs space-y-1.5 leading-relaxed">
                        <p className="font-semibold text-foreground">{order.shipping_address?.full_name || order.user_name}</p>
                        <p className="text-muted-foreground">{order.shipping_address?.phone || "—"}</p>
                        <p>{order.shipping_address?.address_line1}</p>
                        {order.shipping_address?.address_line2 && <p>{order.shipping_address.address_line2}</p>}
                        <p className="font-medium">
                          {order.shipping_address?.city} / {order.shipping_address?.state} - {order.shipping_address?.country}
                        </p>
                      </div>

                      {order.customer_note && (
                        <div className="p-3 rounded-xl border border-amber-500/30 bg-amber-500/5 text-xs space-y-1">
                          <span className="font-semibold text-amber-600 dark:text-amber-400">{t("orders.detail_customer_note_heading")}:</span>
                          <p className="text-muted-foreground">{order.customer_note}</p>
                        </div>
                      )}
                    </div>
                  </div>
                </TabsContent>

                {/* TAB 4: CUSTOMER & FINANCIAL & ADMIN NOTES */}
                <TabsContent value="customer" className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-3">
                      <h4 className="text-sm font-semibold flex items-center gap-2">
                        <User className="h-4 w-4 text-primary" /> {t("orders.detail_customer_info_heading")}
                      </h4>
                      <div className="p-3.5 rounded-xl border text-xs space-y-1 bg-card/60">
                        <p className="font-semibold text-foreground">{order.user_name || "—"}</p>
                        <p className="text-muted-foreground font-mono">{order.user_email}</p>
                        {order.company_name && (
                          <div className="flex items-center gap-1.5 mt-2 text-foreground font-medium">
                            <Building className="h-3.5 w-3.5 text-muted-foreground" />
                            {order.company_name}
                          </div>
                        )}
                      </div>

                      <h4 className="text-sm font-semibold flex items-center gap-2 pt-2">
                        <CreditCard className="h-4 w-4 text-primary" /> {t("orders.detail_payment_heading")}
                      </h4>
                      <div className="p-3.5 rounded-xl border text-xs space-y-1 bg-card/60">
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">{t("orders.detail_payment_status")}:</span>
                          <Badge variant={order.payment_status === "paid" ? "default" : "outline"}>
                            {order.payment_status}
                          </Badge>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">{t("orders.detail_payment_provider")}:</span>
                          <span className="font-mono">{order.payment_provider || "—"}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-muted-foreground">{t("orders.fields.uuid")}:</span>
                          <span className="font-mono">{order.payment_reference || "—"}</span>
                        </div>
                      </div>

                      <h4 className="text-sm font-semibold flex items-center gap-2 pt-2">
                        <CreditCard className="h-4 w-4 text-primary" /> {t("orders.detail_refunds_heading")}
                      </h4>
                      <OrderRefundsPanel orderUuid={order.uuid} />
                    </div>

                    {/* Admin Note Section */}
                    <div className="space-y-3">
                      <h4 className="text-sm font-semibold">{t("orders.detail_admin_note_heading")}</h4>
                      <Textarea
                        placeholder={t("orders.detail_admin_note_placeholder")}
                        rows={6}
                        defaultValue={order.admin_note}
                        onChange={(e) => setAdminNote(e.target.value)}
                      />
                      <Button
                        size="sm"
                        onClick={handleSaveAdminNote}
                        disabled={updateNotesMutation.isPending}
                      >
                        {t("orders.detail_save_note")}
                      </Button>
                    </div>
                  </div>
                </TabsContent>
              </Tabs>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Request Asset Sub-Modal */}
      {order && (
        <OrderRequestAssetDialog
          open={assetModalOpen}
          onOpenChange={setAssetModalOpen}
          orderUuid={order.uuid}
          orderNumber={order.order_number}
        />
      )}
    </>
  );
}
