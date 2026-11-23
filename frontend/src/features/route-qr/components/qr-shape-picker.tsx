"use client";

import { useMemo } from "react";

import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

import { shapeIconPath } from "../lib/qr-engine";
import { TooltipHint } from "@/components/ui/tooltip";

type ShapeKind = "body" | "frame" | "ball";

/** A grid of shape swatches; each icon is drawn by the design engine itself. */
export function QrShapePicker({
  kind,
  shapes,
  value,
  onChange,
  disabled,
  label,
}: {
  kind: ShapeKind;
  shapes: readonly string[];
  value: string;
  onChange: (shape: string) => void;
  disabled?: boolean;
  label: string;
}) {
  const { t } = useLocale();
  const icons = useMemo(
    () => shapes.map((s) => [s, shapeIconPath(kind, s)] as const),
    [kind, shapes],
  );
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="grid grid-cols-5 gap-1.5 sm:grid-cols-6"
    >
      {icons.map(([shape, d]) => {
        const name = t(`routes.qr.shape_${kind}_${shape}`);
        const active = value === shape;
        return (
          <TooltipHint key={shape} label={name}>
            <button
              type="button"
              role="radio"
              aria-checked={active}
              aria-label={name}
              disabled={disabled}
              onClick={() => onChange(shape)}
              className={cn(
                "bg-background flex aspect-square items-center justify-center rounded-md border p-1.5 transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                active
                  ? "border-primary ring-primary/30 ring-2"
                  : "hover:bg-muted",
              )}
            >
              <svg viewBox="-0.5 -0.5 8 8" className="size-full" aria-hidden>
                <path d={d} fill="currentColor" />
              </svg>
            </button>
          </TooltipHint>
        );
      })}
    </div>
  );
}
