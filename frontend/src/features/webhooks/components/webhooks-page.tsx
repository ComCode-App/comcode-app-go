"use client";

import { useCallback, useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { KeyRound, ListChecks, Pencil, Send, Trash2 } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { StatusChip } from "@/components/common/status-chip";
import {
  EntityCreateButton,
  EntityPage,
  EntityRowActions,
  EntityTable,
  useServerListState,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { WebhookDeliveriesDrawer } from "@/features/webhooks/components/webhook-deliveries-drawer";
import { WebhookDialog } from "@/features/webhooks/components/webhook-dialog";
import { WebhookSecretDialog } from "@/features/webhooks/components/webhook-secret-dialog";
import {
  useWebhookMutations,
  useWebhooks,
} from "@/features/webhooks/hooks/use-webhooks";
import type {
  WebhookEndpoint,
  WebhookEndpointWithSecret,
} from "@/features/webhooks/services/webhooks.service";
import { canManageAccount } from "@/lib/auth/types";
import { datetime } from "@/lib/utils";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

/** The active account's outbound webhooks (TEC-603). */
export function WebhooksPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const { can } = usePermission();
  const { confirm, confirmDelete } = useDialogs();
  const canWrite = can(permissions.webhooks.write) && canManageAccount(user);

  const listState = useServerListState({ initialPageSize: 20 });
  const { data, isLoading, isError, refetch } = useWebhooks();
  const { remove, rotate, sendTest } = useWebhookMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<WebhookEndpoint | null>(null);
  const [secret, setSecret] = useState<WebhookEndpointWithSecret | null>(null);
  const [deliveriesOf, setDeliveriesOf] = useState<WebhookEndpoint | null>(
    null,
  );

  const openCreate = useCallback(() => {
    setEditing(null);
    setDialogOpen(true);
  }, []);
  const openEdit = useCallback((endpoint: WebhookEndpoint) => {
    setEditing(endpoint);
    setDialogOpen(true);
  }, []);

  const handleDelete = useCallback(
    async (endpoint: WebhookEndpoint) => {
      const ok = await confirmDelete({
        title: t("webhooks.confirm_delete.title"),
        description: t("webhooks.confirm_delete.description", {
          url: endpoint.url,
        }),
      });
      if (ok) remove.mutate(endpoint.uuid);
    },
    [confirmDelete, remove, t],
  );

  const handleRotate = useCallback(
    async (endpoint: WebhookEndpoint) => {
      const ok = await confirm({
        title: t("webhooks.confirm_rotate.title"),
        description: t("webhooks.confirm_rotate.description"),
        confirmLabel: t("webhooks.actions.rotate"),
        variant: "destructive",
      });
      if (!ok) return;
      const rotated = await rotate.mutateAsync(endpoint.uuid);
      setSecret(rotated);
    },
    [confirm, rotate, t],
  );

  const columns = useMemo<ColumnDef<WebhookEndpoint>[]>(
    () => [
      createColumn<WebhookEndpoint>({
        accessorKey: "url",
        labelKey: "webhooks.columns.url",
        gridPrimary: true,
        cell: ({ row }) => (
          <div className="min-w-0">
            <p className="truncate font-mono text-xs" title={row.original.url}>
              {row.original.url}
            </p>
            {row.original.description ? (
              <p className="text-muted-foreground truncate text-xs">
                {row.original.description}
              </p>
            ) : null}
          </div>
        ),
      }),
      createColumn<WebhookEndpoint>({
        id: "events",
        labelKey: "webhooks.columns.events",
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            {row.original.events.map((name) => (
              <Badge key={name} variant="secondary" className="text-[10px]">
                {t(`webhooks.events.${name}`)}
              </Badge>
            ))}
          </div>
        ),
      }),
      createColumn<WebhookEndpoint>({
        accessorKey: "is_active",
        labelKey: "webhooks.columns.status",
        gridSecondary: true,
        cell: ({ row }) => {
          const ep = row.original;
          if (!ep.is_active)
            return (
              <StatusChip
                label={t("webhooks.status.inactive")}
                tone="default"
              />
            );
          if (ep.consecutive_failures > 0) {
            return (
              <StatusChip
                label={t("webhooks.status.failing", {
                  count: ep.consecutive_failures,
                })}
                tone="danger"
              />
            );
          }
          return (
            <StatusChip label={t("webhooks.status.active")} tone="success" />
          );
        },
      }),
      createColumn<WebhookEndpoint>({
        accessorKey: "last_delivery_at",
        labelKey: "webhooks.columns.last_delivery",
        cell: ({ row }) => {
          const ep = row.original;
          if (!ep.last_delivery_at)
            return (
              <span className="text-muted-foreground text-xs">
                {t("webhooks.status.never")}
              </span>
            );
          return (
            <div className="text-xs">
              <p>{datetime(ep.last_delivery_at, "dd.MM.yyyy HH:mm", locale)}</p>
              {ep.last_success_at ? (
                <p className="text-muted-foreground">
                  {t("webhooks.status.last_success", {
                    date: datetime(
                      ep.last_success_at,
                      "dd.MM.yyyy HH:mm",
                      locale,
                    ),
                  })}
                </p>
              ) : null}
            </div>
          );
        },
      }),
      createColumn<WebhookEndpoint>({
        id: "actions",
        labelKey: "common.actions",
        enableSorting: false,
        cell: ({ row }) => (
          <EntityRowActions
            actions={[
              {
                id: "deliveries",
                label: t("webhooks.actions.deliveries"),
                icon: ListChecks,
                onSelect: () => setDeliveriesOf(row.original),
              },
              {
                id: "test",
                label: t("webhooks.actions.test"),
                icon: Send,
                permission: permissions.webhooks.write,
                disabled: !canWrite,
                onSelect: () => sendTest.mutate(row.original.uuid),
              },
              {
                id: "edit",
                label: t("webhooks.actions.edit"),
                icon: Pencil,
                permission: permissions.webhooks.write,
                disabled: !canWrite,
                onSelect: () => openEdit(row.original),
              },
              {
                id: "rotate",
                label: t("webhooks.actions.rotate"),
                icon: KeyRound,
                permission: permissions.webhooks.write,
                disabled: !canWrite,
                separatorBefore: true,
                onSelect: () => void handleRotate(row.original),
              },
              {
                id: "delete",
                label: t("webhooks.actions.delete"),
                icon: Trash2,
                variant: "destructive",
                permission: permissions.webhooks.write,
                disabled: !canWrite,
                onSelect: () => void handleDelete(row.original),
              },
            ]}
          />
        ),
      }),
    ],
    [canWrite, handleDelete, handleRotate, locale, openEdit, sendTest, t],
  );

  const items = data?.items ?? [];
  const pageCount = Math.max(
    1,
    Math.ceil(items.length / (listState.pagination.pageSize || 20)),
  );

  return (
    <EntityPage
      title={t("webhooks.title")}
      description={t("webhooks.subtitle")}
      permission={permissions.webhooks.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("webhooks.title") },
      ]}
      actions={
        canWrite ? (
          <EntityCreateButton
            onClick={openCreate}
            label={t("webhooks.actions.add")}
            permission={permissions.webhooks.write}
          />
        ) : null
      }
    >
      <EntityTable
        columns={columns}
        data={items}
        getRowId={(row) => row.uuid}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => void refetch()}
        emptyTitle={t("webhooks.empty.title")}
        emptyDescription={t("webhooks.empty.description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{ persistKey: "app-webhooks-v1", rowSelection: false }}
      />
      <p className="text-muted-foreground mt-3 text-xs">
        {t("webhooks.limit_note")}
      </p>

      <WebhookDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        endpoint={editing}
        onCreated={setSecret}
      />
      <WebhookSecretDialog endpoint={secret} onClose={() => setSecret(null)} />
      <WebhookDeliveriesDrawer
        endpoint={deliveriesOf}
        canWrite={canWrite}
        onClose={() => setDeliveriesOf(null)}
      />
    </EntityPage>
  );
}
