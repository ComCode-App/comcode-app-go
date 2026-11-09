"use client";

import { useMemo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Paintbrush,
  Cog,
  Truck,
  CheckCircle2,
  Clock,
  ArrowRight,
  type LucideIcon,
} from "lucide-react";
import type { OrderSummary, OrderStatus } from "../services/orders.service";
import { useOrderMutations } from "../hooks/use-orders";
import { useLocale } from "@/providers/locale-provider";

interface OrderKanbanBoardProps {
  orders: OrderSummary[];
  onSelectOrder: (order: OrderSummary) => void;
}

interface ColumnConfig {
  id: OrderStatus;
  titleKey: string;
  badgeColor: string;
  icon: LucideIcon;
  nextStatus?: OrderStatus;
  nextLabelKey?: string;
}

const KANBAN_COLUMNS: ColumnConfig[] = [
  {
    id: "pending",
    titleKey: "orders.status_pending",
    badgeColor: "bg-blue-500/10 text-blue-500 border-blue-500/20",
    icon: Clock,
    nextStatus: "paid",
    nextLabelKey: "orders.kanban.next_paid",
  },
  {
    id: "design_pending",
    titleKey: "orders.status_design_pending",
    badgeColor: "bg-amber-500/10 text-amber-500 border-amber-500/20",
    icon: Paintbrush,
    nextStatus: "in_production",
    nextLabelKey: "orders.kanban.next_production",
  },
  {
    id: "in_production",
    titleKey: "orders.status_in_production",
    badgeColor: "bg-purple-500/10 text-purple-500 border-purple-500/20",
    icon: Cog,
    nextStatus: "ready_to_ship",
    nextLabelKey: "orders.kanban.next_ready",
  },
  {
    id: "ready_to_ship",
    titleKey: "orders.status_ready_to_ship",
    badgeColor: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20",
    icon: Truck,
    nextStatus: "shipped",
    nextLabelKey: "orders.kanban.next_shipped",
  },
  {
    id: "shipped",
    titleKey: "orders.status_shipped",
    badgeColor: "bg-teal-500/10 text-teal-500 border-teal-500/20",
    icon: Truck,
    nextStatus: "delivered",
    nextLabelKey: "orders.status_delivered",
  },
  {
    id: "delivered",
    titleKey: "orders.status_delivered",
    badgeColor: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
    icon: CheckCircle2,
  },
];

export function OrderKanbanBoard({
  orders,
  onSelectOrder,
}: OrderKanbanBoardProps) {
  const { t, locale } = useLocale();
  const { updateStatusMutation } = useOrderMutations();

  const groupedOrders = useMemo(() => {
    const map: Record<string, OrderSummary[]> = {
      pending: [],
      design_pending: [],
      in_production: [],
      ready_to_ship: [],
      shipped: [],
      delivered: [],
    };

    orders.forEach((ord) => {
      if (ord.status === "paid") {
        map.design_pending.push(ord);
      } else if (ord.status === "design_approved") {
        map.in_production.push(ord);
      } else if (map[ord.status]) {
        map[ord.status].push(ord);
      } else {
        map.pending.push(ord);
      }
    });

    return map;
  }, [orders]);

  const handleAdvance = (
    e: React.MouseEvent,
    ord: OrderSummary,
    nextStatus?: OrderStatus,
  ) => {
    e.stopPropagation();
    if (!nextStatus) return;
    updateStatusMutation.mutate({ uuid: ord.uuid, status: nextStatus });
  };

  return (
    <div className="grid min-w-[1000px] grid-cols-1 gap-3 overflow-x-auto pb-8 md:grid-cols-3 lg:grid-cols-6">
      {KANBAN_COLUMNS.map((col) => {
        const items = groupedOrders[col.id] || [];
        const Icon = col.icon;

        return (
          <div
            key={col.id}
            className="bg-muted/30 flex min-h-[500px] flex-col rounded-xl border p-2.5"
          >
            {/* Column Header */}
            <div className="mb-3 flex items-center justify-between border-b pb-3">
              <div className="flex items-center gap-1.5">
                <Icon className="text-muted-foreground h-4 w-4" />
                <span className="text-foreground text-xs font-semibold">
                  {t(col.titleKey)}
                </span>
              </div>
              <Badge
                variant="outline"
                className={`font-mono text-[11px] ${col.badgeColor}`}
              >
                {items.length}
              </Badge>
            </div>

            {/* Column Cards */}
            <div className="max-h-[calc(100vh-280px)] flex-1 space-y-2.5 overflow-y-auto pr-1">
              {items.map((ord) => (
                <Card
                  key={ord.uuid}
                  onClick={() => onSelectOrder(ord)}
                  className="hover:border-primary/50 bg-card group cursor-pointer shadow-2xs transition-all hover:shadow-md"
                >
                  <CardContent className="space-y-2 p-3">
                    <div className="flex items-center justify-between">
                      <span className="text-foreground group-hover:text-primary font-mono text-xs font-bold transition-colors">
                        {ord.order_number}
                      </span>
                      <span className="text-foreground font-mono text-xs font-semibold">
                        {ord.grand_total.toLocaleString(
                          locale === "tr" ? "tr-TR" : "en-US",
                          { minimumFractionDigits: 0 },
                        )}{" "}
                        {ord.currency}
                      </span>
                    </div>

                    <div className="text-muted-foreground line-clamp-1 text-xs">
                      {ord.user_name || ord.user_email}
                    </div>

                    <div className="text-muted-foreground border-border/50 flex items-center justify-between border-t pt-1 text-[11px]">
                      <span>
                        {ord.items_count} {t("orders.detail_quantity")}
                      </span>
                      <span className="font-mono text-[10px]">
                        {new Date(ord.created_at).toLocaleDateString(
                          locale === "tr" ? "tr-TR" : "en-US",
                        )}
                      </span>
                    </div>

                    {/* Quick Advance Button */}
                    {col.nextStatus && col.nextLabelKey && (
                      <Button
                        size="sm"
                        variant="ghost"
                        className="bg-muted/60 hover:bg-primary hover:text-primary-foreground mt-1 h-6 w-full justify-between px-2 text-[11px] transition-colors"
                        onClick={(e) => handleAdvance(e, ord, col.nextStatus)}
                        disabled={updateStatusMutation.isPending}
                      >
                        <span>{t(col.nextLabelKey)}</span>
                        <ArrowRight className="h-3 w-3" />
                      </Button>
                    )}
                  </CardContent>
                </Card>
              ))}

              {items.length === 0 && (
                <div className="text-muted-foreground py-8 text-center font-mono text-xs">
                  {t("orders.kanban.empty_column")}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
