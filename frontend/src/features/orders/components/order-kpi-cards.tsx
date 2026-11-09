"use client";

import { useQuery } from "@tanstack/react-query";
import { ShoppingBag, Paintbrush, Cog, Truck, DollarSign } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ordersService } from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";
import { useLocale } from "@/providers/locale-provider";

export function OrderKPICards() {
  const { t } = useLocale();
  const { data: kpis, isLoading } = useQuery({
    queryKey: ordersQueryKeys.kpis(),
    queryFn: () => ordersService.kpis(),
  });

  if (isLoading) {
    return (
      <div className="mb-6 grid grid-cols-2 gap-4 md:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-24 w-full rounded-xl" />
        ))}
      </div>
    );
  }

  const cards = [
    {
      title: t("orders.kpis.today"),
      value: kpis?.today_orders ?? 0,
      icon: ShoppingBag,
      color: "text-blue-500",
      bg: "bg-blue-500/10",
      border: "border-blue-500/20",
    },
    {
      title: t("orders.kpis.pending_design"),
      value: kpis?.pending_design_count ?? 0,
      icon: Paintbrush,
      color: "text-amber-500",
      bg: "bg-amber-500/10",
      border: "border-amber-500/20",
    },
    {
      title: t("orders.kpis.in_production"),
      value: kpis?.in_production_count ?? 0,
      icon: Cog,
      color: "text-purple-500",
      bg: "bg-purple-500/10",
      border: "border-purple-500/20",
    },
    {
      title: t("orders.kpis.ready_to_ship"),
      value: kpis?.ready_to_ship_count ?? 0,
      icon: Truck,
      color: "text-emerald-500",
      bg: "bg-emerald-500/10",
      border: "border-emerald-500/20",
    },
    {
      title: t("orders.kpis.total_revenue"),
      value: `${(kpis?.total_revenue ?? 0).toLocaleString("tr-TR", { minimumFractionDigits: 0 })} ₺`,
      icon: DollarSign,
      color: "text-teal-500",
      bg: "bg-teal-500/10",
      border: "border-teal-500/20",
    },
  ];

  return (
    <div className="mb-6 grid grid-cols-2 gap-3.5 md:grid-cols-5">
      {cards.map((card, idx) => {
        const Icon = card.icon;
        return (
          <Card
            key={idx}
            className={`border shadow-xs ${card.border} transition-all hover:shadow-md`}
          >
            <CardContent className="flex items-center justify-between p-4">
              <div>
                <p className="text-muted-foreground text-xs font-medium">
                  {card.title}
                </p>
                <p className="text-foreground mt-1 text-2xl font-bold tracking-tight">
                  {card.value}
                </p>
              </div>
              <div className={`rounded-xl p-2.5 ${card.bg} ${card.color}`}>
                <Icon className="h-5 w-5" />
              </div>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
