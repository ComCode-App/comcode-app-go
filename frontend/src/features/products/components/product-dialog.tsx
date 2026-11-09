"use client";

import { useEffect, useState } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import {
  Sparkles,
  Globe,
  Search,
  Plus,
  Trash2,
  Box,
  Layers,
} from "lucide-react";
import { useLocale } from "@/providers/locale-provider";

import {
  productFormSchema,
  type ProductFormValues,
} from "../schemas/product-form";
import type {
  ProductDetail,
  ProductSummary,
  ProductVariantSummary,
  UpsertProductPayload,
} from "../services/products.service";
import { useProductMutations } from "../hooks/use-products";
import { categoriesService } from "@/features/categories/services/categories.service";
import { categoriesQueryKeys } from "@/features/categories/hooks/query-keys";
import { ViewboxStudioModal } from "./ViewboxStudioModal";
import { Bounded3DStage } from "./Bounded3DStage";

interface ProductDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  product: ProductSummary | ProductDetail | null;
}

export function ProductDialog({
  open,
  onOpenChange,
  product,
}: ProductDialogProps) {
  const { t } = useLocale();
  const {
    createMutation,
    updateMutation,
    createVariantMutation,
    deleteVariantMutation,
  } = useProductMutations();
  const isEditing = Boolean(product);
  const [studioOpen, setStudioOpen] = useState(false);
  const [currentViewbox, setCurrentViewbox] = useState<
    UpsertProductPayload["model_3d_viewbox"]
  >((product as ProductDetail)?.model_3d_viewbox || undefined);

  const [newVariantName, setNewVariantName] = useState("");
  const [newVariantColor, setNewVariantColor] = useState("#000000");
  const [newVariantMaterial] = useState("matte_pvc");
  const [newVariantDelta, setNewVariantDelta] = useState(0);
  const [newVariantStock] = useState(50);

  const categoriesQuery = useQuery({
    queryKey: categoriesQueryKeys.active(),
    queryFn: () => categoriesService.listActive(),
  });

  const {
    register,
    handleSubmit,
    control,
    reset,
    watch,
    formState: { errors },
  } = useForm<ProductFormValues>({
    resolver: zodResolver(productFormSchema),
    defaultValues: {
      name: "",
      slug: "",
      category_uuid: null,
      short_description: "",
      description: "",
      status: "active",
      product_type: "physical_card",
      base_price: 0,
      currency: "TRY",
      tax_rate: 20,
      stock_quantity: 0,
      sku: "",
      allow_custom_print: true,
      model_3d_file_url: "",
      model_3d_usdz_url: "",
      images: [],
      meta_title: "",
      meta_description: "",
      meta_keywords: "",
      canonical_url: "",
      og_image_url: "",
      display_order: 0,
    },
  });

  const metaTitleWatch =
    watch("meta_title") ||
    watch("name") ||
    t("products.placeholders.meta_title");
  const metaDescWatch =
    watch("meta_description") ||
    watch("short_description") ||
    t("products.placeholders.meta_description");
  const slugWatch = watch("slug") || "product-slug";

  useEffect(() => {
    if (product) {
      reset({
        name: product.name,
        slug: product.slug,
        category_uuid: (product as ProductDetail).category_uuid ?? null,
        short_description: product.short_description || "",
        description: (product as ProductDetail).description || "",
        status: (product.status as ProductFormValues["status"]) || "active",
        product_type: product.product_type || "physical_card",
        base_price: product.base_price,
        currency: product.currency || "TRY",
        tax_rate: (product as ProductDetail).tax_rate ?? 20,
        stock_quantity: product.stock_quantity ?? 0,
        sku: product.sku || "",
        allow_custom_print: product.allow_custom_print ?? true,
        model_3d_file_url: product.model_3d_file_url || "",
        model_3d_usdz_url: product.model_3d_usdz_url || "",
        images: product.images || [],
        meta_title: (product as ProductDetail).meta_title || "",
        meta_description: (product as ProductDetail).meta_description || "",
        meta_keywords: (product as ProductDetail).meta_keywords || "",
        canonical_url: (product as ProductDetail).canonical_url || "",
        og_image_url: (product as ProductDetail).og_image_url || "",
        display_order: product.display_order ?? 0,
      });
      setCurrentViewbox((product as ProductDetail).model_3d_viewbox);
    } else {
      reset({
        name: "",
        slug: "",
        category_uuid: null,
        short_description: "",
        description: "",
        status: "active",
        product_type: "physical_card",
        base_price: 990,
        currency: "TRY",
        tax_rate: 20,
        stock_quantity: 100,
        sku: "",
        allow_custom_print: true,
        model_3d_file_url: "",
        model_3d_usdz_url: "",
        images: [],
        meta_title: "",
        meta_description: "",
        meta_keywords: "",
        canonical_url: "",
        og_image_url: "",
        display_order: 0,
      });
      setCurrentViewbox(undefined);
    }
  }, [product, reset]);

  const isPending = createMutation.isPending || updateMutation.isPending;

  const onSubmit = async (values: ProductFormValues) => {
    const payload = {
      ...values,
      category_uuid: values.category_uuid || undefined,
      model_3d_viewbox: currentViewbox,
    };

    if (isEditing && product) {
      await updateMutation.mutateAsync({
        uuid: product.uuid,
        payload: payload as UpsertProductPayload,
      });
    } else {
      await createMutation.mutateAsync(payload as UpsertProductPayload);
    }
    onOpenChange(false);
  };

  const handleAddVariant = async () => {
    if (!newVariantName || !product) return;
    await createVariantMutation.mutateAsync({
      productUuid: product.uuid,
      payload: {
        name: newVariantName,
        material: newVariantMaterial,
        color_code: newVariantColor,
        price_delta: Number(newVariantDelta),
        stock_quantity: Number(newVariantStock),
      },
    });
    setNewVariantName("");
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Box className="text-primary h-5 w-5" />
              {isEditing
                ? t("products.edit_title")
                : t("products.create_title")}
            </DialogTitle>
            <DialogDescription>
              {isEditing
                ? t("products.edit_description")
                : t("products.create_description")}
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <Tabs defaultValue="general" className="w-full">
              <TabsList className="bg-muted mb-4 grid grid-cols-4">
                <TabsTrigger value="general">
                  {t("products.tabs.general")}
                </TabsTrigger>
                <TabsTrigger value="pricing">
                  {t("products.columns.price")} & {t("products.columns.stock")}
                </TabsTrigger>
                <TabsTrigger value="seo" className="flex items-center gap-1">
                  <Globe className="h-3.5 w-3.5" /> {t("products.tabs.seo")}
                </TabsTrigger>
                <TabsTrigger
                  value="3d_studio"
                  className="flex items-center gap-1"
                >
                  <Sparkles className="h-3.5 w-3.5 text-amber-500" />{" "}
                  {t("products.tabs.viewbox")}
                </TabsTrigger>
              </TabsList>

              {/* TAB 1: GENERAL */}
              <TabsContent value="general" className="space-y-4">
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t("products.fields.name")}</Label>
                    <Input
                      placeholder={t("products.placeholders.name")}
                      {...register("name")}
                    />
                    {errors.name && (
                      <p className="text-destructive text-xs">
                        {errors.name.message}
                      </p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label>{t("products.fields.category")}</Label>
                    <Controller
                      name="category_uuid"
                      control={control}
                      render={({ field }) => (
                        <Select
                          value={field.value || "none"}
                          onValueChange={(val) =>
                            field.onChange(val === "none" ? null : val)
                          }
                        >
                          <SelectTrigger>
                            <SelectValue
                              placeholder={t("products.placeholders.category")}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="none">
                              {t("products.placeholders.no_category")}
                            </SelectItem>
                            {(Array.isArray(categoriesQuery.data)
                              ? categoriesQuery.data
                              : []
                            ).map((cat, idx) => (
                              <SelectItem
                                key={cat.uuid || cat.slug || `cat-${idx}`}
                                value={cat.uuid || `cat-${idx}`}
                              >
                                {cat.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t("products.fields.slug")}</Label>
                    <Input
                      placeholder="comcode-metal-nfc-card"
                      {...register("slug")}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label>{t("products.fields.status")}</Label>
                    <Controller
                      name="status"
                      control={control}
                      render={({ field }) => (
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="active">
                              {t("products.status_active")}
                            </SelectItem>
                            <SelectItem value="draft">
                              {t("products.status_draft")}
                            </SelectItem>
                            <SelectItem value="archived">
                              {t("products.status_archived")}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      )}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>{t("products.fields.short_description")}</Label>
                  <Input
                    placeholder={t("products.placeholders.short_description")}
                    {...register("short_description")}
                  />
                </div>

                <div className="space-y-2">
                  <Label>{t("products.fields.description")}</Label>
                  <Textarea
                    placeholder={t("products.placeholders.description")}
                    rows={3}
                    {...register("description")}
                  />
                </div>
              </TabsContent>

              {/* TAB 2: PRICING & STOCK */}
              <TabsContent value="pricing" className="space-y-4">
                <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
                  <div className="space-y-2">
                    <Label>{t("products.fields.price")}</Label>
                    <Input
                      type="number"
                      step="0.01"
                      {...register("base_price")}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label>{t("products.fields.currency")}</Label>
                    <Controller
                      name="currency"
                      control={control}
                      render={({ field }) => (
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="TRY">TRY (₺)</SelectItem>
                            <SelectItem value="USD">USD ($)</SelectItem>
                            <SelectItem value="EUR">EUR (€)</SelectItem>
                          </SelectContent>
                        </Select>
                      )}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label>{t("products.fields.tax_rate")}</Label>
                    <Input type="number" {...register("tax_rate")} />
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t("products.fields.stock")}</Label>
                    <Input type="number" {...register("stock_quantity")} />
                  </div>

                  <div className="space-y-2">
                    <Label>{t("products.fields.sku")}</Label>
                    <Input placeholder="CC-CRD-001" {...register("sku")} />
                  </div>
                </div>

                <div className="flex flex-row items-center justify-between rounded-lg border p-3">
                  <div className="space-y-0.5">
                    <Label className="text-sm">
                      {t("products.fields.allow_custom_print")}
                    </Label>
                    <p className="text-muted-foreground text-xs">
                      {t("products.fields.allow_custom_print_sub")}
                    </p>
                  </div>
                  <Controller
                    name="allow_custom_print"
                    control={control}
                    render={({ field }) => (
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    )}
                  />
                </div>
              </TabsContent>

              {/* TAB 3: SEO & METADATA */}
              <TabsContent value="seo" className="space-y-4">
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div className="space-y-3">
                    <div className="space-y-2">
                      <Label>{t("products.seo.meta_title")}</Label>
                      <Input
                        placeholder={t("products.placeholders.meta_title")}
                        {...register("meta_title")}
                      />
                    </div>

                    <div className="space-y-2">
                      <Label>{t("products.seo.meta_description")}</Label>
                      <Textarea
                        placeholder={t(
                          "products.placeholders.meta_description",
                        )}
                        rows={3}
                        {...register("meta_description")}
                      />
                    </div>

                    <div className="space-y-2">
                      <Label>{t("products.seo.meta_keywords")}</Label>
                      <Input
                        placeholder="nfc card, smart business card, digital profile"
                        {...register("meta_keywords")}
                      />
                    </div>
                  </div>

                  {/* Live Google & Social Snippet Preview */}
                  <div className="space-y-3">
                    <Label className="text-muted-foreground flex items-center gap-1.5 text-xs font-semibold">
                      <Search className="h-3.5 w-3.5" />{" "}
                      {t("products.seo.preview_google")}
                    </Label>
                    <div className="bg-card/60 space-y-1 rounded-xl border p-3.5 text-left shadow-xs">
                      <div className="text-muted-foreground flex items-center gap-1 font-mono text-[11px]">
                        <Globe className="h-3 w-3 text-emerald-500" />
                        https://comcode.app/products/{slugWatch}
                      </div>
                      <div className="line-clamp-1 cursor-pointer text-sm font-medium text-blue-600 hover:underline dark:text-blue-400">
                        {metaTitleWatch}
                      </div>
                      <div className="text-muted-foreground line-clamp-2 text-xs">
                        {metaDescWatch}
                      </div>
                    </div>

                    <Label className="text-muted-foreground flex items-center gap-1.5 pt-1 text-xs font-semibold">
                      <Layers className="h-3.5 w-3.5" />{" "}
                      {t("products.seo.preview_og")}
                    </Label>
                    <div className="bg-card/80 overflow-hidden rounded-xl border text-left">
                      <div className="flex h-24 items-center justify-center bg-gradient-to-r from-zinc-900 to-zinc-800 font-mono text-xs text-zinc-400">
                        [OG Image Preview]
                      </div>
                      <div className="p-2.5">
                        <div className="text-muted-foreground font-mono text-[10px] tracking-wider uppercase">
                          comcode.app
                        </div>
                        <div className="text-foreground line-clamp-1 text-xs font-semibold">
                          {metaTitleWatch}
                        </div>
                        <div className="text-muted-foreground line-clamp-1 text-[11px]">
                          {metaDescWatch}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </TabsContent>

              {/* TAB 4: 3D STUDIO & VARIANTS */}
              <TabsContent value="3d_studio" className="space-y-4">
                {isEditing && product ? (
                  <div className="grid grid-cols-1 gap-4 md:grid-cols-12">
                    {/* 3D Preview Box */}
                    <div className="space-y-2 md:col-span-6">
                      <div className="flex items-center justify-between">
                        <Label className="text-xs font-semibold">
                          {t("products.stage.bounded_preview")}
                        </Label>
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          className="h-7 border-amber-500/40 text-xs text-amber-500 hover:bg-amber-500/10"
                          onClick={() => setStudioOpen(true)}
                        >
                          <Sparkles className="mr-1 h-3.5 w-3.5" />{" "}
                          {t("products.stage.open_studio")}
                        </Button>
                      </div>
                      <Bounded3DStage
                        modelUrl={product.model_3d_file_url}
                        usdzUrl={product.model_3d_usdz_url}
                        viewbox={currentViewbox}
                        className="h-56 w-full rounded-xl"
                      />
                    </div>

                    {/* Variants Management */}
                    <div className="space-y-3 md:col-span-6">
                      <Label className="text-xs font-semibold">
                        {t("products.variants.title")}
                      </Label>

                      {/* Existing variants */}
                      <div className="max-h-40 space-y-2 overflow-y-auto pr-1">
                        {((product as ProductDetail).variants || []).map(
                          (v: ProductVariantSummary) => (
                            <div
                              key={v.uuid}
                              className="bg-muted/40 flex items-center justify-between rounded-lg border p-2 text-xs"
                            >
                              <div className="flex items-center gap-2">
                                <span
                                  className="h-4 w-4 rounded-full border shadow-xs"
                                  style={{
                                    backgroundColor: v.color_code || "#000",
                                  }}
                                />
                                <span className="font-medium">{v.name}</span>
                                <Badge
                                  variant="outline"
                                  className="text-[10px]"
                                >
                                  {v.material}
                                </Badge>
                              </div>
                              <div className="flex items-center gap-2 font-mono">
                                <span>+{v.price_delta}</span>
                                <span className="text-muted-foreground">
                                  {v.stock_quantity}
                                </span>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  className="text-destructive h-6 w-6"
                                  onClick={() =>
                                    deleteVariantMutation.mutate({
                                      productUuid: product.uuid,
                                      variantUuid: v.uuid,
                                    })
                                  }
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              </div>
                            </div>
                          ),
                        )}
                      </div>

                      {/* Add new variant quick inline */}
                      <div className="bg-card/50 space-y-2 rounded-lg border border-dashed p-2.5 text-xs">
                        <span className="text-foreground font-medium">
                          {t("products.variants.title")}
                        </span>
                        <div className="grid grid-cols-2 gap-2">
                          <Input
                            placeholder={t(
                              "products.placeholders.variant_name",
                            )}
                            className="h-7 text-xs"
                            value={newVariantName}
                            onChange={(e) => setNewVariantName(e.target.value)}
                          />
                          <div className="flex items-center gap-1.5">
                            <input
                              type="color"
                              className="h-7 w-8 cursor-pointer rounded border p-0.5"
                              value={newVariantColor}
                              onChange={(e) =>
                                setNewVariantColor(e.target.value)
                              }
                            />
                            <Input
                              placeholder={t(
                                "products.placeholders.price_delta",
                              )}
                              type="number"
                              className="h-7 text-xs"
                              value={newVariantDelta}
                              onChange={(e) =>
                                setNewVariantDelta(Number(e.target.value))
                              }
                            />
                          </div>
                        </div>
                        <Button
                          type="button"
                          size="sm"
                          variant="secondary"
                          className="h-7 w-full text-xs"
                          onClick={handleAddVariant}
                          disabled={
                            !newVariantName || createVariantMutation.isPending
                          }
                        >
                          <Plus className="mr-1 h-3.5 w-3.5" />{" "}
                          {t("products.variants.save_btn")}
                        </Button>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="text-muted-foreground space-y-1 rounded-xl border border-dashed py-8 text-center text-xs">
                    <Sparkles className="mx-auto mb-1 h-6 w-6 text-amber-500" />
                    <p className="text-foreground font-medium">
                      {t("products.variants.hint_heading")}
                    </p>
                    <p>{t("products.variants.hint_desc")}</p>
                  </div>
                )}
              </TabsContent>
            </Tabs>

            <DialogFooter className="pt-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
                disabled={isPending}
              >
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={isPending}>
                {isEditing ? t("common.save") : t("common.create")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* 3D Studio Modal */}
      {isEditing && product && (
        <ViewboxStudioModal
          open={studioOpen}
          onOpenChange={setStudioOpen}
          productUuid={product.uuid}
          productName={product.name}
          initialViewbox={currentViewbox}
          initialModelUrl={product.model_3d_file_url}
          initialUsdzUrl={product.model_3d_usdz_url}
          onSaved={(newVb) => setCurrentViewbox(newVb)}
        />
      )}
    </>
  );
}
