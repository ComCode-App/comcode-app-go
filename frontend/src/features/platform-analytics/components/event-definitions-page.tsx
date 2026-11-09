"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { ListFilter, Pencil, Plus, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";

import { ErrorState } from "@/components/common/error-state";
import { StatusChip } from "@/components/common/status-chip";
import { EntityPage, EntityRowActions, EntityTable, EntityToolbar } from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { eventCategoryLabel, eventTypeLabel } from "@/features/analytics/lib/event-labels";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

import { useEventDefinitionMutations, useEventDefinitions } from "../hooks/use-platform-analytics";
import type { EventDefinition, EventDefinitionInput } from "../services/platform-analytics.service";
import { EventDefinitionDialog } from "./event-definition-dialog";

type Flag = "enabled" | "is_public" | "aggregate_enabled";

/** Full input from a stored definition with one flag flipped (PATCH needs key + name). */
function withFlag(def: EventDefinition, flag: Flag, value: boolean): EventDefinitionInput {
  return {
    key: def.key,
    name: def.name,
    description: def.description,
    category: def.category,
    schema_version: def.schema_version,
    retention_days: def.retention_days,
    required_properties: def.required_properties ?? [],
    enabled: def.enabled,
    is_public: def.is_public,
    aggregate_enabled: def.aggregate_enabled,
    [flag]: value,
  };
}

/** Event registry CRUD (TEC-565). The list endpoint returns every row, so the table filters client-side. */
export function EventDefinitionsPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const { confirmDelete } = useDialogs();
  const router = useRouter();
  const canRead = can(permissions.analytics.read);
  const listQuery = useEventDefinitions();
  const { toggle, remove } = useEventDefinitionMutations();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<EventDefinition | null>(null);

  const openCreate = useCallback(() => {
    setEditing(null);
    setDialogOpen(true);
  }, []);
  const openEdit = useCallback((def: EventDefinition) => {
    setEditing(def);
    setDialogOpen(true);
  }, []);

  const handleDelete = useCallback(
    async (def: EventDefinition) => {
      const ok = await confirmDelete({
        title: t("platform_analytics.definitions.delete_title", { key: eventTypeLabel(t, def.key, def.name) }),
        description: t("platform_analytics.definitions.delete_description"),
      });
      if (ok) remove.mutate(def.id);
    },
    [confirmDelete, remove, t],
  );

  const items = useMemo(() => listQuery.data?.items ?? [], [listQuery.data]);

  const categoryOptions = useMemo(
    () =>
      [...new Set(items.map((d) => d.category))]
        .sort()
        .map((c) => ({ value: c, label: eventCategoryLabel(t, c) })),
    [items, t],
  );

  const columns = useMemo<ColumnDef<EventDefinition, unknown>[]>(() => {
    const flagLabels: Record<Flag, string> = {
      enabled: t("platform_analytics.definitions.fields.enabled"),
      is_public: t("platform_analytics.definitions.fields.is_public"),
      aggregate_enabled: t("platform_analytics.definitions.fields.aggregate_enabled"),
    };
    const flagCell = (flag: Flag) =>
      function FlagCell({ row }: { row: { original: EventDefinition } }) {
        const def = row.original;
        return (
          <Switch
            checked={def[flag]}
            disabled={toggle.isPending}
            aria-label={flagLabels[flag]}
            onCheckedChange={(v) => toggle.mutate({ id: def.id, body: withFlag(def, flag, v) })}
          />
        );
      };
    const yesNo = [
      { value: "true", label: t("common.yes") },
      { value: "false", label: t("common.no") },
    ];

    return [
      createColumn<EventDefinition>({
        accessorKey: "key",
        labelKey: "platform_analytics.definitions.fields.key",
        gridPrimary: true,
        cell: ({ row }) => (
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">
              {eventTypeLabel(t, row.original.key, row.original.name)}
            </div>
            <div className="text-muted-foreground truncate font-mono text-xs">{row.original.key}</div>
          </div>
        ),
      }),
      createColumn<EventDefinition>({
        accessorKey: "category",
        labelKey: "platform_analytics.definitions.fields.category",
        filterVariant: "faceted",
        filterOptions: categoryOptions,
        cell: ({ row }) => (
          <Badge variant="outline" title={row.original.category}>
            {eventCategoryLabel(t, row.original.category)}
          </Badge>
        ),
      }),
      createColumn<EventDefinition>({
        id: "enabled",
        accessorFn: (d) => String(d.enabled),
        labelKey: "platform_analytics.definitions.fields.enabled",
        filterVariant: "faceted",
        filterOptions: yesNo,
        cell: flagCell("enabled"),
      }),
      createColumn<EventDefinition>({
        id: "is_public",
        accessorFn: (d) => String(d.is_public),
        labelKey: "platform_analytics.definitions.fields.is_public",
        filterVariant: "faceted",
        filterOptions: yesNo,
        cell: flagCell("is_public"),
      }),
      createColumn<EventDefinition>({
        id: "aggregate_enabled",
        accessorFn: (d) => String(d.aggregate_enabled),
        labelKey: "platform_analytics.definitions.fields.aggregate_enabled",
        cell: flagCell("aggregate_enabled"),
      }),
      createColumn<EventDefinition>({
        accessorKey: "retention_days",
        labelKey: "platform_analytics.definitions.fields.retention_days",
        cell: ({ row }) => (
          <span className="tabular-nums">
            {t("platform_analytics.definitions.days", { count: row.original.retention_days ?? 0 })}
          </span>
        ),
      }),
      createColumn<EventDefinition>({
        id: "required_properties",
        accessorFn: (d) => (d.required_properties ?? []).join(", "),
        labelKey: "platform_analytics.definitions.fields.required_properties",
        enableSorting: false,
        defaultHidden: true,
        cell: ({ row }) => {
          const props = row.original.required_properties ?? [];
          if (props.length === 0) return <span className="text-muted-foreground">—</span>;
          return (
            <div className="flex flex-wrap gap-1">
              {props.map((p) => (
                <StatusChip key={p} label={p} />
              ))}
            </div>
          );
        },
      }),
      createColumn<EventDefinition>({
        id: "actions",
        labelKey: "common.actions",
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <EntityRowActions
            actions={[
              ...(canRead
                ? [
                    {
                      id: "events",
                      label: t("platform_analytics.definitions.view_events"),
                      icon: ListFilter,
                      onSelect: () =>
                        router.push(
                          `${routes.platform.analytics.events}?event_type=${encodeURIComponent(row.original.key)}`,
                        ),
                    },
                  ]
                : []),
              {
                id: "edit",
                label: t("common.edit"),
                icon: Pencil,
                onSelect: () => openEdit(row.original),
              },
              {
                id: "delete",
                label: t("common.delete"),
                icon: Trash2,
                variant: "destructive" as const,
                onSelect: () => void handleDelete(row.original),
              },
            ]}
          />
        ),
      }),
    ];
  }, [canRead, categoryOptions, handleDelete, openEdit, router, t, toggle]);

  const enabledCount = items.filter((d) => d.enabled).length;

  return (
    <EntityPage
      title={t("platform_analytics.definitions.title")}
      description={t("platform_analytics.definitions.subtitle")}
      permission={permissions.events.manage}
      forbiddenFallback={
        <ErrorState title={t("common.error_forbidden")} description={t("platform_analytics.forbidden")} />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("layout.nav_platform_analytics"), href: routes.platform.analytics.root },
        { label: t("platform_analytics.definitions.title") },
      ]}
      actions={
        <Button onClick={openCreate} className="gap-2">
          <Plus className="size-4" />
          {t("platform_analytics.definitions.add")}
        </Button>
      }
    >
      {listQuery.data ? (
        <p className="text-muted-foreground text-sm">
          {t("platform_analytics.definitions.summary", { total: items.length, enabled: enabledCount })}{" "}
          <Link href={routes.platform.analytics.config} className="underline-offset-4 hover:underline">
            {t("platform_analytics.definitions.config_link")}
          </Link>
        </p>
      ) : null}

      <EntityTable
        columns={columns}
        data={items}
        getRowId={(row) => String(row.id)}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("platform_analytics.definitions.empty_title")}
        emptyDescription={t("platform_analytics.definitions.empty_description")}
        manual={{ filtering: false, sorting: false, pagination: false }}
        features={{ persistKey: "platform-event-definitions-v1", rowSelection: false }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />

      <EventDefinitionDialog open={dialogOpen} onOpenChange={setDialogOpen} definition={editing} />
    </EntityPage>
  );
}
