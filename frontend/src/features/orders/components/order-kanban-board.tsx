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

export function OrderKanbanBoard({ orders, onSelectOrder }: OrderKanbanBoardProps) {
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

  const handleAdvance = (e: React.MouseEvent, ord: OrderSummary, nextStatus?: OrderStatus) => {
    e.stopPropagation();
    if (!nextStatus) return;
    updateStatusMutation.mutate({ uuid: ord.uuid, status: nextStatus });
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-6 gap-3 pb-8 overflow-x-auto min-w-[1000px]">
      {KANBAN_COLUMNS.map((col) => {
        const items = groupedOrders[col.id] || [];
        const Icon = col.icon;

        return (
          <div key={col.id} className="flex flex-col rounded-xl bg-muted/30 border p-2.5 min-h-[500px]">
            {/* Column Header */}
            <div className="flex items-center justify-between pb-3 border-b mb-3">
              <div className="flex items-center gap-1.5">
                <Icon className="h-4 w-4 text-muted-foreground" />
                <span className="text-xs font-semibold text-foreground">{t(col.titleKey)}</span>
              </div>
              <Badge variant="outline" className={`font-mono text-[11px] ${col.badgeColor}`}>
                {items.length}
              </Badge>
            </div>

            {/* Column Cards */}
            <div className="flex-1 space-y-2.5 overflow-y-auto max-h-[calc(100vh-280px)] pr-1">
              {items.map((ord) => (
                <Card
                  key={ord.uuid}
                  onClick={() => onSelectOrder(ord)}
                  className="cursor-pointer hover:border-primary/50 transition-all hover:shadow-md bg-card shadow-2xs group"
                >
                  <CardContent className="p-3 space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-xs font-bold text-foreground group-hover:text-primary transition-colors">
                        {ord.order_number}
                      </span>
                      <span className="font-mono text-xs font-semibold text-foreground">
                        {ord.grand_total.toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 0 })} {ord.currency}
                      </span>
                    </div>

                    <div className="text-xs text-muted-foreground line-clamp-1">
                      {ord.user_name || ord.user_email}
                    </div>

                    <div className="flex items-center justify-between text-[11px] text-muted-foreground pt-1 border-t border-border/50">
                      <span>{ord.items_count} {t("orders.detail_quantity")}</span>
                      <span className="font-mono text-[10px]">
                        {new Date(ord.created_at).toLocaleDateString(locale === "tr" ? "tr-TR" : "en-US")}
                      </span>
                    </div>

                    {/* Quick Advance Button */}
                    {col.nextStatus && col.nextLabelKey && (
                      <Button
                        size="sm"
                        variant="ghost"
                        className="w-full h-6 text-[11px] justify-between px-2 bg-muted/60 hover:bg-primary hover:text-primary-foreground transition-colors mt-1"
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
                <div className="text-center py-8 text-muted-foreground text-xs font-mono">
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
