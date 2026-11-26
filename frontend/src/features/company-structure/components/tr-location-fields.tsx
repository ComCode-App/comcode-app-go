"use client";

import { Check, ChevronsUpDown } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

// Turkey's 81 provinces and 973 districts, from turkey-neighbourhoods 4.0.3 (MIT, see
// tr-locations.LICENSE). Only the names are bundled.
import locations from "../data/tr-locations.json";

type Province = { code: string; name: string; districts: string[] };

const PROVINCES = locations as Province[];

const collator = new Intl.Collator("tr");

function findProvince(name: string): Province | undefined {
  const key = name.trim().toLocaleLowerCase("tr");
  return PROVINCES.find((p) => p.name.toLocaleLowerCase("tr") === key);
}

/** A searchable single choice; a stored value missing from `options` stays shown. */
function SearchSelect({
  id,
  value,
  options,
  placeholder,
  searchPlaceholder,
  emptyText,
  disabled,
  onChange,
}: {
  id: string;
  value: string;
  options: string[];
  placeholder: string;
  searchPlaceholder: string;
  emptyText: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const list =
    value && !options.includes(value) ? [value, ...options] : options;
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          disabled={disabled}
          className="w-full justify-between font-normal"
        >
          <span className={cn("truncate", !value && "text-muted-foreground")}>
            {value || placeholder}
          </span>
          <ChevronsUpDown className="text-muted-foreground size-4 shrink-0" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-(--radix-popover-trigger-width) p-0"
      >
        <Command>
          <CommandInput placeholder={searchPlaceholder} />
          <CommandList className="max-h-64">
            <CommandEmpty>{emptyText}</CommandEmpty>
            {list.map((option) => (
              <CommandItem
                key={option}
                value={option}
                onSelect={() => {
                  onChange(option === value ? "" : option);
                  setOpen(false);
                }}
              >
                <Check
                  className={cn(
                    "size-4",
                    option === value ? "opacity-100" : "opacity-0",
                  )}
                />
                {option}
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

/**
 * Province (il) and district (ilçe) of an office (TEC-1316). The district list follows the
 * province; changing the province clears a district that does not belong to it. Values typed
 * before the lists existed stay selectable.
 */
export function TrLocationFields({
  city,
  district,
  disabled,
  onChange,
}: {
  city: string;
  district: string;
  disabled?: boolean;
  onChange: (next: { city: string; district: string }) => void;
}) {
  const { t } = useLocale();
  const province = findProvince(city);
  const provinceNames = PROVINCES.map((p) => p.name).sort(collator.compare);
  const districts = province
    ? [...province.districts].sort(collator.compare)
    : [];

  return (
    <>
      <div className="space-y-1.5">
        <Label htmlFor="office-city">
          {t("company_structure.offices.fields.city")}
        </Label>
        <SearchSelect
          id="office-city"
          value={province?.name ?? city}
          options={provinceNames}
          placeholder={t("company_structure.offices.fields.city_placeholder")}
          searchPlaceholder={t("company_structure.offices.fields.city_search")}
          emptyText={t("company_structure.offices.fields.no_match")}
          disabled={disabled}
          onChange={(next) => {
            const nextProvince = findProvince(next);
            const keep =
              nextProvince && nextProvince.districts.includes(district);
            onChange({ city: next, district: keep ? district : "" });
          }}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="office-district">
          {t("company_structure.offices.fields.district")}
        </Label>
        <SearchSelect
          id="office-district"
          value={district}
          options={districts}
          placeholder={
            city
              ? t("company_structure.offices.fields.district_placeholder")
              : t("company_structure.offices.fields.district_needs_city")
          }
          searchPlaceholder={t(
            "company_structure.offices.fields.district_search",
          )}
          emptyText={t("company_structure.offices.fields.no_match")}
          disabled={disabled || !city}
          onChange={(next) => onChange({ city, district: next })}
        />
      </div>
    </>
  );
}
