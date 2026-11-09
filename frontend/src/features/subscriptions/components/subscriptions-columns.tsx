"use client";

import Link from "next/link";
import { useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import {
  SubscriptionRowActionsMenu,
  type SubscriptionRowActionHandlers,
} from "@/features/subscriptions/components/subscription-row-actions";
import type { SubscriptionSummary } from "@/features/subscriptions/services/subscriptions.service";
import { useLocale } from "@/providers/locale-provider";

function statusVariant(status: string) {
  switch (status) {
    case "active":
      return "outline" as const;
    case "trialing":
      return "secondary" as const;
    case "cancelled":
    case "expired":
      return "danger" as const;
    default:
      return "secondary" as const;
  }
}

export function useSubscriptionsColumns(
  handlers: SubscriptionRowActionHandlers,
) {
  const { t } = useLocale();

  return useMemo(
    () =>
      [
        createColumn<SubscriptionSummary>({
          accessorKey: "plan_name",
          labelKey: "subscriptions.fields.plan_name",
          enableSorting: false,
          filterVariant: "text",
          gridPrimary: true,
          cell: ({ row }) => (
            <Link
              href={routes.platform.subscriptions.detail(row.original.uuid)}
              className="font-medium hover:underline"
            >
              {row.original.plan_name}
            </Link>
          ),
        }),
        createColumn<SubscriptionSummary>({
          accessorKey: "status",
          labelKey: "subscriptions.fields.status",
          enableSorting: false,
          filterVariant: "select",
          filterOptions: [
            { label: t("subscriptions.status.active"), value: "active" },
            { label: t("subscriptions.status.trialing"), value: "trialing" },
            { label: t("subscriptions.status.cancelled"), value: "cancelled" },
            { label: t("subscriptions.status.expired"), value: "expired" },
          ],
          cell: ({ row }) => (
            <Badge variant={statusVariant(row.original.status)}>
              {t(`subscriptions.status.${row.original.status}`)}
            </Badge>
          ),
        }),
        createColumn<SubscriptionSummary>({
          id: "subject",
          labelKey: "subscriptions.fields.subject",
          enableSorting: false,
          enableColumnFilter: false,
          cell: ({ row }) => (
            <span>
              {t(`subscriptions.subject_type.${row.original.subject.type}`)}:{" "}
              {row.original.subject.name}
            </span>
          ),
        }),
        createColumn<SubscriptionSummary>({
          id: "actions",
          labelKey: "subscriptions.columns.actions",
          enableSorting: false,
          enableHiding: false,
          enableResizing: false,
          enableColumnFilter: false,
          cell: ({ row }) => (
            <SubscriptionRowActionsMenu
              subscription={row.original}
              handlers={handlers}
            />
          ),
        }),
      ] as ColumnDef<SubscriptionSummary, unknown>[],
    [handlers, t],
  );
}
