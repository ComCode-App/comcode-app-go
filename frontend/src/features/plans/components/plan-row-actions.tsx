"use client";

import { Eye, Pencil, Trash2 } from "lucide-react";
import { useMemo } from "react";

import { EntityRowActions, type EntityRowAction } from "@/components/entity";
import { permissions } from "@/config/permissions";
import type { PlanSummary } from "@/features/plans/services/plans.service";
import { useLocale } from "@/providers/locale-provider";

export type PlanRowActionHandlers = {
  onView: (plan: PlanSummary) => void;
  onEdit: (plan: PlanSummary) => void;
  onDelete: (plan: PlanSummary) => void;
};

type PlanRowActionsProps = {
  plan: PlanSummary;
  handlers: PlanRowActionHandlers;
};

export function PlanRowActionsMenu({ plan, handlers }: PlanRowActionsProps) {
  const { t } = useLocale();
  const isProtected = plan.slug === "free";

  const actions = useMemo<EntityRowAction[]>(() => {
    const items: EntityRowAction[] = [
      {
        id: "view",
        label: t("plans.actions.view"),
        icon: Eye,
        permission: permissions.plans.read,
        onSelect: () => handlers.onView(plan),
      },
      {
        id: "edit",
        label: t("plans.actions.edit"),
        icon: Pencil,
        permission: permissions.plans.write,
        onSelect: () => handlers.onEdit(plan),
      },
    ];

    if (!isProtected) {
      items.push({
        id: "delete",
        label: t("plans.actions.delete"),
        icon: Trash2,
        permission: permissions.plans.write,
        variant: "destructive",
        onSelect: () => handlers.onDelete(plan),
      });
    }

    return items;
  }, [handlers, isProtected, plan, t]);

  return <EntityRowActions actions={actions} />;
}
