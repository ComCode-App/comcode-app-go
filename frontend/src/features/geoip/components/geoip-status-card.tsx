"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  CheckCircle2,
  Database,
  DownloadCloud,
  FileCode,
  Layers,
  RefreshCw,
  Server,
  Trash2,
  XCircle,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { permissions } from "@/config/permissions";
import type { GeoIPStatus } from "@/features/geoip/services/geoip.service";
import { geoipService } from "@/features/geoip/services/geoip.service";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

type GeoIPStatusCardProps = {
  status: GeoIPStatus;
  queryKey: readonly unknown[];
};

export function GeoIPStatusCard({ status, queryKey }: GeoIPStatusCardProps) {
  const { t } = useLocale();
  const { can } = usePermission();
  const queryClient = useQueryClient();
  const canManage = can(permissions.geoip.manage);

  const [isUpdating, setIsUpdating] = useState(false);
  const [isClearing, setIsClearing] = useState(false);

  const updateMutation = useMutation({
    mutationFn: () => geoipService.updateNow(),
    onMutate: () => setIsUpdating(true),
    onSettled: () => setIsUpdating(false),
    onSuccess: async (res) => {
      await queryClient.invalidateQueries({ queryKey });
      appToast.success(
        res?.version
          ? `${t("geoip.toast.update_success")} (${res.version})`
          : t("geoip.toast.update_success"),
      );
    },
    onError: (error) => {
      if (isApiError(error)) {
        appToast.error(error.message);
      } else {
        appToast.error(t("geoip.toast.update_failed"));
      }
    },
  });

  const clearCacheMutation = useMutation({
    mutationFn: () => geoipService.clearCache(),
    onMutate: () => setIsClearing(true),
    onSettled: () => setIsClearing(false),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey });
      appToast.success(t("geoip.toast.cache_cleared"));
    },
    onError: (error) => {
      if (isApiError(error)) {
        appToast.error(error.message);
      } else {
        appToast.error(t("geoip.toast.cache_clear_failed"));
      }
    },
  });

  const isLoaded = Boolean(status.database_loaded);
  const isEnabled = Boolean(status.enabled);

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-4">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-lg font-semibold">
            <Server className="text-primary size-5" />
            {t("geoip.status.title")}
          </CardTitle>
          <CardDescription>{t("geoip.status.description")}</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={isEnabled ? "success" : "secondary"}>
            {isEnabled ? t("common.active") : t("common.inactive")}
          </Badge>
          <Badge variant={isLoaded ? "success" : "danger"}>
            {isLoaded ? t("geoip.status.loaded") : t("geoip.status.not_loaded")}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div className="bg-muted/30 flex items-start gap-3 rounded-xl border p-3.5">
            <Database className="text-muted-foreground mt-0.5 size-5" />
            <div className="space-y-1 overflow-hidden">
              <p className="text-muted-foreground text-xs font-medium">
                {t("geoip.status.provider")}
              </p>
              <p className="text-sm font-semibold capitalize">
                {status.provider || "MaxMind"}
              </p>
            </div>
          </div>

          <div className="bg-muted/30 flex items-start gap-3 rounded-xl border p-3.5">
            <FileCode className="text-muted-foreground mt-0.5 size-5" />
            <div className="space-y-1 overflow-hidden">
              <p className="text-muted-foreground text-xs font-medium">
                {t("geoip.status.database_version")}
              </p>
              <p
                className="truncate text-sm font-semibold"
                title={status.database_version || "N/A"}
              >
                {status.database_version || "N/A"}
              </p>
            </div>
          </div>

          <div className="bg-muted/30 flex items-start gap-3 rounded-xl border p-3.5">
            <Layers className="text-muted-foreground mt-0.5 size-5" />
            <div className="space-y-1 overflow-hidden">
              <p className="text-muted-foreground text-xs font-medium">
                {t("geoip.status.cache_entries")}
              </p>
              <p className="text-sm font-semibold">
                {status.cache_size?.toLocaleString() ?? 0}
              </p>
            </div>
          </div>

          <div className="bg-muted/30 flex items-start gap-3 rounded-xl border p-3.5">
            {status.last_update_status === "success" ? (
              <CheckCircle2 className="mt-0.5 size-5 text-emerald-600" />
            ) : status.last_update_status === "failed" ? (
              <XCircle className="mt-0.5 size-5 text-rose-600" />
            ) : (
              <RefreshCw className="mt-0.5 size-5 text-amber-600" />
            )}
            <div className="space-y-1 overflow-hidden">
              <p className="text-muted-foreground text-xs font-medium">
                {t("geoip.status.last_update_status")}
              </p>
              <div className="flex items-center gap-1.5">
                <span className="text-sm font-semibold capitalize">
                  {status.last_update_status || "Pending"}
                </span>
                {status.last_update_at ? (
                  <span className="text-muted-foreground text-xs">
                    ({new Date(status.last_update_at).toLocaleDateString()})
                  </span>
                ) : null}
              </div>
            </div>
          </div>
        </div>

        <div className="text-muted-foreground flex flex-col items-start justify-between gap-4 border-t pt-2 text-xs sm:flex-row sm:items-center">
          <div className="flex max-w-xl items-center gap-2 truncate">
            <span className="text-foreground font-medium">
              {t("geoip.status.database_path")}:
            </span>
            <code className="bg-muted rounded px-1.5 py-0.5 text-xs">
              {status.database_path || "/data/geoip/GeoLite2-City.mmdb"}
            </code>
          </div>

          {canManage ? (
            <div className="flex items-center gap-2 self-end sm:self-auto">
              <Button
                variant="outline"
                size="sm"
                onClick={() => clearCacheMutation.mutate()}
                disabled={isClearing || isUpdating}
              >
                <Trash2 className="mr-1.5 size-3.5" />
                {isClearing
                  ? t("geoip.status.clearing")
                  : t("geoip.status.clear_cache")}
              </Button>

              <Button
                size="sm"
                onClick={() => updateMutation.mutate()}
                disabled={isUpdating || isClearing}
              >
                <DownloadCloud
                  className={`mr-1.5 size-3.5 ${isUpdating ? "animate-pulse" : ""}`}
                />
                {isUpdating
                  ? t("geoip.status.updating")
                  : t("geoip.status.update_now")}
              </Button>
            </div>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}
