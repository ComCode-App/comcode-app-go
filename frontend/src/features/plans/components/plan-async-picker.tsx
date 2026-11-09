"use client";

import { useCallback, useState } from "react";

import { AppCombobox, type ComboboxOption } from "@/components/forms";
import { plansService } from "@/features/plans/services/plans.service";
import { useLocale } from "@/providers/locale-provider";

type PlanAsyncPickerProps = {
  name: string;
  label?: string;
  description?: string;
  className?: string;
  disabled?: boolean;
  activeOnly?: boolean;
};

export function PlanAsyncPicker({
  name,
  label,
  description,
  className,
  disabled,
  activeOnly = true,
}: PlanAsyncPickerProps) {
  const { t } = useLocale();
  const [optionCache, setOptionCache] = useState<ComboboxOption[]>([]);

  const loadOptions = useCallback(
    async (query: string) => {
      const result = await plansService.list({
        limit: 20,
        offset: 0,
        q: query.trim() || undefined,
      });
      const options = result.items
        .filter((plan) => !activeOnly || plan.is_active)
        .map(
          (plan): ComboboxOption => ({
            value: plan.uuid,
            label: plan.name,
            description: plan.slug,
          }),
        );
      setOptionCache((prev) => mergeOptions(prev, options));
      return options;
    },
    [activeOnly],
  );

  return (
    <AppCombobox
      name={name}
      label={label}
      description={description}
      className={className}
      disabled={disabled}
      loadOptions={loadOptions}
      options={optionCache}
      placeholder={t("plans.picker.placeholder")}
      searchPlaceholder={t("plans.picker.search")}
      emptyText={t("plans.picker.empty")}
    />
  );
}

function mergeOptions(
  prev: ComboboxOption[],
  next: ComboboxOption[],
): ComboboxOption[] {
  const byValue = new Map(prev.map((opt) => [opt.value, opt]));
  for (const opt of next) {
    byValue.set(opt.value, opt);
  }
  return [...byValue.values()];
}
