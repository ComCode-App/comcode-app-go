"use client";

import { RefreshCw, Undo2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useLocale } from "@/providers/locale-provider";

import {
  useCompanyBulkJobs,
  useCompanyBulkRollback,
} from "../hooks/use-company-bulk";
import { canRollback, JobStatusBadge } from "./bulk-action-dialog";

/** Recent bulk jobs of the active company with progress and undo (TEC-573). */
export function BulkJobsCard() {
  const { t } = useLocale();
  const jobsQuery = useCompanyBulkJobs();
  const rollback = useCompanyBulkRollback();
  const jobs = jobsQuery.data?.items ?? [];

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-2 space-y-0">
        <div className="space-y-1">
          <CardTitle className="text-base">
            {t("company_bulk.jobs.title")}
          </CardTitle>
          <CardDescription>{t("company_bulk.jobs.description")}</CardDescription>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t("entity.refresh")}
          disabled={jobsQuery.isFetching}
          onClick={() => void jobsQuery.refetch()}
        >
          <RefreshCw className="size-4" />
        </Button>
      </CardHeader>
      <CardContent>
        {jobsQuery.isLoading ? (
          <p className="text-muted-foreground text-sm">{t("common.loading")}</p>
        ) : jobs.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {t("company_bulk.jobs.empty")}
          </p>
        ) : (
          <ul className="divide-y">
            {jobs.map((job) => (
              <li
                key={job.uuid}
                className="flex flex-wrap items-center gap-x-3 gap-y-1 py-2 text-sm"
              >
                <span className="min-w-0 flex-1 font-medium">
                  {t(`bulk.actions.company_profiles.${job.action}`)}
                </span>
                <JobStatusBadge status={job.status} />
                {job.summary ? (
                  <span className="text-muted-foreground text-xs">
                    {t("bulk.result.sync", {
                      succeeded: job.summary.succeeded,
                      failed: job.summary.failed,
                    })}
                  </span>
                ) : null}
                <span className="text-muted-foreground text-xs">
                  {new Date(job.created_at).toLocaleString()}
                </span>
                {canRollback(job) ? (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-7"
                    disabled={rollback.isPending}
                    onClick={() => rollback.mutate(job.uuid)}
                  >
                    <Undo2 className="me-1 size-3.5" />
                    {t("bulk.rollback")}
                  </Button>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
