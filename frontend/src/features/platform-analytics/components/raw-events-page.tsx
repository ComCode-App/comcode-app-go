"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { Eye, X } from "lucide-react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityFilters,
  EntityPage,
  EntityRowActions,
  EntityTable,
  EntityToolbar,
  countActiveFilters,
  useServerListState,
  type EntityFilterDef,
  type EntityFilterValues,
} from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { eventTypeLabel, subjectTypeLabel } from "@/features/analytics/lib/event-labels";
import { datetime } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

import { platformAnalyticsKeys } from "../hooks/query-keys";
import { useEventDefinitions } from "../hooks/use-platform-analytics";
import { dayEndIso, dayStartIso, daysAgo } from "../lib/time-range";
import {
  platformAnalyticsService,
  type RawEvent,
  type RawEventParams,
} from "../services/platform-analytics.service";
import { RawEventDrawer } from "./raw-event-drawer";

/** URL ⇄ filter keys. `from` / `to` are local `yyyy-MM-dd` days. */
const FILTER_KEYS = ["event_type", "subject_type", "subject_id", "country", "from", "to"] as const;
type FilterKey = (typeof FILTER_KEYS)[number];
type Filters = Partial<Record<FilterKey, string>>;

const DEFAULT_WINDOW_DAYS = 7;
const SUBJECT_TYPES = ["route", "profile", "card", "user"];

function one(v: string | string[] | undefined) {
  return (Array.isArray(v) ? v[0] : v)?.trim() || undefined;
}

function toFilters(values: EntityFilterValues): Filters {
  const out: Filters = {};
  for (const k of FILTER_KEYS) {
    const v = one(values[k]);
    if (v) out[k] = k === "country" ? v.toUpperCase() : v;
  }
  return out;
}

/** Raw event drilldown (TEC-565): `GET /v1/analytics/events`, newest first, platform.analytics.read. */
export function RawEventsPage() {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const canRead = can(permissions.analytics.read);

  // A bare visit starts on the last 7 days so the default view stays small and recent. Once the user applies filters (even none) the URL is the only source.
  const [defaultFrom, setDefaultFrom] = useState<string | undefined>(() =>
    searchParams.toString() === "" ? daysAgo(DEFAULT_WINDOW_DAYS - 1) : undefined,
  );

  // Applied filters live in the URL so KPI tiles and definitions can deep-link here.
  const applied = useMemo<Filters>(() => {
    const out: Filters = {};
    for (const k of FILTER_KEYS) {
      const v = searchParams.get(k);
      if (v) out[k] = v;
    }
    if (defaultFrom && Object.keys(out).length === 0) out.from = defaultFrom;
    return out;
  }, [defaultFrom, searchParams]);

  const listState = useServerListState({ initialPageSize: 20, initialSort: "" });
  const { setPagination } = listState;
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [draft, setDraft] = useState<EntityFilterValues>({});
  const [selected, setSelected] = useState<RawEvent | null>(null);

  const applyFilters = useCallback(
    (next: Filters) => {
      const qs = new URLSearchParams();
      for (const k of FILTER_KEYS) if (next[k]) qs.set(k, next[k]!);
      setDefaultFrom(undefined);
      setPagination((p) => ({ ...p, pageIndex: 0 }));
      const s = qs.toString();
      router.replace(s ? `${pathname}?${s}` : pathname, { scroll: false });
    },
    [pathname, router, setPagination],
  );

  // The definitions endpoint needs platform.events.manage; without it the event type is free text.
  const canListDefinitions = can(permissions.events.manage);
  const definitions = useEventDefinitions({}, canListDefinitions);
  const defNames = useMemo(
    () => new Map((definitions.data?.items ?? []).map((d) => [d.key, d.name])),
    [definitions.data],
  );
  const typeLabel = useCallback((type: string) => eventTypeLabel(t, type, defNames.get(type)), [defNames, t]);

  const params = useMemo<RawEventParams>(
    () => ({
      event_type: applied.event_type,
      subject_type: applied.subject_type,
      subject_id: applied.subject_id,
      country: applied.country,
      from: dayStartIso(applied.from),
      to: dayEndIso(applied.to),
      limit: listState.params.limit,
      offset: listState.params.offset,
    }),
    [applied, listState.params.limit, listState.params.offset],
  );

  const listQuery = useQuery({
    queryKey: platformAnalyticsKeys.eventList(params),
    queryFn: () => platformAnalyticsService.listEvents(params),
    enabled: canRead,
    placeholderData: keepPreviousData,
  });

  const filterDefs = useMemo<EntityFilterDef[]>(() => {
    const defs = definitions.data?.items ?? [];
    return [
      defs.length > 0
        ? {
            key: "event_type",
            labelKey: "platform_analytics.events.fields.type",
            variant: "select",
            options: defs.map((d) => ({ value: d.key, label: `${typeLabel(d.key)} (${d.key})` })),
          }
        : {
            key: "event_type",
            labelKey: "platform_analytics.events.fields.type",
            variant: "text",
            placeholderKey: "platform_analytics.events.placeholders.event_type",
          },
      {
        key: "subject_type",
        labelKey: "platform_analytics.events.fields.subject_type",
        variant: "select",
        options: SUBJECT_TYPES.map((s) => ({ value: s, label: subjectTypeLabel(t, s) })),
      },
      {
        key: "subject_id",
        labelKey: "platform_analytics.events.fields.subject_id",
        variant: "text",
        placeholderKey: "platform_analytics.events.placeholders.subject_id",
      },
      {
        key: "country",
        labelKey: "platform_analytics.events.fields.country",
        variant: "text",
        placeholderKey: "platform_analytics.events.placeholders.country",
      },
      { key: "from", labelKey: "platform_analytics.events.filters.from", variant: "date" },
      { key: "to", labelKey: "platform_analytics.events.filters.to", variant: "date" },
    ];
  }, [definitions.data, t, typeLabel]);

  const columns = useMemo<ColumnDef<RawEvent, unknown>[]>(
    () => [
      createColumn<RawEvent>({
        accessorKey: "occurred_at",
        labelKey: "platform_analytics.events.fields.occurred_at",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="tabular-nums whitespace-nowrap">
            {datetime(row.original.occurred_at, "dd.MM.yyyy HH:mm:ss", locale)}
          </span>
        ),
      }),
      createColumn<RawEvent>({
        accessorKey: "type",
        labelKey: "platform_analytics.events.fields.type",
        enableSorting: false,
        gridPrimary: true,
        cell: ({ row }) => (
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{typeLabel(row.original.type)}</div>
            <div className="text-muted-foreground truncate font-mono text-xs">{row.original.type}</div>
          </div>
        ),
      }),
      createColumn<RawEvent>({
        id: "subject",
        accessorFn: (e) => `${e.subject.type}:${e.subject.id}`,
        labelKey: "platform_analytics.events.fields.subject",
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex min-w-0 items-center gap-2">
            <Badge variant="outline" title={row.original.subject.type}>
              {subjectTypeLabel(t, row.original.subject.type)}
            </Badge>
            <span className="text-muted-foreground truncate font-mono text-xs" title={row.original.subject.id}>
              {row.original.subject.id}
            </span>
          </div>
        ),
      }),
      createColumn<RawEvent>({
        id: "location",
        accessorFn: (e) => e.context?.country_code ?? "",
        labelKey: "platform_analytics.events.fields.location",
        enableSorting: false,
        cell: ({ row }) => {
          const c = row.original.context ?? {};
          const text = [c.city, c.country_code].filter(Boolean).join(", ");
          return text || <span className="text-muted-foreground">—</span>;
        },
      }),
      createColumn<RawEvent>({
        id: "device",
        accessorFn: (e) => e.context?.device_type ?? "",
        labelKey: "platform_analytics.events.fields.device",
        enableSorting: false,
        cell: ({ row }) => {
          const c = row.original.context ?? {};
          const text = [c.device_type, c.browser, c.os].filter(Boolean).join(" · ");
          return text || <span className="text-muted-foreground">—</span>;
        },
      }),
      createColumn<RawEvent>({
        accessorKey: "session_id",
        labelKey: "platform_analytics.events.fields.session_id",
        enableSorting: false,
        defaultHidden: true,
        cell: ({ row }) => (
          <span className="font-mono text-xs">{row.original.session_id || "—"}</span>
        ),
      }),
      createColumn<RawEvent>({
        id: "actions",
        labelKey: "common.actions",
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <EntityRowActions
            actions={[
              {
                id: "view",
                label: t("platform_analytics.events.view_detail"),
                icon: Eye,
                onSelect: () => setSelected(row.original),
              },
            ]}
          />
        ),
      }),
    ],
    [locale, t, typeLabel],
  );

  // `total` is capped at 10,000 server-side (TEC-859); past the cap `has_more` adds the next page.
  const total = listQuery.data?.total ?? 0;
  const totalCapped = listQuery.data?.total_capped ?? false;
  const pageCount = Math.max(
    1,
    Math.ceil(total / (listState.pagination.pageSize || 20)),
    listState.pagination.pageIndex + (listQuery.data?.has_more ? 2 : 1),
  );
  const activeCount = countActiveFilters(applied);

  const chipLabels: Record<FilterKey, string> = {
    event_type: t("platform_analytics.events.chip.event_type"),
    subject_type: t("platform_analytics.events.chip.subject_type"),
    subject_id: t("platform_analytics.events.chip.subject_id"),
    country: t("platform_analytics.events.chip.country"),
    from: t("platform_analytics.events.chip.from"),
    to: t("platform_analytics.events.chip.to"),
  };
  const chipValue = (k: FilterKey, v: string) =>
    k === "event_type" ? typeLabel(v) : k === "subject_type" ? subjectTypeLabel(t, v) : v;
  const chips = FILTER_KEYS.filter((k) => applied[k]).map((k) => ({
    key: k,
    label: `${chipLabels[k]}: ${chipValue(k, applied[k]!)}`,
  }));

  return (
    <EntityPage
      title={t("platform_analytics.events.title")}
      description={t("platform_analytics.events.subtitle")}
      permission={permissions.analytics.read}
      forbiddenFallback={
        <ErrorState title={t("common.error_forbidden")} description={t("platform_analytics.forbidden")} />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("layout.nav_platform_analytics"), href: routes.platform.analytics.root },
        { label: t("platform_analytics.events.title") },
      ]}
    >
      <div className="flex flex-wrap items-center gap-2">
        <p className="text-muted-foreground text-sm">
          {listQuery.data
            ? t(totalCapped ? "platform_analytics.events.total_capped" : "platform_analytics.events.total", {
                count: new Intl.NumberFormat(locale).format(total),
              })
            : " "}
        </p>
        {chips.map((chip) => (
          <Badge key={chip.key} variant="secondary" className="gap-1 pr-1">
            <span className="font-mono">{chip.label}</span>
            <button
              type="button"
              className="hover:bg-muted rounded-sm p-0.5"
              aria-label={t("platform_analytics.events.remove_filter")}
              onClick={() => applyFilters({ ...applied, [chip.key]: undefined })}
            >
              <X className="size-3" />
            </button>
          </Badge>
        ))}
        {chips.length > 1 ? (
          <Button variant="ghost" size="sm" className="h-7" onClick={() => applyFilters({})}>
            {t("platform_analytics.events.clear_filters")}
          </Button>
        ) : null}
      </div>

      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        onRowClick={setSelected}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("platform_analytics.events.empty_title")}
        emptyDescription={t("platform_analytics.events.empty_description")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "platform-raw-events-v1",
          rowSelection: false,
          sorting: false,
          globalFilter: false,
          columnFilters: false,
          facetedFilters: false,
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
            onFiltersOpen={() => {
              setDraft(applied);
              setFiltersOpen(true);
            }}
            filtersActive={activeCount > 0}
            filtersLabel={activeCount > 0 ? `${t("entity.filters")} (${activeCount})` : t("entity.filters")}
          />
        }
      />

      <EntityFilters
        open={filtersOpen}
        onOpenChange={setFiltersOpen}
        title={t("platform_analytics.events.filters_title")}
        description={t("platform_analytics.events.filters_description")}
        filters={filterDefs}
        values={draft}
        onChange={(key, value) => setDraft((cur) => ({ ...cur, [key]: value }))}
        onReset={() => setDraft({})}
        onApply={() => applyFilters(toFilters(draft))}
      />

      <RawEventDrawer
        event={selected}
        typeLabel={typeLabel}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
        onFilterSubject={(subject) => {
          setSelected(null);
          applyFilters({ ...applied, subject_type: subject.type, subject_id: subject.id });
        }}
      />
    </EntityPage>
  );
}
