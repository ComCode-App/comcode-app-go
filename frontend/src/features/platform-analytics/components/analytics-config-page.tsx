"use client";

import { Archive, Gauge, Power, ShieldCheck } from "lucide-react";
import { useMemo } from "react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { EntityPage } from "@/components/entity";
import {
  AppForm,
  AppInput,
  AppSelect,
  AppSwitch,
  FormActions,
  FormLayout,
  FormSection,
  createFormSections,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

import {
  useAnalyticsConfig,
  useUpdateAnalyticsConfig,
} from "../hooks/use-platform-analytics";
import {
  DELIVERY_MODES,
  createAnalyticsConfigSchema,
  settingsToFormValues,
  type AnalyticsConfigFormValues,
} from "../schemas/analytics-config-form";
import type { AnalyticsSettings } from "../services/platform-analytics.service";

function ConfigForm({
  settings,
  canWrite,
}: {
  settings: AnalyticsSettings;
  canWrite: boolean;
}) {
  const { t } = useLocale();
  const save = useUpdateAnalyticsConfig();
  const schema = useMemo(() => createAnalyticsConfigSchema(t), [t]);
  const defaultValues = useMemo(
    () => settingsToFormValues(settings),
    [settings],
  );
  const disabled = !canWrite || save.isPending;
  // TEC-936: a changed salt period applies from the next UTC day; today keeps the previous one.
  const todayUtc = new Date().toISOString().slice(0, 10);
  const saltPending =
    settings.ip_hash_salt_rotation_changed_on === todayUtc &&
    settings.ip_hash_salt_rotation_previous_days
      ? settings.ip_hash_salt_rotation_previous_days
      : null;
  const modeLabels: Record<(typeof DELIVERY_MODES)[number], string> = {
    async: t("platform_analytics.config.mode_async"),
    sync: t("platform_analytics.config.mode_sync"),
    hybrid: t("platform_analytics.config.mode_hybrid"),
  };

  const sections = useMemo(
    () =>
      createFormSections([
        {
          key: "pipeline",
          label: t("platform_analytics.config.section_pipeline"),
          icon: Power,
        },
        {
          key: "batching",
          label: t("platform_analytics.config.section_batching"),
          icon: Gauge,
        },
        {
          key: "retention",
          label: t("platform_analytics.config.section_retention"),
          icon: Archive,
        },
        {
          key: "privacy",
          label: t("platform_analytics.config.section_privacy"),
          icon: ShieldCheck,
        },
      ]),
    [t],
  );

  return (
    <AppForm<AnalyticsConfigFormValues>
      // Remount after a save so the form shows what the server stored.
      key={JSON.stringify(defaultValues)}
      schema={schema}
      defaultValues={defaultValues}
      onSubmit={async (values) => {
        await save.mutateAsync(values);
      }}
    >
      {(form) => (
        <FormLayout navItems={sections.navItems}>
          <FormSection
            id={sections.id("pipeline")}
            title={t("platform_analytics.config.section_pipeline")}
            description={t("platform_analytics.config.section_pipeline_hint")}
            columns={2}
          >
            <div className="sm:col-span-2">
              <AppSwitch
                name="enabled"
                label={t("platform_analytics.config.enabled")}
                description={t("platform_analytics.config.enabled_hint")}
                disabled={disabled}
              />
            </div>
            <AppSelect
              name="event_delivery_mode"
              label={t("platform_analytics.config.event_delivery_mode")}
              description={t(
                "platform_analytics.config.event_delivery_mode_hint",
              )}
              options={DELIVERY_MODES.map((m) => ({
                value: m,
                label: modeLabels[m],
              }))}
              disabled={disabled}
            />
            <AppInput
              name="queue_name"
              label={t("platform_analytics.config.queue_name")}
              description={t("platform_analytics.config.queue_name_hint")}
              disabled={disabled}
            />
            <div className="sm:col-span-2">
              <AppSwitch
                name="realtime_enabled"
                label={t("platform_analytics.config.realtime_enabled")}
                description={t(
                  "platform_analytics.config.realtime_enabled_hint",
                )}
                disabled={disabled}
              />
            </div>
          </FormSection>

          <FormSection
            id={sections.id("batching")}
            title={t("platform_analytics.config.section_batching")}
            description={t("platform_analytics.config.section_batching_hint")}
            columns={2}
          >
            <AppInput
              name="batch_size"
              type="number"
              min={1}
              label={t("platform_analytics.config.batch_size")}
              disabled={disabled}
            />
            <AppInput
              name="batch_flush_interval_ms"
              type="number"
              min={1}
              label={t("platform_analytics.config.batch_flush_interval_ms")}
              disabled={disabled}
            />
          </FormSection>

          <FormSection
            id={sections.id("retention")}
            title={t("platform_analytics.config.section_retention")}
            description={t("platform_analytics.config.section_retention_hint")}
            columns={2}
          >
            <AppInput
              name="raw_event_retention_days"
              type="number"
              min={1}
              label={t("platform_analytics.config.raw_event_retention_days")}
              disabled={disabled}
            />
            <AppInput
              name="aggregate_retention_days"
              type="number"
              min={1}
              label={t("platform_analytics.config.aggregate_retention_days")}
              disabled={disabled}
            />
          </FormSection>

          <FormSection
            id={sections.id("privacy")}
            title={t("platform_analytics.config.section_privacy")}
            description={t("platform_analytics.config.section_privacy_hint")}
            columns={2}
          >
            <div className="sm:col-span-2">
              <AppSwitch
                name="anonymize_ip"
                label={t("platform_analytics.config.anonymize_ip")}
                description={t("platform_analytics.config.anonymize_ip_hint")}
                disabled={disabled}
              />
            </div>
            <AppInput
              name="ip_hash_salt_rotation_days"
              type="number"
              min={1}
              max={30}
              label={t("platform_analytics.config.ip_hash_salt_rotation_days")}
              description={
                saltPending
                  ? t(
                      "platform_analytics.config.ip_hash_salt_rotation_pending",
                      { days: saltPending },
                    )
                  : t("platform_analytics.config.ip_hash_salt_rotation_hint")
              }
              disabled={disabled}
            />
          </FormSection>

          {canWrite ? (
            <FormActions>
              <Button
                type="button"
                variant="outline"
                onClick={() => form.reset(defaultValues)}
                disabled={save.isPending}
              >
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={save.isPending}>
                {save.isPending ? t("common.loading") : t("common.save")}
              </Button>
            </FormActions>
          ) : (
            <p className="text-muted-foreground text-sm">
              {t("platform_analytics.config.read_only")}
            </p>
          )}
        </FormLayout>
      )}
    </AppForm>
  );
}

/** Analytics runtime settings (`/v1/platform/analytics/config`); editable with platform.analytics.manage. */
export function AnalyticsConfigPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const canRead = can(permissions.analytics.read);
  const config = useAnalyticsConfig(canRead);

  return (
    <EntityPage
      title={t("platform_analytics.config.title")}
      description={t("platform_analytics.config.subtitle")}
      permission={permissions.analytics.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("platform_analytics.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        {
          label: t("layout.nav_platform_analytics"),
          href: routes.platform.analytics.root,
        },
        { label: t("platform_analytics.config.title") },
      ]}
    >
      {config.isLoading ? <Loading label={t("common.loading")} /> : null}
      {config.isError ? (
        <ErrorState
          title={t("platform_analytics.load_error")}
          retryLabel={t("common.retry")}
          onRetry={() => void config.refetch()}
        />
      ) : null}
      {config.data ? (
        <ConfigForm
          settings={config.data}
          canWrite={can(permissions.analytics.manage)}
        />
      ) : null}
    </EntityPage>
  );
}
