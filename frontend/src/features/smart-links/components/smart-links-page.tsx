"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, Lock, LockOpen, Pencil, QrCode, Trash2 } from "lucide-react";
import { useState } from "react";

import { EmptyState } from "@/components/common/empty-state";
import { Loading } from "@/components/common/loading";
import { EntityCreateButton, EntityPage } from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { QuotaNotice, useQuotaReached } from "@/features/entitlements";
import { entitlementKeys } from "@/features/entitlements/hooks/use-entitlements";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { RouteQrDialog } from "@/features/route-qr";
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

  const linksQuery = useQuery({ queryKey: smartLinkKeys.list, queryFn: smartLinksService.list });
  const profilesQuery = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
  });
  const profileName = (uuid?: string) => {
    const p = profilesQuery.data?.items.find((x) => x.uuid === uuid);
    return p ? `${p.first_name} ${p.last_name}`.trim() : "";
  };

  const [editing, setEditing] = useState<SmartLink | undefined>();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [qrLink, setQrLink] = useState<SmartLink | null>(null);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: smartLinkKeys.list });
    void queryClient.invalidateQueries({ queryKey: entitlementKeys.overview });
  };
  const toggle = useMutation({ mutationFn: smartLinksService.toggleLock, onSuccess: refresh });
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
        <EmptyState title={t("smart_links.empty_title")} description={t("smart_links.empty_body")} />
      ) : null}

      <ul className="grid gap-3 md:grid-cols-2">
        {links.map((link) => {
          const url = smartLinkUrl(link.path);
          const locked = link.status === "locked";
          const targetLabel =
            link.target_type === "direct_url" ? link.direct_url : profileName(link.profile_uuid ?? undefined);
          return (
            <li key={link.uuid} className="bg-card rounded-xl border p-4 shadow-sm">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <a href={url} target="_blank" rel="noopener noreferrer" className="block truncate font-medium hover:underline">
                    /{link.path}
                  </a>
                  <p className="text-muted-foreground mt-1 truncate text-sm">
                    {t(`smart_links.target.${link.target_type}`)}
                    {targetLabel ? ` · ${targetLabel}` : ""}
                  </p>
                </div>
                <Badge variant={locked ? "secondary" : "outline"}>
                  {t(locked ? "smart_links.status.locked" : "smart_links.status.active")}
                </Badge>
              </div>
              <div className="mt-3 flex flex-wrap gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() =>
                    void navigator.clipboard.writeText(url).then(() => appToast.success(t("smart_links.copied")))
                  }
                >
                  <Copy className="size-4" />
                  {t("smart_links.copy")}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setQrLink(link)}>
                  <QrCode className="size-4" />
                  {t("smart_links.qr")}
                </Button>
                {canEdit ? (
                  <>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => {
                        setEditing(link);
                        setDialogOpen(true);
                      }}
                    >
                      <Pencil className="size-4" />
                      {t("smart_links.edit")}
                    </Button>
                    <Button size="sm" variant="ghost" disabled={toggle.isPending} onClick={() => toggle.mutate(link.uuid)}>
                      {locked ? <LockOpen className="size-4" /> : <Lock className="size-4" />}
                      {t(locked ? "smart_links.unlock" : "smart_links.lock")}
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      disabled={remove.isPending}
                      onClick={() =>
                        void confirmDelete({
                          title: t("smart_links.delete_title"),
                          description: t("smart_links.delete_body", { path: `/${link.path}` }),
                          confirmLabel: t("smart_links.delete"),
                        }).then((ok) => ok && remove.mutate(link.uuid))
                      }
                    >
                      <Trash2 className="size-4" />
                      {t("smart_links.delete")}
                    </Button>
                  </>
                ) : null}
              </div>
            </li>
          );
        })}
      </ul>

      <SmartLinkDialog open={dialogOpen} onOpenChange={setDialogOpen} link={editing} />

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
