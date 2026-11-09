"use client";

import Link from "next/link";
import { useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { routes } from "@/config/routes";
import {
  CompanyRowActionsMenu,
  type CompanyRowActionHandlers,
} from "@/features/companies/components/company-row-actions";
import type { CompanySummary } from "@/features/companies/services/companies.service";
import { useLocale } from "@/providers/locale-provider";

export function useCompaniesColumns(handlers: CompanyRowActionHandlers) {
  const { t } = useLocale();

  return useMemo(
    () =>
      [
        createColumn<CompanySummary>({
          accessorKey: "name",
          labelKey: "companies.fields.name",
          enableSorting: false,
          filterVariant: "text",
          gridPrimary: true,
          cell: ({ row }) => (
            <Link
              href={routes.platform.companies.detail(row.original.uuid)}
              className="font-medium hover:underline"
            >
              {row.original.name}
            </Link>
          ),
        }),
        createColumn<CompanySummary>({
          accessorKey: "slug",
          labelKey: "companies.fields.slug",
          enableSorting: false,
          cell: ({ row }) => (
            <code className="text-muted-foreground text-xs">
              {row.original.slug}
            </code>
          ),
        }),
        createColumn<CompanySummary>({
          accessorKey: "status",
          labelKey: "companies.fields.status",
          enableSorting: false,
          filterVariant: "select",
          filterOptions: [
            { label: t("companies.status.active"), value: "active" },
            { label: t("companies.status.suspended"), value: "suspended" },
          ],
          cell: ({ row }) => (
            <Badge
              variant={
                row.original.status === "active" ? "outline" : "secondary"
              }
            >
              {t(`companies.status.${row.original.status}`)}
            </Badge>
          ),
        }),
        createColumn<CompanySummary>({
          id: "actions",
          labelKey: "companies.columns.actions",
          enableSorting: false,
          enableColumnFilter: false,
          cell: ({ row }) => (
            <CompanyRowActionsMenu company={row.original} handlers={handlers} />
          ),
        }),
      ] as ColumnDef<CompanySummary, unknown>[],
    [handlers, t],
  );
}
