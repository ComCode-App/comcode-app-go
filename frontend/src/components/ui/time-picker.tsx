"use client";

import * as React from "react";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils/index";
import { useLocale } from "@/providers/locale-provider";

const TIME_RE = /^([01]\d|2[0-3]):([0-5]\d)$/;

const pad = (n: number) => n.toString().padStart(2, "0");
const HOURS = Array.from({ length: 24 }, (_, i) => pad(i));

export type TimePickerProps = {
  /** `HH:mm` (24h). Anything else shows as `00:00`. */
  value: string;
  /** Called with a `HH:mm` value. */
  onChange: (value: string) => void;
  disabled?: boolean;
  /** Minute granularity (1, 5, 15…). The current minute always stays listed. */
  minuteStep?: number;
  size?: "default" | "sm";
  className?: string;
  id?: string;
};

/**
 * Hour + minute selects that replace `<input type="time">` (TEC-1275).
 * Values stay `HH:mm`, the same shape the native field produced.
 */
export function TimePicker({
  value,
  onChange,
  disabled,
  minuteStep = 1,
  size = "default",
  className,
  id,
}: TimePickerProps) {
  const { t } = useLocale();
  const match = TIME_RE.exec(value);
  const hour = match?.[1] ?? "00";
  const minute = match?.[2] ?? "00";

  const minutes = React.useMemo(() => {
    const step = Math.max(1, Math.floor(minuteStep));
    const list = Array.from({ length: Math.ceil(60 / step) }, (_, i) =>
      pad(i * step),
    );
    return list.includes(minute) ? list : [...list, minute].sort();
  }, [minuteStep, minute]);

  const triggerClass = "w-[4.5rem] gap-1 px-2.5 font-mono tabular-nums";

  return (
    <div
      role="group"
      aria-label={t("common.time_picker_label")}
      data-slot="time-picker"
      className={cn("inline-flex items-center gap-1", className)}
    >
      <Select
        value={hour}
        disabled={disabled}
        onValueChange={(h) => onChange(`${h}:${minute}`)}
      >
        <SelectTrigger
          id={id}
          size={size}
          aria-label={t("common.time_picker_hour")}
          className={triggerClass}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent className="max-h-64">
          {HOURS.map((h) => (
            <SelectItem key={h} value={h} className="font-mono">
              {h}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <span aria-hidden className="text-muted-foreground text-sm">
        :
      </span>
      <Select
        value={minute}
        disabled={disabled}
        onValueChange={(m) => onChange(`${hour}:${m}`)}
      >
        <SelectTrigger
          size={size}
          aria-label={t("common.time_picker_minute")}
          className={triggerClass}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent className="max-h-64">
          {minutes.map((m) => (
            <SelectItem key={m} value={m} className="font-mono">
              {m}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
