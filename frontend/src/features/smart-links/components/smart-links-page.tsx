"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { EmptyState } from "@/components/common/empty-state";
import { Loading } from "@/components/common/loading";
import { EntityCreateButton, EntityPage } from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { QuotaNotice, useQuotaReached } from "@/features/entitlements";
import { entitlementKeys } from "@/features/entitlements/hooks/use-entitlements";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { RouteQrDialog } from "@/features/route-qr";
import { SmartLinkCard } from "@/features/smart-links/components/smart-link-card";
import { SmartLinkDialog } from "@/features/smart-links/components/smart-link-dialog";
import {
  type SmartLink,
  smartLinkKeys,
  smartLinksService,
  smartLinkUrl,
} from "@/features/smart-links/services/smart-links.service";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

/** Customer smart links (TEC-556): /s/{slug} short links to a profile, its vCard or a URL. */
export function SmartLinksPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const canEdit = canManageAccount(user);
  const { confirmDelete } = useDialogs();
  const queryClient = useQueryClient();
  const quotaReached = useQuotaReached("smart_links.max");

  const linksQuery = useQuery({
    queryKey: smartLinkKeys.list,
    queryFn: smartLinksService.list,
  });
  const profilesQuery = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
  });
  const profileOf = (uuid?: string) =>
    uuid ? profilesQuery.data?.items.find((x) => x.uuid === uuid) : undefined;

  const [editing, setEditing] = useState<SmartLink | undefined>();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [qrLink, setQrLink] = useState<SmartLink | null>(null);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: smartLinkKeys.list });
    void queryClient.invalidateQueries({ queryKey: entitlementKeys.overview });
  };
  const toggle = useMutation({
    mutationFn: smartLinksService.toggleLock,
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: smartLinksService.remove,
    onSuccess: () => {
      refresh();
      appToast.success(t("smart_links.toast.deleted"));
    },
  });

  const links = linksQuery.data ?? [];

  return (
    <EntityPage
      title={t("smart_links.title")}
      description={t("smart_links.subtitle")}
      permission={permissions.routes.appRead}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("smart_links.title") },
      ]}
      actions={
        <EntityCreateButton
          label={t("smart_links.create")}
          permission={permissions.routes.appWrite}
          disabled={!canEdit || quotaReached}
          onClick={() => {
            setEditing(undefined);
            setDialogOpen(true);
          }}
        />
      }
    >
      <QuotaNotice feature="smart_links.max" />

      {linksQuery.isLoading ? <Loading label={t("common.loading")} /> : null}
      {!linksQuery.isLoading && links.length === 0 ? (
        <EmptyState
          title={t("smart_links.empty_title")}
          description={t("smart_links.empty_body")}
        />
      ) : null}

      <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {links.map((link) => {
          const url = smartLinkUrl(link.path);
          return (
            <SmartLinkCard
              key={link.uuid}
              link={link}
              url={url}
              profile={profileOf(link.profile_uuid ?? undefined)}
              canEdit={canEdit}
              busy={toggle.isPending || remove.isPending}
              onCopy={() =>
                void navigator.clipboard
                  .writeText(url)
                  .then(() => appToast.success(t("smart_links.copied")))
              }
              onQr={() => setQrLink(link)}
              onEdit={() => {
                setEditing(link);
                setDialogOpen(true);
              }}
              onToggleLock={() => toggle.mutate(link.uuid)}
              onDelete={() =>
                void confirmDelete({
                  title: t("smart_links.delete_title"),
                  description: t("smart_links.delete_body", {
                    path: `/${link.path}`,
                  }),
                  confirmLabel: t("smart_links.delete"),
                }).then((ok) => ok && remove.mutate(link.uuid))
              }
            />
          );
        })}
      </ul>

      <SmartLinkDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        link={editing}
      />

      <RouteQrDialog
        routeUuid={qrLink?.uuid}
        title={qrLink ? `/${qrLink.path}` : ""}
        open={Boolean(qrLink)}
        onOpenChange={(o) => !o && setQrLink(null)}
        suggestedLogoProfileUuid={qrLink?.profile_uuid ?? undefined}
        fileStem={qrLink?.path.replace(/\//g, "-")}
      />
    </EntityPage>
  );
}
