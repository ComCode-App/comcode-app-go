"use client";

import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { FileDown } from "lucide-react";

import { EntityTable, useServerListState } from "@/components/entity";
import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  type BillingPayment,
  billingService,
} from "@/features/billing/services/billing.service";
import { formatPlanPrice } from "@/features/plans/lib/format-price";
import { datetime } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const PAGE_SIZE = 10;

type Translate = (
  key: string,
  values?: Record<string, string | number>,
) => string;

function describePayment(payment: BillingPayment, t: Translate) {
  const kind = t(`billing.payments.kind.${payment.kind}`);
  if (payment.order_number) return `${kind} · ${payment.order_number}`;
  if (payment.plan_name) {
    const interval = payment.billing_interval
      ? ` (${t(`plans.billing.${payment.billing_interval}`)})`
      : "";
    return `${kind} · ${payment.plan_name}${interval}`;
  }
  return kind;
}

function providerLabel(payment: BillingPayment, t: Translate) {
  const provider = payment.provider.trim().toLowerCase();
  if (provider === "paytr" || provider === "iyzico") {
    return t(`billing.payments.provider.${provider}`);
  }
  return payment.provider || t("billing.payments.provider.manual");
}

/**
 * Payment history of the active account with a PDF receipt per row (TEC-590). Receipts confirm
 * a payment; invoices (e-arşiv) are a separate document, which the card says.
 */
export function BillingPaymentsCard({ accountUuid }: { accountUuid?: string }) {
  const { t, locale } = useLocale();
  const listState = useServerListState({
    initialPageSize: PAGE_SIZE,
    initialSort: "-paid_at",
  });
  const { pageIndex, pageSize } = listState.pagination;
  const [downloading, setDownloading] = useState<string | null>(null);

  const paymentsQuery = useQuery({
    queryKey: ["billing", "payments", accountUuid, pageIndex, pageSize],
    queryFn: () =>
      billingService.listPayments({
        limit: pageSize,
        offset: pageIndex * pageSize,
      }),
  });

  const download = useCallback(
    async (payment: BillingPayment) => {
      setDownloading(payment.uuid);
      try {
        await billingService.downloadReceipt(payment, locale);
      } catch {
        // toast-guard: platformDownloadFile throws a plain Error and never toasts.
        appToast.error(t("billing.payments.receipt_failed"));
      } finally {
        setDownloading(null);
      }
    },
    [locale, t],
  );

  const columns = useMemo<ColumnDef<BillingPayment>[]>(
    () => [
      createColumn<BillingPayment>({
        accessorKey: "paid_at",
        labelKey: "billing.payments.columns.paid_at",
        enableSorting: false,
        gridSecondary: true,
        cell: ({ row }) =>
          datetime(row.original.paid_at, "dd.MM.yyyy HH:mm", locale),
      }),
      createColumn<BillingPayment>({
        id: "description",
        labelKey: "billing.payments.columns.description",
        enableSorting: false,
        gridPrimary: true,
        cell: ({ row }) => (
          <span className="font-medium">
            {describePayment(row.original, t)}
          </span>
        ),
      }),
      createColumn<BillingPayment>({
        accessorKey: "provider",
        labelKey: "billing.payments.columns.provider",
        enableSorting: false,
        cell: ({ row }) => (
          <div className="flex flex-wrap items-center gap-2">
            <span>{providerLabel(row.original, t)}</span>
            {row.original.provider_ref ? (
              <Badge variant="outline" className="font-mono text-[10px]">
                {row.original.provider_ref}
              </Badge>
            ) : null}
          </div>
        ),
      }),
      createColumn<BillingPayment>({
        accessorKey: "amount_minor",
        labelKey: "billing.payments.columns.amount",
        enableSorting: false,
        cell: ({ row }) => (
          <span className="font-mono">
            {formatPlanPrice(
              row.original.amount_minor,
              row.original.currency,
              locale === "tr" ? "tr-TR" : "en-US",
            )}
          </span>
        ),
      }),
      createColumn<BillingPayment>({
        accessorKey: "receipt_number",
        labelKey: "billing.payments.columns.receipt",
        enableSorting: false,
        cell: ({ row }) => (
          <Button
            size="sm"
            variant="outline"
            disabled={downloading === row.original.uuid}
            onClick={() => void download(row.original)}
            aria-label={`${t("billing.payments.download_receipt")} ${row.original.receipt_number}`}
            title={t("billing.payments.download_receipt")}
          >
            <FileDown className="size-4" />
            {row.original.receipt_number}
          </Button>
        ),
      }),
    ],
    [download, downloading, locale, t],
  );

  const total = paymentsQuery.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("billing.payments.title")}</CardTitle>
        <CardDescription>{t("billing.payments.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <EntityTable
          columns={columns}
          data={paymentsQuery.data?.items ?? []}
          getRowId={(row) => row.uuid}
          isLoading={paymentsQuery.isLoading}
          isError={paymentsQuery.isError}
          onRetry={() => void paymentsQuery.refetch()}
          emptyTitle={t("billing.payments.empty_title")}
          emptyDescription={t("billing.payments.empty_description")}
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
        <p className="text-muted-foreground mt-3 text-xs">
          {t("billing.payments.not_invoice")}
        </p>
      </CardContent>
    </Card>
  );
}
