"use client";

import { useEffect } from "react";

import { ErrorState } from "@/components/common/error-state";
import { reportError } from "@/lib/error-tracking/catcher";
import { useLocale } from "@/providers/locale-provider";

export default function NotificationsError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const { t } = useLocale();

  useEffect(() => {
    reportError(error);
  }, [error]);

  return (
    <ErrorState
      title={t("common.error_generic")}
      description={error.message || t("notifications.error_boundary")}
      onRetry={reset}
      retryLabel={t("common.retry")}
    />
  );
}
