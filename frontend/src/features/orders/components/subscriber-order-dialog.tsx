"use client";

import React, { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ShoppingBag,
  Sparkles,
  CheckCircle2,
  ChevronRight,
  Plus,
  Minus,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { useLocale } from "@/providers/locale-provider";
import { productsService } from "@/features/products";
import { Bounded3DStage } from "@/features/products/components/Bounded3DStage";
import { ordersService, type CreateOrderPayload } from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";
import { cn } from "@/lib/utils";

interface SubscriberOrderDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
}

export function SubscriberOrderDialog({
  open,
  onOpenChange,
  onSuccess,
}: SubscriberOrderDialogProps) {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();

  const [step, setStep] = useState<"product" | "customize" | "shipping">("product");
  const [selectedProductUuid, setSelectedProductUuid] = useState<string>("");
  const [selectedVariantUuid, setSelectedVariantUuid] = useState<string | undefined>(undefined);
  const [quantity, setQuantity] = useState<number>(1);

  // Customization fields
  const [printName, setPrintName] = useState("");
  const [printTitle, setPrintTitle] = useState("");
  const [printCompany, setPrintCompany] = useState("");
  const [orderNotes, setOrderNotes] = useState("");

  // Shipping Address
  const [shippingFullName, setShippingFullName] = useState("");
  const [shippingPhone, setShippingPhone] = useState("");
  const [shippingAddress, setShippingAddress] = useState("");
  const [shippingCity, setShippingCity] = useState("");
  const [shippingPostalCode, setShippingPostalCode] = useState("");

  // Fetch Public Products
  const { data: productsData } = useQuery({
    queryKey: ["products", "public-list"],
    queryFn: () => productsService.listPublic(),
    enabled: open,
  });

  const products = productsData?.items || [];
  const activeProductUuid = selectedProductUuid || (products.length > 0 ? products[0].uuid : "");

  // Fetch Full Product Detail with Variants
  const { data: currentProduct } = useQuery({
    queryKey: ["products", "public-detail", activeProductUuid],
    queryFn: () => productsService.getPublic(activeProductUuid),
    enabled: Boolean(activeProductUuid),
  });

  const activeVariants = currentProduct?.variants?.filter((v) => v.is_active) || [];
  const selectedVariant = activeVariants.find((v) => v.uuid === selectedVariantUuid) || activeVariants[0];

  // Calculations
  const unitPrice = (currentProduct?.base_price || 0) + (selectedVariant?.price_delta || 0);
  const subtotal = unitPrice * quantity;
  const taxRate = currentProduct?.tax_rate || 20;
  const taxAmount = (subtotal * taxRate) / 100;
  const grandTotal = subtotal + taxAmount;
  const currency = currentProduct?.currency || "TRY";

  // Create Order Mutation
  const createOrderMutation = useMutation({
    mutationFn: (payload: CreateOrderPayload) => ordersService.createMyOrder(payload),
    onSuccess: () => {
      toast.success(t("orders.created_success"));
      queryClient.invalidateQueries({ queryKey: ordersQueryKeys.all });
      queryClient.invalidateQueries({ queryKey: ordersQueryKeys.myOrders() });
      onOpenChange(false);
      onSuccess?.();
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_creating"));
    },
  });

  const handlePlaceOrder = async () => {
    const prodUuid = currentProduct?.uuid || activeProductUuid;
    if (!prodUuid) {
      toast.error(t("orders.subscriber.select_product_error"));
      setStep("product");
      return;
    }

    if (!shippingFullName || !shippingAddress || !shippingCity || !shippingPhone) {
      toast.error(t("orders.fill_shipping_required"));
      setStep("shipping");
      return;
    }

    const payload: CreateOrderPayload = {
      items: [
        {
          product_uuid: prodUuid,
          variant_uuid: selectedVariant?.uuid,
          product_name: currentProduct?.name,
          quantity: quantity,
          unit_price: unitPrice,
          custom_print_data: {
            cardholder_name: printName,
            cardholder_title: printTitle,
            company_name: printCompany,
          },
        },
      ],
      shipping_address: {
        full_name: shippingFullName,
        phone: shippingPhone,
        address_line1: shippingAddress,
        city: shippingCity,
        postal_code: shippingPostalCode,
        country: locale === "tr" ? "Türkiye" : "Turkey",
      },
      billing_address: {
        full_name: shippingFullName,
        phone: shippingPhone,
        address_line1: shippingAddress,
        city: shippingCity,
        postal_code: shippingPostalCode,
        country: locale === "tr" ? "Türkiye" : "Turkey",
      },
      customer_note: orderNotes,
    };

    createOrderMutation.mutate(payload);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[760px] max-h-[92vh] overflow-y-auto p-6">
        <DialogHeader>
          <div className="flex items-center gap-2 mb-1">
            <div className="w-9 h-9 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
              <ShoppingBag className="w-5 h-5" />
            </div>
            <div>
              <DialogTitle className="text-xl font-bold">
                {t("orders.subscriber.order_modal_title")}
              </DialogTitle>
              <DialogDescription className="text-xs">
                {t("orders.subscriber.order_modal_desc")}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        {/* Steps Breadcrumb */}
        <div className="flex items-center justify-between border-y py-2.5 my-1 text-xs">
          <button
            type="button"
            onClick={() => setStep("product")}
            className={cn(
              "flex items-center gap-1.5 font-medium transition-colors",
              step === "product" ? "text-primary font-bold" : "text-muted-foreground hover:text-foreground"
            )}
          >
            <span className="w-5 h-5 rounded-full border flex items-center justify-center text-[10px]">1</span>
            {t("orders.subscriber.step_product")}
          </button>
          <ChevronRight className="w-4 h-4 text-muted-foreground" />
          <button
            type="button"
            onClick={() => setStep("customize")}
            className={cn(
              "flex items-center gap-1.5 font-medium transition-colors",
              step === "customize" ? "text-primary font-bold" : "text-muted-foreground hover:text-foreground"
            )}
          >
            <span className="w-5 h-5 rounded-full border flex items-center justify-center text-[10px]">2</span>
            {t("orders.subscriber.step_customize")}
          </button>
          <ChevronRight className="w-4 h-4 text-muted-foreground" />
          <button
            type="button"
            onClick={() => setStep("shipping")}
            className={cn(
              "flex items-center gap-1.5 font-medium transition-colors",
              step === "shipping" ? "text-primary font-bold" : "text-muted-foreground hover:text-foreground"
            )}
          >
            <span className="w-5 h-5 rounded-full border flex items-center justify-center text-[10px]">3</span>
            {t("orders.subscriber.step_shipping")}
          </button>
        </div>

        {/* STEP 1: PRODUCT & VARIANT SELECTION */}
        {step === "product" && (
          <div className="space-y-4 py-2">
            {/* 3D Interactive Stage Preview */}
            <div className="relative w-full h-64 rounded-2xl overflow-hidden border bg-zinc-950/90 shadow-inner">
              <Bounded3DStage
                modelUrl={selectedVariant?.model_3d_file_url || currentProduct?.model_3d_file_url}
                usdzUrl={selectedVariant?.model_3d_usdz_url || currentProduct?.model_3d_usdz_url}
                viewbox={selectedVariant?.model_3d_viewbox || currentProduct?.model_3d_viewbox}
                cardColor={selectedVariant?.color_code || "#18181b"}
                interactive={true}
                className="w-full h-full"
              />
              <div className="absolute top-3 left-3 bg-black/60 backdrop-blur-md px-2.5 py-1 rounded-lg border border-white/10 text-[11px] text-white flex items-center gap-1.5">
                <Sparkles className="w-3.5 h-3.5 text-amber-400" />
                <span>{t("orders.subscriber.live_3d_preview")}</span>
              </div>
            </div>

            {/* Product Selector */}
            <div className="space-y-2">
              <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t("orders.subscriber.select_product")}
              </Label>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                {products.map((p) => (
                  <div
                    key={p.uuid}
                    onClick={() => {
                      setSelectedProductUuid(p.uuid);
                      setSelectedVariantUuid(undefined);
                    }}
                    className={cn(
                      "p-3 rounded-xl border cursor-pointer transition-all flex items-center justify-between",
                      activeProductUuid === p.uuid
                        ? "border-primary bg-primary/5 ring-1 ring-primary"
                        : "border-border hover:border-border/80 bg-card"
                    )}
                  >
                    <div className="space-y-0.5">
                      <p className="font-semibold text-sm text-foreground">{p.name}</p>
                      <p className="text-xs text-muted-foreground line-clamp-1">{p.short_description}</p>
                    </div>
                    <div className="text-right">
                      <p className="font-bold text-sm text-primary">
                        {p.base_price.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {p.currency}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Variant / Material Selector */}
            {currentProduct?.variants && currentProduct.variants.length > 0 && (
              <div className="space-y-2">
                <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t("orders.subscriber.select_material")}
                </Label>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                  {currentProduct.variants
                    .filter((v) => v.is_active)
                    .map((v) => (
                      <div
                        key={v.uuid || v.id}
                        onClick={() => setSelectedVariantUuid(v.uuid)}
                        className={cn(
                          "p-2.5 rounded-xl border cursor-pointer transition-all flex items-center gap-2.5",
                          selectedVariant?.uuid === v.uuid
                            ? "border-primary bg-primary/5 ring-1 ring-primary"
                            : "border-border hover:border-border/80 bg-card"
                        )}
                      >
                        <div
                          className="w-4 h-4 rounded-full border shadow-sm shrink-0"
                          style={{ backgroundColor: v.color_code || "#18181b" }}
                        />
                        <div className="truncate">
                          <p className="font-medium text-xs text-foreground truncate">{v.name}</p>
                          {v.price_delta > 0 && (
                            <p className="text-[10px] text-muted-foreground">
                              +{v.price_delta.toFixed(2)} {currentProduct.currency}
                            </p>
                          )}
                        </div>
                      </div>
                    ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* STEP 2: CUSTOMIZE & QUANTITY */}
        {step === "customize" && (
          <div className="space-y-4 py-2">
            <div className="p-3 rounded-xl bg-muted/40 border space-y-1">
              <p className="text-xs font-semibold text-foreground flex items-center gap-1.5">
                <Sparkles className="w-4 h-4 text-primary" />
                {t("orders.subscriber.custom_print_heading")}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("orders.subscriber.custom_print_subtext")}
              </p>
            </div>

            <div className="space-y-3">
              <div className="space-y-1.5">
                <Label htmlFor="printName">{t("orders.subscriber.print_name_label")}</Label>
                <Input
                  id="printName"
                  placeholder={locale === "tr" ? "Örn: Furkan Meclis" : "e.g. John Doe"}
                  value={printName}
                  onChange={(e) => setPrintName(e.target.value)}
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="printTitle">{t("orders.subscriber.print_title_label")}</Label>
                  <Input
                    id="printTitle"
                    placeholder={locale === "tr" ? "Örn: CEO & Kurucu" : "e.g. Founder & CEO"}
                    value={printTitle}
                    onChange={(e) => setPrintTitle(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="printCompany">{t("orders.subscriber.print_company_label")}</Label>
                  <Input
                    id="printCompany"
                    placeholder={locale === "tr" ? "Örn: ComCode Yazılım A.Ş." : "e.g. Acme Inc."}
                    value={printCompany}
                    onChange={(e) => setPrintCompany(e.target.value)}
                  />
                </div>
              </div>
            </div>

            {/* Quantity Selector */}
            <div className="pt-2">
              <div className="flex items-center justify-between p-3.5 rounded-xl border bg-card">
                <div className="space-y-0.5">
                  <Label className="text-sm font-semibold">{t("orders.subscriber.quantity_label")}</Label>
                  <p className="text-xs text-muted-foreground">
                    {t("orders.subscriber.quantity_subtext")}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-8 w-8 rounded-lg"
                    onClick={() => setQuantity((q) => Math.max(1, q - 1))}
                  >
                    <Minus className="w-3.5 h-3.5" />
                  </Button>
                  <span className="font-bold text-base w-6 text-center">{quantity}</span>
                  <Button
                    type="button"
                    size="icon"
                    variant="outline"
                    className="h-8 w-8 rounded-lg"
                    onClick={() => setQuantity((q) => q + 1)}
                  >
                    <Plus className="w-3.5 h-3.5" />
                  </Button>
                </div>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="orderNotes">{t("orders.subscriber.notes_label")}</Label>
              <Input
                id="orderNotes"
                placeholder={t("orders.subscriber.notes_placeholder")}
                value={orderNotes}
                onChange={(e) => setOrderNotes(e.target.value)}
              />
            </div>
          </div>
        )}

        {/* STEP 3: SHIPPING ADDRESS & SUMMARY */}
        {step === "shipping" && (
          <div className="space-y-4 py-2">
            <div className="space-y-3">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="shipFullName">{t("orders.subscriber.shipping_name")}</Label>
                  <Input
                    id="shipFullName"
                    placeholder={locale === "tr" ? "Ad Soyad" : "Full Name"}
                    value={shippingFullName}
                    onChange={(e) => setShippingFullName(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="shipPhone">{t("orders.subscriber.shipping_phone")}</Label>
                  <Input
                    id="shipPhone"
                    placeholder={locale === "tr" ? "05XX XXX XX XX" : "+1 (555) 000-0000"}
                    value={shippingPhone}
                    onChange={(e) => setShippingPhone(e.target.value)}
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="shipAddress">{t("orders.subscriber.shipping_address")}</Label>
                <Input
                  id="shipAddress"
                  placeholder={locale === "tr" ? "Mahalle, Cadde, Sokak, Kapı No / Daire" : "Street address, Apt, Suite, Unit"}
                  value={shippingAddress}
                  onChange={(e) => setShippingAddress(e.target.value)}
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="shipCity">{t("orders.subscriber.shipping_city")}</Label>
                  <Input
                    id="shipCity"
                    placeholder={locale === "tr" ? "İstanbul" : "New York"}
                    value={shippingCity}
                    onChange={(e) => setShippingCity(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="shipPostal">{t("orders.subscriber.shipping_postal")}</Label>
                  <Input
                    id="shipPostal"
                    placeholder={locale === "tr" ? "34000" : "10001"}
                    value={shippingPostalCode}
                    onChange={(e) => setShippingPostalCode(e.target.value)}
                  />
                </div>
              </div>
            </div>

            {/* Order Cost Breakdown */}
            <div className="p-4 rounded-xl bg-muted/40 border space-y-2 mt-2">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>{currentProduct?.name} ({selectedVariant?.name || t("orders.subscriber.default_variant")}) x {quantity}</span>
                <span>{subtotal.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {currency}</span>
              </div>
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>{t("orders.subscriber.vat_label")} (%{taxRate})</span>
                <span>{taxAmount.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {currency}</span>
              </div>
              <Separator />
              <div className="flex items-center justify-between text-sm font-bold text-foreground">
                <span>{t("orders.fields.grand_total")}</span>
                <span className="text-primary text-base">
                  {grandTotal.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} {currency}
                </span>
              </div>
            </div>
          </div>
        )}

        <DialogFooter className="flex items-center justify-between sm:justify-between pt-4 border-t">
          {step !== "product" ? (
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                if (step === "shipping") setStep("customize");
                if (step === "customize") setStep("product");
              }}
            >
              {t("common.back")}
            </Button>
          ) : (
            <div />
          )}

          <div className="flex items-center gap-2">
            {step !== "shipping" ? (
              <Button
                type="button"
                onClick={() => {
                  if (step === "product") setStep("customize");
                  if (step === "customize") setStep("shipping");
                }}
                className="gap-1.5"
              >
                <span>{t("orders.subscriber.next_step")}</span>
                <ChevronRight className="w-4 h-4" />
              </Button>
            ) : (
              <Button
                type="button"
                disabled={createOrderMutation.isPending}
                onClick={handlePlaceOrder}
                className="gap-2 shadow-md"
              >
                <CheckCircle2 className="w-4 h-4" />
                {createOrderMutation.isPending
                  ? t("orders.subscriber.placing_order")
                  : t("orders.subscriber.confirm_order_btn")}
              </Button>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
