"use client";

import { useState, useCallback, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Kanban } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { ErrorState } from "@/components/common/error-state";
import {
  EntityPage,
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";

import {
  ordersService,
  type OrderSummary,
} from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";
import { getOrdersColumns } from "./orders-columns";
import { OrderKPICards } from "./order-kpi-cards";
import { OrderDetailDialog } from "./order-detail-dialog";
import { BulkOrderDialog } from "./bulk-order-dialog";
import { Users } from "lucide-react";

export function OrdersPage() {
  const { t } = useLocale();
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [bulkDialogOpen, setBulkDialogOpen] = useState(false);

  const listState = useServerListState({ initialPageSize: 20 });
  const listParams = useMemo(
    () => ({ ...listState.params, q: listState.params.q }),
    [listState.params],
  );

  const listQuery = useQuery({
    queryKey: ordersQueryKeys.list(listParams),
    queryFn: () => ordersService.list(listParams),
  });

  // Cmd+K hits link to /platform/orders?open={uuid} (TEC-595); closing drops the param.
  const router = useRouter();
  const pathname = usePathname();
  const openParam = useSearchParams().get("open");
  const handleDialogChange = useCallback(
    (next: boolean) => {
      setDialogOpen(next);
      if (!next && openParam) router.replace(pathname);
    },
    [openParam, pathname, router],
  );

  const handleView = useCallback((order: OrderSummary) => {
    setSelectedOrderUuid(order.uuid);
    setDialogOpen(true);
  }, []);

  const columns = useMemo(
    () =>
      getOrdersColumns({
        t,
        onView: handleView,
      }),
    [t, handleView],
  );

  const pageCount = useMemo(() => {
    const total = listQuery.data?.total ?? 0;
    const size = listState.pagination.pageSize || 20;
    return Math.max(1, Math.ceil(total / size));
  }, [listQuery.data?.total, listState.pagination.pageSize]);

  return (
    <EntityPage
      title={t("orders.title")}
      description={t("orders.description")}
      permission={permissions.platformOrdersRead}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("orders.title") },
      ]}
      actions={
        <div className="flex items-center gap-2">
          <Button onClick={() => setBulkDialogOpen(true)} className="bg-primary hover:bg-primary/90">
            <Users className="mr-2 h-4 w-4" />
            {t("orders.create_bulk_order_btn")}
          </Button>
          <Button asChild variant="outline">
            <Link href={routes.platform.ordersPipeline}>
              <Kanban className="mr-2 h-4 w-4 text-primary" />
              {t("orders.pipeline_button")}
            </Link>
          </Button>
        </div>
      }
    >
      {/* 1. Overview KPIs Dashboard */}
      <OrderKPICards />

      {/* 2. Orders Table */}
      <EntityTable
        columns={columns}
        data={listQuery.data?.items ?? []}
        getRowId={(row) => row.uuid}
        isLoading={listQuery.isLoading}
        isError={listQuery.isError}
        onRetry={() => void listQuery.refetch()}
        emptyTitle={t("orders.empty")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "platform-orders-v1",
        }}
        toolbarExtra={
          <EntityToolbar
            onRefresh={() => void listQuery.refetch()}
            refreshDisabled={listQuery.isFetching}
          />
        }
      />

      {/* 3. Order Detail & Asset Request Drawer / Dialog */}
      <OrderDetailDialog
        open={dialogOpen || Boolean(openParam)}
        onOpenChange={handleDialogChange}
        orderUuid={selectedOrderUuid ?? openParam}
      />

      {/* 4. Bulk Order Creation Modal */}
      <BulkOrderDialog
        open={bulkDialogOpen}
        onOpenChange={setBulkDialogOpen}
        isPlatformAdmin={true}
      />
    </EntityPage>
  );
}
