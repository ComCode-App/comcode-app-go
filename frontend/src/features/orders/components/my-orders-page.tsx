"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Plus, ShoppingBag, Users } from "lucide-react";

import {
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";

import { ordersService } from "../services/orders.service";
import {
  useCanPayOrders,
  useStartOrderPayment,
} from "../hooks/use-order-payment";
import { ordersQueryKeys } from "../hooks/query-keys";
import { SubscriberOrderDialog } from "./subscriber-order-dialog";
import { BulkOrderDialog } from "./bulk-order-dialog";
import { getMyOrdersColumns } from "./my-orders-columns";

export function MyOrdersPage() {
  const { t, locale } = useLocale();
  const [orderDialogOpen, setOrderDialogOpen] = useState(false);
  const [bulkDialogOpen, setBulkDialogOpen] = useState(false);
  const router = useRouter();

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({
      limit: listState.params.limit,
      offset: listState.params.offset,
      q: listState.params.q,
    }),
    [listState.params],
  );
  const hasSearch = Boolean(listParams.q);

  const listQuery = useQuery({
    queryKey: ordersQueryKeys.myOrders(listParams),
    queryFn: () => ordersService.listMyOrders(listParams),
    placeholderData: keepPreviousData,
  });

  const canPay = useCanPayOrders();
  const startPayment = useStartOrderPayment();
  const startPaymentMutate = startPayment.mutate;

  const total = listQuery.data?.total ?? 0;
  const pageCount = Math.max(
    1,
    Math.ceil(total / (listState.pagination.pageSize || 20)),
  );

  const columns = useMemo(
    () =>
      getMyOrdersColumns({
        t,
        locale,
        canPay,
        payPending: startPayment.isPending,
        onPay: (order) => startPaymentMutate(order.uuid),
      }),
    [t, locale, canPay, startPayment.isPending, startPaymentMutate],
  );

  return (
    <div className="space-y-6 pb-12">
      {/* Header */}
      <div className="flex flex-col justify-between gap-4 border-b pb-2 sm:flex-row sm:items-center">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-foreground flex items-center gap-2 text-2xl font-bold tracking-tight">
              <ShoppingBag className="text-primary h-6 w-6" />
              {t("orders.subscriber.page_title")}
            </h1>
            <Badge variant="secondary" className="font-mono text-xs">
              {total} {t("orders.subscriber.orders_count_badge")}
            </Badge>
          </div>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("orders.subscriber.page_description")}
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <Button
            variant="outline"
            className="gap-2 shadow-sm"
            onClick={() => setBulkDialogOpen(true)}
          >
            <Users className="text-primary h-4 w-4" />
            {t("orders.subscriber.bulk_order_btn")}
          </Button>
          <Button
            variant="default"
            className="gap-2 shadow-sm"
            onClick={() => setOrderDialogOpen(true)}
          >
            <Plus className="h-4 w-4" />
            {t("orders.subscriber.order_new_card_btn")}
          </Button>
        </div>
      </div>

      {/* Orders table (TEC-1319): server-side search and paging; cards below md. */}
      {!listQuery.isLoading &&
      !listQuery.isError &&
      !hasSearch &&
      total === 0 ? (
        /* Empty State */
        <div className="bg-card/40 flex flex-col items-center justify-center space-y-4 rounded-2xl border p-8 py-16 text-center">
          <div className="bg-primary/10 text-primary flex h-16 w-16 items-center justify-center rounded-3xl shadow-inner">
            <ShoppingBag className="h-8 w-8" />
          </div>
          <div className="max-w-md space-y-1">
            <h3 className="text-foreground text-lg font-bold">
              {t("orders.subscriber.empty_title")}
            </h3>
            <p className="text-muted-foreground text-xs">
              {t("orders.subscriber.empty_desc")}
            </p>
          </div>
          <Button
            variant="default"
            className="mt-2 gap-2"
            onClick={() => setOrderDialogOpen(true)}
          >
            <Plus className="h-4 w-4" />
            {t("orders.subscriber.order_new_card_btn")}
          </Button>
        </div>
      ) : (
        <EntityTable
          columns={columns}
          data={listQuery.data?.items ?? []}
          getRowId={(row) => row.uuid}
          isLoading={listQuery.isLoading}
          isError={listQuery.isError}
          onRetry={() => void listQuery.refetch()}
          emptyTitle={t("orders.subscriber.no_results")}
          pageCount={pageCount}
          state={listState.tableState}
          onRowClick={(row) => router.push(routes.app.orders.detail(row.uuid))}
          features={{
            persistKey: "app-my-orders-v1",
            sorting: false,
            rowSelection: false,
            columnFilters: false,
            facetedFilters: false,
          }}
          toolbarExtra={
            <EntityToolbar
              onRefresh={() => void listQuery.refetch()}
              refreshDisabled={listQuery.isFetching}
            />
          }
        />
      )}

      {/* Place Order Dialog */}
      <SubscriberOrderDialog
        open={orderDialogOpen}
        onOpenChange={setOrderDialogOpen}
        onSuccess={() => void listQuery.refetch()}
      />

      {/* Bulk Place Order Dialog */}
      <BulkOrderDialog
        open={bulkDialogOpen}
        onOpenChange={setBulkDialogOpen}
        isPlatformAdmin={false}
      />
    </div>
  );
}
