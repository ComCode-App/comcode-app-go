"use client";

import { useState, useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";

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
  categoriesService,
  type CategorySummary,
} from "../services/categories.service";
import { categoriesQueryKeys } from "../hooks/query-keys";
import { useCategoryMutations } from "../hooks/use-categories";
import { getCategoriesColumns } from "./categories-columns";
import { CategoryDialog } from "./category-dialog";

export function CategoriesPage() {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const { deleteMutation, toggleActiveMutation } = useCategoryMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingCategory, setEditingCategory] =
    useState<CategorySummary | null>(null);

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({ ...listState.params, q: listState.params.q }),
    [listState.params],
  );

  const listQuery = useQuery({
    queryKey: categoriesQueryKeys.list(listParams),
    queryFn: () => categoriesService.list(listParams),
  });

  const handleCreate = useCallback(() => {
    setEditingCategory(null);
    setDialogOpen(true);
  }, []);

  const handleEdit = useCallback((category: CategorySummary) => {
    setEditingCategory(category);
    setDialogOpen(true);
  }, []);

  const handleDelete = useCallback(
    async (uuid: string) => {
      const confirmed = await confirmDelete({
        title: t("categories.delete_confirm_title"),
        description: t("categories.delete_confirm_description"),
      });

      if (confirmed) {
        deleteMutation.mutate(uuid);
      }
    },
    [confirmDelete, deleteMutation, t],
  );

  const handleToggleActive = useCallback(
    (category: CategorySummary) => {
      toggleActiveMutation.mutate({
        uuid: category.uuid,
        payload: {
          name: category.name,
          slug: category.slug,
          description: category.description,
          icon: category.icon,
          image_url: category.image_url,
          display_order: category.display_order,
          is_active: !category.is_active,
        },
        isActive: !category.is_active,
      });
    },
    [toggleActiveMutation],
  );

  const columns = useMemo(
    () =>
      getCategoriesColumns({
        t,
        onEdit: handleEdit,
        onDelete: handleDelete,
        onToggleActive: handleToggleActive,
      }),
    [t, handleEdit, handleDelete, handleToggleActive],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("categories.title")}
      description={t("categories.description")}
      permission={permissions.platformCategoriesRead}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("categories.title") },
      ]}
      actions={
        <Button onClick={handleCreate} className="gap-2 shadow-xs">
          <Plus className="h-4 w-4" />
          <span>{t("categories.new_button")}</span>
        </Button>
      }
    >
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("categories.empty")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "platform-categories-v1",
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />

      <CategoryDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        category={editingCategory}
      />
    </EntityPage>
  );
}
