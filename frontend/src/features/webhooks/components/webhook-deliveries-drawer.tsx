"use client";

import { useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { RotateCcw } from "lucide-react";

import { StatusChip } from "@/components/common/status-chip";
import {
  EntityDrawer,
  EntityTable,
  useServerListState,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useWebhookDeliveries,
  useWebhookMutations,
} from "@/features/webhooks/hooks/use-webhooks";
import type {
  WebhookDelivery,
  WebhookDeliveryStatus,
  WebhookEndpoint,
} from "@/features/webhooks/services/webhooks.service";
import { datetime } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

const PAGE_SIZE = 20;
const STATUSES: WebhookDeliveryStatus[] = [
  "pending",
  "succeeded",
  "failed",
  "exhausted",
];
const TONE: Record<
  WebhookDeliveryStatus,
  "default" | "success" | "warning" | "danger"
> = {
  pending: "warning",
  succeeded: "success",
  failed: "danger",
  exhausted: "default",
};

type Props = {
  endpoint: WebhookEndpoint | null;
  canWrite: boolean;
  onClose: () => void;
};

/** Delivery log of one endpoint with a resend per row (TEC-603). */
export function WebhookDeliveriesDrawer({
  endpoint,
  canWrite,
  onClose,
}: Props) {
  const { t, locale } = useLocale();
  const listState = useServerListState({
    initialPageSize: PAGE_SIZE,
    initialSort: "-created_at",
  });
  const [status, setStatus] = useState<WebhookDeliveryStatus | "all">("all");
  const { resend } = useWebhookMutations();
  const { pageIndex, pageSize } = listState.pagination;
  const query = useWebhookDeliveries(
    endpoint?.uuid ?? null,
    pageIndex,
    pageSize,
    status === "all" ? undefined : status,
  );
  const [expanded, setExpanded] = useState<string | null>(null);

  const columns = useMemo<ColumnDef<WebhookDelivery>[]>(
    () => [
      createColumn<WebhookDelivery>({
        accessorKey: "event_name",
        labelKey: "webhooks.deliveries.columns.event",
        enableSorting: false,
        gridPrimary: true,
        cell: ({ row }) => (
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto p-0 font-mono text-xs"
            onClick={() =>
              setExpanded((cur) =>
                cur === row.original.uuid ? null : row.original.uuid,
              )
            }
          >
            {row.original.event_name}
          </Button>
        ),
      }),
      createColumn<WebhookDelivery>({
        accessorKey: "status",
        labelKey: "webhooks.deliveries.columns.status",
        enableSorting: false,
        gridSecondary: true,
        cell: ({ row }) => (
          <div className="space-y-1">
            <StatusChip
              label={t(`webhooks.deliveries.status.${row.original.status}`)}
              tone={TONE[row.original.status]}
            />
            {row.original.last_error ? (
              <p
                className="text-muted-foreground max-w-56 truncate text-[11px]"
                title={row.original.last_error}
              >
                {row.original.last_error}
              </p>
            ) : null}
            {row.original.status === "failed" &&
            row.original.next_attempt_at ? (
              <p className="text-muted-foreground text-[11px]">
                {t("webhooks.deliveries.next_attempt", {
                  date: datetime(
                    row.original.next_attempt_at,
                    "dd.MM.yyyy HH:mm",
                    locale,
                  ),
                })}
              </p>
            ) : null}
          </div>
        ),
      }),
      createColumn<WebhookDelivery>({
        accessorKey: "attempts",
        labelKey: "webhooks.deliveries.columns.attempts",
        enableSorting: false,
        cell: ({ row }) =>
          `${row.original.attempts} / ${row.original.max_attempts}`,
      }),
      createColumn<WebhookDelivery>({
        accessorKey: "last_status_code",
        labelKey: "webhooks.deliveries.columns.code",
        enableSorting: false,
        cell: ({ row }) =>
          row.original.last_status_code ? (
            <Badge variant="outline" className="font-mono">
              {row.original.last_status_code}
            </Badge>
          ) : (
            "—"
          ),
      }),
      createColumn<WebhookDelivery>({
        accessorKey: "created_at",
        labelKey: "webhooks.deliveries.columns.created_at",
        enableSorting: false,
        cell: ({ row }) =>
          datetime(row.original.created_at, "dd.MM.yyyy HH:mm", locale),
      }),
      createColumn<WebhookDelivery>({
        id: "actions",
        labelKey: "webhooks.deliveries.columns.actions",
        enableSorting: false,
        cell: ({ row }) =>
          canWrite ? (
            <Button
              size="sm"
              variant="outline"
              disabled={
                resend.isPending ||
                (row.original.status === "pending" && row.original.attempts > 0)
              }
              onClick={() =>
                endpoint &&
                resend.mutate({
                  uuid: endpoint.uuid,
                  deliveryUuid: row.original.uuid,
                })
              }
            >
              <RotateCcw className="size-4" />
              {t("webhooks.actions.resend")}
            </Button>
          ) : null,
      }),
    ],
    [canWrite, endpoint, locale, resend, t],
  );

  const total = query.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const expandedRow = query.data?.items.find((d) => d.uuid === expanded);

  return (
    <EntityDrawer
      open={Boolean(endpoint)}
      onOpenChange={(open) => (!open ? onClose() : undefined)}
      title={t("webhooks.deliveries.title")}
      description={t("webhooks.deliveries.description", {
        url: endpoint?.url ?? "",
      })}
      size="xl"
    >
      <div className="space-y-4">
        <div className="flex items-center justify-end">
          <Select
            value={status}
            onValueChange={(v) => setStatus(v as WebhookDeliveryStatus | "all")}
          >
            <SelectTrigger className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {t("webhooks.deliveries.filter.all")}
              </SelectItem>
              {STATUSES.map((s) => (
                <SelectItem key={s} value={s}>
                  {t(`webhooks.deliveries.status.${s}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <EntityTable
          columns={columns}
          data={query.data?.items ?? []}
          getRowId={(row) => row.uuid}
          isLoading={query.isLoading}
          isError={query.isError}
          onRetry={() => void query.refetch()}
          emptyTitle={t("webhooks.deliveries.empty_title")}
          emptyDescription={t("webhooks.deliveries.empty_description")}
          pageCount={pageCount}
          state={listState.tableState}
          manual={{ pagination: true, sorting: true, filtering: true }}
          features={{
            rowSelection: false,
            globalFilter: false,
            columnVisibility: false,
            viewMode: false,
            density: false,
          }}
          pageSizeOptions={[PAGE_SIZE]}
        />
        {expandedRow ? (
          <div className="rounded-lg border p-3">
            <p className="text-muted-foreground mb-2 text-xs font-medium">
              {t("webhooks.deliveries.payload")}
            </p>
            <pre className="bg-muted max-h-72 overflow-auto rounded p-3 font-mono text-[11px]">
              {JSON.stringify(expandedRow.payload, null, 2)}
            </pre>
          </div>
        ) : null}
      </div>
    </EntityDrawer>
  );
}
