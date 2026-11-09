"use client";

import React, { useState, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ShoppingBag,
  FileCheck,
  Upload,
  ExternalLink,
  CheckCircle2,
  AlertCircle,
  Clock,
  Package,
  Truck,
  AlertTriangle,
  RefreshCw,
  MapPin,
  CreditCard,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { isOrderPayable } from "../services/orders.service";
import { useCanPayOrders, useStartOrderPayment } from "../hooks/use-order-payment";
import { useLocale } from "@/providers/locale-provider";
import { ordersService, type AssetRequestSummary } from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";
import { uploadErrorKey } from "@/lib/api/upload-errors";
import { cn } from "@/lib/utils";

interface SubscriberOrderDetailDialogProps {
  orderUuid: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function SubscriberOrderDetailDialog({
  orderUuid,
  open,
  onOpenChange,
}: SubscriberOrderDetailDialogProps) {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();
  const canPay = useCanPayOrders();
  const startPayment = useStartOrderPayment();
  const [uploadingAssetUuid, setUploadingAssetUuid] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [activeToken, setActiveToken] = useState<string>("");
  const [activeAssetUuid, setActiveAssetUuid] = useState<string>("");

  const { data: order, isLoading, refetch } = useQuery({
    queryKey: ["my-orders", "detail", orderUuid],
    queryFn: () => ordersService.getMyOrder(orderUuid!),
    enabled: Boolean(orderUuid) && open,
  });

  const uploadAssetMutation = useMutation({
    mutationFn: async ({ token, file }: { token: string; file: File }) => {
      return ordersService.uploadAsset(token, file);
    },
    onSuccess: () => {
      toast.success(t("orders.subscriber.file_uploaded_success"));
      queryClient.invalidateQueries({ queryKey: ["my-orders"] });
      queryClient.invalidateQueries({ queryKey: ordersQueryKeys.myOrders() });
      queryClient.invalidateQueries({ queryKey: ["my-orders", "detail", orderUuid] });
      refetch();
    },
    onError: (err: Error) => {
      const key = uploadErrorKey(err);
      toast.error(key ? t(key) : err?.message || t("orders.error_asset_request"));
    },
    onSettled: () => {
      setUploadingAssetUuid(null);
      setActiveToken("");
      setActiveAssetUuid("");
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    },
  });

  const handleTriggerUpload = (asset: AssetRequestSummary) => {
    setActiveToken(asset.token);
    setActiveAssetUuid(asset.uuid);
    fileInputRef.current?.click();
  };

  const handleFileSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !activeToken) return;

    setUploadingAssetUuid(activeAssetUuid);
    uploadAssetMutation.mutate({ token: activeToken, file });
  };

  if (!open || !orderUuid) return null;

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "paid":
        return <Badge variant="success">{t("orders.status_paid")}</Badge>;
      case "design_pending":
        return <Badge variant="warning">{t("orders.status_design_pending")}</Badge>;
      case "design_approved":
        return <Badge variant="success">{t("orders.status_design_approved")}</Badge>;
      case "in_production":
        return <Badge variant="default">{t("orders.status_in_production")}</Badge>;
      case "ready_to_ship":
        return <Badge variant="secondary">{t("orders.status_ready_to_ship")}</Badge>;
      case "shipped":
        return <Badge variant="default">{t("orders.status_shipped")}</Badge>;
      case "delivered":
        return <Badge variant="success">{t("orders.status_delivered")}</Badge>;
      case "cancelled":
        return <Badge variant="danger">{t("orders.status_cancelled")}</Badge>;
      case "pending":
      default:
        return <Badge variant="secondary">{t("orders.status_pending")}</Badge>;
    }
  };

  const pendingAssetCount = (order?.asset_requests || []).filter(
    (a) => a.status === "requested" || a.status === "rejected"
  ).length;

  return (
    <>
      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileSelected}
        className="hidden"
        accept=".ai,.svg,.pdf,.eps,.png,.jpg,.jpeg,.zip"
      />

      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto p-6">
          {isLoading || !order ? (
            <div className="py-16 flex flex-col items-center justify-center gap-3">
              <div className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
              <p className="text-sm text-muted-foreground">{t("orders.detail_loading")}</p>
            </div>
          ) : (
            <div className="space-y-5">
              {/* Header */}
              <DialogHeader className="border-b pb-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <DialogTitle className="flex items-center gap-2 text-xl font-bold font-mono">
                      <ShoppingBag className="h-5 w-5 text-primary" />
                      #{order.order_number}
                    </DialogTitle>
                    <DialogDescription className="text-xs text-muted-foreground mt-0.5">
                      {t("orders.subscriber.order_date")}: {new Date(order.created_at).toLocaleDateString(locale === "tr" ? "tr-TR" : "en-US", { dateStyle: "long" })}
                    </DialogDescription>
                  </div>

                  <div className="flex items-center gap-2">
                    {getStatusBadge(order.status)}
                    <span className="font-mono font-bold text-sm text-foreground bg-muted px-2.5 py-1 rounded-lg border">
                      {order.grand_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}
                    </span>
                  </div>
                </div>
              </DialogHeader>

              {isOrderPayable(order) && (
                <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-500/30 bg-amber-500/5 p-3.5">
                  <div className="space-y-0.5">
                    <p className="flex items-center gap-1.5 text-sm font-semibold">
                      <CreditCard className="h-4 w-4 text-amber-600" />
                      {t("orders.checkout.awaiting_payment")}
                    </p>
                    <p className="text-xs text-muted-foreground">{t("orders.checkout.awaiting_payment_desc")}</p>
                  </div>
                  {canPay && (
                    <Button
                      size="sm"
                      className="gap-1.5"
                      disabled={startPayment.isPending}
                      onClick={() => startPayment.mutate(order.uuid)}
                    >
                      <CreditCard className="h-3.5 w-3.5" />
                      {t("orders.checkout.pay_now")}
                    </Button>
                  )}
                </div>
              )}

              {/* Tabs */}
              <Tabs defaultValue={pendingAssetCount > 0 ? "assets" : "items"} className="w-full">
                <TabsList className="grid grid-cols-3 bg-muted mb-4">
                  <TabsTrigger value="assets" className="text-xs flex items-center gap-1.5">
                    <FileCheck className="h-3.5 w-3.5" />
                    <span>{t("orders.subscriber.assets_heading")}</span>
                    {order.asset_requests.length > 0 && (
                      <Badge
                        variant={pendingAssetCount > 0 ? "danger" : "secondary"}
                        className="text-[10px] px-1.5 py-0 h-4 ml-1"
                      >
                        {order.asset_requests.length}
                      </Badge>
                    )}
                  </TabsTrigger>
                  <TabsTrigger value="items" className="text-xs flex items-center gap-1.5">
                    <Package className="h-3.5 w-3.5" />
                    <span>{t("orders.subscriber.items_heading")} ({order.items.length})</span>
                  </TabsTrigger>
                  <TabsTrigger value="shipping" className="text-xs flex items-center gap-1.5">
                    <Truck className="h-3.5 w-3.5" />
                    <span>{t("orders.columns.shipping")}</span>
                  </TabsTrigger>
                </TabsList>

                {/* TAB 1: ASSET & LOGO REQUESTS */}
                <TabsContent value="assets" className="space-y-4">
                  <div className="p-3.5 rounded-xl bg-primary/5 border border-primary/20 space-y-1">
                    <h4 className="text-xs font-bold text-foreground flex items-center gap-1.5">
                      <FileCheck className="h-4 w-4 text-primary" />
                      {t("orders.subscriber.assets_heading")}
                    </h4>
                    <p className="text-xs text-muted-foreground">
                      {t("orders.subscriber.assets_desc")}
                    </p>
                  </div>

                  {order.asset_requests.length === 0 ? (
                    <div className="py-10 text-center border border-dashed rounded-xl space-y-2">
                      <FileCheck className="h-8 w-8 mx-auto text-muted-foreground/40" />
                      <p className="text-xs font-semibold text-foreground">{t("orders.subscriber.no_assets_title")}</p>
                      <p className="text-xs text-muted-foreground max-w-sm mx-auto">{t("orders.subscriber.no_assets_desc")}</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {order.asset_requests.map((asset) => {
                        const isUploading = uploadingAssetUuid === asset.uuid;
                        const isRequested = asset.status === "requested";
                        const isUploaded = asset.status === "uploaded";
                        const isApproved = asset.status === "approved";
                        const isRejected = asset.status === "rejected";

                        return (
                          <div
                            key={asset.uuid}
                            className={cn(
                              "p-4 rounded-xl border transition-all space-y-3",
                              isRejected
                                ? "border-destructive/40 bg-destructive/5"
                                : isRequested
                                ? "border-amber-500/40 bg-amber-500/5"
                                : "bg-card border-border"
                            )}
                          >
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <div className="flex items-center gap-2">
                                <Badge variant="outline" className="font-mono text-[10px] uppercase">
                                  {asset.request_type.replace("_", " ")}
                                </Badge>
                                <span className="font-semibold text-sm text-foreground">{asset.title}</span>
                              </div>

                              <div>
                                {isRequested && (
                                  <Badge variant="warning" className="text-xs gap-1">
                                    <Clock className="w-3 h-3" />
                                    {t("orders.detail_asset_status_requested")}
                                  </Badge>
                                )}
                                {isUploaded && (
                                  <Badge variant="secondary" className="text-xs gap-1">
                                    <FileCheck className="w-3 h-3 text-blue-500" />
                                    {t("orders.detail_asset_status_uploaded")}
                                  </Badge>
                                )}
                                {isApproved && (
                                  <Badge variant="success" className="text-xs gap-1">
                                    <CheckCircle2 className="w-3 h-3" />
                                    {t("orders.detail_asset_status_approved")}
                                  </Badge>
                                )}
                                {isRejected && (
                                  <Badge variant="danger" className="text-xs gap-1">
                                    <AlertTriangle className="w-3 h-3" />
                                    {t("orders.detail_asset_status_rejected")}
                                  </Badge>
                                )}
                              </div>
                            </div>

                            {/* Description from Admin */}
                            {asset.description && (
                              <p className="text-xs text-muted-foreground leading-relaxed">
                                {asset.description}
                              </p>
                            )}

                            {/* Revision Reason Alert Box if rejected */}
                            {isRejected && asset.rejection_reason && (
                              <div className="p-3 rounded-lg border border-destructive/30 bg-destructive/10 text-xs text-destructive space-y-1">
                                <span className="font-semibold flex items-center gap-1.5">
                                  <AlertCircle className="w-3.5 h-3.5" />
                                  {t("orders.subscriber.revision_reason_title")}:
                                </span>
                                <p className="text-destructive/90">{asset.rejection_reason}</p>
                              </div>
                            )}

                            {/* Status Hint and Action */}
                            <div className="flex flex-wrap items-center justify-between gap-3 pt-1 border-t border-border/50 text-xs">
                              <span className="text-muted-foreground text-[11px]">
                                {isRequested && t("orders.subscriber.status_requested_hint")}
                                {isUploaded && t("orders.subscriber.status_uploaded_hint")}
                                {isApproved && t("orders.subscriber.status_approved_hint")}
                                {isRejected && t("orders.subscriber.status_rejected_hint")}
                              </span>

                              <div className="flex items-center gap-2">
                                {asset.uploaded_file_url && (
                                  <a
                                    href={asset.uploaded_file_url}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="inline-flex items-center gap-1 text-xs text-primary hover:underline font-mono"
                                  >
                                    <span>{t("orders.detail_download_file")}</span>
                                    <ExternalLink className="w-3 h-3" />
                                  </a>
                                )}

                                {(isRequested || isRejected || isUploaded) && (
                                  <Button
                                    size="sm"
                                    variant={isRejected || isRequested ? "default" : "outline"}
                                    className="h-8 text-xs gap-1.5 shadow-sm"
                                    onClick={() => handleTriggerUpload(asset)}
                                    disabled={isUploading}
                                  >
                                    {isUploading ? (
                                      <>
                                        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                                        <span>{t("orders.subscriber.uploading_file")}</span>
                                      </>
                                    ) : (
                                      <>
                                        <Upload className="w-3.5 h-3.5" />
                                        <span>
                                          {isUploaded
                                            ? t("orders.subscriber.reupload_btn")
                                            : t("orders.subscriber.upload_file_btn")}
                                        </span>
                                      </>
                                    )}
                                  </Button>
                                )}
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </TabsContent>

                {/* TAB 2: ORDER ITEMS & CARDS */}
                <TabsContent value="items" className="space-y-4">
                  <div className="rounded-xl border overflow-hidden">
                    <table className="w-full text-xs text-left">
                      <thead className="bg-muted/60 text-muted-foreground uppercase font-mono text-[10px]">
                        <tr>
                          <th className="p-3">{t("orders.subscriber.recipient_and_product")}</th>
                          <th className="p-3">{t("orders.subscriber.card_route_col")}</th>
                          <th className="p-3 text-center">{t("orders.detail_quantity")}</th>
                          <th className="p-3 text-right">{t("orders.detail_unit_price")}</th>
                          <th className="p-3 text-right">{t("orders.detail_total")}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {order.items.map((item) => (
                          <tr key={item.uuid} className="hover:bg-muted/30">
                            <td className="p-3">
                              {item.recipient_name && (
                                <div className="font-bold text-foreground text-xs mb-0.5">
                                  {item.recipient_name}
                                  {item.recipient_title && (
                                    <span className="text-muted-foreground font-normal ml-1.5 text-[11px]">
                                      ({item.recipient_title})
                                    </span>
                                  )}
                                </div>
                              )}
                              <div className="font-medium text-muted-foreground">{item.product_name}</div>
                              {item.variant_name && (
                                <span className="text-[11px] text-muted-foreground">{item.variant_name}</span>
                              )}
                              {item.custom_print_data && Object.keys(item.custom_print_data).length > 0 && (
                                <div className="mt-2 p-2 rounded-lg bg-muted/40 border text-[11px] space-y-0.5">
                                  <span className="font-semibold text-primary text-[10px] uppercase tracking-wider block">
                                    {t("orders.subscriber.laser_engraving")}:
                                  </span>
                                  {Object.entries(item.custom_print_data).map(([k, v]) => (
                                    <div key={k} className="text-muted-foreground font-mono">
                                      <span className="font-medium text-foreground">{k}:</span> {String(v)}
                                    </div>
                                  ))}
                                </div>
                              )}
                            </td>
                            <td className="p-3 font-mono">
                              {item.card_smart_code ? (
                                <div className="space-y-1">
                                  <Badge variant="secondary" className="font-mono text-[10px]">
                                    {item.card_smart_code}
                                  </Badge>
                                  {item.route_path && (
                                    <div>
                                      <a
                                        href={item.route_path}
                                        target="_blank"
                                        rel="noreferrer"
                                        className="text-primary hover:underline text-[11px] inline-flex items-center gap-1 font-semibold"
                                      >
                                        {item.route_path} <ExternalLink className="h-3 w-3" />
                                      </a>
                                    </div>
                                  )}
                                </div>
                              ) : (
                                <Badge variant="outline" className="text-[10px] text-muted-foreground">
                                  {item.provisioning_status || t("orders.subscriber.preparing")}
                                </Badge>
                              )}
                            </td>
                            <td className="p-3 text-center font-bold font-mono">
                              <Badge variant="outline">{item.quantity}</Badge>
                            </td>
                            <td className="p-3 text-right font-mono">
                              {item.unit_price.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}
                            </td>
                            <td className="p-3 text-right font-mono font-bold text-foreground">
                              {item.total_price.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  {/* Financial Breakdown */}
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
                        <span className="font-mono text-primary font-bold">
                          {order.grand_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {order.currency}
                        </span>
                      </div>
                    </div>
                  </div>
                </TabsContent>

                {/* TAB 3: SHIPPING & TRACKING */}
                <TabsContent value="shipping" className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {/* Shipping Status & Tracking */}
                    <div className="space-y-3 p-4 rounded-xl border bg-card/60">
                      <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                        <Truck className="h-4 w-4 text-primary" />
                        {t("orders.columns.shipping")}
                      </h4>

                      {order.shipping_carrier ? (
                        <div className="space-y-2 text-xs">
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">{t("orders.detail_carrier_label")}:</span>
                            <span className="font-semibold text-foreground">{order.shipping_carrier}</span>
                          </div>
                          {order.tracking_number && (
                            <div className="flex justify-between">
                              <span className="text-muted-foreground">{t("orders.fields.tracking_number")}:</span>
                              <span className="font-mono font-bold text-foreground">{order.tracking_number}</span>
                            </div>
                          )}
                          {order.tracking_url && (
                            <Button
                              size="sm"
                              variant="outline"
                              className="w-full mt-2 gap-1.5 text-xs"
                              onClick={() => window.open(order.tracking_url, "_blank")}
                            >
                              <span>{t("orders.subscriber.tracking_label")}</span>
                              <ExternalLink className="w-3.5 h-3.5" />
                            </Button>
                          )}
                        </div>
                      ) : (
                        <p className="text-xs text-muted-foreground">
                          {t("orders.status_pending")}
                        </p>
                      )}
                    </div>

                    {/* Address Card */}
                    <div className="space-y-3 p-4 rounded-xl border bg-card/60 text-xs">
                      <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                        <MapPin className="h-4 w-4 text-primary" />
                        {t("orders.subscriber.step_shipping")}
                      </h4>

                      <div className="space-y-1 leading-relaxed">
                        <p className="font-semibold text-foreground">{order.shipping_address?.full_name}</p>
                        <p className="text-muted-foreground">{order.shipping_address?.phone}</p>
                        <p>{order.shipping_address?.address_line1}</p>
                        {order.shipping_address?.address_line2 && <p>{order.shipping_address.address_line2}</p>}
                        <p className="font-medium text-foreground">
                          {order.shipping_address?.city} {order.shipping_address?.postal_code} - {order.shipping_address?.country}
                        </p>
                      </div>

                      {order.customer_note && (
                        <div className="p-2.5 rounded-lg border border-amber-500/20 bg-amber-500/5 text-xs mt-2">
                          <span className="font-semibold text-amber-600 dark:text-amber-400 block">
                            {t("orders.detail_customer_note_heading")}:
                          </span>
                          <p className="text-muted-foreground mt-0.5">{order.customer_note}</p>
                        </div>
                      )}
                    </div>
                  </div>
                </TabsContent>
              </Tabs>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
