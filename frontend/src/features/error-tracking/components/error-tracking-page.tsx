"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bug, Send } from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { permissions } from "@/config/permissions";
import {
  errorTrackingService,
  type ErrorTrackingSettings,
  type ErrorTrackingSettingsPatch,
} from "@/features/error-tracking/services/error-tracking.service";
import { isApiError } from "@/lib/api";
import {
  reloadErrorTracking,
  sendTestError,
} from "@/lib/error-tracking/catcher";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

const QUERY_KEY = ["platform", "error-tracking"] as const;

export function ErrorTrackingPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const canWrite = can(permissions.errorTracking.write);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => errorTrackingService.get(),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Bug className="size-7" />}
        title={t("settings.error_tracking.title")}
        description={t("settings.error_tracking.description")}
      />

      {isLoading ? <Loading label={t("common.loading")} /> : null}

      {isError ? (
        <ErrorState
          title={t("common.error_generic")}
          description={t("settings.error_tracking.load_failed")}
          retryLabel={t("common.retry")}
          onRetry={() => void refetch()}
        />
      ) : null}

      {data ? (
        <>
          <StatusCard settings={data} />
          {/* Remount after each save so the inputs start empty again. */}
          <SettingsForm
            key={data.updated_at}
            settings={data}
            canWrite={canWrite}
          />
          {canWrite ? <TestCard settings={data} /> : null}
        </>
      ) : null}
    </div>
  );
}

function StatusCard({ settings }: { settings: ErrorTrackingSettings }) {
  const { t } = useLocale();
  const frontendOn = settings.enabled && settings.frontend_dsn_configured;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.error_tracking.status.title")}</CardTitle>
        <CardDescription>
          {t("settings.error_tracking.status.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-2">
        <StatusRow label={t("settings.error_tracking.status.backend")}>
          <Badge variant={settings.backend_active ? "success" : "secondary"}>
            {settings.backend_active
              ? t("settings.error_tracking.status.active")
              : t("settings.error_tracking.status.inactive")}
          </Badge>
          <Badge variant="outline">
            {t(`settings.error_tracking.source.${settings.backend_source}`)}
          </Badge>
        </StatusRow>
        <StatusRow label={t("settings.error_tracking.status.frontend")}>
          <Badge variant={frontendOn ? "success" : "secondary"}>
            {frontendOn
              ? t("settings.error_tracking.status.active")
              : t("settings.error_tracking.status.inactive")}
          </Badge>
        </StatusRow>
      </CardContent>
    </Card>
  );
}

function StatusRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3">
      <span className="text-sm font-medium">{label}</span>
      <span className="flex flex-wrap items-center gap-2">{children}</span>
    </div>
  );
}

function SettingsForm({
  settings,
  canWrite,
}: {
  settings: ErrorTrackingSettings;
  canWrite: boolean;
}) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [enabled, setEnabled] = useState(settings.enabled);
  const [backendDsn, setBackendDsn] = useState("");
  const [frontendDsn, setFrontendDsn] = useState("");

  const save = useMutation({
    mutationFn: (body: ErrorTrackingSettingsPatch) =>
      errorTrackingService.patch(body),
    onSuccess: async (next) => {
      queryClient.setQueryData(QUERY_KEY, next);
      // Pick up the new frontend DSN in this browser too.
      await reloadErrorTracking();
      appToast.success(t("settings.error_tracking.toast.saved"));
    },
    onError: (error) => {
      appToast.error(
        isApiError(error)
          ? error.message
          : t("settings.error_tracking.toast.save_failed"),
      );
    },
  });

  const submit = (extra: ErrorTrackingSettingsPatch = {}) => {
    const body: ErrorTrackingSettingsPatch = { enabled, ...extra };
    if (backendDsn.trim() && body.backend_dsn === undefined)
      body.backend_dsn = backendDsn.trim();
    if (frontendDsn.trim() && body.frontend_dsn === undefined)
      body.frontend_dsn = frontendDsn.trim();
    save.mutate(body);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.error_tracking.form.title")}</CardTitle>
        <CardDescription>
          {t("settings.error_tracking.form.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex items-center justify-between gap-4 rounded-lg border p-3">
          <div className="space-y-0.5">
            <Label htmlFor="error-tracking-enabled">
              {t("settings.error_tracking.form.enabled")}
            </Label>
            <p className="text-muted-foreground text-xs">
              {t("settings.error_tracking.form.enabled_hint")}
            </p>
          </div>
          <Switch
            id="error-tracking-enabled"
            checked={enabled}
            onCheckedChange={setEnabled}
            disabled={!canWrite || save.isPending}
          />
        </div>

        <DsnField
          id="error-tracking-backend-dsn"
          label={t("settings.error_tracking.form.backend_dsn")}
          hint={
            settings.backend_dsn_configured
              ? t("settings.error_tracking.form.stored", {
                  value: settings.backend_dsn_hint,
                })
              : settings.env_fallback_configured
                ? t("settings.error_tracking.form.backend_env_fallback")
                : t("settings.error_tracking.form.not_set")
          }
          value={backendDsn}
          onChange={setBackendDsn}
          canClear={settings.backend_dsn_configured}
          onClear={() => submit({ backend_dsn: "" })}
          disabled={!canWrite || save.isPending}
        />

        <DsnField
          id="error-tracking-frontend-dsn"
          label={t("settings.error_tracking.form.frontend_dsn")}
          hint={
            settings.frontend_dsn_configured
              ? t("settings.error_tracking.form.stored", {
                  value: settings.frontend_dsn_hint,
                })
              : t("settings.error_tracking.form.not_set")
          }
          value={frontendDsn}
          onChange={setFrontendDsn}
          canClear={settings.frontend_dsn_configured}
          onClear={() => submit({ frontend_dsn: "" })}
          disabled={!canWrite || save.isPending}
        />
      </CardContent>
      {canWrite ? (
        <CardFooter className="justify-end">
          <Button
            type="button"
            onClick={() => submit()}
            disabled={save.isPending}
          >
            {save.isPending ? t("common.saving") : t("common.save")}
          </Button>
        </CardFooter>
      ) : null}
    </Card>
  );
}

function DsnField({
  id,
  label,
  hint,
  value,
  onChange,
  canClear,
  onClear,
  disabled,
}: {
  id: string;
  label: string;
  hint: string;
  value: string;
  onChange: (value: string) => void;
  canClear: boolean;
  onClear: () => void;
  disabled: boolean;
}) {
  const { t } = useLocale();
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex flex-col gap-2 sm:flex-row">
        <Input
          id={id}
          type="password"
          autoComplete="off"
          spellCheck={false}
          placeholder="https://<key>@<host>/<project>"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
        />
        {canClear ? (
          <Button
            type="button"
            variant="outline"
            onClick={onClear}
            disabled={disabled}
          >
            {t("settings.error_tracking.form.clear")}
          </Button>
        ) : null}
      </div>
      <p className="text-muted-foreground text-xs break-all">{hint}</p>
    </div>
  );
}

function TestCard({ settings }: { settings: ErrorTrackingSettings }) {
  const { t } = useLocale();

  const backendTest = useMutation({
    mutationFn: () => errorTrackingService.sendBackendTest(),
    onSuccess: (res) =>
      appToast.success(
        t("settings.error_tracking.test.sent", { id: res.event_id }),
      ),
    onError: (error) =>
      appToast.error(
        isApiError(error) && error.status === 409
          ? t("settings.error_tracking.test.backend_off")
          : t("settings.error_tracking.test.failed"),
      ),
  });

  const frontendTest = useMutation({
    mutationFn: async () => {
      await reloadErrorTracking();
      return sendTestError("ComCode test error (Hata izleme, browser)");
    },
    onSuccess: (id) =>
      id
        ? appToast.success(t("settings.error_tracking.test.sent", { id }))
        : appToast.error(t("settings.error_tracking.test.frontend_off")),
    onError: () => appToast.error(t("settings.error_tracking.test.failed")),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("settings.error_tracking.test.title")}</CardTitle>
        <CardDescription>
          {t("settings.error_tracking.test.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={() => backendTest.mutate()}
          disabled={backendTest.isPending || !settings.backend_active}
        >
          <Send className="size-4" />
          {t("settings.error_tracking.test.backend")}
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => frontendTest.mutate()}
          disabled={
            frontendTest.isPending ||
            !settings.enabled ||
            !settings.frontend_dsn_configured
          }
        >
          <Send className="size-4" />
          {t("settings.error_tracking.test.frontend")}
        </Button>
      </CardContent>
    </Card>
  );
}
