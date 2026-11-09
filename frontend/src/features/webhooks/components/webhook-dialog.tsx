"use client";

import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  useWebhookEvents,
  useWebhookMutations,
} from "@/features/webhooks/hooks/use-webhooks";
import type {
  WebhookEndpoint,
  WebhookEndpointWithSecret,
  WebhookEventName,
} from "@/features/webhooks/services/webhooks.service";
import { useLocale } from "@/providers/locale-provider";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  endpoint: WebhookEndpoint | null;
  onCreated: (created: WebhookEndpointWithSecret) => void;
};

const URL_RE = /^https:\/\/[^\s/$.?#].[^\s]*$/i;

/** Create / edit dialog: URL, description, subscribed events and the active switch (TEC-603). */
export function WebhookDialog({
  open,
  onOpenChange,
  endpoint,
  onCreated,
}: Props) {
  const { t } = useLocale();
  const isEdit = Boolean(endpoint);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {t(
              isEdit
                ? "webhooks.dialog.edit_title"
                : "webhooks.dialog.create_title",
            )}
          </DialogTitle>
          <DialogDescription>
            {t("webhooks.dialog.description")}
          </DialogDescription>
        </DialogHeader>
        {/* Keyed by endpoint so the form state starts from the row being edited; the content
            unmounts when the dialog closes, which resets it for the next open. */}
        <WebhookForm
          key={endpoint?.uuid ?? "new"}
          endpoint={endpoint}
          onCancel={() => onOpenChange(false)}
          onSaved={(created) => {
            onOpenChange(false);
            if (created) onCreated(created);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}

type FormProps = {
  endpoint: WebhookEndpoint | null;
  onCancel: () => void;
  onSaved: (created: WebhookEndpointWithSecret | null) => void;
};

function WebhookForm({ endpoint, onCancel, onSaved }: FormProps) {
  const { t, locale } = useLocale();
  const eventsQuery = useWebhookEvents();
  const { create, update } = useWebhookMutations();

  const [url, setUrl] = useState(endpoint?.url ?? "");
  const [description, setDescription] = useState(endpoint?.description ?? "");
  const [events, setEvents] = useState<WebhookEventName[]>(
    endpoint?.events ?? [],
  );
  const [isActive, setIsActive] = useState(endpoint?.is_active ?? true);
  const [errors, setErrors] = useState<{ url?: string; events?: string }>({});

  const definitions = useMemo(
    () => eventsQuery.data?.items ?? [],
    [eventsQuery.data],
  );
  const pending = create.isPending || update.isPending;

  const toggle = (name: WebhookEventName, checked: boolean) => {
    setEvents((prev) =>
      checked
        ? Array.from(new Set([...prev, name]))
        : prev.filter((e) => e !== name),
    );
  };

  const submit = async () => {
    const next: typeof errors = {};
    if (!URL_RE.test(url.trim())) next.url = t("webhooks.validation.url");
    if (events.length === 0) next.events = t("webhooks.validation.events");
    setErrors(next);
    if (next.url || next.events) return;
    const body = {
      url: url.trim(),
      description: description.trim(),
      events,
      is_active: isActive,
    };
    if (endpoint) {
      await update.mutateAsync({ uuid: endpoint.uuid, body });
      onSaved(null);
      return;
    }
    onSaved(await create.mutateAsync(body));
  };

  return (
    <form
      className="space-y-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <div className="space-y-2">
        <Label htmlFor="webhook-url">{t("webhooks.fields.url")}</Label>
        <Input
          id="webhook-url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder={t("webhooks.fields.url_placeholder")}
          inputMode="url"
          autoComplete="off"
          aria-invalid={Boolean(errors.url)}
        />
        <p
          className={
            errors.url
              ? "text-destructive text-xs"
              : "text-muted-foreground text-xs"
          }
        >
          {errors.url ?? t("webhooks.fields.url_hint")}
        </p>
      </div>
      <div className="space-y-2">
        <Label htmlFor="webhook-description">
          {t("webhooks.fields.description")}
        </Label>
        <Input
          id="webhook-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder={t("webhooks.fields.description_placeholder")}
          maxLength={500}
        />
      </div>
      <div className="space-y-2">
        <Label>{t("webhooks.fields.events")}</Label>
        <div className="grid gap-2 rounded-lg border p-3">
          {definitions.map((def) => {
            const id = `webhook-event-${def.name}`;
            return (
              <div key={def.name} className="flex items-start gap-3">
                <Checkbox
                  id={id}
                  checked={events.includes(def.name)}
                  onCheckedChange={(checked) =>
                    toggle(def.name, checked === true)
                  }
                />
                <div className="grid gap-0.5 leading-tight">
                  <Label htmlFor={id} className="font-medium">
                    {t(`webhooks.events.${def.name}`)}
                    <span className="text-muted-foreground ml-2 font-mono text-[11px] font-normal">
                      {def.name}
                    </span>
                  </Label>
                  <p className="text-muted-foreground text-xs">
                    {locale === "tr" ? def.description_tr : def.description_en}
                  </p>
                </div>
              </div>
            );
          })}
          {eventsQuery.isLoading ? (
            <Loader2 className="text-muted-foreground size-4 animate-spin" />
          ) : null}
        </div>
        <p
          className={
            errors.events
              ? "text-destructive text-xs"
              : "text-muted-foreground text-xs"
          }
        >
          {errors.events ?? t("webhooks.fields.events_hint")}
        </p>
      </div>
      <div className="flex items-center justify-between rounded-lg border p-3">
        <div className="space-y-0.5">
          <Label htmlFor="webhook-active">
            {t("webhooks.fields.is_active")}
          </Label>
          <p className="text-muted-foreground text-xs">
            {t("webhooks.fields.is_active_hint")}
          </p>
        </div>
        <Switch
          id="webhook-active"
          checked={isActive}
          onCheckedChange={setIsActive}
        />
      </div>
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          disabled={pending}
        >
          {t("common.cancel")}
        </Button>
        <Button type="submit" disabled={pending}>
          {pending ? <Loader2 className="size-4 animate-spin" /> : null}
          {t("common.save")}
        </Button>
      </DialogFooter>
    </form>
  );
}
