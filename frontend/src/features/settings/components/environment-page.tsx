"use client";

import { Cpu, Server } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useSystemInfo } from "@/features/settings/hooks/use-system-info";
import { useLocale } from "@/providers/locale-provider";

export function EnvironmentPage() {
  const { t } = useLocale();
  const { data: info, isLoading, isError, refetch } = useSystemInfo();

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

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-foreground text-xl font-bold tracking-tight md:text-2xl">
          {t("settings.subnav.system_environment")}
        </h1>
        <p className="text-muted-foreground mt-1 text-sm">
          {t("settings.system.env_description")}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {/* Environment Specs */}
        <Card className="border-border/60 shadow-xs">
          <CardHeader>
            <div className="flex items-center gap-2">
              <div className="bg-primary/10 text-primary rounded-lg p-2">
                <Server className="size-4" />
              </div>
              <div>
                <CardTitle className="text-base font-semibold">
                  {t("settings.system.env_app_config")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("settings.system.env_app_config_sub")}
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-xs">
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">App Name</span>
              <span className="text-foreground font-mono font-medium">
                {info.environment.app_name || "comcode-app-go"}
              </span>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Deployment Stage</span>
              <Badge
                variant="outline"
                className="font-mono text-[10px] uppercase"
              >
                {info.environment.env || "production"}
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
              <span className="text-muted-foreground">Server Local Time</span>
              <span className="text-muted-foreground font-mono">
                {new Date(info.runtime.server_time).toLocaleString()}
              </span>
            </div>
          </CardContent>
        </Card>

        {/* Go Runtime Specs */}
        <Card className="border-border/60 shadow-xs">
          <CardHeader>
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-blue-500/10 p-2 text-blue-600 dark:text-blue-400">
                <Cpu className="size-4" />
              </div>
              <div>
                <CardTitle className="text-base font-semibold">
                  {t("settings.system.stat_runtime")}
                </CardTitle>
                <CardDescription className="text-xs">
                  Go runtime environment & target compilation
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-3 text-xs">
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Go Version</span>
              <span className="text-foreground font-mono font-semibold">
                {info.runtime.go_version}
              </span>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">OS / Architecture</span>
              <span className="font-mono font-medium">
                {info.runtime.os} / {info.runtime.arch}
              </span>
            </div>
            <div className="border-border/40 flex items-center justify-between border-b py-1.5">
              <span className="text-muted-foreground">Compiler</span>
              <span className="font-mono">{info.runtime.compiler}</span>
            </div>
            <div className="flex items-center justify-between py-1.5">
              <span className="text-muted-foreground">CPU Core Allocation</span>
              <span className="font-mono font-semibold">
                {info.runtime.num_cpu} logical cores
              </span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
