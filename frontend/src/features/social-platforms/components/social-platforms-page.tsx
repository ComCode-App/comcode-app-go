"use client";

import { useState, useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Plus, Sparkles } from "lucide-react";

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
  socialPlatformsService,
  type SocialPlatformSummary,
} from "../services/social-platforms.service";
import { socialPlatformsKeys } from "../hooks/query-keys";
import { useSocialPlatformMutations } from "../hooks/use-social-platforms";
import { getSocialPlatformsColumns } from "./social-platforms-columns";
import { SocialPlatformDialog } from "./social-platform-dialog";

export function SocialPlatformsPage() {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const {
    deleteMutation,
    updateMutation,
    fetchIconMutation,
    bulkFetchIconsMutation,
  } = useSocialPlatformMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingPlatform, setEditingPlatform] =
    useState<SocialPlatformSummary | null>(null);

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({ ...listState.params, q: listState.params.q }),
    [listState.params],
  );

  const listQuery = useQuery({
    queryKey: socialPlatformsKeys.list(listParams),
    queryFn: () => socialPlatformsService.list(listParams),
  });

  const handleCreate = useCallback(() => {
    setEditingPlatform(null);
    setDialogOpen(true);
  }, []);

  const handleEdit = useCallback((platform: SocialPlatformSummary) => {
    setEditingPlatform(platform);
    setDialogOpen(true);
  }, []);

  const handleFetchIcon = useCallback(
    async (platform: SocialPlatformSummary) => {
      await fetchIconMutation.mutateAsync(platform.uuid);
    },
    [fetchIconMutation],
  );

  const handleBulkFetchIcons = useCallback(async () => {
    await bulkFetchIconsMutation.mutateAsync(undefined);
  }, [bulkFetchIconsMutation]);

  const handleDelete = useCallback(
    async (uuid: string) => {
      const confirmed = await confirmDelete({
        title: t("social_platforms.delete_title"),
        description: t("social_platforms.delete_description"),
      });
      if (!confirmed) return;
      await deleteMutation.mutateAsync(uuid);
    },
    [confirmDelete, deleteMutation, t],
  );

  const handleToggleActive = useCallback(
    async (platform: SocialPlatformSummary, isActive: boolean) => {
      await updateMutation.mutateAsync({
        uuid: platform.uuid,
        body: {
          name: platform.name,
          slug: platform.slug,
          category: platform.category,
          url_pattern: platform.url_pattern,
          placeholder: platform.placeholder,
          description: platform.description,
          display_order: platform.display_order,
          is_active: isActive,
        },
      });
    },
    [updateMutation],
  );

  const columns = useMemo(
    () =>
      getSocialPlatformsColumns({
        t,
        onEdit: handleEdit,
        onFetchIcon: handleFetchIcon,
        onDelete: handleDelete,
        onToggleActive: handleToggleActive,
      }),
    [t, handleEdit, handleFetchIcon, handleDelete, handleToggleActive],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("layout.nav_social_platforms")}
      description={t("social_platforms.subtitle")}
      permission={permissions.socialPlatforms.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("layout.nav_social_platforms") },
      ]}
      actions={
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={handleBulkFetchIcons}
            disabled={bulkFetchIconsMutation.isPending}
            className="gap-2"
          >
            {bulkFetchIconsMutation.isPending ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Sparkles className="w-4 h-4 text-primary" />
            )}
            <span>{t("social_platforms.actions.bulk_fetch_og")}</span>
          </Button>

          <Button onClick={handleCreate} className="gap-2">
            <Plus className="w-4 h-4" />
            <span>{t("social_platforms.actions.add")}</span>
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
        emptyTitle={t("social_platforms.empty_title")}
        emptyDescription={t("social_platforms.empty_description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "platform-social-platforms-v1",
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />

      <SocialPlatformDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        platform={editingPlatform}
      />
    </EntityPage>
  );
}
