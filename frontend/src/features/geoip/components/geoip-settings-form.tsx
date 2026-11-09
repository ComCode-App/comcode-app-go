"use client";

import { useMemo } from "react";
import { Database, HardDrive, Key, Layers, RefreshCw } from "lucide-react";

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
import {
  geoipFormSchema,
  type GeoIPFormValues,
} from "@/features/geoip/schemas/geoip-form";
import type { GeoIPSettings } from "@/features/geoip/services/geoip.service";
import { useLocale } from "@/providers/locale-provider";

type GeoIPSettingsFormProps = {
  settings: GeoIPSettings;
  canWrite: boolean;
  isSaving: boolean;
  onSubmit: (values: GeoIPFormValues) => Promise<void>;
};

export function GeoIPSettingsForm({
  settings,
  canWrite,
  isSaving,
  onSubmit,
}: GeoIPSettingsFormProps) {
  const { t } = useLocale();

  const defaultValues = useMemo<GeoIPFormValues>(() => {
    // Backend cache_ttl might be stored in nanoseconds or hours
    let ttlHours = 24;
    if (typeof settings.cache_ttl === "number" && settings.cache_ttl > 0) {
      if (settings.cache_ttl > 1000000) {
        // Nanoseconds to hours
        ttlHours = Math.round(settings.cache_ttl / (3600 * 1e9));
      } else {
        ttlHours = settings.cache_ttl;
      }
    }

    return {
      enabled: settings.enabled ?? true,
      provider: settings.provider ?? "maxmind",
      database_type: settings.database_type ?? "geolite2-city",
      database_path: settings.database_path ?? "data/geoip/GeoLite2-City.mmdb",
      auto_update_enabled: settings.auto_update_enabled ?? true,
      update_interval_hours: settings.update_interval_hours ?? 24,
      account_id: settings.account_id ?? "",
      license_key: settings.license_key ?? "",
      download_url_template: settings.download_url_template ?? "",
      cache_enabled: settings.cache_enabled ?? true,
      cache_ttl: ttlHours || 24,
      cache_max_entries: settings.cache_max_entries ?? 100000,
    };
  }, [settings]);

  const sections = useMemo(
    () =>
      createFormSections([
        {
          key: "engine",
          label: t("geoip.settings.section_engine"),
          description: t("geoip.settings.section_engine_hint"),
          icon: Database,
        },
        {
          key: "updates",
          label: t("geoip.settings.section_updates"),
          description: t("geoip.settings.section_updates_hint"),
          icon: RefreshCw,
        },
        {
          key: "cache",
          label: t("geoip.settings.section_cache"),
          description: t("geoip.settings.section_cache_hint"),
          icon: Layers,
        },
      ]),
    [t],
  );

  return (
    <AppForm
      schema={geoipFormSchema}
      defaultValues={defaultValues}
      onSubmit={onSubmit}
    >
      {(form) => (
        <FormLayout navItems={sections.navItems}>
          {/* Engine & Provider Section */}
          <FormSection
            id={sections.id("engine")}
            title={t("geoip.settings.section_engine")}
            description={t("geoip.settings.section_engine_hint")}
            columns={2}
          >
            <div className="sm:col-span-2">
              <AppSwitch
                name="enabled"
                label={t("geoip.settings.enabled")}
                description={t("geoip.settings.enabled_hint")}
                disabled={!canWrite || isSaving}
              />
            </div>

            <AppSelect
              name="provider"
              label={t("geoip.settings.provider")}
              options={[
                { value: "maxmind", label: "MaxMind GeoLite2 / GeoIP2" },
                { value: "noop", label: "Disabled / Noop" },
              ]}
              disabled={!canWrite || isSaving}
            />

            <AppInput
              name="database_type"
              label={t("geoip.settings.database_type")}
              disabled={!canWrite || isSaving}
            />

            <div className="sm:col-span-2">
              <AppInput
                name="database_path"
                label={t("geoip.settings.database_path")}
                description={t("geoip.settings.database_path_hint")}
                startIcon={HardDrive}
                disabled={!canWrite || isSaving}
              />
            </div>
          </FormSection>

          {/* MaxMind Updates & Credentials Section */}
          <FormSection
            id={sections.id("updates")}
            title={t("geoip.settings.section_updates")}
            description={t("geoip.settings.section_updates_hint")}
            columns={2}
          >
            <div className="sm:col-span-2">
              <AppSwitch
                name="auto_update_enabled"
                label={t("geoip.settings.auto_update_enabled")}
                description={t("geoip.settings.auto_update_enabled_hint")}
                disabled={!canWrite || isSaving}
              />
            </div>

            <AppInput
              name="update_interval_hours"
              type="number"
              label={t("geoip.settings.update_interval_hours")}
              disabled={!canWrite || isSaving}
            />

            <AppInput
              name="account_id"
              label={t("geoip.settings.account_id")}
              description={t("geoip.settings.account_id_hint")}
              placeholder="e.g. 123456"
              disabled={!canWrite || isSaving}
            />

            <div className="sm:col-span-2">
              <AppInput
                name="license_key"
                label={t("geoip.settings.license_key")}
                description={t("geoip.settings.license_key_hint")}
                placeholder="e.g. ABCDEFGHIJKLMNOP"
                startIcon={Key}
                disabled={!canWrite || isSaving}
              />
            </div>

            <div className="sm:col-span-2">
              <AppInput
                name="download_url_template"
                label={t("geoip.settings.download_url_template")}
                description={t("geoip.settings.download_url_template_hint")}
                placeholder="https://..."
                disabled={!canWrite || isSaving}
              />
            </div>
          </FormSection>

          {/* Cache Configuration Section */}
          <FormSection
            id={sections.id("cache")}
            title={t("geoip.settings.section_cache")}
            description={t("geoip.settings.section_cache_hint")}
            columns={2}
          >
            <div className="sm:col-span-2">
              <AppSwitch
                name="cache_enabled"
                label={t("geoip.settings.cache_enabled")}
                description={t("geoip.settings.cache_enabled_hint")}
                disabled={!canWrite || isSaving}
              />
            </div>

            <AppInput
              name="cache_ttl"
              type="number"
              label={t("geoip.settings.cache_ttl_hours")}
              disabled={!canWrite || isSaving}
            />

            <AppInput
              name="cache_max_entries"
              type="number"
              label={t("geoip.settings.cache_max_entries")}
              disabled={!canWrite || isSaving}
            />
          </FormSection>

          {canWrite ? (
            <FormActions>
              <Button
                type="button"
                variant="outline"
                onClick={() => form.reset(defaultValues)}
                disabled={isSaving}
              >
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving ? t("common.loading") : t("geoip.settings.save")}
              </Button>
            </FormActions>
          ) : null}
        </FormLayout>
      )}
    </AppForm>
  );
}
