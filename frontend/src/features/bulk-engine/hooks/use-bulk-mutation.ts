import { useMutation, useQueryClient } from "@tanstack/react-query";

import { bulkService } from "@/features/bulk-engine/services/bulk.service";
import { BULK_JOB_PATHS } from "@/features/bulk-engine/types";
import type {
  BulkExecuteSyncResult,
  BulkJob,
  BulkResource,
  BulkTarget,
} from "@/features/bulk-engine/types";
import { useLocale } from "@/providers/locale-provider";
import { toast } from "sonner";

function isSyncResult(
  data: BulkJob | BulkExecuteSyncResult,
): data is BulkExecuteSyncResult {
  return "sync" in data && data.sync === true;
}

type ExecuteInput = {
  resource: BulkResource;
  action: string;
  target: BulkTarget;
  /** The action can be rolled back; enables the undo toast where the resource supports it. */
  reversible?: boolean;
  onComplete?: () => void;
};

export function useBulkMutation() {
  const { locale, t } = useLocale();
  const queryClient = useQueryClient();

  const undo = async (uuid: string, variables: ExecuteInput) => {
    try {
      await bulkService.rollback(uuid, variables.resource);
      toast.success(t("bulk.rollback_success"));
    } finally {
      // A failed rollback is reported by the global API error toast.
      variables.onComplete?.();
    }
  };

  return useMutation({
    mutationFn: async ({ resource, action, target }: ExecuteInput) => {
      return bulkService.execute(resource, {
        action,
        target,
        locale,
      });
    },
    onSuccess: (data, variables) => {
      if (isSyncResult(data)) {
        const job = data.job;
        const canUndo =
          variables.reversible === true &&
          BULK_JOB_PATHS[variables.resource] !== undefined &&
          job !== undefined &&
          data.summary.succeeded > 0;
        toast.success(
          t("bulk.result.sync", {
            succeeded: data.summary.succeeded,
            failed: data.summary.failed,
          }),
          canUndo
            ? {
                action: {
                  label: t("bulk.undo"),
                  onClick: () => void undo(job.uuid, variables).catch(() => {}),
                },
              }
            : undefined,
        );
        variables.onComplete?.();
        return;
      }
      toast.info(t("bulk.job.queued"));
      void queryClient.invalidateQueries({ queryKey: ["bulk-jobs"] });
      variables.onComplete?.();
    },
    onError: () => {
      toast.error(t("bulk.job.failed"));
    },
  });
}

export function useBulkRollbackMutation() {
  const { t } = useLocale();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (uuid: string) => bulkService.rollback(uuid),
    onSuccess: () => {
      toast.success(t("bulk.rollback_success"));
      void queryClient.invalidateQueries({ queryKey: ["bulk-jobs"] });
    },
    onError: () => {
      toast.error(t("common.error_generic"));
    },
  });
}
