"use client";

import React, { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  Users,
  Plus,
  Trash2,
  FileSpreadsheet,
  ChevronRight,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { useLocale } from "@/providers/locale-provider";
import { productsService } from "@/features/products";
import {
  ordersService,
  type CreateOrderPayload,
  type CreateOrderItemPayload,
} from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";

interface RecipientRow {
  id: string;
  name: string;
  title: string;
  department: string;
  email: string;
  phone: string;
  printName: string;
}

interface BulkOrderDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isPlatformAdmin?: boolean;
}

export function BulkOrderDialog({
  open,
  onOpenChange,
  isPlatformAdmin = true,
}: BulkOrderDialogProps) {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();

  const [step, setStep] = useState<"product" | "recipients" | "shipping">(
    "product",
  );
  const [selectedProductUuid, setSelectedProductUuid] = useState<string>("");
  const [selectedVariantUuid, setSelectedVariantUuid] = useState<
    string | undefined
  >(undefined);

  // Quick Paste/Import mode toggle
  const [showPasteMode, setShowPasteMode] = useState(false);
  const [pasteText, setPasteText] = useState("");

  // Recipients list
  const [recipients, setRecipients] = useState<RecipientRow[]>([
    {
      id: "1",
      name: "",
      title: "",
      department: "",
      email: "",
      phone: "",
      printName: "",
    },
  ]);

  // Shipping details
  const [shippingFullName, setShippingFullName] = useState("");
  const [shippingCompanyName, setShippingCompanyName] = useState("");
  const [shippingPhone, setShippingPhone] = useState("");
  const [shippingAddress, setShippingAddress] = useState("");
  const [shippingCity, setShippingCity] = useState("");
  const [shippingState, setShippingState] = useState("");
  const [shippingPostalCode, setShippingPostalCode] = useState("");
  const [shippingCountry, setShippingCountry] = useState("Türkiye");
  const [customerNote, setCustomerNote] = useState("");

  // Fetch Public Products
  const { data: productsData } = useQuery({
    queryKey: ["products", "public-list"],
    queryFn: () => productsService.listPublic(),
    enabled: open,
  });

  const products = productsData?.items || [];
  const activeProductUuid =
    selectedProductUuid || (products.length > 0 ? products[0].uuid : "");

  // Fetch Detail of selected product
  const { data: currentProduct } = useQuery({
    queryKey: ["products", "public-detail", activeProductUuid],
    queryFn: () => productsService.getPublic(activeProductUuid),
    enabled: Boolean(activeProductUuid),
  });

  const activeVariants =
    currentProduct?.variants?.filter((v) => v.is_active) || [];
  const selectedVariant =
    activeVariants.find((v) => v.uuid === selectedVariantUuid) ||
    activeVariants[0];

  const unitPrice =
    (currentProduct?.base_price || 0) + (selectedVariant?.price_delta || 0);
  const totalQuantity = recipients.length;
  const subtotal = unitPrice * totalQuantity;
  const taxRate = currentProduct?.tax_rate || 20;
  const taxAmount = (subtotal * taxRate) / 100;
  const grandTotal = subtotal + taxAmount;
  const currency = currentProduct?.currency || "TRY";

  const createOrderMutation = useMutation({
    mutationFn: (payload: CreateOrderPayload) =>
      isPlatformAdmin
        ? ordersService.createBulk(payload)
        : ordersService.createMyOrder(payload),
    onSuccess: () => {
      toast.success(t("orders.created_success"));
      queryClient.invalidateQueries({ queryKey: ordersQueryKeys.all });
      onOpenChange(false);
      // reset
      setStep("product");
      setRecipients([
        {
          id: "1",
          name: "",
          title: "",
          department: "",
          email: "",
          phone: "",
          printName: "",
        },
      ]);
    },
    onError: (err: Error) => {
      toast.error(err?.message || t("orders.error_creating"));
    },
  });

  const handleAddRecipient = () => {
    setRecipients((prev) => [
      ...prev,
      {
        id: String(Date.now() + Math.random()),
        name: "",
        title: "",
        department: "",
        email: "",
        phone: "",
        printName: "",
      },
    ]);
  };

  const handleRemoveRecipient = (id: string) => {
    if (recipients.length <= 1) return;
    setRecipients((prev) => prev.filter((r) => r.id !== id));
  };

  const handleUpdateRecipient = (
    id: string,
    field: keyof RecipientRow,
    val: string,
  ) => {
    setRecipients((prev) =>
      prev.map((r) => (r.id === id ? { ...r, [field]: val } : r)),
    );
  };

  const handleParsePaste = () => {
    if (!pasteText.trim()) return;
    const lines = pasteText
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean);

    const parsed: RecipientRow[] = lines.map((line, idx) => {
      // Split by tab or comma
      const parts = line.includes("\t") ? line.split("\t") : line.split(",");
      const name = parts[0]?.trim() || "";
      const title = parts[1]?.trim() || "";
      const department = parts[2]?.trim() || "";
      const email = parts[3]?.trim() || "";
      const phone = parts[4]?.trim() || "";
      return {
        id: String(Date.now() + idx),
        name,
        title,
        department,
        email,
        phone,
        printName: name,
      };
    });

    if (parsed.length > 0) {
      setRecipients(parsed);
      setShowPasteMode(false);
      setPasteText("");
      toast.success(t("orders.bulk.people_imported", { count: parsed.length }));
    }
  };

  const handleSubmitOrder = async () => {
    const prodUuid = currentProduct?.uuid || activeProductUuid;
    if (!prodUuid) {
      toast.error(t("orders.bulk.select_product_error"));
      setStep("product");
      return;
    }

    const invalid = recipients.some((r) => !r.name.trim());
    if (invalid) {
      toast.error(t("orders.bulk.fill_recipients_error"));
      setStep("recipients");
      return;
    }

    if (
      !shippingFullName ||
      !shippingAddress ||
      !shippingCity ||
      !shippingPhone
    ) {
      toast.error(t("orders.bulk.fill_shipping_error"));
      setStep("shipping");
      return;
    }

    const items: CreateOrderItemPayload[] = recipients.map((r) => ({
      product_uuid: prodUuid,
      variant_uuid: selectedVariant?.uuid,
      quantity: 1,
      recipient_name: r.name,
      recipient_title: r.title,
      recipient_department: r.department,
      recipient_email: r.email,
      recipient_phone: r.phone,
      custom_print_data: {
        print_name: r.printName || r.name,
        title: r.title,
        department: r.department,
      },
    }));

    const payload: CreateOrderPayload = {
      order_type: "bulk",
      items,
      shipping_address: {
        full_name: shippingFullName,
        company_name: shippingCompanyName,
        phone: shippingPhone,
        address_line1: shippingAddress,
        city: shippingCity,
        state: shippingState,
        postal_code: shippingPostalCode,
        country: shippingCountry,
      },
      billing_address: {
        full_name: shippingFullName,
        company_name: shippingCompanyName,
        phone: shippingPhone,
        address_line1: shippingAddress,
        city: shippingCity,
        state: shippingState,
        postal_code: shippingPostalCode,
        country: shippingCountry,
      },
      customer_note: customerNote,
    };

    await createOrderMutation.mutateAsync(payload);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92vh] max-w-4xl overflow-y-auto">
        <DialogHeader className="border-b pb-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="bg-primary/10 text-primary rounded-lg p-2">
                <Users className="h-5 w-5" />
              </div>
              <div>
                <DialogTitle className="text-lg font-bold">
                  {t("orders.bulk.dialog_title")}
                </DialogTitle>
                <DialogDescription className="text-xs">
                  {t("orders.bulk.dialog_desc")}
                </DialogDescription>
              </div>
            </div>

            {/* Stepper indicator */}
            <div className="flex items-center gap-1.5 text-xs font-medium">
              <span
                className={`rounded-full px-2.5 py-1 ${
                  step === "product"
                    ? "bg-primary text-primary-foreground font-bold"
                    : "bg-muted text-muted-foreground"
                }`}
              >
                {t("orders.bulk.step_product")}
              </span>
              <ChevronRight className="text-muted-foreground h-3 w-3" />
              <span
                className={`rounded-full px-2.5 py-1 ${
                  step === "recipients"
                    ? "bg-primary text-primary-foreground font-bold"
                    : "bg-muted text-muted-foreground"
                }`}
              >
                {t("orders.bulk.step_recipients")} ({recipients.length})
              </span>
              <ChevronRight className="text-muted-foreground h-3 w-3" />
              <span
                className={`rounded-full px-2.5 py-1 ${
                  step === "shipping"
                    ? "bg-primary text-primary-foreground font-bold"
                    : "bg-muted text-muted-foreground"
                }`}
              >
                {t("orders.bulk.step_shipping")}
              </span>
            </div>
          </div>
        </DialogHeader>

        {/* STEP 1: PRODUCT & VARIANT SELECTION */}
        {step === "product" && (
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label className="text-xs font-semibold">
                {t("orders.bulk.product_selection_label")}
              </Label>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                {products.map((prod) => (
                  <div
                    key={prod.uuid}
                    onClick={() => {
                      setSelectedProductUuid(prod.uuid);
                      setSelectedVariantUuid(undefined);
                    }}
                    className={`cursor-pointer rounded-xl border p-3.5 text-left transition-all ${
                      selectedProductUuid === prod.uuid ||
                      (!selectedProductUuid && prod === products[0])
                        ? "border-primary bg-primary/5 ring-primary/20 shadow-xs ring-1"
                        : "border-border hover:border-muted-foreground/30 bg-card"
                    }`}
                  >
                    <div className="flex items-start justify-between">
                      <div>
                        <h4 className="text-foreground text-sm font-bold">
                          {prod.name}
                        </h4>
                        <p className="text-muted-foreground mt-0.5 line-clamp-1 text-xs">
                          {prod.short_description || prod.name}
                        </p>
                      </div>
                      <Badge variant="outline" className="font-mono text-xs">
                        {prod.base_price.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 2 },
                        )}{" "}
                        {prod.currency}
                      </Badge>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Variant Selector */}
            {activeVariants.length > 0 && (
              <div className="space-y-2 pt-2">
                <Label className="text-xs font-semibold">
                  {t("orders.bulk.variant_selection_label")}
                </Label>
                <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3 md:grid-cols-4">
                  {activeVariants.map((v) => (
                    <div
                      key={v.uuid}
                      onClick={() => setSelectedVariantUuid(v.uuid)}
                      className={`cursor-pointer rounded-lg border p-2.5 text-left text-xs transition-all ${
                        selectedVariant?.uuid === v.uuid
                          ? "border-primary bg-primary/10 font-bold"
                          : "border-border hover:bg-muted/40"
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span>{v.name}</span>
                        {v.price_delta !== 0 && (
                          <span className="text-muted-foreground font-mono text-[10px]">
                            {v.price_delta > 0
                              ? `+${v.price_delta}`
                              : v.price_delta}
                          </span>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Unit Price Summary */}
            <div className="bg-muted/40 flex items-center justify-between rounded-xl border p-3.5 text-xs">
              <div>
                <span className="text-muted-foreground">
                  {t("orders.bulk.unit_price_label")}
                </span>
                <span className="text-foreground ml-2 font-mono text-sm font-bold">
                  {unitPrice.toLocaleString(
                    locale === "tr" ? "tr-TR" : "en-US",
                    { minimumFractionDigits: 2 },
                  )}{" "}
                  {currency}
                </span>
              </div>
              <Button size="sm" onClick={() => setStep("recipients")}>
                {t("orders.bulk.proceed_to_recipients")} ({recipients.length}){" "}
                <ChevronRight className="ml-1 h-4 w-4" />
              </Button>
            </div>
          </div>
        )}

        {/* STEP 2: RECIPIENTS & MASS ENTRY */}
        {step === "recipients" && (
          <div className="space-y-4 py-2">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="flex items-center gap-2 text-sm font-bold">
                  <Users className="text-primary h-4 w-4" />{" "}
                  {t("orders.bulk.recipients_heading")}
                </h4>
                <p className="text-muted-foreground text-xs">
                  {t("orders.bulk.recipients_subtext")}
                </p>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setShowPasteMode(!showPasteMode)}
                  className="text-xs"
                >
                  <FileSpreadsheet className="mr-1.5 h-3.5 w-3.5" />
                  {showPasteMode
                    ? t("orders.bulk.switch_to_table")
                    : t("orders.bulk.switch_to_paste")}
                </Button>
                <Button
                  size="sm"
                  onClick={handleAddRecipient}
                  className="text-xs"
                >
                  <Plus className="mr-1 h-3.5 w-3.5" />{" "}
                  {t("orders.bulk.add_recipient_btn")}
                </Button>
              </div>
            </div>

            {/* Paste / Quick Import Box */}
            {showPasteMode ? (
              <div className="bg-card space-y-3 rounded-xl border p-4">
                <Label className="text-xs font-semibold">
                  {t("orders.bulk.paste_title")}
                </Label>
                <p className="text-muted-foreground text-[11px]">
                  {t("orders.bulk.paste_hint")}
                </p>
                <Textarea
                  rows={6}
                  placeholder={t("orders.bulk.paste_placeholder")}
                  value={pasteText}
                  onChange={(e) => setPasteText(e.target.value)}
                  className="font-mono text-xs"
                />
                <div className="flex justify-end gap-2">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setShowPasteMode(false)}
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button size="sm" onClick={handleParsePaste}>
                    {t("orders.bulk.import_list_btn")} (
                    {pasteText.split("\n").filter((l) => l.trim()).length})
                  </Button>
                </div>
              </div>
            ) : (
              /* Editable Recipients Data Table */
              <div className="max-h-[360px] overflow-x-auto rounded-xl border">
                <table className="w-full text-left text-xs">
                  <thead className="bg-muted/80 text-muted-foreground sticky top-0 z-10 font-mono text-[11px]">
                    <tr>
                      <th className="w-10 p-2.5 text-center">
                        {t("orders.bulk.table_col_index")}
                      </th>
                      <th className="min-w-[140px] p-2.5">
                        {t("orders.bulk.table_col_name")}
                      </th>
                      <th className="min-w-[120px] p-2.5">
                        {t("orders.bulk.table_col_title")}
                      </th>
                      <th className="min-w-[120px] p-2.5">
                        {t("orders.bulk.table_col_department")}
                      </th>
                      <th className="min-w-[140px] p-2.5">
                        {t("orders.bulk.table_col_email")}
                      </th>
                      <th className="min-w-[110px] p-2.5">
                        {t("orders.bulk.table_col_phone")}
                      </th>
                      <th className="w-10 p-2.5"></th>
                    </tr>
                  </thead>
                  <tbody className="divide-border divide-y">
                    {recipients.map((r, idx) => (
                      <tr key={r.id} className="hover:bg-muted/30">
                        <td className="text-muted-foreground p-2 text-center font-mono font-bold">
                          {idx + 1}
                        </td>
                        <td className="p-1.5">
                          <Input
                            placeholder="John Doe"
                            className="h-8 text-xs"
                            value={r.name}
                            onChange={(e) =>
                              handleUpdateRecipient(
                                r.id,
                                "name",
                                e.target.value,
                              )
                            }
                          />
                        </td>
                        <td className="p-1.5">
                          <Input
                            placeholder="CTO"
                            className="h-8 text-xs"
                            value={r.title}
                            onChange={(e) =>
                              handleUpdateRecipient(
                                r.id,
                                "title",
                                e.target.value,
                              )
                            }
                          />
                        </td>
                        <td className="p-1.5">
                          <Input
                            placeholder="Engineering"
                            className="h-8 text-xs"
                            value={r.department}
                            onChange={(e) =>
                              handleUpdateRecipient(
                                r.id,
                                "department",
                                e.target.value,
                              )
                            }
                          />
                        </td>
                        <td className="p-1.5">
                          <Input
                            placeholder="john@example.com"
                            className="h-8 font-mono text-xs"
                            value={r.email}
                            onChange={(e) =>
                              handleUpdateRecipient(
                                r.id,
                                "email",
                                e.target.value,
                              )
                            }
                          />
                        </td>
                        <td className="p-1.5">
                          <Input
                            placeholder="+123456789"
                            className="h-8 font-mono text-xs"
                            value={r.phone}
                            onChange={(e) =>
                              handleUpdateRecipient(
                                r.id,
                                "phone",
                                e.target.value,
                              )
                            }
                          />
                        </td>
                        <td className="p-1.5 text-center">
                          <Button
                            size="icon"
                            variant="ghost"
                            className="text-muted-foreground hover:text-destructive h-7 w-7"
                            onClick={() => handleRemoveRecipient(r.id)}
                            disabled={recipients.length <= 1}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Bottom summary bar */}
            <div className="bg-muted/40 flex items-center justify-between rounded-xl border p-3.5 text-xs">
              <div className="flex items-center gap-4">
                <span className="text-foreground font-semibold">
                  {t("orders.bulk.total_people_count")}{" "}
                  <Badge variant="secondary" className="font-mono">
                    {recipients.length}
                  </Badge>
                </span>
                <span className="text-foreground font-semibold">
                  {t("orders.bulk.total_amount_vat_incl")}{" "}
                  <span className="text-primary font-mono text-sm font-bold">
                    {grandTotal.toLocaleString(
                      locale === "tr" ? "tr-TR" : "en-US",
                      { minimumFractionDigits: 2 },
                    )}{" "}
                    {currency}
                  </span>
                </span>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setStep("product")}
                >
                  {t("orders.bulk.back_btn")}
                </Button>
                <Button size="sm" onClick={() => setStep("shipping")}>
                  {t("orders.bulk.proceed_to_shipping")}{" "}
                  <ChevronRight className="ml-1 h-4 w-4" />
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* STEP 3: SHIPPING & FINAL CONFIRMATION */}
        {step === "shipping" && (
          <div className="space-y-4 py-2">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-3">
                <h4 className="text-sm font-semibold">
                  {t("orders.bulk.shipping_heading")}
                </h4>

                <div className="space-y-1.5">
                  <Label className="text-xs">
                    {t("orders.bulk.recipient_contact_name")}
                  </Label>
                  <Input
                    placeholder="Full Name"
                    value={shippingFullName}
                    onChange={(e) => setShippingFullName(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs">
                    {t("orders.bulk.company_name_label")}
                  </Label>
                  <Input
                    placeholder="Company Inc."
                    value={shippingCompanyName}
                    onChange={(e) => setShippingCompanyName(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs">
                    {t("orders.bulk.phone_label")}
                  </Label>
                  <Input
                    placeholder="+123456789"
                    value={shippingPhone}
                    onChange={(e) => setShippingPhone(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs">
                    {t("orders.bulk.address_label")}
                  </Label>
                  <Textarea
                    rows={2}
                    placeholder="Street, Building, Unit..."
                    value={shippingAddress}
                    onChange={(e) => setShippingAddress(e.target.value)}
                    className="text-xs"
                  />
                </div>

                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <Label className="text-xs">
                      {t("orders.bulk.city_label")}
                    </Label>
                    <Input
                      placeholder="City"
                      value={shippingCity}
                      onChange={(e) => setShippingCity(e.target.value)}
                      className="h-8 text-xs"
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">
                      {t("orders.bulk.district_label")}
                    </Label>
                    <Input
                      placeholder="District / State"
                      value={shippingState}
                      onChange={(e) => setShippingState(e.target.value)}
                      className="h-8 text-xs"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-2">
                  <div className="space-y-1">
                    <Label className="text-xs">
                      {t("orders.subscriber.shipping_postal")}
                    </Label>
                    <Input
                      placeholder="34000"
                      value={shippingPostalCode}
                      onChange={(e) => setShippingPostalCode(e.target.value)}
                      className="h-8 text-xs"
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">
                      {t("orders.fields.currency")}
                    </Label>
                    <Input
                      placeholder="Türkiye"
                      value={shippingCountry}
                      onChange={(e) => setShippingCountry(e.target.value)}
                      className="h-8 text-xs"
                    />
                  </div>
                </div>
              </div>

              {/* Order Summary Confirmation Card */}
              <div className="space-y-3">
                <h4 className="text-sm font-semibold">
                  {t("orders.bulk.order_summary_heading")}
                </h4>
                <div className="bg-muted/40 space-y-3 rounded-xl border p-4 text-xs">
                  <div className="flex items-center justify-between border-b pb-2">
                    <span className="font-semibold">
                      {currentProduct?.name || "NFC Card"}
                    </span>
                    <Badge variant="secondary">
                      {selectedVariant?.name ||
                        t("orders.subscriber.default_variant")}
                    </Badge>
                  </div>

                  <div className="text-muted-foreground space-y-1.5">
                    <div className="flex justify-between">
                      <span>{t("orders.bulk.card_unit_price")}</span>
                      <span className="font-mono">
                        {unitPrice.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 2 },
                        )}{" "}
                        {currency}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>{t("orders.bulk.recipients_count_summary")}</span>
                      <span className="text-foreground font-mono font-bold">
                        {recipients.length}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>{t("orders.bulk.subtotal")}</span>
                      <span className="font-mono">
                        {subtotal.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 2 },
                        )}{" "}
                        {currency}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>
                        {t("orders.bulk.vat_amount", { rate: taxRate })}
                      </span>
                      <span className="font-mono">
                        {taxAmount.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 2 },
                        )}{" "}
                        {currency}
                      </span>
                    </div>
                    <div className="text-foreground flex justify-between border-t pt-2 text-sm font-bold">
                      <span>{t("orders.bulk.grand_total")}</span>
                      <span className="text-primary font-mono text-base">
                        {grandTotal.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 2 },
                        )}{" "}
                        {currency}
                      </span>
                    </div>
                  </div>

                  <div className="pt-2">
                    <Label className="text-xs">
                      {t("orders.bulk.order_note_label")}
                    </Label>
                    <Input
                      placeholder={t("orders.bulk.order_note_placeholder")}
                      value={customerNote}
                      onChange={(e) => setCustomerNote(e.target.value)}
                      className="mt-1 h-8 text-xs"
                    />
                  </div>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-between border-t pt-3">
              <Button
                size="sm"
                variant="outline"
                onClick={() => setStep("recipients")}
              >
                {t("orders.bulk.back_btn")}
              </Button>
              <Button
                size="sm"
                className="bg-primary hover:bg-primary/90 text-primary-foreground px-6 font-semibold"
                onClick={handleSubmitOrder}
                disabled={createOrderMutation.isPending}
              >
                {createOrderMutation.isPending
                  ? t("orders.bulk.submitting_order")
                  : t("orders.bulk.confirm_bulk_order_btn")}
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
