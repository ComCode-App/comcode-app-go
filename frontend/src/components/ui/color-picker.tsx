"use client";

import * as React from "react";

import {
  ColorPicker as ColorPickerRoot,
  ColorPickerArea,
  ColorPickerContent,
  ColorPickerHueSlider,
} from "@/components/editor/editor-ui/color-picker";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils/index";
import { useLocale } from "@/providers/locale-provider";

const HEX_RE = /^#[0-9a-f]{6}$/i;

/** True for a full `#rrggbb` value (what `ColorPicker` emits). */
export function isHexColor(value: string | null | undefined): value is string {
  return typeof value === "string" && HEX_RE.test(value);
}

/** Swatches shown in every picker; any other colour is one drag or hex away. */
export const DEFAULT_COLOR_PRESETS = [
  "#000000",
  "#374151",
  "#6b7280",
  "#d1d5db",
  "#ffffff",
  "#ef4444",
  "#f97316",
  "#f59e0b",
  "#eab308",
  "#84cc16",
  "#22c55e",
  "#10b981",
  "#14b8a6",
  "#06b6d4",
  "#0ea5e9",
  "#3b82f6",
  "#1e3a8a",
  "#6366f1",
  "#8b5cf6",
  "#a855f7",
  "#d946ef",
  "#ec4899",
  "#f43f5e",
  "#78350f",
] as const;

export type ColorPickerProps = {
  /** Current colour. Anything that is not `#rrggbb` shows as `fallback`. */
  value: string | null | undefined;
  /** Called with a lowercase `#rrggbb` value. */
  onChange: (value: string) => void;
  /** Shown (and edited from) when `value` is empty or invalid. */
  fallback?: string;
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
  /** Classes for the swatch trigger (size, radius). Defaults to `size-9`. */
  className?: string;
  presets?: readonly string[];
  align?: "start" | "center" | "end";
};

/**
 * Swatch button that opens a colour popover: saturation area + hue slider,
 * preset swatches and a validated hex field. Replaces `<input type="color">`
 * (TEC-1275); the surrounding hex `Input` that most forms show stays as is.
 */
export function ColorPicker({
  value,
  onChange,
  fallback = "#000000",
  disabled,
  id,
  "aria-label": ariaLabel,
  className,
  presets = DEFAULT_COLOR_PRESETS,
  align = "start",
}: ColorPickerProps) {
  const { t } = useLocale();
  const current = isHexColor(value) ? value.toLowerCase() : fallback;
  const [open, setOpen] = React.useState(false);
  // Text being typed in the hex field; null shows the committed colour.
  const [draft, setDraft] = React.useState<string | null>(null);
  const hexId = React.useId();

  const emit = React.useCallback(
    (next: string) => {
      if (!isHexColor(next)) return;
      const lower = next.toLowerCase();
      if (lower !== current) onChange(lower);
    },
    [current, onChange],
  );

  const shown = draft ?? current;
  const draftValid = isHexColor(shown);

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setDraft(null);
      }}
    >
      <PopoverTrigger asChild disabled={disabled}>
        <button
          type="button"
          id={id}
          aria-label={ariaLabel ?? t("common.color_picker_open")}
          data-slot="color-picker-trigger"
          className={cn(
            "border-border focus-visible:ring-ring relative shrink-0 cursor-pointer overflow-hidden rounded-md border p-1 shadow-sm transition-shadow focus-visible:ring-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
            className ?? "size-9",
          )}
        >
          <span
            aria-hidden
            className="block size-full rounded-[inherit] border border-black/10"
            style={{ backgroundColor: current }}
          />
        </button>
      </PopoverTrigger>
      <PopoverContent align={align} className="w-64 space-y-3 p-3">
        <ColorPickerRoot
          inline
          value={current}
          onValueChange={emit}
          className="space-y-3"
        >
          <ColorPickerContent className="w-full gap-3 p-0">
            <ColorPickerArea className="h-32" />
            <ColorPickerHueSlider />
          </ColorPickerContent>
        </ColorPickerRoot>
        <div
          role="group"
          aria-label={t("common.color_picker_presets")}
          className="grid grid-cols-8 gap-1.5"
        >
          {presets.map((preset) => {
            const selected = preset.toLowerCase() === current;
            return (
              <button
                key={preset}
                type="button"
                aria-label={preset}
                aria-pressed={selected}
                onClick={() => emit(preset)}
                className={cn(
                  "focus-visible:ring-ring size-6 rounded-md border border-black/10 transition-transform hover:scale-110 focus-visible:ring-2 focus-visible:outline-none",
                  selected && "ring-primary ring-2 ring-offset-1",
                )}
                style={{ backgroundColor: preset }}
              />
            );
          })}
        </div>
        <div className="flex items-center gap-2">
          <label
            htmlFor={hexId}
            className="text-muted-foreground text-xs font-medium"
          >
            {t("common.color_picker_hex")}
          </label>
          <Input
            id={hexId}
            value={shown}
            maxLength={7}
            spellCheck={false}
            autoComplete="off"
            aria-invalid={!draftValid}
            className={cn(
              "h-8 font-mono text-xs",
              !draftValid && "border-destructive",
            )}
            onChange={(e) => {
              let next = e.target.value.trim();
              if (next && !next.startsWith("#")) next = `#${next}`;
              setDraft(next);
              emit(next);
            }}
            onBlur={() => setDraft(null)}
          />
        </div>
      </PopoverContent>
    </Popover>
  );
}
