"use client";

import { motion } from "motion/react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type PricingFrequencyTabProps = {
  label: string;
  selected: boolean;
  onSelect: () => void;
  saveBadge?: string;
};

export function PricingFrequencyTab({
  label,
  selected,
  onSelect,
  saveBadge,
}: PricingFrequencyTabProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "relative w-fit px-4 py-2 text-sm font-semibold capitalize text-foreground transition-colors",
        saveBadge && "flex items-center justify-center gap-2.5",
      )}
    >
      <span className="relative z-10">{label}</span>
      {selected ? (
        <motion.span
          layoutId="billing-pricing-frequency"
          transition={{ type: "spring", duration: 0.4 }}
          className="absolute inset-0 z-0 rounded-full bg-background shadow-sm"
        />
      ) : null}
      {saveBadge ? (
        <Badge
          variant="secondary"
          className={cn(
            "relative z-10 border-0 text-[10px] shadow-none",
            selected
              ? "bg-accent text-accent-foreground"
              : "bg-muted text-muted-foreground",
          )}
        >
          {saveBadge}
        </Badge>
      ) : null}
    </button>
  );
}
