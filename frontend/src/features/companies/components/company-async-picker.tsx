"use client";

import { useCallback, useState } from "react";

import { AppCombobox, type ComboboxOption } from "@/components/forms";
import { companiesService } from "@/features/companies/services/companies.service";
import { useLocale } from "@/providers/locale-provider";

type CompanyAsyncPickerProps = {
  name: string;
  label?: string;
  description?: string;
  className?: string;
  disabled?: boolean;
};

export function CompanyAsyncPicker({
  name,
  label,
  description,
  className,
  disabled,
}: CompanyAsyncPickerProps) {
  const { t } = useLocale();
  const [optionCache, setOptionCache] = useState<ComboboxOption[]>([]);

  const loadOptions = useCallback(async (query: string) => {
    const result = await companiesService.list({
      limit: 20,
      offset: 0,
      q: query.trim() || undefined,
    });
    const options = result.items.map((company): ComboboxOption => ({
      value: company.uuid,
      label: company.name,
      description: company.slug,
    }));
    setOptionCache((prev) => mergeOptions(prev, options));
    return options;
  }, []);

  return (
    <AppCombobox
      name={name}
      label={label}
      description={description}
      className={className}
      disabled={disabled}
      loadOptions={loadOptions}
      options={optionCache}
      placeholder={t("companies.picker.placeholder")}
      searchPlaceholder={t("companies.picker.search")}
      emptyText={t("companies.picker.empty")}
    />
  );
}

function mergeOptions(
  existing: ComboboxOption[],
  incoming: ComboboxOption[],
): ComboboxOption[] {
  const map = new Map<string, ComboboxOption>();
  for (const opt of existing) map.set(opt.value, opt);
  for (const opt of incoming) map.set(opt.value, opt);
  return Array.from(map.values());
}
