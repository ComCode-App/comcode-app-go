"use client";

import { PricingFrequencyTab } from "@/features/billing/components/pricing-frequency-tab";

type PricingFrequency = {
  id: string;
  label: string;
  saveBadge?: string;
};

type PricingHeaderProps = {
  title: string;
  subtitle: string;
  frequencies: PricingFrequency[];
  selectedFrequency: string;
  onFrequencyChange: (frequency: string) => void;
};

export function PricingHeader({
  title,
  subtitle,
  frequencies,
  selectedFrequency,
  onFrequencyChange,
}: PricingHeaderProps) {
  return (
    <div className="space-y-7 text-center">
      <div className="space-y-4">
        <h2 className="text-3xl font-semibold tracking-tight md:text-4xl">
          {title}
        </h2>
        <p className="text-muted-foreground mx-auto max-w-2xl text-sm md:text-base">
          {subtitle}
        </p>
      </div>
      {frequencies.length > 1 ? (
        <div className="bg-muted mx-auto flex w-fit rounded-full p-1">
          {frequencies.map((frequency) => (
            <PricingFrequencyTab
              key={frequency.id}
              label={frequency.label}
              selected={selectedFrequency === frequency.id}
              onSelect={() => onFrequencyChange(frequency.id)}
              saveBadge={frequency.saveBadge}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
