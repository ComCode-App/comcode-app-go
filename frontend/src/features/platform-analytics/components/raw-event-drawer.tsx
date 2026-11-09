"use client";

import { Filter } from "lucide-react";
import type { ReactNode } from "react";

import { EntityDrawer } from "@/components/entity";
import { Button } from "@/components/ui/button";
import { subjectTypeLabel } from "@/features/analytics/lib/event-labels";
import { datetime } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

import type { RawEvent } from "../services/platform-analytics.service";

type RawEventDrawerProps = {
  event: RawEvent | null;
  onOpenChange: (open: boolean) => void;
  onFilterSubject: (subject: RawEvent["subject"]) => void;
  /** Readable label for an event type key (catalog → definition name → prettified key). */
  typeLabel: (type: string) => string;
};

function Field({
  label,
  value,
  mono,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="min-w-0 space-y-1">
      <dt className="text-muted-foreground text-xs font-medium">{label}</dt>
      <dd
        className={mono ? "font-mono text-xs break-all" : "text-sm break-words"}
      >
        {value === undefined || value === null || value === "" ? "—" : value}
      </dd>
    </div>
  );
}

function JsonBlock({ label, value }: { label: string; value: unknown }) {
  if (!value || (typeof value === "object" && Object.keys(value).length === 0))
    return null;
  return (
    <div className="space-y-1 sm:col-span-2">
      <dt className="text-muted-foreground text-xs font-medium">{label}</dt>
      <dd>
        <pre className="bg-muted/40 max-h-72 overflow-auto rounded-md p-3 text-xs">
          {JSON.stringify(value, null, 2)}
        </pre>
      </dd>
    </div>
  );
}

const STAMP = "dd.MM.yyyy HH:mm:ss";

export function RawEventDrawer({
  event,
  onOpenChange,
  onFilterSubject,
  typeLabel,
}: RawEventDrawerProps) {
  const { t, locale } = useLocale();
  const c = event?.context;

  return (
    <EntityDrawer
      open={Boolean(event)}
      onOpenChange={onOpenChange}
      title={t("platform_analytics.events.detail_title")}
      description={event ? typeLabel(event.type) : undefined}
      size="lg"
    >
      {event && c ? (
        <dl className="grid gap-4 sm:grid-cols-2">
          <Field
            label={t("platform_analytics.events.fields.type")}
            value={
              <>
                {typeLabel(event.type)}{" "}
                <span className="text-muted-foreground font-mono text-xs">
                  ({event.type})
                </span>
              </>
            }
          />
          <Field
            label={t("platform_analytics.events.fields.uuid")}
            value={event.uuid}
            mono
          />
          <Field
            label={t("platform_analytics.events.fields.occurred_at")}
            value={datetime(event.occurred_at, STAMP, locale)}
          />
          <Field
            label={t("platform_analytics.events.fields.created_at")}
            value={datetime(event.created_at, STAMP, locale)}
          />
          <div className="space-y-1 sm:col-span-2">
            <dt className="text-muted-foreground text-xs font-medium">
              {t("platform_analytics.events.fields.subject")}
            </dt>
            <dd className="flex flex-wrap items-center gap-2">
              <span className="text-sm">
                {subjectTypeLabel(t, event.subject.type)}
              </span>
              <span className="text-muted-foreground font-mono text-xs break-all">
                {event.subject.id}
              </span>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-7 gap-1.5"
                onClick={() => onFilterSubject(event.subject)}
              >
                <Filter className="size-3.5" />
                {t("platform_analytics.events.filter_subject")}
              </Button>
            </dd>
          </div>
          <Field
            label={t("platform_analytics.events.fields.actor")}
            value={
              event.actor ? `${event.actor.type}:${event.actor.id}` : undefined
            }
            mono
          />
          <Field
            label={t("platform_analytics.events.fields.session_id")}
            value={event.session_id}
            mono
          />
          <Field
            label={t("platform_analytics.events.fields.request_id")}
            value={event.request_id}
            mono
          />
          <Field
            label={t("platform_analytics.events.fields.schema_version")}
            value={event.schema_version}
          />
          <Field
            label={t("platform_analytics.events.fields.location")}
            value={[c.city, c.country_name || c.country_code]
              .filter(Boolean)
              .join(", ")}
          />
          <Field
            label={t("platform_analytics.events.fields.timezone")}
            value={c.timezone}
          />
          <Field
            label={t("platform_analytics.events.fields.device")}
            value={[c.device_type, c.device_brand, c.device_model]
              .filter(Boolean)
              .join(" · ")}
          />
          <Field
            label={t("platform_analytics.events.fields.browser")}
            value={[c.browser, c.browser_version].filter(Boolean).join(" ")}
          />
          <Field
            label={t("platform_analytics.events.fields.os")}
            value={[c.os, c.os_version].filter(Boolean).join(" ")}
          />
          <Field
            label={t("platform_analytics.events.fields.referer")}
            value={c.referer}
            mono
          />
          <Field
            label={t("platform_analytics.events.fields.ip_hash")}
            value={c.ip_hash}
            mono
          />
          <Field
            label={t("platform_analytics.events.fields.delivery")}
            value={[event.metadata?.source, event.metadata?.delivery_mode]
              .filter(Boolean)
              .join(" · ")}
          />
          <JsonBlock
            label={t("platform_analytics.events.fields.properties")}
            value={event.properties}
          />
          <div className="sm:col-span-2">
            <Field
              label={t("platform_analytics.events.fields.user_agent")}
              value={c.user_agent}
              mono
            />
          </div>
        </dl>
      ) : null}
    </EntityDrawer>
  );
}
