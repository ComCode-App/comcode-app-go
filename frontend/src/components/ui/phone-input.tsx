"use client";

import { createContext, useContext, useMemo, useState } from "react";
import * as BasePhoneInput from "react-phone-number-input";
import flags from "react-phone-number-input/flags";

import { cn } from "@/lib/utils/index";
import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxSeparator,
  ComboboxTrigger,
  ComboboxValue,
} from "@/components/ui/combobox";
import { ScrollArea } from "@/components/ui/scroll-area";
import { GlobeIcon } from "lucide-react";

type PhoneInputSize = "sm" | "default" | "lg";

const PhoneInputContext = createContext<{
  variant: PhoneInputSize;
  popupClassName?: string;
  scrollAreaClassName?: string;
}>({
  variant: "default",
  popupClassName: undefined,
  scrollAreaClassName: undefined,
});

type PhoneInputProps = Omit<
  React.ComponentProps<"input">,
  "onChange" | "value" | "ref"
> &
  Omit<
    BasePhoneInput.Props<typeof BasePhoneInput.default>,
    "onChange" | "variant" | "popupClassName" | "scrollAreaClassName"
  > & {
    onChange?: (value: BasePhoneInput.Value) => void;
    variant?: PhoneInputSize;
    popupClassName?: string;
    scrollAreaClassName?: string;
  };

// Helper function to ensure phone numbers passed to react-phone-number-input are E.164 compatible
function normalizeToE164(val?: string): BasePhoneInput.Value | undefined {
  if (!val) return undefined;
  const trimmed = val.trim();
  if (!trimmed) return undefined;
  // If it starts with +, remove spaces/dashes/parentheses
  if (trimmed.startsWith("+")) {
    const cleaned = "+" + trimmed.slice(1).replace(/\D/g, "");
    return cleaned as BasePhoneInput.Value;
  }
  return trimmed as BasePhoneInput.Value;
}

function PhoneInput({
  className,
  variant,
  popupClassName,
  scrollAreaClassName,
  onChange,
  value,
  defaultCountry = "TR",
  ...props
}: PhoneInputProps) {
  const phoneInputSize = variant || "default";
  const normalizedValue = normalizeToE164(value as string | undefined);

  return (
    <PhoneInputContext.Provider
      value={{ variant: phoneInputSize, popupClassName, scrollAreaClassName }}
    >
      <BasePhoneInput.default
        className={cn(
          "flex items-stretch w-full rounded-xl shadow-xs transition-all",
          props["aria-invalid"] &&
            "[&_*[data-slot=combobox-trigger]]:border-destructive [&_*[data-slot=combobox-trigger]]:ring-destructive/50",
          className,
        )}
        defaultCountry={defaultCountry}
        flagComponent={FlagComponent}
        countrySelectComponent={CountrySelect}
        inputComponent={InputComponent}
        smartCaret={false}
        value={normalizedValue}
        onChange={(val) => onChange?.(val || ("" as BasePhoneInput.Value))}
        {...props}
      />
    </PhoneInputContext.Provider>
  );
}

function InputComponent({
  className,
  ...props
}: React.ComponentProps<"input">) {
  const { variant } = useContext(PhoneInputContext);

  return (
    <input
      className={cn(
        "flex-1 bg-background text-foreground border border-input rounded-e-xl rounded-s-none px-3.5 py-2 text-sm font-medium transition-colors outline-none placeholder:text-muted-foreground focus:z-10 focus:border-primary focus:ring-1 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50",
        variant === "sm" && "h-8 text-xs",
        variant === "default" && "h-10 text-sm",
        variant === "lg" && "h-11 text-base",
        className,
      )}
      {...props}
    />
  );
}

type CountryEntry = { label: string; value: BasePhoneInput.Country | undefined };

type CountrySelectProps = {
  disabled?: boolean;
  value: BasePhoneInput.Country;
  options: CountryEntry[];
  onChange: (country: BasePhoneInput.Country) => void;
};

function CountrySelect({
  disabled,
  value: selectedCountry,
  options: countryList,
  onChange,
}: CountrySelectProps) {
  const { variant, popupClassName } = useContext(PhoneInputContext);
  const [searchValue, setSearchValue] = useState("");

  const filteredCountries = useMemo(() => {
    if (!searchValue) return countryList;
    return countryList.filter(({ label }) =>
      label.toLowerCase().includes(searchValue.toLowerCase()),
    );
  }, [countryList, searchValue]);

  return (
    <Combobox
      items={filteredCountries}
      value={selectedCountry || "TR"}
      onValueChange={(country: BasePhoneInput.Country | null) => {
        if (country) {
          onChange(country);
        }
      }}
    >
      <ComboboxTrigger
        render={
          <Button
            type="button"
            variant="outline"
            className={cn(
              "rounded-s-xl rounded-e-none border-input border-e-0 bg-muted/20 hover:bg-muted/40 text-foreground px-3 leading-none focus:z-10 flex items-center justify-center gap-1.5 shrink-0 transition-colors shadow-none",
              variant === "sm" && "h-8 px-2",
              variant === "default" && "h-10 px-3",
              variant === "lg" && "h-11 px-4",
              disabled && "opacity-50",
            )}
            disabled={disabled}
          >
            <span className="sr-only">
              <ComboboxValue />
            </span>
            <FlagComponent
              country={selectedCountry || "TR"}
              countryName={selectedCountry || "Turkey"}
            />
          </Button>
        }
      />
      <ComboboxContent
        className={cn(
          "w-72 *:data-[slot=input-group]:bg-transparent shadow-lg border-border",
          popupClassName,
        )}
      >
        <ComboboxInput
          placeholder="Search country..."
          value={searchValue}
          onChange={(e) => setSearchValue(e.target.value)}
          showTrigger={false}
          className="border-input focus-visible:border-border rounded-none border-0 px-3 py-2 text-sm shadow-none ring-0! outline-none! focus-visible:ring-0"
        />
        <ComboboxSeparator />
        <ComboboxEmpty className="px-4 py-3 text-xs text-muted-foreground">
          No country found.
        </ComboboxEmpty>
        <ComboboxList>
          <div className="relative flex max-h-full">
            <div className="flex max-h-60 w-full scroll-pt-2 scroll-pb-2 flex-col overscroll-contain">
              <ScrollArea className="size-full min-h-0">
                {filteredCountries.map((item: CountryEntry) =>
                  item.value ? (
                    <ComboboxItem
                      key={item.value}
                      value={item.value}
                      className="flex items-center gap-2.5 px-3 py-2 text-xs cursor-pointer hover:bg-muted/60"
                    >
                      <FlagComponent
                        country={item.value}
                        countryName={item.label}
                      />
                      <span className="flex-1 truncate font-medium">
                        {item.label}
                      </span>
                      <span className="text-muted-foreground font-mono text-[11px]">
                        {`+${BasePhoneInput.getCountryCallingCode(item.value)}`}
                      </span>
                    </ComboboxItem>
                  ) : null,
                )}
              </ScrollArea>
            </div>
          </div>
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}

function FlagComponent({ country, countryName }: BasePhoneInput.FlagProps) {
  const Flag = country ? flags[country] : null;

  return (
    <span className="flex h-4 w-5 items-center justify-center overflow-hidden rounded-[3px] shadow-xs shrink-0">
      {Flag ? (
        <Flag title={countryName} />
      ) : (
        <GlobeIcon className="size-4 opacity-60" />
      )}
    </span>
  );
}

export { PhoneInput };
