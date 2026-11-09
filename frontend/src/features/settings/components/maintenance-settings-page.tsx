"use client";

import { PurgeRulesPanel } from "@/features/logs/components/purge-rules-panel";
import { useLocale } from "@/providers/locale-provider";

export function MaintenanceSettingsPage() {
  const { t } = useLocale();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-foreground text-xl font-bold tracking-tight md:text-2xl">
          {t("settings.subnav.purge_rules")}
        </h1>
        <p className="text-muted-foreground mt-1 text-sm">
          {t("settings.system.maintenance_description")}
        </p>
      </div>

      <PurgeRulesPanel />
    </div>
  );
}
