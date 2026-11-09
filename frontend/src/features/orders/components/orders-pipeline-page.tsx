"use client";

import { useState, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { List } from "lucide-react";
import Link from "next/link";

import { ErrorState } from "@/components/common/error-state";
import { EntityPage } from "@/components/entity";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";

import {
  ordersService,
  type OrderSummary,
} from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";
import { OrderKanbanBoard } from "./order-kanban-board";
import { OrderDetailDialog } from "./order-detail-dialog";
import { OrderKPICards } from "./order-kpi-cards";

export function OrdersPipelinePage() {
  const { t } = useLocale();
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  const listQuery = useQuery({
    queryKey: ordersQueryKeys.list({ limit: 100, offset: 0 }),
    queryFn: () => ordersService.list({ limit: 100, offset: 0 }),
  });

  const handleSelectOrder = useCallback((order: OrderSummary) => {
    setSelectedOrderUuid(order.uuid);
    setDialogOpen(true);
  }, []);

  return (
    <EntityPage
      title={t("orders.pipeline_title")}
      description={t("orders.pipeline_description")}
      permission={permissions.platformOrdersRead}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("common.error_forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("orders.title"), href: routes.platform.orders },
        { label: t("orders.pipeline") },
      ]}
      actions={
        <Button asChild variant="outline">
          <Link href={routes.platform.orders}>
            <List className="mr-2 h-4 w-4" />
            {t("orders.list_view")}
          </Link>
        </Button>
      }
    >
      <OrderKPICards />

      {listQuery.isError ? (
        <ErrorState
          title={t("orders.error_title")}
          description={
            listQuery.error instanceof Error
              ? listQuery.error.message
              : t("common.error_occurred")
          }
          onRetry={() => void listQuery.refetch()}
        />
      ) : (
        <OrderKanbanBoard
          orders={listQuery.data?.items ?? []}
          onSelectOrder={handleSelectOrder}
        />
      )}

      <OrderDetailDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        orderUuid={selectedOrderUuid}
      />
    </EntityPage>
  );
}
