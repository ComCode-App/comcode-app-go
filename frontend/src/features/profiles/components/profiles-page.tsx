"use client";

import { useRouter } from "next/navigation";
import { useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityCreateButton,
  EntityPage,
  EntityTable,
  useServerListState,
} from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useProfilesColumns } from "@/features/profiles/components/profiles-columns";
import type { ProfileRowActionHandlers } from "@/features/profiles/components/profile-row-actions";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  useDeleteProfile,
  useSetPrimaryProfile,
} from "@/features/profiles/hooks/use-profile-mutations";
import {
  profilesService,
  type ProfileSummary,
} from "@/features/profiles/services/profiles.service";
import { useDialogs } from "@/providers/dialog-provider";
import { QuotaNotice, useQuotaReached } from "@/features/entitlements";
import { useLocale } from "@/providers/locale-provider";

function firstString(value: string | string[] | undefined) {
  if (Array.isArray(value)) return value[0];
  return value;
}

export function ProfilesPage() {
  const { t } = useLocale();
  const profileQuotaReached = useQuotaReached("profiles.max");
  const router = useRouter();
  const { confirmDelete } = useDialogs();
  const deleteProfile = useDeleteProfile();
  const setPrimary = useSetPrimaryProfile();

  const listState = useServerListState({ initialPageSize: 20 });

  const listParams = useMemo(() => {
    const status = firstString(
      listState.columnFilters.find((filter) => filter.id === "status")
        ?.value as string | string[] | undefined,
    );
    return {
      ...listState.params,
      q: listState.params.q,
      status,
    };
  }, [listState.columnFilters, listState.params]);

  const listQuery = useQuery({
    queryKey: profilesKeys.list(listParams),
    queryFn: () => profilesService.list(listParams),
  });

  const handleView = useCallback(
    (profile: ProfileSummary) =>
      router.push(routes.app.profiles.detail(profile.uuid)),
    [router],
  );

  const handleEdit = useCallback(
    (profile: ProfileSummary) =>
      router.push(routes.app.profiles.edit(profile.uuid)),
    [router],
  );

  const handleDownloadVCard = useCallback(
    async (profile: ProfileSummary) => {
      const name = `${profile.first_name}_${profile.last_name}`.trim() || "contact";
      await profilesService.downloadVCard(profile.uuid, name);
    },
    [],
  );

  const handleSetPrimary = useCallback(
    async (profile: ProfileSummary) => {
      await setPrimary.mutateAsync(profile.uuid);
    },
    [setPrimary],
  );

  const handleDelete = useCallback(
    async (profile: ProfileSummary) => {
      const fullName = `${profile.first_name} ${profile.last_name}`.trim() || "Profile";
      const confirmed = await confirmDelete({
        title: t("profiles.delete_title"),
        description: t("profiles.delete_description", {
          name: fullName,
        }),
      });
      if (!confirmed) return;
      await deleteProfile.mutateAsync(profile.uuid);
    },
    [confirmDelete, deleteProfile, t],
  );

  const rowHandlers = useMemo<ProfileRowActionHandlers>(
    () => ({
      onView: handleView,
      onEdit: handleEdit,
      onDownloadVCard: (p) => void handleDownloadVCard(p),
      onSetPrimary: (p) => void handleSetPrimary(p),
      onDelete: (p) => void handleDelete(p),
    }),
    [handleDelete, handleDownloadVCard, handleEdit, handleSetPrimary, handleView],
  );

  const columns = useProfilesColumns(rowHandlers);

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("profiles.title")}
      description={t("profiles.subtitle")}
      permission={permissions.profiles.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("profiles.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("profiles.title") },
      ]}
      actions={
        <EntityCreateButton
          onClick={() => router.push(routes.app.profiles.create)}
          label={t("profiles.actions.create")}
          permission={permissions.profiles.write}
          disabled={profileQuotaReached}
        />
      }
    >
      <QuotaNotice feature="profiles.max" />
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        onRowClick={handleView}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("profiles.empty_title")}
        emptyDescription={t("profiles.empty_description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "app-profiles-v1",
          rowSelection: false,
        }}
      />
    </EntityPage>
  );
}
