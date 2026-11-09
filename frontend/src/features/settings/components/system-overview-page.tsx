"use client";

import { useState } from "react";
import {
  Clock,
  Copy,
  Cpu,
  Database,
  Gauge,
  HardDrive,
  Layers,
  Radio,
  RefreshCw,
  Search,
  Zap,
} from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  useSystemInfo,
  useSystemPing,
} from "@/features/settings/hooks/use-system-info";
import type { ServiceHealthStatus } from "@/features/settings/types";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

function formatUptime(seconds: number): string {
  if (!seconds) return "0s";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);

  const parts = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  if (secs > 0 || parts.length === 0) parts.push(`${secs}s`);

  return parts.join(" ");
}

function getStatusBadge(
  status: ServiceHealthStatus,
  t: (key: string) => string,
) {
  switch (status) {
    case "healthy":
      return (
        <Badge
          variant="outline"
          className="border-emerald-500/30 bg-emerald-500/10 font-medium text-emerald-600 dark:text-emerald-400"
        >
          <span className="mr-1.5 size-1.5 rounded-full bg-emerald-500" />
          {t("settings.system.status_healthy")}
        </Badge>
      );
    case "degraded":
      return (
        <Badge
          variant="outline"
          className="border-amber-500/30 bg-amber-500/10 font-medium text-amber-600 dark:text-amber-400"
        >
          <span className="mr-1.5 size-1.5 rounded-full bg-amber-500" />
          {t("settings.system.status_degraded")}
        </Badge>
      );
    case "down":
      return (
        <Badge variant="danger" className="font-medium">
          <span className="bg-destructive mr-1.5 size-1.5 animate-pulse rounded-full" />
          {t("settings.system.status_down")}
        </Badge>
      );
    case "disabled":
    default:
      return (
        <Badge
          variant="secondary"
          className="text-muted-foreground font-medium"
        >
          {t("settings.system.status_disabled")}
        </Badge>
      );
  }
}

export function SystemOverviewPage() {
  const { t } = useLocale();
  const { data: info, isLoading, isError, refetch } = useSystemInfo();
  const pingMutation = useSystemPing();
  const [copied, setCopied] = useState(false);

  const handleCopyDiagnostics = () => {
    if (!info) return;
    navigator.clipboard.writeText(JSON.stringify(info, null, 2));
    setCopied(true);
    appToast.success(t("settings.system.toast_copied"));
    setTimeout(() => setCopied(false), 2000);
  };

  if (isLoading) {
    return <Loading label={t("common.loading")} />;
  }

  if (isError || !info) {
    return (
      <ErrorState
        title={t("settings.system.error_title")}
        description={t("settings.system.error_description")}
        onRetry={() => void refetch()}
        retryLabel={t("common.retry")}
      />
    );
  }

  const isHealthy = info.status === "healthy";
  const isDegraded = info.status === "degraded";

  const dbPool = info.services.database.pool;
  const poolMax = dbPool?.max_conns || 25;
  const poolAcquired = dbPool?.acquired_conns || 0;
  const poolUsagePercent = Math.min(
    100,
    Math.round((poolAcquired / Math.max(1, poolMax)) * 100),
  );

  const heapAlloc = info.runtime.memory.heap_alloc_bytes;
  const heapSys = info.runtime.memory.heap_sys_bytes;
  const memoryPercent =
    heapSys > 0 ? Math.min(100, Math.round((heapAlloc / heapSys) * 100)) : 0;

  return (
    <div className="space-y-6">
      {/* Top Header Card */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-foreground text-xl font-bold tracking-tight md:text-2xl">
              {t("settings.system.title")}
            </h1>
            <Badge
              variant="outline"
              className={cn(
                "text-[10px] font-semibold tracking-wider uppercase",
                isHealthy
                  ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                  : isDegraded
                    ? "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400"
                    : "border-rose-500/40 bg-rose-500/10 text-rose-600 dark:text-rose-400",
              )}
            >
              <span
                className={cn("mr-1.5 size-1.5 rounded-full", {
                  "animate-pulse bg-emerald-500": isHealthy,
                  "bg-amber-500": isDegraded,
                  "bg-rose-500": !isHealthy && !isDegraded,
                })}
              />
              {isHealthy
                ? t("settings.system.status_operational")
                : isDegraded
                  ? t("settings.system.status_degraded")
                  : t("settings.system.status_down")}
            </Badge>
          </div>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("settings.system.description")}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleCopyDiagnostics}
            className="text-xs"
          >
            <Copy className="mr-1.5 size-3.5" />
            {copied
              ? t("settings.system.action_copied")
              : t("settings.system.action_copy_json")}
          </Button>
          <Button
            size="sm"
            onClick={() => pingMutation.mutate()}
            disabled={pingMutation.isPending}
            className="text-xs"
          >
            <RefreshCw
              className={cn("mr-1.5 size-3.5", {
                "animate-spin": pingMutation.isPending,
              })}
            />
            {t("settings.system.action_run_probe")}
          </Button>
        </div>
      </div>

      {/* KPI Stats Grid */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {/* Card 1: Server Uptime */}
        <Card className="border-border/60 relative overflow-hidden shadow-xs">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
              {t("settings.system.stat_uptime")}
            </CardTitle>
            <div className="bg-primary/10 text-primary rounded-lg p-2">
              <Clock className="size-4" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-xl font-bold tracking-tight">
              {formatUptime(info.runtime.uptime_seconds)}
            </div>
            <p className="text-muted-foreground mt-1 flex items-center gap-1.5 text-xs">
              <span className="inline-block size-1.5 rounded-full bg-emerald-500" />
              {t("settings.system.stat_uptime_sub")}
            </p>
          </CardContent>
        </Card>

        {/* Card 2: Go Runtime & CPU */}
        <Card className="border-border/60 relative overflow-hidden shadow-xs">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
              {t("settings.system.stat_runtime")}
            </CardTitle>
            <div className="rounded-lg bg-blue-500/10 p-2 text-blue-600 dark:text-blue-400">
              <Cpu className="size-4" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="font-mono text-xl font-bold tracking-tight">
              {info.runtime.go_version}
            </div>
            <p className="text-muted-foreground mt-1 text-xs">
              {info.runtime.num_cpu} {t("settings.system.cores")} •{" "}
              {info.runtime.os}/{info.runtime.arch}
            </p>
          </CardContent>
        </Card>

        {/* Card 3: Concurrency & Goroutines */}
        <Card className="border-border/60 relative overflow-hidden shadow-xs">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
              {t("settings.system.stat_goroutines")}
            </CardTitle>
            <div className="rounded-lg bg-purple-500/10 p-2 text-purple-600 dark:text-purple-400">
              <Layers className="size-4" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="font-mono text-xl font-bold tracking-tight">
              {info.runtime.num_goroutine.toLocaleString()}
            </div>
            <p className="text-muted-foreground mt-1 text-xs">
              {info.runtime.memory.num_gc} {t("settings.system.gc_cycles")}
            </p>
          </CardContent>
        </Card>

        {/* Card 4: Memory Heap */}
        <Card className="border-border/60 relative overflow-hidden shadow-xs">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
              {t("settings.system.stat_memory_heap")}
            </CardTitle>
            <div className="rounded-lg bg-emerald-500/10 p-2 text-emerald-600 dark:text-emerald-400">
              <Gauge className="size-4" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="font-mono text-xl font-bold tracking-tight">
              {formatBytes(info.runtime.memory.heap_alloc_bytes)}
            </div>
            <div className="mt-2">
              <Progress value={memoryPercent} className="h-1.5" />
            </div>
            <p className="text-muted-foreground mt-1 flex justify-between text-[11px]">
              <span>{t("settings.system.stat_sys_total")}</span>
              <span className="font-mono">
                {formatBytes(info.runtime.memory.sys_bytes)}
              </span>
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Backing Services Health Grid */}
      <div>
        <h2 className="text-foreground text-muted-foreground/90 mb-3 text-sm font-semibold tracking-tight tracking-wider uppercase">
          {t("settings.system.section_services")}
        </h2>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {/* PostgreSQL Card */}
          <Card className="border-border/60 hover:border-border shadow-xs transition-colors">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="rounded-lg bg-sky-500/10 p-2 text-sky-600 dark:text-sky-400">
                    <Database className="size-5" />
                  </div>
                  <div>
                    <CardTitle className="text-base font-semibold">
                      PostgreSQL Database
                    </CardTitle>
                    <CardDescription className="text-xs">
                      Primary transactional persistence & pool
                    </CardDescription>
                  </div>
                </div>
                {getStatusBadge(info.services.database.status, t)}
              </div>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="border-border/40 flex items-center justify-between border-b py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.latency")}
                </span>
                <span className="font-mono font-medium">
                  {info.services.database.latency_ms > 0
                    ? `${info.services.database.latency_ms} ms`
                    : "—"}
                </span>
              </div>

              {dbPool && (
                <div className="space-y-1.5 pt-1">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">
                      {t("settings.system.db_pool_utilization")}
                    </span>
                    <span className="font-mono font-semibold">
                      {dbPool.acquired_conns} / {dbPool.max_conns} (
                      {poolUsagePercent}%)
                    </span>
                  </div>
                  <Progress value={poolUsagePercent} className="h-2" />
                  <div className="text-muted-foreground flex justify-between pt-0.5 text-[11px]">
                    <span>
                      {dbPool.idle_conns} {t("settings.system.idle")}
                    </span>
                    <span>
                      {dbPool.total_conns} {t("settings.system.total_open")}
                    </span>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Redis Card */}
          <Card className="border-border/60 hover:border-border shadow-xs transition-colors">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="rounded-lg bg-red-500/10 p-2 text-red-600 dark:text-red-400">
                    <Zap className="size-5" />
                  </div>
                  <div>
                    <CardTitle className="text-base font-semibold">
                      Redis & Asynq Cache / Queue
                    </CardTitle>
                    <CardDescription className="text-xs">
                      Session cache, rate limiter & job workers
                    </CardDescription>
                  </div>
                </div>
                {getStatusBadge(info.services.redis.status, t)}
              </div>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="border-border/40 flex items-center justify-between border-b py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.latency")}
                </span>
                <span className="font-mono font-medium">
                  {info.services.redis.latency_ms > 0
                    ? `${info.services.redis.latency_ms} ms`
                    : "—"}
                </span>
              </div>
              <div className="flex items-center justify-between py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.role")}
                </span>
                <span className="text-foreground font-medium">
                  In-memory Store & Job Queue
                </span>
              </div>
            </CardContent>
          </Card>

          {/* Storage (S3/SeaweedFS) Card */}
          <Card className="border-border/60 hover:border-border shadow-xs transition-colors">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="rounded-lg bg-amber-500/10 p-2 text-amber-600 dark:text-amber-400">
                    <HardDrive className="size-5" />
                  </div>
                  <div>
                    <CardTitle className="text-base font-semibold">
                      S3 Object Storage (SeaweedFS)
                    </CardTitle>
                    <CardDescription className="text-xs">
                      File uploads, media, avatars & documents
                    </CardDescription>
                  </div>
                </div>
                {getStatusBadge(info.services.storage.status, t)}
              </div>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="border-border/40 flex items-center justify-between border-b py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.driver")}
                </span>
                <Badge
                  variant="secondary"
                  className="font-mono text-[11px] uppercase"
                >
                  {info.services.storage.driver}
                </Badge>
              </div>
              <div className="flex items-center justify-between py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.latency")}
                </span>
                <span className="font-mono font-medium">
                  {info.services.storage.latency_ms > 0
                    ? `${info.services.storage.latency_ms} ms`
                    : "—"}
                </span>
              </div>
            </CardContent>
          </Card>

          {/* Meilisearch Card */}
          <Card className="border-border/60 hover:border-border shadow-xs transition-colors">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="rounded-lg bg-pink-500/10 p-2 text-pink-600 dark:text-pink-400">
                    <Search className="size-5" />
                  </div>
                  <div>
                    <CardTitle className="text-base font-semibold">
                      Meilisearch Engine
                    </CardTitle>
                    <CardDescription className="text-xs">
                      Command palette & full-text record search
                    </CardDescription>
                  </div>
                </div>
                {getStatusBadge(info.services.meilisearch.status, t)}
              </div>
            </CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="border-border/40 flex items-center justify-between border-b py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.status")}
                </span>
                <span className="font-medium">
                  {info.environment.search_enabled
                    ? t("settings.system.feature_enabled")
                    : t("settings.system.feature_disabled")}
                </span>
              </div>
              <div className="flex items-center justify-between py-1">
                <span className="text-muted-foreground">
                  {t("settings.system.latency")}
                </span>
                <span className="font-mono font-medium">
                  {info.services.meilisearch.latency_ms > 0
                    ? `${info.services.meilisearch.latency_ms} ms`
                    : "—"}
                </span>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Detailed Memory & Runtime Breakdown */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card className="border-border/60 shadow-xs">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm font-semibold">
              <Cpu className="text-primary size-4" />
              {t("settings.system.section_memory_details")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5 text-xs">
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Heap In-Use</span>
              <span className="font-mono font-semibold">
                {formatBytes(info.runtime.memory.heap_inuse_bytes)}
              </span>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Heap Idle</span>
              <span className="font-mono">
                {formatBytes(info.runtime.memory.heap_idle_bytes)}
              </span>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">
                Total Cumulative Alloc
              </span>
              <span className="font-mono">
                {formatBytes(info.runtime.memory.total_alloc_bytes)}
              </span>
            </div>
            <div className="flex items-center justify-between py-1.5">
              <span className="text-muted-foreground">
                GC (Garbage Collector) Runs
              </span>
              <span className="font-mono font-semibold">
                {info.runtime.memory.num_gc.toLocaleString()} cycles
              </span>
            </div>
          </CardContent>
        </Card>

        <Card className="border-border/60 shadow-xs">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm font-semibold">
              <Radio className="text-primary size-4" />
              {t("settings.system.section_subsystems")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5 text-xs">
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">
                Centrifugo Realtime WebSocket
              </span>
              <Badge
                variant={
                  info.environment.centrifugo_enabled ? "outline" : "secondary"
                }
              >
                {info.environment.centrifugo_enabled
                  ? t("settings.system.feature_active")
                  : t("settings.system.feature_disabled")}
              </Badge>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Web Push VAPID</span>
              <Badge
                variant={
                  info.environment.vapid_enabled ? "outline" : "secondary"
                }
              >
                {info.environment.vapid_enabled
                  ? t("settings.system.feature_active")
                  : t("settings.system.feature_disabled")}
              </Badge>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Log Level</span>
              <Badge
                variant="secondary"
                className="font-mono text-[10px] uppercase"
              >
                {info.environment.log_level || "INFO"}
              </Badge>
            </div>
            <div className="flex items-center justify-between py-1.5">
              <span className="text-muted-foreground">
                Server UTC Timestamp
              </span>
              <span className="text-muted-foreground font-mono">
                {new Date(info.runtime.server_time).toISOString()}
              </span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
