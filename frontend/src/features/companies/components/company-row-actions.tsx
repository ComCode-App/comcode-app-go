"use client";

import { Eye } from "lucide-react";
import { useMemo } from "react";

import { EntityRowActions, type EntityRowAction } from "@/components/entity";
import { permissions } from "@/config/permissions";
import type { CompanySummary } from "@/features/companies/services/companies.service";
import { useLocale } from "@/providers/locale-provider";

export type CompanyRowActionHandlers = {
  onView: (company: CompanySummary) => void;
};

type CompanyRowActionsProps = {
  company: CompanySummary;
  handlers: CompanyRowActionHandlers;
};

export function CompanyRowActionsMenu({
  company,
  handlers,
}: CompanyRowActionsProps) {
  const { t } = useLocale();

  const actions = useMemo<EntityRowAction[]>(() => {
    const items: EntityRowAction[] = [
      {
        id: "view",
        label: t("companies.actions.view"),
        icon: Eye,
        permission: permissions.companies.read,
        onSelect: () => handlers.onView(company),
      },
    ];

    return items;
  }, [handlers, company, t]);

  return <EntityRowActions actions={actions} />;
}
