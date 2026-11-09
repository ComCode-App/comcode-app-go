"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader2, Undo2 } from "lucide-react";
import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { BulkActionDef, BulkJob, BulkTarget } from "@/features/bulk-engine";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { companyBulkKeys } from "../hooks/query-keys";
import {
  useCompanyBulkJob,
  useCompanyBulkRollback,
} from "../hooks/use-company-bulk";
import { itemErrorText, requestErrorText } from "../lib/errors";
import {
  type CompanyBulkAction,
  companyBulkService,
  resultJob,
} from "../services/company-bulk.service";
import {
  ActionForm,
  buildAction,
  initialActionForm,
} from "./action-form";

type Step = "configure" | "preview" | "result";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  actions: BulkActionDef[];
  target: BulkTarget | null;
  /** Profiles the target is expected to reach (shown before running). */
  targetCount: number;
  /** Profile uuid → member name, for item errors. */
  names: Map<string, string>;
};

export function BulkActionDialog(props: Props) {
  // Remount per opening so every run starts from a clean form.
  return props.open ? <BulkActionDialogBody {...props} /> : null;
}

function BulkActionDialogBody({
  open,
  onOpenChange,
  actions,
  target,
  targetCount,
  names,
}: Props) {
  const { t, locale } = useLocale();
  const qc = useQueryClient();
  const [step, setStep] = useState<Step>("configure");
  const [action, setAction] = useState<CompanyBulkAction>(
    (actions[0]?.id as CompanyBulkAction) ?? "set_contact",
  );
  const [form, setForm] = useState(initialActionForm);
  const [jobUuid, setJobUuid] = useState<string | null>(null);
  const [initialJob, setInitialJob] = useState<BulkJob | null>(null);

  const built = useMemo(() => buildAction(action, form, t), [action, form, t]);
  const def = actions.find((a) => a.id === action);

  const run = useMutation({
    meta: { silentErrors: true },
    mutationFn: () => {
      if (!target) throw new Error("no target");
      return companyBulkService.execute({
        action,
        target,
        params: built.params,
        locale,
      });
    },
    onSuccess: (result) => {
      const job = resultJob(result);
      setInitialJob(job);
      setJobUuid(job.uuid);
      setStep("result");
      void qc.invalidateQueries({ queryKey: companyBulkKeys.all });
    },
    onError: (err) => appToast.error(requestErrorText(t, err)),
  });

  const jobQuery = useCompanyBulkJob(jobUuid);
  const job = jobQuery.data ?? initialJob;
  const rollback = useCompanyBulkRollback();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("company_bulk.dialog.title")}</DialogTitle>
          <DialogDescription>
            {t("company_bulk.dialog.description", { count: targetCount })}
          </DialogDescription>
          <ol className="flex flex-wrap gap-2 pt-1 text-xs">
            {(["configure", "preview", "result"] as const).map((s) => (
              <li
                key={s}
                className={cn(
                  "rounded-full border px-2.5 py-0.5",
                  s === step
                    ? "border-primary bg-primary/10 text-primary font-medium"
                    : "text-muted-foreground",
                )}
              >
                {t(`company_bulk.dialog.step_${s}`)}
              </li>
            ))}
          </ol>
        </DialogHeader>

        {step === "configure" ? (
          <div className="space-y-4">
            <div className="grid gap-2 sm:grid-cols-2">
              {actions.map((a) => {
                const Icon = a.icon;
                return (
                  <button
                    key={a.id}
                    type="button"
                    onClick={() => setAction(a.id as CompanyBulkAction)}
                    aria-pressed={a.id === action}
                    className={cn(
                      "flex items-center gap-2 rounded-md border p-3 text-start text-sm transition-colors",
                      a.id === action
                        ? "border-primary bg-primary/5"
                        : "hover:bg-muted/50",
                    )}
                  >
                    <Icon className="text-muted-foreground size-4 shrink-0" />
                    {t(a.label_key)}
                  </button>
                );
              })}
            </div>
            <ActionForm action={action} state={form} onChange={setForm} />
          </div>
        ) : null}

        {step === "preview" ? (
          <div className="space-y-3 text-sm">
            <p className="font-medium">
              {def?.confirm_key
                ? t(def.confirm_key, { count: targetCount })
                : t("bulk.confirm_generic", { count: targetCount })}
            </p>
            <ul className="bg-muted/40 list-disc space-y-1 rounded-md border py-2 ps-6 pe-3">
              {built.summary.map((line) => (
                <li key={line} className="break-words">
                  {line}
                </li>
              ))}
            </ul>
            <p className="text-muted-foreground text-xs">
              {t("company_bulk.preview.scope_hint")}
            </p>
            <p className="text-muted-foreground text-xs">
              {t("company_bulk.preview.rollback_hint")}
            </p>
          </div>
        ) : null}

        {step === "result" && job ? (
          <JobResult job={job} names={names} />
        ) : null}

        <DialogFooter className="gap-2 sm:gap-0">
          {step === "configure" ? (
            <>
              {built.error ? (
                <p className="text-destructive me-auto self-center text-xs">
                  {built.error}
                </p>
              ) : null}
              <Button
                type="button"
                variant="ghost"
                onClick={() => onOpenChange(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button
                type="button"
                disabled={Boolean(built.error)}
                onClick={() => setStep("preview")}
              >
                {t("company_bulk.dialog.to_preview")}
              </Button>
            </>
          ) : null}
          {step === "preview" ? (
            <>
              <Button
                type="button"
                variant="outline"
                onClick={() => setStep("configure")}
              >
                {t("imports.back")}
              </Button>
              <Button
                type="button"
                disabled={run.isPending || !target}
                onClick={() => run.mutate()}
              >
                {run.isPending ? (
                  <Loader2 className="me-1.5 size-4 animate-spin" />
                ) : null}
                {t("company_bulk.dialog.run")}
              </Button>
            </>
          ) : null}
          {step === "result" && job ? (
            <>
              {canRollback(job) ? (
                <Button
                  type="button"
                  variant="outline"
                  disabled={rollback.isPending}
                  onClick={() =>
                    rollback.mutate(job.uuid, {
                      onSuccess: () => void jobQuery.refetch(),
                    })
                  }
                >
                  <Undo2 className="me-1.5 size-4" />
                  {t("bulk.rollback")}
                </Button>
              ) : null}
              <Button type="button" onClick={() => onOpenChange(false)}>
                {t("common.close")}
              </Button>
            </>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function canRollback(job: BulkJob) {
  return (
    job.status === "completed" &&
    (job.summary?.succeeded ?? 0) > 0 &&
    Boolean(job.rollback_until) &&
    new Date(job.rollback_until as string).getTime() > Date.now()
  );
}

export function JobStatusBadge({ status }: { status: string }) {
  const { t } = useLocale();
  const variant =
    status === "completed"
      ? "success"
      : status === "failed"
        ? "danger"
        : status === "rolled_back_partial"
          ? "warning"
          : "secondary";
  return (
    <Badge variant={variant}>{t(`company_bulk.status.${status}`)}</Badge>
  );
}

function JobResult({
  job,
  names,
}: {
  job: BulkJob;
  names: Map<string, string>;
}) {
  const { t } = useLocale();
  const s = job.summary;
  const running = job.status === "queued" || job.status === "processing";
  return (
    <div className="space-y-3 text-sm">
      <div className="flex items-center gap-2">
        {running ? <Loader2 className="size-4 animate-spin" /> : null}
        <JobStatusBadge status={job.status} />
        {running ? (
          <span className="text-muted-foreground text-xs">
            {t("company_bulk.dialog.running")}
          </span>
        ) : null}
      </div>
      {s && !running ? (
        <div className="grid grid-cols-3 gap-2">
          <Stat label={t("imports.preview_total")} value={s.total} />
          <Stat
            label={t("company_bulk.result.succeeded")}
            value={s.succeeded}
            className="text-green-600 dark:text-green-400"
          />
          <Stat
            label={t("company_bulk.result.failed")}
            value={s.failed}
            className="text-destructive"
          />
        </div>
      ) : null}
      {s?.errors?.length ? (
        <ul className="text-destructive max-h-40 list-disc space-y-0.5 overflow-y-auto ps-5 text-xs">
          {s.errors.map((e) => (
            <li key={e.entity_uuid}>
              {names.get(e.entity_uuid) ?? e.entity_uuid.slice(0, 8)}:{" "}
              {itemErrorText(t, e.error)}
            </li>
          ))}
        </ul>
      ) : null}
      {job.error ? <p className="text-destructive text-xs">{job.error}</p> : null}
      {canRollback(job) ? (
        <p className="text-muted-foreground text-xs">
          {t("bulk.rollback_until", {
            date: new Date(job.rollback_until as string).toLocaleString(),
          })}
        </p>
      ) : null}
    </div>
  );
}

function Stat({
  label,
  value,
  className,
}: {
  label: string;
  value: number;
  className?: string;
}) {
  return (
    <div className="rounded-md border p-2 text-center">
      <div className={cn("font-medium", className)}>{value}</div>
      <div className="text-muted-foreground text-xs">{label}</div>
    </div>
  );
}
