"use client";

import type { ReactNode } from "react";

import { EntityUnavailable } from "@/components/entity";
import { useLocale } from "@/providers/locale-provider";

import { useCompanyStructureAccess } from "../lib/access";

/** Company structure only exists for company accounts with `app.company_structure.read`. */
export function CompanyStructureGate({ children }: { children: ReactNode }) {
  const { t } = useLocale();
  const access = useCompanyStructureAccess();
  if (!access.isCompany) {
    return (
      <EntityUnavailable
        title={t("company_structure.unavailable.personal_title")}
        description={t("company_structure.unavailable.personal_description")}
      />
    );
  }
  if (!access.canRead) {
    return (
      <EntityUnavailable
        title={t("common.error_forbidden")}
        description={t("company_structure.unavailable.forbidden_description")}
      />
    );
  }
  return <>{children}</>;
}
