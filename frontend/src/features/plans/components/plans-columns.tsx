"use client";

import Link from "next/link";
import { useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import {
  PlanRowActionsMenu,
  type PlanRowActionHandlers,
} from "@/features/plans/components/plan-row-actions";
import type { PlanSummary } from "@/features/plans/services/plans.service";
import { formatPlanPrice } from "@/features/plans/lib/format-price";
import { useLocale } from "@/providers/locale-provider";

export function usePlansColumns(handlers: PlanRowActionHandlers) {
  const { t } = useLocale();

  return useMemo(
    () =>
      [
        createColumn<PlanSummary>({
          accessorKey: "name",
          labelKey: "plans.fields.name",
          enableSorting: false,
          filterVariant: "text",
          gridPrimary: true,
          cell: ({ row }) => (
            <div className="flex flex-wrap items-center gap-2">
              <Link
                href={routes.platform.plans.detail(row.original.uuid)}
                className="font-medium hover:underline"
              >
                {row.original.name}
              </Link>
              {row.original.slug === "free" ? (
                <Badge variant="secondary" className="text-[10px]">
                  {t("plans.labels.default")}
                </Badge>
              ) : null}
            </div>
          ),
        }),
        createColumn<PlanSummary>({
          accessorKey: "slug",
          labelKey: "plans.fields.slug",
          enableSorting: false,
          cell: ({ row }) => (
            <code className="text-muted-foreground text-xs">
              {row.original.slug}
            </code>
          ),
        }),
        createColumn<PlanSummary>({
          accessorKey: "price_minor",
          labelKey: "plans.fields.price",
          enableSorting: false,
          enableColumnFilter: false,
          cell: ({ row }) =>
            row.original.billing_interval === "free"
              ? t("plans.billing.free")
              : formatPlanPrice(
                  row.original.price_minor,
                  row.original.currency,
                ),
        }),
        createColumn<PlanSummary>({
          accessorKey: "billing_interval",
          labelKey: "plans.fields.billing_interval",
          enableSorting: false,
          enableColumnFilter: false,
          cell: ({ row }) =>
            t(`plans.billing.${row.original.billing_interval}`),
        }),
        createColumn<PlanSummary>({
          accessorKey: "is_active",
          labelKey: "plans.fields.is_active",
          enableSorting: false,
          enableColumnFilter: false,
          cell: ({ row }) =>
            row.original.is_active ? (
              <Badge variant="outline">{t("plans.labels.active")}</Badge>
            ) : (
              <Badge variant="secondary">{t("plans.labels.inactive")}</Badge>
            ),
        }),
        createColumn<PlanSummary>({
          id: "actions",
          labelKey: "plans.columns.actions",
          enableSorting: false,
          enableHiding: false,
          enableResizing: false,
          enableColumnFilter: false,
          cell: ({ row }) => (
            <PlanRowActionsMenu plan={row.original} handlers={handlers} />
          ),
        }),
      ] as ColumnDef<PlanSummary, unknown>[],
    [handlers, t],
  );
}
