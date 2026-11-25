"use client";

import {
  AlertCircle,
  CheckCircle2,
  Clock,
  Package,
  Truck,
  type LucideIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

type BadgeVariant =
  "default" | "secondary" | "outline" | "success" | "warning" | "danger";

const STATUS_CONFIG: Record<
  string,
  { labelKey: string; variant: BadgeVariant; icon: LucideIcon }
> = {
  paid: {
    labelKey: "orders.status_paid",
    variant: "success",
    icon: CheckCircle2,
  },
  design_pending: {
    labelKey: "orders.status_design_pending",
    variant: "warning",
    icon: AlertCircle,
  },
  design_approved: {
    labelKey: "orders.status_design_approved",
    variant: "success",
    icon: CheckCircle2,
  },
  in_production: {
    labelKey: "orders.status_in_production",
    variant: "default",
    icon: Package,
  },
  ready_to_ship: {
    labelKey: "orders.status_ready_to_ship",
    variant: "secondary",
    icon: Package,
  },
  shipped: {
    labelKey: "orders.status_shipped",
    variant: "default",
    icon: Truck,
  },
  delivered: {
    labelKey: "orders.status_delivered",
    variant: "success",
    icon: CheckCircle2,
  },
  cancelled: {
    labelKey: "orders.status_cancelled",
    variant: "danger",
    icon: AlertCircle,
  },
  refunded: {
    labelKey: "orders.status_refunded",
    variant: "danger",
    icon: AlertCircle,
  },
  pending: {
    labelKey: "orders.status_pending",
    variant: "secondary",
    icon: Clock,
  },
};

/** Subscriber-facing order status pill (list rows and the detail page). */
export function OrderStatusBadge({
  status,
  className,
}: {
  status: string;
  className?: string;
}) {
  const { t } = useLocale();
  const config = STATUS_CONFIG[status] ?? STATUS_CONFIG.pending;
  const Icon = config.icon;
  return (
    <Badge variant={config.variant} className={cn("gap-1 text-xs", className)}>
      <Icon className="h-3 w-3" />
      {t(config.labelKey)}
    </Badge>
  );
}
