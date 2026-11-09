"use client";

import { useMemo } from "react";

import {
  AppForm,
  AppInput,
  AppSwitch,
  AppTextarea,
} from "@/components/forms";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useLocale } from "@/providers/locale-provider";

import { useEventDefinitionMutations } from "../hooks/use-platform-analytics";
import {
  KNOWN_EVENT_CATEGORIES,
  createEventDefinitionSchema,
  definitionToFormValues,
  emptyEventDefinitionValues,
  formValuesToInput,
  type EventDefinitionFormValues,
} from "../schemas/event-definition-form";
import type { EventDefinition } from "../services/platform-analytics.service";

type EventDefinitionDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** null → create */
  definition: EventDefinition | null;
};

const FORM_ID = "event-definition-form";
const CATEGORY_LIST_ID = "event-definition-categories";

export function EventDefinitionDialog({ open, onOpenChange, definition }: EventDefinitionDialogProps) {
  const { t } = useLocale();
  const { create, update } = useEventDefinitionMutations();
  const isEdit = Boolean(definition);
  const saving = create.isPending || update.isPending;

  const schema = useMemo(() => createEventDefinitionSchema(t), [t]);
  const defaultValues = useMemo<EventDefinitionFormValues>(
    () => (definition ? definitionToFormValues(definition) : emptyEventDefinitionValues),
    [definition],
  );

  const onSubmit = async (values: EventDefinitionFormValues) => {
    const body = formValuesToInput(values);
    if (definition) {
      await update.mutateAsync({ id: definition.id, body });
    } else {
      await create.mutateAsync(body);
    }
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {isEdit
              ? t("platform_analytics.definitions.dialog.edit_title")
              : t("platform_analytics.definitions.dialog.create_title")}
          </DialogTitle>
          <DialogDescription>{t("platform_analytics.definitions.dialog.description")}</DialogDescription>
        </DialogHeader>

        {open ? (
          <AppForm
            // Remount per definition so default values follow the selected row.
            key={definition?.id ?? "new"}
            id={FORM_ID}
            schema={schema}
            defaultValues={defaultValues}
            onSubmit={onSubmit}
            className="grid gap-4 sm:grid-cols-2"
          >
            <AppInput
              name="key"
              label={t("platform_analytics.definitions.fields.key")}
              description={
                isEdit
                  ? t("platform_analytics.definitions.fields.key_locked")
                  : t("platform_analytics.definitions.fields.key_hint")
              }
              placeholder="card.view"
              autoComplete="off"
              // readOnly, not disabled: react-hook-form drops disabled inputs from the values.
              readOnly={isEdit}
              disabled={saving}
            />
            <AppInput
              name="name"
              label={t("platform_analytics.definitions.fields.name")}
              placeholder="Card View"
              disabled={saving}
            />
            <div className="sm:col-span-2">
              <AppTextarea
                name="description"
                label={t("platform_analytics.definitions.fields.description")}
                rows={2}
                disabled={saving}
              />
            </div>
            <AppInput
              name="category"
              label={t("platform_analytics.definitions.fields.category")}
              list={CATEGORY_LIST_ID}
              autoComplete="off"
              disabled={saving}
            />
            <datalist id={CATEGORY_LIST_ID}>
              {KNOWN_EVENT_CATEGORIES.map((c) => (
                <option key={c} value={c} />
              ))}
            </datalist>
            <AppInput
              name="schema_version"
              type="number"
              min={1}
              label={t("platform_analytics.definitions.fields.schema_version")}
              disabled={saving}
            />
            <AppInput
              name="retention_days"
              type="number"
              min={1}
              label={t("platform_analytics.definitions.fields.retention_days")}
              description={t("platform_analytics.definitions.fields.retention_days_hint")}
              disabled={saving}
            />
            <AppInput
              name="required_properties"
              label={t("platform_analytics.definitions.fields.required_properties")}
              description={t("platform_analytics.definitions.fields.required_properties_hint")}
              placeholder="amount, currency"
              autoComplete="off"
              disabled={saving}
            />
            <div className="grid gap-3 sm:col-span-2">
              <AppSwitch
                name="enabled"
                label={t("platform_analytics.definitions.fields.enabled")}
                description={t("platform_analytics.definitions.fields.enabled_hint")}
                disabled={saving}
              />
              <AppSwitch
                name="is_public"
                label={t("platform_analytics.definitions.fields.is_public")}
                description={t("platform_analytics.definitions.fields.is_public_hint")}
                disabled={saving}
              />
              <AppSwitch
                name="aggregate_enabled"
                label={t("platform_analytics.definitions.fields.aggregate_enabled")}
                description={t("platform_analytics.definitions.fields.aggregate_enabled_hint")}
                disabled={saving}
              />
            </div>
          </AppForm>
        ) : null}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" form={FORM_ID} disabled={saving}>
            {saving ? t("common.loading") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
