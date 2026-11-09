"use client";

import { Eye } from "lucide-react";
import { useMemo } from "react";

import { EntityRowActions, type EntityRowAction } from "@/components/entity";
import { permissions } from "@/config/permissions";
import type { SubscriptionSummary } from "@/features/subscriptions/services/subscriptions.service";
import { useLocale } from "@/providers/locale-provider";

export type SubscriptionRowActionHandlers = {
  onView: (subscription: SubscriptionSummary) => void;
};

type SubscriptionRowActionsProps = {
  subscription: SubscriptionSummary;
  handlers: SubscriptionRowActionHandlers;
};

export function SubscriptionRowActionsMenu({
  subscription,
  handlers,
}: SubscriptionRowActionsProps) {
  const { t } = useLocale();

  const actions = useMemo<EntityRowAction[]>(
    () => [
      {
        id: "view",
        label: t("subscriptions.actions.view"),
        icon: Eye,
        permission: permissions.subscriptions.read,
        onSelect: () => handlers.onView(subscription),
      },
    ],
    [handlers, subscription, t],
  );

  return <EntityRowActions actions={actions} />;
}
