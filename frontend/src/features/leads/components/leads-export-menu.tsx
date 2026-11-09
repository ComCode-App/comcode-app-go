"use client";

import { useQueryClient } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { entitlementKeys, useQuota } from "@/features/entitlements";
import type { ExportFormat, ExportJob } from "@/features/io/types";
import { formatResetDate } from "@/features/leads/components/leads-quota-meter";
import { leadsService } from "@/features/leads/services/leads.service";
import { isPlanLimitError, suppressApiErrorToast } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const FORMATS: ExportFormat[] = ["xlsx", "csv", "pdf", "json"];
const POLL_MS = 1500;
const POLL_TRIES = 20;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Exports the active account's leads (io-engine resource `app.leads`). The job usually finishes
 * in a few seconds, so the menu waits for it and downloads; a slow job is announced by the
 * `exports.ready` notification instead. Each export takes one from the monthly export quota
 * (TEC-941): the button is disabled once it is used up and shows what is left.
 */
export function LeadsExportMenu({
  query,
}: {
  query: Record<string, string | undefined>;
}) {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState(false);
  const quota = useQuota("leads.export.monthly_max");
  const limit = quota?.int_value ?? 0;
  const used = quota?.used ?? 0;
  const off =
    Boolean(quota) && !quota?.unlimited && (!quota?.included || limit <= 0);
  const exhausted = Boolean(quota?.reached);
  let hint: string | null = null;
  if (off) hint = t("leads.export.off");
  else if (exhausted)
    hint = t("leads.export.quota_reached", {
      used,
      limit,
      date: formatResetDate(quota?.resets_at, locale),
    });
  else if (quota && !quota.unlimited)
    hint = t("leads.export.remaining", { count: Math.max(0, limit - used) });

  const run = async (format: ExportFormat) => {
    setPending(true);
    appToast.info(t("leads.export.preparing"));
    try {
      const cleaned: Record<string, string> = {};
      for (const [key, value] of Object.entries(query))
        if (value) cleaned[key] = value;
      let job: ExportJob = await leadsService.requestExport({
        format,
        locale,
        query: cleaned,
      });
      for (
        let i = 0;
        i < POLL_TRIES && job.status !== "completed" && job.status !== "failed";
        i++
      ) {
        await sleep(POLL_MS);
        job = await leadsService.getExport(job.uuid);
      }
      if (job.status === "completed") {
        await leadsService.downloadExport(job);
        appToast.success(t("leads.export.ready"));
      } else if (job.status === "failed") {
        appToast.error(t("leads.export.failed"));
      } else {
        appToast.info(t("leads.export.pending"));
      }
    } catch (error) {
      // 403 PLAN_LIMIT_EXCEEDED keeps its global upgrade toast; any other error shows only the
      // export-specific text (TEC-1117).
      if (!isPlanLimitError(error)) {
        suppressApiErrorToast(error);
        appToast.error(t("leads.export.failed"));
      }
    } finally {
      setPending(false);
      void queryClient.invalidateQueries({
        queryKey: entitlementKeys.overview,
      });
    }
  };

  return (
    <div className="flex flex-col items-start gap-1 md:items-end">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="outline"
            className="gap-2"
            disabled={pending || off || exhausted}
          >
            <Download className="size-4" />
            {t("leads.export.label")}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {FORMATS.map((format) => (
            <DropdownMenuItem key={format} onClick={() => void run(format)}>
              {t(`exports.formats.${format}`)}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {hint ? (
        <span className="text-muted-foreground max-w-xs text-xs md:text-right">
          {hint}
        </span>
      ) : null}
    </div>
  );
}
