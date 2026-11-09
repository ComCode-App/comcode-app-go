"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus, Layers } from "lucide-react";
import Link from "next/link";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityPage,
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

import {
  productsService,
  type ProductSummary,
  type ProductDetail,
} from "../services/products.service";
import { productsQueryKeys } from "../hooks/query-keys";
import { useProductMutations } from "../hooks/use-products";
import { getProductsColumns } from "./products-columns";
import { ProductDialog } from "./product-dialog";
import { ViewboxStudioModal } from "./ViewboxStudioModal";

export function ProductsPage() {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const { deleteMutation } = useProductMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState<ProductSummary | ProductDetail | null>(null);
  const [studioProduct, setStudioProduct] = useState<ProductSummary | null>(null);

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({ ...listState.params, q: listState.params.q }),
    [listState.params],
  );

  const listQuery = useQuery({
    queryKey: productsQueryKeys.list(listParams),
    queryFn: () => productsService.list(listParams),
  });

  const handleCreate = useCallback(() => {
    setEditingProduct(null);
    setDialogOpen(true);
  }, []);

  const handleEdit = useCallback(async (product: ProductSummary) => {
    try {
      const detail = await productsService.get(product.uuid);
      setEditingProduct(detail);
    } catch {
      setEditingProduct(product);
    }
    setDialogOpen(true);
  }, []);

  // Cmd+K hits link to /platform/products?open={uuid} (TEC-595).
  useEffect(() => {
    const url = new URL(window.location.href);
    const uuid = url.searchParams.get("open");
    if (!uuid) return;
    url.searchParams.delete("open");
    window.history.replaceState(null, "", url.pathname + url.search);
    productsService
      .get(uuid)
      .then((detail) => {
        setEditingProduct(detail);
        setDialogOpen(true);
      })
      .catch(() => {});
  }, []);

  const handleOpenStudio = useCallback((product: ProductSummary) => {
    setStudioProduct(product);
  }, []);

  const handleDelete = useCallback(
    async (uuid: string) => {
      const confirmed = await confirmDelete({
        title: t("products.delete_confirm_title"),
        description: t("products.delete_confirm_description"),
      });

      if (confirmed) {
        deleteMutation.mutate(uuid);
      }
    },
    [confirmDelete, deleteMutation, t],
  );

  const columns = useMemo(
    () =>
      getProductsColumns({
        t,
        onEdit: handleEdit,
        onDelete: handleDelete,
        onOpenStudio: handleOpenStudio,
      }),
    [t, handleEdit, handleDelete, handleOpenStudio],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("products.title")}
      description={t("products.description")}
      permission={permissions.platformProductsRead}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("products.title") },
      ]}
      actions={
        <div className="flex items-center gap-2">
          <Button asChild variant="outline">
            <Link href={routes.platform.categories}>
              <Layers className="mr-2 h-4 w-4" />
              {t("categories.title")}
            </Link>
          </Button>
          <Button onClick={handleCreate} className="gap-2 shadow-xs">
            <Plus className="h-4 w-4" />
            <span>{t("products.new_button")}</span>
          </Button>
        </div>
      }
    >
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("products.empty")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "platform-products-v1",
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />

      <ProductDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        product={editingProduct}
      />

      {studioProduct && (
        <ViewboxStudioModal
          open={Boolean(studioProduct)}
          onOpenChange={(open) => !open && setStudioProduct(null)}
          productUuid={studioProduct.uuid}
          productName={studioProduct.name}
          initialModelUrl={studioProduct.model_3d_file_url}
          initialUsdzUrl={studioProduct.model_3d_usdz_url}
          onSaved={() => listQuery.refetch()}
        />
      )}
    </EntityPage>
  );
}
