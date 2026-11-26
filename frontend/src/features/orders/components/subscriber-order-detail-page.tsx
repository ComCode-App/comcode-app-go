"use client";

import React, { useRef, useState } from "react";
import { ProductTypeBadge } from "./product-type-badge";
import { useParams, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  Clock,
  CreditCard,
  ExternalLink,
  FileCheck,
  MapPin,
  Package,
  Receipt,
  RefreshCw,
  Truck,
  Upload,
} from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { PageLoader } from "@/components/common/loading";
import { EntityPage } from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { uploadErrorKey } from "@/lib/api/upload-errors";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { ordersQueryKeys } from "../hooks/query-keys";
import {
  useCanPayOrders,
  useStartOrderPayment,
} from "../hooks/use-order-payment";
import {
  isOrderPayable,
  ordersService,
  type AssetRequestSummary,
} from "../services/orders.service";
import { OrderStatusBadge } from "./order-status-badge";

export const myOrderDetailQueryKey = (uuid: string) =>
  ["my-orders", "detail", uuid] as const;

export function SubscriberOrderDetailPage() {
  const { t, locale } = useLocale();
  const itemsCountLabel = (count: number) =>
    count === 1
      ? t("orders.subscriber.items_count_one", { count })
      : t("orders.subscriber.items_count_other", { count });
  const router = useRouter();
  const params = useParams<{ orderUuid: string }>();
  const orderUuid = params?.orderUuid ?? "";
  const queryClient = useQueryClient();
  const canPay = useCanPayOrders();
  const startPayment = useStartOrderPayment();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [activeAsset, setActiveAsset] = useState<AssetRequestSummary | null>(
    null,
  );
  const [uploadingAssetUuid, setUploadingAssetUuid] = useState<string | null>(
    null,
  );

  const orderQuery = useQuery({
    queryKey: myOrderDetailQueryKey(orderUuid),
    queryFn: () => ordersService.getMyOrder(orderUuid),
    enabled: Boolean(orderUuid),
  });
  const order = orderQuery.data;

  const uploadAssetMutation = useMutation({
    mutationFn: ({ token, file }: { token: string; file: File }) =>
      ordersService.uploadAsset(token, file),
    onSuccess: () => {
      toast.success(t("orders.subscriber.file_uploaded_success"));
      queryClient.invalidateQueries({ queryKey: ["my-orders"] });
      queryClient.invalidateQueries({ queryKey: ordersQueryKeys.myOrders() });
    },
    onError: (err: Error) => {
      const key = uploadErrorKey(err);
      toast.error(
        key ? t(key) : err?.message || t("orders.error_asset_request"),
      );
    },
    onSettled: () => {
      setUploadingAssetUuid(null);
      setActiveAsset(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
    },
  });

  const numberLocale = locale === "tr" ? "tr-TR" : "en-US";
  const formatMoney = (value: number, currency: string) =>
    `${value.toLocaleString(numberLocale, { minimumFractionDigits: 2 })} ${currency}`;

  if (orderQuery.isLoading) {
    return <PageLoader label={t("orders.detail_loading")} />;
  }

  if (orderQuery.isError || !order) {
    return (
      <ErrorState
        title={t("orders.subscriber.detail_not_found_title")}
        description={
          orderQuery.error?.message ||
          t("orders.subscriber.detail_not_found_desc")
        }
        retryLabel={t("orders.subscriber.back_to_orders")}
        onRetry={() => router.push(routes.app.orders.root)}
      />
    );
  }

  const handleTriggerUpload = (asset: AssetRequestSummary) => {
    setActiveAsset(asset);
    fileInputRef.current?.click();
  };

  const handleFileSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !activeAsset) return;
    setUploadingAssetUuid(activeAsset.uuid);
    uploadAssetMutation.mutate({ token: activeAsset.token, file });
  };

  const assetRequests = order.asset_requests || [];
  const items = order.items || [];
  const pendingAssetCount = assetRequests.filter(
    (a) => a.status === "requested" || a.status === "rejected",
  ).length;
  const payable = isOrderPayable(order);
  const orderTitle = `#${order.order_number}`;
  const orderDate = new Date(order.created_at).toLocaleDateString(
    numberLocale,
    { dateStyle: "long" },
  );

  const assetsSection = (
    <Card className="gap-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <FileCheck className="text-primary h-4 w-4" />
          {t("orders.subscriber.assets_heading")}
          {pendingAssetCount > 0 && (
            <Badge variant="danger" className="text-[11px]">
              {pendingAssetCount}
            </Badge>
          )}
        </CardTitle>
        <CardDescription>{t("orders.subscriber.assets_desc")}</CardDescription>
      </CardHeader>
      <CardContent>
        {assetRequests.length === 0 ? (
          <div className="space-y-2 rounded-xl border border-dashed py-8 text-center">
            <FileCheck className="text-muted-foreground/40 mx-auto h-8 w-8" />
            <p className="text-foreground text-sm font-semibold">
              {t("orders.subscriber.no_assets_title")}
            </p>
            <p className="text-muted-foreground mx-auto max-w-sm text-xs">
              {t("orders.subscriber.no_assets_desc")}
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {assetRequests.map((asset) => {
              const isUploading = uploadingAssetUuid === asset.uuid;
              const isRequested = asset.status === "requested";
              const isUploaded = asset.status === "uploaded";
              const isApproved = asset.status === "approved";
              const isRejected = asset.status === "rejected";

              return (
                <div
                  key={asset.uuid}
                  className={cn(
                    "space-y-3 rounded-xl border p-4",
                    isRejected
                      ? "border-destructive/40 bg-destructive/5"
                      : isRequested
                        ? "border-amber-500/40 bg-amber-500/5"
                        : "bg-card border-border",
                  )}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <Badge
                        variant="outline"
                        className="font-mono text-[10px] uppercase"
                      >
                        {asset.request_type.replace("_", " ")}
                      </Badge>
                      <span className="text-foreground text-sm font-semibold">
                        {asset.title}
                      </span>
                    </div>
                    {isRequested && (
                      <Badge variant="warning" className="gap-1 text-xs">
                        <Clock className="h-3 w-3" />
                        {t("orders.detail_asset_status_requested")}
                      </Badge>
                    )}
                    {isUploaded && (
                      <Badge variant="secondary" className="gap-1 text-xs">
                        <FileCheck className="h-3 w-3 text-blue-500" />
                        {t("orders.detail_asset_status_uploaded")}
                      </Badge>
                    )}
                    {isApproved && (
                      <Badge variant="success" className="gap-1 text-xs">
                        <CheckCircle2 className="h-3 w-3" />
                        {t("orders.detail_asset_status_approved")}
                      </Badge>
                    )}
                    {isRejected && (
                      <Badge variant="danger" className="gap-1 text-xs">
                        <AlertTriangle className="h-3 w-3" />
                        {t("orders.detail_asset_status_rejected")}
                      </Badge>
                    )}
                  </div>

                  {asset.description && (
                    <p className="text-muted-foreground text-xs leading-relaxed">
                      {asset.description}
                    </p>
                  )}

                  {isRejected && asset.rejection_reason && (
                    <div className="border-destructive/30 bg-destructive/10 text-destructive space-y-1 rounded-lg border p-3 text-xs">
                      <span className="flex items-center gap-1.5 font-semibold">
                        <AlertCircle className="h-3.5 w-3.5" />
                        {t("orders.subscriber.revision_reason_title")}:
                      </span>
                      <p className="text-destructive/90">
                        {asset.rejection_reason}
                      </p>
                    </div>
                  )}

                  <div className="border-border/50 flex flex-wrap items-center justify-between gap-3 border-t pt-2 text-xs">
                    <span className="text-muted-foreground text-[11px]">
                      {isRequested &&
                        t("orders.subscriber.status_requested_hint")}
                      {isUploaded &&
                        t("orders.subscriber.status_uploaded_hint")}
                      {isApproved &&
                        t("orders.subscriber.status_approved_hint")}
                      {isRejected &&
                        t("orders.subscriber.status_rejected_hint")}
                    </span>
                    <div className="flex items-center gap-2">
                      {asset.uploaded_file_url && (
                        <a
                          href={asset.uploaded_file_url}
                          target="_blank"
                          rel="noreferrer"
                          className="text-primary inline-flex items-center gap-1 text-xs hover:underline"
                        >
                          {t("orders.detail_download_file")}
                          <ExternalLink className="h-3 w-3" />
                        </a>
                      )}
                      {(isRequested || isRejected || isUploaded) && (
                        <Button
                          size="sm"
                          variant={isUploaded ? "outline" : "default"}
                          className="h-8 gap-1.5 text-xs"
                          onClick={() => handleTriggerUpload(asset)}
                          disabled={isUploading}
                        >
                          {isUploading ? (
                            <>
                              <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                              {t("orders.subscriber.uploading_file")}
                            </>
                          ) : (
                            <>
                              <Upload className="h-3.5 w-3.5" />
                              {isUploaded
                                ? t("orders.subscriber.reupload_btn")
                                : t("orders.subscriber.upload_file_btn")}
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
      </CardContent>
    </Card>
  );

  return (
    <EntityPage
      title={orderTitle}
      description={`${t("orders.subscriber.order_date")}: ${orderDate}`}
      permission={permissions.orders.appRead}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        {
          label: t("orders.subscriber.page_title"),
          href: routes.app.orders.root,
        },
        { label: orderTitle },
      ]}
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <OrderStatusBadge status={order.status} />
          {order.tracking_url && (
            <Button size="sm" variant="outline" className="gap-1.5" asChild>
              <a href={order.tracking_url} target="_blank" rel="noreferrer">
                <Truck className="h-4 w-4" />
                {t("orders.subscriber.tracking_label")}
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            </Button>
          )}
        </div>
      }
    >
      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileSelected}
        className="hidden"
        accept=".ai,.svg,.pdf,.eps,.png,.jpg,.jpeg,.zip"
      />

      <div className="space-y-6 pb-12">
        {order.status === "awaiting_approval" && (
          <div className="flex items-start gap-3 rounded-xl border border-violet-500/30 bg-violet-500/5 p-4">
            <Clock className="mt-0.5 h-4 w-4 shrink-0 text-violet-600" />
            <div className="space-y-0.5">
              <p className="text-sm font-semibold">
                {t("orders.approval.awaiting_title")}
              </p>
              <p className="text-muted-foreground text-xs">
                {t("orders.approval.awaiting_desc")}
              </p>
            </div>
          </div>
        )}
        {order.status === "cancelled" && order.rejection_reason && (
          <div className="flex items-start gap-3 rounded-xl border border-rose-500/30 bg-rose-500/5 p-4">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
            <div className="space-y-0.5">
              <p className="text-sm font-semibold">
                {t("orders.approval.rejected_title")}
              </p>
              <p className="text-muted-foreground text-xs">
                {t("orders.approval.rejected_reason", {
                  reason: order.rejection_reason,
                })}
              </p>
            </div>
          </div>
        )}
        {payable && (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-500/30 bg-amber-500/5 p-4">
            <div className="space-y-0.5">
              <p className="flex items-center gap-1.5 text-sm font-semibold">
                <CreditCard className="h-4 w-4 text-amber-600" />
                {t("orders.checkout.awaiting_payment")}
              </p>
              <p className="text-muted-foreground text-xs">
                {t("orders.checkout.awaiting_payment_desc")}
              </p>
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

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div className="space-y-6 lg:col-span-2">
            {pendingAssetCount > 0 && assetsSection}

            {/* Order items */}
            <Card className="gap-4">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Package className="text-primary h-4 w-4" />
                  {t("orders.subscriber.items_heading")}
                  <Badge variant="secondary" className="text-[11px]">
                    {itemsCountLabel(items.length)}
                  </Badge>
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <div className="divide-border divide-y border-t">
                  {items.map((item) => (
                    <div
                      key={item.uuid}
                      className="flex flex-col gap-3 p-4 sm:flex-row sm:items-start sm:justify-between"
                    >
                      <div className="min-w-0 space-y-1">
                        {item.recipient_name && (
                          <p className="text-foreground text-sm font-semibold">
                            {item.recipient_name}
                            {item.recipient_title && (
                              <span className="text-muted-foreground ml-1.5 text-xs font-normal">
                                ({item.recipient_title})
                              </span>
                            )}
                          </p>
                        )}
                        <p
                          className={cn(
                            "text-sm",
                            item.recipient_name
                              ? "text-muted-foreground"
                              : "text-foreground font-semibold",
                          )}
                        >
                          {item.product_name}
                          {item.variant_name && (
                            <span className="text-muted-foreground">
                              {" · "}
                              {item.variant_name}
                            </span>
                          )}
                          <ProductTypeBadge
                            productType={item.product_type}
                            className="ml-1.5 align-middle"
                          />
                        </p>

                        <div className="flex flex-wrap items-center gap-2 pt-1">
                          {item.card_smart_code ? (
                            <>
                              <Badge
                                variant="secondary"
                                className="font-mono text-[10px]"
                              >
                                {item.card_smart_code}
                              </Badge>
                              {item.route_path && (
                                <a
                                  href={item.route_path}
                                  target="_blank"
                                  rel="noreferrer"
                                  className="text-primary inline-flex items-center gap-1 text-xs font-medium hover:underline"
                                >
                                  {item.route_path}
                                  <ExternalLink className="h-3 w-3" />
                                </a>
                              )}
                            </>
                          ) : (
                            <Badge
                              variant="outline"
                              className="text-muted-foreground text-[10px]"
                            >
                              {t("orders.subscriber.preparing")}
                            </Badge>
                          )}
                        </div>

                        {item.custom_print_data &&
                          Object.keys(item.custom_print_data).length > 0 && (
                            <div className="bg-muted/40 mt-2 space-y-0.5 rounded-lg border p-2.5 text-xs">
                              <span className="text-primary block text-[10px] font-semibold tracking-wider uppercase">
                                {t("orders.subscriber.laser_engraving")}
                              </span>
                              {Object.entries(item.custom_print_data).map(
                                ([k, v]) => (
                                  <div
                                    key={k}
                                    className="text-muted-foreground"
                                  >
                                    <span className="text-foreground font-medium">
                                      {k}:
                                    </span>{" "}
                                    {String(v)}
                                  </div>
                                ),
                              )}
                            </div>
                          )}
                      </div>

                      <div className="shrink-0 text-left sm:text-right">
                        <p className="text-foreground text-sm font-semibold">
                          {formatMoney(item.total_price, order.currency)}
                        </p>
                        <p className="text-muted-foreground text-xs">
                          {item.quantity} ×{" "}
                          {formatMoney(item.unit_price, order.currency)}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>

            {pendingAssetCount === 0 && assetsSection}
          </div>

          <div className="space-y-6">
            {/* Summary */}
            <Card className="gap-4">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Receipt className="text-primary h-4 w-4" />
                  {t("orders.subscriber.summary_heading")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                <div className="text-muted-foreground flex justify-between">
                  <span>{t("orders.subscriber.subtotal_label")}</span>
                  <span>{formatMoney(order.subtotal, order.currency)}</span>
                </div>
                <div className="text-muted-foreground flex justify-between">
                  <span>{t("orders.subscriber.vat_label")}</span>
                  <span>{formatMoney(order.tax_total, order.currency)}</span>
                </div>
                <div className="text-muted-foreground flex justify-between">
                  <span>{t("orders.columns.shipping")}</span>
                  <span>
                    {formatMoney(order.shipping_total, order.currency)}
                  </span>
                </div>
                <Separator className="my-2" />
                <div className="flex justify-between font-semibold">
                  <span>{t("orders.fields.grand_total")}</span>
                  <span className="text-primary">
                    {formatMoney(order.grand_total, order.currency)}
                  </span>
                </div>
              </CardContent>
            </Card>

            {/* Shipping */}
            <Card className="gap-4">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Truck className="text-primary h-4 w-4" />
                  {t("orders.subscriber.shipping_heading")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 text-sm">
                {order.shipping_carrier ? (
                  <>
                    <div className="flex justify-between gap-3">
                      <span className="text-muted-foreground">
                        {t("orders.detail_carrier_label")}
                      </span>
                      <span className="font-medium">
                        {order.shipping_carrier}
                      </span>
                    </div>
                    {order.tracking_number && (
                      <div className="flex justify-between gap-3">
                        <span className="text-muted-foreground">
                          {t("orders.fields.tracking_number")}
                        </span>
                        <span className="font-mono font-medium">
                          {order.tracking_number}
                        </span>
                      </div>
                    )}
                  </>
                ) : (
                  <p className="text-muted-foreground text-xs">
                    {t("orders.subscriber.shipping_not_yet")}
                  </p>
                )}
              </CardContent>
            </Card>

            {/* Address */}
            <Card className="gap-4">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <MapPin className="text-primary h-4 w-4" />
                  {t("orders.subscriber.shipping_address")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-1 text-sm leading-relaxed">
                <p className="font-semibold">
                  {order.shipping_address?.full_name}
                </p>
                {order.shipping_address?.phone && (
                  <p className="text-muted-foreground">
                    {order.shipping_address.phone}
                  </p>
                )}
                <p>{order.shipping_address?.address_line1}</p>
                {order.shipping_address?.address_line2 && (
                  <p>{order.shipping_address.address_line2}</p>
                )}
                <p>
                  {[
                    order.shipping_address?.city,
                    order.shipping_address?.postal_code,
                    order.shipping_address?.country,
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </p>

                {order.customer_note && (
                  <div className="mt-3 rounded-lg border border-amber-500/20 bg-amber-500/5 p-2.5 text-xs">
                    <span className="block font-semibold text-amber-600 dark:text-amber-400">
                      {t("orders.detail_customer_note_heading")}
                    </span>
                    <p className="text-muted-foreground mt-0.5">
                      {order.customer_note}
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </EntityPage>
  );
}
